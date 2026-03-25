package parser_test

import (
	"context"
	"testing"
	"time"

	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/parser"
	"github.com/aira/aira/internal/storage"
)

// ─────────────────────────────────────────────
//  minimal mock repository for parser tests
// ─────────────────────────────────────────────

type mockRepo struct {
	storage.Repository
	raws    []*models.RawEntry
	entries []*models.Entry
}

func (m *mockRepo) ListUnparsedRaw(_ context.Context, limit int) ([]*models.RawEntry, error) {
	if limit > 0 && limit < len(m.raws) {
		return m.raws[:limit], nil
	}
	return m.raws, nil
}

func (m *mockRepo) SaveEntry(_ context.Context, e *models.Entry) (int64, error) {
	e.ID = int64(len(m.entries)) + 1
	m.entries = append(m.entries, e)
	return e.ID, nil
}

func (m *mockRepo) GUIDExists(_ context.Context, guid string) (bool, error) {
	for _, e := range m.entries {
		if e.GUID == guid {
			return true, nil
		}
	}
	return false, nil
}

// ─────────────────────────────────────────────
//  Tests
// ─────────────────────────────────────────────

func TestParser_Run_NormalisesHTML(t *testing.T) {
	raw := &models.RawEntry{
		ID:          1,
		SourceID:    1,
		SourceName:  "Test Source",
		GUID:        "g-001",
		Title:       "<b>Attention Is All You Need</b>",
		Description: "<p>We propose a new <em>transformer</em> architecture.</p>",
		Published:   time.Now(),
	}
	repo := &mockRepo{raws: []*models.RawEntry{raw}}
	p := parser.New(repo)

	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Processed != 1 {
		t.Errorf("processed: want 1, got %d", stats.Processed)
	}
	if stats.Skipped != 0 {
		t.Errorf("skipped: want 0, got %d", stats.Skipped)
	}

	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry in repo, got %d", len(repo.entries))
	}
	entry := repo.entries[0]

	// HTML tags should be stripped from title
	if entry.Title == raw.Title {
		t.Errorf("title still contains HTML: %q", entry.Title)
	}
	wantTitle := "Attention Is All You Need"
	if entry.Title != wantTitle {
		t.Errorf("title: got %q, want %q", entry.Title, wantTitle)
	}

	// Summary should contain cleaned text
	if entry.Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestParser_Run_SkipsEmptyTitle(t *testing.T) {
	raw := &models.RawEntry{
		ID: 1, SourceID: 1, GUID: "g-empty",
		Title:     "   ",
		Published: time.Now(),
	}
	repo := &mockRepo{raws: []*models.RawEntry{raw}}
	p := parser.New(repo)

	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Processed != 0 {
		t.Errorf("expected 0 processed (empty title), got %d", stats.Processed)
	}
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", stats.Skipped)
	}
}

func TestParser_Run_PreservesMetadata(t *testing.T) {
	pub := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	raw := &models.RawEntry{
		ID: 1, SourceID: 42, SourceName: "arXiv AI",
		GUID:      "arxiv-2406.001",
		Title:     "Scaling Laws for Transformers",
		Link:      "https://arxiv.org/abs/2406.001",
		Authors:   `["Alice Smith", "Bob Jones"]`,
		Published: pub,
	}
	repo := &mockRepo{raws: []*models.RawEntry{raw}}
	p := parser.New(repo)

	_, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.entries) == 0 {
		t.Fatal("no entries saved")
	}
	e := repo.entries[0]

	if e.SourceID != 42 {
		t.Errorf("SourceID: got %d, want 42", e.SourceID)
	}
	if e.SourceName != "arXiv AI" {
		t.Errorf("SourceName: got %q", e.SourceName)
	}
	if e.Link != raw.Link {
		t.Errorf("Link: got %q, want %q", e.Link, raw.Link)
	}
	if !e.Published.Equal(pub) {
		t.Errorf("Published: got %v, want %v", e.Published, pub)
	}
	if len(e.Authors) == 0 {
		t.Error("expected authors to be parsed")
	}
}

func TestParser_TagExtraction(t *testing.T) {
	raw := &models.RawEntry{
		ID: 1, SourceID: 1, GUID: "tag-test",
		Title:     "Fine-tuning LLMs with LoRA on Kubernetes clusters",
		Published: time.Now(),
	}
	repo := &mockRepo{raws: []*models.RawEntry{raw}}
	p := parser.New(repo)

	_, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.entries) == 0 {
		t.Fatal("no entries")
	}
	tags := repo.entries[0].Tags

	mustHaveTag := func(want string) {
		t.Helper()
		for _, tag := range tags {
			if tag == want {
				return
			}
		}
		t.Errorf("expected tag %q in %v", want, tags)
	}
	mustHaveTag("fine-tuning")
	mustHaveTag("llm")
	mustHaveTag("kubernetes")
}

func TestParser_Run_EmptyRepo(t *testing.T) {
	repo := &mockRepo{}
	p := parser.New(repo)
	stats, err := p.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Processed != 0 || stats.Skipped != 0 {
		t.Errorf("empty repo: processed=%d skipped=%d", stats.Processed, stats.Skipped)
	}
}
