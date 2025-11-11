//go:build integration
// +build integration

package integration

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/radutopala/onemcp/internal/mcpclient"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// HTTPIntegrationTestSuite tests Streamable HTTP transport
type HTTPIntegrationTestSuite struct {
	suite.Suite
	server    *httptest.Server
	mcpServer *mcp.Server
	ctx       context.Context
	cancel    context.CancelFunc
}

// SetupSuite starts the HTTP test server
func (s *HTTPIntegrationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	// Create a test MCP server with some tools
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	s.mcpServer = mcp.NewServer(
		&mcp.Implementation{
			Name:    "test-http-server",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{
			Logger: logger,
		},
	)

	// Add a test tool
	type GreetInput struct {
		Name string `json:"name" jsonschema:"Name to greet"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "greet",
		Description: "Say hello to someone",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GreetInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Hello, " + input.Name + "!"},
			},
		}, nil, nil
	})

	// Create HTTP handler for Streamable HTTP transport
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	// Create test HTTP server
	s.server = httptest.NewServer(handler)
	s.T().Logf("Test HTTP server started at: %s", s.server.URL)
}

// TearDownSuite stops the HTTP test server
func (s *HTTPIntegrationTestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// TestStreamableHTTPConnection tests basic connection to HTTP server
func (s *HTTPIntegrationTestSuite) TestStreamableHTTPConnection() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := mcpclient.MCPServerConfig{
		URL:     s.server.URL,
		Enabled: true,
	}

	// Create MCP client
	client, err := mcpclient.NewMCPClient(s.ctx, "test-client", config, logger)
	require.NoError(s.T(), err, "Failed to create MCP client")
	require.NotNil(s.T(), client)
	defer client.Close()

	// Initialize the client
	err = client.Initialize(s.ctx)
	require.NoError(s.T(), err, "Failed to initialize client")
}

// TestStreamableHTTPListTools tests listing tools via HTTP
func (s *HTTPIntegrationTestSuite) TestStreamableHTTPListTools() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := mcpclient.MCPServerConfig{
		URL:     s.server.URL,
		Enabled: true,
	}

	// Create and initialize MCP client
	client, err := mcpclient.NewMCPClient(s.ctx, "test-client", config, logger)
	require.NoError(s.T(), err)
	defer client.Close()

	err = client.Initialize(s.ctx)
	require.NoError(s.T(), err)

	// List tools
	tools, err := client.ListTools(s.ctx)
	require.NoError(s.T(), err, "Failed to list tools")
	require.NotEmpty(s.T(), tools, "Should have at least one tool")

	// Verify the greet tool exists
	var foundGreet bool
	for _, tool := range tools {
		if tool.Name == "greet" {
			foundGreet = true
			require.Equal(s.T(), "Say hello to someone", tool.Description)
			require.NotNil(s.T(), tool.InputSchema)
		}
	}
	require.True(s.T(), foundGreet, "Should find greet tool")
}

// TestStreamableHTTPCallTool tests calling a tool via HTTP
func (s *HTTPIntegrationTestSuite) TestStreamableHTTPCallTool() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := mcpclient.MCPServerConfig{
		URL:     s.server.URL,
		Enabled: true,
	}

	// Create and initialize MCP client
	client, err := mcpclient.NewMCPClient(s.ctx, "test-client", config, logger)
	require.NoError(s.T(), err)
	defer client.Close()

	err = client.Initialize(s.ctx)
	require.NoError(s.T(), err)

	// Call the greet tool
	result, err := client.CallTool(s.ctx, "greet", map[string]any{
		"name": "World",
	})
	require.NoError(s.T(), err, "Failed to call tool")
	require.NotNil(s.T(), result)

	// Verify the result
	resultMap, ok := result.(map[string]any)
	require.True(s.T(), ok, "Result should be a map")
	require.Contains(s.T(), resultMap, "content", "Result should have content")
	require.Contains(s.T(), resultMap["content"], "Hello, World!")
}

// TestStreamableHTTPSchemaCache tests that schemas are cached
func (s *HTTPIntegrationTestSuite) TestStreamableHTTPSchemaCache() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := mcpclient.MCPServerConfig{
		URL:     s.server.URL,
		Enabled: true,
	}

	// Create and initialize MCP client
	client, err := mcpclient.NewMCPClient(s.ctx, "test-client", config, logger)
	require.NoError(s.T(), err)
	defer client.Close()

	err = client.Initialize(s.ctx)
	require.NoError(s.T(), err)

	// List tools (populates cache)
	tools, err := client.ListTools(s.ctx)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), tools)

	// Check cached schema
	schema, ok := client.GetCachedSchema("greet")
	require.True(s.T(), ok, "Schema should be cached")
	require.NotNil(s.T(), schema)
	require.Equal(s.T(), "object", schema["type"])
}

// TestStreamableHTTPInvalidEndpoint tests error handling for invalid endpoint
func (s *HTTPIntegrationTestSuite) TestStreamableHTTPInvalidEndpoint() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := mcpclient.MCPServerConfig{
		URL:     "http://localhost:99999/nonexistent", // Invalid port
		Enabled: true,
	}

	// Create MCP client - should fail to connect
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := mcpclient.NewMCPClient(ctx, "test-client", config, logger)
	// Should either fail to create or fail to initialize
	if err == nil && client != nil {
		err = client.Initialize(ctx)
		client.Close()
	}
	require.Error(s.T(), err, "Should fail to connect to invalid endpoint")
}

// TestHTTPIntegrationSuite runs the test suite
func TestHTTPIntegrationSuite(t *testing.T) {
	suite.Run(t, new(HTTPIntegrationTestSuite))
}
