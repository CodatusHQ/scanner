package scanner

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

func TestGenerateReport_MixedCompliance(t *testing.T) {
	results := []RepoResult{
		{RepoName: "alpha", Results: []RuleResult{
			{RuleName: "Has repo description", Passed: true},
			{RuleName: "Has .gitignore", Passed: true},
		}},
		{RepoName: "beta", Results: []RuleResult{
			{RuleName: "Has repo description", Passed: false},
			{RuleName: "Has .gitignore", Passed: true},
		}},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Repos scanned:** 2
**Compliant:** 1/2 (50%) *(a repo is compliant when it passes all rules below)*

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has repo description | 1 | 1 | 50% |
| Has .gitignore | 2 | 0 | 100% |

<details>
<summary>Rule reference - what each rule checks and how to fix it</summary>

### Has repo description
**What it checks:** The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page).

**How to fix:** Edit the repo and add a one-line description.

### Has .gitignore
**What it checks:** A .gitignore file exists at the repository root.

**How to fix:** Add a .gitignore at the repo root. [GitHub publishes templates per language](https://github.com/github/gitignore).

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
	results := []RepoResult{
		{RepoName: "alpha", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
		{RepoName: "beta", Results: []RuleResult{{RuleName: "Rule A", Passed: true}}},
	}

	got := generateReport("my-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** my-org
**Scanned:** 2026-04-05 12:00 UTC
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
	results := []RepoResult{
		{RepoName: "repo-1", Results: []RuleResult{
			{RuleName: "Rule A", Passed: false},
			{RuleName: "Rule B", Passed: false},
		}},
		{RepoName: "repo-2", Results: []RuleResult{
			{RuleName: "Rule A", Passed: true},
			{RuleName: "Rule B", Passed: false},
		}},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
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
	results := []RepoResult{
		{RepoName: "repo-1", Results: []RuleResult{
			{RuleName: "Rule A", Passed: true},
			{RuleName: "Rule B", Passed: false},
		}},
		{RepoName: "repo-2", Results: []RuleResult{
			{RuleName: "Rule A", Passed: true},
			{RuleName: "Rule B", Passed: true},
		}},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
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
	got := generateReport("empty-org", nil, testTime)

	want := `# Codatus - Org Compliance Report

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
	results := []RepoResult{
		{RepoName: "good-repo", Results: []RuleResult{
			{RuleName: "Has repo description", Passed: true},
		}},
		{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
		{RepoName: "huge-repo", KnownSkipReason: "file tree too large (truncated by GitHub API)"},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
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
**What it checks:** The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page).

**How to fix:** Edit the repo and add a one-line description.

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
	results := []RepoResult{
		{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
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
	results := []RepoResult{
		{RepoName: "broken-repo", UnknownSkipError: "get tree for broken-repo: status 500"},
		{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
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

func TestGenerateReport_ComplianceDefinitionAppended(t *testing.T) {
	results := []RepoResult{
		{RepoName: "alpha", Results: []RuleResult{
			{RuleName: "Has repo description", Passed: true},
		}},
	}

	got := generateReport("test-org", results, testTime)

	if !strings.Contains(got, "**Compliant:** 1/1 (100%) *(a repo is compliant when it passes all rules below)*") {
		t.Errorf("expected compliance definition appended to Compliant line; got:\n%s", got)
	}
}

func TestGenerateReport_IncludesRuleReference(t *testing.T) {
	// Use rule names that match the real rules in AllRules so the reference
	// section is emitted.
	results := []RepoResult{
		{RepoName: "alpha", Results: []RuleResult{
			{RuleName: "Has SECURITY.md", Passed: false},
			{RuleName: "Has CODEOWNERS", Passed: true},
		}},
	}

	got := generateReport("test-org", results, testTime)

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

func TestGenerateReport_RuleReferenceOmittedForUnknownRules(t *testing.T) {
	results := []RepoResult{
		{RepoName: "alpha", Results: []RuleResult{
			{RuleName: "Made-up rule", Passed: true},
		}},
	}

	got := generateReport("test-org", results, testTime)

	if strings.Contains(got, "<summary>Rule reference") {
		t.Errorf("expected no rule reference when no scanned rules match AllRules; got:\n%s", got)
	}
}
