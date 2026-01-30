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
	limit     int
	labels    []string
	milestone string
	author    string
	state     string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(&limit, "limit", "l", 100, "Maximum number of PRs to fetch when loading from a repo")
	rootCmd.Flags().StringArrayVarP(&labels, "label", "L", nil, "Filter by label (can be repeated)")
	rootCmd.Flags().StringVarP(&milestone, "milestone", "m", "", "Filter by milestone")
	rootCmd.Flags().StringVarP(&author, "author", "a", "", "Filter by author")
	rootCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state (open, closed, all)")
}

var rootCmd = &cobra.Command{
	Use:   "lazypr [ref] [ref...]",
	Short: "TUI for viewing GitHub pull requests",
	Long: `lazypr is a terminal UI for viewing GitHub pull requests.

When run without arguments in a git repository with a GitHub remote,
automatically detects the repository and loads open PRs.

Accepts references in multiple formats:
  - GitHub URL: https://github.com/owner/repo/pull/123
  - PR format: owner/repo#123
  - Repo format: owner/repo (loads open PRs from repo)

Examples:
  lazypr                                      # Auto-detect from git remote
  lazypr tektoncd/operator                    # Load open PRs from repo
  lazypr tektoncd/operator -l 50              # Load up to 50 PRs
  lazypr -L bug -L "needs-review"             # Filter by multiple labels
  lazypr -a octocat                           # Filter by author
  lazypr -s all                               # Show all PRs (open + closed)
  lazypr tektoncd/pipeline#1234               # Load specific PR
  lazypr owner/repo#1 owner/repo#2            # Load multiple PRs
  lazypr https://github.com/owner/repo/pull/1 # Load from URL`,
	Args: cobra.ArbitraryArgs,
	RunE: runLazyPR,
}

func runLazyPR(cmd *cobra.Command, args []string) error {
	// No args: auto-detect from git remote
	if len(args) == 0 {
		repo, err := lazypr.DetectGitHubRemote()
		if err != nil {
			return fmt.Errorf("failed to detect repository: %w", err)
		}
		return runWithRepoRef(repo)
	}

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
	return runWithRepoRef(repo)
}

func runWithRepoRef(repo lazypr.RepoRef) error {
	// Build filter options from CLI flags
	filter := lazypr.FilterOptions{
		Labels:    labels,
		Milestone: milestone,
		Author:    author,
		State:     state,
	}

	// Create and run the TUI with repo loading
	model := NewRepoModelWithFilter(repo, limit, filter)
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
