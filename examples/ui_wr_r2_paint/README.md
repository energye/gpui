# ui_wr_r2_paint — R2 局部 NeedsPaint

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r2_paint                       # 默认无限运行，手动关窗退出
RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint         # 5s 后自动关闭并输出门禁 JSON
# 可选：WR_VISITS=1 采集 paint_visits（访问数，证明只走脏路径）
WR_VISITS=1 RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

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
+----------+---------------------------------------------------+
| HUD: R2 | phase | fps | p95 | policy | paint | presents | 门禁 |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 静态色块网格 | (304,80) 起，4 列 × 3 行，每块 90×90 间隔 110 | 12 个 RepaintBoundary 色块（青/紫混色） | **整段运行永不闪烁**（静态，只录一次 Replay） |
| 2 | 静态小色格 | (744,460) 起，4 列 × 3 行，每块 36×36 间隔 46 | 12 个非 boundary 色块 | **永不闪烁**（普通节点也被局部脏区跳过） |
| 3 | 静态文字 | (304,440) 起，4 列 × 3 行 | `static label 0..11` | **永不闪烁** |
| 4 | 动态热点 hot | (784,80)，90×90 | 大红块 | **每 tick 变色**：Steady 红 → Spike 黄 → Recover 蓝；**全窗口只有它（及其下热点）闪** |
| 5 | 动态热点 hot2 | (904,360)，60×60 | 橙块 | **仅 Spike 相位闪**；其余相位静止不闪 |
| 6 | 相位节奏 | — | Steady 1.5s → Spike 2s → Recover | hot 色随相位跳变，可肉眼对照闪区位置 |
| 7 | HUD | (0,728) | fps/p95/paint/presents/skip + 门禁 | **每 ~0.1s 刷新**：fps≈59.9、paint≈presents、skip 持续累加（4000+）、visits（WR_VISITS=1 时）为几十量级 |

### 判断标准

- 静态块 1/2/3 全程不闪 = 局部 NeedsPaint 生效（脏区不扩散到兄弟/静态层）。
- 只有块 4/5 闪 = MarkNeedsPaint 只刷自己的 RepaintBoundary。
- HUD `paint_count ≤ presents+20` = 无全树重复刷。
- HUD 数值每 ~0.1s 变化 = 真窗指标刷新正常（非静态贴纸）。

## Gates

- 窗口打开且 ≥1 present（GPU 真窗）
- `paint_count > 0` 且 `paint_count ≤ presents+20`（静态 boundary 未被重复刷全树）
- `paint_visits`（WR_VISITS=1 采样）远小于全节点数 → 证明只走脏路径
- §2.2 全族 A–J JSON 输出（schema 校验通过）
