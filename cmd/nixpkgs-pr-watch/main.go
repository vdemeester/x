package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.sbr.pm/x/internal/output"
)

var version = "0.1.0"

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	out := output.Default()

	var (
		host          string
		allHosts      bool
		flakePath     string
		limit         int
		outputFormat  string
		minConfidence string
		user          string
		refreshDeps   bool
		refreshPRs    bool
		refresh       bool
	)

	cmd := &cobra.Command{
		Use:   "nixpkgs-pr-watch",
		Short: "Filter NixOS/nixpkgs PRs based on your system configuration",
		Long: `nixpkgs-pr-watch analyzes your NixOS/home-manager configurations to extract
dependencies, then filters nixpkgs pull requests to show only those relevant
to your system. Helps track updates and security fixes for packages you use.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(out, watchFlags{
				host:          host,
				allHosts:      allHosts,
				flakePath:     flakePath,
				limit:         limit,
				outputFormat:  outputFormat,
				minConfidence: minConfidence,
				user:          user,
				refreshDeps:   refreshDeps || refresh,
				refreshPRs:    refreshPRs || refresh,
			})
		},
	}

	cmd.Flags().StringVar(&host, "host", "", "Analyze specific host (default: current host)")
	cmd.Flags().BoolVar(&allHosts, "all-hosts", false, "Analyze all hosts in flake")
	cmd.Flags().StringVar(&flakePath, "flake", ".", "Path to flake directory")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of PRs to fetch")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "terminal", "Output format (terminal, json)")
	cmd.Flags().StringVar(&minConfidence, "min-confidence", "medium", "Minimum confidence level (high, medium, low)")
	cmd.Flags().StringVar(&user, "user", "", "Filter PRs by author username (e.g., r-ryantm)")
	cmd.Flags().BoolVar(&refreshDeps, "refresh-deps", false, "Refresh dependency cache")
	cmd.Flags().BoolVar(&refreshPRs, "refresh-prs", false, "Refresh PR cache")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh all caches")

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(cacheCmd(out))

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nixpkgs-pr-watch version %s\n", version)
		},
	}
}

type watchFlags struct {
	host          string
	allHosts      bool
	flakePath     string
	limit         int
	outputFormat  string
	minConfidence string
	user          string
	refreshDeps   bool
	refreshPRs    bool
}
