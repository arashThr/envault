package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultFile = ".env"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "config":
		runConfig(args)
	case "new":
		runNew(args)
	case "push":
		runPush(args)
	case "pull":
		runPull(args)
	case "link":
		runLink(args)
	case "list", "ls":
		runList(args)
	case "sync":
		runSync(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fatalf("unknown command %q — run `envault help` for usage\n", cmd)
	}
}

// ── commands ─────────────────────────────────────────────────────────────────

// new <project> [file]
// Creates an empty env file in the local secrets cache and symlinks it into cwd.
func runNew(args []string) {
	project, file := parseProjectFile(args)
	localPath := localSecretPath(project, file)

	if _, err := os.Stat(localPath); err == nil {
		fatalf("secret already exists locally: %s\n", localPath)
	}

	linkPath := filepath.Join(mustCwd(), file)
	if _, err := os.Lstat(linkPath); err == nil {
		fatalf("file already exists in current directory: %s\n", linkPath)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		fatalf("mkdir: %v\n", err)
	}
	if err := os.WriteFile(localPath, []byte{}, 0600); err != nil {
		fatalf("create: %v\n", err)
	}
	if err := os.Symlink(localPath, linkPath); err != nil {
		fatalf("symlink: %v\n", err)
	}

	fmt.Printf("Created %s/%s\n", project, file)
	fmt.Printf("  local  : %s\n", localPath)
	fmt.Printf("  symlink: %s\n", linkPath)
	fmt.Println()
	fmt.Println("Edit the file, then run `envault push` to upload it to the server.")
}

// push <project> [file]
// Reads the local secret and uploads it to the server.
func runPush(args []string) {
	project, file := parseProjectFile(args)
	cfg := mustConfig()

	localPath := localSecretPath(project, file)
	content, err := os.ReadFile(localPath)
	if err != nil {
		fatalf("read local file: %v\n", err)
	}

	url := fmt.Sprintf("%s/api/projects/%s/files/%s", cfg.Server, project, file)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(content))
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("push: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatalf("server error %d: %s\n", resp.StatusCode, body)
	}

	fmt.Printf("Pushed %s/%s to %s\n", project, file, cfg.Server)
}

// pull <project> [file]
// Downloads from server, saves to local cache, and symlinks into cwd.
func runPull(args []string) {
	project, file := parseProjectFile(args)
	cfg := mustConfig()

	url := fmt.Sprintf("%s/api/projects/%s/files/%s", cfg.Server, project, file)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("pull: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fatalf("not found on server: %s/%s\n", project, file)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatalf("server error %d: %s\n", resp.StatusCode, body)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("read response: %v\n", err)
	}

	localPath := localSecretPath(project, file)
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		fatalf("mkdir: %v\n", err)
	}
	if err := os.WriteFile(localPath, content, 0600); err != nil {
		fatalf("write local: %v\n", err)
	}

	linkPath := filepath.Join(mustCwd(), file)
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		if err := os.Symlink(localPath, linkPath); err != nil {
			fatalf("symlink: %v\n", err)
		}
		fmt.Printf("Pulled %s/%s → %s (symlinked)\n", project, file, linkPath)
	} else {
		fmt.Printf("Pulled %s/%s → %s (updated)\n", project, file, localPath)
	}
}

// link <project> [file]
// Creates a symlink in cwd pointing to the local cache (must already exist locally).
func runLink(args []string) {
	project, file := parseProjectFile(args)
	localPath := localSecretPath(project, file)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fatalf("not cached locally — run `envault pull %s %s` first\n", project, file)
	}

	linkPath := filepath.Join(mustCwd(), file)
	if _, err := os.Lstat(linkPath); err == nil {
		fatalf("file already exists: %s\n", linkPath)
	}

	if err := os.Symlink(localPath, linkPath); err != nil {
		fatalf("symlink: %v\n", err)
	}
	fmt.Printf("Linked %s → %s\n", linkPath, localPath)
}

// list [project]
// Lists all projects (or files within a project) from the server.
func runList(args []string) {
	cfg := mustConfig()
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if len(args) == 0 {
		// List all projects
		projects, err := apiGetProjects(cfg)
		if err != nil {
			fatalf("%v\n", err)
		}
		if len(projects) == 0 {
			fmt.Println("No projects yet.")
			return
		}
		fmt.Fprintln(tw, "PROJECT")
		for _, p := range projects {
			fmt.Fprintln(tw, p)
		}
		tw.Flush()
		return
	}

	// List files in a project
	project := args[0]
	files, err := apiGetFiles(cfg, project)
	if err != nil {
		fatalf("%v\n", err)
	}
	if len(files) == 0 {
		fmt.Printf("No files in project %q.\n", project)
		return
	}
	fmt.Fprintln(tw, "FILE\tSIZE\tMODIFIED")
	for _, f := range files {
		fmt.Fprintf(tw, "%s\t%d B\t%s\n", f.Name, f.Size, f.ModTime.Format(time.RFC3339))
	}
	tw.Flush()
}

