// lazypr is a TUI for viewing GitHub pull requests.
package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"go.sbr.pm/x/internal/lazypr"
)

var (
	limit int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of PRs to fetch when loading from a repo")
}

var rootCmd = &cobra.Command{
	Use:   "lazypr <ref> [ref...]",
	Short: "TUI for viewing GitHub pull requests",
	Long: `lazypr is a terminal UI for viewing GitHub pull requests.

Accepts references in multiple formats:
  - GitHub URL: https://github.com/owner/repo/pull/123
  - PR format: owner/repo#123
  - Repo format: owner/repo (loads open PRs from repo)

Examples:
  lazypr tektoncd/operator                    # Load open PRs from repo
  lazypr tektoncd/operator -l 50              # Load up to 50 PRs
  lazypr tektoncd/pipeline#1234               # Load specific PR
  lazypr owner/repo#1 owner/repo#2            # Load multiple PRs
  lazypr https://github.com/owner/repo/pull/1 # Load from URL`,
	Args: cobra.MinimumNArgs(1),
	RunE: runLazyPR,
}

func runLazyPR(cmd *cobra.Command, args []string) error {
	// Check if first arg is a repo reference (no PR number)
	if len(args) == 1 && lazypr.IsRepoRef(args[0]) {
		return runWithRepo(args[0])
	}

	// Parse as PR references
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

func runWithRepo(repoArg string) error {
	repo, err := lazypr.ParseRepoRef(repoArg)
	if err != nil {
		return err
	}

	// Create and run the TUI with repo loading
	model := NewRepoModel(repo, limit)
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
