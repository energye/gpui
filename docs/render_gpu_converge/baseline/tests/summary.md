# P1.3 逐测试文件运行基线报告

> 任务真源：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2.1 / P1.3。
> 证据目录：`docs/render_gpu_converge/baseline/tests/`。
> 状态：**P1.3 采集完成，不改代码；FAIL/SKIP 已逐条登记，修复留给 P2 对应条目。**

## 1. 结论（先看这）

- 磁盘 1398 个 `*_test.go` 全部入账：**1264 PASS / 48 FAIL / 86 SKIP = 1398 行**，无重复、无多余、无漏行。
- 每个文件独立进程执行该文件内的 Test*/Example*；不是一次全量只记总数。
- 失败按根因分三类：**真崩溃/并发缺陷**（T1、app、integration、opt24/28/29）、**真实渲染/行为回归**（render 像素、present、video 逐文件、ui/scene damage 等）、**测试自身 setup/规则问题**（共享布局未 pin、nil ImageCache、无 GPU 前置条件、depcheck 测试缝；另有 video 整包才红的顺序耦合）。
- 首轮 180 秒上限误杀 `render/text/hint/zz_b1_scan_test.go`（实测 270.7 秒 PASS）；长跑文件改 3600 秒单独执行。`render/text/zz_b1_ttf_scan_test.go` 实测 **704.2 秒 PASS**（CJK 3000×65 两轮 + Latin/Myanmar/Indic 全过），`render/text/zz_b2_bitmap_scan_test.go` 实测 **1256.2 秒 PASS**（CJK/Korean 11172 字×15 档 + Thai/TTF），确认 3600 秒长跑口径必要。
- 46 个 `RequestDevice unavailable` 首轮与干净重跑均复现（不是瞬跳）；`buffer_test.go` / `texture_test.go` / `s6_7_resources_test.go` 文件级状态本来就是 PASS（分别 8/32、5/12、5/2 通过/跳过），46 个用例仍 SKIP；干净重跑日志在 `logs/clean-rerun/`，归 P2.5/P2.4a 设备创建环境。
- `ui/scene` 与 `video` 做了整包交叉复核：`ui/scene` 整包仍红同样 5 个用例（排除逐文件隔离假红）；`video` 整包也红逐文件的 3 个播放器用例与 `gate_perf_convert` 性能门禁，另红逐文件通过的 `TestSeekByAndRate`，说明 3 条播放器失败不是假红、性能门禁是真实差距、且 video 测试还存在额外的顺序耦合。

## 2. 覆盖与执行

- 盘点：1398 个测试文件；默认构建 1375 个（139 包、13490 个 Test/Example）；9 个构建标签排除；14 个不在默认包；47 个无 Test/Example。
- 执行：每包预编译一个测试二进制；每个文件独立进程；`-test.run` 精确选该文件内的 Test*/Example*；`-test.count=1 -test.paniconexit0`。
- 三阶段：safe 180 秒/文件；window 300 秒/文件（5 秒沉降 + 显示健康检查）；long 3600 秒/文件。
- 窗口测试串行，每轮检查 `xset q` 与 Xwayland；全程未硬杀进程。详细环境与复现见 [`README.md`](README.md)。
- 静态行（构建标签/不在默认包/无 Test/Example）在 `summary.tsv` 记 `SKIP` 并在 `note` 写原因；它们没有运行日志。

### 2.1 长跑与超时

| 文件 | 实测 | 结论 |
|---|---:|---|
| `render/text/hint/zz_b1_scan_test.go` | 270.7s | 180s 会误杀；长跑 PASS |
| `render/text/zz_b1_ttf_scan_test.go` | 704.2s | 3600s 下 PASS（CJK 3000×65 两轮 + Latin/Myanmar/Indic） |
| `render/text/zz_b2_bitmap_scan_test.go` | 1256.2s | 3600s 下 PASS（CJK/Korean 11172 字×15 档 + Thai/TTF） |

