# Private speech-recognition benchmark

The example manifest defines a 30-sample private evaluation set:

- 10 natural coding-agent prompts;
- 8 prompts containing framework or tool names;
- 5 prompts containing paths or identifiers;
- 4 prompts with hesitation or self-correction;
- 3 silence/noise samples.

## Record the private dataset

The recommended path is the guided recorder. It displays each phrase; press
Enter to start each sample and Ctrl-C once after speaking to save it and move
to the next sample:

```bash
./scripts/record-benchmark.sh
```

Existing recordings are skipped, so the command can be resumed safely. It
offers to run the benchmark after all 30 samples are present.

### Manual alternative

Copy the example, then record each utterance exactly as written:

```bash
cp benchmarks/manifest.example.json benchmarks/manifest.json
mkdir -p benchmarks/audio
./vox record --output benchmarks/audio/natural-001.wav
```

Press Ctrl-C once after speaking; `vox record` finalizes the WAV. Repeat for all
speech samples. For the three silence/noise cases, use one quiet room recording,
one microphone-room-noise recording, and one ordinary non-speech noise recording.
Do not speak secrets. The real manifest and `benchmarks/audio/` are gitignored.

Optional `duration_seconds` may be added to any sample. `identifiers` are checked
case-sensitively and literally; `required_terms` are matched case-insensitively
after punctuation normalization. Hotwords are written to a mode-specific private
temporary file and deleted immediately after each run.

## Run

```bash
./vox benchmark --manifest benchmarks/manifest.json
```

After preparing the pinned CUDA runtime with `./scripts/fetch-whisper.sh`, run
the private recordings through Whisper large-v3-turbo:

```bash
./vox benchmark --manifest benchmarks/manifest.json \
  > .local/whisper-benchmark-results.json
```

This writes the sanitized Markdown report to
`artifacts/whisper-benchmark-report.md`. The manifest `hotwords` are passed as
Whisper's initial context prompt for the paired run. Machine-readable results
and all recognized text remain under gitignored `.local/`. Personal transcripts
are sensitive; inspect every generated report before sharing or committing it.
