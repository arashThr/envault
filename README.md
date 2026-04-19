# Envault

A self-hosted `.env` file manager. Store encrypted secrets in one place, symlink them into project directories, and sync them across machines via a lightweight Go server.

All content is **end-to-end encrypted** using [age](https://age-encryption.org/). Only encrypted blobs reach the server — the passphrase and plaintext never leave your machine.

---

## Install

### CLI

```bash
go install github.com/arashthr/envault/cmd/envault@latest
```

### Server

The server embeds a compiled web UI and cannot be installed via `go install`. Clone and build from source:

```bash
git clone https://github.com/arashthr/envault
cd envault
make build                                      # → bin/envault-server
sudo cp bin/envault-server /usr/local/bin/
```

---

## Quick start

```bash
# 1. Start the server
envault-server -data ~/.envault/data

# 2. Configure the CLI (run once per machine)
envault config set server http://localhost:8080
envault config set key your-passphrase   # encryption passphrase (never sent to server)

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
| `-data` | `DATA_DIR` | `./data` | Directory where encrypted files are stored |
| `-port` | `PORT` | `8080` | Listen port |
| `-debug` | `DEBUG=true` | — | Verbose logging |

The server has **no built-in authentication**. Access control is delegated to a reverse proxy (see [Production deployment](#production-deployment) below). This is intentional: the passphrase used for age encryption is never sent to the server, so even full server access cannot reveal plaintext.

Open `http://localhost:8080` to access the web UI. Projects and file names are visible to anyone who can reach the server; file *contents* require the encryption passphrase to decrypt.

### Production deployment

Run the server behind [Caddy](https://caddyserver.com/) (or nginx) with TLS and HTTP Basic Auth. A `Caddyfile.example` is included in the repo:

```caddy
vault.example.com {
    basicauth * {
        # Generate a hashed password with: caddy hash-password
        envault $2a$14$...replace-with-caddy-hash-password-output...
    }

    reverse_proxy localhost:8080
}
```

```bash
# Generate a hashed password for the Caddyfile:
caddy hash-password

# Start envault-server (no -key needed — Caddy handles access control)
envault-server -data /var/lib/envault -port 8080
```

<details>
<summary>systemd unit example</summary>

```ini
[Unit]
Description=Envault server

[Service]
ExecStart=/usr/local/bin/envault-server -data /var/lib/envault -port 8080
Restart=always
User=envault

[Install]
WantedBy=multi-user.target
```
</details>

### CLI with a Caddy-protected server

Embed the credentials directly in the server URL — Go's HTTP client handles Basic Auth automatically:

```bash
envault config set server https://envault:your-password@vault.example.com
envault config set key your-encryption-passphrase   # separate from the Caddy password
```

---

## CLI

### Configuration

```bash
envault config set server http://your-server:8080
envault config set key    your-encryption-passphrase
envault config show
```

Config is stored at `~/.envault/config.json`. Secrets are cached at `~/.envault/secrets/`.

The `key` is the **age encryption passphrase** — it is used locally to encrypt before upload and to decrypt after download. It is **never sent to the server**.

### Commands

```
envault new    [project] [env]   Create a local secret + symlink it into cwd
envault push   [project] [env]   Encrypt + upload to server
envault pull   [project] [env]   Download + decrypt + symlink into cwd
envault link   [project] [env]   Symlink a cached secret into cwd (no download)
envault remove [project] [env]   Delete local copy (or entire project directory)
envault list   [project]         List projects or environments (status: local-only | server-only | both)
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
```

---

## Web UI

Navigate to `http://your-server:8080` (or your Caddy URL). Projects and file names load immediately. When you open a file, you are prompted for your **encryption passphrase** to decrypt its content in the browser. The passphrase never leaves your machine.

---

## How it works

```
push:  plaintext → age encrypt (passphrase) → ciphertext → PUT /api/…
pull:  GET /api/… → ciphertext → age decrypt (passphrase) → local file
web:   browser → age decrypt (passphrase, in-browser WASM) → editor
```

**Security layers:**

1. **Caddy Basic Auth** — controls who can reach the server (network-level access control)
2. **age encryption** — controls who can read file content (end-to-end, passphrase never leaves the client)

The server stores opaque encrypted blobs. Even with full server access — or a compromised Caddy password — an attacker cannot read the plaintext without the age passphrase.

- Local secrets live at `~/.envault/secrets/<project>/<file>`
- Symlinks in the working directory point to the cached file — they're never committed to git
