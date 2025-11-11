package tools

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MockExternalExecutor implements ExternalToolExecutor for testing
type MockExternalExecutor struct {
	callToolFunc func(ctx context.Context, toolName string, arguments map[string]any) (any, error)
}

func (m *MockExternalExecutor) CallTool(ctx context.Context, toolName string, arguments map[string]any) (any, error) {
	if m.callToolFunc != nil {
		return m.callToolFunc(ctx, toolName, arguments)
	}
	return map[string]any{"result": "mock_result"}, nil
}

// RegistryTestSuite is the test suite for Registry
type RegistryTestSuite struct {
	suite.Suite
	registry *Registry
	ctx      context.Context
}

// SetupTest runs before each test
func (s *RegistryTestSuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet during tests
	}))

	s.registry = NewRegistry(logger)
	s.ctx = context.Background()
}

// TestNewRegistry tests registry creation
func (s *RegistryTestSuite) TestNewRegistry() {
	require.NotNil(s.T(), s.registry)
	require.NotNil(s.T(), s.registry.tools)
	require.NotNil(s.T(), s.registry.externalExecutors)
	require.NotNil(s.T(), s.registry.logger)
}

// TestRegister tests tool registration
func (s *RegistryTestSuite) TestRegister() {
	tool := &Tool{
		Name:        "test_tool",
		Category:    "test",
		Description: "Test tool",
		Source:      SourceInternal,
		InputSchema: map[string]any{"type": "object"},
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "success"}, nil
		},
	}

	err := s.registry.Register(tool)
	require.NoError(s.T(), err)

	// Verify tool is registered
	registered, err := s.registry.Get("test_tool")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "test_tool", registered.Name)
}

// TestRegister_EmptyName tests registration with empty name
func (s *RegistryTestSuite) TestRegister_EmptyName() {
	tool := &Tool{
		Name:     "",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}

	err := s.registry.Register(tool)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "tool name cannot be empty")
}

// TestRegister_NilHandler tests registration of internal tool without handler
func (s *RegistryTestSuite) TestRegister_NilHandler() {
	tool := &Tool{
		Name:     "test_tool",
		Category: "test",
		Source:   SourceInternal,
		Handler:  nil,
	}

	err := s.registry.Register(tool)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "tool handler cannot be nil for internal tools")
}

// TestRegister_Duplicate tests duplicate tool registration
func (s *RegistryTestSuite) TestRegister_Duplicate() {
	tool := &Tool{
		Name:     "test_tool",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}

	err := s.registry.Register(tool)
	require.NoError(s.T(), err)

	// Try to register again
	err = s.registry.Register(tool)
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "already registered")
}

// TestGet tests tool retrieval
func (s *RegistryTestSuite) TestGet() {
	tool := &Tool{
		Name:     "test_tool",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}

	s.registry.Register(tool)

	retrieved, err := s.registry.Get("test_tool")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "test_tool", retrieved.Name)
}

// TestGet_NotFound tests getting non-existent tool
func (s *RegistryTestSuite) TestGet_NotFound() {
	_, err := s.registry.Get("nonexistent")
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "tool not found")
}

// TestRegisterExternalExecutor tests external executor registration
func (s *RegistryTestSuite) TestRegisterExternalExecutor() {
	executor := &MockExternalExecutor{}
	s.registry.RegisterExternalExecutor("test_source", executor)

	registered, ok := s.registry.externalExecutors["test_source"]
	require.True(s.T(), ok)
	require.NotNil(s.T(), registered)
}

// TestRegisterExternalTool tests external tool registration
func (s *RegistryTestSuite) TestRegisterExternalTool() {
	err := s.registry.RegisterExternalTool(
		"test_server",
		"test",
		"my_tool",
		"Test external tool",
		map[string]any{"type": "object"},
	)
	require.NoError(s.T(), err)

	// Tool should be registered with prefix
	tool, err := s.registry.Get("test_server_my_tool")
	require.NoError(s.T(), err)
	require.Equal(s.T(), "test_server_my_tool", tool.Name)
	require.Equal(s.T(), SourceExternal, tool.Source)
	require.Equal(s.T(), "test_server", tool.SourceName)
}

// TestSearch tests tool search
// TestExecute_Internal tests internal tool execution
func (s *RegistryTestSuite) TestExecute_Internal() {
	tool := &Tool{
		Name:     "test_tool",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "success", "input": params["value"]}, nil
		},
	}

	s.registry.Register(tool)

	result, err := s.registry.Execute(s.ctx, "test_tool", map[string]any{"value": "test"})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Success)
	require.Equal(s.T(), "test_tool", result.ToolName)
	require.Equal(s.T(), "success", result.Result["result"])
	require.Equal(s.T(), "test", result.Result["input"])
}

