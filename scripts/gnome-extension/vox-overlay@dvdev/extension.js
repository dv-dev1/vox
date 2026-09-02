// Vox status bar rendered inside gnome-shell itself.
//
// Why this exists: mutter does not implement wlr-layer-shell (the protocol
// gtk-layer-shell needs), so a normal client window has no way to force
// itself above other windows on GNOME/Wayland — it can render behind the
// focused app until alt-tabbed into view. An actor added to gnome-shell's
// own compositor stack via Main.layoutManager.addChrome has no such problem:
// it *is* part of the thing doing the stacking, the same mechanism the
// volume/brightness OSD uses.
//
// This polls the same desktop-status.json / recording.json files that
// scripts/vox-desktop-toggle.sh and the old scripts/vox-overlay.py already
// used, so the shell-side controller needs no changes to talk to it.

import St from 'gi://St';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import Clutter from 'gi://Clutter';
import Pango from 'gi://Pango';

import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const ACCENTS = {
    starting: 'rgba(158,143,255,1)',
    recording: 'rgba(255,77,99,1)',
    processing: 'rgba(158,143,255,1)',
    done: 'rgba(74,214,153,1)',
    error: 'rgba(255,148,79,1)',
};

const STATUS_TEXT = {
    starting: 'Ativando microfone…',
    processing: 'Transcrevendo…',
    done: 'Texto inserido',
};

const BAR_COUNT = 15;
const TICK_MS = 50;
const BOTTOM_MARGIN = 30;
const MIC_SUFFIXES = [' Estéreo analógico', ' Analog Stereo', ' Stereo Analogico'];

function shortMicName(name) {
    let shortened = name;
    for (const suffix of MIC_SUFFIXES) {
        if (shortened.endsWith(suffix)) {
            shortened = shortened.slice(0, -suffix.length);
            break;
        }
    }
    return shortened || 'Microfone padrão';
}

function readJSON(path) {
    try {
        const [ok, bytes] = GLib.file_get_contents(path);
        if (!ok)
            return null;
        return JSON.parse(new TextDecoder('utf-8').decode(bytes));
    } catch (error) {
        return null;
    }
}

