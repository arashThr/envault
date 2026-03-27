package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/arashthr/envault/internal/store"
)

// New builds a chi router with all API routes mounted under /api.
// The caller should mount it at "/api/*" or use it as the full mux.
func New(s *store.Store, apiKey string, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(authMiddleware(apiKey))

	r.Route("/api", func(r chi.Router) {
		// Projects
		r.Get("/projects", listProjects(s))
		r.Delete("/projects/{project}", deleteProject(s))

		// Files within a project
		r.Get("/projects/{project}/files", listFiles(s))
		r.Get("/projects/{project}/files/{file}", getFile(s))
		r.Put("/projects/{project}/files/{file}", putFile(s))
		r.Delete("/projects/{project}/files/{file}", deleteFile(s))
	})

	return r
}

// ── middleware ────────────────────────────────────────────────────────────────

func authMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
					key = auth[7:]
				}
			}
			if key != apiKey {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func listProjects(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := s.ListProjects()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if projects == nil {
			projects = []string{}
		}
		writeJSON(w, map[string]any{"projects": projects})
	}
}

func deleteProject(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		if err := s.DeleteProject(project); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listFiles(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		files, err := s.ListFiles(project)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if files == nil {
			files = []store.FileInfo{}
		}
		writeJSON(w, map[string]any{"files": files})
	}
}

func getFile(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		content, err := s.GetFile(project, file)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)
	}
}

func putFile(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		content, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.PutFile(project, file, content); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func deleteFile(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		if err := s.DeleteFile(project, file); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
