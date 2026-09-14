@echo off
rem Rebuilds the ExcelPlan desktop app: builds the frontend, copies it into
rem the Wails project (go:embed can't reach outside its own directory tree,
rem so this copy step can't be skipped), then compiles the exe.
rem Run this after any frontend or backend change you want in the desktop app
rem -- the Start Menu shortcut always points at the same exe path, so nothing
rem else needs to be redone.
cd /d "%~dp0"

call npm --prefix frontend run build
if errorlevel 1 exit /b 1

rmdir /s /q "backend\cmd\desktop\frontend_dist" 2>nul
xcopy /e /i /y "frontend\dist" "backend\cmd\desktop\frontend_dist" >nul
if errorlevel 1 exit /b 1

pushd backend\cmd\desktop
wails build
set BUILD_RESULT=%errorlevel%
popd
exit /b %BUILD_RESULT%
