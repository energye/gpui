#!/usr/bin/env python3
"""Lint docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md for table errors and append drift.

Run before committing a ledger change:
    python3 scripts/check_tasks_ledger.py

The checks encode the rules in the ledger's §6.1/§6.2:
- tables are structurally valid (consistent columns, legal separator);
- task IDs are unique (no appended duplicate task blocks);
- exactly one active "当前任务验证载体" and zero "上一任务验证载体";
- evidence paths in the main table exist;
- row/file budgets stay small (details belong in evidence/ or the archive).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DOC = ROOT / "docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md"
EVIDENCE_ROOT = ROOT / "docs/render_gpu_converge"

MAX_BYTES = 100 * 1024
MAX_LINES = 600
MAX_CELL = 500
MAX_TIMELINE_CELL = 250
MAX_INDEX_CELL = 250
MAX_CURRENT_CARRIER_LINES = 20

ERRORS: list[str] = []


def error(message: str) -> None:
    ERRORS.append(message)


def cells_of(line: str) -> list[str]:
    return [c.strip() for c in line.strip().strip("|").split("|")]


def check_evidence_paths(where: str, cell: str) -> None:
    for raw in re.findall(r"`((?:evidence|baseline|archive)/[^`]+)`", cell):
        path = raw.split(" ")[0]
        if "*" in path or "{" in path:
            continue
        path = re.sub(r":\d+$", "", path)
        if not (EVIDENCE_ROOT / path).exists():
            error(f"{where}: evidence path does not exist: {path}")


def main() -> int:
    text = DOC.read_text(encoding="utf-8")
    lines = text.splitlines()

    size = len(text.encode("utf-8"))
    if size > MAX_BYTES:
        error(f"file is {size} bytes, over budget {MAX_BYTES}; move details to evidence/")
    if len(lines) > MAX_LINES:
        error(f"file is {len(lines)} lines, over budget {MAX_LINES}; move details to evidence/")

    fences = [i for i, line in enumerate(lines, 1) if line.strip().startswith("```")]
    if len(fences) % 2:
        error(f"unbalanced code fences at lines {fences}")

    # Lines inside code fences are templates/examples, not ledger structure.
    fenced: set[int] = set()
    inside_fence = False
    for i, line in enumerate(lines, 1):
        if line.strip().startswith("```"):
            fenced.add(i)
            inside_fence = not inside_fence
            continue
        if inside_fence:
            fenced.add(i)

    # Collect contiguous table blocks.
    blocks: list[tuple[int, list[tuple[int, str]]]] = []
    current: list[tuple[int, str]] = []
    start = 0
    for i, line in enumerate(lines, 1):
        if i in fenced:
            if current:
                blocks.append((start, current))
                current = []
            continue
        if line.strip().startswith("|"):
            if not current:
                start = i
            current.append((i, line))
        elif current:
            blocks.append((start, current))
            current = []
    if current:
        blocks.append((start, current))

    for start, block in blocks:
        widths = {line.count("|") for _, line in block}
        if len(widths) != 1:
            error(f"table starting line {start} has inconsistent columns: {sorted(widths)}")
            continue
        if len(block) < 2:
            error(f"table starting line {start} has no separator row")
            continue

        header = cells_of(block[0][1])
        separator = cells_of(block[1][1])
        if len(header) != len(separator):
            error(f"table starting line {start}: header has {len(header)} columns, separator has {len(separator)}")
        for col in separator:
            if not re.fullmatch(r":?-{3,}:?", col):
                error(f"table starting line {start}: invalid separator cell {col!r}")

        for line_no, line in block[2:]:
            cells = cells_of(line)
            if any(cell == "" for cell in cells):
                error(f"line {line_no}: empty table cell")
            for cell in cells:
                if len(cell) > MAX_CELL:
                    error(
                        f"line {line_no}: cell is {len(cell)} chars, over budget {MAX_CELL}; "
                        "move details to evidence/"
                    )

        # Main task table.
        if header == ["编号", "任务", "依赖", "状态", "证据目录"]:
            seen: dict[str, int] = {}
            for line_no, line in block[2:]:
                cells = cells_of(line)
                task_id = cells[0]
                if task_id in seen:
                    error(f"line {line_no}: duplicate main task {task_id} (first at line {seen[task_id]})")
                seen[task_id] = line_no
                if not any(s in cells[3] for s in ("✅", "🟡", "⬜", "☑", "🔍", "➖", "⛔", "⏸")):
                    error(f"line {line_no}: main task {task_id} has no status symbol")
                check_evidence_paths(f"line {line_no} ({task_id})", cells[4])

        # P2 todo pool.
        elif header == ["编号", "来源", "现象 / 结论", "归属", "状态 / 证据"]:
            seen = {}
            for line_no, line in block[2:]:
                cells = cells_of(line)
                pool_id = cells[0]
                if pool_id in seen:
                    error(f"line {line_no}: duplicate pool item {pool_id} (first at line {seen[pool_id]})")
                seen[pool_id] = line_no
                if not any(s in cells[4] for s in ("✅", "🟡", "⬜", "☑", "🔍", "➖", "⛔", "⏸")):
                    error(f"line {line_no}: pool item {pool_id} has no status symbol in last column")
                check_evidence_paths(f"line {line_no} ({pool_id})", cells[4])

        # §2.1 special index.
        elif header == ["专项", "结论", "正本"]:
            for line_no, line in block[2:]:
                cells = cells_of(line)
                for cell in cells:
                    if len(cell) > MAX_INDEX_CELL:
                        error(
                            f"line {line_no}: index cell is {len(cell)} chars, "
                            f"over budget {MAX_INDEX_CELL}; move details to evidence/"
                        )
                check_evidence_paths(f"line {line_no}", cells[2])

        # §7/§8 summaries stay short.
        elif header in (["日期", "任务", "变更 / 结论", "证据"], ["日期", "修订"]):
            for line_no, line in block[2:]:
                for cell in cells_of(line):
                    if len(cell) > MAX_TIMELINE_CELL:
                        error(
                            f"line {line_no}: timeline/revision cell is {len(cell)} chars, "
                            f"over budget {MAX_TIMELINE_CELL}; move details to evidence/"
                        )

    # One active carrier, zero closed carrier blocks. Ignore the §4.2 template.
    active_lines = [
        i
        for i, line in enumerate(lines, 1)
        if line.startswith("**当前任务验证载体") and i not in fenced
    ]
    closed_lines = [
        i
        for i, line in enumerate(lines, 1)
        if line.startswith("**上一任务验证载体") and i not in fenced
    ]
    if len(active_lines) > 1:
        error(f"found {len(active_lines)} '当前任务验证载体' blocks; keep exactly one active block")
    if closed_lines:
        error(
            f"found {len(closed_lines)} '上一任务验证载体' blocks; replace a closed carrier with "
            "one index row instead of keeping the full block"
        )

    if len(active_lines) == 1:
        start = active_lines[0] - 1
        end = len(lines)
        for i in range(start + 1, len(lines)):
            if i + 1 in fenced:
                continue
            if lines[i].startswith("**") or lines[i].startswith("#"):
                end = i
                break
        carrier_lines = end - start
        if carrier_lines > MAX_CURRENT_CARRIER_LINES:
            error(
                f"current carrier block is {carrier_lines} lines, over budget "
                f"{MAX_CURRENT_CARRIER_LINES}; move process details to evidence/"
            )

    if ERRORS:
        print(f"FAIL {DOC.relative_to(ROOT)}")
        for message in ERRORS:
            print(f"  - {message}")
        return 1

    print(
        f"OK {DOC.relative_to(ROOT)}: {len(blocks)} tables, {len(lines)} lines, "
        f"{size} bytes, {len(active_lines)} active carrier(s)"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
