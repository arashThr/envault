package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"filippo.io/age"
)

// ageHeader is the prefix of every age binary-format ciphertext.
var ageHeader = []byte("age-encryption.org/v1\n")

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
	case "remove", "rm":
		runRemove(args)
	case "sync":
		runSync(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fatalf("unknown command %q — run `envault help` for usage\n", cmd)
	}
}

// ── commands ─────────────────────────────────────────────────────────────────

// new [project] [env]
// Creates an env file in the local vault and symlinks it into cwd.
// Prompts for project (default = cwd name) and environment when omitted.
// If a matching file already exists in cwd it is adopted into the vault.
func runNew(args []string) {
	project, env := parseProjectEnv(args)
	localPath := localSecretPath(project, env)
	linkPath := filepath.Join(mustCwd(), symlinkName(env))

	if _, err := os.Stat(localPath); err == nil {
		fatalf("secret already exists in vault: %s\n", localPath)
	}

	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			fatalf("%s is already a symlink — run `envault push` to upload it\n", symlinkName(env))
		}
		// Real file: confirm then adopt.
		confirmAdopt(symlinkName(env), project, env, localPath)
		moveToVault(linkPath, localPath, project, env)
		fmt.Println("Run `envault push` to upload it to the server.")
		return
	}

	// No existing file — create a new empty one.
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		fatalf("mkdir: %v\n", err)
	}
	if err := os.WriteFile(localPath, []byte{}, 0600); err != nil {
		fatalf("create: %v\n", err)
	}
	if err := os.Symlink(localPath, linkPath); err != nil {
		fatalf("symlink: %v\n", err)
	}

	fmt.Printf("Created %s/%s\n", project, env)
	fmt.Printf("  vault  : %s\n", localPath)
	fmt.Printf("  symlink: %s\n", linkPath)
	fmt.Println()
	fmt.Println("Edit the file, then run `envault push` to upload it to the server.")
}

// push [project] [env]
// Encrypts the local secret and uploads ciphertext to the server.
// If a real file exists in cwd but not in the vault, auth is checked first,
// then the user is asked to confirm before the file is adopted.
func runPush(args []string) {
	project, env := parseProjectEnv(args)
	cfg := mustConfig()

	localPath := localSecretPath(project, env)
	linkPath := filepath.Join(mustCwd(), symlinkName(env))

	// Adopt a real file from cwd if the vault copy is missing.
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if info, err := os.Lstat(linkPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
			if err := checkAuth(cfg); err != nil {
				fatalf("authentication failed: %v\n", err)
			}
			confirmAdopt(symlinkName(env), project, env, localPath)
			moveToVault(linkPath, localPath, project, env)
		}
	}

	plaintext, err := os.ReadFile(localPath)
	if err != nil {
		fatalf("read local file: %v\n", err)
	}

	content, err := encryptContent(plaintext, cfg.APIKey)
	if err != nil {
		fatalf("encrypt: %v\n", err)
	}

	if err := apiPutFile(cfg, project, env, content); err != nil {
		fatalf("push: %v\n", err)
	}
	fmt.Printf("Pushed %s/%s to %s (encrypted)\n", project, env, cfg.Server)
}

// pull [project] [env]
// Downloads and decrypts a secret from the server, saves it locally, symlinks into cwd.
func runPull(args []string) {
	project, env := parseProjectEnv(args)
	cfg := mustConfig()

	url := fmt.Sprintf("%s/api/projects/%s/files/%s", cfg.Server, project, env)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-API-Key", cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("pull: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		fatalf("not found on server: %s/%s\n", project, env)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatalf("server error %d: %s\n", resp.StatusCode, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("read response: %v\n", err)
	}

	content := data
	if isAgeEncrypted(data) {
		content, err = decryptContent(data, cfg.APIKey)
		if err != nil {
			fatalf("decrypt: %v\n", err)
		}
	}

	localPath := localSecretPath(project, env)
	if err := os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		fatalf("mkdir: %v\n", err)
	}
	if err := os.WriteFile(localPath, content, 0600); err != nil {
		fatalf("write local: %v\n", err)
	}

	linkPath := filepath.Join(mustCwd(), symlinkName(env))
	if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
		if err := os.Symlink(localPath, linkPath); err != nil {
			fatalf("symlink: %v\n", err)
		}
		fmt.Printf("Pulled %s/%s → %s (symlinked)\n", project, env, linkPath)
	} else {
		fmt.Printf("Pulled %s/%s → %s (updated)\n", project, env, localPath)
	}
}

// link [project] [env]
// Creates a symlink in cwd pointing to a locally cached secret.
func runLink(args []string) {
	project, env := parseProjectEnv(args)
	localPath := localSecretPath(project, env)

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fatalf("not cached locally — run `envault pull %s %s` first\n", project, env)
	}

	linkPath := filepath.Join(mustCwd(), symlinkName(env))
	if _, err := os.Lstat(linkPath); err == nil {
		fatalf("file already exists: %s\n", linkPath)
	}

	if err := os.Symlink(localPath, linkPath); err != nil {
		fatalf("symlink: %v\n", err)
	}
	fmt.Printf("Linked %s → %s\n", linkPath, localPath)
}

