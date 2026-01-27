package main

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines colors for the TUI.
type Theme struct {
	// Primary colors
	Accent    lipgloss.Color
	TextFg    lipgloss.Color
	MutedFg   lipgloss.Color
	BorderFg  lipgloss.Color
	BorderDim lipgloss.Color

	// Status colors
	SuccessFg lipgloss.Color
	ErrorFg   lipgloss.Color
	WarnFg    lipgloss.Color
	PendingFg lipgloss.Color
	MergedFg  lipgloss.Color

	// Background
	BgColor      lipgloss.Color
	SelectedBg   lipgloss.Color
	FocusedBg    lipgloss.Color
	UnfocusedBg  lipgloss.Color
}

// DefaultTheme returns the default dark theme (Dracula-inspired).
func DefaultTheme() Theme {
	return Theme{
		Accent:      lipgloss.Color("#BD93F9"),
		TextFg:      lipgloss.Color("#F8F8F2"),
		MutedFg:     lipgloss.Color("#6272A4"),
		BorderFg:    lipgloss.Color("#BD93F9"),
		BorderDim:   lipgloss.Color("#44475A"),

		SuccessFg:   lipgloss.Color("#50FA7B"),
		ErrorFg:     lipgloss.Color("#FF5555"),
		WarnFg:      lipgloss.Color("#F1FA8C"),
		PendingFg:   lipgloss.Color("#FFB86C"),
		MergedFg:    lipgloss.Color("#BD93F9"), // Purple for merged

		BgColor:     lipgloss.Color("#282A36"),
		SelectedBg:  lipgloss.Color("#44475A"),
		FocusedBg:   lipgloss.Color("#383A59"),
		UnfocusedBg: lipgloss.Color("#282A36"),
	}
}

// Styles contains all the styled components.
type Styles struct {
	Theme Theme

	// Container styles
	App         lipgloss.Style
	Header      lipgloss.Style
	Footer      lipgloss.Style
	ListPane    lipgloss.Style
	DetailPane  lipgloss.Style
	FocusedPane lipgloss.Style

	// PR list styles
	PRItem         lipgloss.Style
	PRItemSelected lipgloss.Style
	PRNumber       lipgloss.Style
	PRTitle        lipgloss.Style
	PRAuthor       lipgloss.Style

	// Status indicators
	StatusSuccess lipgloss.Style
	StatusError   lipgloss.Style
	StatusPending lipgloss.Style
	StatusUnknown lipgloss.Style
	StatusMerged  lipgloss.Style

	// Detail pane styles
	SectionTitle lipgloss.Style
	SectionBody  lipgloss.Style
	CommitSHA    lipgloss.Style
	FilePath     lipgloss.Style
	FileAdded    lipgloss.Style
	FileRemoved  lipgloss.Style
	FileModified lipgloss.Style
	Label        lipgloss.Style
}

// NewStyles creates styles with the given theme.
func NewStyles(theme Theme) Styles {
	return Styles{
		Theme: theme,

		App: lipgloss.NewStyle().
			Background(theme.BgColor),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Accent).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(theme.BorderDim),

		Footer: lipgloss.NewStyle().
			Foreground(theme.MutedFg).
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(theme.BorderDim),

		ListPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, true, true, true).
			BorderForeground(theme.BorderDim).
			Padding(0, 1),

		DetailPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, true, true, true).
			BorderForeground(theme.BorderDim).
			Padding(0, 1),

		FocusedPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true, true, true, true).
			BorderForeground(theme.Accent).
			Padding(0, 1),

		PRItem: lipgloss.NewStyle().
			Padding(0, 1),

		PRItemSelected: lipgloss.NewStyle().
			Background(theme.SelectedBg).
			Foreground(theme.TextFg).
			Bold(true).
			Padding(0, 1),

		PRNumber: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true),

		PRTitle: lipgloss.NewStyle().
			Foreground(theme.TextFg),

		PRAuthor: lipgloss.NewStyle().
			Foreground(theme.MutedFg),

		StatusSuccess: lipgloss.NewStyle().
			Foreground(theme.SuccessFg),

		StatusError: lipgloss.NewStyle().
			Foreground(theme.ErrorFg),

		StatusPending: lipgloss.NewStyle().
			Foreground(theme.PendingFg),

		StatusUnknown: lipgloss.NewStyle().
			Foreground(theme.MutedFg),

		StatusMerged: lipgloss.NewStyle().
			Foreground(theme.MergedFg),

		SectionTitle: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true).
			Padding(0, 0, 0, 0),

		SectionBody: lipgloss.NewStyle().
			Foreground(theme.TextFg).
			Padding(0, 0, 0, 2),

		CommitSHA: lipgloss.NewStyle().
			Foreground(theme.WarnFg),

		FilePath: lipgloss.NewStyle().
			Foreground(theme.TextFg),

		FileAdded: lipgloss.NewStyle().
			Foreground(theme.SuccessFg),

		FileRemoved: lipgloss.NewStyle().
			Foreground(theme.ErrorFg),

		FileModified: lipgloss.NewStyle().
			Foreground(theme.WarnFg),

		Label: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Background(theme.BorderDim).
			Padding(0, 1),
	}
}
