# MailCraft

[![CI](https://github.com/yourname/mailcraft/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/yourname/mailcraft/actions/workflows/ci.yml)
[![Release](https://github.com/yourname/mailcraft/actions/workflows/release.yml/badge.svg)](https://github.com/yourname/mailcraft/actions/workflows/release.yml)
[![Latest release](https://img.shields.io/github/v/release/yourname/mailcraft)](https://github.com/yourname/mailcraft/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-purple.svg)](https://opensource.org/licenses/MIT)

SMTP mailcatcher for local development. Catches all outgoing emails from your app and shows them in a modern web UI.

## Features

- SMTP server (default `:1025`) — no authentication required
- Modern dark web UI with real-time updates via SSE
- Full-text search, tags, folders, starring
- Rule engine — auto-tag, color, star, or delete incoming emails
- HTML, link, and spam checks
- Single static binary, assets embedded, zero dependencies

## Quick Start

```bash
# Download and run (see Releases for your platform)
./mailcraft

# Docker (linux/amd64, linux/arm64)
docker run -p 1025:1025 -p 8025:8025 ghcr.io/yourname/mailcraft:latest
```

Open [http://localhost:8025](http://localhost:8025) — SMTP listens on `:1025`.

## Configuration

All flags can be set via environment variables with the `MC_` prefix.

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--smtp-addr` | `MC_SMTP_ADDR` | `:1025` | SMTP bind address |
| `--http-addr` | `MC_HTTP_ADDR` | `:8025` | HTTP bind address |
| `--max-emails` | `MC_MAX_EMAILS` | `5000` | Max emails to keep (oldest evicted) |
| `--max-size` | `MC_MAX_SIZE` | `26214400` | Max message size in bytes |
| `--base-path` | `MC_BASE_PATH` | `/` | Base path behind a reverse proxy |
| `--log-level` | `MC_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Building from source

Requires Go 1.22+ and Node.js 20+.

```bash
git clone https://github.com/yourname/mailcraft.git
cd mailcraft
make build-all
./mailcraft
```

## License

MIT
