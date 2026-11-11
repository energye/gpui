# I4 销账：r20 steady/recover 快照全黑 = CPU pixmap 零读回 seam（example 层）

> 台账来源：§2.2 I4（P2.4d 登记：「R20 r20_steady.png/r20_recover.png 全黑
> 1200x800 RGB 全 0、5585 B，P1.4 归档基线同图；r20_final.png 正常（extrema
> 14–249）说明整条截图链没坏」）。修复日期 2026-11-11，提交 `46aa5454`。

## 复现（2026-11-11 现行树）

`RUN_SECONDS=30 go run ./examples/ui_wr_r20_filter`：stderr 出现
`embedder: snapshot request timed out`（SnapshotAsync 500ms 等待超时，闭包
照常完成）+ `snapshot stored: r20_steady.png` / `r20_recover.png` —— 两文件
**各 5585 字节**，与 P1.4 归档全黑图同字节数；final（74819 B）正常。

## 根因（源码钉死）

- r20/r22 的 `saveSnap` 用 `app.Target().Context().Image()` 读回 ——
  `Image()` = FlushGPU + syncViewFlushIntoPixmap + materializeFilterGPU →
  **CPU pixmap**（`render/context_draw.go:21-25`，`pixmap.ToImage()`）；
- 真 swapchain GPU 帧按 `presentAfterFlush` 语义**不回写 pixmap**
  （非 nil present = 真实 swapchain → EndFrame 释放 view、清 tracking；
  K6 批 4 修复记录与 P4.2 边界登记均明示此 seam）；
- retained/full_paint GPU 窗的 CPU pixmap 从未被写（ClearWithColor 只写
  CPU pixmap 的是 headless 合成路径；窗清屏走引擎 ClearR/G/B）→
  未初始化缓冲全零 → 1200×800 全黑 PNG；
- final 快照正常的原因：走引擎 `SnapshotPath` 保留包回放，不经
  `Context().Image()`。

## 修法（对齐 r1/c8 既有模式，引擎零改动）

r1 的 saveSnap 注释早有结论：「Reading the post-present context Image()
returns a blank/stale pixmap (zero-readback present)」——先用
`embedder.PaintPresentTree` 把活树重画进 Context 再存。r20/r22 是没跟上的
两个（c8 自带 live-blit 探针机制、r1 已修）。修法 = 两窗 saveSnap 改为：

```go
dc.BeginFrame()
embedder.PaintPresentTree(dc, app.Pipeline(), shell.Root, nil,
    clearBG[0], clearBG[1], clearBG[2], 1, true)
dc.SavePNG(path)
```

## 验证（客观判据：非零像素计数 + extrema 对照）

| 快照 | 修前 | 修后 |
|---|---|---|
| r20_steady / r20_recover | 5585 B 全黑 | **960000/960000 非零 px、extrema (14,249)**——与 final 一致 |
| r22_steady / r22_recover | 同病同修 | **960000/960000 非零 px、extrema (14,255)** |

两窗门禁全 PASS（scripted 5/5、6/6；golden 0.0000%）。

## 遗留观察（不在本项扩面）

- `embedder: snapshot request timed out` 仍在：PaintPresentTree 重画
  1200×800 超过 SnapshotAsync 的 500ms 等待——cosmetic（闭包照常完成并
  写盘，r1 同款已知行为）。
- `Context.Image()` 在真 swapchain GPU 窗上静默返回空 pixmap = fail-open
  读回契约问题——K6 批 4/P4.2 边界已有登记，如需 fail-closed 化另行立项
  （影响面大，现有 headless/离屏用例合法依赖 Image()）。
