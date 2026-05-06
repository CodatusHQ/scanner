package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// makeRepo builds a RepoResult that includes a result for every rule in
// scanner.AllRules. Rules absent from perRule default to passing - keeps
// test setup terse for cases that only care about a handful of failures.
func makeRepo(name string, perRule map[string]bool) scanner.RepoResult {
	rr := scanner.RepoResult{RepoName: name}
	for _, r := range scanner.AllRules() {
		passed, ok := perRule[r.Name()]
		if !ok {
			passed = true
		}
		rr.Results = append(rr.Results, scanner.RuleResult{RuleName: r.Name(), Passed: passed})
	}
	return rr
}

func TestBucketCountsFor(t *testing.T) {
	// Construct repos with different scored-rule pass counts:
	//   2 strong repos (5 and 4 passing)
	//   2 moderate (3 and 2 passing)
	//   3 weak (1, 1, 0 passing)
	results := []scanner.RepoResult{
		repoWithScoredPasses("a", 5),
		repoWithScoredPasses("b", 4),
		repoWithScoredPasses("c", 3),
		repoWithScoredPasses("d", 2),
		repoWithScoredPasses("e", 1),
		repoWithScoredPasses("f", 1),
		repoWithScoredPasses("g", 0),
	}
	got := bucketCountsFor(results, scanner.ScoredRules())
	if got.Strong != 2 {
		t.Errorf("Strong = %d, want 2", got.Strong)
	}
	if got.Moderate != 2 {
		t.Errorf("Moderate = %d, want 2", got.Moderate)
	}
	if got.Weak != 3 {
		t.Errorf("Weak = %d, want 3", got.Weak)
	}
}

func TestAggregate_KeyedAndOrderedByInputRules(t *testing.T) {
	// Use scored rules in their importance order; verify the returned
	// orderedRuleAggregates preserves that order regardless of how it
	// would marshal alphabetically.
	results := []scanner.RepoResult{
		makeRepo("a", map[string]bool{
			"Has branch protection":                 false,
			"Has required reviewers":                false,
			"Has required checks": false,
		}),
		makeRepo("b", map[string]bool{
			"Has branch protection": false,
		}),
	}

	got := aggregate(results, scanner.ScoredRules())
	wantKeys := []string{
		"has_branch_protection",
		"has_required_reviewers",
		"has_required_checks",
		"has_codeowners",
		"has_ci_workflow",
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("got %d entries, want %d", len(got), len(wantKeys))
	}
	for i, k := range wantKeys {
		if got[i].Key != k {
			t.Errorf("entry %d: key = %q, want %q", i, got[i].Key, k)
		}
	}

	// Spot-check counts: branch_protection has 0 passing 2 failing -> 0%.
	bp := got[0].Value
	if bp.Passing != 0 || bp.Failing != 2 || bp.PassRate != 0 {
		t.Errorf("has_branch_protection = %+v, want passing=0 failing=2 pass_rate=0", bp)
	}
	// has_codeowners is true by default in makeRepo: 2 passing.
	co := got[3].Value
	if co.Passing != 2 || co.Failing != 0 || co.PassRate != 100 {
		t.Errorf("has_codeowners = %+v, want passing=2 failing=0 pass_rate=100", co)
	}
}

func TestOrderedRuleAggregates_PreservesInsertionOrder(t *testing.T) {
	o := orderedRuleAggregates{
		{Key: "z_first", Value: ruleAggregate{Passing: 1, Failing: 0, PassRate: 100}},
		{Key: "a_second", Value: ruleAggregate{Passing: 0, Failing: 1, PassRate: 0}},
		{Key: "m_third", Value: ruleAggregate{Passing: 1, Failing: 1, PassRate: 50}},
	}
	blob, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	zIdx := strings.Index(string(blob), "z_first")
	aIdx := strings.Index(string(blob), "a_second")
	mIdx := strings.Index(string(blob), "m_third")
	if zIdx == -1 || aIdx == -1 || mIdx == -1 {
		t.Fatalf("missing keys in JSON: %s", blob)
	}
	if !(zIdx < aIdx && aIdx < mIdx) {
		t.Errorf("keys not in insertion order: %s", blob)
	}
}

