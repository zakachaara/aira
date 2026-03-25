package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/aira/aira/internal/delivery"
	digestpkg "github.com/aira/aira/internal/digest"
)

func digestCmd() *cobra.Command {
	var days int
	var outputFile string
	var listOnly bool
	var listLimit int
	var showLatest bool

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Generate a human-readable daily intelligence report",
		Long: `Produces a structured Markdown intelligence digest covering:
  • AI Research Highlights  – Top papers and research advances
  • Model Releases          – New model weights, APIs, and checkpoints
  • AI Infrastructure       – MLOps, serving, and tooling updates
  • Cloud-Native Ecosystem  – Kubernetes, CNCF, eBPF, and more
  • High-Value Signals      – Detected events ranked by relevance score
  • Trending Topics         – Emerging and accelerating topic areas

The digest is saved to the database and optionally written to a file.`,
		Example: `  aira digest
  aira digest --days 1
  aira digest --output ~/reports/aira-$(date +%F).md
  aira digest --list
  aira digest --show-latest`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := getApp(cmd)
			ctx := cmd.Context()

			if listOnly {
				digests, err := app.repo.ListDigests(ctx, listLimit)
				if err != nil {
					return err
				}
				delivery.PrintDigestList(os.Stdout, digests)
				return nil
			}

			if showLatest {
				d, err := app.repo.GetLatestDigest(ctx)
				if err != nil {
					return fmt.Errorf("no digest found – run: aira digest")
				}
				delivery.PrintDigest(os.Stdout, d)
				return nil
			}

			since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			gen := digestpkg.New(app.repo, digestpkg.Config{
				MaxEntriesPerSection: app.cfg.Digest.MaxEntriesPerSection,
				TrendWindowDays:      app.cfg.Digest.TrendWindowDays,
			})

			d, err := gen.Generate(ctx, since)
			if err != nil {
				return fmt.Errorf("digest generation failed: %w", err)
			}

			// Write to explicit file path if requested (markdown)
			if outputFile != "" {
				if err := writeDigestFile(outputFile, d.Markdown); err != nil {
					fmt.Fprintf(os.Stderr, "⚠  could not write markdown file: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "  ✓ Markdown written to %s\n\n", outputFile)
				}
			}

			// Auto-save both .md and .html to the configured output dir
			outDir := app.cfg.Digest.OutputDir
			if outDir == "" {
				home, _ := os.UserHomeDir()
				outDir = filepath.Join(home, ".aira", "digests")
			}
			ts := d.GeneratedAt.Format("2006-01-02T150405")

			mdPath := filepath.Join(outDir, fmt.Sprintf("aira-digest-%s.md", ts))
			if err := writeDigestFile(mdPath, d.Markdown); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠  could not save markdown: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  💾 Markdown → %s\n", mdPath)
			}

			htmlPath := filepath.Join(outDir, fmt.Sprintf("aira-digest-%s.html", ts))
			if err := writeDigestFile(htmlPath, d.HTML); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠  could not save HTML report: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "  🌐 HTML    → %s\n\n", htmlPath)
			}

			delivery.PrintDigest(os.Stdout, d)
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 1, "number of past days to include in the digest")
	cmd.Flags().StringVar(&outputFile, "output", "", "write digest to this file path (Markdown)")
	cmd.Flags().BoolVar(&listOnly, "list", false, "list previously generated digests")
	cmd.Flags().IntVar(&listLimit, "list-limit", 10, "number of digests to show with --list")
	cmd.Flags().BoolVar(&showLatest, "show-latest", false, "display the most recently generated digest")
	return cmd
}

func writeDigestFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
