#!/bin/bash
# ModelGate Startup Script
# Ensures Ollama is running with required models before starting ModelGate

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Starting ModelGate...${NC}"

# Check if Ollama is installed
if ! command -v ollama &> /dev/null; then
    echo -e "${RED}❌ Ollama is not installed${NC}"
    echo "Install with: brew install ollama (macOS) or curl -fsSL https://ollama.com/install.sh | sh (Linux)"
    exit 1
fi

# Check if Ollama is running
if ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo -e "${YELLOW}📦 Starting Ollama server...${NC}"
    ollama serve > /dev/null 2>&1 &
    sleep 3
    
    # Wait for Ollama to be ready
    for i in {1..10}; do
        if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Ollama server started${NC}"
            break
        fi
        sleep 1
    done
fi

# Check and pull required models
EMBEDDING_MODEL="nomic-embed-text"

echo -e "${YELLOW}📦 Checking embedding model: ${EMBEDDING_MODEL}${NC}"
if ! ollama list | grep -q "$EMBEDDING_MODEL"; then
    echo -e "${YELLOW}⬇️  Pulling ${EMBEDDING_MODEL}...${NC}"
    ollama pull "$EMBEDDING_MODEL"
fi
echo -e "${GREEN}✓ Embedding model ready${NC}"

# Start ModelGate
cd "$PROJECT_DIR"

# Build if needed
if [ ! -f "./modelgate" ] || [ "./cmd/modelgate/main.go" -nt "./modelgate" ]; then
    echo -e "${YELLOW}🔨 Building ModelGate...${NC}"
    go build -o modelgate ./cmd/modelgate
fi

echo -e "${GREEN}🌐 Starting ModelGate server...${NC}"
./modelgate -config config.toml

