# OneMCP - Generic MCP Aggregator

A universal Model Context Protocol (MCP) aggregator that combines multiple external MCP servers into a unified interface with progressive discovery.

**Version 1.0.0** - Production-ready generic aggregator with meta-tool architecture for improved efficiency and extensibility.

**Built with the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)** from Anthropic/Google collaboration.

## What is OneMCP?

OneMCP is a **generic MCP aggregator** that:
- Aggregates tools from multiple external MCP servers
- Supports custom internal tools with type-safe registration
- Exposes a unified meta-tool interface to reduce token usage
- Supports progressive tool discovery (search before loading schemas)
- Works with any MCP-compliant server

## Why OneMCP?

When working with many MCP servers, exposing hundreds of tools directly to LLMs consumes massive amounts of tokens and context window. As explained in Anthropic's [Code Execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp) article, the meta-tool pattern solves this by:

1. **Reducing token overhead**: Instead of loading 50+ tool schemas (tens of thousands of tokens), expose just 2 meta-tools
2. **Progressive discovery**: LLMs search for relevant tools only when needed
3. **Preserving context**: More room for actual conversation and code, less for tool definitions
4. **Scaling gracefully**: Add new servers without increasing baseline token usage

OneMCP implements this pattern as a **universal aggregator** that works with:
- Any LLM that supports MCP (Claude, OpenAI, Gemini, local models via Claude Desktop, etc.)
- Any MCP-compliant server
- Any deployment scenario (local development, production APIs, agent frameworks)

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
    └── External MCP Servers (configured via .onemcp.json)
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
2. **Semantic Search**: TF-IDF embeddings for fast, accurate keyword-based discovery
3. **Progressive Discovery**: Four detail levels (names_only → summary → detailed → full_schema)
4. **Schema Caching**: External tool schemas cached at startup, no repeated fetching
5. **In-Memory Vector Store**: Pre-computed embeddings for instant search
6. **Lazy Loading**: Schemas only sent when explicitly requested via detail_level

**Token Usage Examples (default 5 tools):**
- `names_only` search: ~50 tokens total
- `summary` search: ~200-400 tokens total
- `full_schema` search: ~2000-5000 tokens total

### Semantic Search

OneMCP supports two embedding strategies for tool discovery:

#### 1. **TF-IDF** (Default)
- **Best for:** Fast, zero-overhead keyword matching
- **Speed:** Instant (no model loading)
- **Quality:** Excellent for keyword-based search
- **Size:** 0MB overhead
- **Use when:** You want instant startup and good keyword matching

```json
{
  "settings": {
    "embedderType": "tfidf"
  }
}
```

#### 2. **GloVe** (Recommended for Best Quality)
- **Best for:** True semantic understanding ("screenshot" ≈ "capture")
- **Speed:** Instant startup! Downloads in background, hot-swaps when ready
- **Quality:** State-of-the-art pre-trained embeddings (400K vocabulary)
- **Background download:** Starts with TF-IDF, upgrades to GloVe automatically (~30s for 331MB)
- **Cached startup:** ~5 seconds to load 400K word vectors from disk
- **Use when:** You want best semantic search quality without blocking startup

```json
{
  "settings": {
    "embedderType": "glove",
    "gloveModel": "6B.100d",
    "gloveCacheDir": "/tmp/onemcp-glove"
  }
}
```

**Available GloVe models:**
- `6B.50d` - 50 dimensions, 163MB download, 192K vocab
- `6B.100d` - 100 dimensions, 331MB download, 400K vocab (recommended)
- `6B.200d` - 200 dimensions, 661MB download, 400K vocab
- `6B.300d` - 300 dimensions, 990MB download, 400K vocab

**Example:** Query "capture page image" finds `browser_screenshot` because GloVe learned from 6 billion words that "capture", "screenshot", and "image" are semantically related.

**Recommendation:** Use **GloVe** for best semantic search quality, or **TF-IDF** for instant startup and zero downloads.

## Technology

