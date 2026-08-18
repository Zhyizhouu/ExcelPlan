@echo off
cd /d "%~dp0"
start "frontend" cmd /k "cd /d frontend && npm run dev"
start "backend" cmd /k "cd /d backend && go run ./cmd/api"