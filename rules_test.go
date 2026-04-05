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

	if !rule.Check(Repo{Files: []FileEntry{{Name: "README.md"}, {Name: ".gitignore"}, {Name: "main.go"}}}) {
		t.Error("expected pass when .gitignore exists")
	}
}

func TestHasGitignore_Fail(t *testing.T) {
	rule := HasGitignore{}

	if rule.Check(Repo{Files: []FileEntry{{Name: "README.md"}, {Name: "main.go"}}}) {
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

	if !rule.Check(Repo{Files: []FileEntry{{Name: "README.md", Size: 3000}}}) {
		t.Error("expected pass for README.md over 2KB")
	}
}

func TestHasSubstantialReadme_Fail_TooSmall(t *testing.T) {
	rule := HasSubstantialReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Name: "README.md", Size: 2048}}}) {
		t.Error("expected fail for README.md exactly 2048 bytes")
	}
}

func TestHasSubstantialReadme_Fail_Missing(t *testing.T) {
	rule := HasSubstantialReadme{}

	if rule.Check(Repo{Files: []FileEntry{{Name: "main.go"}}}) {
		t.Error("expected fail when README.md is missing")
	}
}
