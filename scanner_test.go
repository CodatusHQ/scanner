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

// repoWithScoredPasses builds a RepoResult passing exactly `passing` of the
// 5 scored rules (the first `passing` of ScoredRules in importance order).
// Helper for Score / BucketOf tests; no-op for additional rules.
func repoWithScoredPasses(name string, passing int) RepoResult {
	rr := RepoResult{RepoName: name}
	scored := ScoredRules()
	for i, r := range scored {
		rr.Results = append(rr.Results, RuleResult{
			RuleName: r.Name(),
			Passed:   i < passing,
		})
	}
	return rr
}

// withDefaultRules populates sr.RulesScored / sr.RulesAdditional with the
// full global rule sets - convenient for tests that construct a synthetic
// ScanResult and don't care about WithAdmin filtering. Tests that DO care
// (e.g. TestScan_NonAdminFiltersAdminOnlyRules) construct rule slices by
// hand instead.
func withDefaultRules(sr ScanResult) ScanResult {
	sr.RulesScored = ScoredRules()
	sr.RulesAdditional = AdditionalRules()
	return sr
}

func TestScore_AverageOfScoredRulePassRates(t *testing.T) {
	// repoWithScoredPasses passes the FIRST n scored rules. With 4 repos
	// passing 5, 4, 3, 2 rules respectively, each rule's pass rate is:
	//   rule 0: 4/4 = 100%   (all 4 repos pass it)
	//   rule 1: 4/4 = 100%
	//   rule 2: 3/4 =  75%
	//   rule 3: 2/4 =  50%
	//   rule 4: 1/4 =  25%
	// Average = (100+100+75+50+25)/5 = 70
	sr := ScanResult{
		Results: []RepoResult{
			repoWithScoredPasses("a", 5),
			repoWithScoredPasses("b", 4),
			repoWithScoredPasses("c", 3),
			repoWithScoredPasses("d", 2),
		},
	}
	score, defined := Score(withDefaultRules(sr))
	if !defined {
		t.Fatal("expected score to be defined")
	}
	if score != 70 {
		t.Errorf("expected score=70, got %d", score)
	}
}

func TestScore_RoundsDown(t *testing.T) {
	// 3 repos, each passing 1 different scored rule:
	//   rule 0: 1/3 = 33%
	//   rules 1-4: 0/3 = 0%
	// Average = 33/5 = 6.6 → 6 (integer division rounds toward zero)
	a := RepoResult{RepoName: "a"}
	b := RepoResult{RepoName: "b"}
	c := RepoResult{RepoName: "c"}
	for i, r := range ScoredRules() {
		a.Results = append(a.Results, RuleResult{RuleName: r.Name(), Passed: i == 0})
		b.Results = append(b.Results, RuleResult{RuleName: r.Name(), Passed: false})
		c.Results = append(c.Results, RuleResult{RuleName: r.Name(), Passed: false})
	}
	score, _ := Score(withDefaultRules(ScanResult{Results: []RepoResult{a, b, c}}))
	if score != 6 {
		t.Errorf("expected score=6 (rounded down), got %d", score)
	}
}

func TestScore_NotDefinedWhenNoRepos(t *testing.T) {
	score, defined := Score(ScanResult{})
	if defined {
		t.Errorf("expected score not defined for empty ScanResult; got score=%d", score)
	}
}

func TestScore_OnlyScoredRulesContribute(t *testing.T) {
	// A repo passing 5/5 additional checks but 0/5 scored should yield score=0.
	rr := RepoResult{RepoName: "a"}
	for _, r := range ScoredRules() {
		rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: false})
	}
	for _, r := range AdditionalRules() {
		rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: true})
	}
	sr := ScanResult{Results: []RepoResult{rr}}
	score, defined := Score(withDefaultRules(sr))
	if !defined {
		t.Fatal("expected score to be defined")
	}
	if score != 0 {
		t.Errorf("expected score=0 (only scored rules contribute), got %d", score)
	}
}

