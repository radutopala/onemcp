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
- Works with any MCP-compliant server

## Architecture

```
OneMCP Aggregator
    ├── Meta-Tools (2)
    │   ├── tool_search        - Discover available tools
    │   └── tool_execute       - Execute a single tool
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

1. **Token Efficiency**: 99% reduction - expose 2 meta-tools instead of hundreds of individual tools
2. **Progressive Discovery**: Search first, load schemas only for needed tools
3. **Universal**: Works with any MCP-compliant server
4. **Flexible**: Support both external servers (config) and internal tools (Go code)
5. **Type-Safe**: Built-in tools leverage Go's type system with automatic schema inference

## Performance Optimizations

OneMCP includes several optimizations for token efficiency and speed:

1. **Configurable Result Limit**: Returns 5 tools per search by default (configurable via `.onemcp.json`)
2. **Hybrid Schema Approach**: Limited tools inline + complete schema file for comprehensive exploration
3. **Progressive Discovery**: Four detail levels (names_only → summary → detailed → full_schema)
4. **Schema Caching**: External tool schemas cached at startup, no repeated fetching
5. **Fuzzy Search**: Levenshtein distance algorithm handles typos without multiple queries
6. **Lazy Loading**: Schemas only sent when explicitly requested via detail_level

**Token Usage Examples (default 5 tools):**
- `names_only` search: ~50 tokens total
- `summary` search: ~200-400 tokens total
- `full_schema` search: ~2000-5000 tokens total
- Schema file path: ~50 tokens (read file separately for complete tool list)

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
GOOS=darwin GOARCH=amd64 go build -o one-mcp ./cmd/one-mcp

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o one-mcp-linux ./cmd/one-mcp
```

### 2. Configure OneMCP

Create `.onemcp.json`:

```json
{
  "settings": {
    "searchResultLimit": 5
  },
  "mcpServers": {
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
}
```

### 3. Run the aggregator

```bash
# Start OneMCP aggregator
./one-mcp

# Or with custom server name/version
MCP_SERVER_NAME=my-aggregator MCP_SERVER_VERSION=1.0.0 ./one-mcp
```