## 3. 计数

| 状态 | 文件数 | 说明 |
|---|---:|---|
| PASS | 1264 | 文件内选中用例全过；文件内允许带原因的 SKIP（另记 `skips.tsv`） |
| FAIL | 48 | 至少一个失败/panic/崩溃/超时 |
| SKIP | 86 | 静态跳过或全部用例按原因跳过 |
| **合计** | **1398** | 覆盖检查：unique=1398、duplicates=0、extra=0 |

## 4. FAIL 分组

### 4.1 崩溃 / 并发（最高优先级，P2 先看）

- **T1** `gpu/rwgpu/queue_workdone_test.go`：`OnSubmittedWorkDone` 返回 null future；cleanup `device.Release()` 在 native `wgpuDeviceRelease` 内 SIGSEGV（`PC=0x1`）。原始栈：`failures/queue_workdone_test.log`。归 P2.4c/P2.4e。
- **并发渲染（老 backend）** `render/internal/gpu/integration_test.go`：5 个 goroutine 同时 `Backend.RenderScene`，wgpu-core 报 `BindGroupLayout[Id(...)] does not exist`，`fatal runtime error` → SIGABRT。原始栈：`failures/integration_test.log`。这是 P2.3 要删除的老 GPU 场景 backend 的崩溃；按 P2.3 删除处理，不把并发安全当成新需求。
- **StubHost 假 X11 句柄** `ui/application/app_test.go`：测试用 `StubHost`，但 `App.Run` 仍把假句柄 `0x1` 送进 `CreateSurfaceFromXlibWindow`，native SIGSEGV。原始栈：`failures/app_test.log`。归 P2.4e。
- **nil ImageCache panic**：`opt24_sticky_index_uniform_test.go`、`opt28_sticky_image_gputex_verts_test.go`、`opt29_image_uniform_slab_test.go`；测试未注入 `ImageCache`，生产 session 由 `GPUShared` 注入。归 P2.5 测试卫生。

### 4.2 测试 setup / 规则问题（不是生产回归，但不能当绿）

- **共享布局未 pin（14 个文件）**：11 个 mask 布局文件（`bindgroup_switch`、`convex_renderer`、`depth_clip`、`opt21`、`opt34`、`opt36`、`opt38`、`render_session`、`sdf_render`、`stencil_pipeline`、`stencil_render`）测试在 `shared.ensurePipelines()` 后替换/新建管线，却没有 `SetMaskBindLayout`；生产 `render/internal/gpu/gpu_shared.go:702/707/712` 会 pin 共享 mask 布局。另有 3 个 image pipeline 文件（`gpu_texture`、`opt27`、`opt40`）的 `clipBindLayout` 为 nil，wgpu 把错误报成 `CreateBuffer: Validation Error`，实际原因是 shader `group 1, binding 0` 不在 pipeline layout 里；生产 `gpu_shared.go:701/706/711` 与 `render_session.go:2587` 会接共享 clip 布局。两类都是测试 session 未注入共享布局，归 P2.5，不是生产 CreateBuffer 缺陷。
- **无 GPU 前置条件（3 个文件）**：`ui/scene/caret_transparent_rerecord_test.go` 要 GPU 路径但测试包没注册 GPU；`textured_budget_test.go` / `textured_test.go` 假设“无 GPU”但实际 `RasterLayerCount=1`/record 成功。整包复核仍红，确认是测试前置条件写错。归 P2.5。
- **depcheck 规则** `ui/depcheck_test.go`：规则把 `ui/rendering` 两个**测试缝**文件里的 `gpu/hal`、`gpu/hal/noop`、`gpu/types`、`gpu/context` 也判红；AGENTS 明确“测试缝与创建入口除外”。归 P2.5。
- **video 额外顺序耦合**：整包复核红逐文件通过的 `TestSeekByAndRate`（`shown pts=800, want 600`），这是全局状态/顺序耦合，归 P2.5/P2.6。3 条播放器失败见 §4.3，VR7-T 性能门禁见 §4.4；整包 5 红明细见 §8。

