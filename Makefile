BIN_DIR   := bin
SERVER    := $(BIN_DIR)/envault-server
CLI       := $(BIN_DIR)/envault
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: all build ui server cli test clean

all: build

build: ui server cli

ui:
	cd ui && npm install && npm run build

server:
	@mkdir -p $(BIN_DIR)
	go build -o $(SERVER) ./cmd/envault-server

cli:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "-X main.version=$(VERSION)" -o $(CLI) ./cmd/envault

test:
	go test ./cmd/... ./internal/... -v -race -count=1

clean:
	rm -rf $(BIN_DIR)
