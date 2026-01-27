package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.sbr.pm/x/internal/lazypr"
)

// Action represents a PR action result.
type actionResult struct {
	success bool
	message string
}

// InputModel handles text input for comments.
type InputModel struct {
	textInput textinput.Model
	title     string
	action    string // "comment" or "changes"
	pr        lazypr.PRDetail
	styles    Styles
}

// NewInputModel creates a new input model.
func NewInputModel(title, action string, pr lazypr.PRDetail, styles Styles) InputModel {
	ti := textinput.New()
	ti.Placeholder = "Enter your message..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 60

	return InputModel{
		textInput: ti,
		title:     title,
		action:    action,
		pr:        pr,
		styles:    styles,
	}
}

// Init implements tea.Model.
func (m InputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.textInput.Value() != "" {
				return m, m.executeAction()
			}
		case "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m InputModel) executeAction() tea.Cmd {
	return func() tea.Msg {
		repo := fmt.Sprintf("%s/%s", m.pr.Owner, m.pr.Repo)
		prNum := fmt.Sprintf("%d", m.pr.Number)
		message := m.textInput.Value()

		var args []string
		switch m.action {
		case "comment":
			args = []string{"pr", "comment", prNum, "-R", repo, "--body", message}
		case "changes":
			args = []string{"pr", "review", prNum, "-R", repo, "--request-changes", "--body", message}
		}

		cmd := exec.Command("gh", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return actionResult{success: false, message: fmt.Sprintf("Error: %v\n%s", err, output)}
		}
		return actionResult{success: true, message: fmt.Sprintf("Action completed: %s", m.action)}
	}
}

// View implements tea.Model.
func (m InputModel) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(1, 2).
		Width(70)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true)

	content := fmt.Sprintf("%s\n\n%s\n\nPress Enter to submit, Esc to cancel",
		titleStyle.Render(m.title),
		m.textInput.View(),
	)

	return boxStyle.Render(content)
}

// approvePRs approves one or more PRs using gh CLI.
func approvePRs(prs []lazypr.PRDetail) tea.Cmd {
	return func() tea.Msg {
		var approved []int
		var failed []string

		for _, pr := range prs {
			repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
			prNum := fmt.Sprintf("%d", pr.Number)

			cmd := exec.Command("gh", "pr", "review", prNum, "-R", repo, "--approve")
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