func TestBucketOf_ByPassingCount(t *testing.T) {
	cases := []struct {
		passing      int
		wantName     string
		wantPct      int
	}{
		{0, "Weak", 0},
		{1, "Weak", 20},
		{2, "Moderate", 40},
		{3, "Moderate", 60},
		{4, "Strong", 80},
		{5, "Strong", 100},
	}
	scored := ScoredRules()
	for _, tc := range cases {
		bucket, scoredPassing, scoredTotal, scorePct := BucketOf(repoWithScoredPasses("r", tc.passing), scored)
		if bucket.Name != tc.wantName {
			t.Errorf("passing=%d: expected bucket=%s, got %s", tc.passing, tc.wantName, bucket.Name)
		}
		if scoredPassing != tc.passing {
			t.Errorf("passing=%d: expected scoredPassing=%d, got %d", tc.passing, tc.passing, scoredPassing)
		}
		if scoredTotal != 5 {
			t.Errorf("passing=%d: expected scoredTotal=5, got %d", tc.passing, scoredTotal)
		}
		if scorePct != tc.wantPct {
			t.Errorf("passing=%d: expected scorePct=%d, got %d", tc.passing, tc.wantPct, scorePct)
		}
	}
}

func TestBuckets_Definition(t *testing.T) {
	got := Buckets()
	want := []Bucket{
		{Name: "Strong", MinPct: 80, MaxPct: 100},
		{Name: "Moderate", MinPct: 30, MaxPct: 79},
		{Name: "Weak", MinPct: 0, MaxPct: 29},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d buckets, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bucket[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBuckets_CoverFullRange(t *testing.T) {
	// Every percentage from 0..100 should be classified by exactly one bucket.
	defs := Buckets()
	for pct := 0; pct <= 100; pct++ {
		matches := 0
		for _, def := range defs {
			if pct >= def.MinPct && pct <= def.MaxPct {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("pct=%d matched %d buckets, want exactly 1", pct, matches)
		}
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	// Look up "Has repo description" by name rather than positional index.
	// The position depends on AllRules order, which can change.
	if got := ruleResult(noDesc, "Has repo description"); got == nil || got.Passed {
		t.Errorf("expected no-desc to fail 'Has repo description'; got %v", got)
	}
	if got := ruleResult(withDesc, "Has repo description"); got == nil || !got.Passed {
		t.Errorf("expected with-desc to pass 'Has repo description'; got %v", got)
	}
}

// ruleResult looks up a rule's result by name in a RepoResult.
func ruleResult(rr RepoResult, ruleName string) *RuleResult {
	for i := range rr.Results {
		if rr.Results[i].RuleName == ruleName {
			return &rr.Results[i]
		}
	}
	return nil
}

func TestScan_PropagatesClientError(t *testing.T) {
	client := &MockGitHubClient{
		Err: fmt.Errorf("API rate limit exceeded"),
	}

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

	_, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{})
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

func TestScan_NonAdminFiltersAdminOnlyRules(t *testing.T) {
	// Non-admin scans (default) should skip rules that implement
	// RequiresAdmin() == true. Currently the only such rule is
	// HasRequiredReviewers; verify it doesn't appear in any RepoResult.
	client := &MockGitHubClient{
		Repos: []Repo{{Name: "repo-a", DefaultBranch: "main"}},
	}
	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{admin: false})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(sr.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(sr.Results))
	}
	for _, res := range sr.Results[0].Results {
		if res.RuleName == "Has required reviewers" {
			t.Error("expected admin-only rule HasRequiredReviewers to be omitted from non-admin scan")
		}
	}
}

func TestScan_AdminIncludesAdminOnlyRules(t *testing.T) {
	// With WithAdmin(true) every rule including admin-only ones runs.
	client := &MockGitHubClient{
		Repos: []Repo{{Name: "repo-a", DefaultBranch: "main"}},
	}
	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{admin: true})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(sr.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(sr.Results))
	}
	found := false
	for _, res := range sr.Results[0].Results {
		if res.RuleName == "Has required reviewers" {
			found = true
		}
	}
	if !found {
		t.Error("expected HasRequiredReviewers to run on admin scans")
	}
}

func TestScore_OmitsAdminOnlyRulesUnderNonAdmin(t *testing.T) {
	// Build two repos missing only HasRequiredReviewers from their results
	// (modeling a non-admin scan). With sr.RulesScored set to the 4-rule
	// subset (admin-only rule excluded), score should be the mean over
	// those 4 rules - not 5 (which would dilute by 20%).
	allScored := ScoredRules()
	var nonAdminScored []Rule
	for _, r := range allScored {
		if r.Name() != "Has required reviewers" {
			nonAdminScored = append(nonAdminScored, r)
		}
	}
	mkRepo := func(name string, passing []bool) RepoResult {
		rr := RepoResult{RepoName: name}
		for i, r := range allScored {
			if r.Name() == "Has required reviewers" {
				continue // admin-only rule omitted
			}
			rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: passing[i]})
		}
		return rr
	}
	sr := ScanResult{
		Results: []RepoResult{
			mkRepo("a", []bool{true, true, true, true, true}),    // 4/4 → 100%
			mkRepo("b", []bool{true, true, false, false, false}), // 1/4 (skipping required-reviewers) → 25%
		},
		RulesScored: nonAdminScored,
	}
	score, defined := Score(sr)
	if !defined {
		t.Fatal("expected defined score")
	}
	// Per-rule pass rates over the 4 evaluated rules:
	//   has_branch_protection: 2/2 = 100
	//   requires_status_checks: 1/2 = 50
	//   has_codeowners: 1/2 = 50
	//   has_ci_workflow: 1/2 = 50
	// Mean = (100+50+50+50)/4 = 62
	if score != 62 {
		t.Errorf("expected score=62, got %d", score)
	}
}

