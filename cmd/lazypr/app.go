package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go.sbr.pm/x/internal/lazypr"
)

const (
	paneList   = 0
	paneDetail = 1

	// Golden ratio split: 38% list, 62% detail
	listPaneRatio   = 0.38
	detailPaneRatio = 0.62
)

// Model is the main BubbleTea model for lazypr.
type Model struct {
	// Data
	prs       []lazypr.PRDetail
	refs      []lazypr.PRRef
	repo      *lazypr.RepoRef       // For loading all PRs from a repo
	repoLimit int                   // Limit when loading from repo
	filter    lazypr.FilterOptions  // Filter options for repo loading
	cursor    int
	selected  map[int]bool // Selected PR indices for multi-select

	// Layout
	width       int
	height      int
	focusedPane int

	// Detail pane viewport for scrolling
	detailViewport viewport.Model

	// List pane scrolling
	listOffset int // First visible item index in list

	// State
	loading    bool
	err        error
	ready      bool
	statusMsg  string // Temporary status message
	statusTime int    // Ticks until status clears

	// Input modal state
	inputMode    bool              // Whether input modal is active
	inputAction  string            // "comment", "changes", or "approve"
	inputModel   textarea.Model    // Multi-line input field for modal
	inputPRs     []lazypr.PRDetail // PRs to act on when input is submitted

	// Help screen
	showHelp bool

	// List filter
	filterMode  bool            // Whether filter input is active
	filterInput textinput.Model // Filter text input
	filterText  string          // Current filter text (applied)

	// Checks modal
	showChecksModal    bool
	checksModalCursor  int
	checksModalChecks  []lazypr.Check // Filtered checks to show

	// Actions modal
	showActionsModal   bool
	actionsModalCursor int
	config             *lazypr.Config // User configuration with custom actions

	// Styles
	styles Styles
}

// prLoadedMsg is sent when PRs have been loaded.
type prLoadedMsg struct {
	prs []lazypr.PRDetail
}

// prErrorMsg is sent when loading fails.
type prErrorMsg struct {
	err error
}

// execDoneMsg is sent when an external process completes.
type execDoneMsg struct{}

// NewModel creates a new model with the given PR references.
func NewModel(refs []lazypr.PRRef) Model {
	cfg, _ := lazypr.LoadConfig(lazypr.DefaultConfigPath())
	return Model{
		refs:        refs,
		cursor:      0,
		selected:    make(map[int]bool),
		focusedPane: paneList,
		loading:     true,
		styles:      NewStyles(DefaultTheme()),
		config:      cfg,
	}
}

// NewRepoModel creates a new model that loads PRs from a repository.
func NewRepoModel(repo lazypr.RepoRef, limit int) Model {
	return NewRepoModelWithFilter(repo, limit, lazypr.FilterOptions{})
}

// NewRepoModelWithFilter creates a new model with filter options.
func NewRepoModelWithFilter(repo lazypr.RepoRef, limit int, filter lazypr.FilterOptions) Model {
	cfg, _ := lazypr.LoadConfig(lazypr.DefaultConfigPath())
	return Model{
		repo:        &repo,
		repoLimit:   limit,
		filter:      filter,
		cursor:      0,
		selected:    make(map[int]bool),
		focusedPane: paneList,
		loading:     true,
		styles:      NewStyles(DefaultTheme()),
		config:      cfg,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadPRs(),
		tea.EnterAltScreen,
	)
}

