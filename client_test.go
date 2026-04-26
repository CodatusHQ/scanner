package scanner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestServer creates an httptest server with the given mux and returns
// a realGitHubClient pointed at it. The server is closed when the test ends.
func setupTestServer(t *testing.T, mux *http.ServeMux) GitHubClient {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return newGitHubClient("test-token", server.URL)
}

// --- ListReposByAccount ---

func TestListReposByAccount_SinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"name": "repo-a", "description": "First", "default_branch": "main", "archived": false},
			{"name": "repo-b", "description": "", "default_branch": "master", "archived": true}
		]`)
	})
	client := setupTestServer(t, mux)

	repos, err := client.ListReposByAccount(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "repo-a" || repos[0].Description != "First" || repos[0].DefaultBranch != "main" || repos[0].Archived {
		t.Errorf("repo-a fields mismatch: %+v", repos[0])
	}
	if repos[1].Name != "repo-b" || repos[1].DefaultBranch != "master" || !repos[1].Archived {
		t.Errorf("repo-b fields mismatch: %+v", repos[1])
	}
}

func TestListReposByAccount_Pagination(t *testing.T) {
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("Link", `<`+serverURL+`/orgs/test-org/repos?page=2>; rel="next"`)
			fmt.Fprint(w, `[{"name": "page1-repo"}]`)
		} else {
			fmt.Fprint(w, `[{"name": "page2-repo"}]`)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	serverURL = server.URL
	client := newGitHubClient("test-token", server.URL)

	repos, err := client.ListReposByAccount(context.Background(), "test-org")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos across pages, got %d", len(repos))
	}
	if repos[0].Name != "page1-repo" || repos[1].Name != "page2-repo" {
		t.Errorf("pagination mismatch: %+v, %+v", repos[0], repos[1])
	}
}

func TestListReposByAccount_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "internal error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.ListReposByAccount(context.Background(), "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Non-404 errors from the org endpoint (e.g. 500 / 503) are returned directly;
// they must not trigger a fallback to the user endpoint, which could mask a
// real outage by succeeding against a different account surface.
func TestListReposByAccount_500PassesThrough(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "server error"}`)
	})
	userEndpointHit := false
	mux.HandleFunc("/users/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		userEndpointHit = true
		fmt.Fprint(w, `[]`)
	})
	client := setupTestServer(t, mux)

	_, err := client.ListReposByAccount(context.Background(), "test-org")
	if err == nil {
		t.Fatal("expected error from 500, got nil")
	}
	if userEndpointHit {
		t.Error("user endpoint must not be called on non-404 errors")
	}
}

// When the org endpoint returns 404 (because the name belongs to a personal
// user account, not an org), ListReposByAccount falls back to the user endpoint.
func TestListReposByAccount_FallsBackToUserOn404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-user/repos", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message": "Not Found"}`)
	})
	mux.HandleFunc("/users/test-user/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"name": "personal-repo", "description": "my project", "default_branch": "main", "archived": false}
		]`)
	})
	client := setupTestServer(t, mux)

	repos, err := client.ListReposByAccount(context.Background(), "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "personal-repo" {
		t.Fatalf("expected personal-repo, got %+v", repos)
	}
}

// --- ListReposByInstallation ---