### 4.3 真实渲染 / 行为回归

- CPU/GPU 像素：`context_clip_depth`（圆裁剪中心黑）、`context_coverage`（fill brush red=0）、`context_misc_draw_gpu`（multiply 255 vs 229）、`rasterizer_tile_value`（TileCompute max diff 2 / 菱形 ink runs）、`text_transform_gpu`（Auto vs Vector 569 px）、`video_direct_upload`（DrawVideoFrame/Bridge 无 damage）、`filter_present_coherence`（Image 仍白）、`ui/scene/shrink_damage`（shrink damage 不覆盖长出的墨迹）、`gpupixel/blit_rrect_corner`（blit 丢圆角裁剪）、`ttc_cjk_gpu_repro`（batch 0 nil atlas view）。
- 行为：`ui/focus`（无 focusable 的 Tab 应不消费）、`examples/ui_l2_shell`（关闭后 overlay=2）。
- 呈现：`gpu/gwgpu/gles/present_damage_x11`（partial present never engaged，full=12 partial=0）、`gpu/webgpu/swapchain_x11`（reconfigure counter=0）。
- 视频播放器：`backend_ffmpeg` / `fault_classify` / `gate_color` 逐文件红、整包也红（frame pix 0 / player md5 空），稳定复现，归 P2.6。
- CPU 彩色字形：`m5_color_glyph_api` 的 `DrawShapedColorGlyphs` CPU fallback 在已安装的 NotoColorEmoji 上无 ink；不是缺字体，归 P2.3。

### 4.4 性能门禁（归 P1.7，不是功能 FAIL）

- `render/s6_9_heavy_budget`：H01/H03/H05/H06/U05 五行超预算（明细在日志）。
- `ui/rendering/m1_noquadratic`：BoxesForRange 全文 10× 行数 → 17.70× 耗时，门禁 ≤15。
- `ui/rendering/m5_keystroke_g1`：NoWrap 1000× 文本 → 1.51×，门禁 ≤1.5。
- `video/gate_perf_convert`：VR7-T ffmpeg parity，逐文件 ours convert avg=2.566ms vs ffmpeg rgba 0.630ms（gap 4.07×；整包复核 3.039ms/4.82×）；日志写明“记 S1 攻坚，不降预算”。

### 4.5 环境

- `examples/video/video_a4_sink`：测试期望静音，但本机 `paplay-pulse` 可用、音频真的响；环境/测试前置条件。
- `ui/platform/x11_s4`：`not x11Ime`；本机 X11 IME 前置条件不满足。

## 5. 逐文件失败表

