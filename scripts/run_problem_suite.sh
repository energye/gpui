#!/usr/bin/env bash
# Comprehensive problem-finding suite (not scoreboard green).
#
# Layers:
#   1) PKS evidence  — pixel / stage / empty / flicker gates
#   2) PKS core      — gates+traps+key stress
#   3) PKS dig      — Skia-facing quality/stability dig wall
#   4) PKS combo     — interaction stress
#   5) PKS mem       — soak / grow / resize
#   6) capability    — C01–C20 (or critical/full)
# Emits PROBLEMS.md tagged by failure mode.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="${PATH:-}:/home/yanghy/app/energy/go/bin"
if [[ -z "${GOROOT:-}" && -d /home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64 ]]; then
  export GOROOT=/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64
  export PATH="$GOROOT/bin:$PATH"
  export GOTOOLCHAIN=local
fi
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/1000/gdm/Xauthority}"
export GOCACHE="${GOCACHE:-/tmp/gpui-go-cache}"
export GOMODCACHE="${GOMODCACHE:-/home/yanghy/app/gopath/pkg/mod}"
export GOWORK=off

OUT="${GPUI_PROBLEM_OUT:-/tmp/problem_suite}"
SEC="${GPUI_ANIM_SECONDS:-6}"
CAP_MODE="${GPUI_PROBLEM_CAP:-wide}"
BISECT_STRESS="${GPUI_PKS_BISECT_STRESS:-0}"

mkdir -p "$OUT/pks_evidence" "$OUT/pks_core" "$OUT/pks_dig" "$OUT/pks_combo" "$OUT/pks_mem" "$OUT/cap" "$OUT/bisect"
PKS_BIN="${PKS_BIN:-/tmp/pks_bin}"
CAP_BIN="${CAP_BIN:-/tmp/cap_bin}"

echo "==== build ===="
go build -o "$PKS_BIN" ./examples/particle_kitchen_sink
go build -o "$CAP_BIN" ./examples/capability_matrix

run_pks() {
  local filter="$1" outdir="$2"
  echo "==== PKS filter=$filter out=$outdir ===="
  GPUI_PKS_FILTER="$filter" GPUI_PKS_OUT="$outdir" GPUI_ANIM_SECONDS="$SEC" \
    GPUI_PKS_BISECT=1 GPUI_PKS_BISECT_STRESS="$BISECT_STRESS" PKS_BIN="$PKS_BIN" \
    scripts/run_pks_matrix.sh >"${outdir}_run.log" 2>&1 || true
}

run_pks evidence "$OUT/pks_evidence"
run_pks core "$OUT/pks_core"
run_pks dig "$OUT/pks_dig"
run_pks combo "$OUT/pks_combo"
run_pks mem "$OUT/pks_mem"

echo "==== capability mode=$CAP_MODE ===="
case "$CAP_MODE" in
  critical) ids=(C01 C07 C11) ;;
  full)
    ids=(C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20
         C21 C22 C23 C24 C25 C26 C27 C28 C29 C30 C31 C32)
    ;;
  *)
    ids=(C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20)
    ;;
esac
for id in "${ids[@]}"; do
  echo "---- $id ----"
  if GPUI_SCENARIO="$id" GPUI_ANIM_SECONDS="$SEC" \
      GPUI_RESULT_FILE="$OUT/cap/${id}.json" \
      "$CAP_BIN" >"$OUT/cap/${id}.log" 2>&1; then
    echo "$id PASS"
  else
    echo "$id FAIL"
  fi
done

echo "==== synthesize PROBLEMS.md ===="
OUT="$OUT" CAP_MODE="$CAP_MODE" python3 - <<'PY2'
import json, glob, os, re
out = os.environ["OUT"]
cap_mode = os.environ.get("CAP_MODE", "wide")
problems = []

def load_dir(d, prefix):
    if not os.path.isdir(d):
        return
    for p in sorted(glob.glob(d + "/*.json")):
        if "/bisect/" in p:
            continue
        try:
            d0 = json.load(open(p))
        except Exception:
            continue
        st = d0.get("status") or ""
        if st == "PASS":
            continue
        pid = d0.get("probe_id") or d0.get("tier") or d0.get("scenario") or d0.get("id") or os.path.basename(p)
        problems.append({
            "suite": prefix,
            "id": pid,
            "class": d0.get("probe_class") or "",
            "status": st,
            "fps": float(d0.get("fps_ema") or d0.get("fps_avg") or 0),
            "cpu": float(d0.get("cpu_avg") or 0),
            "reason": d0.get("fail_reason") or d0.get("reason") or "",
            "bisect": d0.get("bisect_hint") or "",
            "sig_ratio": d0.get("sig_fail_ratio"),
            "n": d0.get("particle_n") or 0,
        })

for name in ("pks_evidence", "pks_core", "pks_dig", "pks_combo", "pks_mem", "cap"):
    load_dir(os.path.join(out, name), name)

