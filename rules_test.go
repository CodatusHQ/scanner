package main

import "testing"

func TestHasRepoDescription_Pass(t *testing.T) {
	rule := AllRules()[0]
	repo := Repo{Name: "my-repo", Description: "A useful service"}

	result := rule.Check(repo)

	if !result.Passed {
		t.Errorf("expected pass for repo with description, got fail")
	}
	if result.RuleName != "Has repo description" {
		t.Errorf("unexpected rule name: %s", result.RuleName)
	}
}

func TestHasRepoDescription_Fail_Empty(t *testing.T) {
	rule := AllRules()[0]
	repo := Repo{Name: "my-repo", Description: ""}

	result := rule.Check(repo)

	if result.Passed {
		t.Errorf("expected fail for repo with empty description, got pass")
	}
}

func TestHasRepoDescription_Fail_WhitespaceOnly(t *testing.T) {
	rule := AllRules()[0]
	repo := Repo{Name: "my-repo", Description: "   \t\n"}

	result := rule.Check(repo)

	if result.Passed {
		t.Errorf("expected fail for repo with whitespace-only description, got pass")
	}
}
