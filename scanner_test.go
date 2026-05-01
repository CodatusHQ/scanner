package scanner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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

// allResults concatenates the scanned + skipped repos from a ScanResult,
// sorted alphabetically by repo name. Convenient for tests that don't care
// about the scanned/skipped distinction and just want to assert against a
// flat ordered list.
func allResults(sr ScanResult) []RepoResult {
	out := make([]RepoResult, 0, len(sr.Results)+len(sr.Skipped))
	out = append(out, sr.Results...)
	out = append(out, sr.Skipped...)
	sort.Slice(out, func(i, j int) bool { return out[i].RepoName < out[j].RepoName })
	return out
}

func TestScan_SkipsArchivedRepos(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "active-repo", Description: "Active", Archived: false},
			{Name: "old-repo", Description: "Old", Archived: true},
		},
	}

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(sr.Results))
	}
	if sr.Results[0].RepoName != "active-repo" {
		t.Errorf("expected active-repo, got %s", sr.Results[0].RepoName)
	}
	if sr.ArchivedExcluded != 1 {
		t.Errorf("expected ArchivedExcluded=1, got %d", sr.ArchivedExcluded)
	}
	if sr.TotalRepos != 2 {
		t.Errorf("expected TotalRepos=2, got %d", sr.TotalRepos)
	}
}

func TestScan_SkipsForkedRepos(t *testing.T) {
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "own-repo", Description: "Ours", Fork: false},
			{Name: "forked-repo", Description: "Fork", Fork: true},
		},
	}

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Results) != 1 {
		t.Fatalf("expected 1 scanned, got %d", len(sr.Results))
	}
	if sr.Results[0].RepoName != "own-repo" {
		t.Errorf("expected own-repo, got %s", sr.Results[0].RepoName)
	}
	if sr.ForksExcluded != 1 {
		t.Errorf("expected ForksExcluded=1, got %d", sr.ForksExcluded)
	}
}

func TestScan_PerRepoMostRecentCommit(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "stale", PushedAt: older},
			{Name: "fresh", PushedAt: newer},
		},
	}

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Results sorted alphabetically: fresh, stale.
	if len(sr.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(sr.Results))
	}
	if !sr.Results[0].MostRecentCommit.Equal(newer) {
		t.Errorf("fresh: expected MostRecentCommit=%v, got %v", newer, sr.Results[0].MostRecentCommit)
	}
	if !sr.Results[1].MostRecentCommit.Equal(older) {
		t.Errorf("stale: expected MostRecentCommit=%v, got %v", older, sr.Results[1].MostRecentCommit)
	}
}

func TestScan_PerRepoMostRecentCommit_Skipped(t *testing.T) {
	pushedAt := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	client := &MockGitHubClient{
		Repos: []Repo{
			{Name: "empty-repo", DefaultBranch: "main", PushedAt: pushedAt},
		},
		TreeErrs: map[string]error{"empty-repo": ErrEmptyRepo},
	}

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(sr.Skipped))
	}
	// Skipped repos still carry their listing-time PushedAt forward so
	// consumers aggregating org-level activity don't lose the signal.
	if !sr.Skipped[0].MostRecentCommit.Equal(pushedAt) {
		t.Errorf("expected skipped repo MostRecentCommit=%v, got %v", pushedAt, sr.Skipped[0].MostRecentCommit)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"alpha", "middle", "zebra"}
	for i, name := range expected {
		if sr.Results[i].RepoName != name {
			t.Errorf("position %d: expected %s, got %s", i, name, sr.Results[i].RepoName)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(sr.Results))
	}

	// Results are sorted alphabetically: no-desc first, then with-desc
	noDesc := sr.Results[0]
	withDesc := sr.Results[1]

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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use rulesets (2 reviewers), not classic (1 reviewer)
	for _, r := range sr.Results[0].Results {
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range sr.Results[0].Results {
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := allResults(sr)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := allResults(sr)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := allResults(sr)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := allResults(sr)
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := allResults(sr)
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}


// Exercises the public Scan() entry point end-to-end against httptest to
// verify that PATAuth hits /orgs/{name}/repos and InstallationAuth hits
// /installation/repositories. This is the only place the actual dispatch
// in Scan() is covered — scanner_test.go elsewhere calls scanWithClient
// directly against a mock.
func TestScan_DispatchesByAuthType(t *testing.T) {
	type hits struct {
		orgEndpoint          bool
		installationEndpoint bool
	}

	newServer := func(t *testing.T, h *hits) string {
		t.Helper()
		mux := http.NewServeMux()
		mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
			h.orgEndpoint = true
			fmt.Fprint(w, `[]`)
		})
		mux.HandleFunc("/installation/repositories", func(w http.ResponseWriter, r *http.Request) {
			h.installationEndpoint = true
			fmt.Fprint(w, `{"total_count":0, "repositories":[]}`)
		})
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		return server.URL
	}

	t.Run("PATAuth calls org endpoint", func(t *testing.T) {
		h := &hits{}
		url := newServer(t, h)
		_, err := Scan(context.Background(), PATAuth{Token: "pat", Name: "acme"}, WithBaseURL(url))
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if !h.orgEndpoint {
			t.Error("expected org endpoint to be called for PATAuth")
		}
		if h.installationEndpoint {
			t.Error("installation endpoint must not be called for PATAuth")
		}
	})

	t.Run("InstallationAuth calls installation endpoint", func(t *testing.T) {
		h := &hits{}
		url := newServer(t, h)
		_, err := Scan(context.Background(), InstallationAuth{Token: "ghs_x", Name: "acme"}, WithBaseURL(url))
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}
		if !h.installationEndpoint {
			t.Error("expected installation endpoint to be called for InstallationAuth")
		}
		if h.orgEndpoint {
			t.Error("org endpoint must not be called for InstallationAuth")
		}
	})
}
