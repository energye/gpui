# ui_wr_c1_boundary_nest — C1 组合窗（R2+R3+R3b+R12b 集成）

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_c1_boundary_nest                     # 默认无限运行，手动关窗退出
RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest      # 10s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_c1_boundary_nest  C1 组合窗（R2+R3+R3b+R12b）      |
+----------+---------------------------------------------------+
| LEGEND   | L1(280)                S 兄弟(160)   4x4 色格       |
| (260px)  |  └L2(220)              └hotS 绿块     36x36x16     |
|          |   └L3(160)             └"sib" 标签   标签列6个      |
|          |    └L4(100)             └色条8个                   |
|          |      └hot 红块(50)                                  |
|          | 各层标签 L1..L4 + L1 静态块x3                       |
+----------+---------------------------------------------------+
| HUD: C1 | phase | fps | p95 | skip | rr | draws | 门禁        |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | L1 4 层嵌套 | (304,80) 起：L1 280×280 → L2 220×220 → L3 160×160 → L4 100×100 | 4 层 RepaintBoundary；每层标签 L1–L4；L1 内 3 个静态块；L2 内静态块 | 各层独立缓存；**内脏不动时外不闪** |
| 2 | L4 内 hot 红块 | (419,195)，50×50（L1@(304,80) → L2@(334,110) → L3@(364,140) → L4@(394,170) → hot@+25） | 每 tick 变红深浅 | **Steady 相位闪**（magenta 只覆盖 L4 区域 = 只内层 rerecord）；L1–L3 不闪 |
| 3 | L1 外层背景 | (304,80) 280×280 | 深蓝紫背景 | **Spike 相位闪烁切换**（只 L1 层 rerecord 整 nest）；**S 兄弟 nest 不闪** |
| 4 | S 兄弟嵌套 | (624,80) 起 160×160，hotS 绿块 @(684,140) + "sib" 标签 | 独立第二 nest | **全程静**：任何相位不闪（跨 nest 隔离） |
| 5 | 静态 4×4 色格 | (824,80) 起，36×36×16 | 非 boundary 密集色格 | 永不闪 |
| 6 | 标签列 + 色条 | (824,290) 起 6 标签；x 从 824 起 8 色条 (40×26) | 混合静态内容 | 永不闪 |
| 7 | 相位行为 | — | Steady 2s → Spike 3s → Recover | Steady：hot+hotS 闪；Spike：L1 背景闪 + hot/hotS 静；Recover：全窗无闪 |
| 8 | HUD | (0,728) | fps/p95/skip/rr/draws + 门禁 | **每 ~0.1s 刷新**：fps≈59.9、skip>1500、rr>400、draws 持续累加（万级） |

### 判断标准

- 只内层闪、外层不闪 = 内层局部 rerecord（R3）。
- 只 L1 闪、S 不闪 = 跨 nest 隔离（R3/R3b）。
- magenta 叠加与脏区完全一致 = R12b debug repaint 在组合场景正确。
- 组合窗门禁同时要求 R3 + R3b + R12b 指标 = C1 不是单 R 的替身（各 R 有独立真窗）。

## Gates

- `boundary_skip ≥ 3`（干净层 Replay — R3）
- `boundary_rerecord ≥ 2`（脏层重录 — R3）
- `boundary_count ≥ 3` + `boundary_max_depth ≥ 2`（R3b 合成位发现）
- `debug_draws ≥ 1`（R12b 叠加在组合场景生效）
- 内脏→只内 rerecord、外脏→只外 rerecord（人眼 + skip/rr 数字对照）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