export default class VoxOverlayExtension extends Extension {
    enable() {
        const runtimeDir = GLib.build_filenamev([GLib.get_user_runtime_dir(), 'vox']);
        this._statusPath = GLib.build_filenamev([runtimeDir, 'desktop-status.json']);
        this._statePath = GLib.build_filenamev([runtimeDir, 'recording.json']);
        this._lastStatusMtimeUsec = -1;
        this._mode = null;
        this._startedAtMs = null;
        this._exitDeadlineUsec = null;
        this._levels = new Array(BAR_COUNT).fill(0.08);
        this._smoothedLevel = 0;

        this._buildActor();

        this._timeoutId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, TICK_MS, () => {
            this._tick();
            return GLib.SOURCE_CONTINUE;
        });
    }

    disable() {
        if (this._timeoutId) {
            GLib.source_remove(this._timeoutId);
            this._timeoutId = null;
        }
        if (this._actor) {
            Main.layoutManager.removeChrome(this._actor);
            this._actor.destroy();
            this._actor = null;
        }
        this._dot = null;
        this._label = null;
        this._barsBox = null;
        this._bars = null;
        this._timerLabel = null;
        this._micLabel = null;
    }

    _buildActor() {
        this._actor = new St.BoxLayout({
            style: 'background-color: rgba(14,15,18,0.97); ' +
                'border: 1px solid rgba(255,255,255,0.13); ' +
                'border-radius: 23px; padding: 7px 18px;',
            vertical: false,
            visible: false,
        });

        this._dot = new St.Widget({
            style: `width: 10px; height: 10px; border-radius: 5px; background-color: ${ACCENTS.starting};`,
            y_align: Clutter.ActorAlign.CENTER,
        });

        this._label = new St.Label({
            style: 'color: rgba(235,232,238,0.95); font-size: 11.5pt; font-weight: 600; ' +
                'padding-left: 10px; padding-right: 4px;',
            y_align: Clutter.ActorAlign.CENTER,
        });
        this._label.clutter_text.ellipsize = Pango.EllipsizeMode.END;

        this._barsBox = new St.BoxLayout({
            vertical: false,
            style: 'spacing: 4px; padding-left: 8px; padding-right: 4px;',
            y_align: Clutter.ActorAlign.CENTER,
            visible: false,
        });
        this._bars = [];
        for (let i = 0; i < BAR_COUNT; i++) {
            const bar = new St.Widget({
                style: 'width: 3px; border-radius: 2px; background-color: rgba(245,242,238,0.7);',
                height: 8,
                y_align: Clutter.ActorAlign.CENTER,
            });
            this._bars.push(bar);
            this._barsBox.add_child(bar);
        }

        this._timerLabel = new St.Label({
            style: 'color: rgba(184,182,189,1); font-size: 10.5pt; font-family: monospace; padding-left: 8px;',
            visible: false,
            y_align: Clutter.ActorAlign.CENTER,
        });

        this._micLabel = new St.Label({
            style: 'color: rgba(222,220,227,1); font-size: 10.7pt; padding-left: 12px;',
            visible: false,
            y_align: Clutter.ActorAlign.CENTER,
        });
        this._micLabel.clutter_text.ellipsize = Pango.EllipsizeMode.END;

        this._actor.add_child(this._dot);
        this._actor.add_child(this._label);
        this._actor.add_child(this._barsBox);
        this._actor.add_child(this._timerLabel);
        this._actor.add_child(this._micLabel);

        // GNOME Shell 50 strictly validates addChrome's params object and
        // threw "Unrecognized parameter" on affectsInputRegion (put the
        // extension in the ERROR state). Editing extension.js does not take
        // effect until gnome-shell restarts — its ESM module is cached in
        // memory for the process lifetime, disable()/enable() just re-runs
        // the already-imported code — so this call is deliberately bare:
        // no params object at all, to avoid guessing at a 50-specific key
        // name and burning another logout on a second wrong guess.
        // reactive:false is the click-passthrough substitute for the
        // affectsInputRegion key this dropped.
        this._actor.reactive = false;
        Main.layoutManager.addChrome(this._actor);
    }

    _monitorGeometry() {
        const index = global.display.get_current_monitor();
        return Main.layoutManager.monitors[index] || Main.layoutManager.primaryMonitor;
    }

    _reposition() {
        const geometry = this._monitorGeometry();
        if (!geometry)
            return;
        const width = this._actor.width;
        const height = this._actor.height;
        const x = geometry.x + Math.round((geometry.width - width) / 2);
        const y = geometry.y + geometry.height - height - BOTTOM_MARGIN;
        this._actor.set_position(x, y);
    }

    _readStatus() {
        let mtimeUsec;
        try {
            const file = Gio.File.new_for_path(this._statusPath);
            const info = file.query_info(
                'time::modified,time::modified-usec', Gio.FileQueryInfoFlags.NONE, null);
            mtimeUsec = info.get_attribute_uint64('time::modified') * 1000000 +
                info.get_attribute_uint32('time::modified-usec');
        } catch (error) {
            return;
        }
        if (mtimeUsec === this._lastStatusMtimeUsec)
            return;
        this._lastStatusMtimeUsec = mtimeUsec;

        const status = readJSON(this._statusPath);
        if (status)
            this._applyStatus(status);
    }

    _applyStatus(status) {
        const mode = ACCENTS[status.state] ? status.state : 'error';
        this._mode = mode;

        this._dot.set_style(
            `width: 10px; height: 10px; border-radius: 5px; background-color: ${ACCENTS[mode]};`);
        const text = mode === 'error' ? status.message || '' : STATUS_TEXT[mode] || status.message || '';
        this._label.set_text(text);

        const micName = shortMicName(status.microphone_name || 'Microfone padrão');
        this._micLabel.set_text(micName);
        const recording = mode === 'recording';
        this._micLabel.visible = recording;
        this._barsBox.visible = recording;
        this._timerLabel.visible = recording;

        if (recording) {
            const state = readJSON(this._statePath);
            const startedAt = state && typeof state.started_at === 'string' ?
                Date.parse(state.started_at) : NaN;
            this._startedAtMs = Number.isNaN(startedAt) ? Date.now() : startedAt;
            this._exitDeadlineUsec = null;
        } else if (mode === 'done') {
            this._exitDeadlineUsec = GLib.get_monotonic_time() + 1.2 * 1e6;
        } else if (mode === 'error') {
            this._exitDeadlineUsec = GLib.get_monotonic_time() + 4.5 * 1e6;
        } else {
            this._exitDeadlineUsec = null;
        }

        this._actor.visible = true;
        this._reposition();
    }

    _updateLevel() {
        let raw = 0;
        if (this._mode === 'recording') {
            const state = readJSON(this._statePath);
            const audioPath = state && typeof state.audio_path === 'string' ? state.audio_path : null;
            if (audioPath)
                raw = this._audioLevel(audioPath);
        }
        this._smoothedLevel = this._smoothedLevel * 0.58 + raw * 0.42;
        if (this._mode === 'recording') {
            this._levels.shift();
            this._levels.push(Math.max(0.06, Math.min(1, this._smoothedLevel)));
            for (let i = 0; i < BAR_COUNT; i++)
                this._bars[i].height = Math.round(3.5 + this._levels[i] * 24);
        }
    }

    _audioLevel(audioPath) {
        let stream;
        try {
            const file = Gio.File.new_for_path(audioPath);
            const info = file.query_info('standard::size', Gio.FileQueryInfoFlags.NONE, null);
            const size = info.get_size();
            if (size <= 44)
                return 0;
            const start = Math.max(44, size - 4096);
            stream = file.read(null);
            stream.seek(start, GLib.SeekType.SET, null);
            const bytes = stream.read_bytes(size - start, null);
            const data = bytes.get_data();
            const sampleCount = Math.floor(data.length / 2);
            if (sampleCount < 1)
                return 0;
            const view = new DataView(data.buffer, data.byteOffset, sampleCount * 2);
            let sumSquares = 0;
            for (let i = 0; i < sampleCount; i++) {
                const sample = view.getInt16(i * 2, true);
                sumSquares += sample * sample;
            }
            const rms = Math.sqrt(sumSquares / sampleCount);
            return Math.min(1, rms / 6200);
        } catch (error) {
            return 0;
        } finally {
            if (stream)
                stream.close(null);
        }
    }

    _updateTimer() {
        if (this._mode !== 'recording' || this._startedAtMs === null)
            return;
        const elapsed = Math.max(0, Math.floor((Date.now() - this._startedAtMs) / 1000));
        const minutes = String(Math.floor(elapsed / 60)).padStart(2, '0');
        const seconds = String(elapsed % 60).padStart(2, '0');
        this._timerLabel.set_text(`${minutes}:${seconds}`);
    }

    _tick() {
        this._readStatus();
        this._updateLevel();
        this._updateTimer();
        if (this._actor.visible)
            this._reposition();
        if (this._exitDeadlineUsec !== null && GLib.get_monotonic_time() >= this._exitDeadlineUsec) {
            this._actor.visible = false;
            this._mode = null;
            this._exitDeadlineUsec = null;
        }
    }
}
