package vectorstore

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GloVeEmbedder implements embeddings using pre-trained GloVe vectors
type GloVeEmbedder struct {
	vectors map[string][]float32
	dim     int
	logger  *slog.Logger
}

// GloVeModelConfig holds configuration for a GloVe model
type GloVeModelConfig struct {
	URL      string
	Filename string
	Dim      int
}

// GloVe model configurations
var gloveModels = map[string]GloVeModelConfig{
	"6B.50d":  {"https://archive.org/download/glove.6B.50d-300d/glove.6B.50d.txt", "glove.6B.50d.txt", 50},
	"6B.100d": {"https://archive.org/download/glove.6B.50d-300d/glove.6B.100d.txt", "glove.6B.100d.txt", 100},
	"6B.200d": {"https://archive.org/download/glove.6B.50d-300d/glove.6B.200d.txt", "glove.6B.200d.txt", 200},
	"6B.300d": {"https://archive.org/download/glove.6B.50d-300d/glove.6B.300d.txt", "glove.6B.300d.txt", 300},
}

// GetGloVeModelConfig returns the configuration for a named GloVe model
func GetGloVeModelConfig(modelName string) (GloVeModelConfig, bool) {
	config, ok := gloveModels[modelName]
	return config, ok
}

// NewGloVeEmbedder creates a new GloVe embedder
// modelName: "6B.50d", "6B.100d", "6B.200d", or "6B.300d"
// cacheDir: directory to store downloaded models (e.g., "/tmp/glove")
func NewGloVeEmbedder(modelName string, cacheDir string, logger *slog.Logger) (*GloVeEmbedder, error) {
	modelConfig, ok := gloveModels[modelName]
	if !ok {
		return nil, fmt.Errorf("unknown GloVe model: %s (available: 6B.50d, 6B.100d, 6B.200d, 6B.300d)", modelName)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Check if model file already exists
	modelPath := filepath.Join(cacheDir, modelConfig.Filename)

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		logger.Info("GloVe model not found, downloading...", "model", modelName, "path", modelPath)

		// Download file directly (no ZIP extraction needed)
		if err := downloadGloVe(modelConfig.URL, modelPath, logger); err != nil {
			return nil, fmt.Errorf("failed to download GloVe model: %w", err)
		}

		logger.Info("GloVe model downloaded successfully", "model", modelName)
	} else {
		logger.Info("Using cached GloVe model", "model", modelName, "path", modelPath)
	}

	// Load vectors from file
	vectors, err := loadGloVeVectors(modelPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load GloVe vectors: %w", err)
	}

	logger.Info("GloVe embedder ready", "model", modelName, "vocabulary_size", len(vectors), "dimension", modelConfig.Dim)

	return &GloVeEmbedder{
		vectors: vectors,
		dim:     modelConfig.Dim,
		logger:  logger,
	}, nil
}

// downloadGloVe downloads a GloVe model file directly from Internet Archive
func downloadGloVe(url, destPath string, logger *slog.Logger) error {
	logger.Info("Downloading GloVe model...", "url", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy with progress logging
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Info("Download complete", "size_mb", written/(1024*1024))

	return nil
}

// loadGloVeVectors loads GloVe vectors from a text file
func loadGloVeVectors(path string, logger *slog.Logger) (map[string][]float32, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	vectors := make(map[string][]float32)
	scanner := bufio.NewScanner(file)

	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount%10000 == 0 {
			logger.Info("Loading GloVe vectors...", "loaded", lineCount)
		}

		line := scanner.Text()
		parts := strings.Fields(line)

		if len(parts) < 2 {
			continue // Skip malformed lines
		}

		word := parts[0]
		vec := make([]float32, len(parts)-1)

		for i, s := range parts[1:] {
			val, err := strconv.ParseFloat(s, 32)
			if err != nil {
				continue // Skip if parsing fails
			}
			vec[i] = float32(val)
		}

		vectors[word] = vec
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	logger.Info("GloVe vectors loaded", "total_words", len(vectors))

	return vectors, nil
}

// Generate creates an embedding by averaging word vectors
func (e *GloVeEmbedder) Generate(text string) ([]float32, error) {
	words := e.tokenize(text)
	if len(words) == 0 {
		return make([]float32, e.dim), nil
	}

	// Average word vectors
	embedding := make([]float32, e.dim)
	count := 0

	for _, word := range words {
		if vec, ok := e.vectors[word]; ok {
			for i := 0; i < e.dim; i++ {
				embedding[i] += vec[i]
			}
			count++
		}
	}

	// Average
	if count > 0 {
		for i := range embedding {
			embedding[i] /= float32(count)
		}
	}

	return normalize(embedding), nil
}

// tokenize splits text into lowercase words
func (e *GloVeEmbedder) tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	// Filter out very short words and stop words
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true,
		"at": true, "be": true, "by": true, "for": true, "from": true,
		"has": true, "he": true, "in": true, "is": true, "it": true,
		"its": true, "of": true, "on": true, "that": true, "the": true,
		"this": true, "to": true, "was": true, "will": true, "with": true,
	}

	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) > 1 && !stopWords[word] {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

// Dimension returns the embedding dimension
func (e *GloVeEmbedder) Dimension() int {
	return e.dim
}

// GetVocabularySize returns the number of words in the vocabulary
func (e *GloVeEmbedder) GetVocabularySize() int {
	return len(e.vectors)
}

// normalize normalizes a vector to unit length
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
