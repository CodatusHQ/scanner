package scanner

import "testing"

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

func TestHasGitignore_Pass(t *testing.T) {
	rule := HasGitignore{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: ".gitignore"}}}) {
		t.Error("expected pass when .gitignore exists")
	}
}

func TestHasGitignore_Fail(t *testing.T) {
	rule := HasGitignore{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "README.md"}}}) {
		t.Error("expected fail when .gitignore is missing")
	}
}

func TestHasGitignore_Fail_EmptyFiles(t *testing.T) {
	rule := HasGitignore{}

	if rule.Check(Repo{Files: nil}) {
		t.Error("expected fail when file list is empty")
	}
}

func TestHasSubstantialReadme_Pass(t *testing.T) {
	rule := HasSubstantialReadme{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "README.md", Size: 3000}}}) {
		t.Error("expected pass for README.md over 2KB")
	}
}

func TestHasSubstantialReadme_Fail_TooSmall(t *testing.T) {
	rule := HasSubstantialReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "README.md", Size: 2048}}}) {
		t.Error("expected fail for README.md exactly 2048 bytes")
	}
}

func TestHasSubstantialReadme_Fail_Missing(t *testing.T) {
	rule := HasSubstantialReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "main.go"}}}) {
		t.Error("expected fail when README.md is missing")
	}
}

func TestHasLicense_Pass_LICENSE(t *testing.T) {
	rule := HasLicense{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "LICENSE"}}}) {
		t.Error("expected pass when LICENSE exists")
	}
}

func TestHasLicense_Pass_LICENSEmd(t *testing.T) {
	rule := HasLicense{}

	if !rule.Check(Repo{Files: []FileEntry{{Path: "LICENSE.md"}}}) {
		t.Error("expected pass when LICENSE.md exists")
	}
}

func TestHasLicense_Fail(t *testing.T) {
	rule := HasLicense{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "README.md"}}}) {
		t.Error("expected fail when no LICENSE file exists")
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

func TestHasTestDirectory_Pass(t *testing.T) {
	for _, dir := range []string{"test", "tests", "__tests__", "spec", "specs"} {
		rule := HasTestDirectory{}

		if !rule.Check(Repo{Files: []FileEntry{{Path: dir, Type: "tree"}}}) {
			t.Errorf("expected pass for directory %q", dir)
		}
	}
}

func TestHasTestDirectory_Fail(t *testing.T) {
	rule := HasTestDirectory{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "src", Type: "tree"}}}) {
		t.Error("expected fail when no test directory exists")
	}
}

func TestHasTestDirectory_Fail_FileNotDir(t *testing.T) {
	rule := HasTestDirectory{}

	if rule.Check(Repo{Files: []FileEntry{{Path: "test", Type: "blob"}}}) {
		t.Error("expected fail when 'test' is a file, not a directory")
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

func TestHasRequiredStatusChecks_Pass(t *testing.T) {
	rule := HasRequiredStatusChecks{}

	bp := &BranchProtection{RequiredStatusChecks: []string{"ci/build"}}
	if !rule.Check(Repo{BranchProtection: bp}) {
		t.Error("expected pass when status checks are configured")
	}
}

func TestHasRequiredStatusChecks_Fail_Empty(t *testing.T) {
	rule := HasRequiredStatusChecks{}

	bp := &BranchProtection{RequiredStatusChecks: []string{}}
	if rule.Check(Repo{BranchProtection: bp}) {
		t.Error("expected fail when status checks list is empty")
	}
}

func TestHasRequiredStatusChecks_Fail_NoProtection(t *testing.T) {
	rule := HasRequiredStatusChecks{}

	if rule.Check(Repo{BranchProtection: nil}) {
		t.Error("expected fail when branch protection is nil")
	}
}

func TestAllRules_DescriptionAndHowToFix_NonEmpty(t *testing.T) {
	for _, r := range AllRules() {
		if r.Description() == "" {
			t.Errorf("rule %q returned empty Description", r.Name())
		}
		if r.HowToFix() == "" {
			t.Errorf("rule %q returned empty HowToFix", r.Name())
		}
	}
}
