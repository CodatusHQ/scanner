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

	auth := scanner.PATAuth{Token: token, Name: org}

	sr, err := scanner.Scan(context.Background(), auth)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("org=%s total=%d forks_excluded=%d archived_excluded=%d scanned=%d skipped=%d",
		sr.Org, sr.TotalRepos, sr.ForksExcluded, sr.ArchivedExcluded, len(sr.Results), len(sr.Skipped))

	fmt.Println(scanner.GenerateReport(sr))
}
