# P4.3 步骤①：分段计时归因（2026-11-10）

> 性质：动刀前置诊断。用户硬约束「①的归因结果出来后、动 graph 核心前，停下
> 报告拍板裁剪方案」。本文是拍板依据；②③未动、graph 核心零行为改动。
> 复现环境：分支 feat/p2-收敛整合 HEAD `8cdb7983`，Linux/X11，940MX + NVIDIA 580，
> `WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`，离屏独立设备（不呈现，非真窗）。

## 0 复现命令

```bash
# 粗测（端到端耗时 + 走 GPU 还是 CPU）
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go test ./render \
  -run '^TestP43RoughProbe$' -count=1 -v -timeout 10m
# 分段计时（env 门控，默认关）
GPUI_P43_TIMING=1 WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go test ./render \
  -run '^TestP43RoughProbe$' -count=1 -v -timeout 10m
```

计时点（均 env 门控 `GPUI_P43_TIMING=1`、默认零开销、无公开 API）：
- `render/internal/gpu/filter_gpu_graph.go` `runGPUFilterGraphEx`：setup / upload /
  encode / submit / map 五段 + total（helper 在 `render/internal/gpu/p43_timing.go`）。
- `render/filter_ops_gpu.go` `tryApplyFilterGraphGPU`：`finish()` 内 hasLayer 分支的
  layer 段（materializeFilterGPU + cropFilterOutputToLayerBounds +
  seedTopLayerGPUFromPixmap）+ total（helper 在 `render/p43_timing.go`）。
- 尺寸参数 `render/testdata/p43_timing_cases.json`；原始输出
  `rough-probe.txt`、`segmented-timing.txt`（本目录）。

## 1 先确认路径：走的是真 GPU 图，不是 CPU blur

粗测（`rough-probe.txt`）：三尺寸 `gpu_ops>0`、`cpu_fb=0`、`frame_flushes=2`。
`DrawShadow` 走 `ApplyBlur → tryApplyFilterGraphGPU → runGPUFilterGraphEx`，
`wantPixels=false`（FromView 零读回发布 view），不是 CPU `blurApply`。

**端到端实测（离屏真 GPU 路，单次 DrawShadow + `dc.Image()` 强制收尾）**：

| 画布 | 立项文档「blur 单独」 | 粗测 blur-only | 粗测 drawshadow | gpu_ops | cpu_fb |
|---|---|---|---|---|---|
| 200×200 | 29ms | 12.8ms | 17.7ms | 3 | 0 |
| 800×800 | 533ms | 39.9ms | 58.0ms | 3 | 0 |
| 1200×1200 | **1.15s** | 64.3ms | **102.0ms** | 3 | 0 |

**立项文档那组「随面积线性暴涨到 1.15s」的数据，在这条真 GPU 路上复现不了**
（1200² 实测 102ms，差 ~11×；且 17.7→102ms 只有 5.8×，不是面积 36× 的线性）。
按 memory 已立的教训「离线探针不 import render/gpu 会假绿/假慢」，那组探针极可能是
**没接 GPU 的 CPU-only 路径**（CPU 高斯 O(面积×半径)，故线性且秒级），或含窗口
6 阴影叠加与其它现场开销。窗口 JSON 的 55ms/阴影、330ms/帧，与本次 1200² 102ms
按面积折算到 1200×800 约 68ms/阴影**量级吻合**——所以窗口症状真实，但成本归属
要按下面的分段数据重新落位，不能照立项文档「graph 全画布 ping-pong」去动刀。

## 2 分段数据（同一次跑，segmented-timing.txt）

| 尺寸 | 模式 | graph setup | upload | encode | submit | map | **graph total** | **ops total** | **layer 段** | P43ROUGH total |
|---|---|---|---|---|---|---|---|---|---|---|
| 200² | blur-only | 2.838 | 0 | 0.264 | 0.313 | 0 | 3.442 | 5.176 | 0 | 10.4 |
| 200² | drawshadow | 0.006 | 0 | 0.224 | 0.259 | 0 | 0.493 | 9.890 | **5.336** | 15.4 |
| 800² | blur-only | 0.045 | 0 | 0.187 | 0.289 | 0 | 0.524 | 7.508 | 0 | 44.4 |
| 800² | drawshadow | 0.009 | 0 | 0.279 | 0.356 | 0 | 0.648 | 46.170 | **35.328** | 60.8 |
| 1200² | blur-only | 0.050 | 0 | 0.214 | 0.145 | 0 | 0.411 | 9.929 | 0 | 69.0 |
| 1200² | drawshadow | 0.008 | 0 | 0.196 | 0.150 | 0 | 0.357 | 73.249 | **66.401** | 96.7 |

（单位 ms。200 blur-only 的 setup 2.838ms 是首次设备/池初始化，稳态 0.006–0.05ms。）

读法：
- `upload=0`：走 FromView，无 CPU 全画布上传；`map=0`：`wantPixels=false`，
  graph 内无像素回读；`encode+submit` 合计 0.3–0.6ms 且**与画布面积无关**——
  GPU 命令只是排队，设备侧异步执行。
