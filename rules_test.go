package scanner

import (
	"testing"
	"time"
)

func TestHasRepoDescription_Pass(t *testing.T) {
	rule := HasRepoDescription{}

	if !rule.Check(Repo{Name: "my-repo", Description: "A useful service"}) {
		t.Error("expected pass for repo with description")
	}
}

func TestHasRepoDescription_Fail_Empty(t *testing.T) {
	rule := HasRepoDescription{}

	if rule.Check(Repo{Name: "my-repo", Description: ""}) {
		t.Error("expected fail for repo with empty description")
	}
}

func TestHasRepoDescription_Fail_WhitespaceOnly(t *testing.T) {
	rule := HasRepoDescription{}

	if rule.Check(Repo{Name: "my-repo", Description: "   \t\n"}) {
		t.Error("expected fail for repo with whitespace-only description")
	}
}

func TestHasActivity_Pass_RecentPush(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rule := HasActivity{Now: now}

	// Pushed 6 months ago - within the 12-month window.
	if !rule.Check(Repo{PushedAt: now.AddDate(0, -6, 0)}) {
		t.Error("expected pass for repo pushed 6 months ago")
	}
}

func TestHasActivity_Fail_OldPush(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rule := HasActivity{Now: now}

	// Pushed 2 years ago - outside the 12-month window.
	if rule.Check(Repo{PushedAt: now.AddDate(-2, 0, 0)}) {
		t.Error("expected fail for repo pushed 2 years ago")
	}
}

func TestHasActivity_Fail_ExactlyOneYearAgo(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rule := HasActivity{Now: now}

	// Pushed exactly 12 months ago (boundary): the window is "after now-1y",
	// so the boundary itself is NOT after now-1y - it should fail.
	if rule.Check(Repo{PushedAt: now.AddDate(-1, 0, 0)}) {
		t.Error("expected fail for repo pushed exactly 12 months ago (boundary)")
	}
}

func TestHasActivity_Fail_ZeroPushedAt(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	rule := HasActivity{Now: now}

	// Zero PushedAt (e.g. repo never pushed) should fail.
	if rule.Check(Repo{}) {
		t.Error("expected fail when PushedAt is zero")
	}
}

func TestHasActivity_DefaultsToTimeNow(t *testing.T) {
	rule := HasActivity{} // Now is zero - falls back to time.Now()

	// A repo pushed yesterday must pass regardless of when this test runs.
	if !rule.Check(Repo{PushedAt: time.Now().Add(-24 * time.Hour)}) {
		t.Error("expected pass for repo pushed yesterday with default Now")
	}
}

func TestHasReadme_Pass_README_md(t *testing.T) {
	rule := HasReadme{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "README.md"}}}) {
		t.Error("expected pass when README.md exists")
	}
}

func TestHasReadme_Pass_README_NoExtension(t *testing.T) {
	rule := HasReadme{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "README"}}}) {
		t.Error("expected pass when README (no extension) exists")
	}
}

func TestHasReadme_PassRegardlessOfSize(t *testing.T) {
	rule := HasReadme{}

	// The previous "substantial" variant required >2 KB. The replacement
	// drops the size threshold entirely - tiny READMEs still pass.
	if !rule.Check(Repo{Files: []FileEntry{{Path: "README.md", Size: 10}}}) {
		t.Error("expected pass for tiny README.md (size threshold dropped)")
	}
}

func TestHasReadme_Fail_Missing(t *testing.T) {
	rule := HasReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "main.go"}}}) {
		t.Error("expected fail when no README is present")
	}
}

func TestHasReadme_Pass_VariousCasesAndExtensions(t *testing.T) {
	rule := HasReadme{}

	// Wildcard match: case-insensitive, any extension or none.
	for _, name := range []string{"readme.md", "Readme.md", "README.rst", "README.txt", "README.markdown", "readme"} {
		if !rule.Check(Repo{Files: []FileEntry{{Path: name}}}) {
			t.Errorf("expected pass for %q", name)
		}
	}
}

func TestHasReadme_Fail_NestedReadmeOnly(t *testing.T) {
	rule := HasReadme{}

	// READMEs in subdirectories don't count as the project README.
	if rule.Check(Repo{Files: []FileEntry{{Path: "docs/README.md"}}}) {
		t.Error("expected fail for nested README only (root README is required)")
	}
}

func TestHasReadme_Fail_NameStartsWithReadmeButIsnt(t *testing.T) {
	rule := HasReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "READMEv2-stuff"}}}) {
		t.Error("expected fail for filenames that start with `readme` but aren't `readme` or `readme.<ext>`")
	}
}

