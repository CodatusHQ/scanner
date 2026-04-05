package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Repo represents a GitHub repository with the fields the scanner needs.
type Repo struct {
	Name          string
	Description   string
	DefaultBranch string
	Archived      bool
	Files         []string // root-level file and directory names
}

// GitHubClient is the interface for all GitHub API interactions.
// The scanner depends only on this interface, making it testable via mocks.
type GitHubClient interface {
	ListRepos(ctx context.Context, org string) ([]Repo, error)
	ListFiles(ctx context.Context, owner, repo string) ([]string, error)
	CreateIssue(ctx context.Context, owner, repo, title, body string) error
}

type realGitHubClient struct {
	token      string
	httpClient *http.Client
}

// NewGitHubClient creates a GitHubClient that calls the GitHub REST API.
func NewGitHubClient(token string) GitHubClient {
	return &realGitHubClient{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *realGitHubClient) doRequest(ctx context.Context, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request for %s: %w", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s: status %d", url, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response from %s: %w", url, err)
	}
	return nil
}

type ghRepo struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Archived      bool   `json:"archived"`
}

func (c *realGitHubClient) ListRepos(ctx context.Context, org string) ([]Repo, error) {
	var allRepos []Repo
	page := 1

	for {
		url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=%d", org, page)
		var ghRepos []ghRepo
		if err := c.doRequest(ctx, url, &ghRepos); err != nil {
			return nil, fmt.Errorf("list repos for org %s: %w", org, err)
		}

		if len(ghRepos) == 0 {
			break
		}

		for _, r := range ghRepos {
			allRepos = append(allRepos, Repo{
				Name:          r.Name,
				Description:   r.Description,
				DefaultBranch: r.DefaultBranch,
				Archived:      r.Archived,
			})
		}

		if len(ghRepos) < 100 {
			break
		}
		page++
	}

	return allRepos, nil
}

func (c *realGitHubClient) ListFiles(ctx context.Context, owner, repo string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents", owner, repo)

	var entries []struct {
		Name string `json:"name"`
	}
	if err := c.doRequest(ctx, url, &entries); err != nil {
		return nil, fmt.Errorf("list files for %s/%s: %w", owner, repo, err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

func (c *realGitHubClient) CreateIssue(ctx context.Context, owner, repo, title, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", owner, repo)

	payload := struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Title: title, Body: body}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal issue payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request for %s: %w", url, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create issue in %s/%s: status %d", owner, repo, resp.StatusCode)
	}

	return nil
}
