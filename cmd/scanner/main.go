package main

import (
	"context"
	"log"
	"os"

	"github.com/CodatusHQ/scanner"
)

func main() {
	org := os.Getenv("CODATUS_ORG")
	token := os.Getenv("CODATUS_TOKEN")
	reportRepo := os.Getenv("CODATUS_REPORT_REPO") // repo name only, org is inferred from CODATUS_ORG

	if org == "" {
		log.Fatal("CODATUS_ORG is required")
	}
	if token == "" {
		log.Fatal("CODATUS_TOKEN is required")
	}
	if reportRepo == "" {
		log.Fatal("CODATUS_REPORT_REPO is required")
	}

	cfg := scanner.Config{
		Org:        org,
		Token:      token,
		ReportRepo: reportRepo,
	}

	if err := scanner.Run(context.Background(), cfg); err != nil {
		log.Fatal(err)
	}
}
