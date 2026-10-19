@echo off
setlocal
cd /d "%~dp0"

echo Starting South Park Messenger...
docker compose up --build --detach
if errorlevel 1 (
    echo.
    echo The application could not be started. Make sure Docker Desktop is running.
    pause
    exit /b 1
)

echo Opening the South Park message console...
start "South Park Messages" powershell.exe -NoExit -Command "Set-Location -LiteralPath '%~dp0'; docker compose logs --follow --no-log-prefix consumer"

echo South Park Messenger is running.
exit /b 0
