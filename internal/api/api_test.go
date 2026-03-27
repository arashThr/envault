package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arashthr/envault/internal/api"
	"github.com/arashthr/envault/internal/store"
)

const testKey = "test-api-key"

// setup creates a test server backed by a temp store.
// It returns the handler and a function that sends authenticated requests.
func setup(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := store.New(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return api.New(s, testKey, logger), s
}

// do sends a request to the handler and returns the response.
func do(t *testing.T, h http.Handler, method, path, body, key string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Result()
}

// auth is a shortcut for do with the correct key.
func auth(t *testing.T, h http.Handler, method, path, body string) *http.Response {
	return do(t, h, method, path, body, testKey)
}

// decodeJSON reads JSON from a response body into v.
func decodeJSON(t *testing.T, res *http.Response, v any) {
	t.Helper()
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// bodyText reads the response body as a string.
func bodyText(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func mustStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want %d\nbody: %s", res.StatusCode, want, body)
	}
}

// ── Authentication ────────────────────────────────────────────────────────────

func TestAuth_missingKey(t *testing.T) {
	h, _ := setup(t)
	res := do(t, h, http.MethodGet, "/api/projects", "", "")
	mustStatus(t, res, http.StatusUnauthorized)
}

func TestAuth_wrongKey(t *testing.T) {
	h, _ := setup(t)
	res := do(t, h, http.MethodGet, "/api/projects", "", "wrong-key")
	mustStatus(t, res, http.StatusUnauthorized)
}

func TestAuth_bearerToken(t *testing.T) {
	h, _ := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	mustStatus(t, w.Result(), http.StatusOK)
}

func TestAuth_errorBody(t *testing.T) {
	h, _ := setup(t)
	res := do(t, h, http.MethodGet, "/api/projects", "", "bad")
	mustStatus(t, res, http.StatusUnauthorized)
	var body map[string]string
	decodeJSON(t, res, &body)
	if body["error"] == "" {
		t.Error("expected error field in response body")
	}
}

// ── GET /api/projects ─────────────────────────────────────────────────────────

func TestListProjects_empty(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodGet, "/api/projects", "")
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Projects []string `json:"projects"`
	}
	decodeJSON(t, res, &body)
	if len(body.Projects) != 0 {
		t.Errorf("expected empty projects, got %v", body.Projects)
	}
}

func TestListProjects(t *testing.T) {
	h, s := setup(t)
	_ = s.PutFile("alpha", ".env", []byte("A=1"))
	_ = s.PutFile("beta", ".env", []byte("B=2"))

	res := auth(t, h, http.MethodGet, "/api/projects", "")
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Projects []string `json:"projects"`
	}
	decodeJSON(t, res, &body)
	if len(body.Projects) != 2 {
		t.Errorf("expected 2 projects, got %v", body.Projects)
	}
}

// ── DELETE /api/projects/{project} ────────────────────────────────────────────

func TestDeleteProject(t *testing.T) {
	h, s := setup(t)
	_ = s.PutFile("proj", ".env", []byte("X=1"))

	res := auth(t, h, http.MethodDelete, "/api/projects/proj", "")
	mustStatus(t, res, http.StatusNoContent)

	// Verify it's gone
	res = auth(t, h, http.MethodGet, "/api/projects/proj/files", "")
	mustStatus(t, res, http.StatusNotFound)
}

// ── GET /api/projects/{project}/files ─────────────────────────────────────────

func TestListFiles_empty(t *testing.T) {
	h, s := setup(t)
	_ = s.PutFile("proj", ".env", []byte("A=1"))
	_ = s.DeleteFile("proj", ".env")

	res := auth(t, h, http.MethodGet, "/api/projects/proj/files", "")
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Files []map[string]any `json:"files"`
	}
	decodeJSON(t, res, &body)
	if len(body.Files) != 0 {
		t.Errorf("expected empty files list, got %v", body.Files)
	}
}

func TestListFiles(t *testing.T) {
	h, s := setup(t)
	_ = s.PutFile("proj", ".env", []byte("A=1"))
	_ = s.PutFile("proj", ".env.staging", []byte("A=stage"))

	res := auth(t, h, http.MethodGet, "/api/projects/proj/files", "")
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Files []struct {
			Name string `json:"name"`
			Size int64  `json:"size"`
		} `json:"files"`
	}
	decodeJSON(t, res, &body)
	if len(body.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(body.Files))
	}
	names := map[string]bool{}
	for _, f := range body.Files {
		names[f.Name] = true
		if f.Size == 0 {
			t.Errorf("expected non-zero size for %q", f.Name)
		}
	}
	if !names[".env"] || !names[".env.staging"] {
		t.Errorf("unexpected file names: %v", names)
	}
}

