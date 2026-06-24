# Whisper Transcription Service

A Go web service for speech-to-text transcription with a built-in browser microphone UI.

It supports two transcription backends:

- **Local Whisper** through `whisper.cpp`
- **OpenAI API** through `POST /v1/audio/transcriptions`

The web UI can record microphone audio, transcribe automatically when recording stops, append new text to the transcript, use push-to-talk, and optionally send transcript text to another HTTP service.

## Features

- Browser microphone recording
- Automatic transcription on stop
- Push-to-talk keyboard mode
- Local `whisper.cpp` backend
- Optional OpenAI API backend
- Transcript append mode
- Clear transcript button
- Manual or automatic text forwarding to a URL
- Test receiver server for checking forwarded payloads

## Requirements

- Go 1.22 or newer
- Windows PowerShell for the included setup scripts
- For local mode: `whisper.cpp` binary and a `ggml-*.bin` Whisper model
- For API mode: an OpenAI API key

## Quick Start: Local Whisper On Windows

Clone the repo, then run the installer. This downloads `whisper.cpp`, downloads the selected model, creates `.env.local`, runs tests, and builds the executables.

```powershell
.\scripts\install.ps1 -InstallRoot "D:\WhisperLocal" -Model "base.en"
```

If PowerShell blocks scripts on your machine, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\install.ps1 -InstallRoot "D:\WhisperLocal" -Model "base.en"
```

The installer creates `.env.local` and saves these User environment variables:

```text
TRANSCRIPTION_BACKEND=local
WHISPER_CPP_BINARY=...
WHISPER_MODEL_PATH=...
```

Start the service:

```powershell
.\scripts\run-local-service.cmd
```

Open:

```text
http://localhost:8080/
```

You can also open:

```text
http://localhost:8080/v1/transcribe
```

To only build after Whisper is already installed:

```powershell
.\scripts\install.ps1 -SkipWhisperSetup
```

To only set up Whisper without building:

```powershell
.\scripts\install.ps1 -SkipBuild
```

## Browser UI

The browser UI includes:

- **Start** and **Stop** recording buttons
- **Retry Transcription** for the last recording
- **Clear Text** to clear the transcript box
- Cog button for the settings view
- **Enable push to talk** checkbox
- **Push to talk key** selector
- **Send text URL** for forwarding transcript text
- **Enable text sending** checkbox
- **Send automatically** checkbox
- **Send Text** button

Push-to-talk workflow:

1. Click the cog button to open settings.
2. Enable **Enable push to talk**.
3. Click the **Push to talk key** field.
4. Press the key you want to use.
5. Hold that key to record.
6. Release the key to stop and transcribe automatically.

## Text Forwarding

The UI can send transcript text to another HTTP service.

Set **Send text URL**, enable **Enable text sending**, then use **Send Text** manually or enable **Send automatically**.

The service sends a `POST` request with JSON:

```json
{
  "text": "latest transcribed text",
  "full_text": "all text currently shown in the transcript box",
  "source": "whisper-transcription-service",
  "sent_at": "2026-06-17T12:00:00Z"
}
```

For local testing, run the test receiver:

```powershell
.\scripts\run-test-receiver.cmd
```

Then set **Send text URL** to:

```text
http://127.0.0.1:8081/receive
```

Received payloads are written to:

```text
work/test-receiver.log
```

## OpenAI API Mode

Set the backend and API key:

```powershell
$env:TRANSCRIPTION_BACKEND="api"
$env:OPENAI_API_KEY="your_api_key_here"
go run ./cmd/server
```

Or set these values in `.env.local`.

## API Usage

Transcribe audio:

```powershell
curl.exe -X POST http://localhost:8080/v1/transcribe `
  -F "backend=local" `
  -F "file=@C:\path\to\audio.wav" `
  -F "language=en"
```

Use OpenAI API mode per request:

```powershell
curl.exe -X POST http://localhost:8080/v1/transcribe `
  -F "backend=api" `
  -F "file=@C:\path\to\audio.mp3" `
  -F "language=en" `
  -F "response_format=json"
```

Send text to a receiver:

```powershell
curl.exe -X POST http://localhost:8080/v1/send-text `
  -H "Content-Type: application/json" `
  -d "{\"url\":\"http://127.0.0.1:8081/receive\",\"text\":\"hello\",\"full_text\":\"hello\"}"
```

Health check:

```powershell
curl.exe http://localhost:8080/healthz
```

## Configuration

Copy `.env.example` to `.env.local` and edit it for your machine:

```powershell
Copy-Item .env.example .env.local
```

The installer creates `.env.local` automatically for local Whisper mode.

Environment variables:

- `TRANSCRIPTION_BACKEND`: `local` or `api`; default is `api`
- `OPENAI_API_KEY`: required only for API requests
- `OPENAI_BASE_URL`: default `https://api.openai.com/v1`
- `TRANSCRIPTION_MODEL`: default `whisper-1`
- `WHISPER_CPP_BINARY`: path to `whisper-cli.exe`
- `WHISPER_MODEL_PATH`: path to a `ggml-*.bin` model file
- `PORT`: default `8080`
- `MAX_UPLOAD_BYTES`: default `26214400`
- `REQUEST_TIMEOUT_SECONDS`: default `300`

## Supported Audio

Local mode depends on your `whisper.cpp` binary. The current Windows binary reports support for:

- `flac`
- `mp3`
- `ogg`
- `wav`

The browser recorder sends `recording.wav`.

OpenAI API mode supports the formats accepted by OpenAI's transcription endpoint.

## Docker

The included Dockerfile is best for API mode:

```powershell
docker build -t whisper-transcription-service .
docker run --rm -p 8080:8080 `
  -e TRANSCRIPTION_BACKEND=api `
  -e OPENAI_API_KEY="your_api_key_here" `
  whisper-transcription-service
```

Local Whisper in Docker requires adding a `whisper.cpp` binary and model file to the image or mounting them into the container.

## Repository Notes

Do not commit:

- `.env.local`
- API keys
- downloaded model files
- generated `.exe` files
- `work/` build cache or logs

See [SECURITY.md](SECURITY.md) before exposing the service outside localhost or a trusted network.
