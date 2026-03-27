BIN_DIR   := bin
SERVER    := $(BIN_DIR)/envault-server
CLI       := $(BIN_DIR)/envault

.PHONY: all build server cli test clean

all: build

build: server cli

server:
	@mkdir -p $(BIN_DIR)
	go build -o $(SERVER) ./cmd/server

cli:
	@mkdir -p $(BIN_DIR)
	go build -o $(CLI) ./cmd/envault

test:
	go test ./... -v -race -count=1

clean:
	rm -rf $(BIN_DIR)
