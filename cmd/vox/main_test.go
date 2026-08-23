package main

import (
	"context"
	"testing"
)

func TestVersionCommandDoesNotRequireRuntimeDependencies(t *testing.T) {
	if err := run(context.Background(), []string{"version"}); err != nil {
		t.Fatal(err)
	}
}
