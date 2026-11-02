#!/usr/bin/env bash
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch1a
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
{ echo "# conformance dual-backend $(date -Iseconds) HEAD=$(git rev-parse --short HEAD)"
  go test -count=1 -run TestConformanceDualBackend -timeout 900s ./render 2>&1 | tail -8
} > "$E/conformance.txt" 2>&1
echo "conformance rc=$?" >> "$E/progress.log"
{ echo "# race -count=2 $(date -Iseconds)"
  for pkg in ./render/internal/image ./render/recording; do echo "--- $pkg ---"; go test -race -count=2 -timeout 1200s "$pkg" 2>&1 | tail -3; done
  echo "--- ./render (image tests only, whole-package run has the registered native abort) ---"
  go test -race -count=2 -timeout 900s -run "TestImageEncoding|TestImageOrientation|TestLoadImage|TestPreserve|TestEachOrientation|TestStaticDecoders" ./render 2>&1 | tail -3
} > "$E/race/race.txt" 2>&1
echo "race rc=$?" >> "$E/progress.log"
bash "$E/build-gate.sh" > "$E/build-gate-run.log" 2>&1
echo "build rc=$?" >> "$E/progress.log"
echo ALLDONE >> "$E/progress.log"
