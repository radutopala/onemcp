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

	server, err := NewAggregatorServer("test-server", "0.1.0", logger)
	require.NoError(s.T(), err, "Failed to create test server")

	// Register test tools
	s.registerTestTools(server)

	// Regenerate schema file after registering test tools
	err = server.generateSchemaFile()
	require.NoError(s.T(), err, "Failed to regenerate schema file")

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

	// Should find exactly 1 tool
	totalCount := int(response["total_count"].(float64))
	require.Equal(s.T(), 1, totalCount, "Should find 1 tool matching 'test_tool_1'")

	// Verify detailed format (has parameters)
	toolsArray := response["tools"].([]any)
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

	// Verify pagination fields with fixed limit of 2
	require.Equal(s.T(), float64(0), response["offset"])
	require.Equal(s.T(), float64(2), response["limit"], "Should use fixed limit of 2")
	require.LessOrEqual(s.T(), int(response["returned_count"].(float64)), 2, "Should return at most 2 tools")

	// Second page
	input.Offset = 2
	result, _, err = s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response = s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(2), response["offset"])
	require.Equal(s.T(), float64(2), response["limit"], "Should use fixed limit of 2")
}

// TestToolSearch_FixedLimit tests the fixed limit of 2
func (s *AggregatorServerTestSuite) TestToolSearch_FixedLimit() {
	input := ToolSearchInput{
		DetailLevel: "names_only",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(2), response["limit"], "Should use fixed limit of 2")
	require.LessOrEqual(s.T(), int(response["returned_count"].(float64)), 2, "Should return at most 2 tools")
}

// TestSchemaFileGeneration tests that schema file is created and contains all tools
func (s *AggregatorServerTestSuite) TestSchemaFileGeneration() {
	// Verify schemaFilePath is set
	require.NotEmpty(s.T(), s.server.schemaFilePath, "Schema file path should be set")

	// Verify file exists
	_, err := os.Stat(s.server.schemaFilePath)
	require.NoError(s.T(), err, "Schema file should exist")

	// Read and parse the file
	data, err := os.ReadFile(s.server.schemaFilePath)
	require.NoError(s.T(), err, "Should be able to read schema file")

	var toolSchemas []tools.ToolMetadata
	err = json.Unmarshal(data, &toolSchemas)
	require.NoError(s.T(), err, "Schema file should contain valid JSON")

	// Verify file contains tools (at least our test tools, excluding meta-tools)
	require.GreaterOrEqual(s.T(), len(toolSchemas), 3, "Schema file should contain at least 3 test tools")

	// Verify each tool has required fields
	for _, tool := range toolSchemas {
		require.NotEmpty(s.T(), tool.Name, "Each tool should have a name")
		require.NotEmpty(s.T(), tool.Category, "Each tool should have a category")
		require.NotEmpty(s.T(), tool.Description, "Each tool should have a description")
		// Parameters field is optional (can be nil for tools without parameters)
	}
}

// TestToolSearch_IncludesSchemaFile tests that search response includes schema file info
func (s *AggregatorServerTestSuite) TestToolSearch_IncludesSchemaFile() {
	input := ToolSearchInput{
		DetailLevel: "summary",
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Verify schema_file field is present
	require.Contains(s.T(), response, "schema_file", "Response should contain schema_file field")
	schemaFile, ok := response["schema_file"].(string)
	require.True(s.T(), ok, "schema_file should be a string")
	require.NotEmpty(s.T(), schemaFile, "schema_file should not be empty")
	require.Equal(s.T(), s.server.schemaFilePath, schemaFile, "schema_file should match server's schemaFilePath")

	// Verify message field is present
	require.Contains(s.T(), response, "message", "Response should contain message field")
	message, ok := response["message"].(string)
	require.True(s.T(), ok, "message should be a string")
	require.Contains(s.T(), message, schemaFile, "Message should mention the schema file path")
}

// TestSchemaFileCleanup tests that schema file is removed on server close
func (s *AggregatorServerTestSuite) TestSchemaFileCleanup() {
	schemaFilePath := s.server.schemaFilePath

	// Verify file exists before close
	_, err := os.Stat(schemaFilePath)
	require.NoError(s.T(), err, "Schema file should exist before close")

	// Close the server
	err = s.server.Close()
	require.NoError(s.T(), err, "Close should not error")

	// Verify file is removed after close
	_, err = os.Stat(schemaFilePath)
	require.True(s.T(), os.IsNotExist(err), "Schema file should be removed after close")
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
