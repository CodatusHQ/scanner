package scanner

import "context"

// MockGitHubClient implements GitHubClient with canned responses for testing.
type MockGitHubClient struct {
	Repos         []Repo
	Err           error
	Tree          map[string][]FileEntry        // repo name -> file entries
	TreeErr       error
	Protection    map[string]*BranchProtection   // repo name -> branch protection
	ProtectionErr error
	IssueErr      error
	// CreatedIssue records the last CreateIssue call for assertions.
	CreatedIssue struct {
		Owner, Repo, Title, Body string
	}
}

func (m *MockGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	return m.Repos, m.Err
}

func (m *MockGitHubClient) GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error) {
	if m.TreeErr != nil {
		return nil, m.TreeErr
	}
	if m.Tree != nil {
		return m.Tree[repo], nil
	}
	return nil, nil
}

func (m *MockGitHubClient) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	if m.ProtectionErr != nil {
		return nil, m.ProtectionErr
	}
	if m.Protection != nil {
		return m.Protection[repo], nil
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
