package scanner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
)

// Config holds the configuration needed to run a scan.
type Config struct {
	Org        string
	Token      string
	ReportRepo string
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

// Run is the high-level entry point. It constructs a client, scans the org,
// generates a Markdown report, and posts it as a GitHub Issue.
func Run(ctx context.Context, cfg Config) error {
	client := NewGitHubClient(cfg.Token)

	results, err := Scan(ctx, client, cfg.Org)
	if err != nil {
		return fmt.Errorf("scan org %s: %w", cfg.Org, err)
	}

	scanned := 0
	skipped := 0
	for _, r := range results {
		if r.Skipped() {
			skipped++
		} else {
			scanned++
		}
	}
	log.Printf("scanned %d repos in org %s (%d skipped)", scanned, cfg.Org, skipped)

	report := GenerateReport(cfg.Org, results)

	title := fmt.Sprintf("Codatus - %s Compliance Report", cfg.Org)
	if err := client.CreateIssue(ctx, cfg.Org, cfg.ReportRepo, title, report); err != nil {
		return fmt.Errorf("post report to %s/%s: %w", cfg.Org, cfg.ReportRepo, err)
	}

	log.Printf("report posted to %s", cfg.ReportRepo)
	return nil
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

// Scan lists all non-archived repos in the org and evaluates every rule against each.
func Scan(ctx context.Context, client GitHubClient, org string) ([]RepoResult, error) {
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
