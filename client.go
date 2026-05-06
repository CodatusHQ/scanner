package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/go-github/v72/github"
)

// Sentinel errors for per-repo scan failures.
var (
	ErrEmptyRepo     = errors.New("repository is empty")
	ErrTruncatedTree = errors.New("tree truncated by GitHub API")
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
	Fork             bool
	PushedAt         time.Time         // most recent push to any branch (from list-repos)
	License          string            // SPDX id GitHub auto-detected (Licensee), "" if none
	Files            []FileEntry       // all files and directories in the repo
	BranchProtection *BranchProtection // nil if no protection configured
}

// GitHubClient is the interface for all GitHub API interactions.
// The scanner depends only on this interface, making it testable via mocks.
type GitHubClient interface {
	// ListReposByAccount lists repos for a named org (falls back to user on 404).
	// Used by PAT auth.
	ListReposByAccount(ctx context.Context, name string) ([]Repo, error)
	// ListReposByInstallation lists the repos the current GitHub App installation
	// was granted access to. Used by installation-token auth.
	ListReposByInstallation(ctx context.Context) ([]Repo, error)
	GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error)
	GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
	GetRulesets(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
	// GetBranchInfo reads the public GET /repos/{o}/{r}/branches/{br}
	// endpoint, which exposes the protected flag and (for classic
	// per-repo branch protection) the required-status-check contexts to
	// any reader - including non-admins on public repos. This is the
	// fallback when the admin GetBranchProtection 404s and there are no
	// rulesets, so the scanner can still tell whether protection is on
	// and which status checks are required. Required-reviewer counts
	// are NOT exposed here (admin-only field on classic protection).
	GetBranchInfo(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
}

type realGitHubClient struct {
	client *github.Client
}

// NewGitHubClient creates a GitHubClient that calls the public GitHub REST API.
func NewGitHubClient(token string) GitHubClient {
	return newGitHubClient(token, "")
}

// newGitHubClient creates a GitHubClient with an optional custom base URL.
// An empty baseURL uses the default GitHub API URL.
func newGitHubClient(token, baseURL string) GitHubClient {
	client := github.NewClient(nil).WithAuthToken(token)
	if baseURL != "" {
		u, _ := url.Parse(baseURL + "/")
		client.BaseURL = u
	}
	return &realGitHubClient{client: client}
}

// IsRateLimitError reports whether an error is a GitHub rate limit error
// (primary or secondary). Rate limit errors must never be swallowed -
// they indicate a global problem that affects all subsequent API calls.
// Exported so callers (e.g., bulk-scan) can decide whether to abort a
// multi-org run on the first rate-limited org rather than continue and
// fail every subsequent call.
func IsRateLimitError(err error) bool {
	var rateLimitErr *github.RateLimitError
	var abuseErr *github.AbuseRateLimitError
	return errors.As(err, &rateLimitErr) || errors.As(err, &abuseErr)
}

// repoFromGitHub builds a Repo from a go-github *Repository payload. The
// listing response is rich (~50 fields) but the scanner needs only a
// handful, plus GitHub's auto-detected license SPDX id (which lets the
// HasLicense rule pass even when the file is named LICENSE.txt, COPYING,
// LICENCE, etc. - anything Licensee recognizes).
func repoFromGitHub(r *github.Repository) Repo {
	license := ""
	if r.License != nil {
		license = r.License.GetSPDXID()
	}
	return Repo{
		Name:          r.GetName(),
		Description:   r.GetDescription(),
		DefaultBranch: r.GetDefaultBranch(),
		Archived:      r.GetArchived(),
		Fork:          r.GetFork(),
		PushedAt:      r.GetPushedAt().Time,
		License:       license,
	}
}

// ListReposByAccount lists repos for a named account. Tries /orgs/{name}/repos
// first; on 404 (not an org) falls back to /users/{name}/repos.
func (c *realGitHubClient) ListReposByAccount(ctx context.Context, name string) ([]Repo, error) {
	repos, err := c.listOrgRepos(ctx, name)
	var errResp *github.ErrorResponse
	if err != nil && errors.As(err, &errResp) && errResp.Response.StatusCode == http.StatusNotFound {
		return c.listUserRepos(ctx, name)
	}
	return repos, err
}

func (c *realGitHubClient) listOrgRepos(ctx context.Context, org string) ([]Repo, error) {
	var allRepos []Repo
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		ghRepos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			if IsRateLimitError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("list repos for org %s: %w", org, err)
		}

		for _, r := range ghRepos {
			allRepos = append(allRepos, repoFromGitHub(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *realGitHubClient) listUserRepos(ctx context.Context, user string) ([]Repo, error) {
	var allRepos []Repo
	opts := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		ghRepos, resp, err := c.client.Repositories.ListByUser(ctx, user, opts)
		if err != nil {
			if IsRateLimitError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("list repos for user %s: %w", user, err)
		}

		for _, r := range ghRepos {
			allRepos = append(allRepos, repoFromGitHub(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

// ListReposByInstallation lists the repositories the current GitHub App
// installation can access. The token passed to NewGitHubClient must be an
// installation access token.
func (c *realGitHubClient) ListReposByInstallation(ctx context.Context) ([]Repo, error) {
	var allRepos []Repo
	opts := &github.ListOptions{PerPage: 100}

	for {
		result, resp, err := c.client.Apps.ListRepos(ctx, opts)
		if err != nil {
			if IsRateLimitError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("list installation repos: %w", err)
		}

		for _, r := range result.Repositories {
			allRepos = append(allRepos, repoFromGitHub(r))
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *realGitHubClient) GetTree(ctx context.Context, owner, repo, branch string) ([]FileEntry, error) {
	tree, resp, err := c.client.Git.GetTree(ctx, owner, repo, branch, true)
	if err != nil {
		if IsRateLimitError(err) {
			return nil, err
		}
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return nil, ErrEmptyRepo
		}
		return nil, fmt.Errorf("get tree for %s/%s: %w", owner, repo, err)
	}

	if tree.GetTruncated() {
		return nil, ErrTruncatedTree
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
		if IsRateLimitError(err) {
			return nil, err
		}
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

// GetRulesets reads the public /repos/{o}/{r}/rules/branches/{br}
// endpoint and aggregates merge-gate signals into a BranchProtection.
//
// We bypass go-github's typed GetRulesForBranch in favor of raw JSON
// parsing because go-github v72-v80 lacks a typed wrapper for the
// `code_quality` rule type (a real, currently-used rule type). With
// raw JSON, supporting a new rule type is a one-line change in the
// switch below rather than tracking go-github releases.
//
// Two categories contribute to BranchProtection:
//   - pull_request: review count goes into RequiredReviewers
//   - merge gates: any rule type that requires "something to pass before
//     merge" contributes its rule-type name to RequiredStatusChecks. The
//     rule's Check function only needs len() > 0; the strings are
//     placeholders, not diagnostic
//
// Other rule types (deletion, non_fast_forward, required_signatures,
// merge_queue, etc.) enforce branch shape or routing, not "must pass
// before merge," and are intentionally ignored.
func (c *realGitHubClient) GetRulesets(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	req, err := c.client.NewRequest(http.MethodGet, fmt.Sprintf("repos/%s/%s/rules/branches/%s", owner, repo, branch), nil)
	if err != nil {
		return nil, fmt.Errorf("build request for branch rules %s/%s: %w", owner, repo, err)
	}

	var rules []struct {
		Type       string          `json:"type"`
		Parameters json.RawMessage `json:"parameters"`
	}
	resp, err := c.client.Do(ctx, req, &rules)
	if err != nil {
		if IsRateLimitError(err) {
			return nil, err
		}
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden) {
			return nil, nil
		}
		return nil, fmt.Errorf("get branch rules for %s/%s: %w", owner, repo, err)
	}

	var bp BranchProtection
	for _, r := range rules {
		switch r.Type {
		case "pull_request":
			var p struct {
				RequiredApprovingReviewCount int `json:"required_approving_review_count"`
			}
			if json.Unmarshal(r.Parameters, &p) == nil && p.RequiredApprovingReviewCount > bp.RequiredReviewers {
				bp.RequiredReviewers = p.RequiredApprovingReviewCount
			}
		case "required_status_checks", "workflows", "code_scanning", "code_quality", "required_deployments":
			bp.RequiredStatusChecks = append(bp.RequiredStatusChecks, r.Type)
		}
	}

	if bp.RequiredReviewers == 0 && len(bp.RequiredStatusChecks) == 0 {
		return nil, nil
	}
	return &bp, nil
}

// GetBranchInfo calls the public GET /repos/{o}/{r}/branches/{br}
// endpoint and pulls protection details out of the inline `protection`
// object. This is the public fallback for non-admin scans: the admin
// branch-protection endpoint 404s for non-admins, but this endpoint is
// readable by anyone for public repos and exposes:
//
//   - protected: bool (used to populate has_branch_protection)
//   - protection.required_status_checks.contexts: []string (used to
//     populate has_required_status_checks)
//
// Returns nil when the branch isn't protected. Required-reviewer count
// is not exposed by this endpoint - that field stays admin-only.
func (c *realGitHubClient) GetBranchInfo(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	br, resp, err := c.client.Repositories.GetBranch(ctx, owner, repo, branch, 0)
	if err != nil {
		if IsRateLimitError(err) {
			return nil, err
		}
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden) {
			return nil, nil
		}
		return nil, fmt.Errorf("get branch info for %s/%s: %w", owner, repo, err)
	}
	if !br.GetProtected() {
		return nil, nil
	}
	bp := &BranchProtection{}
	if prot := br.GetProtection(); prot != nil {
		if rsc := prot.GetRequiredStatusChecks(); rsc != nil && rsc.Contexts != nil {
			bp.RequiredStatusChecks = append([]string(nil), *rsc.Contexts...)
		}
	}
	return bp, nil
}
