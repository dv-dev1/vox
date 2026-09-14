package asr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	voxaudio "github.com/yuribodo/vox/internal/audio"
)

type ParakeetCPPConfig struct {
	Binary  string
	Model   string
	Threads int
}

type ParakeetCPP struct {
	config ParakeetCPPConfig
}

var (
	parakeetLoadTimePattern  = regexp.MustCompile(`parakeet_print_timings:\s+load time =\s+([0-9.]+) ms`)
	parakeetTotalTimePattern = regexp.MustCompile(`parakeet_print_timings:\s+total time =\s+([0-9.]+) ms`)
)

func NewParakeetCPP(config ParakeetCPPConfig) (*ParakeetCPP, error) {
	if config.Binary == "" || config.Model == "" {
		return nil, errors.New("parakeet binary and model are required")
	}
	if _, err := os.Stat(config.Binary); err != nil {
		return nil, fmt.Errorf("parakeet binary: %w", err)
	}
	if _, err := os.Stat(config.Model); err != nil {
		return nil, fmt.Errorf("parakeet model: %w", err)
	}
	if config.Threads < 1 {
		config.Threads = 4
	}
	return &ParakeetCPP{config: config}, nil
}

// Transcribe ignora request.HotwordsPath: o Parakeet não aceita prompt de
// vocabulário, então hotwords.txt não tem efeito neste motor.
func (p *ParakeetCPP) Transcribe(ctx context.Context, request Request) (Result, error) {
	info, err := voxaudio.InspectWAV(request.AudioPath)
	if err != nil {
		return Result{}, err
	}
	started := time.Now()
	cmd := exec.CommandContext(ctx, p.config.Binary,
		"-t", strconv.Itoa(p.config.Threads),
		"-m", p.config.Model,
		"-f", request.AudioPath,
	)
	stdout := limitedBuffer{limit: 1024 * 1024}
	stderr := limitedBuffer{limit: 1024 * 1024}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("parakeet failed: %w: %s", err, tail(stderr.String(), 1200))
	}
	if stdout.exceeded || stderr.exceeded {
		return Result{}, errors.New("parakeet output exceeds 1 MiB")
	}
	latency := time.Since(started)
	load, total := parseParakeetTimings(stderr.String())
	inference := total - load
	if inference <= 0 {
		inference = latency
	}
	return Result{
		Text:             parakeetText(stdout.String()),
		AudioDuration:    info.Duration,
		Latency:          latency,
		ModelLoadLatency: load,
		InferenceLatency: inference,
		RealTimeFactor:   inference.Seconds() / info.Duration.Seconds(),
	}, nil
}

// A saída pode vir quebrada em linhas, e cada \n colado vira Enter na janela
// de destino.
func parakeetText(output string) string {
	return strings.Join(strings.Fields(output), " ")
}

func parseParakeetTimings(output string) (time.Duration, time.Duration) {
	return parseMilliseconds(output, parakeetLoadTimePattern), parseMilliseconds(output, parakeetTotalTimePattern)
}

func DefaultParakeetCPPConfig(projectRoot string) ParakeetCPPConfig {
	binary := firstExisting(
		os.Getenv("VOX_PARAKEET_BIN"),
		filepath.Join(projectRoot, ".local", "runtime", "whisper.cpp-v1.9.1-cpu", "bin", "parakeet-cli"),
	)
	model := os.Getenv("VOX_PARAKEET_MODEL")
	if model == "" {
		model = filepath.Join(projectRoot, ".local", "models", "parakeet-tdt-0.6b-v3", "ggml-parakeet-tdt-0.6b-v3-q8_0.bin")
	}
	return ParakeetCPPConfig{Binary: binary, Model: model, Threads: envThreads()}
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := w.limit - w.buffer.Len()
	if remaining < len(data) {
		w.exceeded = true
	}
	if remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		_, _ = w.buffer.Write(data[:remaining])
	}
	return written, nil
}

func (w *limitedBuffer) String() string {
	return w.buffer.String()
}

func parseMilliseconds(output string, pattern *regexp.Regexp) time.Duration {
	match := pattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0
	}
	milliseconds, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	return time.Duration(milliseconds * float64(time.Millisecond))
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	for _, path := range paths {
		if path != "" {
			return path
		}
	}
	return ""
}

func tail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}

func envThreads() int {
	if value := os.Getenv("VOX_THREADS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 8
}
