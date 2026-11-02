#!/usr/bin/env bash
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch1a
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
# 等五目标构建跑完，避免并发把墙钟敏感的门禁打歪
while ! grep -q ALLDONE "$E/progress.log" 2>/dev/null; do sleep 20; done
bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
echo "per-file rc=$?" >> "$E/progress.log"
bash tools/per-file-tests.sh "$R/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
echo "per-file-extra rc=$?" >> "$E/progress.log"
echo PERFILEDONE >> "$E/progress.log"