func TestHasLicense_Pass_FromMetadata(t *testing.T) {
	rule := HasLicense{}

	// HasLicense reads repo.License (GitHub-auto-detected SPDX id),
	// regardless of which file produced it - no file-tree match.
	if !rule.Check(Repo{License: "MIT"}) {
		t.Error("expected pass when repo.License is set")
	}
}

func TestHasLicense_Fail_NoLicenseDetected(t *testing.T) {
	rule := HasLicense{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "LICENSE"}}}) {
		t.Error("expected fail when license file exists but GitHub didn't auto-detect it (License field empty)")
	}
}

func TestHasSecurityMd_Pass_Root(t *testing.T) {
	rule := HasSecurityMd{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "SECURITY.md"}}}) {
		t.Error("expected pass when SECURITY.md exists at root")
	}
}

func TestHasSecurityMd_Pass_GitHub(t *testing.T) {
	rule := HasSecurityMd{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: ".github/SECURITY.md"}}}) {
		t.Error("expected pass when .github/SECURITY.md exists")
	}
}

func TestHasSecurityMd_Pass_Docs(t *testing.T) {
	rule := HasSecurityMd{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "docs/SECURITY.md"}}}) {
		t.Error("expected pass when docs/SECURITY.md exists")
	}
}

func TestHasSecurityMd_Fail(t *testing.T) {
	rule := HasSecurityMd{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "README.md"}}}) {
		t.Error("expected fail when SECURITY.md is missing")
	}
}

func TestHasCIWorkflow_Pass_Yml(t *testing.T) {
	rule := HasCIWorkflow{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: ".github/workflows/ci.yml"}}}) {
		t.Error("expected pass when .yml workflow exists")
	}
}

func TestHasCIWorkflow_Pass_Yaml(t *testing.T) {
	rule := HasCIWorkflow{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: ".github/workflows/build.yaml"}}}) {
		t.Error("expected pass when .yaml workflow exists")
	}
}

func TestHasCIWorkflow_Fail_NoWorkflows(t *testing.T) {
	rule := HasCIWorkflow{}

	if rule.Check(Repo{Files: []FileEntry{{Path: ".github/CODEOWNERS"}}}) {
		t.Error("expected fail when no workflow files exist")
	}
}

func TestHasCIWorkflow_Fail_WrongExtension(t *testing.T) {
	rule := HasCIWorkflow{}

	if rule.Check(Repo{Files: []FileEntry{{Path: ".github/workflows/README.md"}}}) {
		t.Error("expected fail for non-yaml file in workflows")
	}
}

func TestHasCIWorkflow_Pass_OtherProviders(t *testing.T) {
	rule := HasCIWorkflow{}
	cases := []struct {
		name string
		path string
	}{
		{"CircleCI", ".circleci/config.yml"},
		{"GitLab CI", ".gitlab-ci.yml"},
		{"Travis CI", ".travis.yml"},
		{"Buildkite (any file under .buildkite/)", ".buildkite/pipelines/main.yml"},
		{"Azure Pipelines", "azure-pipelines.yml"},
		{"Jenkins", "Jenkinsfile"},
	}
	for _, tc := range cases {
		if !rule.Check(Repo{Files: []FileEntry{{Path: tc.path}}}) {
			t.Errorf("expected pass for %s (%s)", tc.name, tc.path)
		}
	}
}

func TestHasCodeowners_Pass_Root(t *testing.T) {
	rule := HasCodeowners{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "CODEOWNERS"}}}) {
		t.Error("expected pass when CODEOWNERS exists at root")
	}
}

func TestHasCodeowners_Pass_Docs(t *testing.T) {
	rule := HasCodeowners{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "docs/CODEOWNERS"}}}) {
		t.Error("expected pass when docs/CODEOWNERS exists")
	}
}

func TestHasCodeowners_Pass_GitHub(t *testing.T) {
	rule := HasCodeowners{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: ".github/CODEOWNERS"}}}) {
		t.Error("expected pass when .github/CODEOWNERS exists")
	}
}

func TestHasCodeowners_Fail(t *testing.T) {
	rule := HasCodeowners{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "README.md"}}}) {
		t.Error("expected fail when CODEOWNERS is missing from all locations")
	}
}

func TestHasBranchProtection_Pass(t *testing.T) {
	rule := HasBranchProtection{}

	if !rule.Check(Repo{BranchProtection: &BranchProtection{}}) {
		t.Error("expected pass when branch protection is enabled")
	}
}

func TestHasBranchProtection_Fail(t *testing.T) {
	rule := HasBranchProtection{}

	if rule.Check(Repo{BranchProtection: nil}) {
		t.Error("expected fail when branch protection is nil")
	}
}

