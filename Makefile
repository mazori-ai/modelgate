# ModelGate Makefile
# =============================================================================
# Build targets for backend, frontend, GraphQL generation, and Docker
# =============================================================================

.PHONY: all build modelgate graphql web web-build web-dev web-install web-logs \
        docker docker-build docker-build-local docker-push docker-run docker-stop docker-logs docker-restart \
        compose-up compose-down compose-logs compose-clean compose-rebuild \
        run run-foreground run-all stop stop-all logs dev test lint fmt fmt-go tidy clean setup tools help

# Variables
BINARY_NAME := modelgate
BUILD_DIR := bin
CMD_DIR := cmd/modelgate
WEB_DIR := web
PID_FILE := $(BUILD_DIR)/.modelgate.pid

# Config file (can be overridden: make run CONFIG=myconfig.toml)
CONFIG ?= config.toml

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Build version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker settings
DOCKER_REGISTRY ?= 
IMAGE_TAG ?= latest
DOCKER_IMAGE := $(DOCKER_REGISTRY)/modelgate:$(IMAGE_TAG)
DOCKER_CONTAINER := modelgate
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64

# Docker Compose (optional - for those who have it installed)
DOCKER_COMPOSE ?= docker-compose

# =============================================================================
# Main Targets
# =============================================================================

# Build everything (backend + frontend)
all: modelgate web-build
	@echo "✅ All components built successfully"

# Alias for backward compatibility
build: modelgate

# =============================================================================
# Backend (Go)
# =============================================================================

# Build the ModelGate binary
modelgate: fmt-go
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Format Go code (runs before build)
fmt-go:
	@gofmt -w cmd/ internal/

# Run the application (builds backend + web, runs in background)
run: modelgate
	@# Auto-build web UI if not present
	@if [ ! -d "$(WEB_DIR)/dist" ]; then \
		echo "🌐 Web UI not built. Building..."; \
		cd $(WEB_DIR) && pnpm install --silent && pnpm run build; \
		echo "✅ Web UI built"; \
	fi
	@echo "🚀 Starting ModelGate with config: $(CONFIG)"
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "⚠️  ModelGate is already running (PID: $$(cat $(PID_FILE)))"; \
		echo "   Run 'make stop' first to stop it."; \
		exit 1; \
	fi
	@nohup ./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG) > $(BUILD_DIR)/modelgate.log 2>&1 & echo $$! > $(PID_FILE)
	@sleep 1
	@if kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ""; \
		echo "╔══════════════════════════════════════════════════════════════════╗"; \
		echo "║                      ModelGate Running                           ║"; \
		echo "╚══════════════════════════════════════════════════════════════════╝"; \
		echo ""; \
		echo "   Web UI:      http://localhost:8080"; \
		echo "   API:         http://localhost:8080/v1/"; \
		echo "   GraphQL:     http://localhost:8080/graphql"; \
		echo "   Playground:  http://localhost:8080/playground"; \
		echo ""; \
		echo "   Config: $(CONFIG)"; \
		echo "   Log:    $(BUILD_DIR)/modelgate.log"; \
		echo "   PID:    $$(cat $(PID_FILE))"; \
		echo ""; \
		echo "   Run 'make stop' to stop the server"; \
		echo "   Run 'make logs' to view logs"; \
		echo ""; \
	else \
		echo "❌ Failed to start ModelGate. Check $(BUILD_DIR)/modelgate.log"; \
		exit 1; \
	fi

# Run the application in foreground (for debugging)
run-foreground: modelgate
	@# Auto-build web UI if not present
	@if [ ! -d "$(WEB_DIR)/dist" ]; then \
		echo "🌐 Web UI not built. Building..."; \
		cd $(WEB_DIR) && pnpm install --silent && pnpm run build; \
		echo "✅ Web UI built"; \
	fi
	@echo "🚀 Starting ModelGate (foreground) with config: $(CONFIG)"
	./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG)

# Stop the running application
stop:
	@echo "🛑 Stopping ModelGate..."
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID; \
			sleep 1; \
			if kill -0 $$PID 2>/dev/null; then \
				echo "   Forcing stop..."; \
				kill -9 $$PID 2>/dev/null || true; \
			fi; \
			echo "✅ ModelGate stopped (PID: $$PID)"; \
		else \
			echo "⚠️  Process not running (stale PID file)"; \
		fi; \
		rm -f $(PID_FILE); \
	else \
		echo "⚠️  No PID file found. ModelGate may not be running."; \
		echo "   Checking for running processes..."; \
		pkill -f "$(BINARY_NAME)" 2>/dev/null && echo "✅ Killed running process" || echo "   No process found"; \
	fi

