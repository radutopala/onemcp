package vectorstore

import (
	"log/slog"
	"os"
	"testing"

	"github.com/radutopala/onemcp/internal/tools"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type VectorStoreTestSuite struct {
	suite.Suite
	logger *slog.Logger
	store  *InMemoryVectorStore
	tools  []*tools.Tool
}

func TestVectorStoreTestSuite(t *testing.T) {
	suite.Run(t, new(VectorStoreTestSuite))
}

func (s *VectorStoreTestSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create sample tools
	s.tools = []*tools.Tool{
		{
			Name:        "browser_navigate",
			Category:    "browser",
			Description: "Navigate to a URL in the browser",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"url": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "browser_screenshot",
			Category:    "browser",
			Description: "Take a screenshot of the current page",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"fullpage": map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Name:        "file_read",
			Category:    "filesystem",
			Description: "Read contents of a file",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "file_write",
			Category:    "filesystem",
			Description: "Write data to a file",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"path":    map[string]any{"type": "string"},
					"content": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "http_fetch",
			Category:    "network",
			Description: "Fetch data from an HTTP endpoint",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"url":    map[string]any{"type": "string"},
					"method": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "database_query",
			Category:    "database",
			Description: "Execute a SQL query against the database",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"sql": map[string]any{"type": "string"},
				},
			},
		},
	}

	// Create embedder and store
	embedder := NewTFIDFEmbedder(s.logger)
	s.store = NewInMemoryVectorStore(embedder, s.logger)

	// Build the vector store
	err := s.store.BuildFromTools(s.tools)
	require.NoError(s.T(), err)
}

func (s *VectorStoreTestSuite) TestBuildFromTools() {
	require.Equal(s.T(), len(s.tools), s.store.GetToolCount(), "All tools should be indexed")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_ExactMatch() {
	results, err := s.store.Search("browser navigation", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// First result should be browser_navigate
	require.Equal(s.T(), "browser_navigate", results[0].Name)
}

func (s *VectorStoreTestSuite) TestSemanticSearch_RelatedTerms() {
	results, err := s.store.Search("capture webpage image", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// Should find browser_screenshot (page/image are related to screenshot)
	found := false
	for _, result := range results {
		if result.Name == "browser_screenshot" {
			found = true
			break
		}
	}
	require.True(s.T(), found, "Should find browser_screenshot for 'capture webpage image'")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_FileOperations() {
	results, err := s.store.Search("read document", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// Should find file_read
	found := false
	for _, result := range results {
		if result.Name == "file_read" {
			found = true
			break
		}
	}
	require.True(s.T(), found, "Should find file_read for 'read document'")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_NetworkOperations() {
	results, err := s.store.Search("download data from web", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// Should find http_fetch
	found := false
	for _, result := range results {
		if result.Name == "http_fetch" {
			found = true
			break
		}
	}
	require.True(s.T(), found, "Should find http_fetch for 'download data from web'")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_DatabaseOperations() {
	results, err := s.store.Search("sql select statement", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// Should find database_query
	found := false
	for _, result := range results {
		if result.Name == "database_query" {
			found = true
			break
		}
	}
	require.True(s.T(), found, "Should find database_query for 'sql select statement'")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_TopKLimit() {
	// Request only 2 results
	results, err := s.store.Search("file", 2)
	require.NoError(s.T(), err)
	require.Len(s.T(), results, 2, "Should return exactly 2 results")
}

func (s *VectorStoreTestSuite) TestSemanticSearch_EmptyQuery() {
	results, err := s.store.Search("", 5)
	require.NoError(s.T(), err)
	// Empty query should still return results (based on overall similarity)
	require.NotEmpty(s.T(), results)
}

func (s *VectorStoreTestSuite) TestSemanticSearch_MultiWordQuery() {
	results, err := s.store.Search("navigate browser to website", 3)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), results)

	// Should prioritize browser_navigate
	require.Equal(s.T(), "browser_navigate", results[0].Name)
}

func (s *VectorStoreTestSuite) TestCosineSimilarity() {
	// Test identical vectors
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	sim := cosineSimilarity(a, b)
	require.InDelta(s.T(), 1.0, sim, 0.0001, "Identical vectors should have similarity of 1.0")

	// Test orthogonal vectors
	c := []float32{1.0, 0.0, 0.0}
	d := []float32{0.0, 1.0, 0.0}
	sim = cosineSimilarity(c, d)
	require.InDelta(s.T(), 0.0, sim, 0.0001, "Orthogonal vectors should have similarity of 0.0")

	// Test opposite vectors
	e := []float32{1.0, 0.0, 0.0}
	f := []float32{-1.0, 0.0, 0.0}
	sim = cosineSimilarity(e, f)
	require.InDelta(s.T(), -1.0, sim, 0.0001, "Opposite vectors should have similarity of -1.0")
}

func (s *VectorStoreTestSuite) TestCosineSimilarity_DifferentLengths() {
	a := []float32{1.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	sim := cosineSimilarity(a, b)
	require.Equal(s.T(), float32(0.0), sim, "Different length vectors should return 0")
}

func (s *VectorStoreTestSuite) TestCosineSimilarity_ZeroVectors() {
	a := []float32{0.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	sim := cosineSimilarity(a, b)
	require.Equal(s.T(), float32(0.0), sim, "Zero vector should return 0 similarity")
}
