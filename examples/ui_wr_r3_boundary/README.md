# ui_wr_r3_boundary — R3 Boundary 缓存

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r3_boundary                  # 默认无限运行，手动关窗退出
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary    # 10s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r3_boundary  R3 Boundary 缓存                      |
+----------+---------------------------------------------------+
| LEGEND   | nest1(240)  nest2(180)   deep 链(200) 4x4色格      |
| (260px)  |  └mid(180)  └inner2(100)  └d2(150)   40x40x16      |
|          |    └inner(120) └hot2      └d3(100)                 |
|          |      └hot红块     蓝块     |                        |
|          | 8 静态标签（左下）                                   |
+----------+---------------------------------------------------+
| HUD: R3 | phase | fps | p95 | skip | rr | presents | 门禁     |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | nest1 3 层嵌套 | (304,80) 起：outer 240×240 → mid 180×180 → inner 120×120 | 3 层 RepaintBoundary + 每层内色块/标签 + 内层 hot 红块 | 各层独立缓存：**内脏不动时外不闪**；hot 变色时仅 inner 层重录（magenta 只闪最内层区域） |
| 2 | nest1 内 hot 红块 | (394,170) 附近，60×60 | 每 tick 变色的红块 | **Steady 相位每 tick 闪**；仅 inner 层 rerecord，outer/mid 及所有兄弟不闪 |
| 3 | nest2 兄弟嵌套 | (584,80) 起，outer2 180×180 → inner2 100×100 → hot2 蓝块 | 独立第二 nest | **全程静**：任何相位（含 nest1 外脏 Spike）都不闪 = 隔离证明 |
| 4 | deep 深链 | (804,360) 起：deep 200×200 → d2 150×150 → d3 100×100 | 深度 3 链 + 标签 | 全程静；rerecord 从不涉及其内容；boundary_count/max_depth 贡献 |
| 5 | 静态 4×4 色格 | (804,80) 起，40×40×16 | 非 boundary 密集色格 | 永不闪 |
| 6 | 静态标签 8 个 | (304,360) 起 4 列 × 2 行 | `grid label 0..7` | 永不闪 |
| 7 | 相位行为 | — | Steady 2s（内层脏）→ Spike 3s（外层脏）→ Recover（无脏） | Steady：hot+hot2 闪；Spike：outer 背景闪烁切换、nest2/deep/色格全静；Recover：全窗口无闪 |
| 8 | HUD | (0,728) | fps/p95/skip/rr + 门禁 | **每 ~0.1s 刷新**：fps≈59.9；Steady/Recover 期 `skip` 快速累加（>1000）；脏帧期 `rr` 累加（>100） |

### 判断标准

- nest1 外脏（Spike）时 nest2/deep/色格不闪 = 跨 nest 缓存隔离（U21）。
- hot 闪但 outer/mid 不闪 = 内层局部 rerecord，非全 nest 重录。
- Recover 相位 skip 只增不降 = 纯 Replay 帧成立。

## Gates

- `boundary_skip ≥ 3`（干净层 Replay）
- `boundary_rerecord ≥ 2`（脏层重录）
- `present_policy = full_paint`（W1 波 FullPaint 主路径）
- §2.2 全族 A–J JSON 输出
