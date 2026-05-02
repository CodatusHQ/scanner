package samples_test

import (
	"strings"
	"testing"

	"github.com/CodatusHQ/scanner"
	"github.com/CodatusHQ/scanner/samples"
)

// TestFixtureRenders is a smoke test: with no committed sample.md to keep
// in sync, this is the only thing in CI that catches a fixture/formatter
// mismatch (e.g. a rule renamed without updating the fixture, or a fixture
// shape the report writer can't handle). It asserts the major sections
// land in the output, not their exact contents.
func TestFixtureRenders(t *testing.T) {
	got := scanner.GenerateReport(samples.Fixture())

	for _, want := range []string{
		"## Scored rules",
		"**Score:",
		"## Additional checks",
		"## Repository details",
		"### Strong",
		"### Moderate",
		"### Weak",
		"## Rule reference",
		"## ⚠️ Skipped",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered scorecard missing %q", want)
		}
	}
}
