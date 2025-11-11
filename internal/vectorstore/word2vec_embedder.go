package vectorstore

import (
	"math"
	"strings"
	"unicode"
)

// Word2VecEmbedder implements word embeddings using co-occurrence matrix
// This is a simplified word2vec-style approach that learns word relationships
// from how often words appear together in the corpus.
type Word2VecEmbedder struct {
	vocabulary   map[string]int  // word -> vocabulary index
	cooccurrence [][]float32     // co-occurrence matrix
	wordVectors  [][]float32     // final word embeddings
	dimension    int             // embedding dimension
	windowSize   int             // context window size
	stopWords    map[string]bool // common words to ignore
}

// NewWord2VecEmbedder creates a new word2vec-style embedder
func NewWord2VecEmbedder(windowSize, dimension int) *Word2VecEmbedder {
	return &Word2VecEmbedder{
		vocabulary: make(map[string]int),
		dimension:  dimension,
		windowSize: windowSize,
		stopWords:  buildStopWords(),
	}
}

// BuildVocabulary builds the vocabulary and trains embeddings from documents
func (e *Word2VecEmbedder) BuildVocabulary(documents []string) {
	// Step 1: Build vocabulary
	allWords := make(map[string]int) // word -> total frequency

	for _, doc := range documents {
		words := e.tokenize(doc)
		seen := make(map[string]bool)
		for _, word := range words {
			if !seen[word] {
				allWords[word]++
				seen[word] = true
			}
		}
	}

	// Create vocabulary mapping
	idx := 0
	for word := range allWords {
		e.vocabulary[word] = idx
		idx++
	}

	vocabSize := len(e.vocabulary)

	// Step 2: Build co-occurrence matrix
	e.cooccurrence = make([][]float32, vocabSize)
	for i := range e.cooccurrence {
		e.cooccurrence[i] = make([]float32, vocabSize)
	}

	// Count co-occurrences within window
	for _, doc := range documents {
		words := e.tokenize(doc)

		for i, word1 := range words {
			idx1, ok1 := e.vocabulary[word1]
			if !ok1 {
				continue
			}

			// Look at words within window
			start := max(0, i-e.windowSize)
			end := min(len(words), i+e.windowSize+1)

			for j := start; j < end; j++ {
				if i == j {
					continue
				}

				word2 := words[j]
				idx2, ok2 := e.vocabulary[word2]
				if !ok2 {
					continue
				}

				// Weight by distance (closer words have more weight)
				distance := abs(i - j)
				weight := 1.0 / float32(distance)

				e.cooccurrence[idx1][idx2] += weight
			}
		}
	}

	// Step 3: Create word vectors using PMI (Positive Pointwise Mutual Information)
	e.wordVectors = make([][]float32, vocabSize)

	// Calculate word frequencies
	wordFreq := make([]float32, vocabSize)
	totalCooc := float32(0)
	for i := range e.cooccurrence {
		for j := range e.cooccurrence[i] {
			wordFreq[i] += e.cooccurrence[i][j]
			totalCooc += e.cooccurrence[i][j]
		}
	}

	// Create embeddings using truncated SVD approximation
	// We use co-occurrence patterns as features
	for i := range e.wordVectors {
		e.wordVectors[i] = make([]float32, e.dimension)

		// Get top co-occurring words as features
		topCooc := e.getTopCooccurrences(i, e.dimension)

		for dim := 0; dim < e.dimension && dim < len(topCooc); dim++ {
			wordIdx := topCooc[dim]

			// Calculate PMI: log(P(i,j) / (P(i) * P(j)))
			pij := e.cooccurrence[i][wordIdx] / totalCooc
			pi := wordFreq[i] / totalCooc
			pj := wordFreq[wordIdx] / totalCooc

			if pij > 0 && pi > 0 && pj > 0 {
				pmi := float32(math.Log(float64(pij / (pi * pj))))
				// Use positive PMI
				if pmi > 0 {
					e.wordVectors[i][dim] = pmi
				}
			}
		}

		// Normalize vector
		e.wordVectors[i] = normalize(e.wordVectors[i])
	}
}

// Generate creates an embedding for the given text by averaging word vectors
func (e *Word2VecEmbedder) Generate(text string) ([]float32, error) {
	words := e.tokenize(text)
	if len(words) == 0 {
		return make([]float32, e.dimension), nil
	}

	// Average word vectors
	embedding := make([]float32, e.dimension)
	count := 0

	for _, word := range words {
		if idx, ok := e.vocabulary[word]; ok {
			for i := 0; i < e.dimension; i++ {
				embedding[i] += e.wordVectors[idx][i]
			}
			count++
		}
	}

	if count > 0 {
		for i := range embedding {
			embedding[i] /= float32(count)
		}
	}

	return normalize(embedding), nil
}

// getTopCooccurrences returns indices of words with highest co-occurrence
func (e *Word2VecEmbedder) getTopCooccurrences(wordIdx int, topK int) []int {
	type pair struct {
		idx   int
		score float32
	}

	pairs := make([]pair, 0, len(e.cooccurrence[wordIdx]))
	for i, score := range e.cooccurrence[wordIdx] {
		if score > 0 && i != wordIdx {
			pairs = append(pairs, pair{i, score})
		}
	}

	// Simple selection sort for top K
	result := make([]int, 0, topK)
	for k := 0; k < topK && k < len(pairs); k++ {
		maxIdx := k
		for i := k + 1; i < len(pairs); i++ {
			if pairs[i].score > pairs[maxIdx].score {
				maxIdx = i
			}
		}
		pairs[k], pairs[maxIdx] = pairs[maxIdx], pairs[k]
		result = append(result, pairs[k].idx)
	}

	return result
}

// tokenize splits text into words and removes stop words
func (e *Word2VecEmbedder) tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 1 && !e.isStopWord(word) {
			filtered = append(filtered, word)
		}
	}
	return filtered
}

// isStopWord checks if a word is a common stop word
func (e *Word2VecEmbedder) isStopWord(word string) bool {
	return e.stopWords[word]
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func normalize(vec []float32) []float32 {
	var sum float32
	for _, v := range vec {
		sum += v * v
	}

	if sum == 0 {
		return vec
	}

	norm := float32(math.Sqrt(float64(sum)))
	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = v / norm
	}
	return result
}

// buildStopWords returns a set of common English stop words
func buildStopWords() map[string]bool {
	words := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for",
		"from", "has", "he", "in", "is", "it", "its", "of", "on",
		"that", "the", "this", "to", "was", "will", "with",
	}

	stopWords := make(map[string]bool)
	for _, word := range words {
		stopWords[word] = true
	}
	return stopWords
}

// Dimension returns the embedding dimension (implements EmbeddingGenerator interface)
func (e *Word2VecEmbedder) Dimension() int {
	return e.dimension
}

// GetDimension returns the embedding dimension (deprecated: use Dimension)
func (e *Word2VecEmbedder) GetDimension() int {
	return e.dimension
}

// GetVocabularySize returns the size of the vocabulary
func (e *Word2VecEmbedder) GetVocabularySize() int {
	return len(e.vocabulary)
}