OneMCP is built with:
- **[Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)** v1.1.0 - Anthropic/Google collaboration
- **Go 1.25** - Modern, efficient, and type-safe
- **JSON-RPC 2.0** - Standard protocol for MCP communication
- **Multiple Transports** - Stdio (command), HTTP (SSE), and more

The official SDK provides:
- Type-safe tool registration with automatic schema inference
- Multiple transport options (stdio via CommandTransport, HTTP via SSE, StreamableHTTP, in-memory for testing)
- Built-in client for connecting to external servers
- Full support for MCP protocol features

**Supported Transports:**
- **Command (stdio)**: Execute local commands and communicate via stdin/stdout using JSON-RPC - most common for local tools
- **Streamable HTTP**: Connect to remote HTTP-based MCP servers using JSON-RPC over HTTP with optional SSE streaming (MCP spec 2025-03-26+) - ideal for cloud services
- **In-Memory**: Direct in-process communication - useful for testing

**Protocol Details:**
- All MCP communication uses **JSON-RPC 2.0** for message encoding
- **Stdio transport**: JSON-RPC messages over stdin/stdout
- **Streamable HTTP transport**: JSON-RPC via HTTP POST/GET with optional Server-Sent Events (SSE) for streaming responses
  - Single endpoint (no dual endpoint complexity)
  - Supports both request/response and streaming
  - Session management via `Mcp-Session-Id` header
  - Automatic reconnection with `Last-Event-ID` for resilience

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
    "searchResultLimit": 5,
    "embedderType": "tfidf"
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

### 4. Use with MCP Clients

Add to your MCP client config. For example, Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json`):

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

OneMCP uses `.onemcp.json` for configuration. The configuration file supports **JSON with comments (JSONC)** format - add `//` for line comments or `/* */` for block comments to document your configuration.

See `.onemcp.json.example` for a complete example with comments.

### Settings

Configure OneMCP behavior:

```json
{
  "settings": {
    "searchResultLimit": 5,
    "embedderType": "tfidf"
  }
}
```

**Available Settings:**
- `searchResultLimit` (number) - Number of tools to return per search query. Default: 5. Lower values reduce token usage but require more searches for discovery.
- `embedderType` (string) - Type of semantic search embedder. Options:
  - `"tfidf"` (default) - Fast, accurate keyword matching. Works great with any tool set size.
  - `"glove"` - Pre-trained GloVe embeddings. Best semantic understanding, auto-downloads model (~100MB).
- `gloveModel` (string) - GloVe model to use. Options: `"6B.50d"`, `"6B.100d"` (default), `"6B.200d"`, `"6B.300d"`. Higher dimensions = better quality but larger download.
- `gloveCacheDir` (string) - Directory to cache downloaded GloVe models. Default: `"/tmp/onemcp-glove"`.

### External Server Configuration

Define external MCP servers in the `mcpServers` section. OneMCP supports multiple transport types:

**1. Command Transport (stdio)** - Most common, runs a local command:
```json
{
  "mcpServers": {
    "playwright": {
      "command": "npx",                // Command to execute
      "args": ["-y", "@playwright/mcp"], // Command arguments
      "env": {                          // Optional: Environment variables
        "DEBUG": "1"
      },
      "category": "browser",           // Optional: Category for grouping tools
      "enabled": true                  // Required: Whether to load this server
    }
  }
}
```

**2. HTTP Transport (Streamable HTTP)** - Connect to remote MCP server via HTTP:
```json
{
  "mcpServers": {
    "remote-server": {
      "url": "https://api.example.com/mcp", // HTTP endpoint URL (Streamable HTTP)
      "category": "api",
      "enabled": true
    }
  }
}
```

**Note:** OneMCP uses Streamable HTTP transport (MCP spec 2025-03-26+) for all HTTP connections. This is the modern standard that replaces the deprecated SSE transport.

**Configuration Fields:**
- `command` (string) - Command to execute (for stdio transport)
- `args` (array) - Command arguments (stdio only)
- `url` (string) - HTTP endpoint URL (for Streamable HTTP transport)
- `env` (object) - Environment variables (stdio only)
- `category` (string) - Category for grouping tools
- `enabled` (boolean) - Whether to load this server