- `graph total` 稳态 0.36–0.65ms，**1200² 反而最小**：graph 核心（立项文档点名的
  全画布 ping-pong RT）**不是成本大头**。
- `ops total − graph total`（blur-only）：4.8 / 7.0 / 9.5ms，随面积增长——prelude
  （`syncViewFlushIntoPixmap` / 前置 materialize / `FlushGPU` / 全画布
  `copy(c.filterSrcScratch, src.Data())`），量级小。
- **`layer 段`（hasLayer=true）= 5.3 / 35.3 / 66.4ms，随面积线性，占 drawshadow
  的 54% / 76% / 91%——这就是大头**。它是 `finish()` 里的
  `materializeFilterGPU()`（把发布的整幅 GPU 纹理 Map 回 CPU pixmap，内部
  CopyTextureToBuffer + Poll(Wait) + MapBuffer，同步等 GPU 把整幅 blur 算完再搬
  整幅像素）+ `cropFilterOutputToLayerBounds()` + `seedTopLayerGPUFromPixmap()`
  （整幅再上传回 layer RT）。cpu_raster_pct 99.8% 由此而来：CPU 在同步等待 + memcpy。
- `P43ROUGH total − ops total`：剪影 fill / Push-PopLayer / `dc.Image()` 触发的第二次
  materialize 等，1200² 约 23ms（`dc.Image()` 是探针强制收尾，窗口 present 不必然每帧调）。

## 3 归因结论（4 条）

1. **1.15s 在真 GPU 路复现不了**；可复现的成本是 1200² drawshadow ≈ 96.7ms
   （窗口 1200×800 单阴影 ≈55ms、6 阴影 ≈330ms/帧与此同源）。
2. **成本不在 graph 核心**：上传/编码/Submit/Map 全段合计 <1ms 且与面积无关。
3. **成本大头 = 隔离层（PushLayerIsolated）的全画布 GPU↔CPU 往返**
   （`materializeFilterGPU` 回读 + `seedTopLayerGPUFromPixmap` 回种），随画布面积
   线性，是「整画布处理」债的真正落点；次要项 = prelude 的全画布 scratch 拷贝。
4. 因此「模糊区域裁剪」若只改 `runGPUFilterGraphEx` 的 ping-pong RT 尺寸，收益有限；
   **必须把隔离层 RT 与其回读/回种范围一起裁到阴影紧致界**，才能让 GPU blur
   只算小块、回读只搬小块，两头同时从 O(画布) 降到 O(块)。跨帧缓存（③）则
   让静态帧连这条往返都不付。

## 4 对「停下报告·拍板裁剪方案」的影响

立项文档 §3 ②给的两个候选是「graph 子区域 或 bounded layer RT」。按本归因：
- **graph 子区域**（只给 runGPUFilterGraphEx 加子区域 RT）：layer 回读链仍全画布，
  实测 66ms 大头不动，收益有限。
- **bounded layer RT**（把 DrawShadow 的隔离层 RT、materialize 回读、seed 回种
  统一裁到阴影设备紧致界 path 界+offsetY+3σ）：正对 66ms 大头，且与 Skia
  `SkGpuBlurUtils` 的「clipped bounds + σ pad 出 RT」口径一致。

影响面（改动层 = `render/`，高风险，按 gpui-wr-engine §1.2 停下报告）：
- 落点文件（预期）：`render/context_misc_draw.go`（DrawShadow 算紧致界、按界建
  隔离层）、`render/context_layer_stack.go` / `filter_ops_gpu.go`（layer RT 范围与
  回读/回种按界）、`render/internal/gpu/filter_gpu_graph.go`（graph 接受子区域，
  若拍板方案需要）。**均不碰 `gpu/` 后端、不改公开 API**（预期零）。
- 上游 `ui/rendering`、`ui/scene` 调 DrawShadow 的语义不变（阴影像素逐位不变，
  双后端金图 0.0000% 锁定不重种即验收口径）。
- 下游 `gpu/hal` 接口不变。
- 双后端：native 已分段；go/GL 按 P0.4 口径无 surfaceless 独立设备，分段数据
  留 ④ 真窗 `ui_wr_p41_drawshadow`（`GPUI_BACKEND=go`）验证；metal 不可验（无真机）。
- 门禁：先红后绿（成本断言 + 像素逐位一致 + 缓存命中失效）、recording/render
  触及包全跑、race/vet/apidoc rc=0、显存账本关窗无泄漏。

## 5 状态

- ① 分段归因：✅ 完成（本文 + 两份原始输出）。
- ②③：**未动**，停在「停下报告」等待用户拍板裁剪方案。
- 临时诊断件（`render/p43_rough_probe_test.go`、两处 `p43_timing.go` helper、
  12 处计时点）保留到 ④ 做成本断言基线，验收完按「钩子用完即删」清理；
  分段计时默认关、零行为变化。
