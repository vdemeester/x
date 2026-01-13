package pr

import (
	"context"
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
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Author      ghAuthor  `json:"author"`
	BaseRefName string    `json:"baseRefName"`
	Labels      []ghLabel `json:"labels"`
	Files       []ghFile  `json:"files"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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

// FetchNixpkgsPRs fetches open PRs from NixOS/nixpkgs.
// PRs are returned sorted by creation date descending (newest first).
func (f *Fetcher) FetchNixpkgsPRs(limit int) ([]PullRequest, error) {
	// Use context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "pr", "list",
		"--repo", "NixOS/nixpkgs",
		"--state", "open",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "number,title,url,author,baseRefName,labels,files,createdAt,updatedAt",
		"--order", "created",
		"--sort", "created")

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
			BaseRef:   ghpr.BaseRefName,
			Labels:    labels,
			Files:     files,
			CreatedAt: ghpr.CreatedAt,
			UpdatedAt: ghpr.UpdatedAt,
		}
	}

	return prs, nil
}

// FetchNixpkgsPRsWithCursor fetches PRs using cursor-based pagination.
// It automatically batches requests to respect GitHub's 100-record limit per request.
// Returns the PRs, the cursor for the next page, and any error.
//
// This allows fetching additional PRs beyond what's cached without refetching everything.
// For example, if cache has 300 PRs and you request 500, this fetches only the additional 200.
func (f *Fetcher) FetchNixpkgsPRsWithCursor(limit int, afterCursor string) ([]PullRequest, string, error) {
	const maxPerRequest = 100

	var allPRs []PullRequest
	currentCursor := afterCursor
	remaining := limit

	for remaining > 0 {
		batchSize := remaining
		if batchSize > maxPerRequest {
			batchSize = maxPerRequest
		}

		prs, cursor, err := f.fetchPRBatch(batchSize, currentCursor)
		if err != nil {
			return allPRs, currentCursor, err
		}

		allPRs = append(allPRs, prs...)
		currentCursor = cursor
		remaining -= len(prs)

		// If we got fewer PRs than requested, we've reached the end
		if len(prs) < batchSize {
			break
		}
	}

	return allPRs, currentCursor, nil
}

// fetchPRBatch fetches a single batch of PRs (max 100).
func (f *Fetcher) fetchPRBatch(limit int, afterCursor string) ([]PullRequest, string, error) {
	// Use context with timeout to prevent hanging (30s per batch)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build GraphQL query
	query := `query($limit: Int!, $after: String) {
		repository(owner: "NixOS", name: "nixpkgs") {
			pullRequests(first: $limit, after: $after, states: OPEN, orderBy: {field: CREATED_AT, direction: DESC}) {
				pageInfo {
					hasNextPage
					endCursor
				}
				nodes {
					number
					title
					url
					author {
						login
					}
					baseRefName
					labels(first: 10) {
						nodes {
							name
						}
					}
					files(first: 100) {
						nodes {
							path
							additions
							deletions
						}
					}
					createdAt
					updatedAt
				}
			}
		}
	}`

	cmd := exec.CommandContext(ctx, "gh", "api", "graphql", "-f", "query="+query,
		"-F", fmt.Sprintf("limit=%d", limit))
	if afterCursor != "" {
		cmd.Args = append(cmd.Args, "-f", "after="+afterCursor)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, "", fmt.Errorf("gh API failed: %s", string(exitErr.Stderr))
		}
		return nil, "", fmt.Errorf("failed to run gh API: %w", err)
	}

	// Parse GraphQL response
	var response struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						Number int    `json:"number"`
						Title  string `json:"title"`
						URL    string `json:"url"`
						Author struct {
							Login string `json:"login"`
						} `json:"author"`
						BaseRefName string `json:"baseRefName"`
						Labels      struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						Files struct {
							Nodes []struct {
								Path      string `json:"path"`
								Additions int    `json:"additions"`
								Deletions int    `json:"deletions"`
							} `json:"nodes"`
						} `json:"files"`
						CreatedAt time.Time `json:"createdAt"`
						UpdatedAt time.Time `json:"updatedAt"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, "", fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Convert to our PR type
	prs := make([]PullRequest, len(response.Data.Repository.PullRequests.Nodes))
	for i, node := range response.Data.Repository.PullRequests.Nodes {
		labels := make([]string, len(node.Labels.Nodes))
		for j, l := range node.Labels.Nodes {
			labels[j] = l.Name
		}

		files := make([]File, len(node.Files.Nodes))
		for j, f := range node.Files.Nodes {
			files[j] = File{
				Path:      f.Path,
				Additions: f.Additions,
				Deletions: f.Deletions,
			}
		}

		prs[i] = PullRequest{
			Number:    node.Number,
			Title:     node.Title,
			URL:       node.URL,
			Author:    node.Author.Login,
			BaseRef:   node.BaseRefName,
			Labels:    labels,
			Files:     files,
			CreatedAt: node.CreatedAt,
			UpdatedAt: node.UpdatedAt,
		}
	}

	return prs, response.Data.Repository.PullRequests.PageInfo.EndCursor, nil
}
