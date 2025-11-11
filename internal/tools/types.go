package tools

import (
	"context"
)

// ToolSource indicates where a tool is implemented
type ToolSource string

const (
	SourceInternal ToolSource = "internal" // Internal tools
	SourceExternal ToolSource = "external" // External MCP server tools
)

// ToolHandler represents a function that handles tool execution
type ToolHandler func(context.Context, map[string]any) (map[string]any, error)

// Tool represents a single executable tool with its metadata and handler.
type Tool struct {
	Name        string      // Tool name
	Category    string      // Category for organizing tools (e.g., "browser", "playwright", etc.)
	Description string      // Tool description
	InputSchema interface{} // Schema for tool parameters (can be map[string]any or struct with jsonschema tags)
	Handler     ToolHandler // Handler function for internal tools (nil for external)
	Source      ToolSource  // Where the tool is implemented
	SourceName  string      // Name of external MCP server (if external)
}

// ExecutionResult represents the result of a tool execution.
type ExecutionResult struct {
	Success         bool           `json:"success"`
	ToolName        string         `json:"tool_name"`
	Result          map[string]any `json:"result,omitempty"`
	Error           string         `json:"error,omitempty"`
	ErrorType       string         `json:"error_type,omitempty"`
	ExecutionTimeMs int64          `json:"execution_time_ms"`
}

// BatchExecutionRequest represents a request to execute multiple tools.
type BatchExecutionRequest struct {
	Tools           []ToolExecution `json:"tools"`
	ContinueOnError bool            `json:"continue_on_error"`
}

// ToolExecution represents a single tool execution request.
type ToolExecution struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
}

// BatchExecutionResult represents the result of a batch execution.
type BatchExecutionResult struct {
	Results              []ExecutionResult `json:"results"`
	TotalExecutionTimeMs int64             `json:"total_execution_time_ms"`
	SuccessfulCount      int               `json:"successful_count"`
	FailedCount          int               `json:"failed_count"`
}

// ToolMetadata represents tool information for search results.
type ToolMetadata struct {
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"` // Schema as map
}
