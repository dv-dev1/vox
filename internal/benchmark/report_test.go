package benchmark

import (
	"strings"
	"testing"
	"time"
)

func TestMarkdownShowsMetricsWithoutLeakingTranscript(t *testing.T) {
	report := Report{
		GeneratedAt: time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC),
		Samples: []SampleResult{{
			ID:       "technical-001",
			Category: "technical-vocabulary",
			Runs: []ModeResult{
				{Mode: "greedy", Transcript: "PRIVATE GREEDY TEXT", TermRecall: Recall{Matched: 0, Total: 2}, RTF: 0.04},
				{Mode: "hotwords", Transcript: "PRIVATE HOTWORD TEXT", TermRecall: Recall{Matched: 1, Total: 2}, IdentifierRecall: Recall{Matched: 0, Total: 1}, RTF: 0.05},
			},
		}},
	}

	markdown := report.Markdown()
	if strings.Contains(markdown, "PRIVATE") {
		t.Fatal("versioned Markdown must not include recognized transcript text")
	}
	if !strings.Contains(markdown, "1/2 (50.0%)") {
		t.Fatal("expected measured context-prompt recall in report")
	}
}

func TestHotwordComparisonCountsChanges(t *testing.T) {
	report := Report{Samples: []SampleResult{
		{Runs: []ModeResult{{Mode: "greedy", WER: 0.5, TermRecall: Recall{Matched: 0}}, {Mode: "hotwords", WER: 0.25, TermRecall: Recall{Matched: 1}}}},
		{Runs: []ModeResult{{Mode: "greedy", WER: 0.1, TermRecall: Recall{Matched: 2}}, {Mode: "hotwords", WER: 0.2, TermRecall: Recall{Matched: 1}}}},
	}}

	comparison := report.hotwordComparison()
	if comparison.Pairs != 2 || comparison.TermImproved != 1 || comparison.TermRegressed != 1 || comparison.WERImproved != 1 || comparison.WERRegressed != 1 {
		t.Fatalf("unexpected comparison: %+v", comparison)
	}
}

func TestMarkdownUsesWhisperMetadata(t *testing.T) {
	report := Report{
		GeneratedAt: time.Date(2026, 8, 23, 5, 0, 0, 0, time.UTC),
		Samples: []SampleResult{{Runs: []ModeResult{
			{Mode: "greedy", TermRecall: Recall{Matched: 2, Total: 2}, RTF: 0.04},
			{Mode: "hotwords", TermRecall: Recall{Matched: 2, Total: 2}, RTF: 0.05},
		}}},
	}
	markdown := report.Markdown()
	for _, expected := range []string{"Whisper large-v3-turbo benchmark", "whisper.cpp 1.9.1", "initial context prompts", "Setup constraints", "Hallucinations"} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("missing %q", expected)
		}
	}
}

func TestMarkdownCountsHallucinations(t *testing.T) {
	report := Report{
		Samples: []SampleResult{{Runs: []ModeResult{
			{Mode: "greedy", Hallucination: true, TermRecall: Recall{Matched: 2, Total: 2}},
			{Mode: "hotwords", TermRecall: Recall{Matched: 2, Total: 2}},
		}}},
	}
	markdown := report.Markdown()
	if !strings.Contains(markdown, "| greedy | 1 | 0 |") || !strings.Contains(markdown, "| 1 |") {
		t.Fatal("expected aggregate hallucination count in report")
	}
}
