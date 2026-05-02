# Codatus

Codatus scans every repository in a GitHub organization or user account against a set of engineering standards and produces a Markdown scorecard.

It answers one question: **how does each repo in your org measure up against the standards you care about?**

This repository is a Go library and a CLI. Posting the scorecard (e.g., as a GitHub Issue) is the caller's responsibility - the scanner returns structured results and Markdown, nothing more.

---

## How it works

1. Codatus receives a GitHub account to scan (organization or user).
2. It lists the repositories accessible to the token, then filters out **archived** and **forked** repos. Both exclusions are reported in the scorecard header so the reader can see the full breakdown (`Total repos`, `Forks excluded`, `Archived excluded`, `Repos scanned`).
3. For each remaining repo, it runs 11 rule checks (see below).
4. It produces a single Markdown scorecard summarizing pass/fail per repo per rule, plus a structured `ScanResult` value the caller can post-process (e.g. the bulk-scan binary serializes per-rule aggregates to JSON).

The CLI prints the Markdown to stdout. Callers using the library get a `ScanResult` (org name, scan timestamp, exclusion counts, per-repo results, skipped repos) and can generate the Markdown via `GenerateReport(scanResult)`.

---

## Rules

Each rule produces a **pass** or **fail** result per repository. Rules fall into two categories:

- **Scored rules** drive the org-level score. The score is the arithmetic mean of pass rates across the 5 scored rules. Per-repo classification (Strong / Moderate / Weak) is also based on what fraction of scored rules a repo passes.
- **Additional checks** are informational only. They appear in the report as coverage numbers but do not affect the score. They surface "nice to have" hygiene that's worth seeing but isn't load-bearing for whether a repo's standards are in good shape.

### Scored rules (drive the org-level score)

#### 1. Has branch protection

