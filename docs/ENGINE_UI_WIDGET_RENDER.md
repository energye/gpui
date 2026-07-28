# 自定义控件渲染基座 — Flutter 对齐（统一真源）

> **版本：2.1** | 日期：2026-07-28  
> **地位：** 自定义控件 **渲染基座 + 排期** 唯一真源。  
> **施工口径：** 只做任意自定义 RO 所需的 Flutter **帧经济学**（脏区·Boundary·层合成·虚拟化宿主·Overlay）。  
> **暂缓：** Kit/Ant、IME 产品、a11y 桥、Win/mac 深做。  
> **验收硬规则（v2.1）：**  
> **每一项能力（§R / 每一 W 交付）必须同时具备：**  
> ① **真实窗口** 示例（`examples/`，GPU Present，非仅 `NewContext` CPU 单测）  
> ② **JSON/stderr 指标**（可脚本判定 FAIL）  
> ③ **可观察视觉效果**（人眼或像素采样约定写进示例 README）  
> CPU 单测可作回归，**不能单独关闭 W 项**。  
>  
> **并读：** [`ENGINE_UI_RENDER_BASE.md`](./ENGINE_UI_RENDER_BASE.md)（画什么）· [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) · [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)

---

## 0. 目标（收敛）

```text
要：自定义 RO 在约定下 → 局部脏、Boundary 缓存、层 Present、虚拟化、Overlay 分层；
    指标可证伪；真窗 GPU 下内容正确且可测「省」。
不要：控件库皮肤、半 retained 假 60fps、万级 RO 列表/画布。
```

| 层 | 本阶段 |
|----|--------|
| L2 RO/Layer/Boundary/Composite | **主施工 W0–W6** |
| L1 render Present | 已收口；为 L2 提供 RT/Picture |
| L0 Host | 用现成 Linux 验窗；SPI 不深做 |
| L3–L5 Kit | 暂缓 |

**可行性：** 对齐 Flutter `Layout→Paint/Record→Composite→Present`，不 1:1 抄 Widget/Material。

---

## 1. 能力清单 §R（每项 = 实现 + 真窗 + 指标 + 效果）

> 作者 = 自写 RO，无官方 Kit。  
> **「完成」定义：** 代码合入 **且** 对应真窗示例绿 **且** 指标门禁写进示例 stderr/JSON。

### 1.1 主能力表

| ID | 能力 | 代码落点（目标） | **真窗示例** | **指标（须输出）** | **视觉/效果约定** | 波次 |
|----|------|------------------|--------------|--------------------|-------------------|------|
| **R1** | 局部 NeedsLayout | `PipelineOwner` 传播 | `…/w_layout`：只脏一子改高 | `layout_count` 动画期不涨或可解释 | 仅该子高度变，邻域不抖 | 持续/W1+ |
| **R2** | 局部 NeedsPaint | RO 脏标志 | 与 R3 同窗 | `paint_count` / visits | 颜色变仅热点 | 配 R3 |
| **R3** | Boundary **真缓存** | Picture 或 RT | `…/w_boundary` | `boundary_rerecord`、`boundary_skip` | 静块不动；脏块每帧变；skip>0 | **W1** |
| **R4** | 层 Composite Present | embedder+scene | `…/w_layer_present` | `present_mode`、`damage_area_px`、策略名 | Retained 下静块在、damage≪全屏 | **W2** |
| **R5** | Picture 录制 | scene Picture | 含于 w_boundary / w_layer | `picture_ops` 或 rerecord | 重放像素与直绘一致 | W1–W2 |
| **R6** | Opacity/Transform **层动画** | scene 层+animation | `…/w_comp_anim` | `paint_count` 稳、`hitch_rate` | 旋转/淡入流畅；静背景不闪 | **W5** |
| **R7** | 虚拟化宿主 | VirtualList+Viewport | `…/w_virtlist` | `bind_count`≪N、p95、hitch | 只见视口 cell；快滑无空洞约定 | **W3** |
| **R8** | Overlay 独立合成 | overlay+embedder | `…/w_overlay` | 主树 `paint_count` 开浮层后不涨 | 面板盖上；底下静内容仍在 | **W4** |
| **R9** | 文本 measure 缓存 | text/face | 含于 w_boundary 多标签 | 可选 `measure_hits`（可后） | 多帧同文无抖宽 | W1+ |
| **R10** | 图异步→局部脏 | ui/io + boundary | `…/w_async_image` 或并入 w_virtlist | 解码后 `rerecord` 仅 cell | 占位→出图仅该格变 | W3 |
| **R11** | DPR/尺寸缓存失效 | Host Scale+缓存键 | `…/w_dpr` 或 resize 手测脚本 | 失效后 rerecord 上升一次 | resize 后无残影/错位 | W1–W2 |
| **R12** | 帧指标完备 | MetricsStore | **每个** 窗例 stderr+JSON | 见 §1.3 公共字段 | 人可读 + 可 `FAIL:` | 全程 |
| **R13** | Hit≡绘 | transform/overlay hit | `…/w_hit` 或 cliplayer | 点击日志命中 ID | 点哪高亮哪 | W2+ |
| **R14** | 缓存预算 | boundary 池 | w_boundary 压力 | RSS/`cache_entries` | 超预算淘汰仍正确 | W6 |
| **R15** | UI/raster 所有权 | raster.Loop | 长时间 soak | 无读回；无 race 崩溃 | soak 不挂 | W2+ |
| **R16** | 首帧/恢复/丢表面 | WarmUp、full force | w0 + 手动恢复 | `present_mode=full` 首帧 | 首帧有内容；恢复不黑屏 | **W0**/后 |
| **R17** | 不可见降频 | 最小化钩子 | 后置可测 | frame 间隔变大 | 后 | 后 |

