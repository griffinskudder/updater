.PHONY: build run test fmt vet clean tidy docs-serve docs-build docs-clean help

# Build the application
build:
	go build -o bin/updater ./cmd/updater

# Run the application
run:
	go run ./cmd/updater

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Vet code for issues
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy:
	go mod tidy

# Run all checks (format, vet, test)
check: fmt vet test

# Serve documentation locally for development using Docker
docs-serve:
	@echo "Starting MkDocs development server with Docker..."
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "Docker not found. Please install Docker to use documentation commands."; \
		exit 1; \
	fi
	docker run --rm -it -p 8000:8000 -v "$(PWD)":/docs squidfunk/mkdocs-material:latest

# Build documentation site using Docker
docs-build:
	@echo "Building documentation site with Docker..."
	@if ! command -v docker >/dev/null 2>&1; then \
		echo "Docker not found. Please install Docker to use documentation commands."; \
		exit 1; \
	fi
	docker run --rm -v "$(PWD)":/docs squidfunk/mkdocs-material:latest build

# Clean built documentation
docs-clean:
	@echo "Cleaning documentation build artifacts..."
	rm -rf site/

# Help target
help:
	@echo "Available targets:"
	@echo "  build       - Build the application to bin/updater"
	@echo "  run         - Run the application"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  vet         - Vet code for issues"
	@echo "  clean       - Clean build artifacts"
	@echo "  tidy        - Tidy dependencies"
	@echo "  check       - Run format, vet, and test"
	@echo "  docs-serve  - Start MkDocs development server"
	@echo "  docs-build  - Build documentation site"
	@echo "  docs-clean  - Clean documentation build artifacts"
	@echo "  help        - Show this help"