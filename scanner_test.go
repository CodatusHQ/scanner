package scanner

import (
	"context"
	"fmt"
	"testing"
)

func TestScan_SkipsArchivedRepos(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "active-repo", Description: "Active", Archived: false},
			{Name: "old-repo", Description: "Old", Archived: true},
		},
	}

	results, err := Scan(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RepoName != "active-repo" {
		t.Errorf("expected active-repo, got %s", results[0].RepoName)
	}
}

func TestScan_ResultsSortedAlphabetically(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "zebra", Description: "Z"},
			{Name: "alpha", Description: "A"},
			{Name: "middle", Description: "M"},
		},
	}

	results, err := Scan(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"alpha", "middle", "zebra"}
	for i, name := range expected {
		if results[i].RepoName != name {
			t.Errorf("position %d: expected %s, got %s", i, name, results[i].RepoName)
		}
	}
}

func TestScan_EvaluatesRulesPerRepo(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "with-desc", Description: "Has a description"},
			{Name: "no-desc", Description: ""},
		},
	}

	results, err := Scan(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Results are sorted alphabetically: no-desc first, then with-desc
	noDesc := results[0]
	withDesc := results[1]

	if noDesc.RepoName != "no-desc" {
		t.Fatalf("expected no-desc first, got %s", noDesc.RepoName)
	}
	if withDesc.RepoName != "with-desc" {
		t.Fatalf("expected with-desc second, got %s", withDesc.RepoName)
	}

	if noDesc.Results[0].Passed {
		t.Errorf("expected no-desc to fail 'Has repo description'")
	}
	if !withDesc.Results[0].Passed {
		t.Errorf("expected with-desc to pass 'Has repo description'")
	}
}

func TestScan_PropagatesClientError(t *testing.T) {
	client := &MockGitHubClient{
		Err: fmt.Errorf("API rate limit exceeded"),
	}

	_, err := Scan(context.Background(), client, "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
