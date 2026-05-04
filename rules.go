package scanner

import (
	"strings"
	"time"
)

// RuleCategory classifies a rule as either a *scored* rule (contributes to
// the org-level score) or an *additional* check (informational only).
type RuleCategory string

const (
	CategoryScored     RuleCategory = "scored"
	CategoryAdditional RuleCategory = "additional"
)

// Rule defines a named check that produces a pass/fail result for a repo.
// Description and HowToFix supply the per-rule text used by the Markdown
// scorecard's Rule reference section. Category determines whether the rule
// feeds into the org-level score or appears in the informational-only
// "Additional checks" section.
type Rule interface {
	Name() string
	Category() RuleCategory
	Check(repo Repo) bool
	Description() string
	HowToFix() string
}

// RuleResult holds the outcome of a single rule check for a single repo.
type RuleResult struct {
	RuleName string
	Passed   bool
}

// AllRules returns the ordered list of rules the scanner evaluates. The
// order is fixed and meaningful: scored rules first (by importance), then
// additional checks (by importance). Callers that want only one category
// can use ScoredRules or AdditionalRules.
func AllRules() []Rule {
	return []Rule{
		// Scored rules - drive the org-level score.
		HasBranchProtection{},
		HasRequiredReviewers{},
		HasRequiredStatusChecks{},
		HasCodeowners{},
		HasCIWorkflow{},
		// Additional checks - informational only.
		HasReadme{},
		HasLicense{},
		HasRepoDescription{},
		HasActivity{},
		HasSecurityMd{},
	}
}

// ScoredRules returns just the rules with CategoryScored, in AllRules order.
func ScoredRules() []Rule {
	return filterByCategory(CategoryScored)
}

// AdditionalRules returns just the rules with CategoryAdditional, in AllRules order.
func AdditionalRules() []Rule {
	return filterByCategory(CategoryAdditional)
}

func filterByCategory(c RuleCategory) []Rule {
	var out []Rule
	for _, r := range AllRules() {
		if r.Category() == c {
			out = append(out, r)
		}
	}
	return out
}

// HasBranchProtection checks that the default branch has protection rules enabled.
type HasBranchProtection struct{}

func (r HasBranchProtection) Name() string             { return "Has branch protection" }
func (r HasBranchProtection) Category() RuleCategory   { return CategoryScored }
func (r HasBranchProtection) Check(repo Repo) bool {
	return repo.BranchProtection != nil
}
func (r HasBranchProtection) Description() string {
	return "A branch-protection rule is configured on the default branch."
}
func (r HasBranchProtection) HowToFix() string {
	return "In repo Settings > Branches, add a protection rule for the default branch. [GitHub docs](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches)."
}

// HasRequiredReviewers checks that at least one approving review is required.
//
// This rule is admin-only: the required-approving-reviewer count on a
// classic per-repo branch protection is exposed only via the admin API
// (GET /repos/{o}/{r}/branches/{br}/protection, returns 404 to non-admins).
// Rulesets surface the count publicly, so repos using rulesets are still
// counted in non-admin scans, but most classic-protected repos can't be
// distinguished from "no protection." Rather than fail those silently,
// the scanner skips this rule entirely on non-admin scans (see
// WithAdmin in scanner.go).
type HasRequiredReviewers struct{}

func (r HasRequiredReviewers) Name() string           { return "Has required reviewers" }
func (r HasRequiredReviewers) Category() RuleCategory { return CategoryScored }
func (r HasRequiredReviewers) RequiresAdmin() bool    { return true }
func (r HasRequiredReviewers) Check(repo Repo) bool {
	return repo.BranchProtection != nil && repo.BranchProtection.RequiredReviewers >= 1
}
func (r HasRequiredReviewers) Description() string {
	return "The default branch's protection rules require at least one approving review before a PR can be merged."
}
func (r HasRequiredReviewers) HowToFix() string {
	return `In repo Settings > Branches, edit the default-branch protection rule and turn on "Require pull request reviews before merging" with at least 1 required reviewer.`
}

// HasRequiredStatusChecks checks that at least one status check is required before merging.
type HasRequiredStatusChecks struct{}

