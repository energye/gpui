# ui_wr_c4_shell_overlay — C4 组合真窗（W4 · v2 全新重写）

**Covers:** R3（Boundary 真缓存）+ R8（Overlay 独立合成）+ R21（壳/内容分层）
**集成命题：** 顶栏壳 + 可滚体内容 + 浮层面板 —— 三层同屏独立，互不重录。

> 本包为 2026-08-24 模式 2 反攻重关产物：旧实现整体弃用，本文件与 `main.go`
> 按 §3.1.2 C4 行 + §3.1.1 + §2.7（U21 像素断言/Golden）从零重写。
> 相比被弃用版本的关键差异：新增 6 个脚本化命中探针、9 项像素断言
> （F0/F5/F6/F8 形态矩阵）、Golden 静态区逐位对比、三张相位快照留档。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_c4_shell_overlay            # 交互模式：不自动关窗，弹层反复循环，点 X 关闭
RUN_SECONDS=25 go run ./examples/ui_wr_c4_shell_overlay   # 关闭跑：25s 固定脚本 + 全门禁
```

快照输出目录可用 `C4_SNAP_DIR` 覆盖（默认 `examples/ui_wr_c4_shell_overlay/golden/`）。

## 实现要点（v2 最终态）

- **相位钟按 tick 计数**（45 tick/s 保守预算）：ticker dt 是墙钟派生且被钳制，
  秒制时间线在宿主抖动下可能跑不满固定 RunFor；tick 制保证 Steady→Spike×3 循环
  →Recover 全脚本必然完成（确定性，对齐 §2.7 pumpFrames 语义）。
- **§2.7 快照单次批量**：三张快照在 Recover 起点合并为一次 `SnapshotAsync`
  排空内完成（steady=关净树 → open=重建模态浮层 → recover=再清空），
  整场只付一次读回停顿，避免三次回读把 fps_interval 平均值拖破 55 线。
  steady 拍摄前把行采样点按当前 scrollOffset 现算（布局链驱动，不跨线程改状态）。
- **HUD 刷新 ~10Hz**：`MetricsStore.Snapshot()` 每次调用对 256 样本环做插入排序，
  60 次/秒的 UI 线程排序与 raster 线程争抢导致帧超槽；节流后 fps 回到 57-59。

## Window / Close duration

- 客户区 **1200×800**（U15）；关闭用 **RUN_SECONDS=25**（弹窗≥5s 后：Steady5 + 3×(开5+关1) + Recover 沉降）。
- `RUN_SECONDS<5` → `FAIL:` + exit 1（U16）；关闭跑必须 ≥15 且覆盖全部三种浮层。
- present_policy = **retained**（Plan C 姿态，停下报告时用户确认；稳态帧 CompositeOnly + damage present，只有脏层重录）。

## Visible effect

| Region | Expectation |
|--------|-------------|
| SHELL 壳层演示带（标题+双按钮+字号样张） | 全程像素静止；体滚动、浮层开合都不得让它重画（rr=0，skip 持续增长） |
| BODY 列表（左列 420px 宽，紧贴 Legend 右缘） | 平滑滚动 2px/帧（Spike 6px/帧），行号随滚动变化；列表裁剪在自身列宽内，不外溢 |
| DENSE 静态密集区（4×4 色格嵌套 boundary + 8 标签） | 静止不动，boundary 缓存回放（R3） |
| HOT 自绘热点 | 两个变色圆（脉冲核心 + 环绕卫星）每帧重绘，全程活跃（含浮层开着时） |
| 浮层（Spike 相位循环） | 对话框+menu 叠层（全屏屏障，随窗口大小自适应）/ 下拉菜单 / Tooltip 三种轮换：每种开 ≥5s / 关 1s，每次全新 entry；Recover 相位全部关净 |
| 状态行 / 探针横幅 | `PHASE=… scr=… ov=OPEN/closed cyc=N rr/skip/main/excess` 实时刷新；`HIT n/6 px n/9` 显示命中与像素断言进度 |
| LiveHUD | 实时 fps/p95/policy/paint/presents + shell_rr/skip/main=X/base excess + 门禁趋势绿/红 |

## Gates（§3 C4 门禁并集 ∪(R3,R8,R21) + §2.2 全族）

| Gate | Line | Source |
|------|------|--------|
| `shell_rerecord_scroll` | ==0（滚动全程壳零重录；resize 帧 3 帧沉降排除） | R21 §2 主表 |
| `shell_skip_scroll` | >0（壳 Picture 回放命中） | R21 |
| `main_dirty_excess_frames` | ==0（开浮层帧主带脏计数 ≤ closed 态最坏基线） | R8 |
| `overlay_entries_on_open` | ≥1（浮层带是被标脏的一侧） | R8 |
| `overlay_open_observed` / `overlay_still_open` | true / false（Recover 必须关净） | R8 |
| `overlay_cycles` / kinds | ≥3 次且三种浮层全出现（15s 关闭跑） | §3.1.2 R8 行「三种轮换」 |
| `boundary_skip` | ≥1（静态密集缓存回放） | R3 |
| `scripted_ok` | ==6/6（壳按钮/密集格/滚动行/浮层按钮/屏障消费/恢复命中） | §2.7 三证据·逻辑侧 |
| `pixel_checks_ok` | ==9/9（F0/F5/F6/F8，见下表） | §2.7 U21 |
| `pixel_golden_diff_pct` | ==0（第 2 次运行起，静态掩码逐位对比；首跑只产基线） | §2.7 规则 5 |
| `present_policy` | retained | 用户确认姿态 |
| fps_interval | ≥55（持续 tick 类） | §2.2 族 A / U13 |
| interval_p95_ms | ≤22 | §2.2 族 A |
| vsync_source | 必须输出；fallback 禁止宣称锁 60Hz | U13 |
| cpu_pct_avg / cpu_ui_pct / cpu_raster_pct | 必采，禁双 0 装绿 | §2.2 族 D / U14 |
| rss_start/end/peak/slope | 必采；15s 窗斜率处启动爬坡段属预期（R8 600s 长跑实测稳态 ~167KB/min） | §2.2 族 E |
| cpu_fallback_ops | 热路径无故暴涨 → FAIL | §2.2 族 F |

### 像素断言清单（§2.7 形态×断言矩阵选型 · 容差显式声明）

| 断言 | 形态 | 快照 | 容差（依据） |
|------|------|------|--------------|
| shell_btn2_base_color | F0 静态色块精确点 | steady / recover | ≤8/通道（AA/舍入） |
| dense_cell_base_color | F0 | steady | ≤8/通道 |
| scrolled_row_chip_color | F0+F1 锚点采样（坐标来自布局链+实时 scrollY） | steady | ≤8/通道 |
| dense_panel_base_color | F0 | steady | ≤8/通道 |
| shell_title_text_density | F6 区域级文字断言（盒内非背景像素 ≥120，禁单点采字隙） | steady | 阈值判定 |
| barrier_blend_over_dense_bg_F5 | F5 半透明混合：out = 0.45·src + 0.55·bg（公式声明） | open | ≤12/通道 |
| popup_button_base_color | F0（浮层内容真实绘制到屏） | open | ≤8/通道 |
| barrier_removed_dense_bg_restored_F8 | F8 双态恢复（临时视觉态必须回到基色，防污染） | recover | ≤8/通道 |
| shell_band_intact_at_recover | F0 回归 | recover | ≤8/通道 |

采样纪律：采样点坐标一律取自引擎布局链（body+offset 链 / overlay Entry 字段），
禁止手算绝对坐标；动态目标在已知稳定段采样（Steady 平台期 / 浮层打开后平台期 /
Recover 关净后），避开相位切换帧与 resize 后 3 帧。Golden 对比仅取静态掩码
（TopBar / Legend / 壳层演示带 / 密集面板），动态区域按形态矩阵另行断言；
同引擎零容差（任一通道差即计差异像素），首跑只产基线不定 PASS。

## Implementation notes（U20 六维）

- **正确性**：壳层演示带整带一个 `SetShellBoundary`；retained Plan C 只脏层重录；浮层合成在主树之上（FramePacket.Root→.Overlay）；密集格每格嵌套 RepaintBoundary。
- **脏区**：分带观测——`LastShellBoundaryFrame` 采壳分区计数；`LastOverlayBandFrame` 挂接前后分采主带/浮层脏 id 计数；closed Spike 帧刷新基线、open 帧不得超基线。
- **缓存**：壳 Picture 回放贯穿全程（skip 增长 rr 保持 0）；密集格/列表 cell 各自缓存；每轮浮层全新 entry 分配全新层纹理。
- **边界条件**：resize 帧 3 帧沉降排除且几何从布局链重新解析；长跑回绕 8 帧沉降；Recover 即时关净；HUD/状态行/横幅都在壳 boundary 外（自脏不破坏分层）。
- **失败模式**：壳重录 / 壳零回放 / 开浮层超主带基线 / 浮层未开或退出仍开 / 任一探针不命中 / 任一像素断言失败 / Golden 有差异（≥第 2 跑）→ FAIL exit 1。
- **窗内如何看出**：HUD 实时数字 + 横幅 HIT n/6 px n/9 + 顶栏像素静止 vs 行滚动 vs 浮层堆叠三层对比；快照 PNG 留档供人眼复查与 Golden 复算。
