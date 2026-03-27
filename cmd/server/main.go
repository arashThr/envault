package main

import (
	"context"
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
type Config struct {
	Port    string
	DataDir string
	APIKey  string
	Debug   bool
}

func newConfig() Config {
	port    := flag.String("port",  envOr("PORT", "8080"),         "listen port")
	dataDir := flag.String("data",  envOr("DATA_DIR", "./data"),   "directory to store env files")
	apiKey  := flag.String("key",   envOr("API_KEY", ""),          "API key for authentication")
	debug   := flag.Bool("debug",   envOr("DEBUG", "") == "true",  "enable debug logging")
	flag.Parse()
	return Config{
		Port:    *port,
		DataDir: *dataDir,
		APIKey:  *apiKey,
		Debug:   *debug,
	}
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
	)
	return logger
}

func newStore(cfg Config, logger *slog.Logger) (*store.Store, error) {
	if cfg.APIKey == "" {
		logger.Error("API_KEY is required — set via -key flag or API_KEY environment variable")
		return nil, fmt.Errorf("API_KEY is required")
	}
	logger.Debug("configuration loaded", "key_len", len(cfg.APIKey))

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
	mux.Handle("/api/", api.New(s, cfg.APIKey, logger))
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
