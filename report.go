package scanner

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// GenerateReport produces a Markdown compliance report from scan results.
func GenerateReport(org string, results []RepoResult) string {
	return generateReport(org, results, time.Now())
}

func generateReport(org string, results []RepoResult, now time.Time) string {
	var b strings.Builder

	scanned, skipped := splitScanned(results)
	compliant, nonCompliant := splitByCompliance(scanned)

	b.WriteString("# Codatus - Org Compliance Report\n\n")
	fmt.Fprintf(&b, "**Org:** %s\n", org)
	fmt.Fprintf(&b, "**Scanned:** %s\n", now.UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "**Repos scanned:** %d\n", len(scanned))
	if len(scanned) > 0 {
		fmt.Fprintf(&b, "**Compliant:** %d/%d (%d%%)\n", len(compliant), len(scanned), len(compliant)*100/len(scanned))
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
	}

	if len(compliant) > 0 {
		writeCompliantSection(&b, org, compliant)
	}

	if len(nonCompliant) > 0 {
		writeNonCompliantSection(&b, org, nonCompliant)
	}

	if len(skipped) > 0 {
		writeSkippedSection(&b, org, skipped)
	}

	return b.String()
}

func splitScanned(results []RepoResult) (scanned, skipped []RepoResult) {
	for _, rr := range results {
		if rr.Skipped() {
			skipped = append(skipped, rr)
		} else {
			scanned = append(scanned, rr)
		}
	}
	return
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