// remove [project] [env]
// Deletes a locally cached secret file (or an entire project directory).
// Does not touch the server; use the web UI to delete from the server.
func runRemove(args []string) {
	var project, env string
	switch len(args) {
	case 0:
		project = promptLine("Project", filepath.Base(mustCwd()))
	case 1:
		project = args[0]
	default:
		project, env = args[0], args[1]
	}

	if env != "" {
		path := localSecretPath(project, env)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fatalf("no local secret found for %s/%s\n", project, env)
		}
		if err := os.Remove(path); err != nil {
			fatalf("remove: %v\n", err)
		}
		fmt.Printf("Removed local %s/%s\n", project, env)
		fmt.Println("Note: any symlink pointing to this file is now dangling.")
		return
	}

	dir := filepath.Join(localSecretsDir(), project)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fatalf("no local project %q found\n", project)
	}
	fmt.Printf("Delete all local secrets for project %q? [y/N] ", project)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if answer := strings.TrimSpace(strings.ToLower(scanner.Text())); answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		fatalf("remove: %v\n", err)
	}
	fmt.Printf("Removed local project %s\n", project)
	fmt.Println("Note: any symlinks pointing to these files are now dangling.")
}

// list [project]
// Lists all projects, or all environments within a project.
// Merges local (~/.envault/secrets) and server results, showing status for each.
func runList(args []string) {
	cfg, _ := loadConfig() // best-effort; server fetch may fail gracefully
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if len(args) == 0 {
		// Fetch from server (optional — no server configured or unreachable is ok)
		serverProjects := []string{}
		serverWarn := ""
		if cfg.Server != "" {
			var err error
			serverProjects, err = apiGetProjects(cfg)
			if err != nil {
				serverWarn = fmt.Sprintf("(server unreachable: %v)", err)
			}
		}

		serverSet := make(map[string]bool)
		for _, p := range serverProjects {
			serverSet[p] = true
		}
		localSet := make(map[string]bool)
		for _, p := range localProjectNames() {
			localSet[p] = true
		}

		all := mergedKeys(serverSet, localSet)
		if len(all) == 0 {
			fmt.Println("No projects yet.")
			return
		}

		fmt.Fprintln(tw, "PROJECT\tSTATUS")
		for _, p := range all {
			var status string
			switch {
			case serverSet[p] && localSet[p]:
				status = "synced"
			case localSet[p]:
				status = "local-only"
			default:
				status = "server-only"
			}
			fmt.Fprintf(tw, "%s\t%s\n", p, status)
		}
		tw.Flush()
		if serverWarn != "" {
			fmt.Fprintln(os.Stderr, serverWarn)
		}
		return
	}

	project := args[0]

	serverFiles := []fileEntry{}
	serverWarn := ""
	if cfg.Server != "" {
		var err error
		serverFiles, err = apiGetFiles(cfg, project)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			serverWarn = fmt.Sprintf("(server: %v)", err)
		}
	}

	serverSet := make(map[string]fileEntry)
	for _, f := range serverFiles {
		serverSet[f.Name] = f
	}
	localSet := make(map[string]bool)
	for _, e := range localEnvNames(project) {
		localSet[e] = true
	}

	boolServerSet := make(map[string]bool)
	for k := range serverSet {
		boolServerSet[k] = true
	}
	all := mergedKeys(boolServerSet, localSet)
	if len(all) == 0 {
		fmt.Printf("No environments in project %q.\n", project)
		return
	}

	fmt.Fprintln(tw, "ENVIRONMENT\tSIZE\tMODIFIED\tSTATUS")
	for _, env := range all {
		sf, onServer := serverSet[env]
		onLocal := localSet[env]
		var status string
		switch {
		case onServer && onLocal:
			status = "synced"
		case onLocal:
			status = "local-only"
		default:
			status = "server-only"
		}
		if onServer {
			fmt.Fprintf(tw, "%s\t%d B\t%s\t%s\n", env, sf.Size, sf.ModTime.Format(time.RFC3339), status)
		} else {
			fmt.Fprintf(tw, "%s\t—\t—\t%s\n", env, status)
		}
	}
	tw.Flush()
	if serverWarn != "" {
		fmt.Fprintln(os.Stderr, serverWarn)
	}
}

