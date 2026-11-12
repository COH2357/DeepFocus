@echo off
setlocal

go build -ldflags="-s -w -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus.exe ./cmd/deepfocus
if errorlevel 1 exit /b %errorlevel%

go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus_gui.exe ./cmd/deepfocus_gui
if errorlevel 1 exit /b %errorlevel%

go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o deepfocus_daemon.exe ./cmd/deepfocus_daemon
if errorlevel 1 exit /b %errorlevel%

makensis /DVERSION=dev-build installer.nsi
if errorlevel 1 exit /b %errorlevel%

echo Build complete.
exit /b 0
