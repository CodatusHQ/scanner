package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Auth identifies how the scanner authenticates to GitHub. It is a sealed
// interface — only PATAuth and InstallationAuth in this package satisfy it.
// New auth types are added by defining a struct with an isAuth() method.
type Auth interface {
	isAuth()
}

// PATAuth uses a Personal Access Token targeting a named account. Scanner
// lists repositories via /orgs/{Name}/repos and falls back to
// /users/{Name}/repos on 404, so it works for both org and user accounts.
type PATAuth struct {
	Token string
	Name  string // org or user login to scan
}

// InstallationAuth uses a GitHub App installation access token. Scanner
// lists repositories via /installation/repositories, which returns exactly
// the repos the installation was granted access to (no public-repo leak
// on "Selected repositories" installs).
type InstallationAuth struct {
	Token string
	Name  string // org or user login the app is installed on (used in repo URLs)
}

func (PATAuth) isAuth()          {}
func (InstallationAuth) isAuth() {}

// RepoResult holds all rule results for a single repository.
// KnownSkipReason and UnknownSkipError are mutually exclusive.
type RepoResult struct {
	RepoName         string
	MostRecentCommit time.Time // PushedAt from the listing; zero if unknown
	Results          []RuleResult
	KnownSkipReason  string
	UnknownSkipError string
}

func (rr RepoResult) Skipped() bool {
	return rr.KnownSkipReason != "" || rr.UnknownSkipError != ""
}

// ScanResult bundles the scan outcome with the listing-time exclusion counts
// the scanner accumulates while filtering archived and forked repos. The
// counts let callers report a full breakdown ("32 total, 4 forks excluded,
// 2 archived excluded, 26 scanned") without re-querying GitHub.
//
// The library does not expose a precomputed "most recent commit across the
// org" — each RepoResult carries its own MostRecentCommit and consumers
// aggregate as needed.
type ScanResult struct {
	Org              string
	ScannedAt        time.Time
	TotalRepos       int          // total repos returned by GitHub before any filtering
	ArchivedExcluded int          // archived repos filtered out at listing time
	ForksExcluded    int          // forked repos filtered out at listing time
	Skipped          []RepoResult // empty repos, truncated trees, or unexpected errors during the scan
	Results          []RepoResult // repos that finished scanning (success or fail per-rule)
}

// Score computes the org-level score: the arithmetic mean of pass rates
// across the scored rules in sr.Results. Returns the score (0-100) and a
// flag indicating whether it's defined. When sr has no scanned repos,
// defined=false and the caller should render "N/A". Result is rounded
// to the nearest integer for display.
func Score(sr ScanResult) (score int, defined bool) {
	if len(sr.Results) == 0 {
		return 0, false
	}
	scored := ScoredRules()
	if len(scored) == 0 {
		return 0, false
	}
	totalPassRate := 0
	for _, rule := range scored {
		passing := 0
		for _, rr := range sr.Results {
			for _, res := range rr.Results {
				if res.RuleName == rule.Name() && res.Passed {
					passing++
					break
				}
			}
		}
		totalPassRate += passing * 100 / len(sr.Results)
	}
	return totalPassRate / len(scored), true
}

// Bucket classifies a repo by what fraction of the scored rules it passes.
// Each bucket covers an integer percentage range; the full bucket set
// returned by Buckets() covers [0, 100] without gaps or overlaps. Display
// labels are derived from MinPct/MaxPct at render time (see report.go).
type Bucket struct {
	Name   string // "Strong", "Moderate", "Weak"
	MinPct int    // inclusive lower bound (0..100)
	MaxPct int    // inclusive upper bound (0..100)
}

// Buckets returns the score-range buckets in display order (highest range
// first). Adding/removing buckets, renaming them, or shifting thresholds
// is a one-place edit here - report and stats output both derive from this
// list and need no separate updates.
func Buckets() []Bucket {
	return []Bucket{
		{Name: "Strong", MinPct: 80, MaxPct: 100},
		{Name: "Moderate", MinPct: 40, MaxPct: 79},
		{Name: "Weak", MinPct: 0, MaxPct: 39},
	}
}

// BucketOf classifies a single repo by the percentage of scored rules it
// passes. Returns the matching Bucket plus the underlying counts so callers
// don't re-derive them. If there are zero scored rules the result is the
// last-defined bucket (i.e. Weak) with zero counts; this only happens in
// test fixtures with no scored rules registered.
func BucketOf(rr RepoResult) (b Bucket, scoredPassing, scoredTotal, scorePct int) {
	scored := ScoredRules()
	scoredTotal = len(scored)
	scoredNames := make(map[string]bool, scoredTotal)
	for _, r := range scored {
		scoredNames[r.Name()] = true
	}
	for _, res := range rr.Results {
		if scoredNames[res.RuleName] && res.Passed {
			scoredPassing++
		}
	}
	defs := Buckets()
	if scoredTotal == 0 {
		return defs[len(defs)-1], 0, 0, 0
	}
	scorePct = scoredPassing * 100 / scoredTotal
	for _, def := range defs {
		if scorePct >= def.MinPct && scorePct <= def.MaxPct {
			return def, scoredPassing, scoredTotal, scorePct
		}
	}
	return defs[len(defs)-1], scoredPassing, scoredTotal, scorePct
}

