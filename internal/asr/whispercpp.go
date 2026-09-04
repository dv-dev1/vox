package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	voxaudio "github.com/yuribodo/vox/internal/audio"
)

type WhisperCPPConfig struct {
	Binary    string
	Model     string
	VADModel  string
	Language  string
	Threads   int
	UseGPU    bool
	Translate bool
	BeamSize  int
	BestOf    int
	AudioCtx  int
	UseVAD    bool
}

type WhisperCPP struct {
	config WhisperCPPConfig
}

var (
	whisperLoadTimePattern  = regexp.MustCompile(`whisper_print_timings:\s+load time =\s+([0-9.]+) ms`)
	whisperTotalTimePattern = regexp.MustCompile(`whisper_print_timings:\s+total time =\s+([0-9.]+) ms`)
)

func NewWhisperCPP(config WhisperCPPConfig) (*WhisperCPP, error) {
	if config.Binary == "" || config.Model == "" || config.VADModel == "" {
		return nil, errors.New("whisper.cpp binary, model, and VAD model are required")
	}
	if _, err := os.Stat(config.Binary); err != nil {
		return nil, fmt.Errorf("whisper.cpp binary: %w", err)
	}
	if _, err := os.Stat(config.Model); err != nil {
		return nil, fmt.Errorf("whisper.cpp model: %w", err)
	}
	if _, err := os.Stat(config.VADModel); err != nil {
		return nil, fmt.Errorf("whisper.cpp VAD model: %w", err)
	}
	if config.Language == "" {
		config.Language = "en"
	}
	if config.Threads < 1 {
		config.Threads = 4
	}
	if config.BeamSize < 1 {
		config.BeamSize = 1
	}
	if config.BestOf < 1 {
		config.BestOf = 1
	}
	if config.AudioCtx < 0 {
		config.AudioCtx = 0
	}
	return &WhisperCPP{config: config}, nil
}

func (w *WhisperCPP) Transcribe(ctx context.Context, request Request) (Result, error) {
	info, err := voxaudio.InspectWAV(request.AudioPath)
	if err != nil {
		return Result{}, err
	}
	prompt, err := whisperPrompt(request.HotwordsPath)
	if err != nil {
		return Result{}, err
	}

	// whisper.cpp creates its JSON output itself. Keep it inside a private
	// directory so the transcript cannot become world-readable through the
	// process umask on a multi-user machine.
	outputDir, err := os.MkdirTemp("", "vox-whisper-*")
	if err != nil {
		return Result{}, fmt.Errorf("create private whisper output directory: %w", err)
	}
	if err := os.Chmod(outputDir, 0o700); err != nil {
		_ = os.RemoveAll(outputDir)
		return Result{}, fmt.Errorf("protect whisper output directory: %w", err)
	}
	defer os.RemoveAll(outputDir)
	prefix := filepath.Join(outputDir, "transcript")
	jsonPath := prefix + ".json"

	args := []string{
		"-m", w.config.Model,
		"-f", request.AudioPath,
		"-l", w.config.Language,
		"-t", strconv.Itoa(w.config.Threads),
		// whisper-cli defaults to 5 beams + best-of-5, which redoes the
		// (cheap) decode pass 5x for a translation-quality gain that is
		// usually marginal. Greedy (1/1) leaves encode time — the real
		// cost on this CPU — untouched and cuts decode close to zero.
		"-bs", strconv.Itoa(w.config.BeamSize),
		"-bo", strconv.Itoa(w.config.BestOf),
		"-oj",
		"-of", prefix,
	}
	// Sem `-nt` de propósito. O token de timestamp é a âncora que faz o decoder
	// avançar a janela: com `--no-timestamps` uma fala de 116 s devolve 4 frases
	// de 20, sem ele devolve 20 de 20. O texto colado não muda — os timestamps
	// vivem em campos próprios do JSON, fora de `transcription[].text`.
	if w.config.UseVAD {
		// Desligado por padrão porque não paga: em ditado curto não economiza
		// nada (2536 ms contra 2569 ms) e em fala longa ficou 1 s mais lento com
		// o mesmo texto. Só vale para gravação com silêncio longo de verdade.
		args = append(args, "--vad", "--vad-model", w.config.VADModel)
	}
	if w.config.AudioCtx > 0 {
		// O encoder custa por janela, não por segundo falado: uma frase de 4 s
		// paga quase o mesmo que uma de 20 s. Encurtar o contexto de áudio de
		// 1500 para 768 corta ~30% do tempo sem mudar o texto (medido). Abaixo
		// disso degrada: em 512 o modelo perde palavras e entra em loop de
		// repetição, ficando mais lento que o padrão.
		// ponytail: 768 vale para o ditado curto de sempre; fala longa e
		// contínua pode querer mais contexto — subir via VOX_WHISPER_AUDIO_CTX.
		args = append(args, "-ac", strconv.Itoa(w.config.AudioCtx))
	}
	if !w.config.UseGPU {
		args = append(args, "-ng")
	}
	if w.config.Translate {
		args = append(args, "--translate")
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}

	started := time.Now()
	cmd := exec.CommandContext(ctx, w.config.Binary, args...)
	stderr := limitedBuffer{limit: 1024 * 1024}
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("whisper.cpp failed: %w: %s", err, tail(stderr.String(), 1200))
	}
	if stderr.exceeded {
		return Result{}, errors.New("whisper.cpp diagnostic output exceeds 1 MiB")
	}
	latency := time.Since(started)
	output, err := readLimitedFile(jsonPath, 8*1024*1024)
	if err != nil {
		return Result{}, fmt.Errorf("read whisper.cpp JSON: %w", err)
	}
	text, err := parseWhisperJSON(output)
	if err != nil {
		return Result{}, fmt.Errorf("parse whisper.cpp JSON: %w", err)
	}
	loadLatency, totalLatency := parseWhisperTimings(stderr.String())
	inferenceLatency := totalLatency - loadLatency
	if inferenceLatency <= 0 {
		inferenceLatency = latency - loadLatency
	}
	if inferenceLatency <= 0 {
		inferenceLatency = latency
	}

	return Result{
		Text:             text,
		AudioDuration:    info.Duration,
		Latency:          latency,
		ModelLoadLatency: loadLatency,
		InferenceLatency: inferenceLatency,
		RealTimeFactor:   inferenceLatency.Seconds() / info.Duration.Seconds(),
	}, nil
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