# View logs from running server
logs:
	@if [ -f $(BUILD_DIR)/modelgate.log ]; then \
		tail -f $(BUILD_DIR)/modelgate.log; \
	else \
		echo "⚠️  No log file found. Server may not be running."; \
	fi

# Run in development mode (no build, hot reload with go run)
dev:
	@echo "🔧 Running in development mode with config: $(CONFIG)"
	@go run ./$(CMD_DIR) -config $(CONFIG)

# Run all services (backend + web UI) for local development
run-all: modelgate web-install
	@echo "🚀 Starting ModelGate (backend + web UI)..."
	@# Start backend in background
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "⚠️  Backend already running (PID: $$(cat $(PID_FILE)))"; \
	else \
		nohup ./$(BUILD_DIR)/$(BINARY_NAME) -config $(CONFIG) > $(BUILD_DIR)/modelgate.log 2>&1 & echo $$! > $(PID_FILE); \
		sleep 1; \
		if kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
			echo "✅ Backend started (PID: $$(cat $(PID_FILE)))"; \
		else \
			echo "❌ Failed to start backend"; \
			exit 1; \
		fi; \
	fi
	@# Start web UI in background
	@echo "🌐 Starting Web UI dev server..."
	@cd $(WEB_DIR) && nohup pnpm run dev > ../$(BUILD_DIR)/web.log 2>&1 & echo $$! > ../$(BUILD_DIR)/.web.pid
	@sleep 3
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                    ModelGate Running                             ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "   Backend API:    http://localhost:8080"
	@echo "   GraphQL:        http://localhost:8080/graphql"
	@echo "   Playground:     http://localhost:8080/playground"
	@echo "   Web UI:         http://localhost:5173"
	@echo ""
	@echo "   Run 'make stop-all' to stop all services"
	@echo "   Run 'make logs' for backend logs"
	@echo "   Run 'make web-logs' for web UI logs"
	@echo ""

# Stop all services (backend + web UI)
stop-all: stop
	@echo "🛑 Stopping Web UI..."
	@if [ -f $(BUILD_DIR)/.web.pid ]; then \
		PID=$$(cat $(BUILD_DIR)/.web.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID 2>/dev/null || true; \
			echo "✅ Web UI stopped"; \
		fi; \
		rm -f $(BUILD_DIR)/.web.pid; \
	fi
	@# Also kill any pnpm/vite processes for web
	@pkill -f "vite.*$(WEB_DIR)" 2>/dev/null || true
	@echo "✅ All services stopped"

# View web UI logs
web-logs:
	@if [ -f $(BUILD_DIR)/web.log ]; then \
		tail -f $(BUILD_DIR)/web.log; \
	else \
		echo "⚠️  No web log file found. Web UI may not be running."; \
	fi

# =============================================================================
# GraphQL Code Generation
# =============================================================================

# Generate GraphQL code from schema
graphql:
	@echo "📊 Generating GraphQL code..."
	@cd internal/graphql && go run github.com/99designs/gqlgen generate
	@echo "✅ GraphQL code generated in internal/graphql/generated/"

# Install gqlgen tool
graphql-tools:
	@echo "📦 Installing gqlgen..."
	@go install github.com/99designs/gqlgen@latest

# =============================================================================
# Frontend (Web UI)
# =============================================================================

# Build frontend for production
web-build:
	@echo "🌐 Building Web UI..."
	@cd $(WEB_DIR) && pnpm install && pnpm run build
	@echo "✅ Web UI built in $(WEB_DIR)/dist/"

# Alias: 'web' builds the frontend
web: web-build

# Install frontend dependencies
web-install:
	@echo "📦 Installing Web UI dependencies..."
	@cd $(WEB_DIR) && pnpm install

# Run frontend dev server
web-dev:
	@echo "🔧 Starting Web UI development server..."
	@cd $(WEB_DIR) && pnpm run dev

# =============================================================================
# Docker
# =============================================================================

# Build and push multi-platform Docker image to registry
docker-build:
	@echo "🐳 Building multi-platform image: $(DOCKER_IMAGE)"
	@echo "   Platforms: $(DOCKER_PLATFORMS)"
	@echo ""
	@# Ensure buildx builder exists
	@docker buildx inspect modelgate-builder >/dev/null 2>&1 || \
		docker buildx create --name modelgate-builder --use
	@docker buildx use modelgate-builder
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE) \
		-f Dockerfile \
		--push \
		.
	@echo ""
	@echo "✅ Image built and pushed: $(DOCKER_IMAGE)"
	@echo "   Platforms: $(DOCKER_PLATFORMS)"

