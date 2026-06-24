param(
    [string]$InstallRoot = "D:\WhisperLocal",
    [ValidateSet("tiny.en", "base.en", "small.en", "medium.en", "tiny", "base", "small", "medium", "large-v3-turbo", "large-v3")]
    [string]$Model = "base.en",
    [switch]$SkipWhisperSetup,
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

function Find-Go {
    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if ($goCommand) {
        return $goCommand.Source
    }

    $defaultGo = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $defaultGo) {
        return $defaultGo
    }

    throw "Go was not found. Install Go 1.22 or newer from https://go.dev/dl/ and run this script again."
}

$projectRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $projectRoot

Write-Host "Installing Whisper Transcription Service"
Write-Host "Project root: $projectRoot"
Write-Host ""

$go = Find-Go
Write-Host "Using Go: $go"
& $go version

if (-not $SkipWhisperSetup) {
    Write-Host ""
    Write-Host "Setting up local Whisper model and binary..."
    & (Join-Path $PSScriptRoot "setup-local-whisper.ps1") -InstallRoot $InstallRoot -Model $Model
}

if (-not $SkipBuild) {
    Write-Host ""
    Write-Host "Building executables..."
    New-Item -ItemType Directory -Force -Path "work" | Out-Null
    $env:GOCACHE = Join-Path $projectRoot "work\go-build"

    & $go test ./...
    & $go build -buildvcs=false -o "work\whisper-service.exe" ./cmd/server
    & $go build -buildvcs=false -o "work\test-receiver.exe" ./cmd/testreceiver
}

Write-Host ""
Write-Host "Install complete."
Write-Host ""
Write-Host "Start the service:"
Write-Host "  .\scripts\run-local-service.cmd"
Write-Host ""
Write-Host "Open:"
Write-Host "  http://localhost:8080/"
Write-Host ""
Write-Host "Optional test receiver:"
Write-Host "  .\scripts\run-test-receiver.cmd"
