# Envault

A self-hosted `.env` file manager. Store secrets securely in one place, symlink them into project directories, and sync them across machines via a lightweight Go server.

## How it works

- Secrets are kept in `~/.envault/secrets/<project>/<file>` — never inside a git repo
- A symlink is created in your working directory pointing to the cached file
- The server acts as a remote backup you can push to and pull from

## Running the server

```bash
go build -o envault-server ./cmd/server

API_KEY=your-secret-key ./envault-server
# options:
#   -port  PORT     (default 8080, or $PORT)
#   -data  DIR      (default ./data, or $DATA_DIR)
#   -key   KEY      (required, or $API_KEY)
```

Open `http://localhost:8080` to access the web UI.

## CLI setup

```bash
go build -o envault ./cmd/envault

envault config set server http://your-server:8080
envault config set key your-secret-key
```

## Typical workflow

```bash
# In a project directory — create a new .env file
cd ~/projects/cool-project
envault new cool-project        # creates ~/.envault/secrets/cool-project/.env
                                # and symlinks .env here

# Fill in secrets, then push to server
envault push cool-project

# On another machine — pull it down
cd ~/projects/cool-project
envault pull cool-project       # downloads + symlinks .env

# Push all local secrets at once
envault sync
```

## Deployment

The server is a single self-contained binary with no runtime dependencies. Recommended setup:

1. Run behind nginx/Caddy with TLS
2. Set `API_KEY` via environment variable
3. Mount a persistent volume at the data directory
