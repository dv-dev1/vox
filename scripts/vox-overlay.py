#!/usr/bin/env python3
from __future__ import annotations

import argparse
import array
import cairo
import datetime as dt
import fcntl
import json
import math
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

import gi

gi.require_version("Gtk", "3.0")
gi.require_version("Gdk", "3.0")
gi.require_version("Pango", "1.0")
gi.require_version("PangoCairo", "1.0")
from gi.repository import Gdk, GLib, Gtk, Pango, PangoCairo  # noqa: E402


ACCENTS = {
    "starting": (0.62, 0.56, 1.0),
    "recording": (1.0, 0.30, 0.39),
    "processing": (0.62, 0.56, 1.0),
    "done": (0.29, 0.84, 0.60),
    "error": (1.0, 0.58, 0.31),
}

BAR_WIDTHS = {
    "starting": 184,
    "recording": 376,
    "processing": 178,
    "done": 154,
    "error": 218,
}

STATUS_TEXT = {
    "starting": "Ativando microfone…",
    "processing": "Transcrevendo…",
    "done": "Texto inserido",
}


class FlowBar(Gtk.DrawingArea):
    def __init__(self) -> None:
        super().__init__()
        self.mode = "starting"
        self.message = ""
        self.timer = "00:00"
        self.levels = [0.08] * 15
        self.phase = 0.0
        self.microphone_name = "Microfone padrão"
        self.microphone_hovered = False
        self.set_size_request(BAR_WIDTHS[self.mode], 44)
        self.connect("draw", self._draw)

    def set_mode(self, mode: str, message: str) -> None:
        self.mode = mode
        self.message = message
        self.set_size_request(BAR_WIDTHS[mode], 44)
        self.queue_resize()
        self.queue_draw()

    def update(self, level: float, timer: str) -> None:
        self.timer = timer
        self.phase += 0.22
        if self.mode == "recording":
            self.levels.pop(0)
            self.levels.append(max(0.06, min(1.0, level)))
        self.queue_draw()

    def set_microphone(self, name: str) -> None:
        self.microphone_name = self._short_microphone_name(name)
        self.queue_draw()

    def set_microphone_hovered(self, hovered: bool) -> None:
        if self.microphone_hovered != hovered:
            self.microphone_hovered = hovered
            self.queue_draw()

    def microphone_rect(self) -> tuple[int, int, int, int]:
        width = self.get_allocated_width()
        return (190, 5, max(1, width - 197), 34)

    def microphone_hit(self, x: float, y: float) -> bool:
        left, top, width, height = self.microphone_rect()
        return left <= x <= left + width and top <= y <= top + height

    @staticmethod
    def _short_microphone_name(name: str) -> str:
        shortened = name
        for suffix in (" Estéreo analógico", " Analog Stereo", " Stereo Analogico"):
            if shortened.endswith(suffix):
                shortened = shortened[: -len(suffix)]
        return shortened or "Microfone padrão"

    @staticmethod
    def _circle(context: cairo.Context, x: float, y: float, radius: float) -> None:
        context.new_sub_path()
        context.arc(x, y, radius, 0, math.tau)
        context.fill()

    @staticmethod
    def _rounded_rectangle(
        context: cairo.Context,
        x: float,
        y: float,
        width: float,
        height: float,
        radius: float,
    ) -> None:
        radius = min(radius, width / 2, height / 2)
        context.new_sub_path()
        context.arc(x + width - radius, y + radius, radius, -math.pi / 2, 0)
        context.arc(x + width - radius, y + height - radius, radius, 0, math.pi / 2)
        context.arc(x + radius, y + height - radius, radius, math.pi / 2, math.pi)
        context.arc(x + radius, y + radius, radius, math.pi, math.pi * 1.5)
        context.close_path()

    @classmethod
    def _microphone_icon(cls, context: cairo.Context, x: float, y: float) -> None:
        context.set_source_rgba(0.76, 0.75, 0.79, 1.0)
        context.set_line_width(1.45)
        context.set_line_cap(cairo.LineCap.ROUND)
        cls._rounded_rectangle(context, x - 3.2, y - 7.2, 6.4, 10.2, 3.2)
        context.stroke()
        context.arc(x, y - 1.4, 5.1, 0, math.pi)
        context.stroke()
        context.move_to(x, y + 3.8)
        context.line_to(x, y + 7.0)
        context.move_to(x - 3.0, y + 7.0)
        context.line_to(x + 3.0, y + 7.0)
        context.stroke()

    @staticmethod
    def _text(
        context: cairo.Context,
        text: str,
        x: float,
        width: float,
        color: tuple[float, float, float, float],
        *,
        family: str = "Inter",
        size: float = 11.5,
        weight: Pango.Weight = Pango.Weight.MEDIUM,
        align: Pango.Alignment = Pango.Alignment.LEFT,
    ) -> None:
        layout = PangoCairo.create_layout(context)
        description = Pango.FontDescription()
        description.set_family(family)
        description.set_absolute_size(size * Pango.SCALE)
        description.set_weight(weight)
        layout.set_font_description(description)
        layout.set_text(text, -1)
        layout.set_width(max(1, int(width * Pango.SCALE)))
        layout.set_ellipsize(Pango.EllipsizeMode.END)
        layout.set_alignment(align)
        _, logical = layout.get_pixel_extents()
        context.set_source_rgba(*color)
        context.move_to(x, (44 - logical.height) / 2 - logical.y)
        PangoCairo.show_layout(context, layout)

    def _draw_recording(self, context: cairo.Context, width: int) -> None:
        red = ACCENTS["recording"]
        context.set_source_rgba(*red, 1.0)
        self._circle(context, 17, 22, 4.1)

        x = 32.0
        bar_width = 2.6
        gap = 3.7
        for index, level in enumerate(self.levels):
            bar_height = 3.5 + level * 24
            y = (44 - bar_height) / 2
            alpha = 0.42 + 0.58 * ((index + 1) / len(self.levels))
            context.set_source_rgba(0.96, 0.95, 0.93, alpha)
            context.set_line_width(bar_width)
            context.set_line_cap(cairo.LineCap.ROUND)
            context.move_to(x, y + bar_width / 2)
            context.line_to(x, y + bar_height - bar_width / 2)
            context.stroke()
            x += gap + bar_width

        self._text(
            context,
            self.timer,
            132,
            40,
            (0.72, 0.71, 0.74, 1.0),
            family="JetBrains Mono",
            size=10.5,
            weight=Pango.Weight.SEMIBOLD,
            align=Pango.Alignment.RIGHT,
        )

        context.set_source_rgba(1.0, 1.0, 1.0, 0.10)
        context.set_line_width(1.0)
        context.move_to(181.5, 12)
        context.line_to(181.5, 32)
        context.stroke()

        left, top, button_width, button_height = self.microphone_rect()
        if self.microphone_hovered:
            context.set_source_rgba(1.0, 1.0, 1.0, 0.075)
            self._rounded_rectangle(context, left, top, button_width, button_height, 17)
            context.fill()

        self._microphone_icon(context, left + 16, 21.5)
        self._text(
            context,
            self.microphone_name,
            left + 29,
            button_width - 49,
            (0.87, 0.86, 0.89, 1.0),
            size=10.7,
            weight=Pango.Weight.MEDIUM,
        )

        chevron_x = left + button_width - 15
        context.set_source_rgba(0.58, 0.57, 0.61, 1.0)
        context.set_line_width(1.35)
        context.set_line_cap(cairo.LineCap.ROUND)
        context.set_line_join(cairo.LineJoin.ROUND)
        context.move_to(chevron_x - 3, 20)
        context.line_to(chevron_x, 23)
        context.line_to(chevron_x + 3, 20)
        context.stroke()

    def _draw_status(self, context: cairo.Context, width: int) -> None:
        accent = ACCENTS[self.mode]
        context.set_source_rgba(*accent, 0.15)
        self._circle(context, 20, 22, 9)

        if self.mode == "done":
            context.set_source_rgba(*accent, 1.0)
            context.set_line_width(2.0)
            context.set_line_cap(cairo.LineCap.ROUND)
            context.set_line_join(cairo.LineJoin.ROUND)
            context.move_to(16.5, 22)
            context.line_to(19.1, 24.6)
            context.line_to(24.1, 18.9)
            context.stroke()
        elif self.mode == "error":
            self._text(
                context,
                "!",
                13,
                14,
                (*accent, 1.0),
                size=12,
                weight=Pango.Weight.BOLD,
                align=Pango.Alignment.CENTER,
            )
        elif self.mode == "processing":
            for index in range(3):
                height = 4 + 5 * (0.5 + 0.5 * math.sin(self.phase + index * 1.2))
                context.set_source_rgba(*accent, 0.62 + index * 0.16)
                context.set_line_width(2.2)
                context.set_line_cap(cairo.LineCap.ROUND)
                context.move_to(17 + index * 3, 22 - height / 2)
                context.line_to(17 + index * 3, 22 + height / 2)
                context.stroke()
        else:
            context.set_source_rgba(*accent, 1.0)
            self._circle(context, 20, 22, 3.5)

        text = self.message if self.mode == "error" else STATUS_TEXT[self.mode]
        self._text(
            context,
            text,
            38,
            width - 52,
            (0.92, 0.91, 0.93, 1.0),
            size=11.3,
            weight=Pango.Weight.SEMIBOLD,
        )

    def _draw(self, _widget: Gtk.Widget, context: cairo.Context) -> bool:
        width = self.get_allocated_width()
        if self.mode == "recording":
            self._draw_recording(context, width)
        else:
            self._draw_status(context, width)
        return False


