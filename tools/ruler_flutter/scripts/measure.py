#!/usr/bin/env python3
"""Same-machine CPU/RSS/fps measurement for the Flutter/Godot/gpui ruler.

The measurement window is:
  * wait for the real X11 window (not just the process);
  * warm up (default 5 s);
  * sample /proc/<pid>/stat + /proc/<pid>/status for `--duration` seconds;
  * screenshot the client area and check that it is not an all-black window;
  * request a normal close with WM_DELETE_WINDOW (SIGTERM is only a fallback).

CPU% is process-wide utime+stime over the measured wall window. RSS slope is
a linear fit in KB/min over the same window. FPS/p95/hitch come from the
application's own RULER_METRIC lines when present; Godot can use --print-fps.

Usage:
  python3 scripts/measure.py --scene spin --out /tmp/ruler/spin_1
  python3 scripts/measure.py --app /path/to/app --arg --flag \
      --env KEY=VAL --window-match TITLE --out /tmp/ruler/run
"""

from __future__ import annotations

import argparse
import collections
import json
import os
import re
import shutil
import statistics
import subprocess
import sys
import time
from pathlib import Path

try:
    from PIL import Image
except Exception:  # pragma: no cover - only used by the evidence path
    Image = None

try:
    from Xlib import X, display, protocol
except Exception:  # pragma: no cover
    display = None
    protocol = None


def client_geometry(win):
    """Absolute client geometry, robust against WM reparenting/placement."""
    try:
        out = subprocess.check_output(
            ["xwininfo", "-id", hex(win.id)],
            text=True, stderr=subprocess.DEVNULL,
        )
        vals = {}
        for line in out.splitlines():
            s = line.strip()
            for key, name in (
                ("x", "Absolute upper-left X:"),
                ("y", "Absolute upper-left Y:"),
                ("w", "Width:"),
                ("h", "Height:"),
            ):
                if s.startswith(name):
                    vals[key] = int(s.split(":", 1)[1])
        if len(vals) == 4:
            return vals["x"], vals["y"], vals["w"], vals["h"]
    except Exception:
        pass
    g = win.get_geometry()
    return g.x, g.y, g.width, g.height


def find_window(title_match: str, timeout_s: float = 20.0):
    if display is None:
        return None
    d = display.Display()
    root = d.screen().root
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        found = []
        stack = [root]
        while stack:
            w = stack.pop()
            try:
                children = w.query_tree().children
            except Exception:
                children = []
            for c in children:
                try:
                    name = c.get_wm_name()
                    if isinstance(name, bytes):
                        name = name.decode("utf-8", "replace")
                    if not name:
                        prop = c.get_full_property(
                            d.intern_atom("_NET_WM_NAME"), 0
                        )
                        if prop is not None:
                            v = prop.value
                            name = v.decode("utf-8", "replace") if isinstance(v, bytes) else str(v)
                    if name and title_match in name:
                        g = c.get_geometry()
                        if g.width > 20 and g.height > 20:
                            found.append((g.width * g.height, c, name, g))
                except Exception:
                    pass
                stack.append(c)
        if found:
            found.sort(key=lambda x: x[0], reverse=True)
            _, win, name, geom = found[0]
            ax, ay, ww, wh = client_geometry(win)
            return d, root, win, name, (ax, ay, ww, wh)
        time.sleep(0.2)
    return None


def raise_window(root, win):
    """Raise the client and its WM frame, then focus it."""
    try:
        top = win
        while True:
            parent = top.query_tree().parent
            if parent is None or parent == root:
                break
            top = parent
        top.raise_window()
        top.configure(stack_mode=X.Above)
    except Exception:
        try:
            win.raise_window()
        except Exception:
            pass


def request_close(d, win) -> bool:
    """Send WM_DELETE_WINDOW so the app can run its normal close path."""
    if d is None or win is None or protocol is None:
        return False
    try:
        wm_protocols = d.intern_atom("WM_PROTOCOLS")
        wm_delete = d.intern_atom("WM_DELETE_WINDOW")
        ev = protocol.event.ClientMessage(
            window=win,
            client_type=wm_protocols,
            data=(32, [wm_delete, X.CurrentTime, 0, 0, 0]),
        )
        win.send_event(ev, event_mask=X.NoEventMask, propagate=False)
        d.flush()
        return True
    except Exception as e:
        print(f"WM_DELETE_WINDOW failed: {e}", file=sys.stderr)
        return False


