// Package storage provides the persistence layer for AIRA.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/aira/aira/internal/models"
)

// ─────────────────────────────────────────────
//  Repository interfaces
// ─────────────────────────────────────────────

// Repository is the aggregate data-access interface for AIRA.
type Repository interface {
	SourceRepository
	RawEntryRepository
	EntryRepository
	SignalRepository
	TrendRepository
	DigestRepository
	Close() error
	Migrate(ctx context.Context) error
}

type SourceRepository interface {
	SaveSource(ctx context.Context, s *models.Source) error
	GetSource(ctx context.Context, id int64) (*models.Source, error)
	ListSources(ctx context.Context, activeOnly bool) ([]*models.Source, error)
	DeleteSource(ctx context.Context, id int64) error
	UpdateSourceFetchTime(ctx context.Context, id int64, t time.Time) error
}

type RawEntryRepository interface {
	SaveRawEntry(ctx context.Context, e *models.RawEntry) (int64, error)
	GUIDExists(ctx context.Context, guid string) (bool, error)
	ListUnparsedRaw(ctx context.Context, limit int) ([]*models.RawEntry, error)
}

type EntryRepository interface {
	SaveEntry(ctx context.Context, e *models.Entry) (int64, error)
	GetEntry(ctx context.Context, id int64) (*models.Entry, error)
	ListEntries(ctx context.Context, q EntryQuery) ([]*models.Entry, error)
	CountEntries(ctx context.Context, since time.Time) (int, error)
	ListUnclassified(ctx context.Context, limit int) ([]*models.Entry, error)
}

type SignalRepository interface {
	SaveSignal(ctx context.Context, s *models.Signal) error
	ListSignals(ctx context.Context, since time.Time, limit int) ([]*models.Signal, error)
}

type TrendRepository interface {
	SaveTrend(ctx context.Context, t *models.Trend) error
	ListTrends(ctx context.Context, window string, limit int) ([]*models.Trend, error)
}

type DigestRepository interface {
	SaveDigest(ctx context.Context, d *models.Digest) error
	GetLatestDigest(ctx context.Context) (*models.Digest, error)
	ListDigests(ctx context.Context, limit int) ([]*models.Digest, error)
}

// EntryQuery encapsulates filters for entry listing.
type EntryQuery struct {
	Category models.Category
	Since    time.Time
	Until    time.Time
	Limit    int
	Offset   int
}

// ─────────────────────────────────────────────
//  SQLite implementation
// ─────────────────────────────────────────────

// SQLiteRepo implements Repository over SQLite.
type SQLiteRepo struct {
	db *sql.DB
}

// NewSQLite opens (or creates) the SQLite database at dsn.
func NewSQLite(dsn string) (*SQLiteRepo, error) {
	if strings.HasPrefix(dsn, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dsn = filepath.Join(home, dsn[2:])
	}
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}
	db, err := sql.Open("sqlite3", dsn+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &SQLiteRepo{db: db}, nil
}

func (r *SQLiteRepo) Close() error { return r.db.Close() }

