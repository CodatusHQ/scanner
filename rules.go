package scanner

import (
	"strings"
)

// Rule defines a named check that produces a pass/fail result for a repo.
// Description and HowToFix supply the per-rule text used by the Markdown
// report's Rule reference section.
type Rule interface {
	Name() string
	Check(repo Repo) bool
	Description() string
	HowToFix() string
}

// RuleResult holds the outcome of a single rule check for a single repo.
type RuleResult struct {
	RuleName string
	Passed   bool
}

// AllRules returns the ordered list of rules the scanner evaluates.
func AllRules() []Rule {
	return []Rule{
		HasRepoDescription{},
		HasGitignore{},
		HasSubstantialReadme{},
		HasLicense{},
		HasSecurityMd{},
		HasCIWorkflow{},
		HasTestDirectory{},
		HasCodeowners{},
		HasBranchProtection{},
		HasRequiredReviewers{},
		HasRequiredStatusChecks{},
	}
}

// HasRepoDescription checks that the repo description field is not blank.
type HasRepoDescription struct{}

func (r HasRepoDescription) Name() string { return "Has repo description" }
func (r HasRepoDescription) Check(repo Repo) bool {
	return strings.TrimSpace(repo.Description) != ""
}
func (r HasRepoDescription) Description() string {
	return "The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page)."
}
func (r HasRepoDescription) HowToFix() string {
	return "Edit the repo and add a one-line description."
}

// HasGitignore checks that a .gitignore file exists in the repo root.
type HasGitignore struct{}

func (r HasGitignore) Name() string { return "Has .gitignore" }
func (r HasGitignore) Check(repo Repo) bool {
	return hasFile(repo.Files, ".gitignore")
}
func (r HasGitignore) Description() string {
	return "A .gitignore file exists at the repository root."
}
func (r HasGitignore) HowToFix() string {
	return "Add a .gitignore at the repo root. [GitHub publishes templates per language](https://github.com/github/gitignore)."
}

// HasSubstantialReadme checks that README.md exists and is larger than 2048 bytes.
type HasSubstantialReadme struct{}

func (r HasSubstantialReadme) Name() string { return "Has substantial README" }
func (r HasSubstantialReadme) Check(repo Repo) bool {
	f, ok := findFile(repo.Files, "README.md")
	return ok && f.Size > 2048
}
func (r HasSubstantialReadme) Description() string {
	return "A README.md file exists at the repository root and is larger than 2 KB."
}
func (r HasSubstantialReadme) HowToFix() string {
	return "Expand your README to cover what the project is, how to install it, and how to use it."
}

// HasLicense checks that a LICENSE or LICENSE.md file exists in the repo root.
type HasLicense struct{}

func (r HasLicense) Name() string { return "Has LICENSE" }
func (r HasLicense) Check(repo Repo) bool {
	return hasFile(repo.Files, "LICENSE") || hasFile(repo.Files, "LICENSE.md")
}
func (r HasLicense) Description() string {
	return "A LICENSE or LICENSE.md file exists at the repository root."
}
func (r HasLicense) HowToFix() string {
	return "Pick a license at [choosealicense.com](https://choosealicense.com) and add it to your repo root."
}

// HasSecurityMd checks that SECURITY.md exists in the repo root or .github/.
type HasSecurityMd struct{}

func (r HasSecurityMd) Name() string { return "Has SECURITY.md" }
func (r HasSecurityMd) Check(repo Repo) bool {
	return hasFile(repo.Files, "SECURITY.md") || hasFile(repo.Files, ".github/SECURITY.md")
}
func (r HasSecurityMd) Description() string {
	return "A SECURITY.md file exists at the repository root or in .github/."
}
func (r HasSecurityMd) HowToFix() string {
	return "Add a SECURITY.md describing how to report vulnerabilities. [GitHub's template](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository)."
}

// HasCIWorkflow checks that at least one .yml or .yaml file exists under .github/workflows/.
type HasCIWorkflow struct{}

