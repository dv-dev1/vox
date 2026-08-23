//go:build ignore

// Generate deterministic local audio fixtures without external audio tools.
package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
)

func main() {
	dir := filepath.Join(".local", "fixtures")
	must(os.MkdirAll(dir, 0o700))
	must(writeSilence(filepath.Join(dir, "silence-1s.wav"), 16000))
	must(writeSilence(filepath.Join(dir, "empty.wav"), 0))
	must(os.WriteFile(filepath.Join(dir, "malformed.wav"), []byte("not a wav\n"), 0o600))
}

func writeSilence(path string, samples uint32) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	dataBytes := samples * 2
	values := []any{
		[]byte("RIFF"), uint32(36) + dataBytes, []byte("WAVE"),
		[]byte("fmt "), uint32(16), uint16(1), uint16(1), uint32(16000),
		uint32(32000), uint16(2), uint16(16), []byte("data"), dataBytes,
	}
	for _, value := range values {
		if err := binary.Write(f, binary.LittleEndian, value); err != nil {
			return err
		}
	}
	_, err = f.Write(make([]byte, dataBytes))
	return err
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
