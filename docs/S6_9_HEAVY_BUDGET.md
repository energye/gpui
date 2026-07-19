# S6.9 — 重场景分级预算与总回归

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.9 关闭**  
> 依赖：S6.0 冻结基线 `tmp/s6_present_baseline.json` + S6.1–S6.8 优化  
> 架构：present-only 计时；**禁止** ReadPixels 墙钟充当 60fps

---

## 1. 目标

1. 为 **U05 / layer-blend / path / nested clip-layer** 建立 **分级预算**（**非**一律 16.7ms）  
2. 相对 **S6.0 冻结 p50** 要求可解释改善（P2）或防回归（P3）  
3. 锁存 JSON + 门禁测试；P0 主路径 **硬** ≤16.7ms 不回退  
4. 不降像素语义、无 silent CPU、`GPUOps>0`

**非目标**：控件层；把 layer 整段搬 GPU；Skia 绝对 FPS 对照（S6.10 可选附录）。

---

## 2. 分级策略

| 级别 | 代表场景 | 预算策略 | 60fps 宣称 |
|------|----------|----------|------------|
| **floor** | P01 | p50 ≤ **16.7ms** | 地板参考 |
| **P0_main** | U01–U04, B15like | p50 ≤ **16.7ms** 硬门禁 | **是**（主路径） |
| **P1_density** | H01 全量重绘反模式, H04 文本, H05 图块 | abs 帽 + **相对 S6.0 不恶化 >10%**；H04/H05 abs=16.7；H01 abs=**33.4**（2 帧） | H04/H05 可；H01 为反模式对照 |
| **P2_heavy** | U05, H02 layer×blend, H03 path stroke | **≤ abs 帽** 且 **≤ S6.0** 且 **≤ S6.0×factor**（默认 factor=0.92，须 ≥8% 改善；H03 factor=0.70） | **否**（重特效应力） |
| **P3_stress** | H06 nested clip×layer×text | 防回归：≤ S6.0×**1.15** + abs 帽 | **否** |

### 本机绝对帽（ms，含测量余量）

| 场景 | abs cap |
|------|---------|
| U05_KitchenSinkStress | 145 |
| H02_LayerBlendStack | 210 |
| H03_PathStrokeCloud | 90 |
| H06_NestedClipLayerText | 150 |
| H01_FullRedrawShell | 33.4 |
| H04_TextRows40 / H05_ImageTileGrid | 16.7 |

> 绝对帽来自 S6.9 实测 + 余量；**改场景绘制内容必须 bump** `s6.0-present-1` / `s6.9-heavy-budget-1`。

---

## 3. 本机对照表（S6.0 → S6.9）

环境：`WGPU_NATIVE_PATH=.../libwgpu_native.so`，warmup=3，iters=6，present-only offscreen。  
JSON：`tmp/s6_9_heavy_budget.json`（version `s6.9-heavy-budget-1`）。

| 场景 | tier | S6.0 p50 | S6.9 p50 | ratio | budget 要点 | 状态 |
|------|------|----------|----------|-------|-------------|------|
| P01_SolidPresent | floor | 3.08 | ~2.5 | ~0.81 | ≤16.7 | ✅ |
| U01_StaticShell | P0_main | 7.40 | ~0.2 | ~0.03 | ≤16.7 | ✅ |
| U02_ListScrollMorph | P0_main | 5.10 | ~0.6 | ~0.12 | ≤16.7 | ✅ |
| U03_FormFieldDamage | P0_main | 2.55 | ~0.2 | ~0.09 | ≤16.7 | ✅ |
| U04_ModalStatic | P0_main | 6.39 | ~0.7 | ~0.11 | ≤16.7 | ✅ |
| B15like_MultiDamage | P0_main | 2.10 | ~0.2 | ~0.08 | ≤16.7 | ✅ |
| H01_FullRedrawShell | P1_density | 21.61 | ~0.4 | ~0.02 | ≤33.4 & ≤S6.0×1.1 | ✅ |
| H04_TextRows40 | P1_density | 13.27 | ~2.2 | ~0.17 | ≤16.7 | ✅ |
| H05_ImageTileGrid | P1_density | 12.89 | ~0.2 | ~0.01 | ≤16.7 | ✅ |
| U05_KitchenSinkStress | P2_heavy | 160.43 | ~127 | ~0.79 | ≤145 & ≤S6.0×0.92 | ✅ |
| H02_LayerBlendStack | P2_heavy | 223.65 | ~181 | ~0.81 | ≤210 & ≤S6.0×0.92 | ✅ |
| H03_PathStrokeCloud | P2_heavy | 131.38 | ~66 | ~0.50 | ≤90 & ≤S6.0×0.70 | ✅ |
| H06_NestedClipLayerText | P3_stress | 124.57 | ~86 | ~0.69 | ≤150 & ≤S6.0×1.15 | ✅ |

