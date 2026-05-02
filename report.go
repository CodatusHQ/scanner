package scanner

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateReport produces a Markdown engineering-standards scorecard from a
// ScanResult. The structure is fixed and meaningful for prospects landing
// from a cold-email link:
//
//   1. Header with totals and exclusion counts
//   2. ## Scored rules table (importance order, drives the score)
//   3. **Score: N/100** inline callout (or **Score: N/A** when no repos)
//   4. ## Additional checks table (importance order, "Coverage" column)
//   5. ## Repository details with three score-based buckets
//   6. ## Rule reference (collapsed <details>, split by category)
//   7. ## ⚠️ Skipped (only if any repos couldn't be scanned)
func GenerateReport(sr ScanResult) string {
	var b strings.Builder

	scanned := sr.Results
	skipped := sr.Skipped

	writeHeader(&b, sr)

	if len(scanned) == 0 && len(skipped) == 0 {
		b.WriteString("\nNo repos found.\n")
		return b.String()
	}

	if len(scanned) > 0 {
		writeScoredRulesSection(&b, scanned)
		writeScoreCallout(&b, sr)
		writeAdditionalChecksSection(&b, scanned)
		writeRepoDetailsSection(&b, sr.Org, scanned)
		writeRuleReferenceSection(&b, scanned)
	} else {
		// No scanned repos but some were skipped: emit the score callout
		// in its N/A form so the structure is consistent.
		writeScoreCallout(&b, sr)
	}

	if len(skipped) > 0 {
		writeSkippedSection(&b, sr.Org, skipped)
	}

	return b.String()
}

func writeHeader(b *strings.Builder, sr ScanResult) {
	b.WriteString("# Codatus - Engineering Standards Scorecard\n\n")
	fmt.Fprintf(b, "**Org:** %s\n", sr.Org)
	fmt.Fprintf(b, "**Scanned:** %s\n", sr.ScannedAt.UTC().Format("2006-01-02 15:04 UTC"))
	if sr.TotalRepos > 0 {
		fmt.Fprintf(b, "**Total repos:** %d\n", sr.TotalRepos)
	}
	if sr.ForksExcluded > 0 {
		fmt.Fprintf(b, "**Forks excluded:** %d\n", sr.ForksExcluded)
	}
	if sr.ArchivedExcluded > 0 {
		fmt.Fprintf(b, "**Archived excluded:** %d\n", sr.ArchivedExcluded)
	}
	fmt.Fprintf(b, "**Repos scanned:** %d\n", len(sr.Results))
	if len(sr.Skipped) > 0 {
		fmt.Fprintf(b, "**Skipped:** %d\n", len(sr.Skipped))
	}
}

type ruleAggregate struct {
	rule     Rule
	passing  int
	failing  int
	passRate int
}

// aggregate counts pass/fail across results for a fixed list of rules,
// preserving the rules' input order. Rule names not present in the
// scan results contribute zero counts (they still appear in the table).
func aggregate(results []RepoResult, rules []Rule) []ruleAggregate {
	out := make([]ruleAggregate, len(rules))
	total := len(results)
	for i, rule := range rules {
		out[i].rule = rule
		for _, rr := range results {
			for _, res := range rr.Results {
				if res.RuleName == rule.Name() {
					if res.Passed {
						out[i].passing++
					} else {
						out[i].failing++
					}
					break
				}
			}
		}
		if total > 0 {
			out[i].passRate = out[i].passing * 100 / total
		}
	}
	return out
}

func writeScoredRulesSection(b *strings.Builder, scanned []RepoResult) {
	b.WriteString("\n## Scored rules\n\n")
	b.WriteString("| Rule | Passing | Failing | Pass rate |\n")
	b.WriteString("|------|---------|---------|----------|\n")
	for _, agg := range aggregate(scanned, ScoredRules()) {
		fmt.Fprintf(b, "| %s | %d | %d | %d%% |\n", agg.rule.Name(), agg.passing, agg.failing, agg.passRate)
	}
}

func writeAdditionalChecksSection(b *strings.Builder, scanned []RepoResult) {
	b.WriteString("\n## Additional checks\n\n")
	b.WriteString("| Check | Passing | Failing | Coverage |\n")
	b.WriteString("|------|---------|---------|----------|\n")
	for _, agg := range aggregate(scanned, AdditionalRules()) {
		fmt.Fprintf(b, "| %s | %d | %d | %d%% |\n", agg.rule.Name(), agg.passing, agg.failing, agg.passRate)
	}
}

func writeScoreCallout(b *strings.Builder, sr ScanResult) {
	score, defined := Score(sr)
	if defined {
		fmt.Fprintf(b, "\n**Score: %d/100** (average pass rate across the scored rules above)\n", score)
	} else {
		b.WriteString("\n**Score: N/A** (no repos available to score)\n")
	}
}

