package scanner

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v72/github"
)

// newRateLimitError creates a *github.RateLimitError suitable for mock testing.
func newRateLimitError() *github.RateLimitError {
	return &github.RateLimitError{
		Rate: github.Rate{
			Limit:     5000,
			Remaining: 0,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
		Response: &http.Response{StatusCode: http.StatusForbidden},
		Message:  "API rate limit exceeded",
	}
}

func TestScan_SkipsArchivedRepos(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "active-repo", Description: "Active", Archived: false},
			{Name: "old-repo", Description: "Old", Archived: true},
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
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

	results, err := scanWithClient(context.Background(), client, "test-org")
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

	results, err := scanWithClient(context.Background(), client, "test-org")
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

	_, err := scanWithClient(context.Background(), client, "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScan_UsesRulesetsWhenAvailable(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "modern-repo", Description: "Uses rulesets", DefaultBranch: "main"},
		},
		Rulesets: map[string]*BranchProtection{
			"modern-repo": {RequiredReviewers: 2},
		},
		Protection: map[string]*BranchProtection{
			"modern-repo": {RequiredReviewers: 1},
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use rulesets (2 reviewers), not classic (1 reviewer)
	for _, r := range results[0].Results {
		if r.RuleName == "Has required reviewers" && !r.Passed {
			t.Error("expected pass from rulesets")
		}
	}
}

func TestScan_FallsBackToClassicProtection(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "legacy-repo", Description: "Uses classic", DefaultBranch: "main"},
		},
		// No rulesets - returns nil
		Protection: map[string]*BranchProtection{
			"legacy-repo": {RequiredReviewers: 1},
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range results[0].Results {
		if r.RuleName == "Has branch protection" && !r.Passed {
			t.Error("expected pass from classic branch protection fallback")
		}
	}
}

func TestScan_SkipsEmptyRepo(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "empty-repo", Description: "Empty", DefaultBranch: "main"},
			{Name: "normal-repo", Description: "Normal", DefaultBranch: "main"},
		},
		TreeErrs: map[string]error{
			"empty-repo": ErrEmptyRepo,
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Results sorted alphabetically: empty-repo first
	if !results[0].Skipped() || results[0].KnownSkipReason != "repository is empty" {
		t.Errorf("expected empty-repo to be skipped with known reason, got %+v", results[0])
	}
	if results[1].Skipped() {
		t.Errorf("expected normal-repo to not be skipped")
	}
	if len(results[1].Results) == 0 {
		t.Error("expected normal-repo to have rule results")
	}
}

func TestScan_SkipsTruncatedTree(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "huge-repo", Description: "Huge", DefaultBranch: "main"},
		},
		TreeErrs: map[string]error{
			"huge-repo": ErrTruncatedTree,
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped() || results[0].KnownSkipReason == "" {
		t.Error("expected huge-repo to be skipped with known reason")
	}
}

func TestScan_SkipsUnexpectedGetTreeError(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "broken-repo", Description: "Broken", DefaultBranch: "main"},
			{Name: "good-repo", Description: "Good", DefaultBranch: "main"},
		},
		TreeErrs: map[string]error{
			"broken-repo": fmt.Errorf("get tree for broken-repo: status 500"),
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Sorted: broken-repo first
	if !results[0].Skipped() || results[0].UnknownSkipError == "" {
		t.Errorf("expected broken-repo to be skipped with unknown error, got %+v", results[0])
	}
	if results[1].Skipped() {
		t.Error("expected good-repo to not be skipped")
	}
}

func TestScan_SkipsUnexpectedRulesetsError(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "broken-repo", Description: "Broken", DefaultBranch: "main"},
			{Name: "good-repo", Description: "Good", DefaultBranch: "main"},
		},
		RulesetsErrs: map[string]error{
			"broken-repo": fmt.Errorf("get rulesets: status 500"),
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !results[0].Skipped() || results[0].UnknownSkipError == "" {
		t.Errorf("expected broken-repo to be skipped, got %+v", results[0])
	}
	if results[1].Skipped() {
		t.Error("expected good-repo to not be skipped")
	}
}

func TestScan_SkipsUnexpectedBranchProtectionError(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "broken-repo", Description: "Broken", DefaultBranch: "main"},
		},
		// No rulesets -> falls through to classic protection
		ProtectionErrs: map[string]error{
			"broken-repo": fmt.Errorf("get protection: status 500"),
		},
	}

	results, err := scanWithClient(context.Background(), client, "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Skipped() || results[0].UnknownSkipError == "" {
		t.Errorf("expected broken-repo to be skipped, got %+v", results[0])
	}
}

func TestScan_AbortsOnRateLimitDuringGetTree(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "repo-a", Description: "A", DefaultBranch: "main"},
		},
		TreeErr: newRateLimitError(),
	}

	_, err := scanWithClient(context.Background(), client, "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScan_AbortsOnRateLimitDuringGetRulesets(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "repo-a", Description: "A", DefaultBranch: "main"},
		},
		RulesetsErr: newRateLimitError(),
	}

	_, err := scanWithClient(context.Background(), client, "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestScan_AbortsOnRateLimitDuringGetBranchProtection(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "repo-a", Description: "A", DefaultBranch: "main"},
		},
		ProtectionErr: newRateLimitError(),
	}

	_, err := scanWithClient(context.Background(), client, "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
