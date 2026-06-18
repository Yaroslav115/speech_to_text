# Security

This project is intended primarily for local and trusted-network use.

## Webhook Sending

The `/v1/send-text` endpoint forwards transcript text to a user-provided URL.
That is convenient for local automation, but it should not be exposed directly
to the public internet without additional protections.

Recommended production hardening:

- Put the service behind authentication.
- Restrict `/v1/send-text` to an allowlist of trusted destination hosts.
- Avoid binding the service to a public interface unless you need remote access.
- Treat transcripts as sensitive data.

## API Keys

Never commit `.env.local`, `.env`, OpenAI API keys, downloaded model files, or
generated binaries. The included `.gitignore` excludes those by default.
