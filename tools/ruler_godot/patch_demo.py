#!/usr/bin/env python3
"""Patch a *copy* of the upstream bullet_shower demo for the ruler.

The upstream checkout is never modified. The patch only does two things:
  * register the tiny FPS autoload (fps_probe.gd is copied by setup.sh);
  * make BULLET_COUNT configurable with GODOT_BULLETS for capacity probes.

The default remains the upstream 500 bullets.
"""

from __future__ import annotations

import sys
from pathlib import Path


def main() -> int:
    root = Path(sys.argv[1])
    project = root / "project.godot"
    bullets = root / "bullets.gd"

    p = project.read_text()
    if "FpsProbe=" not in p:
        line = 'FpsProbe="*res://fps_probe.gd"\n'
        if "[autoload]" in p:
            p = p.replace("[autoload]\n", "[autoload]\n\n" + line, 1)
        else:
            p += "\n[autoload]\n\n" + line
        project.write_text(p)

    b = bullets.read_text()
    if "var bullet_count := 500" not in b:
        b = b.replace("const BULLET_COUNT = 500", "var bullet_count := 500", 1)
    marker = "func _ready() -> void:\n"
    if "GODOT_BULLETS" not in b:
        b = b.replace(
            marker,
            marker
            + '\tif OS.has_environment("GODOT_BULLETS"):\n'
            + '\t\tbullet_count = int(OS.get_environment("GODOT_BULLETS"))\n',
            1,
        )
    b = b.replace("for _i in BULLET_COUNT:", "for _i in bullet_count:")
    bullets.write_text(b)

    if (
        "var bullet_count := 500" not in b
        or "GODOT_BULLETS" not in b
        or "for _i in bullet_count:" not in b
        or "BULLET_COUNT" in b
    ):
        raise SystemExit(f"patch failed: unexpected upstream demo at {root}")
    if "FpsProbe=" not in project.read_text():
        raise SystemExit("patch failed: FpsProbe autoload missing")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
