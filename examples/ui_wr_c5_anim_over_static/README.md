# ui_wr_c5_anim_over_static — C5 组合窗（R6+R3+R4 · W5）

**层动画盖在静态缓存上**：R6 三种层动画（Opacity/Transform/Clip）持续同跑，
半透明 FLOAT 卡呼吸透明度、z-order 叠在 R3 嵌套 boundary 静态缓存的色格上方；
R4 retained 姿态下动画帧只脏动画 band，静态缓存照常回放——证明「控件动画层
不影响静态缓存」。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/ui_wr_c5_anim_over_static
```

不设 `RUN_SECONDS`：交互模式（像素断言只在关闭跑触发）。

## Window / Close duration

- 窗口 **1200×800**（U15）
- 关闭用时长 **30s**（§2.5 动画层档 = max(R6=30, R3=8, R4=15)）；`RUN_SECONDS<30` → FAIL
- `RUN_SECONDS<5` → FAIL（U16）；GPU 真窗必须（needs_gpu_window）

## Visible effect

| Region | Expectation |
|---|---|
| OPA 板 | 蓝卡整体呼吸淡入淡出（α 0.1..1.0），板底不动 |
| ROT 板 | 红臂绕中心持续旋转，中心白盘不动 |
| CLIP 板 | 绿卡圆角 12↔20 呼吸 |
| **FLOAT 卡** | 橙色半透明卡叠在 DENSE 色格上方，α 0.3..0.8 呼吸——底下色格被精确混合透出 |
| DENSE 区 | 4×4 嵌套 boundary 色格 + 8 标签 + 注记：**全程静止不闪不重录** |
| HUD | 相位 / fps / p95 / α·θ 实时数字 |
| 相位 | Steady 5s → Spike 5s（转速 ×2 幅度 ×1.3）→ Recover 5s → 循环 |

## Gates（§3 C5 行 + §3.1.2 + §2.2 族 A 动画层档；硬，不许放）

| 门禁 | 阈值 | 出处 |
|---|---|---|
| fps_wall ≥55 | RequirePersistentFPS + MinFPSWall=55 | §2.2.2 动画层档 |
| interval_p95_ms ≤22 | MaxP95Ms=22 | §2.2 族 A |
| hitch_rate_per_min ≤5/min | §2.2.3 默认预算（同 R6 动画层档） | §2.2 族 A |
| present_policy=retained + skip/rerecord ≥1 | 静区回放 + 动画板重录分带 | §3.1.2 damage 只在动画区 |
| paint_count 稳 | max−min ≤40 | retained 语义 |
| cpu_pct_avg 与 ui/raster 至少一项 >0 | 非双 0 | §2.2 族 D |
| scripted 像素断言 6/6 | FLOAT 混合公式解（底=cell(3,1)）+ OPA 混合公式解 + ROT/CLIP 中心不变量 + 静区色格 + F6 文本密度 | §2.7/U21 |
| **Golden 掩码绕开 FLOAT 卡矩形** | 顶栏/图例/静区四块并集跨跑逐位一致零容差 | §2.7 |
| RSS 字段必采如实上报 | 30s 无硬线；泄漏归 R15 soak | §2.2 族 E |

## 像素断言点（无竞态设计）

- **float_card_group_blend**：FLOAT 卡中心 = 终帧冻结 α 的精确混合解
  out = α·橙 + (1−α)·cell(3,1)（卡只盖纯色格，无文字干扰）
- **opa_card_group_blend**：OPA 卡中心 = α·蓝 + (1−α)·板底（终帧冻结 α）
- **rot_center_invariant / clip_center**：变换不变几何中心采样
- **dense_cell_under_float_host**：未遮挡静区色格 = 原色（外不变）
- **dense_note_text_density**：F6 区域级文本密度

## U20 实现点六维

- **正确性**：三 RO 层动画走 shipped 路径（R6 已关）；FLOAT=RenderOpacity 叠在嵌套 boundary 缓存上，retained 树 OpacityLayer→BoundaryLayer 复合正确
- **脏区**：稳态帧只重录动画 band；DENSE 区 boundary_skip 单调增长；FLOAT 的 α 呼吸只脏自己的 boundary
- **缓存**：静态缓存跨帧回放（golden 佐证）；动画板逐帧 rerecord 属预期
- **边界条件**：Spike 幅度加大 clamp01 不越界；resize 后探针重解析
- **失败模式**：动画污染静区 → golden 红/dense_cell 红；全画幅退化 → paint drift 红
- **窗内如何看出**：HUD 相位/fps 数字；DENSE 区肉眼完全静止而 FLOAT 卡在其上呼吸
