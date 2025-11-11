package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ExternalToolExecutor defines the interface for executing external tools.
type ExternalToolExecutor interface {
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error)
}

// Registry manages all available tools and their execution.
type Registry struct {
	tools             map[string]*Tool
	externalExecutors map[string]ExternalToolExecutor // Map of source name -> executor
	logger            *slog.Logger
}

// NewRegistry creates a new tool registry.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		tools:             make(map[string]*Tool),
		externalExecutors: make(map[string]ExternalToolExecutor),
		logger:            logger,
	}
}

// RegisterExternalExecutor registers an executor for external tools from a specific source.
func (r *Registry) RegisterExternalExecutor(sourceName string, executor ExternalToolExecutor) {
	r.externalExecutors[sourceName] = executor
	r.logger.Info("Registered external tool executor", "source", sourceName)
}

// RegisterExternalTool registers a tool from an external MCP server.
func (r *Registry) RegisterExternalTool(sourceName, category string, toolName, description string, inputSchema map[string]interface{}) error {
	// Prefix tool name with server name to avoid conflicts
	prefixedName := sourceName + "_" + toolName

	tool := &Tool{
		Name:        prefixedName,
		Category:    category,
		Description: description,
		Source:      SourceExternal,
		SourceName:  sourceName,
		InputSchema: inputSchema,
		Handler:     nil, // External tools don't have handlers
	}

	return r.Register(tool)
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool *Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	// Only internal tools require a handler; external tools are executed remotely
	if tool.Source == SourceInternal && tool.Handler == nil {
		return fmt.Errorf("tool handler cannot be nil for internal tools")
	}
	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}

	r.tools[tool.Name] = tool
	r.logger.Info("Registered tool", "name", tool.Name, "category", tool.Category, "source", tool.Source)
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (*Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

// Search finds tools matching the given criteria.
func (r *Registry) Search(query, category string) []*Tool {
	var results []*Tool

	queryLower := strings.ToLower(query)

	for _, tool := range r.tools {
		// Filter by category if specified
		if category != "" && tool.Category != category {
			continue
		}

		// Filter by query if specified
		if query != "" {
			nameLower := strings.ToLower(tool.Name)
			descLower := strings.ToLower(tool.Description)
			if !strings.Contains(nameLower, queryLower) && !strings.Contains(descLower, queryLower) {
				continue
			}
		}

		results = append(results, tool)
	}

	return results
}

// Execute runs a tool with the given parameters.
func (r *Registry) Execute(ctx context.Context, toolName string, parameters map[string]any) (*ExecutionResult, error) {
	start := time.Now()

	tool, err := r.Get(toolName)
	if err != nil {
		return &ExecutionResult{
			Success:         false,
			ToolName:        toolName,
			Error:           err.Error(),
			ErrorType:       "tool_not_found",
			ExecutionTimeMs: time.Since(start).Milliseconds(),
		}, nil
	}

	r.logger.InfoContext(ctx, "Executing tool", "name", toolName, "source", tool.Source, "parameters", parameters)

	var result map[string]any
	var execErr error

	// Route execution based on source
	if tool.Source == SourceInternal {
		// Execute internal tool via handler
		result, execErr = tool.Handler(ctx, parameters)
	} else if tool.Source == SourceExternal {
		// Execute external tool via MCP client
		executor, ok := r.externalExecutors[tool.SourceName]
		if !ok {
			return &ExecutionResult{
				Success:         false,
				ToolName:        toolName,
				Error:           fmt.Sprintf("external executor not found: %s", tool.SourceName),
				ErrorType:       "executor_not_found",
				ExecutionTimeMs: time.Since(start).Milliseconds(),
			}, nil
		}

		// Convert parameters to map[string]interface{} for external call
		paramsInterface := make(map[string]interface{})
		for k, v := range parameters {
			paramsInterface[k] = v
		}

		// Strip the server name prefix before calling external tool
		// toolName format: "servername_originaltoolname"
		originalToolName := strings.TrimPrefix(toolName, tool.SourceName+"_")

		externalResult, err := executor.CallTool(ctx, originalToolName, paramsInterface)
		if err != nil {
			execErr = err
		} else {
			// Convert result to map[string]any
			if resultMap, ok := externalResult.(map[string]interface{}); ok {
				result = make(map[string]any)
				for k, v := range resultMap {
					result[k] = v
				}
			} else {
				// Wrap non-map results
				result = map[string]any{"result": externalResult}
			}
		}
	} else {
		execErr = fmt.Errorf("unknown tool source: %s", tool.Source)
	}

	executionTime := time.Since(start).Milliseconds()

	if execErr != nil {
		r.logger.ErrorContext(ctx, "Tool execution failed", "name", toolName, "source", tool.Source, "error", execErr)
		return &ExecutionResult{
			Success:         false,
			ToolName:        toolName,
			Error:           execErr.Error(),
			ErrorType:       "execution_error",
			ExecutionTimeMs: executionTime,
		}, nil
	}

	r.logger.InfoContext(ctx, "Tool execution successful", "name", toolName, "source", tool.Source, "execution_time_ms", executionTime)

	return &ExecutionResult{
		Success:         true,
		ToolName:        toolName,
		Result:          result,
		ExecutionTimeMs: executionTime,
	}, nil
}

// ExecuteBatch runs multiple tools in sequence.
func (r *Registry) ExecuteBatch(ctx context.Context, request *BatchExecutionRequest) (*BatchExecutionResult, error) {
	start := time.Now()

	results := make([]ExecutionResult, 0, len(request.Tools))
	successCount := 0
	failedCount := 0

	for _, toolExec := range request.Tools {
		result, err := r.Execute(ctx, toolExec.ToolName, toolExec.Parameters)
		if err != nil {
			// This shouldn't happen as Execute returns ExecutionResult even on error
			return nil, err
		}

		results = append(results, *result)

		if result.Success {
			successCount++
		} else {
			failedCount++
			if !request.ContinueOnError {
				r.logger.WarnContext(ctx, "Stopping batch execution due to error", "tool", toolExec.ToolName)
				break
			}
		}
	}

	totalTime := time.Since(start).Milliseconds()

	return &BatchExecutionResult{
		Results:              results,
		TotalExecutionTimeMs: totalTime,
		SuccessfulCount:      successCount,
		FailedCount:          failedCount,
	}, nil
}

// ListAll returns all registered tools.
func (r *Registry) ListAll() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}
