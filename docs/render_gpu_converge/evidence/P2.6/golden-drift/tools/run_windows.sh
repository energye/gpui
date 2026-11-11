#!/usr/bin/env bash
# 22-window golden drift collection: serial, one window per round, isolated
# rootless Xwayland :3, standard self-quit (RUN_SECONDS -> Quit -> Close).
# Usage: tools/run_windows.sh <repo-root> <out-root> [window ...]
set -u

REPO=${1:?repo root}
OUT=${2:?out root}
shift 2 || true
WINDOWS=("$@")
if [ ${#WINDOWS[@]} -eq 0 ]; then
  WINDOWS=(
    ui_render_base_backdrop ui_render_base_cliprrect ui_render_base_dither
    ui_render_base_filterops ui_render_base_maskops ui_render_base_miscdraw
    ui_render_base_pathbool ui_render_base_svg ui_render_base_textlow
    ui_render_base_textspacing ui_render_base_textstroke
    ui_render_base_verticesmesh ui_render_base_webp ui_wr_shadermask
    ui_wr_p38_color_font ui_wr_p38_picture ui_wr_p38_svg_doc
    ui_wr_p38_svg_filter ui_wr_p38_svg_paint ui_wr_p38_svg_showcase
    ui_wr_p38_svg_structure ui_wr_p38_svg_text
  )
fi

cd "$REPO" || exit 2
mkdir -p "$OUT/runs"
: >"$OUT/run-index.tsv"
printf 'window\tbackend\trc\tgolden_pct\ttotal_px\tlogic_pass\tjson_backend\tscreen_ok\tscreen_skipped\tsnapshot\tbase\n' >>"$OUT/run-index.tsv"

export LD_LIBRARY_PATH="$REPO/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export WGPU_NATIVE_PATH="$REPO/lib/libwgpu_native.so"
export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR:-/run/user/$(id -u)}

health() {
  DISPLAY=:3 xdpyinfo >/dev/null 2>&1
}

round=0
for w in "${WINDOWS[@]}"; do
  for backend in native go; do
    round=$((round + 1))
    dir="$OUT/runs/$w/$backend"
    mkdir -p "$dir"
    if ! health; then
      echo "HEALTH_FAIL before $w/$backend at round $round" | tee -a "$OUT/batch-health.log"
      # restart isolated Xwayland once, serial; never touch the desktop server
      (Xwayland :3 -rootless >"$OUT/xwayland3-restart-$round.log" 2>&1 &)
      sleep 2
      if ! health; then
        echo "HEALTH_FAIL_UNRECOVERED at $w/$backend; abort batch" | tee -a "$OUT/batch-health.log"
        exit 3
      fi
      echo "HEALTH_RESTARTED before $w/$backend" >>"$OUT/batch-health.log"
    fi

    envs=(-u WAYLAND_DISPLAY "DISPLAY=:3" "RUN_SECONDS=5")
    if [ "$backend" = go ]; then
      envs+=("GPUI_BACKEND=go")
    fi
    start=$(date +%s)
    env "${envs[@]}" timeout 180 "/tmp/opencode/p26-golden/bin/$w" -auto-only \
      >"$dir/stdout.json" 2>"$dir/stderr.log"
    rc=$?
    end=$(date +%s)
    echo "$rc" >"$dir/rc"
    echo "$((end - start))" >"$dir/elapsed_sec"

    snap=$(sed -n 's/^snapshot: //p' "$dir/stderr.log" | tail -1)
    if [ -n "$snap" ] && [ -f "$snap" ]; then
      cp -f "$snap" "$dir/current.png"
      base="${snap%.png}_base.png"
      if [ -f "$base" ]; then
        cp -f "$base" "$dir/base.png"
      fi
    fi

    if ! health; then
      echo "HEALTH_FAIL_AFTER $w/$backend round $round (rc=$rc)" | tee -a "$OUT/batch-health.log"
      exit 4
    fi

    python3 - "$dir" "$w" "$backend" "$rc" "$OUT/run-index.tsv" <<'PY'
import json, os, sys
d, w, backend, rc, index = sys.argv[1:6]
golden = total = logic = jbackend = screen_ok = screen_skip = ""
try:
    with open(os.path.join(d, "stdout.json")) as f:
        j = json.load(f)
    e = j.get("ability_extra") or {}
    golden = e.get("pixel_golden_diff_pct", "")
    total = e.get("pixel_golden_total_px", "")
    logic = e.get("logic_pass", "")
    jbackend = j.get("backend", "")
    screen_ok = e.get("screen_check_ok", "")
    screen_skip = e.get("screen_check_skipped", "")
except Exception as exc:
    print("JSON_PARSE_FAIL", w, backend, exc, file=sys.stderr)
snap = os.path.join(d, "current.png") if os.path.exists(os.path.join(d, "current.png")) else ""
base = os.path.join(d, "base.png") if os.path.exists(os.path.join(d, "base.png")) else ""
with open(index, "a") as f:
    f.write(f"{w}\t{backend}\t{rc}\t{golden}\t{total}\t{logic}\t{jbackend}\t{screen_ok}\t{screen_skip}\t{snap}\t{base}\n")
PY
    echo "round=$round $w/$backend rc=$rc golden=$(sed -n 's/^golden .*diff=\([0-9.]*\)%.*/\1/p' "$dir/stderr.log" | tail -1)%"
  done
done
echo "BATCH_DONE rounds=$round"
