package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInspectWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	dataBytes := uint32(32000)
	values := []any{[]byte("RIFF"), uint32(36) + dataBytes, []byte("WAVE"), []byte("fmt "), uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16), []byte("data"), dataBytes}
	for _, value := range values {
		if err := binary.Write(f, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.Write(make([]byte, dataBytes)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := InspectWAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Duration != time.Second || info.SampleRate != 16000 || info.Channels != 1 || info.BitsPerSample != 16 {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestInspectWAVRejectsMalformedAudio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.wav")
	if err := os.WriteFile(path, []byte("not wav"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWAV(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestInspectWAVRejectsEmptyAudio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	values := []any{[]byte("RIFF"), uint32(36), []byte("WAVE"), []byte("fmt "), uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16), []byte("data"), uint32(0)}
	for _, value := range values {
		if err := binary.Write(f, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWAV(path); err == nil {
		t.Fatal("expected empty audio error")
	}
}

func TestInspectWAVRejectsChunkLargerThanFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.wav")
	data := make([]byte, 20)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], 12)
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], ^uint32(0))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectWAV(path); err == nil {
		t.Fatal("expected oversized chunk rejection")
	}
}

func TestInspectWAVRejectsNonRegularFile(t *testing.T) {
	if _, err := InspectWAV(t.TempDir()); err == nil {
		t.Fatal("expected directory rejection")
	}
}
