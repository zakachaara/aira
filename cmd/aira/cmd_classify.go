package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/classifier"
	"github.com/aira/aira/internal/delivery"
	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

func classifyCmd() *cobra.Command {
	var showResults bool
	var limit int

	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Categorize entries using keyword and topic detection",
		Long: `Assigns each unclassified entry a category (ai_research, model_release,
ai_infrastructure, cloud_native) and a confidence score using a
weighted keyword-matching engine.

Categories:
  ai_research       – Papers, studies, experiments, benchmarks
  model_release     – New model weights, API launches, checkpoints
  ai_infrastructure – MLOps, serving, inference, vector DBs
  cloud_native      – Kubernetes, CNCF projects, observability, eBPF`,
		Example: `  aira classify
  aira classify --show
  aira classify --show --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()

			c := classifier.New(app.repo)
			count, err := c.Run(ctx)
			if err != nil {
				return fmt.Errorf("classification failed: %w", err)
			}

			fmt.Printf("\n  ✓ Classified %d entries\n\n", count)

			if showResults {
				entries, err := app.repo.ListEntries(ctx, storage.EntryQuery{
					Category: models.CategoryUncategorized,
					Limit:    limit,
				})
				if err != nil {
					return err
				}
				// Show a sample of recently classified entries across all categories
				all, err := app.repo.ListEntries(ctx, storage.EntryQuery{Limit: limit})
				if err != nil {
					return err
				}
				_ = entries
				delivery.PrintEntries(os.Stdout, all, false)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showResults, "show", false, "print classified entries after run")
	cmd.Flags().IntVar(&limit, "limit", 30, "number of entries to display with --show")
	return cmd
}
