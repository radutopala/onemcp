package tools

import "strings"

// fuzzyMatch returns true if query fuzzy matches target.
// Uses Levenshtein distance with a threshold based on query length.
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}

	queryLower := strings.ToLower(query)
	targetLower := strings.ToLower(target)

	// Exact substring match (fast path)
	if strings.Contains(targetLower, queryLower) {
		return true
	}

	// Calculate allowed edit distance based on query length
	// Shorter queries get stricter matching
	maxDistance := len(queryLower) / 3
	if maxDistance < 1 {
		maxDistance = 1
	}
	if maxDistance > 3 {
		maxDistance = 3
	}

	// Split target by common delimiters (space, underscore, dash)
	words := strings.FieldsFunc(targetLower, func(r rune) bool {
		return r == ' ' || r == '_' || r == '-'
	})

	for _, word := range words {
		// Check if query matches word with fuzzy distance
		if levenshteinDistance(queryLower, word) <= maxDistance {
			return true
		}
	}

	return false
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
// This is the minimum number of single-character edits (insertions, deletions, or substitutions)
// required to change one string into another.
func levenshteinDistance(s1, s2 string) int {
	len1, len2 := len(s1), len(s2)

	// Create a 2D slice for dynamic programming
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first row and column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
