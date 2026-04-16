package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/arashthr/envault/internal/store"
)

// New builds a chi router with all API routes mounted under /api.
// apiKeyHash is the SHA-256 hash of the expected password; the plaintext is
// never held in memory after the caller computes the hash at startup.
// When noAuth is true the auth middleware is skipped entirely.
func New(s *store.Store, apiKeyHash [32]byte, noAuth bool, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	if !noAuth {
		r.Use(authMiddleware(apiKeyHash, logger))
	}

	r.Route("/api", func(r chi.Router) {
		r.Get("/projects", listProjects(s, logger))
		r.Delete("/projects/{project}", deleteProject(s, logger))

		r.Get("/projects/{project}/files", listFiles(s, logger))
		r.Get("/projects/{project}/files/{file}", getFile(s, logger))
		r.Put("/projects/{project}/files/{file}", putFile(s, logger))
		r.Delete("/projects/{project}/files/{file}", deleteFile(s, logger))
	})

	return r
}

// ── middleware ────────────────────────────────────────────────────────────────

func authMiddleware(apiKeyHash [32]byte, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := extractKey(r)
			provided := sha256.Sum256([]byte(key))
			if subtle.ConstantTimeCompare(provided[:], apiKeyHash[:]) != 1 {
				logger.Warn("authentication failed",
					"remote_addr", r.RemoteAddr,
					"method", r.Method,
					"path", r.URL.Path,
					"has_key", key != "",
					"id", middleware.GetReqID(r.Context()),
				)
				w.Header().Set("WWW-Authenticate", `Basic realm="Envault"`)
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractKey pulls the credential from the request, trying three locations
// in priority order:
//  1. X-API-Key header (CLI legacy)
//  2. Authorization: Bearer <token>
//  3. Authorization: Basic base64(user:password) — username is ignored
func extractKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if strings.HasPrefix(auth, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err == nil {
			// format is "username:password"; we only validate the password
			if _, password, ok := strings.Cut(string(decoded), ":"); ok {
				return password
			}
		}
	}
	return ""
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			logger.Debug("request started",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"id", middleware.GetReqID(r.Context()),
			)

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			level := slog.LevelInfo
			if ww.Status() >= 500 {
				level = slog.LevelError
			} else if ww.Status() >= 400 {
				level = slog.LevelWarn
			}

			logger.Log(r.Context(), level, "request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", duration.Milliseconds(),
				"remote_addr", r.RemoteAddr,
				"id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func listProjects(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projects, err := s.ListProjects()
		if err != nil {
			logger.Error("list projects failed", "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if projects == nil {
			projects = []string{}
		}
		writeJSON(w, map[string]any{"projects": projects})
	}
}

func deleteProject(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		if err := s.DeleteProject(project); err != nil {
			logger.Error("delete project failed", "project", project, "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listFiles(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		files, err := s.ListFiles(project)
		if err != nil {
			logger.Warn("list files failed", "project", project, "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if files == nil {
			files = []store.FileInfo{}
		}
		writeJSON(w, map[string]any{"files": files})
	}
}

func getFile(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		content, err := s.GetFile(project, file)
		if err != nil {
			logger.Warn("get file failed", "project", project, "file", file, "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)
	}
}

func putFile(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		content, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			logger.Error("failed to read request body", "project", project, "file", file, "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.PutFile(project, file, content); err != nil {
			logger.Error("put file failed", "project", project, "file", file, "err", err, "id", middleware.GetReqID(r.Context()))
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func deleteFile(s *store.Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := chi.URLParam(r, "project")
		file := chi.URLParam(r, "file")
		if err := s.DeleteFile(project, file); err != nil {
			logger.Error("delete file failed", "project", project, "file", file, "err", err, "id", middleware.GetReqID(r.Context()))
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
