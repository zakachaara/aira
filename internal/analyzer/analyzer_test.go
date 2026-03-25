package analyzer_test

import (
	"context"
	"testing"
	"time"

	"github.com/zakachaara/aira/internal/analyzer"
	"github.com/zakachaara/aira/internal/models"
	"github.com/zakachaara/aira/internal/storage"
)

// ─────────────────────────────────────────────
//  Mock repository
// ─────────────────────────────────────────────

type mockRepo struct {
	storage.Repository
	entries []*models.Entry
	signals []*models.Signal
	trends  []*models.Trend
}

func (m *mockRepo) ListEntries(_ context.Context, q storage.EntryQuery) ([]*models.Entry, error) {
	var out []*models.Entry
	for _, e := range m.entries {
		if !q.Since.IsZero() && e.Published.Before(q.Since) {
			continue
		}
		if q.Category != "" && e.Category != q.Category {
			continue
		}
		out = append(out, e)
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}

func (m *mockRepo) SaveSignal(_ context.Context, s *models.Signal) error {
	s.ID = int64(len(m.signals)) + 1
	m.signals = append(m.signals, s)
	return nil
}

func (m *mockRepo) SaveTrend(_ context.Context, t *models.Trend) error {
	t.ID = int64(len(m.trends)) + 1
	m.trends = append(m.trends, t)
	return nil
}

func (m *mockRepo) ListTrends(_ context.Context, window string, limit int) ([]*models.Trend, error) {
	var out []*models.Trend
	for _, t := range m.trends {
		if t.Window == window {
			out = append(out, t)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ─────────────────────────────────────────────
//  Signal extraction tests
// ─────────────────────────────────────────────

func makeEntry(title, summary string, cat models.Category) *models.Entry {
	return &models.Entry{
		ID:        1,
		Title:     title,
		Summary:   summary,
		Category:  cat,
		Published: time.Now(),
	}
}

func TestExtractSignals_ModelRelease(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			makeEntry(
				"Introducing GPT-X: our latest model release",
				"We are announcing our new open weights checkpoint, available now via API.",
				models.CategoryModelRelease,
			),
		},
	}
	a := analyzer.New(repo)
	count, err := a.ExtractSignals(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ExtractSignals: %v", err)
	}
	if count == 0 {
		t.Error("expected at least one signal for model release entry")
	}
	found := false
	for _, s := range repo.signals {
		if s.Type == models.SignalModelRelease {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected model_release signal; got types: %v", signalTypes(repo.signals))
	}
}

func TestExtractSignals_ResearchBreakthrough(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			makeEntry(
				"State-of-the-art results on MMLU: a novel approach that surpasses all prior work",
				"We achieve a new record score, outperforms GPT-4 on multiple benchmarks.",
				models.CategoryAIResearch,
			),
		},
	}
	a := analyzer.New(repo)
	_, err := a.ExtractSignals(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, s := range repo.signals {
		if s.Type == models.SignalResearchBreakthrough {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected research_breakthrough signal; got: %v", signalTypes(repo.signals))
	}
}

func TestExtractSignals_IrrelevantEntry(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			makeEntry("Generic Tuesday Update", "Nothing of note happened today.", models.CategoryUncategorized),
		},
	}
	a := analyzer.New(repo)
	count, err := a.ExtractSignals(context.Background(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// May or may not produce a signal; just verify no panic and clean return.
	t.Logf("signals from generic entry: %d", count)
}

func TestExtractSignals_ScoreRange(t *testing.T) {
	repo := &mockRepo{
		entries: []*models.Entry{
			makeEntry(
				"New model release: open weights available now",
				"Announcing the release of our latest checkpoint. Version 2.0 general availability.",
				models.CategoryModelRelease,
			),
		},
	}
	a := analyzer.New(repo)
	_, _ = a.ExtractSignals(context.Background(), time.Now().Add(-time.Hour))
	for _, s := range repo.signals {
		if s.Score < 0 || s.Score > 1 {
			t.Errorf("signal score %f out of [0,1] range", s.Score)
		}
	}
}

// ─────────────────────────────────────────────
//  Trend detection tests
// ─────────────────────────────────────────────

func TestDetectTrends_ReturnsResults(t *testing.T) {
	// Seed entries mentioning "agent" / "agentic" frequently
	var entries []*models.Entry
	for i := 0; i < 10; i++ {
		entries = append(entries, &models.Entry{
			ID:        int64(i + 1),
			Title:     "Agentic AI frameworks for autonomous tool use",
			Summary:   "Building AI agents with function calling and tool integration.",
			Category:  models.CategoryAIResearch,
			Published: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}

	repo := &mockRepo{entries: entries}
	a := analyzer.New(repo)

	trends, err := a.DetectTrends(context.Background())
	if err != nil {
		t.Fatalf("DetectTrends: %v", err)
	}
	if len(trends) == 0 {
		t.Error("expected at least one trend from repeated agent-related entries")
	}

	// Check that saved trends have required fields
	for _, tr := range repo.trends {
		if tr.Topic == "" {
			t.Error("trend has empty topic")
		}
		if tr.Window == "" {
			t.Error("trend has empty window")
		}
		if tr.Frequency < 0 {
			t.Errorf("trend frequency %d is negative", tr.Frequency)
		}
	}
}

func TestDetectTrends_EmptyRepo(t *testing.T) {
	repo := &mockRepo{}
	a := analyzer.New(repo)
	trends, err := a.DetectTrends(context.Background())
	if err != nil {
		t.Fatalf("DetectTrends on empty repo: %v", err)
	}
	// Empty repo should return empty trends, not error
	t.Logf("trends from empty repo: %d", len(trends))
}

func TestDetectTrends_VelocityIsNonNegative(t *testing.T) {
	var entries []*models.Entry
	for i := 0; i < 5; i++ {
		entries = append(entries, &models.Entry{
			ID:        int64(i + 1),
			Title:     "Kubernetes operator for ML workloads",
			Category:  models.CategoryCloudNative,
			Published: time.Now().Add(-time.Duration(i) * 2 * time.Hour),
		})
	}
	repo := &mockRepo{entries: entries}
	a := analyzer.New(repo)
	_, _ = a.DetectTrends(context.Background())

	for _, tr := range repo.trends {
		if tr.Velocity < 0 {
			t.Errorf("velocity %f is negative for topic %q", tr.Velocity, tr.Topic)
		}
	}
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

func signalTypes(signals []*models.Signal) []models.SignalType {
	types := make([]models.SignalType, 0, len(signals))
	for _, s := range signals {
		types = append(types, s.Type)
	}
	return types
}
