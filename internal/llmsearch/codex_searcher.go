package llmsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// CodexSearcher uses Codex CLI to semantically match queries against tools
type CodexSearcher struct {
	model       string
	codexBinary string
	logger      *slog.Logger
}

// NewCodexSearcher creates a new Codex-based embedder
func NewCodexSearcher(model string, logger *slog.Logger) (*CodexSearcher, error) {
	// Default to gpt-5-codex-mini if not specified
	if model == "" {
		model = "gpt-5-codex-mini"
	}

	// Find codex binary
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return nil, fmt.Errorf("codex CLI not found in PATH: %w", err)
	}

	logger.Info("Created Codex embedder", "model", model, "binary", codexPath)

	return &CodexSearcher{
		model:       model,
		codexBinary: codexPath,
		logger:      logger,
	}, nil
}

// Generate is not used for Codex embedder (we use direct search instead)
// This satisfies the EmbeddingGenerator interface but shouldn't be called
func (e *CodexSearcher) Generate(text string) ([]float32, error) {
	return nil, fmt.Errorf("Codex embedder doesn't support Generate() - use Search() directly")
}

// Dimension returns a dummy dimension (Codex doesn't use vector embeddings)
func (e *CodexSearcher) Dimension() int {
	return 0 // No vector embeddings
}

// SearchTools uses Codex to find relevant tools for a query
// Returns tool names ranked by relevance
func (e *CodexSearcher) SearchTools(query string, toolSchemas []byte, topK int) ([]string, error) {
	// Build prompt for Codex
	prompt := fmt.Sprintf(`You are helping match a user query to the most relevant tools.

Given this query: "%s"

And these available tools (JSON array with name, description, category, parameters):
%s

Return ONLY a JSON array of EXACTLY %d tool names, ranked by relevance.
Format: ["tool_name_1", "tool_name_2", ...]
IMPORTANT: Return no more and no less than %d tools.

Consider:
- Semantic similarity between query and tool description
- Tool category and parameters
- Likely user intent

Return ONLY the JSON array, no explanation.`, query, string(toolSchemas), topK, topK)

	// Call codex CLI with exec subcommand
	cmd := exec.Command(
		e.codexBinary,
		"exec",
		"--json",
		"--model", e.model,
		"--dangerously-bypass-approvals-and-sandbox",
		prompt,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Debug("Calling Codex CLI", "query", query, "topK", topK)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex CLI failed: %w, stderr: %s", err, stderr.String())
	}

	// Log raw response for debugging
	e.logger.Debug("Codex raw response", "stdout", stdout.String())

	// Parse Codex's JSON Lines response
	// The CLI returns multiple JSON objects, we need the one with type="item.completed" and item.type="agent_message"
	var responseText string
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event struct {
			Type string `json:"type"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}

		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip lines that don't parse
		}

		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			responseText = event.Item.Text
			e.logger.Debug("Parsed Codex response", "text", responseText)
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no agent_message in codex response: %s", stdout.String())
	}

	// Parse the JSON array of tool names from Codex's response
	// Codex might wrap it in markdown code blocks, so clean that up
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var toolNames []string
	if err := json.Unmarshal([]byte(responseText), &toolNames); err != nil {
		return nil, fmt.Errorf("failed to parse tool names from codex: %w, text: %s", err, responseText)
	}

	e.logger.Info("Codex search completed", "query", query, "found", len(toolNames))

	return toolNames, nil
}