// sync
// Pushes all locally cached secrets to the server.
func runSync(args []string) {
	cfg := mustConfig()
	secretsDir := localSecretsDir()

	projects, err := os.ReadDir(secretsDir)
	if os.IsNotExist(err) {
		fmt.Println("No local secrets to sync.")
		return
	}
	if err != nil {
		fatalf("read secrets dir: %v\n", err)
	}

	pushed := 0
	for _, pd := range projects {
		if !pd.IsDir() {
			continue
		}
		project := pd.Name()
		files, err := os.ReadDir(filepath.Join(secretsDir, project))
		if err != nil {
			continue
		}
		for _, fd := range files {
			if fd.IsDir() {
				continue
			}
			file := fd.Name()
			localPath := filepath.Join(secretsDir, project, file)
			content, err := os.ReadFile(localPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  skip %s/%s: %v\n", project, file, err)
				continue
			}
			url := fmt.Sprintf("%s/api/projects/%s/files/%s", cfg.Server, project, file)
			req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(content))
			req.Header.Set("X-API-Key", cfg.APIKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  skip %s/%s: %v\n", project, file, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("  pushed %s/%s\n", project, file)
				pushed++
			} else {
				fmt.Fprintf(os.Stderr, "  failed %s/%s: HTTP %d\n", project, file, resp.StatusCode)
			}
		}
	}
	fmt.Printf("Synced %d file(s) to %s\n", pushed, cfg.Server)
}

// config [set server <url> | set key <apikey> | show]
func runConfig(args []string) {
	if len(args) == 0 || args[0] == "show" {
		cfg, err := loadConfig()
		if err != nil {
			fmt.Println("No config found. Run `envault config set server <url>` and `envault config set key <key>`.")
			return
		}
		fmt.Printf("server : %s\n", cfg.Server)
		fmt.Printf("api key: %s\n", maskKey(cfg.APIKey))
		fmt.Printf("config : %s\n", configPath())
		fmt.Printf("secrets: %s\n", localSecretsDir())
		return
	}

	if len(args) >= 3 && args[0] == "set" {
		cfg, _ := loadConfig()
		switch args[1] {
		case "server":
			cfg.Server = strings.TrimRight(args[2], "/")
			saveConfig(cfg)
			fmt.Printf("Server set to %s\n", cfg.Server)
		case "key":
			cfg.APIKey = args[2]
			saveConfig(cfg)
			fmt.Println("API key saved.")
		default:
			fatalf("unknown config key %q\n", args[1])
		}
		return
	}

	fatalf("usage: envault config [show | set server <url> | set key <apikey>]\n")
}

// ── API helpers ───────────────────────────────────────────────────────────────

type fileEntry struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

func apiGetProjects(cfg config) ([]string, error) {
	req, _ := http.NewRequest(http.MethodGet, cfg.Server+"/api/projects", nil)
	req.Header.Set("X-API-Key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var out struct {
		Projects []string `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Projects, nil
}

func apiGetFiles(cfg config, project string) ([]fileEntry, error) {
	url := fmt.Sprintf("%s/api/projects/%s/files", cfg.Server, project)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-API-Key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("project %q not found on server", project)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var out struct {
		Files []fileEntry `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Files, nil
}

// ── config ────────────────────────────────────────────────────────────────────

type config struct {
	Server string `json:"server"`
	APIKey string `json:"api_key"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".envault", "config.json")
}

func localSecretsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".envault", "secrets")
}

func localSecretPath(project, file string) string {
	return filepath.Join(localSecretsDir(), project, file)
}

func loadConfig() (config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return config{}, err
	}
	var cfg config
	return cfg, json.Unmarshal(data, &cfg)
}

func saveConfig(cfg config) {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		fatalf("mkdir config dir: %v\n", err)
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		fatalf("write config: %v\n", err)
	}
}

func mustConfig() config {
	cfg, err := loadConfig()
	if err != nil {
		fatalf("no config — run `envault config set server <url>` and `envault config set key <key>`\n")
	}
	if cfg.Server == "" {
		fatalf("server not configured — run `envault config set server <url>`\n")
	}
	if cfg.APIKey == "" {
		fatalf("api key not configured — run `envault config set key <apikey>`\n")
	}
	return cfg
}

// ── misc helpers ──────────────────────────────────────────────────────────────

func parseProjectFile(args []string) (project, file string) {
	if len(args) == 0 {
		fatalf("usage: envault <command> <project> [file]\n")
	}
	project = args[0]
	file = defaultFile
	if len(args) >= 2 {
		file = args[1]
	}
	return
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v\n", err)
	}
	return cwd
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "envault: "+format, args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Print(`envault — secure .env file manager

USAGE:
  envault <command> [arguments]

COMMANDS:
  new  <project> [file]   Create a new env file locally and symlink it into cwd
  push <project> [file]   Upload local env file to the server
  pull <project> [file]   Download env file from server; symlink into cwd
  link <project> [file]   Symlink a cached env file into cwd
  list [project]          List projects (or files within a project)
  sync                    Push all locally cached env files to the server

  config show             Show current configuration
  config set server <url> Set the server URL
  config set key <key>    Set the API key

DEFAULTS:
  file defaults to ".env" when not specified

EXAMPLES:
  envault config set server http://localhost:8080
  envault config set key mysecretkey

  cd ~/projects/cool-project
  envault new cool-project          # creates .env + symlink
  # edit .env …
  envault push cool-project         # upload to server

  cd ~/projects/cool-project-clone
  envault pull cool-project         # download + symlink .env
`)
}
