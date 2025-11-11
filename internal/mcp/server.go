package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/radutopala/onemcp/internal/mcpclient"
	"github.com/radutopala/onemcp/internal/tools"
	"github.com/radutopala/onemcp/internal/vectorstore"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config represents the complete OneMCP configuration
type Config struct {
	Settings        Settings                             `json:"settings"`
	ExternalServers map[string]mcpclient.MCPServerConfig `json:"mcpServers"`
}

// Settings represents OneMCP settings
type Settings struct {
	SearchResultLimit int `json:"searchResultLimit"` // Number of tools to return per search (default: 5)
}

// AggregatorServer implements a generic MCP aggregator
type AggregatorServer struct {
	server            *mcp.Server
	logger            *slog.Logger
	registry          *tools.Registry
	vectorStore       vectorstore.VectorStore // Semantic search engine
	externalClients   map[string]*mcpclient.MCPClient
	searchResultLimit int // Number of tools to return per search
}

// NewAggregatorServer creates a new generic aggregator server
func NewAggregatorServer(name, version string, logger *slog.Logger) (*AggregatorServer, error) {
	ctx := context.Background()

	aggregator := &AggregatorServer{
		logger:            logger,
		registry:          tools.NewRegistry(logger),
		externalClients:   make(map[string]*mcpclient.MCPClient),
		searchResultLimit: 5, // Default limit
	}

	// Load configuration and initialize external MCP servers
	config, err := aggregator.loadConfig()
	if err != nil {
		logger.Warn("Failed to load config, using defaults", "error", err)
	} else {
		// Apply settings from config
		if config.Settings.SearchResultLimit > 0 {
			aggregator.searchResultLimit = config.Settings.SearchResultLimit
			logger.Info("Using custom search result limit", "limit", config.Settings.SearchResultLimit)
		}

		// Initialize external servers from config
		if err := aggregator.initializeExternalServersFromConfig(ctx, config.ExternalServers); err != nil {
			logger.Warn("Failed to initialize external servers, continuing without them", "error", err)
		}
	}

	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    name,
			Version: version,
		},
		nil,
	)

	// Register meta-tools (both in MCP server and registry)
	if err := aggregator.registerMetaTools(server); err != nil {
		return nil, fmt.Errorf("failed to register meta-tools: %w", err)
	}

	aggregator.server = server

	// Initialize vector store for semantic search
	if err := aggregator.initializeVectorStore(); err != nil {
		logger.Warn("Failed to initialize vector store, semantic search disabled", "error", err)
	}

	return aggregator, nil
}

// loadConfig loads the .onemcp.json configuration file
func (s *AggregatorServer) loadConfig() (*Config, error) {
	configPath := os.Getenv("ONEMCP_CONFIG")
	if configPath == "" {
		configPath = ".onemcp.json"
	}

	s.logger.Info("Looking for config", "path", configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Info("No config found, using defaults", "path", configPath)
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	s.logger.Info("Found config", "path", configPath, "size_bytes", len(data))

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// initializeExternalServersFromConfig connects to external MCP servers from config
func (s *AggregatorServer) initializeExternalServersFromConfig(ctx context.Context, servers map[string]mcpclient.MCPServerConfig) error {
	if len(servers) == 0 {
		s.logger.Info("No external servers configured")
		return nil
	}

	// Initialize each external server
	for name, serverConfig := range servers {
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

// initializeVectorStore builds the in-memory vector store for semantic search
func (s *AggregatorServer) initializeVectorStore() error {
	// Get all tools from registry
	allTools := s.registry.ListAll()

	if len(allTools) == 0 {
		s.logger.Info("No tools to index in vector store")
		return nil
	}

	// Create TF-IDF embedder (pure Go, no external dependencies)
	embedder := vectorstore.NewTFIDFEmbedder(s.logger)

	// Create vector store
	store := vectorstore.NewInMemoryVectorStore(embedder, s.logger)

	// Build index from all tools
	if err := store.BuildFromTools(allTools); err != nil {
		return fmt.Errorf("failed to build vector store: %w", err)
	}

	s.vectorStore = store
	s.logger.Info("Vector store initialized successfully", "indexed_tools", store.GetToolCount())

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
		Description: "Search and discover available tools using semantic search. Supports natural language queries (e.g., 'capture webpage screenshot', 'navigate browser', 'fetch data'). Returns up to 5 tools per query ranked by relevance. Use 'summary' or 'detailed' level to see descriptions and schemas.",
	}, s.handleToolSearch)

	// Register tool_execute
	mcp.AddTool(server, &mcp.Tool{
		Name:        "tool_execute",
		Description: "Execute a single tool by name with parameters. Use tool_search first to discover available tools.",
	}, s.handleToolExecute)

	return nil
}

// === META-TOOL HANDLERS ===

// ToolSearchInput defines the input for tool_search
type ToolSearchInput struct {
	Query       string `json:"query,omitempty" jsonschema:"Search term to filter tools by name or description. Supports natural language queries (e.g., 'capture screenshot', 'navigate browser', 'read file')."`
	Category    string `json:"category,omitempty" jsonschema:"Optional category filter"`
	DetailLevel string `json:"detail_level,omitempty" jsonschema:"Detail level: 'names_only' (just names, for broad exploration), 'summary' (name + description, recommended for targeted search), 'detailed' (includes parameter schema), 'full_schema' (complete schema). Default: 'summary'. Use 'summary' or 'detailed' when searching for specific functionality."`
	Offset      int    `json:"offset,omitempty" jsonschema:"Number of results to skip for pagination. Default: 0"`
}

func (s *AggregatorServer) handleToolSearch(ctx context.Context, req *mcp.CallToolRequest, input ToolSearchInput) (*mcp.CallToolResult, any, error) {
	detailLevel := input.DetailLevel
	if detailLevel == "" {
		detailLevel = "summary"
	}

	// Use configured limit
	limit := s.searchResultLimit

	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	var foundTools []*tools.Tool

	s.logger.Info("Tool search request", "query", input.Query, "category", input.Category, "detail_level", input.DetailLevel, "offset", offset, "limit", limit)

	// Use semantic search with vector store
	if s.vectorStore != nil {
		var err error
		foundTools, err = s.vectorStore.Search(input.Query, limit*3) // Get more results for filtering
		if err != nil {
			s.logger.Error("Semantic search failed", "error", err)
			foundTools = []*tools.Tool{} // Return empty results on error
		} else {
			s.logger.Info("Semantic search completed", "query", input.Query, "results_found", len(foundTools))
		}

		// Apply category filter if specified
		if input.Category != "" {
			filtered := make([]*tools.Tool, 0, len(foundTools))
			for _, tool := range foundTools {
				if tool.Category == input.Category {
					filtered = append(filtered, tool)
				}
			}
			s.logger.Info("Applied category filter", "category", input.Category, "before", len(foundTools), "after", len(filtered))
			foundTools = filtered
		}
	} else {
		// No vector store available
		s.logger.Warn("Vector store not initialized")
		foundTools = []*tools.Tool{}
	}

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

	s.logger.Info("Tool search response", "total_found", totalCount, "returned", len(paginatedTools), "offset", offset, "limit", limit)

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
