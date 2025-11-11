package main

import (
	"context"
	"log/slog"
	"os"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/radutopala/onemcp/internal/mcp"
)

func main() {
	// Create log file
	logPath := os.Getenv("MCP_LOG_FILE")
	if logPath == "" {
		logPath = "/tmp/one-mcp-server.log"
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fallback to stderr if we can't open the log file
		logFile = os.Stderr
	} else {
		defer logFile.Close()
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	// Get server name and version from environment or use defaults
	serverName := os.Getenv("MCP_SERVER_NAME")
	if serverName == "" {
		serverName = "one-mcp-aggregator"
	}

	serverVersion := os.Getenv("MCP_SERVER_VERSION")
	if serverVersion == "" {
		serverVersion = "0.1.0"
	}

	// Initialize MCP Aggregator Server
	mcpServer, err := mcp.NewAggregatorServer(serverName, serverVersion, logger)
	if err != nil {
		logger.Error("Failed to create OneMCP aggregator server", "error", err)
		os.Exit(1)
	}
	defer mcpServer.Close()

	// Start serving over stdio
	logger.Info("Starting OneMCP aggregator server over stdio...", "name", serverName, "version", serverVersion)
	if err := mcpServer.Run(ctx, &mcpsdk.StdioTransport{}); err != nil {
		logger.Error("OneMCP aggregator server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("OneMCP aggregator server finished")
}
