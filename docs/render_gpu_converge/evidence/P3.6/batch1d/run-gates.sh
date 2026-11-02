#!/usr/bin/env bash
# P3.6 批 1d 门禁（泵契约合一 + D18 回归钉）。口径同批 1c/批 2：
# apidoc / vet 触及包 / 定向 / conformance / -race 触及四包 / 五目标×{默认,nogpu} /
# 逐文件主+补充 sweep / 对批 2 基线比 FAIL 名单。
# 金图：本批零绘制路径变化，存量金图按构造性论证（真窗金图归批 1d 步骤 anim-image 真窗）。
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch1d
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
mkdir -p "$E/race"

log() { echo "$(date -Iseconds) $1" >> "$E/progress.log"; }

{ echo "# apidoc $(date -Iseconds) HEAD=$(git rev-parse --short HEAD)"; go run ./scripts/apidoc; echo "rc=$?"; } > "$E/apidoc.txt" 2>&1
log "apidoc done"

{ echo "# vet 触及包 $(date -Iseconds)"
  for pkg in ./ui/scheduler ./ui/rendering ./ui/textinput ./ui/embedder; do echo "--- $pkg ---"; go vet "$pkg" 2>&1 | tail -4; done; } > "$E/vet.txt" 2>&1
log "vet done"

{ echo "# targeted 泵契约钉 + D18 钉 $(date -Iseconds)"
  echo "--- ui/rendering（注册表契约 + NextTickerWake + D18 原地换帧重放）---"
  go test -count=1 -timeout 300s -run "TestOwner_TickerRegistry|TestOwner_NextTickerWake|TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame" ./ui/rendering 2>&1 | tail -3
  echo "--- ui/textinput（光标注册存续/大步长奇偶/相位截止）---"
  go test -count=1 -timeout 300s -run "TestBlinkFocusRegistration|TestBlinkParity_OversizedStep|TestBlinkDeadline_TracksPhase" ./ui/textinput 2>&1 | tail -3
  echo "--- ui/scheduler（ticker 契约全家，含 PollNextWake）---"
  go test -count=1 -timeout 300s ./ui/scheduler 2>&1 | tail -3; } > "$E/targeted.txt" 2>&1
log "targeted done"

{ echo "# conformance $(date -Iseconds)"; go test -count=1 -run TestConformanceDualBackend -timeout 900s ./render 2>&1 | tail -4; } > "$E/conformance.txt" 2>&1
log "conformance done"

{ echo "# race -count=2 本批触及四包 $(date -Iseconds)"
  for pkg in ./ui/scheduler ./ui/rendering ./ui/textinput ./ui/embedder; do echo "--- $pkg ---"; go test -race -count=2 -timeout 1200s "$pkg" 2>&1 | tail -3; done; } > "$E/race/race.txt" 2>&1
log "race done"

bash "$E/build-gate.sh" > "$E/build-gate.log" 2>&1
log "build done"

bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
log "per-file main done"

bash tools/per-file-tests.sh "$R/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
log "per-file extra done"

bash "$E/compare-baseline.sh" > "$E/baseline-compare.txt" 2>&1
log "baseline compare done"

echo "B1DDONE" >> "$E/progress.log"
