#!/usr/bin/env bash
# Isolation probe matrix for particle_kitchen_sink.
# Surfaces real present-path problems (fps / cpu_fb / content / mem / resize / cpu budget).
#
# Usage:
#   scripts/run_pks_matrix.sh
#   GPUI_PKS_FILTER=gate scripts/run_pks_matrix.sh
#   GPUI_PKS_FILTER=core scripts/run_pks_matrix.sh   # daily wall + key stressors
#   GPUI_PKS_FILTER=combo scripts/run_pks_matrix.sh
#   GPUI_PKS_FILTER=dig scripts/run_pks_matrix.sh     # Skia-facing dig wall
#   GPUI_PKS_BISECT=1 scripts/run_pks_matrix.sh
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

SECONDS_EACH="${GPUI_ANIM_SECONDS:-6}"
OUTDIR="${GPUI_PKS_OUT:-/tmp/pks_matrix}"
BIN="${PKS_BIN:-/tmp/pks_bin}"
FILTER="${GPUI_PKS_FILTER:-all}"
DO_BISECT="${GPUI_PKS_BISECT:-1}"
export GPUI_PKS_FILTER="$FILTER"
export GPUI_ANIM_SECONDS="$SECONDS_EACH"
mkdir -p "$OUTDIR"

echo "==== build particle_kitchen_sink ===="
go build -o "$BIN" ./examples/particle_kitchen_sink

GPUI_LIST_PROBES=1 "$BIN" >"$OUTDIR/CATALOG.md" || true

mapfile -t PROBES < <(OUTDIR="$OUTDIR" FILTER="$FILTER" python3 - <<'PY'
import os, re
text = open(os.environ["OUTDIR"] + "/CATALOG.md").read()
filt = os.environ.get("FILTER", "all").lower()
rows = []
for line in text.splitlines():
    m = re.match(r"^\| (P_[A-Z0-9_]+) \| (\w+) \|", line)
    if m:
        rows.append((m.group(1), m.group(2)))
combo = {
    "P_BLEND_GLOW", "P_LAYER_BLEND", "P_MULTI_LAYER", "P_ALPHA_MESH", "P_ATLAS_TEXT",
    "P_FULL_STAGE", "P_BLEND_CPU", "P_L2", "P_L3", "P_L4", "P_SUBMIT_PATH", "P_HIGH_N", "P_GROW_N",
    "P_FLICKER_BLEND", "P_COMBO_UI", "P_FILTER_FLICKER", "P_PATH_XFORM", "P_CPU_MESH",
}
dig = {
    "P_CLIP_NEST", "P_GRAD_RT", "P_FILTER_TILE", "P_BLEND_SEP", "P_PATH_XFORM",
    "P_EVENODD", "P_DASH", "P_MESH_WAVE", "P_TEXT_BI", "P_IMAGE_PX",
    "P_FPS_JIT", "P_FILTER_FLICKER", "P_COMBO_UI", "P_CPU_MESH", "P_XFORM_STACK",
    "P_PIXEL", "P_STAGE_SIG", "P_EMPTY_TRAP", "P_FLICKER",
}
pure_axes_exclude = combo | dig | {"P_RESIZE", "P_MEM_SOAK", "P_L0", "P_L1", "P_L2", "P_L3", "P_L4", "P_FPS_JIT", "P_XFORM_STACK"}
out = []
for pid, cls in rows:
    if filt in ("", "all"):
        out.append(pid)
    elif filt == "gate" and cls == "gate":
        out.append(pid)
    elif filt == "stress" and cls == "stress":
        out.append(pid)
    elif filt == "trap" and cls == "trap":
        out.append(pid)
    elif filt in ("axes", "axis") and pid not in pure_axes_exclude and not pid.startswith("P_L"):
        out.append(pid)
    elif filt in ("combo", "combos") and pid in combo:
        out.append(pid)
    elif filt in ("dig", "quality") and pid in dig:
        out.append(pid)
    elif filt == "core" and (cls in ("gate", "trap") or pid in ("P_BLEND_GLOW", "P_RESIZE", "P_SUBMIT_PATH", "P_BLEND_CPU", "P_MULTI_LAYER")):
        out.append(pid)
    elif filt == "evidence" and pid in ("P_PIXEL", "P_STAGE_SIG", "P_EMPTY_TRAP", "P_FLICKER", "P_DARK_STAGE", "P_CLEAR_ALT"):
        out.append(pid)
    elif filt == "mem" and pid in ("P_MEM_SOAK", "P_MEM_LONG", "P_GROW_N", "P_RESIZE", "P_MULTI_LAYER"):
        out.append(pid)
print("\n".join(out))
PY
)

