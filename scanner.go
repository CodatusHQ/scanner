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

	// RulesScored and RulesAdditional are the rules actually run against
	// each repo, split by category. They reflect WithAdmin filtering: an
	// admin-only rule skipped on a non-admin scan does NOT appear here,
	// so all downstream math (Score, BucketOf, table aggregation) is
	// driven directly by these slices instead of inferring evaluated
	// rules from RepoResult.Results.
	//
	// JSON-tagged "-" because Rule is an interface and consumers that
	// marshal a ScanResult should instead build their own per-rule
	// payload (see cmd/bulk-scan for an example). The fields are stable
	// for in-process use only.
	RulesScored     []Rule `json:"-"`
	RulesAdditional []Rule `json:"-"`
}

// Score computes the org-level score: the arithmetic mean of pass rates
// across sr.RulesScored. Returns the score (0-100) and a flag indicating
// whether it's defined. When sr has no scanned repos OR no scored rules
// were evaluated (e.g., a non-admin scan with admin-only rules filtered
// out and no scored rules left), defined=false and the caller should
// render "N/A". Result is rounded to the nearest integer for display.
//
// The denominator is len(sr.RulesScored), not the size of the global
// scored-rule set - that's how non-admin scans get the math right
// without rules they couldn't evaluate dragging the score down.
func Score(sr ScanResult) (score int, defined bool) {
	if len(sr.Results) == 0 || len(sr.RulesScored) == 0 {
		return 0, false
	}
	totalPassRate := 0
	for _, rule := range sr.RulesScored {
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
	return totalPassRate / len(sr.RulesScored), true
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
		{Name: "Moderate", MinPct: 30, MaxPct: 79},
		{Name: "Weak", MinPct: 0, MaxPct: 29},
	}
}

