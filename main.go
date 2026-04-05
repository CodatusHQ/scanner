package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

// ScanConfig holds the configuration needed to run a scan.
type ScanConfig struct {
	Org        string
	Token      string
	ReportRepo string
}

func main() {
	cfg := ScanConfig{
		Org:        os.Getenv("CODATUS_ORG"),
		Token:      os.Getenv("CODATUS_TOKEN"),
		ReportRepo: os.Getenv("CODATUS_REPORT_REPO"),
	}

	if cfg.Org == "" {
		log.Fatal("CODATUS_ORG is required")
	}
	if cfg.Token == "" {
		log.Fatal("CODATUS_TOKEN is required")
	}
	if cfg.ReportRepo == "" {
		log.Fatal("CODATUS_REPORT_REPO is required")
	}

	ctx := context.Background()

	// TODO: replace with real GitHubClient implementation
	_ = ctx
	_ = cfg
	fmt.Println("codatus scanner — not yet wired up")
}
