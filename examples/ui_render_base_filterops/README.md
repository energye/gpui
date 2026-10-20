# ui_render_base_filterops — 内容滤镜真窗

A 组第 14 项（`RENDER_FEATURE_COVERAGE` §1 #40）的独立真窗。把 `render.Context` 的四个内容滤镜入口摆到画面上：

- `ApplyInvert()` — 反色
- `ApplyBlurXY(rx, ry)` — 横向/纵向高斯模糊
- `ApplyImageFilterGraph(nodes...)` — 多节点滤镜图
- `GPUFilterTexture()` + `DrawGPUTexture` — 零读回发布视图并直接上屏

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_filterops -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_filterops

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_filterops -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_filterops -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_filterops -auto-only
```

## 画面（1200×800，2×2 四卡）

每卡左半是源图，右半是滤镜结果；下排是源 4× / 结果 4× / 差异放大。

| 卡 | 内容 | 证明什么 |
|---|---|---|
| A 反色 | 四条纯色横带；`ApplyInvert` | 每个不透明像素严格满足 `结果 = 255 − 源`（整卡 25870px 最大误差 1/255）；反色两次回到原图 |
| B 纵向模糊 | 深色底 + 黄色横带 + 青色竖条；`ApplyBlurXY(0,4)` | 横带上下边软化、带中心与远处背景不变；竖条左右边保持锐利（`rx=0` 不横向扩散） |
| C 滤镜图 | 顶部黄带 + 左红右青两块；`BlurXY(0,4) → Grayscale → Invert` | 结果三通道相等，且等于 `255 − Rec.709 luma`；纵向模糊过渡保留，左右分界仍锐 |
| D 发布视图 | 不透明深底 + 橙带 + 青条；`ApplyBlurXY(0,6)` 后 `GPUFilterTexture` 发布 199×130 视图，卡面每帧用 `DrawGPUTexture` 直绘 | 零读回呈现路径真的上屏；发布视图保留 CPU 写入的不透明底色（四角不是透明黑）；与同参数 CPU 参考图对照有界（最大 6/255、均值 ≤1.5/255） |

指标行浮在画面左上角，Golden 时被盖住；Golden 只比表头 + 图例 + 四卡（627200px），逐位零容差。

## 证据

- 逻辑 16/16（子进程 `GOGPU_RENDER_MODE=cpu` 离屏值级）：nil 安全、反色逐通道精确、反色两次、半透明预乘反色、灰度 Rec.709、零半径模糊恒等、纵向/横向模糊方向性、alpha 保持、图链=逐步、颜色矩阵合并、空图/空节点、确定性、单节点模糊=直接调用、CPU 路径不发布 GPU 纹理。
- 像素 26/26（GPU 快照）：A 卡四点 + 整卡反色关系；B 卡带中心/远处背景/边沿软化/竖条保锐/区域平坦/横向锐边/上下光晕；C 卡左/右/顶三点 + 灰度区域 + 模糊过渡层数 + 左右分界；D 卡四角底色/带中心/竖条/边沿/区域均匀/非空 + 与 CPU 参考图对照；另加卡间隙底色。
- Golden：表头 + 图例 + 四卡 `627200px`，`diff=0.0000%`。
- 抓屏：`xwininfo` 取窗口 id + `xwd -id` 读窗口自身像素（`ffmpeg` 转 PNG），与快照同 627200px 对照 `diff=0.0000%`（per-channel tol ±2/255）。优先用 `xwd` 是为了不被遮挡窗口误判；没有 `xwd` 时退回 `ffmpeg x11grab`。
- GPU 路线：四张卡各 `gpu_ops=1`、`cpu_fallback_ops=0`；窗口绘制 `gpu_ops=67803`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`；D 卡发布视图 `filter_d_texture_ok=true`、`199×130`、`filter_d_texture_before=false`、live paints 301（manual 16s 为 960）。
- 门禁 JSON：`present_policy=full_paint`、`vsync_source=true`、`presents=300`、`fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）；manual 16s 收真实 pointer 事件 3 条，`presents=959`、`fps_wall≈58.31`、`screen_check_diff_pct=0`。

## 同批修的两个引擎缺陷

1. **GPU 反色在预乘存储上用 `1 − c`**（`render/internal/gpu/filter_gpu_graph.go`）
   半透明像素会被算成大于 alpha 的通道。Skia/Flutter 的颜色滤镜在直通 alpha 空间做，反色应为 `straight' = 1 − c/a`，存回预乘就是 `a − c`。窗内 A 卡整卡关系与逻辑探针 `filter_invert_translucent_premul` 同时钉住。
2. **GPU 滤镜的离屏 RT 只重放待画命令，丢掉 CPU 已写好的底**（`render/filter_ops_gpu.go`、`render/internal/gpu/gpu_render_context.go`、`render/internal/gpu/filter_flush_coalesce.go`、`render/accelerator.go`）
   `ClearWithColor` / CPU 回退绘制写在 pixmap 里，零读回滤镜 RT 首 pass 却按 Clear 开始，底被清成透明。修法：跟踪 `pixmapBaseContent`，在重放待画命令前把 CPU 底上传进 RT（`GPURenderTarget.SeedFromData` + `seedViewFromRGBA`），并把视图标记为已有颜色内容，让首 pass 走 Load；纯 GPU 的透明清屏/glow 帧仍保持零读回。D 卡四角底色与 `filter_d_texture_before=false` 是这条修复的窗级钉子。

永久回归：

- `render/filter_ops_contract_test.go` — 3 例 CPU 契约（反色预乘与 alpha 不变、`ApplyBlurXY` 横纵各向异性、图链顺序=逐步执行）
- `render/filter_ops_gpu_test.go` — 5 例 GPU（反色半透明、跨上下文 blur、effect publish 保底、CPU 回退底、图链对 CPU）

## 限界（`RENDER_FEATURE_COVERAGE` §6）

1. **`GPUFilterTexture` 只在 GPU 图注册时可用**：CPU 路径返回 `ok=false`（逻辑探针 `filter_texture_cpu_unavailable` 钉住）。D 卡依赖窗口进程里的 GPU 图注册；没有 GPU 时本卡没有可直绘的发布视图。
2. **GPU 与 CPU 模糊不是逐位一致**：D 卡的 CPU 参考图只用于对照，断言是最大 6/255、均值 ≤1.5/255；不声称逐位相等。A 卡的反色是逐像素精确关系，不受这条影响。
3. **仍是原子验证**：本窗直接调 `render.Context`，只证明能力本身；`ApplyInvert` / `ApplyBlurXY` / `ApplyImageFilterGraph` / `GPUFilterTexture` 目前没有组件库或 2.5D/视频模块的生产调用方。
4. **CPU 底播种会整面上传**：当 pixmap 真的持有底内容（不透明 clear / CPU 绘制）时，本次滤镜会把整面底上传进 RT；纯 GPU 的透明清屏帧不传，走原来的零读回路径。CPU 底不存在的帧没有这笔上传。

## 性质

原子验证：直接调 `render.Context` 公开接口，一次验一族内容滤镜。示例不算生产接线。
