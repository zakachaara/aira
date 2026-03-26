package storage_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

// ─────────────────────────────────────────────
//  Test helpers
// ─────────────────────────────────────────────

func newTestDB(t *testing.T) *storage.SQLiteRepo {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "aira-test-*.db")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.Close()

	repo, err := storage.NewSQLite(f.Name())
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func ctx() context.Context { return context.Background() }

// ─────────────────────────────────────────────
//  Sources
// ─────────────────────────────────────────────

func TestSaveAndGetSource(t *testing.T) {
	repo := newTestDB(t)

	src := &models.Source{
		Name:     "arXiv AI",
		URL:      "https://arxiv.org/rss/cs.AI",
		Category: models.CategoryAIResearch,
		Active:   true,
	}
	if err := repo.SaveSource(ctx(), src); err != nil {
		t.Fatalf("SaveSource: %v", err)
	}
	if src.ID == 0 {
		t.Fatal("expected non-zero ID after save")
	}

	got, err := repo.GetSource(ctx(), src.ID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got.Name != src.Name {
		t.Errorf("name: got %q, want %q", got.Name, src.Name)
	}
	if got.Category != src.Category {
		t.Errorf("category: got %q, want %q", got.Category, src.Category)
	}
	if !got.Active {
		t.Error("expected source to be active")
	}
}

func TestListSources_ActiveOnly(t *testing.T) {
	repo := newTestDB(t)

	active := &models.Source{Name: "Active", URL: "https://a.com/rss", Active: true}
	inactive := &models.Source{Name: "Inactive", URL: "https://b.com/rss", Active: true}
	_ = repo.SaveSource(ctx(), active)
	_ = repo.SaveSource(ctx(), inactive)
	_ = repo.DeleteSource(ctx(), inactive.ID)

	all, err := repo.ListSources(ctx(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("want 2 total sources, got %d", len(all))
	}

	onlyActive, err := repo.ListSources(ctx(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyActive) != 1 {
		t.Errorf("want 1 active source, got %d", len(onlyActive))
	}
	if onlyActive[0].Name != "Active" {
		t.Errorf("want Active, got %s", onlyActive[0].Name)
	}
}

func TestUpsertSource_DuplicateURL(t *testing.T) {
	repo := newTestDB(t)
	src := &models.Source{Name: "Original", URL: "https://dup.com/rss", Active: true}
	_ = repo.SaveSource(ctx(), src)

	updated := &models.Source{Name: "Updated", URL: "https://dup.com/rss", Active: true}
	if err := repo.SaveSource(ctx(), updated); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	sources, _ := repo.ListSources(ctx(), false)
	if len(sources) != 1 {
		t.Fatalf("want 1 source after upsert, got %d", len(sources))
	}
	if sources[0].Name != "Updated" {
		t.Errorf("want Updated, got %s", sources[0].Name)
	}
}

// ─────────────────────────────────────────────
//  Raw Entries
// ─────────────────────────────────────────────

func TestSaveRawEntry_GUIDDedup(t *testing.T) {
	repo := newTestDB(t)

	src := &models.Source{Name: "S", URL: "https://s.com/rss", Active: true}
	_ = repo.SaveSource(ctx(), src)

	raw := &models.RawEntry{
		SourceID:   src.ID,
		SourceName: src.Name,
		GUID:       "guid-001",
		Title:      "First Entry",
		Published:  time.Now(),
	}
	id1, err := repo.SaveRawEntry(ctx(), raw)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Saving same GUID should be silently ignored (INSERT OR IGNORE)
	id2, err := repo.SaveRawEntry(ctx(), raw)
	if err != nil {
		t.Fatalf("duplicate save: %v", err)
	}
	if id2 != 0 && id2 != id1 {
		t.Errorf("expected id2==0 (ignored) or id2==id1, got id1=%d id2=%d", id1, id2)
	}
}

func TestGUIDExists(t *testing.T) {
	repo := newTestDB(t)
	src := &models.Source{Name: "S", URL: "https://s.com/rss", Active: true}
	_ = repo.SaveSource(ctx(), src)

	exists, err := repo.GUIDExists(ctx(), "not-yet")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected guid to not exist yet")
	}

	_ , _ = repo.SaveRawEntry(ctx(), &models.RawEntry{
		SourceID: src.ID, GUID: "not-yet", Title: "T", Published: time.Now(),
	})

	exists, err = repo.GUIDExists(ctx(), "not-yet")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected guid to exist after insert")
	}
}

// ─────────────────────────────────────────────
//  Entries
// ─────────────────────────────────────────────

func seedEntry(t *testing.T, repo *storage.SQLiteRepo, title string, cat models.Category) *models.Entry {
	t.Helper()
	src := &models.Source{Name: "S-" + title, URL: "https://x.com/" + title, Active: true}
	_ = repo.SaveSource(ctx(), src)

	raw := &models.RawEntry{
		SourceID: src.ID, GUID: "guid-" + title, Title: title, Published: time.Now(),
	}
	rawID, _ := repo.SaveRawEntry(ctx(), raw)

	e := &models.Entry{
		RawID: rawID, SourceID: src.ID, SourceName: src.Name,
		GUID: "guid-" + title, Title: title, Link: "https://x.com/" + title,
		Summary: "Summary of " + title, Published: time.Now(),
		Category: cat, Confidence: 0.8, ClassifiedAt: time.Now(),
	}
	_, err := repo.SaveEntry(ctx(), e)
	if err != nil {
		t.Fatalf("seedEntry SaveEntry: %v", err)
	}
	return e
}

func TestListEntries_FilterByCategory(t *testing.T) {
	repo := newTestDB(t)
	seedEntry(t, repo, "research-paper", models.CategoryAIResearch)
	seedEntry(t, repo, "model-launch", models.CategoryModelRelease)
	seedEntry(t, repo, "k8s-update", models.CategoryCloudNative)

	entries, err := repo.ListEntries(ctx(), storage.EntryQuery{
		Category: models.CategoryAIResearch,
		Limit:    10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("want 1 ai_research entry, got %d", len(entries))
	}
	if entries[0].Category != models.CategoryAIResearch {
		t.Errorf("category: got %q", entries[0].Category)
	}
}

func TestCountEntries(t *testing.T) {
	repo := newTestDB(t)
	seedEntry(t, repo, "e1", models.CategoryAIResearch)
	seedEntry(t, repo, "e2", models.CategoryModelRelease)

	n, err := repo.CountEntries(ctx(), time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("want 2, got %d", n)
	}
}

func TestListUnclassified(t *testing.T) {
	repo := newTestDB(t)
	src := &models.Source{Name: "S", URL: "https://s.com/rss", Active: true}
	_ = repo.SaveSource(ctx(), src)
	raw := &models.RawEntry{SourceID: src.ID, GUID: "uc-1", Title: "Unclassified", Published: time.Now()}
	rawID, _ := repo.SaveRawEntry(ctx(), raw)

	// Entry without classified_at
	e := &models.Entry{
		RawID: rawID, SourceID: src.ID, SourceName: "S", GUID: "uc-1",
		Title: "Unclassified", Published: time.Now(),
		Category: models.CategoryUncategorized,
	}
	_, _ = repo.SaveEntry(ctx(), e)

	unclassified, err := repo.ListUnclassified(ctx(), 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range unclassified {
		if u.GUID == "uc-1" {
			found = true
		}
	}
	if !found {
		t.Error("expected unclassified entry in results")
	}
}

// ─────────────────────────────────────────────
//  Signals
// ─────────────────────────────────────────────

func TestSaveAndListSignals(t *testing.T) {
	repo := newTestDB(t)
	e := seedEntry(t, repo, "signal-entry", models.CategoryModelRelease)

	sig := &models.Signal{
		EntryID:     e.ID,
		Type:        models.SignalModelRelease,
		Description: "GPT-X released",
		Score:       0.92,
		Keywords:    []string{"release", "model"},
	}
	if err := repo.SaveSignal(ctx(), sig); err != nil {
		t.Fatalf("SaveSignal: %v", err)
	}

	signals, err := repo.ListSignals(ctx(), time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) == 0 {
		t.Fatal("expected at least one signal")
	}
	if signals[0].Type != models.SignalModelRelease {
		t.Errorf("type: got %q", signals[0].Type)
	}
	if len(signals[0].Keywords) == 0 {
		t.Error("expected keywords to be deserialised")
	}
}

// ─────────────────────────────────────────────
//  Trends
// ─────────────────────────────────────────────

func TestSaveAndListTrends(t *testing.T) {
	repo := newTestDB(t)

	trend := &models.Trend{
		Topic:    "LLM Agents",
		Category: models.CategoryAIResearch,
		Velocity: 0.73,
		Frequency: 42,
		Window:   "7d",
		TimeSeries: []models.TrendPoint{
			{Date: time.Now(), Count: 10, Topic: "LLM Agents"},
		},
	}
	if err := repo.SaveTrend(ctx(), trend); err != nil {
		t.Fatalf("SaveTrend: %v", err)
	}

	trends, err := repo.ListTrends(ctx(), "7d", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(trends) == 0 {
		t.Fatal("expected at least one trend")
	}
	if trends[0].Topic != "LLM Agents" {
		t.Errorf("topic: got %q", trends[0].Topic)
	}
	if len(trends[0].TimeSeries) == 0 {
		t.Error("expected time series to be deserialised")
	}
}

// ─────────────────────────────────────────────
//  Digests
// ─────────────────────────────────────────────

func TestSaveAndGetLatestDigest(t *testing.T) {
	repo := newTestDB(t)

	d := &models.Digest{
		DateRange:    "2024-01-01 – 2024-01-02",
		TotalEntries: 99,
		Sections: []models.DigestSection{
			{Title: "AI Research", Entries: []string{"- Paper A", "- Paper B"}},
		},
		Markdown: "# AIRA Digest\n\nTest content.",
	}
	if err := repo.SaveDigest(ctx(), d); err != nil {
		t.Fatalf("SaveDigest: %v", err)
	}
	if d.ID == 0 {
		t.Fatal("expected non-zero digest ID")
	}

	latest, err := repo.GetLatestDigest(ctx())
	if err != nil {
		t.Fatalf("GetLatestDigest: %v", err)
	}
	if latest.TotalEntries != 99 {
		t.Errorf("total_entries: got %d, want 99", latest.TotalEntries)
	}
	if len(latest.Sections) != 1 {
		t.Errorf("sections: got %d, want 1", len(latest.Sections))
	}
	if latest.Markdown == "" {
		t.Error("expected markdown to be stored and retrieved")
	}
}
