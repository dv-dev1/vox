#!/usr/bin/env python3
"""Paste stdin into whatever currently has focus, without submitting.

Wayland (GNOME/mutter) has no equivalent of X11 window ids or
XSendEvent-based focus control, so unlike paste-x11.py this script does not
target a specific window: it sets the clipboard and asks ydotoold to press
Ctrl+V, exactly as if the user had pressed it themselves. Whatever has
keyboard focus at that moment receives the paste, so the caller must invoke
this immediately after toggling recording off.
"""
from __future__ import annotations

import subprocess
import sys
import time

KEY_LEFTCTRL = 29
KEY_V = 47


def run(*args: str, input_bytes: bytes | None = None, check: bool = True) -> subprocess.CompletedProcess:
    process = subprocess.run(
        args,
        input=input_bytes,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if check and process.returncode != 0:
        detail = process.stderr.decode("utf-8", "replace").strip() or f"exit status {process.returncode}"
        raise RuntimeError(f"{args[0]} failed: {detail}")
    return process


def run_daemonizing(*args: str, input_bytes: bytes | None = None, check: bool = True) -> int:
    # wl-copy double-forks and keeps running in the background to serve the
    # clipboard. It inherits stdout/stderr from us, so subprocess.run(...,
    # stdout=PIPE) would block forever waiting for those pipes to close —
    # they never do while the daemonized copy is alive. Discard them instead.
    process = subprocess.run(
        args,
        input=input_bytes,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if check and process.returncode != 0:
        raise RuntimeError(f"{args[0]} failed: exit status {process.returncode}")
    return process.returncode


def snapshot_clipboard() -> bytes | None:
    process = run("wl-paste", "--no-newline", check=False)
    if process.returncode != 0:
        # No selection owner yet (empty clipboard) — nothing to restore.
        return None
    return process.stdout


def set_clipboard(data: bytes) -> None:
    run_daemonizing("wl-copy", input_bytes=data)


def restore_clipboard(snapshot: bytes | None) -> None:
    if snapshot is None:
        run_daemonizing("wl-copy", "--clear", check=False)
    else:
        run_daemonizing("wl-copy", input_bytes=snapshot, check=False)


def paste(text: str) -> None:
    if not text:
        raise ValueError("refusing to paste empty text")
    if "\x00" in text:
        raise ValueError("refusing to paste text containing NUL")

    snapshot = snapshot_clipboard()
    try:
        # Note: unlike X11, a Wayland clipboard manager watching the
        # selection (wl-paste --watch) can observe the transcript here.
        # There is no portable way to suppress that on Wayland.
        set_clipboard(text.encode("utf-8"))
        run("ydotool", "key", f"{KEY_LEFTCTRL}:1", f"{KEY_V}:1", f"{KEY_V}:0", f"{KEY_LEFTCTRL}:0")
        # Give the focused client time to request the selection before we
        # hand the clipboard back to whatever the user had copied before.
        time.sleep(1.0)
    finally:
        restore_clipboard(snapshot)


def main() -> int:
    try:
        maximum_bytes = 1024 * 1024
        data = sys.stdin.buffer.read(maximum_bytes + 1)
        if len(data) > maximum_bytes:
            raise ValueError("transcript exceeds the 1 MiB paste limit")
        text = data.decode("utf-8")
        paste(text)
        return 0
    except (OSError, RuntimeError, ValueError) as error:
        print(f"vox: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
