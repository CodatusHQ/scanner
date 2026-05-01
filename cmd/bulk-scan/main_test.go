package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CodatusHQ/scanner"
)

func TestReadOrgs_SkipsBlankLinesNoCommentHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orgs.txt")
	content := "acme-corp\n\nwayne-enterprises\n   \nstark-industries\n# this is a literal org slug, not a comment\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readOrgs(path)
	if err != nil {
		t.Fatalf("readOrgs: %v", err)
	}

	want := []string{"acme-corp", "wayne-enterprises", "stark-industries", "# this is a literal org slug, not a comment"}
	if len(got) != len(want) {
		t.Fatalf("got %d orgs %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("orgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadOrgs_FileNotFound(t *testing.T) {
	_, err := readOrgs("/nonexistent/path/orgs.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// passingResults builds n RepoResult entries each with the same rule pass/fail.
// Used to make compliance/aggregation arithmetic obvious in tests.
func makeRepo(name string, perRule map[string]bool) scanner.RepoResult {
	rr := scanner.RepoResult{RepoName: name}
	for _, r := range scanner.AllRules() {
		passed, ok := perRule[r.Name()]
		if !ok {
			// Default to pass for rules not listed - keeps test data terse.
			passed = true
		}
		rr.Results = append(rr.Results, scanner.RuleResult{RuleName: r.Name(), Passed: passed})
	}
	return rr
}

func TestComputeCompliance(t *testing.T) {
	tests := []struct {
		name       string
		results    []scanner.RepoResult
		wantPct    int
		wantFullyC int
	}{
		{
			name:       "empty",
			results:    nil,
			wantPct:    0,
			wantFullyC: 0,
		},
		{
			name: "all compliant",
			results: []scanner.RepoResult{
				makeRepo("a", nil),
				makeRepo("b", nil),
			},
			wantPct:    100,
			wantFullyC: 2,
		},
		{
			name: "none compliant",
			results: []scanner.RepoResult{
				makeRepo("a", map[string]bool{"Has activity": false}),
				makeRepo("b", map[string]bool{"Has activity": false}),
			},
			wantPct:    0,
			wantFullyC: 0,
		},
		{
			name: "half compliant rounds down",
			results: []scanner.RepoResult{
				makeRepo("a", nil), // compliant
				makeRepo("b", map[string]bool{"Has activity": false}),
				makeRepo("c", nil), // compliant
			},
			wantPct:    66, // 2/3 = 66.66 -> 66 (integer division)
			wantFullyC: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct, fc := computeCompliance(scanner.ScanResult{Results: tt.results})
			if pct != tt.wantPct {
				t.Errorf("pct = %d, want %d", pct, tt.wantPct)
			}
			if fc != tt.wantFullyC {
				t.Errorf("fullyCompliant = %d, want %d", fc, tt.wantFullyC)
			}
		})
	}
}

func TestAggregateRules(t *testing.T) {
	results := []scanner.RepoResult{
		makeRepo("a", map[string]bool{
			"Has SECURITY.md":         false,
			"Has CODEOWNERS":          false,
			"Has branch protection":   false,
		}),
		makeRepo("b", map[string]bool{
			"Has SECURITY.md": false,
			"Has CODEOWNERS":  true,
		}),
	}

	got := aggregateRules(results)

	// Has SECURITY.md: 0 passing, 2 failing -> 0%.
	sec := got["has_security_md"]
	if sec.Passing != 0 || sec.Failing != 2 || sec.PassRate != 0 {
		t.Errorf("has_security_md = %+v, want passing=0 failing=2 pass_rate=0", sec)
	}

	// Has CODEOWNERS: 1 passing, 1 failing -> 50%.
	cow := got["has_codeowners"]
	if cow.Passing != 1 || cow.Failing != 1 || cow.PassRate != 50 {
		t.Errorf("has_codeowners = %+v, want passing=1 failing=1 pass_rate=50", cow)
	}

	// Has branch protection: 1 passing (default), 1 failing -> 50%.
	bp := got["has_branch_protection"]
	if bp.Passing != 1 || bp.Failing != 1 || bp.PassRate != 50 {
		t.Errorf("has_branch_protection = %+v, want passing=1 failing=1 pass_rate=50", bp)
	}

	// Rule with all passes (e.g., Has activity, default true).
	act := got["has_activity"]
	if act.Passing != 2 || act.Failing != 0 || act.PassRate != 100 {
		t.Errorf("has_activity = %+v, want passing=2 failing=0 pass_rate=100", act)
	}
}

func TestAggregateRules_IgnoresUnknownRuleNames(t *testing.T) {
	results := []scanner.RepoResult{
		{RepoName: "a", Results: []scanner.RuleResult{
			{RuleName: "Made-up rule", Passed: true},
			{RuleName: "Has activity", Passed: true},
		}},
	}

	got := aggregateRules(results)

	if _, exists := got["made_up_rule"]; exists {
		t.Errorf("expected unknown rule to be omitted from aggregate; got: %+v", got)
	}
	if act, ok := got["has_activity"]; !ok || act.Passing != 1 {
		t.Errorf("expected has_activity passing=1; got: %+v", got)
	}
}

func TestBuildStats_ProducesExpectedJSONKeys(t *testing.T) {
	older := mustParseTime(t, "2026-04-15T12:00:00Z")
	newer := mustParseTime(t, "2026-04-29T12:00:00Z")
	a := makeRepo("a", nil)
	a.MostRecentCommit = older
	b := makeRepo("b", nil)
	b.MostRecentCommit = newer
	c := makeRepo("c", map[string]bool{"Has activity": false})
	c.MostRecentCommit = older

	sr := scanner.ScanResult{
		Org:              "acme-corp",
		ScannedAt:        mustParseTime(t, "2026-04-30T10:15:00Z"),
		TotalRepos:       32,
		ForksExcluded:    4,
		ArchivedExcluded: 2,
		Results:          []scanner.RepoResult{a, b, c},
	}

	got := buildStats(sr)

	// Marshal and unmarshal into a generic map to verify JSON keys.
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wantTopLevel := []string{
		"org", "scanned_at", "total_public_repos", "forks_excluded",
		"archived_excluded", "repos_scanned", "compliance_percentage",
		"fully_compliant_count", "non_compliant_count", "rule_results",
		"most_recent_commit",
	}
	for _, key := range wantTopLevel {
		if _, ok := asMap[key]; !ok {
			t.Errorf("missing top-level JSON key %q in: %s", key, blob)
		}
	}

	// Spot-check field values to guarantee correct mapping.
	if got.Org != "acme-corp" {
		t.Errorf("Org = %q, want acme-corp", got.Org)
	}
	if got.TotalPublicRepos != 32 {
		t.Errorf("TotalPublicRepos = %d, want 32", got.TotalPublicRepos)
	}
	if got.ForksExcluded != 4 {
		t.Errorf("ForksExcluded = %d, want 4", got.ForksExcluded)
	}
	if got.ArchivedExcluded != 2 {
		t.Errorf("ArchivedExcluded = %d, want 2", got.ArchivedExcluded)
	}
	if got.ReposScanned != 3 {
		t.Errorf("ReposScanned = %d, want 3", got.ReposScanned)
	}
	if got.FullyCompliantCount != 2 {
		t.Errorf("FullyCompliantCount = %d, want 2", got.FullyCompliantCount)
	}
	if got.NonCompliantCount != 1 {
		t.Errorf("NonCompliantCount = %d, want 1", got.NonCompliantCount)
	}
	if got.MostRecentCommit != "2026-04-29" {
		t.Errorf("MostRecentCommit = %q, want 2026-04-29", got.MostRecentCommit)
	}

	// Verify rule_results uses snake_case JSON keys.
	ruleResults, ok := asMap["rule_results"].(map[string]any)
	if !ok {
		t.Fatalf("rule_results not an object: %v", asMap["rule_results"])
	}
	for _, r := range scanner.AllRules() {
		if _, ok := ruleResults[jsonKey(r.Name())]; !ok {
			t.Errorf("rule_results missing key %q (rule %q)", jsonKey(r.Name()), r.Name())
		}
	}
}

func TestJSONKey_DerivesSnakeCaseFromRuleName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Has repo description", "has_repo_description"},
		{"Has substantial README", "has_substantial_readme"},
		{"Has LICENSE", "has_license"},
		{"Has SECURITY.md", "has_security_md"},
		{"Has CI workflow", "has_ci_workflow"},
		{"Has test directory", "has_test_directory"},
		{"Has CODEOWNERS", "has_codeowners"},
		{"Has branch protection", "has_branch_protection"},
		{"Has required reviewers", "has_required_reviewers"},
		{"Requires status checks before merging", "requires_status_checks_before_merging"},
		{"Has activity", "has_activity"},
		// Edge cases.
		{"  Leading spaces", "leading_spaces"},
		{"Trailing dots...", "trailing_dots"},
		{"Multiple   spaces", "multiple_spaces"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonKey(tt.name); got != tt.want {
				t.Errorf("jsonKey(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestJSONKey_KeysAreUniqueAcrossAllRules(t *testing.T) {
	seen := make(map[string]string)
	for _, r := range scanner.AllRules() {
		key := jsonKey(r.Name())
		if other, dup := seen[key]; dup {
			t.Errorf("derived JSON key %q is shared by rules %q and %q", key, other, r.Name())
		}
		seen[key] = r.Name()
	}
}

func TestBuildStats_EmptyMostRecentCommit(t *testing.T) {
	// No Results / no Skipped -> no per-repo MostRecentCommit values to
	// aggregate, so the JSON field should be the empty string.
	sr := scanner.ScanResult{
		Org:       "empty-org",
		ScannedAt: mustParseTime(t, "2026-04-30T10:15:00Z"),
	}

	got := buildStats(sr)
	if got.MostRecentCommit != "" {
		t.Errorf("expected empty MostRecentCommit when no repos; got %q", got.MostRecentCommit)
	}
}

func TestWriteOrgOutput_WritesBothFiles(t *testing.T) {
	dir := t.TempDir()
	sr := scanner.ScanResult{
		Org:        "acme-corp",
		ScannedAt:  mustParseTime(t, "2026-04-30T10:15:00Z"),
		TotalRepos: 1,
		Results: []scanner.RepoResult{
			makeRepo("a", nil),
		},
	}

	if err := writeOrgOutput(dir, sr); err != nil {
		t.Fatalf("writeOrgOutput: %v", err)
	}

	mdPath := filepath.Join(dir, "acme-corp", "scorecard.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("scorecard.md missing at %s: %v", mdPath, err)
	}
	jsonPath := filepath.Join(dir, "acme-corp", "stats.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("stats.json missing at %s: %v", jsonPath, err)
	}

	// stats.json must be valid JSON with the expected top-level keys.
	blob, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read stats.json: %v", err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatalf("unmarshal stats.json: %v", err)
	}
	if asMap["org"] != "acme-corp" {
		t.Errorf("stats.json org = %v, want acme-corp", asMap["org"])
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tt
}