// loadPRs fetches PR details.
func (m Model) loadPRs() tea.Cmd {
	return func() tea.Msg {
		fetcher := lazypr.NewFetcher()

		var prs []lazypr.PRDetail
		var err error

		if m.repo != nil {
			// Load PRs from repository with filter
			prs, err = fetcher.FetchRepoPRsWithFilter(*m.repo, m.repoLimit, m.filter)
		} else {
			// Load specific PRs
			prs, err = fetcher.FetchPRDetails(m.refs)
		}

		if err != nil {
			return prErrorMsg{err: err}
		}
		return prLoadedMsg{prs: prs}
	}
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle help screen
	if m.showHelp {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "esc", "?":
				m.showHelp = false
				return m, nil
			}
		}
		return m, nil
	}

	// Handle filter mode
	if m.filterMode {
		return m.updateFilterMode(msg)
	}

	// Handle input modal mode
	if m.inputMode {
		return m.updateInputMode(msg)
	}

	// Handle checks modal
	if m.showChecksModal {
		return m.updateChecksModal(msg)
	}

	// Handle actions modal
	if m.showActionsModal {
		return m.updateActionsModal(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize or resize viewport
		detailWidth, detailHeight := m.detailPaneDimensions()
		if !m.ready {
			// First time: create viewport
			m.detailViewport = viewport.New(detailWidth, detailHeight)
			m.ready = true
		} else {
			// Resize: update dimensions
			m.detailViewport.Width = detailWidth
			m.detailViewport.Height = detailHeight
		}
		m.detailViewport.SetContent(m.renderDetailContent())

		// Update list scrolling for new dimensions
		m.ensureCursorVisible()

		// Force a clear screen to ensure proper redraw
		return m, tea.ClearScreen

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "esc":
			// If filter is active, clear it first
			if m.filterText != "" {
				m.filterText = ""
				m.cursor = 0
				m.listOffset = 0
				m.updateDetailViewport()
				return m, nil
			}
			return m, tea.Quit

		// Navigation in list
		case "j", "down":
			prs := m.filteredPRs()
			if m.focusedPane == paneList && len(prs) > 0 {
				m.cursor = (m.cursor + 1) % len(prs)
				m.ensureCursorVisible()
				m.updateDetailViewport()
			}

		case "k", "up":
			prs := m.filteredPRs()
			if m.focusedPane == paneList && len(prs) > 0 {
				m.cursor = (m.cursor - 1 + len(prs)) % len(prs)
				m.ensureCursorVisible()
				m.updateDetailViewport()
			}

		// Selection
		case " ":
			if m.focusedPane == paneList && len(m.prs) > 0 {
				if m.selected[m.cursor] {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = true
				}
				// Move to next item after selection
				if m.cursor < len(m.prs)-1 {
					m.cursor++
					m.ensureCursorVisible()
					m.updateDetailViewport()
				}
			}

		case "v":
			// Toggle select all / clear all
			if len(m.prs) > 0 {
				if len(m.selected) == len(m.prs) {
					// All selected, clear
					m.selected = make(map[int]bool)
				} else {
					// Select all
					for i := range m.prs {
						m.selected[i] = true
					}
				}
			}

		// Switch panes
		case "tab":
			m.focusedPane = (m.focusedPane + 1) % 2
			m.resizeViewport()

		case "1":
			m.focusedPane = paneList
			m.resizeViewport()

		case "2":
			m.focusedPane = paneDetail
			m.resizeViewport()

		// PgUp/PgDown scroll detail pane while in list (like gh-news)
		case "pgup":
			m.detailViewport.HalfPageUp()

		case "pgdown":
			m.detailViewport.HalfPageDown()

		// Scroll detail pane when focused
		case "ctrl+u":
			if m.focusedPane == paneDetail {
				m.detailViewport.HalfPageUp()
			}

		case "ctrl+d":
			if m.focusedPane == paneDetail {
				m.detailViewport.HalfPageDown()
			}

		// Actions
		case "o":
			if len(m.prs) > 0 {
				return m, m.openInBrowser()
			}

		case "d":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				return m, m.showDiff()
			}

		case "l":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				pr := m.prs[m.cursor]
				if len(pr.Checks) > 0 {
					m.showChecksModal = true
					m.checksModalCursor = 0
					m.checksModalChecks = pr.Checks
				} else {
					m.statusMsg = "No checks for this PR"
					m.statusTime = 30
				}
				return m, nil
			}

		case "y":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				return m, copyToClipboard(m.prs[m.cursor].URL)
			}

		case "x":
			// Show custom actions modal
			if m.config != nil && len(m.config.Actions) > 0 && len(m.prs) > 0 {
				m.showActionsModal = true
				m.actionsModalCursor = 0
				return m, nil
			} else if m.config == nil || len(m.config.Actions) == 0 {
				m.statusMsg = "No custom actions configured"
				m.statusTime = 30
			}

		case "a":
			prs := m.getSelectedPRs()
			if len(prs) > 0 {
				m.inputMode = true
				m.inputAction = "approve"
				m.inputPRs = prs
				m.inputModel = textarea.New()
				if len(prs) > 1 {
					m.inputModel.Placeholder = fmt.Sprintf("Optional comment for %d PRs...", len(prs))
				} else {
					m.inputModel.Placeholder = "Optional comment..."
				}
				m.inputModel.Focus()
				m.inputModel.CharLimit = 1000
				m.inputModel.SetWidth(60)
				m.inputModel.SetHeight(3)
				m.selected = make(map[int]bool) // Clear selection
				return m, textarea.Blink
			}

		case "c":
			prs := m.getSelectedPRs()
			if len(prs) > 0 {
				m.inputMode = true
				m.inputAction = "comment"
				m.inputPRs = prs
				m.inputModel = textarea.New()
				if len(prs) > 1 {
					m.inputModel.Placeholder = fmt.Sprintf("Comment for %d PRs...", len(prs))
				} else {
					m.inputModel.Placeholder = "Enter your comment..."
				}
				m.inputModel.Focus()
				m.inputModel.CharLimit = 1000
				m.inputModel.SetWidth(60)
				m.inputModel.SetHeight(3)
				m.selected = make(map[int]bool) // Clear selection
				return m, textarea.Blink
			}

		case "r":
			prs := m.getSelectedPRs()
			if len(prs) > 0 {
				m.inputMode = true
				m.inputAction = "changes"
				m.inputPRs = prs
				m.inputModel = textarea.New()
				if len(prs) > 1 {
					m.inputModel.Placeholder = fmt.Sprintf("Request changes for %d PRs...", len(prs))
				} else {
					m.inputModel.Placeholder = "Enter reason for requesting changes..."
				}
				m.inputModel.Focus()
				m.inputModel.CharLimit = 1000
				m.inputModel.SetWidth(60)
				m.inputModel.SetHeight(3)
				m.selected = make(map[int]bool) // Clear selection
				return m, textarea.Blink
			}

		case "R":
			m.loading = true
			return m, m.loadPRs()

		case "?":
			m.showHelp = true
			return m, nil

		case "/":
			// Enter filter mode
			m.filterMode = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "Filter PRs..."
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			m.filterInput.SetValue(m.filterText)
			return m, textinput.Blink

		}

		// Detail pane navigation when focused
		if m.focusedPane == paneDetail {
			switch msg.String() {
			case "j", "down":
				m.detailViewport.ScrollDown(1)
			case "k", "up":
				m.detailViewport.ScrollUp(1)
			case "g":
				m.detailViewport.GotoTop()
			case "G":
				m.detailViewport.GotoBottom()
			}
		}

	case prLoadedMsg:
		m.prs = msg.prs
		m.loading = false
		m.updateDetailViewport()

	case prErrorMsg:
		m.err = msg.err
		m.loading = false

	case actionResult:
		m.statusMsg = msg.message
		m.statusTime = 30 // Show for ~3 seconds (assuming 10 ticks/sec)

	case execDoneMsg:
		// Force a full redraw after returning from external process
		m.updateDetailViewport()
		return m, tea.ClearScreen

	case diffErrorMsg:
		m.statusMsg = msg.message
		m.statusTime = 50 // Show for ~5 seconds
	}

	// Update viewport
	m.detailViewport, cmd = m.detailViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) updateDetailViewport() {
	m.detailViewport.SetContent(m.renderDetailContent())
	m.detailViewport.GotoTop()
}

func (m *Model) resizeViewport() {
	detailWidth, detailHeight := m.detailPaneDimensions()
	m.detailViewport.Width = detailWidth
	m.detailViewport.Height = detailHeight
	m.detailViewport.SetContent(m.renderDetailContent())
}

