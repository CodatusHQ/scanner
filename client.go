package main

import "context"

// Repo represents a GitHub repository with the fields the scanner needs.
type Repo struct {
	Name          string
	Description   string
	DefaultBranch string
	Archived      bool
}

// GitHubClient is the interface for all GitHub API interactions.
// The scanner depends only on this interface, making it testable via mocks.
type GitHubClient interface {
	ListRepos(ctx context.Context, org string) ([]Repo, error)
}