func TestBucketOf_DenominatorIsEvaluatedRules(t *testing.T) {
	// With HasRequiredReviewers omitted from sr.RulesScored (non-admin
	// scan), a repo passing 3 of the 4 remaining scored rules should
	// land at 75% - not 60% (which would be 3/5 if the missing rule
	// were still in the denominator).
	var scored []Rule
	for _, r := range ScoredRules() {
		if r.Name() == "Has required reviewers" {
			continue
		}
		scored = append(scored, r)
	}
	rr := RepoResult{RepoName: "a"}
	for _, r := range scored {
		rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: false})
	}
	// Mark the first 3 of the 4 evaluated rules as passing.
	for i := 0; i < 3; i++ {
		rr.Results[i].Passed = true
	}
	bucket, scoredPassing, scoredTotal, scorePct := BucketOf(rr, scored)
	if scoredTotal != 4 {
		t.Errorf("expected scoredTotal=4 (HasRequiredReviewers omitted), got %d", scoredTotal)
	}
	if scoredPassing != 3 {
		t.Errorf("expected scoredPassing=3, got %d", scoredPassing)
	}
	if scorePct != 75 {
		t.Errorf("expected scorePct=75, got %d", scorePct)
	}
	if bucket.Name != "Moderate" {
		t.Errorf("expected Moderate (75%% lands in 30-79), got %s", bucket.Name)
	}
}

