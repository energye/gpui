#!/usr/bin/env bash
# P3.5 真窗收口门禁长跑：-race → 主逐文件 → 补充分包 → 五目标构建。
set -u
ROOT=/home/yanghy/app/projects/gogpu/gpui
E=$ROOT/docs/render_gpu_converge/evidence/P3.5/windows-close
export LD_LIBRARY_PATH=$ROOT/lib
export WGPU_NATIVE_PATH=$ROOT/lib/libwgpu_native.so
export CGO_ENABLED=1
cd "$ROOT" || exit 1

{ echo "# -race -count=2 真窗收口 $(date -Iseconds)"
  for pkg in ./render ./render/internal/gpu ./render/recording; do
    echo "--- $pkg ---"
    go test -race -count=2 -timeout 1800s "$pkg" 2>&1 | tail -4
  done
} > "$E/race/race.txt" 2>&1
echo "race done rc=$?" >> "$E/progress.log"

bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
echo "per-file main done rc=$?" >> "$E/progress.log"

bash tools/per-file-tests.sh "$ROOT/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
echo "per-file extra done rc=$?" >> "$E/progress.log"

bash "$E/build-gate-windows.sh" > "$E/build-gate-run.log" 2>&1
echo "build gate done rc=$?" >> "$E/progress.log"
echo "ALL DONE" >> "$E/progress.log"