func (r *SQLiteRepo) Migrate(_ context.Context) error {
	_, err := r.db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS sources (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	url          TEXT NOT NULL UNIQUE,
	category     TEXT NOT NULL DEFAULT 'uncategorized',
	active       INTEGER NOT NULL DEFAULT 1,
	last_fetched DATETIME,
	created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS raw_entries (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id    INTEGER NOT NULL REFERENCES sources(id),
	source_name  TEXT NOT NULL,
	guid         TEXT NOT NULL UNIQUE,
	title        TEXT NOT NULL,
	link         TEXT,
	description  TEXT,
	content      TEXT,
	authors      TEXT,
	published    DATETIME,
	fetched_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	raw_payload  TEXT
);

CREATE INDEX IF NOT EXISTS idx_raw_entries_source    ON raw_entries(source_id);
CREATE INDEX IF NOT EXISTS idx_raw_entries_published ON raw_entries(published);

CREATE TABLE IF NOT EXISTS entries (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	raw_id        INTEGER NOT NULL REFERENCES raw_entries(id),
	source_id     INTEGER NOT NULL REFERENCES sources(id),
	source_name   TEXT NOT NULL,
	guid          TEXT NOT NULL UNIQUE,
	title         TEXT NOT NULL,
	link          TEXT,
	summary       TEXT,
	authors       TEXT,
	tags          TEXT,
	published     DATETIME,
	parsed_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	category      TEXT NOT NULL DEFAULT 'uncategorized',
	confidence    REAL NOT NULL DEFAULT 0,
	classified_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_entries_category   ON entries(category);
CREATE INDEX IF NOT EXISTS idx_entries_published  ON entries(published);
CREATE INDEX IF NOT EXISTS idx_entries_classified ON entries(classified_at);

CREATE TABLE IF NOT EXISTS signals (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	entry_id    INTEGER NOT NULL REFERENCES entries(id),
	type        TEXT NOT NULL,
	description TEXT NOT NULL,
	score       REAL NOT NULL DEFAULT 0,
	keywords    TEXT,
	detected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_signals_type        ON signals(type);
CREATE INDEX IF NOT EXISTS idx_signals_detected_at ON signals(detected_at);

CREATE TABLE IF NOT EXISTS trends (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	topic       TEXT NOT NULL,
	category    TEXT NOT NULL,
	velocity    REAL NOT NULL DEFAULT 0,
	frequency   INTEGER NOT NULL DEFAULT 0,
	window      TEXT NOT NULL,
	time_series TEXT,
	detected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS digests (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	generated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	date_range    TEXT NOT NULL,
	total_entries INTEGER NOT NULL DEFAULT 0,
	sections      TEXT,
	signals       TEXT,
	trends        TEXT,
	markdown      TEXT
);
`

// ── Sources ──────────────────────────────────

func (r *SQLiteRepo) SaveSource(ctx context.Context, s *models.Source) error {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO sources (name, url, category, active, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET name=excluded.name, category=excluded.category, active=excluded.active`,
		s.Name, s.URL, string(s.Category), boolInt(s.Active), time.Now().UTC())
	if err != nil {
		return err
	}
	if s.ID == 0 {
		s.ID, _ = res.LastInsertId()
	}
	return nil
}

func (r *SQLiteRepo) GetSource(ctx context.Context, id int64) (*models.Source, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,url,category,active,last_fetched,created_at FROM sources WHERE id=?`, id)
	return scanSource(row)
}

func (r *SQLiteRepo) ListSources(ctx context.Context, activeOnly bool) ([]*models.Source, error) {
	q := `SELECT id,name,url,category,active,last_fetched,created_at FROM sources`
	if activeOnly {
		q += ` WHERE active=1`
	}
	q += ` ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SQLiteRepo) DeleteSource(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sources SET active=0 WHERE id=?`, id)
	return err
}

func (r *SQLiteRepo) UpdateSourceFetchTime(ctx context.Context, id int64, t time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sources SET last_fetched=? WHERE id=?`, t.UTC(), id)
	return err
}

// ── Raw Entries ───────────────────────────────

func (r *SQLiteRepo) SaveRawEntry(ctx context.Context, e *models.RawEntry) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO raw_entries
			(source_id, source_name, guid, title, link, description, content, authors, published, fetched_at, raw_payload)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		e.SourceID, e.SourceName, e.GUID, e.Title, e.Link,
		e.Description, e.Content, e.Authors,
		nullTime(e.Published), time.Now().UTC(), e.RawPayload,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *SQLiteRepo) GUIDExists(ctx context.Context, guid string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM raw_entries WHERE guid=?`, guid).Scan(&count)
	return count > 0, err
}

func (r *SQLiteRepo) ListUnparsedRaw(ctx context.Context, limit int) ([]*models.RawEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.source_id, r.source_name, r.guid, r.title, r.link,
		       r.description, r.content, r.authors, r.published, r.fetched_at, r.raw_payload
		FROM raw_entries r
		LEFT JOIN entries e ON e.raw_id = r.id
		WHERE e.id IS NULL
		ORDER BY r.published DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.RawEntry
	for rows.Next() {
		var e models.RawEntry
		var pub sql.NullTime
		if err := rows.Scan(&e.ID, &e.SourceID, &e.SourceName, &e.GUID, &e.Title, &e.Link,
			&e.Description, &e.Content, &e.Authors, &pub, &e.FetchedAt, &e.RawPayload); err != nil {
			return nil, err
		}
		if pub.Valid {
			e.Published = pub.Time
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// ── Entries ───────────────────────────────────

func (r *SQLiteRepo) SaveEntry(ctx context.Context, e *models.Entry) (int64, error) {
	authors, _ := json.Marshal(e.Authors)
	tags, _ := json.Marshal(e.Tags)
	res, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO entries
			(raw_id, source_id, source_name, guid, title, link, summary, authors, tags,
			 published, parsed_at, category, confidence, classified_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.RawID, e.SourceID, e.SourceName, e.GUID, e.Title, e.Link,
		e.Summary, string(authors), string(tags),
		nullTime(e.Published), time.Now().UTC(),
		string(e.Category), e.Confidence, nullTime(e.ClassifiedAt),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return id, nil
}

func (r *SQLiteRepo) GetEntry(ctx context.Context, id int64) (*models.Entry, error) {
	row := r.db.QueryRowContext(ctx, entrySelectSQL+` WHERE e.id=?`, id)
	return scanEntry(row)
}

func (r *SQLiteRepo) ListEntries(ctx context.Context, q EntryQuery) ([]*models.Entry, error) {
	where, args := []string{"1=1"}, []interface{}{}
	if q.Category != "" {
		where = append(where, "e.category=?")
		args = append(args, string(q.Category))
	}
	if !q.Since.IsZero() {
		where = append(where, "e.published >= ?")
		args = append(args, q.Since.UTC())
	}
	if !q.Until.IsZero() {
		where = append(where, "e.published <= ?")
		args = append(args, q.Until.UTC())
	}
	limit := q.Limit
	if limit == 0 {
		limit = 100
	}
	stmt := entrySelectSQL + " WHERE " + strings.Join(where, " AND ") +
		" ORDER BY e.published DESC LIMIT ? OFFSET ?"
	args = append(args, limit, q.Offset)
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *SQLiteRepo) CountEntries(ctx context.Context, since time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM entries WHERE published >= ?`, since.UTC()).Scan(&count)
	return count, err
}

