package main

import (
	"fmt"
	"os/exec"
	"strings"

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
	repo      *lazypr.RepoRef // For loading all PRs from a repo
	repoLimit int             // Limit when loading from repo
	cursor    int

	// Layout
	width       int
	height      int
	focusedPane int

	// Detail pane viewport for scrolling
	detailViewport viewport.Model

	// State
	loading    bool
	err        error
	ready      bool
	statusMsg  string // Temporary status message
	statusTime int    // Ticks until status clears

	// Input modal state
	inputMode    bool            // Whether input modal is active
	inputAction  string          // "comment" or "changes"
	inputModel   textinput.Model // Input field for modal

	// Help screen
	showHelp bool

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

// NewModel creates a new model with the given PR references.
func NewModel(refs []lazypr.PRRef) Model {
	return Model{
		refs:        refs,
		cursor:      0,
		focusedPane: paneList,
		loading:     true,
		styles:      NewStyles(DefaultTheme()),
	}
}

// NewRepoModel creates a new model that loads PRs from a repository.
func NewRepoModel(repo lazypr.RepoRef, limit int) Model {
	return Model{
		repo:        &repo,
		repoLimit:   limit,
		cursor:      0,
		focusedPane: paneList,
		loading:     true,
		styles:      NewStyles(DefaultTheme()),
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
			// Load PRs from repository
			prs, err = fetcher.FetchRepoPRs(*m.repo, m.repoLimit)
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

	// Handle input modal mode
	if m.inputMode {
		return m.updateInputMode(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Initialize viewport with detail pane dimensions
		detailWidth, detailHeight := m.detailPaneDimensions()
		m.detailViewport = viewport.New(detailWidth, detailHeight)
		m.detailViewport.SetContent(m.renderDetailContent())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit

		// Navigation in list
		case "j", "down":
			if m.focusedPane == paneList && len(m.prs) > 0 {
				m.cursor = (m.cursor + 1) % len(m.prs)
				m.updateDetailViewport()
			}

		case "k", "up":
			if m.focusedPane == paneList && len(m.prs) > 0 {
				m.cursor = (m.cursor - 1 + len(m.prs)) % len(m.prs)
				m.updateDetailViewport()
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

		case "y":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				return m, copyToClipboard(m.prs[m.cursor].URL)
			}

		case "a":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				return m, approvePR(m.prs[m.cursor])
			}

		case "c":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				m.inputMode = true
				m.inputAction = "comment"
				m.inputModel = textinput.New()
				m.inputModel.Placeholder = "Enter your comment..."
				m.inputModel.Focus()
				m.inputModel.CharLimit = 1000
				m.inputModel.Width = 60
				return m, textinput.Blink
			}

		case "r":
			if len(m.prs) > 0 && m.cursor < len(m.prs) {
				m.inputMode = true
				m.inputAction = "changes"
				m.inputModel = textinput.New()
				m.inputModel.Placeholder = "Enter reason for requesting changes..."
				m.inputModel.Focus()
				m.inputModel.CharLimit = 1000
				m.inputModel.Width = 60
				return m, textinput.Blink
			}

		case "R":
			m.loading = true
			return m, m.loadPRs()

		case "?":
			m.showHelp = true
			return m, nil

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

func (m Model) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.inputModel.Value() != "" {
				pr := m.prs[m.cursor]
				message := m.inputModel.Value()
				m.inputMode = false
				return m, m.executeInputAction(pr, m.inputAction, message)
			}
		case "esc", "ctrl+c":
			m.inputMode = false
			return m, nil
		}
	case actionResult:
		m.statusMsg = msg.message
		m.statusTime = 30
		m.inputMode = false
		return m, nil
	}

	var cmd tea.Cmd
	m.inputModel, cmd = m.inputModel.Update(msg)
	return m, cmd
}

