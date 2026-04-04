package main

import "strings"

// RuleResult holds the outcome of a single rule check for a single repo.
type RuleResult struct {
	RuleName string
	Passed   bool
}

// Rule defines a named check that produces a pass/fail result for a repo.
type Rule struct {
	Name  string
	Check func(repo Repo) RuleResult
}

// AllRules returns the ordered list of rules the scanner evaluates.
func AllRules() []Rule {
	return []Rule{
		{
			Name: "Has repo description",
			Check: func(repo Repo) RuleResult {
				passed := strings.TrimSpace(repo.Description) != ""
				return RuleResult{RuleName: "Has repo description", Passed: passed}
			},
		},
	}
}
