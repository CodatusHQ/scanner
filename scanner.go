package scanner

import (
	"context"
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
type RepoResult struct {
	RepoName string
	Results  []RuleResult
}

// Run is the high-level entry point. It constructs a client, scans, and
// (in the future) posts the report.
func Run(ctx context.Context, cfg Config) error {
	client := NewGitHubClient(cfg.Token)

	results, err := Scan(ctx, client, cfg.Org)
	if err != nil {
		return fmt.Errorf("scan org %s: %w", cfg.Org, err)
	}

	log.Printf("scanned %d repos in org %s", len(results), cfg.Org)

	// TODO: generate report and post as GitHub issue to cfg.ReportRepo
	_ = results

	return nil
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
