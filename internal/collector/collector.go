// Package collector fetches RSS/Atom feeds and persists raw entries.
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"go.uber.org/zap"

	"github.com/aira/aira/internal/config"
	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

// Collector fetches feeds and persists raw entries.
type Collector struct {
	repo   storage.Repository
	cfg    config.CollectConfig
	parser *gofeed.Parser
}

// New creates a Collector with the given repository and config.
func New(repo storage.Repository, cfg config.CollectConfig) *Collector {
	fp := gofeed.NewParser()
	fp.UserAgent = cfg.UserAgent
	fp.Client = &http.Client{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	}
	return &Collector{repo: repo, cfg: cfg, parser: fp}
}

// Run fetches all active sources concurrently and returns collection statistics.
func (c *Collector) Run(ctx context.Context) (*models.CollectStats, error) {
	sources, err := c.repo.ListSources(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("listing sources: %w", err)
	}
	if len(sources) == 0 {
		logger.Info("no active sources configured – skipping collect")
		return &models.CollectStats{}, nil
	}

	sem := make(chan struct{}, maxInt(c.cfg.MaxConcurrent, 1))
	type result struct {
		sourceID int64
		count    int
		err      error
	}
	results := make(chan result, len(sources))
	var wg sync.WaitGroup

	for _, src := range sources {
		src := src // capture
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			count, err := c.fetchSource(ctx, src)
			results <- result{sourceID: src.ID, count: count, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	stats := &models.CollectStats{SourcesFetched: len(sources)}
	for res := range results {
		if res.err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("source %d: %v", res.sourceID, res.err))
			logger.Error("fetch error", zap.Int64("source_id", res.sourceID), zap.Error(res.err))
		} else {
			stats.NewEntries += res.count
		}
	}
	return stats, nil
}

// fetchSource fetches a single source with retry logic.
func (c *Collector) fetchSource(ctx context.Context, src *models.Source) (int, error) {
	var feed *gofeed.Feed
	var err error

	for attempt := 0; attempt <= c.cfg.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(time.Duration(attempt*2) * time.Second):
			}
		}
		feed, err = c.parser.ParseURLWithContext(src.URL, ctx)
		if err == nil {
			break
		}
		logger.Warn("fetch attempt failed",
			zap.String("source", src.Name),
			zap.Int("attempt", attempt+1),
			zap.Error(err))
	}
	if err != nil {
		return 0, fmt.Errorf("after %d attempts: %w", c.cfg.RetryAttempts+1, err)
	}

	count := 0
	for _, item := range feed.Items {
		guid := resolveGUID(item)
		exists, err := c.repo.GUIDExists(ctx, guid)
		if err != nil {
			logger.Error("guid check failed", zap.String("guid", guid), zap.Error(err))
			continue
		}
		if exists {
			continue
		}

		raw, err := json.Marshal(item)
		if err != nil {
			raw = []byte("{}")
		}

		entry := &models.RawEntry{
			SourceID:    src.ID,
			SourceName:  src.Name,
			GUID:        guid,
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Content:     item.Content,
			Authors:     authorsString(item.Authors),
			Published:   resolvePublished(item),
			RawPayload:  string(raw),
		}

		if _, err := c.repo.SaveRawEntry(ctx, entry); err != nil {
			logger.Error("saving raw entry", zap.String("guid", guid), zap.Error(err))
			continue
		}
		count++
	}

	_ = c.repo.UpdateSourceFetchTime(ctx, src.ID, time.Now())
	logger.Info("source fetched",
		zap.String("source", src.Name),
		zap.Int("new_entries", count))
	return count, nil
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func resolveGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	if item.Link != "" {
		return item.Link
	}
	return item.Title
}

func resolvePublished(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Now()
}

func authorsString(authors []*gofeed.Person) string {
	if len(authors) == 0 {
		return ""
	}
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	b, _ := json.Marshal(names)
	return string(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
