# OneMCP - Generic MCP Aggregator

A universal Model Context Protocol (MCP) aggregator that combines multiple external MCP servers into a unified interface with progressive discovery.

**Version 0.2.0** - Generic aggregator with meta-tool architecture for improved efficiency and extensibility.

**Built with the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)** from Anthropic/Google collaboration.

## What is OneMCP?

OneMCP is a **generic MCP aggregator** that:
- Aggregates tools from multiple external MCP servers
- Supports custom internal tools with type-safe registration
- Exposes a unified meta-tool interface to reduce token usage
- Supports progressive tool discovery (search before loading schemas)
- Enables batch execution across multiple servers
- Works with any MCP-compliant server

## Architecture

```
OneMCP Aggregator
    ├── Meta-Tools (3)
    │   ├── tool_search        - Discover available tools
    │   ├── tool_execute       - Execute a single tool
    │   └── tool_execute_batch - Execute multiple tools
    │
    ├── Internal Tools (optional)
    │   └── Custom Go-based tools with type-safe handlers
    │
    └── External MCP Servers (configured via .mcp-servers.json)
        ├── Playwright (21 tools) - Browser automation
        ├── Filesystem (N tools) - File operations
        └── Your Server (N tools) - Any MCP-compliant server
```

### Benefits

1. **Token Efficiency**: 98% reduction - expose 3 meta-tools instead of hundreds of individual tools
2. **Progressive Discovery**: Search first, load schemas only for needed tools
3. **Batch Execution**: Execute multiple operations in one call
4. **Universal**: Works with any MCP-compliant server
5. **Flexible**: Support both external servers (config) and internal tools (Go code)
6. **Type-Safe**: Built-in tools leverage Go's type system with automatic schema inference

## Performance Optimizations

OneMCP includes several optimizations for token efficiency and speed:

1. **Progressive Discovery**: Four detail levels (names_only → summary → detailed → full_schema)
2. **Schema Caching**: External tool schemas cached at startup, no repeated fetching
3. **Pagination**: Limit results to reduce token usage (default: 50, max: 200 per request)
4. **Lazy Loading**: Schemas only sent when explicitly requested via detail_level
5. **Single Round-Trip**: Batch execution combines multiple operations

**Token Usage Examples:**
- `names_only` search: ~10 tokens per tool
- `summary` search: ~50-100 tokens per tool
- `full_schema` search: ~500-1000 tokens per tool

## Technology

OneMCP is built with:
- **[Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)** v1.1.0 - Anthropic/Google collaboration
- **Go 1.25** - Modern, efficient, and type-safe
- **JSON-RPC 2.0** - Standard protocol for MCP communication
- **Stdio Transport** - Simple and reliable process communication

The official SDK provides:
- Type-safe tool registration with automatic schema inference
- Multiple transport options (stdio, HTTP, SSE, in-memory)
- Built-in client for connecting to external servers
- Full support for MCP protocol features

## Quick Start

### 1. Build the aggregator

```bash
# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o one-mcp-server ./cmd/one-mcp-server

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o one-mcp-server-linux ./cmd/one-mcp-server
```

### 2. Configure external servers

Create `.mcp-servers.json`:

```json
{
  "playwright": {
    "command": "npx",
    "args": ["-y", "@playwright/mcp"],
    "category": "browser",
    "enabled": true
  },
  "filesystem": {
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
    "category": "filesystem",
    "enabled": true
  }
}
```

### 3. Run the aggregator

```bash
# Start OneMCP aggregator
./one-mcp-server

# Or with custom server name/version
MCP_SERVER_NAME=my-aggregator MCP_SERVER_VERSION=1.0.0 ./one-mcp-server
```

