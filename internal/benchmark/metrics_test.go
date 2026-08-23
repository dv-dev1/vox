package benchmark

import "testing"

func TestCalculateWER(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		edits    int
		words    int
	}{
		{"exact ignores case and punctuation", "Use Zod.", "use zod", 0, 2},
		{"substitution", "use react now", "use vue now", 1, 3},
		{"insertion", "use react", "please use react", 1, 2},
		{"deletion", "use react now", "use react", 1, 3},
		{"empty reference hallucination", "", "hello", 1, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := CalculateWER(test.expected, test.actual)
			if got.Edits != test.edits || got.ReferenceWords != test.words {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestTechnicalTermRecallNormalizesCaseAndPunctuation(t *testing.T) {
	got := TechnicalTermRecall("Use REACT-hook form, with zod.", []string{"React Hook Form", "Zod", "Prisma"})
	if got.Matched != 2 || got.Total != 3 {
		t.Fatalf("got %#v", got)
	}
}

func TestTechnicalTermMatchingRequiresWholeWords(t *testing.T) {
	got := TechnicalTermRecall("The nextjsapp package", []string{"Next.js"})
	if got.Matched != 0 {
		t.Fatalf("false match: %#v", got)
	}
}

func TestExactIdentifierAccuracyIsCaseSensitive(t *testing.T) {
	got := ExactIdentifierAccuracy("Call parseConfig, not ParseConfig.", []string{"parseConfig", "settings.ts"})
	if got.Matched != 1 || got.Total != 2 {
		t.Fatalf("got %#v", got)
	}
}