### 1.2 Present 策略

| 策略 | 行为 | 默认 |
|------|------|------|
| **FullPaint** | 每帧全树 paint（防 Clear 丢静态） | **现在** |
| **Retained** | 脏 boundary 重录 + blit | **W6 门禁后** |
| **Hybrid** | 可选过渡 | 非必须 |

```text
禁止：CompositeOnly 跳过静态 + GPU 矢量 LoadOpClear 并存。
```

### 1.3 每个真窗示例的 **公共指标最低集**（stderr + 一行 JSON）

```text
必选：
  present_count, frame_count
  present_mode, damage_area_px, present_policy   // policy: full_paint|retained|…
  layout_count, paint_count
  avg/p50/p95 interval_ms, hitch_count, hitch_rate_per_min, vsync_source
  rss_start_kb, rss_end_kb, rss_slope_kb_per_min（有 proc 时）
  gpu_ops, cpu_fallback_ops（有则）

按能力追加：
  R3: boundary_rerecord, boundary_skip
  R7: bind_count, item_count
  R4: damage_area_px / surface_area 比值
```

**判定：** 示例 `main` 在不达标时 `os.Exit(1)` 并打印 `FAIL: …`（与 perfsoak 同风格）。  
仅打日志不算关闭能力。

### 1.4 作者纪律（自定义 RO）

1. 视觉→Paint；尺寸→Layout  
2. 热点→RepaintBoundary（有缓存义务）  
3. 长列表→VirtualList，禁止 itemCount 级 child  
4. OnPaint 禁 IO/改树  
5. 动画优先层 opacity/transform  
6. 浮层→Overlay  
7. 海量图元→独立 Canvas 层，禁万 RO  

---

## 2. W0 重新评审（正确性 · FullPaint）

### 2.1 原宣称

> `PaintPresentTree` 稳态全树 `FlushPaint`；单元测试 `SteadyFullPaintKeepsStatic`；示例静态可见。标 **✅**。

### 2.2 对照「真窗 + 指标 + 效果」——**未闭环**

| 项 | 状态 | 说明 |
|----|------|------|
| 代码：稳态全树 paint | **有** | `paintForce := force \|\| !compositeOnly` |
| 代码：`PresentPolicy` 枚举/JSON 字段 | **无** | 无法从指标读出当前策略 |
| CPU 单测静态保留 | **有** | `present_damage_test`（**非** GPU 真窗） |
| **真窗** 自动 FAIL（静态像素/内容门禁） | **无** | geometry 等只人工看；无 `FAIL: static missing` |
| 真窗指标含 `present_policy=full_paint` | **无** | |
| 首帧/WarmUp/resize full 路径窗测门禁 | **弱** | 有 WarmUp 代码，无专用门禁 |
| 文档曾标 W0 ✅ | **过声称** | 主路径修复 ≠ 能力关闭 |

### 2.3 W0 修订结论

| 字段 | 修订后 |
|------|--------|
| **状态** | **🔄 主路径已改 · 真窗门禁未闭环**（**不得**再写 W0✅ 完成） |
| **范围** | 正确性：GPU Present 下自定义/示例树 **静态+动态皆可见**；策略名为 FullPaint |
| **非范围** | 省绘、Retained、damage≪全屏（那是 W2+） |

### 2.4 W0 关闭清单（必须做完才标 ✅）

| # | 交付 | 真窗 | 指标 | 效果 |
|---|------|------|------|------|
| W0.1 | `PresentPolicy`（或等价）默认 `full_paint`，进 Metrics/JSON | 所有 PipelineApp 窗例 | `present_policy` 字段 | — |
| W0.2 | 专用示例 `examples/ui_widget_render_w0`（或扩 geometry）：**静色块 + 动画块** | **必须** `go run` GPU | JSON 含 policy/paint_count/presents | 静块始终在；动块变 |
| W0.3 | 门禁：`presents≥N`；可选像素/区域采样或「关键色计数」；失败 `FAIL:` exit 1 | 真窗 | stderr 明确 | 人眼：静红/动蓝不丢 |
| W0.4 | resize 一帧后仍满内容（force full） | 真窗或脚本 | `present_mode` 含 full | 无黑块 |
| W0.5 | 文档/README：W0 跑法 + 通过标准 | — | — | — |
| W0.6 | 单测保留 CPU 回归；**注明不能替代 W0.2** | — | — | — |

