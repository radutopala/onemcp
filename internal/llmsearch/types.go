package llmsearch

import "github.com/radutopala/onemcp/internal/tools"

// SearchStore defines the interface for LLM-based semantic search
type SearchStore interface {
	// BuildFromTools prepares the search store with all available tools
	BuildFromTools(allTools []*tools.Tool) error

	// Search finds tools semantically similar to the query using LLM
	Search(query string, topK int) ([]*tools.Tool, error)

	// GetToolCount returns the number of tools indexed
	GetToolCount() int
}
