package main

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/aira/aira/internal/analyzer"
	"github.com/aira/aira/internal/classifier"
	"github.com/aira/aira/internal/collector"
	"github.com/aira/aira/internal/digest"
	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/parser"
	"github.com/aira/aira/internal/storage"
)

// ─────────────────────────────────────────────
//  Default source seeding
// ─────────────────────────────────────────────

// seedDefaultSources writes the built-in curated source list to the repository.
// Conflicts (duplicate URLs) are silently skipped via UPSERT.
func seedDefaultSources(repo storage.Repository) error {
	ctx := context.Background()
	for _, s := range defaultSources() {
		src := s // capture
		if err := repo.SaveSource(ctx, &src); err != nil {
			return err
		}
	}
	return nil
}

// ─────────────────────────────────────────────
//  Synchronous pipeline helpers
// ─────────────────────────────────────────────

// runCollectPipelineOnce runs the full collect→parse→classify→signals→trends
// pipeline synchronously in a single call.
func runCollectPipelineOnce(ctx context.Context, app *appCtx) {
	start := time.Now()
	logger.Info("⟳ collect pipeline starting")

	c := collector.New(app.repo, app.cfg.Collect)
	stats, err := c.Run(ctx)
	if err != nil {
		logger.Error("collect failed", zap.Error(err))
		return
	}
	logger.Info("collect done", zap.Int("new_entries", stats.NewEntries))

	p := parser.New(app.repo)
	pStats, err := p.Run(ctx)
	if err != nil {
		logger.Error("parse failed", zap.Error(err))
		return
	}
	logger.Info("parse done", zap.Int("processed", pStats.Processed))

	cl := classifier.New(app.repo)
	n, err := cl.Run(ctx)
	if err != nil {
		logger.Error("classify failed", zap.Error(err))
		return
	}
	logger.Info("classify done", zap.Int("classified", n))

	a := analyzer.New(app.repo)
	nsig, err := a.ExtractSignals(ctx, time.Now().Add(-6*time.Hour))
	if err != nil {
		logger.Error("signal extraction failed", zap.Error(err))
	} else {
		logger.Info("signals extracted", zap.Int("count", nsig))
	}

	if _, err := a.DetectTrends(ctx); err != nil {
		logger.Error("trend detection failed", zap.Error(err))
	}

	logger.Info("⟳ collect pipeline complete",
		zap.Duration("total", time.Since(start)))
}

// runDigestPipelineOnce generates a digest for the configured look-back window.
func runDigestPipelineOnce(ctx context.Context, app *appCtx) {
	since := time.Now().Add(-time.Duration(app.cfg.Digest.TrendWindowDays) * 24 * time.Hour)
	g := digest.New(app.repo, digest.Config{
		MaxEntriesPerSection: app.cfg.Digest.MaxEntriesPerSection,
		TrendWindowDays:      app.cfg.Digest.TrendWindowDays,
	})
	d, err := g.Generate(ctx, since)
	if err != nil {
		logger.Error("digest generation failed", zap.Error(err))
		return
	}
	logger.Info("digest generated",
		zap.Int64("id", d.ID),
		zap.Int("total_entries", d.TotalEntries))
}
