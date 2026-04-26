package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	Results          []RuleResult
	KnownSkipReason  string
	UnknownSkipError string
}

func (rr RepoResult) Skipped() bool {
	return rr.KnownSkipReason != "" || rr.UnknownSkipError != ""
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
// against each non-archived repo.
func Scan(ctx context.Context, auth Auth, opts ...Option) ([]RepoResult, error) {
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
		return nil, fmt.Errorf("unsupported auth type: %T", auth)
	}

	client := newGitHubClient(token, o.baseURL)
	return scanWithClient(ctx, client, auth)
}

// skipRepo converts a per-repo error into a RepoResult that records the
// skip reason. Known errors get a clean reason; unknown errors get a
// generic reason plus the raw error message.
func skipRepo(name string, err error) RepoResult {
	if errors.Is(err, ErrEmptyRepo) {
		return RepoResult{RepoName: name, KnownSkipReason: "repository is empty"}
	}
	if errors.Is(err, ErrTruncatedTree) {
		return RepoResult{RepoName: name, KnownSkipReason: "file tree too large (truncated by GitHub API)"}
	}
	return RepoResult{RepoName: name, UnknownSkipError: err.Error()}
}

// scanWithClient is the internal scan loop used by both the public Scan
// (which constructs a real client) and by tests (which pass a mock client).
// Listing strategy depends on the auth type.
func scanWithClient(ctx context.Context, client GitHubClient, auth Auth) ([]RepoResult, error) {
	var repos []Repo
	var owner string

	switch a := auth.(type) {
	case PATAuth:
		r, err := client.ListReposByAccount(ctx, a.Name)
		if err != nil {
			return nil, fmt.Errorf("list repos for %s: %w", a.Name, err)
		}
		repos, owner = r, a.Name
	case InstallationAuth:
		r, err := client.ListReposByInstallation(ctx)
		if err != nil {
			return nil, fmt.Errorf("list installation repos: %w", err)
		}
		repos, owner = r, a.Name
	default:
		return nil, fmt.Errorf("unsupported auth type: %T", auth)
	}

	rules := AllRules()
	var results []RepoResult

	for _, repo := range repos {
		if repo.Archived {
			continue
		}

		files, err := client.GetTree(ctx, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			if isRateLimitError(err) {
				return nil, fmt.Errorf("get tree for repo %s: %w", repo.Name, err)
			}
			results = append(results, skipRepo(repo.Name, err))
			continue
		}
		repo.Files = files

		protection, err := client.GetRulesets(ctx, owner, repo.Name, repo.DefaultBranch)
		if err != nil {
			if isRateLimitError(err) {
				return nil, fmt.Errorf("get rulesets for repo %s: %w", repo.Name, err)
			}
			results = append(results, skipRepo(repo.Name, err))
			continue
		}
		if protection == nil {
			protection, err = client.GetBranchProtection(ctx, owner, repo.Name, repo.DefaultBranch)
			if err != nil {
				if isRateLimitError(err) {
					return nil, fmt.Errorf("get branch protection for repo %s: %w", repo.Name, err)
				}
				results = append(results, skipRepo(repo.Name, err))
				continue
			}
		}
		repo.BranchProtection = protection

		rr := RepoResult{RepoName: repo.Name}
		for _, rule := range rules {
			rr.Results = append(rr.Results, RuleResult{
				RuleName: rule.Name(),
				Passed:   rule.Check(repo),
			})
		}
		results = append(results, rr)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RepoName < results[j].RepoName
	})

	return results, nil
}
