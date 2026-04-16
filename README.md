# Envault

A self-hosted `.env` file manager. Store encrypted secrets in one place, symlink them into project directories, and sync them across machines via a lightweight Go server.

All content is **end-to-end encrypted** using [age](https://age-encryption.org/). Only encrypted blobs reach the server — the password and plaintext never leave your machine.

---

## Install

### CLI

```bash
go install github.com/arashthr/envault/cmd/envault@latest
```

### Server

```bash
go install github.com/arashthr/envault/cmd/envault-server@latest
```

### Build from source

```bash
git clone https://github.com/arashthr/envault
cd envault
make build      # → bin/envault and bin/envault-server
make test       # run tests with race detector
```

---

## Quick start

```bash
# 1. Start the server
envault-server -data ~/.envault/data -key your-password

# 2. Configure the CLI (run once per machine)
envault config set server http://localhost:8080
envault config set key your-password   # same password; also the encryption passphrase

# 3. Add a secret in a project directory
cd ~/projects/myapp
envault new                 # prompts for project name (default: myapp) and environment
# edit .env, then upload
envault push                # encrypts locally, sends ciphertext to server
```

---

## Server

### Running

```bash
envault-server [options]
```

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `-key` | `API_KEY` | — | Password for HTTP Basic Auth. Omit to disable auth. |
| `-data` | `DATA_DIR` | `./data` | Directory where encrypted files are stored |
| `-port` | `PORT` | `8080` | Listen port |
| `-debug` | `DEBUG=true` | — | Verbose logging |

When `-key` is set the server validates HTTP Basic Auth on all API requests and logs a startup confirmation. When omitted the server runs with no access control — suitable for a fully trusted private network where security relies solely on age encryption.

Open `http://localhost:8080` to access the web UI. Enter the password to connect, then enter it again when opening a file to decrypt its content (same password used for both).

### Production deployment

Recommended: run as a systemd service behind nginx or Caddy (which handles TLS).

```bash
# Minimal start
API_KEY=your-strong-password envault-server -data /var/lib/envault -port 8080
```

<details>
<summary>systemd unit example</summary>

```ini
[Unit]
Description=Envault server

[Service]
ExecStart=/usr/local/bin/envault-server -data /var/lib/envault -port 8080
Environment=API_KEY=your-strong-password
Restart=always
User=envault

[Install]
WantedBy=multi-user.target
```
</details>

---

## CLI

### Configuration

```bash
envault config set server http://your-server:8080
envault config set key    your-password
envault config show
```

Config is stored at `~/.envault/config.json`. Secrets are cached at `~/.envault/secrets/`.

### Commands

```
envault new    [project] [env]   Create a local secret + symlink it into cwd
envault push   [project] [env]   Encrypt + upload to server
envault pull   [project] [env]   Download + decrypt + symlink into cwd
envault link   [project] [env]   Symlink a cached secret into cwd (no download)
envault remove [project] [env]   Delete local copy (or entire project directory)
envault list   [project]         List projects or environments (local + server, with sync status)
envault sync                     Encrypt + push all local secrets to server
```

`project` defaults to the current directory name when omitted.
`env` defaults to `local`, which symlinks as `.env`. Any other name symlinks as `.env.<name>` (e.g. `production` → `.env.production`).

### Typical workflow

```bash
# New project — create a secret
cd ~/projects/myapp
envault new                         # prompts: project=myapp, env=local → creates .env
# fill in .env, then push
envault push

# On another machine — pull the secret
cd ~/projects/myapp
envault pull myapp                  # downloads + symlinks as .env

# Multiple environments
envault new myapp production        # creates .env.production
envault push myapp production
envault list myapp                  # shows local vs server status for each env

# Clean up a local copy
envault remove myapp production     # deletes local cached file
envault remove myapp                # deletes entire local project (with confirmation)

# Push everything at once
envault sync
```

---

## Web UI

Navigate to `http://your-server:8080`. Enter your password to connect — this authenticates with the server and loads the project list. When you open a file, the same password is used to decrypt its content in the browser.

---

## How it works

```
push:  plaintext → age encrypt (passphrase) → ciphertext → PUT /api/…
pull:  GET /api/… → ciphertext → age decrypt (passphrase) → local file
web:   browser → age decrypt (passphrase, in-browser) → editor
```

- Local secrets live at `~/.envault/secrets/<project>/<env>`
- Symlinks in the working directory point to the cached file — they're never committed to git
- The server stores opaque encrypted blobs and cannot read content even with full server access
- Auth uses HTTP Basic Auth; the password doubles as the age encryption passphrase
