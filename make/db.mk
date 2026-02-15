##@ Database

SQLC_IMAGE := sqlc/sqlc:latest

.PHONY: sqlc-generate sqlc-vet

sqlc-generate: ## Generate Go code from SQL schemas
	@echo "Generating Go code from SQL schemas..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate

sqlc-vet: ## Validate SQL schemas and queries
	@echo "Validating SQL schemas and queries..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) vet