func TestResolveBranchProtection_MergesAcrossSources(t *testing.T) {
	// Each subtest seeds a different combination of source responses and
	// asserts that resolveBranchProtection unions their data. The merge
	// matters for hybrid configs (e.g. a ruleset enforcing PR review and
	// classic protection enforcing status checks on the same branch); a
	// first-non-nil chain would lose the second layer's data.
	cases := []struct {
		name             string
		rulesets         *BranchProtection
		classic          *BranchProtection
		branchInfo       *BranchProtection
		wantHasBP        bool
		wantReviewers    int
		wantStatusChecks []string
	}{
		{
			name:             "all nil → no protection, all rules fail",
			wantHasBP:        false,
			wantReviewers:    0,
			wantStatusChecks: nil,
		},
		{
			name:             "rulesets only (modern config)",
			rulesets:         &BranchProtection{RequiredReviewers: 2, RequiredStatusChecks: []string{"ci/build"}},
			wantHasBP:        true,
			wantReviewers:    2,
			wantStatusChecks: []string{"ci/build"},
		},
		{
			name:             "classic only (admin scan, legacy config)",
			classic:          &BranchProtection{RequiredReviewers: 1, RequiredStatusChecks: []string{"ci/test"}},
			wantHasBP:        true,
			wantReviewers:    1,
			wantStatusChecks: []string{"ci/test"},
		},
		{
			name:             "branch info only (non-admin classic-protected repo)",
			branchInfo:       &BranchProtection{RequiredStatusChecks: []string{"ci/lint"}},
			wantHasBP:        true,
			wantReviewers:    0,
			wantStatusChecks: []string{"ci/lint"},
		},
		{
			// The hybrid case: the bug this merge fixes. A ruleset
			// configures PR review enforcement; classic protection on
			// the same branch configures status checks. On a non-admin
			// scan, classic returns nil but branch info exposes the
			// status checks. Pre-merge, the chain stopped at rulesets
			// and missed them.
			name:             "hybrid: rulesets PR review + branch info status checks (non-admin)",
			rulesets:         &BranchProtection{RequiredReviewers: 1},
			branchInfo:       &BranchProtection{RequiredStatusChecks: []string{"ci/build"}},
			wantHasBP:        true,
			wantReviewers:    1,
			wantStatusChecks: []string{"ci/build"},
		},
		{
			// Both layers contribute status checks; both must pass, so
			// the effective requirement is the union.
			name:             "all three sources present: union status checks, max reviewers",
			rulesets:         &BranchProtection{RequiredReviewers: 1, RequiredStatusChecks: []string{"ci/build"}},
			classic:          &BranchProtection{RequiredReviewers: 2, RequiredStatusChecks: []string{"ci/build", "ci/test"}},
			branchInfo:       &BranchProtection{RequiredStatusChecks: []string{"ci/test", "ci/lint"}},
			wantHasBP:        true,
			wantReviewers:    2,
			wantStatusChecks: []string{"ci/build", "ci/test", "ci/lint"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &MockGitHubClient{
				Repos:      []Repo{{Name: "r", DefaultBranch: "main"}},
				Rulesets:   map[string]*BranchProtection{"r": tc.rulesets},
				Protection: map[string]*BranchProtection{"r": tc.classic},
				BranchInfo: map[string]*BranchProtection{"r": tc.branchInfo},
			}
			bp, err := resolveBranchProtection(context.Background(), client, "test-org", "r", "main")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if (bp != nil) != tc.wantHasBP {
				t.Fatalf("Has branch protection: got bp=%v, want has=%v", bp, tc.wantHasBP)
			}
			if !tc.wantHasBP {
				return
			}
			if bp.RequiredReviewers != tc.wantReviewers {
				t.Errorf("RequiredReviewers: got %d, want %d", bp.RequiredReviewers, tc.wantReviewers)
			}
			if !sameStringSet(bp.RequiredStatusChecks, tc.wantStatusChecks) {
				t.Errorf("RequiredStatusChecks: got %v, want %v (order-insensitive)", bp.RequiredStatusChecks, tc.wantStatusChecks)
			}
		})
	}
}

// sameStringSet reports whether two []string have the same elements
// regardless of order. Used by the merge test where the union order
// reflects iteration order across sources, which is implementation
// detail rather than contract.
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	return true
}

func TestScan_NonAdminKeepsNonAdminScoredRules(t *testing.T) {
	// Companion to TestScan_NonAdminFiltersAdminOnlyRules: ensure that
	// filtering out HasRequiredReviewers doesn't accidentally affect
	// the other scored rules. Catches a regression where someone widens
	// RequiresAdmin() to other rules without realizing.
	client := &MockGitHubClient{
		Repos: []Repo{{Name: "repo-a", DefaultBranch: "main"}},
	}
	sr, err := scanWithClient(context.Background(), client, PATAuth{Name: "test-org"}, scanOptions{admin: false})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	wantPresent := []string{
		"Has branch protection",
		"Requires status checks before merging",
		"Has CODEOWNERS",
		"Has CI workflow",
	}
	gotNames := make(map[string]bool)
	for _, res := range sr.Results[0].Results {
		gotNames[res.RuleName] = true
	}
	for _, name := range wantPresent {
		if !gotNames[name] {
			t.Errorf("non-admin scan should still evaluate %q (only HasRequiredReviewers is admin-only)", name)
		}
	}
	if gotNames["Has required reviewers"] {
		t.Error("non-admin scan should NOT evaluate Has required reviewers")
	}
}

func TestScore_NotDefinedWhenNoScoredRules(t *testing.T) {
	// Score requires both repos AND scored rules to be defined. An
	// org-level scan that filtered every scored rule out (only possible
	// today if every scored rule were marked admin-only and admin=false)
	// should render N/A, not a divide-by-zero.
	sr := ScanResult{
		Results: []RepoResult{{RepoName: "a"}},
		// RulesScored intentionally empty
	}
	score, defined := Score(sr)
	if defined {
		t.Errorf("expected undefined score when no scored rules; got %d", score)
	}
}