// BucketOf classifies a single repo by the percentage of scored rules it
// passes. The caller passes the scored-rule set so the denominator is
// stable across the org's scan: every repo gets the same denominator,
// regardless of which rules happen to appear in any one repo's results.
// Pass sr.RulesScored from the parent ScanResult.
//
// Returns the matching Bucket plus the underlying counts so callers
// don't re-derive them. If scoredRules is empty the result is the
// last-defined bucket (i.e. Weak) with zero counts; this only happens
// in test fixtures with no scored rules registered.
func BucketOf(rr RepoResult, scoredRules []Rule) (b Bucket, scoredPassing, scoredTotal, scorePct int) {
	scoredTotal = len(scoredRules)
	defs := Buckets()
	if scoredTotal == 0 {
		return defs[len(defs)-1], 0, 0, 0
	}
	scoredNames := make(map[string]bool, scoredTotal)
	for _, r := range scoredRules {
		scoredNames[r.Name()] = true
	}
	for _, res := range rr.Results {
		if scoredNames[res.RuleName] && res.Passed {
			scoredPassing++
		}
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
	admin   bool
}

// Option configures optional scan behavior.
type Option func(*scanOptions)

// WithBaseURL sets a custom GitHub API base URL.
// Defaults to the public GitHub API when unset. Useful for testing against
// a mock server or pointing at a GitHub Enterprise instance.
func WithBaseURL(url string) Option {
	return func(o *scanOptions) { o.baseURL = url }
}

// WithAdmin signals that the auth has admin access on every repo it can
// see. When true, the scanner runs all rules, including those that need
// admin-only API endpoints (currently: required-reviewers visibility on
// classic per-repo branch protection). When false (the default), rules
// marked admin-only are silently skipped - they don't appear in the
// per-repo results, the JSON output, or the Markdown report. Their
// absence is invisible to downstream consumers, who simply don't see
// those keys/columns.
//
// Pass true when scanning with an installation token issued by the
// Codatus GitHub App (which is granted admin) or a PAT belonging to an
// admin of every target org. Pass false (or leave default) for
// third-party / public scans where admin signals can't be read.
func WithAdmin(b bool) Option {
	return func(o *scanOptions) { o.admin = b }
}

// adminRequiringRule is the optional interface a Rule can implement to
// signal that it depends on admin-only API responses. Rules that don't
// implement it are treated as admin-not-required (the default for every
// existing rule except HasRequiredReviewers).
type adminRequiringRule interface {
	RequiresAdmin() bool
}

// ruleRequiresAdmin reports whether a rule needs admin access. Uses a
// type assertion against adminRequiringRule so adding the requirement
// is a per-rule opt-in - new rules don't have to change the Rule
// interface to declare they're public-safe.
func ruleRequiresAdmin(r Rule) bool {
	if ar, ok := r.(adminRequiringRule); ok {
		return ar.RequiresAdmin()
	}
	return false
}

// effectiveRules returns AllRules filtered for the admin context. Public
// scans drop admin-only rules so they don't run, don't produce results,
// and don't appear anywhere downstream.
func effectiveRules(admin bool) []Rule {
	all := AllRules()
	if admin {
		return all
	}
	out := make([]Rule, 0, len(all))
	for _, r := range all {
		if !ruleRequiresAdmin(r) {
			out = append(out, r)
		}
	}
	return out
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
	return scanWithClient(ctx, client, auth, o)
}

// resolveBranchProtection asks the client for branch-protection details
// from three different APIs and merges the results into a single
// BranchProtection. Each source covers a different population of repos
// AND a different subset of fields, so we union rather than short-circuit:
//
//  1. Rulesets - publicly readable; surfaces required-reviewer counts and
//     required-status-check contexts when the org uses GitHub's modern
//     rulesets feature.
//  2. Classic per-repo protection - admin-only; full classic-protection
//     payload (reviewer counts AND status checks). Returns nil to
//     non-admin tokens (404).
//  3. Public branch info - publicly readable; exposes the protected flag
//     and the required-status-check contexts even for classic-protected
//     repos visible to non-admins. (Reviewer count is admin-only and not
//     exposed here.)
//
// Why merge instead of first-non-nil: a repo can have BOTH a ruleset
// (e.g., a pull_request rule for review enforcement) AND classic
// protection (e.g., status checks) on the same branch. Returning only
// the first non-nil source loses the other layer's data - in particular,
// non-admin scans of hybrid-configured repos would see the ruleset's
// reviewer count but miss the classic layer's status checks (which the
// public branch endpoint WOULD have surfaced if we'd kept walking).
//
// Merge semantics:
//   - RequiredReviewers: max across sources. Both layers' enforcement
//     stacks, so the effective minimum is the higher count.
//   - RequiredStatusChecks: deduplicated union. All required checks
//     across all layers must pass to merge.
//
// Returns nil when every source returns nil (no protection anywhere).
// Errors from any source short-circuit and propagate to the caller,
// which distinguishes rate-limit aborts from per-repo skips. Errors are
// wrapped with the source name for clarity.
func resolveBranchProtection(ctx context.Context, client GitHubClient, owner, repo, branch string) (*BranchProtection, error) {
	type source struct {
		name string
		fn   func(context.Context, string, string, string) (*BranchProtection, error)
	}
	sources := []source{
		{"rulesets", client.GetRulesets},
		{"branch protection", client.GetBranchProtection},
		{"branch info", client.GetBranchInfo},
	}

	var merged *BranchProtection
	seenChecks := make(map[string]bool)
	for _, src := range sources {
		bp, err := src.fn(ctx, owner, repo, branch)
		if err != nil {
			return nil, fmt.Errorf("get %s for %s/%s: %w", src.name, owner, repo, err)
		}
		if bp == nil {
			continue
		}
		if merged == nil {
			merged = &BranchProtection{}
		}
		if bp.RequiredReviewers > merged.RequiredReviewers {
			merged.RequiredReviewers = bp.RequiredReviewers
		}
		for _, c := range bp.RequiredStatusChecks {
			if !seenChecks[c] {
				seenChecks[c] = true
				merged.RequiredStatusChecks = append(merged.RequiredStatusChecks, c)
			}
		}
	}
	return merged, nil
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
func scanWithClient(ctx context.Context, client GitHubClient, auth Auth, o scanOptions) (ScanResult, error) {
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

	rules := effectiveRules(o.admin)

	// Split the effective rule set by category once so the result
	// carries it for downstream math. Done before the scan loop so even
	// a zero-repo scan still describes which rules WOULD have run.
	var rulesScored, rulesAdditional []Rule
	for _, r := range rules {
		switch r.Category() {
		case CategoryScored:
			rulesScored = append(rulesScored, r)
		case CategoryAdditional:
			rulesAdditional = append(rulesAdditional, r)
		}
	}

	sr := ScanResult{
		Org:             owner,
		ScannedAt:       time.Now().UTC(),
		TotalRepos:      len(repos),
		RulesScored:     rulesScored,
		RulesAdditional: rulesAdditional,
	}

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

		protection, err := resolveBranchProtection(ctx, client, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			if IsRateLimitError(err) {
				return ScanResult{}, err
			}
			allResults = append(allResults, skipRepo(repo, err))
			continue
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
