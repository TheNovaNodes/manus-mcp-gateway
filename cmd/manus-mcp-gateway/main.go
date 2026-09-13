package main

import (
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
// It supports multiline values enclosed in double or single quotes.
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	var currentKey string
	var currentValue strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if !inQuote {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			parts := strings.SplitN(rawLine, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				if (strings.HasPrefix(val, "\"") || strings.HasPrefix(val, "'")) &&
					!(len(val) >= 2 && val[0] == val[len(val)-1] && !strings.HasSuffix(val, `\`+string(val[0]))) {
					inQuote = true
					quoteChar = val[0]
					currentKey = key
					currentValue.Reset()
					currentValue.WriteString(val[1:])
					currentValue.WriteString("\n")
				} else {
					val = strings.Trim(val, `"'`)
					if os.Getenv(key) == "" {
						_ = os.Setenv(key, val)
					}
				}
			}
		} else {
			if strings.HasSuffix(trimmed, string(quoteChar)) {
				inQuote = false
				lineWithoutQuote := strings.TrimSuffix(trimmed, string(quoteChar))
				currentValue.WriteString(lineWithoutQuote)
				if os.Getenv(currentKey) == "" {
					_ = os.Setenv(currentKey, currentValue.String())
				}
				currentValue.Reset()
			} else {
				currentValue.WriteString(rawLine)
				currentValue.WriteString("\n")
			}
		}
	}
}

func main() {
	if envFile := os.Getenv("MANUS_ENV_FILE"); envFile != "" {
		loadDotEnv(envFile)
	}
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
	if rawKeys == "" {
		if filePath := os.Getenv("MANUS_KEYS_FILE"); filePath != "" {
			if strings.HasSuffix(filePath, ".env") {
				loadDotEnv(filePath)
				rawKeys = os.Getenv("MANUS_KEYS")
			}
			if rawKeys == "" {
				data, err := os.ReadFile(filePath)
				if err == nil {
					rawKeys = string(data)
				} else {
					logger.Error(fmt.Sprintf("Failed to read MANUS_KEYS_FILE (%s): %v", filePath, err))
				}
			}
		}
	}
	keyEntries := pool.ParseKeys(rawKeys)
	if len(keyEntries) == 0 {
		logger.Warn("MANUS_KEYS environment variable is empty or contains no valid keys. Tools requiring API keys will return errors.")
	} else {
		logger.Info(fmt.Sprintf("Initialized Manus Capacity Pool with %d configured keys", len(keyEntries)))
	}

	defaultProfile := os.Getenv("MANUS_DEFAULT_PROFILE")
	if defaultProfile == "" {
		defaultProfile = "max"
	}
	logger.Info(fmt.Sprintf("Default agent profile configured as '%s'", defaultProfile))

	var defaultConnectors []string
	if rawConnectors := os.Getenv("MANUS_DEFAULT_CONNECTORS"); rawConnectors != "" {
		parts := strings.Split(rawConnectors, ",")
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				defaultConnectors = append(defaultConnectors, trimmed)
			}
		}
		if len(defaultConnectors) > 0 {
			logger.Info(fmt.Sprintf("Default Custom MCP connectors configured: %s", strings.Join(defaultConnectors, ", ")))
		}
	}

	keyPool := pool.NewPool(client, keyEntries, 3*time.Minute)
	srv := server.NewServer(keyPool, client, logger, defaultProfile, defaultConnectors)

	logger.Info("Starting manus-mcp-gateway (stdio transport)...")
	if err := mcpserver.ServeStdio(srv.MCPServer()); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: MCP Server terminated with error: %v\n", err)
		os.Exit(1)
	}
}
