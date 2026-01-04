#!/bin/bash

# Prüfen ob Ollama läuft
if ! pgrep -x "ollama" > /dev/null; then
    echo "[WARN] Ollama läuft nicht. Starte Service..."
    ollama serve &
    sleep 5
fi

echo "[INFO] Starte CodeRefinery Server..."
echo "[INFO] Server läuft auf http://localhost:8080"
echo ""

# Startet Server im aktuellen Verzeichnis
./bin/refinery serve .