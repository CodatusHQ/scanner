package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Config holds the configuration needed to run a scan.
type Config struct {
	Org   string
	Token string
}

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

// Scan lists all non-archived repos in the org and evaluates every rule against each.
func Scan(ctx context.Context, cfg Config, opts ...Option) ([]RepoResult, error) {
	o := scanOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	client := newGitHubClient(cfg.Token, o.baseURL)
	return scanWithClient(ctx, client, cfg.Org)
}

// skipReasonForError returns a human-readable skip reason and an optional raw
// error string for unexpected failures. Known errors get a clean reason with no
// error detail. Unknown errors get a generic reason with the error message.
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
func scanWithClient(ctx context.Context, client GitHubClient, org string) ([]RepoResult, error) {
	repos, err := client.ListRepos(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("list repos for org %s: %w", org, err)
	}

	rules := AllRules()
	var results []RepoResult

	for _, repo := range repos {
		if repo.Archived {
			continue
		}

		files, err := client.GetTree(ctx, org, repo.Name, repo.DefaultBranch)
		if err != nil {
			if isRateLimitError(err) {
				return nil, fmt.Errorf("get tree for repo %s: %w", repo.Name, err)
			}
			results = append(results, skipRepo(repo.Name, err))
			continue
		}
		repo.Files = files

		protection, err := client.GetRulesets(ctx, org, repo.Name, repo.DefaultBranch)
		if err != nil {
			if isRateLimitError(err) {
				return nil, fmt.Errorf("get rulesets for repo %s: %w", repo.Name, err)
			}
			results = append(results, skipRepo(repo.Name, err))
			continue
		}
		if protection == nil {
			protection, err = client.GetBranchProtection(ctx, org, repo.Name, repo.DefaultBranch)
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
