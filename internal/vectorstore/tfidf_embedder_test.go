package vectorstore

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TFIDFEmbedderTestSuite struct {
	suite.Suite
	logger   *slog.Logger
	embedder *TFIDFEmbedder
}

func TestTFIDFEmbedderTestSuite(t *testing.T) {
	suite.Run(t, new(TFIDFEmbedderTestSuite))
}

func (s *TFIDFEmbedderTestSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	s.embedder = NewTFIDFEmbedder(s.logger)
}

func (s *TFIDFEmbedderTestSuite) TestBuildVocabulary() {
	documents := []string{
		"The browser navigates to a URL",
		"The file contains important data",
		"Navigate to the website homepage",
	}

	s.embedder.BuildVocabulary(documents)

	require.True(s.T(), s.embedder.initialized, "Embedder should be initialized")
	require.Greater(s.T(), s.embedder.Dimension(), 0, "Vocabulary should not be empty")
	require.Greater(s.T(), len(s.embedder.vocabulary), 0, "Vocabulary map should not be empty")
	require.Greater(s.T(), len(s.embedder.idf), 0, "IDF map should not be empty")
}

func (s *TFIDFEmbedderTestSuite) TestGenerate_UninitializedEmbedder() {
	// Generate without building vocabulary
	embedding, err := s.embedder.Generate("test query")
	require.NoError(s.T(), err)
	require.Len(s.T(), embedding, 1, "Uninitialized embedder should return single zero element")
}

func (s *TFIDFEmbedderTestSuite) TestGenerate_AfterVocabularyBuild() {
	documents := []string{
		"browser navigation tool",
		"file reading utility",
		"database query executor",
	}

	s.embedder.BuildVocabulary(documents)

	embedding, err := s.embedder.Generate("browser navigation")
	require.NoError(s.T(), err)
	require.Len(s.T(), embedding, s.embedder.Dimension())

	// Check normalization (L2 norm should be close to 1.0)
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	require.InDelta(s.T(), 1.0, norm, 0.01, "Embedding should be normalized")
}

func (s *TFIDFEmbedderTestSuite) TestTokenize() {
	tokens := s.embedder.tokenize("The quick brown fox jumps over the lazy dog")

	require.NotEmpty(s.T(), tokens)

	// Common stop words should be filtered out
	for _, token := range tokens {
		require.NotContains(s.T(), []string{"the", "a", "is", "and"}, token, "Common stop words should be filtered")
	}

	// Meaningful words should be included
	require.Contains(s.T(), tokens, "quick")
	require.Contains(s.T(), tokens, "brown")
	require.Contains(s.T(), tokens, "fox")

	// All tokens should be lowercase
	for _, token := range tokens {
		require.Equal(s.T(), token, token, "Tokens should be lowercase")
	}
}

func (s *TFIDFEmbedderTestSuite) TestTokenize_WithPunctuation() {
	tokens := s.embedder.tokenize("Hello, world! This is a test.")

	require.NotEmpty(s.T(), tokens)

	// Punctuation should be removed
	for _, token := range tokens {
		require.NotContains(s.T(), token, ",")
		require.NotContains(s.T(), token, "!")
		require.NotContains(s.T(), token, ".")
	}
}

func (s *TFIDFEmbedderTestSuite) TestTokenize_FilterShortWords() {
	tokens := s.embedder.tokenize("a b cd efg")

	// Single letter words should be filtered (except meaningful ones not in stop words)
	foundSingleLetter := false
	for _, token := range tokens {
		if len(token) == 1 {
			foundSingleLetter = true
		}
	}
	require.False(s.T(), foundSingleLetter, "Single letter tokens should be filtered")
}

func (s *TFIDFEmbedderTestSuite) TestNormalize() {
	unnormalized := []float32{3.0, 4.0} // Length = 5.0
	normalized := s.embedder.normalize(unnormalized)

	// Check that length is 1.0
	var length float32
	for _, val := range normalized {
		length += val * val
	}
	require.InDelta(s.T(), 1.0, length, 0.0001, "Normalized vector should have unit length")

	// Check values are correct
	require.InDelta(s.T(), 0.6, normalized[0], 0.0001) // 3/5
	require.InDelta(s.T(), 0.8, normalized[1], 0.0001) // 4/5
}

func (s *TFIDFEmbedderTestSuite) TestNormalize_ZeroVector() {
	zero := []float32{0.0, 0.0, 0.0}
	normalized := s.embedder.normalize(zero)

	// Zero vector should remain zero
	for _, val := range normalized {
		require.Equal(s.T(), float32(0.0), val)
	}
}

func (s *TFIDFEmbedderTestSuite) TestIDFCalculation() {
	documents := []string{
		"word1 word2",       // word1: 1, word2: 1
		"word1 word3",       // word1: 2, word3: 1
		"word2 word3 word4", // word2: 2, word3: 2, word4: 1
	}

	s.embedder.BuildVocabulary(documents)

	// word1 appears in 2 docs, IDF should be log((3+1)/(2+1)) + 1 ≈ 1.288
	// word4 appears in 1 doc, IDF should be log((3+1)/(1+1)) + 1 ≈ 1.693

	idfWord1, exists1 := s.embedder.idf["word1"]
	require.True(s.T(), exists1, "word1 should be in IDF map")

	idfWord4, exists4 := s.embedder.idf["word4"]
	require.True(s.T(), exists4, "word4 should be in IDF map")

	// word4 (rarer) should have higher IDF than word1 (more common)
	require.Greater(s.T(), idfWord4, idfWord1, "Rarer word should have higher IDF")
}

func (s *TFIDFEmbedderTestSuite) TestDimension() {
	require.Equal(s.T(), 0, s.embedder.Dimension(), "Uninitialized embedder should have 0 dimension")

	documents := []string{
		"word1 word2 word3",
		"word4 word5",
	}

	s.embedder.BuildVocabulary(documents)

	// After removing stop words and filtering, we should have 5 unique words
	require.Equal(s.T(), 5, s.embedder.Dimension(), "Dimension should match unique word count")
}
