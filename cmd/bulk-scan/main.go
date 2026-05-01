// bulk-scan reads a list of GitHub orgs/users from a file, runs the scanner
// against each one, and writes per-org output files (scorecard.md + stats.json)
// into a destination folder.
//
// Per-org failures (404, 403, etc.) are logged and the run continues.
// Global failures (rate limit, auth) abort immediately so partial runs do
// not poison every subsequent call. Output is written incrementally so any
// org that completed successfully before an abort keeps its files.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CodatusHQ/scanner"
)

// stats is the JSON shape written to <out>/<org>/stats.json. Field names
// match the snake_case keys requested by the bulk-scan use case (cold-email
// research personalization).
type stats struct {
	Org                  string                       `json:"org"`
	ScannedAt            time.Time                    `json:"scanned_at"`
	TotalPublicRepos     int                          `json:"total_public_repos"`
	ForksExcluded        int                          `json:"forks_excluded"`
	ArchivedExcluded     int                          `json:"archived_excluded"`
	ReposScanned         int                          `json:"repos_scanned"`
	CompliancePercentage int                          `json:"compliance_percentage"`
	FullyCompliantCount  int                          `json:"fully_compliant_count"`
	NonCompliantCount    int                          `json:"non_compliant_count"`
	RuleResults          map[string]ruleAggregate     `json:"rule_results"`
	MostRecentCommit     string                       `json:"most_recent_commit"`
}

type ruleAggregate struct {
	Passing  int `json:"passing"`
	Failing  int `json:"failing"`
	PassRate int `json:"pass_rate"`
}

func main() {
	var (
		orgsFile = flag.String("orgs", "", "path to file with one org/user slug per line (required)")
		outDir   = flag.String("out", "./bulk-scan-output", "output directory (one subfolder per org)")
		token    = flag.String("token", "", "GitHub token (defaults to $CODATUS_TOKEN)")
	)
	flag.Parse()

	if *orgsFile == "" {
		fmt.Fprintln(os.Stderr, "usage: bulk-scan --orgs file.txt [--out dir] [--token tok]")
		flag.PrintDefaults()
		os.Exit(2)
	}
	if *token == "" {
		*token = os.Getenv("CODATUS_TOKEN")
	}
	if *token == "" {
		log.Fatal("token is required (--token or $CODATUS_TOKEN)")
	}

	orgs, err := readOrgs(*orgsFile)
	if err != nil {
		log.Fatalf("read orgs from %s: %v", *orgsFile, err)
	}
	if len(orgs) == 0 {
		log.Fatalf("no orgs found in %s", *orgsFile)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("create output dir %s: %v", *outDir, err)
	}

	type orgFailure struct {
		org string
		err error
	}
	var (
		succeeded    []string
		failed       []orgFailure
		notAttempted []string
		aborted      bool
	)

	ctx := context.Background()
	total := len(orgs)
	for i, org := range orgs {
		if aborted {
			notAttempted = append(notAttempted, org)
			continue
		}

		fmt.Fprintf(os.Stderr, "[%d/%d] %s ...", i+1, total, org)

		sr, scanErr := scanner.Scan(ctx, scanner.PATAuth{Token: *token, Name: org})
		if scanErr != nil {
			if scanner.IsRateLimitError(scanErr) {
				fmt.Fprintf(os.Stderr, " ABORT: %v\n", scanErr)
				failed = append(failed, orgFailure{org, scanErr})
				aborted = true
				continue
			}
			fmt.Fprintf(os.Stderr, " FAILED: %v\n", scanErr)
			failed = append(failed, orgFailure{org, scanErr})
			continue
		}

		if writeErr := writeOrgOutput(*outDir, sr); writeErr != nil {
			fmt.Fprintf(os.Stderr, " FAILED writing output: %v\n", writeErr)
			failed = append(failed, orgFailure{org, writeErr})
			continue
		}

		succeeded = append(succeeded, org)
		compliancePct, fullyCompliant := computeCompliance(sr)
		fmt.Fprintf(os.Stderr, " ok (%d scanned, %d/%d compliant = %d%%)\n",
			len(sr.Results), fullyCompliant, len(sr.Results), compliancePct)
	}

	fmt.Fprintf(os.Stderr, "\nSummary: %d succeeded, %d failed, %d not attempted.\n",
		len(succeeded), len(failed), len(notAttempted))
	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "Failed:")
		for _, f := range failed {
			fmt.Fprintf(os.Stderr, "  - %s: %v\n", f.org, f.err)
		}
	}
	if len(notAttempted) > 0 {
		fmt.Fprintln(os.Stderr, "Not attempted (run aborted before reaching them):")
		for _, org := range notAttempted {
			fmt.Fprintf(os.Stderr, "  - %s\n", org)
		}
	}

	if len(failed) > 0 || len(notAttempted) > 0 {
		os.Exit(1)
	}
}

