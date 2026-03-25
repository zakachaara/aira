package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/analyzer"
	"github.com/aira/aira/internal/delivery"
)

func trendsCmd() *cobra.Command {
	var window string
	var runDetection bool
	var limit int

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Analyze frequency and detect emerging topics",
		Long: `Analyses entry frequency by topic across a time window to detect
emerging or accelerating topics in AI research, model releases,
AI infrastructure, and cloud-native tooling.

A velocity score reflects the growth rate: topics accelerating in the
most recent half of the window score higher than stable or declining ones.

Supported windows: 24h, 7d`,
		Example: `  aira trends
  aira trends --window 7d
  aira trends --detect --window 24h
  aira trends --limit 15`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()

			if runDetection {
				a := analyzer.New(app.repo)
				trends, err := a.DetectTrends(ctx)
				if err != nil {
					return fmt.Errorf("trend detection failed: %w", err)
				}
				fmt.Printf("\n  📈 Detected %d trend(s) for window %q\n", len(trends), window)
			}

			trends, err := app.repo.ListTrends(ctx, window, limit)
			if err != nil {
				return fmt.Errorf("listing trends: %w", err)
			}

			delivery.PrintTrends(os.Stdout, trends, window)
			return nil
		},
	}

	cmd.Flags().StringVar(&window, "window", "7d", "time window: 24h or 7d")
	cmd.Flags().BoolVar(&runDetection, "detect", false, "run trend detection before listing")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum trends to display")
	return cmd
}
