package tools

import "testing"

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"a", "a", 0},
		{"a", "b", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"navigate", "navgate", 1},      // missing 'i'
		{"screenshot", "screnshoot", 2}, // two chars different
	}

	for _, tt := range tests {
		result := levenshteinDistance(tt.s1, tt.s2)
		if result != tt.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d", tt.s1, tt.s2, result, tt.expected)
		}
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query    string
		target   string
		expected bool
		reason   string
	}{
		// Exact substring matches
		{"nav", "navigate", true, "exact substring"},
		{"screenshot", "take_screenshot", true, "exact substring"},
		{"browser", "browser_navigate", true, "exact substring"},

		// Case insensitive
		{"NAV", "navigate", true, "case insensitive"},
		{"Navigate", "browser_navigate", true, "case insensitive"},

		// Fuzzy matches (small typos)
		{"navgate", "navigate", true, "1 char missing"},
		{"navigat", "navigate", true, "1 char missing at end"},
		{"screnshoot", "screenshot", true, "2 chars different"},

		// Word boundary matches
		{"click", "Click on element", true, "word match in description"},
		{"take", "Take a screenshot", true, "word match at start"},

		// Should NOT match (too different)
		{"xyz", "navigate", false, "completely different"},
		{"aaaa", "bbbb", false, "no common chars"},
		{"navigate", "click", false, "too different"},

		// Empty query matches everything
		{"", "anything", true, "empty query"},

		// Short queries
		{"go", "goto", true, "short exact substring"},
		{"go", "navigate", false, "short query, no match"},
	}

	for _, tt := range tests {
		result := fuzzyMatch(tt.query, tt.target)
		if result != tt.expected {
			t.Errorf("fuzzyMatch(%q, %q) = %v, expected %v (%s)",
				tt.query, tt.target, result, tt.expected, tt.reason)
		}
	}
}

func TestFuzzyMatchInSearch(t *testing.T) {
	// Test that fuzzy matching works for tool names with underscores and prefixes
	tests := []struct {
		query    string
		toolName string
		expected bool
	}{
		{"browser", "playwright_browser_navigate", true},
		{"navigate", "playwright_browser_navigate", true},
		{"navgate", "playwright_browser_navigate", true}, // fuzzy
		{"click", "playwright_browser_click", true},
		{"clik", "playwright_browser_click", true}, // fuzzy (1 char different)
		{"screenshot", "playwright_browser_take_screenshot", true},
		{"scrshot", "playwright_browser_take_screenshot", false}, // too different
		{"xyz", "playwright_browser_navigate", false},
	}

	for _, tt := range tests {
		result := fuzzyMatch(tt.query, tt.toolName)
		if result != tt.expected {
			t.Errorf("fuzzyMatch(%q, %q) = %v, expected %v",
				tt.query, tt.toolName, result, tt.expected)
		}
	}
}
