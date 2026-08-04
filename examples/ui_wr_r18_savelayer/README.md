# ui_wr_r18_savelayer — R18 SaveLayer+预算

W2 R18 独立真窗：证明**组内半透明对走离屏 SaveLayer 合成，超预算拒批可观测**——
每帧两个 OnPaint 组请求 SaveLayer，`SaveLayerMaxOps=1` 帧预算下第一组允许
（savelayer_allow），第二组拒批（savelayer_reject），拒批在窗内红框可视。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
```

## Window / Close duration

1200×800 · **10s**（§2 主表 + §2.5 L264）。`RUN_SECONDS<10 → FAIL`（U16）。GPU 真窗必需。

## 能力与场景（U17）

| Region | Expectation（人眼） |
|--------|---------------------|
| A 组 1（190×120，`SaveLayer(w,h,0.55)` 半透明组） | 每帧首个 SaveLayer → **允许**：两色块叠加处半透明混合（离屏合成可见）；角标绿 |
| B 组 2（190×120，同帧第二个 `SaveLayer`） | 预算 MaxOps=1 → **拒批**：无离屏合成，组内容直绘 + 红框「BUDGET REJECT」 |
| C 组 3（190×120，Spike 相出现） | Spike 期间再追加一个 SaveLayer 请求 → 拒批累计 2/帧 vs 允许 1/帧（HUD 数字跳得快） |
| D 静态密集 4×4 色格 + 8 标签 | 始终静止（U17 静态密集） |
| HOT 热点 | 每帧变色（U17 动态热点；live paint 非 boundary） |
| HUD 计数行 | `SL: allow=n reject=n (MaxOps=1)` 实时累加 + 相位行 |

相位脚本：**Steady**（0–5s：组 A 允许 + 组 B 拒批）→ **Spike**（5–8s：组 C 出现，
拒批 2/帧）→ **Recover**（≥8s：组 C 消失，回到 1/1）。

## Gates（§2 主表 + §2.2 全族 FAIL 线）

| Gate | 值 | 判定 |
|------|-----|------|
| savelayer_allow ≥ 1（§2 主表 + §2.6） | 每帧 1 → 10s ≈ 600 | **硬** |
| savelayer_reject ≥ 1（§2 主表 + §2.6） | 每帧 ≥1 → 10s ≈ 600+ | **硬** |
| 场景：两个 SaveLayer 一允许一拒批 | A/B 组每帧 | 硬（§2.6 L355） |
| presents ≥ 1 | ≥1 | 硬 |
| policy = full_paint | full_paint | 正确性窗（10s，不冒充 retained） |
| fps_interval ≥ 55（HOT 持续 tick） | ≥55 | 预算 AT_DEFAULT |
| p95 ≤ 22ms | ≤22 | 预算 AT_DEFAULT |
| vsync_source / fallback=0 | true / 0 | 诚实性 |
| slope_gate = off | — | **10s 正确性窗 §2.2.4 <15s 允许**（R4b/R5/R11 先例） |

## U20 实现点六维

| 维度 | 回答 |
|------|------|
| **正确性** | 第一组 SaveLayer 必 allow、第二组必 reject：预算 MaxOps=1 帧复位 + LayerStats 原子计数，门禁 allow/reject 各 ≥1 |
| **脏区** | 组 A/B 每帧 MarkNeedsPaint 重录（重请求 SaveLayer）；HOT live paint 独立；静态密集零重绘 |
| **缓存** | 预算/统计在 paint 线程注入（paintPresentTreeWithOpts），每帧 `budget.Reset()`；Retained 开关不影响计数 |
| **边界条件** | 无预算（其它窗默认）→ 全 allow、reject=0（既有 TestSaveLayer_* 覆盖）；拒绝不推层（depth 不变，既有单测） |
| **失败模式** | 任一侧为 0 → `FAIL: savelayer_allow=n want >=1` + exit 1；预算不 Reset → 10s 全 reject（Spike 可辨） |
| **窗内如何看出** | 组 B/C 红框 + HUD `SL: allow/reject` 实时行 + Spike 相 reject 跳动 |

## 诚实性声明

- allow/reject 来自引擎层 `SaveLayerStats`（paint_context.go 原子计数）→
  `PipelineApp.SaveLayerStats()` → 示例层直读，非示例自算。
- 每次 paint 重录都会重新调用 `SaveLayer`（OnPaint 每帧执行），非「跑一次记一次」。
- 预算仅在 `PipelineOptions.SaveLayerMaxOps>0` 时注入，其它窗零影响（全 allow 路径有
  单测 `TestSaveLayer_StatsAllowNoBudget`）。
- 10s 正确性窗：slope_gate=off（§2.2.4 <15s 允许）；fps/p95 预算未放宽。
- 能力层单测：`TestSaveLayer_StatsCountsAllowReject`（本次补）+ 既有
  `TestSaveLayer_BudgetRejectsExcessOps/Area` 等 6 个。
