package vectorstore

import (
	"log/slog"
	"math"
	"strings"
	"unicode"
)

// TFIDFEmbedder generates embeddings using TF-IDF weighting
// This is a pure Go implementation that works entirely in memory
type TFIDFEmbedder struct {
	vocabulary  map[string]int     // word -> index
	idf         map[string]float32 // word -> inverse document frequency
	dimension   int
	logger      *slog.Logger
	initialized bool
}

// NewTFIDFEmbedder creates a new TF-IDF based embedding generator
func NewTFIDFEmbedder(logger *slog.Logger) *TFIDFEmbedder {
	return &TFIDFEmbedder{
		vocabulary:  make(map[string]int),
		idf:         make(map[string]float32),
		logger:      logger,
		initialized: false,
	}
}

// BuildVocabulary builds the vocabulary and IDF scores from a corpus of documents
func (e *TFIDFEmbedder) BuildVocabulary(documents []string) {
	// Count document frequency for each word
	docFreq := make(map[string]int)
	totalDocs := len(documents)

	for _, doc := range documents {
		words := e.tokenize(doc)
		seen := make(map[string]bool)

		for _, word := range words {
			if !seen[word] {
				docFreq[word]++
				seen[word] = true
			}
		}
	}

	// Build vocabulary and calculate IDF
	idx := 0
	for word, freq := range docFreq {
		e.vocabulary[word] = idx
		// IDF = log((N + 1) / (df + 1)) + 1
		e.idf[word] = float32(math.Log(float64(totalDocs+1)/float64(freq+1))) + 1.0
		idx++
	}

	e.dimension = len(e.vocabulary)
	e.initialized = true

	e.logger.Info("Built TF-IDF vocabulary",
		"vocab_size", e.dimension,
		"documents", totalDocs)
}

// Generate creates an embedding vector for the given text
func (e *TFIDFEmbedder) Generate(text string) ([]float32, error) {
	if !e.initialized {
		// Return zero vector if not initialized
		return make([]float32, 1), nil
	}

	// Tokenize and count term frequency
	words := e.tokenize(text)
	termFreq := make(map[string]int)

	for _, word := range words {
		termFreq[word]++
	}

	// Create TF-IDF vector
	embedding := make([]float32, e.dimension)
	totalTerms := float32(len(words))

	for word, count := range termFreq {
		if idx, exists := e.vocabulary[word]; exists {
			// TF = count / total_terms
			tf := float32(count) / totalTerms
			// TF-IDF = TF * IDF
			embedding[idx] = tf * e.idf[word]
		}
	}

	// Normalize to unit length
	return e.normalize(embedding), nil
}

// Dimension returns the dimensionality of generated embeddings
func (e *TFIDFEmbedder) Dimension() int {
	if !e.initialized {
		return 0
	}
	return e.dimension
}

// tokenize converts text to lowercase tokens
func (e *TFIDFEmbedder) tokenize(text string) []string {
	text = strings.ToLower(text)

	// Split on whitespace and punctuation
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	// Filter out very short words and common stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"as": true, "is": true, "was": true, "are": true, "were": true,
		"be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
	}

	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 1 && !stopWords[word] {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

// normalize performs L2 normalization on the embedding
func (e *TFIDFEmbedder) normalize(embedding []float32) []float32 {
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		normalized := make([]float32, len(embedding))
		for i, val := range embedding {
			normalized[i] = val / norm
		}
		return normalized
	}

	return embedding
}
