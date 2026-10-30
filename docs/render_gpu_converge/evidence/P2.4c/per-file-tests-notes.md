# P2.4c 逐测试文件回归定性（零新增红）

- 范围：本步触及的 4 个包（`render`、`render/internal/gpu/res`、`render/internal/gpu`、`render/internal/image`），341 个 `*_test.go`，一次只跑一个文件。
- 结果：**PASS=301 / NO_TEST=12 / SKIP_TAGGED=1 / FAIL=27**（明细 `per-file-tests.txt`；`stencil_metal_repro_test.go` 被构建标签排除）。
- 分包子项：render 175 PASS+9 FAIL；res 6 PASS；gpu 110 PASS+18 FAIL+1 SKIP_TAGGED；image 10 PASS。
- 环境：`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so` 已设（H1 纪律），串行执行，本轮无 `Not enough memory left` 的 RequestDevice 跳过。

## 27 个 FAIL 的 A/B 定性

- 全部 27 个在**干净 HEAD `2204ad5a`**（worktree `/tmp/p24c-clean`）用同一取词跑出**同测试名、同原因、27/27 FAIL、0 PASS**：见 `per-file-tests-ab-clean.txt`。→ 零新增红，非本步回归。
- 27 个也是 P2.4b 那份 32 FAIL 的子集（P2.4b 多出的 5 个属本步范围外的包：`ui/scene` 4 + `render/text` 的 `zz_b2_bitmap_scan_test.go` 长跑；后者按 P1.3 3600s 口径在 P2.4b 已单跑 PASS）。

## 失败类别（均为既有环境/设备面）

1. **stencil/mask layout 未 pin**：`no mask layout (shared layouts must be pinned by GPUShared first)`，波及 sdf / stencil / convex 管线创建类（`sdf_render_test.go`、`stencil_pipeline_test.go`、`stencil_render_test.go`、`render_session_test.go` 部分、`convex_renderer_test.go`、`depth_clip_test.go`）。
2. **surface 模式 stencil 纹理断言**：`expected non-nil stencil textures` / `surface mode must allocate stencil`（`render_session_test.go` 4 个用例，干净树同红）。
3. **wgpu 校验 panic / 着色器阶段不匹配**：`Error matching ShaderStages(FRAGMENT) shader requirements against the pipeline` → nil pointer panic（`opt24/opt28/opt29` 等 opt 系列）。
4. 其余零散：`ttc_cjk_gpu_repro_test.go` 的 `nil atlas view`、`m5_color_glyph_api_test.go`、`rasterizer_tile_value_test.go`、`filter_present_coherence_test.go`、`bindgroup_switch_test.go`、`gpu_texture_test.go`、`text_transform_gpu_test.go`、`video_direct_upload_test.go`、`opt21/opt34/opt36/opt38/opt40` 等，全部在干净树逐项同红。

结论：本步 27 红全部既有，`-race` 对改动路径（res 状态机 + 会话退休口 + ImageCache 钩子）通过，见 `race.txt`。