func TestListFiles_projectNotFound(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodGet, "/api/projects/ghost/files", "")
	mustStatus(t, res, http.StatusNotFound)

	var body map[string]string
	decodeJSON(t, res, &body)
	if body["error"] == "" {
		t.Error("expected error message in body")
	}
}

// ── PUT /api/projects/{project}/files/{file} ──────────────────────────────────

func TestPutFile(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodPut, "/api/projects/myapp/files/.env", "SECRET=abc")
	mustStatus(t, res, http.StatusOK)

	var body map[string]string
	decodeJSON(t, res, &body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body)
	}
}

func TestPutFile_appears_in_list(t *testing.T) {
	h, _ := setup(t)
	auth(t, h, http.MethodPut, "/api/projects/myapp/files/.env", "SECRET=abc")

	res := auth(t, h, http.MethodGet, "/api/projects/myapp/files", "")
	mustStatus(t, res, http.StatusOK)

	var body struct {
		Files []struct{ Name string `json:"name"` } `json:"files"`
	}
	decodeJSON(t, res, &body)
	if len(body.Files) != 1 || body.Files[0].Name != ".env" {
		t.Errorf("unexpected files after put: %v", body.Files)
	}
}

func TestPutFile_invalidName(t *testing.T) {
	h, _ := setup(t)
	// Path traversal via URL-encoded segments handled by chi (404, not 400).
	// Direct invalid name via store validation:
	res := auth(t, h, http.MethodPut, "/api/projects/../evil/files/.env", "X=1")
	// chi won't route this — expect 404 or 400, not 200.
	if res.StatusCode == http.StatusOK {
		t.Error("expected non-200 for path-traversal attempt")
	}
}

// ── GET /api/projects/{project}/files/{file} ──────────────────────────────────

func TestGetFile(t *testing.T) {
	h, s := setup(t)
	content := "DB=postgres://localhost/app\nSECRET=xyz\n"
	_ = s.PutFile("myapp", ".env", []byte(content))

	res := auth(t, h, http.MethodGet, "/api/projects/myapp/files/.env", "")
	mustStatus(t, res, http.StatusOK)

	got := bodyText(t, res)
	if got != content {
		t.Errorf("content mismatch\n got: %q\nwant: %q", got, content)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("unexpected Content-Type: %s", ct)
	}
}

func TestGetFile_notFound(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodGet, "/api/projects/ghost/files/.env", "")
	mustStatus(t, res, http.StatusNotFound)

	var body map[string]string
	decodeJSON(t, res, &body)
	if body["error"] == "" {
		t.Error("expected error message in body")
	}
}

func TestGetFile_roundTrip(t *testing.T) {
	h, _ := setup(t)
	content := "KEY=value123\nANOTHER=secret\n"

	// Put via API
	res := auth(t, h, http.MethodPut, "/api/projects/app/files/.env", content)
	mustStatus(t, res, http.StatusOK)

	// Get via API
	res = auth(t, h, http.MethodGet, "/api/projects/app/files/.env", "")
	mustStatus(t, res, http.StatusOK)

	got := bodyText(t, res)
	if got != content {
		t.Errorf("round-trip mismatch\n got: %q\nwant: %q", got, content)
	}
}

// ── DELETE /api/projects/{project}/files/{file} ───────────────────────────────

func TestDeleteFile(t *testing.T) {
	h, s := setup(t)
	_ = s.PutFile("proj", ".env", []byte("X=1"))

	res := auth(t, h, http.MethodDelete, "/api/projects/proj/files/.env", "")
	mustStatus(t, res, http.StatusNoContent)

	res = auth(t, h, http.MethodGet, "/api/projects/proj/files/.env", "")
	mustStatus(t, res, http.StatusNotFound)
}

func TestDeleteFile_notFound(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodDelete, "/api/projects/ghost/files/.env", "")
	// File doesn't exist — store returns an error.
	if res.StatusCode == http.StatusOK {
		t.Error("expected non-200 deleting nonexistent file")
	}
}

// ── Route coverage ────────────────────────────────────────────────────────────

func TestUnknownRoute(t *testing.T) {
	h, _ := setup(t)
	res := auth(t, h, http.MethodGet, "/api/does-not-exist", "")
	mustStatus(t, res, http.StatusNotFound)
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := setup(t)
	// POST is not a valid method for /api/projects
	res := auth(t, h, http.MethodPost, "/api/projects", "")
	mustStatus(t, res, http.StatusMethodNotAllowed)
}