func (r *SQLiteRepo) ListUnclassified(ctx context.Context, limit int) ([]*models.Entry, error) {
	rows, err := r.db.QueryContext(ctx,
		entrySelectSQL+` WHERE e.classified_at IS NULL ORDER BY e.parsed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ── Signals ───────────────────────────────────

func (r *SQLiteRepo) SaveSignal(ctx context.Context, s *models.Signal) error {
	kw, _ := json.Marshal(s.Keywords)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO signals (entry_id, type, description, score, keywords, detected_at)
		VALUES (?,?,?,?,?,?)`,
		s.EntryID, string(s.Type), s.Description, s.Score, string(kw), time.Now().UTC())
	return err
}

func (r *SQLiteRepo) ListSignals(ctx context.Context, since time.Time, limit int) ([]*models.Signal, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entry_id, type, description, score, keywords, detected_at
		FROM signals WHERE detected_at >= ? ORDER BY score DESC, detected_at DESC LIMIT ?`,
		since.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Signal
	for rows.Next() {
		var s models.Signal
		var kw string
		if err := rows.Scan(&s.ID, &s.EntryID, &s.Type, &s.Description, &s.Score, &kw, &s.DetectedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(kw), &s.Keywords)
		out = append(out, &s)
	}
	return out, rows.Err()
}

// ── Trends ────────────────────────────────────

func (r *SQLiteRepo) SaveTrend(ctx context.Context, t *models.Trend) error {
	ts, _ := json.Marshal(t.TimeSeries)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO trends (topic, category, velocity, frequency, window, time_series, detected_at)
		VALUES (?,?,?,?,?,?,?)`,
		t.Topic, string(t.Category), t.Velocity, t.Frequency, t.Window, string(ts), time.Now().UTC())
	return err
}

