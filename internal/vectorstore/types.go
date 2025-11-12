package vectorstore

import "github.com/radutopala/onemcp/internal/tools"

// ToolEmbedding represents a tool with its vector embedding
type ToolEmbedding struct {
	Tool      *tools.Tool // Reference to the original tool
	Embedding []float32   // Vector embedding
	Score     float32     // Similarity score (used during search)
}

// VectorStore defines the interface for vector-based search
type VectorStore interface {
	// BuildFromTools pre-computes embeddings/indexes for all tools
	BuildFromTools(allTools []*tools.Tool) error

	// Search finds tools semantically similar to the query
	Search(query string, topK int) ([]*tools.Tool, error)

	// GetToolCount returns the number of tools indexed
	GetToolCount() int
}

// EmbeddingGenerator defines the interface for generating embeddings
type EmbeddingGenerator interface {
	// Generate creates an embedding vector for the given text
	Generate(text string) ([]float32, error)

	// Dimension returns the dimensionality of generated embeddings
	Dimension() int
}
