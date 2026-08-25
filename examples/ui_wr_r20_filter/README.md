# ui_wr_r20_filter — R20 Filter 层 独立真窗（W5 可选）

模糊/灰度**滤镜子树** + 外部不变内容（§2 R20 行）。retained 姿态：滤镜参数
相位切换只重录两块滤镜板自己的 RepaintBoundary，静区与对照卡全程回放——
证明「控件滤镜效果局部化」。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=10 go run ./examples/ui_wr_r20_filter
```

不设 `RUN_SECONDS`：交互模式，窗不自动关、相位无限循环（像素断言只在关闭跑触发）。

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **10s**（§2.5 SaveLayer/Filter 档：R18/R20/C6 = 10）；`RUN_SECONDS<10` → FAIL
- `RUN_SECONDS<5` → FAIL（U16）；GPU 真窗必须（needs_gpu_window）

## Visible effect

| Region | Expectation |
|---|---|
| GRAY 板 | 彩色格纹卡整体变灰（BT.601），卡内灰字仍可读；Spike 相位切回彩色、Recover 再变灰 |
| BLUR 板 | 橙色竖条被高斯糊化；Spike 半径 ×2 更糊；Recover 回 4px |
| RAW 板 | 同构图紫卡**无滤镜**：全程原色鲜艳——「外不变」的硬证据 |
| PULSE 板 | 绿盘呼吸缩放 0.9×–1.1×（持续动画热点，不过滤镜） |
| DENSE 区 | 4×4 嵌套 boundary 色格 + 8 标签 + 注记：**全程静止不受滤镜影响** |
| HUD | 相位 / fps / p95 / **flt=filter_layer_count（Spike 切换时 0↔1 跳变）** / rr·skip |
| 相位 | Steady 3s（灰开·blur4）→ Spike 3s（identity·blur8）→ Recover 3s 回落 → 循环 |

## Gates（§2 R20 行 + §2.2 全族 + §2.5 SaveLayer/Filter 档；硬，不许放）

| 门禁 | 阈值 | 出处 |
|---|---|---|
| present_policy=retained | RequireRetainedPolicy | 「局部 rerecord」语义前提 |
| boundary_skip ≥1 / boundary_rerecord ≥1 | 静区真实回放 + 切换真实脏子树 | §2 主表「局部 rerecord」 |
| **filter_layer_count ≥1** | 滤镜必须真实应用过（防没画装绿） | §2.5「层计数」+ FrameMetrics 字段 |
| paint_count 稳 | max−min ≤40 | §2 主表 |
| fps_wall ≥45 | RequirePersistentFPS（README 声明预算） | §2.2 族 A（非动画层档，自证不降画质取紧值） |
| interval_p95_ms ≤22 | MaxP95Ms=22 | §2.2 族 A |
| hitch_rate_per_min ≤30/min | README 声明预算：**稳态帧零 hitch**（p99≈17ms）；hitch 集中在参数组合首帧的一次性滤镜结果重生成（SaveLayer/Filter 档本质成本，循环回切命中旧缓存不重复付费） | §2.2 族 A |
| cpu_pct_avg 与 ui/raster 至少一项 >0 | 非双 0 | §2.2 族 D |
| scripted 像素断言 5/5 | 矩阵公式解 + 外不变 + blur 中心保持 + 中心不变量 + F6 文本密度 | §2.7/U21 |
| **Golden 静态掩码逐位一致** | diff=0（次跑起；首跑产基线） | §2.7 零容差 |
| **快照双张留档** | steady 中段 + Recover 段（栅格线程 SnapshotAsync） | U21 §4 快照纪律 |
| RSS 字段必采如实上报 | 10s 无硬线：启动期驱动纹理池爬坡定性；泄漏归 R15 soak | §2.2 族 E |

## 像素断言点（无竞态设计）

- **gray_red_band_matrix_exact**：红带中心 = 终帧矩阵状态的精确公式解
  （identity 直通 / 灰度矩阵逐通道 luma），期望取自 run 后冻结的节点状态——同源无竞态
- **raw_control_outside_invariant**：无滤镜对照卡中心 = 原色（tol ≤8/255），
  「外不变」硬断言
- **blur_bar_center_preserved**：对称核中心不变量——彩条中部远离边缘处 ≈ 源色（tol ≤12/255）
- **pulse_disc_center_invariant**：变换不变几何中心采样
- **dense_note_text_density**：F6 区域级文本密度 ≥120px

## U20 实现点六维

- **正确性**：RenderGrayscale/RenderImageFilter 子树走 SaveLayer→PushLayerIsolated 真离屏隔离，合成期应用滤镜；retained 树映射 scene.ColorFilterLayer/ImageFilterLayer
- **脏区**：矩阵/半径相位切换只脏两块滤镜板 boundary；boundary_rerecord 随切换增长、DENSE 区 skip 只增
- **缓存**：identity/零半径整层省略（稳态层树不变）；静区嵌套 boundary 回放
- **边界条件**：负半径 clamp 0；空子树 leaf picture 兜底；resize 后探针几何重解析
- **失败模式**：滤镜从未应用 → filter_layer_count 门禁红；外泄 → 外不变断言红；全画幅退化 → paint drift 红
- **窗内如何看出**：HUD flt 数字 0↔1 跳变 + blur 值；GRAY 卡肉眼灰↔彩翻转；RAW 卡恒艳
