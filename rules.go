package scanner

import "strings"

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

func (r HasSubstantialReadme) Name() string { return "Has README over 2KB" }
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

func findFile(files []FileEntry, name string) (FileEntry, bool) {
	for _, f := range files {
		if f.Name == name {
			return f, true
		}
	}
	return FileEntry{}, false
}

func hasFile(files []FileEntry, name string) bool {
	_, ok := findFile(files, name)
	return ok
}
