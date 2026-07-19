# S4.0 性能基线（measure-only）

> 版本：1.1 | 日期：2026-07-15（文档归档 2026-07-19）  
> 状态：**S4.0 已关闭**（只测量、不改算法）；**harness 已移除**  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 原始 JSON：`tmp/s4_baseline.json`（历史产物）  
> 后续回归：使用 `TestS6_PresentBaseline_Scenes` / `TestS6_RegressionLock_Contract`（`render/s6_perf_baseline_test.go`）

---

## 1. 目的与非目标

### 目的

在 **P1 / 阶段 A** 代表性压力场景上建立可回归的 wall-time 基线，供 S4.1+ 优化切片对比：

| 指标 | 来源 | 说明 |
|------|------|------|
| total_ms / fps_est | harness wall clock | draw + 最终 `FlushGPU` |
| draw_ms / flush_ms | harness 分段计时 | draw 含场景内中间 flush（如 backdrop） |
| `gpu_ops` / `cpu_fallback_ops` | `RenderPathStats` | 最后一次测量帧 |
| upload / draw 计数 | **N/A** | 当前 render 层无一等计数器 |

### 非目标

- 不改 batch / atlas / cache / damage 算法  
- 不与 Skia 绝对 FPS 对标（后置）  
- 不把性能数字作为阶段 A 关闭条件（A 已关）  
- 不实现控件层

---

## 2. 机器与环境

| 项 | 值 |
|----|-----|
| Host | `yanghy-pc` |
| OS | Linux 6.8.0-124-generic（Ubuntu 22.04），x86_64 / amd64 |
| CPU | 4 logical CPUs |
| GPU | Intel HD Graphics 520 (`0x8086:0x1916`) + NVIDIA GeForce 940MX (`0x10de:0x134b`) |
| NVIDIA driver | 580.159.03 |
| `WGPU_NATIVE_PATH` | `/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so` |
| Go | module `github.com/energye/gpui`（go 1.25） |
| Date (RFC3339) | `2026-07-15T19:44:46+08:00` |
| Warmup / Iters | **3 / 20**（每场景；可用 `S4_PERF_WARMUP` / `S4_PERF_ITERS` 覆盖） |
| Frame model | **每个测量帧新建 `render.Context`**（避免 retained 摊销；含读回路径） |

### 复现命令

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH
# optional: S6_PERF_WARMUP=3 S6_PERF_ITERS=10 S6_PERF_JSON=tmp/s6_present_baseline.json
# Note: original S4 harness removed; use S6 present baseline for live measurement.

