// Package analyzer extracts signals and detects trends from classified entries.
package analyzer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/aira/aira/internal/logger"
	"github.com/aira/aira/internal/models"
	"github.com/aira/aira/internal/storage"
)

// Analyzer runs signal extraction and trend detection.
type Analyzer struct {
	repo          storage.Repository
	signalRules   []signalRule
	trendWindows  []string
}

// New creates an Analyzer with default detection rules.
func New(repo storage.Repository) *Analyzer {
	return &Analyzer{
		repo:         repo,
		signalRules:  defaultSignalRules(),
		trendWindows: []string{"24h", "7d"},
	}
}

// ─────────────────────────────────────────────
//  Signal Extraction
// ─────────────────────────────────────────────

// signalRule defines patterns for a specific signal type.
type signalRule struct {
	signalType  models.SignalType
	keywords    []string
	minScore    float64
	description string
}

// ExtractSignals scans recent entries and saves detected signals.
func (a *Analyzer) ExtractSignals(ctx context.Context, since time.Time) (int, error) {
	entries, err := a.repo.ListEntries(ctx, storage.EntryQuery{
		Since: since,
		Limit: 2000,
	})
	if err != nil {
		return 0, fmt.Errorf("listing entries for signal extraction: %w", err)
	}

	logger.Info("extracting signals", zap.Int("entries", len(entries)))
	count := 0

	for _, e := range entries {
		corpus := strings.ToLower(e.Title + " " + e.Summary)
		for _, rule := range a.signalRules {
			score, matched := scoreRule(corpus, rule.keywords)
			if score < rule.minScore {
				continue
			}
			sig := &models.Signal{
				EntryID:     e.ID,
				Type:        rule.signalType,
				Description: fmt.Sprintf("[%s] %s", rule.signalType, e.Title),
				Score:       clampF(score, 0, 1),
				Keywords:    matched,
				DetectedAt:  time.Now().UTC(),
			}
			if err := a.repo.SaveSignal(ctx, sig); err != nil {
				logger.Error("saving signal", zap.Error(err))
				continue
			}
			count++
		}
	}

	logger.Info("signal extraction complete", zap.Int("signals_detected", count))
	return count, nil
}

// ─────────────────────────────────────────────
//  Trend Detection
// ─────────────────────────────────────────────

// DetectTrends analyses entry frequency by keyword topic across time windows.
func (a *Analyzer) DetectTrends(ctx context.Context) ([]*models.Trend, error) {
	var allTrends []*models.Trend

	for _, window := range a.trendWindows {
		trends, err := a.detectWindow(ctx, window)
		if err != nil {
			logger.Error("trend detection failed for window",
				zap.String("window", window), zap.Error(err))
			continue
		}
		for _, t := range trends {
			if err := a.repo.SaveTrend(ctx, t); err != nil {
				logger.Error("saving trend", zap.Error(err))
			}
		}
		allTrends = append(allTrends, trends...)
	}
	return allTrends, nil
}

func (a *Analyzer) detectWindow(ctx context.Context, window string) ([]*models.Trend, error) {
	since, buckets, bucketDuration := windowParams(window)

	entries, err := a.repo.ListEntries(ctx, storage.EntryQuery{
		Since: since,
		Limit: 5000,
	})
	if err != nil {
		return nil, err
	}

	// Count keyword occurrences per time bucket
	type bucketKey struct {
		topic  string
		bucket int
	}
	counts := make(map[bucketKey]int)
	topicTotal := make(map[string]int)
	topicCategory := make(map[string]models.Category)

	for _, e := range entries {
		corpus := strings.ToLower(e.Title + " " + e.Summary)
		bucketIdx := int(time.Since(e.Published) / bucketDuration)
		if bucketIdx >= buckets {
			bucketIdx = buckets - 1
		}

		for _, topic := range trendTopics {
			for _, kw := range topic.keywords {
				if strings.Contains(corpus, kw) {
					counts[bucketKey{topic: topic.name, bucket: bucketIdx}]++
					topicTotal[topic.name]++
					topicCategory[topic.name] = e.Category
					break // count topic once per entry
				}
			}
		}
	}

	// Build trend objects for topics with sufficient frequency
	var trends []*models.Trend
	for _, topic := range trendTopics {
		total := topicTotal[topic.name]
		if total < 2 {
			continue
		}

		// Build time series (bucket 0 = most recent)
		ts := make([]models.TrendPoint, 0, buckets)
		for i := 0; i < buckets; i++ {
			t := time.Now().Add(-time.Duration(i) * bucketDuration)
			ts = append(ts, models.TrendPoint{
				Date:  t,
				Count: counts[bucketKey{topic: topic.name, bucket: i}],
				Topic: topic.name,
			})
		}

		velocity := computeVelocity(ts)
		cat := topicCategory[topic.name]
		if cat == "" {
			cat = models.CategoryUncategorized
		}

		trends = append(trends, &models.Trend{
			Topic:      topic.name,
			Category:   cat,
			Velocity:   velocity,
			Frequency:  total,
			Window:     window,
			TimeSeries: ts,
			DetectedAt: time.Now().UTC(),
		})
	}

	// Sort by velocity descending
	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Velocity > trends[j].Velocity
	})

	// Return top 20
	if len(trends) > 20 {
		trends = trends[:20]
	}
	return trends, nil
}

