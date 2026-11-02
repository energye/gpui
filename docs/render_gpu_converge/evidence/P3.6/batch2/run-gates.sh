#!/usr/bin/env bash
set -u
export LC_ALL=C
R=/home/yanghy/app/projects/gogpu/gpui
E=$R/docs/render_gpu_converge/evidence/P3.6/batch2
export LD_LIBRARY_PATH=$R/lib WGPU_NATIVE_PATH=$R/lib/libwgpu_native.so CGO_ENABLED=1
cd "$R" || exit 1
{ echo "# apidoc $(date -Iseconds) HEAD=$(git rev-parse --short HEAD)"; go run ./scripts/apidoc 2>&1 | tail -3; echo "rc=$?"; } > "$E/apidoc.txt" 2>&1
echo "apidoc done" >> "$E/progress.log"
{ echo "# vet 触及包 $(date -Iseconds)"
  for pkg in ./render ./render/internal/image ./render/internal/gpu ./render/recording ./ui/io ./gpu/hal ./gpu/rwgpu ./engine/tex; do echo "--- $pkg ---"; go vet "$pkg" 2>&1 | tail -4; done; } > "$E/vet.txt" 2>&1
echo "vet done" >> "$E/progress.log"
{ echo "# targeted (批 2) $(date -Iseconds)"
  go test -count=1 -timeout 900s -run "TestR1_ImageMipmap|TestR1_MipLevelBlend|TestMipChain|TestDownsample|TestGenerateMipmaps|TestMipmap|TestExportImageBuf|TestImageBuf|TestR72_Export|TestAnimated" ./render ./render/internal/image 2>&1 | tail -6
  echo "--- gpu 定向（换代际/退役逐出/合并谓词）---"
  go test -count=1 -timeout 900s -run "TestImageCache_|TestImageGeneration|TestImagePipeline_DifferentGeneration|TestImageMip|TestImageCache_Mip|TestSamplerDescriptorForImageFilter" ./render/internal/gpu 2>&1 | tail -4; } > "$E/targeted.txt" 2>&1
echo "targeted rc=$?" >> "$E/progress.log"
{ echo "# conformance $(date -Iseconds)"; go test -count=1 -run TestConformanceDualBackend -timeout 900s ./render 2>&1 | tail -4; } > "$E/conformance.txt" 2>&1
echo "conformance rc=$?" >> "$E/progress.log"
{ echo "# race -count=2 逐组判定（包级 rc 只作线索，见 race/race-verdicts.txt 与 race-attribution.md）$(date -Iseconds)"
  for pkg in ./render/internal/image ./render/recording ./ui/io ./engine/tex; do echo "--- $pkg ---"; go test -race -count=2 -timeout 1200s "$pkg" 2>&1 | tail -3; done
  # gpu/hal 的测试非 -count=2 安全（全局注册表不撤销 + 二次枚举适配器为 0，干净树 82ab30ef A/B 同红）：
  # 本批把它纳入观察，按可复现的 -count=1 跑，并在 race-attribution.md 记根因与待办。
  echo "--- ./gpu/hal (-count=1，理由见 race-attribution.md) ---"
  go test -race -count=1 -timeout 1200s ./gpu/hal 2>&1 | tail -3
  echo "--- ./render/internal/gpu (换代际逐出) ---"
  go test -race -count=2 -timeout 1200s -run "TestImageCache_|TestImageGeneration|TestImagePipeline_DifferentGeneration|TestImageMip|TestImageCache_Mip|TestSamplerDescriptorForImageFilter" ./render/internal/gpu 2>&1 | tail -3
  echo "--- ./render (P3.6 动图/导出钉；整包=既有 native abort 已登记) ---"
  go test -race -count=2 -timeout 900s -run "TestR1_ImageMipmap|TestR1_MipLevelBlend|TestAnimated|TestExportImageBuf|TestImageBuf|TestR72_Export|TestLoadAnimated" ./render 2>&1 | tail -3; } > "$E/race/race.txt" 2>&1
echo "race rc=$?" >> "$E/progress.log"
bash "$E/build-gate.sh" > "$E/build-gate.log" 2>&1
echo "build rc=$?" >> "$E/progress.log"
bash tools/per-file-tests.sh tools/per-file-tests.pkgs "$E/per-file-tests" > "$E/per-file-tests-run.log" 2>&1
echo "per-file rc=$?" >> "$E/progress.log"
bash tools/per-file-tests.sh "$R/docs/render_gpu_converge/evidence/P3.5/per-file-extra.pkgs" "$E/per-file-tests-extra" > "$E/per-file-tests-extra-run.log" 2>&1
echo "perfile-extra rc=$?" >> "$E/progress.log"
echo "B2DONE" >> "$E/progress.log"
