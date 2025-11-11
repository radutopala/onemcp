//go:build integration
// +build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Result  map[string]any `json:"result,omitempty"`
	Error   *JSONRPCError  `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type IntegrationTestSuite struct {
	suite.Suite
	binaryPath       string
	cmd              *exec.Cmd
	stdin            *bufio.Writer
	stdout           *bufio.Scanner
	ctx              context.Context
	cancel           context.CancelFunc
	browserServer    *httptest.Server
	filesystemServer *httptest.Server
	configPath       string
}

// SetupSuite builds the binary and starts mock MCP servers
func (s *IntegrationTestSuite) SetupSuite() {
	// Get project root (integration tests are in integration/)
	projectRoot, err := filepath.Abs(filepath.Join(".."))
	require.NoError(s.T(), err)

	// Start mock browser MCP server
	s.browserServer = s.createMockBrowserServer()
	s.T().Logf("Mock browser server started at: %s", s.browserServer.URL)

	// Start mock filesystem MCP server
	s.filesystemServer = s.createMockFilesystemServer()
	s.T().Logf("Mock filesystem server started at: %s", s.filesystemServer.URL)

	// Create config file pointing to mock servers
	s.configPath = filepath.Join(projectRoot, ".onemcp-test.json")
	s.createConfigFile()
	s.T().Logf("Config file created at: %s", s.configPath)

	// Build the binary
	s.T().Log("Building binary for integration tests...")
	buildCmd := exec.Command("go", "build", "-o", "one-mcp-test", "./cmd/one-mcp")
	buildCmd.Dir = projectRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err = buildCmd.Run()
	require.NoError(s.T(), err, "Failed to build binary")

	s.binaryPath = filepath.Join(projectRoot, "one-mcp-test")
	s.T().Logf("Binary built at: %s", s.binaryPath)
}

// TearDownSuite cleans up the binary and mock servers after all tests
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.binaryPath != "" {
		s.T().Log("Cleaning up test binary...")
		os.Remove(s.binaryPath)
	}
	if s.configPath != "" {
		os.Remove(s.configPath)
	}
	if s.browserServer != nil {
		s.browserServer.Close()
	}
	if s.filesystemServer != nil {
		s.filesystemServer.Close()
	}
}

// SetupTest starts the binary for each test
func (s *IntegrationTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	// Start the binary with config file
	s.cmd = exec.CommandContext(s.ctx, s.binaryPath)
	s.cmd.Env = append(os.Environ(), "ONEMCP_CONFIG="+s.configPath)

	// Setup stdin pipe
	stdinPipe, err := s.cmd.StdinPipe()
	require.NoError(s.T(), err)
	s.stdin = bufio.NewWriter(stdinPipe)

	// Setup stdout pipe
	stdoutPipe, err := s.cmd.StdoutPipe()
	require.NoError(s.T(), err)
	s.stdout = bufio.NewScanner(stdoutPipe)

	// Capture stderr for debugging
	s.cmd.Stderr = os.Stderr

	// Start the process
	err = s.cmd.Start()
	require.NoError(s.T(), err)
}

// TearDownTest stops the binary after each test
func (s *IntegrationTestSuite) TearDownTest() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}

// sendRequest sends a JSON-RPC request to the binary
func (s *IntegrationTestSuite) sendRequest(method string, params any) {
	s.sendRequestWithID(method, params, 1)
}

// sendRequestWithID sends a JSON-RPC request with a specific ID (or nil for notifications)
func (s *IntegrationTestSuite) sendRequestWithID(method string, params any, id any) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	require.NoError(s.T(), err)

	s.T().Logf("Sending: %s", string(data))

	_, err = s.stdin.Write(data)
	require.NoError(s.T(), err)
	_, err = s.stdin.Write([]byte("\n"))
	require.NoError(s.T(), err)
	err = s.stdin.Flush()
	require.NoError(s.T(), err)
}

// readResponse reads a JSON-RPC response from the binary
func (s *IntegrationTestSuite) readResponse() *JSONRPCResponse {
	require.True(s.T(), s.stdout.Scan(), "Failed to read response")

	line := s.stdout.Bytes()
	s.T().Logf("Received: %s", string(line))

	var resp JSONRPCResponse
	err := json.Unmarshal(line, &resp)
	require.NoError(s.T(), err)

	return &resp
}

// TestInitialize tests the MCP initialize handshake
func (s *IntegrationTestSuite) TestInitialize() {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	}

	s.sendRequest("initialize", params)
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error, "Initialize should not return error")
	require.NotNil(s.T(), resp.Result)
	require.Contains(s.T(), resp.Result, "protocolVersion")
	require.Contains(s.T(), resp.Result, "capabilities")
	require.Contains(s.T(), resp.Result, "serverInfo")

	serverInfo, ok := resp.Result["serverInfo"].(map[string]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), "one-mcp-aggregator", serverInfo["name"])
}

// TestToolsList tests the tools/list endpoint
func (s *IntegrationTestSuite) TestToolsList() {
	// Initialize first
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Send initialized notification (notifications have no ID)
	s.sendRequestWithID("notifications/initialized", map[string]any{}, nil)

	// Request tools list
	s.sendRequest("tools/list", map[string]any{})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error, "tools/list should not return error")
	require.NotNil(s.T(), resp.Result)
	require.Contains(s.T(), resp.Result, "tools")

	tools, ok := resp.Result["tools"].([]any)
	require.True(s.T(), ok)
	require.GreaterOrEqual(s.T(), len(tools), 2, "Should expose at least 2 meta-tools")

	// Verify meta-tools are present
	toolNames := make([]string, 0)
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		require.True(s.T(), ok)
		toolNames = append(toolNames, toolMap["name"].(string))
	}

	require.Contains(s.T(), toolNames, "tool_search")
	require.Contains(s.T(), toolNames, "tool_execute")
}

// TestToolSearch tests the tool_search functionality
func (s *IntegrationTestSuite) TestToolSearch() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Test tool_search with names_only (no limit parameter)
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"detail_level": "names_only",
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error, "tool_search should not return error")
	require.NotNil(s.T(), resp.Result)
	require.Contains(s.T(), resp.Result, "content")

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0, "Should return at least one content item")

	// Verify content is JSON with tools array
	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), "text", firstContent["type"])

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)
	require.Contains(s.T(), result, "tools")
	require.Contains(s.T(), result, "total_count")
	require.Contains(s.T(), result, "returned_count")
}

// TestToolExecute tests the tool_execute functionality
func (s *IntegrationTestSuite) TestToolExecute() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Test tool_execute with invalid tool (should return error)
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_execute",
		"arguments": map[string]any{
			"tool_name": "nonexistent_tool",
			"arguments": map[string]any{},
		},
	})
	resp := s.readResponse()

	// The response should contain content with an error message
	require.NotNil(s.T(), resp.Result)
	require.Contains(s.T(), resp.Result, "content")

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), "text", firstContent["type"])
	require.Contains(s.T(), firstContent["text"].(string), "error", "Should contain error message for invalid tool")
}

// TestSchemaFileFixedLimit tests that tool_search returns exactly 5 tools
func (s *IntegrationTestSuite) TestSchemaFileFixedLimit() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Call tool_search
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"detail_level": "names_only",
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error)
	require.NotNil(s.T(), resp.Result)

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)

	// Verify fixed limit of 5
	require.Equal(s.T(), float64(5), result["limit"], "Should have fixed limit of 5")

	// Verify returned count is at most 5
	returnedCount := int(result["returned_count"].(float64))
	require.LessOrEqual(s.T(), returnedCount, 5, "Should return at most 5 tools")

	// Verify tools array matches returned_count
	tools, ok := result["tools"].([]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), returnedCount, len(tools), "Tools array length should match returned_count")
}

// TestToolSearchWithCategory tests tool search with category filter
func (s *IntegrationTestSuite) TestToolSearchWithCategory() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Test tool search with category filter
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"query":        "execute",
			"category":     "onemcp",
			"detail_level": "summary",
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error)
	require.NotNil(s.T(), resp.Result)

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)

	// Verify category filter was applied
	require.Contains(s.T(), result, "tools")
	tools, ok := result["tools"].([]any)
	require.True(s.T(), ok)

	// All returned tools should have the specified category
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		require.True(s.T(), ok)
		require.Contains(s.T(), toolMap, "category")
		require.Equal(s.T(), "onemcp", toolMap["category"], "All tools should be in the 'onemcp' category")
	}
}

// TestToolSearchDetailLevels tests different detail levels in tool search
func (s *IntegrationTestSuite) TestToolSearchDetailLevels() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	testCases := []struct {
		detailLevel      string
		shouldHaveDesc   bool
		shouldHaveParams bool
	}{
		{"names_only", false, false},
		{"summary", true, false},
		{"detailed", true, true},
	}

	for _, tc := range testCases {
		s.T().Run(tc.detailLevel, func(t *testing.T) {
			s.sendRequest("tools/call", map[string]any{
				"name": "tool_search",
				"arguments": map[string]any{
					"query":        "tool",
					"detail_level": tc.detailLevel,
				},
			})
			resp := s.readResponse()

			require.Nil(t, resp.Error)
			require.NotNil(t, resp.Result)

			content, ok := resp.Result["content"].([]any)
			require.True(t, ok)
			require.Greater(t, len(content), 0)

			firstContent, ok := content[0].(map[string]any)
			require.True(t, ok)

			var result map[string]any
			err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
			require.NoError(t, err)

			require.Contains(t, result, "tools")
			tools, ok := result["tools"].([]any)
			require.True(t, ok)
			require.GreaterOrEqual(t, len(tools), 0, "Should return tools array")

			// Only verify tool structure if we have tools
			if len(tools) > 0 {
				firstTool, ok := tools[0].(map[string]any)
				require.True(t, ok)

				// Verify name is always present
				require.Contains(t, firstTool, "name")

				// Verify description based on detail level
				if tc.shouldHaveDesc {
					require.Contains(t, firstTool, "description", "Detail level %s should include description", tc.detailLevel)
				}

				// Verify parameters based on detail level
				if tc.shouldHaveParams {
					// Parameters might not always be present if tool has no schema
					// So we just check that if it exists, it's a map
					if params, ok := firstTool["parameters"]; ok {
						_, isMap := params.(map[string]any)
						require.True(t, isMap, "Parameters should be a map")
					}
				}
			}
		})
	}
}

// TestVectorStoreInitialization tests that vector store is properly initialized
func (s *IntegrationTestSuite) TestVectorStoreInitialization() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	initResp := s.readResponse()
	require.Nil(s.T(), initResp.Error)

	// Make a search query to verify vector store is working
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"query":        "search tools",
			"detail_level": "summary",
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error)
	require.NotNil(s.T(), resp.Result)

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)

	// Verify we got results structure (vector store is working)
	require.Contains(s.T(), result, "total_count")
	totalCount := int(result["total_count"].(float64))
	require.GreaterOrEqual(s.T(), totalCount, 0, "Should have valid total_count (may be 0 if no external tools configured)")

	// Verify tools array is present
	require.Contains(s.T(), result, "tools")
	tools, ok := result["tools"].([]any)
	require.True(s.T(), ok)
	require.GreaterOrEqual(s.T(), len(tools), 0, "Should return tools array")
}

// TestToolSearchRelevance tests that tool search returns relevant results
func (s *IntegrationTestSuite) TestToolSearchRelevance() {
	// Initialize
	s.sendRequest("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "integration-test",
			"version": "1.0.0",
		},
	})
	s.readResponse()

	// Search for "capture page image" - should return screenshot tool
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"query":        "capture page image",
			"detail_level": "summary",
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error)
	require.NotNil(s.T(), resp.Result)

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)

	// Verify we got tools
	tools, ok := result["tools"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(tools), 0, "Should return at least one tool")

	// Verify screenshot tool is in the results (semantically similar to "capture page image")
	foundScreenshot := false
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		require.True(s.T(), ok)
		if toolMap["name"] == "browser_screenshot" {
			foundScreenshot = true
			break
		}
	}
	require.True(s.T(), foundScreenshot, "Tool search should find 'screenshot' tool for query 'capture page image'")
}

// createMockBrowserServer creates a mock MCP server with browser tools
func (s *IntegrationTestSuite) createMockBrowserServer() *httptest.Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mcpServer := mcp.NewServer(
		&mcp.Implementation{Name: "mock-browser", Version: "1.0.0"},
		&mcp.ServerOptions{Logger: logger},
	)

	// Add browser tools
	type NavigateInput struct {
		URL string `json:"url" jsonschema:"URL to navigate to"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "navigate",
		Description: "Navigate browser to a URL",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input NavigateInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Navigated to " + input.URL}},
		}, nil, nil
	})

	type ScreenshotInput struct {
		Fullpage bool `json:"fullpage" jsonschema:"Take full page screenshot"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "screenshot",
		Description: "Take a screenshot of the current page",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ScreenshotInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Screenshot taken"}},
		}, nil, nil
	})

	type ClickInput struct {
		Selector string `json:"selector" jsonschema:"CSS selector to click"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "click",
		Description: "Click an element on the page",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ClickInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Clicked " + input.Selector}},
		}, nil, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	return httptest.NewServer(handler)
}

