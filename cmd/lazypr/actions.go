package main

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"go.sbr.pm/x/internal/lazypr"
)

// Action represents a PR action result.
type actionResult struct {
	success bool
	message string
}

// approvePRs approves one or more PRs using gh CLI (without comment).
func approvePRs(prs []lazypr.PRDetail) tea.Cmd {
	return approvePRsWithComment(prs, "")
}

// approvePRsWithComment approves one or more PRs with an optional comment.
func approvePRsWithComment(prs []lazypr.PRDetail, comment string) tea.Cmd {
	return func() tea.Msg {
		var approved []int
		var failed []string

		for _, pr := range prs {
			repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
			prNum := fmt.Sprintf("%d", pr.Number)

			var cmd *exec.Cmd
			if comment != "" {
				cmd = exec.Command("gh", "pr", "review", prNum, "-R", repo, "--approve", "--body", comment)
			} else {
				cmd = exec.Command("gh", "pr", "review", prNum, "-R", repo, "--approve")
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				failed = append(failed, fmt.Sprintf("#%d: %v", pr.Number, string(output)))
			} else {
				approved = append(approved, pr.Number)
			}
		}

		if len(failed) > 0 {
			return actionResult{success: false, message: fmt.Sprintf("Failed: %s", strings.Join(failed, ", "))}
		}
		if len(approved) == 1 {
			return actionResult{success: true, message: fmt.Sprintf("Approved PR #%d", approved[0])}
		}
		return actionResult{success: true, message: fmt.Sprintf("Approved %d PRs", len(approved))}
	}
}

// commentPRs adds a comment to one or more PRs.
func commentPRs(prs []lazypr.PRDetail, body string) tea.Cmd {
	return func() tea.Msg {
		var succeeded int
		var failed []string

		for _, pr := range prs {
			repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
			prNum := fmt.Sprintf("%d", pr.Number)

			cmd := exec.Command("gh", "pr", "comment", prNum, "-R", repo, "--body", body)
			output, err := cmd.CombinedOutput()
			if err != nil {
				failed = append(failed, fmt.Sprintf("#%d: %v", pr.Number, string(output)))
			} else {
				succeeded++
			}
		}

		if len(failed) > 0 {
			return actionResult{success: false, message: fmt.Sprintf("Failed: %s", strings.Join(failed, ", "))}
		}
		if succeeded == 1 {
			return actionResult{success: true, message: "Comment added"}
		}
		return actionResult{success: true, message: fmt.Sprintf("Commented on %d PRs", succeeded)}
	}
}

// requestChangesPRs requests changes on one or more PRs.
func requestChangesPRs(prs []lazypr.PRDetail, body string) tea.Cmd {
	return func() tea.Msg {
		var succeeded int
		var failed []string

		for _, pr := range prs {
			repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
			prNum := fmt.Sprintf("%d", pr.Number)

			cmd := exec.Command("gh", "pr", "review", prNum, "-R", repo, "--request-changes", "--body", body)
			output, err := cmd.CombinedOutput()
			if err != nil {
				failed = append(failed, fmt.Sprintf("#%d: %v", pr.Number, string(output)))
			} else {
				succeeded++
			}
		}

		if len(failed) > 0 {
			return actionResult{success: false, message: fmt.Sprintf("Failed: %s", strings.Join(failed, ", "))}
		}
		if succeeded == 1 {
			return actionResult{success: true, message: "Changes requested"}
		}
		return actionResult{success: true, message: fmt.Sprintf("Requested changes on %d PRs", succeeded)}
	}
}

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		// Try different clipboard commands
		var cmd *exec.Cmd
		switch {
		case commandExists("wl-copy"):
			cmd = exec.Command("wl-copy", text)
		case commandExists("xclip"):
			cmd = exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
		case commandExists("pbcopy"):
			cmd = exec.Command("pbcopy")
			cmd.Stdin = strings.NewReader(text)
		default:
			return actionResult{success: false, message: "No clipboard command found"}
		}

		if err := cmd.Run(); err != nil {
			return actionResult{success: false, message: fmt.Sprintf("Failed to copy: %v", err)}
		}
		return actionResult{success: true, message: "URL copied to clipboard"}
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
