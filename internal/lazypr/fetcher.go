package lazypr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Fetcher fetches PR details from GitHub.
type Fetcher struct {
	timeout time.Duration
}

// NewFetcher creates a new PR fetcher.
func NewFetcher() *Fetcher {
	return &Fetcher{
		timeout: 30 * time.Second,
	}
}

// graphqlResponse represents the response from the GitHub GraphQL API.
type graphqlResponse struct {
	Data struct {
		Repository struct {
			PullRequest graphqlPR `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type graphqlPR struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	Mergeable   string    `json:"mergeable"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	BaseRefName string    `json:"baseRefName"`
	HeadRefName string    `json:"headRefName"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	Reviews struct {
		Nodes []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			State string `json:"state"`
			Body  string `json:"body"`
		} `json:"nodes"`
	} `json:"reviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				Oid             string `json:"oid"`
				MessageHeadline string `json:"messageHeadline"`
				Author          struct {
					Name string `json:"name"`
				} `json:"author"`
				StatusCheckRollup struct {
					State    string `json:"state"`
					Contexts struct {
						Nodes []struct {
							TypeName   string `json:"__typename"`
							Name       string `json:"name"`
							Context    string `json:"context"`
							State      string `json:"state"`
							Status     string `json:"status"`
							Conclusion string `json:"conclusion"`
							TargetURL  string `json:"targetUrl"`
							StartedAt  string `json:"startedAt"`
						} `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Files struct {
		Nodes []struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"nodes"`
	} `json:"files"`
}

// FetchPRDetail fetches detailed information about a PR.
func (f *Fetcher) FetchPRDetail(ref PRRef) (PRDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	query := `query($owner: String!, $repo: String!, $number: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequest(number: $number) {
				number
				title
				body
				state
				mergeable
				url
				createdAt
				updatedAt
				baseRefName
				headRefName
				author {
					login
				}
				labels(first: 20) {
					nodes {
						name
					}
				}
				comments {
					totalCount
				}
				reviews(first: 50) {
					nodes {
						author {
							login
						}
						state
						body
					}
				}
				commits(last: 50) {
					nodes {
						commit {
							oid
							messageHeadline
							author {
								name
							}
							statusCheckRollup {
								state
								contexts(first: 100) {
									nodes {
										__typename
										... on CheckRun {
											name
											status
											conclusion
											startedAt
										}
										... on StatusContext {
											context
											state
											targetUrl
										}
									}
								}
							}
						}
					}
				}
				files(first: 100) {
					nodes {
						path
						additions
						deletions
					}
				}
			}
		}
	}`

	cmd := exec.CommandContext(ctx, "gh", "api", "graphql",
		"-f", "query="+query,
		"-f", fmt.Sprintf("owner=%s", ref.Owner),
		"-f", fmt.Sprintf("repo=%s", ref.Repo),
		"-F", fmt.Sprintf("number=%d", ref.Number),
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return PRDetail{}, fmt.Errorf("gh API failed: %s", string(exitErr.Stderr))
		}
		return PRDetail{}, fmt.Errorf("failed to run gh CLI: %w", err)
	}

	var resp graphqlResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return PRDetail{}, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return PRDetail{}, fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	pr := resp.Data.Repository.PullRequest
	return f.convertPR(pr, ref), nil
}

// FetchPRDetails fetches details for multiple PRs.
func (f *Fetcher) FetchPRDetails(refs []PRRef) ([]PRDetail, error) {
	details := make([]PRDetail, 0, len(refs))
	for _, ref := range refs {
		detail, err := f.FetchPRDetail(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %w", ref.String(), err)
		}
		details = append(details, detail)
	}
	return details, nil
}

// FetchRepoPRs fetches open PRs from a repository.
func (f *Fetcher) FetchRepoPRs(repo RepoRef, limit int) ([]PRDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	query := `query($owner: String!, $repo: String!, $limit: Int!) {
		repository(owner: $owner, name: $repo) {
			pullRequests(first: $limit, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}) {
				nodes {
					number
					title
					body
					state
					mergeable
					url
					createdAt
					updatedAt
					baseRefName
					headRefName
					author {
						login
					}
					labels(first: 20) {
						nodes {
							name
						}
					}
					comments {
						totalCount
					}
					reviews(first: 50) {
						nodes {
							author {
								login
							}
							state
							body
						}
					}
					commits(last: 50) {
						nodes {
							commit {
								oid
								messageHeadline
								author {
									name
								}
								statusCheckRollup {
									state
									contexts(first: 100) {
										nodes {
											__typename
											... on CheckRun {
												name
												status
												conclusion
												startedAt
											}
											... on StatusContext {
												context
												state
												targetUrl
											}
										}
									}
								}
							}
						}
					}
					files(first: 100) {
						nodes {
							path
							additions
							deletions
						}
					}
				}
			}
		}
	}`

	cmd := exec.CommandContext(ctx, "gh", "api", "graphql",
		"-f", "query="+query,
		"-f", fmt.Sprintf("owner=%s", repo.Owner),
		"-f", fmt.Sprintf("repo=%s", repo.Repo),
		"-F", fmt.Sprintf("limit=%d", limit),
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh API failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run gh CLI: %w", err)
	}

	var resp struct {
		Data struct {
			Repository struct {
				PullRequests struct {
					Nodes []graphqlPR `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", resp.Errors[0].Message)
	}

	// Convert to PRDetail
	details := make([]PRDetail, 0, len(resp.Data.Repository.PullRequests.Nodes))
	for _, pr := range resp.Data.Repository.PullRequests.Nodes {
		ref := PRRef{Owner: repo.Owner, Repo: repo.Repo, Number: pr.Number}
		details = append(details, f.convertPR(pr, ref))
	}

	return details, nil
}

func (f *Fetcher) convertPR(pr graphqlPR, ref PRRef) PRDetail {
	// Extract labels
	labels := make([]string, len(pr.Labels.Nodes))
	for i, l := range pr.Labels.Nodes {
		labels[i] = l.Name
	}

	// Extract reviews
	reviews := make([]Review, len(pr.Reviews.Nodes))
	approvals := 0
	changesRequired := 0
	for i, r := range pr.Reviews.Nodes {
		reviews[i] = Review{
			Author: r.Author.Login,
			State:  r.State,
			Body:   r.Body,
		}
		if r.State == "APPROVED" {
			approvals++
		} else if r.State == "CHANGES_REQUESTED" {
			changesRequired++
		}
	}

	// Extract commits
	commits := make([]Commit, len(pr.Commits.Nodes))
	for i, c := range pr.Commits.Nodes {
		commits[i] = Commit{
			SHA:     c.Commit.Oid,
			Message: c.Commit.MessageHeadline,
			Author:  c.Commit.Author.Name,
		}
	}

	// Extract files
	files := make([]File, len(pr.Files.Nodes))
	for i, f := range pr.Files.Nodes {
		files[i] = File{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		}
	}

	// Extract status checks from the latest commit
	var statusState string
	var checks []Check
	if len(pr.Commits.Nodes) > 0 {
		lastCommit := pr.Commits.Nodes[len(pr.Commits.Nodes)-1].Commit
		statusState = lastCommit.StatusCheckRollup.State
		for _, ctx := range lastCommit.StatusCheckRollup.Contexts.Nodes {
			check := Check{}
			if ctx.TypeName == "CheckRun" {
				check.Name = ctx.Name
				check.Status = ctx.Status
				check.Conclusion = ctx.Conclusion
				if ctx.StartedAt != "" {
					if t, err := time.Parse(time.RFC3339, ctx.StartedAt); err == nil {
						check.StartedAt = t
					}
				}
			} else if ctx.TypeName == "StatusContext" {
				check.Name = ctx.Context
				check.Status = "completed"
				check.Conclusion = ctx.State
				check.URL = ctx.TargetURL
			}
			checks = append(checks, check)
		}
	}

	return PRDetail{
		Number:          pr.Number,
		Title:           pr.Title,
		Author:          pr.Author.Login,
		State:           pr.State,
		Mergeable:       pr.Mergeable,
		URL:             pr.URL,
		Body:            pr.Body,
		CreatedAt:       pr.CreatedAt,
		UpdatedAt:       pr.UpdatedAt,
		Owner:           ref.Owner,
		Repo:            ref.Repo,
		BaseRef:         pr.BaseRefName,
		HeadRef:         pr.HeadRefName,
		StatusState:     statusState,
		Checks:          checks,
		Commits:         commits,
		Files:           files,
		Labels:          labels,
		Reviews:         reviews,
		Comments:        pr.Comments.TotalCount,
		Approvals:       approvals,
		ChangesRequired: changesRequired,
	}
}
