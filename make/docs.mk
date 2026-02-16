##@ Documentation

.PHONY: docs-serve docs-build docs-clean openapi-validate

docs-serve: ## Start MkDocs development server (http://localhost:8000)
	@echo "Starting MkDocs development server with Docker..."
	docker run --rm -it -p 8000:8000 -v "$(CURDIR):/docs" squidfunk/mkdocs-material:latest

docs-build: openapi-validate ## Build documentation site
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
