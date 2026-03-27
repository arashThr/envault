package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

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

	if *apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: API_KEY is required (-key flag or API_KEY env var)")
		os.Exit(1)
	}

	s, err := store.New(*dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	a := api.New(s, *apiKey)

	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("web embed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", a)
	mux.Handle("/", http.FileServer(http.FS(webRoot)))

	addr := ":" + *port
	log.Printf("envault server listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
