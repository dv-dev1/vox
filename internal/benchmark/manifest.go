package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Manifest struct {
	Samples []Sample `json:"samples"`
}

type Sample struct {
	ID            string   `json:"id"`
	Category      string   `json:"category"`
	Audio         string   `json:"audio"`
	Expected      string   `json:"expected"`
	RequiredTerms []string `json:"required_terms,omitempty"`
	Hotwords      []string `json:"hotwords,omitempty"`
	Identifiers   []string `json:"identifiers,omitempty"`
	DurationSecs  float64  `json:"duration_seconds,omitempty"`
}

func LoadManifest(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Manifest{}, err
	}
	for index := range manifest.Samples {
		if !filepath.IsAbs(manifest.Samples[index].Audio) {
			manifest.Samples[index].Audio = filepath.Join(base, manifest.Samples[index].Audio)
		}
	}
	return manifest, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("manifest contains more than one JSON value")
		}
		return fmt.Errorf("decode trailing manifest data: %w", err)
	}
	return nil
}

func (m Manifest) Validate() error {
	if len(m.Samples) == 0 {
		return errors.New("manifest must contain at least one sample")
	}
	seen := make(map[string]struct{}, len(m.Samples))
	for index, sample := range m.Samples {
		prefix := fmt.Sprintf("sample %d", index)
		if strings.TrimSpace(sample.ID) == "" {
			return fmt.Errorf("%s: id is required", prefix)
		}
		if _, ok := seen[sample.ID]; ok {
			return fmt.Errorf("%s: duplicate id %q", prefix, sample.ID)
		}
		seen[sample.ID] = struct{}{}
		if strings.TrimSpace(sample.Category) == "" {
			return fmt.Errorf("%s (%s): category is required", prefix, sample.ID)
		}
		if strings.TrimSpace(sample.Audio) == "" {
			return fmt.Errorf("%s (%s): audio is required", prefix, sample.ID)
		}
		if sample.DurationSecs < 0 {
			return fmt.Errorf("%s (%s): duration_seconds cannot be negative", prefix, sample.ID)
		}
	}
	return nil
}
