// Package models defines the core domain types for AIRA.
package models

import "time"

// Category represents a classification bucket for feed entries.
type Category string

const (
	CategoryAIResearch      Category = "ai_research"
	CategoryModelRelease    Category = "model_release"
	CategoryAIInfrastructure Category = "ai_infrastructure"
	CategoryCloudNative     Category = "cloud_native"
	CategoryUncategorized   Category = "uncategorized"
)

// SignalType classifies the nature of a detected signal.
type SignalType string

const (
	SignalModelRelease          SignalType = "model_release"
	SignalResearchBreakthrough  SignalType = "research_breakthrough"
	SignalInfrastructureRelease SignalType = "infrastructure_release"
	SignalDatasetRelease        SignalType = "dataset_release"
	SignalBenchmarkResult       SignalType = "benchmark_result"
	SignalEmergingTopic         SignalType = "emerging_topic"
)

// Source represents a configured RSS feed source.
type Source struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Category    Category  `json:"category"`
	Active      bool      `json:"active"`
	LastFetched time.Time `json:"last_fetched"`
	CreatedAt   time.Time `json:"created_at"`
}

// RawEntry is the unprocessed record as fetched from an RSS feed.
type RawEntry struct {
	ID          int64     `json:"id"`
	SourceID    int64     `json:"source_id"`
	SourceName  string    `json:"source_name"`
	GUID        string    `json:"guid"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Authors     string    `json:"authors"`
	Published   time.Time `json:"published"`
	FetchedAt   time.Time `json:"fetched_at"`
	RawPayload  string    `json:"raw_payload"` // JSON-encoded original item
}

// Entry is the normalized, structured feed item after parsing.
type Entry struct {
	ID          int64     `json:"id"`
	RawID       int64     `json:"raw_id"`
	SourceID    int64     `json:"source_id"`
	SourceName  string    `json:"source_name"`
	GUID        string    `json:"guid"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Summary     string    `json:"summary"`
	Authors     []string  `json:"authors"`
	Tags        []string  `json:"tags"`
	Published   time.Time `json:"published"`
	ParsedAt    time.Time `json:"parsed_at"`

	// Classification results
	Category    Category  `json:"category"`
	Confidence  float64   `json:"confidence"`
	ClassifiedAt time.Time `json:"classified_at"`

	// Signal detection
	Signals     []Signal  `json:"signals,omitempty"`
}

// Signal represents a meaningful event extracted from an entry.
type Signal struct {
	ID          int64      `json:"id"`
	EntryID     int64      `json:"entry_id"`
	Type        SignalType `json:"type"`
	Description string     `json:"description"`
	Score       float64    `json:"score"` // relevance/confidence score 0–1
	Keywords    []string   `json:"keywords"`
	DetectedAt  time.Time  `json:"detected_at"`
}

// TrendPoint is a data point in a trend analysis time series.
type TrendPoint struct {
	Date    time.Time `json:"date"`
	Count   int       `json:"count"`
	Topic   string    `json:"topic"`
}

// Trend captures an emerging topic across a time window.
type Trend struct {
	ID          int64        `json:"id"`
	Topic       string       `json:"topic"`
	Category    Category     `json:"category"`
	Velocity    float64      `json:"velocity"`    // growth rate (entries/day delta)
	Frequency   int          `json:"frequency"`   // total mentions in window
	Window      string       `json:"window"`      // e.g. "7d", "24h"
	TimeSeries  []TrendPoint `json:"time_series"`
	DetectedAt  time.Time    `json:"detected_at"`
}

// DigestSection is a named section within a digest report.
type DigestSection struct {
	Title   string   `json:"title"`
	Entries []string `json:"entries"` // formatted bullet lines
}

// Digest is the structured daily intelligence report.
type Digest struct {
	ID          int64           `json:"id"`
	GeneratedAt time.Time       `json:"generated_at"`
	DateRange   string          `json:"date_range"`
	TotalEntries int            `json:"total_entries"`
	Sections    []DigestSection `json:"sections"`
	Signals     []Signal        `json:"signals"`
	Trends      []Trend         `json:"trends"`
	Markdown    string          `json:"markdown"` // fully rendered report
	HTML        string          `json:"html"`     // fully rendered HTML report
}

// CollectStats summarises a collect run.
type CollectStats struct {
	SourcesFetched int
	NewEntries     int
	Errors         []string
	Duration       time.Duration
}

// ParseStats summarises a parse run.
type ParseStats struct {
	Processed int
	Skipped   int
	Errors    []string
	Duration  time.Duration
}
