@echo off
setlocal

echo [INFO] Pruefe ob Ollama laeuft...
tasklist /FI "IMAGENAME eq ollama.exe" 2>NUL | find /I /N "ollama.exe">NUL
if "%ERRORLEVEL%"=="1" (
    echo [WARN] Ollama laeuft nicht. Starte Ollama im Hintergrund...
    start /B ollama serve
    timeout /t 5 >nul
)

echo [INFO] Starte CodeRefinery Server...
echo [INFO] Server laeuft auf http://localhost:8080
echo.

REM Startet den Server im aktuellen Verzeichnis (.)
.\bin\refinery.exe serve .

pause
