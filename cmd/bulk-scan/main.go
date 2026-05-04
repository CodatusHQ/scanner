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

// stats is the JSON shape written to <out>/<org>/stats.json. The split
// between scored_rules and additional_checks mirrors the Markdown
// scorecard's two-section model: scored rules drive the org-level score,
// additional checks are informational coverage.
//
// Score is *int (rather than int) so we can serialize "N/A" as JSON null
// when the org had no scanned repos.
type stats struct {
	Org              string                `json:"org"`
	ScannedAt        time.Time             `json:"scanned_at"`
	TotalPublicRepos int                   `json:"total_public_repos"`
	ForksExcluded    int                   `json:"forks_excluded"`
	ArchivedExcluded int                   `json:"archived_excluded"`
	ReposScanned     int                   `json:"repos_scanned"`
	Score            *int                  `json:"score"`
	RepoBuckets      bucketCounts          `json:"repo_buckets"`
	ScoredRules      orderedRuleAggregates `json:"scored_rules"`
	AdditionalChecks orderedRuleAggregates `json:"additional_checks"`
	MostRecentCommit string                `json:"most_recent_commit"`
}

type ruleAggregate struct {
	Passing  int `json:"passing"`
	Failing  int `json:"failing"`
	PassRate int `json:"pass_rate"`
}

type bucketCounts struct {
	Strong   int `json:"strong"`
	Moderate int `json:"moderate"`
	Weak     int `json:"weak"`
}

// orderedRuleAggregates is a slice of (key, value) pairs that marshals to a
// JSON object preserving slice order. Go's default map[string]X marshalling
// sorts keys alphabetically, which would scramble the importance order
// callers care about.
type orderedRuleAggregates []ruleAggregateEntry

type ruleAggregateEntry struct {
	Key   string
	Value ruleAggregate
}

func (o orderedRuleAggregates) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteString("{")
	for i, e := range o {
		if i > 0 {
			b.WriteString(",")
		}
		keyJSON, err := json.Marshal(e.Key)
		if err != nil {
			return nil, err
		}
		valJSON, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		b.Write(keyJSON)
		b.WriteString(":")
		b.Write(valJSON)
	}
	b.WriteString("}")
	return []byte(b.String()), nil
}

func main() {
	var (
		orgsFile = flag.String("orgs", "", "path to file with one org/user slug per line (required)")
		outDir   = flag.String("out", "./bulk-scan-output", "output directory (one subfolder per org)")
		token    = flag.String("token", "", "GitHub token (defaults to $CODATUS_TOKEN)")
		admin    = flag.Bool("admin", false, "the token has admin access on every target repo. When false (default), admin-only rules are skipped and don't appear in the per-org JSON or Markdown output.")
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

		sr, scanErr := scanner.Scan(ctx, scanner.PATAuth{Token: *token, Name: org}, scanner.WithAdmin(*admin))
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
		score, defined := scanner.Score(sr)
		if defined {
			fmt.Fprintf(os.Stderr, " ok (%d scanned, score %d/100)\n", len(sr.Results), score)
		} else {
			fmt.Fprintf(os.Stderr, " ok (%d scanned, score N/A)\n", len(sr.Results))
		}
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
	mostRecent := ""
	if t := mostRecentCommit(sr); !t.IsZero() {
		mostRecent = t.Format("2006-01-02")
	}
	var scorePtr *int
	if score, defined := scanner.Score(sr); defined {
		scorePtr = &score
	}
	return stats{
		Org:              sr.Org,
		ScannedAt:        sr.ScannedAt,
		TotalPublicRepos: sr.TotalRepos,
		ForksExcluded:    sr.ForksExcluded,
		ArchivedExcluded: sr.ArchivedExcluded,
		ReposScanned:     len(sr.Results),
		Score:            scorePtr,
		RepoBuckets:      bucketCountsFor(sr.Results, sr.RulesScored),
		ScoredRules:      aggregate(sr.Results, sr.RulesScored),
		AdditionalChecks: aggregate(sr.Results, sr.RulesAdditional),
		MostRecentCommit: mostRecent,
	}
}

// bucketCountsFor counts repos per bucket. The struct fields match the
// JSON contract (strong/moderate/weak); if scanner.Buckets() ever adds a
// new bucket name this function silently drops it - that mismatch is
// caught by TestBucketCountsFor and TestBuildStats_NewShape.
func bucketCountsFor(results []scanner.RepoResult, scoredRules []scanner.Rule) bucketCounts {
	var bc bucketCounts
	for _, rr := range results {
		bucket, _, _, _ := scanner.BucketOf(rr, scoredRules)
		switch bucket.Name {
		case "Strong":
			bc.Strong++
		case "Moderate":
			bc.Moderate++
		case "Weak":
			bc.Weak++
		}
	}
	return bc
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

// aggregate counts pass/fail across results for a fixed list of rules,
// preserving the rules' input order. The caller is responsible for
// passing the right rules - typically sr.RulesScored or
// sr.RulesAdditional from the parent ScanResult, which correctly
// reflects WithAdmin filtering. Rules absent from results yield zero
// counts (legitimate when zero repos pass/fail, won't happen for the
// rule-set the scan actually evaluated).
func aggregate(results []scanner.RepoResult, rules []scanner.Rule) orderedRuleAggregates {
	out := make(orderedRuleAggregates, 0, len(rules))
	total := len(results)
	for _, rule := range rules {
		var agg ruleAggregate
		for _, rr := range results {
			for _, res := range rr.Results {
				if res.RuleName != rule.Name() {
					continue
				}
				if res.Passed {
					agg.Passing++
				} else {
					agg.Failing++
				}
				break
			}
		}
		if total > 0 {
			agg.PassRate = agg.Passing * 100 / total
		}
		out = append(out, ruleAggregateEntry{Key: jsonKey(rule.Name()), Value: agg})
	}
	return out
}
