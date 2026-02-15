# Root Makefile - shared variables, includes, auto-documenting help
#
# All targets run inside Docker containers. Only Docker is required locally.
# See make/*.mk for target definitions.

# Guard: require Docker
DOCKER := $(shell command -v docker 2>/dev/null)
ifndef DOCKER
    $(error Docker is required but not found. Install Docker to use this Makefile)
endif

# Shared variables
APP_NAME   := updater
BIN_DIR    := bin
GO_IMAGE   := golang:1.25-alpine
GO_MOD_CACHE   := $(APP_NAME)-go-mod-cache
GO_BUILD_CACHE := $(APP_NAME)-go-build-cache
GO_DOCKER  := docker run --rm \
    -v "$(CURDIR):/app" \
    -v "$(GO_MOD_CACHE):/go/pkg/mod" \
    -v "$(GO_BUILD_CACHE):/root/.cache/go-build" \
    -w /app \
    -e CGO_ENABLED=0 \
    $(GO_IMAGE)

# Include all category makefiles
include make/*.mk

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@awk '/^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)}' $(MAKEFILE_LIST)
	@awk '/^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-18s\033[0m %s\n", $$1, substr($$0, index($$0, "##") + 3)}' $(MAKEFILE_LIST)
