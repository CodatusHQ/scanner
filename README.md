# Codatus

Codatus scans every repository in a GitHub organization or user account against a set of engineering standards and produces a Markdown compliance report.

It answers one question: **does each repo in your account meet the baseline you care about?**

This repository is a Go library and a CLI. Posting the report (e.g., as a GitHub Issue) is the caller's responsibility - the scanner returns structured results and Markdown, nothing more.

---

## How it works

1. Codatus receives a GitHub account to scan (organization or user).
2. It lists the non-archived repositories accessible to the token.
3. For each repo, it runs 11 rule checks (see below).
4. It produces a single Markdown report summarizing pass/fail per repo per rule.

The CLI prints the Markdown to stdout. Callers using the library get both the structured `[]RepoResult` and can generate the Markdown via `GenerateReport`.

---

## Rules

Each rule produces a **pass** or **fail** result per repository. There are no scores, weights, or severity levels - just pass/fail.

### Repo basics

#### 1. Has repo description

**Check:** the GitHub repository description field is not blank.

**Pass:** description is set and non-empty.
**Fail:** description is blank or not set.

#### 2. Has .gitignore

**Check:** a `.gitignore` file exists in the repo root.

**Pass:** file found.
**Fail:** file not found.

#### 3. Has substantial README

**Check:** a `README.md` file exists in the repo root and is larger than 2048 bytes.

**Pass:** `README.md` exists and is >2048 bytes.
**Fail:** `README.md` is missing, or exists but is ≤2048 bytes.

#### 4. Has LICENSE

**Check:** a `LICENSE` or `LICENSE.md` file exists in the repo root.

**Pass:** file found.
**Fail:** file not found.

#### 5. Has SECURITY.md

**Check:** a `SECURITY.md` file exists in the repo root or `.github/SECURITY.md`.

**Pass:** file found in either location.
**Fail:** file not found.

### Code quality & process

#### 6. Has CI workflow

**Check:** at least one file exists under `.github/workflows/` with a `.yml` or `.yaml` extension.

**Pass:** one or more workflow files found.
**Fail:** `.github/workflows/` is missing or empty.

#### 7. Has test directory

**Check:** a directory exists at the repo root level whose name indicates tests. Recognized names: `test`, `tests`, `__tests__`, `spec`, `specs`.

**Pass:** at least one matching directory found.
**Fail:** none found.

#### 8. Has CODEOWNERS

**Check:** a `CODEOWNERS` file exists in one of the three standard locations: root (`/CODEOWNERS`), `docs/CODEOWNERS`, or `.github/CODEOWNERS`.

**Pass:** file found in any of the three locations.
**Fail:** file not found in any location.

### Branch protection

#### 9. Has branch protection

