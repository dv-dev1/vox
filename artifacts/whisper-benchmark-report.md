# Whisper large-v3-turbo benchmark

Generated: 2026-08-23 04:51:14Z

## Environment

- Target: CachyOS Linux, kernel 7.2.0-1-cachyos, AMD Ryzen 7 5800H, 15.5 GiB RAM, NVIDIA RTX 3050 Laptop GPU (4 GiB).
- NVIDIA driver 610.57.04, CUDA 13.3.1, cuDNN 9.25.0.15; PipeWire 1.6.7; tmux 3.7c.
- Runtime: whisper.cpp 1.9.1, built locally for CUDA compute capability 8.6.
- Model: OpenAI Whisper large-v3-turbo, official whisper.cpp Q8_0 conversion, with Silero VAD 6.2.0.
- Provider: CUDA. Manifest hotwords are supplied as a bounded initial vocabulary prompt.
- Acquisition: pinned official assets and SHA-256 digests in `docs/dependencies.md`.

## Personal benchmark results

Transcripts remain only in the gitignored machine-readable result. The versioned report records status and metrics without personal recognized text.

| Sample | Category | Mode | Status | WER | Term recall | Identifier accuracy | Cold wall | Inference | RTF |
| --- | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| natural-001 | natural-prompt | greedy | ok | 9.1% | 0/0 | 0/0 | 1054 ms | 421 ms | 0.033 |
| natural-002 | natural-prompt | greedy | ok | 7.7% | 0/0 | 0/0 | 978 ms | 413 ms | 0.046 |
| natural-003 | natural-prompt | greedy | ok | 8.3% | 0/0 | 0/0 | 999 ms | 420 ms | 0.035 |
| natural-004 | natural-prompt | greedy | ok | 0.0% | 0/0 | 0/0 | 986 ms | 407 ms | 0.066 |
| natural-005 | natural-prompt | greedy | ok | 0.0% | 0/0 | 0/0 | 979 ms | 396 ms | 0.077 |
| natural-006 | natural-prompt | greedy | ok | 0.0% | 0/0 | 0/0 | 965 ms | 391 ms | 0.072 |
| natural-007 | natural-prompt | greedy | ok | 10.0% | 0/0 | 0/0 | 968 ms | 399 ms | 0.065 |
| natural-008 | natural-prompt | greedy | ok | 12.5% | 0/0 | 0/0 | 956 ms | 395 ms | 0.068 |
| natural-009 | natural-prompt | greedy | ok | 0.0% | 0/0 | 0/0 | 967 ms | 397 ms | 0.071 |
| natural-010 | natural-prompt | greedy | ok | 0.0% | 0/0 | 0/0 | 976 ms | 413 ms | 0.059 |
| technical-001 | technical-vocabulary | greedy | ok | 0.0% | 2/2 | 0/0 | 965 ms | 404 ms | 0.071 |
| technical-001 | technical-vocabulary | hotwords | ok | 0.0% | 2/2 | 0/0 | 963 ms | 407 ms | 0.071 |
| technical-002 | technical-vocabulary | greedy | ok | 0.0% | 1/1 | 0/0 | 975 ms | 410 ms | 0.073 |
| technical-002 | technical-vocabulary | hotwords | ok | 0.0% | 1/1 | 0/0 | 990 ms | 411 ms | 0.074 |
| technical-003 | technical-vocabulary | greedy | ok | 0.0% | 2/2 | 0/0 | 971 ms | 401 ms | 0.072 |
| technical-003 | technical-vocabulary | hotwords | ok | 0.0% | 2/2 | 0/0 | 978 ms | 410 ms | 0.074 |
| technical-004 | technical-vocabulary | greedy | ok | 0.0% | 1/1 | 0/0 | 963 ms | 401 ms | 0.073 |
| technical-004 | technical-vocabulary | hotwords | ok | 0.0% | 1/1 | 0/0 | 978 ms | 408 ms | 0.075 |
| technical-005 | technical-vocabulary | greedy | ok | 0.0% | 2/2 | 0/0 | 989 ms | 404 ms | 0.072 |
| technical-005 | technical-vocabulary | hotwords | ok | 0.0% | 2/2 | 0/0 | 963 ms | 407 ms | 0.072 |
| technical-006 | technical-vocabulary | greedy | ok | 37.5% | 0/2 | 0/0 | 1017 ms | 407 ms | 0.083 |
| technical-006 | technical-vocabulary | hotwords | ok | 12.5% | 2/2 | 0/0 | 984 ms | 409 ms | 0.083 |
| technical-007 | technical-vocabulary | greedy | ok | 14.3% | 2/2 | 0/0 | 966 ms | 389 ms | 0.083 |
| technical-007 | technical-vocabulary | hotwords | ok | 14.3% | 2/2 | 0/0 | 978 ms | 402 ms | 0.086 |
| technical-008 | technical-vocabulary | greedy | ok | 22.2% | 1/2 | 0/0 | 949 ms | 393 ms | 0.075 |
| technical-008 | technical-vocabulary | hotwords | ok | 22.2% | 1/2 | 0/0 | 968 ms | 401 ms | 0.077 |
| identifier-001 | identifier | greedy | ok | 54.5% | 0/1 | 0/1 | 990 ms | 415 ms | 0.060 |
| identifier-001 | identifier | hotwords | ok | 27.3% | 1/1 | 1/1 | 990 ms | 422 ms | 0.061 |
| identifier-002 | identifier | greedy | ok | 12.5% | 2/2 | 2/2 | 988 ms | 416 ms | 0.075 |
| identifier-002 | identifier | hotwords | ok | 12.5% | 2/2 | 2/2 | 977 ms | 413 ms | 0.074 |
| identifier-003 | identifier | greedy | ok | 38.5% | 0/0 | 0/0 | 996 ms | 416 ms | 0.057 |
| identifier-003 | identifier | hotwords | ok | 30.8% | 0/0 | 0/0 | 1010 ms | 428 ms | 0.059 |
| identifier-004 | identifier | greedy | ok | 28.6% | 0/0 | 0/0 | 949 ms | 390 ms | 0.063 |
| identifier-004 | identifier | hotwords | ok | 28.6% | 0/0 | 0/0 | 971 ms | 398 ms | 0.065 |
| identifier-005 | identifier | greedy | ok | 175.0% | 0/2 | 0/2 | 1003 ms | 408 ms | 0.075 |
| identifier-005 | identifier | hotwords | ok | 50.0% | 2/2 | 2/2 | 978 ms | 418 ms | 0.076 |
| hesitation-001 | hesitation | greedy | ok | 8.3% | 1/1 | 0/0 | 999 ms | 412 ms | 0.071 |
| hesitation-001 | hesitation | hotwords | ok | 0.0% | 1/1 | 0/0 | 983 ms | 417 ms | 0.072 |
| hesitation-002 | hesitation | greedy | ok | 11.1% | 0/1 | 0/0 | 974 ms | 405 ms | 0.086 |
| hesitation-002 | hesitation | hotwords | ok | 0.0% | 1/1 | 0/0 | 977 ms | 408 ms | 0.087 |
| hesitation-003 | hesitation | greedy | ok | 10.0% | 2/2 | 0/0 | 969 ms | 406 ms | 0.071 |
| hesitation-003 | hesitation | hotwords | ok | 10.0% | 2/2 | 0/0 | 982 ms | 419 ms | 0.073 |
| hesitation-004 | hesitation | greedy | ok | 15.4% | 0/1 | 0/1 | 1007 ms | 421 ms | 0.067 |
| hesitation-004 | hesitation | hotwords | ok | 30.8% | 1/1 | 1/1 | 1006 ms | 419 ms | 0.067 |
| silence-001 | silence-noise | greedy | ok | 0.0% | 0/0 | 0/0 | 582 ms | 19 ms | 0.003 |
| silence-002 | silence-noise | greedy | ok | 0.0% | 0/0 | 0/0 | 562 ms | 17 ms | 0.003 |
| silence-003 | silence-noise | greedy | ok | 0.0% | 0/0 | 0/0 | 575 ms | 18 ms | 0.002 |

