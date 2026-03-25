package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/delivery"
	"github.com/aira/aira/internal/parser"
)

func parseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "parse",
		Short: "Normalize and structure raw feed items into unified entries",
		Long: `Reads all raw entries that have not yet been parsed, normalises their
content (strips HTML entities, extracts authors and domain tags, truncates
summaries) and saves them as structured Entry records ready for classification.`,
		Example: `  aira parse
  aira parse --log-level debug`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app := getApp(cmd)
			p := parser.New(app.repo)
			stats, err := p.Run(cmd.Context())
			if err != nil {
				return fmt.Errorf("parse failed: %w", err)
			}
			delivery.PrintParseStats(stats)
			return nil
		},
	}
}
