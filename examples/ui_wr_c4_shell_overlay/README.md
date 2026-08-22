# ui_wr_c4_shell_overlay — C4 组合真窗（W4）

**Covers:** R3（Boundary 真缓存）+ R8（Overlay 独立合成）+ R21（壳/内容分层）
**集成命题：** 顶栏壳 + 可滚体内容 + 浮层面板 —— 三层同屏独立，互不重录。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_wr_c4_shell_overlay
```

## Window / Close duration

- 客户区 **1200×800**（U15）；关闭用 **RUN_SECONDS=15**（§2.5 组合窗 C4 行）。
- `RUN_SECONDS<5` → `FAIL:` + exit 1（U16）。
- present_policy = **retained**（Plan C 姿态，停下报告时用户确认；稳态帧 CompositeOnly + damage present，只有脏层重录）。

## Visible effect

| Region | Expectation |
|--------|-------------|
| SHELL 壳带（TopBar 下标题+按钮+字号样张） | 全程像素静止；体滚动、浮层开合都不得让它重画 |
| BODY 体内容（60 项虚拟列表） | 每帧持续滚动（Steady 慢滚 / Spike 快滚 / Recover 回落），无限回绕 |
| DENSE 静态密集区（4×4 色格嵌套 boundary + 8 标签） | 静止不动，boundary 缓存回放（R3） |
| HOT 热点 | 每帧变色，全程活跃（含浮层开着时） |
| 浮层（Spike 相位循环开合） | 对话框+menu 叠层 / 下拉 / tooltip 三种轮换，2s 开 / 1.5s 关，每次全新 entry；Recover 相位全部关闭 |
| LiveHUD | 实时 fps/p95/policy/paint/presents + `shell_rr / skip / main=X/base / excess / ov=N cyc=K OPEN/closed`；门禁趋势绿/红 |

## Gates（§3 C4 门禁并集 ∪(R3,R8,R21) + §2.2 全族）

| Gate | Line | Source |
|------|------|--------|
| `shell_rerecord_scroll` | ==0（滚动全程壳零重录；resize 帧 3 帧沉降排除） | R21 §2 主表 |
| `shell_skip_scroll` | >0（壳 Picture 回放命中） | R21 |
| `main_dirty_excess_frames` | ==0（开浮层帧主带脏计数 ≤ Steady 窗基线 MAX） | R8 |
| `overlay_entries_on_open` | ≥1（浮层带是被标脏的一侧） | R8 |
| `overlay_open_observed` / `overlay_still_open` | true / false（Recover 必须关净） | R8 |
| `boundary_skip` | ≥1（静态密集缓存回放） | R3 |
| `present_policy` | retained | 用户确认姿态 |
| fps_interval | ≥55（持续 tick 类） | §2.2 族 A / U13 |
| interval_p95_ms | ≤22 | §2.2 族 A |
| vsync_source | 必须输出；fallback 禁止宣称锁 60Hz | U13 |
| cpu_pct_avg / cpu_ui_pct / cpu_raster_pct | 必采，禁双 0 装绿 | §2.2 族 D / U14 |
| rss_start/end/peak/slope | 必采；15s 窗斜率处启动爬坡段属预期（R8 600s 长跑实测稳态 ~167KB/min） | §2.2 族 E |
| cpu_fallback_ops | 热路径无故暴涨 → FAIL | §2.2 族 F |

## Implementation notes（U20 六维）

- **正确性**：壳带整带一个 `SetShellBoundary`；retained Plan C 只脏层重录；overlay 合成在主树之上（FramePacket.Root→.Overlay）。
- **脏区**：分带观测——`LastShellBoundaryFrame` 采壳分区计数；`LastOverlayBandFrame` 挂接前后分采主带/浮层脏 id 计数，开浮层不污染主带计数。
- **缓存**：壳 Picture 回放命中贯穿全程；密集格/列表 cell 嵌套 boundary 各自缓存；浮层面板/menu/下拉各自是 RepaintBoundary。
- **边界条件**：resize 帧 3 帧沉降排除防 WM 干扰；每轮浮层全新 entry、三种轮换；无限滚动回绕。
- **失败模式**：壳重录 / 壳零回放 / 主带脏超基线 / 浮层未观察到或退出仍开 / boundary_skip=0 任一 → FAIL exit 1。
- **窗内如何看出**：HUD 实时数字 + 顶栏像素静止 vs 行滚动 vs 浮层堆叠三层对比。
