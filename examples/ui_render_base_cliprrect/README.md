# ui_render_base_cliprrect — 圆角裁剪图片/文字真窗

C 组第 6 项（`RENDER_FEATURE_COVERAGE` §1 #12 / §3.3）的独立真窗。把
`render.Context` 的 `ClipRoundRect` 与图像、GPU 纹理、字形三条消费路径摆到画面上，
补上此前「有实现、无直调真窗像素证据」的缺口：

- `DrawGPUTexture` → `textured_quad.wgsl` 的 `rrect_clip_coverage`
- `DrawImageEx` → 图片/覆盖管线
- `DrawString` → 字形位图 / glyph-mask / MSDF 管线
- `Push` / `Pop` → 裁剪恢复

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 门禁：5 秒脚本跑，stdout 出 §2.2 JSON，失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_cliprrect -auto-only

# 人工常驻：开窗直到手动关闭（事件写日志并改标题）
go run ./examples/ui_render_base_cliprrect

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_cliprrect -manual-seconds 15
```

Golden 首次播种（只在基图不存在时生效，播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_cliprrect -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_cliprrect -auto-only
```

## 画面（1200×800，2×2 四卡）

每张卡先画卡片底色，再把内容画得比圆角裁剪框更大；如果圆角裁剪没生效，四角的
切角位置会直接露出图像或文字。指标行浮在画面左上角，Golden 时被盖住。

| 卡 | 内容 | 证明什么 |
|---|---|---|
| A GPU 纹理 | 200×160 GPU 视图（`ApplyInvert` 后 `GPUFilterTexture` 发布），每帧用 `DrawGPUTexture` 直绘在 184×146、r=38 的圆角框内 | `textured_quad` 的 RRect 覆盖真生效：框内可见、圆角切角透卡底、框外不残留；`gpu_ops>0`、`cpu_fallback_ops=0`，视图 live paints 持续增长 |
| B ImageBuf | 四象限纯色图（`DrawImageEx`，源 200×160）画在 184×146、r=38 的圆角框内 | 图片消费路径同样被圆角裁剪，圆角外是卡底而不是图片角 |
| C 文字 | 三行 40px `DrawString`，文字起点在裁剪框左侧，字符串越过裁剪框右边界 | 字形墨迹只在圆角框内出现；框外（2px 容差带之外）整卡扫描为 0 个亮字形像素，切角位置为卡底 |
| D Push/Pop | 左边用 r=32 圆角裁同一张图；`Pop` 后右边再画同一张图，不裁剪 | 左图切角透底、右图四角保留图片内容，证明 `ClipRoundRect` 随 `Pop` 恢复而不是永久残留 |

Golden 只比表头 + 图例 + 四卡（627200px），逐位零容差。抓屏用 `xwininfo` 取窗口
id + `xwd -id` 读窗口自身像素（`ffmpeg` 转 PNG），与快照同区对比，per-channel
tol ±2/255。

## 证据

- 逻辑 11/11（子进程 `GOGPU_RENDER_MODE=cpu` 离屏值级）：图片中心/圆角切角/框外、
  r=0 矩形退化为直角、嵌套 RRect+Rect 交集、Push/Pop 后右图保角、文字框内可见、
  框外无墨、Pop 后文字恢复。
- 像素 15/15（GPU 快照）：A 纹理中心/切角/上边/框外；B 图片中心/切角/上边/框外；
  C 文字框内亮像素计数、框外 0、切角为卡底；D 左图切角/中心、右图切角/中心。
- Golden：表头 + 图例 + 四卡 `627200px`，`diff=0.0000%`。
- 抓屏 vs 快照：同 `627200px`，`diff=0.0000%`（tol ±2/255）。
- GPU 路线：`gpu_ops=25803`、`cpu_fallback_ops=0`、`gpu_fallbacks=0`、`presents=300`；
  卡 A 纹理视图 `200×160`、live paints 301；纹理源滤镜 `gpu_ops=1`、
  `cpu_fallback_ops=0`。
- 门禁 JSON：`present_policy=full_paint`、`vsync_source=true`、
  `fps_gate=off_correctness`（正确性窗，不拿 fps 当及格线）。
- 人工档 `-manual-seconds 6`：`presents=360`、`screen check diff=0.0000%/627200px`、
  `logic=11/11`、`pixel=15/15`，退出 0；窗口事件日志与标题更新路径同门禁共用。

## 同批修的引擎缺陷

**CPU 位图/别名文本绕过裁剪栈**（`render/text_dispatch.go` + 新增
`render/text_clip_cpu.go`）

GPU 字形 mask/MSDF 路径已经通过 `setGPUClipRect` 把裁剪传给 shader，但 CPU
`drawStringBitmap` / `drawStringScaled` / `drawStringCPUAliased` 直接调
`text.Draw*` 写 `c.pixmap`，既不经过 scissor，也不经过 `ClipStack`。本窗的逻辑
探针先复现：C 卡文字在 CPU 离屏上下文里框外泄漏 1191 个亮像素；GPU 窗口因为
字形 mask 路径正常而没有暴露。

修法（对齐 Skia save/clip 后所有绘制共享裁剪的语义）：新增
`Context.tryDrawTextWithClip`，有活动裁剪时先把文本 run 画进临时透明 RGBA，再用
`ClipStack.Coverage` 生成 alpha 掩码合成回 `c.pixmap`；无裁剪时保持原快路径。
覆盖 CPU 位图、缩放位图、Aliased 三个入口。

永久回归（`render/cliprrect_consumer_test.go`）：

- `TestClipRoundRectCPUBitmapText` — CPU 位图文本在 RRect 内可见、框外 0 亮像素。
- `TestClipRoundRectCPUAliasedText` — CPU 别名文本同契约（修前框外泄漏 1200px）。
- `TestClipRoundRectCPUScaledText` — CPU 缩放位图文本同契约（2× 设备缩放，修前泄漏 4662px）。
- `TestClipRRectGPUTextureAndText` — GPU `DrawGPUTexture` 的 RRect 切角/中心，以及
  GPU 字形框外 0 墨。

## 限界

1. **CPU 文本裁剪按当前裁剪包围盒分配临时图和掩码**：窗口呈现主路是 GPU 字形路径，
   不受影响；CPU-only/回退路径在整窗大裁剪下每 run 会多一次临时分配。后续可按
   `text.Measure` 收窄，但当前先保证裁剪正确性。
2. **本窗是原子验证**：直接调 `render.Context` 公开接口，只证明 `ClipRoundRect`
   与图像/文字/GpuTexture 的能力本身；目前没有组件库或 2.5D/视频模块的生产调用方。
3. **A 卡依赖 GPU 滤镜发布视图**：`GPUFilterTexture` 只在 GPU 图注册时可用；
   窗口进程有 GPU 时正常，CPU-only 下 A 卡没有可直绘视图（逻辑探针不覆盖 A 卡）。

## 性质

原子验证：直接调 `render.Context` 公开接口，一次验一族裁剪消费路径。示例不算
生产接线。
