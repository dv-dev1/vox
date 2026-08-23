package tmux

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type call struct {
	args  []string
	stdin string
}

type fakeRunner struct {
	calls []call
	fail  int
}

func (f *fakeRunner) Run(_ context.Context, stdin io.Reader, args ...string) error {
	var value string
	if stdin != nil {
		data, _ := io.ReadAll(stdin)
		value = string(data)
	}
	f.calls = append(f.calls, call{args: append([]string(nil), args...), stdin: value})
	if f.fail > 0 && len(f.calls) == f.fail {
		return fmt.Errorf("failed")
	}
	return nil
}

func TestInsertUsesStdinAndBracketedPasteWithoutNewline(t *testing.T) {
	runner := &fakeRunner{}
	text := "Unicode: café 🚀\nrm -rf / # inserted, never executed"
	if err := (Inserter{Runner: runner}).Insert(context.Background(), "%42", text); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("got %d calls", len(runner.calls))
	}
	if runner.calls[0].stdin != text {
		t.Fatalf("stdin changed: %q", runner.calls[0].stdin)
	}
	if strings.HasSuffix(runner.calls[0].stdin, "\n") {
		t.Fatal("inserter appended a newline")
	}
	if !reflect.DeepEqual(runner.calls[0].args[:3], []string{"load-buffer", "-b", runner.calls[0].args[2]}) {
		t.Fatalf("unexpected load args: %#v", runner.calls[0].args)
	}
	name := runner.calls[0].args[2]
	wantPaste := []string{"paste-buffer", "-p", "-d", "-b", name, "-t", "%42"}
	if !reflect.DeepEqual(runner.calls[1].args, wantPaste) {
		t.Fatalf("got %#v, want %#v", runner.calls[1].args, wantPaste)
	}
}

func TestInsertRejectsTargetThatCouldBecomeAnOption(t *testing.T) {
	for _, target := range []string{"", "1", "-t", "%1; run-shell bad", "%abc"} {
		if err := (Inserter{Runner: &fakeRunner{}}).Insert(context.Background(), target, "text"); err == nil {
			t.Fatalf("target %q accepted", target)
		}
	}
}

func TestInsertCleansBufferAfterPasteFailure(t *testing.T) {
	runner := &fakeRunner{fail: 2}
	if err := (Inserter{Runner: runner}).Insert(context.Background(), "%1", "text"); err == nil {
		t.Fatal("expected failure")
	}
	if len(runner.calls) != 3 || runner.calls[2].args[0] != "delete-buffer" {
		t.Fatalf("buffer was not cleaned: %#v", runner.calls)
	}
}
