package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Fetcher fetches pull requests from GitHub
type Fetcher struct {
	rateLimiter *RateLimiter
}

// NewFetcher creates a new PR fetcher with default rate limiting
// Uses 100ms base delay, 1.5x exponential backoff, capped at 5s
func NewFetcher() *Fetcher {
	return &Fetcher{
		rateLimiter: NewRateLimiter(100*time.Millisecond, 1.5, 5*time.Second),
	}
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
			Number:      ghpr.Number,
			Title:       ghpr.Title,
			URL:         ghpr.URL,
			Author:      ghpr.Author.Login,
			BaseRef:     ghpr.BaseRefName,
			Mergeable:   "",       // Not available in simple gh pr list
			StatusState: "",       // Not available in simple gh pr list
			Labels:      labels,
			Files:       files,
			CreatedAt:   ghpr.CreatedAt,
			UpdatedAt:   ghpr.UpdatedAt,
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
//
// The baseBranch parameter filters PRs by target branch (empty string = no filter).
func (f *Fetcher) FetchNixpkgsPRsWithCursor(limit int, afterCursor string, baseBranch string) ([]PullRequest, string, error) {
	const maxPerRequest = 100

	var allPRs []PullRequest
	currentCursor := afterCursor
	remaining := limit

	batchNum := 0
	for remaining > 0 {
		batchNum++

		// Apply rate limiting with exponential backoff
		delay := f.rateLimiter.Wait()
		if delay > 0 {
			fmt.Fprintf(os.Stderr, "⏱️  Rate limiting: waiting %v before batch %d...\n", delay.Round(time.Millisecond), batchNum)
		}

		batchSize := remaining
		if batchSize > maxPerRequest {
			batchSize = maxPerRequest
		}

		prs, cursor, err := f.fetchPRBatchWithRetry(batchSize, currentCursor, baseBranch, 3)
		if err != nil {
			return allPRs, currentCursor, err
		}

		f.rateLimiter.recordRequest()

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

// fetchPRBatchWithRetry fetches a batch with retry logic for transient errors
func (f *Fetcher) fetchPRBatchWithRetry(limit int, afterCursor string, baseBranch string, maxRetries int) ([]PullRequest, string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		prs, cursor, err := f.fetchPRBatch(limit, afterCursor, baseBranch)
		if err == nil {
			return prs, cursor, nil
		}

		lastErr = err

		// Check if error is retryable (502, 503, 504, network issues)
		if !isRetryableError(err) {
			return nil, "", err
		}

		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			fmt.Fprintf(os.Stderr, "⚠️  GitHub API error (attempt %d/%d): %v\n", attempt, maxRetries, err)
			fmt.Fprintf(os.Stderr, "   Retrying in %v...\n", backoff)
			time.Sleep(backoff)
		}
	}

	return nil, "", fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// isRetryableError returns true for transient errors that should be retried
func isRetryableError(err error) bool {
	errStr := err.Error()
	// HTTP 502 Bad Gateway, 503 Service Unavailable, 504 Gateway Timeout
	return strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset")
}

// fetchPRBatch fetches a single batch of PRs (max 100).
func (f *Fetcher) fetchPRBatch(limit int, afterCursor string, baseBranch string) ([]PullRequest, string, error) {
	// Use context with timeout to prevent hanging (30s per batch)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build GraphQL query with optional base branch filter
	var query string
	if baseBranch != "" {
		query = `query($limit: Int!, $after: String, $baseRefName: String!) {
			repository(owner: "NixOS", name: "nixpkgs") {
				pullRequests(first: $limit, after: $after, states: OPEN, baseRefName: $baseRefName, orderBy: {field: CREATED_AT, direction: DESC}) {`
	} else {
		query = `query($limit: Int!, $after: String) {
			repository(owner: "NixOS", name: "nixpkgs") {
				pullRequests(first: $limit, after: $after, states: OPEN, orderBy: {field: CREATED_AT, direction: DESC}) {`
	}
	query += `

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
					mergeable
					commits(last: 1) {
						nodes {
							commit {
								statusCheckRollup {
									state
								}
							}
						}
					}
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
	if baseBranch != "" {
		cmd.Args = append(cmd.Args, "-f", "baseRefName="+baseBranch)
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
						Number      int    `json:"number"`
						Title       string `json:"title"`
						URL         string `json:"url"`
						Author      struct {
							Login string `json:"login"`
						} `json:"author"`
						BaseRefName string `json:"baseRefName"`
						Mergeable   string `json:"mergeable"`
						Commits     struct {
							Nodes []struct {
								Commit struct {
									StatusCheckRollup struct {
										State string `json:"state"`
									} `json:"statusCheckRollup"`
								} `json:"commit"`
							} `json:"nodes"`
						} `json:"commits"`
						Labels struct {
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

		// Extract status check state from the latest commit
		statusState := ""
		if len(node.Commits.Nodes) > 0 && node.Commits.Nodes[0].Commit.StatusCheckRollup.State != "" {
			statusState = node.Commits.Nodes[0].Commit.StatusCheckRollup.State
		}

		prs[i] = PullRequest{
			Number:      node.Number,
			Title:       node.Title,
			URL:         node.URL,
			Author:      node.Author.Login,
			BaseRef:     node.BaseRefName,
			Mergeable:   node.Mergeable,
			StatusState: statusState,
			Labels:      labels,
			Files:       files,
			CreatedAt:   node.CreatedAt,
			UpdatedAt:   node.UpdatedAt,
		}
	}

	return prs, response.Data.Repository.PullRequests.PageInfo.EndCursor, nil
}
