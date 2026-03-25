package digest_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aira/aira/internal/digest"
	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

// ─────────────────────────────────────────────
//  Mock repository
// ─────────────────────────────────────────────

type mockRepo struct {
	storage.Repository
	entries []*models.Entry
	signals []*models.Signal
	trends  []*models.Trend
	digests []*models.Digest
}

func (m *mockRepo) CountEntries(_ context.Context, since time.Time) (int, error) {
	count := 0
	for _, e := range m.entries {
		if e.Published.After(since) {
			count++
		}
	}
	return count, nil
}

func (m *mockRepo) ListEntries(_ context.Context, q storage.EntryQuery) ([]*models.Entry, error) {
	var out []*models.Entry
	for _, e := range m.entries {
		if q.Category != "" && e.Category != q.Category {
			continue
		}
		if !q.Since.IsZero() && e.Published.Before(q.Since) {
			continue
		}
		out = append(out, e)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}

func (m *mockRepo) ListSignals(_ context.Context, since time.Time, limit int) ([]*models.Signal, error) {
	var out []*models.Signal
	for _, s := range m.signals {
		if s.DetectedAt.After(since) {
			out = append(out, s)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *mockRepo) ListTrends(_ context.Context, _ string, limit int) ([]*models.Trend, error) {
	out := m.trends
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *mockRepo) SaveDigest(_ context.Context, d *models.Digest) error {
	d.ID = int64(len(m.digests)) + 1
	m.digests = append(m.digests, d)
	return nil
}

func (m *mockRepo) GetLatestDigest(_ context.Context) (*models.Digest, error) {
	if len(m.digests) == 0 {
		return nil, nil
	}
	return m.digests[len(m.digests)-1], nil
}

// ─────────────────────────────────────────────
//  Tests
// ─────────────────────────────────────────────

func TestGenerate_ProducesDigest(t *testing.T) {
	since := time.Now().Add(-24 * time.Hour)
	repo := &mockRepo{
		entries: []*models.Entry{
			{ID: 1, Title: "Attention Is All You Need", Link: "https://arxiv.org/1",
				Category: models.CategoryAIResearch, Published: time.Now(), Summary: "Transformer paper"},
			{ID: 2, Title: "GPT-5 Released", Link: "https://openai.com/blog/gpt5",
				Category: models.CategoryModelRelease, Published: time.Now(), Summary: "New model"},
			{ID: 3, Title: "vLLM v0.5 Ships", Link: "https://blog.vllm.ai/v05",
				Category: models.CategoryAIInfrastructure, Published: time.Now()},
			{ID: 4, Title: "Kubernetes 1.30 Released", Link: "https://k8s.io/blog/130",
				Category: models.CategoryCloudNative, Published: time.Now()},
		},
		signals: []*models.Signal{
			{ID: 1, EntryID: 2, Type: models.SignalModelRelease,
				Description: "GPT-5 Released", Score: 0.95, DetectedAt: time.Now()},
		},
		trends: []*models.Trend{
			{ID: 1, Topic: "LLM Agents", Category: models.CategoryAIResearch,
				Velocity: 0.8, Frequency: 24, Window: "7d"},
		},
	}

	gen := digest.New(repo, digest.Config{
		MaxEntriesPerSection: 5,
		TrendWindowDays:      7,
	})

	d, err := gen.Generate(context.Background(), since)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if d.ID == 0 {
		t.Error("expected non-zero digest ID")
	}
	if d.TotalEntries == 0 {
		t.Error("expected non-zero TotalEntries")
	}
	if d.Markdown == "" {
		t.Error("expected non-empty Markdown")
	}
	if len(d.Sections) == 0 {
		t.Error("expected at least one section")
	}
}

func TestGenerate_MarkdownStructure(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			{ID: 1, Title: "Research Paper A", Link: "https://arxiv.org/abs/001",
				Category: models.CategoryAIResearch, Published: time.Now()},
		},
	}
	gen := digest.New(repo, digest.Config{MaxEntriesPerSection: 5, TrendWindowDays: 7})

	d, err := gen.Generate(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	md := d.Markdown

	requiredPhrases := []string{
		"AIRA",
		"Intelligence Digest",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(md, phrase) {
			t.Errorf("markdown missing %q\n---\n%s", phrase, md[:min(200, len(md))])
		}
	}
}

func TestGenerate_EmptyRepo(t *testing.T) {
	repo := &mockRepo{}
	gen := digest.New(repo, digest.Config{MaxEntriesPerSection: 10, TrendWindowDays: 7})
	d, err := gen.Generate(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Generate on empty repo: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil digest even for empty repo")
	}
	if d.Markdown == "" {
		t.Error("expected non-empty markdown even for empty digest")
	}
}

func TestGenerate_SectionTitles(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			{ID: 1, Title: "AI Paper", Category: models.CategoryAIResearch,
				Link: "https://x.com", Published: time.Now()},
			{ID: 2, Title: "New Model", Category: models.CategoryModelRelease,
				Link: "https://y.com", Published: time.Now()},
		},
	}
	gen := digest.New(repo, digest.Config{MaxEntriesPerSection: 5, TrendWindowDays: 7})
	d, err := gen.Generate(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	titles := make(map[string]bool)
	for _, s := range d.Sections {
		if len(s.Entries) > 0 {
			titles[s.Title] = true
		}
	}
	if !titles["🔬 AI Research Highlights"] && !titles["🚀 Model Releases"] {
		t.Errorf("expected research and model release sections; got sections: %v", sectionTitles(d.Sections))
	}
}

func TestGenerate_PersistsToRepo(t *testing.T) {
	repo := &mockRepo{}
	gen := digest.New(repo, digest.Config{MaxEntriesPerSection: 5, TrendWindowDays: 7})
	_, err := gen.Generate(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.digests) != 1 {
		t.Errorf("expected 1 digest persisted, got %d", len(repo.digests))
	}
}

func TestGenerate_DateRange(t *testing.T) {
	repo := &mockRepo{}
	gen := digest.New(repo, digest.Config{MaxEntriesPerSection: 5, TrendWindowDays: 7})
	since := time.Now().Add(-48 * time.Hour)
	d, err := gen.Generate(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if d.DateRange == "" {
		t.Error("expected non-empty DateRange")
	}
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func sectionTitles(sections []models.DigestSection) []string {
	titles := make([]string, 0, len(sections))
	for _, s := range sections {
		titles = append(titles, s.Title)
	}
	return titles
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
