@echo off
setlocal

cd /d "%~dp0.."

if exist "work\test-receiver.exe" (
  "work\test-receiver.exe"
) else (
  go run ./cmd/testreceiver
)