def read_ticks(pid: int) -> int:
    with open(f"/proc/{pid}/stat", "r") as f:
        fields = f.read().split()
    # fields[13]=utime, fields[14]=stime (1-based after comm in parens)
    return int(fields[13]) + int(fields[14])


def read_rss_kb(pid: int) -> int:
    try:
        with open(f"/proc/{pid}/status", "r") as f:
            for line in f:
                if line.startswith("VmRSS:"):
                    return int(line.split()[1])
    except FileNotFoundError:
        return 0
    return 0


def screen_size() -> tuple[int, int]:
    """Return the X root size; ffmpeg x11grab cannot infer it itself."""
    try:
        out = subprocess.check_output(
            ["xwininfo", "-root"], text=True, stderr=subprocess.DEVNULL
        )
        vals = {}
        for line in out.splitlines():
            s = line.strip()
            for key, name in (("w", "Width:"), ("h", "Height:")):
                if s.startswith(name):
                    vals[key] = int(s.split(":", 1)[1])
        if len(vals) == 2:
            return vals["w"], vals["h"]
    except Exception:
        pass
    return 1920, 1080


def capture_client(
    out_png: Path, x: int, y: int, w: int, h: int,
    root_size: tuple[int, int] = (1920, 1080),
) -> dict:
    if Image is None or shutil.which("ffmpeg") is None:
        return {"ok": False, "reason": "PIL/ffmpeg unavailable"}
    root_png = out_png.with_name(out_png.stem + "_root.png")
    cmd = [
        "ffmpeg", "-hide_banner", "-loglevel", "error",
        "-f", "x11grab", "-video_size", f"{root_size[0]}x{root_size[1]}",
        "-i", os.environ.get("DISPLAY", ":0"),
        "-frames:v", "1", "-y", str(root_png),
    ]
    subprocess.run(cmd, check=True)
    im = Image.open(root_png).convert("RGB")
    sw, sh = im.size
    x0 = max(0, min(sw, x))
    y0 = max(0, min(sh, y))
    x1 = max(x0, min(sw, x + w))
    y1 = max(y0, min(sh, y + h))
    crop = im.crop((x0, y0, x1, y1))
    crop.save(out_png)
    pixels = list(crop.getdata())
    counts = collections.Counter(pixels)
    top_color, top_n = counts.most_common(1)[0]
    non_top = 1.0 - top_n / max(1, len(pixels))
    black = sum(1 for p in pixels if p == (0, 0, 0)) / max(1, len(pixels))
    mean = tuple(round(sum(p[i] for p in pixels) / max(1, len(pixels)), 2) for i in range(3))
    return {
        "ok": True,
        "png": str(out_png),
        "crop": [x0, y0, x1 - x0, y1 - y0],
        "unique_colors": len(counts),
        "top_color": list(top_color),
        "top_color_fraction": round(top_n / max(1, len(pixels)), 4),
        "non_top_color_fraction": round(non_top, 4),
        "black_fraction": round(black, 4),
        "mean_rgb": list(mean),
    }