// writeRuleReferenceSection emits a collapsed <details> block listing
// every rule actually present in the scan results, split into Scored
// rules and Additional checks subsections. Each subsection's content is
// ordered to match the corresponding summary table above.
//
// Rules absent from results (e.g., ad-hoc test fixtures using made-up
// rule names) are omitted; if no rules survive the filter, the entire
// section is skipped.
func writeRuleReferenceSection(b *strings.Builder, results []RepoResult) {
	seen := make(map[string]bool)
	for _, rr := range results {
		for _, result := range rr.Results {
			seen[result.RuleName] = true
		}
	}

	scored := filterPresent(ScoredRules(), seen)
	additional := filterPresent(AdditionalRules(), seen)
	if len(scored) == 0 && len(additional) == 0 {
		return
	}

	b.WriteString("\n## Rule reference\n\n<details>\n<summary>What each rule checks and how to fix it</summary>\n")

	if len(scored) > 0 {
		b.WriteString("\n### Scored rules\n")
		writeRuleReferenceEntries(b, scored)
	}
	if len(additional) > 0 {
		b.WriteString("\n### Additional checks\n")
		writeRuleReferenceEntries(b, additional)
	}

	b.WriteString("\n</details>\n")
}

func filterPresent(rules []Rule, seen map[string]bool) []Rule {
	var out []Rule
	for _, r := range rules {
		if seen[r.Name()] {
			out = append(out, r)
		}
	}
	return out
}

func writeRuleReferenceEntries(b *strings.Builder, rules []Rule) {
	for i, r := range rules {
		fmt.Fprintf(b, "\n#### %s\n\n", r.Name())
		fmt.Fprintf(b, "- **What it checks:** %s\n", r.Description())
		fmt.Fprintf(b, "- **How to fix:** %s\n", r.HowToFix())
		if i < len(rules)-1 {
			b.WriteString("\n---\n")
		}
	}
}

func writeRepoDetailsSection(b *strings.Builder, org string, scanned []RepoResult) {
	type bucketEntry struct {
		bucket Bucket
		repos  []RepoResult
	}

	defs := Buckets()
	groups := make([]*bucketEntry, len(defs))
	for i, def := range defs {
		groups[i] = &bucketEntry{bucket: def}
	}
	for _, rr := range scanned {
		b, _, _, _ := BucketOf(rr)
		for _, g := range groups {
			if g.bucket.Name == b.Name {
				g.repos = append(g.repos, rr)
				break
			}
		}
	}

	// Suppress the entire ## Repository details heading if every bucket is
	// empty. This shouldn't happen if we get here (caller guards on
	// len(scanned) > 0) but keeps the writer robust.
	hasAny := false
	for _, g := range groups {
		if len(g.repos) > 0 {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return
	}

	b.WriteString("\n## Repository details\n")
	for _, g := range groups {
		if len(g.repos) == 0 {
			continue
		}
		writeBucketSection(b, org, g.bucket, g.repos)
	}
}

// bucketRangeLabel renders a Bucket's percentage span for display. The
// open-ended buckets (top and bottom) get a one-sided label; middle
// buckets get a hyphenated span. Derived from MinPct/MaxPct so the int
// definitions in Buckets() are the single source of truth.
func bucketRangeLabel(b Bucket) string {
	switch {
	case b.MaxPct == 100:
		return fmt.Sprintf("≥%d%%", b.MinPct)
	case b.MinPct == 0:
		return fmt.Sprintf("≤%d%%", b.MaxPct)
	default:
		return fmt.Sprintf("%d-%d%%", b.MinPct, b.MaxPct)
	}
}

func writeBucketSection(b *strings.Builder, org string, bucket Bucket, repos []RepoResult) {
	sort.Slice(repos, func(i, j int) bool { return repos[i].RepoName < repos[j].RepoName })

	fmt.Fprintf(b, "\n### %s (%s)\n", bucket.Name, bucketRangeLabel(bucket))

	scoredNames := make(map[string]bool)
	for _, r := range ScoredRules() {
		scoredNames[r.Name()] = true
	}

	for _, rr := range repos {
		_, scoredPassing, scoredTotal, scorePct := BucketOf(rr)

		var failingScored, failingAdditional []string
		for _, res := range rr.Results {
			if res.Passed {
				continue
			}
			if scoredNames[res.RuleName] {
				failingScored = append(failingScored, res.RuleName)
			} else {
				failingAdditional = append(failingAdditional, res.RuleName)
			}
		}

		fmt.Fprintf(b,
			"\n<details>\n<summary><a href=\"https://github.com/%s/%s\">%s</a> - %d%% (%d/%d scored rules passing)</summary>\n",
			org, rr.RepoName, rr.RepoName, scorePct, scoredPassing, scoredTotal,
		)
		if len(failingScored) > 0 {
			b.WriteString("\n**Failing scored rules:**\n")
			for _, name := range failingScored {
				fmt.Fprintf(b, "- %s\n", name)
			}
		}
		if len(failingAdditional) > 0 {
			b.WriteString("\n**Additional check failures:**\n")
			for _, name := range failingAdditional {
				fmt.Fprintf(b, "- %s\n", name)
			}
		}
		b.WriteString("\n</details>\n")
	}
}

func pluralRepos(n int) string {
	if n == 1 {
		return "1 repo"
	}
	return fmt.Sprintf("%d repos", n)
}

func writeSkippedSection(b *strings.Builder, org string, skipped []RepoResult) {
	fmt.Fprintf(b, "\n## ⚠️ Skipped (%s)\n\n", pluralRepos(len(skipped)))
	for _, rr := range skipped {
		if rr.KnownSkipReason != "" {
			fmt.Fprintf(b, "- [%s](https://github.com/%s/%s) - %s\n", rr.RepoName, org, rr.RepoName, rr.KnownSkipReason)
		} else {
			fmt.Fprintf(b, "- [%s](https://github.com/%s/%s) - unexpected error: %s\n", rr.RepoName, org, rr.RepoName, rr.UnknownSkipError)
		}
	}
}
