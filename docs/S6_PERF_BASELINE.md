# S6.0 — 深基线与回归锁

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.0 关闭**  
> 依赖：S5 关闭（`docs/S5_*.md`）+ MAINLINE S6 定义  
> 架构：`render.PresentFrame*` → webgpu → rwgpu → libwgpu_native

---

## 1. 目标

1. **冻结** S6 性能场景集（名称/内容/计量方式）。  
2. 锁定 warmup/iters 与主路径预算。  
3. 建立 **present-only 主基线** + **读回对照样例**（双轨，禁止混用）。  
4. 固化 **L0–L4 回归契约** 与主路径回退阈值。  
5. **L2 全量 Comp** 在本切片锁存一次。

**本切片只测量、不改算法。**

---

## 2. 锁定参数

| 参数 | 值 | Env 覆盖 |
|------|-----|----------|
| warmup | **3** | `S6_PERF_WARMUP` |
| iters | **10**（本机采集可用 8；文档默认 10） | `S6_PERF_ITERS` |
| 主路径预算 | **p50 ≤ 16.7ms** | `S6_MAIN_PATH_BUDGET` |
| 主路径回退阈值 | **p50 恶化 >10% 必须修/书面说明** | `S6_REGRESS_PCT` |
| 主计量路径 | present-only（无 ReadPixels） | — |
| JSON | `tmp/s6_present_baseline.json` | `S6_PERF_JSON` |
| baseline version | `s6.0-present-1` | 改场景名必须 bump |

---

## 3. 冻结场景（13）

| 场景 | Tier | 意图 |
|------|------|------|
| P01_SolidPresent | floor | 地板 |
| U01–U04 | **P0_main** | S5 主路径 60fps；**禁止回退** |
| B15like_MultiDamage | P0_main | 多 region damage |
| U05_KitchenSinkStress | P2_heavy | layer 应力 |
| H01_FullRedrawShell | P1_density | 全量重绘反模式（对照 U01 脏区） |
| H02_LayerBlendStack | P2_heavy | layer×blend |
| H03_PathStrokeCloud | P2_heavy | path stroke/dash |
| H04_TextRows40 | P1_density | 文本行压 |
| H05_ImageTileGrid | P1_density | 图块密度 |
| H06_NestedClipLayerText | P3_stress | 嵌套 clip×layer×text |

**禁止**在未 bump `version` 的情况下改场景绘制内容或删场景。

---

## 4. 本机 present-only 基线

主机/环境见 JSON：`hostname=yanghy-pc`，`wgpu=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so`。  
采集：warmup=3, iters=8。

| 场景 | tier | 尺寸 | p50_ms | avg_ms | fps_p50 | gpu | cpu_fb | retained | damage |
|------|------|------|--------|--------|---------|-----|--------|----------|--------|
| B15like_MultiDamage | P0_main | 320×200 | 2.10 | 2.17 | 476.7 | 4 | 0 | True | True |
| H01_FullRedrawShell | P1_density | 800×480 | 21.61 | 21.92 | 46.3 | 19 | 0 | True | False |
| H02_LayerBlendStack | P2_heavy | 480×360 | 223.65 | 225.98 | 4.5 | 13 | 0 | True | False |
| H03_PathStrokeCloud | P2_heavy | 480×360 | 131.38 | 131.68 | 7.6 | 25 | 0 | True | False |
| H04_TextRows40 | P1_density | 640×480 | 13.27 | 13.84 | 75.4 | 41 | 0 | True | False |
| H05_ImageTileGrid | P1_density | 512×512 | 12.89 | 12.93 | 77.6 | 65 | 0 | True | False |
| H06_NestedClipLayerText | P3_stress | 560×400 | 124.57 | 126.15 | 8.0 | 9 | 0 | True | False |
| P01_SolidPresent | floor | 640×400 | 3.08 | 3.17 | 324.9 | 1 | 0 | False | False |
| U01_StaticShell | P0_main | 800×480 | 7.40 | 7.89 | 135.1 | 3 | 0 | True | True |
| U02_ListScrollMorph | P0_main | 400×560 | 5.10 | 5.57 | 196.0 | 9 | 0 | True | True |
| U03_FormFieldDamage | P0_main | 400×300 | 2.55 | 2.69 | 391.6 | 3 | 0 | True | True |
| U04_ModalStatic | P0_main | 480×320 | 6.39 | 7.01 | 156.5 | 6 | 0 | True | False |
| U05_KitchenSinkStress | P2_heavy | 480×320 | 160.43 | 162.01 | 6.2 | 18 | 0 | True | False |

