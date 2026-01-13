package pr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Fetcher fetches pull requests from GitHub
type Fetcher struct{}

// NewFetcher creates a new PR fetcher
func NewFetcher() *Fetcher {
	return &Fetcher{}
}

// ghPR represents a PR as returned by gh CLI
type ghPR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Author    ghAuthor  `json:"author"`
	Labels    []ghLabel `json:"labels"`
	Files     []ghFile  `json:"files"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// FetchNixpkgsPRs fetches open PRs from NixOS/nixpkgs
func (f *Fetcher) FetchNixpkgsPRs(limit int) ([]PullRequest, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--repo", "NixOS/nixpkgs",
		"--state", "open",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,url,author,labels,files,createdAt,updatedAt")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh CLI failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run gh CLI: %w", err)
	}

	var ghPRs []ghPR
	if err := json.Unmarshal(output, &ghPRs); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}

	// Convert to our PR type
	prs := make([]PullRequest, len(ghPRs))
	for i, ghpr := range ghPRs {
		labels := make([]string, len(ghpr.Labels))
		for j, l := range ghpr.Labels {
			labels[j] = l.Name
		}

		files := make([]File, len(ghpr.Files))
		for j, f := range ghpr.Files {
			files[j] = File{
				Path:      f.Path,
				Additions: f.Additions,
				Deletions: f.Deletions,
			}
		}

		prs[i] = PullRequest{
			Number:    ghpr.Number,
			Title:     ghpr.Title,
			URL:       ghpr.URL,
			Author:    ghpr.Author.Login,
			Labels:    labels,
			Files:     files,
			CreatedAt: ghpr.CreatedAt,
			UpdatedAt: ghpr.UpdatedAt,
		}
	}

	return prs, nil
}