### 4. Use with Claude Desktop

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "onemcp": {
      "command": "/path/to/one-mcp",
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
Discover available tools with optional filtering. Uses **fuzzy matching** to handle typos and small variations. Returns **5 tools per query by default** (configurable via `.onemcp.json`).

**Arguments:**
- `query` (optional) - Search term to filter by name or description. Must be a single word for fuzzy matching (e.g., "browser", "navigate", "screenshot")
- `category` (optional) - Filter by category (e.g., "browser", "filesystem")
- `detail_level` (optional) - Level of detail to return:
  - `"names_only"` - Just tool names and categories (minimal tokens)
  - `"summary"` - Name, category, and description (default)
  - `"detailed"` - Includes argument schema
  - `"full_schema"` - Complete schema with all details
- `offset` (optional) - Number of results to skip for pagination (default: 0)

**Fuzzy Matching:** Query terms support fuzzy matching using Levenshtein distance, allowing small typos (e.g., "navgate" matches "navigate", "clik" matches "click"). The matching threshold adapts based on query length.

**Schema Caching:** External tool schemas are cached at startup for fast repeated searches.

**Hybrid Approach:** Search returns **5 tools inline by default** (configurable) plus a `schema_file` path (`/tmp/onemcp-tools-schema.json`) containing **ALL executable tools with full schemas** (external and internal tools only, excluding meta-tools which are already exposed via MCP's `tools/list`). For comprehensive tool exploration, search the schema file using filesystem tools instead of paginating through search results. This reduces token usage while maintaining access to complete tool information.

**Example - Basic search:**
```json
{
  "tool_name": "tool_search",
  "arguments": {
    "query": "navigate",
    "detail_level": "summary"
  }
}
```

**Example - Paginated search:**
```json
{
  "tool_name": "tool_search",
  "arguments": {
    "query": "browser",
    "category": "browser",
    "detail_level": "detailed",
    "offset": 5
  }
}
```

**Returns:**
```json
{
  "total_count": 21,
  "returned_count": 5,
  "offset": 0,
  "limit": 5,
  "has_more": true,
  "schema_file": "/tmp/onemcp-tools-schema.json",
  "message": "Showing 5 of 21 tools. For complete tool list with full schemas, search with filesystem tools in: /tmp/onemcp-tools-schema.json",
  "tools": [
    {
      "name": "playwright_browser_navigate",
      "category": "browser",
      "description": "Navigate to a URL",
      "schema": {...}
    },
    {
      "name": "playwright_browser_click",
      "category": "browser",
      "description": "Click an element",
      "schema": {...}
    }
  ]
}
```

### 2. `tool_execute`
Execute a single tool by name.

**Arguments:**
- `tool_name` (required) - Name of the tool (e.g., `playwright_browser_navigate`)
- `arguments` (required) - Tool-specific arguments

**Example:**
```json
{
  "tool_name": "tool_execute",
  "arguments": {
    "tool_name": "playwright_browser_navigate",
    "arguments": {
      "url": "https://example.com"
    }
  }
}
```

## Configuration

OneMCP uses `.onemcp.json` for configuration. See `.onemcp.json.example` for a complete example.

### Settings

Configure OneMCP behavior:

```json
{
  "settings": {
    "searchResultLimit": 5
  }
}
```

**Available Settings:**
- `searchResultLimit` (number) - Number of tools to return per search query. Default: 5. Lower values reduce token usage but require more searches for discovery.

### External Server Configuration

Define external MCP servers in the `mcpServers` section:

```json
{
  "mcpServers": {
    "server-name": {
      "command": "path/to/server",     // Required: Command to run
      "args": ["--arg1", "value1"],    // Optional: Command arguments
      "env": {                          // Optional: Environment variables
        "API_KEY": "secret",
        "LOG_LEVEL": "info"
      },
      "category": "browser",           // Optional: Category for grouping tools
      "enabled": true                  // Required: Whether to load this server
    }
  }
}
```

### Environment Variables

- `MCP_SERVER_NAME` - Server name (default: "one-mcp-aggregator")
- `MCP_SERVER_VERSION` - Server version (default: "0.1.0")
- `MCP_LOG_FILE` - Log file path (default: "/tmp/one-mcp.log")

## Tool Naming Convention

External tools are automatically prefixed with their server name:

- `browser_navigate` from `playwright` → `playwright_browser_navigate`
- `take_screenshot` from `chrome` → `chrome_take_screenshot`

This prevents naming conflicts when aggregating multiple servers.

## Progressive Discovery Workflow

The recommended workflow for Claude:

1. **Search for tools**: Use `tool_search` with filters to find relevant tools
2. **Get detailed schemas**: Use `detail_level: "full_schema"` for tools you plan to use
3. **Execute tools**: Use `tool_execute` with validated arguments

**Example conversation:**
```
User: "Take a screenshot of example.com"

Claude: Let me search for screenshot tools...
→ tool_search(query="screenshot", detail_level="full_schema")

Claude: Found playwright_browser_navigate and playwright_browser_take_screenshot. 
Let me navigate first...
→ tool_execute(tool_name: "playwright_browser_navigate", arguments: {url: "https://example.com"})

Claude: Now taking screenshot...
→ tool_execute(tool_name: "playwright_browser_take_screenshot", arguments: {filename: "example.png"})
```

## Logging

Logs are written to the file specified by `MCP_LOG_FILE` (default: `/tmp/one-mcp.log`):

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

- Use `tool_search` with `detail_level: "full_schema"` to see required arguments
- Check argument types match the schema
- Review logs for detailed error messages

## Development

### Project Structure

```
.
├── cmd/
│   └── one-mcp/
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
- **Struct Tags**: Use `jsonschema:"description"` to document arguments
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
