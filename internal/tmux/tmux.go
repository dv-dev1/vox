package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

var panePattern = regexp.MustCompile(`^%[0-9]+$`)
var bufferCounter atomic.Uint64

type Runner interface {
	Run(ctx context.Context, stdin io.Reader, args ...string) error
}

type ExecRunner struct {
	Binary string
}

func (r ExecRunner) Run(ctx context.Context, stdin io.Reader, args ...string) error {
	binary := r.Binary
	if binary == "" {
		binary = "tmux"
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux %s failed: %w: %s", args[0], err, stderr.String())
	}
	return nil
}

type Inserter struct {
	Runner Runner
}

func ValidatePane(target string) error {
	if !panePattern.MatchString(target) {
		return fmt.Errorf("invalid tmux pane %q: expected format %%<number>", target)
	}
	return nil
}

func (i Inserter) Insert(ctx context.Context, target, text string) error {
	if err := ValidatePane(target); err != nil {
		return err
	}
	if text == "" {
		return errors.New("refusing to insert empty text")
	}
	if i.Runner == nil {
		i.Runner = ExecRunner{}
	}
	name := "vox-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(bufferCounter.Add(1), 10)
	if err := i.Runner.Run(ctx, bytes.NewBufferString(text), "load-buffer", "-b", name, "-"); err != nil {
		return err
	}
	// -p requests bracketed paste. -d deletes the named buffer after pasting.
	if err := i.Runner.Run(ctx, nil, "paste-buffer", "-p", "-d", "-b", name, "-t", target); err != nil {
		_ = i.Runner.Run(context.Background(), nil, "delete-buffer", "-b", name)
		return err
	}
	return nil
}
