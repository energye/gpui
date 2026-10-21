# ui_wr_c2_retained_scene — C2 组合真窗（R3+R4+R4b+R5 集成）

> 状态：**✅v2**（2026-10-21 模式 2 三证据重关；`-auto-only` 15s 两连跑 0 退出）

W2 组合窗：retained 整场景——多 boundary + Picture 回放 + 远离脏点 + damage present。
只做集成回归，**不替代** R3/R4/R4b/R5 单能力窗（U6）。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# gate：15s，冻结 Steady 相位，JSON + 退出码
go run ./examples/ui_wr_c2_retained_scene -auto-only
RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene -auto-only

# manual：跑完整 Steady→Spike→Recover（15s 覆盖三相位）
go run ./examples/ui_wr_c2_retained_scene
go run ./examples/ui_wr_c2_retained_scene -manual-seconds 15

# 首次播种 Golden（之后必须连跑两次 0.0000%）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene -auto-only
```

## Window / Close duration

- 客户区 **1200×800**（U15），标题 `gpui ui_wr_c2_retained_scene — R3+R4+R4b+R5 集成`
- gate 档 **15s**（`-auto-only` 或 `RUN_SECONDS=15`；U16 下限 5s，§2.5 本窗 15s）
- manual 档默认常驻，`-manual-seconds 15` 可走完三相位；关窗/到期后出汇总
- GPU 真窗；无 GPU/X11 → `FAIL: window open (needs_gpu_window)` + exit 1

## Visible effect

| Region | Gate 档（Steady 冻结） | Manual 档 |
|--------|------------------------|-----------|
| 左上 区 A（R3 嵌套） | outer 220 / mid 160 / inner 100 三层 boundary；inner 50×50 红块 `(0.95,0.20,0.20)` 固定，外层静 | inner 每帧变色（仅 inner 重录）；Spike 相 outer 背景翻转（整巢重录，巢外不变） |
| 中上 区 B（R4 静态阵） | 5×3 色格（70×70，全 boundary）+ 静态标签，全程静止 | 同左 |
| 右下 hotA（R4 动画区） | `.78,.82`，60×60 红块固定；**已移出 R5 回放区** | 随相位变色/微移（`.78,.82`→`.80,.84`） |
| 右上/左下 区 C（R4b 双热点） | hot1 `.90,.12` 蓝、hot2 `.12,.85` 绿，固定 | 每帧变脏 → 两独立 dirty layer id；Spike 加速变色/微移 |
| 中下 区 D（R5 Picture） | 左 DIRECT `.22,.55`、右 REPLAY `.50,.55`，190×190 并排**留缝不重叠**；直绘/回放同构图 8 ops | 0.5s 低频刷新；Spike 每帧刷新 |
| 顶部 TopBar / 底部 HUD | 静态标题；HUD 实时 fps/p95/policy/mode/skip/rr/sum/ids/ops/union | 同左，相位变色 |

> 布局修复（本次三证据发现）：旧 hotA `.60,.42` 被 R5 replay 框盖住；旧 direct `.34` / replay `.52` 两个 190px 框重叠约 61px，direct 的图片被 replay 覆盖。像素探针把这两个“看不见的洞”抓出来后，已把 hotA 移到 `.78,.82`、Picture 两框改为 `.22/.50` 留缝并排。

## Gates（`-auto-only` 必须全绿）

门禁 = ∪(各 R 门禁)（§3.1.1）：

| 来源 | Gate | 硬 FAIL 线 | 本次实测 |
|------|------|-----------|----------|
| R3 | `boundary_skip` | `>=3` | **36365** |
| R3 | `boundary_rerecord` | `>=2` | **217** |
| R4 | `present_policy` | `== retained` | **retained** |
| R4 | `damage_ratio_sum` | `<=0.35`（sum of rects） | **0.186875** |
| R4 | `present_mode` | 非 `full`/`idle` | **damage_multi** |
| R4b | `dirty_layer_id_max` | `>=2` | **16**（last frame 12 ids） |
| R4b | `damage_multi_frames` | `>=1` | **892** |
| R5 | `picture_op_count` | `>=3` | **8** |
| 持续 tick | `fps_interval` | `>=55` | **59.94** |
| 持续 tick | `interval_p95_ms` | `<=22` | **18.33ms** |
| 正确性 | 像素 14/14 + Picture 配对 2/2 + Golden | 全绿 | **全绿** |
| 诚实性 | `vsync_source` / fallback | true / 0 | **true / 0** |

`damage_ratio_avg=0.766`（union bbox）是脏层分散的几何观测，不是门禁；门禁只用
`damage_ratio_sum=0.187`（真实重绘像素）。15s 窗 `hitch_count=1` 为观测，不设硬门禁。

## 三证据（U21）

### 1) 逻辑 / 集成门禁

`presents=899`、`paint_count=3`（retained root 不重刷）、`boundary_count=28`、
`boundary_max_depth=3`、`skip=36365`、`rr=217`、`ids=12/16`、`multi=892`、
`ops=8`、`sum=0.187`、`mode=damage_multi`、`fps=59.94`、`p95=18.33ms`。

### 2) 像素 14/14（GPU 抓屏）

| Probe | 期望 | 实测 |
|-------|------|------|
| `r3_inner_hot` | (0.95,0.20,0.20) | 同值 |
| `r4_static_00/11/22` | (0.15,0.42,0.32)、(0.22,0.50,0.50)、(0.29,0.42,0.41) | 同值 |
| `r4_hotA` | (0.95,0.20,0.20) | 同值（修复后可见） |
| `r4b_hot1` | (0.20,0.50,0.95) | (0.200,0.502,0.949) |
| `r4b_hot2` | (0.25,0.80,0.40) | 同值 |
| `r5_direct_tri` / `r5_replay_tri` | (0.90,0.30,0.20) | 同值 |
| `r5_direct_img` / `r5_replay_img` | (0.20,0.749,0.349) | 同值（修复后 direct 不再被 replay 覆盖） |
| `body_bg` / `legend_bg` / `top_bg` | 三块面板底色 | 同值 |

### 3) Picture 配对 + Golden

- direct vs replay 同点配对 **2/2**（三角、图片，容差 0.02，实测 RGB 完全一致）。
- Golden header 48 + legend 260×656 + body 904×656 = **821184 px** 逐位零容差；
  播种后两连 **0.0000%**。Gate 档冻结 Steady，避免动画污染基线。

## 集成验证（§3.1.2 C2 行）

| 跨能力边界 | 本窗如何证明 |
|-----------|-------------|
| damage ratio ≪ 1 | retained 稳态 `sumRatio=0.187`；静态色格/嵌套层不参与损伤 |
| dirty_layer_id ≥ 2 | hot1+hot2 对角同帧脏 → 两独立 id；R4 动画区再贡献；末帧 12 ids |
| Picture 回放正确 | 直绘/回放同构图并排（8 ops），像素配对 2/2 + Golden |
| boundary skip > 0 | 区 A mid/outer + 区 B 色格每帧 skip（36365） |

**引擎洞记录（本窗暴露，已修 ui/rendering）**：`SubtreeNeedsPaint` 原穿透子 boundary，内层脏会误判所有嵌套祖先进 DirtyLayerIDs → 被迫 re-record（每帧 14 ids、RSS 暴涨）。修复 = `layerSubtreeNeedsPaint`（遇 boundary 停，BuildLayerTree 专用）；单测 `TestBuildLayerTree_NestedBoundaryInnerOnly` + ui 全树回归 + R3/R4b/R4 GPU 真窗回归全绿。详见 `docs/ENGINE_UI_WIDGET_RENDER.md` §10。
