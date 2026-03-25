// Package scheduler provides cron-based automation for the AIRA pipeline.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/aira/aira/internal/analyzer"
	"github.com/aira/aira/internal/classifier"
	"github.com/aira/aira/internal/collector"
	"github.com/aira/aira/internal/config"
	"github.com/aira/aira/internal/digest"
	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/parser"
	"github.com/aira/aira/internal/storage"
)

// Scheduler wraps a cron runner with the AIRA pipeline stages.
type Scheduler struct {
	cron      *cron.Cron
	repo      storage.Repository
	cfg       *config.Config
	collector *collector.Collector
	parser    *parser.Parser
	classifier *classifier.Classifier
	analyzer  *analyzer.Analyzer
	digest    *digest.Generator
}

// New creates a new Scheduler with the full pipeline wired up.
func New(repo storage.Repository, cfg *config.Config) *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{
		cron:       c,
		repo:       repo,
		cfg:        cfg,
		collector:  collector.New(repo, cfg.Collect),
		parser:     parser.New(repo),
		classifier: classifier.New(repo),
		analyzer:   analyzer.New(repo),
		digest:     digest.New(repo, digest.Config{
			MaxEntriesPerSection: cfg.Digest.MaxEntriesPerSection,
			TrendWindowDays:      cfg.Digest.TrendWindowDays,
		}),
	}
}

// Start registers cron jobs and starts the scheduler.
func (s *Scheduler) Start() error {
	// Collect job
	if s.cfg.Schedule.Collect != "" {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.Collect, func() {
			s.runCollectPipeline()
		}); err != nil {
			return fmt.Errorf("registering collect job (%q): %w", s.cfg.Schedule.Collect, err)
		}
		logger.Info("scheduled collect job", zap.String("cron", s.cfg.Schedule.Collect))
	}

	// Digest job
	if s.cfg.Schedule.Digest != "" {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.Digest, func() {
			s.runDigestPipeline()
		}); err != nil {
			return fmt.Errorf("registering digest job (%q): %w", s.cfg.Schedule.Digest, err)
		}
		logger.Info("scheduled digest job", zap.String("cron", s.cfg.Schedule.Digest))
	}

	s.cron.Start()
	logger.Info("scheduler started")
	return nil
}

// Stop gracefully stops the scheduler and waits for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Info("scheduler stopped")
}

// RunNow executes the full pipeline once synchronously (useful for --now flag).
func (s *Scheduler) RunNow(ctx context.Context) error {
	s.runCollectPipeline()
	s.runDigestPipeline()
	return nil
}

// ─────────────────────────────────────────────
//  Pipeline stages
// ─────────────────────────────────────────────

func (s *Scheduler) runCollectPipeline() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger.Info("⟳ scheduled collect pipeline starting")
	start := time.Now()

	// 1. Collect
	stats, err := s.collector.Run(ctx)
	if err != nil {
		logger.Error("collect failed", zap.Error(err))
		return
	}
	logger.Info("collect done",
		zap.Int("new_entries", stats.NewEntries),
		zap.Int("sources", stats.SourcesFetched))

	// 2. Parse
	pStats, err := s.parser.Run(ctx)
	if err != nil {
		logger.Error("parse failed", zap.Error(err))
		return
	}
	logger.Info("parse done", zap.Int("processed", pStats.Processed))

	// 3. Classify
	classified, err := s.classifier.Run(ctx)
	if err != nil {
		logger.Error("classify failed", zap.Error(err))
		return
	}
	logger.Info("classify done", zap.Int("classified", classified))

	// 4. Extract signals
	since := time.Now().Add(-6 * time.Hour)
	signals, err := s.analyzer.ExtractSignals(ctx, since)
	if err != nil {
		logger.Error("signal extraction failed", zap.Error(err))
	} else {
		logger.Info("signals extracted", zap.Int("count", signals))
	}

	// 5. Trend detection
	if _, err := s.analyzer.DetectTrends(ctx); err != nil {
		logger.Error("trend detection failed", zap.Error(err))
	}

	logger.Info("⟳ collect pipeline complete",
		zap.Duration("total_duration", time.Since(start)))
}

func (s *Scheduler) runDigestPipeline() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("⟳ scheduled digest pipeline starting")
	since := time.Now().Add(-time.Duration(s.cfg.Digest.TrendWindowDays) * 24 * time.Hour)

	d, err := s.digest.Generate(ctx, since)
	if err != nil {
		logger.Error("digest generation failed", zap.Error(err))
		return
	}
	logger.Info("⟳ digest generated",
		zap.Int64("digest_id", d.ID),
		zap.Int("entries", d.TotalEntries))
}