**Note:** Provide either `command` or `url`, not both.

### Environment Variables

- `MCP_SERVER_NAME` - Server name (default: "one-mcp-aggregator")
- `MCP_SERVER_VERSION` - Server version (default: "1.0.0")
- `MCP_LOG_FILE` - Log file path (default: "/tmp/one-mcp.log")

## Tool Naming Convention

External tools are automatically prefixed with their server name:

- `browser_navigate` from `playwright` → `playwright_browser_navigate`
- `take_screenshot` from `chrome` → `chrome_take_screenshot`

This prevents naming conflicts when aggregating multiple servers.

## Progressive Discovery Workflow

The recommended workflow for LLMs:

1. **Search for tools**: Use `tool_search` with filters to find relevant tools
2. **Get detailed schemas**: Use `detail_level: "full_schema"` for tools you plan to use
3. **Execute tools**: Use `tool_execute` with validated arguments

**Example conversation:**
```
User: "Take a screenshot of example.com"

LLM: Let me search for screenshot tools...
→ tool_search(query="screenshot", detail_level="full_schema")

LLM: Found playwright_browser_navigate and playwright_browser_take_screenshot. 
Let me navigate first...
→ tool_execute(tool_name: "playwright_browser_navigate", arguments: {url: "https://example.com"})

LLM: Now taking screenshot...
→ tool_execute(tool_name: "playwright_browser_take_screenshot", arguments: {filename: "example.png"})
```

## Logging

Logs are written to the file specified by `MCP_LOG_FILE` (default: `/tmp/one-mcp.log`):

```
time=2025-11-11T10:00:00.000+00:00 level=INFO msg="Starting OneMCP aggregator server over stdio..." name=one-mcp-aggregator version=1.0.0
time=2025-11-11T10:00:01.000+00:00 level=INFO msg="Loaded external MCP server" name=playwright tools=21 category=browser
time=2025-11-11T10:00:02.000+00:00 level=INFO msg="Registered tool" name=playwright_browser_navigate category=browser
time=2025-11-11T10:00:03.000+00:00 level=INFO msg="Executing tool" name=playwright_browser_navigate
time=2025-11-11T10:00:04.000+00:00 level=INFO msg="Tool execution successful" name=playwright_browser_navigate execution_time_ms=245
```

## Troubleshooting

### External server fails to start

- Check that the command path is correct in `.onemcp.json`
- Verify required environment variables are set
- Check logs in `MCP_LOG_FILE` for startup errors
- Test the server command manually: `command args...`

### Tool not found

- Use `tool_search` to verify the tool exists
- Check that tool names include the server prefix (e.g., `playwright_browser_navigate`)
- Verify the external server is enabled in `.onemcp.json`

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
├── .onemcp.json                 # Configuration (settings + external servers)
├── go.mod
└── README.md
```

### Adding External Servers

Simply add to the `mcpServers` section in `.onemcp.json` - no code changes required:

```json
{
  "settings": {
    "searchResultLimit": 5,
    "embedderType": "tfidf"
  },
  "mcpServers": {
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
}
```

OneMCP will automatically:
1. Start the external server
2. Fetch its tool list
3. Prefix tool names with `your-server_`
4. Make tools discoverable via `tool_search`
5. Route `tool_execute` calls to the external server

### Adding Internal Tools

**Note**: Adding internal tools requires modifying the OneMCP source code. You'll need to:
1. Clone this repository: `git clone https://github.com/radutopala/onemcp.git`
2. Make your changes (see steps below)
3. Rebuild the binary: `go build -o one-mcp ./cmd/one-mcp`

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

Internal tools are directly exposed via `tools/list` alongside the 2 meta-tools, making them immediately available without needing `tool_search`.

**When to use internal tools vs external servers:**
- **Use external servers** (recommended): For most use cases - no code changes needed, just configuration
- **Use internal tools**: Only when you need tight integration with OneMCP's core logic or want Go's type safety for custom business logic

## License

MIT License - See LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or PR on GitHub.