// scanOptions holds optional parameters configurable via functional options.
type scanOptions struct {
	baseURL string
}

// Option configures optional scan behavior.
type Option func(*scanOptions)

// WithBaseURL sets a custom GitHub API base URL.
// Defaults to the public GitHub API when unset. Useful for testing against
// a mock server or pointing at a GitHub Enterprise instance.
func WithBaseURL(url string) Option {
	return func(o *scanOptions) { o.baseURL = url }
}

// Scan lists repositories accessible to auth and evaluates every rule
// against each non-archived, non-forked repo. Forks and archived repos
// are excluded at listing time and surface in the returned ScanResult's
// ForksExcluded / ArchivedExcluded counts.
func Scan(ctx context.Context, auth Auth, opts ...Option) (ScanResult, error) {
	o := scanOptions{}
	for _, opt := range opts {
		opt(&o)
	}

	var token string
	switch a := auth.(type) {
	case PATAuth:
		token = a.Token
	case InstallationAuth:
		token = a.Token
	default:
		return ScanResult{}, fmt.Errorf("unsupported auth type: %T", auth)
	}

	client := newGitHubClient(token, o.baseURL)
	return scanWithClient(ctx, client, auth)
}

// skipRepo converts a per-repo error into a RepoResult that records the
// skip reason. Known errors get a clean reason; unknown errors get a
// generic reason plus the raw error message. Carries PushedAt forward as
// MostRecentCommit so consumers aggregating org-level activity can include
// skipped repos (a repo can be empty/truncated and still have recent pushes).
func skipRepo(repo Repo, err error) RepoResult {
	rr := RepoResult{
		RepoName:         repo.Name,
		MostRecentCommit: repo.PushedAt,
	}
	switch {
	case errors.Is(err, ErrEmptyRepo):
		rr.KnownSkipReason = "repository is empty"
	case errors.Is(err, ErrTruncatedTree):
		rr.KnownSkipReason = "file tree too large (truncated by GitHub API)"
	default:
		rr.UnknownSkipError = err.Error()
	}
	return rr
}

// scanWithClient is the internal scan loop used by both the public Scan
// (which constructs a real client) and by tests (which pass a mock client).
// Listing strategy depends on the auth type.
func scanWithClient(ctx context.Context, client GitHubClient, auth Auth) (ScanResult, error) {
	var repos []Repo
	var owner string

	switch a := auth.(type) {
	case PATAuth:
		r, err := client.ListReposByAccount(ctx, a.Name)
		if err != nil {
			return ScanResult{}, fmt.Errorf("list repos for %s: %w", a.Name, err)
		}
		repos, owner = r, a.Name
	case InstallationAuth:
		r, err := client.ListReposByInstallation(ctx)
		if err != nil {
			return ScanResult{}, fmt.Errorf("list installation repos: %w", err)
		}
		repos, owner = r, a.Name
	default:
		return ScanResult{}, fmt.Errorf("unsupported auth type: %T", auth)
	}

	sr := ScanResult{
		Org:        owner,
		ScannedAt:  time.Now().UTC(),
		TotalRepos: len(repos),
	}

	rules := AllRules()
	var allResults []RepoResult

	for _, repo := range repos {
		if repo.Archived {
			sr.ArchivedExcluded++
			continue
		}
		if repo.Fork {
			sr.ForksExcluded++
			continue
		}

		files, err := client.GetTree(ctx, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			if IsRateLimitError(err) {
				return ScanResult{}, fmt.Errorf("get tree for repo %s: %w", repo.Name, err)
			}
			allResults = append(allResults, skipRepo(repo, err))
			continue
		}
		repo.Files = files

		protection, err := client.GetRulesets(ctx, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			if IsRateLimitError(err) {
				return ScanResult{}, fmt.Errorf("get rulesets for repo %s: %w", repo.Name, err)
			}
			allResults = append(allResults, skipRepo(repo, err))
			continue
		}
		if protection == nil {
			protection, err = client.GetBranchProtection(ctx, owner, repo.Name, repo.DefaultBranch)
			if err != nil {
				if IsRateLimitError(err) {
					return ScanResult{}, fmt.Errorf("get branch protection for repo %s: %w", repo.Name, err)
				}
				allResults = append(allResults, skipRepo(repo, err))
				continue
			}
		}
		repo.BranchProtection = protection

		rr := RepoResult{
			RepoName:         repo.Name,
			MostRecentCommit: repo.PushedAt,
		}
		for _, rule := range rules {
			rr.Results = append(rr.Results, RuleResult{
				RuleName: rule.Name(),
				Passed:   rule.Check(repo),
			})
		}
		allResults = append(allResults, rr)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].RepoName < allResults[j].RepoName
	})

	for _, rr := range allResults {
		if rr.Skipped() {
			sr.Skipped = append(sr.Skipped, rr)
		} else {
			sr.Results = append(sr.Results, rr)
		}
	}

	return sr, nil
}