**解读（可解释下降）**

| 热点 | 相对 S6.0 | 主要贡献切片 |
|------|-----------|--------------|
| P0 / P1 主路径与密度 | **数倍～数十倍** | S6.1 帧模型、S6.2 提交、S6.3 batch、S6.5/S6.7 文本图资源 |
| H03 path/stroke | **~50%** | S6.6 tess/cache/zero-copy |
| U05 / H02 layer | **~19–21%** | S6.4 pixmap/filter 池；层合成仍偏 CPU，故 **不宣称 60fps** |
| H06 nested | **~31%** | 池化 + 提交路径；P3 仅防回归 |

---

## 4. 测试与产物

| 项 | 路径 |
|----|------|
| 分级门禁 | `TestS69_HeavyBudget_TierGates` |
| JSON 契约 | `TestS69_Contract_FromJSON` |
| L0 主路径 | `TestS69_L0_MainPathStillGreen` |
| 输出 JSON | `tmp/s6_9_heavy_budget.json` |
| S6.0 对照 | `tmp/s6_present_baseline.json`（只读） |

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

S6_PERF_WARMUP=3 S6_PERF_ITERS=6 \
  go test -count=1 ./render -run 'TestS69_' -timeout 300s

# L0 + 抽样正确性（每切片）
go test -count=1 ./render -run 'TestS6_L0_|TestS69_L0_|TestS61_|TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

Env：

| 变量 | 含义 |
|------|------|
| `S6_9_JSON` | 输出路径（默认 `tmp/s6_9_heavy_budget.json`） |
| `S6_9_REGRESS_ONLY=1` | P2 仅要求 ≤abs 且不差于 S6.0（跳过 must-improve） |
| `S6_PERF_WARMUP` / `S6_PERF_ITERS` | 与 S6.0 相同语义 |

---

## 5. 不变量（准确性）

1. 计时路径 = draw + `PresentFrame*`（**无** ReadPixels）  
2. 全部场景 `GPUOps>0` 且 `cpu_fallback_ops=0`  
3. P0 **永不**用重场景预算放宽  
4. 禁止靠减场景内容 / 关 AA / silent CPU 刷分  
5. 改冻结场景绘制或预算常数 → **bump version** 并改本文表格  

---

## 6. 退出条件

| 条件 | 状态 |
|------|------|
| 分级预算文档 | ✅ 本文 |
| `TestS69_*` 绿 + JSON | ✅ |
| P0 ≤16.7 且显著优于 S6.0 | ✅ |
| P2 相对 S6.0 下降并可解释 | ✅（layer ~20%，path ~50%） |
| P3 未恶化超 15% | ✅ |
| 无 silent CPU / 无降语义 | ✅ |
| L2 全量 Comp 分片 201 | ✅ `total=201 fail=0` |

**S6.9 关闭。** 下一：S6 总关闭清单（L2 全量 Comp 等）或可选 **S6.10 Skia FPS 附录**。