func TestBuildStats_NewShape(t *testing.T) {
	older := mustParseTime(t, "2026-04-15T12:00:00Z")
	newer := mustParseTime(t, "2026-04-29T12:00:00Z")

	a := repoWithScoredPasses("a", 5) // strong
	a.MostRecentCommit = older
	b := repoWithScoredPasses("b", 3) // moderate
	b.MostRecentCommit = newer
	c := repoWithScoredPasses("c", 0) // weak
	c.MostRecentCommit = older

	sr := scanner.ScanResult{
		Org:              "acme-corp",
		ScannedAt:        mustParseTime(t, "2026-04-30T10:15:00Z"),
		TotalRepos:       32,
		ForksExcluded:    4,
		ArchivedExcluded: 2,
		Results:          []scanner.RepoResult{a, b, c},
		RulesScored:      scanner.ScoredRules(),
		RulesAdditional:  scanner.AdditionalRules(),
	}

	got := buildStats(sr)

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(blob, &asMap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Every documented top-level key must be present (and old keys absent).
	wantTopLevel := []string{
		"org", "scanned_at", "total_public_repos", "forks_excluded",
		"archived_excluded", "repos_scanned", "score", "repo_buckets",
		"scored_rules", "additional_checks", "most_recent_commit",
	}
	for _, key := range wantTopLevel {
		if _, ok := asMap[key]; !ok {
			t.Errorf("missing top-level JSON key %q", key)
		}
	}
	for _, deprecated := range []string{"compliance_percentage", "fully_compliant_count", "non_compliant_count", "rule_results"} {
		if _, ok := asMap[deprecated]; ok {
			t.Errorf("expected deprecated key %q to be absent in new shape", deprecated)
		}
	}

	// Spot-check field values.
	if got.Org != "acme-corp" {
		t.Errorf("Org = %q, want acme-corp", got.Org)
	}
	if got.ReposScanned != 3 {
		t.Errorf("ReposScanned = %d, want 3", got.ReposScanned)
	}
	if got.RepoBuckets.Strong != 1 {
		t.Errorf("Strong = %d, want 1", got.RepoBuckets.Strong)
	}
	if got.RepoBuckets.Moderate != 1 {
		t.Errorf("Moderate = %d, want 1", got.RepoBuckets.Moderate)
	}
	if got.RepoBuckets.Weak != 1 {
		t.Errorf("Weak = %d, want 1", got.RepoBuckets.Weak)
	}
	if got.MostRecentCommit != "2026-04-29" {
		t.Errorf("MostRecentCommit = %q, want 2026-04-29", got.MostRecentCommit)
	}

	// Score is a *int, not nil for a non-empty scan.
	if got.Score == nil {
		t.Error("expected Score to be non-nil for non-empty results")
	}

	// scored_rules must contain the 5 scored keys in importance order.
	wantScoredOrder := []string{
		"has_branch_protection",
		"has_required_reviewers",
		"has_required_checks",
		"has_codeowners",
		"has_ci_workflow",
	}
	prev := -1
	for _, k := range wantScoredOrder {
		idx := strings.Index(string(blob), `"`+k+`"`)
		if idx == -1 {
			t.Errorf("scored_rules missing key %q in JSON", k)
			continue
		}
		if idx <= prev {
			t.Errorf("scored_rules key %q out of importance order in JSON", k)
		}
		prev = idx
	}

	// additional_checks must contain the 5 additional keys in importance order.
	wantAdditionalOrder := []string{
		"has_readme",
		"has_license",
		"has_repo_description",
		"has_activity",
		"has_security_md",
	}
	prev = strings.Index(string(blob), `"additional_checks"`)
	for _, k := range wantAdditionalOrder {
		idx := strings.Index(string(blob), `"`+k+`"`)
		if idx == -1 {
			t.Errorf("additional_checks missing key %q in JSON", k)
			continue
		}
		if idx <= prev {
			t.Errorf("additional_checks key %q out of importance order in JSON", k)
		}
		prev = idx
	}
}

func TestBuildStats_ScoreNullWhenNoRepos(t *testing.T) {
	sr := scanner.ScanResult{
		Org:       "empty-org",
		ScannedAt: mustParseTime(t, "2026-04-30T10:15:00Z"),
	}
	got := buildStats(sr)
	if got.Score != nil {
		t.Errorf("expected Score=nil for empty results, got %v", *got.Score)
	}

	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(blob), `"score":null`) {
		t.Errorf("expected JSON to contain `\"score\":null`; got: %s", blob)
	}
}

