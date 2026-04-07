package scanner

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v72/github"
)

// FileEntry represents a file or directory in a repo.
type FileEntry struct {
	Path string // full path relative to repo root (e.g., ".github/workflows/ci.yml")
	Size int
	Type string // "blob" (file) or "tree" (directory)
}

// BranchProtection holds the branch protection settings the scanner needs.
type BranchProtection struct {
	RequiredReviewers    int
	RequiredStatusChecks []string
}

// Repo represents a GitHub repository with the fields the scanner needs.
type Repo struct {
	Name             string
	Description      string
	DefaultBranch    string
	Archived         bool
	Files            []FileEntry       // all files and directories in the repo
	BranchProtection *BranchProtection // nil if no protection configured
}

// GitHubClient is the interface for all GitHub API interactions.
// The scanner depends only on this interface, making it testable via mocks.
type GitHubClient interface {
	ListRepos(ctx context.Context, org string) ([]Repo, error)
	GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error)
	GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
	GetRulesets(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
	CreateIssue(ctx context.Context, owner, repo, title, body string) error
}

type realGitHubClient struct {
	client *github.Client
}

// NewGitHubClient creates a GitHubClient that calls the GitHub REST API.
func NewGitHubClient(token string) GitHubClient {
	return &realGitHubClient{
		client: github.NewClient(nil).WithAuthToken(token),
	}
}

func (c *realGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	var allRepos []Repo
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		ghRepos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("list repos for org %s: %w", org, err)
		}

		for _, r := range ghRepos {
			allRepos = append(allRepos, Repo{
				Name:          r.GetName(),
				Description:   r.GetDescription(),
				DefaultBranch: r.GetDefaultBranch(),
				Archived:      r.GetArchived(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *realGitHubClient) GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error) {
	tree, _, err := c.client.Git.GetTree(ctx, owner, repo, branch, true)
	if err != nil {
		return nil, fmt.Errorf("get tree for %s/%s: %w", owner, repo, err)
	}

	files := make([]FileEntry, len(tree.Entries))
	for i, e := range tree.Entries {
		files[i] = FileEntry{
			Path: e.GetPath(),
			Type: e.GetType(),
			Size: e.GetSize(),
		}
	}
	return files, nil
}

func (c *realGitHubClient) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	prot, resp, err := c.client.Repositories.GetBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden) {
			return nil, nil
		}
		return nil, fmt.Errorf("get branch protection for %s/%s: %w", owner, repo, err)
	}

	bp := &BranchProtection{}
	if prot.RequiredPullRequestReviews != nil {
		bp.RequiredReviewers = prot.RequiredPullRequestReviews.RequiredApprovingReviewCount
	}
	if prot.RequiredStatusChecks != nil && prot.RequiredStatusChecks.Contexts != nil {
		bp.RequiredStatusChecks = *prot.RequiredStatusChecks.Contexts
	}
	return bp, nil
}

func (c *realGitHubClient) GetRulesets(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	rules, resp, err := c.client.Repositories.GetRulesForBranch(ctx, owner, repo, branch, nil)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden) {
			return nil, nil
		}
		return nil, fmt.Errorf("get branch rules for %s/%s: %w", owner, repo, err)
	}

	var bp BranchProtection
	found := false

	for _, pr := range rules.PullRequest {
		found = true
		if pr.Parameters.RequiredApprovingReviewCount > bp.RequiredReviewers {
			bp.RequiredReviewers = pr.Parameters.RequiredApprovingReviewCount
		}
	}

	for _, sc := range rules.RequiredStatusChecks {
		found = true
		for _, check := range sc.Parameters.RequiredStatusChecks {
			bp.RequiredStatusChecks = append(bp.RequiredStatusChecks, check.Context)
		}
	}

	if !found {
		return nil, nil
	}
	return &bp, nil
}

func (c *realGitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string) error {
	req := &github.IssueRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(body),
	}

	_, _, err := c.client.Issues.Create(ctx, owner, repo, req)
	if err != nil {
		return fmt.Errorf("create issue in %s/%s: %w", owner, repo, err)
	}
	return nil
}
