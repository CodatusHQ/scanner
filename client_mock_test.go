package scanner

import "context"

// MockGitHubClient implements GitHubClient with canned responses for testing.
type MockGitHubClient struct {
	Repos    []Repo
	Err      error
	Files    map[string][]string // repo name -> file list
	FilesErr error
	IssueErr error
	// CreatedIssue records the last CreateIssue call for assertions.
	CreatedIssue struct {
		Owner, Repo, Title, Body string
	}
}

func (m *MockGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	return m.Repos, m.Err
}

func (m *MockGitHubClient) ListFiles(ctx context.Context, owner, repo string) ([]string, error) {
	if m.FilesErr != nil {
		return nil, m.FilesErr
	}
	if m.Files != nil {
		return m.Files[repo], nil
	}
	return nil, nil
}

func (m *MockGitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string) error {
	m.CreatedIssue.Owner = owner
	m.CreatedIssue.Repo = repo
	m.CreatedIssue.Title = title
	m.CreatedIssue.Body = body
	return m.IssueErr
}
