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
	}
}

// HasRepoDescription checks that the repo description field is not blank.
type HasRepoDescription struct{}

func (r HasRepoDescription) Name() string { return "Has repo description" }
func (r HasRepoDescription) Check(repo Repo) bool {
	return strings.TrimSpace(repo.Description) != ""
}
