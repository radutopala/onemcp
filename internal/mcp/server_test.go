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
		Limit:       2,
		Offset:      0,
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)

	// Verify pagination fields
	require.Equal(s.T(), float64(0), response["offset"])
	require.Equal(s.T(), float64(2), response["limit"])
	require.Equal(s.T(), 2, int(response["returned_count"].(float64)))
	require.True(s.T(), response["has_more"].(bool), "Should have more results")

	// Second page
	input.Offset = 2
	result, _, err = s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response = s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(2), response["offset"])
}

// TestToolSearch_DefaultLimit tests the default limit
func (s *AggregatorServerTestSuite) TestToolSearch_DefaultLimit() {
	input := ToolSearchInput{
		DetailLevel: "names_only",
		// No limit specified, should default to 50
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(50), response["limit"], "Should use default limit of 50")
}

// TestToolSearch_MaxLimit tests the maximum limit cap
func (s *AggregatorServerTestSuite) TestToolSearch_MaxLimit() {
	input := ToolSearchInput{
		DetailLevel: "names_only",
		Limit:       300, // More than max of 200
	}

	result, _, err := s.server.handleToolSearch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolSearchResponse(result)
	require.Equal(s.T(), float64(200), response["limit"], "Should cap limit at 200")
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

// TestToolExecuteBatch tests batch execution
func (s *AggregatorServerTestSuite) TestToolExecuteBatch() {
	input := ToolExecuteBatchInput{
		Tools: []tools.ToolExecution{
			{
				ToolName:  "test_tool_1",
				Arguments: map[string]any{"param1": "value1"},
			},
			{
				ToolName:  "test_tool_2",
				Arguments: map[string]any{"param2": 42},
			},
		},
		ContinueOnError: false,
	}

	result, _, err := s.server.handleToolExecuteBatch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolExecuteResponse(result)

	// Verify batch results
	results := response["results"].([]any)
	require.Len(s.T(), results, 2)
	require.Equal(s.T(), float64(2), response["successful_count"])
	require.Equal(s.T(), float64(0), response["failed_count"])
}

// TestToolExecuteBatch_WithFailure tests continue_on_error behavior
func (s *AggregatorServerTestSuite) TestToolExecuteBatch_WithFailure() {
	input := ToolExecuteBatchInput{
		Tools: []tools.ToolExecution{
			{
				ToolName:  "test_tool_1",
				Arguments: map[string]any{"param1": "value1"},
			},
			{
				ToolName:  "nonexistent_tool",
				Arguments: map[string]any{},
			},
			{
				ToolName:  "test_tool_2",
				Arguments: map[string]any{"param2": 42},
			},
		},
		ContinueOnError: true, // Continue despite failure
	}

	result, _, err := s.server.handleToolExecuteBatch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolExecuteResponse(result)

	// Verify batch results with failure
	results := response["results"].([]any)
	require.Len(s.T(), results, 3)
	require.Equal(s.T(), float64(2), response["successful_count"])
	require.Equal(s.T(), float64(1), response["failed_count"])

	// Check that second result is the failure
	secondResult := results[1].(map[string]any)
	require.False(s.T(), secondResult["success"].(bool), "Second execution should fail")
}

// TestToolExecuteBatch_StopOnError tests stop-on-error behavior
func (s *AggregatorServerTestSuite) TestToolExecuteBatch_StopOnError() {
	input := ToolExecuteBatchInput{
		Tools: []tools.ToolExecution{
			{
				ToolName:  "test_tool_1",
				Arguments: map[string]any{"param1": "value1"},
			},
			{
				ToolName:  "nonexistent_tool",
				Arguments: map[string]any{},
			},
			{
				ToolName:  "test_tool_2",
				Arguments: map[string]any{"param2": 42},
			},
		},
		ContinueOnError: false, // Stop on first failure
	}

	result, _, err := s.server.handleToolExecuteBatch(s.ctx, nil, input)
	require.NoError(s.T(), err)

	response := s.parseToolExecuteResponse(result)

	// Should stop after second tool fails
	results := response["results"].([]any)
	require.Len(s.T(), results, 2, "Should stop after failure")
	require.Equal(s.T(), float64(1), response["successful_count"])
	require.Equal(s.T(), float64(1), response["failed_count"])
}

// TestAggregatorServerTestSuite runs the test suite
func TestAggregatorServerTestSuite(t *testing.T) {
	suite.Run(t, new(AggregatorServerTestSuite))
}
