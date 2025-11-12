package llmsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// CopilotSearcher uses GitHub Copilot CLI to semantically match queries against tools
type CopilotSearcher struct {
	model         string
	copilotBinary string
	logger        *slog.Logger
}

// NewCopilotSearcher creates a new Copilot-based searcher
func NewCopilotSearcher(model string, logger *slog.Logger) (*CopilotSearcher, error) {
	// Default to claude-haiku-4.5
	if model == "" {
		model = "claude-haiku-4.5"
	}

	// Find copilot binary
	copilotPath, err := exec.LookPath("copilot")
	if err != nil {
		return nil, fmt.Errorf("copilot CLI not found in PATH: %w", err)
	}

	logger.Info("Created Copilot searcher", "model", model, "binary", copilotPath)

	return &CopilotSearcher{
		model:         model,
		copilotBinary: copilotPath,
		logger:        logger,
	}, nil
}

// SearchTools uses GitHub Copilot to find relevant tools for a query
// Returns tool names ranked by relevance
func (s *CopilotSearcher) SearchTools(query string, toolSchemas []byte, topK int) ([]string, error) {
	// Build prompt for Copilot
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

	// Call copilot CLI in non-interactive mode
	cmd := exec.Command(
		s.copilotBinary,
		"--model", s.model,
		"--allow-all-tools",
		"--prompt", prompt,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	s.logger.Debug("Calling Copilot CLI", "query", query, "topK", topK)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("copilot CLI failed: %w, stderr: %s", err, stderr.String())
	}

	// Log raw response for debugging
	s.logger.Debug("Copilot raw response", "stdout", stdout.String())

	// Copilot returns the response directly in stdout (not wrapped in JSON)
	responseText := stdout.String()

	// Parse the JSON array of tool names from Copilot's response
	// Clean up markdown code blocks if present
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var toolNames []string
	if err := json.Unmarshal([]byte(responseText), &toolNames); err != nil {
		return nil, fmt.Errorf("failed to parse tool names from copilot: %w, text: %s", err, responseText)
	}

	s.logger.Info("Copilot search completed", "query", query, "found", len(toolNames))

	return toolNames, nil
}
