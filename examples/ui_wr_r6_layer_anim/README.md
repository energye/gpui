# ui_wr_r6_layer_anim — R6 层动画 独立真窗（W5）

Opacity/Transform/Clip **三种层动画同跑** + 大面积静态背景（§2 R6 行）。
retained 姿态：动画帧只重录四个演示板自己的 RepaintBoundary，静态密集区全程
回放——证明「控件层动画不影响静态缓存」。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_r6_layer_anim
```

不设 `RUN_SECONDS`：交互模式，窗不自动关、相位无限循环（像素断言只在关闭跑触发，
证据 = 引擎终帧快照 `R6_SNAP_DIR`（默认 /tmp/r6_layer_anim）/r6_final.png）。

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **30s**（§2.5 动画层档：R6/C5 = 30）；显式 `RUN_SECONDS<30` → FAIL
- `RUN_SECONDS<5` → FAIL（U16）；GPU 真窗必须（needs_gpu_window）

## Visible effect

| Region | Expectation |
|---|---|
| OPA 板（左上） | 橙色卡整体呼吸淡入淡出（α 0.1..1.0），板底色不动 |
| ROT 板（右上） | 红臂绕自身中心持续旋转，中心白盘位置不变 |
| PULSE 板（左下） | 蓝标靶呼吸缩放 0.85×–1.15×，中心盘不动 |
| CLIP 板（右下） | 绿卡圆角 10↔26 呼吸；黄对照卡圆角恒定 16 |
| DENSE 区（右侧） | 4×4 嵌套 boundary 色格 + 8 标签 + 注记行：**全程静止不闪** |
| HUD（底栏） | 相位 / fps / p95 / paint 计数 / α·θ·scale·radius 实时数字 |
| 相位 | Steady 5s → Spike 5s（转速 ×2、幅度 ×1.5）→ Recover 5s → 循环 |

## Gates（§2 R6 行 + §2.2 全族 + §2.5 动画层档；硬，不许放）

| 门禁 | 阈值 | 出处 |
|---|---|---|
| fps_wall ≥55 | RequirePersistentFPS + MinFPSWall=55 | §2.2.2 动画档 |
| interval_p95_ms ≤22 | MaxP95Ms=22 | §2.2.2 |
| hitch_rate_per_min ≤5/min | README 声明预算 | §2 主表 hitch_rate |
| present_policy=retained | RequireRetainedPolicy | 「paint_count 稳」语义前提 |
| boundary_skip ≥1 | MinBoundarySkip=1 | 静态缓存真实回放 |
| boundary_rerecord ≥1 | 动画必须逐帧重录动画板边界（防没画装绿） | §2 主表 |
| **paint_count 稳** | max−min ≤40（全画幅重绘退化 ≈ 帧数 ≫40） | §2 主表 |
| cpu_pct_avg 与 ui/raster 至少一项 >0 | 非双 0 | §2.2 族 D |
| scripted 像素断言 6/6 | 中心不变量 ×3 + 对照卡 + 终态 α 混合 + F6 文本密度 | §2.7/U21 |
| **Golden 静态掩码逐位一致** | diff=0（次跑起；首跑产基线） | §2.7 零容差「静背景不闪」 |
| RSS 字段必采如实上报 | **30s 无硬线**：启动期驱动纹理池爬坡（§10 R7 修订行定性）；泄漏检测归 R15 soak；本窗内存语义由 paint_drift 承担 | §2.2 族 E |

### 运行纪律（2026-08-25 增补）

**关闭跑必须无人值守**：关闭用 30s 内不得拖拽窗口/遮挡/切换工作区。交互拖拽会把引擎
置入 resize 全量恢复（逐帧 swapchain reconfigure + 纹理池重分配波 + 强制全画），这是
设计内行为，但会如实抬高族 A 帧时指标（fps<55 / hitch>5）——那是恢复成本被计入，
不是稳态退化。稳态无人值守连跑 8 次 fps 59.3–59.9、hitch 0–4/min 全部在门禁内。

拖拽过的跑：`resize_during_run=true` 写入 JSON，Golden 逐位对比自动跳过（快照与
基线尺寸不同时逐位对比无定义）；探针几何按新布局重解析。**族 A 门禁（fps/p95/hitch）
在拖拽跑上显式 SKIP**——`family_a_gate` 字段标注 `skipped:` 与原因，stderr 打印 SKIP
行；其余族照常硬判。JSON 另带 `last_resize_at_sec` / `calm_after_resize_sec` 定位
交互窗口。要取关闭证据请重新跑一次不碰窗口的 `RUN_SECONDS=30`（此时
`family_a_gate=enforced`，全门禁生效）。

## 像素断言点（无竞态设计）

- 旋转/缩放/裁剪目标采**几何中心**——变换不变量，与当前相位无关
- 透明度卡期望 = **终帧节点状态**的精确公式解 out = α·fill + (1−α)·bg：
  引擎在 loop 停止后于栅格线程拍最终帧（`PipelineOptions.SnapshotPath`），
  终帧节点状态就是最后一次 tick 写入的值——期望与画面同源，无跨帧竞态，
  且运行中零停顿（hitch 预算不被快照污染）
- Golden 掩码只含静态区（顶栏/图例/密集区）；HUD 在掩码外（约 10 次/秒自刷新）

## U20 实现点六维

- **正确性**：三种层各自走 shipped RO（RenderOpacity 组隔离 / RenderTransform 绕枢轴 CTM / RenderClipRRect 圆角裁剪），retained 层树产物 scene.OpacityLayer/TransformLayer/ClipRRectLayer
- **脏区**：动画只脏四块板的 boundary；boundary_rerecord 随动画增长、静态区 skip 只增
- **缓存**：静态密集区嵌套 RepaintBoundary 回放（MinBoundarySkip≥1 强制）；动画板边界逐帧重录属预期
- **边界条件**：Spike 幅度加大不越界（clamp01）；resize 后探针几何重解析；快照缺失时断言 FAIL 不静默
- **失败模式**：全画幅重绘退化 → paint drift 门禁红；静区闪烁 → Golden 红；层洞 → 中心断言红
- **窗内如何看出**：HUD α/θ/s/r 数字 + paint 计数；DENSE 区肉眼应完全静止
