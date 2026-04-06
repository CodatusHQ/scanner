# Codatus

Codatus scans every repository in a GitHub organization against a set of engineering standards and produces a Markdown compliance report posted as a GitHub Issue.

It answers one question: **does each repo in your org meet the baseline you care about?**

No dashboard. No database. No setup beyond installing the GitHub App. Scan, report, done.

---

## How it works

1. Codatus receives a GitHub org to scan.
2. It lists all non-archived repositories in the org.
3. For each repo, it runs 11 rule checks (see below).
4. It produces a single Markdown report summarizing pass/fail per repo per rule.
5. The report is posted as a GitHub Issue in a designated repository.

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

The report is a single Markdown document posted as a GitHub Issue. Structure:

```
# Codatus - Org Compliance Report

**Org:** {org_name}
**Scanned:** {timestamp}
**Repos scanned:** {count}

## Summary

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|-----------|
| Has CI workflow | 42 | 8 | 84% |
| Has CODEOWNERS | 30 | 20 | 60% |
| ... | ... | ... | ... |

## Results by repository

### repo-name-1

| Rule | Result |
|------|--------|
| Has repo description | ✅ |
| Has .gitignore | ✅ |
| Has substantial README | ❌ |
| ... | ... |

### repo-name-2

...
```

Repositories are sorted alphabetically. The summary table is sorted by pass rate ascending (worst compliance first).

---

## Scanner configuration

The scanner module accepts a `ScanConfig` struct with the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Org` | `string` | Yes | GitHub organization name to scan |
| `Token` | `string` | Yes | GitHub token (PAT or GitHub App installation token) |
| `ReportRepo` | `string` | Yes | Repository name where the compliance issue is created (org is taken from `Org`) |

The token must have the following permissions across the org:
- `repo` (read access to repo contents and branch protection)
- `admin:org` (read access to list org repos)

How these values are sourced (env vars, CLI flags, config file) is the responsibility of the caller, not the scanner module.

---

## What Codatus is not

- **Not a velocity/DORA metrics tool.** It does not measure cycle time, deployment frequency, or review speed. That's a different product category.
- **Not a security scanner.** It checks whether `SECURITY.md` exists and whether branch protection is on, but it does not scan code for vulnerabilities.
- **Not a developer portal.** There is no service catalog, no scaffolding, no self-service actions. Just standards compliance.