# Build Docker image for local use only (single platform, no push)
docker-build-local:
	@echo "🐳 Building local image: $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .
	@echo "✅ Image built: $(DOCKER_IMAGE)"

# Push existing image to registry (use after docker-build-local)
docker-push:
	@echo "🐳 Pushing $(DOCKER_IMAGE) to registry..."
	docker push $(DOCKER_IMAGE)
	@echo "✅ Image pushed: $(DOCKER_IMAGE)"

# Run container (standalone, requires external postgres)
# Serves API + Web UI on port 8080, mounts config file
docker-run: docker-build-local
	@echo "🐳 Running $(DOCKER_CONTAINER) container..."
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(DOCKER_CONTAINER)$$"; then \
		echo "⚠️  Container '$(DOCKER_CONTAINER)' already exists. Removing..."; \
		docker rm -f $(DOCKER_CONTAINER) > /dev/null; \
	fi
	@if [ ! -f $(CONFIG) ]; then \
		echo "⚠️  Config file '$(CONFIG)' not found. Using defaults."; \
		docker run -d --name $(DOCKER_CONTAINER) \
			-p 8080:8080 \
			-e MODELGATE_DB_HOST=host.docker.internal \
			-e MODELGATE_DB_PORT=5432 \
			-e MODELGATE_DB_USER=postgres \
			-e MODELGATE_DB_PASSWORD=postgres \
			-e MODELGATE_DB_NAME=modelgate \
			$(DOCKER_IMAGE); \
	else \
		echo "📄 Using config file: $(CONFIG)"; \
		docker run -d --name $(DOCKER_CONTAINER) \
			-p 8080:8080 \
			-v $(PWD)/$(CONFIG):/app/config/config.toml:ro \
			-e MODELGATE_DB_HOST=host.docker.internal \
			$(DOCKER_IMAGE) -config /app/config/config.toml; \
	fi
	@sleep 2
	@if docker ps --format '{{.Names}}' | grep -q "^$(DOCKER_CONTAINER)$$"; then \
		echo ""; \
		echo "╔══════════════════════════════════════════════════════════════════╗"; \
		echo "║                 ModelGate Docker Container                       ║"; \
		echo "╚══════════════════════════════════════════════════════════════════╝"; \
		echo ""; \
		echo "   Web UI:      http://localhost:8080"; \
		echo "   API:         http://localhost:8080/v1/"; \
		echo "   GraphQL:     http://localhost:8080/graphql"; \
		echo "   Playground:  http://localhost:8080/playground"; \
		echo "   Health:      http://localhost:8080/health"; \
		echo ""; \
		echo "   Run 'make docker-stop' to stop"; \
		echo "   Run 'make docker-logs' to view logs"; \
		echo ""; \
	else \
		echo "❌ Failed to start container. Check logs:"; \
		docker logs $(DOCKER_CONTAINER) 2>/dev/null || true; \
		exit 1; \
	fi

# Stop and remove container
docker-stop:
	@echo "🛑 Stopping container '$(DOCKER_CONTAINER)'..."
	@if docker ps -a --format '{{.Names}}' | grep -q "^$(DOCKER_CONTAINER)$$"; then \
		docker stop $(DOCKER_CONTAINER) > /dev/null 2>&1 || true; \
		docker rm $(DOCKER_CONTAINER) > /dev/null 2>&1 || true; \
		echo "✅ Container stopped and removed"; \
	else \
		echo "⚠️  Container '$(DOCKER_CONTAINER)' not found"; \
	fi

# View Docker container logs
docker-logs:
	@docker logs -f $(DOCKER_CONTAINER)

# Restart Docker container
docker-restart: docker-stop docker-run

# Alias: 'docker' builds images
docker: docker-build

# =============================================================================
# Docker Compose (requires docker-compose or docker compose plugin)
# =============================================================================

# Start all services with compose (PostgreSQL + Ollama + ModelGate)
compose-up:
	@echo "🐳 Starting Docker Compose stack (PostgreSQL + Ollama + ModelGate)..."
	@echo "   This will pull nomic-embed-text model on first run (~270MB)"
	@echo ""
	$(DOCKER_COMPOSE) up -d --build
	@echo ""
	@echo "⏳ Waiting for services to initialize..."
	@sleep 5
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                 ModelGate Docker Stack                           ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "   Web UI:         http://localhost:8080"
	@echo "   API:            http://localhost:8080/v1/"
	@echo "   GraphQL:        http://localhost:8080/graphql"
	@echo "   Playground:     http://localhost:8080/playground"
	@echo ""
	@echo "   Services:"
	@echo "   - PostgreSQL:   localhost:5432 (pgvector enabled)"
	@echo "   - Ollama:       localhost:11434 (nomic-embed-text)"
	@echo "   - ModelGate:    localhost:8080"
	@echo ""
	@echo "   Run 'make compose-logs' to view logs"
	@echo "   Run 'make compose-down' to stop"
	@echo ""

