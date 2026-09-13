@echo off
title Lakhan Bhandar POS - Desktop Application
echo ========================================================
echo   Launching Lakhan Bhandar POS Desktop Application...
echo   NilLang Engine + Alap Declarative UI Architecture
echo ========================================================

start "" /B pos-server.exe

timeout /t 1 /nobreak >nul

if exist "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" (
    start "" "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" --app=http://localhost:8090 --window-size=1366,768
) else if exist "C:\Program Files\Google\Chrome\Application\chrome.exe" (
    start "" "C:\Program Files\Google\Chrome\Application\chrome.exe" --app=http://localhost:8090 --window-size=1366,768
) else (
    start http://localhost:8090
)

echo App window launched.
