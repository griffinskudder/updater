##@ Docker Operations

.PHONY: docker-build docker-scan docker-run docker-dev docker-prod \
    docker-obs-up docker-obs-down docker-clean docker-push

docker-build: ## Build secure Docker image
	@echo "Building secure Docker image..."
	./scripts/docker-build.sh

docker-scan: ## Scan Docker image for vulnerabilities
	@echo "Scanning Docker image for vulnerabilities..."
	./scripts/docker-build.sh --scan

docker-run: ## Run container with security defaults
	@echo "Running Docker container with security defaults..."
	docker run --rm -p 8080:8080 \
		--read-only \
		--tmpfs /tmp:noexec,nosuid,size=10m \
		--cap-drop=ALL \
		--security-opt=no-new-privileges:true \
		-e UPDATER_CONFIG_SECTION=development \
		localhost/$(APP_NAME):latest

docker-dev: ## Start development environment with Docker Compose
	@echo "Starting development environment..."
	docker-compose up -d
	@echo "Service available at http://localhost:8080"
	@echo "View logs: docker-compose logs -f"
	@echo "Stop: docker-compose down"

docker-prod: ## Run with production configuration (for testing)
	@echo "Running container with production configuration..."
	@echo "Note: This is for testing production config locally"
	docker run -d --name $(APP_NAME)-prod-test -p 8080:8080 \
		--restart=unless-stopped \
		--read-only \
		--tmpfs /tmp:noexec,nosuid,nodev,size=5m \
		--tmpfs /app/data:noexec,nosuid,size=50m \
		--cap-drop=ALL \
		--security-opt=no-new-privileges:true \
		--memory=256m --cpus="1.0" \
		-e UPDATER_CONFIG_SECTION=production \
		--env-file=.env.example \
		localhost/$(APP_NAME):latest
	@echo "Production test container started"
	@echo "View logs: docker logs -f $(APP_NAME)-prod-test"
	@echo "Stop: docker stop $(APP_NAME)-prod-test && docker rm $(APP_NAME)-prod-test"

docker-obs-up: ## Start observability stack
	@echo "Starting observability stack..."
	docker compose -f docker-compose.yml -f docker-compose.observability.yml up -d --build
	@echo "Services:"
	@echo "  Updater:    http://localhost:8080"
	@echo "  Metrics:    http://localhost:9090/metrics"
	@echo "  Jaeger UI:  http://localhost:16686"
	@echo "  Prometheus: http://localhost:9091"
	@echo "  Grafana:    http://localhost:3000"

docker-obs-down: ## Stop observability stack
	docker compose -f docker-compose.yml -f docker-compose.observability.yml down

docker-clean: ## Clean Docker artifacts
	@echo "Cleaning Docker artifacts..."
	docker system prune -f
	docker image prune -f

docker-push: ## Build and push Docker image to registry
	@echo "Building and pushing Docker image to registry..."
	./scripts/docker-build.sh --push
