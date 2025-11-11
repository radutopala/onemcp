package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/radutopala/onemcp/internal/tools"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// AggregatorServerTestSuite is the test suite for AggregatorServer
type AggregatorServerTestSuite struct {
	suite.Suite
	server *AggregatorServer
	ctx    context.Context
}

// SetupTest runs before each test
func (s *AggregatorServerTestSuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet during tests
	}))

	server, err := NewAggregatorServer("test-server", "1.0.0", logger)
	require.NoError(s.T(), err, "Failed to create test server")

	// Register test tools
	s.registerTestTools(server)

	// Initialize vector store after registering test tools
	err = server.initializeVectorStore()
	require.NoError(s.T(), err, "Failed to initialize vector store")

	s.server = server
	s.ctx = context.Background()
}

// registerTestTools adds test tools to the registry
func (s *AggregatorServerTestSuite) registerTestTools(server *AggregatorServer) {
	// Register test tools
	server.registry.Register(&tools.Tool{
		Name:        "test_tool_1",
		Category:    "test",
		Description: "First test tool",
		Source:      tools.SourceInternal,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param1": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "test1"}, nil
		},
	})

	server.registry.Register(&tools.Tool{
		Name:        "test_tool_2",
		Category:    "test",
		Description: "Second test tool",
		Source:      tools.SourceInternal,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"param2": map[string]any{"type": "number"},
			},
		},
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "test2"}, nil
		},
	})

	server.registry.Register(&tools.Tool{
		Name:        "another_category_tool",
		Category:    "other",
		Description: "Tool in another category",
		Source:      tools.SourceInternal,
		InputSchema: map[string]any{
			"type": "object",
		},
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "other"}, nil
		},
	})
}

// parseToolSearchResponse is a helper to parse tool_search responses
func (s *AggregatorServerTestSuite) parseToolSearchResponse(result *mcp.CallToolResult) map[string]any {
	require.NotNil(s.T(), result)
	require.Len(s.T(), result.Content, 1)

	text := result.Content[0].(*mcp.TextContent).Text
	var response map[string]any
	err := json.Unmarshal([]byte(text), &response)
	require.NoError(s.T(), err, "Failed to parse response")

	return response
}

// parseToolExecuteResponse is a helper to parse tool_execute responses
func (s *AggregatorServerTestSuite) parseToolExecuteResponse(result *mcp.CallToolResult) map[string]any {
	return s.parseToolSearchResponse(result)
}