func (r HasRequiredStatusChecks) Name() string           { return "Requires status checks before merging" }
func (r HasRequiredStatusChecks) Category() RuleCategory { return CategoryScored }
func (r HasRequiredStatusChecks) Check(repo Repo) bool {
	return repo.BranchProtection != nil && len(repo.BranchProtection.RequiredStatusChecks) > 0
}
func (r HasRequiredStatusChecks) Description() string {
	return "The default branch's protection rules require at least one status check to pass before a PR can be merged."
}
func (r HasRequiredStatusChecks) HowToFix() string {
	return `In repo Settings > Branches, edit the default-branch protection rule and turn on "Require status checks to pass before merging".`
}

// HasCodeowners checks that a CODEOWNERS file exists in root, docs/, or .github/.
type HasCodeowners struct{}

func (r HasCodeowners) Name() string           { return "Has CODEOWNERS" }
func (r HasCodeowners) Category() RuleCategory { return CategoryScored }
func (r HasCodeowners) Check(repo Repo) bool {
	return hasFile(repo.Files, "CODEOWNERS") ||
		hasFile(repo.Files, "docs/CODEOWNERS") ||
		hasFile(repo.Files, ".github/CODEOWNERS")
}
func (r HasCodeowners) Description() string {
	return "A CODEOWNERS file exists at the repo root, in .github/, or in docs/."
}
func (r HasCodeowners) HowToFix() string {
	return "Add a CODEOWNERS file mapping paths to GitHub users or teams. [GitHub docs](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)."
}

// HasCIWorkflow checks that the repo has a CI workflow configured for any
// of the well-known CI providers, not just GitHub Actions. Detected via
// the presence of one of these signals at the repo root or under their
// canonical directory:
//
//   - GitHub Actions:  .github/workflows/*.yml or *.yaml
//   - CircleCI:        .circleci/config.yml
//   - GitLab CI:       .gitlab-ci.yml
//   - Travis CI:       .travis.yml
//   - Buildkite:       any file under .buildkite/
//   - Azure Pipelines: azure-pipelines.yml
//   - Jenkins:         Jenkinsfile
//
// Repos using a CI integration that lives entirely server-side (e.g.,
// CircleCI without a checked-in config) are still missed; this is a
// best-effort signal based on what's visible in the repo.
type HasCIWorkflow struct{}

func (r HasCIWorkflow) Name() string           { return "Has CI workflow" }
func (r HasCIWorkflow) Category() RuleCategory { return CategoryScored }
func (r HasCIWorkflow) Check(repo Repo) bool {
	for _, f := range repo.Files {
		// GitHub Actions workflows under .github/workflows/<anything>.yml|yaml.
		if strings.HasPrefix(f.Path, ".github/workflows/") &&
			(strings.HasSuffix(f.Path, ".yml") || strings.HasSuffix(f.Path, ".yaml")) {
			return true
		}
		// Buildkite uses a directory; any file inside counts.
		if strings.HasPrefix(f.Path, ".buildkite/") {
			return true
		}
		// Single-file CI configs at known paths.
		switch f.Path {
		case ".circleci/config.yml",
			".gitlab-ci.yml",
			".travis.yml",
			"azure-pipelines.yml",
			"Jenkinsfile":
			return true
		}
	}
	return false
}
func (r HasCIWorkflow) Description() string {
	return "The repo has a CI workflow configured: GitHub Actions (.github/workflows/), CircleCI (.circleci/config.yml), GitLab CI (.gitlab-ci.yml), Travis (.travis.yml), Buildkite (.buildkite/), Azure Pipelines (azure-pipelines.yml), or Jenkins (Jenkinsfile)."
}
func (r HasCIWorkflow) HowToFix() string {
	return "Add a workflow file for the CI provider you use - the simplest path on GitHub is a YAML workflow under .github/workflows/. [GitHub Actions quickstart](https://docs.github.com/en/actions/quickstart)."
}

// HasReadme checks that some form of README file exists at the repo root.
// Matches case-insensitively on the filename and accepts any extension
// (or no extension), so README.md, readme.rst, README.txt, Readme,
// README.markdown all pass. Subdirectory READMEs (e.g., docs/README.md)
// don't count - the rule is about a top-level project README.
//
// (No size threshold - the previous "substantial" variant was dropped
// because 2 KB is too low to discriminate quality and too high to reward
// minimal but useful READMEs.)
type HasReadme struct{}

