// Package parser normalises raw feed entries into structured Entry records.
package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"github.com/zakachaara/aira/internal/logger"
	"github.com/zakachaara/aira/internal/models"
	"github.com/zakachaara/aira/internal/storage"
)

const defaultBatchSize = 500

// Parser transforms raw entries into normalised Entry records.
type Parser struct {
	repo      storage.Repository
	tagExtractor *TagExtractor
}

// New creates a Parser backed by repo.
func New(repo storage.Repository) *Parser {
	return &Parser{
		repo:         repo,
		tagExtractor: newTagExtractor(),
	}
}

// Run processes all unparsed raw entries and returns parse statistics.
func (p *Parser) Run(ctx context.Context) (*models.ParseStats, error) {
	stats := &models.ParseStats{}
	start := time.Now()

	raws, err := p.repo.ListUnparsedRaw(ctx, defaultBatchSize)
	if err != nil {
		return nil, fmt.Errorf("listing unparsed raw entries: %w", err)
	}
	logger.Info("starting parse run", zap.Int("raw_count", len(raws)))

	for _, raw := range raws {
		entry, err := p.normalize(raw)
		if err != nil {
			stats.Errors = append(stats.Errors,
				fmt.Sprintf("raw_id=%d: %v", raw.ID, err))
			stats.Skipped++
			continue
		}
		if _, err := p.repo.SaveEntry(ctx, entry); err != nil {
			stats.Errors = append(stats.Errors,
				fmt.Sprintf("save raw_id=%d: %v", raw.ID, err))
			stats.Skipped++
			continue
		}
		stats.Processed++
	}

	stats.Duration = time.Since(start)
	logger.Info("parse run complete",
		zap.Int("processed", stats.Processed),
		zap.Int("skipped", stats.Skipped),
		zap.Duration("duration", stats.Duration))
	return stats, nil
}

// normalize converts a RawEntry into a cleaned, structured Entry.
func (p *Parser) normalize(raw *models.RawEntry) (*models.Entry, error) {
	title := cleanText(raw.Title)
	if title == "" {
		return nil, fmt.Errorf("empty title")
	}

	summary := buildSummary(raw.Description, raw.Content)

	var authors []string
	if raw.Authors != "" {
		_ = json.Unmarshal([]byte(raw.Authors), &authors)
	}
	authors = dedupeStrings(authors)

	tags := p.tagExtractor.Extract(title + " " + summary)

	return &models.Entry{
		RawID:      raw.ID,
		SourceID:   raw.SourceID,
		SourceName: raw.SourceName,
		GUID:       raw.GUID,
		Title:      title,
		Link:       raw.Link,
		Summary:    truncate(summary, 1200),
		Authors:    authors,
		Tags:       tags,
		Published:  raw.Published,
		ParsedAt:   time.Now().UTC(),
		Category:   models.CategoryUncategorized,
	}, nil
}

// ─────────────────────────────────────────────
//  Tag extractor
// ─────────────────────────────────────────────

// TagExtractor extracts domain-relevant keyword tags from free text.
type TagExtractor struct {
	patterns []*tagPattern
}

type tagPattern struct {
	tag  string
	re   *regexp.Regexp
}

func newTagExtractor() *TagExtractor {
	defs := []struct{ tag, pattern string }{
		{"llm", `\bllm[s]?\b|large language model`},
		{"transformer", `\btransformer[s]?\b`},
		{"diffusion", `\bdiffusion model[s]?\b`},
		{"rlhf", `\brlhf\b|reinforcement learning from human feedback`},
		{"fine-tuning", `\bfine.tun(ing|ed)\b`},
		{"benchmark", `\bbenchmark[s]?\b`},
		{"multimodal", `\bmultimodal\b`},
		{"vision", `\bvision model[s]?\b|\bvision.language\b`},
		{"rag", `\brag\b|retrieval.augmented generation`},
		{"agent", `\bagent[s]?\b|agentic`},
		{"kubernetes", `\bkubernetes\b|\bk8s\b`},
		{"wasm", `\bwasm\b|webassembly`},
		{"ebpf", `\bebpf\b`},
		{"opentelemetry", `\bopentelemetry\b|\botel\b`},
		{"service-mesh", `\bservice.mesh\b|\benvoy\b|\bistio\b`},
		{"gpu", `\bgpu[s]?\b|\bcuda\b`},
		{"inference", `\binference\b`},
		{"quantization", `\bquantiz(ation|ed)\b`},
		{"dataset", `\bdataset[s]?\b`},
		{"safety", `\bai.safety\b|\balignment\b`},
		{"open-source", `\bopen.source\b|\bopen-weight\b`},
		{"reasoning", `\breasoning\b|\bchain.of.thought\b`},
	}

	te := &TagExtractor{}
	for _, d := range defs {
		re, err := regexp.Compile(`(?i)` + d.pattern)
		if err != nil {
			continue
		}
		te.patterns = append(te.patterns, &tagPattern{tag: d.tag, re: re})
	}
	return te
}

// Extract returns a deduplicated slice of matched tags from text.
func (te *TagExtractor) Extract(text string) []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, p := range te.patterns {
		if p.re.MatchString(text) {
			if _, ok := seen[p.tag]; !ok {
				seen[p.tag] = struct{}{}
				tags = append(tags, p.tag)
			}
		}
	}
	return tags
}

// ─────────────────────────────────────────────
//  Text helpers
// ─────────────────────────────────────────────

var (
	htmlTagRe     = regexp.MustCompile(`<[^>]*>`)
	multiSpaceRe  = regexp.MustCompile(`\s+`)
	arXivAbstract = regexp.MustCompile(`(?i)abstract[:\s]+`)
)

// cleanText strips HTML tags, decodes entities, and normalises whitespace.
func cleanText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// buildSummary picks the best available text for the summary field.
func buildSummary(description, content string) string {
	text := content
	if text == "" || (len(description) > 0 && len(description) < len(text)) {
		text = description
	}
	return cleanText(text)
}

// truncate returns s truncated to maxRunes runes, appending "…" if cut.
func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}

// dedupeStrings removes duplicate strings while preserving order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
