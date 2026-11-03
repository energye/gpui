#!/usr/bin/env bash
# P3.6 批 3 门禁（三扇真窗 + D20 转发口 + imageCache 诊断原子化）。口径同批 1d。
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch3
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
mkdir -p "$E/race" "$E/windows"

log() { echo "$(date -Iseconds) $1" >> "$E/progress.log"; }

{ echo "# apidoc $(date -Iseconds) HEAD=$(git rev-parse --short HEAD)"; go run ./scripts/apidoc; echo "rc=$?"; } > "$E/apidoc.txt" 2>&1
log "apidoc done"

{ echo "# vet 触及包 $(date -Iseconds)"
  for pkg in ./render ./ui/rendering ./ui/embedder ./ui/scheduler ./ui/kit; do echo "--- $pkg ---"; go vet "$pkg" 2>&1 | tail -4; done; } > "$E/vet.txt" 2>&1
log "vet done"

{ echo "# targeted $(date -Iseconds)"
  echo "--- render（D20 转发口 + 图像缓存）---"
  go test -count=1 -timeout 600s -run "TestImageCacheSnapshotForwarding|TestImageCache_|TestRenderImage_Animated" ./render 2>&1 | tail -3
  echo "--- ui/rendering + ui/kit + ui/scheduler + ui/textinput（契约钉）---"
  go test -count=1 -timeout 300s -run "TestOwner_TickerRegistry|TestOwner_NextTickerWake|TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame|TestKitAnimatedImage|TestBlinkFocusRegistration" ./ui/rendering ./ui/kit ./ui/scheduler ./ui/textinput 2>&1 | tail -6; } > "$E/targeted.txt" 2>&1
log "targeted done"

{ echo "# conformance $(date -Iseconds)"; go test -count=1 -run TestConformanceDualBackend -timeout 900s ./render 2>&1 | tail -4; } > "$E/conformance.txt" 2>&1
log "conformance done"

{ echo "# race $(date -Iseconds)"
  echo "--- render/internal/gpu（imageCache 诊断原子化：缓存全测）---"
  go test -race -count=2 -timeout 1200s ./render/internal/gpu 2>&1 | tail -3
  echo "--- render（D20 转发口 + 动图/解码定向）---"
  go test -race -count=2 -timeout 900s -run "TestImageCacheSnapshotForwarding|TestAnimated|TestDecode|TestLoadAnimated" ./render 2>&1 | tail -3
  for pkg in ./ui/scheduler ./ui/kit; do echo "--- $pkg ---"; go test -race -count=2 -timeout 1200s "$pkg" 2>&1 | tail -3; done
  echo "--- ./ui/rendering（定向；全包性能四测 -race 红已归因 gate-attribution §1/§4）---"
  go test -race -count=2 -timeout 1200s -run "TestOwner_TickerRegistry|TestRenderImage_Animated|TestRenderImage_InPlace" ./ui/rendering 2>&1 | tail -3
  echo "--- ./ui/textinput ./ui/embedder（全包；EditorBufferMirror 全包 -race 债已归因 §4）---"
  go test -race -count=2 -timeout 1200s ./ui/embedder 2>&1 | tail -3; } > "$E/race/race.txt" 2>&1
log "race done"

bash "$E/build-gate.sh" > "$E/build-gate.log" 2>&1
log "build done"

bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
log "per-file main done"

bash tools/per-file-tests.sh "$R/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
log "per-file extra done"

bash "$E/compare-baseline.sh" > "$E/baseline-compare.txt" 2>&1
log "baseline compare done"

{ echo "# three windows x two backends $(date -Iseconds)"
  for be in native go; do
    for w in ui_wr_p36_imgfmt ui_wr_p36_animplay ui_wr_p36_mipmap; do
      echo "--- $be $w ---"
      GPUI_BACKEND=$be timeout 200 go run ./examples/$w -auto-only 2>&1 | grep -E "OK presents|FAIL" | tail -1
    done
  done; } > "$E/windows/three-windows.txt" 2>&1
log "windows done"

echo "B3DONE" >> "$E/progress.log"