func (r HasReadme) Name() string           { return "Has README" }
func (r HasReadme) Category() RuleCategory { return CategoryAdditional }
func (r HasReadme) Check(repo Repo) bool {
	for _, f := range repo.Files {
		if strings.Contains(f.Path, "/") {
			continue // not at root
		}
		lower := strings.ToLower(f.Path)
		if lower == "readme" || strings.HasPrefix(lower, "readme.") {
			return true
		}
	}
	return false
}
func (r HasReadme) Description() string {
	return "A README file exists at the repository root. The match is case-insensitive and accepts any extension or none, so README.md, README.rst, README.txt, readme, etc. all pass."
}
func (r HasReadme) HowToFix() string {
	return "Add a README that explains what the project is, how to install it, and how to use it."
}

// HasLicense uses GitHub's auto-detected license (Licensee) instead of
// a path-pattern match, so any conventionally-named license file works:
// LICENSE, LICENSE.md, LICENSE.txt, LICENCE (British), COPYING (GNU),
// MIT-LICENSE, etc. - anything GitHub recognizes and surfaces as the
// repo's `license.spdx_id` in the listing payload.
//
// Custom-text licenses GitHub can't auto-detect won't pass even though
// the file may be present. That's a known false negative; the trade-off
// is worth it for the much broader correct-positive coverage.
type HasLicense struct{}

func (r HasLicense) Name() string           { return "Has LICENSE" }
func (r HasLicense) Category() RuleCategory { return CategoryAdditional }
func (r HasLicense) Check(repo Repo) bool {
	return repo.License != ""
}
func (r HasLicense) Description() string {
	return "GitHub auto-detected an open-source license for the repository (any of LICENSE, LICENSE.md, COPYING, etc., recognizable by the Licensee gem)."
}
func (r HasLicense) HowToFix() string {
	return "Pick a license at [choosealicense.com](https://choosealicense.com) and add it to your repo root - GitHub will pick it up automatically."
}

// HasRepoDescription checks that the repo description field is not blank.
type HasRepoDescription struct{}

func (r HasRepoDescription) Name() string           { return "Has repo description" }
func (r HasRepoDescription) Category() RuleCategory { return CategoryAdditional }
func (r HasRepoDescription) Check(repo Repo) bool {
	return strings.TrimSpace(repo.Description) != ""
}
func (r HasRepoDescription) Description() string {
	return "The repository has a non-empty description set in repo settings (visible at the top of the GitHub repo page)."
}
func (r HasRepoDescription) HowToFix() string {
	return "Edit the repo and add a one-line description."
}

// HasActivity checks that the repo has had a commit (push) within the last
// 12 months. Set Now to a fixed time for deterministic testing; the zero
// value means time.Now() is used at check time.
type HasActivity struct {
	Now time.Time
}

func (r HasActivity) Name() string           { return "Has activity" }
func (r HasActivity) Category() RuleCategory { return CategoryAdditional }
func (r HasActivity) Check(repo Repo) bool {
	now := r.Now
	if now.IsZero() {
		now = time.Now()
	}
	return repo.PushedAt.After(now.AddDate(-1, 0, 0))
}
func (r HasActivity) Description() string {
	return "The repository has had a commit (push) within the last 12 months."
}
func (r HasActivity) HowToFix() string {
	return "Push a commit, or archive the repository if it is no longer maintained."
}

// HasSecurityMd checks that SECURITY.md exists in any of the three
// locations GitHub recognizes for security policies: repo root,
// .github/, or docs/.
type HasSecurityMd struct{}

func (r HasSecurityMd) Name() string           { return "Has SECURITY.md" }
func (r HasSecurityMd) Category() RuleCategory { return CategoryAdditional }
func (r HasSecurityMd) Check(repo Repo) bool {
	return hasFile(repo.Files, "SECURITY.md") ||
		hasFile(repo.Files, ".github/SECURITY.md") ||
		hasFile(repo.Files, "docs/SECURITY.md")
}
func (r HasSecurityMd) Description() string {
	return "A SECURITY.md file exists at the repository root, in .github/, or in docs/."
}
func (r HasSecurityMd) HowToFix() string {
	return "Add a SECURITY.md describing how to report vulnerabilities. [GitHub's template](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository)."
}

func findFile(files []FileEntry, path string) (FileEntry, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return FileEntry{}, false
}

func hasFile(files []FileEntry, path string) bool {
	_, ok := findFile(files, path)
	return ok
}