# Stop all services
compose-down:
	@echo "🛑 Stopping Docker Compose stack..."
	$(DOCKER_COMPOSE) down

# View logs
compose-logs:
	$(DOCKER_COMPOSE) logs -f

# Clean up volumes
compose-clean:
	@echo "🧹 Cleaning Docker Compose volumes..."
	$(DOCKER_COMPOSE) down -v

# Full rebuild with compose
compose-rebuild: compose-clean docker-build compose-up

# =============================================================================
# Testing & Quality
# =============================================================================

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

# Lint the code
lint:
	@echo "🔍 Running linter..."
	golangci-lint run ./...

# Format code (Go + imports)
fmt:
	@echo "✨ Formatting code..."
	@gofmt -w cmd/ internal/
	@goimports -w cmd/ internal/ 2>/dev/null || true

# Tidy dependencies
tidy:
	@echo "📦 Tidying dependencies..."
	go mod tidy

# =============================================================================
# Setup & Tools
# =============================================================================

# Full development setup
setup: tools web-install
	@echo "🔧 Setting up development environment..."
	@cp -n .env.example .env 2>/dev/null || true
	@go mod download
	@echo "✅ Setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Edit .env with your settings"
	@echo "  2. Run 'make dev' to start backend"
	@echo "  3. Run 'make web-dev' to start frontend"

# Install development tools
tools:
	@echo "📦 Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/99designs/gqlgen@latest
	@echo "✅ Tools installed"

# =============================================================================
# Cleanup
# =============================================================================

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -rf $(WEB_DIR)/dist
	rm -rf $(WEB_DIR)/node_modules
	rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Clean Go build cache
clean-cache:
	@echo "🧹 Cleaning Go cache..."
	go clean -cache

# =============================================================================
# Help
# =============================================================================

help:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║                 ModelGate - Build Commands                       ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Main Targets:"
	@echo "  make all              Build backend + frontend (production)"
	@echo "  make modelgate        Build Go binary only"
	@echo "  make graphql          Generate GraphQL code from schema"
	@echo "  make web              Build frontend for production"
	@echo "  make docker           Build Docker images"
	@echo ""
	@echo "Run Targets (Local - all on port 8080):"
	@echo "  make run              Build and run (API + Web UI on 8080)"
	@echo "  make run-foreground   Run in foreground (debugging)"
	@echo "  make stop             Stop the server"
	@echo "  make logs             View logs (tail -f)"
	@echo "  make dev              Run with go run (hot reload backend)"
	@echo "  make web-dev          Run frontend dev server (port 5173)"
	@echo "  make setup            Full development setup"
	@echo ""
	@echo "Docker (Registry: $(DOCKER_REGISTRY)):"
	@echo "  make docker-build       Build multi-platform & push to registry"
	@echo "  make docker-build-local Build for local use only (no push)"
	@echo "  make docker-push        Push existing image to registry"
	@echo "  make docker-run         Build local & run container"
	@echo "  make docker-stop        Stop and remove container"
	@echo "  make docker-logs        View container logs (tail -f)"
	@echo "  make docker-restart     Restart container"
	@echo ""
	@echo "Docker Compose (Full Stack: Postgres + Ollama + ModelGate):"
	@echo "  make compose-up       Start all services (includes Ollama)"
	@echo "  make compose-down     Stop all services"
	@echo "  make compose-logs     View logs"
	@echo "  make compose-clean    Remove volumes"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test             Run tests"
	@echo "  make test-coverage    Run tests with coverage report"
	@echo "  make lint             Run linter"
	@echo "  make fmt              Format code"
	@echo ""
	@echo "Maintenance:"
	@echo "  make tidy             Tidy Go dependencies"
	@echo "  make clean            Clean all build artifacts"
	@echo "  make tools            Install dev tools (gqlgen, lint, etc.)"
	@echo "  make help             Show this help"
	@echo ""
	@echo "Configuration:"
	@echo "  CONFIG=<file>         Override config file (default: config.toml)"
	@echo ""
	@echo "Examples:"
	@echo "  make run                              # Run locally (API + Web UI on 8080)"
	@echo "  make docker-run                       # Docker local (API + Web UI on 8080)"
	@echo "  make docker-build                     # Build & push to mazoriai/modelgate"
	@echo "  make docker-build IMAGE_TAG=v1.0.0   # Build & push specific version"
	@echo "  make compose-up                       # Full stack with Postgres"
	@echo "  make run CONFIG=prod.toml             # Run with custom config"
	@echo ""
