# P4.3 提前立项：DrawShadow 阴影模糊整画布成本 + 无跨帧缓存（2026-11-10）

> 性质：性能专项评估（数据齐、未动引擎）。用户人工窗验收时抓出：drawshadow 窗
> live 帧阴影上线后 330ms/帧 ~3fps、最大化/最小化整窗跟慢。拍板「现在就修」，
> 本文为开工登记与归因数据；实现按 §3 计划执行。

## 1 归因数据（探针实测，Probe: TestShadowCostProbe/Split，已删）

| 画布 | 单次 DrawShadow+flush | fill 单独 | blur 单独 |
|---|---|---|---|
| 200×200 | 30ms | 1.3ms | **29ms** |
| 800×800 | 533ms | — | — |
| 1200×1200 | 1.21s | 68ms | **1.15s** |

- **blur 成本随画布面积线性**（29ms→1.15s ≈ 36 倍像素 40 倍耗时），不随块大小——
  模糊作用在**整幅 pixmap/图层 RT**（filter_ops_gpu.go `tryApplyFilterGraphGPU`：
  `w, h := src.Width(), src.Height()` 全画布），块本体只占 ~2% 面积。
- 窗口 1200×800：每阴影 ≈55ms × 6 个/帧 = **330ms/帧 ≈ 3fps**。
- 窗 JSON：frame_raster≈450-490ms、cpu_raster_pct 99.8%、frame_flushes=8、
  filter-graph pool 496 hits/84 evictions（有缓存但每帧阴影键变化/被逐）。
- 放大/最小化慢的原因：交互帧排在同一光栅队列，每次都付同样的 330-490ms；
  resize 另加 InFullRecovery 全量重绘强制。

## 2 与标准方案的差距（对齐 Skia）

Skia 对 saveLayer+blur 的做法：**模糊 RT 裁剪到「几何紧致边界 + 模糊边距」**
（SkGpuBlurUtils 按 clipped draw bounds 出 RT，σ 换算 pad），并有 **blur 缓存**
（同 sigma/形状复用）。本实现：layer RT 全画布、blur 输入全画布、零跨帧缓存
（每帧录制克隆路径 → 内容/指针键都不命中）。

## 3 实现计划（P4.3，按序）

1. **分段计时归因**（第一步，动刀前必做）：`runGPUFilterGraphEx`
   （filter_gpu_graph.go:1085）env 门控分段计时——staging 上传 / 编码 /
   Submit / （wantPixels 时）MapBuffer 回读 / materialize——把 1.15s 钉到段。
2. **模糊区域裁剪**：DrawShadow 先算阴影设备紧致界（path 界 + offsetY +
   3σ 边距），blur 输入与发布 RT 裁剪到该界（graph 支持子区域或 bounded
   layer RT），块级成本从 O(画布) 降到 O(块)。
3. **跨帧缓存**：键=(path 内容哈希, elevation, color, CTM, dpr)，缓存已发布
   阴影 view（零读回发布机制现成），静态画面第二帧起只做一次 DrawGPUTexture
   合成；LRU+显存账本入池（谁建谁放）。
4. **红测**：区域裁剪前后阴影像素逐位一致（金图/探针）；缓存命中/失效断言
   （改 elevation/颜色必须重算）；成本断言（静态二帧第二帧 < 首帧的 1/4，
   mutation 反证）。
5. **验收**：drawshadow 窗 fps 回到 55+（或 interval_p95 < 22ms 线）、双后端
   金图 0.0000%、显存账本无泄漏。

## 4 状态

⬜ 立项登记（数据齐、方案定、动刀未开始）。归属 P4.3 性能线（mask 临时纹理
池化/backdrop 损伤区合成同族——都是「整画布处理该裁剪处理」的债）。