if [[ ${#PROBES[@]} -eq 0 ]]; then
  echo "no probes resolved (filter=$FILTER)" >&2
  exit 2
fi

echo "==== matrix filter=$FILTER n=${#PROBES[@]} out=$OUTDIR seconds=$SECONDS_EACH ===="
pass=0; fail=0; stress_fail=0
declare -a FAILED=()

run_one() {
  local id="$1"
  local tag="${2:-$id}"
  echo "---- $tag ----"
  if env GPUI_PROBE="$id" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
      GPUI_RESULT_FILE="$OUTDIR/${tag}.json" \
      "$BIN" >"$OUTDIR/${tag}.log" 2>&1; then
    return 0
  else
    return 1
  fi
}

for id in "${PROBES[@]}"; do
  if run_one "$id"; then
    pass=$((pass+1))
  else
    fail=$((fail+1))
    FAILED+=("$id")
    cls=$(python3 -c "import json;d=json.load(open('$OUTDIR/${id}.json'));print(d.get('probe_class',''))" 2>/dev/null || echo "")
    if [[ "$cls" == "stress" ]]; then
      stress_fail=$((stress_fail+1))
    fi
    echo "FAIL $id"
  fi
  if [[ -f "$OUTDIR/${id}.json" ]]; then
    python3 -c "import json;d=json.load(open('$OUTDIR/${id}.json'));print(d.get('probe_id',d['tier']),d['status'],'class',d.get('probe_class'),'fps',round(d['fps_avg'],1),'ema',round(d['fps_ema'],1),'cpu',round(d['cpu_avg'],1),'fb',d['cpu_fallback_ops'],'n',d['particle_n'],'se',d.get('present_errors_steady',0),'re',d.get('present_errors_resize',0),'reason',d.get('fail_reason',''),'warn',';'.join(d.get('warnings') or []))"
  fi
done

if [[ "$DO_BISECT" == "1" && ${#FAILED[@]} -gt 0 ]]; then
  echo "==== auto bisect on failures ===="
  mkdir -p "$OUTDIR/bisect"
  : > "$OUTDIR/bisect/README.md"
  for id in "${FAILED[@]}"; do
    jf="$OUTDIR/${id}.json"
    [[ -f "$jf" ]] || continue
    cls=$(python3 -c "import json;d=json.load(open('$jf'));print(d.get('probe_class',''))")
    if [[ "$cls" == "stress" && "${GPUI_PKS_BISECT_STRESS:-1}" != "1" ]]; then
      echo "skip bisect stress $id (set GPUI_PKS_BISECT_STRESS=1)"
      continue
    fi
    feats=$(python3 -c "import json;d=json.load(open('$jf'));print(d.get('features',''))")
    echo "## bisect $id feats=$feats" | tee -a "$OUTDIR/bisect/README.md"
    for sw in BLEND GLOW MESH ATLAS TEXT LAYER TRAILS; do
      low=$(echo "$sw" | tr 'A-Z' 'a-z')
      if echo "$feats" | grep -Eq "$low|blend_per_circle|path_submit"; then
        tag="${id}_no${low}"
        env GPUI_PROBE="$id" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
          GPUI_ENABLE_${sw}=0 GPUI_RESULT_FILE="$OUTDIR/bisect/${tag}.json" \
          "$BIN" >"$OUTDIR/bisect/${tag}.log" 2>&1 || true
        if [[ -f "$OUTDIR/bisect/${tag}.json" ]]; then
          python3 -c "import json;d=json.load(open('$OUTDIR/bisect/${tag}.json'));print('  ',d['status'],'fps',round(d['fps_ema'],1),'reason',d.get('fail_reason',''),'feats',d.get('features'))" | tee -a "$OUTDIR/bisect/README.md"
        fi
      fi
    done
    if echo "$feats" | grep -qi blend; then
      for bc in 16 32 64; do
        tag="${id}_bc${bc}"
        env GPUI_PROBE="$id" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
          GPUI_BLEND_CIRCLES=$bc GPUI_RESULT_FILE="$OUTDIR/bisect/${tag}.json" \
          "$BIN" >"$OUTDIR/bisect/${tag}.log" 2>&1 || true
        if [[ -f "$OUTDIR/bisect/${tag}.json" ]]; then
          python3 -c "import json;d=json.load(open('$OUTDIR/bisect/${tag}.json'));print('  bc=$bc',d['status'],'fps',round(d['fps_ema'],1),d.get('fail_reason',''))" | tee -a "$OUTDIR/bisect/README.md"
        fi
      done
    fi
  done
fi

OUTDIR="$OUTDIR" python3 - <<'PY'
import json, os, glob
outdir = os.environ["OUTDIR"]
rows = []
for p in sorted(glob.glob(outdir + "/*.json")):
    if "/bisect/" in p:
        continue
    try:
        d = json.load(open(p))
    except Exception:
        continue
    rows.append(d)
rows.sort(key=lambda d: d.get("probe_id") or d.get("tier") or "")
lines = []
lines.append("# particle_kitchen_sink isolation matrix")
lines.append("")
lines.append(f"- filter: `{os.environ.get('GPUI_PKS_FILTER', 'all')}`")
lines.append(f"- seconds: `{os.environ.get('GPUI_ANIM_SECONDS', '6')}`")
lines.append(f"- pass={sum(1 for r in rows if r.get('status')=='PASS')} fail={sum(1 for r in rows if r.get('status')!='PASS')}")
lines.append("")
lines.append("| probe | class | status | fps_ema | fps_avg | cpu% | cpu_fb | n | steady_err | resize_err | content | reason | warn |")
lines.append("|-------|-------|--------|---------|---------|------|--------|---|------------|------------|---------|--------|------|")
for d in rows:
    pid = d.get("probe_id") or d.get("tier")
    warns = ";".join(d.get("warnings") or [])
    lines.append(
        "| {pid} | {cls} | {st} | {ema:.1f} | {avg:.1f} | {cpu:.1f} | {fb} | {n} | {se} | {re} | {cok} | {reason} | {warn} |".format(
            pid=pid,
            cls=d.get("probe_class", ""),
            st=d.get("status", ""),
            ema=float(d.get("fps_ema") or 0),
            avg=float(d.get("fps_avg") or 0),
            cpu=float(d.get("cpu_avg") or 0),
            fb=d.get("cpu_fallback_ops", 0),
            n=d.get("particle_n", 0),
            se=d.get("present_errors_steady", 0),
            re=d.get("present_errors_resize", 0),
            cok=d.get("content_ok", ""),
            reason=(d.get("fail_reason") or "").replace("|", "/"),
            warn=warns.replace("|", "/"),
        )
    )
lines.append("")
lines.append("## How to read FAIL")
lines.append("")
lines.append("| reason prefix | meaning | next |")
lines.append("|---------------|---------|------|")
lines.append("| `fps_low_*` | present path too slow | bisect ENABLE_* / engine path |")
lines.append("| `trap_hot_path_still_slow` | per-circle blend still ~1fps | dual_tex / advanced blend |")
lines.append("| `cpu_fallback_ops` | silent/explicit CPU fallback | GPU_FIRST |")
lines.append("| `gpu_ops=0` | nothing hit GPU | device/provider |")
lines.append("| `content_fail` | empty or gutted content | do NOT lower N |")
lines.append("| `present_errors_steady` | present broken outside resize grace | swapchain |")
lines.append("| `present_errors_resize` | too many resize-side glitches | Resize recover |")
lines.append("| `resize_recover_fails` | never recovered after resize | BeginFrame recover |")
lines.append("| `cpu_over_budget` | process CPU exceeds probe budget | main-thread / batching |")
lines.append("| `rss_*` / `mem_*` | memory climb | layer RT pool / leak |")
lines.append("")
lines.append("## Policy")
lines.append("")
lines.append("- **gate** FAIL = regression wall (must fix engine, not gut content)")
lines.append("- **trap** FAIL = known-bad path still hot")
lines.append("- **stress** FAIL = diagnostic signal under heavy load")
lines.append("- Never pass by reducing particle density below probe MinN")
open(outdir + "/SUMMARY.md", "w").write("\n".join(lines) + "\n")
print("\n".join(lines))
PY

echo "==== SUMMARY pass=$pass fail=$fail stress_fail=$stress_fail out=$OUTDIR ===="
gate_trap_fail=$(OUTDIR="$OUTDIR" python3 - <<'PY'
import json, glob, os
n = 0
for p in glob.glob(os.environ["OUTDIR"] + "/*.json"):
    if "/bisect/" in p:
        continue
    d = json.load(open(p))
    if d.get("status") == "PASS":
        continue
    if d.get("probe_class") in ("gate", "trap", ""):
        n += 1
print(n)
PY
)
if [[ "${GPUI_PKS_STRICT:-0}" == "1" ]]; then
  [[ "$fail" -eq 0 ]]
else
  [[ "$gate_trap_fail" -eq 0 ]]
fi
