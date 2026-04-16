package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/arashthr/envault/internal/api"
	"github.com/arashthr/envault/internal/store"
)

//go:embed web
var webFiles embed.FS

// Config holds all startup configuration parsed from flags and env vars.
// APIKeyHash is the SHA-256 of the plaintext password; the plaintext is
// discarded immediately after hashing so it is never held in memory.
// When NoAuth is true the server runs without authentication.
type Config struct {
	Port       string
	DataDir    string
	APIKeyHash [32]byte
	NoAuth     bool
	Debug      bool
}

func newConfig() (Config, error) {
	port    := flag.String("port",  envOr("PORT", "8080"),         "listen port")
	dataDir := flag.String("data",  envOr("DATA_DIR", "./data"),   "directory to store env files")
	apiKey  := flag.String("key",   envOr("API_KEY", ""),          "password for authentication (omit to disable auth)")
	debug   := flag.Bool("debug",   envOr("DEBUG", "") == "true",  "enable debug logging")
	flag.Parse()

	cfg := Config{
		Port:    *port,
		DataDir: *dataDir,
		Debug:   *debug,
	}
	if *apiKey != "" {
		cfg.APIKeyHash = sha256.Sum256([]byte(*apiKey))
	} else {
		cfg.NoAuth = true
	}
	return cfg, nil
}

func newLogger(cfg Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	logger.Info("envault starting",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"debug", cfg.Debug,
		"auth", !cfg.NoAuth,
	)
	if cfg.NoAuth {
		logger.Warn("running WITHOUT authentication — all API endpoints are public")
	}
	return logger
}

func newStore(cfg Config, logger *slog.Logger) (*store.Store, error) {
	s, err := store.New(cfg.DataDir, logger)
	if err != nil {
		logger.Error("failed to open store", "dir", cfg.DataDir, "err", err)
		return nil, fmt.Errorf("open store: %w", err)
	}
	logger.Info("store ready", "dir", cfg.DataDir)
	return s, nil
}

func newWebRoot(logger *slog.Logger) (fs.FS, error) {
	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		logger.Error("web embed failed", "err", err)
		return nil, fmt.Errorf("web embed: %w", err)
	}
	logger.Debug("web UI embedded and ready")
	return webRoot, nil
}

func newMux(s *store.Store, webRoot fs.FS, cfg Config, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/api/", api.New(s, cfg.APIKeyHash, cfg.NoAuth, logger))
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	return mux
}

func newServer(cfg Config, mux *http.ServeMux) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func runServer(lc fx.Lifecycle, srv *http.Server, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("server listening", "addr", "http://localhost"+srv.Addr)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("server stopped unexpectedly", "err", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("server shutting down")
			return srv.Shutdown(ctx)
		},
	})
}

func main() {
	app := fx.New(
		fx.Provide(
			newConfig,
			newLogger,
			newStore,
			newWebRoot,
			newMux,
			newServer,
		),
		fx.Invoke(runServer),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
	app.Run()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