// TestToolSearch_NamesOnly tests the names_only detail level
func (s *AggregatorServerTestSuite) TestToolSearch_NamesOnly() {
	input := ToolSearchInput{
		DetailLevel: "names_only",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Check total count includes at least the meta-tools
	totalCount := int(response["total_count"].(float64))
	require.GreaterOrEqual(s.T(), totalCount, 3, "Should have at least 3 meta-tools")

	// Verify tools array
	toolsArray := response["tools"].([]any)
	require.NotEmpty(s.T(), toolsArray, "Should have at least one tool")

	// Verify names_only format (no description)
	firstTool := toolsArray[0].(map[string]any)
	require.Contains(s.T(), firstTool, "name")
	require.Contains(s.T(), firstTool, "category")

	// Description should be empty string for names_only
	if desc, ok := firstTool["description"]; ok {
		require.Empty(s.T(), desc, "names_only should have empty description")
	}
}

// TestToolSearch_Summary tests the summary detail level
func (s *AggregatorServerTestSuite) TestToolSearch_Summary() {
	input := ToolSearchInput{
		DetailLevel: "summary",
		Category:    "test",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Should find our 2 test tools
	totalCount := int(response["total_count"].(float64))
	require.Equal(s.T(), 2, totalCount, "Should find 2 tools in 'test' category")

	// Verify summary format (has description, no parameters)
	toolsArray := response["tools"].([]any)
	require.Len(s.T(), toolsArray, 2)

	firstTool := toolsArray[0].(map[string]any)
	require.Contains(s.T(), firstTool, "description")
	require.NotEmpty(s.T(), firstTool["description"], "summary should include description")
	require.NotContains(s.T(), firstTool, "parameters", "summary should not include parameters")
}

// TestToolSearch_Detailed tests the detailed detail level
func (s *AggregatorServerTestSuite) TestToolSearch_Detailed() {
	input := ToolSearchInput{
		DetailLevel: "detailed",
		Query:       "test_tool_1",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// With semantic search, may find multiple similar tools
	totalCount := int(response["total_count"].(float64))
	require.GreaterOrEqual(s.T(), totalCount, 1, "Should find at least 1 tool matching 'test_tool_1'")

	// Verify detailed format (has parameters)
	toolsArray := response["tools"].([]any)
	require.Greater(s.T(), len(toolsArray), 0, "Should return at least one tool")
	firstTool := toolsArray[0].(map[string]any)
	require.Contains(s.T(), firstTool, "parameters", "detailed should include parameters")
}

// TestToolSearch_Pagination tests pagination functionality
func (s *AggregatorServerTestSuite) TestToolSearch_Pagination() {
	// First page
	input := ToolSearchInput{
		DetailLevel: "names_only",
		Offset:      0,
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Verify pagination fields with fixed limit of 5
	require.Equal(s.T(), float64(0), response["offset"])
	require.Equal(s.T(), float64(5), response["limit"], "Should use fixed limit of 5")
	require.LessOrEqual(s.T(), int(response["returned_count"].(float64)), 5, "Should return at most 5 tools")

	// Second page
	input.Offset = 2
	result, _, err = s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response = s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(2), response["offset"])
	require.Equal(s.T(), float64(5), response["limit"], "Should use fixed limit of 5")
}

// TestToolSearch_FixedLimit tests the fixed limit of 5
func (s *AggregatorServerTestSuite) TestToolSearch_FixedLimit() {
	input := ToolSearchInput{
		DetailLevel: "names_only",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(5), response["limit"], "Should use fixed limit of 5")
	require.LessOrEqual(s.T(), int(response["returned_count"].(float64)), 5, "Should return at most 5 tools")
}

// TestVectorStoreInitialization tests that vector store is initialized with tools
func (s *AggregatorServerTestSuite) TestVectorStoreInitialization() {
	// Verify vector store is initialized
	require.NotNil(s.T(), s.server.vectorStore, "Vector store should be initialized")

	// Verify vector store has indexed tools
	require.Greater(s.T(), s.server.vectorStore.GetToolCount(), 0, "Vector store should have indexed tools")
}

// TestToolSearch_IncludesSchemaFile tests that search response includes schema file info
func (s *AggregatorServerTestSuite) TestToolSearch_ReturnsResults() {
	input := ToolSearchInput{
		DetailLevel: "summary",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Verify response structure
	require.Contains(s.T(), response, "tools", "Response should contain tools")
	require.Contains(s.T(), response, "total_count", "Response should contain total_count")
	require.Contains(s.T(), response, "returned_count", "Response should contain returned_count")
}

// TestServerClose tests that server closes cleanly
func (s *AggregatorServerTestSuite) TestServerClose() {
	// Close the server
	err := s.server.Close()
	require.NoError(s.T(), err, "Close should not error")
}

// TestToolExecute tests successful tool execution
func (s *AggregatorServerTestSuite) TestToolExecute() {
	input := ToolExecuteInput{
		ToolName:  "test_tool_1",
		Arguments: map[string]any{"param1": "value1"},
	}

	result, _, err := s.server.handleToolExecute(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolExecuteResponse(result)

	// Verify execution result
	require.True(s.T(), response["success"].(bool), "Execution should succeed")
	require.Equal(s.T(), "test_tool_1", response["tool_name"])

	// Check the result contains our handler's response
	toolResult := response["result"].(map[string]any)
	require.Equal(s.T(), "test1", toolResult["result"])
}

// TestToolExecute_NotFound tests error handling for missing tools
func (s *AggregatorServerTestSuite) TestToolExecute_NotFound() {
	input := ToolExecuteInput{
		ToolName:  "nonexistent_tool",
		Arguments: map[string]any{},
	}

	result, _, err := s.server.handleToolExecute(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolExecuteResponse(result)

	// Should fail with tool not found
	require.False(s.T(), response["success"].(bool), "Execution should fail")
	require.Equal(s.T(), "tool_not_found", response["error_type"])
}

// TestAggregatorServerTestSuite runs the test suite
func TestAggregatorServerTestSuite(t *testing.T) {
	suite.Run(t, new(AggregatorServerTestSuite))
}
