# ui_render_base_webp — WebP 加载/解码真窗

A 组第 13 项（`RENDER_FEATURE_COVERAGE` §1 #22）的独立真窗。验证 `render.LoadWebP` 与 `render.LoadImage` 的 WebP 加载路径：无损逐像素（含 Alpha）、有损 VP8、`.webp` 扩展名与未知扩展名内容识别、缺失/非法/空数据错误，以及解码结果真正经 GPU 画到屏幕上。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_webp -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_webp

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_webp -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_webp -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_webp -auto-only
```

## 画面（1200×800，四卡）

| 卡 | 内容 | 证明什么 |
|---|---|---|
| A 无损 + Alpha | 无损 WebP 2× 最近邻铺在棋盘格上；右侧 4× 裁片（含透明洞）与 1× PNG 参考 | 无损解码与 PNG 参考四通道逐位相同；`alpha=0` 透出棋盘格；`alpha=128` 按 straight-alpha 正确混到棋盘格 |
| B 有损 q80 | VP8 有损图 1×；右侧 2× 最近邻裁片 | VP8 有损路径可解码、可绘制；2× 最近邻每个源像素成 2×2 同色块 |
| C 四条加载路径 | `LoadWebP(.webp)` / `LoadImage(.webp)` / `LoadImage(.bin)` / `LoadPNG(ref)` 四张同源缩略图 | 扩展名分派和未知扩展名内容识别都通；四张缩略图逐像素相同 |
| D 失败模式 | 缺失文件、PNG 数据当 WebP、空文件三行红叉；末行绿勾 | 三种坏输入都返回 error，不崩溃、不画占位图 |

指标行浮在画面左上角，Golden 时被盖住；Golden 只比表头 + 图例 + 四卡（627200px），逐位零容差。

## 证据

- 逻辑 19/19（子进程 `GOGPU_RENDER_MODE=cpu`）：无损加载/尺寸/RGBA8 格式；无损 vs PNG 四通道零差；Alpha 0/部分/255 三类计数（379/64/5701）；`.webp` 扩展名、`.bin` 内容识别、两条路径与 `LoadWebP` 逐位相同；有损尺寸/不透明/颜色数 ≥1000/与参考均值差 ≤0.05；两次解码逐字节相同；缺失/非 WebP/空文件三错误。
- 像素 24/24（GPU 快照）：A 卡红/绿/蓝源点、`alpha=128` 合成、`alpha=0` 透棋盘、PNG 参考点、4× 裁片点；B 卡渐变/亮圆/暗块/2× 裁片点；C 卡四路径各一点 + 四缩略图两两比对 3×6144=18432px 零差；D 卡红/绿徽章与 A/B 卡间隙；另有 A 4× 块、B 2× 块、C 透明洞透卡底色三项区域探针。容差 2–4/255（Alpha 合成 4/255，有损 3/255）。
- Golden：表头 + 图例 + 四卡 `627200px`，`diff=0.0000%`。
- 抓屏：`xwininfo` 取客户区 + `ffmpeg x11grab`，与快照同区 `diff=0.0000%`（per-channel tol ±2/255）。
- GPU 路线：窗口绘制 `gpu_ops=105303`、`cpu_fallback_ops=0`；门禁 `presents=300`；15 秒人工窗 `presents=899`、`fps_wall≈58.28`、`hitch_count=0`。
- 门禁 JSON：`present_policy=full_paint`、`vsync_source=true`、`fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）。

## 永久回归

`render/webp_contract_test.go` 5 例，数据在 `render/testdata/webp/`：

- 无损 WebP 对 PNG 四通道逐位零差 + Alpha 三类计数。
- `LoadImage` 的 `.webp` 扩展名与 `.bin` 内容识别，结果与 `LoadWebP` 逐位相同。
- 有损 VP8 尺寸/不透明/颜色多样性 + 与参考解码均值差有界。
- 缺失、非 WebP、空文件三种错误；两次解码确定性。

## 范围与限界

- 本窗只覆盖**静态** WebP（VP8 有损 / VP8L 无损，含 Alpha）；动画 WebP 的时间轴/多帧不在 `LoadWebP` 单图 API 与本窗范围。
- 有损 VP8 解码不保证跨解码器逐位一致；永久回归用「均值差有界」而不是逐位相等。无损 VP8L 才做逐位对照。
- WebP 目前仍**无生产消费者**；本窗与单测属原子验证，不算生产接线。
- 测试图由 PIL 生成并入库：`testdata/webp_lossless.webp`（无损 + 透明洞 + Alpha 128 块）、`webp_lossless_ref.png`（同源 PNG）、`webp_lossless.bin`（同字节未知扩展名）、`webp_lossy.webp`（VP8 q80）、`webp_lossy_ref.png`（从有损 WebP 导出的解码参考）。
