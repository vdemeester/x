// Package lazypr provides types and utilities for the lazypr TUI.
package lazypr

import (
	"time"
)

// PRRef represents a reference to a GitHub pull request.
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// Check represents a CI check or status.
type Check struct {
	Name       string
	Status     string // queued, in_progress, completed
	Conclusion string // success, failure, neutral, cancelled, skipped, timed_out, action_required
	URL        string
	StartedAt  time.Time
	Duration   time.Duration
}

// Commit represents a commit in a PR.
type Commit struct {
	SHA     string
	Message string
	Author  string
}

// File represents a file changed in a PR.
type File struct {
	Path      string
	Additions int
	Deletions int
	Status    string // added, removed, modified, renamed, copied
}

// Review represents a review on a PR.
type Review struct {
	Author string
	State  string // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING, DISMISSED
	Body   string
}

// PRDetail contains comprehensive details about a pull request.
type PRDetail struct {
	// Basic info
	Number    int
	Title     string
	Author    string
	State     string // OPEN, CLOSED, MERGED
	Mergeable string // MERGEABLE, CONFLICTING, UNKNOWN
	URL       string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Repository context
	Owner   string
	Repo    string
	BaseRef string
	HeadRef string

	// CI Status
	StatusState string // SUCCESS, FAILURE, PENDING, ERROR
	Checks      []Check

	// Content
	Commits  []Commit
	Files    []File
	Labels   []string
	Reviews  []Review
	Comments int

	// Review summary
	Approvals       int
	ChangesRequired int
}

// HasConflicts returns true if the PR has merge conflicts.
func (pr *PRDetail) HasConflicts() bool {
	return pr.Mergeable == "CONFLICTING"
}

// HasBuildFailure returns true if the PR has a failed status check.
func (pr *PRDetail) HasBuildFailure() bool {
	return pr.StatusState == "FAILURE" || pr.StatusState == "ERROR"
}

// NeedsAttention returns true if the PR has conflicts or build failures.
func (pr *PRDetail) NeedsAttention() bool {
	return pr.HasConflicts() || pr.HasBuildFailure()
}

// StatusIcon returns an icon representing the overall PR status.
func (pr *PRDetail) StatusIcon() string {
	// For merged/closed PRs, show state icon
	if pr.State == "MERGED" {
		return "⊕"
	}
	if pr.State == "CLOSED" {
		return "⊖"
	}
	if pr.HasConflicts() {
		return "!"
	}
	switch pr.StatusState {
	case "SUCCESS":
		return "✓"
	case "FAILURE", "ERROR":
		return "✗"
	case "PENDING":
		return "●"
	default:
		return "○"
	}
}

// IsMerged returns true if the PR has been merged.
func (pr *PRDetail) IsMerged() bool {
	return pr.State == "MERGED"
}

// IsClosed returns true if the PR is closed (not merged).
func (pr *PRDetail) IsClosed() bool {
	return pr.State == "CLOSED"
}

// IsOpen returns true if the PR is open.
func (pr *PRDetail) IsOpen() bool {
	return pr.State == "OPEN"
}

// MergeableIcon returns an icon representing merge status.
func (pr *PRDetail) MergeableIcon() string {
	// For merged PRs, show merged icon
	if pr.State == "MERGED" {
		return "⊕"
	}
	if pr.State == "CLOSED" {
		return "⊖"
	}
	switch pr.Mergeable {
	case "MERGEABLE":
		return "✓"
	case "CONFLICTING":
		return "✗"
	default:
		return "○"
	}
}

// MergeableText returns a human-readable merge status.
func (pr *PRDetail) MergeableText() string {
	if pr.State == "MERGED" {
		return "MERGED"
	}
	if pr.State == "CLOSED" {
		return "CLOSED"
	}
	if pr.Mergeable == "UNKNOWN" {
		return "CHECKING..."
	}
	return pr.Mergeable
}
