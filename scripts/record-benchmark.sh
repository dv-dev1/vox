#!/usr/bin/env bash
set -euo pipefail

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly example_manifest="$project_root/benchmarks/manifest.example.json"
readonly private_manifest="$project_root/benchmarks/manifest.json"
readonly vox_binary="$project_root/vox"

umask 077
cd "$project_root"

for dependency in jq go pw-record; do
  if ! command -v "$dependency" >/dev/null 2>&1; then
    printf 'Dependência ausente: %s\n' "$dependency" >&2
    exit 1
  fi
done

if [[ ! -f "$private_manifest" ]]; then
  cp "$example_manifest" "$private_manifest"
  chmod 600 "$private_manifest"
fi

go build -o "$vox_binary" ./cmd/vox
mkdir -p "$project_root/benchmarks/audio"

readonly total="$(jq '.samples | length' "$private_manifest")"
index=0

while IFS= read -r sample; do
  index=$((index + 1))
  id="$(jq -r '.id' <<<"$sample")"
  category="$(jq -r '.category' <<<"$sample")"
  expected="$(jq -r '.expected' <<<"$sample")"
  audio_relative="$(jq -r '.audio' <<<"$sample")"

  case "$audio_relative" in
    /*|*..*)
      printf 'Caminho de áudio inseguro no sample %s: %s\n' "$id" "$audio_relative" >&2
      exit 1
      ;;
  esac

  audio_path="$project_root/benchmarks/$audio_relative"
  mkdir -p "$(dirname "$audio_path")"

  printf '\n[%d/%d] %s — %s\n' "$index" "$total" "$id" "$category"
  if [[ -n "$expected" ]]; then
    printf '\nFale exatamente:\n\n  %s\n\n' "$expected"
  else
    case "$id" in
      silence-001) printf '\nFique em silêncio por aproximadamente 3 segundos.\n\n' ;;
      silence-002) printf '\nGrave apenas o ruído normal do ambiente por aproximadamente 3 segundos.\n\n' ;;
      silence-003) printf '\nFaça um ruído não vocal, como digitar no teclado, por aproximadamente 3 segundos.\n\n' ;;
      *) printf '\nNão fale durante esta amostra.\n\n' ;;
    esac
  fi

  if [[ -f "$audio_path" ]]; then
    printf 'Já gravado; mantendo o arquivo existente: %s\n' "$audio_relative"
    continue
  fi

  read -r -p 'Pressione Enter para COMEÇAR a gravar... ' < /dev/tty
  printf 'GRAVANDO — fale agora; Ctrl-C para PARAR e avançar.\n'
  if ! "$vox_binary" record --output "$audio_path"; then
    printf 'A gravação falhou; tente esta amostra novamente.\n' >&2
    exit 1
  fi
  if [[ ! -f "$audio_path" ]]; then
    printf 'A gravação não foi salva; tente esta amostra novamente.\n' >&2
    exit 1
  fi
  printf 'Salvo: %s\n' "$audio_relative"
done < <(jq -c '.samples[]' "$private_manifest")

printf '\nTodas as %s amostras estão prontas.\n' "$total"
read -r -p 'Executar o benchmark agora? [S/n] ' answer < /dev/tty
if [[ ! "$answer" =~ ^[Nn]$ ]]; then
  "$vox_binary" benchmark --manifest "$private_manifest" \
    > "$project_root/.local/benchmark-results.json"
  printf '\nRelatório salvo em artifacts/whisper-benchmark-report.md\n'
fi

printf '\nConcluído. Pressione Enter para fechar.\n'
read -r < /dev/tty
