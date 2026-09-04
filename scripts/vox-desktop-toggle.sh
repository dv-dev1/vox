#!/usr/bin/env bash
set -euo pipefail
umask 077

readonly project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly vox_binary="${project_root}/vox"
readonly paste_script="${project_root}/scripts/paste-wayland.py"
readonly sources_script="${project_root}/scripts/vox_audio_sources.py"
readonly hotwords_file="${HOME}/.config/vox/hotwords.txt"
readonly translate_script="${project_root}/scripts/translate-pt-en.py"
readonly translate_python="${project_root}/.local/venv-translate/bin/python"

# O `--translate` nunca devolve texto quebrado: força uma frase bem formada e
# paga em paráfrase o que o modo transcrição cobra em erro acústico cru.
# Português literal: VOX_WHISPER_TRANSLATE=0.
export VOX_WHISPER_TRANSLATE="${VOX_WHISPER_TRANSLATE:-1}"
export VOX_TRANSLATE_TO_EN="${VOX_TRANSLATE_TO_EN:-0}"
if [[ -n "${XDG_RUNTIME_DIR:-}" ]]; then
  [[ "$XDG_RUNTIME_DIR" == /* && "$XDG_RUNTIME_DIR" != "/" ]] || {
    printf 'vox: XDG_RUNTIME_DIR must be an absolute, non-root path\n' >&2
    exit 1
  }
  readonly runtime_root="${XDG_RUNTIME_DIR}/vox"
else
  readonly runtime_root="/tmp/vox-$(id -u)"
fi
readonly state_file="${runtime_root}/recording.json"
readonly status_file="${runtime_root}/desktop-status.json"
readonly source_file="${runtime_root}/desktop-source.json"
readonly transcript_file="${runtime_root}/desktop-transcript.txt"
readonly translated_file="${runtime_root}/desktop-translated.txt"
readonly command_log="${runtime_root}/desktop-command.log"

if [[ -L "$runtime_root" ]]; then
  printf 'vox: refusing symlink runtime directory: %s\n' "$runtime_root" >&2
  exit 1
fi
mkdir -p -m 700 "$runtime_root"
[[ -d "$runtime_root" && ! -L "$runtime_root" ]] || {
  printf 'vox: runtime path is not a real directory: %s\n' "$runtime_root" >&2
  exit 1
}
[[ "$(stat -c %u "$runtime_root")" == "$(id -u)" ]] || {
  printf 'vox: runtime directory is not owned by the current user\n' >&2
  exit 1
}
chmod 700 "$runtime_root"

exec 9>"${runtime_root}/desktop-toggle.lock"
flock -n 9 || exit 0

write_status() {
  local state="$1"
  local message="$2"
  local temporary microphone_name microphone_node
  microphone_name="$(jq -r '.description // empty' "$source_file" 2>/dev/null || true)"
  microphone_node="$(jq -r '.name // empty' "$source_file" 2>/dev/null || true)"
  temporary="$(mktemp "${runtime_root}/.desktop-status.XXXXXX")"
  chmod 600 "$temporary"
  jq -n \
    --arg state "$state" \
    --arg message "$message" \
    --arg microphone_name "$microphone_name" \
    --arg microphone_node "$microphone_node" \
    --arg updated_at "$(date --iso-8601=seconds)" \
    '{state:$state,message:$message,updated_at:$updated_at}
      + if $microphone_name == "" then {} else
          {microphone_name:$microphone_name,microphone_node:$microphone_node}
        end' > "$temporary"
  mv -f "$temporary" "$status_file"
}

write_source() {
  local source_json="$1"
  local temporary
  jq -e 'select(
    (.id | type == "number") and
    (.name | type == "string" and length > 0) and
    (.description | type == "string" and length > 0)
  ) | {id,name,description}' <<<"$source_json" >/dev/null
  temporary="$(mktemp "${runtime_root}/.desktop-source.XXXXXX")"
  chmod 600 "$temporary"
  jq '{id,name,description}' <<<"$source_json" >"$temporary"
  mv -f "$temporary" "$source_file"
}

capture_default_source() {
  local snapshot source_json
  snapshot="$(python3 "$sources_script" list)"
  source_json="$(jq -c '.sources[] | select(.default == true)' <<<"$snapshot" | head -n 1)"
  [[ -n "$source_json" ]] || return 1
  write_source "$source_json"
}

friendly_error() {
  local detail="$1"
  case "$detail" in
    "transcription was empty"*) printf '%s\n' "Nenhuma fala detectada" ;;
    "transcription failed"*) printf '%s\n' "Falha ao transcrever — áudio preservado" ;;
    "recording is not usable"*) printf '%s\n' "Não consegui usar a gravação" ;;
    "write transcript failed"*) printf '%s\n' "Não consegui preparar o texto" ;;
    *) printf '%s\n' "$detail" ;;
  esac
}

select_source() {
  local source_id="$1"
  local selected source_node
  [[ "$source_id" =~ ^[0-9]+$ ]] || return 1
  selected="$(python3 "$sources_script" select --id "$source_id")" || {
    write_status "error" "Microfone não está mais disponível"
    return 1
  }
  write_source "$selected"
  source_node="$(jq -r '.name' <<<"$selected")"

  if [[ ! -f "$state_file" ]]; then
    return 0
  fi

  write_status "starting" "Trocando microfone"
  if ! (exec 9>&-; cd "$project_root" && "$vox_binary" cancel) >>"$command_log" 2>&1; then
    write_status "error" "Não consegui trocar o microfone"
    return 1
  fi
  if ! (exec 9>&-; cd "$project_root" && "$vox_binary" toggle \
    --source "$source_node" --output "$transcript_file" "${hotwords_args[@]}") >>"$command_log" 2>&1; then
    write_status "error" "Não consegui iniciar o novo microfone"
    return 1
  fi
  write_status "recording" "Ctrl + Alt + Espaço para concluir"
}

if [[ ! -x "$vox_binary" ]]; then
  write_status "error" "Compile o Vox primeiro: make build"
  exit 1
fi

# Technical-vocabulary context prompt for whisper (helps HTML/CSS/etc not get
# mangled by the small model). Optional: skipped if the user never created it.
hotwords_args=()
[[ -f "$hotwords_file" ]] && hotwords_args=(--hotwords "$hotwords_file")

if [[ "${1:-}" == "--select-source" ]]; then
  [[ $# -eq 2 ]] || exit 2
  select_source "$2"
  exit $?
fi

action="start"
if [[ -f "$state_file" ]]; then
  action="stop"
fi

if [[ "$action" == "start" ]]; then
  if [[ -f "$transcript_file" ]]; then
    mv "$transcript_file" "${runtime_root}/desktop-transcript-$(date +%Y%m%d-%H%M%S).txt"
  fi
  # GNOME/mutter (Wayland) exposes no portable way for an external process to
  # query which window has focus, unlike X11's xdotool getactivewindow. The
  # paste below is delivered to whatever has focus the moment it runs, so
  # there is no pre-flight target check here.
  if ! capture_default_source; then
    write_status "error" "Nenhum microfone disponível"
    exit 1
  fi
  write_status "starting" "Conectando ao microfone"
else
  write_status "processing" "Transcrevendo localmente"
fi

source_node="$(jq -r '.name // empty' "$source_file" 2>/dev/null || true)"

if (exec 9>&-; cd "$project_root" && "$vox_binary" toggle \
  --source "$source_node" --output "$transcript_file" "${hotwords_args[@]}") 2>"$command_log"; then
  if [[ "$action" == "start" ]]; then
    write_status "recording" "Ctrl + Alt + Espaço para concluir"
  else
    # Segundo passo, desligado por padrão. Se o venv não existir ou o script
    # falhar, cola o português mesmo assim — entregar no idioma errado é melhor
    # que perder o ditado.
    paste_source="$transcript_file"
    if [[ "$VOX_TRANSLATE_TO_EN" == "1" && "$VOX_WHISPER_TRANSLATE" == "0" && -x "$translate_python" ]] &&
      "$translate_python" "$translate_script" <"$transcript_file" >"$translated_file" 2>>"$command_log"; then
      paste_source="$translated_file"
    fi
    if ! python3 "$paste_script" < "$paste_source" 2>>"$command_log"; then
      write_status "error" "Texto preservado: não foi possível colar"
      exit 1
    fi
    rm -f -- "$transcript_file" "$translated_file"
    write_status "done" "Texto inserido sem enviar"
  fi
  exit 0
fi

detail="$(tail -n 1 "$command_log" | sed -E 's/^vox:[[:space:]]*//' | cut -c1-180)"
if [[ -z "$detail" ]]; then
  detail="O Vox não conseguiu concluir a ação"
fi
message="$(friendly_error "$detail")"
write_status "error" "$message"
exit 1
