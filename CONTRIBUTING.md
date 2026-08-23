# Contributing to Vox

Thanks for helping improve private, local dictation on Linux.

## Before opening a change

- Read the current limitations in `README.md`. The supported desktop target is
  intentionally narrow: Cinnamon, X11, PipeWire, and NVIDIA CUDA compute 8.6.
- Open an issue before making a large architectural change or adding another
  speech-recognition engine.
- Never commit recordings, real transcripts, model weights, benchmark
  manifests, credentials, or machine-identifying diagnostics.
- Report security issues privately as described in `SECURITY.md`.

## Development checks

```bash
make verify
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

Changes to recording, process signaling, clipboard handling, tmux insertion,
runtime files, or external downloads should include tests for their security
invariants. Keep every insertion path review-only: Vox must never send Enter or
otherwise submit transcribed text.

## Pull requests

Keep pull requests focused, explain user-visible behavior and security impact,
and update the relevant documentation. By contributing, you represent that the
work is original or that you have the right to submit it, that it contains no
confidential material, and that it is licensed under the repository's MIT
License.
