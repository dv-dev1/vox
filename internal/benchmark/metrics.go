package benchmark

import (
	"strings"
	"unicode"
)

type WordError struct {
	Edits          int
	ReferenceWords int
}

func (w WordError) Rate() float64 {
	if w.ReferenceWords == 0 {
		if w.Edits == 0 {
			return 0
		}
		return 1
	}
	return float64(w.Edits) / float64(w.ReferenceWords)
}

func CalculateWER(expected, actual string) WordError {
	reference := normalizedWords(expected)
	hypothesis := normalizedWords(actual)
	previous := make([]int, len(hypothesis)+1)
	for column := range previous {
		previous[column] = column
	}
	for row, ref := range reference {
		current := make([]int, len(hypothesis)+1)
		current[0] = row + 1
		for column, hyp := range hypothesis {
			cost := 1
			if ref == hyp {
				cost = 0
			}
			current[column+1] = min(
				previous[column+1]+1,
				current[column]+1,
				previous[column]+cost,
			)
		}
		previous = current
	}
	return WordError{Edits: previous[len(hypothesis)], ReferenceWords: len(reference)}
}

type Recall struct {
	Matched int
	Total   int
}

func (r Recall) Rate() float64 {
	if r.Total == 0 {
		return 1
	}
	return float64(r.Matched) / float64(r.Total)
}

func TechnicalTermRecall(actual string, terms []string) Recall {
	actualWords := normalizedWords(actual)
	result := Recall{Total: len(terms)}
	for _, term := range terms {
		if containsWords(actualWords, normalizedWords(term)) {
			result.Matched++
		}
	}
	return result
}

func ExactIdentifierAccuracy(actual string, identifiers []string) Recall {
	result := Recall{Total: len(identifiers)}
	for _, identifier := range identifiers {
		if identifier != "" && strings.Contains(actual, identifier) {
			result.Matched++
		}
	}
	return result
}

func normalizedWords(value string) []string {
	value = strings.ToLower(value)
	return strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func containsWords(haystack, needle []string) bool {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return false
	}
	for start := 0; start <= len(haystack)-len(needle); start++ {
		matched := true
		for offset := range needle {
			if haystack[start+offset] != needle[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
