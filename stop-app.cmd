@echo off
setlocal
cd /d "%~dp0"

echo Stopping South Park Messenger...
docker compose down
if errorlevel 1 (
    echo.
    echo The application could not be stopped cleanly.
    pause
    exit /b 1
)

echo South Park Messenger has stopped.
timeout /t 2 /nobreak >nul
exit /b 0
