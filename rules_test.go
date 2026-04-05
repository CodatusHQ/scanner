package scanner

import "testing"

func TestHasRepoDescription_Pass(t *testing.T) {
	rule := HasRepoDescription{}

	if !rule.Check(Repo{Name: "my-repo", Description: "A useful service"}) {
		t.Errorf("expected pass for repo with description")
	}
}

func TestHasRepoDescription_Fail_Empty(t *testing.T) {
	rule := HasRepoDescription{}

	if rule.Check(Repo{Name: "my-repo", Description: ""}) {
		t.Errorf("expected fail for repo with empty description")
	}
}

func TestHasRepoDescription_Fail_WhitespaceOnly(t *testing.T) {
	rule := HasRepoDescription{}

	if rule.Check(Repo{Name: "my-repo", Description: "   \t\n"}) {
		t.Errorf("expected fail for repo with whitespace-only description")
	}
}