// readOrgs reads a list of org/user slugs from the file at path. One slug
// per line; blank lines are skipped. No comment handling - every non-blank
// line is treated as an org.
func readOrgs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var orgs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		orgs = append(orgs, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return orgs, nil
}

// writeOrgOutput writes <out>/<org>/scorecard.md and <out>/<org>/stats.json
// for a single completed scan.
func writeOrgOutput(outDir string, sr scanner.ScanResult) error {
	dir := filepath.Join(outDir, sr.Org)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	mdPath := filepath.Join(dir, "scorecard.md")
	if err := os.WriteFile(mdPath, []byte(scanner.GenerateReport(sr)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", mdPath, err)
	}

	jsonPath := filepath.Join(dir, "stats.json")
	statsBlob, err := json.MarshalIndent(buildStats(sr), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	if err := os.WriteFile(jsonPath, statsBlob, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", jsonPath, err)
	}

	return nil
}

func buildStats(sr scanner.ScanResult) stats {
	pct, fullyCompliant := computeCompliance(sr)
	mostRecent := ""
	if t := mostRecentCommit(sr); !t.IsZero() {
		mostRecent = t.Format("2006-01-02")
	}
	return stats{
		Org:                  sr.Org,
		ScannedAt:            sr.ScannedAt,
		TotalPublicRepos:     sr.TotalRepos,
		ForksExcluded:        sr.ForksExcluded,
		ArchivedExcluded:     sr.ArchivedExcluded,
		ReposScanned:         len(sr.Results),
		CompliancePercentage: pct,
		FullyCompliantCount:  fullyCompliant,
		NonCompliantCount:    len(sr.Results) - fullyCompliant,
		RuleResults:          aggregateRules(sr.Results),
		MostRecentCommit:     mostRecent,
	}
}

func computeCompliance(sr scanner.ScanResult) (pct int, fullyCompliant int) {
	for _, rr := range sr.Results {
		if isFullyCompliant(rr) {
			fullyCompliant++
		}
	}
	if len(sr.Results) == 0 {
		return 0, 0
	}
	return fullyCompliant * 100 / len(sr.Results), fullyCompliant
}

func isFullyCompliant(rr scanner.RepoResult) bool {
	for _, r := range rr.Results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// mostRecentCommit returns the latest MostRecentCommit across every repo in
// the scan result (both successfully scanned and skipped). Skipped repos
// still have a meaningful PushedAt - an empty repo can have recent activity
// even if we couldn't read its file tree. Zero time if there are no repos.
func mostRecentCommit(sr scanner.ScanResult) time.Time {
	var latest time.Time
	for _, rr := range sr.Results {
		if rr.MostRecentCommit.After(latest) {
			latest = rr.MostRecentCommit
		}
	}
	for _, rr := range sr.Skipped {
		if rr.MostRecentCommit.After(latest) {
			latest = rr.MostRecentCommit
		}
	}
	return latest
}

// jsonKey converts a rule's display name to a snake_case JSON key. Lowercase
// the input, then collapse runs of non-alphanumeric characters into a single
// underscore, then trim leading/trailing underscores.
//
//   "Has repo description"                       -> "has_repo_description"
//   "Has SECURITY.md"                            -> "has_security_md"
//   "Requires status checks before merging"      -> "requires_status_checks_before_merging"
//
// We derive instead of hard-coding because the scanner library shouldn't
// know or care about how its consumers serialize results.
func jsonKey(ruleName string) string {
	var b strings.Builder
	prevUnderscore := true // start true so leading non-alnum is dropped
	for _, r := range strings.ToLower(ruleName) {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	out := b.String()
	return strings.TrimRight(out, "_")
}

// aggregateRules computes per-rule passing/failing/pass_rate counts keyed by
// the snake_case form of each rule's display name. Only rules listed in
// scanner.AllRules() are included - if a result names a rule we don't know
// about, it is dropped (defensive against renamed/removed rules).
func aggregateRules(results []scanner.RepoResult) map[string]ruleAggregate {
	known := make(map[string]bool)
	for _, r := range scanner.AllRules() {
		known[r.Name()] = true
	}

	out := make(map[string]ruleAggregate)
	total := len(results)
	for _, rr := range results {
		for _, rule := range rr.Results {
			if !known[rule.RuleName] {
				continue
			}
			key := jsonKey(rule.RuleName)
			agg := out[key]
			if rule.Passed {
				agg.Passing++
			} else {
				agg.Failing++
			}
			out[key] = agg
		}
	}
	for key, agg := range out {
		if total > 0 {
			agg.PassRate = agg.Passing * 100 / total
		}
		out[key] = agg
	}
	return out
}
