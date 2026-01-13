package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.sbr.pm/x/internal/cache"
	"go.sbr.pm/x/internal/output"
)

func cacheCmd(out *output.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage cache",
		Long:  "Manage dependency and PR caches",
	}

	cmd.AddCommand(cacheClearCmd(out))
	cmd.AddCommand(cacheInfoCmd(out))

	return cmd
}

func cacheClearCmd(out *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear all caches",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.New(24*time.Hour, "nixpkgs-pr-watch")
			if err != nil {
				return fmt.Errorf("failed to initialize cache: %w", err)
			}

			if err := c.Clear(); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			out.Success("Cache cleared")
			return nil
		},
	}
}

func cacheInfoCmd(out *output.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show cache information",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cache.New(24*time.Hour, "nixpkgs-pr-watch")
			if err != nil {
				return fmt.Errorf("failed to initialize cache: %w", err)
			}

			info, err := c.Info()
			if err != nil {
				return fmt.Errorf("failed to get cache info: %w", err)
			}

			out.Info("Cache directory: %s", info.Directory)
			out.Info("Total entries: %d", info.EntryCount)
			out.Info("Total size: %d bytes", info.TotalSize)

			return nil
		},
	}
}
