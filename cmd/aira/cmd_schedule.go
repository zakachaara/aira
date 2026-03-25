package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/scheduler"
)

func scheduleCmd() *cobra.Command {
	var runNow bool

	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Start the AIRA background scheduler",
		Long: `Starts the long-running AIRA daemon. Two cron jobs are registered:

  collect job  – fetch, parse, classify, extract signals, detect trends
  digest job   – generate the daily intelligence digest

Cron expressions are read from config (schedule.collect / schedule.digest).
Defaults: collect every 4 hours, digest daily at 08:00 local time.

The process runs until it receives SIGINT or SIGTERM.`,
		Example: `  aira schedule
  aira schedule --run-now   # run full pipeline once then enter schedule loop`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()

			s := scheduler.New(app.repo, app.cfg)

			if runNow {
				fmt.Println("\n  ▶ Running full pipeline immediately (--run-now)…\n")
				if err := s.RunNow(ctx); err != nil {
					return err
				}
			}

			if err := s.Start(); err != nil {
				return fmt.Errorf("starting scheduler: %w", err)
			}

			fmt.Printf("\n  ✓ AIRA scheduler running\n")
			fmt.Printf("    Collect:  %s\n", app.cfg.Schedule.Collect)
			fmt.Printf("    Digest:   %s\n", app.cfg.Schedule.Digest)
			fmt.Println("  Send SIGINT (Ctrl+C) to stop.\n")

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			logger.Info("shutdown signal received")
			s.Stop()
			fmt.Println("\n  AIRA scheduler stopped.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&runNow, "run-now", false, "run full pipeline immediately before entering schedule loop")
	return cmd
}

// ─────────────────────────────────────────────

var (
	Version   = "1.0.0"
	GitCommit = "dev"
	BuildDate = "unknown"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print AIRA version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("AIRA %s (commit=%s, built=%s)\n", Version, GitCommit, BuildDate)
		},
	}
}
