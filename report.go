package scanner

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateReport produces a Markdown engineering-standards scorecard from a
// ScanResult. The header emits totals and exclusion counts (forks, archived);
// scanned-repo sections render the same Markdown the app posts to GitHub
// Issues and the codatus.com sample fetches via marked.js.
func GenerateReport(sr ScanResult) string {
	var b strings.Builder

	scanned := sr.Results
	skipped := sr.Skipped
	compliant, nonCompliant := splitByCompliance(scanned)

	b.WriteString("# Codatus - Engineering Standards Scorecard\n\n")
	fmt.Fprintf(&b, "**Org:** %s\n", sr.Org)
	fmt.Fprintf(&b, "**Scanned:** %s\n", sr.ScannedAt.UTC().Format("2006-01-02 15:04 UTC"))
	if sr.TotalRepos > 0 {
		fmt.Fprintf(&b, "**Total repos:** %d\n", sr.TotalRepos)
	}
	if sr.ForksExcluded > 0 {
		fmt.Fprintf(&b, "**Forks excluded:** %d\n", sr.ForksExcluded)
	}
	if sr.ArchivedExcluded > 0 {
		fmt.Fprintf(&b, "**Archived excluded:** %d\n", sr.ArchivedExcluded)
	}
	fmt.Fprintf(&b, "**Repos scanned:** %d\n", len(scanned))
	if len(scanned) > 0 {
		fmt.Fprintf(&b, "**Compliant:** %d/%d (%d%%) *(a repo is compliant when it passes all rules below)*\n",
			len(compliant), len(scanned), len(compliant)*100/len(scanned))
	}
	if len(skipped) > 0 {
		fmt.Fprintf(&b, "**Skipped:** %d\n", len(skipped))
	}

	if len(scanned) == 0 && len(skipped) == 0 {
		b.WriteString("\nNo repos found.\n")
		return b.String()
	}

	if len(scanned) > 0 {
		b.WriteString("\n## Summary\n\n")
		writeSummaryTable(&b, scanned)
		writeRuleReferenceSection(&b, scanned)
	}

	if len(compliant) > 0 {
		writeCompliantSection(&b, sr.Org, compliant)
	}

	if len(nonCompliant) > 0 {
		writeNonCompliantSection(&b, sr.Org, nonCompliant)
	}

	if len(skipped) > 0 {
		writeSkippedSection(&b, sr.Org, skipped)
	}

	return b.String()
}

func splitByCompliance(results []RepoResult) (compliant, nonCompliant []RepoResult) {
	for _, rr := range results {
		if isFullyCompliant(rr) {
			compliant = append(compliant, rr)
		} else {
			nonCompliant = append(nonCompliant, rr)
		}
	}
	return
}

func isFullyCompliant(rr RepoResult) bool {
	for _, r := range rr.Results {
		if !r.Passed {
			return false
		}
	}
	return true
}

func failingRules(rr RepoResult) []string {
	var names []string
	for _, r := range rr.Results {
		if !r.Passed {
			names = append(names, r.RuleName)
		}
	}
	return names
}

type ruleSummary struct {
	name     string
	passing  int
	failing  int
	passRate int
}

func writeSummaryTable(b *strings.Builder, results []RepoResult) {
	if len(results) == 0 {
		return
	}

	ruleOrder := make([]string, 0)
	counts := make(map[string]*ruleSummary)

	for _, rr := range results {
		for _, result := range rr.Results {
			s, ok := counts[result.RuleName]
			if !ok {
				s = &ruleSummary{name: result.RuleName}
				counts[result.RuleName] = s
				ruleOrder = append(ruleOrder, result.RuleName)
			}
			if result.Passed {
				s.passing++
			} else {
				s.failing++
			}
		}
	}

	summaries := make([]ruleSummary, 0, len(ruleOrder))
	total := len(results)
	for _, name := range ruleOrder {
		s := counts[name]
		s.passRate = s.passing * 100 / total
		summaries = append(summaries, *s)
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].passRate < summaries[j].passRate
	})

	b.WriteString("| Rule | Passing | Failing | Pass rate |\n")
	b.WriteString("|------|---------|---------|----------|\n")
	for _, s := range summaries {
		fmt.Fprintf(b, "| %s | %d | %d | %d%% |\n", s.name, s.passing, s.failing, s.passRate)
	}
}

// writeRuleReferenceSection emits a collapsed <details> block that lists,
// for each rule actually present in the scan results, its name plus the
// "what it checks" / "how to fix" text the rule provides. Iteration order
// follows AllRules so the section is deterministic regardless of how the
// summary table is sorted. If none of the rule names in results match a
// known rule (e.g. ad-hoc tests), the section is omitted entirely.
func writeRuleReferenceSection(b *strings.Builder, results []RepoResult) {
	seen := make(map[string]bool)
	for _, rr := range results {
		for _, result := range rr.Results {
			seen[result.RuleName] = true
		}
	}

	var entries []Rule
	for _, r := range AllRules() {
		if seen[r.Name()] {
			entries = append(entries, r)
		}
	}

	if len(entries) == 0 {
		return
	}

	b.WriteString("\n<details>\n<summary>Rule reference - what each rule checks and how to fix it</summary>\n\n")
	for i, r := range entries {
		fmt.Fprintf(b, "### %s\n\n", r.Name())
		fmt.Fprintf(b, "- **What it checks:** %s\n", r.Description())
		fmt.Fprintf(b, "- **How to fix:** %s\n\n", r.HowToFix())
		if i < len(entries)-1 {
			b.WriteString("---\n\n")
		}
	}
	b.WriteString("</details>\n")
}

func pluralRepos(n int) string {
	if n == 1 {
		return "1 repo"
	}
	return fmt.Sprintf("%d repos", n)
}

func writeCompliantSection(b *strings.Builder, org string, compliant []RepoResult) {
	fmt.Fprintf(b, "\n## ✅ Fully compliant (%s)\n\n", pluralRepos(len(compliant)))
	b.WriteString("<details>\n<summary>All rules passing</summary>\n\n")
	for _, rr := range compliant {
		fmt.Fprintf(b, "[%s](https://github.com/%s/%s)\n", rr.RepoName, org, rr.RepoName)
	}
	b.WriteString("\n</details>\n")
}

func writeNonCompliantSection(b *strings.Builder, org string, nonCompliant []RepoResult) {
	fmt.Fprintf(b, "\n## ❌ Non-compliant (%s)\n\n", pluralRepos(len(nonCompliant)))
	for _, rr := range nonCompliant {
		failing := failingRules(rr)
		fmt.Fprintf(b, "<details>\n<summary><a href=\"https://github.com/%s/%s\">%s</a> - %d failing</summary>\n\n",
			org, rr.RepoName, rr.RepoName, len(failing))
		for _, name := range failing {
			fmt.Fprintf(b, "- %s\n", name)
		}
		b.WriteString("\n</details>\n\n")
	}
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
