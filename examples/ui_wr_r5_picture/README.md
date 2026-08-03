# ui_wr_r5_picture — R5 Picture 录/回放

> 状态：**🔄 质量返工（模式 2 反攻重关）** —— ui 层 Picture 能力推翻重写（skia/flutter 对齐）+ 真窗按 U17/U18/U20 升维。

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
能力元素（直绘区/回放区/标签）一律 **§2.6.1 Align 布局驱动**（`Body.Align`，比例定位，resize 自动跟随）；静态色格阵为装饰用 `Place`。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r5_picture  R5 Picture 录/回放                     |
+----------+---------------------------------------------------+
| LEGEND   | DIRECT 直绘区           REPLAY 回放区   4x3 色格    |
| (260px)  | Align(0.10,0.10)       Align(0.55,0.10)  40x40     |
|          | 200x300 同构图          200x300 同构图              |
|          | 三角填充(红) 描边(蓝) 填充(绿) 描边(黄)             |
|          | 图 1:1+缩放(绿) 文字 PIC-REPLAY / v2               |
+----------+---------------------------------------------------+
| HUD: R5 | phase | fps | p95 | picture_ops | bounds | 门禁     |
+--------------------------------------------------------------+
```

| # | 区域 | 内容 | 预期效果 |
|---|------|------|----------|
| 1 | DIRECT 直绘区 | 每帧直绘 8 元素：红三角（FillPath）+ 蓝描边（StrokePath 3px）+ 绿填充矩形 + 黄描边矩形 + 绿图 1:1(150,20) + 绿图缩放 32×32(150,100) + 白字 `PIC-REPLAY`/`v2` | 始终完整显示，与回放区**逐像素一致** |
| 2 | REPLAY 回放区 | 保留 Picture（`scene.RecordPicture`，**8 ops**）每帧 `Replay` | 与左区**逐像素一致**（几何/颜色完全相同） |
| 3 | 区标签 | `DIRECT (直绘)` / `REPLAY (回放)` | 常驻，Align 跟随 |
| 4 | 静态 4×3 色格 | (964,80) 起，40×40×12 | 永不闪（非 picture） |
| 5 | 相位行为 | Steady 2s / Spike 2s / Recover | Steady+Spike 每帧重绘两区；Recover 纯 Replay（无闪） |
| 6 | HUD | fps/p95/picture_ops/**bounds**/presents + 门禁 | 每 ~0.1s 刷新：fps≈59.9、picture_ops=8 常驻、bounds=(18,20 164x210) |

### 判断标准

- 左右两区图像完全一致 = 回放等价（记录/回放无丢失、无错序）。
- 静态色格不闪 + Recover 相位无闪 = Picture 保留在层树内只回放不重录。
- HUD picture_ops=8 = op 计数诚实（2 path + 2 rect + 2 image + 2 text）。
- HUD bounds=(18,20 164x210) = 重写后 Bounds 记入 path/image 几何的证明（旧实现 image/path 不记 Bounds；实测为全部几何 op 的 union：stroke inflate 1.5 外扩 → minX=18，maxY=FillRect 230）。

> 诚实边界：`DrawString` 依赖系统 face；无 face 时文字 op 不录（op=6，仍 ≥3 门禁）。Bounds 为逻辑坐标取整（float64 内部累加 → ceil 保守）。

## Gates

- `picture_op_count ≥ 3`（实测 8：2 path + 2 rect + 2 image + 2 text；face 缺失时 6）
- 回放像素 ≡ 直绘（同几何同色，人眼对照）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
- `slope_gate=off`：本窗 5s < 15s **正确性窗**（§2.2.4），`rss_slope_kb_per_min=1320727` 为进程启动一次性分配（wgpu/字体加载 8.9MB→128MB）在短窗的放大斜率，**非泄漏指示**，不做 slope 门禁；长窗 soak 才判 slope。

## 实现点六维（U20）

| 维度 | 回答 |
|------|------|
| 正确性 | 录制 8 op → Replay 逐 op 转发，直绘 vs 回放同构图逐像素对照 |
| 脏区 | Picture 每帧只 Replay 不重录（层内 retained）；Recover 相位无 MarkNeedsPaint → 纯层树回放 |
| 缓存 | Picture 显示列表记录一次（`RecordPicture` 冻结 ops + detached 副本），重写后 op 不可变、paint 录制期规范化 |
| 边界条件 | face 缺失 → 文字 op 跳过（op=6 仍过门禁）；image 引用 ImageBuf 生命周期贯穿窗（defer Dispose）；空 Bounds → HUD 显示 no-bounds |
| 失败模式 | op_count<3 → 门禁 FAIL；窗开不了 → FAIL: window open (needs_gpu_window)；Bounds 空 → HUD no-bounds 可见（不是静默） |
| 窗内如何看出 | HUD picture_ops 数字 + bounds 矩形 + 左右区逐像素一致 + 相位切换 |
