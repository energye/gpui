# 04 无人用 / 仅测试 / 目录过时

> 判定口径：全仓 grep 生产 import/调用；测试与示例调用不算生产。目录指 `RENDER_API_CATALOG.md`。

## 1 目录成立（真没人用，5 抽查全过）

> 复核依据：调用有无用 01 的入口清单与 02 的转发关系表交叉验证，测试/示例调用不算生产。

- `render/recording`：生产 0 import；仅 `examples/render_recording/main.go:20` + `s3c_m3_gpu_gate_test.go:18` + 文档自提。
- `render/surface`：生产 0 import（R4 review 也记零命中）；`render.Context` 未接入。
- `render/svg`：生产 0 import；`ParseSVGPath` 仅包内用。
- `Trim/WithCorners/Discrete/BooleanPath`：仅 `p1_capability_matrix_closers_test.go:3299`、`p1_complex_ui_matrix_test.go:4136`、`s3c_m3_residual_gate_test.go:178` 等测试。
- `render/gpu SetDeviceProvider/AbandonDevice` ⚠️生产未调：仅 `s6_8_window_present_x11_linux_test.go:75`、`auto_recover_depth_test.go:66`，`ui/embedder` 0 命中。
- 附带：`DrawImageNine` 仅门面（`ui/rendering/image_draw.go:48` 包了一层）+ 示例 `scene.go:199` + 测试，生产 widget 0 调；`DrawMesh` 仅 `p1_capability_matrix_closers_test.go:2918`。

## 2 目录反例（1 个，需改目录）

- `DrawShapedGlyphs`：目录 `：172` + `§7.2:314` 写“仅测试”，实际生产 6 处：`ui/rendering/text.go:988,1077`（Paint 批量主路）、`ui/rendering/text_picture.go:205,232,244`（分区提交）、`ui/scene/picture.go:225`（录制回放）。与 `DrawShapedColorGlyphs` 同为生产，需改 🔗 并补证据。
- 旧文残留（非现目录反例）：`ENGINE_UI_WIDGET_RENDER.md:622` 仍列 `DrawImageQuad` 为 🔌，现目录 `§3.5:135-136` 已改 ✅（真窗 game_quad + quad_cpu_test），以现目录为准。

## 3 半废并存（废分配 + 未废销毁）

- `TexturePool Acquire/Release/EndFrame`：头注释自述生产不用（`texture_pool.go:24-28`），但 `GPUShared.PurgeAllSurfaceResources` 仍调 `DestroyAll`（`gpu_shared.go:360-363`，另 `:471,550`）。删留见收敛批。
- `offscreenPool`：复用分支注释禁用，当前只分配不复用（`gpu_render_context.go:3439-3446`），`drainOffscreenPool` 仍在 purge/Close 路上。
