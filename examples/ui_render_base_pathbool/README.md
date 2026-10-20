# ui_render_base_pathbool — 布尔路径（Path.Op）真窗

覆盖 C 组缺窗清单第 5 项（`docs/RENDER_FEATURE_COVERAGE.md` §1 #5 / §3.3）：
`render.Path.Op` / `BooleanPath` 的 **Union / Intersect / Difference / Xor**。
本窗是该族的第一个直调画面证据；之前只有值级 `render/path_boolean_test.go` 和
`s3c_m3_residual_gate_test.go` 门禁，没有真窗。

## 怎么跑

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

go run ./examples/ui_render_base_pathbool                 # 人工窗：常驻，关窗后出汇总
go run ./examples/ui_render_base_pathbool -manual-seconds 10  # 人工窗，自动关闭
RUN_SECONDS=5 go run ./examples/ui_render_base_pathbool  # 定时窗（≥5s）
go run ./examples/ui_render_base_pathbool -auto-only     # 门禁：JSON + exit code
```

首次跑若 Golden 缺失，先确认画面后用
`GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_pathbool -auto-only`
播种 `testdata/showcase_pathbool_base.png`；之后每次必须逐位零差。

## 画面（四张卡，1200×800）

| 卡 | 演示 | 为什么必须这样画 |
|----|------|------------------|
| A | `Difference`：同一外框+分数洞，左侧 NonZero、右侧 EvenOdd | 证明布尔输出在两种填充规则下都正确；分数洞同时钉住 0.25px 级精确顶点，不被量化到整数栅格 |
| B | `Union`：两个交叠圆，半透明 α=0.72 | 交叠区只能做一次 source-over；半透明把“重复填/重复预乘”直接暴露成深色缝，不透明填充看不出来 |
| C | `Intersect`：菱形 ∩ 圆 | 菱形和圆同为曲线展平后的多边形，交点必须是解析几何交点，不能是像素网格量化点 |
| D | `Xor`：两圆在 x≈3000 处运算，再整体平移进卡 | 旧实现有 2048px 静默截断；运算本身在 2048 之外，证明没有截断，平移只用于显示 |

## 证据（2026-10-20）

自动门禁 `-auto-only`，1200×800，5s：

- 逻辑探针 **10/10**：精确轮廓/verbs、分数顶点、Union 单轮廓、Intersect 精确交点、
  Xor 两瓣、远坐标、NonZero/EvenOdd 一致、曲线展平、空操作数、自交蝴蝶结。
- 像素探针 **18/18**：A/B/C/D 四卡点采样，oracle=调色板常量 + 解析 source-over，
  容差 8/255，另加通道带。
- Golden：表头 + 图例 + 四卡共 **627200px**，`diff=0.0000%`（播种后复跑）。
- 抓屏 vs 快照：同 **627200px**，`diff=0.0000%`，per-channel tol ±2/255；
  `xwininfo` 取客户区 + `ffmpeg x11grab`，抓屏失败/尺寸不符即红。
- GPU 路线：`gpu_ops=29703`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`，
  `present_policy=full_paint`、`vsync_source=true`、`presents=300`。
- 5s 运行：`fps_interval≈60.12`、`p95≈17.43ms`、`hitch=0`。
  这是正确性窗，`fps_gate=off_correctness`，性能门禁不在这里冒充通过。
- 人工 `-manual-seconds 10`：`presents=599`、`fps_interval≈59.98`、`p95≈17.33ms`、
  `screen=true`、`gpu_ops=59304`、`cpu_fallback_ops=0`，退出 0。

## 回归

- `go test ./render -run '^(TestPathBoolean|TestS3c_M3_PathBoolean)' -count=1`：绿。
- `go test ./render/internal/gpu -run '^(TestRenderSessionStencilPremultipliedColor|TestRenderSessionStencilOnly|TestRenderSessionMixed|TestMakeCoverUniform|TestMakeCoverUniformDoesNotPremultiplyTwice|TestPremultiplyRGBA|TestCoverUniformStickyContract|TestStencilPathCommandFields)$' -count=1`：8/8 绿。
- `go test ./render/internal/gpu -count=1` 仍有 7 个**存量**失败；用 `HEAD` 干净 worktree 同跑，失败集合逐项相同（BindGroupLayoutSwitch_StencilThenSDF、Opt21/34/36/38），本次未新增失败。
- `go build ./examples/ui_render_base_pathbool ./render/internal/gpu`、`gofmt -l`、`git diff --check`、`go run ./scripts/apidoc` 均通过（apidoc：覆盖 render 主包 + scene/recording/surface/svg 全部公开符号）。

## 同批修的引擎缺陷（重要）

B 卡一开始得到 `(0.188, 0.322, 0.537)`，而正确的一次 source-over 是
`(0.250, 0.434, 0.730)`；alpha 仍是 0.72。数值正好等于 **RGB×α²**，不是
“画了两遍”（两遍 source-over 会得到 `(0.286, ...)`）。

根因在 `render/internal/gpu` 的颜色契约：

- `StencilPathCommand.Color` 的注释和调用方都按 **预乘 RGBA** 传入；
- `render_session.go` 的 slab stencil uniform 又乘了一次 alpha；
- `stencil_renderer.go:updateRenderBuffersSticky` 又乘了一次；
- `makeCoverUniform` 又乘了一次。

不透明路径（α=1）乘几次都一样，所以 A/C/D 一直正常；半透明 path 才把问题暴露成
RGB 变暗、alpha 不变。修法：stencil 内部统一为预乘色，三处直通；独立
`StencilRenderer.RenderPath` 的公开入口仍收直通色，在边界预乘一次。

永久回归：

- `render/internal/gpu/stencil_render_test.go:TestMakeCoverUniformDoesNotPremultiplyTwice`
  —— 值级钉住 uniform 不再乘 alpha。
- `render/internal/gpu/render_session_test.go:TestRenderSessionStencilPremultipliedColor`
  —— GPU 采样 α=0.5 的预乘色；若重复预乘，RGB 会减半。

## 已知限界

- **输出是折线，不保曲线**：输入曲线按 0.1px 展平后交给 Clipper，输出只有
  MoveTo/LineTo/Close；Skia pathops 会保留曲线段，本实现当前不保。见
  `RENDER_FEATURE_COVERAGE.md` §6。
- **布尔结果的生产消费者仍为零**：本窗是直调 `render` 的原子验证，不算生产接线。
  组件/2.5D/视频真正调用 `Path.Op` 后才另记生产状态。
- 坐标精度为 Clipper 的 1/1024px；本窗只断言显示相关的 0.25px 级顶点，不去声称
  任意超长坐标的逐位浮点等价。