// TestExecute_NotFound tests execution of non-existent tool
func (s *RegistryTestSuite) TestExecute_NotFound() {
	result, err := s.registry.Execute(s.ctx, "nonexistent", map[string]any{})
	require.NoError(s.T(), err) // Execute returns result, not error
	require.False(s.T(), result.Success)
	require.Equal(s.T(), "tool_not_found", result.ErrorType)
}

// TestExecute_External tests external tool execution
func (s *RegistryTestSuite) TestExecute_External() {
	// Register executor
	executor := &MockExternalExecutor{
		callToolFunc: func(ctx context.Context, toolName string, arguments map[string]any) (any, error) {
			return map[string]any{"external_result": "ok", "tool": toolName}, nil
		},
	}
	s.registry.RegisterExternalExecutor("external_server", executor)

	// Register external tool
	s.registry.RegisterExternalTool(
		"external_server",
		"external",
		"remote_tool",
		"Remote tool",
		map[string]any{"type": "object"},
	)

	result, err := s.registry.Execute(s.ctx, "external_server_remote_tool", map[string]any{"param": "value"})
	require.NoError(s.T(), err)
	require.True(s.T(), result.Success)
	require.Equal(s.T(), "ok", result.Result["external_result"])
	require.Equal(s.T(), "remote_tool", result.Result["tool"]) // Should strip prefix
}

// TestExecute_ExternalExecutorNotFound tests external tool with missing executor
func (s *RegistryTestSuite) TestExecute_ExternalExecutorNotFound() {
	// Register external tool without executor
	s.registry.RegisterExternalTool(
		"missing_server",
		"external",
		"remote_tool",
		"Remote tool",
		map[string]any{"type": "object"},
	)

	result, err := s.registry.Execute(s.ctx, "missing_server_remote_tool", map[string]any{})
	require.NoError(s.T(), err)
	require.False(s.T(), result.Success)
	require.Equal(s.T(), "executor_not_found", result.ErrorType)
}

// TestExecuteBatch tests batch execution
func (s *RegistryTestSuite) TestExecuteBatch() {
	// Register tools
	tool1 := &Tool{
		Name:     "tool1",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "tool1_success"}, nil
		},
	}
	tool2 := &Tool{
		Name:     "tool2",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "tool2_success"}, nil
		},
	}

	s.registry.Register(tool1)
	s.registry.Register(tool2)

	// Execute batch
	request := &BatchExecutionRequest{
		Tools: []ToolExecution{
			{ToolName: "tool1", Arguments: map[string]any{}},
			{ToolName: "tool2", Arguments: map[string]any{}},
		},
		ContinueOnError: true,
	}

	result, err := s.registry.ExecuteBatch(s.ctx, request)
	require.NoError(s.T(), err)
	require.Len(s.T(), result.Results, 2)
	require.Equal(s.T(), 2, result.SuccessfulCount)
	require.Equal(s.T(), 0, result.FailedCount)
}

// TestExecuteBatch_StopOnError tests batch execution stopping on error
func (s *RegistryTestSuite) TestExecuteBatch_StopOnError() {
	tool1 := &Tool{
		Name:     "tool1",
		Category: "test",
		Source:   SourceInternal,
		Handler: func(ctx context.Context, params map[string]any) (map[string]any, error) {
			return map[string]any{"result": "success"}, nil
		},
	}

	s.registry.Register(tool1)

	// Execute batch with one failing tool
	request := &BatchExecutionRequest{
		Tools: []ToolExecution{
			{ToolName: "tool1", Arguments: map[string]any{}},
			{ToolName: "nonexistent", Arguments: map[string]any{}},
			{ToolName: "tool1", Arguments: map[string]any{}}, // Should not execute
		},
		ContinueOnError: false,
	}

	result, err := s.registry.ExecuteBatch(s.ctx, request)
	require.NoError(s.T(), err)
	require.Len(s.T(), result.Results, 2) // Should stop after second tool
	require.Equal(s.T(), 1, result.SuccessfulCount)
	require.Equal(s.T(), 1, result.FailedCount)
}

// TestListAll tests listing all tools
func (s *RegistryTestSuite) TestListAll() {
	// Register some tools
	for i := 0; i < 3; i++ {
		tool := &Tool{
			Name:     "tool_" + string(rune('a'+i)),
			Category: "test",
			Source:   SourceInternal,
			Handler:  func(ctx context.Context, params map[string]any) (map[string]any, error) { return nil, nil },
		}
		s.registry.Register(tool)
	}

	tools := s.registry.ListAll()
	require.Len(s.T(), tools, 3)
}

// TestRegistryTestSuite runs the test suite
func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