// ensureCursorVisible adjusts listOffset so the cursor is visible.
// Each PR item takes 2 lines (title + author).
func (m *Model) ensureCursorVisible() {
	if len(m.prs) == 0 {
		return
	}

	// Calculate visible items based on height
	// Each item is 2 lines, so visible items = height / 2
	_, height := m.listPaneDimensions()
	linesPerItem := 2
	visibleItems := height / linesPerItem
	if visibleItems < 1 {
		visibleItems = 1
	}

	// Adjust offset if cursor is out of view
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	} else if m.cursor >= m.listOffset+visibleItems {
		m.listOffset = m.cursor - visibleItems + 1
	}

	// Clamp offset to valid range
	maxOffset := len(m.prs) - visibleItems
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.listOffset > maxOffset {
		m.listOffset = maxOffset
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m Model) listPaneDimensions() (int, int) {
	if m.width == 0 || m.height == 0 {
		return 40, 24
	}
	// Calculate list pane width based on focus
	var listRatio float64
	if m.focusedPane == paneList {
		listRatio = detailPaneRatio // 62% when focused
	} else {
		listRatio = listPaneRatio // 38% when unfocused
	}
	contentWidth := int(float64(m.width)*listRatio) - 4
	contentHeight := m.height - 6 // Account for header/footer/borders
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentWidth, contentHeight
}

func (m Model) updateFilterMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Apply filter and exit filter mode
			m.filterText = m.filterInput.Value()
			m.filterMode = false
			m.cursor = 0
			m.listOffset = 0
			m.updateDetailViewport()
			return m, nil
		case "esc":
			// Cancel filter editing (keep existing filter)
			m.filterMode = false
			return m, nil
		case "ctrl+c":
			// Clear filter and exit
			m.filterText = ""
			m.filterMode = false
			m.cursor = 0
			m.listOffset = 0
			m.updateDetailViewport()
			return m, nil
		}
	}

	// Let textinput handle all other input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m Model) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+j":
			// Ctrl+J inserts a newline (traditional LF)
			m.inputModel.InsertString("\n")
			return m, nil
		case "ctrl+d", "ctrl+s":
			// Ctrl+D or Ctrl+S submits the input
			message := m.inputModel.Value()
			prs := m.inputPRs
			action := m.inputAction
			m.inputMode = false
			m.inputPRs = nil

			switch action {
			case "approve":
				return m, approvePRsWithComment(prs, message)
			case "comment":
				if message == "" {
					m.statusMsg = "Comment cannot be empty"
					m.statusTime = 30
					return m, nil
				}
				return m, commentPRs(prs, message)
			case "changes":
				if message == "" {
					m.statusMsg = "Reason cannot be empty"
					m.statusTime = 30
					return m, nil
				}
				return m, requestChangesPRs(prs, message)
			}
		case "esc":
			m.inputMode = false
			m.inputPRs = nil
			return m, nil
		case "ctrl+c":
			m.inputMode = false
			m.inputPRs = nil
			return m, nil
		}
	case actionResult:
		m.statusMsg = msg.message
		m.statusTime = 30
		m.inputMode = false
		return m, nil
	}

	// Let textarea handle all other input (including Enter for newlines)
	var cmd tea.Cmd
	m.inputModel, cmd = m.inputModel.Update(msg)
	return m, cmd
}

func (m Model) updateChecksModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.checksModalCursor < len(m.checksModalChecks)-1 {
				m.checksModalCursor++
			}
		case "k", "up":
			if m.checksModalCursor > 0 {
				m.checksModalCursor--
			}
		case "enter":
			// View logs for the selected check
			if m.checksModalCursor < len(m.checksModalChecks) {
				check := m.checksModalChecks[m.checksModalCursor]
				m.showChecksModal = false
				if m.cursor < len(m.prs) {
					pr := m.prs[m.cursor]
					return m, m.showCheckLogs(check, pr)
				}
			}
		case "e":
			// Explain check with AI
			if m.checksModalCursor < len(m.checksModalChecks) {
				check := m.checksModalChecks[m.checksModalCursor]
				m.showChecksModal = false
				if m.cursor < len(m.prs) {
					pr := m.prs[m.cursor]
					return m, m.explainCheckWithAI(check, pr)
				}
			}
		case "esc", "q", "l":
			m.showChecksModal = false
			return m, tea.ClearScreen
		}
	}
	return m, nil
}

func (m Model) updateActionsModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.config == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.actionsModalCursor < len(m.config.Actions)-1 {
				m.actionsModalCursor++
			}
		case "k", "up":
			if m.actionsModalCursor > 0 {
				m.actionsModalCursor--
			}
		case "enter":
			// Execute the selected action
			if m.actionsModalCursor < len(m.config.Actions) {
				action := m.config.Actions[m.actionsModalCursor]
				m.showActionsModal = false
				prs := m.getSelectedPRs()
				if len(prs) > 0 {
					return m, m.executeAction(action, prs)
				}
			}
		case "esc", "q", "x":
			m.showActionsModal = false
			return m, tea.ClearScreen
		}
	}
	return m, nil
}

