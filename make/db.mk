##@ Database

SQLC_IMAGE := sqlc/sqlc:latest

.PHONY: sqlc-generate sqlc-vet migrate-up migrate-down migrate-status migrate-create sqlc-diff

sqlc-generate: ## Generate Go code from SQL schemas
	@echo "Generating Go code from SQL schemas..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate

sqlc-vet: ## Validate SQL schemas and queries
	@echo "Validating SQL schemas and queries..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) vet

##@ Migrations

DIALECT ?= sqlite
DSN     ?= ./data/updater.db

migrate-up: ## Apply pending migrations (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" up

migrate-down: ## Roll back one migration (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" down

migrate-status: ## Show migration status (DIALECT=sqlite DSN=./data/updater.db)
	$(GO_DOCKER) go run ./cmd/migrate --dialect $(DIALECT) --dsn "$(DSN)" status

migrate-create: ## Create a new migration file (NAME=add_column DIALECT=sqlite)
ifndef NAME
	$(error NAME is required, e.g. make migrate-create NAME=add_column DIALECT=sqlite)
endif
	@next=$$(($$(ls internal/storage/migrations/$(DIALECT)/[0-9]*.sql 2>/dev/null | sed 's|.*/\([0-9]*\)_.*|\1|' | sort -n | tail -1 || echo 0) + 1)); \
	file=internal/storage/migrations/$(DIALECT)/$$(printf '%03d' $$next)_$(NAME).sql; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > "$$file"; \
	echo "Created migration: $$file"

sqlc-diff: ## Fail if sqlc-generate would change generated output
	@echo "Checking for sqlc drift..."
	docker run --rm -v "$(CURDIR):/src" -w /src $(SQLC_IMAGE) generate
	@if [ -n "$$(git diff --name-only internal/storage/sqlc/)" ]; then \
		echo "FAIL: sqlc-generate produced changes:"; \
		git diff --name-only internal/storage/sqlc/; \
		git checkout -- internal/storage/sqlc/; \
		exit 1; \
	fi
	@echo "OK: generated code is up to date"
