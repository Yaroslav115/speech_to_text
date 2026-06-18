@echo off
setlocal

cd /d "%~dp0.."

if exist ".env.local" (
  for /f "usebackq eol=# tokens=1,* delims==" %%A in (".env.local") do (
    if not "%%A"=="" set "%%A=%%B"
  )
)

if "%TRANSCRIPTION_BACKEND%"=="" set "TRANSCRIPTION_BACKEND=local"
if "%PORT%"=="" set "PORT=8080"

if exist "work\whisper-service.exe" (
  "work\whisper-service.exe"
) else (
  go run ./cmd/server
)
