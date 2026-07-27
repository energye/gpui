# 92 · F01–F18 交叉表

> 架构条款 × 能力 ID × 分册。

## §19 F01–F18 交叉表

| F | 内容 | 状态 | 关联能力 ID | 残余 |
|---|------|------|-------------|------|
| F01 | Layer COW | A | FL-* FramePacket | — |
| F02 | 静态层复用/脏 re-raster | A/C | FL-PICTURE RasterizeDirty | 非 GPU RT |
| F03 | RelayoutBoundary | A | FRO-RELAYOUT-B | — |
| F04 | compositing 传播 | A/C | FRO-MARK-COMP | 加深 |
| F05 | 滚动协议 | A | FScroll-* | 可变高 D |
| F06 | 真 vsync | C | FSch-VSYNC | exhost |
| F07 | 逻辑/物理/Y-down | A | FCoord-* | — |
| F08 | 输入不等 Present | A | SubmitLatest | — |
| F09 | 动画 compositor-only | C | FX-OPACITY FL-OPACITY | Present 全画 |
| F10 | GPU/render 热路径 | A/B | render 丰富 ui 薄 | 暴露 P0 |
| F11 | warm-up | A | FSch-WARMUP | — |
| F12 | IO 解码 | A | FImg-CODEC | — |
| F13 | Overlay band | A | FL-OVERLAY-BAND | — |
| F14 | 帧计时 JSON | A/C | §20 | 分位/CPU/RSS |
| F15 | 遮挡停帧 | C | — | 加深 |
| F16 | saveLayer 预算 | C | FF-BUDGET FC-SAVELAYER | 真层 P1 |
| F17 | Ticker/Animation | A | FSch-TICKER | — |
| F18 | Present 策略可文档化 | C | §18.3 | 本文 + P6 |

---


## 分册映射

| F | 分册 | 备注 |
|---|------|------|
| F01–F04 | [01](./01_pipeline_dirty.md) · [08](./08_scene_picture.md) | COW/脏/boundary/compositing |
| F05 | [10](./10_scroll_viewport.md) | 滚动 |
| F06 F11 F14 F17 | [09](./09_scheduler_vsync.md) · [90](./90_metrics.md) | vsync/warmup/timings/ticker |
| F07 | [11](./11_coords_hittest.md) | 坐标 |
| F08 | [09](./09_scheduler_vsync.md) | SubmitLatest |
| F09 | [05](./05_transform.md) · [08](./08_scene_picture.md) | opacity/transform compositor-only **C** |
| F10 | [03](./03_draw_geometry.md) | render 富 |
| F12 | [06](./06_image.md) | IO 解码 |
| F13 | [08](./08_scene_picture.md) | Overlay |
| F15 | [09](./09_scheduler_vsync.md) | 遮挡停帧浅 |
| F16 | [04](./04_path_clip_savelayer.md) · [12](./12_filter_shadow.md) | saveLayer 预算 |
| F18 | [00](./00_meta.md) · [08](./08_scene_picture.md) | Present 策略文档化 |