// computeVelocity returns the rate-of-change between the first and second half
// of a time series, normalised to entries-per-bucket.
func computeVelocity(ts []models.TrendPoint) float64 {
	if len(ts) < 2 {
		return 0
	}
	// ts[0] is most recent; last element is oldest
	half := len(ts) / 2
	var recent, older float64
	for i, p := range ts {
		if i < half {
			recent += float64(p.Count)
		} else {
			older += float64(p.Count)
		}
	}
	if older == 0 {
		if recent > 0 {
			return 1
		}
		return 0
	}
	return math.Log1p(recent/older) // log ratio avoids extreme values
}

// ─────────────────────────────────────────────
//  Topic & rule definitions
// ─────────────────────────────────────────────

type topicDef struct {
	name     string
	keywords []string
}

var trendTopics = []topicDef{
	{"LLM Agents", []string{"agent", "agentic", "tool use", "function calling"}},
	{"Reasoning Models", []string{"reasoning", "chain-of-thought", "o1", "o3", "r1"}},
	{"Multimodal AI", []string{"multimodal", "vision-language", "image-text", "audio model"}},
	{"Model Quantization", []string{"quantization", "quantized", "gguf", "ggml", "int4", "int8"}},
	{"RAG & Retrieval", []string{"rag", "retrieval-augmented", "vector database", "embedding search"}},
	{"Open Source Models", []string{"open weights", "open-weight", "open source model", "llama", "mistral", "falcon"}},
	{"AI Safety & Alignment", []string{"alignment", "ai safety", "rlhf", "constitutional ai", "red teaming"}},
	{"Inference Efficiency", []string{"inference speed", "latency", "throughput", "speculative decoding", "flash attention"}},
	{"Kubernetes & Cloud Native", []string{"kubernetes", "k8s", "helm", "operator"}},
	{"eBPF & Observability", []string{"ebpf", "opentelemetry", "otel", "tracing", "prometheus"}},
	{"Service Mesh", []string{"service mesh", "istio", "linkerd", "envoy"}},
	{"WebAssembly", []string{"wasm", "webassembly", "wasi", "wasmtime"}},
	{"GPU Acceleration", []string{"gpu", "cuda", "h100", "a100", "tpu", "accelerator"}},
	{"Fine-tuning Methods", []string{"fine-tuning", "lora", "qlora", "adapter", "peft"}},
	{"Diffusion Models", []string{"diffusion model", "stable diffusion", "score matching", "denoising"}},
}

func defaultSignalRules() []signalRule {
	return []signalRule{
		{
			signalType:  models.SignalModelRelease,
			minScore:    0.25,
			description: "New model release or launch",
			keywords: []string{
				"release", "launch", "introducing", "announcing", "available now",
				"new model", "open weights", "model weights", "checkpoint released",
				"hugging face", "api available",
			},
		},
		{
			signalType:  models.SignalResearchBreakthrough,
			minScore:    0.30,
			description: "Research breakthrough or significant advance",
			keywords: []string{
				"state-of-the-art", "sota", "outperforms", "surpasses",
				"breakthrough", "significant improvement", "novel approach",
				"first to", "new record", "unprecedented",
			},
		},
		{
			signalType:  models.SignalInfrastructureRelease,
			minScore:    0.25,
			description: "Infrastructure tool or platform release",
			keywords: []string{
				"release", "v1.", "v2.", "version", "general availability", "ga",
				"kubernetes", "cncf", "cloud native", "operator", "platform",
				"vllm", "triton", "tgi", "ray serve",
			},
		},
		{
			signalType:  models.SignalDatasetRelease,
			minScore:    0.30,
			description: "New dataset or benchmark release",
			keywords: []string{
				"dataset", "benchmark", "evaluation suite", "corpus", "releasing data",
				"open data", "data release", "training data",
			},
		},
		{
			signalType:  models.SignalBenchmarkResult,
			minScore:    0.25,
			description: "Benchmark performance result",
			keywords: []string{
				"benchmark", "leaderboard", "mmlu", "hellaswag", "humaneval",
				"gsm8k", "arc", "truthfulqa", "score", "accuracy", "performance",
			},
		},
	}
}

// ─────────────────────────────────────────────
//  Helpers
// ─────────────────────────────────────────────

// scoreRule returns a normalised score and matched keywords for a corpus.
func scoreRule(corpus string, keywords []string) (float64, []string) {
	var matched []string
	for _, kw := range keywords {
		if strings.Contains(corpus, kw) {
			matched = append(matched, kw)
		}
	}
	if len(keywords) == 0 {
		return 0, nil
	}
	return float64(len(matched)) / float64(len(keywords)), matched
}

// windowParams returns (since, numBuckets, bucketDuration) for a window string.
func windowParams(window string) (time.Time, int, time.Duration) {
	switch window {
	case "24h":
		return time.Now().Add(-24 * time.Hour), 24, time.Hour
	case "7d":
		return time.Now().Add(-7 * 24 * time.Hour), 7, 24 * time.Hour
	default:
		return time.Now().Add(-24 * time.Hour), 24, time.Hour
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
