# 05 测试与假绿信号

> 只标信号，不判假绿。Skip 带原因不算假绿，但 CI 绿不代表画过。

## 1 数量

- render 主包：顶层约 137 个 `_test.go`（`quad_cpu_test.go`、`depth_r5_test.go`、`text_transform_test.go`、`nan_safety_test.go:13 个 NoCrash`、`context_coverage_test.go:60+`、`vec_test.go` 等；诊断窗默认 Skip：clipping/scene/images/basic_gpu_visual）。
- `render/internal/gpu`：138 个 `_test.go`（根约 126：s6_3、depth_clip、compute_pass、opt40 等；tilecompute 7；res 5）。`^func Test` 241+ 行（截断，实际更多）。
- `gpu/rwgpu`：37 个 `_test.go`，约 168 个 Test（s1_ae 7、s1_skia 枚举、null_guard 18、device_lost、math 17、abi、vram_ledger）。
- `gpu/webgpu`：18 个（顶层 14 + browser 3 + thread 1），约 96 个 Test（gate、swapchain、s2_convert、r7_6_convert、x11 真窗、browser 枚举 20+）。

## 2 Skip 空转（带原因）

- render 子树 119+ 行：没字库跳（text_wrap:86、text_aliased:76、text_transform:45、text_quality:105 等十几处）；没 GPU 跳（r7_2:21 no accelerator、s6_1:25、s6_2:23 offscreen unavailable、quad_cpu:313、depth_r5:463 GPU flush unavailable）；诊断窗默认跳（须 `GPUI_*_VISUAL=1`）。
- gpu 子树 61 行：rwgpu（queue_workdone:13 WGPU_NATIVE_PATH required、safety:92、null_guard:510 wgpu-native unavailable）；webgpu X11（swapchain_x11:33 no DISPLAY/XOpenDisplay 失败；auto_recover:36、recover_vram:27）。

## 3 只断没崩（不断画面）

- 自家帮手为主（assertPixel/assertVertex/assertFloat/assertNonEmpty/assert*VisualStrict），129+ 行。
- `nan_safety_test.go` 13 个：`// must not crash` 无像素断言（`:18,40,103`）。
- EdgeNoCrash 系：quad:217（哨兵错 + 仍白）、vertices_cpu:229、atlas_r4:396、depth_r5:295。
- `// just verify no panic` / `// No assertion needed`：alpha_runs_extended:202、software_coverage:655、sdf_accelerator_coverage:403。
- `svg_test.go:308 assertNonEmpty`（有非透明像素即可）定义 `:688`；`text_transform_test.go:182,801` 数非白像素不比形状。

## 4 金图容差位置

- 全局：`imagediff.go:62 d>2` 算 changed；`:41 ChangedPixels/MeanAbs/RMSE/MaxDelta`。
- quad：`quad_cases.json:1 max_changed_pct 1.0/max_mean_abs 1.0`，判线 `quad_cpu_test.go:289`，探针 tol 逐点冻（多为 0）。
- depth：`depth_r5_cases.json:583 max_changed_pct 1.0`。
- 裁剪诊断严格档：`clipping_gpu_visual_test.go:99 RMSE>24/MeanAbs>4、ratio 0.80–1.20、bbox 18、leak 8`，但默认不执行（须 `GPUI_CLIPPING_VISUAL_STRICT=1 :58`）。
- 文本变换无真金图：`text_transform_test.go:819` 只比像素数与重叠率 <90%，`SAVE_DIFFS=1 :620` 才落 `tmp/`。
