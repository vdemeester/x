package pr

import (
	"time"
)

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Author    string    `json:"author"`
	Labels    []string  `json:"labels"`
	Files     []File    `json:"files"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
