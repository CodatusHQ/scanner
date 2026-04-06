# AGENTS.MD - Codatus Scanner

## Project in 10 lines
Codatus is a scanner that checks GitHub repos against configurable engineering standards.
It connects to a GitHub org via the GitHub API, scans every repo against a set of rules
(file existence, GitHub API-based checks), and produces a Markdown compliance report posted
as a GitHub Issue.

No database. No web UI. No persistence. Scan → report.

The GitHub App that installs and triggers this scanner lives in a separate repository.
Domain specification (rules, report structure) lives in `README.md`.

## Operating protocol (must follow)

This protocol assumes you are running inside the Claude Code VS Code extension, interacting with the user directly in chat. All communication (design briefs, questions, approvals) happens in the chat - not via GitHub issue comments.

### Phase 1 - Clarify and design (no code)

For trivial changes (typo, config tweak, single-line fix), skip the design brief and go straight to Phase 2.

**Step 1: Clarify inputs.**
If the task description is missing information (expected behavior, constraints, edge cases), ask the user brief questions. Wait for answers before proceeding. If inputs are clear, skip to Step 2.

**Step 2: Present a design brief.**
Before writing any code, present a design brief in chat. It must be concrete enough that the user can predict what the code will look like.

Structure (use these headers):

1) Problem statement
- What we are changing and why (1-3 paragraphs).

2) Current system touchpoints
- Identify the specific existing pieces involved (with file paths).
- For each: what it currently does and why it matters for this change.

3) Precedents and patterns to follow
- Search the repo for existing, similar functionality/patterns (naming, folder placement, error handling, logging).
- If found:
  - List the exact file paths and the specific parts to emulate.
  - State what you will reuse vs what you will add.
  - Ensure the proposed solution aligns with these precedents unless there is a clear reason to diverge.
- If not found:
  - Say "No precedents found" and propose a minimal, consistent pattern.

4) Proposed solution overview
- Describe the approach end-to-end as a flow:
  Inputs -> processing steps -> outputs.
- Call out major components/files involved and their responsibilities.

5) Detailed changes
For each change, include:
- Where: exact files to edit / new files to add
- What: functions/sections to add/modify (by name if applicable)
- How: algorithms/logic, important corner cases
- Error handling strategy
- What stays backward compatible, what breaks (if anything)

End the design brief with:
- "Reply OK to proceed to implementation. If you want changes, tell me what to adjust."
- Do not write code until the user replies with explicit "OK".

### Phase 2 - Implement and test

After design approval:
1. Pull latest `main`, then create a feature branch. Branch name must match the pattern `type/short-description` using the same types as PR titles (e.g., `feat/add-ci-workflow-rule`, `fix/pass-rate-calculation`).
2. Implement the full approved design.
3. Include tests (see "Testing approach" below). Tests are mandatory.
4. Run `go test ./...` and ensure all tests pass. If any test fails, fix the code or the test before proceeding.
5. Do not commit yet - leave changes uncommitted so the user can review the diff in their IDE.

### Phase 3 - Review, commit, and open PR

After implementation is complete and tests pass:
1. Present a summary of all changes (files added/modified, what each does).
2. Wait for the user to review the uncommitted changes in their IDE. If the user requests changes, apply them and re-run tests before proceeding.
3. Once the user approves the changes, commit to the feature branch with a message following the PR title convention.
4. Explicitly ask: "Ready to push and create the PR?"
5. Do not push or open a PR until the user confirms. If the user declines, ask what needs to change.
6. Push the branch and open a pull request targeting `main`.

