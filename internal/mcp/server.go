package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/radutopala/onemcp/internal/mcpclient"
	"github.com/radutopala/onemcp/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExternalServerConfig represents configuration for external MCP servers
type ExternalServerConfig struct {
	ExternalServers map[string]mcpclient.MCPServerConfig `json:"external_servers"`
}

// AggregatorServer implements a generic MCP aggregator
type AggregatorServer struct {
	server          *mcp.Server
	logger          *slog.Logger
	registry        *tools.Registry
	externalClients map[string]*mcpclient.MCPClient
}

// NewAggregatorServer creates a new generic aggregator server
func NewAggregatorServer(name, version string, logger *slog.Logger) (*AggregatorServer, error) {
	ctx := context.Background()

	aggregator := &AggregatorServer{
		logger:          logger,
		registry:        tools.NewRegistry(logger),
		externalClients: make(map[string]*mcpclient.MCPClient),
	}

	// Load and initialize external MCP servers
	if err := aggregator.initializeExternalServers(ctx); err != nil {
		logger.Warn("Failed to initialize external servers, continuing without them", "error", err)
	}

	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    name,
			Version: version,
		},
		nil,
	)

	// Register meta-tools
	if err := aggregator.registerMetaTools(server); err != nil {
		return nil, fmt.Errorf("failed to register meta-tools: %w", err)
	}

	aggregator.server = server

	return aggregator, nil
}

// initializeExternalServers loads and connects to external MCP servers.
func (s *AggregatorServer) initializeExternalServers(ctx context.Context) error {
	// Try to load external server configuration
	configPath := os.Getenv("MCP_SERVERS_CONFIG")
	if configPath == "" {
		configPath = ".mcp-servers.json"
	}

	s.logger.Info("Looking for external servers config", "path", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Info("No external servers config found, skipping", "path", configPath)
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	s.logger.Info("Found external servers config", "path", configPath, "size_bytes", len(data))

	var config ExternalServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Initialize each external server
	for name, serverConfig := range config.ExternalServers {
		if !serverConfig.Enabled {
			s.logger.Info("Skipping disabled external server", "name", name)
			continue
		}

		if err := s.connectExternalServer(ctx, name, serverConfig); err != nil {
			s.logger.Error("Failed to connect external server", "name", name, "error", err)
			continue
		}
	}

	s.logger.Info("Initialized external servers", "count", len(s.externalClients))
	return nil
}

// connectExternalServer connects to a single external MCP server and registers its tools.
func (s *AggregatorServer) connectExternalServer(ctx context.Context, name string, config mcpclient.MCPServerConfig) error {
	// Create MCP client
	client, err := mcpclient.NewMCPClient(ctx, name, config, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Initialize the connection
	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// List available tools
	externalTools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Register the executor
	s.registry.RegisterExternalExecutor(name, client)

	// Register each tool
	category := config.Category
	if category == "" {
		category = name // Use server name as category if not specified
	}
	for _, tool := range externalTools {
		if err := s.registry.RegisterExternalTool(name, category, tool.Name, tool.Description, tool.InputSchema); err != nil {
			s.logger.Warn("Failed to register external tool", "server", name, "tool", tool.Name, "error", err)
			continue
		}
	}

	// Store the client
	s.externalClients[name] = client

	s.logger.Info("Connected to external MCP server", "name", name, "tools", len(externalTools))
	return nil
}

// Close shuts down all external MCP server connections.
func (s *AggregatorServer) Close() error {
	for name, client := range s.externalClients {
		if err := client.Close(); err != nil {
			s.logger.Warn("Error closing external client", "name", name, "error", err)
		}
	}
	return nil
}

// Run starts the MCP server with the given transport
func (s *AggregatorServer) Run(ctx context.Context, transport mcp.Transport) error {
	return s.server.Run(ctx, transport)
}

// === META-TOOLS REGISTRATION ===

func (s *AggregatorServer) registerMetaTools(server *mcp.Server) error {
	// Register tool_search
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tool_search",
		Description: "Search and discover available tools. Returns tool metadata with optional parameter schemas. When searching for specific functionality, use 'summary' or 'detailed' level to see descriptions and determine if tools match your needs. Use 'names_only' for broad exploration only.",
	}, s.handleToolSearch)

	// Register tool_execute
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tool_execute",
		Description: "Execute a single tool by name with parameters. Use tool_search first to discover available tools.",
	}, s.handleToolExecute)

	// Register tool_execute_batch
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tool_execute_batch",
		Description: "Execute multiple tools in sequence. Returns results for all executions.",
	}, s.handleToolExecuteBatch)

	return nil
}

// === META-TOOL HANDLERS ===

