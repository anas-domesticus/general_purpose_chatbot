package memory_service

import (
	"strings"
	"unicode"
)

// extractWords extracts and normalises words from text for indexing.
// Words are converted to lowercase and split on whitespace and punctuation.
func extractWords(text string) map[string]struct{} {
	result := make(map[string]struct{})

	// Split on whitespace and iterate
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if word != "" && len(word) > 1 { // Skip single-character words
			result[word] = struct{}{}
		}
	}

	return result
}

// wordsToSlice converts a word set to a slice.
func wordsToSlice(words map[string]struct{}) []string {
	result := make([]string, 0, len(words))
	for word := range words {
		result = append(result, word)
	}
	return result
}

// sliceToWords converts a slice to a word set.
func sliceToWords(slice []string) map[string]struct{} {
	result := make(map[string]struct{}, len(slice))
	for _, word := range slice {
		result[word] = struct{}{}
	}
	return result
}

// checkMapsIntersect checks if two word sets have any common elements.
func checkMapsIntersect(m1, m2 map[string]struct{}) bool {
	if len(m1) == 0 || len(m2) == 0 {
		return false
	}

	// Iterate over the smaller map for efficiency
	if len(m1) > len(m2) {
		m1, m2 = m2, m1
	}

	for k := range m1 {
		if _, ok := m2[k]; ok {
			return true
		}
	}

	return false
}
