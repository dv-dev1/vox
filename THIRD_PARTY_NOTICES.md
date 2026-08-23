# Third-party notices

The Vox source repository does not vendor or redistribute third-party source,
binaries, model weights, fonts, icons, or media. The preparation script
downloads the following components into the gitignored `.local/` directory for
local use.

## Downloaded components

| Component | Use | License and attribution |
| --- | --- | --- |
| [whisper.cpp 1.9.1](https://github.com/ggml-org/whisper.cpp/tree/v1.9.1) | Local Whisper inference | MIT; copyright 2023-2026 The ggml authors |
| [Whisper large-v3-turbo](https://huggingface.co/openai/whisper-large-v3-turbo) | Speech-recognition model | MIT; published by OpenAI |
| [whisper.cpp converted models](https://huggingface.co/ggerganov/whisper.cpp) | Q8_0 GGML conversion | MIT; published by the whisper.cpp maintainers |
| [Silero VAD 6.2.0](https://github.com/snakers4/silero-vad) | Voice-activity detection | MIT; copyright 2020-present Silero Team |
| [CMake 4.1.0](https://cmake.org/download/) | Local build tool | BSD 3-Clause; copyright 2000-2025 Kitware, Inc. and contributors |

Vox is written in Go. A distributed Vox executable contains portions of the Go
runtime and standard library under Go's BSD-style license and must retain its
license notice. See the [Go license](https://go.dev/LICENSE).

System components such as PipeWire, GTK, PyGObject, Cinnamon, X11, tmux, CUDA,
and NVIDIA drivers are invoked from the user's installation and are not bundled
with this repository. Their own terms apply independently.

## Distribution rule

The current source-only release does not include the downloaded components. If
a release later bundles a binary, model, build tool, or third-party source, it
must also bundle the exact license text and required notices from that pinned
version. This summary is informational; upstream license files control.

Third-party names and marks identify their respective projects and do not imply
endorsement.
