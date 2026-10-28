# P1.2b nogpu 测试标签修复（2026-10-28）

> 性质：P1.2 采集时发现的 nogpu 测试编译红项，用户决定按 P1.1b 先例插队修复。
> 范围：只给测试文件补 `//go:build !nogpu`，不改生产代码、不改测试逻辑、不改默认标签行为。

## 1 问题（修复前实测）

`go vet -tags nogpu` 与 `go test -tags nogpu` 在 2 个包无法编译测试代码：

| 包 | 失败文件数 | 代表错误 | 原因 |
|---|---|---|---|
| `render/internal/gpu` | 9 | `empty_clip_skip_test.go:18:9: undefined: GPURenderContext` | 测试文件引用只在 `!nogpu` 生产文件里定义的 GPU 类型/函数，但文件本身没打标签 |
| `render_test`（`render/`） | 11 | `f1_adv_layer_present_test.go:14:2: undefined: requireNativeGPU` | 同上；`requireNativeGPU`、`sampleRGBA`、`p1Sample`、`residualFont` 等辅助函数只在 `!nogpu` 测试文件里定义 |

- 同包其余 GPU 测试早已采用 `//go:build !nogpu`（例如 `render/internal/gpu/analytic_filler_test.go`、`render/p1_gpu_fixed_pixel_test.go`），这 20 个文件是漏标。
- 影响：`go vet -tags nogpu ./...` 无法完成这 2 个包的分析（不满足 P1.2「两种标签组合都能完成分析」）；`go test -tags nogpu` 直接 build failed。
- 原始输出：`before-vet-nogpu.txt`、`before-test-compile-nogpu.txt`；用 `-gcflags=all=-e` 展开全部错误见 `before-test-compile-gpu-e.txt`、`before-test-compile-render-e.txt`。

## 2 改动（20 个文件，各加 2 行）

统一在文件顶部加：

```go
//go:build !nogpu

```

| 包 | 文件 |
|---|---|
| `render/internal/gpu`（9） | `empty_clip_skip_test.go`、`glyph_placement_test.go`、`image_bicubic_test.go`、`offscreen_pool_r6_test.go`、`opt41_surface_rp_reuse_test.go`、`opt42_dualtex_scratch_test.go`、`retire_queue_test.go`、`tex_shrink_r6_test.go`、`view_raw_attack_test.go` |
| `render`（11） | `f1_adv_layer_present_test.go`、`filter_present_coherence_test.go`、`layer_rt_clear_test.go`、`m5_color_glyph_api_test.go`、`p01_layer_gpu_test.go`、`p02_brush_native_test.go`、`p03_advanced_blend_test.go`、`p04_filter_gpu_test.go`、`p11_text_shape_cache_test.go`、`p12_clip_mask_gpu_test.go`、`p13_frame_flush_test.go` |

- 同包其余未标标签的测试文件（如 `damage_scissor_test.go`、`stroke_width_test.go`）在 nogpu 下能编译，未改动。
- 没有改任何 `_test.go` 的测试内容，也没有动生产文件。

## 3 验证（修复后）

| 检查 | 结果 | 证据 |
|---|---|---|
| `go vet -tags nogpu <348 包>` | 完成分析；58 条 unsafe.Pointer 告警（与默认标签同源）；0 类型错误 | `after-vet-nogpu.txt`、`baseline/vet.txt` §3 |
| `go test -tags nogpu -run '^$' <348 包>` | 退出码 0；137 个有测试的包全部编译通过，211 个无测试包，0 FAIL | `after-test-compile-nogpu.txt` |
| `go vet <352 包>`（默认） | 除 `view_raw_attack_test.go` 行号 11→13 位移外，与修复前逐字节一致（61 条诊断不变） | `baseline/vet.txt` §2 |
| 默认标签受影响包测试编译 | `go test -run '^$' ./render ./render/internal/gpu` 退出码 0 | 本会话实测 |

- nogpu 包数 348 比默认 352 少 4，是只在默认标签下存在的 GPU 示例包（`examples/render_gpu`、`examples/render_scene_gpu`、`render/examples/compute_clip`、`render/examples/compute_pipeline`），不是本次改动造成。
- 未跑测试用例本身（`-run '^$'` 只编译）；逐测试文件执行归 P1.3。

## 4 未修项（登记到基线）

以下告警不在 P1.2b 范围，已在 `baseline/vet.txt` §4/§5 逐条登记：

- `ui/platform` 56 条 unsafe.Pointer：Wayland/X11 FFI 指针写法，属 `ui/platform` 线，本收敛线外。
- 3 条测试伪造指针告警（`render/internal/gpu/res/view_test.go`、`view_raw_attack_test.go`）：测试夹具，归 P2.5 测试卫生。
- 2 条 `unreachable code`（`render/internal/gpu/gpu_render_context.go:4296,4344`）：R8 路径暂时关闭留下的死代码，归 P2.5 零行为变化清理。

## 5 证据清单

```
before-vet-nogpu.txt              # 修复前 go vet -tags nogpu 过滤清单输出（2 包类型错误 + 58 告警）
before-test-compile-nogpu.txt     # 修复前 go test -tags nogpu 编译输出（2 包 build failed）
before-test-compile-gpu-e.txt     # render/internal/gpu 全部错误（-gcflags=all=-e）
before-test-compile-render-e.txt  # render 全部错误（-gcflags=all=-e）
after-vet-nogpu.txt               # 修复后 go vet -tags nogpu（58 告警，0 类型错误）
after-test-compile-nogpu.txt      # 修复后 go test -tags nogpu -run '^$'（137 ok / 211 no test / 0 FAIL）
packages-default.txt              # 默认标签 vet 包清单（352）
packages-nogpu.txt                # nogpu vet 包清单（348）
```
