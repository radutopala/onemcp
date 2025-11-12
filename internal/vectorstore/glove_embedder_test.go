package vectorstore

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGloVeEmbedder_ModelValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Test invalid model name
	_, err := NewGloVeEmbedder("invalid-model", "/tmp/test-glove", logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown GloVe model")
}

func TestGloVeEmbedder_Tokenization(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: make(map[string][]float32),
		dim:     100,
		logger:  logger,
	}

	tokens := embedder.tokenize("Hello, World! This is a Test-123.")

	// Should be lowercase
	require.NotContains(t, tokens, "Hello")
	require.NotContains(t, tokens, "World")

	// Should contain actual words (stop words removed)
	require.Contains(t, tokens, "hello")
	require.Contains(t, tokens, "world")
	require.Contains(t, tokens, "test")
	require.Contains(t, tokens, "123")

	// Stop words should be removed
	require.NotContains(t, tokens, "this")
	require.NotContains(t, tokens, "is")
	require.NotContains(t, tokens, "a")
}

func TestGloVeEmbedder_GenerateWithMockVectors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create embedder with mock vectors
	embedder := &GloVeEmbedder{
		vectors: map[string][]float32{
			"screenshot": {0.1, 0.2, 0.3},
			"browser":    {0.4, 0.5, 0.6},
			"navigate":   {0.7, 0.8, 0.9},
		},
		dim:    3,
		logger: logger,
	}

	// Generate embedding
	vec, err := embedder.Generate("screenshot browser")
	require.NoError(t, err)
	require.Equal(t, 3, len(vec))

	// Should be average of screenshot and browser vectors
	// (0.1+0.4)/2, (0.2+0.5)/2, (0.3+0.6)/2 = 0.25, 0.35, 0.45
	// Then normalized
	require.Greater(t, vec[0], float32(0.0))
	require.Greater(t, vec[1], float32(0.0))
	require.Greater(t, vec[2], float32(0.0))
}

func TestGloVeEmbedder_GenerateEmpty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: make(map[string][]float32),
		dim:     50,
		logger:  logger,
	}

	vec, err := embedder.Generate("")
	require.NoError(t, err)
	require.Equal(t, 50, len(vec))

	// Should be zero vector
	for _, v := range vec {
		require.Equal(t, float32(0.0), v)
	}
}

func TestGloVeEmbedder_GenerateUnknownWords(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: map[string][]float32{
			"known": {1.0, 2.0, 3.0},
		},
		dim:    3,
		logger: logger,
	}

	// Query with unknown words only
	vec, err := embedder.Generate("unknown words")
	require.NoError(t, err)
	require.Equal(t, 3, len(vec))

	// Should return zero vector (no known words)
	for _, v := range vec {
		require.Equal(t, float32(0.0), v)
	}
}

func TestGloVeEmbedder_Dimension(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: make(map[string][]float32),
		dim:     200,
		logger:  logger,
	}

	require.Equal(t, 200, embedder.Dimension())
}

func TestGloVeEmbedder_VocabularySize(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: map[string][]float32{
			"word1": {1.0},
			"word2": {2.0},
			"word3": {3.0},
		},
		dim:    1,
		logger: logger,
	}

	require.Equal(t, 3, embedder.GetVocabularySize())
}

func TestLoadGloVeVectors_MockFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a mock GloVe file
	tmpDir := t.TempDir()
	mockFile := filepath.Join(tmpDir, "glove.mock.txt")

	content := `the 0.1 0.2 0.3
is 0.4 0.5 0.6
hello 0.7 0.8 0.9
world 1.0 1.1 1.2
`
	err := os.WriteFile(mockFile, []byte(content), 0644)
	require.NoError(t, err)

	// Load vectors
	vectors, err := loadGloVeVectors(mockFile, logger)
	require.NoError(t, err)
	require.Equal(t, 4, len(vectors))

	// Check specific vectors
	require.Contains(t, vectors, "the")
	require.Contains(t, vectors, "hello")
	require.Equal(t, []float32{0.7, 0.8, 0.9}, vectors["hello"])
}

func TestGloVeEmbedder_NormalizationConsistency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	embedder := &GloVeEmbedder{
		vectors: map[string][]float32{
			"test": {3.0, 4.0, 0.0}, // Will normalize to length 1
		},
		dim:    3,
		logger: logger,
	}

	// Generate same embedding multiple times
	vec1, _ := embedder.Generate("test")
	vec2, _ := embedder.Generate("test")

	require.Equal(t, vec1, vec2, "Same input should produce same embedding")

	// Check normalization (should have length ~1)
	var length float32
	for _, v := range vec1 {
		length += v * v
	}
	require.InDelta(t, 1.0, length, 0.01, "Vector should be normalized to length 1")
}

func TestGloVeEmbedder_ModelNames(t *testing.T) {
	testCases := []struct {
		modelName string
		valid     bool
		dim       int
	}{
		{"6B.50d", true, 50},
		{"6B.100d", true, 100},
		{"6B.200d", true, 200},
		{"6B.300d", true, 300},
		{"invalid", false, 0},
		{"", false, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.modelName, func(t *testing.T) {
			config, ok := gloveModels[tc.modelName]
			if tc.valid {
				require.True(t, ok, "Model should be valid")
				require.Equal(t, tc.dim, config.Dim)
			} else {
				require.False(t, ok, "Model should be invalid")
			}
		})
	}
}