| 包 / 文件 | 失败数 | 原始现象（日志摘要） | 归类 | P2 归属 | 日志 |
|---|---:|---|---|---|---|
| `gpu/rwgpu/queue_workdone_test.go` | crash | OnSubmittedWorkDone returned null future; device.Release SIGSEGV PC=0x1 | `crash-native-release` | P2.4c/P2.4e | [queue_workdone_test](logs/github.com_energye_gpui_gpu_rwgpu__queue_workdone_test.log) |
| `render/internal/gpu/integration_test.go` | crash | old GPU scene Backend shared by 5 goroutines; wgpu-core BindGroupLayout does not exist; SIGABRT; P2.3 deletes old backend and test | `crash-concurrent-pipeline` | P2.3 | [integration_test](logs/github.com_energye_gpui_render_internal_gpu__integration_test.log) |
| `ui/application/app_test.go` | crash | StubHost fake X11 handle 0x1 reaches CreateSurfaceFromXlibWindow; SIGSEGV | `crash-stub-surface` | P2.4e | [app_test](logs/github.com_energye_gpui_ui_application__app_test.log) |
| `render/internal/gpu/bindgroup_switch_test.go` | 1 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [bindgroup_switch_test](logs/github.com_energye_gpui_render_internal_gpu__bindgroup_switch_test.log) |
| `render/internal/gpu/convex_renderer_test.go` | 4 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [convex_renderer_test](logs/github.com_energye_gpui_render_internal_gpu__convex_renderer_test.log) |
| `render/internal/gpu/depth_clip_test.go` | 1 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [depth_clip_test](logs/github.com_energye_gpui_render_internal_gpu__depth_clip_test.log) |
| `render/internal/gpu/gpu_texture_test.go` | 1 | image pipeline clipBindLayout nil; wgpu says CreateBuffer Validation Error but actual cause is shader group 1 binding 0 missing from pipeline layout | `test-setup-shared-layout` | P2.5 | [gpu_texture_test](logs/github.com_energye_gpui_render_internal_gpu__gpu_texture_test.log) |
| `render/internal/gpu/opt21_layer_defer_submit_test.go` | 1 | test replaces shared pipelines after ensurePipelines; no mask layout | `test-setup-shared-layout` | P2.5 | [opt21_layer_defer_submit_test](logs/github.com_energye_gpui_render_internal_gpu__opt21_layer_defer_submit_test.log) |
| `render/internal/gpu/opt27_gputex_bg_cache_test.go` | 1 | image pipeline clipBindLayout nil; wgpu says CreateBuffer Validation Error but actual cause is shader group 1 binding 0 missing from pipeline layout | `test-setup-shared-layout` | P2.5 | [opt27_gputex_bg_cache_test](logs/github.com_energye_gpui_render_internal_gpu__opt27_gputex_bg_cache_test.log) |
| `render/internal/gpu/opt34_shared_encoder_no_flush_leading_test.go` | 2 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [opt34_shared_encoder_no_flush_leading_test](logs/github.com_energye_gpui_render_internal_gpu__opt34_shared_encoder_no_flush_leading_test.log) |
| `render/internal/gpu/opt36_filter_seed_shared_encoder_test.go` | 1 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [opt36_filter_seed_shared_encoder_test](logs/github.com_energye_gpui_render_internal_gpu__opt36_filter_seed_shared_encoder_test.log) |
| `render/internal/gpu/opt38_ensure_pipelines_ready_test.go` | 2 | test replaces shared pipelines after ensurePipelines; no mask layout | `test-setup-shared-layout` | P2.5 | [opt38_ensure_pipelines_ready_test](logs/github.com_energye_gpui_render_internal_gpu__opt38_ensure_pipelines_ready_test.log) |
| `render/internal/gpu/opt40_gputex_uniform_slab_test.go` | 1 | image pipeline clipBindLayout nil; wgpu says CreateBuffer Validation Error but actual cause is shader group 1 binding 0 missing from pipeline layout | `test-setup-shared-layout` | P2.5 | [opt40_gputex_uniform_slab_test](logs/github.com_energye_gpui_render_internal_gpu__opt40_gputex_uniform_slab_test.log) |
| `render/internal/gpu/render_session_test.go` | 11 | no mask layout; stencil textures not created as a consequence | `test-setup-shared-layout` | P2.5 | [render_session_test](logs/github.com_energye_gpui_render_internal_gpu__render_session_test.log) |
| `render/internal/gpu/sdf_render_test.go` | 5 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [sdf_render_test](logs/github.com_energye_gpui_render_internal_gpu__sdf_render_test.log) |
| `render/internal/gpu/stencil_pipeline_test.go` | 4 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [stencil_pipeline_test](logs/github.com_energye_gpui_render_internal_gpu__stencil_pipeline_test.log) |
| `render/internal/gpu/stencil_render_test.go` | 7 | no mask layout (shared layouts must be pinned by GPUShared first) | `test-setup-shared-layout` | P2.5 | [stencil_render_test](logs/github.com_energye_gpui_render_internal_gpu__stencil_render_test.log) |
| `render/internal/gpu/opt24_sticky_index_uniform_test.go` | 1 | nil ImageCache panic | `test-setup-image-cache` | P2.5 | [opt24_sticky_index_uniform_test](logs/github.com_energye_gpui_render_internal_gpu__opt24_sticky_index_uniform_test.log) |
| `render/internal/gpu/opt28_sticky_image_gputex_verts_test.go` | 1 | nil ImageCache panic | `test-setup-image-cache` | P2.5 | [opt28_sticky_image_gputex_verts_test](logs/github.com_energye_gpui_render_internal_gpu__opt28_sticky_image_gputex_verts_test.log) |
| `render/internal/gpu/opt29_image_uniform_slab_test.go` | 1 | nil ImageCache panic | `test-setup-image-cache` | P2.5 | [opt29_image_uniform_slab_test](logs/github.com_energye_gpui_render_internal_gpu__opt29_image_uniform_slab_test.log) |
| `ui/scene/caret_transparent_rerecord_test.go` | 1 | caret bar has no live texture (test needs the GPU path) | `test-setup-gpu-state` | P2.5 | [caret_transparent_rerecord_test](logs/github.com_energye_gpui_ui_scene__caret_transparent_rerecord_test.log) |
| `ui/scene/textured_budget_test.go` | 1 | record without GPU should fail but succeeded | `test-setup-gpu-state` | P2.5 | [textured_budget_test](logs/github.com_energye_gpui_ui_scene__textured_budget_test.log) |
| `ui/scene/textured_test.go` | 2 | RasterLayerCount=1 want 0 (no GPU); ReplayedOps=0 want >=1 | `test-setup-gpu-state` | P2.5 | [textured_test](logs/github.com_energye_gpui_ui_scene__textured_test.log) |
| `ui/depcheck_test.go` | 1 | UI test seams import gpu/hal, gpu/hal/noop, gpu/types, gpu/context; rule does not exempt test seams | `test-rule-layering` | P2.5 | [depcheck_test](logs/github.com_energye_gpui_ui__depcheck_test.log) |
| `render/context_clip_depth_test.go` | 1 | circle clip center pixel black, want red | `real-render` | P2.3 | [context_clip_depth_test](logs/github.com_energye_gpui_render__context_clip_depth_test.log) |
| `render/context_coverage_test.go` | 1 | fill brush red 0.0, want >=0.8 | `real-render` | P2.3 | [context_coverage_test](logs/github.com_energye_gpui_render__context_coverage_test.log) |
| `render/context_misc_draw_gpu_test.go` | 1 | advanced blend multiply 255,0,0 want 229,0,0 | `real-render` | P2.3 | [context_misc_draw_gpu_test](logs/github.com_energye_gpui_render__context_misc_draw_gpu_test.log) |
| `render/m5_color_glyph_api_test.go` | 1 | NotoColorEmoji present; DrawShapedColorGlyphs CPU fallback draws no ink | `real-render` | P2.3 | [m5_color_glyph_api_test](logs/github.com_energye_gpui_render__m5_color_glyph_api_test.log) |
| `render/rasterizer_tile_value_test.go` | 1 | TileCompute max diff 2; diamond ink runs mismatch | `real-render` | P2.3 | [rasterizer_tile_value_test](logs/github.com_energye_gpui_render__rasterizer_tile_value_test.log) |
| `render/text_transform_gpu_test.go` | 1 | Auto vs Vector rotated text diff 569 px, want <=8 | `real-render` | P2.3 | [text_transform_gpu_test](logs/github.com_energye_gpui_render__text_transform_gpu_test.log) |
| `render/video_direct_upload_test.go` | 3 | DrawVideoFrame and bridge/planes fallback leave no damage | `real-render` | P2.3/P2.4e | [video_direct_upload_test](logs/github.com_energye_gpui_render__video_direct_upload_test.log) |
| `render/internal/gpu/ttc_cjk_gpu_repro_test.go` | 1 | nil atlas view for batch 0 (TTC CJK glyph mask) | `real-render` | P2.3 | [ttc_cjk_gpu_repro_test](logs/github.com_energye_gpui_render_internal_gpu__ttc_cjk_gpu_repro_test.log) |
| `ui/scene/shrink_damage_test.go` | 1 | shrink damage does not cover grown ink; cleared tail not repainted | `real-render` | P2.3 | [shrink_damage_test](logs/github.com_energye_gpui_ui_scene__shrink_damage_test.log) |
| `ui/scene/internal/gpupixel/blit_rrect_corner_test.go` | 1 | rounded-corner hole is green, want dark background; blit lost rounded clip | `real-render` | P2.3 | [blit_rrect_corner_test](logs/github.com_energye_gpui_ui_scene_internal_gpupixel__blit_rrect_corner_test.log) |
| `gpu/gwgpu/gles/present_damage_x11_test.go` | 1 | partial present never engaged: full=12 partial=0 | `real-present` | P2.4e | [present_damage_x11_test](logs/github.com_energye_gpui_gpu_gwgpu_gles__present_damage_x11_test.log) |
| `gpu/webgpu/swapchain_x11_linux_test.go` | 1 | expected reconfigure counter, got 0 (multi-frame present) | `real-present` | P2.4e | [swapchain_x11_linux_test](logs/github.com_energye_gpui_gpu_webgpu__swapchain_x11_linux_test.log) |
| `render/filter_present_coherence_test.go` | 1 | Image() still white after present (255,255,255,255) | `real-present` | P2.4e | [filter_present_coherence_test](logs/github.com_energye_gpui_render__filter_present_coherence_test.log) |
| `video/backend_ffmpeg_test.go` | 1 | per-file and whole-package both fail: frame pix 0 | `real-video-player` | P2.6 | [backend_ffmpeg_test](logs/github.com_energye_gpui_video__backend_ffmpeg_test.log) |
| `video/fault_classify_test.go` | 1 | per-file and whole-package both fail: H265 frame pix 0 | `real-video-player` | P2.6 | [fault_classify_test](logs/github.com_energye_gpui_video__fault_classify_test.log) |
| `video/gate_color_test.go` | 1 | per-file and whole-package both fail: player md5 empty vs direct md5 for 3 clips | `real-video-player` | P2.6 | [gate_color_test](logs/github.com_energye_gpui_video__gate_color_test.log) |
| `render/s6_9_heavy_budget_test.go` | 2 | H01/H03/H05/H06/U05 heavy-budget rows fail | `perf-gate` | P1.7 | [s6_9_heavy_budget_test](logs/github.com_energye_gpui_render__s6_9_heavy_budget_test.log) |
| `ui/rendering/m1_noquadratic_test.go` | 1 | BoxesForRange full text 10x lines -> 17.7x time, gate <=15 | `perf-gate` | P1.7 | [m1_noquadratic_test](logs/github.com_energye_gpui_ui_rendering__m1_noquadratic_test.log) |
| `ui/rendering/m5_keystroke_g1_test.go` | 1 | keystroke NoWrap 1000x text -> 1.51x, gate <=1.5 | `perf-gate` | P1.7 | [m5_keystroke_g1_test](logs/github.com_energye_gpui_ui_rendering__m5_keystroke_g1_test.log) |
| `video/gate_perf_convert_test.go` | 1 | VR7-T ffmpeg parity: per-file ours convert avg=2.566ms vs ffmpeg rgba 0.630ms (gap 4.07x); full-package crosscheck 3.039ms (4.82x); budget not lowered | `perf-gate` | P1.7 | [gate_perf_convert_test](logs/github.com_energye_gpui_video__gate_perf_convert_test.log) |
| `examples/ui_l2_shell/scene_test.go` | 1 | overlay after close=2 | `real-behavior` | P2.6 | [scene_test](logs/github.com_energye_gpui_examples_ui_l2_shell__scene_test.log) |
| `ui/focus/focus_test.go` | 1 | tab with no focusables should be unconsumed | `real-behavior` | P2.6 | [focus_test](logs/github.com_energye_gpui_ui_focus__focus_test.log) |
| `examples/video/video_a4_sink/a4_sink_test.go` | 1 | expected mute but paplay-pulse is available and audio is audible | `environment-audio` | P2.5/P2.6 | [a4_sink_test](logs/github.com_energye_gpui_examples_video_video_a4_sink__a4_sink_test.log) |
| `ui/platform/x11_s4_test.go` | 1 | not x11Ime | `environment-ime` | P2.5 | [x11_s4_test](logs/github.com_energye_gpui_ui_platform__x11_s4_test.log) |