**Check:** the default branch has branch protection rules enabled (via the GitHub API's branch protection endpoint).

**Pass:** branch protection is enabled on the default branch.
**Fail:** branch protection is not enabled, or the API returns 404 (no protection configured).

#### 10. Has required reviewers

**Check:** the default branch's branch protection rules require at least one approving review before merging (via the GitHub API - `required_pull_request_reviews.required_approving_review_count >= 1`).

**Pass:** required reviewers is set to 1 or more.
**Fail:** required reviewers is not configured, or set to 0, or branch protection is not enabled.

#### 11. Requires status checks before merging

**Check:** the default branch's branch protection rules require at least one status check to pass before merging (via the GitHub API - `required_status_checks` is configured with one or more contexts).

**Pass:** at least one required status check is configured.
**Fail:** required status checks are not configured, or the list of required contexts is empty, or branch protection is not enabled.

---

## Report format

The report is a single Markdown document. Structure:

```
# Codatus - Org Compliance Report

**Org:** {org_name}
**Scanned:** {timestamp}
**Repos scanned:** {count}
**Compliant:** {n}/{total} ({percent}%) *(a repo is compliant when it passes all rules below)*
**Skipped:** {count}              <-- only if any repos were skipped

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has branch protection | 1 | 3 | 25% |
| Has required reviewers | 1 | 3 | 25% |
| ... | ... | ... | ... |

<details>
<summary>Rule reference - what each rule checks and how to fix it</summary>

### Has repo description
**What it checks:** ...

**How to fix:** ...

### Has .gitignore
...

</details>

## ✅ Fully compliant ({n} repos)

<details>
<summary>All rules passing</summary>

[repo-a](https://github.com/{org}/repo-a)
[repo-b](https://github.com/{org}/repo-b)

</details>

## ❌ Non-compliant ({n} repos)

<details>
<summary><a href="https://github.com/{org}/repo-c">repo-c</a> - {n} failing</summary>

- Has branch protection
- Has required reviewers

</details>

## ⚠️ Skipped ({n} repos)          <-- only if any repos were skipped

- [empty-repo](https://github.com/{org}/empty-repo) - repository is empty
- [huge-repo](https://github.com/{org}/huge-repo) - file tree too large (truncated by GitHub API)
```

The summary table is sorted by pass rate ascending (worst compliance first). The Rule reference section is collapsed by default and lists, in `AllRules` order, the "what it checks" / "how to fix" text for every rule that appears in the scan results - this carries into the GitHub issue when the report is posted, so the issue is self-contained. Sections are omitted when empty (e.g., no "Fully compliant" section if all repos have failures). Skipped repos are those that could not be scanned (empty repos, truncated file trees, API errors) - they are excluded from the compliance count.

---

## Scanner configuration

`scanner.Scan(ctx, auth, opts...)` takes an `Auth` - a sealed interface implemented by two concrete types. Pick the one that matches your token.

### PATAuth - personal access token

For scanning with a user-generated token (classic or fine-grained PAT). Scanner calls `/orgs/{Name}/repos` and falls back to `/users/{Name}/repos` on 404, so it works for both org and user accounts.

```go
results, err := scanner.Scan(ctx, scanner.PATAuth{
    Token: "ghp_...",
    Name:  "my-org",        // or a user login like "octocat"
})
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Token` | `string` | Yes | Personal access token |
| `Name` | `string` | Yes | GitHub organization or user login to scan |

### InstallationAuth - GitHub App installation

For scanning as a GitHub App. Scanner calls `/installation/repositories`, which returns exactly the repos the installation was granted access to. Works identically for org and user installs, and respects "Selected repositories" mode (no leak of other public repos).

```go
results, err := scanner.Scan(ctx, scanner.InstallationAuth{
    Token: "ghs_...",       // short-lived installation access token
    Name:  "my-org",        // the account the app is installed on
})
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Token` | `string` | Yes | Installation access token (from `/app/installations/{id}/access_tokens`) |
| `Name` | `string` | Yes | Account the app is installed on; used for per-repo URL construction |

### Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url string)` | Override the GitHub API base URL. Defaults to the public GitHub API. Useful for testing against a mock server or targeting GitHub Enterprise. |

### Required token permissions

**Classic PAT:**
- `repo` — read repo contents and branch protection
- `read:org` — required when `Name` is an organization

**Fine-grained PAT:** scoped to the target account, with Repository permissions:
- Metadata: Read
- Contents: Read
- Administration: Read (for branch protection)

**Installation token:** permissions come from the GitHub App's configured repository permissions (not PAT scopes). At minimum the app needs Contents (read) and Metadata (read); Administration (read) is required for branch protection rules to resolve.

How these values are sourced (env vars, CLI flags, config file) is the responsibility of the caller, not the scanner module.

## CLI

The `codatus` binary reads `CODATUS_ORG` and `CODATUS_TOKEN` from the environment, wraps them in `PATAuth`, runs a scan, and prints the Markdown report to stdout. Log output (scan summary, errors) goes to stderr so stdout stays clean for piping.

Despite the name, `CODATUS_ORG` accepts either an organization slug or a user login - the library dispatches automatically.

```sh
# Organization
CODATUS_ORG=myorg CODATUS_TOKEN=ghp_... codatus > report.md

# User account
CODATUS_ORG=my-username CODATUS_TOKEN=ghp_... codatus > report.md
```

---

## What Codatus is not

- **Not a velocity/DORA metrics tool.** It does not measure cycle time, deployment frequency, or review speed. That's a different product category.
- **Not a security scanner.** It checks whether `SECURITY.md` exists and whether branch protection is on, but it does not scan code for vulnerabilities.
- **Not a developer portal.** There is no service catalog, no scaffolding, no self-service actions. Just standards compliance.
