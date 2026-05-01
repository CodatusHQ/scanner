package scanner

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

func TestGenerateReport_MixedCompliance(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
				{RuleName: "Has activity", Passed: true},
			}},
			{RepoName: "beta", Results: []RuleResult{
				{RuleName: "Has repo description", Passed: false},
				{RuleName: "Has activity", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 2
**Repos scanned:** 2
**Compliant:** 1/2 (50%) *(a repo is compliant when it passes all rules below)*

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has repo description | 1 | 1 | 50% |
| Has activity | 2 | 0 | 100% |

<details>
<summary>Rule reference - what each rule checks and how to fix it</summary>

### Has repo description

- **What it checks:** The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page).
- **How to fix:** Edit the repo and add a one-line description.

---

### Has activity

- **What it checks:** The repository has had a commit (push) within the last 12 months.
- **How to fix:** Push a commit, or archive the repository if it is no longer maintained.

</details>

## ✅ Fully compliant (1 repo)

<details>
<summary>All rules passing</summary>

[alpha](https://github.com/test-org/alpha)

</details>

## ❌ Non-compliant (1 repo)

<details>
<summary><a href="https://github.com/test-org/beta">beta</a> - 1 failing</summary>

- Has repo description

</details>

`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_AllCompliant(t *testing.T) {
	sr := ScanResult{
		Org:        "my-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
			{RepoName: "beta", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** my-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 2
**Repos scanned:** 2
**Compliant:** 2/2 (100%) *(a repo is compliant when it passes all rules below)*

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Rule A | 2 | 0 | 100% |

## ✅ Fully compliant (2 repos)

<details>
<summary>All rules passing</summary>

[alpha](https://github.com/my-org/alpha)
[beta](https://github.com/my-org/beta)

</details>
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_AllNonCompliant(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "repo-1", Results: []RuleResult{
				{RuleName: "Rule A", Passed: false},
				{RuleName: "Rule B", Passed: false},
			}},
			{RepoName: "repo-2", Results: []RuleResult{
				{RuleName: "Rule A", Passed: true},
				{RuleName: "Rule B", Passed: false},
			}},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 2
**Repos scanned:** 2
**Compliant:** 0/2 (0%) *(a repo is compliant when it passes all rules below)*

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Rule B | 0 | 2 | 0% |
| Rule A | 1 | 1 | 50% |

## ❌ Non-compliant (2 repos)

<details>
<summary><a href="https://github.com/test-org/repo-1">repo-1</a> - 2 failing</summary>

- Rule A
- Rule B

</details>

<details>
<summary><a href="https://github.com/test-org/repo-2">repo-2</a> - 1 failing</summary>

- Rule B

</details>

`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_SummarySortedByPassRateAscending(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "repo-1", Results: []RuleResult{
				{RuleName: "Rule A", Passed: true},
				{RuleName: "Rule B", Passed: false},
			}},
			{RepoName: "repo-2", Results: []RuleResult{
				{RuleName: "Rule A", Passed: true},
				{RuleName: "Rule B", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 2
**Repos scanned:** 2
**Compliant:** 1/2 (50%) *(a repo is compliant when it passes all rules below)*

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Rule B | 1 | 1 | 50% |
| Rule A | 2 | 0 | 100% |

## ✅ Fully compliant (1 repo)

<details>
<summary>All rules passing</summary>

[repo-2](https://github.com/test-org/repo-2)

</details>

## ❌ Non-compliant (1 repo)

<details>
<summary><a href="https://github.com/test-org/repo-1">repo-1</a> - 1 failing</summary>

- Rule B

</details>

`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_EmptyResults(t *testing.T) {
	sr := ScanResult{
		Org:       "empty-org",
		ScannedAt: testTime,
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** empty-org
**Scanned:** 2026-04-05 12:00 UTC
**Repos scanned:** 0

No repos found.
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_WithSkippedRepos(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 3,
		Results: []RepoResult{
			{RepoName: "good-repo", Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
			}},
		},
		Skipped: []RepoResult{
			{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
			{RepoName: "huge-repo", KnownSkipReason: "file tree too large (truncated by GitHub API)"},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 3
**Repos scanned:** 1
**Compliant:** 1/1 (100%) *(a repo is compliant when it passes all rules below)*
**Skipped:** 2

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has repo description | 1 | 0 | 100% |

<details>
<summary>Rule reference - what each rule checks and how to fix it</summary>

### Has repo description

- **What it checks:** The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page).
- **How to fix:** Edit the repo and add a one-line description.

</details>

## ✅ Fully compliant (1 repo)

<details>
<summary>All rules passing</summary>

[good-repo](https://github.com/test-org/good-repo)

</details>

## ⚠️ Skipped (2 repos)

- [empty-repo](https://github.com/test-org/empty-repo) - repository is empty
- [huge-repo](https://github.com/test-org/huge-repo) - file tree too large (truncated by GitHub API)
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_OnlySkippedRepos(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Skipped: []RepoResult{
			{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 1
**Repos scanned:** 0
**Skipped:** 1

## ⚠️ Skipped (1 repo)

- [empty-repo](https://github.com/test-org/empty-repo) - repository is empty
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_WithUnexpectedSkipError(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Skipped: []RepoResult{
			{RepoName: "broken-repo", UnknownSkipError: "get tree for broken-repo: status 500"},
			{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
		},
	}

	got := GenerateReport(sr)

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Total repos:** 2
**Repos scanned:** 0
**Skipped:** 2

## ⚠️ Skipped (2 repos)

- [broken-repo](https://github.com/test-org/broken-repo) - unexpected error: get tree for broken-repo: status 500
- [empty-repo](https://github.com/test-org/empty-repo) - repository is empty
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

// TestGenerateReport_ForksExcludedLineEmitted verifies the **Forks excluded:**
// header line appears only when ScanResult.ForksExcluded > 0, and shows the
// correct count.
func TestGenerateReport_ForksExcludedLineEmitted(t *testing.T) {
	sr := ScanResult{
		Org:           "test-org",
		ScannedAt:     testTime,
		TotalRepos:    5,
		ForksExcluded: 2,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
			{RepoName: "beta", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
			{RepoName: "gamma", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		},
	}

	got := GenerateReport(sr)

	if !strings.Contains(got, "**Total repos:** 5") {
		t.Errorf("expected '**Total repos:** 5' line; got:\n%s", got)
	}
	if !strings.Contains(got, "**Forks excluded:** 2") {
		t.Errorf("expected '**Forks excluded:** 2' line; got:\n%s", got)
	}
	if strings.Contains(got, "**Archived excluded:**") {
		t.Errorf("expected no archived-excluded line when count is 0; got:\n%s", got)
	}
}

// TestGenerateReport_ArchivedExcludedLineEmitted verifies the
// **Archived excluded:** header line appears only when
// ScanResult.ArchivedExcluded > 0, and shows the correct count.
func TestGenerateReport_ArchivedExcludedLineEmitted(t *testing.T) {
	sr := ScanResult{
		Org:              "test-org",
		ScannedAt:        testTime,
		TotalRepos:       4,
		ArchivedExcluded: 1,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
			{RepoName: "beta", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
			{RepoName: "gamma", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		},
	}

	got := GenerateReport(sr)

	if !strings.Contains(got, "**Archived excluded:** 1") {
		t.Errorf("expected '**Archived excluded:** 1' line; got:\n%s", got)
	}
	if strings.Contains(got, "**Forks excluded:**") {
		t.Errorf("expected no forks-excluded line when count is 0; got:\n%s", got)
	}
}

// TestGenerateReport_AllExclusionLinesEmittedTogether verifies that when
// both ForksExcluded and ArchivedExcluded are > 0, both lines appear and
// in the documented order: Total -> Forks -> Archived -> Repos scanned.
func TestGenerateReport_AllExclusionLinesEmittedTogether(t *testing.T) {
	sr := ScanResult{
		Org:              "acme-corp",
		ScannedAt:        testTime,
		TotalRepos:       32,
		ForksExcluded:    4,
		ArchivedExcluded: 2,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		},
	}

	got := GenerateReport(sr)

	// All four lines must appear in order.
	totalIdx := strings.Index(got, "**Total repos:** 32")
	forksIdx := strings.Index(got, "**Forks excluded:** 4")
	archivedIdx := strings.Index(got, "**Archived excluded:** 2")
	scannedIdx := strings.Index(got, "**Repos scanned:** 1")

	if totalIdx == -1 || forksIdx == -1 || archivedIdx == -1 || scannedIdx == -1 {
		t.Fatalf("expected all four header lines; got:\n%s", got)
	}
	if !(totalIdx < forksIdx && forksIdx < archivedIdx && archivedIdx < scannedIdx) {
		t.Errorf("expected header lines in order Total -> Forks -> Archived -> Scanned; got:\n%s", got)
	}
}

// TestGenerateReport_OmitsExclusionLinesWhenZero verifies that when no
// exclusions occurred, the optional lines (Total repos when 0, Forks
// excluded, Archived excluded) are omitted entirely.
func TestGenerateReport_OmitsExclusionLinesWhenZero(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 0,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		},
	}

	got := GenerateReport(sr)

	if strings.Contains(got, "**Total repos:**") {
		t.Errorf("expected no '**Total repos:**' line when TotalRepos is 0; got:\n%s", got)
	}
	if strings.Contains(got, "**Forks excluded:**") {
		t.Errorf("expected no '**Forks excluded:**' line when count is 0; got:\n%s", got)
	}
	if strings.Contains(got, "**Archived excluded:**") {
		t.Errorf("expected no '**Archived excluded:**' line when count is 0; got:\n%s", got)
	}
}

func TestGenerateReport_ComplianceDefinitionAppended(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	if !strings.Contains(got, "**Compliant:** 1/1 (100%) *(a repo is compliant when it passes all rules below)*") {
		t.Errorf("expected compliance definition appended to Compliant line; got:\n%s", got)
	}
}

func TestGenerateReport_IncludesRuleReference(t *testing.T) {
	// Use rule names that match the real rules in AllRules so the reference
	// section is emitted.
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{
				{RuleName: "Has SECURITY.md", Passed: false},
				{RuleName: "Has CODEOWNERS", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	if !strings.Contains(got, "<summary>Rule reference - what each rule checks and how to fix it</summary>") {
		t.Errorf("expected rule reference summary; got:\n%s", got)
	}
	if !strings.Contains(got, "### Has SECURITY.md") {
		t.Errorf("expected ### Has SECURITY.md heading in rule reference; got:\n%s", got)
	}
	if !strings.Contains(got, "### Has CODEOWNERS") {
		t.Errorf("expected ### Has CODEOWNERS heading in rule reference; got:\n%s", got)
	}
	if !strings.Contains(got, "**What it checks:**") {
		t.Errorf("expected What it checks lines in rule reference; got:\n%s", got)
	}
	if !strings.Contains(got, "**How to fix:**") {
		t.Errorf("expected How to fix lines in rule reference; got:\n%s", got)
	}

	// Reference order must follow AllRules - HasSecurityMd appears before
	// HasCodeowners in AllRules, regardless of how the summary table sorts.
	secIdx := strings.Index(got, "### Has SECURITY.md")
	cowIdx := strings.Index(got, "### Has CODEOWNERS")
	if secIdx == -1 || cowIdx == -1 || secIdx >= cowIdx {
		t.Errorf("expected rule reference order to follow AllRules (SECURITY.md before CODEOWNERS)")
	}
}

// TestGenerateReport_RuleReferenceFormatting verifies the precise Markdown
// shape of each rule reference entry: blank line after the H3, bullet list
// of "What it checks" + "How to fix", and a `---` separator between rules
// (but not after the last).
func TestGenerateReport_RuleReferenceFormatting(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{
				{RuleName: "Has repo description", Passed: true},
				{RuleName: "Has activity", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	wantBlock := "### Has repo description\n\n- **What it checks:** "
	if !strings.Contains(got, wantBlock) {
		t.Errorf("expected blank line after H3 followed by bullet list; got:\n%s", got)
	}
	// `---` separator must appear between two rules - i.e. between the end of
	// the first rule's bullet block and the next rule's H3.
	if !strings.Contains(got, "\n\n---\n\n### Has activity\n") {
		t.Errorf("expected `---` separator between rules; got:\n%s", got)
	}
	// No trailing `---` separator after the last rule (Has activity is last
	// in AllRules). Use LastIndex to skip past summary-table cells that share
	// the rule name.
	lastH3 := strings.LastIndex(got, "### Has activity")
	if lastH3 == -1 {
		t.Fatal("expected ### Has activity heading in rule reference")
	}
	closingIdx := strings.Index(got[lastH3:], "</details>")
	if closingIdx == -1 {
		t.Fatal("expected </details> after last rule")
	}
	tail := got[lastH3 : lastH3+closingIdx]
	if strings.Contains(tail, "\n---\n") {
		t.Errorf("expected no trailing `---` separator after last rule; got tail:\n%s", tail)
	}
}

func TestGenerateReport_RuleReferenceOmittedForUnknownRules(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{RepoName: "alpha", Results: []RuleResult{
				{RuleName: "Made-up rule", Passed: true},
			}},
		},
	}

	got := GenerateReport(sr)

	if strings.Contains(got, "<summary>Rule reference") {
		t.Errorf("expected no rule reference when no scanned rules match AllRules; got:\n%s", got)
	}
}
