# ModelGate Justfile
# =============================================================================
# Alternative to Makefile using 'just' command runner
# Install: brew install just (macOS) or cargo install just
# =============================================================================

# Default recipe - show help
default: help

# Variables
binary := "modelgate"
build_dir := "bin"
web_dir := "web"

# Docker settings
docker_registry := env_var_or_default("DOCKER_REGISTRY", "")
image_tag := env_var_or_default("IMAGE_TAG", "latest")
docker_image := if docker_registry != "" { docker_registry + "/modelgate:" + image_tag } else { "modelgate:" + image_tag }

# Docker Compose (optional)
docker_compose := env_var_or_default("DOCKER_COMPOSE", "docker-compose")

# =============================================================================
# Main Targets
# =============================================================================

# Build everything (backend + frontend)
all: modelgate web-build
    @echo "✅ All components built successfully"

# Alias for build
build: modelgate

# =============================================================================
# Backend (Go)
# =============================================================================

# Build the ModelGate binary
modelgate:
    @echo "🔨 Building {{binary}}..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -ldflags "-s -w" -o {{build_dir}}/{{binary}} ./cmd/modelgate
    @echo "✅ Build complete: {{build_dir}}/{{binary}}"

# Run the application
run: modelgate
    @echo "🚀 Starting ModelGate..."
    ./{{build_dir}}/{{binary}}

# Run in development mode
dev:
    @echo "🔧 Running in development mode..."
    go run ./cmd/modelgate

# =============================================================================
# GraphQL Code Generation
# =============================================================================

# Generate GraphQL code from schema
graphql:
    @echo "📊 Generating GraphQL code..."
    cd internal/graphql && go run github.com/99designs/gqlgen generate
    @echo "✅ GraphQL code generated"

# Install gqlgen tool
graphql-tools:
    @echo "📦 Installing gqlgen..."
    go install github.com/99designs/gqlgen@latest

# =============================================================================
# Frontend (Web UI)
# =============================================================================

# Build frontend for production
web-build:
    @echo "🌐 Building Web UI..."
    cd {{web_dir}} && pnpm install && pnpm run build
    @echo "✅ Web UI built"

# Alias
web: web-build

# Install frontend dependencies
web-install:
    @echo "📦 Installing Web UI dependencies..."
    cd {{web_dir}} && pnpm install

# Run frontend dev server
web-dev:
    @echo "🔧 Starting Web UI dev server..."
    cd {{web_dir}} && pnpm run dev

# =============================================================================
# Docker
# =============================================================================

# Build Docker image
docker-build:
    @echo "🐳 Building {{docker_image}}..."
    docker build -t {{docker_image}} -f Dockerfile .
    @echo "✅ Image built: {{docker_image}}"

# Run container
docker-run:
    @echo "🚀 Running container..."
    docker run -d --name modelgate -p 8080:8080 -p 9090:9090 \
        -e MODELGATE_DB_HOST=host.docker.internal {{docker_image}}
    @echo "✅ Running at http://localhost:8080"

# Stop backend container
docker-stop:
    @echo "🛑 Stopping..."
    -docker stop modelgate
    -docker rm modelgate

# Alias
docker: docker-build

# =============================================================================
# Docker Compose (requires docker-compose)
# =============================================================================

# Start all services
compose-up:
    @echo "🚀 Starting compose stack..."
    {{docker_compose}} up -d
    @echo "✅ Services: API=:8080, Web=:3000"

# Start with Ollama
compose-up-ollama:
    @echo "🚀 Starting with Ollama..."
    {{docker_compose}} --profile with-ollama up -d

# Stop services
compose-down:
    @echo "🛑 Stopping..."
    {{docker_compose}} down

# View logs
compose-logs:
    {{docker_compose}} logs -f

# Clean volumes
compose-clean:
    @echo "🧹 Cleaning..."
    {{docker_compose}} down -v

# Full rebuild
compose-rebuild: compose-clean docker-build compose-up

# =============================================================================
# Testing & Quality
# =============================================================================

# Run tests
test:
    @echo "🧪 Running tests..."
    go test -v -race -cover ./...

# Run tests with coverage
test-coverage:
    @echo "🧪 Running tests with coverage..."
    go test -v -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "📊 Coverage: coverage.html"

# Lint code
lint:
    @echo "🔍 Linting..."
    golangci-lint run ./...

# Format code
fmt:
    @echo "✨ Formatting..."
    go fmt ./...
    goimports -w .

# Tidy deps
tidy:
    @echo "📦 Tidying deps..."
    go mod tidy

# =============================================================================
# Setup
# =============================================================================

# Full development setup
setup: tools web-install
    @echo "🔧 Setting up..."
    -cp .env.example .env
    go mod download
    @echo "✅ Setup complete!"

# Install tools
tools:
    @echo "📦 Installing tools..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    go install golang.org/x/tools/cmd/goimports@latest
    go install github.com/99designs/gqlgen@latest

# =============================================================================
# Cleanup
# =============================================================================

# Clean artifacts
clean:
    @echo "🧹 Cleaning..."
    rm -rf {{build_dir}}
    rm -rf {{web_dir}}/dist
    rm -f coverage.out coverage.html

# =============================================================================
# Help
# =============================================================================

# Show help
help:
    @echo ""
    @echo "╔══════════════════════════════════════════════════════════════════╗"
    @echo "║                 ModelGate - Build Commands                       ║"
    @echo "╚══════════════════════════════════════════════════════════════════╝"
    @echo ""
    @echo "Main:"
    @echo "  just all              Build backend + frontend"
    @echo "  just modelgate        Build Go binary"
    @echo "  just graphql          Generate GraphQL code"
    @echo "  just web              Build frontend"
    @echo "  just docker           Build Docker images"
    @echo ""
    @echo "Development:"
    @echo "  just dev              Run backend (dev mode)"
    @echo "  just web-dev          Run frontend (dev mode)"
    @echo "  just run              Build and run backend"
    @echo "  just setup            Full dev setup"
    @echo ""
    @echo "Docker:"
    @echo "  just docker-build     Build all images"
    @echo "  just docker-run       Run backend container"
    @echo "  just docker-stop      Stop backend container"
    @echo ""
    @echo "Docker Compose:"
    @echo "  just compose-up       Start all services"
    @echo "  just compose-down     Stop services"
    @echo "  just compose-logs     View logs"
    @echo ""
    @echo "Testing:"
    @echo "  just test             Run tests"
    @echo "  just test-coverage    Tests with coverage"
    @echo "  just lint             Run linter"
    @echo "  just fmt              Format code"
    @echo ""
    @echo "Other:"
    @echo "  just tidy             Tidy Go deps"
    @echo "  just clean            Clean artifacts"
    @echo "  just tools            Install dev tools"
    @echo ""

