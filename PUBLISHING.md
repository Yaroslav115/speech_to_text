# Publishing Checklist

Use the existing project folder as the GitHub repository root.

## Commit These

- `cmd/`
- `scripts/`
- `outputs/.gitkeep`
- `.env.example`
- `.gitignore`
- `Dockerfile`
- `go.mod`
- `LICENSE`
- `README.md`
- `SECURITY.md`
- `PUBLISHING.md`

## Do Not Commit These

- `.env.local`
- `.env`
- `work/`
- `*.exe`
- Whisper model files such as `ggml-base.en.bin`
- API keys or private URLs

## Before Push

Run:

```powershell
.\scripts\install.ps1 -SkipWhisperSetup
```

Optional build checks:

```powershell
go build -o work\whisper-service.exe ./cmd/server
go build -o work\test-receiver.exe ./cmd/testreceiver
```

The generated `.exe` files are ignored and should remain local only.
