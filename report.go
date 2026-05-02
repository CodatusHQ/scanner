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
//   1. Header: title, org, scan time, single-line repo stats
//   2. ## Scored rules table (importance order, drives the score)
//   3. **Score: N/100** inline callout (or **Score: N/A** when no repos)
//   4. ## Additional checks table (importance order, same columns as scored)
//   5. ## Repository details: ### Strong / Moderate / Weak / Skipped subsections
//   6. ## Rule reference (collapsed <details>, split by category)
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
	} else {
		// No scanned repos but some were skipped: emit the score callout
		// in its N/A form so the structure is consistent.
		writeScoreCallout(&b, sr)
	}

	writeRepoDetailsSection(&b, sr.Org, scanned, skipped)

	if len(scanned) > 0 {
		writeRuleReferenceSection(&b, scanned)
	}

	return b.String()
}

func writeHeader(b *strings.Builder, sr ScanResult) {
	b.WriteString("# Codatus - Engineering Standards Scorecard\n\n")
	// Each header line ends with `<br>` so spec-compliant Markdown
	// renderers (marked.js, kramdown, GitHub) emit one line per item
	// instead of folding consecutive single-newlines into one paragraph.
	// CommonMark's Raw HTML rule allows inline phrasing tags like br.
	fmt.Fprintf(b, "**Org:** %s<br>\n", sr.Org)
	fmt.Fprintf(b, "**Scanned:** %s<br>\n", sr.ScannedAt.UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(b, "**Repos:** %s\n", repoStatsLine(sr))
}

// repoStatsLine collapses scanned/total/forks/archived/skipped into one
// readable line: `10 of 15 scanned (3 forks excluded, 1 archived excluded,
// 1 skipped)`. Zero-valued breakdown fields drop out of the parenthetical;
// when nothing was excluded or skipped, the parenthetical is omitted
// entirely. Falls back to plain `N scanned` when TotalRepos is unknown.
func repoStatsLine(sr ScanResult) string {
	scanned := len(sr.Results)
	headline := fmt.Sprintf("%d scanned", scanned)
	if sr.TotalRepos > 0 {
		headline = fmt.Sprintf("%d of %d scanned", scanned, sr.TotalRepos)
	}

	var parts []string
	if sr.ForksExcluded > 0 {
		parts = append(parts, fmt.Sprintf("%d forks excluded", sr.ForksExcluded))
	}
	if sr.ArchivedExcluded > 0 {
		parts = append(parts, fmt.Sprintf("%d archived excluded", sr.ArchivedExcluded))
	}
	if len(sr.Skipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", len(sr.Skipped)))
	}
	if len(parts) == 0 {
		return headline
	}
	return fmt.Sprintf("%s (%s)", headline, strings.Join(parts, ", "))
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
	// Same column layout as Scored rules so the two tables visually
	// align in any renderer that auto-sizes by header text. The section
	// heading already conveys "informational only".
	b.WriteString("\n## Additional checks\n\n")
	b.WriteString("| Rule | Passing | Failing | Pass rate |\n")
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

// writeRepoDetailsSection renders a single ## Repository details
// section that groups every repo - successfully scanned and skipped -
// into ### subsections: Strong / Moderate / Weak by score, then Skipped
// for repos that couldn't be evaluated. Empty subsections are omitted;
// the section header itself is suppressed when nothing has any rows.
func writeRepoDetailsSection(b *strings.Builder, org string, scanned, skipped []RepoResult) {
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

	hasScored := false
	for _, g := range groups {
		if len(g.repos) > 0 {
			hasScored = true
			break
		}
	}
	if !hasScored && len(skipped) == 0 {
		return
	}

	b.WriteString("\n## Repository details\n")
	for _, g := range groups {
		if len(g.repos) == 0 {
			continue
		}
		writeBucketSection(b, org, g.bucket, g.repos)
	}
	if len(skipped) > 0 {
		writeSkippedSubsection(b, org, skipped)
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
		_, _, _, scorePct := BucketOf(rr)

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
			"\n<details>\n<summary><a href=\"https://github.com/%s/%s\">%s</a> - %d%%</summary>\n",
			org, rr.RepoName, rr.RepoName, scorePct,
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

// writeSkippedSubsection emits the ### Skipped (N) heading and the list
// of repos that couldn't be scanned. Rendered as the last subsection
// inside ## Repository details so skipped repos read as another
// classification (after Weak) rather than a separate document section.
func writeSkippedSubsection(b *strings.Builder, org string, skipped []RepoResult) {
	fmt.Fprintf(b, "\n### Skipped (%s)\n\n", pluralRepos(len(skipped)))
	for _, rr := range skipped {
		if rr.KnownSkipReason != "" {
			fmt.Fprintf(b, "- [%s](https://github.com/%s/%s) - %s\n", rr.RepoName, org, rr.RepoName, rr.KnownSkipReason)
		} else {
			fmt.Fprintf(b, "- [%s](https://github.com/%s/%s) - unexpected error: %s\n", rr.RepoName, org, rr.RepoName, rr.UnknownSkipError)
		}
	}
}
