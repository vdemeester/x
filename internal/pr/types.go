package pr

import (
	"time"
)

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number          int       `json:"number"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	Author          string    `json:"author"`
	BaseRef         string    `json:"baseRefName"`   // Base branch (e.g., "master", "staging")
	Mergeable       string    `json:"mergeable"`     // MERGEABLE, CONFLICTING, UNKNOWN
	StatusState     string    `json:"statusState"`   // SUCCESS, FAILURE, PENDING, ERROR, EXPECTED
	Labels          []string  `json:"labels"`
	Files           []File    `json:"files"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// File represents a file changed in a PR
type File struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Match represents a single match between a PR and a dependency
type Match struct {
	Type       string `json:"type"`       // "package", "module", "title"
	Dependency string `json:"dependency"` // Which dependency matched
	FilePath   string `json:"file_path,omitempty"`
	Confidence string `json:"confidence"` // "high", "medium", "low"
}

// MatchResult represents the result of matching a PR against dependencies
type MatchResult struct {
	PR           PullRequest `json:"pr"`
	Score        int         `json:"score"` // 0-100
	Matches      []Match     `json:"matches"`
	TotalMatches int         `json:"total_matches"`
}

// HighestConfidence returns the highest confidence level among all matches
func (mr *MatchResult) HighestConfidence() string {
	highest := "low"
	for _, m := range mr.Matches {
		if m.Confidence == "high" {
			return "high"
		}
		if m.Confidence == "medium" && highest == "low" {
			highest = "medium"
		}
	}
	return highest
}

// HasBaseBranch returns true if the PR targets the specified base branch
func (pr *PullRequest) HasBaseBranch(baseBranch string) bool {
	return pr.BaseRef == baseBranch
}

// HasConflicts returns true if the PR has merge conflicts
func (pr *PullRequest) HasConflicts() bool {
	return pr.Mergeable == "CONFLICTING"
}

// HasBuildFailure returns true if the PR has a failed status check
func (pr *PullRequest) HasBuildFailure() bool {
	return pr.StatusState == "FAILURE" || pr.StatusState == "ERROR"
}

// NeedsAttention returns true if the PR has conflicts or build failures
func (pr *PullRequest) NeedsAttention() bool {
	return pr.HasConflicts() || pr.HasBuildFailure()
}

// FilterByBaseBranch filters PRs to only those targeting the specified base branch
func FilterByBaseBranch(prs []PullRequest, baseBranch string) []PullRequest {
	if baseBranch == "" {
		return prs
	}

	filtered := make([]PullRequest, 0, len(prs))
	for _, pr := range prs {
		if pr.HasBaseBranch(baseBranch) {
			filtered = append(filtered, pr)
		}
	}
	return filtered
}
