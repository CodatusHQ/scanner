package scanner

import (
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
**Compliant:** 1/2 (50%)

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has repo description | 1 | 1 | 50% |
| Has .gitignore | 2 | 0 | 100% |

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
**Compliant:** 2/2 (100%)

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
**Compliant:** 0/2 (0%)

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
**Compliant:** 1/2 (50%)

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
		{RepoName: "empty-repo", Skipped: true, SkipReason: "repository is empty"},
		{RepoName: "huge-repo", Skipped: true, SkipReason: "file tree too large (truncated by GitHub API)"},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Repos scanned:** 1
**Compliant:** 1/1 (100%)
**Skipped:** 2

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has repo description | 1 | 0 | 100% |

## ✅ Fully compliant (1 repo)

<details>
<summary>All rules passing</summary>

[good-repo](https://github.com/test-org/good-repo)

</details>

## ⚠️ Skipped (2 repos)

<details>
<summary><a href="https://github.com/test-org/empty-repo">empty-repo</a> - repository is empty</summary>

This repository was excluded from compliance results.

</details>

<details>
<summary><a href="https://github.com/test-org/huge-repo">huge-repo</a> - file tree too large (truncated by GitHub API)</summary>

This repository was excluded from compliance results.

</details>

`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_OnlySkippedRepos(t *testing.T) {
	results := []RepoResult{
		{RepoName: "empty-repo", Skipped: true, SkipReason: "repository is empty"},
	}

	got := generateReport("test-org", results, testTime)

	want := `# Codatus - Org Compliance Report

**Org:** test-org
**Scanned:** 2026-04-05 12:00 UTC
**Repos scanned:** 0
**Skipped:** 1

## ⚠️ Skipped (1 repo)

<details>
<summary><a href="https://github.com/test-org/empty-repo">empty-repo</a> - repository is empty</summary>

This repository was excluded from compliance results.

</details>

`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}