func (m Model) executeInputAction(pr lazypr.PRDetail, action, message string) tea.Cmd {
	return func() tea.Msg {
		repo := fmt.Sprintf("%s/%s", pr.Owner, pr.Repo)
		prNum := fmt.Sprintf("%d", pr.Number)

		var args []string
		switch action {
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
		return actionResult{success: true, message: fmt.Sprintf("Action completed: %s on PR #%d", action, pr.Number)}
	}
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
	lines = append(lines, sectionStyle.Render("Actions"))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("a"), descStyle.Render("Approve PR")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("c"), descStyle.Render("Add comment")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("r"), descStyle.Render("Request changes")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("o"), descStyle.Render("Open PR in browser")))
	lines = append(lines, fmt.Sprintf("  %s         %s", keyStyle.Render("y"), descStyle.Render("Copy PR URL to clipboard")))
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
	switch m.inputAction {
	case "comment":
		title = "Add Comment"
	case "changes":
		title = "Request Changes"
	default:
		title = "Input"
	}

	if m.cursor < len(m.prs) {
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

	content := fmt.Sprintf("%s\n\n%s\n\nPress Enter to submit, Esc to cancel",
		titleStyle.Render(title),
		m.inputModel.View(),
	)

	return boxStyle.Render(content)
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
	if len(m.prs) == 0 {
		if m.loading {
			return "Loading PRs..."
		}
		return "No PRs to display"
	}

	var lines []string
	for i, pr := range m.prs {
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
		case pr.StatusState == "SUCCESS":
			statusStyle = m.styles.StatusSuccess
		case pr.StatusState == "FAILURE" || pr.StatusState == "ERROR":
			statusStyle = m.styles.StatusError
		case pr.StatusState == "PENDING":
			statusStyle = m.styles.StatusPending
		default:
			statusStyle = m.styles.StatusUnknown
		}

		// Format PR line
		prNum := m.styles.PRNumber.Render(fmt.Sprintf("#%d", pr.Number))
		status := statusStyle.Render(statusIcon)
		author := m.styles.PRAuthor.Render(fmt.Sprintf("@%s", pr.Author))

		// Truncate title to fit
		titleWidth := width - 15
		if titleWidth < 10 {
			titleWidth = 10
		}
		title := pr.Title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "..."
		}

		line := fmt.Sprintf("%s %s %s\n  %s", status, prNum, title, author)

		// Apply selection style
		if i == m.cursor {
			line = m.styles.PRItemSelected.Width(width).Render(line)
		} else {
			line = m.styles.PRItem.Width(width).Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderDetailPane(width, height int) string {
	return m.detailViewport.View()
}

func (m Model) renderDetailContent() string {
	if len(m.prs) == 0 || m.cursor >= len(m.prs) {
		return "Select a PR to view details"
	}

	pr := m.prs[m.cursor]
	width, _ := m.detailPaneDimensions()

	var sections []string

	// Title and basic info
	titleSection := m.styles.SectionTitle.Render(pr.Title)
	sections = append(sections, titleSection)

	// Separator
	sections = append(sections, strings.Repeat("─", width))

	// Metadata
	meta := []string{
		fmt.Sprintf("Author: @%s", pr.Author),
		fmt.Sprintf("State: %s", pr.State),
		fmt.Sprintf("Created: %s", pr.CreatedAt.Format("2006-01-02")),
		fmt.Sprintf("Base: %s <- %s", pr.BaseRef, pr.HeadRef),
		fmt.Sprintf("Mergeable: %s %s", pr.MergeableIcon(), pr.MergeableText()),
	}
	if pr.StatusState != "" {
		meta = append(meta, fmt.Sprintf("CI Status: %s", pr.StatusState))
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
			var icon string
			var style lipgloss.Style
			switch check.Conclusion {
			case "success", "SUCCESS":
				icon = "✓"
				style = m.styles.StatusSuccess
			case "failure", "FAILURE":
				icon = "✗"
				style = m.styles.StatusError
			case "":
				if check.Status == "in_progress" || check.Status == "IN_PROGRESS" {
					icon = "●"
					style = m.styles.StatusPending
				} else {
					icon = "○"
					style = m.styles.StatusUnknown
				}
			default:
				icon = "○"
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
				icon = "✓"
				style = m.styles.StatusSuccess
			case "CHANGES_REQUESTED":
				icon = "✗"
				style = m.styles.StatusError
			default:
				icon = "●"
				style = m.styles.StatusUnknown
			}
			reviews = append(reviews, style.Render(icon)+" "+r.Author+" - "+r.State)
		}
		sections = append(sections, m.styles.SectionBody.Render(strings.Join(reviews, "\n")))
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
		hints = append(hints, "j/k: navigate")
	} else {
		hints = append(hints, "j/k: scroll")
	}
	hints = append(hints, "Tab: switch pane", "PgUp/PgDn: scroll detail")
	hints = append(hints, "a: approve", "c: comment", "r: request changes", "y: copy", "o: open", "q: quit")

	footer := strings.Join(hints, "  ")
	return m.styles.Footer.Width(m.width).Render(footer)
}