### 4. Use with Claude Desktop

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "onemcp": {
      "command": "/path/to/one-mcp-server",
      "env": {
        "MCP_SERVER_NAME": "my-aggregator",
        "MCP_LOG_FILE": "/tmp/onemcp.log"
      }
    }
  }
}
```

## Meta-Tools API

### 1. `tool_search`
Discover available tools with optional filtering and pagination.

**Parameters:**
- `query` (optional) - Search term to filter by name or description
- `category` (optional) - Filter by category (e.g., "browser", "filesystem")
- `detail_level` (optional) - Level of detail to return:
  - `"names_only"` - Just tool names and categories (minimal tokens)
  - `"summary"` - Name, category, and description (default)
  - `"detailed"` - Includes parameter schema
  - `"full_schema"` - Complete schema with all details
- `offset` (optional) - Number of results to skip (default: 0)
- `limit` (optional) - Maximum results to return (default: 50, max: 200)

**Schema Caching:** External tool schemas are cached at startup for fast repeated searches.

**Example - Names only with pagination:**
```json
{
  "tool_name": "tool_search",
  "parameters": {
    "detail_level": "names_only",
    "limit": 10,
    "offset": 0
  }
}
```

**Example - Full search:**
```json
{
  "tool_name": "tool_search",
  "parameters": {
    "query": "navigate",
    "category": "browser",
    "detail_level": "full_schema"
  }
}
```

**Returns:**
```json
{
  "total_count": 21,
  "returned_count": 2,
  "offset": 0,
  "limit": 50,
  "has_more": false,
  "tools": [
    {
      "name": "playwright_browser_navigate",
      "category": "browser",
      "description": "Navigate to a URL",
      "parameters": {...}
    }
  ]
}
```

### 2. `tool_execute`
Execute a single tool by name.

**Parameters:**
- `tool_name` (required) - Name of the tool (e.g., `playwright_browser_navigate`)
- `parameters` (required) - Tool-specific parameters

**Example:**
```json
{
  "tool_name": "tool_execute",
  "parameters": {
    "tool_name": "playwright_browser_navigate",
    "parameters": {
      "url": "https://example.com"
    }
  }
}
```

### 3. `tool_execute_batch`
Execute multiple tools in sequence.

**Parameters:**
- `tools` (required) - Array of `{tool_name, parameters}` objects
- `continue_on_error` (optional) - Continue if a tool fails (default: false)

**Example:**
```json
{
  "tool_name": "tool_execute_batch",
  "parameters": {
    "tools": [
      {
        "tool_name": "playwright_browser_navigate",
        "parameters": {"url": "https://example.com"}
      },
      {
        "tool_name": "playwright_browser_take_screenshot",
        "parameters": {"filename": "screenshot.png"}
      }
    ],
    "continue_on_error": true
  }
}
```

## Configuration

### External Server Configuration

The `.mcp-servers.json` file defines external MCP servers to aggregate:

```json
{
  "server-name": {
    "command": "path/to/server",           // Required: Command to run
    "args": ["--arg1", "value1"],          // Optional: Command arguments
    "env": {                                // Optional: Environment variables
      "API_KEY": "secret",
      "LOG_LEVEL": "info"
    },
    "category": "browser",                 // Optional: Category for grouping tools
    "enabled": true                        // Required: Whether to load this server
  }
}
```

### Environment Variables

- `MCP_SERVER_NAME` - Server name (default: "one-mcp-aggregator")
- `MCP_SERVER_VERSION` - Server version (default: "0.1.0")
- `MCP_LOG_FILE` - Log file path (default: "/tmp/one-mcp-server.log")

## Tool Naming Convention

External tools are automatically prefixed with their server name:

- `browser_navigate` from `playwright` → `playwright_browser_navigate`
- `take_screenshot` from `chrome` → `chrome_take_screenshot`

This prevents naming conflicts when aggregating multiple servers.

## Progressive Discovery Workflow

The recommended workflow for Claude:

1. **Search for tools**: Use `tool_search` with filters to find relevant tools
2. **Get detailed schemas**: Use `detail_level: "full_schema"` for tools you plan to use
3. **Execute tools**: Use `tool_execute` or `tool_execute_batch` with validated parameters

**Example conversation:**
```
User: "Take a screenshot of example.com"

Claude: Let me search for screenshot tools...
→ tool_search(query="screenshot", detail_level="full_schema")

Claude: Found playwright_browser_take_screenshot. Let me navigate and capture...
→ tool_execute_batch(tools=[
    {tool_name: "playwright_browser_navigate", parameters: {url: "https://example.com"}},
    {tool_name: "playwright_browser_take_screenshot", parameters: {filename: "example.png"}}
  ])
