##@ Go Development

.PHONY: build run test fmt fmt-check vet clean tidy check

build: ## Build the application to bin/updater
	$(GO_DOCKER) go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/$(APP_NAME)

run: ## Run the application
	$(GO_DOCKER) go run ./cmd/$(APP_NAME)

test: ## Run tests
	$(GO_DOCKER) go test ./...

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

check: fmt vet test ## Run format, vet, and test
