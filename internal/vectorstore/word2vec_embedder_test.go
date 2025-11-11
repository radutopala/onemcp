package vectorstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWord2VecEmbedder_Basic(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)
	require.NotNil(t, embedder)
	require.Equal(t, 50, embedder.GetDimension())
}

func TestWord2VecEmbedder_BuildVocabulary(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

	documents := []string{
		"navigate to website",
		"browse website pages",
		"take screenshot of page",
		"capture image of browser",
	}

	embedder.BuildVocabulary(documents)

	// Should have vocabulary
	require.Greater(t, embedder.GetVocabularySize(), 0)

	// Should exclude stop words (to, of)
	_, hasTo := embedder.vocabulary["to"]
	_, hasOf := embedder.vocabulary["of"]
	require.False(t, hasTo, "Stop word 'to' should be excluded")
	require.False(t, hasOf, "Stop word 'of' should be excluded")

	// Should include meaningful words
	_, hasNavigate := embedder.vocabulary["navigate"]
	_, hasScreenshot := embedder.vocabulary["screenshot"]
	require.True(t, hasNavigate, "Should include 'navigate'")
	require.True(t, hasScreenshot, "Should include 'screenshot'")
}

func TestWord2VecEmbedder_Generate(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

	documents := []string{
		"navigate to website",
		"browse website pages",
		"take screenshot of page",
		"capture image of browser",
	}

	embedder.BuildVocabulary(documents)

	// Generate embedding
	vec, err := embedder.Generate("navigate website")
	require.NoError(t, err)
	require.Equal(t, 50, len(vec))

	// Vector should be normalized
	var sum float32
	for _, v := range vec {
		sum += v * v
	}
	require.InDelta(t, 1.0, sum, 0.01, "Vector should be normalized")
}

func TestWord2VecEmbedder_Similarity(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 100)

	documents := []string{
		"take screenshot capture image picture photo",
		"screenshot of browser window page",
		"navigate browse go to website url",
		"navigate to webpage site",
		"read file content data",
		"write save file data",
	}

	embedder.BuildVocabulary(documents)

	// Generate embeddings for similar concepts
	vec1, _ := embedder.Generate("screenshot image")
	vec2, _ := embedder.Generate("capture photo")
	vec3, _ := embedder.Generate("navigate website")
	vec4, _ := embedder.Generate("read file")

	// Similar concepts should have higher similarity
	simScreenshot := cosineSimilarity(vec1, vec2)
	simCross1 := cosineSimilarity(vec1, vec3)
	simCross2 := cosineSimilarity(vec1, vec4)

	require.Greater(t, simScreenshot, simCross1, "screenshot-capture should be more similar than screenshot-navigate")
	require.Greater(t, simScreenshot, simCross2, "screenshot-capture should be more similar than screenshot-file")
}

func TestWord2VecEmbedder_EmptyText(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

	documents := []string{"some text"}
	embedder.BuildVocabulary(documents)

	vec, err := embedder.Generate("")
	require.NoError(t, err)
	require.Equal(t, 50, len(vec))
}

func TestWord2VecEmbedder_UnknownWords(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

	documents := []string{"navigate website"}
	embedder.BuildVocabulary(documents)

	// Query with unknown words should still work
	vec, err := embedder.Generate("unknown words that were not in training")
	require.NoError(t, err)
	require.Equal(t, 50, len(vec))
}

func TestWord2VecEmbedder_Tokenization(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

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

func TestWord2VecEmbedder_CooccurrenceWindow(t *testing.T) {
	embedder := NewWord2VecEmbedder(2, 50) // window size = 2

	documents := []string{
		"word1 word2 word3 word4 word5",
	}

	embedder.BuildVocabulary(documents)

	// Check that co-occurrence was built
	require.Greater(t, embedder.GetVocabularySize(), 0)

	// word1 should co-occur with word2, word3 (within window=2)
	idx1 := embedder.vocabulary["word1"]
	idx2 := embedder.vocabulary["word2"]
	idx3 := embedder.vocabulary["word3"]
	idx5 := embedder.vocabulary["word5"]

	// word1-word2 should have higher co-occurrence than word1-word5
	cooc12 := embedder.cooccurrence[idx1][idx2]
	cooc15 := embedder.cooccurrence[idx1][idx5]

	require.Greater(t, cooc12, float32(0), "word1 and word2 should co-occur")
	require.Equal(t, float32(0), cooc15, "word1 and word5 should not co-occur with window=2")

	// Adjacent words should have highest weight
	cooc23 := embedder.cooccurrence[idx2][idx3]
	require.Greater(t, cooc23, float32(0), "Adjacent words should co-occur")
}

func TestWord2VecEmbedder_LargeVocabulary(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 100)

	// Create 100 unique documents
	documents := make([]string, 100)
	for i := 0; i < 100; i++ {
		documents[i] = string(rune('a'+i%26)) + "tool does something useful"
	}

	embedder.BuildVocabulary(documents)

	require.Greater(t, embedder.GetVocabularySize(), 10)

	vec, err := embedder.Generate("tool does something")
	require.NoError(t, err)
	require.Equal(t, 100, len(vec))
}

func TestWord2VecEmbedder_NormalizationConsistency(t *testing.T) {
	embedder := NewWord2VecEmbedder(5, 50)

	documents := []string{
		"test document one",
		"test document two",
	}

	embedder.BuildVocabulary(documents)

	// Generate same embedding multiple times
	vec1, _ := embedder.Generate("test document")
	vec2, _ := embedder.Generate("test document")

	require.Equal(t, vec1, vec2, "Same input should produce same embedding")

	// Both should be normalized
	var sum1, sum2 float32
	for i := range vec1 {
		sum1 += vec1[i] * vec1[i]
		sum2 += vec2[i] * vec2[i]
	}
	require.InDelta(t, 1.0, sum1, 0.01)
	require.InDelta(t, 1.0, sum2, 0.01)
}