## 6. SKIP 分类

最终 `skips.tsv` 共 **239** 行（按用例计；静态行按文件计）。主要类：

- **47**：no Test/Example functions (helpers/benchmarks/fuzz only)
- **46**：webgpu RequestDevice unavailable: wgpu: failed to request device: wgpu: RequestDevice: Validation Error
- **22**：golden fixture not available (open ../../tmp/min1_final.dxil: no such file or directory) — skipping mutation-based tests
- **14**：No variable font available on this system
- **12**：darwin only; not built on linux/amd64
- **5**：need full lib (write/encode), current=/home/yanghy/app/projects/gogpu/gpui/lib/ffmpeg/linux-x64/libgpui_ffmpeg.so variant=base
- **4**：TTC test requires Windows system fonts
- **3**：build tag `windows && !(js && wasm)` excludes it on linux/amd64
- **3**：no system font with composites available
- **2**：build tag `windows` excludes it on linux/amd64
- **2**：skipping: cannot read ../../../tmp/min1_final.dxil: open ../../../tmp/min1_final.dxil: no such file or directory
- **2**：skipping: cannot read ../../../tmp/dxc_trivial_vs.dxil: open ../../../tmp/dxc_trivial_vs.dxil: no such file or directory

- 46 个 `RequestDevice unavailable` 首轮与干净重跑均复现（不是瞬跳）；3 个文件文件级状态本来就是 PASS，46 个用例仍 SKIP；干净重跑日志在 `logs/clean-rerun/`，归 P2.5/P2.4a 设备创建环境。
- 其余 SKIP 均带 `t.Skipf` 原因（缺数据/缺字体/缺 GPU/平台不支持/无 Test 函数），没有无原因静默跳过。

