#!/usr/bin/env bash
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch1b
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
{ echo "# targeted $(date -Iseconds) HEAD=$(git rev-parse --short HEAD)"
  go test -count=1 -run "TestAnimated|TestDisposal|TestSingleFrame|TestUIIO|TestLoadAnimated|TestImageEncoding|TestLoadImage|TestPreserve|TestStatic" ./render ./render/internal/image 2>&1 | tail -4
} > "$E/targeted.txt" 2>&1
echo "targeted rc=$?" >> "$E/progress.log"
{ echo "# conformance $(date -Iseconds)"; go test -count=1 -run TestConformanceDualBackend -timeout 900s ./render 2>&1 | tail -4; } > "$E/conformance.txt" 2>&1
echo "conformance rc=$?" >> "$E/progress.log"
{ echo "# race -count=2 $(date -Iseconds)"
  for pkg in ./render/internal/image ./ui/io ./render/recording; do echo "--- $pkg ---"; go test -race -count=2 -timeout 1200s "$pkg" 2>&1 | tail -3; done
  echo "--- ./render (P3.6 测试；整包=批 4 已登记既有 native abort) ---"
  go test -race -count=2 -timeout 900s -run "TestAnimated|TestDisposal|TestSingleFrame|TestUIIO|TestLoadAnimated|TestImageEncoding|TestLoadImage|TestPreserve|TestStatic" ./render 2>&1 | tail -3
} > "$E/race/race.txt" 2>&1
echo "race rc=$?" >> "$E/progress.log"
bash "$E/build-gate.sh" > "$E/build-gate-run.log" 2>&1
echo "build rc=$?" >> "$E/progress.log"
bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
echo "per-file rc=$?" >> "$E/progress.log"
bash tools/per-file-tests.sh "$R/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
echo "perfile-extra rc=$?" >> "$E/progress.log"
echo B1BDONE >> "$E/progress.log"
