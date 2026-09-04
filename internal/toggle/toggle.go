package toggle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yuribodo/vox/internal/asr"
	voxaudio "github.com/yuribodo/vox/internal/audio"
	"github.com/yuribodo/vox/internal/record"
	voxtmux "github.com/yuribodo/vox/internal/tmux"
)

type State struct {
	PID               int       `json:"pid"`
	ProcessStartTicks uint64    `json:"process_start_ticks"`
	Target            string    `json:"target,omitempty"`
	OutputPath        string    `json:"output_path,omitempty"`
	AudioPath         string    `json:"audio_path"`
	StartedAt         time.Time `json:"started_at"`
}

type Toggle struct {
	StateDir     string
	Recorder     record.Recorder
	Transcriber  asr.Transcriber
	Inserter     voxtmux.Inserter
	HotwordsPath string
}

func DefaultStateDir() string {
	if runtime := os.Getenv("XDG_RUNTIME_DIR"); runtime != "" && filepath.IsAbs(runtime) && filepath.Clean(runtime) != string(filepath.Separator) {
		return filepath.Join(runtime, "vox")
	}
	return filepath.Join(os.TempDir(), "vox-"+strconv.Itoa(os.Getuid()))
}

func (t Toggle) Run(ctx context.Context, target string) (string, error) {
	if err := voxtmux.ValidatePane(target); err != nil {
		return "", err
	}
	return t.run(ctx, State{Target: target})
}

func (t Toggle) RunToFile(ctx context.Context, outputPath string) (string, error) {
	if !filepath.IsAbs(outputPath) {
		return "", errors.New("transcript output path must be absolute")
	}
	return t.run(ctx, State{OutputPath: filepath.Clean(outputPath)})
}

// Cancel stops and discards the active recording without transcribing or
// changing its destination. It is used when the desktop switches microphone.
func (t Toggle) Cancel(ctx context.Context) error {
	if t.StateDir == "" {
		t.StateDir = DefaultStateDir()
	}
	statePath := filepath.Join(t.StateDir, "recording.json")
	state, err := readState(statePath, t.StateDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("no recording session is active")
		}
		return err
	}
	if err := stopRecorder(ctx, state); err != nil {
		return err
	}
	if err := os.Remove(state.AudioPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("discard recording audio: %w", err)
	}
	if err := os.Remove(statePath); err != nil {
		return fmt.Errorf("remove recording state: %w", err)
	}
	return nil
}

func (t Toggle) run(ctx context.Context, destination State) (string, error) {
	if t.StateDir == "" {
		t.StateDir = DefaultStateDir()
	}
	statePath := filepath.Join(t.StateDir, "recording.json")
	state, err := readState(statePath, t.StateDir)
	if errors.Is(err, os.ErrNotExist) {
		if destination.OutputPath != "" {
			if _, statErr := os.Lstat(destination.OutputPath); statErr == nil {
				return "", fmt.Errorf("transcript output already exists: %s", destination.OutputPath)
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return "", statErr
			}
		}
		return t.start(statePath, destination)
	}
	if err != nil {
		return "", err
	}
	if state.Target != destination.Target || state.OutputPath != destination.OutputPath {
		return "", errors.New("recording destination changed; stop it with the original destination")
	}
	return t.stop(ctx, statePath, state)
}

func (t Toggle) start(statePath string, destination State) (string, error) {
	if err := ensurePrivateDir(filepath.Dir(statePath)); err != nil {
		return "", err
	}
	stateFile, err := os.OpenFile(statePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", errors.New("another recording session is active")
		}
		return "", err
	}
	cleanupState := true
	defer func() {
		stateFile.Close()
		if cleanupState {
			_ = os.Remove(statePath)
		}
	}()

	audioFile, err := os.CreateTemp(t.StateDir, "recording-*.wav")
	if err != nil {
		return "", err
	}
	audioPath := audioFile.Name()
	if err := audioFile.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(audioPath); err != nil {
		return "", err
	}

	binary := t.Recorder.Binary
	if binary == "" {
		binary = "pw-record"
	}
	cmd := exec.Command(binary, t.Recorder.Args(audioPath)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return "", err
	}
	cmd.Stdout, cmd.Stderr = devnull, devnull
	if err := cmd.Start(); err != nil {
		devnull.Close()
		return "", fmt.Errorf("start pw-record: %w", err)
	}
	devnull.Close()
	startTicks, err := processStartTicks(cmd.Process.Pid)
	if err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		return "", fmt.Errorf("identify recorder process: %w", err)
	}
	state := destination
	state.PID = cmd.Process.Pid
	state.ProcessStartTicks = startTicks
	state.AudioPath = audioPath
	state.StartedAt = time.Now().UTC()
	if err := json.NewEncoder(stateFile).Encode(state); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		return "", err
	}
	if err := stateFile.Sync(); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		return "", err
	}
	cleanupState = false
	return "recording", nil
}