func (r *SQLiteRepo) ListTrends(ctx context.Context, window string, limit int) ([]*models.Trend, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, topic, category, velocity, frequency, window, time_series, detected_at
		FROM trends WHERE window=? ORDER BY velocity DESC LIMIT ?`, window, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Trend
	for rows.Next() {
		var t models.Trend
		var ts string
		if err := rows.Scan(&t.ID, &t.Topic, &t.Category, &t.Velocity, &t.Frequency,
			&t.Window, &ts, &t.DetectedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ts), &t.TimeSeries)
		out = append(out, &t)
	}
	return out, rows.Err()
}

// ── Digests ───────────────────────────────────

func (r *SQLiteRepo) SaveDigest(ctx context.Context, d *models.Digest) error {
	sec, _ := json.Marshal(d.Sections)
	sig, _ := json.Marshal(d.Signals)
	trn, _ := json.Marshal(d.Trends)
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO digests (generated_at, date_range, total_entries, sections, signals, trends, markdown)
		VALUES (?,?,?,?,?,?,?)`,
		time.Now().UTC(), d.DateRange, d.TotalEntries, string(sec), string(sig), string(trn), d.Markdown)
	if err != nil {
		return err
	}
	d.ID, _ = res.LastInsertId()
	return nil
}

func (r *SQLiteRepo) GetLatestDigest(ctx context.Context) (*models.Digest, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, generated_at, date_range, total_entries, sections, signals, trends, markdown
		FROM digests ORDER BY generated_at DESC LIMIT 1`)
	return scanDigest(row)
}

func (r *SQLiteRepo) ListDigests(ctx context.Context, limit int) ([]*models.Digest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, generated_at, date_range, total_entries, sections, signals, trends, markdown
		FROM digests ORDER BY generated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Digest
	for rows.Next() {
		d, err := scanDigest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ─────────────────────────────────────────────
//  Scanner helpers
// ─────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanSource(row rowScanner) (*models.Source, error) {
	var s models.Source
	var lastFetched sql.NullTime
	var active int
	if err := row.Scan(&s.ID, &s.Name, &s.URL, &s.Category, &active, &lastFetched, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Active = active == 1
	if lastFetched.Valid {
		s.LastFetched = lastFetched.Time
	}
	return &s, nil
}

const entrySelectSQL = `
SELECT e.id, e.raw_id, e.source_id, e.source_name, e.guid, e.title, e.link,
       e.summary, e.authors, e.tags, e.published, e.parsed_at, e.category, e.confidence, e.classified_at
FROM entries e`

func scanEntry(row rowScanner) (*models.Entry, error) {
	var e models.Entry
	var authors, tags string
	var pub, classifiedAt sql.NullTime
	if err := row.Scan(&e.ID, &e.RawID, &e.SourceID, &e.SourceName, &e.GUID, &e.Title, &e.Link,
		&e.Summary, &authors, &tags, &pub, &e.ParsedAt, &e.Category, &e.Confidence, &classifiedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(authors), &e.Authors)
	_ = json.Unmarshal([]byte(tags), &e.Tags)
	if pub.Valid {
		e.Published = pub.Time
	}
	if classifiedAt.Valid {
		e.ClassifiedAt = classifiedAt.Time
	}
	return &e, nil
}

func scanDigest(row rowScanner) (*models.Digest, error) {
	var d models.Digest
	var sec, sig, trn string
	if err := row.Scan(&d.ID, &d.GeneratedAt, &d.DateRange, &d.TotalEntries,
		&sec, &sig, &trn, &d.Markdown); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(sec), &d.Sections)
	_ = json.Unmarshal([]byte(sig), &d.Signals)
	_ = json.Unmarshal([]byte(trn), &d.Trends)
	return &d, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}