func TestListReposByInstallation_SinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"total_count": 2,
			"repositories": [
				{"name": "granted-a", "description": "A", "default_branch": "main", "archived": false},
				{"name": "granted-b", "description": "B", "default_branch": "develop", "archived": true}
			]
		}`)
	})
	client := setupTestServer(t, mux)

	repos, err := client.ListReposByInstallation(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "granted-a" || repos[0].DefaultBranch != "main" || repos[0].Archived {
		t.Errorf("granted-a fields mismatch: %+v", repos[0])
	}
	if repos[1].Name != "granted-b" || repos[1].DefaultBranch != "develop" || !repos[1].Archived {
		t.Errorf("granted-b fields mismatch: %+v", repos[1])
	}
}

func TestListReposByInstallation_Pagination(t *testing.T) {
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("Link", `<`+serverURL+`/installation/repositories?page=2>; rel="next"`)
			fmt.Fprint(w, `{"total_count": 2, "repositories": [{"name": "page1-repo"}]}`)
		} else {
			fmt.Fprint(w, `{"total_count": 2, "repositories": [{"name": "page2-repo"}]}`)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	serverURL = server.URL
	client := newGitHubClient("test-token", server.URL)

	repos, err := client.ListReposByInstallation(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "page1-repo" || repos[1].Name != "page2-repo" {
		t.Errorf("pagination mismatch: %+v", repos)
	}
}

func TestListReposByInstallation_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "internal error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.ListReposByInstallation(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListReposByInstallation_RateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/installation/repositories", rateLimitHandler())
	client := setupTestServer(t, mux)

	_, err := client.ListReposByInstallation(context.Background())
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rate limit error type, got: %v", err)
	}
}

// --- GetTree ---

func TestGetTree_MapsFields(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") != "1" {
			t.Error("expected recursive=1 query parameter")
		}
		fmt.Fprint(w, `{
			"sha": "abc123",
			"tree": [
				{"path": "README.md", "type": "blob", "size": 4096},
				{"path": ".github/workflows", "type": "tree", "size": 0},
				{"path": ".github/workflows/ci.yml", "type": "blob", "size": 512}
			]
		}`)
	})
	client := setupTestServer(t, mux)

	files, err := client.GetTree(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(files))
	}
	if files[0].Path != "README.md" || files[0].Type != "blob" || files[0].Size != 4096 {
		t.Errorf("file 0 mismatch: %+v", files[0])
	}
	if files[1].Path != ".github/workflows" || files[1].Type != "tree" {
		t.Errorf("file 1 mismatch: %+v", files[1])
	}
}

func TestGetTree_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "internal error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.GetTree(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetBranchProtection ---

func TestGetBranchProtection_WithReviewersAndChecks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"required_pull_request_reviews": {
				"required_approving_review_count": 2
			},
			"required_status_checks": {
				"strict": true,
				"contexts": ["ci/test", "ci/lint"]
			}
		}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp == nil {
		t.Fatal("expected branch protection, got nil")
	}
	if bp.RequiredReviewers != 2 {
		t.Errorf("expected 2 reviewers, got %d", bp.RequiredReviewers)
	}
	if len(bp.RequiredStatusChecks) != 2 || bp.RequiredStatusChecks[0] != "ci/test" || bp.RequiredStatusChecks[1] != "ci/lint" {
		t.Errorf("status checks mismatch: %v", bp.RequiredStatusChecks)
	}
}

func TestGetBranchProtection_NoReviewers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"required_status_checks": {
				"contexts": ["ci/test"]
			}
		}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp == nil {
		t.Fatal("expected branch protection, got nil")
	}
	if bp.RequiredReviewers != 0 {
		t.Errorf("expected 0 reviewers, got %d", bp.RequiredReviewers)
	}
	if len(bp.RequiredStatusChecks) != 1 {
		t.Errorf("expected 1 status check, got %d", len(bp.RequiredStatusChecks))
	}
}

func TestGetBranchProtection_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message": "Branch not protected"}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp != nil {
		t.Errorf("expected nil, got %+v", bp)
	}
}

func TestGetBranchProtection_403(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message": "Upgrade to GitHub Pro"}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp != nil {
		t.Errorf("expected nil, got %+v", bp)
	}
}

func TestGetBranchProtection_500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "server error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetRulesets ---

func TestGetRulesets_WithPullRequestAndStatusChecks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{
				"type": "pull_request",
				"parameters": {
					"required_approving_review_count": 1,
					"dismiss_stale_reviews_on_push": false,
					"require_code_owner_review": false,
					"require_last_push_approval": false,
					"required_review_thread_resolution": false,
					"allowed_merge_methods": ["squash"]
				}
			},
			{
				"type": "required_status_checks",
				"parameters": {
					"required_status_checks": [
						{"context": "test"},
						{"context": "lint"}
					],
					"strict_required_status_checks_policy": false
				}
			}
		]`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp == nil {
		t.Fatal("expected branch protection, got nil")
	}
	if bp.RequiredReviewers != 1 {
		t.Errorf("expected 1 reviewer, got %d", bp.RequiredReviewers)
	}
	if len(bp.RequiredStatusChecks) != 2 || bp.RequiredStatusChecks[0] != "test" || bp.RequiredStatusChecks[1] != "lint" {
		t.Errorf("status checks mismatch: %v", bp.RequiredStatusChecks)
	}
}

