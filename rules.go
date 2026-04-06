package scanner

import (
	"strings"
)

// Rule defines a named check that produces a pass/fail result for a repo.
type Rule interface {
	Name() string
	Check(repo Repo) bool
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

// HasGitignore checks that a .gitignore file exists in the repo root.
type HasGitignore struct{}

func (r HasGitignore) Name() string { return "Has .gitignore" }
func (r HasGitignore) Check(repo Repo) bool {
	return hasFile(repo.Files, ".gitignore")
}

// HasSubstantialReadme checks that README.md exists and is larger than 2048 bytes.
type HasSubstantialReadme struct{}

func (r HasSubstantialReadme) Name() string { return "Has substantial README" }
func (r HasSubstantialReadme) Check(repo Repo) bool {
	f, ok := findFile(repo.Files, "README.md")
	return ok && f.Size > 2048
}

// HasLicense checks that a LICENSE or LICENSE.md file exists in the repo root.
type HasLicense struct{}

func (r HasLicense) Name() string { return "Has LICENSE" }
func (r HasLicense) Check(repo Repo) bool {
	return hasFile(repo.Files, "LICENSE") || hasFile(repo.Files, "LICENSE.md")
}

// HasSecurityMd checks that SECURITY.md exists in the repo root or .github/.
type HasSecurityMd struct{}

func (r HasSecurityMd) Name() string { return "Has SECURITY.md" }
func (r HasSecurityMd) Check(repo Repo) bool {
	return hasFile(repo.Files, "SECURITY.md") || hasFile(repo.Files, ".github/SECURITY.md")
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

// HasCodeowners checks that a CODEOWNERS file exists in root, docs/, or .github/.
type HasCodeowners struct{}

func (r HasCodeowners) Name() string { return "Has CODEOWNERS" }
func (r HasCodeowners) Check(repo Repo) bool {
	return hasFile(repo.Files, "CODEOWNERS") ||
		hasFile(repo.Files, "docs/CODEOWNERS") ||
		hasFile(repo.Files, ".github/CODEOWNERS")
}

// HasBranchProtection checks that the default branch has protection rules enabled.
type HasBranchProtection struct{}

func (r HasBranchProtection) Name() string { return "Has branch protection" }
func (r HasBranchProtection) Check(repo Repo) bool {
	return repo.BranchProtection != nil
}

// HasRequiredReviewers checks that at least one approving review is required.
type HasRequiredReviewers struct{}

func (r HasRequiredReviewers) Name() string { return "Has required reviewers" }
func (r HasRequiredReviewers) Check(repo Repo) bool {
	return repo.BranchProtection != nil && repo.BranchProtection.RequiredReviewers >= 1
}

// HasRequiredStatusChecks checks that at least one status check is required before merging.
type HasRequiredStatusChecks struct{}

func (r HasRequiredStatusChecks) Name() string { return "Requires status checks before merging" }
func (r HasRequiredStatusChecks) Check(repo Repo) bool {
	return repo.BranchProtection != nil && len(repo.BranchProtection.RequiredStatusChecks) > 0
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
