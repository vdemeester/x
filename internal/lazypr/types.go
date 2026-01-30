// Package lazypr provides types and utilities for the lazypr TUI.
package lazypr

import (
	"strings"
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

// Comment represents a discussion comment on a PR.
type Comment struct {
	Author string
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
	Comments []Comment

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
	return pr.EffectiveStatus() == "FAILURE" || pr.EffectiveStatus() == "ERROR"
}

// EffectiveStatus computes the status from individual checks.
// This fixes GitHub's rollup which shows FAILURE even if some checks are just pending.
func (pr *PRDetail) EffectiveStatus() string {
	if len(pr.Checks) == 0 {
		return pr.StatusState // Fall back to GitHub's state if no checks
	}

	hasFailure := false
	hasPending := false
	hasSuccess := false

	for _, check := range pr.Checks {
		name := strings.ToLower(check.Name)
		conclusion := strings.ToLower(check.Conclusion)
		status := strings.ToLower(check.Status)

		// Skip non-CI status contexts (tide is Prow's merge status, not a real check)
		if name == "tide" {
			continue
		}

		switch conclusion {
		case "failure", "timed_out", "action_required":
			hasFailure = true
		case "success":
			hasSuccess = true
		case "error":
			// "error" from StatusContext often means "not applicable" (like tide)
			// Only count it as failure if it's a real CI check (has "completed" status)
			// and the name suggests it's a real test/build
			if status == "completed" && !isNonCIContext(name) {
				hasFailure = true
			}
		case "skipped", "cancelled", "neutral":
			// These don't affect the overall status
		case "pending":
			hasPending = true
		case "":
			// No conclusion yet - check the status
			if status == "in_progress" || status == "queued" || status == "pending" {
				hasPending = true
			}
		}
	}

	if hasFailure {
		return "FAILURE"
	}
	if hasPending {
		return "PENDING"
	}
	if hasSuccess {
		return "SUCCESS"
	}
	return pr.StatusState // Fall back
}

// isNonCIContext returns true for status contexts that aren't real CI checks.
func isNonCIContext(name string) bool {
	nonCI := []string{"tide", "easycla", "cla", "merge", "hold"}
	for _, n := range nonCI {
		if strings.Contains(name, n) {
			return true
		}
	}
	return false
}

// NeedsAttention returns true if the PR has conflicts or build failures.
func (pr *PRDetail) NeedsAttention() bool {
	return pr.HasConflicts() || pr.HasBuildFailure()
}

// CI/Status icon constants (matching lazyworktree style)
const (
	IconSuccess   = "✓"
	IconFailure   = "⊗"
	IconPending   = "◷"
	IconSkipped   = "○"
	IconCancelled = "⊘"
	IconMerged    = "⊕"
	IconClosed    = "⊖"
	IconConflict  = "!"
	IconUnknown   = "○"
)

// StatusIcon returns an icon representing the overall PR status.
func (pr *PRDetail) StatusIcon() string {
	// For merged/closed PRs, show state icon
	if pr.State == "MERGED" {
		return IconMerged
	}
	if pr.State == "CLOSED" {
		return IconClosed
	}
	if pr.HasConflicts() {
		return IconConflict
	}
	switch pr.EffectiveStatus() {
	case "SUCCESS":
		return IconSuccess
	case "FAILURE", "ERROR":
		return IconFailure
	case "PENDING":
		return IconPending
	default:
		return IconUnknown
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
		return IconMerged
	}
	if pr.State == "CLOSED" {
		return IconClosed
	}
	switch pr.Mergeable {
	case "MERGEABLE":
		return IconSuccess
	case "CONFLICTING":
		return IconFailure
	default:
		return IconUnknown
	}
}

// CheckIcon returns the appropriate icon for a CI check conclusion.
func CheckIcon(conclusion, status string) string {
	switch conclusion {
	case "success", "SUCCESS":
		return IconSuccess
	case "failure", "FAILURE":
		return IconFailure
	case "skipped", "SKIPPED":
		return IconSkipped
	case "cancelled", "CANCELLED":
		return IconCancelled
	case "":
		// No conclusion yet, check status
		if status == "in_progress" || status == "IN_PROGRESS" || status == "queued" || status == "QUEUED" {
			return IconPending
		}
		return IconUnknown
	default:
		return IconUnknown
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
