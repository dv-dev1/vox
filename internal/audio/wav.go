package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

type WAVInfo struct {
	Channels      uint16
	SampleRate    uint32
	BitsPerSample uint16
	DataBytes     uint32
	Duration      time.Duration
}

// InspectWAV validates the subset accepted by the ASR path: mono, 16-bit PCM
// WAV. Chunk order and optional metadata chunks are supported.
func InspectWAV(path string) (WAVInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return WAVInfo{}, fmt.Errorf("open audio: %w", err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return WAVInfo{}, fmt.Errorf("inspect audio: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return WAVInfo{}, errors.New("audio must be a regular file")
	}
	fileSize := stat.Size()
	if fileSize < 12 {
		return WAVInfo{}, errors.New("audio is too short to be a WAV file")
	}

	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return WAVInfo{}, fmt.Errorf("read WAV header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return WAVInfo{}, errors.New("audio is not a RIFF/WAVE file")
	}
	if declared := int64(binary.LittleEndian.Uint32(header[4:8])) + 8; declared > fileSize {
		return WAVInfo{}, errors.New("WAV RIFF size exceeds the file size")
	}

	var info WAVInfo
	var audioFormat uint16
	for {
		offset, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return WAVInfo{}, fmt.Errorf("locate WAV chunk: %w", err)
		}
		remaining := fileSize - offset
		if remaining == 0 {
			break
		}
		if remaining < 8 {
			return WAVInfo{}, errors.New("truncated WAV chunk header")
		}
		var chunk [8]byte
		if _, err := io.ReadFull(f, chunk[:]); err != nil {
			return WAVInfo{}, fmt.Errorf("read WAV chunk: %w", err)
		}
		size := binary.LittleEndian.Uint32(chunk[4:8])
		paddedSize := int64(size) + int64(size%2)
		if paddedSize > remaining-8 {
			return WAVInfo{}, fmt.Errorf("WAV chunk %q exceeds the file size", string(chunk[0:4]))
		}
		switch string(chunk[0:4]) {
		case "fmt ":
			if size < 16 {
				return WAVInfo{}, errors.New("invalid WAV fmt chunk")
			}
			var buf [16]byte
			if _, err := io.ReadFull(f, buf[:]); err != nil {
				return WAVInfo{}, fmt.Errorf("read WAV format: %w", err)
			}
			audioFormat = binary.LittleEndian.Uint16(buf[0:2])
			info.Channels = binary.LittleEndian.Uint16(buf[2:4])
			info.SampleRate = binary.LittleEndian.Uint32(buf[4:8])
			info.BitsPerSample = binary.LittleEndian.Uint16(buf[14:16])
			if size > 16 {
				if _, err := f.Seek(int64(size-16), io.SeekCurrent); err != nil {
					return WAVInfo{}, fmt.Errorf("seek WAV format extension: %w", err)
				}
			}
		case "data":
			if uint64(info.DataBytes)+uint64(size) > uint64(^uint32(0)) {
				return WAVInfo{}, errors.New("WAV audio data is too large")
			}
			info.DataBytes += size
			if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
				return WAVInfo{}, fmt.Errorf("seek WAV data: %w", err)
			}
		default:
			if _, err := f.Seek(int64(size), io.SeekCurrent); err != nil {
				return WAVInfo{}, fmt.Errorf("seek WAV chunk: %w", err)
			}
		}
		if size%2 == 1 {
			_, _ = f.Seek(1, io.SeekCurrent)
		}
	}

	if audioFormat != 1 {
		return WAVInfo{}, fmt.Errorf("unsupported WAV encoding %d: want PCM", audioFormat)
	}
	if info.Channels != 1 {
		return WAVInfo{}, fmt.Errorf("unsupported channel count %d: want mono", info.Channels)
	}
	if info.BitsPerSample != 16 {
		return WAVInfo{}, fmt.Errorf("unsupported sample size %d: want 16-bit PCM", info.BitsPerSample)
	}
	if info.SampleRate == 0 || info.DataBytes == 0 {
		return WAVInfo{}, errors.New("WAV contains no audio samples")
	}
	if info.DataBytes%uint32(info.Channels*(info.BitsPerSample/8)) != 0 {
		return WAVInfo{}, errors.New("WAV data is not aligned to complete samples")
	}
	bytesPerSecond := float64(info.SampleRate) * float64(info.Channels) * float64(info.BitsPerSample/8)
	info.Duration = time.Duration(float64(info.DataBytes) / bytesPerSecond * float64(time.Second))
	return info, nil
}
