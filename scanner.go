package scanner

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
)

// Config holds the configuration needed to run a scan.
type Config struct {
	Org        string
	Token      string
	ReportRepo string
}

// RepoResult holds all rule results for a single repository.
type RepoResult struct {
	RepoName string
	Results  []RuleResult
}

// Run is the high-level entry point. It constructs a client, scans the org,
// generates a Markdown report, and posts it as a GitHub Issue.
func Run(ctx context.Context, cfg Config) error {
	client := NewGitHubClient(cfg.Token)

	results, err := Scan(ctx, client, cfg.Org)
	if err != nil {
		return fmt.Errorf("scan org %s: %w", cfg.Org, err)
	}

	log.Printf("scanned %d repos in org %s", len(results), cfg.Org)

	report := GenerateReport(cfg.Org, results)

	owner, repo, err := parseReportRepo(cfg.ReportRepo)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("Codatus — %s Compliance Report", cfg.Org)
	if err := client.CreateIssue(ctx, owner, repo, title, report); err != nil {
		return fmt.Errorf("post report to %s: %w", cfg.ReportRepo, err)
	}

	log.Printf("report posted to %s", cfg.ReportRepo)
	return nil
}

func parseReportRepo(reportRepo string) (string, string, error) {
	parts := strings.SplitN(reportRepo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid report repo format %q: expected owner/repo", reportRepo)
	}
	return parts[0], parts[1], nil
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
