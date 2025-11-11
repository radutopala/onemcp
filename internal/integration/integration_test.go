//go:build integration
// +build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
	binaryPath string
	cmd        *exec.Cmd
	stdin      *bufio.Writer
	stdout     *bufio.Scanner
	ctx        context.Context
	cancel     context.CancelFunc
}

// SetupSuite builds the binary before running tests
func (s *IntegrationTestSuite) SetupSuite() {
	// Get project root (integration tests are in internal/integration/)
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(s.T(), err)

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

// TearDownSuite cleans up the binary after all tests
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.binaryPath != "" {
		s.T().Log("Cleaning up test binary...")
		os.Remove(s.binaryPath)
	}
}

// SetupTest starts the binary for each test
func (s *IntegrationTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Second)

	// Start the binary
	s.cmd = exec.CommandContext(s.ctx, s.binaryPath)

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
	require.GreaterOrEqual(s.T(), len(tools), 3, "Should expose at least 3 meta-tools")

	// Verify meta-tools are present
	toolNames := make([]string, 0)
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		require.True(s.T(), ok)
		toolNames = append(toolNames, toolMap["name"].(string))
	}

	require.Contains(s.T(), toolNames, "tool_search")
	require.Contains(s.T(), toolNames, "tool_execute")
	require.Contains(s.T(), toolNames, "tool_execute_batch")
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

	// Test tool_search with names_only
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_search",
		"arguments": map[string]any{
			"detail_level": "names_only",
			"limit":        10,
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

// TestToolExecuteBatch tests the tool_execute_batch functionality
func (s *IntegrationTestSuite) TestToolExecuteBatch() {
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

	// Test tool_execute_batch with multiple tool_search calls
	s.sendRequest("tools/call", map[string]any{
		"name": "tool_execute_batch",
		"arguments": map[string]any{
			"tools": []map[string]any{
				{
					"tool_name": "tool_search",
					"arguments": map[string]any{
						"detail_level": "names_only",
						"limit":        5,
					},
				},
				{
					"tool_name": "tool_search",
					"arguments": map[string]any{
						"detail_level": "summary",
						"limit":        3,
					},
				},
			},
			"continue_on_error": true,
		},
	})
	resp := s.readResponse()

	require.Nil(s.T(), resp.Error, "tool_execute_batch should not return error")
	require.NotNil(s.T(), resp.Result)
	require.Contains(s.T(), resp.Result, "content")

	content, ok := resp.Result["content"].([]any)
	require.True(s.T(), ok)
	require.Greater(s.T(), len(content), 0)

	firstContent, ok := content[0].(map[string]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), "text", firstContent["type"])

	var result map[string]any
	err := json.Unmarshal([]byte(firstContent["text"].(string)), &result)
	require.NoError(s.T(), err)
	require.Contains(s.T(), result, "results")

	results, ok := result["results"].([]any)
	require.True(s.T(), ok)
	require.Equal(s.T(), 2, len(results), "Should have 2 results for 2 operations")
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(IntegrationTestSuite))
}