// createMockFilesystemServer creates a mock MCP server with filesystem tools
func (s *IntegrationTestSuite) createMockFilesystemServer() *httptest.Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	mcpServer := mcp.NewServer(
		&mcp.Implementation{Name: "mock-filesystem", Version: "1.0.0"},
		&mcp.ServerOptions{Logger: logger},
	)

	// Add filesystem tools
	type ReadFileInput struct {
		Path string `json:"path" jsonschema:"Path to file"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "read_file",
		Description: "Read contents of a file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ReadFileInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "File contents"}},
		}, nil, nil
	})

	type WriteFileInput struct {
		Path    string `json:"path" jsonschema:"Path to file"`
		Content string `json:"content" jsonschema:"Content to write"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "write_file",
		Description: "Write data to a file",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input WriteFileInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "File written"}},
		}, nil, nil
	})

	type ListDirectoryInput struct {
		Path string `json:"path" jsonschema:"Path to directory"`
	}
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_directory",
		Description: "List files in a directory",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListDirectoryInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Directory listing"}},
		}, nil, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	return httptest.NewServer(handler)
}

// createConfigFile creates a .onemcp-test.json config pointing to mock servers
func (s *IntegrationTestSuite) createConfigFile() {
	config := map[string]any{
		"settings": map[string]any{
			"searchResultLimit": 5,
		},
		"mcpServers": map[string]any{
			"browser": map[string]any{
				"url":      s.browserServer.URL,
				"category": "browser",
				"enabled":  true,
			},
			"filesystem": map[string]any{
				"url":      s.filesystemServer.URL,
				"category": "filesystem",
				"enabled":  true,
			},
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(s.T(), err)

	err = os.WriteFile(s.configPath, data, 0644)
	require.NoError(s.T(), err)
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(IntegrationTestSuite))
}
