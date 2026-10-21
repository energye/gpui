# ui_wr_r18_savelayer — R18 SaveLayer+预算

> 状态：**✅v2**（2026-10-21 模式 2 三证据重关；`-auto-only` 10s 两连跑 0 退出）

W2 R18 独立真窗：证明**组内半透明对走离屏 SaveLayer 合成，超预算拒批可观测**——
每帧两个 OnPaint 组请求 SaveLayer，`SaveLayerMaxOps=1` 帧预算下第一组允许
（savelayer_allow），第二组拒批（savelayer_reject），拒批在窗内红框可视。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# gate：10s，冻结 HOT/计数横幅/组 C，JSON + 退出码
go run ./examples/ui_wr_r18_savelayer -auto-only
RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer -auto-only

# manual：HOT 动画 + 计数横幅实时 + 5–8s Spike 组 C 出现
go run ./examples/ui_wr_r18_savelayer
go run ./examples/ui_wr_r18_savelayer -manual-seconds 10

# 首次播种 Golden（之后必须连跑两次 0.0000%）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer -auto-only
```

## Window / Close duration

1200×800 · gate 档 **10s**（`-auto-only` 或 `RUN_SECONDS=10`；U16 下限 5s，§2.5 本窗 10s）；manual 档
默认常驻，`-manual-seconds 10` 可走完 Steady→Spike→Recover。GPU 真窗必需。

## 能力与场景（U17）

| Region | Gate 档（10s，冻结） | Manual 档 |
|--------|---------------------|-----------|
| A 组 1（190×120，`SaveLayer(w,h,0.55)`） | **允许**：红块 0.85→0.514、蓝块 0.6 组内叠加→0.33 过 BG，离屏组透明可见 | 同左 |
| B 组 2（同帧第二个 `SaveLayer`） | **拒批**：绿块保持原始 0.35/0.65/0.55（无组透明）+ 6px 红框 | 同左 |
| C 组 3（Spike 相出现） | **隐藏**，保证 10s 快照确定性和 allow/reject=1/1 | 5–8s 出现，拒批累计 2/帧 |
| D 静态密集 4×4 色格 + 8 标签 | 静止 | 静止 |
| HOT 热点 | 冻结 `(0.95,0.30,0.25)` | 每帧变色 |
| `SL:` 计数横幅 | 冻结 `SL: gate frozen (HUD live)` | `allow=n reject=n` 实时累加 |
| HUD | 实时 fps/p95/allow/reject/spike | 同左 |

## 三证据（U21）

### 1) 逻辑 5/5

`savelayer_allow≥1`、`savelayer_reject≥1`、`savelayer_groups=2`、`presents>0`、
`paint_count>0`。实测两连（10s）：**allow=601/602、reject=601/602、presents=600/601、paint_count=599/600**。
manual 10s（不跑像素/Golden）实测 **allow=601、reject=781、presents=600**，Spike 5–8s 拒批 2/帧，exit 0。

### 2) 像素 10/10（GPU 抓屏）

| Probe | 位置（逻辑） | 期望 | 实测 |
|-------|--------------|------|------|
| `groupA_red_group_alpha` | A 组红块 (30,30) | (0.514,0.212,0.212) = 0.85×0.55 over BG | 同值 |
| `groupA_blue_group_alpha` | A 组蓝块 (140,50) | (0.159,0.232,0.376) = 0.6×0.55 过 BG | (0.157,0.224,0.376) |
| `groupB_green_raw` | B 组绿块 (30,30) | (0.35,0.65,0.55) = 无组透明直绘 | 同值 |
| `groupB_reject_frame` | B 组左红框 (3,60) | (1.0,0.25,0.25) 拒批可见 | 同值 |
| `groupC_absent_bg` | C 组原位置 (30,30) | Body BG (0.10,0.11,0.13) | 同值 |
| `dense_grid` | D 格 0,0 中心 | (0.25,0.40,0.62) | 同值 |
| `hot_frozen` | HOT 中心 | (0.95,0.30,0.25) | (0.949,0.298,0.251) |
| `body_bg` / `legend_bg` / `top_bg` | 三块面板 | 0.10/0.11/0.13、0.11/0.12/0.15、0.12/0.14/0.18 | 同值 |

A 组两个像素是「SaveLayer 真的在合成」的直接证据（不是仅看计数器）：红块从裸色
0.85 降到 0.514；B 组绿块保持裸色 0.35，说明拒批后**没有**偷偷套组透明。

### 3) Golden 821184 px 逐位

header 48px + legend 260×656 + body 904×656 = **821184 px**，RGBA 零容差；
播种后两连 **0.0000%**。Gate 档冻结 HOT/计数横幅/组 C，避免动画帧污染基线。

## Gates（`-auto-only` 必须全绿）

- 窗口打开且 ≥1 present；`present_policy=full_paint`。
- allow≥1、reject≥1、两组、paint_count>0。
- 像素 10/10 + Golden 0.0000%。
- `vsync_source=true`、`gpu_fallbacks=0`、`cpu_fallback_ops=0`。
- 观测：fps_interval **59.97**、p95 **17.17ms**、hitch **0**、gpu_ops **60603**。
  `fps_gate=off_correctness`、`slope_gate=off`（10s 正确性窗，不冒充长跑性能窗）。

## U20 实现点六维

| 维度 | 回答 |
|------|------|
| **正确性** | 第一组 SaveLayer 必 allow、第二组必 reject：预算 MaxOps=1 帧复位 + LayerStats 原子计数；像素验证组透明/裸色差异 |
| **脏区** | 组 A/B 每帧 MarkNeedsPaint 重录（重请求 SaveLayer）；HOT live paint 独立；静态密集零重绘 |
| **缓存** | 预算/统计在 paint 线程注入（paintPresentTreeWithOpts），每帧 `budget.Reset()`；Retained 开关不影响计数 |
| **边界条件** | 无预算（其它窗默认）→ 全 allow、reject=0（既有 TestSaveLayer_* 覆盖）；拒绝不推层（depth 不变，既有单测） |
| **失败模式** | 任一侧为 0 → `FAIL: savelayer_allow=n want >=1`；像素/组透明不符 → 红；Goldens 漂移 → 红 |
| **窗内如何看出** | A 组半透明叠色、B 组红框「BUDGET REJECT」、HUD `SL: allow/reject` 实时行、Spike 相 reject 跳得快 |

## 诚实性声明

- allow/reject 来自引擎层 `SaveLayerStats`（paint_context.go 原子计数）→
  `PipelineApp.SaveLayerStats()` → 示例层直读，非示例自算。
- Gate 档冻结 HOT/横幅/组 C 是**为了确定性 Golden**，不是隐藏失败；HUD 仍实时，
  manual 档保留全部动画与 Spike 组 C。
- 每次 paint 重录都会重新调用 `SaveLayer`（OnPaint 每帧执行），非「跑一次记一次」。
- 预算仅在 `PipelineOptions.SaveLayerMaxOps>0` 时注入，其它窗零影响（全 allow 路径有
  单测 `TestSaveLayer_StatsAllowNoBudget`）。
- 能力层单测：`TestSaveLayer_StatsCountsAllowReject` + 既有
  `TestSaveLayer_BudgetRejectsExcessOps/Area` 等 6 个。
