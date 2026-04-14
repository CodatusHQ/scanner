package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/CodatusHQ/scanner"
)

func main() {
	org := os.Getenv("CODATUS_ORG")
	token := os.Getenv("CODATUS_TOKEN")

	if org == "" {
		log.Fatal("CODATUS_ORG is required")
	}
	if token == "" {
		log.Fatal("CODATUS_TOKEN is required")
	}

	cfg := scanner.Config{Org: org, Token: token}

	results, err := scanner.Scan(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}

	scanned := 0
	skipped := 0
	for _, r := range results {
		if r.Skipped() {
			skipped++
		} else {
			scanned++
		}
	}
	log.Printf("scanned %d repos in org %s (%d skipped)", scanned, org, skipped)

	fmt.Println(scanner.GenerateReport(org, results))
}