func (m Model) executeAction(action lazypr.Action, prs []lazypr.PRDetail) tea.Cmd {
	// Substitute placeholders in command
	cmd := lazypr.SubstituteBatchPlaceholders(action.Command, prs)

	// Log the command being executed
	logFile, _ := os.OpenFile("/tmp/lazypr-actions.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if logFile != nil {
		fmt.Fprintf(logFile, "[%s] Action: %s\n", time.Now().Format(time.RFC3339), action.Name)
		fmt.Fprintf(logFile, "[%s] Command: %s\n", time.Now().Format(time.RFC3339), cmd)
		logFile.Close()
	}

	if action.Interactive {
		// Interactive: suspend TUI and run with full terminal access
		c := exec.Command("bash", "-c", cmd)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				if logFile, _ := os.OpenFile("/tmp/lazypr-actions.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); logFile != nil {
					fmt.Fprintf(logFile, "[%s] Interactive error: %v\n", time.Now().Format(time.RFC3339), err)
					logFile.Close()
				}
			}
			return execDoneMsg{}
		})
	}

	// Non-interactive: run in background
	return func() tea.Msg {
		c := exec.Command("bash", "-c", cmd)
		var stdout, stderr strings.Builder
		c.Stdout = &stdout
		c.Stderr = &stderr
		err := c.Run()

		// Log the result
		if logFile, _ := os.OpenFile("/tmp/lazypr-actions.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); logFile != nil {
			if err != nil {
				fmt.Fprintf(logFile, "[%s] Error: %v\n", time.Now().Format(time.RFC3339), err)
			}
			if stdout.Len() > 0 {
				fmt.Fprintf(logFile, "[%s] Stdout: %s\n", time.Now().Format(time.RFC3339), stdout.String())
			}
			if stderr.Len() > 0 {
				fmt.Fprintf(logFile, "[%s] Stderr: %s\n", time.Now().Format(time.RFC3339), stderr.String())
			}
			fmt.Fprintf(logFile, "[%s] Exit code: %d\n", time.Now().Format(time.RFC3339), c.ProcessState.ExitCode())
			logFile.Close()
		}

		if err != nil {
			errMsg := err.Error()
			if stderr.Len() > 0 {
				// Show first line of stderr
				firstLine := strings.Split(stderr.String(), "\n")[0]
				if len(firstLine) > 50 {
					firstLine = firstLine[:47] + "..."
				}
				errMsg = firstLine
			}
			return actionResult{success: false, message: fmt.Sprintf("Failed: %s - %s", action.Name, errMsg)}
		}
		return actionResult{success: true, message: fmt.Sprintf("Executed: %s", action.Name)}
	}
}

