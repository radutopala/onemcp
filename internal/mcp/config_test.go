package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"log/slog"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigWithComments(t *testing.T) {
	// Create a temporary config file with comments
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".onemcp.json")

	configContent := `{
  // This is a comment
  "settings": {
    "searchResultLimit": 10,  // Another comment
    "searchProvider": "claude"   /* Block comment */
  },
  /* Multi-line
     comment */
  "mcpServers": {
    "test-server": {
      "command": "echo",
      "args": ["test"],
      "category": "test",
      "enabled": true
    }
  }
}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable to use our test config
	oldEnv := os.Getenv("ONEMCP_CONFIG")
	defer os.Setenv("ONEMCP_CONFIG", oldEnv)
	os.Setenv("ONEMCP_CONFIG", configPath)

	// Create aggregator server
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	server := &AggregatorServer{
		logger: logger,
	}

	// Load config
	config, err := server.loadConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify settings
	require.Equal(t, 10, config.Settings.SearchResultLimit)
	require.Equal(t, "claude", config.Settings.SearchProvider)

	// Verify servers
	require.Len(t, config.ExternalServers, 1)
	require.Contains(t, config.ExternalServers, "test-server")

	testServer := config.ExternalServers["test-server"]
	require.Equal(t, "echo", testServer.Command)
	require.Equal(t, []string{"test"}, testServer.Args)
	require.Equal(t, "test", testServer.Category)
	require.True(t, testServer.Enabled)
}

func TestLoadConfigWithoutComments(t *testing.T) {
	// Create a temporary config file without comments (standard JSON)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".onemcp.json")

	configContent := `{
  "settings": {
    "searchResultLimit": 15,
    "searchProvider": "codex",
    "codexModel": "gpt-5-codex-mini"
  },
  "mcpServers": {}
}`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable to use our test config
	oldEnv := os.Getenv("ONEMCP_CONFIG")
	defer os.Setenv("ONEMCP_CONFIG", oldEnv)
	os.Setenv("ONEMCP_CONFIG", configPath)

	// Create aggregator server
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	server := &AggregatorServer{
		logger: logger,
	}

	// Load config
	config, err := server.loadConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify settings
	require.Equal(t, 15, config.Settings.SearchResultLimit)
	require.Equal(t, "codex", config.Settings.SearchProvider)
	require.Equal(t, "gpt-5-codex-mini", config.Settings.CodexModel)

	// Verify no servers
	require.Len(t, config.ExternalServers, 0)
}

func TestLoadConfigMissingFile(t *testing.T) {
	// Set environment variable to non-existent file
	oldEnv := os.Getenv("ONEMCP_CONFIG")
	defer os.Setenv("ONEMCP_CONFIG", oldEnv)
	os.Setenv("ONEMCP_CONFIG", "/tmp/non-existent-config.json")

	// Create aggregator server
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	server := &AggregatorServer{
		logger: logger,
	}

	// Load config - should return empty config without error
	config, err := server.loadConfig()
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Equal(t, 0, config.Settings.SearchResultLimit)
}
