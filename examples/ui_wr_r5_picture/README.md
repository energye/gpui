# ui_wr_r5_picture — R5 Picture 录/回放

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r5_picture                   # 默认无限运行，手动关窗退出
RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture     # 5s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r5_picture  R5 Picture 录/回放                     |
+----------+---------------------------------------------------+
| LEGEND   | DIRECT 直绘区           REPLAY 回放区   4x3 色格    |
| (260px)  | (304,80) 200x300       (544,80) 200x300  40x40     |
|          | 三角填充(红)             同构图(逐像素一致)          |
|          | 描边矩形(蓝)                                         |
|          | 填充矩形(绿) + 描边矩形(黄)                          |
|          | 文字 PIC-REPLAY / v2                                 |
+----------+---------------------------------------------------+
| HUD: R5 | phase | fps | p95 | picture_ops | presents | 门禁    |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | DIRECT 直绘区 | (304,80)，200×300 | 每帧直绘：红三角（FillPath）+ 蓝描边矩形（StrokePath 3px）+ 绿填充矩形 + 黄描边矩形 + 白字 `PIC-REPLAY`/`v2` | 始终完整显示 6 个图形元素，与回放区**逐像素一致** |
| 2 | REPLAY 回放区 | (544,80)，200×300 | 从保留 Picture（`scene.RecordPicture`，op=6）每帧 `Replay` | 与左区**逐像素一致**（几何/颜色完全相同） |
| 3 | 区标签 | (304,390) / (544,390) | `DIRECT (直绘)` / `REPLAY (回放)` 白字 | 常驻 |
| 4 | 静态 4×3 色格 | (804,80) 起，40×40×12 | 非 picture 密集色格 | 永不闪 |
| 5 | 相位行为 | — | Steady 2s / Spike 2s / Recover | Steady+Spike 每帧重绘两区；Recover 纯 Replay（无闪） |
| 6 | HUD | (0,728) | fps/p95/picture_ops/presents + 门禁 | **每 ~0.1s 刷新**：fps≈59.9、picture_ops=6 常驻（= 录制的 op 数） |

### 判断标准

- 左右两区图像完全一致 = 回放等价（记录/回放无丢失、无错序）。
- 静态色格不闪 + Recover 相位无闪 = Picture 保留在层树内只回放不重录。
- HUD picture_ops=6 = op 计数诚实（2 path + 2 rect + 2 text）。

## Gates

- `picture_op_count ≥ 3`（实测 6：2×FillPath/StrokePath + 2×FillRect/StrokeRect + 2×DrawString）
- 回放像素 ≡ 直绘（同几何同色，人眼对照）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
