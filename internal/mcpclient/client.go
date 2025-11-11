package mcpclient

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClient represents a client connection to an external MCP server.
type MCPClient struct {
	name        string
	session     *mcp.ClientSession
	logger      *slog.Logger
	schemaCache map[string]map[string]any // Cache tool schemas: toolName -> schema
}

// MCPServerConfig represents configuration for an external MCP server.
// Supports multiple transport types:
// - Command transport (stdio): Provide "command" field
// - HTTP transports (Streamable HTTP, SSE): Provide "url" field
type MCPServerConfig struct {
	Command  string            `json:"command,omitempty"`  // Command to execute (for stdio transport)
	Args     []string          `json:"args,omitempty"`     // Command arguments
	URL      string            `json:"url,omitempty"`      // HTTP URL (for Streamable HTTP or SSE transport)
	Env      map[string]string `json:"env,omitempty"`      // Environment variables (stdio only)
	Category string            `json:"category,omitempty"` // Category for grouping tools
	Enabled  bool              `json:"enabled"`            // Whether to load this server
}

// Tool represents an MCP tool from an external server.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// NewMCPClient creates a new MCP client connected to an external server.
// Supports multiple transport types based on configuration:
// - Command transport (stdio): When config.Command is provided
// - Streamable HTTP transport: When config.URL is provided (recommended for HTTP)
// - SSE transport: Fallback for older servers (deprecated)
func NewMCPClient(ctx context.Context, name string, config MCPServerConfig, logger *slog.Logger) (*MCPClient, error) {
	// Create MCP client
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "one-mcp-aggregator",
			Version: "1.0.0",
		},
		nil,
	)

	var transport mcp.Transport
	var transportType string

	// Determine transport type based on configuration
	if config.URL != "" {
		// HTTP-based transport (Streamable HTTP - modern standard)
		transport = &mcp.StreamableClientTransport{
			Endpoint:   config.URL,
			MaxRetries: 5, // Default retry count
		}
		transportType = "streamable-http"
		logger.Info("Using Streamable HTTP transport", "name", name, "endpoint", config.URL)
	} else if config.Command != "" {
		// Command transport (stdio)
		cmd := exec.Command(config.Command, config.Args...)

		// Set environment variables
		if len(config.Env) > 0 {
			env := os.Environ() // Start with current environment
			for k, v := range config.Env {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			cmd.Env = env
		}

		transport = &mcp.CommandTransport{
			Command: cmd,
		}
		transportType = "stdio"
		logger.Info("Using stdio transport", "name", name, "command", config.Command)
	} else {
		return nil, fmt.Errorf("no transport configured: must provide either 'command' or 'url'")
	}

	// Connect to the server (this also initializes the connection)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server (%s): %w", transportType, err)
	}

	logger.Info("Connected to external MCP server", "name", name, "transport", transportType)

	return &MCPClient{
		name:        name,
		session:     session,
		logger:      logger,
		schemaCache: make(map[string]map[string]any),
	}, nil
}

// Initialize is now a no-op since connection happens in NewMCPClient
func (c *MCPClient) Initialize(ctx context.Context) error {
	// The official SDK handles initialization during Connect()
	c.logger.Info("External MCP server initialized", "name", c.name)
	return nil
}

// ListTools retrieves all tools from the external MCP server.
func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("tools/list failed: %w", err)
	}

	tools := make([]Tool, len(result.Tools))
	for i, t := range result.Tools {
		// Convert InputSchema to map[string]any and cache it
		schemaMap := make(map[string]any)
		if t.InputSchema != nil {
			// The schema is already a map[string]any in the official SDK
			if schema, ok := t.InputSchema.(map[string]any); ok {
				schemaMap = schema
				// Cache the schema for this tool
				c.schemaCache[t.Name] = schemaMap
			}
		}

		tools[i] = Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schemaMap,
		}
	}

	c.logger.Info("Listed tools from external MCP server", "name", c.name, "count", len(tools), "cached_schemas", len(c.schemaCache))
	return tools, nil
}

// GetCachedSchema retrieves a cached schema for a tool
func (c *MCPClient) GetCachedSchema(toolName string) (map[string]any, bool) {
	schema, ok := c.schemaCache[toolName]
	return schema, ok
}

// CallTool executes a tool on the external MCP server.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]any) (any, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("tools/call failed: %w", err)
	}

	// Extract the result from the response
	// The official SDK returns CallToolResult with Content
	if result.IsError {
		// Tool execution failed
		errorMsg := "unknown error"
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				errorMsg = textContent.Text
			}
		}
		return nil, fmt.Errorf("tool execution error: %s", errorMsg)
	}

	// Success - extract content
	resultMap := make(map[string]any)
	for i, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			if i == 0 && len(result.Content) == 1 {
				// Single text content - try to return as-is
				resultMap["content"] = textContent.Text
			} else {
				// Multiple contents - store by index
				resultMap[fmt.Sprintf("content_%d", i)] = textContent.Text
			}
		}
	}

	return resultMap, nil
}

// Close terminates the connection to the external MCP server.
func (c *MCPClient) Close() error {
	if err := c.session.Close(); err != nil {
		c.logger.Warn("External MCP server close error", "name", c.name, "error", err)
		return err
	}

	c.logger.Info("Closed external MCP server", "name", c.name)
	return nil
}
