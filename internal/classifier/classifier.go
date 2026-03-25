// Package classifier assigns categories and confidence scores to parsed entries.
package classifier

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"go.uber.org/zap"

	"github.com/zakachaara/aira/internal/logger"
	"github.com/zakachaara/aira/internal/models"
	"github.com/zakachaara/aira/internal/storage"
)

const batchSize = 500

// rule associates a set of keywords with a category and a per-match weight.
type rule struct {
	category models.Category
	keywords []string
	weight   float64
}

// Classifier assigns categories to entries using weighted keyword matching.
type Classifier struct {
	repo  storage.Repository
	rules []rule
}

// New creates a Classifier with sensible default rules.
func New(repo storage.Repository) *Classifier {
	return &Classifier{
		repo:  repo,
		rules: defaultRules(),
	}
}

// Run classifies all unclassified entries in the repository.
func (c *Classifier) Run(ctx context.Context) (int, error) {
	start := time.Now()
	entries, err := c.repo.ListUnclassified(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("listing unclassified: %w", err)
	}
	logger.Info("starting classify run", zap.Int("unclassified", len(entries)))

	classified := 0
	for _, e := range entries {
		cat, conf := c.classify(e)
		e.Category = cat
		e.Confidence = conf
		e.ClassifiedAt = time.Now().UTC()
		if _, err := c.repo.SaveEntry(ctx, e); err != nil {
			logger.Error("saving classified entry", zap.Int64("entry_id", e.ID), zap.Error(err))
			continue
		}
		classified++
	}

	logger.Info("classify run complete",
		zap.Int("classified", classified),
		zap.Duration("duration", time.Since(start)))
	return classified, nil
}

// classify returns the best-matching category and a 0–1 confidence score.
func (c *Classifier) classify(e *models.Entry) (models.Category, float64) {
	corpus := strings.ToLower(e.Title + " " + e.Summary + " " + strings.Join(e.Tags, " "))

	scores := make(map[models.Category]float64)
	for _, r := range c.rules {
		for _, kw := range r.keywords {
			if strings.Contains(corpus, kw) {
				scores[r.category] += r.weight
			}
		}
	}

	// Source-name heuristic boost
	srcLower := strings.ToLower(e.SourceName)
	switch {
	case strings.Contains(srcLower, "arxiv") || strings.Contains(srcLower, "papers with code"):
		scores[models.CategoryAIResearch] += 0.4
	case strings.Contains(srcLower, "cncf") || strings.Contains(srcLower, "cloud native"):
		scores[models.CategoryCloudNative] += 0.4
	case strings.Contains(srcLower, "openai") || strings.Contains(srcLower, "anthropic") ||
		strings.Contains(srcLower, "google deepmind") || strings.Contains(srcLower, "mistral"):
		scores[models.CategoryModelRelease] += 0.3
	}

	best := models.CategoryUncategorized
	var bestScore float64
	for cat, score := range scores {
		if score > bestScore {
			bestScore = score
			best = cat
		}
	}

	// Normalise confidence to 0–1 using a simple sigmoid-like cap
	conf := clamp(bestScore/3.0, 0, 1)
	if best == models.CategoryUncategorized {
		conf = 0
	}
	return best, conf
}

// ─────────────────────────────────────────────
//  Default classification rules
// ─────────────────────────────────────────────

func defaultRules() []rule {
	return []rule{
		// ── AI Research ────────────────────────────────────────────────
		{
			category: models.CategoryAIResearch,
			weight:   0.5,
			keywords: words(`
				neural network deep learning machine learning arxiv preprint
				paper research study experiment ablation evaluation
				transformer attention mechanism self-supervised contrastive
				generative adversarial diffusion score model
				natural language processing nlp computer vision cv
				reinforcement learning reward policy gradient
				foundation model pre-training pre-trained
				benchmark leaderboard state-of-the-art sota
				dataset training fine-tuning instruction tuning
				rlhf dpo ppo proximal policy optimization
				chain-of-thought reasoning emergent capability
				multimodal vision language audio speech
			`),
		},
		// ── Model Releases ─────────────────────────────────────────────
		{
			category: models.CategoryModelRelease,
			weight:   0.7,
			keywords: words(`
				model release launch introducing new model
				announcing gpt claude gemini llama mistral falcon
				open weights open source model weights checkpoint
				api access available now model card
				version update upgrade improved performance
				quantized gguf ggml ollama huggingface model hub
				inference endpoint deployment production
				context window tokens parameters billion
			`),
		},
		// ── AI Infrastructure ──────────────────────────────────────────
		{
			category: models.CategoryAIInfrastructure,
			weight:   0.6,
			keywords: words(`
				inference serving triton tensorrt vllm tgi text generation inference
				gpu cluster distributed training accelerator tpu
				mlops mlflow kubeflow ray pytorch lightning
				model optimization quantization pruning distillation
				onnx runtime deployment serving latency throughput
				vector database embedding store retrieval
				rag pipeline orchestration langchain llamaindex
				monitoring observability model drift data drift
				feature store data pipeline etl batch inference
				cuda rocm hip mixed precision fp16 bf16 int8
			`),
		},
		// ── Cloud Native ───────────────────────────────────────────────
		{
			category: models.CategoryCloudNative,
			weight:   0.6,
			keywords: words(`
				kubernetes k8s container docker helm
				cncf cloud native computing foundation
				service mesh istio linkerd envoy
				prometheus grafana loki tempo opentelemetry otel
				ebpf cilium calico network policy
				wasm webassembly wasi
				argocd flux gitops continuous delivery
				tekton pipeline ci cd
				etcd raft distributed consensus
				operator custom resource definition crd
				knative serverless event-driven
				cert-manager ingress nginx gateway api
				security rbac pod security admission webhook
				cluster autoscaler karpenter node provisioning
			`),
		},
	}
}

// words splits a multiline whitespace-separated keyword string into a slice.
func words(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	return fields
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
