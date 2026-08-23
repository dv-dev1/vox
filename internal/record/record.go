package record

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	voxaudio "github.com/yuribodo/vox/internal/audio"
)

type Recorder struct {
	Binary string
	Target string
}

func (r Recorder) Args(output string) []string {
	args := []string{"--rate=16000", "--channels=1", "--format=s16", "--channel-map=MONO"}
	if r.Target != "" {
		args = append(args, "--target="+r.Target)
	}
	return append(args, output)
}

// Record writes through a private sibling file and links it into place only
// after pw-record exits successfully. The final path can never be overwritten.
func (r Recorder) Record(ctx context.Context, output string) error {
	if r.Binary == "" {
		r.Binary = "pw-record"
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("refusing to overwrite existing file %q", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output: %w", err)
	}
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	// pw-record creates the output file itself and may honor a permissive umask.
	// A private parent directory keeps audio inaccessible until it is linked to
	// the requested 0600 destination.
	privateDir, err := os.MkdirTemp(dir, ".vox-recording-*")
	if err != nil {
		return fmt.Errorf("create private recording directory: %w", err)
	}
	if err := os.Chmod(privateDir, 0o700); err != nil {
		_ = os.RemoveAll(privateDir)
		return fmt.Errorf("protect private recording directory: %w", err)
	}
	defer os.RemoveAll(privateDir)
	tmpPath := filepath.Join(privateDir, "audio.wav")

	cmd := exec.CommandContext(ctx, r.Binary, r.Args(tmpPath)...)
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// pw-record commonly reports the interrupt used to finish an explicit
		// push-to-talk recording. Accept it only when a valid WAV was finalized.
		if ctx.Err() == nil {
			return fmt.Errorf("pw-record failed: %w", err)
		}
		if _, inspectErr := voxaudio.InspectWAV(tmpPath); inspectErr != nil {
			return fmt.Errorf("recording interrupted before a valid WAV was finalized: %w", inspectErr)
		}
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("protect recording permissions: %w", err)
	}
	if err := os.Link(tmpPath, output); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("refusing to overwrite existing file %q", output)
		}
		return fmt.Errorf("publish recording: %w", err)
	}
	return nil
}
