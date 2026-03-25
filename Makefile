# ─────────────────────────────────────────────────────────────────────────────
# AIRA Makefile
# ─────────────────────────────────────────────────────────────────────────────

BINARY      := aira
CMD_PATH    := ./cmd/aira
BUILD_DIR   := ./bin

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -ldflags "-X main.Version=$(VERSION) \
                          -X main.GitCommit=$(GIT_COMMIT) \
                          -X main.BuildDate=$(BUILD_DATE) \
                          -s -w"

GO          := go
GOFLAGS     ?=

.PHONY: all build clean test lint vet tidy run install docker-build help

# Default target
all: tidy vet build

## build: Compile AIRA binary to ./bin/aira
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "  ✓ Built $(BUILD_DIR)/$(BINARY) ($(VERSION))"

## install: Install AIRA to $GOPATH/bin
install:
	CGO_ENABLED=1 $(GO) install $(GOFLAGS) $(LDFLAGS) $(CMD_PATH)
	@echo "  ✓ Installed aira to $(GOPATH)/bin/aira"

## run: Build and run with --help
run: build
	$(BUILD_DIR)/$(BINARY) --help

## test: Run all tests
test:
	CGO_ENABLED=1 $(GO) test ./... -v -count=1 -race -timeout 120s

## test-short: Run tests skipping slow integration tests
test-short:
	CGO_ENABLED=1 $(GO) test ./... -short -count=1 -timeout 60s

## cover: Run tests with coverage report
cover:
	CGO_ENABLED=1 $(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "  ✓ Coverage report: coverage.html"

## lint: Run golangci-lint (requires golangci-lint installed)
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## tidy: Tidy and verify go.mod / go.sum
tidy:
	$(GO) mod tidy
	$(GO) mod verify

## clean: Remove build artefacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html
	@echo "  ✓ Cleaned"

## docker-build: Build the AIRA Docker image
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t aira:$(VERSION) \
		-t aira:latest \
		.
	@echo "  ✓ Docker image: aira:$(VERSION)"

## docker-run: Run AIRA in Docker (mount ~/.aira for persistence)
docker-run:
	docker run --rm -it \
		-v $(HOME)/.aira:/root/.aira \
		aira:latest --help

## release: Cross-compile for Linux, macOS, Windows
release: tidy
	@mkdir -p $(BUILD_DIR)/release
	@for os_arch in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
		GOOS=$$(echo $$os_arch | cut -d/ -f1); \
		GOARCH=$$(echo $$os_arch | cut -d/ -f2); \
		out=$(BUILD_DIR)/release/aira-$$GOOS-$$GOARCH; \
		[ "$$GOOS" = "windows" ] && out=$$out.exe; \
		echo "  building $$GOOS/$$GOARCH…"; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH \
			$(GO) build $(LDFLAGS) -o $$out $(CMD_PATH) 2>/dev/null || \
		CGO_ENABLED=1 GOOS=$$GOOS GOARCH=$$GOARCH \
			$(GO) build $(LDFLAGS) -o $$out $(CMD_PATH); \
	done
	@echo "  ✓ Release binaries in $(BUILD_DIR)/release/"

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
