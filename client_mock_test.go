package scanner

import "context"

// MockGitHubClient implements GitHubClient with canned responses for testing.
type MockGitHubClient struct {
	Repos []Repo
	Err   error
}

func (m *MockGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	return m.Repos, m.Err
}

func (m *MockGitHubClient) ListFiles(ctx context.Context, owner, repo string) ([]string, error) {
	return nil, nil
}
