# ui_wr_r2_paint — R2 局部 NeedsPaint

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# gate（相位冻结在 Steady，5s，JSON + 退出码）
go run ./examples/ui_wr_r2_paint -auto-only
RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint -auto-only

# manual（动画 Steady→Spike→Recover；关窗或 -manual-seconds 到期后出汇总）
go run ./examples/ui_wr_r2_paint
go run ./examples/ui_wr_r2_paint -manual-seconds 15

# 首次播种 Golden（之后必须连跑两次 0.0000% 才算过）
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint -auto-only

# 可选：采集 paint_visits（访问数，证明只走脏路径）
WR_VISITS=1 RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint -auto-only
```

## Window / Close duration

1200×800 · `-auto-only` 或 `RUN_SECONDS` 为门禁档（相位冻结，默认 5s，`<5 → FAIL (U16)`）；不带两者为常驻人工窗，`-manual-seconds N` 可定时关闭。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px（标题）→ 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r2_paint  R2 局部 NeedsPaint                      |
+----------+---------------------------------------------------+
| LEGEND   | hot      hot2                       (Body 区域)    |
| (260px)  | [RED块]  [ORANGE块]                                |
|          | 4x3 色块网格(90x90)    4x3 小色格(36x36)           |
|          | 12 静态标签                                          |
+--------------------------------------------------------------+
| HUD: R2 | phase | fps | p95 | policy | paint | presents | 门禁 |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 静态色块网格 | (304,80) 起，4 列 × 3 行，每块 90×90 间隔 110 | 12 个 RepaintBoundary 色块 | **整段运行永不闪烁**（静态，只录一次 Replay） |
| 2 | 静态小色格 | (744,460) 起，4 列 × 3 行，每块 36×36 间隔 46 | 12 个非 boundary 色块 | **永不闪烁**（普通节点也被局部脏区跳过） |
| 3 | 静态文字 | (304,440) 起，4 列 × 3 行 | `static label 0..11` | **永不闪烁** |
| 4 | 动态热点 hot | (784,80)，90×90 | 大红块 | **门禁档固定红色**；manual 下 Steady 红 → Spike 黄 → Recover 蓝，只有它闪 |
| 5 | 动态热点 hot2 | (904,360)，60×60 | 橙块 | **门禁档固定橙色**；manual 下仅 Spike 相位闪 |
| 6 | 相位节奏 | — | Steady 1.5s → Spike 2s → Recover | manual 可肉眼对照闪区；gate 冻结 Steady 以便 Golden 确定性 |
| 7 | HUD | (0,728) | fps/p95/paint/presents/skip + 门禁 | **每 ~0.1s 刷新**；HUD 不纳入 Golden 掩码 |

### 判断标准

- 静态块 1/2/3 全程不闪 = 局部 NeedsPaint 生效（脏区不扩散到兄弟/静态层）。
- 只有块 4/5 闪 = MarkNeedsPaint 只刷自己的 RepaintBoundary；gate 档 `hot` 每帧重录但颜色不变。
- `paint_count ≤ presents+20` = 无全树重复刷；门禁档实测 `paint_count=3`、`presents=300`。
- HUD 数值每 ~0.1s 变化 = 真窗指标刷新正常（非静态贴纸）。

## Gates（`-auto-only` 必须全绿）

- 窗口打开且 ≥1 present（GPU 真窗）。
- 逻辑 6/6：`paint_count>0`、`paint_count≤presents+20`、`presents>0`、静态 boundary=12、`boundary_skip>0`、`boundary_rerecord>0`。
- 像素 13/13：
  - `static_00/11/22/30`：四角静态色块中心色；
  - `dense_00/11/22`：非 boundary 小色格中心色；
  - `hot_red` / `hot2_orange`：门禁档固定热点色；
  - `label_ink`：静态文字笔画（非 Body 背景）；
  - `body_bg` / `legend_bg` / `top_bg`：三块面板底色。
- Golden：header 48px + legend 260×656 + body 904×656 = **821184 px**，逐位 RGBA 零容差；播种后连跑两次 **0.0000%**。
- §2.2 全族 A–J JSON 输出；`present_policy` 当前默认 `retained`（W6 默认切换），R2 只要求局部隔离，不强制 `full_paint`。
- `fps_gate=off_correctness`、`slope_gate=off`：5s 正确性窗，长度/斜率不在本窗判。

## 已知限界

- Golden 只盖 header/legend/body，HUD 的实时数字（fps/visits）不纳入；动画面相由 manual 模式人眼验收。
- 门禁档把相位冻结在 Steady；Steady→Spike→Recover 的动态像素不在本窗 Golden 内，避免动画抖动污染基线。
