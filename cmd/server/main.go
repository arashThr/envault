package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/arashthr/envault/internal/api"
	"github.com/arashthr/envault/internal/store"
)

//go:embed web
var webFiles embed.FS

func main() {
	port    := flag.String("port",  envOr("PORT", "8080"),   "listen port")
	dataDir := flag.String("data",  envOr("DATA_DIR", "./data"), "directory to store env files")
	apiKey  := flag.String("key",   envOr("API_KEY", ""),    "API key for authentication")
	debug   := flag.Bool("debug",   envOr("DEBUG", "") == "true", "enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	logger.Info("envault starting",
		"port", *port,
		"data_dir", *dataDir,
		"debug", *debug,
	)

	if *apiKey == "" {
		logger.Error("API_KEY is required — set via -key flag or API_KEY environment variable")
		os.Exit(1)
	}
	logger.Debug("configuration loaded", "key_len", len(*apiKey))

	s, err := store.New(*dataDir, logger)
	if err != nil {
		logger.Error("failed to open store", "dir", *dataDir, "err", err)
		os.Exit(1)
	}
	logger.Info("store ready", "dir", *dataDir)

	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		logger.Error("web embed failed", "err", err)
		os.Exit(1)
	}
	logger.Debug("web UI embedded and ready")

	mux := http.NewServeMux()
	mux.Handle("/api/", api.New(s, *apiKey, logger))
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	addr := ":" + *port
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("server listening", "addr", "http://localhost"+addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server stopped unexpectedly", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
