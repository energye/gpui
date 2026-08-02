# ui_wr_r12b_debug_repaint — R12b 重绘调试可视化

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r12b_debug_repaint                       # 默认无限运行，手动关窗退出
RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint         # 8s 后自动关闭并输出门禁 JSON
# 关闭叠加（对照相位）：
WR_DEBUG_REPAINT=0 RUN_SECONDS=5 go run ./examples/ui_wr_r12b_debug_repaint
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r12b_debug_repaint  R12b 重绘调试可视化            |
+----------+---------------------------------------------------+
| LEGEND   | 4 静态蓝块(100x80)    hotA 红块(90x90)             |
| (260px)  | 8 静态标签             hotB 绿块(60x60)             |
|          | 8 静态色条(46x34)                                   |
+----------+---------------------------------------------------+
| HUD: R12b| phase | fps | p95 | on | flash_draws | 门禁        |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 静态蓝块 4 个 | (304,80) 起，100×80×4 间隔 130 | RepaintBoundary 蓝块 | **从不闪 magenta**（未重绘） |
| 2 | 静态标签 8 个 | (304,200) 起 4 列 × 2 行 | `static 0..7` | 从不闪 |
| 3 | 静态色条 8 个 | (304,290) 起，46×34 间隔 52 | 非 boundary 色块 | 从不闪 |
| 4 | hotA 红块 | (844,80)，90×90 | 每 tick 变红深浅 | **Steady/Spike 相位闪 magenta**（活重绘）；Recover 不闪 |
| 5 | hotB 绿块 | (844,200)，60×60 | 绿块 | **仅 Spike 相位闪**；其余静 |
| 6 | Recover 相位 | — | 无脏帧 | 全窗口无任何 magenta |
| 7 | `WR_DEBUG_REPAINT=0` | — | 关闭叠加 | **全程无 magenta**（包括 hotA/hotB 重绘时） |
| 8 | HUD | (0,728) | fps/p95/on/flash_draws + 门禁 | **每 ~0.1s 刷新**：on 时 flash_draws 持续累加（数千）、off 时恒 0；fps≈59.9 |

### 判断标准

- on：只有 hotA/hotB 区域闪 magenta、静态区全黑 = 叠加只覆盖真实脏区（boundary 级）。
- off：全程无 magenta = 开关生效（`SetDebugRepaint` 端到端）。
- Recover 相位无闪 = 无脏即无叠加（debug 不制造脏区）。

## Gates

- `debug_repaint_on=true` → `debug_draws ≥ 1`（脏区有叠加）
- `debug_repaint_on=false`（WR_DEBUG_REPAINT=0 跑）→ `debug_draws == 0`
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