go test -count=1 ./render -run 'TestS6_PresentBaseline_Scenes' -timeout 10m -v
```

硬门禁：真 native 库；每帧 `GPUOps > 0`；`cpu_fallback_ops` 记入表（本轮全部为 0）。

---

## 3. 场景表（B01–B15）

| ID | 场景 | 尺寸 | 对应 / 意图 |
|----|------|------|-------------|
| B01 | SolidFill | 800×600 | 地板：全屏填充 + flush/readback |
| B02 | ManyRects200 | 800×600 | 同类 draw 批压（S4.1 输入） |
| B03 | TextRows40 | 640×480 | 文本/glyph 压（S4.2 输入） |
| B04 | D05_LayerBlendClip | 280×200 | A 组合：layer × blend × clip |
| B05 | D06_ImageTextClipBackdrop | 360×240 | A 组合：image × text × clip × backdrop |
| B06 | D08_MultiRegionRedraw | 320×200 | 多区域脏语义预热（S4.4 输入） |
| B07 | D36_KitchenSinkMaxMix | 560×400 | A kitchen-sink 最大混合 |
| B08 | ListScrollMorphology | 480×640 | 列表形态：clip + 行 + 圆头像 |
| B09 | BlendStackPressure | 400×400 | 多 blend mode 叠压 |
| B10 | ImageTileGrid | 512×512 | 图像/clip 网格（S4.3 输入） |
| B11 | StressNestedClipLayerText | 640×480 | 嵌套 clip/layer/text 应力 |
| B12 | PathStrokeDashCloud | 480×360 | path stroke + dash 云（S4.3 输入） |
| B13 | ImageBatchNoClip | 512×512 | **S4.1** 同纹理无 clip 批压（64 tiles） |
| B14 | RetainedPathText | 480×360 | **S4.4** retained Context：path + text |
| B15 | RetainedMultiDamage | 320×200 | **S4.4** retained + multi-region damage |

---

## 4. 基线数字（warmup=3, iters=20）

单位：毫秒。`fps_est = 1000 / total_ms_avg`（单线程重建帧估算，**非** 窗口 present 帧率）。

| 场景 | avg_ms | p50_ms | p95_ms | min | max | draw_ms | flush_ms | fps_est | gpu_ops | cpu_fb |
|------|--------|--------|--------|-----|-----|---------|----------|---------|---------|--------|
| B01_SolidFill | 53.73 | 49.05 | 75.10 | 43.16 | 88.07 | 1.58 | 52.15 | 18.6 | 2 | 0 |
| B02_ManyRects200 | 55.80 | 44.60 | 97.81 | 34.40 | 107.23 | 1.98 | 53.83 | 17.9 | 201 | 0 |
| B03_TextRows40 | 174.99 | 171.09 | 208.94 | 159.02 | 226.76 | 6.00 | 169.00 | 5.7 | 42 | 0 |
| B04_D05_LayerBlendClip | 26.76 | 26.24 | 29.29 | 24.37 | 31.04 | 23.91 | 2.85 | 37.4 | 4 | 0 |
| B05_D06_ImageTextClipBackdrop | 278.24 | 278.10 | 289.88 | 264.53 | 292.31 | 271.53 | 6.70 | 3.6 | 13 | 0 |
| B06_D08_MultiRegionRedraw | 142.53 | 142.08 | 146.23 | 137.06 | 156.55 | 138.73 | 3.80 | 7.0 | 11 | 0 |
| B07_D36_KitchenSinkMaxMix | 349.57 | 348.05 | 365.00 | 337.69 | 365.18 | 335.07 | 14.51 | 2.9 | 18 | 0 |
| B08_ListScrollMorphology | 163.55 | 163.31 | 173.42 | 153.01 | 181.92 | 3.13 | 160.42 | 6.1 | 75 | 0 |
| B09_BlendStackPressure | 365.97 | 365.09 | 386.16 | 344.00 | 392.33 | 360.45 | 5.52 | 2.7 | 38 | 0 |
| B10_ImageTileGrid | 152.05 | 150.69 | 159.57 | 140.59 | 166.32 | 0.84 | 151.21 | 6.6 | 129 | 0 |
| B11_StressNestedClipLayerText | 453.53 | 450.74 | 473.76 | 432.43 | 479.95 | 445.56 | 7.97 | 2.2 | 18 | 0 |
| B12_PathStrokeDashCloud | 254.36 | 251.76 | 281.86 | 237.00 | 291.32 | 254.36 | 0.00 | 3.9 | 42 | 0 |

全部场景 **`cpu_fallback_ops = 0`**，**`gpu_ops > 0`**。

---

## 5. 观察（供 S4.1+ 选型，非优化承诺）

1. **读回/flush 主导**的场景：B01/B02/B03/B08/B10 — `flush_ms` 远大于 `draw_ms`；后续若加 present 无读回路径，应另记一列 `present_ms`。  
2. **CPU/同步路径主导**的场景：B05/B06/B07/B09/B11/B12 — `draw_ms` 主导（含中间 `FlushGPU`、layer/backdrop、blend、path）。  
3. **S4.1 batch** 优先对比：**B02**（200 rect）、**B08/B10**（高 `gpu_ops`）。  
4. **S4.2 glyph/atlas** 优先对比：**B03**、**B08**、kitchen-sink 文本段。  
5. **S4.3 path/texture cache** 优先对比：**B12**、**B10**、**B07**。  
6. **S4.4 damage/retained** 对比：**B06** + **B14/B15**（`Retained=true`，复用 Context）。  
7. **upload/draw 计数仍 N/A** — S4.1 前可按需给 accelerator 加轻量计数器，但不阻塞 S4.0 关闭。

---

## 6. 回归约定

每个 S4.x 优化切片结束后：

1. 重跑本 harness（同 warmup/iters），更新对比表或附录  
2. 回归：`TestS3*` / `TestP1_*` / `TestP1_Comp_*` 像素与 `GPUOps>0`  
3. 禁止 silent CPU 冒充 GPU 加速  
4. 不得删减阶段 A 组合断言

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestS6_PresentBaseline_Scenes|TestS3|TestP1_' -timeout 180s
```

---

## 7. 退出条件检查

| 条件 | 状态 |
|------|------|
| 只测量、不改算法 | ✅ |
| P1/A 场景 wall-time + path stats | ✅ B01–B15 |
| 产出本文 + `tmp/s4_baseline.json` | ✅ |
| 真 `WGPU_NATIVE_PATH` + `GPUOps>0` | ✅ |
| upload/draw 可得则记 | ⚠️ N/A（书面记录） |

**S4.0 关闭。** 后续切片均已关闭：

- S4.1 `docs/S4_1_BATCH.md`
- S4.2 `docs/S4_2_GLYPH_ATLAS.md`
- S4.3 `docs/S4_3_PATH_TEXTURE_CACHE.md`
- S4.4 `docs/S4_4_DAMAGE_RETAINED.md`

**S4.x 全线关闭。**

---

## 8. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.1 | 增补 B13–B15（image batch / retained path-text / multi-damage）；S4.x 关闭交叉引用 |
| 2026-07-15 | 1.0 | 首版基线：12 场景 × 20 iters；Intel/NVIDIA 双 GPU 机器实测 |
