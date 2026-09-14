package asr

import (
	"testing"
	"time"
)

func TestParseParakeetTimings(t *testing.T) {
	output := "parakeet_print_timings:     load time =   369.25 ms\n" +
		"parakeet_print_timings:    total time =  4337.50 ms\n"
	load, total := parseParakeetTimings(output)
	if load != 369250*time.Microsecond || total != 4337500*time.Microsecond {
		t.Fatalf("load=%s total=%s", load, total)
	}
}

func TestParakeetTextKeepsOneLine(t *testing.T) {
	if got := parakeetText("O objetivo\nprincipal  da ciência.\n"); got != "O objetivo principal da ciência." {
		t.Fatalf("got %q", got)
	}
}

func TestLimitedBufferCapsDiagnosticOutput(t *testing.T) {
	buffer := limitedBuffer{limit: 4}
	data := []byte("abcdef")
	written, err := buffer.Write(data)
	if err != nil || written != len(data) {
		t.Fatalf("write returned %d, %v", written, err)
	}
	if got := buffer.String(); got != "abcd" || !buffer.exceeded {
		t.Fatalf("buffer=%q exceeded=%v", got, buffer.exceeded)
	}
}