// parseGitHubActionsURL extracts run ID and job ID from a GitHub Actions URL.
// URL format: https://github.com/owner/repo/actions/runs/<run-id>/job/<job-id>
func parseGitHubActionsURL(url string) (runID, jobID string, ok bool) {
	re := regexp.MustCompile(`github\.com/[^/]+/[^/]+/actions/runs/(\d+)/job/(\d+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) == 3 {
		return matches[1], matches[2], true
	}
	return "", "", false
}

// showCheckLogs displays logs for a CI check in a pager.
func (m Model) showCheckLogs(check lazypr.Check, pr lazypr.PRDetail) tea.Cmd {
	repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)

	// Try to parse as GitHub Actions URL
	runID, jobID, ok := parseGitHubActionsURL(check.URL)
	if !ok {
		// Not a GitHub Actions URL - open in browser as fallback
		return func() tea.Msg {
			if check.URL != "" {
				_ = runCommand("xdg-open", check.URL)
			} else {
				checksURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d/checks", pr.Owner, pr.Repo, pr.Number)
				_ = runCommand("xdg-open", checksURL)
			}
			return execDoneMsg{}
		}
	}

	// Build gh run view command
	ghArgs := []string{"run", "view", runID, "-R", repo, "--job", jobID, "--log"}

	// Detect pager
	pager := os.Getenv("PAGER")
	if pager == "" {
		if commandExists("less") {
			pager = "less"
		} else if commandExists("more") {
			pager = "more"
		} else {
			pager = "cat"
		}
	}

	// Build command pipeline
	cmdStr := fmt.Sprintf("gh %s | %s", strings.Join(ghArgs, " "), pager)
	c := exec.Command("bash", "-c", cmdStr)

	// Use tea.ExecProcess to suspend TUI and run pager
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{}
	})
}

// getAICommand returns the AI command to use for explanations.
// Checks LAZYPR_AI_CMD env var, then tries common AI CLI tools.
func getAICommand() string {
	if cmd := os.Getenv("LAZYPR_AI_CMD"); cmd != "" {
		return cmd
	}
	// Try common AI CLI tools
	for _, cmd := range []string{"aichat", "gemini", "llm", "sgpt"} {
		if commandExists(cmd) {
			return cmd
		}
	}
	return ""
}

// explainCheckWithAI sends check logs to an AI for explanation.
func (m Model) explainCheckWithAI(check lazypr.Check, pr lazypr.PRDetail) tea.Cmd {
	repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)

	// Get AI command
	aiCmd := getAICommand()
	if aiCmd == "" {
		return func() tea.Msg {
			return actionResult{success: false, message: "No AI command found. Set LAZYPR_AI_CMD or install aichat/gemini/llm"}
		}
	}

	// Try to parse as GitHub Actions URL
	runID, jobID, ok := parseGitHubActionsURL(check.URL)
	if !ok {
		return func() tea.Msg {
			return actionResult{success: false, message: "Can only explain GitHub Actions logs"}
		}
	}

	// Build command to fetch logs and pipe to AI
	prompt := fmt.Sprintf("Analyze this CI log from GitHub Actions check '%s' and explain why it failed or what happened. Be concise and focus on the actual error or issue:", check.Name)

	// Detect pager
	pager := os.Getenv("PAGER")
	if pager == "" {
		if commandExists("less") {
			pager = "less"
		} else {
			pager = "cat"
		}
	}

	// Build command: fetch logs, pipe to AI with prompt, show in pager
	cmdStr := fmt.Sprintf("gh run view %s -R %s --job %s --log | %s %q | %s",
		runID, repo, jobID, aiCmd, prompt, pager)
	c := exec.Command("bash", "-c", cmdStr)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execDoneMsg{}
	})
}

func (m Model) detailPaneDimensions() (int, int) {
	if m.width == 0 || m.height == 0 {
		return 80, 24
	}
	// Account for borders, header, footer
	// Detail pane width depends on focus state (golden ratio)
	var detailRatio float64
	if m.focusedPane == paneDetail {
		detailRatio = detailPaneRatio // 62% when focused
	} else {
		detailRatio = listPaneRatio // 38% when unfocused
	}
	contentWidth := int(float64(m.width)*detailRatio) - 4
	contentHeight := m.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentWidth, contentHeight
}

// getSelectedPRs returns the selected PRs, or the current PR if none selected.
func (m Model) getSelectedPRs() []lazypr.PRDetail {
	if len(m.selected) > 0 {
		prs := make([]lazypr.PRDetail, 0, len(m.selected))
		for idx := range m.selected {
			if idx < len(m.prs) {
				prs = append(prs, m.prs[idx])
			}
		}
		return prs
	}
	// No selection, return current PR
	if m.cursor >= 0 && m.cursor < len(m.prs) {
		return []lazypr.PRDetail{m.prs[m.cursor]}
	}
	return nil
}

// filteredPRs returns PRs matching the current filter
func (m Model) filteredPRs() []lazypr.PRDetail {
	if m.filterText == "" {
		return m.prs
	}
	filter := strings.ToLower(m.filterText)
	var result []lazypr.PRDetail
	for _, pr := range m.prs {
		// Match against title, author, or PR number
		if strings.Contains(strings.ToLower(pr.Title), filter) ||
			strings.Contains(strings.ToLower(pr.Author), filter) ||
			strings.Contains(fmt.Sprintf("%d", pr.Number), filter) {
			result = append(result, pr)
		}
	}
	return result
}

func (m Model) openInBrowser() tea.Cmd {
	return func() tea.Msg {
		if m.cursor >= 0 && m.cursor < len(m.prs) {
			pr := m.prs[m.cursor]
			// Use gh to open in browser
			_ = runCommand("gh", "pr", "view", "--web", "-R",
				fmt.Sprintf("%s/%s", pr.Owner, pr.Repo),
				fmt.Sprintf("%d", pr.Number))
		}
		return nil
	}
}

// diffErrorMsg is sent when diff viewing fails.
type diffErrorMsg struct {
	message string
}

// showDiff shows the PR diff in a pager.
func (m Model) showDiff() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.prs) {
		return nil
	}
	pr := m.prs[m.cursor]
	repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
	prNum := fmt.Sprintf("%d", pr.Number)

	// First check if the diff is available (not too large)
	checkCmd := exec.Command("gh", "pr", "diff", prNum, "-R", repo)
	if err := checkCmd.Run(); err != nil {
		// Diff failed - likely too large, offer to open in browser
		return func() tea.Msg {
			return diffErrorMsg{message: fmt.Sprintf("Diff too large for #%d - press 'o' to view in browser", pr.Number)}
		}
	}

	// Build the gh pr diff command
	ghArgs := []string{"pr", "diff", prNum, "-R", repo}

	// Detect pager: check $PAGER, then fall back to less, then more
	pager := os.Getenv("PAGER")
	if pager == "" {
		if commandExists("less") {
			pager = "less"
		} else if commandExists("more") {
			pager = "more"
		} else {
			pager = "cat" // fallback, no paging
		}
	}

	// Check if delta is available for syntax highlighting
	useDelta := commandExists("delta")

	// Build the command pipeline
	var cmdStr string
	if useDelta {
		cmdStr = fmt.Sprintf("gh %s | delta | %s", strings.Join(ghArgs, " "), pager)
	} else {
		cmdStr = fmt.Sprintf("gh %s | %s", strings.Join(ghArgs, " "), pager)
	}

	// Create the command
	c := exec.Command("bash", "-c", cmdStr)

	// Use tea.ExecProcess to suspend TUI and run pager
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// Return execDoneMsg to trigger a redraw
		return execDoneMsg{}
	})
}

// View implements tea.Model.
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	// Show help screen
	if m.showHelp {
		return m.renderHelpScreen()
	}

	// Build the layout
	header := m.renderHeader()
	content := m.renderContent()
	footer := m.renderFooter()

	view := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)

	// Overlay input modal if active
	if m.inputMode {
		modal := m.renderInputModal()
		view = m.overlayModal(view, modal)
	}

	// Overlay checks modal if active
	if m.showChecksModal {
		modal := m.renderChecksModal()
		view = m.overlayModal(view, modal)
	}

	// Overlay actions modal if active
	if m.showActionsModal {
		modal := m.renderActionsModal()
		view = m.overlayModal(view, modal)
	}

	return view
}

func (m Model) renderHelpScreen() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.WarnFg)

	descStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.TextFg)

	var lines []string
	lines = append(lines, titleStyle.Render("lazypr - GitHub PR Viewer"))
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("Navigation"))
	lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render("j/k, ↑/↓"), descStyle.Render("Navigate PR list / scroll detail")))
	lines = append(lines, fmt.Sprintf("  %s       %s", keyStyle.Render("Tab"), descStyle.Render("Switch between list and detail pane")))
	lines = append(lines, fmt.Sprintf("  %s       %s", keyStyle.Render("1/2"), descStyle.Render("Focus list / detail pane")))
	lines = append(lines, fmt.Sprintf("  %s  %s", keyStyle.Render("PgUp/PgDn"), descStyle.Render("Scroll detail pane (works from list)")))
	lines = append(lines, fmt.Sprintf("  %s       %s", keyStyle.Render("g/G"), descStyle.Render("Go to top / bottom of detail")))
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("Filter"))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("/"), descStyle.Render("Filter PRs by title/author/number")))
	lines = append(lines, fmt.Sprintf("  %s       %s", keyStyle.Render("Esc"), descStyle.Render("Clear filter (when active)")))
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("Selection"))
	lines = append(lines, fmt.Sprintf("  %s     %s", keyStyle.Render("Space"), descStyle.Render("Toggle selection on current PR")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("v"), descStyle.Render("Select all / clear all")))
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("Actions (apply to selected or current PR)"))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("a"), descStyle.Render("Approve PR(s)")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("c"), descStyle.Render("Add comment to PR(s)")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("r"), descStyle.Render("Request changes on PR(s)")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("d"), descStyle.Render("View PR diff in pager")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("l"), descStyle.Render("View CI check logs (Enter=view, e=AI explain)")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("o"), descStyle.Render("Open PR in browser")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("y"), descStyle.Render("Copy PR URL to clipboard")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("x"), descStyle.Render("Custom actions menu")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("R"), descStyle.Render("Refresh PR data")))
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("General"))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("?"), descStyle.Render("Toggle this help screen")))
	lines = append(lines, fmt.Sprintf("  %s     %s", keyStyle.Render("q/Esc"), descStyle.Render("Quit")))
	lines = append(lines, "")
	lines = append(lines, descStyle.Render("Press any key to close this help screen"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(1, 2).
		Margin(2)

	help := boxStyle.Render(strings.Join(lines, "\n"))

	// Center on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, help)
}

func (m Model) renderInputModal() string {
	var title string
	var helpText string
	switch m.inputAction {
	case "approve":
		title = "Approve PR"
		helpText = "Ctrl+D to approve (comment optional), Enter/Ctrl+J for newline, Esc to cancel"
	case "comment":
		title = "Add Comment"
		helpText = "Ctrl+D to submit, Enter/Ctrl+J for newline, Esc to cancel"
	case "changes":
		title = "Request Changes"
		helpText = "Ctrl+D to submit, Enter/Ctrl+J for newline, Esc to cancel"
	default:
		title = "Input"
		helpText = "Ctrl+D to submit, Enter/Ctrl+J for newline, Esc to cancel"
	}

	if len(m.inputPRs) > 1 {
		title += fmt.Sprintf(" (%d PRs)", len(m.inputPRs))
	} else if m.cursor < len(m.prs) {
		title += fmt.Sprintf(" - PR #%d", m.prs[m.cursor].Number)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(1, 2).
		Width(70)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true)

	hintStyle := lipgloss.NewStyle().Faint(true)

	content := fmt.Sprintf("%s\n\n%s\n\n%s",
		titleStyle.Render(title),
		m.inputModel.View(),
		hintStyle.Render(helpText),
	)

	return boxStyle.Render(content)
}

func (m Model) renderChecksModal() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(1, 2).
		Width(70)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true)

	var lines []string
	lines = append(lines, titleStyle.Render("CI Checks"))
	lines = append(lines, "")

	for i, check := range m.checksModalChecks {
		icon := lazypr.CheckIcon(check.Conclusion, check.Status)
		var style lipgloss.Style
		switch icon {
		case lazypr.IconSuccess:
			style = m.styles.StatusSuccess
		case lazypr.IconFailure:
			style = m.styles.StatusError
		case lazypr.IconPending:
			style = m.styles.StatusPending
		default:
			style = m.styles.StatusUnknown
		}

		line := fmt.Sprintf("%s %s", style.Render(icon), check.Name)

		if i == m.checksModalCursor {
			line = m.styles.PRItemSelected.Render("▸ " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Faint(true).Render("j/k: navigate  Enter: view logs  e: AI explain  Esc: close"))

	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderActionsModal() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Accent).
		Padding(1, 2).
		Width(60)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Accent).
		Bold(true)

	var lines []string

	// Show title with selection count
	prs := m.getSelectedPRs()
	if len(prs) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Custom Actions (%d PRs)", len(prs))))
	} else if len(prs) == 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Custom Actions - PR #%d", prs[0].Number)))
	} else {
		lines = append(lines, titleStyle.Render("Custom Actions"))
	}
	lines = append(lines, "")

	if m.config == nil || len(m.config.Actions) == 0 {
		lines = append(lines, "No custom actions configured")
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Configure actions in ~/.config/lazypr/config.toml"))
	} else {
		for i, action := range m.config.Actions {
			var interactiveHint string
			if action.Interactive {
				interactiveHint = " [interactive]"
			}
			line := fmt.Sprintf("%s%s", action.Name, interactiveHint)

			if i == m.actionsModalCursor {
				line = m.styles.PRItemSelected.Render("▸ " + line)
			} else {
				line = "  " + line
			}
			lines = append(lines, line)
		}

		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("j/k: navigate  Enter: execute  Esc: close"))
	}

	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) overlayModal(background, modal string) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	// Center the modal vertically
	startY := (len(bgLines) - len(modalLines)) / 2
	if startY < 0 {
		startY = 0
	}

	// Center horizontally
	modalWidth := lipgloss.Width(modal)
	startX := (m.width - modalWidth) / 2
	if startX < 0 {
		startX = 0
	}

	// Overlay modal on background
	for i, line := range modalLines {
		row := startY + i
		if row < len(bgLines) {
			bgLine := bgLines[row]
			// Replace portion of background with modal line
			padding := strings.Repeat(" ", startX)
			newLine := padding + line
			if len(newLine) < len(bgLine) {
				newLine += bgLine[len(newLine):]
			}
			bgLines[row] = newLine
		}
	}

	return strings.Join(bgLines, "\n")
}

func (m Model) renderHeader() string {
	var title string
	if m.repo != nil {
		title = fmt.Sprintf("lazypr - %s - %d PRs", m.repo.String(), len(m.prs))
	} else {
		title = fmt.Sprintf("lazypr - %d PRs", len(m.prs))
	}
	if m.loading {
		title += " (loading...)"
	}

	helpHint := "[?] help"
	padding := m.width - lipgloss.Width(title) - lipgloss.Width(helpHint) - 4
	if padding < 1 {
		padding = 1
	}

	header := title + strings.Repeat(" ", padding) + helpHint
	return m.styles.Header.Width(m.width).Render(header)
}

func (m Model) renderContent() string {
	// Calculate pane widths based on golden ratio
	// Focused pane gets the larger portion (62%), unfocused gets smaller (38%)
	var listWidth, detailWidth int
	if m.focusedPane == paneList {
		listWidth = int(float64(m.width) * detailPaneRatio)  // 62% for focused list
		detailWidth = m.width - listWidth                    // 38% for unfocused detail
	} else {
		listWidth = int(float64(m.width) * listPaneRatio)    // 38% for unfocused list
		detailWidth = m.width - listWidth                    // 62% for focused detail
	}

	// Content height (total - header - footer)
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render panes
	listContent := m.renderListPane(listWidth-4, contentHeight-2)
	detailContent := m.renderDetailPane(detailWidth-4, contentHeight-2)

	// Apply pane styles with focus indication
	var listPane, detailPane string
	if m.focusedPane == paneList {
		listPane = m.styles.FocusedPane.Width(listWidth-2).Height(contentHeight).Render(listContent)
		detailPane = m.styles.DetailPane.Width(detailWidth-2).Height(contentHeight).Render(detailContent)
	} else {
		listPane = m.styles.ListPane.Width(listWidth-2).Height(contentHeight).Render(listContent)
		detailPane = m.styles.FocusedPane.Width(detailWidth-2).Height(contentHeight).Render(detailContent)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

func (m Model) renderListPane(width, height int) string {
	var output []string

	// Get filtered PRs
	prs := m.filteredPRs()

	// Show PR count header
	var countText string
	if m.filterText != "" {
		countText = fmt.Sprintf("%d/%d PRs", len(prs), len(m.prs))
	} else {
		countText = fmt.Sprintf("%d PRs", len(prs))
	}
	output = append(output, m.styles.PRAuthor.Render(countText))
	height-- // Reduce available height

	// Show filter bar if filtering or filter is active
	if m.filterMode {
		filterBar := fmt.Sprintf("/ %s", m.filterInput.View())
		output = append(output, m.styles.PRNumber.Render(filterBar))
		height-- // Reduce available height
	} else if m.filterText != "" {
		filterBar := fmt.Sprintf("Filter: %s", m.filterText)
		output = append(output, m.styles.PRAuthor.Render(filterBar))
		height-- // Reduce available height
	}

	if len(prs) == 0 {
		if m.loading {
			output = append(output, "Loading PRs...")
		} else if m.filterText != "" {
			output = append(output, "No PRs match filter")
		} else {
			output = append(output, "No PRs to display")
		}
		return strings.Join(output, "\n")
	}

	// Calculate visible range based on listOffset
	linesPerItem := 2
	visibleItems := height / linesPerItem
	if visibleItems < 1 {
		visibleItems = 1
	}

	startIdx := m.listOffset
	endIdx := startIdx + visibleItems
	if endIdx > len(prs) {
		endIdx = len(prs)
	}

	for i := startIdx; i < endIdx; i++ {
		pr := prs[i]

		// Selection marker
		selectMarker := " "
		if m.selected[i] {
			selectMarker = "●"
		}

		// Determine status icon and style
		statusIcon := pr.StatusIcon()
		var statusStyle lipgloss.Style
		switch {
		case pr.IsMerged():
			statusStyle = m.styles.StatusMerged
		case pr.IsClosed():
			statusStyle = m.styles.StatusError
		case pr.HasConflicts():
			statusStyle = m.styles.StatusError
		case pr.EffectiveStatus() == "SUCCESS":
			statusStyle = m.styles.StatusSuccess
		case pr.EffectiveStatus() == "FAILURE" || pr.EffectiveStatus() == "ERROR":
			statusStyle = m.styles.StatusError
		case pr.EffectiveStatus() == "PENDING":
			statusStyle = m.styles.StatusPending
		default:
			statusStyle = m.styles.StatusUnknown
		}

		// Format PR line
		prNum := m.styles.PRNumber.Render(fmt.Sprintf("#%d", pr.Number))
		status := statusStyle.Render(statusIcon)
		author := m.styles.PRAuthor.Render(fmt.Sprintf("@%s", pr.Author))

		// Truncate title to fit
		titleWidth := width - 17 // Account for selection marker
		if titleWidth < 10 {
			titleWidth = 10
		}
		title := pr.Title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "..."
		}

		line := fmt.Sprintf("%s %s %s %s\n    %s", selectMarker, status, prNum, title, author)

		// Apply selection style
		if i == m.cursor {
			line = m.styles.PRItemSelected.Width(width).Render(line)
		} else {
			line = m.styles.PRItem.Width(width).Render(line)
		}

		output = append(output, line)
	}

	return strings.Join(output, "\n")
}

func (m Model) renderDetailPane(width, height int) string {
	return m.detailViewport.View()
}

// stateIconAndStyle returns icon and style for PR state
func (m Model) stateIconAndStyle(state string) (string, lipgloss.Style) {
	switch state {
	case "OPEN":
		return "●", m.styles.StatusSuccess
	case "MERGED":
		return lazypr.IconMerged, m.styles.StatusMerged
	case "CLOSED":
		return lazypr.IconClosed, m.styles.StatusError
	default:
		return "○", m.styles.StatusUnknown
	}
}

// mergeableIconAndStyle returns icon and style for mergeable status
func (m Model) mergeableIconAndStyle(pr lazypr.PRDetail) (string, lipgloss.Style) {
	if pr.State == "MERGED" {
		return lazypr.IconMerged, m.styles.StatusMerged
	}
	if pr.State == "CLOSED" {
		return lazypr.IconClosed, m.styles.StatusError
	}
	switch pr.Mergeable {
	case "MERGEABLE":
		return lazypr.IconSuccess, m.styles.StatusSuccess
	case "CONFLICTING":
		return lazypr.IconConflict, m.styles.StatusError
	case "UNKNOWN":
		return lazypr.IconPending, m.styles.StatusPending
	default:
		return lazypr.IconUnknown, m.styles.StatusUnknown
	}
}

// ciStatusIconAndStyle returns icon and style for CI status
func (m Model) ciStatusIconAndStyle(status string) (string, lipgloss.Style) {
	switch status {
	case "SUCCESS":
		return lazypr.IconSuccess, m.styles.StatusSuccess
	case "FAILURE", "ERROR":
		return lazypr.IconFailure, m.styles.StatusError
	case "PENDING":
		return lazypr.IconPending, m.styles.StatusPending
	default:
		return lazypr.IconUnknown, m.styles.StatusUnknown
	}
}

func (m Model) renderDetailContent() string {
	prs := m.filteredPRs()
	if len(prs) == 0 || m.cursor >= len(prs) {
		return "Select a PR to view details"
	}

	pr := prs[m.cursor]
	width, _ := m.detailPaneDimensions()

	var sections []string

	// Title and basic info
	titleSection := m.styles.SectionTitle.Render(pr.Title)
	sections = append(sections, titleSection)

	// Separator
	sections = append(sections, strings.Repeat("─", width))

	// Metadata with colors
	var meta []string

	// Author
	meta = append(meta, fmt.Sprintf("Author: %s", m.styles.PRNumber.Render("@"+pr.Author)))

	// State with color
	stateIcon, stateStyle := m.stateIconAndStyle(pr.State)
	meta = append(meta, fmt.Sprintf("State: %s %s", stateStyle.Render(stateIcon), stateStyle.Render(pr.State)))

	// Created date
	meta = append(meta, fmt.Sprintf("Created: %s", m.styles.PRAuthor.Render(pr.CreatedAt.Format("2006-01-02"))))

	// Base/Head branches
	meta = append(meta, fmt.Sprintf("Base: %s <- %s",
		m.styles.PRNumber.Render(pr.BaseRef),
		m.styles.CommitSHA.Render(pr.HeadRef)))

	// Mergeable with icon and color
	mergeIcon, mergeStyle := m.mergeableIconAndStyle(pr)
	meta = append(meta, fmt.Sprintf("Mergeable: %s %s", mergeStyle.Render(mergeIcon), mergeStyle.Render(pr.MergeableText())))

	// CI Status with icon and color
	if pr.StatusState != "" {
		ciIcon, ciStyle := m.ciStatusIconAndStyle(pr.EffectiveStatus())
		meta = append(meta, fmt.Sprintf("CI Status: %s %s", ciStyle.Render(ciIcon), ciStyle.Render(pr.EffectiveStatus())))
	}

	sections = append(sections, m.styles.SectionBody.Render(strings.Join(meta, "\n")))

	// Labels
	if len(pr.Labels) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("Labels (%d)", len(pr.Labels))))
		labelText := strings.Join(pr.Labels, ", ")
		sections = append(sections, m.styles.SectionBody.Render(labelText))
	}

	// CI Checks
	if len(pr.Checks) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("CI Checks (%d)", len(pr.Checks))))
		var checks []string
		for _, check := range pr.Checks {
			icon := lazypr.CheckIcon(check.Conclusion, check.Status)
			var style lipgloss.Style
			switch icon {
			case lazypr.IconSuccess:
				style = m.styles.StatusSuccess
			case lazypr.IconFailure:
				style = m.styles.StatusError
			case lazypr.IconPending:
				style = m.styles.StatusPending
			default:
				style = m.styles.StatusUnknown
			}
			checks = append(checks, style.Render(icon)+" "+check.Name)
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(checks, "\n")))
	}

	// Commits
	if len(pr.Commits) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("Commits (%d)", len(pr.Commits))))
		var commits []string
		for _, c := range pr.Commits {
			sha := c.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			commits = append(commits, m.styles.CommitSHA.Render(sha)+" "+c.Message)
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(commits, "\n")))
	}

	// Files
	if len(pr.Files) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("Files (%d)", len(pr.Files))))
		var files []string
		for _, f := range pr.Files {
			var changeIndicator string
			if f.Additions > 0 {
				changeIndicator = m.styles.FileAdded.Render(fmt.Sprintf("+%d", f.Additions))
			}
			if f.Deletions > 0 {
				if changeIndicator != "" {
					changeIndicator += " "
				}
				changeIndicator += m.styles.FileRemoved.Render(fmt.Sprintf("-%d", f.Deletions))
			}
			files = append(files, fmt.Sprintf("%s %s", m.styles.FilePath.Render(f.Path), changeIndicator))
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(files, "\n")))
	}

	// Reviews
	if len(pr.Reviews) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("Reviews (%d)", len(pr.Reviews))))
		var reviews []string
		for _, r := range pr.Reviews {
			var icon string
			var style lipgloss.Style
			switch r.State {
			case "APPROVED":
				icon = lazypr.IconSuccess
				style = m.styles.StatusSuccess
			case "CHANGES_REQUESTED":
				icon = lazypr.IconFailure
				style = m.styles.StatusError
			case "COMMENTED":
				icon = lazypr.IconSkipped
				style = m.styles.StatusUnknown
			default:
				icon = lazypr.IconPending
				style = m.styles.StatusPending
			}
			reviewLine := style.Render(icon) + " " + r.Author + " - " + r.State
			if r.Body != "" {
				// Truncate long comments and show first line
				body := r.Body
				if idx := strings.Index(body, "\n"); idx != -1 {
					body = body[:idx]
				}
				if len(body) > 80 {
					body = body[:77] + "..."
				}
				reviewLine += "\n    " + m.styles.PRAuthor.Render(body)
			}
			reviews = append(reviews, reviewLine)
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(reviews, "\n")))
	}

	// Comments
	if len(pr.Comments) > 0 {
		sections = append(sections, "")
		sections = append(sections, m.styles.SectionTitle.Render(fmt.Sprintf("Comments (%d)", len(pr.Comments))))
		var comments []string
		for _, c := range pr.Comments {
			// Truncate long comments and show first line
			body := c.Body
			if idx := strings.Index(body, "\n"); idx != -1 {
				body = body[:idx]
			}
			if len(body) > 80 {
				body = body[:77] + "..."
			}
			commentLine := fmt.Sprintf("@%s: %s", c.Author, body)
			comments = append(comments, m.styles.PRAuthor.Render(commentLine))
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(comments, "\n")))
	}

	return strings.Join(sections, "\n")
}

func (m Model) renderFooter() string {
	// Show status message if present
	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(m.styles.Theme.WarnFg).
			Bold(true)
		return m.styles.Footer.Width(m.width).Render(statusStyle.Render(m.statusMsg))
	}

	var hints []string
	if m.focusedPane == paneList {
		hints = append(hints, "j/k: navigate", "Space: select", "v: all")
	} else {
		hints = append(hints, "j/k: scroll")
	}
	hints = append(hints, "Tab: pane")

	// Show selection count if any selected
	if len(m.selected) > 0 {
		hints = append(hints, fmt.Sprintf("[%d selected]", len(m.selected)))
	}

	hints = append(hints, "/: filter", "a: approve", "c: comment", "d: diff", "l: logs", "o: open", "?: help")

	footer := strings.Join(hints, "  ")
	return m.styles.Footer.Width(m.width).Render(footer)
}
