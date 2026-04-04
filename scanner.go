package main

import (
	"context"
	"fmt"
	"sort"
)

// RepoResult holds all rule results for a single repository.
type RepoResult struct {
	RepoName string
	Results  []RuleResult
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
			rr.Results = append(rr.Results, rule.Check(repo))
		}
		results = append(results, rr)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RepoName < results[j].RepoName
	})

	return results, nil
}
