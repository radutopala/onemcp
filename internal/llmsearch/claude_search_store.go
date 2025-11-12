package llmsearch

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/radutopala/onemcp/internal/tools"
)

// ClaudeSearchStore uses Claude CLI for semantic search without embeddings
type ClaudeSearchStore struct {
	embedder *ClaudeSearcher
	tools    []*tools.Tool
	schemas  []byte // Cached JSON schemas
	logger   *slog.Logger
}

// NewClaudeSearchStore creates a vector store that uses Claude CLI
func NewClaudeSearchStore(embedder *ClaudeSearcher, logger *slog.Logger) *ClaudeSearchStore {
	return &ClaudeSearchStore{
		embedder: embedder,
		tools:    make([]*tools.Tool, 0),
		logger:   logger,
	}
}

// BuildFromTools caches tool schemas for Claude queries
func (s *ClaudeSearchStore) BuildFromTools(allTools []*tools.Tool) error {
	s.logger.Info("Building Claude vector store", "tool_count", len(allTools))

	s.tools = allTools

	// Build tool metadata with full schemas for Claude
	toolSchemas := make([]tools.ToolMetadata, len(allTools))
	for i, tool := range allTools {
		metadata := tools.ToolMetadata{
			Name:        tool.Name,
			Category:    tool.Category,
			Description: tool.Description,
		}

		// Include full schema
		if tool.InputSchema != nil {
			if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
				metadata.Parameters = schemaMap
			}
		}

		toolSchemas[i] = metadata
	}

	// Marshal to JSON for Claude
	schemas, err := json.Marshal(toolSchemas)
	if err != nil {
		return fmt.Errorf("failed to marshal tool schemas: %w", err)
	}

	s.schemas = schemas

	s.logger.Info("Claude vector store built", "tool_count", len(s.tools), "schema_size_kb", len(schemas)/1024)

	return nil
}

// Search uses Claude CLI to find relevant tools
func (s *ClaudeSearchStore) Search(query string, topK int) ([]*tools.Tool, error) {
	if len(s.tools) == 0 {
		return []*tools.Tool{}, nil
	}

	// Ask Claude to rank tools
	toolNames, err := s.embedder.SearchTools(query, s.schemas, topK)
	if err != nil {
		return nil, fmt.Errorf("claude search failed: %w", err)
	}

	// Map tool names back to tool objects
	toolMap := make(map[string]*tools.Tool)
	for _, tool := range s.tools {
		toolMap[tool.Name] = tool
	}

	results := make([]*tools.Tool, 0, len(toolNames))
	for _, name := range toolNames {
		if tool, ok := toolMap[name]; ok {
			results = append(results, tool)
		}
	}

	s.logger.Debug("Claude search results", "query", query, "requested", topK, "returned", len(results))

	return results, nil
}

// GetToolCount returns the number of tools indexed
func (s *ClaudeSearchStore) GetToolCount() int {
	return len(s.tools)
}