**不变量（已由 `TestS6_RegressionLock_Contract` 强制）**：

- 全部 `gpu_ops > 0`  
- 全部 `cpu_fallback_ops = 0`  
- U01–U04 `p50 ≤ 16.7`  

### 双轨对照（诊断 only）

| 项 | ms |
|----|-----|
| present-only p50（小 solid） | 2.45 |
| FlushGPU 读回 p50 | 4.24 |

> 读回 **不得** 用于 60fps 宣称（S5/S6 硬规则）。

---

## 5. 回归契约（准确性）

| 层级 | 套件 | S6.0 | 后续 S6.x 每切片 |
|------|------|------|------------------|
| **L0** | `TestS6_L0_MainPathStillGreen` + `TestS52_*` / `TestS53_*` | ✅ | 强制 |
| **L1** | `TestS3*` + Comp 抽样 D01/D06/D08/D36/D63/D152 | ✅ 本切片 | 强制 |
| **L2** | 全量 `TestP1_Comp_` D01–D200 | ✅ **锁存** | 关闭 S6 再全量；中期可抽样 |
| **L3** | `TestS6_PresentBaseline_Scenes` 前后对比 | ✅ 建基线 | 强制（相对本 JSON） |
| **L4** | 真窗口 present | ⬜ S6.8 | S6.8 |

### 准确性规则

1. 像素/结构断言不得删减；失败即切片失败。  
2. 性能用 **p50**；固定 warmup/iters。  
3. 主路径 p50 相对本基线恶化 **>10%** → 必须修复或书面豁免。  
4. 禁止靠减场景内容、关 AA、silent CPU 刷分。  
5. `S5_ALLOW_SLOW` / 过载软过 **不得** 作为关闭常规手段。

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

# L3 基线 + 锁
S6_PERF_WARMUP=3 S6_PERF_ITERS=10 go test -count=1 ./render -run 'TestS6_' -timeout 400s

# L0/L1 抽样
go test -count=1 ./render -run 'TestS6_L0_|TestS52_|TestS53_|TestS3|TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s

# L2 全量（S6.0 / S6 关闭）
go test -count=1 ./render -run 'TestP1_Comp_' -timeout 600s
```

---

## 6. 给后续切片的优化提示（非承诺）

| 热点（本基线） | p50 量级 | 优先切片 |
|----------------|----------|----------|
| H02 LayerBlendStack | ~220ms | S6.4 |
| U05 KitchenSink | ~170ms | S6.4 |
| H03 PathStrokeCloud | ~130ms | S6.6 |
| H06 NestedClipLayer | ~125ms | S6.1/S6.4 |
| H01 FullRedrawShell | ~34ms | S6.1（对照 U01~6ms） |
| H04/H05 text/image | ~12–14ms | S6.3/S6.5/S6.7 |

U01 脏区壳 vs H01 全量重绘：同形态数量级差 **数倍** → 帧模型强制（S6.1）性价比最高。

---

## 7. 退出条件

| 条件 | 状态 |
|------|------|
| 场景冻结 + version | ✅ |
| present-only JSON | ✅ `tmp/s6_present_baseline.json` |
| 读回对照样例 | ✅ |
| 回归契约文档 | ✅ 本文 |
| U01–U04 预算锁 | ✅ |
| `TestS6_*` 绿 | ✅ |
| L2 全量 Comp | ✅ 分片全绿（D0/D1[0-4]/D1[5-9]/D2，`TestP1_Comp_`） |
| 只测不改算法 | ✅ |

**S6.0 关闭。** 下一焦点：**S6.1 帧模型强制**。

## 8. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 13 场景深基线 + 回归锁 + 双轨对照 |
