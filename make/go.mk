##@ Go Development

.PHONY: build run test integration-test cover fmt fmt-check vet clean tidy check security secrets

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -X 'updater/internal/version.Version=$(VERSION)' \
              -X 'updater/internal/version.GitCommit=$(GIT_COMMIT)' \
              -X 'updater/internal/version.BuildDate=$(BUILD_DATE)'

build: ## Build the application to bin/updater
	$(GO_DOCKER) go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

run: ## Run the application
	$(GO_DOCKER) go run ./cmd/$(APP_NAME)

test: ## Run tests
	$(GO_DOCKER) go test ./...

integration-test: ## Run integration tests
	$(GO_DOCKER) go test -tags integration ./internal/integration/...

cover: ## Run tests with coverage report
	$(GO_DOCKER) sh -c 'go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep ^total'

fmt: ## Format code
	$(GO_DOCKER) go fmt ./...

fmt-check: ## Check formatting without modifying files
	$(GO_DOCKER) sh -c 'unformatted=$$(gofmt -l .); [ -z "$$unformatted" ] || (printf "Unformatted files:\n$$unformatted\n" && exit 1)'

vet: ## Vet code for issues
	$(GO_DOCKER) go vet ./...

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)

tidy: ## Tidy dependencies
	$(GO_DOCKER) go mod tidy

check: fmt-check vet test ## Run format check, vet, and test

security: ## Run gosec security scanner
	docker run --rm \
		-v "$(CURDIR):/app" \
		-w /app \
		securego/gosec:latest \
		-severity high -confidence medium ./...

secrets: ## Scan for committed secrets with gitleaks
	docker run --rm \
		-v "$(CURDIR):/repo" \
		zricethezav/gitleaks:latest \
		detect --source /repo --config /repo/.gitleaks.toml
