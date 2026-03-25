# AIRA — AI Research Aggregator

> A modular, Go-based CLI intelligence platform for monitoring AI research,  
> model releases, and cloud-native ecosystem updates via RSS feeds.

```
  ┌────────────────────────────────────────────────────────────┐
  │  collect → parse → classify → signals → trends → digest   │
  └────────────────────────────────────────────────────────────┘
```

AIRA continuously monitors high-signal RSS feeds from arXiv, Papers with Code,
CNCF, OpenAI, HuggingFace, Kubernetes.io, and more — then classifies, analyses,
and surfaces the most relevant developments in a clean terminal digest.

---

## Features

| Feature | Description |
|---|---|
| **Collect** | Parallel RSS/Atom fetching with retry, deduplication, and configurable concurrency |
| **Parse** | HTML stripping, entity decoding, author extraction, domain-tag inference |
| **Classify** | Weighted keyword + source-name heuristics → 4 categories + confidence score |
| **Signals** | Rule-based detection of model releases, research breakthroughs, infra launches |
| **Trends** | Time-bucketed frequency analysis with log-velocity scoring across 15+ topics |
| **Digest** | Structured Markdown report with sections, signals table, and trending topics |
| **Schedule** | Cron-compatible background daemon for fully automated pipeline execution |
| **Sources** | Full CRUD management of feed sources with built-in curated defaults |

---

## Installation

### From source

```bash
git clone https://github.com/zakachaara/aira
cd aira
make build           # produces ./aira binary
make install         # installs to $GOPATH/bin
```

**Requirements:** Go 1.22+, CGO enabled (required by `go-sqlite3`)


## Quick Start

```bash
# 1. Bootstrap: create config, DB, and seed 16 default feeds
aira init

# 2. Fetch all active sources
aira collect

# 3. Normalise raw entries into structured records
aira parse

# 4. Categorise entries by topic
aira classify

# 5. Extract high-value events
aira signals --extract

# 6. Detect emerging topics
aira trends --detect

# 7. Generate today's intelligence digest
aira digest
```

---

## Commands

### `aira collect`

Fetches all active RSS/Atom sources in parallel and persists new raw entries.

```
aira collect
aira collect --log-level debug
```

- Concurrency: `collect.max_concurrent` (default 5)
- Timeout: `collect.timeout_seconds` (default 30)
- Retries: `collect.retry_attempts` (default 2)
- Deduplication: by GUID (or URL if GUID absent)

---

### `aira parse`

Normalises unparsed raw entries — strips HTML, decodes entities, infers
domain tags, extracts author lists, and truncates summaries.

```
aira parse
```

---

### `aira classify`

Assigns a category and confidence score to each unclassified entry using
weighted keyword matching against four rule sets.

```
aira classify
aira classify --show --limit 50
```

**Categories:**

| Category | Description |
|---|---|
| `ai_research` | Papers, studies, benchmarks, preprints |
| `model_release` | New model weights, APIs, checkpoints |
| `ai_infrastructure` | MLOps, inference serving, vector DBs |
| `cloud_native` | Kubernetes, CNCF, eBPF, observability |

---

### `aira signals`

Extracts and displays high-value events ranked by relevance score.

```
aira signals
aira signals --extract --hours 48
aira signals --limit 30
```

**Signal types:**

| Type | Description |
|---|---|
| `model_release` | New model weight or API announcement |
| `research_breakthrough` | SOTA result or novel technique |
| `infrastructure_release` | New MLOps or cloud-native tool version |
| `dataset_release` | New training or evaluation dataset |
| `benchmark_result` | Performance metric on known benchmark |

---

### `aira trends`

Surfaces topics whose mention frequency is accelerating, using a log-velocity
score across 15 pre-defined topic clusters.

```
aira trends
aira trends --detect --window 24h
aira trends --window 7d --limit 15
```

**Tracked topics include:** LLM Agents, Reasoning Models, RAG & Retrieval,
Open Source Models, GPU Acceleration, Fine-tuning Methods, Kubernetes,
eBPF & Observability, Service Mesh, WebAssembly, Diffusion Models, and more.

---

### `aira digest`

Generates a structured Markdown intelligence report and prints it to stdout.

```
aira digest
aira digest --days 3
aira digest --output ~/reports/today.md
aira digest --list
aira digest --show-latest
```

Output is also saved to `digest.output_dir` (default `~/.aira/digests/`).

---

### `aira sources`

Full CRUD management of RSS feed sources.

```
aira sources list                              # list active sources
aira sources list --all                        # include inactive
aira sources add --name "arXiv AI" \
                --url  "https://arxiv.org/rss/cs.AI" \
                --category ai_research
aira sources remove 5                          # deactivate by ID
aira sources enable  5                         # re-enable
aira sources init-defaults                     # seed built-in list
```

