package scanner

import (
	"strings"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

// allScored returns a []RuleResult that has every scored rule, with the
// first `passing` ones marked as passed. Test helper for building repo
// results in the new score/bucket-aware tests.
func allScored(passing int) []RuleResult {
	var out []RuleResult
	for i, r := range ScoredRules() {
		out = append(out, RuleResult{RuleName: r.Name(), Passed: i < passing})
	}
	return out
}

func TestGenerateReport_StrongAndWeakBuckets(t *testing.T) {
	// Two repos: one strong (5/5 scored passing), one weak (0/5 scored).
	// Tests the new core flow: header → scored table → score callout →
	// additional checks → rule reference (split) → repo details (buckets).
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "alpha", Results: allScored(5)},
			{RepoName: "beta", Results: allScored(0)},
		},
	}

	got := GenerateReport(withDefaultRules(sr))

	want := `# Codatus - Engineering Standards Scorecard

**Org:** test-org<br>
**Scanned:** 2026-04-05 12:00 UTC<br>
**Repos:** 2 of 2 scanned

## Scored rules

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has branch protection | 1 | 1 | 50% |
| Has required reviewers | 1 | 1 | 50% |
| Requires status checks before merging | 1 | 1 | 50% |
| Has CODEOWNERS | 1 | 1 | 50% |
| Has CI workflow | 1 | 1 | 50% |

**Score: 50/100** (average pass rate across the scored rules above)

## Additional checks

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has README | 0 | 0 | 0% |
| Has LICENSE | 0 | 0 | 0% |
| Has repo description | 0 | 0 | 0% |
| Has activity | 0 | 0 | 0% |
| Has SECURITY.md | 0 | 0 | 0% |

## Rule reference

<details>
<summary>How each rule works and how to fix failures</summary>

### Scored rules

#### Has branch protection

Checks that the default branch has a protection rule in place. Detected via any of three GitHub APIs: the modern repository rulesets (Settings > Rules > Rulesets), the legacy classic branch-protection rules (Settings > Branches > Branch protection rules), or the ` + "`protected`" + ` flag on the public branch endpoint. To fix: add a rule for the default branch via either Rulesets or classic Branch protection rules. [GitHub docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches).

---

#### Has required reviewers

Checks that the default branch's protection requires at least one approving review before a PR can be merged. The reviewer count is read from both modern repository rulesets (a ` + "`pull_request`" + ` rule with ` + "`required_approving_review_count >= 1`" + `) and legacy classic branch protection (the ` + "`required_pull_request_reviews.required_approving_review_count`" + ` field). To fix: edit the default-branch rule (or ruleset) and enable the pull-request review requirement with at least 1 required reviewer.

---

#### Requires status checks before merging

Checks that the default branch's protection requires at least one status check to pass before a PR can be merged. Detected from any of three sources: modern repository rulesets (a ` + "`required_status_checks`" + ` rule), legacy classic branch protection (` + "`required_status_checks.contexts`" + `), or the public branch endpoint's ` + "`protection.required_status_checks.contexts`" + ` field. To fix: edit the default-branch rule (or ruleset), enable "Require status checks to pass before merging", and select at least one check.

---

#### Has CODEOWNERS

Checks for a CODEOWNERS file in any of the three locations GitHub honors: the repo root, ` + "`.github/`" + `, or ` + "`docs/`" + `. To fix: add a CODEOWNERS file in one of those locations mapping paths to GitHub users or teams. [GitHub docs](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

---

#### Has CI workflow

Checks for a checked-in CI configuration file from any of the major providers: GitHub Actions (any ` + "`.yml`" + ` or ` + "`.yaml`" + ` file under ` + "`.github/workflows/`" + `), CircleCI (` + "`.circleci/config.yml`" + `), GitLab CI (` + "`.gitlab-ci.yml`" + `), Travis (` + "`.travis.yml`" + `), Buildkite (any file under ` + "`.buildkite/`" + `), Azure Pipelines (` + "`azure-pipelines.yml`" + `), or Jenkins (` + "`Jenkinsfile`" + `). Setups whose configuration lives entirely server-side (no checked-in file) are not detected. To fix: add a workflow file for the provider you use. The simplest path on GitHub is a YAML workflow under ` + "`.github/workflows/`" + `. [GitHub Actions quickstart](https://docs.github.com/en/actions/quickstart).

### Additional checks

#### Has README

Checks for a README file at the repository root. The match is case-insensitive and accepts any extension or none, so ` + "`README.md`" + `, ` + "`README.rst`" + `, ` + "`README.txt`" + `, ` + "`Readme`" + `, ` + "`readme.markdown`" + ` all pass. READMEs in subdirectories don't count. To fix: add a top-level README that explains what the project is, how to install it, and how to use it.

---

#### Has LICENSE

Checks GitHub's auto-detected license field, which GitHub populates by running the Licensee gem against the repo and recognizing conventionally-named license files: ` + "`LICENSE`" + `, ` + "`LICENSE.md`" + `, ` + "`LICENSE.txt`" + `, ` + "`LICENCE`" + `, ` + "`COPYING`" + `, ` + "`MIT-LICENSE`" + `, and similar variants. Custom-text licenses Licensee can't classify won't pass even if a file is present. To fix: pick a license at [choosealicense.com](https://choosealicense.com) and add it to your repo root using one of the recognized filenames. GitHub will detect it automatically.

---

#### Has repo description

Checks that the repository's description field (set via the About panel, shown at the top of the GitHub repo page) is non-empty. To fix: edit the repo's About panel and add a one-line description.

---

#### Has activity

Checks that the repository has had a commit (push) within the last 12 months, based on GitHub's ` + "`pushed_at`" + ` timestamp on the repo. To fix: push a commit, or archive the repository if it's no longer maintained.

---

#### Has SECURITY.md

Checks for a SECURITY.md file in any of the three locations GitHub recognizes for security policies: the repo root, ` + "`.github/`" + `, or ` + "`docs/`" + `. To fix: add a SECURITY.md describing how to report vulnerabilities. [GitHub's template](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository).

</details>

## Repository details

### Strong (≥80%)

<details>
<summary><a href="https://github.com/test-org/alpha">alpha</a> - 100%</summary>

</details>

### Weak (≤29%)

<details>
<summary><a href="https://github.com/test-org/beta">beta</a> - 0%</summary>

**Failing scored rules:**
- Has branch protection
- Has required reviewers
- Requires status checks before merging
- Has CODEOWNERS
- Has CI workflow

</details>
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_EmptyResults(t *testing.T) {
	sr := ScanResult{Org: "empty-org", ScannedAt: testTime}
	got := GenerateReport(withDefaultRules(sr))

	want := `# Codatus - Engineering Standards Scorecard

