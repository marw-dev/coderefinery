#!/bin/bash
set -e

echo "=========================================="
echo "  CodeRefinery Setup (Linux/Mac)"
echo "=========================================="

# 1. Check Prerequisites
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go ist nicht installiert!"
    exit 1
fi

if ! command -v ollama &> /dev/null; then
    echo "[ERROR] Ollama ist nicht installiert!"
    exit 1
fi

# 2. Initialize Go Module
if [ ! -f "go.mod" ]; then
    echo "[INFO] Initialisiere Go Modul..."
    go mod init coderefinery
fi

# 3. Install Dependencies
echo "[INFO] Installiere Go Dependencies..."
go get github.com/gin-gonic/gin
go get github.com/fsnotify/fsnotify
go mod tidy

# 4. Build Project
echo "[INFO] Kompiliere Anwendung..."
mkdir -p bin
go build -ldflags="-s -w" -o bin/refinery ./cmd/refinery/main.go

# 5. Pull Ollama Models
echo "[INFO] Lade KI-Modelle..."

# Das kleine, schnelle Embedding Modell
echo "   - Pulling nomic-embed-text..."
ollama pull nomic-embed-text

# Das 14B Reasoning Modell für deine 16GB Karte
echo "   - Pulling deepseek-r1:14b..."
ollama pull deepseek-r1:14b

echo ""
echo "=========================================="
echo "  SETUP ERFOLGREICH!"
echo "=========================================="
echo "Starte nun './start.sh'"