##@ Documentation

.PHONY: docs-serve docs-build docs-clean docs-generate docs-db openapi-validate

DOCS_PG_CONTAINER := updater-docs-pg

docs-serve: ## Start MkDocs development server (http://localhost:8000)
	@echo "Starting MkDocs development server with Docker..."
	docker run --rm -it -p 8000:8000 -v "$(CURDIR):/docs" squidfunk/mkdocs-material:latest

docs-build: openapi-validate docs-generate docs-db ## Build documentation site
	@echo "Building documentation site with Docker..."
	docker run --rm -v "$(CURDIR):/docs" squidfunk/mkdocs-material:latest build

docs-clean: ## Clean documentation build artifacts
	@echo "Cleaning documentation build artifacts..."
	rm -rf site

openapi-validate: ## Validate OpenAPI specification with Redocly CLI
	@echo "Validating OpenAPI specification..."
	docker run --rm \
	    -v "$(CURDIR)/internal/api/openapi:/spec:ro" \
	    redocly/cli:latest \
	    lint /spec/openapi.yaml

docs-generate: ## Generate model reference docs from Go source comments using gomarkdoc
	@echo "Generating model reference docs..."
	@mkdir -p docs/models/auto
	docker run --rm \
		-v "$(CURDIR):/app" \
		-v updater-go-mod-cache:/go/pkg/mod \
		-v updater-go-build-cache:/root/.cache/go-build \
		-w /app \
		golang:1.25-alpine \
		sh -c "go run github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0 \
			--output docs/models/auto/models.md \
			./internal/models/..."

docs-db: ## Generate database schema docs from PostgreSQL migrations using tbls
	@echo "Generating database schema docs..."
	@docker rm -f $(DOCS_PG_CONTAINER) 2>/dev/null || true
	docker run -d --name $(DOCS_PG_CONTAINER) \
		-v "$(CURDIR)/internal/storage/sqlc/schema/postgres:/migrations:ro" \
		-e POSTGRES_PASSWORD=docs \
		-e POSTGRES_USER=docs \
		-e POSTGRES_DB=updater \
		postgres:17-alpine
	@until docker exec $(DOCS_PG_CONTAINER) psql -U docs -d updater -c "SELECT 1" >/dev/null 2>&1; do sleep 1; done
	@docker exec $(DOCS_PG_CONTAINER) sh -c 'for f in /migrations/*.sql; do psql -U docs -d updater -f "$$f"; done'
	@rm -rf docs/db && mkdir -p docs/db
	docker run --rm \
		-v "$(CURDIR)/docs/db:/out" \
		--network container:$(DOCS_PG_CONTAINER) \
		ghcr.io/k1low/tbls:latest doc --force \
		"postgres://docs:docs@127.0.0.1/updater?sslmode=disable" /out
	@docker stop $(DOCS_PG_CONTAINER) && docker rm $(DOCS_PG_CONTAINER)
