package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/TheNovaNodes/manus-mcp-gateway/internal/manus"
	"github.com/TheNovaNodes/manus-mcp-gateway/internal/pool"
	"github.com/TheNovaNodes/manus-mcp-gateway/internal/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// loadDotEnv parses a local .env file if present without external dependencies.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}

func main() {
	loadDotEnv(".env")

	// FastMCP stdio server logs to stderr so stdin/stdout are preserved for MCP JSON-RPC
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	baseURL := os.Getenv("MANUS_BASE_URL")
	if baseURL == "" {
		baseURL = manus.DefaultBaseURL
	}

	client := manus.NewClient(
		manus.WithBaseURL(baseURL),
	)

	rawKeys := os.Getenv("MANUS_KEYS")
	keyEntries := pool.ParseKeys(rawKeys)
	if len(keyEntries) == 0 {
		logger.Warn("MANUS_KEYS environment variable is empty or contains no valid keys. Tools requiring API keys will return errors.")
	} else {
		logger.Info(fmt.Sprintf("Initialized Manus Capacity Pool with %d configured keys", len(keyEntries)))
	}

	keyPool := pool.NewPool(client, keyEntries, 3*time.Minute)
	srv := server.NewServer(keyPool, client, logger)

	logger.Info("Starting manus-mcp-gateway (stdio transport)...")
	if err := mcpserver.ServeStdio(srv.MCPServer()); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: MCP Server terminated with error: %v\n", err)
		os.Exit(1)
	}
}
