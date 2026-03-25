# ─────────────────────────────────────────────────────────────────────────────
# AIRA – Multi-stage Dockerfile
# ─────────────────────────────────────────────────────────────────────────────

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-bookworm AS builder

ARG VERSION=dev
ARG GIT_COMMIT=none
ARG BUILD_DATE=unknown

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Compile (CGO required for go-sqlite3)
RUN CGO_ENABLED=1 GOOS=linux go build \
      -ldflags "-X main.Version=${VERSION} \
                -X main.GitCommit=${GIT_COMMIT} \
                -X main.BuildDate=${BUILD_DATE} \
                -s -w" \
      -o /usr/local/bin/aira \
      ./cmd/aira

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates \
      tzdata \
    && rm -rf /var/lib/apt/lists/*

# Non-root user
RUN useradd -m -u 1001 aira
USER aira
WORKDIR /home/aira

COPY --from=builder /usr/local/bin/aira /usr/local/bin/aira

# Persist DB and config outside the container
VOLUME ["/home/aira/.aira"]

ENTRYPOINT ["aira"]
CMD ["--help"]
