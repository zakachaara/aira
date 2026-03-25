package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/collector"
	"github.com/aira/aira/internal/delivery"
)

func collectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Fetch RSS feeds and persist raw entries",
		Long: `Connects to all configured active RSS/Atom sources, fetches new items,
and persists them as raw entries in the database.

Duplicate entries are automatically skipped via GUID deduplication.
HTTP fetches are parallelised up to collect.max_concurrent (default 5)
with configurable retries and timeouts.`,
		Example: `  aira collect
  aira collect --log-level debug`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := getApp(cmd)
			c := collector.New(app.repo, app.cfg.Collect)

			start := time.Now()
			stats, err := c.Run(cmd.Context())
			if err != nil {
				return fmt.Errorf("collection failed: %w", err)
			}
			stats.Duration = time.Since(start)
			delivery.PrintCollectStats(stats)
			return nil
		},
	}
	return cmd
}