func TestJSONKey_DerivesSnakeCaseFromRuleName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Has repo description", "has_repo_description"},
		{"Has README", "has_readme"},
		{"Has LICENSE", "has_license"},
		{"Has SECURITY.md", "has_security_md"},
		{"Has CI workflow", "has_ci_workflow"},
		{"Has CODEOWNERS", "has_codeowners"},
		{"Has branch protection", "has_branch_protection"},
		{"Has required reviewers", "has_required_reviewers"},
		{"Has required checks", "has_required_checks"},
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
		Results:    []scanner.RepoResult{repoWithScoredPasses("a", 5)},
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

// repoWithScoredPasses builds a RepoResult passing the FIRST `passing`
// scored rules in importance order, and failing the rest. Additional
// rules are not included - tests using this helper care only about
// score / bucket math.
func repoWithScoredPasses(name string, passing int) scanner.RepoResult {
	rr := scanner.RepoResult{RepoName: name}
	for i, r := range scanner.ScoredRules() {
		rr.Results = append(rr.Results, scanner.RuleResult{
			RuleName: r.Name(),
			Passed:   i < passing,
		})
	}
	// Also emit additional-rule results so aggregate() (which now skips
	// rules absent from results) doesn't drop them. Default false; tests
	// don't currently care about additional-check counts.
	for _, r := range scanner.AdditionalRules() {
		rr.Results = append(rr.Results, scanner.RuleResult{
			RuleName: r.Name(),
			Passed:   false,
		})
	}
	return rr
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tt
}

func TestBuildStats_NonAdminOmitsAdminOnlyRule(t *testing.T) {
	// When sr.RulesScored doesn't include the admin-only rule (modeling
	// a non-admin scan), the marshaled JSON's scored_rules object must
	// omit the has_required_reviewers key entirely - not render it as
	// {passing: 0, failing: 0, pass_rate: 0} (which would be
	// indistinguishable from a real-but-failing rule).
	var nonAdminScored []scanner.Rule
	for _, r := range scanner.ScoredRules() {
		if r.Name() != "Has required reviewers" {
			nonAdminScored = append(nonAdminScored, r)
		}
	}
	rr := scanner.RepoResult{RepoName: "a"}
	for _, r := range nonAdminScored {
		rr.Results = append(rr.Results, scanner.RuleResult{RuleName: r.Name(), Passed: true})
	}
	for _, r := range scanner.AdditionalRules() {
		rr.Results = append(rr.Results, scanner.RuleResult{RuleName: r.Name(), Passed: false})
	}
	sr := scanner.ScanResult{
		Org:             "test",
		ScannedAt:       mustParseTime(t, "2026-04-30T10:15:00Z"),
		TotalRepos:      1,
		Results:         []scanner.RepoResult{rr},
		RulesScored:     nonAdminScored,
		RulesAdditional: scanner.AdditionalRules(),
	}
	got := buildStats(sr)
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// has_required_reviewers must not appear anywhere in the JSON.
	if strings.Contains(string(blob), "has_required_reviewers") {
		t.Errorf("non-admin scan should not include has_required_reviewers key; got JSON:\n%s", blob)
	}
	// The other 4 scored rules must still appear in their importance order.
	for _, k := range []string{
		"has_branch_protection",
		"has_required_checks",
		"has_codeowners",
		"has_ci_workflow",
	} {
		if !strings.Contains(string(blob), `"`+k+`"`) {
			t.Errorf("expected scored_rules key %q in non-admin JSON; got:\n%s", k, blob)
		}
	}
	// Additional checks unchanged.
	if !strings.Contains(string(blob), `"has_security_md"`) {
		t.Errorf("expected additional_checks to remain populated in non-admin JSON; got:\n%s", blob)
	}
}
