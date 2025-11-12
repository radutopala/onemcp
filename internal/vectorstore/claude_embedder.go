package vectorstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ClaudeEmbedder uses Claude CLI to semantically match queries against tools
type ClaudeEmbedder struct {
	model        string
	claudeBinary string
	logger       *slog.Logger
}

// NewClaudeEmbedder creates a new Claude-based embedder
func NewClaudeEmbedder(model string, logger *slog.Logger) (*ClaudeEmbedder, error) {
	// Default to haiku if not specified
	if model == "" {
		model = "haiku"
	}

	// Find claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found in PATH: %w", err)
	}

	logger.Info("Created Claude embedder", "model", model, "binary", claudePath)

	return &ClaudeEmbedder{
		model:        model,
		claudeBinary: claudePath,
		logger:       logger,
	}, nil
}

// Generate is not used for Claude embedder (we use direct search instead)
// This satisfies the EmbeddingGenerator interface but shouldn't be called
func (e *ClaudeEmbedder) Generate(text string) ([]float32, error) {
	return nil, fmt.Errorf("Claude embedder doesn't support Generate() - use Search() directly")
}

// Dimension returns a dummy dimension (Claude doesn't use vector embeddings)
func (e *ClaudeEmbedder) Dimension() int {
	return 0 // No vector embeddings
}

// SearchTools uses Claude to find relevant tools for a query
// Returns tool names ranked by relevance
func (e *ClaudeEmbedder) SearchTools(query string, toolSchemas []byte, topK int) ([]string, error) {
	// Build prompt for Claude
	prompt := fmt.Sprintf(`You are helping match a user query to the most relevant tools.

Given this query: "%s"

And these available tools (JSON array with name, description, category, parameters):
%s

Return ONLY a JSON array of the top %d most relevant tool names, ranked by relevance.
Format: ["tool_name_1", "tool_name_2", ...]

Consider:
- Semantic similarity between query and tool description
- Tool category and parameters
- Likely user intent

Return ONLY the JSON array, no explanation.`, query, string(toolSchemas), topK)

	// Call claude CLI with prompt as last argument
	cmd := exec.Command(
		e.claudeBinary,
		"--print",
		"--output-format", "json",
		"--model", e.model,
		"--dangerously-skip-permissions",
		"--tools", "", // Disable all tools
		"--", // End of options
		prompt,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Debug("Calling Claude CLI", "query", query, "topK", topK)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w, stderr: %s", err, stderr.String())
	}

	// Parse Claude's JSON response
	// The CLI returns: {"type":"result","result":"...", ...}
	var response struct {
		Type   string `json:"type"`
		Result string `json:"result"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse claude response: %w, output: %s", err, stdout.String())
	}

	if response.Result == "" {
		return nil, fmt.Errorf("no result in claude response")
	}

	responseText := response.Result

	// Parse the JSON array of tool names from Claude's response
	// Claude might wrap it in markdown code blocks, so clean that up
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var toolNames []string
	if err := json.Unmarshal([]byte(responseText), &toolNames); err != nil {
		return nil, fmt.Errorf("failed to parse tool names from claude: %w, text: %s", err, responseText)
	}

	e.logger.Info("Claude search completed", "query", query, "found", len(toolNames))

	return toolNames, nil
}