// localProjectNames returns the names of all locally cached projects.
func localProjectNames() []string {
	entries, _ := os.ReadDir(localSecretsDir())
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// localEnvNames returns the names of all locally cached environments for a project.
func localEnvNames(project string) []string {
	entries, _ := os.ReadDir(filepath.Join(localSecretsDir(), project))
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// mergedKeys returns a sorted union of keys from two bool maps.
func mergedKeys(a, b map[string]bool) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sync encrypts and pushes all locally cached secrets to the server.
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
		envs, err := os.ReadDir(filepath.Join(secretsDir, project))
		if err != nil {
			continue
		}
		for _, fd := range envs {
			if fd.IsDir() {
				continue
			}
			env := fd.Name()
			localPath := filepath.Join(secretsDir, project, env)
			plaintext, err := os.ReadFile(localPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  skip %s/%s: %v\n", project, env, err)
				continue
			}
			content, err := encryptContent(plaintext, cfg.APIKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  skip %s/%s: encrypt: %v\n", project, env, err)
				continue
			}
			if err := apiPutFile(cfg, project, env, content); err != nil {
				fmt.Fprintf(os.Stderr, "  failed %s/%s: %v\n", project, env, err)
				continue
			}
			fmt.Printf("  pushed %s/%s\n", project, env)
			pushed++
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

// ── crypto ────────────────────────────────────────────────────────────────────

func encryptContent(plaintext []byte, passphrase string) ([]byte, error) {
	recipient, err := age.NewScryptRecipient(passphrase)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decryptContent(ciphertext []byte, passphrase string) ([]byte, error) {
	identity, err := age.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

// isAgeEncrypted returns true if data begins with the age binary-format header.
func isAgeEncrypted(data []byte) bool {
	return bytes.HasPrefix(data, ageHeader)
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

func apiPutFile(cfg config, project, env string, content []byte) error {
	url := fmt.Sprintf("%s/api/projects/%s/files/%s", cfg.Server, project, env)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(content))
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error %d: %s", resp.StatusCode, body)
	}
	return nil
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

func localSecretPath(project, env string) string {
	return filepath.Join(localSecretsDir(), project, env)
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

// symlinkName returns the filename used in the working directory for a given
// environment: "local" maps to ".env"; all others map to ".env.<env>".
func symlinkName(env string) string {
	if env == "local" {
		return ".env"
	}
	return ".env." + env
}

// parseProjectEnv resolves the project and environment from CLI args,
// falling back to interactive prompts for any that are missing.
func parseProjectEnv(args []string) (project, env string) {
	cwd := mustCwd()
	defaultProject := filepath.Base(cwd)

	switch len(args) {
	case 0:
		project = promptLine("Project", defaultProject)
		env = promptEnv()
	case 1:
		project = args[0]
		env = promptEnv()
	default:
		project, env = args[0], args[1]
	}
	return
}

// promptLine prints a prompt with a default value and reads a line from stdin.
// Returns the default if the user just presses Enter.
func promptLine(label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return def
	}
	return v
}

// promptEnv asks for the environment name with a default of "local".
func promptEnv() string {
	fmt.Print("Environment (e.g. local, production, staging) [local]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return "local"
	}
	return v
}

// checkAuth makes a lightweight request to verify the configured API key.
func checkAuth(cfg config) error {
	req, _ := http.NewRequest(http.MethodGet, cfg.Server+"/api/projects", nil)
	req.Header.Set("X-API-Key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach server: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("wrong API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// confirmAdopt shows what will happen and requires explicit [y/N] confirmation.
// Exits cleanly if the user declines.
func confirmAdopt(filename, project, env, vaultPath string) {
	fmt.Printf("Found existing %s in the current directory.\n", filename)
	fmt.Printf("  Project    : %s\n", project)
	fmt.Printf("  Environment: %s\n", env)
	fmt.Printf("  Vault path : %s\n", vaultPath)
	fmt.Printf("  The file will be moved to the vault and replaced with a symlink.\n")
	fmt.Print("Proceed? [y/N] ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if answer := strings.TrimSpace(strings.ToLower(scanner.Text())); answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		os.Exit(0)
	}
}

// moveToVault moves srcPath into the vault at dstPath and replaces it with a symlink.
func moveToVault(srcPath, dstPath, project, env string) {
	fmt.Printf("Moving to vault as %s/%s...\n", project, env)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		fatalf("mkdir vault: %v\n", err)
	}
	if err := os.Rename(srcPath, dstPath); err != nil {
		fatalf("move to vault: %v\n", err)
	}
	if err := os.Chmod(dstPath, 0600); err != nil {
		fatalf("chmod: %v\n", err)
	}
	if err := os.Symlink(dstPath, srcPath); err != nil {
		fatalf("symlink: %v\n", err)
	}
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
  envault <command> [project] [environment]

COMMANDS:
  new    [project] [env]   Add an env file to the vault and symlink it into cwd
  push   [project] [env]   Encrypt and upload a local env file to the server
  pull   [project] [env]   Download and decrypt an env file; symlink into cwd
  link   [project] [env]   Symlink a cached env file into cwd (no download)
  remove [project] [env]   Delete a locally cached env file (or entire project)
  list   [project]         List projects (or environments), merged from local + server
  sync                     Encrypt and push all locally cached env files

  config show              Show current configuration
  config set server <url>  Set the server URL
  config set key <key>     Set the API key / encryption passphrase

DEFAULTS:
  project     defaults to the current directory name
  environment defaults to "local" (symlinked as .env)
              other environments are symlinked as .env.<environment>

EXAMPLES:
  envault new                       # prompts for project and environment
  envault new myapp production      # no prompts
  envault push myapp production     # encrypt + upload
  envault pull myapp production     # download + decrypt + symlink as .env.production
  envault list myapp                # show environments for myapp (local + server)
  envault remove myapp local        # delete local copy of myapp/local
  envault remove myapp              # delete all local secrets for myapp
`)
}
