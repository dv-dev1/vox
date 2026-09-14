#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly cmake_version="4.1.0"
readonly cmake_archive="cmake-${cmake_version}-linux-x86_64.tar.gz"
readonly cmake_sha256="2637dab096e65c7d011ca0504fc0c563f8ffb531919754156ddec4b7a2f8584d"
readonly cmake_url="https://github.com/Kitware/CMake/releases/download/v${cmake_version}/${cmake_archive}"

readonly whisper_version="1.9.1"
readonly whisper_commit="f049fff95a089aa9969deb009cdd4892b3e74916"
readonly whisper_repo="https://github.com/ggml-org/whisper.cpp.git"

# Parakeet is built from the whisper.cpp tree: parakeet-cli links libwhisper.
readonly parakeet_revision="35156454d1a39de06863303dd209fd2bed6ee079"
readonly parakeet_file="ggml-parakeet-tdt-0.6b-v3-q8_0.bin"
readonly parakeet_sha256="4d64e9e96c2792186d072fde0034df0ad670cf680a2f53069052ead827fd600e"
readonly parakeet_url="https://huggingface.co/ggml-org/parakeet-GGUF/resolve/${parakeet_revision}/${parakeet_file}"

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly download_dir="${project_root}/.local/downloads"
readonly tool_dir="${project_root}/.local/tools"
readonly source_dir="${project_root}/.local/src/whisper.cpp-v${whisper_version}"
readonly runtime_dir="${project_root}/.local/runtime/whisper.cpp-v${whisper_version}-cpu"
readonly parakeet_dir="${project_root}/.local/models/parakeet-tdt-0.6b-v3"
readonly cmake_dir="${tool_dir}/cmake-${cmake_version}-linux-x86_64"

mkdir -p "$download_dir" "$tool_dir" "$(dirname "$source_dir")" "$runtime_dir" "$parakeet_dir"

fetch_and_verify() {
  local url="$1"
  local output="$2"
  local expected="$3"

  if [[ ! -f "$output" ]]; then
    curl --fail --location --proto '=https' --proto-redir '=https' \
      --tlsv1.2 --continue-at - --output "$output" "$url"
  fi
  printf '%s  %s\n' "$expected" "$output" | sha256sum --check --status || {
    printf 'checksum mismatch: %s\n' "$output" >&2
    return 1
  }
}

fetch_and_verify "$cmake_url" "$download_dir/$cmake_archive" "$cmake_sha256"
if [[ ! -x "$cmake_dir/bin/cmake" ]]; then
  tar -xzf "$download_dir/$cmake_archive" -C "$tool_dir"
fi

if [[ ! -d "$source_dir/.git" ]]; then
  git clone --branch "v${whisper_version}" --depth 1 "$whisper_repo" "$source_dir"
fi
actual_commit="$(git -C "$source_dir" rev-parse HEAD)"
if [[ "$actual_commit" != "$whisper_commit" ]]; then
  printf 'unexpected whisper.cpp commit: got %s, want %s\n' "$actual_commit" "$whisper_commit" >&2
  exit 1
fi
if [[ -n "$(git -C "$source_dir" status --porcelain --untracked-files=all)" ]]; then
  printf 'whisper.cpp source tree has local changes; refusing an unreproducible build\n' >&2
  exit 1
fi

fetch_and_verify "$parakeet_url" "$parakeet_dir/$parakeet_file" "$parakeet_sha256"

# No NVIDIA GPU on this machine: CPU-only build (AVX2/AVX512 auto-detected via
# GGML_NATIVE) instead of the upstream project's mandatory CUDA build.
"$cmake_dir/bin/cmake" \
  -S "$source_dir" \
  -B "$runtime_dir" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_C_COMPILER=/usr/bin/gcc \
  -DCMAKE_CXX_COMPILER=/usr/bin/g++ \
  -DGGML_NATIVE=ON \
  -DWHISPER_BUILD_TESTS=OFF
"$cmake_dir/bin/cmake" --build "$runtime_dir" --target parakeet-cli --parallel "$(nproc)"

printf 'Parakeet runtime: %s/bin/parakeet-cli\n' "$runtime_dir"
printf 'Parakeet model:   %s/%s\n' "$parakeet_dir" "$parakeet_file"
