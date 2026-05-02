// Package samples provides the canonical sample scorecard used to drive the
// landing page hero and the app's dev-seed data. The fixture is hand-built
// rather than captured from a real scan so the numbers stay stable and
// don't leak details about real orgs.
//
// Fixture() is the single source of truth; consumers either call
// scanner.GenerateReport(samples.Fixture()) in process (the app's dev-seed
// does this) or pipe `go run github.com/CodatusHQ/scanner/cmd/generate-sample`
// into a Markdown file (codatus.com refreshes its committed copy this way).
// No rendered .md is committed in this repo - the fixture is the artifact.
package samples

import (
	"time"

	"github.com/CodatusHQ/scanner"
)

// Fixture returns a deterministic ScanResult for the fictional "acme-corp"
// org. The shape is tuned so the generated scorecard exercises every
// section: Scored rules table with mixed pass rates, Score callout in the
// Moderate range, Additional checks table, all three buckets in Repository
// details (Strong / Moderate / Weak), Rule reference (split by category),
// and a Skipped section.
func Fixture() scanner.ScanResult {
	scannedAt := time.Date(2026, 5, 1, 10, 30, 0, 0, time.UTC)

	// Recency markers used for PushedAt. Concrete instants so the JSON
	// stats output (which surfaces most_recent_commit) is deterministic.
	recent := scannedAt.AddDate(0, 0, -2)
	thisQuarter := scannedAt.AddDate(0, -2, 0)
	thisYear := scannedAt.AddDate(0, -8, 0)
	stale := scannedAt.AddDate(-2, 0, 0)

	scoredRuleNames := []string{
		"Has branch protection",
		"Has required reviewers",
		"Requires status checks before merging",
		"Has CODEOWNERS",
		"Has CI workflow",
	}
	additionalRuleNames := []string{
		"Has README",
		"Has LICENSE",
		"Has repo description",
		"Has activity",
		"Has SECURITY.md",
	}

	// passing[ruleName] = true means the repo passes that rule. Any rule in
	// scoredRuleNames+additionalRuleNames not listed here is recorded as
	// failing. Keeping the per-repo set small and explicit makes the
	// fixture easy to audit when tuning the sample.
	repos := []struct {
		name     string
		pushedAt time.Time
		passing  map[string]bool
	}{
		// Strong (5/5 scored): polished public repos, full hygiene.
		{
			name:     "acme-platform",
			pushedAt: recent,
			passing: setOf(
				"Has branch protection", "Has required reviewers",
				"Requires status checks before merging", "Has CODEOWNERS",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity", "Has SECURITY.md",
			),
		},
		{
			name:     "acme-api",
			pushedAt: recent,
			passing: setOf(
				"Has branch protection", "Has required reviewers",
				"Requires status checks before merging", "Has CODEOWNERS",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity", "Has SECURITY.md",
			),
		},

		// Moderate (3/5 scored): protected branches and CI in place, gaps
		// on status checks and/or CODEOWNERS - the typical SaaS shape.
		{
			name:     "acme-billing",
			pushedAt: thisQuarter,
			passing: setOf(
				"Has branch protection", "Has required reviewers",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity", "Has SECURITY.md",
			),
		},
		{
			name:     "acme-dashboard",
			pushedAt: thisQuarter,
			passing: setOf(
				"Has branch protection", "Has required reviewers",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity",
			),
		},
		{
			name:     "acme-mobile",
			pushedAt: thisQuarter,
			passing: setOf(
				"Has branch protection", "Has required reviewers",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity",
			),
		},
		{
			name:     "acme-analytics",
			pushedAt: thisYear,
			passing: setOf(
				"Has branch protection", "Has CODEOWNERS",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity",
			),
		},
		{
			name:     "acme-search",
			pushedAt: thisYear,
			passing: setOf(
				"Has branch protection", "Requires status checks before merging",
				"Has CI workflow",
				"Has README", "Has LICENSE", "Has repo description",
				"Has activity",
			),
		},

		// Weak (1/5 scored): only CI is in place. Older repos with no
		// active maintenance.
		{
			name:     "acme-legacy",
			pushedAt: stale,
			passing: setOf(
				"Has CI workflow",
				"Has README", "Has repo description",
			),
		},
		{
			name:     "acme-prototype",
			pushedAt: stale,
			passing: setOf(
				"Has CI workflow",
			),
		},
		{
			name:     "acme-internal-tools",
			pushedAt: stale,
			passing: setOf(
				"Has CI workflow",
				"Has README",
			),
		},
	}

	results := make([]scanner.RepoResult, 0, len(repos))
	allRules := append(append([]string{}, scoredRuleNames...), additionalRuleNames...)
	for _, r := range repos {
		ruleResults := make([]scanner.RuleResult, 0, len(allRules))
		for _, name := range allRules {
			ruleResults = append(ruleResults, scanner.RuleResult{
				RuleName: name,
				Passed:   r.passing[name],
			})
		}
		results = append(results, scanner.RepoResult{
			RepoName:         r.name,
			MostRecentCommit: r.pushedAt,
			Results:          ruleResults,
		})
	}

	skipped := []scanner.RepoResult{
		{
			RepoName:         "acme-empty",
			MostRecentCommit: scannedAt.AddDate(0, -1, 0),
			KnownSkipReason:  "repository is empty",
		},
	}

	return scanner.ScanResult{
		Org:              "acme-corp",
		ScannedAt:        scannedAt,
		TotalRepos:       15, // 10 scanned + 1 skipped + 3 forks + 1 archived
		ForksExcluded:    3,
		ArchivedExcluded: 1,
		Results:          results,
		Skipped:          skipped,
	}
}

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
