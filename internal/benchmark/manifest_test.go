package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	data := `{"samples":[{"id":"one","category":"natural","audio":"audio/one.wav","expected":"hello"}]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "audio", "one.wav")
	if manifest.Samples[0].Audio != want {
		t.Fatalf("got %q, want %q", manifest.Samples[0].Audio, want)
	}
}

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"empty", `{"samples":[]}`, "at least one"},
		{"duplicate", `{"samples":[{"id":"x","category":"c","audio":"a"},{"id":"x","category":"c","audio":"b"}]}`, "duplicate"},
		{"missing audio", `{"samples":[{"id":"x","category":"c","audio":""}]}`, "audio is required"},
		{"unknown field", `{"samples":[{"id":"x","category":"c","audio":"a","secret":"x"}]}`, "unknown field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(path, []byte(test.json), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want substring %q", err, test.want)
			}
		})
	}
}
