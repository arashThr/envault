BIN_DIR   := bin
SERVER    := $(BIN_DIR)/envault-server
CLI       := $(BIN_DIR)/envault

.PHONY: all build ui server cli test clean

all: build

build: ui server cli

ui:
	cd ui && npm install && npm run build

server:
	@mkdir -p $(BIN_DIR)
	go build -o $(SERVER) ./cmd/server

cli:
	@mkdir -p $(BIN_DIR)
	go build -o $(CLI) ./cmd/envault

test:
	go test ./cmd/... ./internal/... -v -race -count=1

clean:
	rm -rf $(BIN_DIR)