**W0 不做：** Boundary 缓存、damage 变小、CompositeOnly 默认开。

### 2.5 W0 与 §R 映射

W0 直接关闭：**正确性底座**（支撑后续一切真窗）。  
关联 R16 首帧；为 R4 的 FullPaint 默认提供可观测策略名（R12）。

---

## 3. 域与 L0（压缩 · 非施工单）

**域 G0–G17 / X1–X10** 仅作需求地图（录入 G7、浮层 G10、滚动 G12、窗壳 G14 等）。  
本阶段 **不** 按域做 Kit；只保证 §R 宿主够用。

**L0：** 三平台窗/DPR/事件/IME SPI/多窗 — **预留**；真窗测本阶段 **Linux GPU** 即可，CI 可声明平台。

---

## 4. 模块落点

| 能力 | 包 |
|------|-----|
| Present 策略 + PaintPresentTree | `ui/embedder` |
| 脏 layout/paint、VirtualList | `ui/rendering` |
| Picture、Layer、Composite | `ui/scene` |
| Overlay | `ui/overlay` |
| 指标 | `ui/scheduler` |
| **真窗示例** | `examples/ui_widget_render_*`（**禁止**进 `ui/`） |

---

## 5. 分期（每 W = 能力子集 + 真窗包）

### 5.1 总则

```text
关闭任一 W 必须：
  ① 代码  ② examples 真窗可跑  ③ 指标门禁 exit 码  ④ 效果写进该例 README
  ⑤ 回写本节状态表
```

### 5.2 状态表

| W | 状态 | 关闭的 §R | 真窗包（最低） | 核心指标门禁 |
|---|------|-----------|----------------|--------------|
| **W0** | **🔄 重开关闭清单 §2.4** | 正确性 FullPaint | `ui_widget_render_w0` | policy=full_paint；静+动皆在；presents≥N |
| **W1** | ⬜ | R3 R5 R11(部分) R12 | `ui_widget_render_w1_boundary` | rerecord 仅脏；skip>0；静块不变 |
| **W2** | ⬜ | R4 R5 R13 | `…_w2_layer` | retained 下 damage/surface < 阈值；静在 |
| **W3** | ⬜ | R7 R10 | `…_w3_virtlist` | bind≪item_count；p95/hitch 预算 |
| **W4** | ⬜ | R8 | `…_w4_overlay` | 开 overlay 后主 paint 不涨；底仍在 |
| **W5** | ⬜ | R6 | `…_w5_anim` | 动画期 layout 不风暴；hitch 可控 |
| **W6** | ⬜ | R12 R14 默认 Retained | 回归跑 W0–W5 全套 | 默认 policy=retained；预算不炸 |

### 5.3 依赖

```text
W0 真窗门禁闭环
  └─ W1 Boundary 缓存真窗
       ├─ W2 层 Present 真窗 ── W6 默认 Retained + 全套回归
       │     ├─ W4 Overlay 真窗
       │     └─ W5 合成动画真窗
       └─ W3 虚拟化真窗（可与 W2 末并行）
```

### 5.4 迭代

- 每轮 **只关一个 W**（或 W0 关闭清单中的一条）。  
- **先 W0 按 §2.4 闭环**，再 W1。  
- 熔断：无真窗就标 ✅；CompositeOnly+Clear 装省绘；列表万 RO。

### 5.5 90 天（修订）

```text
0–15d   W0 真窗门禁闭环（§2.4）→ 标 W0 ✅
15–45d  W1 Boundary 真窗+指标
45–75d  W2 层 Present 真窗
75–90d  W2 收口或 W3 启动
```

---

## 6. 宣称

```text
允许：有真窗+指标证据时，描述该 W/§R 已达门禁
禁止：无真窗关闭 W；FullPaint 冒充 Retained 省绘；无 baseline 谈更顺
```

---

## 7. 开放问题

| ID | 问题 | 倾向 |
|----|------|------|
| Q1 | W1 缓存先 Picture 还是 RT？ | MVP Picture replay；再 RT |
| Q2 | 真窗像素采样 vs 仅指标？ | W0 至少「双区域存在性」；W1+ 加 rerecord |
| Q3 | CI 无显示？ | xvfb 或标 `needs_gpu_window`；禁止假绿 |

---

## 8. 修订

| 版本 | 说明 |
|------|------|
| **2.1** | **回归检查**：去冗；§R 每项绑真窗+指标+效果；**W0 降为 🔄 并给关闭清单**；W 表强制真窗包 |
| 2.0 | 四册合一 |
| ≤1.x | 分册演进 |

---

## 9. 一句话

> **每个 §R/W 都必须真窗可跑、指标可 FAIL、效果可看。**  
> **W0 代码已修路径，但真窗门禁未闭环 → 先做完 §2.4 再 W1。**
