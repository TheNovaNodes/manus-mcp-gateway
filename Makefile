.PHONY: build test test-race lint clean run

BINARY_NAME=manus-mcp-gateway
BIN_DIR=bin

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/manus-mcp-gateway

test:
	go test -v ./...

test-race:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

run: build
	./$(BIN_DIR)/$(BINARY_NAME)
