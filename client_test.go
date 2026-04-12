package scanner

import (
	"context"
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
	return newTestGitHubClient(server.URL)
}

// --- ListRepos ---

func TestListRepos_SinglePage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
			{"name": "repo-a", "description": "First", "default_branch": "main", "archived": false},
			{"name": "repo-b", "description": "", "default_branch": "master", "archived": true}
		]`)
	})
	client := setupTestServer(t, mux)

	repos, err := client.ListRepos(context.Background(), "test-org")
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

func TestListRepos_Pagination(t *testing.T) {
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
	client := newTestGitHubClient(server.URL)

	repos, err := client.ListRepos(context.Background(), "test-org")
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

func TestListRepos_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/test-org/repos", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message": "internal error"}`)
	})
	client := setupTestServer(t, mux)

	_, err := client.ListRepos(context.Background(), "test-org")
	if err == nil {
		t.Fatal("expected error, got nil")
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

// --- CreateIssue ---

func TestCreateIssue_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"number": 42, "title": "Test Issue"}`)
	})
	client := setupTestServer(t, mux)

	err := client.CreateIssue(context.Background(), "org", "repo", "Test Issue", "Body text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateIssue_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/org/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"message": "Validation Failed"}`)
	})
	client := setupTestServer(t, mux)

	err := client.CreateIssue(context.Background(), "org", "repo", "Test", "Body")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
