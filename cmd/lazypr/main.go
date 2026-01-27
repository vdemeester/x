// lazypr is a TUI for viewing GitHub pull requests.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
	"go.sbr.pm/x/internal/lazypr"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "lazypr <pr-ref> [pr-ref...]",
	Short: "TUI for viewing GitHub pull requests",
	Long: `lazypr is a terminal UI for viewing GitHub pull requests.

Accepts PR references in multiple formats:
  - GitHub URL: https://github.com/owner/repo/pull/123
  - Short format: owner/repo#123

Examples:
  lazypr https://github.com/tektoncd/pipeline/pull/1234
  lazypr tektoncd/pipeline#1234 tektoncd/pipeline#5678
  lazypr owner/repo#1 owner/repo#2 owner/repo#3`,
	Args: cobra.MinimumNArgs(1),
	RunE: runLazyPR,
}

func runLazyPR(cmd *cobra.Command, args []string) error {
	// Parse PR references
	refs, err := lazypr.ParsePRRefs(args)
	if err != nil {
		return err
	}

	// Create and run the TUI
	model := NewModel(refs)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// runCommand executes a command and returns any error.
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