func TestHasRequiredReviewers_Pass(t *testing.T) {
	rule := HasRequiredReviewers{}

	if !rule.Check(Repo{BranchProtection: &BranchProtection{RequiredReviewers: 1}}) {
		t.Error("expected pass when required reviewers >= 1")
	}
}

func TestHasRequiredReviewers_Fail_Zero(t *testing.T) {
	rule := HasRequiredReviewers{}

	if rule.Check(Repo{BranchProtection: &BranchProtection{RequiredReviewers: 0}}) {
		t.Error("expected fail when required reviewers is 0")
	}
}

func TestHasRequiredReviewers_Fail_NoProtection(t *testing.T) {
	rule := HasRequiredReviewers{}

	if rule.Check(Repo{BranchProtection: nil}) {
		t.Error("expected fail when branch protection is nil")
	}
}

func TestHasRequiredChecks_Pass(t *testing.T) {
	rule := HasRequiredChecks{}

	bp := &BranchProtection{RequiredStatusChecks: []string{"ci/build"}}
	if !rule.Check(Repo{BranchProtection: bp}) {
		t.Error("expected pass when status checks are configured")
	}
}

func TestHasRequiredChecks_Fail_Empty(t *testing.T) {
	rule := HasRequiredChecks{}

	bp := &BranchProtection{RequiredStatusChecks: []string{}}
	if rule.Check(Repo{BranchProtection: bp}) {
		t.Error("expected fail when status checks list is empty")
	}
}

func TestHasRequiredChecks_Fail_NoProtection(t *testing.T) {
	rule := HasRequiredChecks{}

	if rule.Check(Repo{BranchProtection: nil}) {
		t.Error("expected fail when branch protection is nil")
	}
}

func TestAllRules_Description_NonEmpty(t *testing.T) {
	for _, r := range AllRules() {
		if r.Description() == "" {
			t.Errorf("rule %q returned empty Description", r.Name())
		}
	}
}

func TestAllRules_CategorySetCorrectly(t *testing.T) {
	wantScored := map[string]bool{
		"Has branch protection":                  true,
		"Has required reviewers":                 true,
		"Has required checks":  true,
		"Has CODEOWNERS":                         true,
		"Has CI workflow":                        true,
	}
	wantAdditional := map[string]bool{
		"Has README":           true,
		"Has LICENSE":          true,
		"Has repo description": true,
		"Has activity":         true,
		"Has SECURITY.md":      true,
	}

	for _, r := range AllRules() {
		switch r.Category() {
		case CategoryScored:
			if !wantScored[r.Name()] {
				t.Errorf("rule %q is CategoryScored but not in expected scored set", r.Name())
			}
		case CategoryAdditional:
			if !wantAdditional[r.Name()] {
				t.Errorf("rule %q is CategoryAdditional but not in expected additional set", r.Name())
			}
		default:
			t.Errorf("rule %q has unknown category %q", r.Name(), r.Category())
		}
	}

	// Every rule in the expected sets must actually be in AllRules.
	gotNames := make(map[string]bool)
	for _, r := range AllRules() {
		gotNames[r.Name()] = true
	}
	for name := range wantScored {
		if !gotNames[name] {
			t.Errorf("expected scored rule %q missing from AllRules", name)
		}
	}
	for name := range wantAdditional {
		if !gotNames[name] {
			t.Errorf("expected additional rule %q missing from AllRules", name)
		}
	}
}

func TestAllRules_ImportanceOrder(t *testing.T) {
	got := AllRules()
	want := []string{
		// Scored, in importance order:
		"Has branch protection",
		"Has required reviewers",
		"Has required checks",
		"Has CODEOWNERS",
		"Has CI workflow",
		// Additional, in importance order:
		"Has README",
		"Has LICENSE",
		"Has repo description",
		"Has activity",
		"Has SECURITY.md",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d rules, got %d", len(want), len(got))
	}
	for i, name := range want {
		if got[i].Name() != name {
			t.Errorf("position %d: expected %q, got %q", i, name, got[i].Name())
		}
	}
}

func TestScoredRules_OnlyScored(t *testing.T) {
	for _, r := range ScoredRules() {
		if r.Category() != CategoryScored {
			t.Errorf("ScoredRules returned %q which has category %q", r.Name(), r.Category())
		}
	}
}

func TestAdditionalRules_OnlyAdditional(t *testing.T) {
	for _, r := range AdditionalRules() {
		if r.Category() != CategoryAdditional {
			t.Errorf("AdditionalRules returned %q which has category %q", r.Name(), r.Category())
		}
	}
}
