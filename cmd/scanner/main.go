package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/CodatusHQ/scanner"
)

func main() {
	admin := flag.Bool("admin", false, "the token has admin access on every target repo (e.g. you are admin of CODATUS_ORG, or this is an installation token from the Codatus GitHub App). When false, admin-only rules are skipped.")
	flag.Parse()

	org := os.Getenv("CODATUS_ORG")
	token := os.Getenv("CODATUS_TOKEN")

	if org == "" {
		log.Fatal("CODATUS_ORG is required")
	}
	if token == "" {
		log.Fatal("CODATUS_TOKEN is required")
	}

	auth := scanner.PATAuth{Token: token, Name: org}

	sr, err := scanner.Scan(context.Background(), auth, scanner.WithAdmin(*admin))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("org=%s total=%d forks_excluded=%d archived_excluded=%d scanned=%d skipped=%d",
		sr.Org, sr.TotalRepos, sr.ForksExcluded, sr.ArchivedExcluded, len(sr.Results), len(sr.Skipped))

	fmt.Println(scanner.GenerateReport(sr))
}