**Org:** empty-org<br>
**Scanned:** 2026-04-05 12:00 UTC<br>
**Repos:** 0 scanned

No repos found.
`
	if got != want {
		t.Errorf("report mismatch.\n\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestGenerateReport_ScoreNAWhenNoScannedRepos(t *testing.T) {
	// Only skipped repos, no successful scans → score is N/A.
	sr := ScanResult{
		Org:       "test-org",
		ScannedAt: testTime,
		Skipped: []RepoResult{
			{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
		},
	}

	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "**Score: N/A** (no repos available to score)") {
		t.Errorf("expected inline N/A score callout; got:\n%s", got)
	}
}

func TestGenerateReport_BucketSectionOmittedWhenEmpty(t *testing.T) {
	// All repos are strong - no Moderate or Weak headers should appear.
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 2,
		Results: []RepoResult{
			{RepoName: "a", Results: allScored(5)},
			{RepoName: "b", Results: allScored(4)},
		},
	}

	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "### Strong (≥80%)") {
		t.Errorf("expected Strong section; got:\n%s", got)
	}
	if strings.Contains(got, "### Moderate") {
		t.Errorf("did not expect Moderate section when no moderate repos; got:\n%s", got)
	}
	if strings.Contains(got, "### Weak") {
		t.Errorf("did not expect Weak section when no weak repos; got:\n%s", got)
	}
}

func TestGenerateReport_BothTablesShareColumnLayout(t *testing.T) {
	// Scored rules and Additional checks should render with identical
	// column headers so the two tables visually align. Use a fixture
	// that exercises both - allScored(3) only emits scored entries, so
	// the Additional checks section would otherwise be suppressed (the
	// new aggregate() drops sections whose rules have zero results).
	rr := RepoResult{RepoName: "a", Results: allScored(3)}
	for _, r := range AdditionalRules() {
		rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: false})
	}
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{rr},
	}
	got := GenerateReport(withDefaultRules(sr))

	if strings.Count(got, "| Rule | Passing | Failing | Pass rate |") != 2 {
		t.Errorf("expected both tables to use 'Rule | Passing | Failing | Pass rate'; got:\n%s", got)
	}
	if strings.Contains(got, "| Check |") {
		t.Errorf("did not expect legacy 'Check' column header anywhere; got:\n%s", got)
	}
	if strings.Contains(got, "Coverage |") {
		t.Errorf("did not expect legacy 'Coverage' column header anywhere; got:\n%s", got)
	}
}

func TestGenerateReport_NoCompliantOrSkippedHeaderLines(t *testing.T) {
	// The old header had "**Compliant: X/Y**". The new format drops it.
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{{RepoName: "a", Results: allScored(5)}},
	}
	got := GenerateReport(withDefaultRules(sr))

	if strings.Contains(got, "**Compliant:**") {
		t.Errorf("expected no Compliant header line in new format; got:\n%s", got)
	}
	if strings.Contains(got, "(a repo is compliant when it passes all rules below)") {
		t.Errorf("expected no compliance-definition footnote; got:\n%s", got)
	}
}

func TestGenerateReport_PerRepoOmitsEmptyFailureSection(t *testing.T) {
	// A strong repo with no failing additional checks should NOT render an
	// empty "Additional check failures:" section.
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{
				RepoName: "perfect",
				Results: append(allScored(5), []RuleResult{
					{RuleName: "Has README", Passed: true},
					{RuleName: "Has LICENSE", Passed: true},
					{RuleName: "Has repo description", Passed: true},
					{RuleName: "Has activity", Passed: true},
					{RuleName: "Has SECURITY.md", Passed: true},
				}...),
			},
		},
	}
	got := GenerateReport(withDefaultRules(sr))

	if strings.Contains(got, "**Failing scored rules:**") {
		t.Errorf("expected no 'Failing scored rules' for a 5/5 repo; got:\n%s", got)
	}
	if strings.Contains(got, "**Additional check failures:**") {
		t.Errorf("expected no 'Additional check failures' for a fully-passing repo; got:\n%s", got)
	}
}

func TestGenerateReport_PerRepoSplitsFailuresByCategory(t *testing.T) {
	// A moderate repo: 3/5 scored passing, missing CODEOWNERS and CI;
	// also missing two additional checks. Both subsections must appear.
	results := append(allScored(3), []RuleResult{
		{RuleName: "Has README", Passed: true},
		{RuleName: "Has LICENSE", Passed: false},
		{RuleName: "Has repo description", Passed: false},
	}...)

	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{{RepoName: "midrepo", Results: results}},
	}
	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "**Failing scored rules:**") {
		t.Errorf("expected 'Failing scored rules' section; got:\n%s", got)
	}
	if !strings.Contains(got, "**Additional check failures:**") {
		t.Errorf("expected 'Additional check failures' section; got:\n%s", got)
	}

	// Failing scored: Has CODEOWNERS, Has CI workflow (positions 3 and 4).
	if !strings.Contains(got, "- Has CODEOWNERS\n") {
		t.Errorf("expected scored failure 'Has CODEOWNERS' listed; got:\n%s", got)
	}
	if !strings.Contains(got, "- Has CI workflow\n") {
		t.Errorf("expected scored failure 'Has CI workflow' listed; got:\n%s", got)
	}
	// Failing additional: Has LICENSE, Has repo description.
	if !strings.Contains(got, "- Has LICENSE\n") {
		t.Errorf("expected additional failure 'Has LICENSE' listed; got:\n%s", got)
	}
	if !strings.Contains(got, "- Has repo description\n") {
		t.Errorf("expected additional failure 'Has repo description' listed; got:\n%s", got)
	}
}

func TestGenerateReport_RuleReferenceSplitByCategory(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results: []RepoResult{
			{RepoName: "a", Results: append(allScored(2), RuleResult{RuleName: "Has README", Passed: true})},
		},
	}
	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "## Rule reference\n") {
		t.Errorf("expected '## Rule reference' heading; got:\n%s", got)
	}
	if !strings.Contains(got, "<summary>How each rule works and how to fix failures</summary>") {
		t.Errorf("expected rule reference summary; got:\n%s", got)
	}
	scoredHeaderIdx := strings.Index(got, "### Scored rules\n")
	additionalHeaderIdx := strings.Index(got, "### Additional checks\n")
	if scoredHeaderIdx == -1 || additionalHeaderIdx == -1 {
		t.Fatalf("expected both reference subsections; got:\n%s", got)
	}
	if scoredHeaderIdx >= additionalHeaderIdx {
		t.Errorf("expected Scored rules to appear before Additional checks in reference")
	}
}

// TestGenerateReport_RuleReferenceBeforeRepositoryDetails locks in the
// section order: ## Rule reference must precede ## Repository details so a
// reader has rule definitions in hand before reading per-repo failure lists
// (which only mention rule names).
func TestGenerateReport_RuleReferenceBeforeRepositoryDetails(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{{RepoName: "a", Results: allScored(3)}},
	}
	got := GenerateReport(withDefaultRules(sr))

	refIdx := strings.Index(got, "## Rule reference\n")
	repoIdx := strings.Index(got, "## Repository details\n")
	if refIdx == -1 || repoIdx == -1 {
		t.Fatalf("expected both '## Rule reference' and '## Repository details'; got:\n%s", got)
	}
	if refIdx >= repoIdx {
		t.Errorf("expected '## Rule reference' to appear before '## Repository details'; ref=%d repo=%d", refIdx, repoIdx)
	}
}

// TestGenerateReport_RuleReferenceOmitsAdminOnlyRulesForPublicScans verifies
// that on a non-admin scan (where HasRequiredReviewers is filtered out of
// sr.RulesScored), the Rule reference section does NOT include its
// description. The reference iterates sr.RulesScored / sr.RulesAdditional,
// so this is structurally guaranteed - the test pins the behavior so a
// future refactor can't accidentally start rendering all global rules.
func TestGenerateReport_RuleReferenceOmitsAdminOnlyRulesForPublicScans(t *testing.T) {
	// Build the scored-rule set the way scanWithClient(admin: false) would:
	// every scored rule except those marked admin-only.
	var publicScored []Rule
	for _, r := range ScoredRules() {
		if !ruleRequiresAdmin(r) {
			publicScored = append(publicScored, r)
		}
	}

	rr := RepoResult{RepoName: "a"}
	for _, r := range publicScored {
		rr.Results = append(rr.Results, RuleResult{RuleName: r.Name(), Passed: true})
	}

	sr := ScanResult{
		Org:             "test-org",
		ScannedAt:       testTime,
		TotalRepos:      1,
		Results:         []RepoResult{rr},
		RulesScored:     publicScored,
		RulesAdditional: AdditionalRules(),
	}
	got := GenerateReport(sr)

	if strings.Contains(got, "#### Has required reviewers") {
		t.Errorf("non-admin scan must not emit a 'Has required reviewers' rule-reference entry; got:\n%s", got)
	}
	// The rule's description text is the strongest content fingerprint - if
	// it leaks anywhere, this catches it.
	reviewerDescFragment := "edit the default-branch rule (or ruleset) and enable the pull-request review requirement"
	if strings.Contains(got, reviewerDescFragment) {
		t.Errorf("non-admin scan must not emit Has-required-reviewers description text; got:\n%s", got)
	}
	// Sanity: a rule that should still be present is.
	if !strings.Contains(got, "#### Has branch protection") {
		t.Errorf("expected non-admin-only scored rule 'Has branch protection' in reference; got:\n%s", got)
	}
}

func TestGenerateReport_WithSkippedRepos(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 3,
		Results:    []RepoResult{{RepoName: "good-repo", Results: allScored(5)}},
		Skipped: []RepoResult{
			{RepoName: "empty-repo", KnownSkipReason: "repository is empty"},
			{RepoName: "huge-repo", KnownSkipReason: "file tree too large (truncated by GitHub API)"},
		},
	}
	got := GenerateReport(withDefaultRules(sr))

	// Skipped count surfaces in the one-line repo-stats header line.
	if !strings.Contains(got, "2 skipped") {
		t.Errorf("expected '2 skipped' in repo-stats header; got:\n%s", got)
	}
	// Skipped renders as a sibling subsection under ## Repository details
	// (no longer a top-level section, no warning emoji).
	if !strings.Contains(got, "### Skipped (2 repos)") {
		t.Errorf("expected '### Skipped (2 repos)' subsection heading; got:\n%s", got)
	}
	if strings.Contains(got, "## ⚠️ Skipped") || strings.Contains(got, "⚠️") {
		t.Errorf("did not expect legacy '## ⚠️ Skipped' section anywhere; got:\n%s", got)
	}
	if !strings.Contains(got, "[empty-repo](https://github.com/test-org/empty-repo) - repository is empty") {
		t.Errorf("expected empty-repo entry; got:\n%s", got)
	}

	// Skipped subsection appears after Strong (the only bucket present here).
	strongIdx := strings.Index(got, "### Strong")
	skippedIdx := strings.Index(got, "### Skipped")
	if strongIdx == -1 || skippedIdx == -1 || strongIdx >= skippedIdx {
		t.Errorf("expected ### Skipped to appear after ### Strong; got:\n%s", got)
	}
}

func TestGenerateReport_WithUnexpectedSkipError(t *testing.T) {
	sr := ScanResult{
		Org:       "test-org",
		ScannedAt: testTime,
		Skipped: []RepoResult{
			{RepoName: "broken-repo", UnknownSkipError: "get tree: status 500"},
		},
	}
	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "[broken-repo](https://github.com/test-org/broken-repo) - unexpected error: get tree: status 500") {
		t.Errorf("expected unexpected-error rendering; got:\n%s", got)
	}
}

func TestGenerateReport_HeaderRepoStatsLineWithExclusions(t *testing.T) {
	sr := ScanResult{
		Org:              "test-org",
		ScannedAt:        testTime,
		TotalRepos:       14,
		ForksExcluded:    3,
		ArchivedExcluded: 1,
		Results:          []RepoResult{{RepoName: "a", Results: allScored(5)}},
		Skipped:          []RepoResult{{RepoName: "empty", KnownSkipReason: "repository is empty"}},
	}
	got := GenerateReport(withDefaultRules(sr))

	want := "**Repos:** 1 of 14 scanned (3 forks excluded, 1 archived excluded, 1 skipped)"
	if !strings.Contains(got, want) {
		t.Errorf("expected one-line repo-stats header %q; got:\n%s", want, got)
	}
	// The legacy line-per-field format must not leak through.
	for _, legacy := range []string{
		"**Total repos:**",
		"**Forks excluded:**",
		"**Archived excluded:**",
		"**Repos scanned:**",
		"**Skipped:** ",
	} {
		if strings.Contains(got, legacy) {
			t.Errorf("did not expect legacy header line %q; got:\n%s", legacy, got)
		}
	}
}

func TestGenerateReport_HeaderRepoStatsLineWithoutExclusions(t *testing.T) {
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{{RepoName: "a", Results: allScored(5)}},
	}
	got := GenerateReport(withDefaultRules(sr))

	// No forks, no archived, no skipped → parenthetical is omitted entirely.
	if !strings.Contains(got, "**Repos:** 1 of 1 scanned\n") {
		t.Errorf("expected '**Repos:** 1 of 1 scanned' (no parenthetical); got:\n%s", got)
	}
	if strings.Contains(got, "scanned (") {
		t.Errorf("expected no parenthetical breakdown when nothing was excluded or skipped; got:\n%s", got)
	}
}

func TestGenerateReport_HeaderUsesBrLineBreaks(t *testing.T) {
	// CommonMark folds single newlines into one paragraph; explicit <br>
	// keeps each header line on its own line in any spec-compliant renderer.
	sr := ScanResult{
		Org:        "test-org",
		ScannedAt:  testTime,
		TotalRepos: 1,
		Results:    []RepoResult{{RepoName: "a", Results: allScored(5)}},
	}
	got := GenerateReport(withDefaultRules(sr))

	if !strings.Contains(got, "**Org:** test-org<br>\n") {
		t.Errorf("expected **Org:** line to end with <br>; got:\n%s", got)
	}
	if !strings.Contains(got, "**Scanned:** 2026-04-05 12:00 UTC<br>\n") {
		t.Errorf("expected **Scanned:** line to end with <br>; got:\n%s", got)
	}
}
