param(
    [string]$InstallRoot = "D:\WhisperLocal",
    [ValidateSet("tiny.en", "base.en", "small.en", "medium.en", "tiny", "base", "small", "medium", "large-v3-turbo", "large-v3")]
    [string]$Model = "base.en",
    [switch]$SkipBinary,
    [switch]$SkipModel
)

$ErrorActionPreference = "Stop"

function Download-File {
    param(
        [string]$Url,
        [string]$OutputPath
    )

    if (Test-Path $OutputPath) {
        Write-Host "Already exists: $OutputPath"
        return
    }

    Write-Host "Downloading $Url"
    Invoke-WebRequest -Uri $Url -OutFile $OutputPath -UseBasicParsing
}

function Require-Drive {
    param([string]$Path)

    $drive = Split-Path -Qualifier $Path
    if (-not $drive -or -not (Test-Path $drive)) {
        throw "Drive does not exist: $drive"
    }
}

Require-Drive $InstallRoot

$downloadDir = Join-Path $InstallRoot "downloads"
$binaryRoot = Join-Path $InstallRoot "whisper.cpp"
$modelDir = Join-Path $InstallRoot "models"

New-Item -ItemType Directory -Force -Path $downloadDir, $binaryRoot, $modelDir | Out-Null

if (-not $SkipBinary) {
    $binaryZip = Join-Path $downloadDir "whisper-bin-x64.zip"
    $binaryUrl = "https://github.com/ggml-org/whisper.cpp/releases/latest/download/whisper-bin-x64.zip"

    Download-File -Url $binaryUrl -OutputPath $binaryZip
    Expand-Archive -Path $binaryZip -DestinationPath $binaryRoot -Force
}

$whisperBinary = Get-ChildItem -Path $binaryRoot -Recurse -File |
    Where-Object { $_.Name -in @("whisper-cli.exe", "main.exe") } |
    Sort-Object @{ Expression = { if ($_.Name -eq "whisper-cli.exe") { 0 } else { 1 } } }, FullName |
    Select-Object -First 1

if (-not $whisperBinary) {
    throw "Could not find whisper-cli.exe or main.exe under $binaryRoot"
}

$modelPath = Join-Path $modelDir "ggml-$Model.bin"
if (-not $SkipModel) {
    $modelUrl = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-$Model.bin"
    Download-File -Url $modelUrl -OutputPath $modelPath
}

if (-not (Test-Path $modelPath)) {
    throw "Model file does not exist: $modelPath"
}

[Environment]::SetEnvironmentVariable("TRANSCRIPTION_BACKEND", "local", "User")
[Environment]::SetEnvironmentVariable("WHISPER_CPP_BINARY", $whisperBinary.FullName, "User")
[Environment]::SetEnvironmentVariable("WHISPER_MODEL_PATH", $modelPath, "User")

$envFile = Join-Path (Get-Location) ".env.local"
@"
TRANSCRIPTION_BACKEND=local
WHISPER_CPP_BINARY=$($whisperBinary.FullName)
WHISPER_MODEL_PATH=$modelPath
"@ | Set-Content -Path $envFile -Encoding ASCII

Write-Host ""
Write-Host "Local Whisper setup complete."
Write-Host "Install root:          $InstallRoot"
Write-Host "Whisper binary:       $($whisperBinary.FullName)"
Write-Host "Whisper model:        $modelPath"
Write-Host "Project env file:     $envFile"
Write-Host ""
Write-Host "Restart PowerShell or Codex to pick up the saved User environment variables."
