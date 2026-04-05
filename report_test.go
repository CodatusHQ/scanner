package scanner

import (
	"strings"
	"testing"
)

func TestGenerateReport_Structure(t *testing.T) {
	results := []RepoResult{
		{
			RepoName: "alpha",
			Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
			},
		},
		{
			RepoName: "beta",
			Results: []RuleResult{
				{RuleName: "Has repo description", Passed: false},
			},
		},
	}

	report := GenerateReport("test-org", results)

	// Header
	if !strings.Contains(report, "# Codatus — Org Compliance Report") {
		t.Error("missing report title")
	}
	if !strings.Contains(report, "**Org:** test-org") {
		t.Error("missing org name")
	}
	if !strings.Contains(report, "**Repos scanned:** 2") {
		t.Error("missing repo count")
	}

	// Summary table
	if !strings.Contains(report, "## Summary") {
		t.Error("missing summary section")
	}
	if !strings.Contains(report, "| Has repo description | 1 | 1 | 50% |") {
		t.Error("missing or incorrect summary row")
	}

	// Per-repo tables
	if !strings.Contains(report, "### alpha") {
		t.Error("missing alpha repo section")
	}
	if !strings.Contains(report, "### beta") {
		t.Error("missing beta repo section")
	}
}

func TestGenerateReport_PassFailIcons(t *testing.T) {
	results := []RepoResult{
		{
			RepoName: "my-repo",
			Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
			},
		},
	}

	report := GenerateReport("test-org", results)

	if !strings.Contains(report, "| Has repo description | ✅ |") {
		t.Error("expected ✅ for passing rule")
	}

	results[0].Results[0].Passed = false
	report = GenerateReport("test-org", results)

	if !strings.Contains(report, "| Has repo description | ❌ |") {
		t.Error("expected ❌ for failing rule")
	}
}

func TestGenerateReport_SummarySortedByPassRateAscending(t *testing.T) {
	results := []RepoResult{
		{
			RepoName: "repo-1",
			Results: []RuleResult{
				{RuleName: "Rule A", Passed: true},
				{RuleName: "Rule B", Passed: false},
			},
		},
		{
			RepoName: "repo-2",
			Results: []RuleResult{
				{RuleName: "Rule A", Passed: true},
				{RuleName: "Rule B", Passed: true},
			},
		},
	}

	report := GenerateReport("test-org", results)

	// Rule B: 1 pass / 2 repos = 50%. Rule A: 2 pass / 2 repos = 100%.
	// Sorted ascending: Rule B first, then Rule A.
	posB := strings.Index(report, "| Rule B |")
	posA := strings.Index(report, "| Rule A |")

	if posB == -1 || posA == -1 {
		t.Fatal("missing rule rows in summary")
	}
	if posB > posA {
		t.Error("expected Rule B (50%) before Rule A (100%) in summary")
	}
}

func TestGenerateReport_EmptyResults(t *testing.T) {
	report := GenerateReport("empty-org", nil)

	if !strings.Contains(report, "**Repos scanned:** 0") {
		t.Error("expected 0 repos scanned")
	}
	if !strings.Contains(report, "## Summary") {
		t.Error("missing summary section")
	}
}
