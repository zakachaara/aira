package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/analyzer"
	"github.com/aira/aira/internal/delivery"
)

func signalsCmd() *cobra.Command {
	var hours int
	var runExtraction bool
	var limit int

	cmd := &cobra.Command{
		Use:   "signals",
		Short: "Extract and display meaningful events",
		Long: `Detects high-value events from classified entries such as:
  • model_release          – New weights, APIs, or checkpoints announced
  • research_breakthrough  – SOTA results, novel approaches, significant advances
  • infrastructure_release – New versions of MLOps or cloud-native tooling
  • dataset_release        – New training or evaluation datasets
  • benchmark_result       – Performance metrics reported on known benchmarks

Use --extract to run a fresh detection pass before listing.`,
		Example: `  aira signals
  aira signals --hours 48
  aira signals --extract
  aira signals --extract --hours 72 --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()
			since := time.Now().Add(-time.Duration(hours) * time.Hour)

			if runExtraction {
				a := analyzer.New(app.repo)
				count, err := a.ExtractSignals(ctx, since)
				if err != nil {
					return fmt.Errorf("signal extraction failed: %w", err)
				}
				fmt.Printf("\n  ⚡ Extracted %d new signal(s)\n", count)
			}

			signals, err := app.repo.ListSignals(ctx, since, limit)
			if err != nil {
				return fmt.Errorf("listing signals: %w", err)
			}

			fmt.Printf("\n  Signals from last %dh  (showing up to %d)\n\n", hours, limit)
			delivery.PrintSignals(os.Stdout, signals)
			return nil
		},
	}

	cmd.Flags().IntVar(&hours, "hours", 24, "look-back window in hours")
	cmd.Flags().BoolVar(&runExtraction, "extract", false, "run signal extraction before listing")
	cmd.Flags().IntVar(&limit, "limit", 40, "maximum signals to display")
	return cmd
}