def tag_reason(reason):
    tags = []
    r = reason or ""
    rules = [
        (r"fps_low|fps_hitch|fps_jitter", "perf_fps"),
        (r"cpu_over_budget", "perf_cpu"),
        (r"text_bi", "text_glyph"),
        (r"cpu_fallback|cpu_fb", "gpu_fallback"),
        (r"gpu_ops=0", "gpu_dead"),
        (r"pixel_fail", "pixel_raster"),
        (r"stage_sig|intermittent_content", "content_dropout"),
        (r"content_fail|content_gutted", "content_empty"),
        (r"present_errors|resize_recover", "present_resize"),
        (r"rss_|mem_", "memory"),
        (r"trap_hot", "blend_regression"),
        (r"too_few_frames", "hang_or_crash"),
    ]
    for pat, tag in rules:
        if re.search(pat, r, re.I):
            tags.append(tag)
    if not tags and r:
        tags.append("other")
    return tags

mode_hits = {}
for p in problems:
    for t in tag_reason(p["reason"]):
        mode_hits.setdefault(t, []).append("%s/%s" % (p["suite"], p["id"]))

lines = [
    "# Problem suite — failures only",
    "",
    "- total_failures: **%d**" % len(problems),
    "- capability_mode: `%s`" % cap_mode,
    "- philosophy: surface real present-path issues; never gut content to pass",
    "",
    "## Failure-mode hits",
    "",
    "| mode | count | examples |",
    "|------|-------|----------|",
]
for mode in sorted(mode_hits.keys()):
    ex = ", ".join(mode_hits[mode][:4])
    lines.append("| `%s` | %d | %s |" % (mode, len(mode_hits[mode]), ex))
if not mode_hits:
    lines.append("| (none) | 0 | all green — recheck density floors |")

lines += [
    "",
    "| suite | id | class | fps | cpu | reason | modes | bisect |",
    "|-------|----|-------|-----|-----|--------|-------|--------|",
]
for p in problems:
    modes = ",".join(tag_reason(p["reason"]))
    lines.append("| %s | %s | %s | %.1f | %.0f | %s | %s | %s |" % (
        p["suite"], p["id"], p["class"], p["fps"], p["cpu"],
        (p["reason"] or "").replace("|", "/"), modes, (p["bisect"] or "").replace("|", "/")))

lines += ["", "## Evidence wall", ""]
for dname in ("pks_evidence", "pks_core"):
    for key in ("P_PIXEL", "P_STAGE_SIG", "P_EMPTY_TRAP", "P_FLICKER"):
        pth = os.path.join(out, dname, key + ".json")
        if not os.path.exists(pth):
            continue
        d = json.load(open(pth))
        lines.append("- `%s` status=%s pixel=%s stage=%s sig_fail_ratio=%s" % (
            d.get("probe_id"), d.get("status"), d.get("pixel_ok"), d.get("stage_sig_ok"), d.get("sig_fail_ratio")))

cap_pass = cap_fail = 0
cap_fail_rows = []
for pth in sorted(glob.glob(os.path.join(out, "cap", "*.json"))):
    d = json.load(open(pth))
    st = d.get("status")
    if st == "PASS":
        cap_pass += 1
    else:
        cap_fail += 1
        cap_fail_rows.append((d.get("scenario") or os.path.basename(pth), st,
                              float(d.get("fps_ema") or 0), d.get("cpu_fallback_ops"),
                              d.get("fail_reason") or ""))
lines += ["", "## Capability (%s) pass=%d fail=%d" % (cap_mode, cap_pass, cap_fail), ""]
if cap_fail_rows:
    for sid, st, fps, fb, reason in cap_fail_rows:
        lines.append("- **%s FAIL** fps=%.1f fb=%s reason=%s" % (sid, fps, fb, reason))
elif cap_pass:
    lines.append("- all %d scenarios PASS" % cap_pass)

lines += [
    "",
    "## How to dig",
    "",
    "1. `GPUI_PROBE=ID GPUI_ANIM_SECONDS=8 /tmp/pks_bin`",
    "2. `GPUI_SCENARIO=C07 /tmp/cap_bin`",
    "3. One-switch: `GPUI_ENABLE_*=0` / `GPUI_BLEND_CIRCLES`",
    "4. `P_HIGH_N` PASS + combo FAIL → advanced path, never gut N",
    "5. See examples/particle_kitchen_sink/COVERAGE.md",
    "",
    "Artifacts: `%s/`" % out,
]
open(os.path.join(out, "PROBLEMS.md"), "w").write("\n".join(lines) + "\n")
json.dump({"failures": problems, "modes": mode_hits}, open(os.path.join(out, "problems.json"), "w"), indent=2)
print("\n".join(lines))
print("failures=%d" % len(problems))
PY2

echo "==== DONE out=$OUT ===="
cat "$OUT/PROBLEMS.md" | head -80
python3 -c "import json,sys; d=json.load(open('$OUT/problems.json')); n=len(d.get('failures',[])); print('fail_count',n); sys.exit(0 if n==0 else 1)" || true