def parse_app_metrics(path: Path, measured_t0: float = 0.0) -> dict:
    fps, p95, hitch, frames = [], [], 0, 0
    godot_fps = []
    if not path.exists():
        return {"fps_median": None, "p95_median_ms": None, "hitch_per_min": None}
    for line in path.read_text(errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("{"):
            try:
                obj = json.loads(line)
            except Exception:
                continue
            if obj.get("event") in ("RULER_METRIC", "RULER_FINAL"):
                if obj.get("t_s") is not None:
                    try:
                        if float(obj["t_s"]) < measured_t0:
                            continue
                    except Exception:
                        pass
                if "fps" in obj:
                    fps.append(float(obj["fps"]))
                if "fps_avg" in obj:
                    fps.append(float(obj["fps_avg"]))
                if "p95_ms" in obj and float(obj["p95_ms"]) > 0:
                    p95.append(float(obj["p95_ms"]))
                if "hitch" in obj:
                    hitch += int(obj["hitch"])
                frames += int(obj.get("frames", 0))
        # Godot --print-fps prints "FPS: 60" (and frame time) lines; the
        # ruler copy also has a tiny autoload printing GODOT_FPS=...
        if line.startswith("FPS:") or line.startswith("Project FPS:"):
            try:
                godot_fps.append(float(line.split(":", 1)[1].split()[0]))
            except Exception:
                pass
        elif line.startswith("GODOT_FPS="):
            try:
                rest = line.split("=", 1)[1].split()
                value = float(rest[0])
                t_app = None
                for token in rest[1:]:
                    if token.startswith("t="):
                        t_app = float(token.split("=", 1)[1])
                if t_app is not None and t_app < measured_t0:
                    continue
                godot_fps.append(value)
            except Exception:
                pass
        # glxgears prints "305 frames in 5.0 seconds = 60.9 FPS".
        m = re.search(r"=\s*([0-9.]+)\s*FPS", line)
        if m:
            try:
                godot_fps.append(float(m.group(1)))
            except Exception:
                pass
    if not fps and godot_fps:
        fps = godot_fps
    return {
        "fps_median": round(statistics.median(fps), 2) if fps else None,
        "fps_min": round(min(fps), 2) if fps else None,
        "fps_max": round(max(fps), 2) if fps else None,
        "p95_median_ms": round(statistics.median(p95), 2) if p95 else None,
        "hitch": hitch,
        "frames_reported": frames,
        "samples": len(fps),
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--app", default=None)
    ap.add_argument("--scene", choices=["spin", "list", "text"], default=None)
    ap.add_argument("--arg", action="append", default=[])
    ap.add_argument("--env", action="append", default=[])
    ap.add_argument("--window-match", required=True)
    ap.add_argument("--duration", type=float, default=60.0)
    ap.add_argument("--warmup", type=float, default=5.0)
    ap.add_argument("--out", required=True)
    ap.add_argument("--extra-seconds", type=float, default=15.0)
    ap.add_argument("--no-screenshot", action="store_true")
    args = ap.parse_args()

    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    app = Path(args.app) if args.app else Path(__file__).resolve().parents[1] / "build/linux/x64/release/bundle/ruler_flutter"
    if not app.exists():
        print(f"app not found: {app}", file=sys.stderr)
        return 2

    env = os.environ.copy()
    if args.scene:
        env["RULER_SCENE"] = args.scene
        env["RULER_SECONDS"] = str(int(args.duration + args.warmup + args.extra_seconds))
    for item in args.env:
        k, v = item.split("=", 1)
        env[k] = v

    stdout_path = out / "app.stdout"
    stderr_path = out / "app.stderr"
    stdout_f = open(stdout_path, "w")
    stderr_f = open(stderr_path, "w")
    cmd = [str(app)] + args.arg
    t_launch = time.monotonic()
    proc = subprocess.Popen(cmd, env=env, stdout=stdout_f, stderr=stderr_f)
    result = {
        "scene": args.scene or Path(args.app).name,
        "cmd": cmd,
        "pid": proc.pid,
        "duration_s": args.duration,
        "warmup_s": args.warmup,
    }
    d = None
    win = None
    try:
        found = find_window(args.window_match, timeout_s=20.0)
        if not found:
            result["error"] = "window not found"
            raise RuntimeError("window not found")
        d, root, win, name, (wx, wy, ww, wh) = found
        raise_window(root, win)
        d.sync()
        result["window"] = {"name": name, "x": wx, "y": wy, "w": ww, "h": wh}
        print(f"window: {name!r} {ww}x{wh}+{wx}+{wy}", file=sys.stderr)

        time.sleep(args.warmup)
        # The WM may place/reparent the window during startup; re-read the
        # client geometry after warmup so the screenshot crop is correct.
        wx, wy, ww, wh = client_geometry(win)
        result["window"] = {"name": name, "x": wx, "y": wy, "w": ww, "h": wh}
        raise_window(root, win)
        d.sync()
        if not args.no_screenshot:
            try:
                shot = capture_client(
                    out / "shot.png", wx, wy, ww, wh, screen_size()
                )
            except Exception as e:
                shot = {"ok": False, "error": str(e)}
                print(f"screenshot failed: {e}", file=sys.stderr)
            result["screenshot"] = shot
            if not shot.get("ok"):
                result["error"] = "screenshot missing"
                raise RuntimeError("screenshot missing")
            if float(shot.get("black_fraction", 1.0)) >= 0.999:
                result["error"] = "screenshot is all black"
                raise RuntimeError("screenshot is all black")

        # Start the measurement after warmup. The screenshot above is taken
        # before the first /proc sample; its ffmpeg call is not part of the
        # CPU window.
        clk = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
        t0 = time.monotonic()
        tick0 = read_ticks(proc.pid)
        rss_series = []
        sample_dt = 0.2
        while True:
            now = time.monotonic()
            if now - t0 >= args.duration:
                break
            rss = read_rss_kb(proc.pid)
            if rss:
                rss_series.append((now - t0, rss))
            time.sleep(sample_dt)
        t1 = time.monotonic()
        tick1 = read_ticks(proc.pid)
        rss_end = read_rss_kb(proc.pid)
        measured = t1 - t0
        cpu = (tick1 - tick0) / clk / measured * 100.0 if measured > 0 else 0.0
        result["measured_s"] = round(measured, 3)
        result["cpu_pct"] = round(cpu, 2)
        if rss_series:
            result["rss_start_mb"] = round(rss_series[0][1] / 1024.0, 1)
            result["rss_end_mb"] = round(rss_end / 1024.0, 1)
            result["rss_peak_mb"] = round(max(x[1] for x in rss_series) / 1024.0, 1)
            n = len(rss_series)
            xs = [x[0] for x in rss_series]
            ys = [x[1] for x in rss_series]
            mx = sum(xs) / n
            my = sum(ys) / n
            den = sum((x - mx) ** 2 for x in xs)
            slope_kb_s = sum((x - mx) * (y - my) for x, y in zip(xs, ys)) / den if den else 0.0
            result["rss_slope_kb_per_min"] = round(slope_kb_s * 60.0, 1)
        result["app_metrics"] = parse_app_metrics(stdout_path, measured_t0=args.warmup)
        result["exit_during_run"] = proc.poll() is not None
    except Exception as e:
        result.setdefault("error", str(e))
    finally:
        try:
            stdout_f.flush()
            stderr_f.flush()
        except Exception:
            pass
        close_requested = False
        if proc.poll() is None and d is not None and win is not None:
            close_requested = request_close(d, win)
            if close_requested:
                result["close_request"] = "wm_delete"
        if proc.poll() is None and close_requested:
            try:
                # WM_DELETE_WINDOW is the normal title-bar close path: the app
                # sees EventCloseRequested and runs Quit -> Close -> win.Close.
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                pass
        if proc.poll() is None:
            # Fallback for apps that do not implement WM_DELETE_WINDOW. SIGTERM
            # is a request, not a force-kill: a GPU app must trap it and run the
            # normal Close path. Hard-killing a GPU window can crash the Xorg
            # server (nvidia_drv.so nvidiaUnlock on the 940MX test box).
            result["close_request"] = (
                "wm_delete+sigterm" if close_requested else "sigterm"
            )
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                # Last resort only. On a desktop Xorg a SIGKILL can leave the
                # GPU surface alive long enough to crash the server; require an
                # explicit opt-in reserved for isolated test displays.
                if os.environ.get("GPUI_ALLOW_GPU_HARD_KILL") == "1":
                    print("app ignored SIGTERM; hard-killing (isolated display opt-in)", file=sys.stderr)
                    result["close_request"] += "+sigkill"
                    proc.kill()
                    proc.wait()
                else:
                    result["error"] = "app did not exit after SIGTERM; refusing hard kill"
                    print(
                        "app ignored SIGTERM; refusing hard kill. "
                        "Use an isolated display and set GPUI_ALLOW_GPU_HARD_KILL=1 only there.",
                        file=sys.stderr,
                    )
        stdout_f.close()
        stderr_f.close()

    result["app_stdout"] = str(stdout_path)
    result["app_stderr"] = str(stderr_path)
    with open(out / "report.json", "w") as f:
        json.dump(result, f, indent=2, ensure_ascii=False)
        f.write("\n")
    print(json.dumps(result, indent=2, ensure_ascii=False))
    return 0 if "error" not in result else 1


if __name__ == "__main__":
    raise SystemExit(main())