func (t Toggle) stop(ctx context.Context, statePath string, state State) (string, error) {
	if err := stopRecorder(ctx, state); err != nil {
		return "", err
	}
	if _, err := voxaudio.InspectWAV(state.AudioPath); err != nil {
		_ = os.Remove(statePath)
		return "", fmt.Errorf("recording is not usable (kept at %s): %w", state.AudioPath, err)
	}
	if t.Transcriber == nil {
		return "", errors.New("transcriber is not configured")
	}
	result, err := t.Transcriber.Transcribe(ctx, asr.Request{AudioPath: state.AudioPath, HotwordsPath: t.HotwordsPath})
	if err != nil {
		_ = os.Remove(statePath)
		return "", fmt.Errorf("transcription failed; audio kept at %s: %w", state.AudioPath, err)
	}
	if result.Text == "" {
		_ = os.Remove(statePath)
		return "", fmt.Errorf("transcription was empty; destination was not changed; audio kept at %s", state.AudioPath)
	}
	if state.OutputPath != "" {
		if err := writeTranscript(state.OutputPath, result.Text); err != nil {
			_ = os.Remove(statePath)
			return "", fmt.Errorf("write transcript failed; audio kept at %s: %w", state.AudioPath, err)
		}
	} else {
		if state.Target == "" {
			return "", errors.New("recording state has no destination")
		}
		if err := t.Inserter.Insert(ctx, state.Target, result.Text); err != nil {
			_ = os.Remove(statePath)
			return "", fmt.Errorf("insertion failed; audio kept at %s: %w", state.AudioPath, err)
		}
	}
	_ = os.Remove(state.AudioPath)
	_ = os.Remove(statePath)
	if state.OutputPath != "" {
		return "written", nil
	}
	return "inserted", nil
}

func stopRecorder(ctx context.Context, state State) error {
	if err := validateRecorderProcess(state); err != nil {
		return err
	}
	if err := syscall.Kill(-state.PID, syscall.SIGINT); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("stop recording: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for processExists(state.PID) && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if processExists(state.PID) {
		return errors.New("pw-record did not stop within 5 seconds")
	}
	return nil
}

func writeTranscript(path, text string) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if cleanup {
			_ = os.Remove(path)
		}
	}()
	if _, err = file.WriteString(text); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func readState(path, stateDir string) (State, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return State{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return State{}, errors.New("recording state must be a private regular file")
	}
	if info.Size() > 16*1024 {
		return State{}, errors.New("recording state exceeds 16 KiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	var state State
	decoder := json.NewDecoder(io.LimitReader(f, 16*1024+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("read recording state: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return State{}, errors.New("recording state contains trailing data")
	}
	if err := validateState(state, stateDir); err != nil {
		return State{}, err
	}
	return state, nil
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create private runtime directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect private runtime directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("runtime path must be a real directory, not a symlink")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return errors.New("runtime directory is not owned by the current user")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("protect private runtime directory: %w", err)
	}
	return nil
}

func validateState(state State, stateDir string) error {
	if state.PID <= 1 || state.ProcessStartTicks == 0 || state.AudioPath == "" {
		return errors.New("invalid recording state; remove it manually after inspection")
	}
	audioPath := filepath.Clean(state.AudioPath)
	cleanDir := filepath.Clean(stateDir)
	base := filepath.Base(audioPath)
	if !filepath.IsAbs(audioPath) || filepath.Dir(audioPath) != cleanDir ||
		!strings.HasPrefix(base, "recording-") || !strings.HasSuffix(base, ".wav") {
		return errors.New("recording state contains an unsafe audio path")
	}
	return nil
}

func validateRecorderProcess(state State) error {
	startTicks, err := processStartTicks(state.PID)
	if err != nil {
		return fmt.Errorf("recorder process is unavailable; audio was preserved: %w", err)
	}
	if startTicks != state.ProcessStartTicks {
		return errors.New("recorder PID was reused; refusing to signal an unrelated process")
	}
	processGroup, err := syscall.Getpgid(state.PID)
	if err != nil {
		return fmt.Errorf("inspect recorder process group: %w", err)
	}
	if processGroup != state.PID {
		return errors.New("recorder is not the expected process-group leader")
	}
	return nil
}

func processStartTicks(pid int) (uint64, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, err
	}
	closingParen := strings.LastIndexByte(string(data), ')')
	if closingParen < 0 {
		return 0, errors.New("malformed process stat")
	}
	fields := strings.Fields(string(data[closingParen+1:]))
	// The suffix starts at proc(5) field 3; starttime is field 22.
	if len(fields) <= 19 {
		return 0, errors.New("process stat is missing start time")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
