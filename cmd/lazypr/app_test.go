package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"go.sbr.pm/x/internal/lazypr"
)

func TestUpdateInputMode_SubmitKeys(t *testing.T) {
	submitKeys := []string{"ctrl+d", "ctrl+s"}

	for _, key := range submitKeys {
		t.Run(key, func(t *testing.T) {
			m := Model{
				inputMode:   true,
				inputAction: "approve",
				inputPRs: []lazypr.PRDetail{
					{Owner: "test", Repo: "repo", Number: 1},
				},
				inputModel: textarea.New(),
			}
			m.inputModel.SetValue("test comment")

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			// Parse the key string to get proper KeyMsg
			switch key {
			case "ctrl+d":
				msg = tea.KeyMsg{Type: tea.KeyCtrlD}
			case "ctrl+s":
				msg = tea.KeyMsg{Type: tea.KeyCtrlS}
			}

			newModel, cmd := m.updateInputMode(msg)
			model := newModel.(Model)

			if model.inputMode {
				t.Errorf("inputMode should be false after %s", key)
			}
			if model.inputPRs != nil {
				t.Errorf("inputPRs should be nil after %s", key)
			}
			if cmd == nil {
				t.Errorf("cmd should not be nil after %s (should return approve command)", key)
			}
		})
	}
}

func TestUpdateInputMode_CtrlJAddsNewline(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "comment",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	m.inputModel.Focus()
	m.inputModel.SetValue("line1")
	// Move cursor to end
	m.inputModel.CursorEnd()

	msg := tea.KeyMsg{Type: tea.KeyCtrlJ}
	newModel, _ := m.updateInputMode(msg)
	model := newModel.(Model)

	// Input mode should still be active
	if !model.inputMode {
		t.Error("inputMode should still be true after Ctrl+J (adds newline)")
	}

	// Value should contain a newline
	if !strings.Contains(model.inputModel.Value(), "\n") {
		t.Errorf("expected newline in value after Ctrl+J, got %q", model.inputModel.Value())
	}
}

func TestUpdateInputMode_Escape(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "comment",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	m.inputModel.SetValue("test comment")

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := m.updateInputMode(msg)
	model := newModel.(Model)

	if model.inputMode {
		t.Error("inputMode should be false after Esc")
	}
	if model.inputPRs != nil {
		t.Error("inputPRs should be nil after Esc")
	}
	if cmd != nil {
		t.Error("cmd should be nil after Esc (cancelled)")
	}
}

func TestUpdateInputMode_CommentRequiresMessage(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "comment",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	// Leave inputModel empty

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	newModel, cmd := m.updateInputMode(msg)
	model := newModel.(Model)

	if model.statusMsg != "Comment cannot be empty" {
		t.Errorf("expected 'Comment cannot be empty' status, got %q", model.statusMsg)
	}
	if cmd != nil {
		t.Error("cmd should be nil when comment is empty")
	}
}

func TestUpdateInputMode_ChangesRequiresMessage(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "changes",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	// Leave inputModel empty

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	newModel, cmd := m.updateInputMode(msg)
	model := newModel.(Model)

	if model.statusMsg != "Reason cannot be empty" {
		t.Errorf("expected 'Reason cannot be empty' status, got %q", model.statusMsg)
	}
	if cmd != nil {
		t.Error("cmd should be nil when reason is empty")
	}
}

func TestUpdateInputMode_ApproveAllowsEmptyComment(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "approve",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	// Leave inputModel empty - should still work for approve

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	newModel, cmd := m.updateInputMode(msg)
	model := newModel.(Model)

	if model.inputMode {
		t.Error("inputMode should be false after approve")
	}
	if cmd == nil {
		t.Error("cmd should not be nil - approve should work with empty comment")
	}
}

func TestRenderInputModal_ActionTitles(t *testing.T) {
	tests := []struct {
		action        string
		expectedTitle string
		expectedHelp  string
	}{
		{"approve", "Approve PR", "Ctrl+D to approve"},
		{"comment", "Add Comment", "Ctrl+D to submit"},
		{"changes", "Request Changes", "Ctrl+D to submit"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			m := Model{
				inputMode:   true,
				inputAction: tt.action,
				inputPRs: []lazypr.PRDetail{
					{Owner: "test", Repo: "repo", Number: 1},
				},
				prs: []lazypr.PRDetail{
					{Owner: "test", Repo: "repo", Number: 1},
				},
				inputModel: textarea.New(),
				cursor:     0,
				styles:     NewStyles(DefaultTheme()),
			}

			rendered := m.renderInputModal()

			if !strings.Contains(rendered, tt.expectedTitle) {
				t.Errorf("expected title %q in output, got:\n%s", tt.expectedTitle, rendered)
			}
			if !strings.Contains(rendered, tt.expectedHelp) {
				t.Errorf("expected help text containing %q in output", tt.expectedHelp)
			}
		})
	}
}

func TestRenderInputModal_MultiplePRs(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "approve",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
			{Owner: "test", Repo: "repo", Number: 2},
			{Owner: "test", Repo: "repo", Number: 3},
		},
		prs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
			{Owner: "test", Repo: "repo", Number: 2},
			{Owner: "test", Repo: "repo", Number: 3},
		},
		inputModel: textarea.New(),
		cursor:     0,
		styles:     NewStyles(DefaultTheme()),
	}

	rendered := m.renderInputModal()

	if !strings.Contains(rendered, "(3 PRs)") {
		t.Errorf("expected '(3 PRs)' in title for multiple PRs, got:\n%s", rendered)
	}
}

func TestUpdateInputMode_EnterAddsNewline(t *testing.T) {
	m := Model{
		inputMode:   true,
		inputAction: "comment",
		inputPRs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		inputModel: textarea.New(),
	}
	m.inputModel.Focus()
	m.inputModel.SetValue("line1")

	// Send Enter key - should be passed to textarea (adds newline)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.updateInputMode(msg)
	model := newModel.(Model)

	// Input mode should still be active
	if !model.inputMode {
		t.Error("inputMode should still be true after Enter (Enter adds newlines)")
	}
}

func TestApproveKeyShowsInputModal(t *testing.T) {
	m := Model{
		prs: []lazypr.PRDetail{
			{Owner: "test", Repo: "repo", Number: 1},
		},
		cursor:   0,
		selected: make(map[int]bool),
		styles:   NewStyles(DefaultTheme()),
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	newModel, _ := m.Update(msg)
	model := newModel.(Model)

	if !model.inputMode {
		t.Error("pressing 'a' should activate input mode")
	}
	if model.inputAction != "approve" {
		t.Errorf("inputAction should be 'approve', got %q", model.inputAction)
	}
	if len(model.inputPRs) != 1 {
		t.Errorf("inputPRs should have 1 PR, got %d", len(model.inputPRs))
	}
}
