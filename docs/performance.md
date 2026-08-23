# Performance evidence and interpretation

This document answers a practical question: does Vox keep consuming notebook
resources after dictation, or noticeably reduce day-to-day performance?

## Short answer

On the target notebook, Vox has no persistent performance cost. It does not run
a daemon and it does not leave the Whisper model loaded. Recording and the Flow
Bar exist only between the two shortcut presses; local inference creates a
short CPU/GPU burst after the second press, then every Vox-related process
exits.

That transient work is real. If another application is saturating the GPU or
CPU at the same moment, the two workloads can briefly contend. The current
design optimizes for no idle cost rather than for zero inference-time cost.

## Target system

- AMD Ryzen 7 5800H;
- 15.5 GiB system RAM;
- NVIDIA GeForce RTX 3050 Laptop GPU with 4 GiB VRAM;
- CachyOS Linux, Cinnamon/X11, and PipeWire;
- whisper.cpp 1.9.1 compiled for CUDA compute capability 8.6;
- Whisper large-v3-turbo Q8_0 with Silero VAD 6.2.0.

The relevant runtime and hardware versions are preserved in the sanitized
[benchmark report](../artifacts/whisper-benchmark-report.md).

## Evidence

### Idle state

A process check after a completed dictation found no running:

- `vox`;
- `whisper-cli`;
- `pw-record`;
- `vox-overlay.py`.

The shortcut is a Cinnamon keybinding, not a Vox background service. Therefore
the application's incremental idle CPU, process RAM, and process VRAM are zero.
Model files remain on disk and filesystem pages may stay in the operating
system's reclaimable cache; that cache is automatically available to other
applications under memory pressure.

### Storage

The active Q8_0 model is approximately 834 MiB. The complete development
`.local/` directory measured 1.3 GiB. It includes the active model plus pinned
Whisper source, build tools, runtime artifacts, and downloads. These are disk
costs, not resident RAM consumption.

### Personal benchmark

The accepted benchmark ran 47 Whisper transcriptions sequentially:

- 47 completed successfully;
- no out-of-memory event occurred;
- median inference for a synthetic 20-second sample was 493 ms;
- maximum measured real-time factor across the personal set was 0.087;
- an isolated 20-second run peaked at 515,808 KiB process RSS;
- that isolated run peaked at 1,230 MiB process VRAM.

The 4 GiB target GPU therefore had substantial headroom for this isolated Vox
workload. Available headroom is lower when other GPU-heavy applications are
open.

The sanitized per-sample results are in
[`artifacts/whisper-benchmark-report.md`](../artifacts/whisper-benchmark-report.md).
Personal audio and recognized text remain gitignored.

### Current cold recheck

On 2026-08-23, the current binary transcribed the existing 12.86-second
`natural-001.wav` benchmark sample with:

| Metric | Result |
| --- | ---: |
| End-to-end wall time | 1.44 s |
| Reported cold wall | 1.436 s |
| Model load | 492 ms |
| Inference | 562 ms |
| Real-time factor | 0.044 |

Immediately afterward, no Vox, Whisper, recorder, or overlay process remained.
NVIDIA memory usage was 526 MiB, effectively the same as the 530 MiB desktop
baseline measured before the run. This confirms that model VRAM was released.

The baseline GPU utilization was not zero because Cinnamon and other desktop
applications also use the GPU. For this reason, the benchmark attributes peak
memory to the Whisper process instead of treating whole-GPU utilization as Vox
usage.

## Why Q8_0 helps

Q8_0 stores model weights in an 8-bit quantized representation. Compared with
a full-precision representation, this reduces the model's storage and working
memory requirements at the cost of some numerical precision. The project chose
the quantized model only after measuring transcription quality, technical-term
recall, silence handling, latency, and repeated-run stability on the target
machine.

Silero VAD also avoids expensive speech decoding for silence: the three
silence/noise benchmark samples completed in 17–19 ms of inference with no
hallucinated text.

## Practical impact

- Between dictations: no Vox process is present, so there is no ongoing Vox CPU,
  RAM, VRAM, or battery use.
- While recording: PipeWire and the small GTK Flow Bar are active; Whisper is
  not loaded yet.
- After stopping: Whisper briefly uses CPU, approximately 0.5 GiB peak process
  RAM, and up to the measured 1.23 GiB process VRAM.
- After paste or error: the inference process exits and its RAM/VRAM is
  reclaimed.

The likely worst user-visible effect is a brief slowdown if dictation finishes
while a game, renderer, local model, or other GPU-heavy application is already
near the hardware limit. It does not permanently lower notebook performance.

Normal GPU workloads do consume energy and create heat. Hardware safety remains
the responsibility of the NVIDIA driver, firmware, and laptop thermal/power
limits; these tests demonstrate workload size and resource release, not a
formal hardware-safety guarantee.

## Reproducing the checks

Run the project health and correctness suite:

```bash
make doctor
make verify
```

Run the private benchmark only when its gitignored manifest/audio are present:

```bash
./vox benchmark \
  --manifest benchmarks/manifest.json \
  --report artifacts/whisper-benchmark-report.md
```

Check that nothing remains resident after a dictation:

```bash
pgrep -a -x vox
pgrep -a -x whisper-cli
pgrep -a -x pw-record
pgrep -af '^python.*vox-overlay\.py'
```

No output is expected from those process checks while Vox is idle.