## 7. 崩溃原始栈

| 文件 | 现象 | 原始栈 |
|---|---|---|
| `gpu/rwgpu/queue_workdone_test.go` | T1 SIGSEGV（`OnSubmittedWorkDone` null future → `device.Release` native） | [`failures/queue_workdone_test.log`](failures/queue_workdone_test.log) |
| `ui/application/app_test.go` | StubHost 假句柄 `0x1` → `CreateSurfaceFromXlibWindow` SIGSEGV | [`failures/app_test.log`](failures/app_test.log) |
| `render/internal/gpu/integration_test.go` | 并发 `RenderScene` → wgpu-core panic → SIGABRT | [`failures/integration_test.log`](failures/integration_test.log) |
| `render/internal/gpu/opt24_sticky_index_uniform_test.go` | nil ImageCache panic | [`failures/opt24_sticky_index_uniform_test.log`](failures/opt24_sticky_index_uniform_test.log) |
| `render/internal/gpu/opt28_sticky_image_gputex_verts_test.go` | nil ImageCache panic | [`failures/opt28_sticky_image_gputex_verts_test.log`](failures/opt28_sticky_image_gputex_verts_test.log) |
| `render/internal/gpu/opt29_image_uniform_slab_test.go` | nil ImageCache panic | [`failures/opt29_image_uniform_slab_test.log`](failures/opt29_image_uniform_slab_test.log) |

