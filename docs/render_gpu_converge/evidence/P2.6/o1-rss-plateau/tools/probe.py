#!/usr/bin/env python3
"""Launch one real window and sample its process memory.

Samples /proc/<pid>/status at a fixed interval and records VmRSS plus the
RssAnon/RssFile/RssShmem split, which separates Go/driver anonymous pages from
file-backed mappings (GPU driver / shared memory). Also parses GODEBUG=gctrace
lines produced by the child into a heap series.

Usage:
  probe.py --bin /tmp/o1/bin/ui_l1_spinner --out <dir> --secs 55 \
           --env RUN_SECONDS=55 --env GPUI_BACKEND=go
"""

import argparse
import os
import re
import signal
import subprocess
import sys
import time

SAMPLES = ("VmRSS", "VmHWM", "RssAnon", "RssFile", "RssShmem")
GCTRACE = re.compile(
    r"^gc (\d+) @([0-9.]+)s .*?([0-9]+)->([0-9]+)->([0-9]+) MB, ([0-9]+) MB goal"
)


def read_status(pid):
    vals = {}
    try:
        with open(f"/proc/{pid}/status", "r") as f:
            for line in f:
                key, _, rest = line.partition(":")
                if key in SAMPLES:
                    parts = rest.split()
                    if parts:
                        vals[key] = int(parts[0])
    except (FileNotFoundError, ProcessLookupError):
        return None
    return vals


def read_smaps(pid):
    """Aggregate VmRSS per mapping name (anonymous VMAs have no name)."""
    agg = {}
    try:
        with open(f"/proc/{pid}/smaps", "r") as f:
            name = None
            for line in f:
                if line[0] not in " " and "-" in line:
                    parts = line.split()
                    name = " ".join(parts[5:]) if len(parts) >= 6 else "(anon)"
                    agg.setdefault(name, 0)
                elif line.startswith("Rss:"):
                    agg[name] = agg.get(name, 0) + int(line.split()[1])
    except (FileNotFoundError, ProcessLookupError, ValueError):
        return None
    return agg


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bin", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--secs", type=int, default=55)
    ap.add_argument("--interval", type=float, default=0.25)
    ap.add_argument("--smaps-at", default="", help="comma-separated seconds for smaps snapshots")
    ap.add_argument("--arg", action="append", default=[], help="extra child argument (repeatable)")
    ap.add_argument("--env", action="append", default=[])
    args = ap.parse_args()

    os.makedirs(args.out, exist_ok=True)
    env = dict(os.environ)
    for kv in args.env:
        k, _, v = kv.partition("=")
        env[k] = v

    stdout_path = os.path.join(args.out, "stdout.txt")
    stderr_path = os.path.join(args.out, "stderr.txt")
    rss_path = os.path.join(args.out, "rss.tsv")
    heap_path = os.path.join(args.out, "gctrace.tsv")

    smaps_at = sorted(float(x) for x in args.smaps_at.split(",") if x)
    smaps_done = set()
    t0 = time.time()
    with open(stdout_path, "w") as so, open(stderr_path, "w") as se:
        proc = subprocess.Popen([args.bin] + args.arg, stdout=so, stderr=se, env=env)
        samples = []
        while True:
            now = time.time() - t0
            st = read_status(proc.pid)
            if st is None:
                break
            samples.append((now, st))
            for at in smaps_at:
                if at not in smaps_done and now >= at:
                    snap = read_smaps(proc.pid)
                    if snap is not None:
                        with open(os.path.join(args.out, f"smaps_{int(at)}s.tsv"), "w") as sf:
                            sf.write("mapping\trss_kb\n")
                            for k, v in sorted(snap.items(), key=lambda kv: -kv[1]):
                                sf.write(f"{k!r}\t{v}\n")
                    smaps_done.add(at)
            if proc.poll() is not None:
                break
            time.sleep(args.interval)
        try:
            rc = proc.wait(timeout=args.secs + 20)
        except subprocess.TimeoutExpired:
            proc.send_signal(signal.SIGTERM)
            try:
                rc = proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                rc = -999

    with open(rss_path, "w") as f:
        f.write("t_sec\t" + "\t".join(SAMPLES) + "\n")
        for t, st in samples:
            f.write(f"{t:.3f}\t" + "\t".join(str(st.get(k, 0)) for k in SAMPLES) + "\n")

    heap = []
    with open(stderr_path, "r", errors="replace") as f:
        for line in f:
            m = GCTRACE.match(line.strip())
            if m:
                heap.append(tuple(float(x) if i in (1, 2, 3, 4, 5) else int(x)
                                  for i, x in enumerate(m.groups())))
    with open(heap_path, "w") as f:
        f.write("gc\tt_sec\theap_start_mb\theap_end_mb\theap_live_mb\tgoal_mb\n")
        for row in heap:
            f.write(f"{int(row[0])}\t{row[1]:.3f}\t{row[2]:.0f}\t{row[3]:.0f}\t{row[4]:.0f}\t{row[5]:.0f}\n")

    wall = time.time() - t0
    print(f"rc={rc} wall={wall:.1f}s samples={len(samples)} gcs={len(heap)}")
    if samples:
        first, last = samples[0][1], samples[-1][1]
        print(f"rss start={first.get('VmRSS', 0)} end={last.get('VmRSS', 0)} "
              f"peak={max(s[1].get('VmHWM', 0) for s in samples)} KB")
        print(f"anon start={first.get('RssAnon', 0)} end={last.get('RssAnon', 0)} KB; "
              f"file start={first.get('RssFile', 0)} end={last.get('RssFile', 0)} KB")
    return 0 if rc == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
