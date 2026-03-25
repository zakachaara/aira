// Package digest generates structured intelligence reports from processed entries.
package digest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

// Config controls digest generation behaviour.
type Config struct {
	MaxEntriesPerSection int
	TrendWindowDays      int
}

// Generator produces daily intelligence digests.
type Generator struct {
	repo storage.Repository
	cfg  Config
}

// New creates a Generator.
func New(repo storage.Repository, cfg Config) *Generator {
	return &Generator{repo: repo, cfg: cfg}
}

// Generate builds and persists a digest for the given time window.
func (g *Generator) Generate(ctx context.Context, since time.Time) (*models.Digest, error) {
	logger.Info("generating digest", zap.Time("since", since))

	now := time.Now().UTC()
	maxPer := g.cfg.MaxEntriesPerSection
	if maxPer == 0 {
		maxPer = 10
	}

	// ── Fetch data ──────────────────────────────────
	totalCount, err := g.repo.CountEntries(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("counting entries: %w", err)
	}

	aiResearch, err := g.repo.ListEntries(ctx, storage.EntryQuery{
		Category: models.CategoryAIResearch, Since: since, Limit: maxPer,
	})
	if err != nil {
		return nil, err
	}

	modelReleases, err := g.repo.ListEntries(ctx, storage.EntryQuery{
		Category: models.CategoryModelRelease, Since: since, Limit: maxPer,
	})
	if err != nil {
		return nil, err
	}

	aiInfra, err := g.repo.ListEntries(ctx, storage.EntryQuery{
		Category: models.CategoryAIInfrastructure, Since: since, Limit: maxPer,
	})
	if err != nil {
		return nil, err
	}

	cloudNative, err := g.repo.ListEntries(ctx, storage.EntryQuery{
		Category: models.CategoryCloudNative, Since: since, Limit: maxPer,
	})
	if err != nil {
		return nil, err
	}

	signals, err := g.repo.ListSignals(ctx, since, 20)
	if err != nil {
		return nil, err
	}

	windowStr := "7d"
	if time.Since(since) < 25*time.Hour {
		windowStr = "24h"
	}
	trends, err := g.repo.ListTrends(ctx, windowStr, 10)
	if err != nil {
		return nil, err
	}

	// ── Build sections ──────────────────────────────
	sections := []models.DigestSection{
		buildSection("🔬 AI Research Highlights", aiResearch),
		buildSection("🚀 Model Releases", modelReleases),
		buildSection("⚙️  AI Infrastructure", aiInfra),
		buildSection("☁️  Cloud-Native Ecosystem", cloudNative),
	}

	// ── Render markdown ─────────────────────────────
	md := renderMarkdown(sections, signals, trends, since, now, totalCount)

	digest := &models.Digest{
		GeneratedAt:  now,
		DateRange:    fmt.Sprintf("%s – %s", since.Format("2006-01-02 15:04"), now.Format("2006-01-02 15:04")),
		TotalEntries: totalCount,
		Sections:     sections,
		Signals:      dereferSignals(signals),
		Trends:       dereferTrends(trends),
		Markdown:     md,
	}

	if err := g.repo.SaveDigest(ctx, digest); err != nil {
		return nil, fmt.Errorf("saving digest: %w", err)
	}

	logger.Info("digest generated",
		zap.Int("total_entries", totalCount),
		zap.Int("signals", len(signals)),
		zap.Int("trends", len(trends)))
	return digest, nil
}

// ─────────────────────────────────────────────
//  Section & Markdown helpers
// ─────────────────────────────────────────────

func buildSection(title string, entries []*models.Entry) models.DigestSection {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		authors := ""
		if len(e.Authors) > 0 {
			a := e.Authors
			if len(a) > 3 {
				a = append(a[:3], "et al.")
			}
			authors = " — " + strings.Join(a, ", ")
		}
		tagStr := ""
		if len(e.Tags) > 0 {
			tags := e.Tags
			if len(tags) > 4 {
				tags = tags[:4]
			}
			tagStr = " `" + strings.Join(tags, "` `") + "`"
		}
		line := fmt.Sprintf("- **[%s](%s)**%s%s",
			e.Title, e.Link, authors, tagStr)
		if e.Summary != "" {
			summary := truncate(e.Summary, 200)
			line += "\n  " + summary
		}
		lines = append(lines, line)
	}
	return models.DigestSection{Title: title, Entries: lines}
}

func renderMarkdown(
	sections []models.DigestSection,
	signals []*models.Signal,
	trends []*models.Trend,
	since, now time.Time,
	total int,
) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# AIRA Intelligence Digest\n\n")
	sb.WriteString(fmt.Sprintf("> 📅 **Period:** %s → %s\n",
		since.Format("Mon 02 Jan 2006 15:04 UTC"),
		now.Format("Mon 02 Jan 2006 15:04 UTC")))
	sb.WriteString(fmt.Sprintf("> 📊 **Total entries analysed:** %d\n\n", total))
	sb.WriteString("---\n\n")

	// Sections
	for _, sec := range sections {
		if len(sec.Entries) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", sec.Title))
		for _, line := range sec.Entries {
			sb.WriteString(line)
			sb.WriteString("\n\n")
		}
	}

	// Signals
	if len(signals) > 0 {
		sb.WriteString("---\n\n## ⚡ High-Value Signals\n\n")
		sb.WriteString("| Score | Type | Description |\n")
		sb.WriteString("|------:|------|-------------|\n")
		for _, s := range signals {
			sb.WriteString(fmt.Sprintf("| %.2f | `%s` | %s |\n",
				s.Score, s.Type, mdEscape(s.Description)))
		}
		sb.WriteString("\n")
	}

	// Trends
	if len(trends) > 0 {
		sb.WriteString("---\n\n## 📈 Trending Topics\n\n")
		sb.WriteString("| Rank | Topic | Category | Freq | Velocity |\n")
		sb.WriteString("|-----:|-------|----------|-----:|---------:|\n")
		for i, t := range trends {
			sb.WriteString(fmt.Sprintf("| %d | %s | `%s` | %d | %.3f |\n",
				i+1, t.Topic, t.Category, t.Frequency, t.Velocity))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("*Generated by AIRA at %s*\n", now.Format(time.RFC1123)))
	return sb.String()
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return truncate(s, 120)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func dereferSignals(in []*models.Signal) []models.Signal {
	out := make([]models.Signal, 0, len(in))
	for _, s := range in {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

func dereferTrends(in []*models.Trend) []models.Trend {
	out := make([]models.Trend, 0, len(in))
	for _, t := range in {
		if t != nil {
			out = append(out, *t)
		}
	}
	return out
}
