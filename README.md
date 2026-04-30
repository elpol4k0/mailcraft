<h1 align="center">
  MailCraft - email testing for developers
</h1>

<div align="center">
    <a href="https://github.com/elpol4k0/mailcraft/actions/workflows/ci.yml"><img src="https://github.com/elpol4k0/mailcraft/actions/workflows/ci.yml/badge.svg" alt="CI Tests status"></a>
    <a href="https://github.com/elpol4k0/mailcraft/actions/workflows/release.yml"><img src="https://github.com/elpol4k0/mailcraft/actions/workflows/release.yml/badge.svg" alt="Release status"></a>
    <a href="https://goreportcard.com/report/github.com/elpol4k0/mailcraft"><img src="https://goreportcard.com/badge/github.com/elpol4k0/mailcraft" alt="Go Report Card"></a>
    <br>
    <a href="https://github.com/elpol4k0/mailcraft/releases/latest"><img src="https://img.shields.io/github/v/release/elpol4k0/mailcraft.svg" alt="Latest release"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-purple.svg" alt="License: MIT"></a>
</div>

<br>

**MailCraft** is a small, fast, low memory, zero-dependency, multi-platform email testing tool for developers.

![MailCraft](https://raw.githubusercontent.com/elpol4k0/mailcraft/HEAD/screenshot.png)

It acts as an SMTP server and provides a modern web UI to view and inspect captured emails.


## Features

- Runs entirely from a single static binary or multi-architecture [Docker images](https://github.com/elpol4k0/mailcraft/pkgs/container/mailcraft)
- Modern dark web UI with real-time updates via SSE and optional browser notifications
- Full-text search across sender, recipient, subject and body
- Tag system — manual or automated tagging via rules
- Rule engine — automatically tag, color, star or delete emails based on conditions
- HTML check to test mail client compatibility with HTML emails
- Link check to test message links and linked images
- Spam check via a running SpamAssassin server
- Export emails as `.eml` files (single or bulk)
- Keyboard navigation throughout the UI
- In-memory storage (zero persistence, zero config)
- Single static binary, frontend assets embedded, zero runtime dependencies


## Installation

The MailCraft web UI listens by default on `http://0.0.0.0:8025` and the SMTP port on `0.0.0.0:1025`.


### Download static binary (Windows, Linux and Mac)

Static binaries can be found on the [releases page](https://github.com/elpol4k0/mailcraft/releases/latest). Extract and run:

```shell
./mailcraft
```


### Docker

```shell
docker run -p 1025:1025 -p 8025:8025 ghcr.io/elpol4k0/mailcraft:latest
```


### Build from source

Requires Go 1.24+ and Node.js 20+.

```shell
git clone https://github.com/elpol4k0/mailcraft.git
cd mailcraft
make build-all
./mailcraft
```


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


## Usage

Run `mailcraft -h` to see all options.

Point your application's SMTP settings at `localhost:1025` — no authentication required.
