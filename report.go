package scanner

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// GenerateReport produces a Markdown compliance report from scan results.
// The format matches the specification in README.md.
func GenerateReport(org string, results []RepoResult) string {
	var b strings.Builder

	b.WriteString("# Codatus - Org Compliance Report\n\n")
	fmt.Fprintf(&b, "**Org:** %s\n", org)
	fmt.Fprintf(&b, "**Scanned:** %s\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "**Repos scanned:** %d\n", len(results))

	b.WriteString("\n## Summary\n\n")
	writeSummaryTable(&b, results)

	b.WriteString("\n## Results by repository\n")
	for _, rr := range results {
		writeRepoTable(&b, rr)
	}

	return b.String()
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

	// Aggregate pass/fail counts per rule name.
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

	// Calculate pass rates.
	summaries := make([]ruleSummary, 0, len(ruleOrder))
	total := len(results)
	for _, name := range ruleOrder {
		s := counts[name]
		s.passRate = s.passing * 100 / total
		summaries = append(summaries, *s)
	}

	// Sort by pass rate ascending (worst compliance first).
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].passRate < summaries[j].passRate
	})

	b.WriteString("| Rule | Passing | Failing | Pass rate |\n")
	b.WriteString("|------|---------|---------|----------|\n")
	for _, s := range summaries {
		fmt.Fprintf(b, "| %s | %d | %d | %d%% |\n", s.name, s.passing, s.failing, s.passRate)
	}
}

func writeRepoTable(b *strings.Builder, rr RepoResult) {
	fmt.Fprintf(b, "\n### %s\n\n", rr.RepoName)
	b.WriteString("| Rule | Result |\n")
	b.WriteString("|------|--------|\n")
	for _, result := range rr.Results {
		icon := "❌"
		if result.Passed {
			icon = "✅"
		}
		fmt.Fprintf(b, "| %s | %s |\n", result.RuleName, icon)
	}
}
