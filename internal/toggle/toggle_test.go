package toggle

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestWriteTranscriptPreservesExactText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	want := "Unicode: café 🚀\nsecond line without a trailing newline"
	if err := writeTranscript(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode is %o, want 600", got)
	}
}

func TestWriteTranscriptRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeTranscript(path, "replacement"); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "keep me" {
		t.Fatalf("existing transcript changed to %q", got)
	}
}

func TestRunToFileRejectsRelativeOutput(t *testing.T) {
	if _, err := (Toggle{}).RunToFile(context.Background(), "transcript.txt"); err == nil {
		t.Fatal("expected relative path rejection")
	}
}

func TestRunToFileRejectsStaleOutputBeforeRecording(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.txt")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Toggle{StateDir: dir}).RunToFile(context.Background(), path); err == nil {
		t.Fatal("expected stale output rejection")
	}
}

func TestCancelStopsAndDiscardsActiveRecording(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "recording-test.wav")
	if err := os.WriteFile(audioPath, []byte("partial audio"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	waitComplete := false
	t.Cleanup(func() {
		if waitComplete {
			return
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-waited:
		case <-time.After(time.Second):
		}
	})

	statePath := filepath.Join(dir, "recording.json")
	stateFile, err := os.OpenFile(statePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	startTicks, err := processStartTicks(cmd.Process.Pid)
	if err != nil {
		stateFile.Close()
		t.Fatal(err)
	}
	state := State{PID: cmd.Process.Pid, ProcessStartTicks: startTicks, OutputPath: filepath.Join(dir, "transcript.txt"), AudioPath: audioPath, StartedAt: time.Now().UTC()}
	if err := json.NewEncoder(stateFile).Encode(state); err != nil {
		stateFile.Close()
		t.Fatal(err)
	}
	if err := stateFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := (Toggle{StateDir: dir}).Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("recording state still exists: %v", err)
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Fatalf("partial audio still exists: %v", err)
	}
	select {
	case <-waited:
		waitComplete = true
	case <-time.After(time.Second):
		t.Fatal("recorder process did not exit")
	}
}

func TestCancelRejectsMissingSession(t *testing.T) {
	err := (Toggle{StateDir: t.TempDir()}).Cancel(context.Background())
	if err == nil || err.Error() != "no recording session is active" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelRejectsAudioOutsideStateDirectory(t *testing.T) {
	dir := t.TempDir()
	state := State{
		PID:               os.Getpid(),
		ProcessStartTicks: 1,
		AudioPath:         filepath.Join(t.TempDir(), "recording-outside.wav"),
		StartedAt:         time.Now().UTC(),
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recording.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Toggle{StateDir: dir}).Cancel(context.Background()); err == nil {
		t.Fatal("expected unsafe audio path rejection")
	}
}

func TestReadStateRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "recording.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readState(link, dir); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestDefaultStateDirRejectsUnsafeRuntimeEnvironment(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/")
	want := filepath.Join(os.TempDir(), "vox-"+strconv.Itoa(os.Getuid()))
	if got := DefaultStateDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	t.Setenv("XDG_RUNTIME_DIR", "relative")
	if got := DefaultStateDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