**Built-in default sources (16):**

| Source | Category |
|---|---|
| arXiv cs.AI / cs.LG / cs.CL / cs.CV | `ai_research` |
| Papers With Code | `ai_research` |
| OpenAI Blog | `model_release` |
| Anthropic News | `model_release` |
| Google DeepMind Blog | `ai_research` |
| Mistral AI Blog | `model_release` |
| Meta AI Blog | `ai_research` |
| Hugging Face Blog | `model_release` |
| LangChain Blog | `ai_infrastructure` |
| LlamaIndex Blog | `ai_infrastructure` |
| CNCF Blog | `cloud_native` |
| Kubernetes Blog | `cloud_native` |
| The New Stack | `cloud_native` |

---

### `aira schedule`

Runs AIRA as a long-lived background daemon with configurable cron schedules.

```
aira schedule
aira schedule --run-now        # run full pipeline once, then schedule
```

Default schedules (configurable via `~/.aira/config.yaml`):

```
collect:  "0 */4 * * *"   # every 4 hours
digest:   "0 8 * * *"     # daily at 08:00
```

---

## Configuration

AIRA is configured via `~/.aira/config.yaml` (auto-created by `aira init`).

```yaml
database:
  driver: sqlite3              # sqlite3 (default) | postgres
  dsn: ~/.aira/aira.db

log:
  level: info                  # debug | info | warn | error
  pretty: true                 # false = JSON (for log aggregators)

collect:
  timeout_seconds: 30
  max_concurrent: 5
  retry_attempts: 2
  user_agent: "AIRA/1.0 (+https://github.com/zakachaara/aira)"

digest:
  max_entries_per_section: 10
  trend_window_days: 7
  output_dir: ~/.aira/digests

schedule:
  collect: "0 */4 * * *"
  digest:  "0 8 * * *"
```

**Environment overrides** use the `AIRA_` prefix:

```bash
AIRA_DATABASE_DSN=/data/aira.db aira collect
AIRA_LOG_LEVEL=debug aira parse
AIRA_CONFIG=/etc/aira/config.yaml aira digest
```

---

## Architecture

```
aira/
├── cmd/aira/                  # CLI layer (Cobra commands)
│   ├── main.go                # root command, bootstrap, DI
│   ├── cmd_collect.go
│   ├── cmd_parse.go
│   ├── cmd_classify.go
│   ├── cmd_signals.go
│   ├── cmd_trends.go
│   ├── cmd_digest.go
│   ├── cmd_sources.go
│   ├── cmd_schedule.go
│   ├── cmd_init.go
│   └── runtime.go             # pipeline wiring, seed data
│
└── internal/
    ├── models/     # domain types: Source, Entry, Signal, Trend, Digest
    ├── config/     # Viper-based config loading with defaults
    ├── logger/     # zap wrapper (structured logging)
    ├── storage/    # Repository interfaces + SQLite implementation
    ├── collector/  # RSS/Atom fetcher (gofeed, parallel, retry)
    ├── parser/     # Normaliser: HTML stripping, tag extraction
    ├── classifier/ # Weighted keyword → category + confidence
    ├── analyzer/   # Signal extraction + trend detection
    ├── digest/     # Markdown report generator
    ├── delivery/   # Terminal renderer (colour, tables, sparklines)
    └── scheduler/  # robfig/cron wrapper for automated runs
```

**Dependency flow:** CLI → Core packages → Storage → Models  
Each layer imports only downward; no circular dependencies.

---

## Database Schema

AIRA uses SQLite (WAL mode) by default with five tables:

```
sources      – configured RSS feeds
raw_entries  – unprocessed items as fetched
entries      – normalised, classified records
signals      – detected high-value events
trends       – computed trend snapshots
digests      – generated report records
```

All indexes are created automatically by `Migrate()` on first run.

---

## Development

```bash
make tidy          # go mod tidy + verify
make vet           # go vet
make lint          # golangci-lint
make test          # go test -race ./...
make test-cover    # generate coverage.html
make build         # compile binary
make pipeline      # init + full pipeline run
make release       # cross-compile for all platforms
make docker        # build Docker image
```

---

## Roadmap

- [ ] PostgreSQL support (storage driver already abstracted)
- [ ] LLM-powered summarisation pass (pluggable via config)
- [ ] Webhook / Slack delivery channel
- [ ] HTTP API server mode (`aira serve`)
- [ ] Custom classification rules via YAML config
- [ ] Feed health monitoring and alerting
- [ ] Export to OPML / JSON / CSV

---

## License

MIT — see [LICENSE](LICENSE)
