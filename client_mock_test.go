package scanner

import "context"

// MockGitHubClient implements GitHubClient with canned responses for testing.
type MockGitHubClient struct {
	Repos          []Repo
	Err            error
	Tree           map[string][]FileEntry       // repo name -> file entries
	TreeErr        error                        // global tree error (used if TreeErrs is nil)
	TreeErrs       map[string]error             // repo name -> per-repo tree error
	Protection     map[string]*BranchProtection  // repo name -> classic branch protection
	ProtectionErr  error                        // global protection error (used if ProtectionErrs is nil)
	ProtectionErrs map[string]error             // repo name -> per-repo protection error
	Rulesets       map[string]*BranchProtection  // repo name -> rulesets protection
	RulesetsErr    error                        // global rulesets error (used if RulesetsErrs is nil)
	RulesetsErrs   map[string]error             // repo name -> per-repo rulesets error
	IssueErr       error
	// CreatedIssue records the last CreateIssue call for assertions.
	CreatedIssue struct {
		Owner, Repo, Title, Body string
	}
}

func (m *MockGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	return m.Repos, m.Err
}

func (m *MockGitHubClient) GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error) {
	if m.TreeErrs != nil {
		if err, ok := m.TreeErrs[repo]; ok {
			return nil, err
		}
	}
	if m.TreeErr != nil {
		return nil, m.TreeErr
	}
	if m.Tree != nil {
		return m.Tree[repo], nil
	}
	return nil, nil
}

func (m *MockGitHubClient) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	if m.ProtectionErrs != nil {
		if err, ok := m.ProtectionErrs[repo]; ok {
			return nil, err
		}
	}
	if m.ProtectionErr != nil {
		return nil, m.ProtectionErr
	}
	if m.Protection != nil {
		return m.Protection[repo], nil
	}
	return nil, nil
}

func (m *MockGitHubClient) GetRulesets(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	if m.RulesetsErrs != nil {
		if err, ok := m.RulesetsErrs[repo]; ok {
			return nil, err
		}
	}
	if m.RulesetsErr != nil {
		return nil, m.RulesetsErr
	}
	if m.Rulesets != nil {
		return m.Rulesets[repo], nil
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