## 8. 整包交叉复核

### 8.1 `ui/scene`

- 整包命令：`go test -count=1 -run '^(Test|Example)' -v -timeout 900s ./ui/scene`。原始日志：[`logs/crosscheck/ui-scene.log`](logs/crosscheck/ui-scene.log)。
- 结果：**整包也红同样 5 个用例**（caret、textured_budget、textured ×2、shrink_damage），排除“逐文件隔离造成假红”。
- 结论：caret/textured 是测试无 GPU 前置条件写错；shrink_damage 是真实 damage 缺陷；两者都保留在 P1.3 失败表。

### 8.2 `video`

- 整包命令：`go test -count=1 -run '^(Test|Example)' -v -timeout 900s ./video`。原始日志：[`logs/crosscheck/video.log`](logs/crosscheck/video.log)。
- 结果：整包 5 红：**逐文件红的 `backend_ffmpeg` / `fault_classify` / `gate_color` 整包也红**（同样 frame pix 0 / player md5 空）；`gate_perf_convert` 的 VR7-T 性能门禁也红（逐文件 4.07×；整包 4.82×，P1.7）；另红逐文件通过的 `TestSeekByAndRate`（`shown pts = 800, want 600`）。
- 结论：3 条播放器失败是稳定复现的真实问题（P2.6）；`gate_perf_convert` 是性能门禁（P1.7）；`TestSeekByAndRate` 是额外的顺序/全局状态耦合（P2.5/P2.6），不能忽略。P2.6 行为验收时两种模式都重放。