class VoxOverlay:
    def __init__(self, runtime_dir: Path) -> None:
        self.runtime_dir = runtime_dir
        self.status_file = runtime_dir / "desktop-status.json"
        self.state_file = runtime_dir / "recording.json"
        self.last_status_mtime = 0
        self.mode = "starting"
        self.started_at: dt.datetime | None = None
        self.smoothed_level = 0.0
        self.exit_deadline: float | None = None
        self.positioned_size: tuple[int, int] | None = None
        self.microphone_node = ""
        self.microphone_menu: Gtk.Menu | None = None
        self.sources_script = Path(__file__).with_name("vox_audio_sources.py")
        self.controller_script = Path(__file__).with_name("vox-desktop-toggle.sh")

        lock_path = runtime_dir / "desktop-overlay.lock"
        self.lock = lock_path.open("a+")
        try:
            fcntl.flock(self.lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise SystemExit(0)

        self.window = Gtk.Window(title="Vox Voice Capture")
        self.window.set_name("vox-overlay")
        self.window.set_resizable(False)
        self.window.set_decorated(False)
        self.window.set_keep_above(True)
        self.window.set_skip_taskbar_hint(True)
        self.window.set_skip_pager_hint(True)
        self.window.set_accept_focus(False)
        self.window.set_focus_on_map(False)
        self.window.set_type_hint(Gdk.WindowTypeHint.NOTIFICATION)
        self.window.set_app_paintable(True)

        screen = self.window.get_screen()
        visual = screen.get_rgba_visual()
        if visual is not None:
            self.window.set_visual(visual)

        self.bar = FlowBar()
        self.bar.add_events(
            Gdk.EventMask.BUTTON_PRESS_MASK
            | Gdk.EventMask.ENTER_NOTIFY_MASK
            | Gdk.EventMask.LEAVE_NOTIFY_MASK
        )
        self.bar.set_tooltip_text("Trocar microfone")
        self.bar.connect("button-press-event", self._on_button_press)
        self.bar.connect("enter-notify-event", self._on_pointer_enter)
        self.bar.connect("leave-notify-event", self._on_pointer_leave)
        surface = Gtk.Box()
        surface.set_name("vox-surface")
        surface.set_margin_start(1)
        surface.set_margin_end(1)
        surface.set_margin_top(1)
        surface.set_margin_bottom(1)
        surface.pack_start(self.bar, True, True, 0)
        self.window.add(surface)

        css = Gtk.CssProvider()
        css.load_from_data(
            b"""
            #vox-overlay { background-color: transparent; }
            #vox-surface {
                background-color: rgba(14, 15, 18, 0.97);
                border: 1px solid rgba(255, 255, 255, 0.13);
                border-radius: 23px;
            }
            """
        )
        Gtk.StyleContext.add_provider_for_screen(screen, css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

        self.window.connect("destroy", Gtk.main_quit)
        self.window.connect("realize", self._update_input_shape)
        self.window.connect("size-allocate", self._on_size_allocate)
        self.window.show_all()
        GLib.idle_add(self._position_window)
        GLib.timeout_add(50, self._tick)

    def _update_input_shape(self, *_args: object) -> bool:
        gdk_window = self.window.get_window()
        if gdk_window is not None:
            if self.mode == "recording":
                left, top, width, height = self.bar.microphone_rect()
                region = cairo.Region(cairo.RectangleInt(left + 1, top + 1, width, height))
            else:
                region = cairo.Region()
            gdk_window.input_shape_combine_region(region, 0, 0)
        return False

    def _on_size_allocate(self, _window: Gtk.Window, allocation: Gdk.Rectangle) -> None:
        size = (allocation.width, allocation.height)
        if size != self.positioned_size:
            self.positioned_size = size
            GLib.idle_add(self._position_window)
            GLib.idle_add(self._update_input_shape)

    def _on_pointer_enter(self, *_args: object) -> bool:
        if self.mode == "recording":
            self.bar.set_microphone_hovered(True)
        return False

    def _on_pointer_leave(self, *_args: object) -> bool:
        self.bar.set_microphone_hovered(False)
        return False

    def _on_button_press(self, _widget: Gtk.Widget, event: Gdk.EventButton) -> bool:
        if event.button != 1 or self.mode != "recording":
            return False
        if not self.bar.microphone_hit(event.x, event.y):
            return False
        self._show_microphone_menu(event)
        return True

    def _source_snapshot(self) -> dict[str, object]:
        completed = subprocess.run(
            (sys.executable, str(self.sources_script), "list"),
            check=True,
            capture_output=True,
            text=True,
            timeout=4,
        )
        return json.loads(completed.stdout)

    def _show_microphone_menu(self, event: Gdk.EventButton) -> None:
        try:
            snapshot = self._source_snapshot()
            sources = snapshot.get("sources", [])
            if not isinstance(sources, list) or not sources:
                raise RuntimeError("nenhum microfone disponível")
        except (json.JSONDecodeError, OSError, RuntimeError, subprocess.SubprocessError) as error:
            print(f"vox: não foi possível listar microfones: {error}", file=sys.stderr)
            return

        if self.microphone_menu is not None:
            self.microphone_menu.popdown()
        menu = Gtk.Menu()
        self.microphone_menu = menu
        group: list[Gtk.RadioMenuItem] = []
        for source in sources:
            if not isinstance(source, dict):
                continue
            description = source.get("description")
            source_id = source.get("id")
            node_name = source.get("name")
            if not isinstance(description, str) or not isinstance(source_id, int):
                continue
            if not isinstance(node_name, str):
                continue
            item = Gtk.RadioMenuItem.new_with_label(group, description)
            group = item.get_group()
            selected = node_name == self.microphone_node
            item.set_active(selected)
            if selected:
                item.set_sensitive(False)
            else:
                item.connect("activate", self._select_microphone, source_id)
            menu.append(item)

        menu.connect("deactivate", self._on_microphone_menu_deactivate)
        menu.show_all()
        left, top, width, _height = self.bar.microphone_rect()
        anchor = Gdk.Rectangle()
        anchor.x = left + 1
        anchor.y = top + 1
        anchor.width = width
        anchor.height = 1
        menu.popup_at_rect(
            self.window.get_window(),
            anchor,
            Gdk.Gravity.NORTH_EAST,
            Gdk.Gravity.SOUTH_EAST,
            event,
        )

    def _on_microphone_menu_deactivate(self, *_args: object) -> None:
        self.bar.set_microphone_hovered(False)
        self.microphone_menu = None

    def _select_microphone(self, _item: Gtk.MenuItem, source_id: int) -> None:
        try:
            with open(os.devnull, "wb") as devnull:
                subprocess.Popen(
                    (str(self.controller_script), "--select-source", str(source_id)),
                    stdin=devnull,
                    stdout=devnull,
                    stderr=devnull,
                    start_new_session=True,
                )
        except OSError as error:
            print(f"vox: não foi possível trocar o microfone: {error}", file=sys.stderr)

    def _position_window(self) -> bool:
        display = Gdk.Display.get_default()
        if display is None:
            return False
        seat = display.get_default_seat()
        pointer = seat.get_pointer() if seat else None
        if pointer:
            _, pointer_x, pointer_y = pointer.get_position()
            monitor = display.get_monitor_at_point(pointer_x, pointer_y)
        else:
            monitor = display.get_primary_monitor()
        if monitor is None:
            monitor = display.get_monitor(0)
        if monitor is None:
            return False
        geometry = monitor.get_geometry()
        width = self.window.get_allocated_width()
        height = self.window.get_allocated_height()
        x = geometry.x + (geometry.width - width) // 2
        y = geometry.y + geometry.height - height - 30
        self.window.move(x, y)
        return False

    def _read_status(self) -> dict[str, str] | None:
        try:
            stat = self.status_file.stat()
            if stat.st_mtime_ns == self.last_status_mtime:
                return None
            self.last_status_mtime = stat.st_mtime_ns
            return json.loads(self.status_file.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return None

    def _read_recording_state(self) -> dict[str, object] | None:
        try:
            return json.loads(self.state_file.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return None

    def _audio_level(self, audio_path: str | None) -> float:
        if not audio_path:
            return 0.0
        try:
            with open(audio_path, "rb") as audio:
                size = audio.seek(0, os.SEEK_END)
                audio.seek(max(44, size - 4096))
                data = audio.read()
            if len(data) < 2:
                return 0.0
            data = data[: len(data) - len(data) % 2]
            samples = array.array("h")
            samples.frombytes(data)
            if sys.byteorder != "little":
                samples.byteswap()
            rms = math.sqrt(sum(sample * sample for sample in samples) / max(1, len(samples)))
            return min(1.0, rms / 6200.0)
        except OSError:
            return 0.0

    def _apply_status(self, status: dict[str, str]) -> None:
        mode = status.get("state", "starting")
        self.mode = mode if mode in ACCENTS else "error"
        self.bar.set_mode(self.mode, status.get("message", ""))
        microphone_name = status.get("microphone_name", "Microfone padrão")
        self.microphone_node = status.get("microphone_node", "")
        self.bar.set_microphone(microphone_name)
        GLib.idle_add(self._update_input_shape)

        if self.mode == "recording":
            state = self._read_recording_state()
            value = state.get("started_at") if state else None
            if isinstance(value, str):
                try:
                    self.started_at = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
                except ValueError:
                    self.started_at = dt.datetime.now(dt.timezone.utc)
            self.exit_deadline = None
        elif self.mode == "done":
            self.exit_deadline = time.monotonic() + 1.2
        elif self.mode == "error":
            self.exit_deadline = time.monotonic() + 4.5

    def _tick(self) -> bool:
        status = self._read_status()
        if status:
            self._apply_status(status)

        state = self._read_recording_state()
        audio_path = state.get("audio_path") if state else None
        raw_level = self._audio_level(audio_path if isinstance(audio_path, str) else None)
        self.smoothed_level = self.smoothed_level * 0.58 + raw_level * 0.42

        timer = "00:00"
        if self.mode == "recording" and self.started_at:
            elapsed = max(0, int((dt.datetime.now(dt.timezone.utc) - self.started_at).total_seconds()))
            timer = f"{elapsed // 60:02d}:{elapsed % 60:02d}"
        self.bar.update(self.smoothed_level, timer)

        if self.exit_deadline is not None and time.monotonic() >= self.exit_deadline:
            Gtk.main_quit()
            return False
        return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--runtime-dir", required=True, type=Path)
    args = parser.parse_args()
    if args.runtime_dir.is_symlink():
        raise SystemExit("vox: runtime directory must not be a symlink")
    args.runtime_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    runtime_stat = args.runtime_dir.stat()
    if not args.runtime_dir.is_dir() or runtime_stat.st_uid != os.getuid():
        raise SystemExit("vox: runtime directory must be owned by the current user")
    args.runtime_dir.chmod(0o700)
    signal.signal(signal.SIGTERM, lambda *_args: Gtk.main_quit())
    VoxOverlay(args.runtime_dir)
    Gtk.main()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
