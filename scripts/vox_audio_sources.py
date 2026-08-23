#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from typing import Any


DEFAULT_NODE_PATTERN = re.compile(r'^\s*\*?\s*node\.name = "(.*)"\s*$')


def parse_sources(payload: str) -> list[dict[str, object]]:
    objects: Any = json.loads(payload)
    if not isinstance(objects, list):
        raise RuntimeError("PipeWire returned an unexpected payload")
    sources: list[dict[str, object]] = []
    for item in objects:
        if not isinstance(item, dict):
            continue
        if item.get("type") != "PipeWire:Interface:Node":
            continue
        info = item.get("info")
        if not isinstance(info, dict):
            continue
        props = info.get("props")
        if not isinstance(props, dict):
            continue
        if props.get("media.class") != "Audio/Source":
            continue
        node_name = props.get("node.name")
        if not isinstance(node_name, str) or not node_name or node_name.endswith(".monitor"):
            continue
        description = props.get("node.description") or props.get("node.nick") or node_name
        source_id = item.get("id")
        if not isinstance(source_id, int) or source_id < 0:
            continue
        clean_description = " ".join(str(description).split())
        sources.append(
            {
                "id": source_id,
                "name": node_name,
                "description": clean_description or node_name,
            }
        )
    return sorted(sources, key=lambda source: str(source["description"]).casefold())


def parse_default_node(payload: str) -> str:
    for line in payload.splitlines():
        match = DEFAULT_NODE_PATTERN.match(line)
        if match:
            return match.group(1)
    raise RuntimeError("PipeWire did not report a default audio source")


def command_output(*args: str) -> str:
    completed = subprocess.run(
        args,
        check=True,
        capture_output=True,
        text=True,
        timeout=4,
    )
    return completed.stdout


def source_snapshot() -> dict[str, object]:
    sources = parse_sources(command_output("pw-dump"))
    default_name = parse_default_node(command_output("wpctl", "inspect", "@DEFAULT_AUDIO_SOURCE@"))
    for source in sources:
        source["default"] = source["name"] == default_name
    return {"default": default_name, "sources": sources}


def select_source(source_id: int) -> dict[str, object]:
    snapshot = source_snapshot()
    selected = next(
        (source for source in snapshot["sources"] if source["id"] == source_id),
        None,
    )
    if selected is None:
        raise RuntimeError(f"audio source {source_id} is no longer available")
    subprocess.run(
        ("wpctl", "set-default", str(source_id)),
        check=True,
        capture_output=True,
        text=True,
        timeout=4,
    )
    selected["default"] = True
    return selected


def main() -> int:
    parser = argparse.ArgumentParser(description="List and select PipeWire microphones")
    subcommands = parser.add_subparsers(dest="command", required=True)
    subcommands.add_parser("list")
    select = subcommands.add_parser("select")
    select.add_argument("--id", required=True, type=int)
    args = parser.parse_args()

    try:
        result = source_snapshot() if args.command == "list" else select_source(args.id)
        print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))
        return 0
    except (json.JSONDecodeError, KeyError, OSError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"vox: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
