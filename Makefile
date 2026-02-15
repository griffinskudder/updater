.PHONY: build run test fmt vet clean tidy check docs-serve docs-build docs-clean \
	docker-build docker-scan docker-run docker-dev docker-prod \
	docker-clean docker-push docker-obs-up docker-obs-down \
	sqlc-generate sqlc-vet help

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
	rm -rf bin

# Tidy dependencies
tidy:
	go mod tidy

# Run all checks (format, vet, test)
check: fmt vet test

# Serve documentation locally for development using Docker
docs-serve:
	@echo "Starting MkDocs development server with Docker..."
	docker run --rm -it -p 8000:8000 -v "$(CURDIR):/docs" squidfunk/mkdocs-material:latest

# Build documentation site using Docker
docs-build:
	@echo "Building documentation site with Docker..."
	docker run --rm -v "$(CURDIR):/docs" squidfunk/mkdocs-material:latest build

# Clean built documentation
docs-clean:
	@echo "Cleaning documentation build artifacts..."
	rm -rf site

# =============================================================================
# Docker Commands
# =============================================================================

# Build Docker image
docker-build:
	@echo "Building secure Docker image..."
	./scripts/docker-build.sh

# Scan Docker image for vulnerabilities
docker-scan:
	@echo "Scanning Docker image for vulnerabilities..."
	./scripts/docker-build.sh --scan

# Run Docker container with security defaults
docker-run:
	@echo "Running Docker container with security defaults..."
	docker run --rm -p 8080:8080 --read-only --tmpfs /tmp:noexec,nosuid,size=10m --cap-drop=ALL --security-opt=no-new-privileges:true -e UPDATER_CONFIG_SECTION=development localhost/updater:latest

# Start development environment with Docker Compose
docker-dev:
	@echo "Starting development environment..."
	docker-compose up -d
	@echo "Service available at http://localhost:8080"
	@echo "View logs: docker-compose logs -f"
	@echo "Stop: docker-compose down"

# Run container with production configuration (for testing)
docker-prod:
	@echo "Running container with production configuration..."
	@echo "Note: This is for testing production config locally"
	docker run -d --name updater-prod-test -p 8080:8080 --restart=unless-stopped --read-only --tmpfs /tmp:noexec,nosuid,nodev,size=5m --tmpfs /app/data:noexec,nosuid,size=50m --cap-drop=ALL --security-opt=no-new-privileges:true --memory=256m --cpus="1.0" -e UPDATER_CONFIG_SECTION=production --env-file=.env.example localhost/updater:latest
	@echo "Production test container started"
	@echo "View logs: docker logs -f updater-prod-test"
	@echo "Stop: docker stop updater-prod-test && docker rm updater-prod-test"

# Start observability stack (updater + Jaeger + Prometheus + Grafana)
docker-obs-up:
	@echo "Starting observability stack..."
	docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d
	@echo "Services:"
	@echo "  Updater:    http://localhost:8080"
	@echo "  Metrics:    http://localhost:9090/metrics"
	@echo "  Jaeger UI:  http://localhost:16686"
	@echo "  Prometheus: http://localhost:9091"
	@echo "  Grafana:    http://localhost:3000"

# Stop observability stack
docker-obs-down:
	docker compose -f docker-compose.yml -f docker-compose.observability.yml down

# Clean Docker artifacts
docker-clean:
	@echo "Cleaning Docker artifacts..."
	docker system prune -f
	docker image prune -f

# Push Docker image to registry
docker-push:
	@echo "Building and pushing Docker image to registry..."
	./scripts/docker-build.sh --push

# =============================================================================
# Database Commands
# =============================================================================

# Generate Go code from SQL schemas using sqlc
sqlc-generate:
	@echo "Generating Go code from SQL schemas..."
	sqlc generate

# Validate SQL schemas and queries
sqlc-vet:
	@echo "Validating SQL schemas and queries..."
	sqlc vet

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Go Development:"
	@echo "  build         - Build the application to bin/updater"
	@echo "  run           - Run the application"
	@echo "  test          - Run tests"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code for issues"
	@echo "  clean         - Clean build artifacts"
	@echo "  tidy          - Tidy dependencies"
	@echo "  check         - Run format, vet, and test"
	@echo ""
	@echo "Documentation:"
	@echo "  docs-serve    - Start MkDocs development server"
	@echo "  docs-build    - Build documentation site"
	@echo "  docs-clean    - Clean documentation build artifacts"
	@echo ""
	@echo "Docker Commands:"
	@echo "  docker-build    - Build secure Docker image"
	@echo "  docker-scan     - Scan Docker image for vulnerabilities"
	@echo "  docker-run      - Run Docker container with security defaults"
	@echo "  docker-dev      - Start development environment with Docker Compose"
	@echo "  docker-prod     - Run container with production configuration (for testing)"
	@echo "  docker-obs-up   - Start observability stack (updater + Jaeger + Prometheus + Grafana)"
	@echo "  docker-obs-down - Stop observability stack"
	@echo "  docker-clean    - Clean Docker artifacts"
	@echo "  docker-push     - Build and push Docker image to registry"
	@echo ""
	@echo "Database Commands:"
	@echo "  sqlc-generate   - Generate Go code from SQL schemas using sqlc"
	@echo "  sqlc-vet        - Validate SQL schemas and queries"
	@echo ""
	@echo "  help          - Show this help"
