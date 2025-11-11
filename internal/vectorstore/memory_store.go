package vectorstore

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"github.com/radutopala/onemcp/internal/tools"
)

// InMemoryVectorStore implements an in-memory vector store with semantic search
type InMemoryVectorStore struct {
	embeddings []*ToolEmbedding
	embedder   EmbeddingGenerator
	dimension  int
	logger     *slog.Logger
}

// NewInMemoryVectorStore creates a new in-memory vector store
func NewInMemoryVectorStore(embedder EmbeddingGenerator, logger *slog.Logger) *InMemoryVectorStore {
	return &InMemoryVectorStore{
		embeddings: make([]*ToolEmbedding, 0),
		embedder:   embedder,
		dimension:  embedder.Dimension(),
		logger:     logger,
	}
}

// BuildFromTools pre-computes embeddings for all tools
func (s *InMemoryVectorStore) BuildFromTools(allTools []*tools.Tool) error {
	s.logger.Info("Building vector store from tools", "tool_count", len(allTools))

	// First, if using TF-IDF, build vocabulary from all tool descriptions
	if tfidfEmbedder, ok := s.embedder.(*TFIDFEmbedder); ok {
		documents := make([]string, len(allTools))
		for i, tool := range allTools {
			documents[i] = s.createSearchableText(tool)
		}
		tfidfEmbedder.BuildVocabulary(documents)
		s.dimension = tfidfEmbedder.Dimension()
	}

	// Generate embeddings for each tool
	for _, tool := range allTools {
		searchText := s.createSearchableText(tool)

		embedding, err := s.embedder.Generate(searchText)
		if err != nil {
			s.logger.Warn("Failed to generate embedding for tool",
				"tool", tool.Name,
				"error", err)
			continue
		}

		s.embeddings = append(s.embeddings, &ToolEmbedding{
			Tool:      tool,
			Embedding: embedding,
		})
	}

	s.logger.Info("Vector store built successfully",
		"indexed_tools", len(s.embeddings),
		"dimension", s.dimension)

	return nil
}

// Search finds tools semantically similar to the query
func (s *InMemoryVectorStore) Search(query string, topK int) ([]*tools.Tool, error) {
	if len(s.embeddings) == 0 {
		return []*tools.Tool{}, nil
	}

	// Generate query embedding
	queryEmbedding, err := s.embedder.Generate(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Calculate cosine similarity for all tools
	for _, toolEmbed := range s.embeddings {
		toolEmbed.Score = cosineSimilarity(queryEmbedding, toolEmbed.Embedding)
	}

	// Sort by score (descending)
	sortedEmbeddings := make([]*ToolEmbedding, len(s.embeddings))
	copy(sortedEmbeddings, s.embeddings)

	sort.Slice(sortedEmbeddings, func(i, j int) bool {
		return sortedEmbeddings[i].Score > sortedEmbeddings[j].Score
	})

	// Return top K tools
	limit := topK
	if limit > len(sortedEmbeddings) {
		limit = len(sortedEmbeddings)
	}

	results := make([]*tools.Tool, limit)
	for i := 0; i < limit; i++ {
		results[i] = sortedEmbeddings[i].Tool
	}

	s.logger.Debug("Vector search completed",
		"query", query,
		"results", limit,
		"top_score", sortedEmbeddings[0].Score)

	return results, nil
}

// GetToolCount returns the number of tools indexed
func (s *InMemoryVectorStore) GetToolCount() int {
	return len(s.embeddings)
}

// createSearchableText creates a text representation of a tool for embedding
func (s *InMemoryVectorStore) createSearchableText(tool *tools.Tool) string {
	// Combine name, category, and description for better semantic search
	var parts []string

	// Tool name (with underscores replaced by spaces)
	name := strings.ReplaceAll(tool.Name, "_", " ")
	parts = append(parts, name)

	// Category
	if tool.Category != "" {
		parts = append(parts, tool.Category)
	}

	// Description
	if tool.Description != "" {
		parts = append(parts, tool.Description)
	}

	// Include parameter names if available (helps with search)
	if tool.InputSchema != nil {
		if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
			if props, ok := schemaMap["properties"].(map[string]any); ok {
				for paramName := range props {
					parts = append(parts, strings.ReplaceAll(paramName, "_", " "))
				}
			}
		}
	}

	return strings.Join(parts, ". ")
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