func TestGetRulesets_MultiplePullRequestRules(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{
				"type": "pull_request",
				"ruleset_source": "repo-level",
				"parameters": {
					"required_approving_review_count": 1,
					"allowed_merge_methods": ["squash"]
				}
			},
			{
				"type": "pull_request",
				"ruleset_source": "org-level",
				"parameters": {
					"required_approving_review_count": 3,
					"allowed_merge_methods": ["squash"]
				}
			}
		]`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp == nil {
		t.Fatal("expected branch protection, got nil")
	}
	if bp.RequiredReviewers != 3 {
		t.Errorf("expected 3 reviewers (highest), got %d", bp.RequiredReviewers)
	}
}

func TestGetRulesets_NoMatchingRules(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"type": "creation"},
			{"type": "deletion"}
		]`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp != nil {
		t.Errorf("expected nil (no relevant rules), got %+v", bp)
	}
}

func TestGetRulesets_404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message": "Not Found"}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp != nil {
		t.Errorf("expected nil, got %+v", bp)
	}
}

func TestGetRulesets_403(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message": "Upgrade to GitHub Pro"}`)
	})
	client := setupTestServer(t, mux)

	bp, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp != nil {
		t.Errorf("expected nil, got %+v", bp)
	}
}

func TestGetRulesets_500(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "server error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- GetTree: empty repo and truncated ---

func TestGetTree_EmptyRepo_409(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"message": "Git Repository is empty."}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.GetTree(context.Background(), "org", "repo", "main")
	if !errors.Is(err, ErrEmptyRepo) {
		t.Fatalf("expected ErrEmptyRepo, got %v", err)
	}
}

func TestGetTree_Truncated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"sha": "abc123",
			"truncated": true,
			"tree": [
				{"path": "README.md", "type": "blob", "size": 100}
			]
		}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.GetTree(context.Background(), "org", "repo", "main")
	if !errors.Is(err, ErrTruncatedTree) {
		t.Fatalf("expected ErrTruncatedTree, got %v", err)
	}
}

// --- Rate limit tests ---

// rateLimitHandler returns a handler that simulates a GitHub rate limit response.
func rateLimitHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Reset", "1924905600")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message": "API rate limit exceeded for user.", "documentation_url": "https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting"}`)
	}
}

func TestGetBranchProtection_RateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/branches/main/protection", rateLimitHandler())
	client := setupTestServer(t, mux)

	_, err := client.GetBranchProtection(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rate limit error type, got: %v", err)
	}
}

func TestGetRulesets_RateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/rules/branches/main", rateLimitHandler())
	client := setupTestServer(t, mux)

	_, err := client.GetRulesets(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rate limit error type, got: %v", err)
	}
}

func TestListReposByAccount_RateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", rateLimitHandler())
	client := setupTestServer(t, mux)

	_, err := client.ListReposByAccount(context.Background(), "test-org")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rate limit error type, got: %v", err)
	}
}

func TestGetTree_RateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/git/trees/main", rateLimitHandler())
	client := setupTestServer(t, mux)

	_, err := client.GetTree(context.Background(), "org", "repo", "main")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("expected rate limit error type, got: %v", err)
	}
}