func (r HasCIWorkflow) Name() string { return "Has CI workflow" }
func (r HasCIWorkflow) Check(repo Repo) bool {
	for _, f := range repo.Files {
		if strings.HasPrefix(f.Path, ".github/workflows/") &&
			(strings.HasSuffix(f.Path, ".yml") || strings.HasSuffix(f.Path, ".yaml")) {
			return true
		}
	}
	return false
}
func (r HasCIWorkflow) Description() string {
	return "At least one .yml or .yaml workflow file exists in .github/workflows/."
}
func (r HasCIWorkflow) HowToFix() string {
	return "Add a YAML workflow in .github/workflows/. [GitHub Actions quickstart](https://docs.github.com/en/actions/quickstart)."
}

// HasTestDirectory checks that a recognized test directory exists at the repo root.
type HasTestDirectory struct{}

func (r HasTestDirectory) Name() string { return "Has test directory" }
func (r HasTestDirectory) Check(repo Repo) bool {
	testDirs := []string{"test", "tests", "__tests__", "spec", "specs"}
	for _, dir := range testDirs {
		if hasDir(repo.Files, dir) {
			return true
		}
	}
	return false
}
func (r HasTestDirectory) Description() string {
	return "A directory named test, tests, __tests__, spec, or specs exists at the repository root."
}
func (r HasTestDirectory) HowToFix() string {
	return "Create a test/ or tests/ directory at the repo root and add at least one test file."
}

// HasCodeowners checks that a CODEOWNERS file exists in root, docs/, or .github/.
type HasCodeowners struct{}

func (r HasCodeowners) Name() string { return "Has CODEOWNERS" }
func (r HasCodeowners) Check(repo Repo) bool {
	return hasFile(repo.Files, "CODEOWNERS") ||
		hasFile(repo.Files, "docs/CODEOWNERS") ||
		hasFile(repo.Files, ".github/CODEOWNERS")
}
func (r HasCodeowners) Description() string {
	return "A CODEOWNERS file exists at the repo root, in .github/, or in docs/."
}
func (r HasCodeowners) HowToFix() string {
	return "Add a CODEOWNERS file mapping paths to GitHub users or teams. [GitHub docs](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)."
}

// HasBranchProtection checks that the default branch has protection rules enabled.
type HasBranchProtection struct{}

func (r HasBranchProtection) Name() string { return "Has branch protection" }
func (r HasBranchProtection) Check(repo Repo) bool {
	return repo.BranchProtection != nil
}
func (r HasBranchProtection) Description() string {
	return "A branch-protection rule is configured on the default branch."
}
func (r HasBranchProtection) HowToFix() string {
	return "In repo Settings > Branches, add a protection rule for the default branch. [GitHub docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches)."
}

// HasRequiredReviewers checks that at least one approving review is required.
type HasRequiredReviewers struct{}

func (r HasRequiredReviewers) Name() string { return "Has required reviewers" }
func (r HasRequiredReviewers) Check(repo Repo) bool {
	return repo.BranchProtection != nil && repo.BranchProtection.RequiredReviewers >= 1
}
func (r HasRequiredReviewers) Description() string {
	return "The default branch's protection rules require at least one approving review before a PR can be merged."
}
func (r HasRequiredReviewers) HowToFix() string {
	return `In repo Settings > Branches, edit the default-branch protection rule and turn on "Require pull request reviews before merging" with at least 1 required reviewer.`
}

// HasRequiredStatusChecks checks that at least one status check is required before merging.
type HasRequiredStatusChecks struct{}

func (r HasRequiredStatusChecks) Name() string { return "Requires status checks before merging" }
func (r HasRequiredStatusChecks) Check(repo Repo) bool {
	return repo.BranchProtection != nil && len(repo.BranchProtection.RequiredStatusChecks) > 0
}
func (r HasRequiredStatusChecks) Description() string {
	return "The default branch's protection rules require at least one status check to pass before a PR can be merged."
}
func (r HasRequiredStatusChecks) HowToFix() string {
	return `In repo Settings > Branches, edit the default-branch protection rule and turn on "Require status checks to pass before merging".`
}

func findFile(files []FileEntry, path string) (FileEntry, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return FileEntry{}, false
}

func hasFile(files []FileEntry, path string) bool {
	_, ok := findFile(files, path)
	return ok
}

func hasDir(files []FileEntry, path string) bool {
	for _, f := range files {
		if f.Path == path && f.Type == "tree" {
			return true
		}
	}
	return false
}
