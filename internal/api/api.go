package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/arashthr/envault/internal/store"
)

// API handles all /api/* routes.
type API struct {
	store  *store.Store
	apiKey string
}

// New creates an API handler.
func New(s *store.Store, apiKey string) *API {
	return &API{store: s, apiKey: apiKey}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.authenticate(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Strip /api/ prefix then split into segments.
	path := strings.TrimPrefix(r.URL.Path, "/api")
	parts := splitPath(path)

	switch {
	// GET /api/projects
	case match(parts, "projects") && r.Method == http.MethodGet:
		a.listProjects(w, r)

	// DELETE /api/projects/{project}
	case match(parts, "projects", "*") && r.Method == http.MethodDelete:
		a.deleteProject(w, r, parts[1])

	// GET /api/projects/{project}/files
	case match(parts, "projects", "*", "files") && r.Method == http.MethodGet:
		a.listFiles(w, r, parts[1])

	// GET /api/projects/{project}/files/{file}
	case match(parts, "projects", "*", "files", "*") && r.Method == http.MethodGet:
		a.getFile(w, r, parts[1], parts[3])

	// PUT /api/projects/{project}/files/{file}
	case match(parts, "projects", "*", "files", "*") && r.Method == http.MethodPut:
		a.putFile(w, r, parts[1], parts[3])

	// DELETE /api/projects/{project}/files/{file}
	case match(parts, "projects", "*", "files", "*") && r.Method == http.MethodDelete:
		a.deleteFile(w, r, parts[1], parts[3])

	default:
		http.NotFound(w, r)
	}
}

// ── handlers ────────────────────────────────────────────────────────────────

func (a *API) listProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := a.store.ListProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []string{}
	}
	writeJSON(w, map[string]any{"projects": projects})
}

func (a *API) deleteProject(w http.ResponseWriter, _ *http.Request, project string) {
	if err := a.store.DeleteProject(project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listFiles(w http.ResponseWriter, _ *http.Request, project string) {
	files, err := a.store.ListFiles(project)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if files == nil {
		files = []store.FileInfo{}
	}
	writeJSON(w, map[string]any{"files": files})
}

func (a *API) getFile(w http.ResponseWriter, _ *http.Request, project, filename string) {
	content, err := a.store.GetFile(project, filename)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

func (a *API) putFile(w http.ResponseWriter, r *http.Request, project, filename string) {
	content, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.PutFile(project, filename, content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (a *API) deleteFile(w http.ResponseWriter, _ *http.Request, project, filename string) {
	if err := a.store.DeleteFile(project, filename); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (a *API) authenticate(r *http.Request) bool {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key == a.apiKey
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == a.apiKey
	}
	return false
}

// splitPath turns "/projects/foo/files/bar" into ["projects","foo","files","bar"].
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// match checks that parts has the same length as pattern and each non-"*"
// segment equals the corresponding part.
func match(parts []string, pattern ...string) bool {
	if len(parts) != len(pattern) {
		return false
	}
	for i, seg := range pattern {
		if seg != "*" && seg != parts[i] {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