### 8.3 `render` / `render/internal/gpu`

- `render` 包包含窗口/呈现测试，未整包重跑（避免在当前桌面批量开窗，违反 GPU 真窗安全条）；逐文件 FAIL 原样登记，P2.3/P2.4e 复核。
- `render/internal/gpu` 整包会命中 `integration_test.go` 的并发 abort，未整包重跑；逐文件 FAIL 与原始栈已登记。

## 9. P2 交接

| P2 条目 | 本报告对应项 |
|---|---|
| P2.3 | 老 backend 并发崩溃（随 P2.3 删除）、render 像素回归、video upload、gpupixel、ttc_cjk、CPU 彩色字形 |
| P2.4c | T1 native release |
| P2.4e | StubHost surface、GL/WebGPU present、filter present |
| P2.5 | 共享布局未 pin（14 文件）、nil ImageCache、ui/scene 无 GPU 前置条件、depcheck 测试缝、video 额外顺序耦合 |
| P2.6 | ui/focus、ui_l2_shell、3 条视频播放器失败、video 行为验收（整包 `TestSeekByAndRate`） |
| P1.7 | s6_9、m1、m5、video/gate_perf_convert（VR7-T）性能门禁 |

> 归类依据是原始日志 + 源码定位（如 `gpu_shared.go:702/707/712`、`app.go:294`、`pipeline_app.go:992/1199`）；P2 修复前仍需按 AGENTS「禁止猜着修」复现确认。

## 10. 复现

见 [`README.md`](README.md)。关键命令模式：

```bash
GOWORK=off WGPU_NATIVE_PATH="$PWD/lib/libwgpu_native.so" LD_LIBRARY_PATH="$PWD/lib" \
GPUI_SURFACE_SAMPLE_COUNT=1 GOMAXPROCS=2 \
go test -count=1 -run '^(TestA|TestB)$' -v -timeout 3600s ./pkg
```

逐文件执行器按包预编译、按文件独立进程；日志命名与阶段划分见 README。`render/s6_9_heavy_budget_test.go` 会写 `tmp/s6_9_heavy_budget.json`，`tmp/` 已 gitignore，不作为证据提交。