func readLimitedFile(path string, maximumBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximumBytes {
		return nil, fmt.Errorf("output exceeds %d bytes", maximumBytes)
	}
	return data, nil
}

func parseWhisperJSON(output []byte) (string, error) {
	var result struct {
		Transcription []struct {
			Text string `json:"text"`
		} `json:"transcription"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}
	if result.Transcription == nil {
		return "", errors.New("missing transcription array")
	}
	var text strings.Builder
	for _, segment := range result.Transcription {
		text.WriteString(segment.Text)
	}
	return strings.TrimSpace(text.String()), nil
}

func parseWhisperTimings(output string) (time.Duration, time.Duration) {
	return parseMilliseconds(output, whisperLoadTimePattern), parseMilliseconds(output, whisperTotalTimePattern)
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

func whisperPrompt(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("context prompt file: %w", err)
	}
	defer file.Close()
	const maximumPromptBytes = 16 * 1024
	data, err := io.ReadAll(io.LimitReader(file, maximumPromptBytes+1))
	if err != nil {
		return "", fmt.Errorf("read context prompt file: %w", err)
	}
	if len(data) > maximumPromptBytes {
		return "", errors.New("context prompt file exceeds 16 KiB")
	}
	var phrases []string
	for _, line := range strings.Split(string(data), "\n") {
		phrase := strings.Join(strings.Fields(line), " ")
		if phrase != "" {
			phrases = append(phrases, phrase)
		}
	}
	if len(phrases) == 0 {
		return "", nil
	}
	return "Vocabulary: " + strings.Join(phrases, ", ") + ".", nil
}

func DefaultWhisperCPPConfig(projectRoot string) WhisperCPPConfig {
	runtimeRoot := filepath.Join(projectRoot, ".local", "runtime", "whisper.cpp-v1.9.1-cpu")
	binary := firstExisting(
		os.Getenv("VOX_WHISPER_BIN"),
		filepath.Join(runtimeRoot, "bin", "whisper-cli"),
	)
	model := os.Getenv("VOX_WHISPER_MODEL")
	if model == "" {
		model = filepath.Join(projectRoot, ".local", "models", "whisper-small-q8_0", "ggml-small-q8_0.bin")
	}
	vadModel := os.Getenv("VOX_WHISPER_VAD_MODEL")
	if vadModel == "" {
		vadModel = filepath.Join(projectRoot, ".local", "models", "whisper-vad", "ggml-silero-v6.2.0.bin")
	}
	threads := 8
	if value := os.Getenv("VOX_THREADS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			threads = parsed
		}
	}
	// This machine has no NVIDIA GPU: whisper-cli is built CPU-only (see
	// scripts/fetch-whisper.sh), so GPU offload is never requested.
	language := os.Getenv("VOX_WHISPER_LANGUAGE")
	if language == "" {
		language = "pt"
	}
	translate := true
	if value := os.Getenv("VOX_WHISPER_TRANSLATE"); value != "" {
		translate = value != "0"
	}
	beamSize := 1
	if value := os.Getenv("VOX_WHISPER_BEAM_SIZE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			beamSize = parsed
		}
	}
	bestOf := 1
	if value := os.Getenv("VOX_WHISPER_BEST_OF"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			bestOf = parsed
		}
	}
	// Desligado por padrão. Encurtar o contexto de áudio corta ~30% do tempo em
	// frase curta, mas em fala longa e contínua o encoder perde trecho e o
	// modelo inventa texto para preencher. Precisão vale mais que os 30%.
	audioCtx := 0
	if value := os.Getenv("VOX_WHISPER_AUDIO_CTX"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			audioCtx = parsed
		}
	}
	useVAD := os.Getenv("VOX_WHISPER_VAD") == "1"
	return WhisperCPPConfig{Binary: binary, Model: model, VADModel: vadModel, Language: language, Threads: threads, UseGPU: false, Translate: translate, BeamSize: beamSize, BestOf: bestOf, AudioCtx: audioCtx, UseVAD: useVAD}
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
