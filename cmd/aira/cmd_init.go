package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Bootstrap AIRA: create config, migrate database, and seed default sources",
		Long: `Sets up a fresh AIRA installation in a single step:

  1. Creates ~/.aira/ directory if it does not exist
  2. Runs all database migrations (creates tables)
  3. Seeds the built-in curated list of RSS sources

This command is idempotent — safe to re-run. Existing data is preserved.`,
		Example: `  aira init
  aira init --config /etc/aira/config.yaml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := getApp(cmd)

			if err := seedDefaultSources(app.repo); err != nil {
				return fmt.Errorf("seeding default sources: %w", err)
			}

			fmt.Println()
			fmt.Println("  ✓ AIRA initialised successfully")
			fmt.Printf("  ✓ Database  : %s\n", app.cfg.Database.DSN)
			fmt.Printf("  ✓ Sources   : %d default feeds seeded\n", len(defaultSources()))
			fmt.Println()
			fmt.Println("  Quick-start:")
			fmt.Println("    aira sources list            # review configured feeds")
			fmt.Println("    aira collect                 # fetch from all active sources")
			fmt.Println("    aira parse && aira classify  # normalise and categorise")
			fmt.Println("    aira signals --extract       # detect high-value events")
			fmt.Println("    aira trends  --detect        # surface emerging topics")
			fmt.Println("    aira digest                  # generate your first digest")
			fmt.Println()
			return nil
		},
	}
}
