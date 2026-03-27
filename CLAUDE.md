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
│   │   ├── api.go           ← chi router, auth middleware, request logging
│   │   └── api_test.go      ← integration tests (httptest)
│   └── store/
│       ├── store.go         ← file storage layer (path-traversal-safe)
│       └── store_test.go    ← unit tests
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

## Logging requirements (critical)

Logging is a first-class requirement. Every failure path must produce a log entry
with enough context to diagnose the problem without needing a debugger.

**Log levels:**

| Level | When to use |
|-------|-------------|
| DEBUG | Read operations (ListProjects, ListFiles, GetFile), config details, entry/exit of key functions |
| INFO  | Write/delete operations (PutFile, DeleteFile, DeleteProject), server startup, store ready |
| WARN  | Auth failures (log remote addr + path), 404s for resources that should exist |
| ERROR | Unexpected filesystem errors, server startup failures, 5xx conditions |

**Rules:**
- Auth failures → `WARN` with `remote_addr`, `method`, `path`, `has_key` fields
- Every HTTP request → one log line after completion with `method`, `path`, `status`,
  `bytes`, `duration_ms`, `remote_addr`, request `id`; level follows status code
  (INFO for 2xx, WARN for 4xx, ERROR for 5xx)
- Store reads → DEBUG with project/file context
- Store writes/deletes → INFO with project/file/bytes fields
- Store errors → ERROR with full path context and `err` field
- Server startup → INFO for each init step; ERROR + exit(1) on failure

**Debug mode:** pass `-debug` flag or `DEBUG=true` env var to enable DEBUG-level output.

## Testing requirements (critical)

Tests are the primary feedback loop for changes. All new features and bug fixes must
be accompanied by tests. Run `go test ./...` before every commit.

**Test layout:**
- `internal/store/store_test.go` — unit tests for the storage layer
- `internal/api/api_test.go` — integration tests using `net/http/httptest`

**Test conventions:**
- Use `t.TempDir()` for store directories — automatically cleaned up
- Silence logs in tests: `slog.New(slog.NewTextHandler(io.Discard, nil))`
- `setup(t)` helper returns `(http.Handler, *store.Store)` for API tests
- `mustStatus(t, res, wantCode)` helper for concise status assertions
- Table-driven tests for validation edge cases (invalid names, path traversal)
- Test both happy path and error paths for every handler
- Auth tests must cover: missing key, wrong key, Bearer token, error body format

**Running tests:**
```bash
go test ./...                        # all packages
go test ./internal/api/... -v        # verbose API tests
go test ./internal/store/... -run TestPutFile  # single test
```

## Running locally

```bash
# build both binaries into bin/
make build

# or run directly
go run ./cmd/server -key mysecretkey

# with debug logging
go run ./cmd/server -key mysecretkey -debug

# CLI
go run ./cmd/envault config set server http://localhost:8080
go run ./cmd/envault config set key mysecretkey
go run ./cmd/envault new cool-project
```

## Makefile targets

| Target       | Description                              |
|--------------|------------------------------------------|
| `make`       | Build everything (default)               |
| `make build` | Build server + CLI into `bin/`           |
| `make server`| Build `bin/envault-server` only          |
| `make cli`   | Build `bin/envault` only                 |
| `make test`  | Run `go test ./... -v -race -count=1`    |
| `make clean` | Remove `bin/`                            |

## CI

GitHub Actions workflow at `.github/workflows/ci.yml` runs on every push to `main`
and on every pull request targeting `main`. Steps: checkout → setup Go (version from
`go.mod`) → `go build ./...` → `go test ./... -v -race -count=1`.

## Future work / known gaps

- No TLS — deploy behind a reverse proxy (nginx/caddy) with HTTPS in production.
- No file versioning or history.
- Web UI stores API key in `localStorage` — acceptable for single-user self-hosted.
- CLI `sync` only pushes local → server; no pull-all yet.
- Could add `envault edit <project> [file]` to open `$EDITOR` directly.
