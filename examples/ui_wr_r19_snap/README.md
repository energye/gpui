# ui_wr_r19_snap — R19 1px / 设备像素对齐

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r19_snap                    # 默认无限运行，手动关窗退出
RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap      # 5s 后自动关闭并输出门禁 JSON
# 严格网格断言（SnapCoord 结果必须落在 1/dpr 网格上）：
WR_SNAP_STRICT=1 RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r19_snap  R19 1px/设备像素对齐                     |
+----------+---------------------------------------------------+
| LEGEND   | SNAPPED 绿网格           RAW 红网格（对照）          |
| (260px)  | (304,80) 500x400        (844,80) 500x400           |
|          | 1px 发丝线 20px 间距      偏移 0.5px → 模糊/双像素    |
|          | SnapLine/SnapRect 对齐   不 Snap 对照               |
|          | 橙 1px 外边框                                       |
+----------+---------------------------------------------------+
| HUD: R19 | fps | p95 | dpr | snapped | presents | 门禁        |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | SNAPPED 绿网格 | (304,80)，500×400 | 20px 间距 1px 发丝线（SnapLineX/Y）+ 橙 1px 外边框（SnapRect） | **线清晰锐利**（落在设备像素网格）；DPR 变化（如拖动到 HiDPI 屏）时重新 Snap，依旧锐利 |
| 2 | RAW 红网格 | (844,80)，500×400 | 同构图但坐标偏移 0.5px（不 Snap） | **对照：发丝线可能模糊/双像素/粗细不均**（尤其 DPR>1 时） |
| 3 | 区标签 | (304,500) / (844,500) | `SNAPPED (crisp)` / `RAW (may blur)` | 常驻 |
| 4 | DPR 变化 | 拖窗到其他显示器/缩放 | — | 网格自动重绘且仍对齐（DPR 改变触发 MarkNeedsPaint） |
| 5 | HUD | (0,728) | fps/p95/dpr/snapped + 门禁 | **每 ~0.1s 刷新**：dpr=1.0（本机 X11）、snapped=(2,2)（SnapRect 起点）；DPR 变化时 dpr 数字随之更新 |

### 判断标准

- 绿网格所有线锐利、红网格可见模糊差异 = Snap 有效。
- `WR_SNAP_STRICT=1` 通过 = SnapCoord/SnapLine/SnapRect 全部落在 1/dpr 网格（代码级断言）。

## Gates

- 窗口打开且 ≥1 present（GPU 真窗）
- `SnapCoord/SnapLine/SnapRect` 结果落在 `1/dpr` 网格上（`WR_SNAP_STRICT=1` 时断言）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
