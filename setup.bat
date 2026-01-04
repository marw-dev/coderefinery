@echo off
setlocal

echo ==========================================
echo   CodeRefinery Setup (Windows)
echo ==========================================

REM 1. Check Prerequisites
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go ist nicht installiert! Bitte installieren: https://go.dev/dl/
    pause
    exit /b 1
)

ollama --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Ollama ist nicht installiert! Bitte installieren: https://ollama.com/
    pause
    exit /b 1
)

REM 2. Initialize Go Module (falls nicht vorhanden)
if not exist "go.mod" (
    echo [INFO] Initialisiere Go Modul...
    go mod init coderefinery
)

REM 3. Install Dependencies
echo [INFO] Installiere Go Dependencies...
go get github.com/gin-gonic/gin
go get github.com/fsnotify/fsnotify
go mod tidy

REM 4. Build Project
echo [INFO] Kompiliere Anwendung...
if not exist "bin" mkdir bin
go build -ldflags="-s -w" -o bin/refinery.exe ./cmd/refinery/main.go
if %errorlevel% neq 0 (
    echo [ERROR] Build fehlgeschlagen!
    pause
    exit /b 1
)

REM 5. Pull Ollama Models
echo [INFO] Lade KI-Modelle...
echo    - Embedding Modell (nomic-embed-text)...
ollama pull nomic-embed-text

echo    - Logic Modell (DeepSeek R1 14B)...
REM Wir laden das 14B Modell
ollama pull deepseek-r1:14b

echo.
echo ==========================================
echo   SETUP ERFOLGREICH!
echo ==========================================
echo Starte nun 'start.bat'
pause