# Codatus

Codatus scans every repository in a GitHub organization or user account against a set of repo standards and produces a Markdown scorecard.

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

**Pass:** branch protection is configured on the default branch (via rulesets, classic per-repo protection, or the public branch endpoint's `protected` flag).
**Fail:** none of the three signals indicate protection.

The scanner consults three GitHub APIs in priority order: rulesets (publicly readable), the admin-only classic-protection endpoint (returns 404 to non-admins), and the public branch endpoint (which exposes `protected: true/false` to anyone with read access). The third fallback exists so non-admin scans can still detect classic protection without admin privileges - they just can't read all the *details* of that protection.

#### 2. Has required reviewers

**Check:** the default branch's protection rules require at least one approving review before merging.

**Admin-only.** This rule reads `required_pull_request_reviews.required_approving_review_count`, which is admin-only on classic per-repo protection. Public scans (without `--admin`) **skip** this rule entirely - it doesn't appear in the JSON output, the Markdown table, or the score calculation. When you scan with admin access (e.g. via the Codatus GitHub App, or your own org with a PAT belonging to an admin), pass `WithAdmin(true)` (library) or `--admin` (CLI) to include it.

**Pass:** required reviewers is set to 1 or more.
**Fail:** required reviewers is not configured, or set to 0, or branch protection is not enabled.

#### 3. Has required checks

**Check:** the default branch's protection requires at least one programmatic check to pass before a PR can be merged.

**Pass:** at least one merge-blocking check requirement is configured. Detection unions five rulesets rule types (`required_status_checks`, `workflows`, `code_scanning`, `code_quality`, `required_deployments`), classic branch protection's `required_status_checks.contexts`, and the public branch endpoint's `protection.required_status_checks.contexts`.
**Fail:** none of those signals reveal a check-passing requirement.

The public branch endpoint exposes the `contexts` array even to non-admin readers, so this rule is correctly answered for both admin and public scans. New ruleset rule types that act as merge gates can be added by extending the `switch` in `GetRulesets`.

#### 4. Has CODEOWNERS

**Check:** a `CODEOWNERS` file exists in one of the three standard locations: root (`/CODEOWNERS`), `docs/CODEOWNERS`, or `.github/CODEOWNERS`.

**Pass:** file found in any of the three locations.
**Fail:** file not found in any location.

#### 5. Has CI workflow

**Check:** the repo has a CI workflow configured for any of the supported providers:
- GitHub Actions:  `.github/workflows/*.yml` or `*.yaml`
- CircleCI:        `.circleci/config.yml`
- GitLab CI:       `.gitlab-ci.yml`
- Travis CI:       `.travis.yml`
- Buildkite:       any file under `.buildkite/`
- Azure Pipelines: `azure-pipelines.yml`
- Jenkins:         `Jenkinsfile`

**Pass:** at least one of the above paths is present in the repo.
**Fail:** none of the recognized CI configurations exist. (Repos with server-side-only CI integrations - e.g. CircleCI without a checked-in config - are still missed; the rule is best-effort based on what's in the tree.)

### Additional checks (informational only)

#### 6. Has README

**Check:** a README file exists at the repository root, matched case-insensitively with any extension or none. So `README.md`, `Readme.rst`, `README.txt`, `README.markdown`, `readme` all pass.

**Pass:** file found at root.
**Fail:** no root-level file whose name is `readme` or starts with `readme.` (case-insensitive). Subdirectory READMEs (e.g. `docs/README.md`) don't count.

There is no size threshold - any README counts. The previous "substantial" variant required >2 KB, which discriminated poorly.

#### 7. Has LICENSE

**Check:** GitHub auto-detected an open-source license for the repository (the `license.spdx_id` field on the listing payload is non-empty). GitHub uses the [Licensee](https://github.com/licensee/licensee) gem to detect license files at any conventional path or filename - `LICENSE`, `LICENSE.md`, `LICENSE.txt`, `COPYING`, `LICENCE`, etc.

**Pass:** GitHub returned a license SPDX id.
**Fail:** GitHub couldn't auto-detect a license (no recognized license file, or a custom-text license Licensee doesn't recognize).

#### 8. Has repo description

**Check:** the GitHub repository description field is not blank.

**Pass:** description is set and non-empty.
**Fail:** description is blank or not set.

#### 9. Has activity

**Check:** the most recent push to any branch is within the last 12 months (via the GitHub API's `pushed_at` field on the repository).

**Pass:** the repository was pushed within the last 12 months.
**Fail:** the repository has not been pushed in the last 12 months, or has never been pushed. Archived repositories are filtered out before scanning, so they never reach this rule.

#### 10. Has SECURITY.md

**Check:** a `SECURITY.md` file exists in any of the three locations GitHub recognizes for security policies: repo root, `.github/SECURITY.md`, or `docs/SECURITY.md`.

**Pass:** file found in any of those three locations.
**Fail:** file not found.

### Score and bucketing

The org-level score is the arithmetic mean of pass rates across the scored rules **that were actually evaluated**. For an admin scan that's all 5 scored rules; for a public scan it's 4 (HasRequiredReviewers is admin-only and silently skipped). The denominator adapts so the score isn't dragged toward zero by rules the scan couldn't see.

```
admin scan:   score = mean of 5 per-rule pass rates
public scan:  score = mean of 4 per-rule pass rates (no required-reviewers)
```

Each repo also gets a percentage based on the fraction of evaluated scored rules it passes (5-rule scans land at 0/20/40/60/80/100; 4-rule scans land at 0/25/50/75/100), and is bucketed:

- **Strong (≥80%)**
- **Moderate (30-79%)**
- **Weak (≤29%)**

Additional checks do not affect either the org score or the per-repo bucket.

---

## Scorecard format

The scorecard is a single Markdown document. Structure:

```
# Codatus - Repo Standards Scorecard

**Org:** {org_name}<br>
**Scanned:** {timestamp}<br>
**Repos:** {scanned} of {total} scanned ({forks} forks excluded, {archived} archived excluded, {skipped} skipped)

## Scored rules

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has branch protection | 11 | 42 | 20% |
| Has required reviewers | 4 | 49 | 7% |
| Has required checks | 4 | 49 | 7% |
| Has CODEOWNERS | 3 | 50 | 5% |
| Has CI workflow | 27 | 26 | 50% |

**Score: 18/100** (average pass rate across the scored rules above)

## Additional checks

| Rule | Passing | Failing | Pass rate |
|------|---------|---------|----------|
| Has README | 50 | 3 | 94% |
| Has LICENSE | 38 | 15 | 71% |
| Has repo description | 46 | 7 | 86% |
| Has activity | 43 | 10 | 81% |
| Has SECURITY.md | 3 | 50 | 5% |

## Rule reference

<details>
<summary>How each rule works and how to fix failures</summary>

### Scored rules

#### Has branch protection

Checks that the default branch has a protection rule in place. Detected via any of three GitHub APIs: ...

---

#### Has required reviewers

...

### Additional checks

#### Has README

...

</details>

## Repository details

### Strong (≥80%)

<details>
<summary><a href="https://github.com/{org}/repo-a">repo-a</a> - 100%</summary>

</details>

### Moderate (30-79%)

<details>
<summary><a href="https://github.com/{org}/repo-b">repo-b</a> - 60%</summary>

**Failing scored rules:**
- Has CODEOWNERS
- Has required checks

**Additional check failures:**
- Has SECURITY.md

</details>

### Weak (≤29%)

<details>
<summary><a href="https://github.com/{org}/repo-c">repo-c</a> - 0%</summary>

**Failing scored rules:**
- Has branch protection
- Has required reviewers
- Has required checks
- Has CODEOWNERS
- Has CI workflow

</details>

### Skipped ({n} repos)             <-- only if any repos were skipped

- [empty-repo](https://github.com/{org}/empty-repo) - repository is empty
- [huge-repo](https://github.com/{org}/huge-repo) - file tree too large (truncated by GitHub API)
```

Header line breaks use explicit `<br>` so spec-compliant Markdown renderers (CommonMark/marked.js/kramdown/GitHub) emit one line per item instead of folding consecutive single-newlines into one paragraph. The repo-stats parenthetical drops fields that are zero - with no exclusions and no skipped repos, it collapses to `**Repos:** {scanned} of {total} scanned`.

Tables render in fixed importance order (not sorted by pass rate). Both tables share the same column layout. The Rule reference section is collapsed by default; each rule has a single self-contained description that names what's checked, every detection path the rule walks (legacy and modern GitHub mechanisms), and how to fix it. Rule reference precedes Repository details so the rule definitions are in hand before the per-repo failure lists (which only mention rule names). Subsections (and entire buckets) are omitted when empty. Skipped repos are those that could not be scanned (empty repos, truncated file trees, API errors); they are excluded from the score and bucket counts, and render as the last subsection inside Repository details.

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
| `WithAdmin(b bool)` | Tell the scanner the auth has admin access on every target repo. Default `false`. When `true`, the scanner runs admin-only rules (currently: `Has required reviewers`). When `false`, those rules are silently skipped - they don't appear in any per-repo result, the JSON output, or the Markdown report. Pass `true` for installation-token scans (the Codatus GitHub App is granted admin), or for PAT scans where you're an admin of every target org. |

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
# admin-mode scan (you're an admin of every listed org, so include the
# `Has required reviewers` rule too):
CODATUS_TOKEN=ghp_... bulk-scan --orgs orgs.txt --out ./scans --admin
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