**PR requirements:**
- **Title** must follow conventional commits: `type: description` (e.g., `feat: add CI workflow rule`, `fix: correct pass rate calculation`, `refactor: extract report formatting`). Allowed types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`.
- **Description** must include a summary of the design brief (problem, approach, files changed) so the PR is self-contained without reading the full chat history.
- **Link** the PR to the originating GitHub issue, if one exists.
- **Do not** add `Co-Authored-By` trailers to commits.
- **Do not** add "Generated with Claude Code" or similar AI attribution footers to PR descriptions.

### Handling PR review comments

When the user requests changes on the PR (either in chat or via GitHub review comments):
- Push follow-up commits to the same branch addressing each comment.
- Do not force-push or squash on the branch - the user needs to see what changed since their review.
- If a review comment requires a design change (not just a code tweak), explain the updated approach in chat before implementing it.
- After pushing fixes, run `go test ./...` again.

---

## Language and stack
- **Go** (latest stable).
- No database. No ORM. No web framework.
- Standard library preferred. Third-party dependencies only when clearly justified.

---

## Repository layout

The scanner is an importable Go package (`package scanner`) at the module root. The CLI is a thin wrapper in `cmd/scanner/`.

```
.
├── .claude/                # Claude Code settings
├── cmd/
│   └── scanner/
│       └── main.go         # thin CLI wrapper (reads env, calls scanner.Run)
├── AGENTS.md
├── README.md
├── go.mod
├── go.sum
├── client.go               # GitHubClient interface + real implementation
├── client_mock_test.go     # mock GitHubClient (test-only)
├── scanner.go              # Config, Run(), Scan()
├── rules.go                # rule definitions + evaluation
├── report.go               # Markdown report generation (future)
├── scanner_test.go         # tests
├── rules_test.go
└── report_test.go
```

---

## Go style preferences

### General
- Never use em dashes (-). Use simple hyphens (-) everywhere - in code, comments, docs, and reports.
- Concise, explicit code. No clever abstractions.
- Flat control flow - early returns, avoid deep nesting.
- Errors are values. Wrap with context: `fmt.Errorf("scan repo %s: %w", name, err)`.
- No `panic` except in truly unrecoverable startup failures.
- Log with `log.Printf` / `log.Fatalf` (standard library). No structured logging libraries yet.

### Functions
- Small functions with clear names. If a function needs a comment explaining what it does, rename it.
- Accept interfaces, return structs.
- No global mutable state. Pass dependencies explicitly via function parameters.

### Naming
- Follow Go conventions: `MixedCase` for exported, `mixedCase` for unexported.
- Interfaces named by what they do: `RepoLister`, `FileChecker`, not `IClient` or `ClientInterface`.
- The main GitHub interface is `GitHubClient` (exception to the verb rule - it's the central abstraction).

---

## Testing approach

### Interface-based GitHub client
The scanner must interact with GitHub exclusively through a `GitHubClient` interface. Example shape:

```go
type GitHubClient interface {
    ListRepos(ctx context.Context, org string) ([]Repo, error)
    ListFiles(ctx context.Context, owner, repo string) ([]string, error)
    GetFileContent(ctx context.Context, owner, repo, path string) ([]byte, error)
    GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error)
    // ... expand as needed
}
```

The real implementation calls GitHub's REST API. Tests use a mock implementation (`client_mock.go`) that returns canned data representing different repo states.

### Test structure
- Tests exercise the scanner end-to-end through the mock client: set up a mock with a known repo state → run scan → assert the report/results.
- Use Go's standard `testing` package. No test frameworks.
- Test files live next to the code they test (`scanner_test.go` next to `scanner.go`).

### Test scenarios to cover
At minimum, every rule must have:
- A passing case (repo satisfies the rule)
- A failing case (repo violates the rule)
- An edge case where applicable (e.g., README exists but is under 2KB)

Report generation tests must verify the Markdown output matches expected structure.

---

## Dependency and validation rules

### Avoid global variables
- Do not rely on mutable module-level/global state.
- Prefer passing required values explicitly via function parameters or well-defined config structs.
- If you must read from the environment, do it once in `main()`, then pass values down.
- Keep functions "pure-ish": given inputs → deterministic outputs, minimal side effects.

### Make dependencies obvious
- Each function should declare what it needs (args) at the top.
- Prefer explicit parameters over reaching into shared state.
- Prefer returning values over writing to implicit global files/dirs unless explicitly part of the interface.

### Do not duplicate verification
- If something (inputs, invariants, auth, file existence) is verified upstream, do not re-verify downstream.
- Instead, document the precondition at the boundary (function docstring/comment).
- Only re-check downstream if a boundary is crossed (e.g., untrusted external input, network call).

### Follow precedents
- Before introducing a new pattern, search for existing similar implementations in the repo.
- Reuse existing conventions for:
  - CLI flags and help text
  - env var naming and validation
  - error formatting and exit codes
  - logging structure and verbosity
  - file naming, folder placement
- Cite the precedent files in the design brief (Phase 1, section 3) and describe what you are copying/adapting.

---

## Do-s
- Prefer concise, explicit code over clever abstractions.
- Validate required inputs up front in `main()` - fail fast with actionable messages.
- Use `context.Context` for GitHub API calls - respect cancellation and timeouts.
- Keep the `GitHubClient` interface narrow. Only add methods when a rule actually needs them.
- Handle GitHub API rate limits gracefully - log remaining quota, fail clearly when exhausted.

## Don't-s
- Don't add abstractions until they're needed by real code (no speculative interfaces or generic helpers).
- Don't write huge functions - if a function exceeds ~40 lines, it probably does too much.
- Don't log secrets (tokens, private repo contents).
- Don't ignore errors. Every `err` must be checked.
- Don't reach for external packages when the standard library suffices.
