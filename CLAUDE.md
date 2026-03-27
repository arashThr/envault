# Envault — project context for Claude

## What this is

Envault is a personal `.env` file manager. The core idea: keep all secret files in
one secure local directory (`~/.envault/secrets/<project>/<file>`), symlink them
into project working directories so they are never committed to git, and optionally
sync them to a self-hosted server for backup and cross-machine access.

It replaces the original `newsecret.sh` bash script with a proper Go application.

## Repository layout

```
envault/
├── cmd/
│   ├── server/          ← HTTP server binary (API + embedded web UI)
│   │   ├── main.go
│   │   └── web/
│   │       └── index.html   ← single-page web app (vanilla JS, dark theme)
│   └── envault/         ← CLI binary
│       └── main.go
├── internal/
│   ├── api/
│   │   └── api.go       ← chi router, auth middleware, request logging
│   └── store/
│       └── store.go     ← file storage layer (path-traversal-safe)
├── go.mod / go.sum
├── newsecret.sh         ← original bash script (kept for reference)
├── CLAUDE.md            ← this file
└── README.md
```

## Architecture decisions

- **Single user** — no user model, no database. Auth is a single API key (Bearer
  token or `X-API-Key` header).
- **Files on disk** — server stores env files under `DATA_DIR/<project>/<filename>`.
  No database needed.
- **chi router** — used in `internal/api` for clean route definitions with URL
  params. No other external dependencies in the server.
- **slog** — structured logging via stdlib `log/slog` (text handler). Logger is
  created in `cmd/server/main.go` and passed down to store and API.
- **embed** — the web UI (`cmd/server/web/`) is compiled into the server binary via
  `//go:embed web`. Keep web assets under `cmd/server/web/`.
- **CLI local cache** — secrets are cached at `~/.envault/secrets/<project>/<file>`.
  Symlinks are created in `$(pwd)/<file>`. Config lives at `~/.envault/config.json`.

## API surface

All routes require auth. Base path: `/api`.

| Method   | Path                                  | Description             |
|----------|---------------------------------------|-------------------------|
| GET      | /api/projects                         | List all projects       |
| DELETE   | /api/projects/{project}               | Delete a project        |
| GET      | /api/projects/{project}/files         | List files in a project |
| GET      | /api/projects/{project}/files/{file}  | Download a file         |
| PUT      | /api/projects/{project}/files/{file}  | Upload / update a file  |
| DELETE   | /api/projects/{project}/files/{file}  | Delete a file           |

## CLI commands

```
envault new  <project> [file]   create local file + symlink in cwd
envault push <project> [file]   upload local file to server
envault pull <project> [file]   download from server + symlink in cwd
envault link <project> [file]   symlink cached file into cwd (no download)
envault list [project]          list projects or files on server
envault sync                    push all local secrets to server

envault config show
envault config set server <url>
envault config set key <apikey>
```

## Running locally

```bash
# server
go run ./cmd/server -key mysecretkey

# CLI
go run ./cmd/envault config set server http://localhost:8080
go run ./cmd/envault config set key mysecretkey
go run ./cmd/envault new cool-project
```

## Future work / known gaps

- No TLS — deploy behind a reverse proxy (nginx/caddy) with HTTPS in production.
- No file versioning or history.
- Web UI stores API key in `localStorage` — acceptable for single-user self-hosted.
- CLI `sync` only pushes local → server; no pull-all yet.
- Could add `envault edit <project> [file]` to open `$EDITOR` directly.