// ToolSearchInput defines the input for tool_search
type ToolSearchInput struct {
	Query       string `json:"query,omitempty" jsonschema:"Optional search term to filter tools by name or description"`
	Category    string `json:"category,omitempty" jsonschema:"Optional category filter"`
	DetailLevel string `json:"detail_level,omitempty" jsonschema:"Detail level: 'names_only' (just names, for broad exploration), 'summary' (name + description, recommended for targeted search), 'detailed' (includes parameter schema), 'full_schema' (complete schema). Default: 'summary'. Use 'summary' or 'detailed' when searching for specific functionality."`
	Offset      int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination. Default: 0"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Maximum number of results to return. Default: 50, max: 200"`
}

func (s *AggregatorServer) handleToolSearch(ctx context.Context, req *mcp.CallToolRequest, input ToolSearchInput) (*mcp.CallToolResult, any, error) {
	detailLevel := input.DetailLevel
	if detailLevel == "" {
		detailLevel = "summary"
	}

	// Set default limit and validate
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	foundTools := s.registry.Search(input.Query, input.Category)
	totalCount := len(foundTools)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}
	end := start + limit
	if end > totalCount {
		end = totalCount
	}
	paginatedTools := foundTools[start:end]

	toolMetadata := make([]tools.ToolMetadata, len(paginatedTools))
	for i, tool := range paginatedTools {
		metadata := tools.ToolMetadata{
			Name:     tool.Name,
			Category: tool.Category,
		}

		// Include fields based on detail level
		if detailLevel != "names_only" {
			metadata.Description = tool.Description
		}

		// Include schema based on detail level
		if detailLevel == "detailed" || detailLevel == "full_schema" {
			if tool.InputSchema != nil {
				if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
					metadata.Parameters = schemaMap
				}
			}
		}

		toolMetadata[i] = metadata
	}

	result := map[string]any{
		"total_count":    totalCount,
		"returned_count": len(toolMetadata),
		"offset":         offset,
		"limit":          limit,
		"has_more":       end < totalCount,
		"tools":          toolMetadata,
	}

	// Convert result to JSON for the text content
	resultJSON, _ := json.Marshal(result)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil, nil
}

// ToolExecuteInput defines the input for tool_execute
type ToolExecuteInput struct {
	ToolName  string         `json:"tool_name" jsonschema:"Name of the tool to execute"`
	Arguments map[string]any `json:"arguments" jsonschema:"Tool-specific arguments as an object"`
}

func (s *AggregatorServer) handleToolExecute(ctx context.Context, req *mcp.CallToolRequest, input ToolExecuteInput) (*mcp.CallToolResult, any, error) {
	result, err := s.registry.Execute(ctx, input.ToolName, input.Arguments)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: err.Error()},
			},
		}, nil, nil
	}

	// Convert ExecutionResult to map[string]any
	resultMap := map[string]any{
		"success":           result.Success,
		"tool_name":         result.ToolName,
		"result":            result.Result,
		"error":             result.Error,
		"error_type":        result.ErrorType,
		"execution_time_ms": result.ExecutionTimeMs,
	}

	resultJSON, _ := json.Marshal(resultMap)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil, nil
}

// ToolExecuteBatchInput defines the input for tool_execute_batch
type ToolExecuteBatchInput struct {
	Tools           []tools.ToolExecution `json:"tools" jsonschema:"Array of tool executions"`
	ContinueOnError bool                  `json:"continue_on_error,omitempty" jsonschema:"If true, continues executing remaining tools even if one fails. Default: false"`
}

func (s *AggregatorServer) handleToolExecuteBatch(ctx context.Context, req *mcp.CallToolRequest, input ToolExecuteBatchInput) (*mcp.CallToolResult, any, error) {
	request := &tools.BatchExecutionRequest{
		Tools:           input.Tools,
		ContinueOnError: input.ContinueOnError,
	}

	result, err := s.registry.ExecuteBatch(ctx, request)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: err.Error()},
			},
		}, nil, nil
	}

	// Convert BatchExecutionResult to map[string]any
	results := make([]map[string]any, len(result.Results))
	for i, r := range result.Results {
		results[i] = map[string]any{
			"success":           r.Success,
			"tool_name":         r.ToolName,
			"result":            r.Result,
			"error":             r.Error,
			"error_type":        r.ErrorType,
			"execution_time_ms": r.ExecutionTimeMs,
		}
	}

	resultMap := map[string]any{
		"results":                 results,
		"total_execution_time_ms": result.TotalExecutionTimeMs,
		"successful_count":        result.SuccessfulCount,
		"failed_count":            result.FailedCount,
	}

	resultJSON, _ := json.Marshal(resultMap)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil, nil
}