## Aggregates

| Mode | Successful | Failures | WER | Term recall | Identifier accuracy | Median cold wall | Median inference | Mean / max RTF | Hallucinations |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| greedy | 30 | 0 | 14.0% | 66.7% | 33.3% | 974 ms | 404 ms | 0.061 / 0.086 | 0 |
| hotwords | 17 | 0 | 13.1% | 95.8% | 100.0% | 978 ms | 410 ms | 0.073 / 0.087 | 0 |

## Context-prompt findings

- Required-term recall across all context-eligible samples: **23/24 (95.8%)**.
- Technical-vocabulary category recall: **13/14 (92.9%)**.
- Exact identifier accuracy with initial context prompts: **6/6 (100.0%)**.
- Across 17 paired runs, initial context prompts improved required-term matches in 5 and regressed them in 0; WER improved in 6 and regressed in 1.

## Provider, duration, and resource evidence

The Q8_0 model runs on the RTX 3050 with Silero VAD enabled. The 47-run personal benchmark completed sequentially without failure or OOM. Direct `/proc` and NVIDIA process sampling on an isolated 20 second run observed 515,808 KiB peak process RSS and 1,230 MiB peak process VRAM.

| Synthetic duration | Median inference (5 runs) | Median RTF |
| ---: | ---: | ---: |
| 5 s | 434 ms | 0.087 |
| 10 s | 521 ms | 0.052 |
| 20 s | 493 ms | 0.025 |

## Recording and insertion evidence

PipeWire recording produced mono 16 kHz PCM WAV and finalized cleanly on Ctrl-C. The tmux transport remained bound to the captured pane, preserved Unicode and multiline content, inserted dangerous-looking text without executing it, and never sent Enter. Live insertion passed in a disposable shell, coding assistant, and Node REPL. See `docs/manual-integration.md`.

## Setup constraints

Inference starts a fresh process for each file, so cold wall latency includes
model loading. Results are specific to the hardware and software versions above.
Re-run `vox doctor` after kernel, driver, CUDA, or model changes.
