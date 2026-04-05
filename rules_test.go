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

func TestHasGitignore_Pass(t *testing.T) {
	rule := HasGitignore{}

	if !rule.Check(Repo{Files: []string{"README.md", ".gitignore", "main.go"}}) {
		t.Error("expected pass when .gitignore exists")
	}
}

func TestHasGitignore_Fail(t *testing.T) {
	rule := HasGitignore{}

	if rule.Check(Repo{Files: []string{"README.md", "main.go"}}) {
		t.Error("expected fail when .gitignore is missing")
	}
}

func TestHasGitignore_Fail_EmptyFiles(t *testing.T) {
	rule := HasGitignore{}

	if rule.Check(Repo{Files: nil}) {
		t.Error("expected fail when file list is empty")
	}
}
