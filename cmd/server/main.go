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
	port    := flag.String("port", envOr("PORT", "8080"), "listen port")
	dataDir := flag.String("data", envOr("DATA_DIR", "./data"), "directory to store env files")
	apiKey  := flag.String("key",  envOr("API_KEY", ""),  "API key for authentication")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if *apiKey == "" {
		logger.Error("API_KEY is required", "hint", "use -key flag or API_KEY env var")
		os.Exit(1)
	}

	s, err := store.New(*dataDir, logger)
	if err != nil {
		logger.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	logger.Info("store ready", "dir", *dataDir)

	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		logger.Error("web embed failed", "err", err)
		os.Exit(1)
	}

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

	logger.Info("envault server starting", "addr", "http://localhost"+addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server stopped", "err", err)
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