**Check:** the default branch has branch protection rules enabled (via the GitHub API's branch protection endpoint).

**Pass:** branch protection is enabled on the default branch.
**Fail:** branch protection is not enabled, or the API returns 404 (no protection configured).

#### 2. Has required reviewers

**Check:** the default branch's branch protection rules require at least one approving review before merging (via the GitHub API - `required_pull_request_reviews.required_approving_review_count >= 1`).

**Pass:** required reviewers is set to 1 or more.
**Fail:** required reviewers is not configured, or set to 0, or branch protection is not enabled.

#### 3. Requires status checks before merging

**Check:** the default branch's branch protection rules require at least one status check to pass before merging (via the GitHub API - `required_status_checks` is configured with one or more contexts).

**Pass:** at least one required status check is configured.
**Fail:** required status checks are not configured, or the list of required contexts is empty, or branch protection is not enabled.

#### 4. Has CODEOWNERS

**Check:** a `CODEOWNERS` file exists in one of the three standard locations: root (`/CODEOWNERS`), `docs/CODEOWNERS`, or `.github/CODEOWNERS`.

**Pass:** file found in any of the three locations.
**Fail:** file not found in any location.

#### 5. Has CI workflow

**Check:** at least one file exists under `.github/workflows/` with a `.yml` or `.yaml` extension.

**Pass:** one or more workflow files found.
**Fail:** `.github/workflows/` is missing or empty.

### Additional checks (informational only)

#### 6. Has README

**Check:** a `README.md` or `README` file exists in the repo root.

**Pass:** file found.
**Fail:** file not found.

There is no size threshold - any README counts. The previous "substantial" variant required >2 KB, which discriminated poorly (too low to weed out stubs, too high to reward minimal but useful READMEs).

#### 7. Has LICENSE

**Check:** a `LICENSE` or `LICENSE.md` file exists in the repo root.

**Pass:** file found.
**Fail:** file not found.

#### 8. Has repo description

**Check:** the GitHub repository description field is not blank.

**Pass:** description is set and non-empty.
**Fail:** description is blank or not set.

#### 9. Has activity

**Check:** the most recent push to any branch is within the last 12 months (via the GitHub API's `pushed_at` field on the repository).

**Pass:** the repository was pushed within the last 12 months.
**Fail:** the repository has not been pushed in the last 12 months, or has never been pushed. Archived repositories are filtered out before scanning, so they never reach this rule.

#### 10. Has SECURITY.md

**Check:** a `SECURITY.md` file exists in the repo root or `.github/SECURITY.md`.

**Pass:** file found in either location.
**Fail:** file not found.

### Score and bucketing

The org-level score is the arithmetic mean of pass rates across the 5 scored rules:

```
score = (pass_rate_branch_protection
       + pass_rate_required_reviewers
       + pass_rate_status_checks
       + pass_rate_codeowners
       + pass_rate_ci_workflow) / 5
```

Each repo also gets a percentage based on how many of the 5 scored rules it passes (0%, 20%, 40%, 60%, 80%, 100%) and is bucketed:

- **Strong (≥80%):** 4 or 5 scored rules passing
- **Moderate (40-79%):** 2 or 3 scored rules passing
- **Weak (≤39%):** 0 or 1 scored rules passing

Additional checks do not affect either the org score or the per-repo bucket.

---

## Scorecard format

The scorecard is a single Markdown document. Structure:

```
# Codatus - Engineering Standards Scorecard

**Org:** {org_name}
**Scanned:** {timestamp}
**Total repos:** {count}              <-- only if > 0
**Forks excluded:** {count}           <-- only if > 0
**Archived excluded:** {count}        <-- only if > 0
**Repos scanned:** {count}
**Skipped:** {count}                  <-- only if any repos were skipped

## Scored rules

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has branch protection | 11 | 42 | 20% |
| Has required reviewers | 4 | 49 | 7% |
| Requires status checks before merging | 4 | 49 | 7% |
| Has CODEOWNERS | 3 | 50 | 5% |
| Has CI workflow | 27 | 26 | 50% |

**Score: 18/100** (average pass rate across the scored rules above)

## Additional checks

| Check | Passing | Failing | Coverage |
|------|---------|---------|----------|
| Has README | 50 | 3 | 94% |
| Has LICENSE | 38 | 15 | 71% |
| Has repo description | 46 | 7 | 86% |
| Has activity | 43 | 10 | 81% |
| Has SECURITY.md | 3 | 50 | 5% |

## Repository details

### Strong (≥80%)

<details>
<summary><a href="https://github.com/{org}/repo-a">repo-a</a> - 100% (5/5 scored rules passing)</summary>

</details>

### Moderate (40-79%)

<details>
<summary><a href="https://github.com/{org}/repo-b">repo-b</a> - 60% (3/5 scored rules passing)</summary>

**Failing scored rules:**
- Has CODEOWNERS
- Requires status checks before merging

**Additional check failures:**
- Has SECURITY.md

</details>

### Weak (≤39%)

<details>
<summary><a href="https://github.com/{org}/repo-c">repo-c</a> - 0% (0/5 scored rules passing)</summary>

**Failing scored rules:**
- Has branch protection
- Has required reviewers
- Requires status checks before merging
- Has CODEOWNERS
- Has CI workflow

</details>

## Rule reference

<details>
<summary>What each rule checks and how to fix it</summary>

### Scored rules

#### Has branch protection
- **What it checks:** ...
- **How to fix:** ...

---

#### Has required reviewers
...

### Additional checks

#### Has README
...

</details>

## ⚠️ Skipped ({n} repos)          <-- only if any repos were skipped

- [empty-repo](https://github.com/{org}/empty-repo) - repository is empty
- [huge-repo](https://github.com/{org}/huge-repo) - file tree too large (truncated by GitHub API)
```

Tables render in fixed importance order (not sorted by pass rate). The Rule reference section is collapsed by default and lists, for every rule actually present in the scan results, the "what it checks" / "how to fix" text - split into Scored rules and Additional checks subsections. Sections (and entire buckets) are omitted when empty. Skipped repos are those that could not be scanned (empty repos, truncated file trees, API errors) - they are excluded from the score and bucket counts, and appear at the very end.

When `repos_scanned` is 0, the Score line reads `**Score: N/A** (no repos available to score)`.

### Canonical sample fixture

`samples.Fixture()` returns a deterministic `ScanResult` for the fictional `acme-corp` org. It's the single source of truth for the sample scorecard shown on the landing page and used as dev-seed data in the app, replacing what used to be hand-typed Markdown in each downstream repo.

Go consumers render it in process:

```go
md := scanner.GenerateReport(samples.Fixture())
```

Non-Go consumers use the generator binary, which writes Markdown to stdout (or to `--out`):

```
go run github.com/CodatusHQ/scanner/cmd/generate-sample > sample-scorecard.md
```

No rendered `.md` is committed here - downstream copies are refreshed on demand by re-running the generator.

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

The `codatus` binary reads `CODATUS_ORG` and `CODATUS_TOKEN` from the environment, wraps them in `PATAuth`, runs a scan, and prints the Markdown scorecard to stdout. Log output (scan summary, errors) goes to stderr so stdout stays clean for piping.

Despite the name, `CODATUS_ORG` accepts either an organization slug or a user login - the library dispatches automatically.

```sh
# Organization
CODATUS_ORG=myorg CODATUS_TOKEN=ghp_... codatus > scorecard.md

# User account
CODATUS_ORG=my-username CODATUS_TOKEN=ghp_... codatus > scorecard.md
```

### Bulk scan (many orgs at once)

The `bulk-scan` binary reads a list of orgs/users from a file (one slug per line, blank lines skipped, no comment handling) and scans them sequentially. For each org it writes a `scorecard.md` and a `stats.json` into a per-org subfolder, so partial runs preserve completed work even if a later scan aborts.

```sh
# orgs.txt
acme-corp
wayne-enterprises
stark-industries

# Run
bulk-scan --orgs orgs.txt --out ./scans --token ghp_...
# or with the token in env:
CODATUS_TOKEN=ghp_... bulk-scan --orgs orgs.txt --out ./scans
```

Output layout:

```
scans/
├── acme-corp/
│   ├── scorecard.md     # same Markdown the single-org CLI produces
│   └── stats.json       # structured aggregates: per-rule pass rates, totals, exclusion counts
├── wayne-enterprises/
│   ├── scorecard.md
│   └── stats.json
└── ...
```

Progress prints to stderr per org (`[2/3] wayne-enterprises ... ok (42 scanned, 18/42 compliant = 42%)`); a final summary lists succeeded / failed / not-attempted counts.

**Failure handling:**
- Per-org errors (`404`, `403`, "user not found", etc.) - logged, run continues to the next org.
- Global errors (`429` rate limit, `401` auth) - run aborts immediately. Files for orgs that already completed remain on disk; the un-scanned tail is reported in the summary as "not attempted."

Exit code is non-zero if any org failed or was not attempted.

---

## What Codatus is not

- **Not a velocity/DORA metrics tool.** It does not measure cycle time, deployment frequency, or review speed. That's a different product category (Swarmia, LinearB, CodePulse).
- **Not a security scanner.** It checks whether `SECURITY.md` exists and whether branch protection is on, but it does not scan code for vulnerabilities (use Snyk, Dependabot, or OpenSSF Scorecard).
- **Not a developer portal.** There is no service catalog, no scaffolding, no self-service actions (Backstage, Cortex, OpsLevel cover that). Just standards.
