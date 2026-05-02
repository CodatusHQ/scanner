// generate-sample renders samples.Fixture() through scanner.GenerateReport
// and writes the resulting Markdown to stdout (or to a file via --out).
//
// Downstream consumers regenerate on demand instead of consuming a committed
// artifact, so the Go fixture is the single source of truth. Typical use:
//
//	go run github.com/CodatusHQ/scanner/cmd/generate-sample > sample-scorecard.md
//
// Go consumers (e.g. the app's dev-seed) skip this binary entirely and call
// scanner.GenerateReport(samples.Fixture()) in process.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/CodatusHQ/scanner"
	"github.com/CodatusHQ/scanner/samples"
)

func main() {
	out := flag.String("out", "", "path to write the rendered scorecard (default stdout)")
	flag.Parse()

	md := scanner.GenerateReport(samples.Fixture())
	if *out == "" {
		fmt.Print(md)
		return
	}
	if err := os.WriteFile(*out, []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
}