```

## Logging

Logs are written to the file specified by `MCP_LOG_FILE` (default: `/tmp/one-mcp-server.log`):

```
time=2025-11-11T10:00:00.000+00:00 level=INFO msg="Starting OneMCP aggregator server over stdio..." name=one-mcp-aggregator version=0.2.0
time=2025-11-11T10:00:01.000+00:00 level=INFO msg="Loaded external MCP server" name=playwright tools=21 category=browser
time=2025-11-11T10:00:02.000+00:00 level=INFO msg="Registered tool" name=playwright_browser_navigate category=browser
time=2025-11-11T10:00:03.000+00:00 level=INFO msg="Executing tool" name=playwright_browser_navigate
time=2025-11-11T10:00:04.000+00:00 level=INFO msg="Tool execution successful" name=playwright_browser_navigate execution_time_ms=245
```

## Troubleshooting

### External server fails to start

- Check that the command path is correct in `.mcp-servers.json`
- Verify required environment variables are set
- Check logs in `MCP_LOG_FILE` for startup errors
- Test the server command manually: `command args...`

### Tool not found

- Use `tool_search` to verify the tool exists
- Check that tool names include the server prefix (e.g., `playwright_browser_navigate`)
- Verify the external server is enabled in `.mcp-servers.json`

### Tool execution fails

- Use `tool_search` with `detail_level: "full_schema"` to see required parameters
- Check parameter types match the schema
- Review logs for detailed error messages

## Development

### Project Structure

```
.
├── cmd/
│   └── one-mcp-server/
│       └── main.go              # Entry point
├── internal/
│   ├── mcp/
│   │   └── server.go            # Aggregator server with meta-tools
│   ├── tools/
│   │   ├── types.go             # Tool type definitions
│   │   └── registry.go          # Tool registry and dispatcher
│   └── mcpclient/
│       └── client.go            # External MCP server client
├── .mcp-servers.json            # External server configuration
├── go.mod
└── README.md
```

### Adding External Servers

Simply add to `.mcp-servers.json` - no code changes required:

```json
{
  "your-server": {
    "command": "/path/to/your-mcp-server",
    "args": ["--config", "config.json"],
    "env": {
      "API_KEY": "your-key"
    },
    "category": "custom",
    "enabled": true
  }
}
```

OneMCP will automatically:
1. Start the external server
2. Fetch its tool list
3. Prefix tool names with `your-server_`
4. Make tools discoverable via `tool_search`
5. Route `tool_execute` calls to the external server

### Adding Internal Tools

To add custom internal tools directly to the OneMCP aggregator:

#### 1. Define your tool struct with input/output types

```go
// internal/tools/mytools.go
package tools

type CalculatorInput struct {
    A int `json:"a" jsonschema:"First number"`
    B int `json:"b" jsonschema:"Second number"`
}

type CalculatorOutput struct {
    Result int `json:"result" jsonschema:"Calculation result"`
}
```

#### 2. Implement the tool handler

```go
func (s *AggregatorServer) handleCalculate(ctx context.Context, req *mcp.CallToolRequest, input CalculatorInput) (*mcp.CallToolResult, any, error) {
    result := CalculatorOutput{
        Result: input.A + input.B,
    }
    
    resultJSON, _ := json.Marshal(result)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: string(resultJSON)},
        },
    }, nil, nil
}
```

#### 3. Register the tool in the server

```go
// internal/mcp/server.go - in registerMetaTools() or a new registration function
func (s *AggregatorServer) registerCustomTools(server *mcp.Server) error {
    mcp.AddTool(server, &mcp.Tool{
        Name:        "calculate",
        Description: "Add two numbers together",
    }, s.handleCalculate)
    
    return nil
}
```

#### 4. Call the registration function

```go
// In NewAggregatorServer(), after registerMetaTools()
if err := aggregator.registerCustomTools(server); err != nil {
    return nil, fmt.Errorf("failed to register custom tools: %w", err)
}
```

#### Key Points

- **Type Safety**: The official SDK automatically infers schemas from your Go structs
- **Struct Tags**: Use `jsonschema:"description"` to document parameters
- **Handler Signature**: `func(ctx, *CallToolRequest, InputType) (*CallToolResult, any, error)`
- **Response Format**: Always return JSON in TextContent for consistency with meta-tools
- **No Schema Required**: If you don't provide `inputSchema` in the Tool struct, it's inferred from your input type

#### Example: Echo Tool

```go
// Simple echo tool that returns what you send
type EchoInput struct {
    Message string `json:"message" jsonschema:"Message to echo back"`
}

func (s *AggregatorServer) handleEcho(ctx context.Context, req *mcp.CallToolRequest, input EchoInput) (*mcp.CallToolResult, any, error) {
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: input.Message},
        },
    }, nil, nil
}

// Register in server
mcp.AddTool(server, &mcp.Tool{
    Name:        "echo",
    Description: "Echo back a message",
}, s.handleEcho)
```

Internal tools are directly exposed via `tools/list` alongside the 3 meta-tools, making them immediately available without needing `tool_search`.

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or PR on GitHub.
