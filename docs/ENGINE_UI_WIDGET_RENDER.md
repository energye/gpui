# 控件工业级渲染基座 — Flutter 对齐设计

> **版本：1.0** | 日期：2026-07-28  
> **状态：设计真源（待分期实现）**  
> **范围：** 使 **底层渲染/管线基座** 达到可承载 **工业级 UI 控件库 / 自定义控件** 的渲染性能与帧语义，观感目标对齐 **Flutter 原生丝滑**（稳帧、局部脏、滚动与分层合成）。  
> **本阶段明确后置：** IME、完整键盘路由、焦点产品化、读屏生态（可留 hook，不挡渲染主线）。  
> **交叉：**  
> - 能力母表 [`ENGINE_UI_RENDER_BASE.md`](./ENGINE_UI_RENDER_BASE.md)（§22 主路径已收口 · §25 终审）  
> - 架构真源 [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)（F 契约 · 四层）  
> - 总览 [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)  
> - L2 壳 [`ENGINE_PHASE_P5.md`](./ENGINE_PHASE_P5.md) · L3 控件后置 [`antd/`](./antd/) · P4–P7 [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)  
> - 纪律 [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)

---

## 0. 可行性结论（先回答「是否可行」）

| 问题 | 结论 |
|------|------|
| **是否可行？** | **可行。** 与 Flutter 同构的路径清晰：`Pipeline → RenderObject → Layer/Picture → Raster → Composite → Present`，gpui 已具备 L1 主路径与 L2 骨架，缺的是 **retained 默走与边界缓存**，不是从零发明模型。 |
| **能否 1:1 抄 Flutter？** | **不必、也不应。** 对齐的是 **帧语义与性能经济学**（脏区、层、合成、虚拟列表），不是 Dart Widget/Element 语法或 Impeller 全量。 |
| **与「渲染基座 §22 收口」关系？** | §22 = **能画/能滚/可观测** 的引擎主路径。本文 = **控件工业级渲染性能** 的下一层真源；**不否定** 收口，**升级** 合成与脏区。 |
| **最大风险？** | ① 在「半 retained」下宣传 dirty 60fps（已有 LoadOpClear 教训）；② 用万级 RO 扛列表；③ 未分层就做重特效。 |
| **成功判据？** | 见 **§11 门禁**：列表滚、Tab 切、Dialog 开、主题切、单 boundary hover 的 **p95/hitch/layout/paint/damage** 可证伪，而非「API 多」。 |

```text
可行路径（摘要）：
  短：Present 契约正确（Clear 与跳过静态不可并存）
  中：RepaintBoundary → RT/Picture 缓存 + Pipeline 层 Present
  稳：壳/内容/Overlay 分层 + 滚动 offset 合成 + 主题 revision 脏
  后：IME/键盘/完整 a11y（不挡本设计主线）
```

---

## 1. 目标与非目标

### 1.1 目标

使底层基座达到：

1. **任意** 符合本设计的 **库控件 / 自定义控件**，在正确使用 Boundary/虚拟列表/分层约定时，可达到接近 Flutter 的 **渲染侧** 体验：跟手、稳帧、静态不闪、滚动与弹层不拖垮整树。  
2. 覆盖控件域的 **渲染需求**（先不实现全部 L3 皮肤）：

| 域 | 产品含义（L3 将来） | 本设计管的「渲染」 |
|----|-------------------|-------------------|
| **基础** | Button、Text、Icon、Switch… | 状态变 → 仅 boundary 重绘 |
| **导航** | AppBar、Tab、Drawer、页转场 | 壳/内容分层；转场走合成 |
| **反馈** | Dialog、SnackBar、Tooltip、遮罩 | Overlay 独层；主树可冻结 |
| **数据** | List、Table、树表 | 虚拟化 + cell 缓存 + 滚偏移 |
| **布局** | Flex/Stack/展开折叠 | 局部 layout；与 paint 解耦 |
| **主题** | 色/字/圆角/密度/暗色 | revision → 最小脏集 |

3. **架构对齐 Flutter 设计**（§3）：同一套脏区/层/合成思想，映射到 gpui 模块，不引入 `ui → gpu`。

### 1.2 非目标（本文件）

| 非目标 | 说明 |
|--------|------|
| 实现完整 Ant/Material 控件集 | L3 / `docs/antd` · P7 |
| IME、TextEditing、系统输入法 | 后置；可留 Focus/Semantics hook |
| 完整键盘快捷键/菜单加速键产品 | 后置 |
| 10 万自定义图元合批 | **另一轨道**（Multishape）；可与本设计共享 Composite，数据面分离 |
| 母表 B/C/D 清零 | 对照全集 ≠ 本设计必做 |
| 宣称「已对齐 Flutter 全量」 | 仅对齐 **控件渲染帧语义**；见禁止宣称 §12 |

### 1.3 与现有文档分工

| 文档 | 管什么 |
|------|--------|
| `ENGINE_UI_RENDER_BASE` | Canvas/Path/Text/Image… **画什么** + §22 依赖序 |
| **本文** | 控件场景下 **怎么画得省、怎么合成、怎么分层** |
| `ENGINE_FLUTTER_SKIA_ARCH` | 全局 F 契约与模块依赖 |
| `ENGINE_PHASE_P5` / P7 | L2 交互壳 / L3 控件波次 |
| `antd/` | 具体控件需求与皮肤 |

---

## 2. 问题陈述：现状 vs 工业级控件渲染

### 2.1 已有（可依赖）

| 能力 | 位置 | 备注 |
|------|------|------|
| RO / Pipeline / 脏标记 | `ui/rendering` | S2/S4 门禁思想 |
| PaintContext + draw 主路径 | rendering + render | 几何/文/图/clip/filter 主项 |
| RepaintBoundary **标志** | RO | **缺稳定离屏缓存默走** |
| Layer 类型 + CompositeToContext | `ui/scene` | **PipelineApp 默认仍 RO 直绘** |
| Picture 显示列表子集 | scene | rect/path/text/image；非 GPU 缓存 |
| VirtualList + clamp fling | rendering | 产品默认与 cell 缓存仍要夯 |
| Overlay band | `ui/overlay` | 机制有；与主树 damage 隔离弱 |
| 主题 Tokens/Provider | `ui/theme` | 骨架；无 revision 脏模型 |
| 调度/指标/dirty Present 契约 | scheduler · embedder | Present **A/C**；矢量 Clear 问题见下 |
| 异步 Present | raster.Loop | 不堵 UI |

### 2.2 核心断裂（必须写进设计）

```text
Flutter：脏 boundary → 重录/重栅格该层 → 合成器 blit（表面 Load 友好）
gpui 现状（默路径）：
  稳态 CompositeOnly 跳过干净 RO
  + GPU 矢量帧 MSAA 常 LoadOpClear
  = 静态被跳过又被清屏 → 内容丢
  或被迫全树重绘 → 「脏区」名存实亡
```

**结论：** 控件工业级渲染的第一矛盾不是 API 少，而是 **半 retained 帧语义**。  
本文全部后续设计都服务于：**结束半 retained，进入真 retained（或明确的 FullPaint 开发策略）。**

### 2.3 性能经济学（控件页）

| 反模式 | 后果 | 正模式 |
|--------|------|--------|
| 每控件每帧矢量 Fill | CPU/GPU 与状态切换爆 | Boundary 纹理 + 合成 |
| 列表 itemCount 个 RO 全挂载 | walk/layout 爆 | VirtualList 只挂视口 |
| 开 Dialog 重绘整棵主树 | 卡顿 | 主树 bake + Overlay 独绘 |
| 主题切换全树无差别 layout+paint | 尖峰 | revision 脏集 / 壳层 recolor |
| 滚动每帧重 layout 全列表 | 掉帧 | offset 合成 + cell 缓存 |
| 无 Boundary 的深树动画 | 全树 paint | 动画优先 opacity/transform 层 |

---

## 3. Flutter 架构对齐（设计映射）

### 3.1 Flutter 一帧（控件相关，简化）

```text
Scheduler（Vsync）
  → Build（Widget，gpui 可无 1:1）
  → Layout（RenderObject，脏子树）
  → Paint（录制 Layer/Picture，非每帧打到屏幕像素）
  → 合成 / Raster（层纹理、DisplayList）
  → Present
```

### 3.2 gpui 目标一帧（对齐语义）

```text
FrameScheduler.WaitFramePace（真 VSync 优先）
  → PipelineOwner.FlushLayout（仅 layout 脏子树）
  → 对每个 paint 脏的 RepaintBoundary：
        Record(Picture) 和/或 Rasterize → Layer 纹理
  → 干净 Boundary：复用上次纹理 / 跳过 record
  → scene.Composite（blit + opacity/transform/clip 层）
  → PresentTarget.PresentWithAuto（LoadOpLoad 为主）
  → Metrics（layout/paint/damage/hitch/gpu）
```

| Flutter | gpui 映射 | 对齐要点 |
|---------|----------|----------|
| SchedulerBinding / Vsync | `ui/scheduler` + Host VSync | `vsync_source` 诚实 |
| WidgetsFlutterBinding | 无强制 Widget；**命令式 RO + 将来控件封装** | 帧相位仍分 layout/paint/composite |
| RenderObject | `ui/rendering` RO | NeedsLayout/NeedsPaint/Boundary |
| Layer tree | `ui/scene` Layer | Offset/Transform/Opacity/Clip/Picture/… |
| Picture / DisplayList | `scene.Picture` + 扩展 | 录制与 GPU 缓存 |
| Rasterizer / Compositor | raster 线程 + Composite + render Present | **禁止 ui→gpu** |
| RepaintBoundary | RO 标志 + **RT/纹理义务** | 标志必须带缓存语义 |
| RepaintBoundary 子树 | 独立 dirty 与独立 layer id | FramePacket.DirtyLayerIDs |
| Overlay | `ui/overlay` | 高于 main 的 band；独立 damage |
| Scrollable / Viewport / Sliver | Viewport + VirtualList | 成本 ∝ 视口 |
| Theme | `ui/theme` + **revision** | 依赖传播脏 |

### 3.3 明确不 1:1 的部分（诚实）

| Flutter | gpui 选择 | 理由 |
|---------|----------|------|
| Widget/Element/BuildOwner | 命令式 RO + 控件封装层（未来） | 已有 L1 模型；避免双框架 |
| Impeller/Skia 全后端 | render + wgpu | 已定 |
| 完整 Material 3 | L3/主题后置 | 本设计只保渲染底座 |
| PlatformView | 远期 D | 不挡控件主路径 |
| 多窗口 Engine | 单窗先 | 可扩 PresentTarget |

### 3.4 与 F 契约的关系（`ENGINE_FLUTTER_SKIA_ARCH`）

实现本设计时 **不得削弱** 已有 F 门禁方向（脏区、Boundary、滚动 bind、动画 IDLE 等）。  
新增门禁见 §11；与 F09（compositor-only 动画）、F 滚动/列表条款交叉处，以 **更严的可证伪指标** 为准。

---

## 4. 目标架构

### 4.1 逻辑分层（窗口内）

```text
┌─────────────────────────────────────────────────────────┐
│  Host / VSync / 指针命中（事件后置增强，命中几何本设计要稳）   │
├─────────────────────────────────────────────────────────┤
│  PipelineApp                                            │
│    Layout → Record/RasterizeDirty → Composite → Present │
├──────────────┬──────────────────────┬───────────────────┤
│  Shell 层    │  Content 滚动层       │  Overlay 反馈层    │
│  AppBar/Nav  │  页面 + 数据区        │  Dialog/Snack/Tip │
│  低频脏      │  VirtualList+cells   │  独立 damage      │
│  宜整块 bake │  cell boundary 缓存  │  主树可冻结纹理    │
├──────────────┴──────────────────────┴───────────────────┤
│  scene.Layer 树 + Picture/Texture 缓存                   │
├─────────────────────────────────────────────────────────┤
│  render.Context / PresentTarget（LoadOpLoad 合成路径）    │
└─────────────────────────────────────────────────────────┘
```

**自定义控件** 只挂在某一层的 RO 子树上，遵守 Boundary/虚拟化约定，即可继承同一性能模型。

### 4.2 数据流（真 retained）

```text
RO.markNeedsPaint(boundary)
  → Pipeline 收集 dirty boundaries
  → 对每个 dirty：
       Paint 进 PictureRecorder 或直接栅格到 Layer 纹理
  → FramePacket{ layers, DirtyLayerIDs }
  → RasterizeDirty（仅脏层）
  → CompositeToContext / GPU blit 合成
  → PresentFrameAuto（damage = 脏层屏幕投影）
```

### 4.3 Present 策略（产品必须可配）

| 策略 | 行为 | 何时用 |
|------|------|--------|
| **`FullPaint`** | 每帧（或每 present）全树 paint；可仍尝试 damage 指标 | 开发早期；retained 未就绪时 **默认保正确** |
| **`Retained`** | 仅脏 boundary 重录/重栅格；Composite blit；LoadOpLoad | **产品目标默认** |
| **`Hybrid`** | 壳 Full/ bake；内容 Retained；Overlay 独立 | 过渡期推荐 |

**硬规则：**

```text
禁止：Retained 语义下「CompositeOnly 跳过静态」+「表面 LoadOpClear 矢量直绘」并存。
若本帧 GPU 路径会 Clear 且无层纹理可 blit：必须 FullPaint 该帧或升级为层纹理路径。
```

### 4.4 RepaintBoundary 契约（对齐 Flutter）

| 义务 | 说明 |
|------|------|
| **独立 layer 身份** | 稳定 LayerID；进 DirtyLayerIDs |
| **缓存** | 至少一种：Picture 可 replay **或** GPU/CPU 纹理 |
| **子树 clip/transform** | 录在 boundary 内或父层，合成结果一致 |
| **脏传播** | 子脏默认停在 boundary（除非强制） |
| **尺寸变化** | 丢弃缓存并重录 |
| **设备像素比变化** | 丢弃缓存 |

无缓存的「仅布尔 Boundary」**不得** 称为工业级完成。

### 4.5 与「动态图 / 海量图元」边界（预留，不展开实现）

```text
控件树：RO + Boundary + 虚拟列表
海量图：单一 MultishapeLayer / CanvasHost（SoA + 合批）
二者在 Composite 汇合；禁止把 10 万 shape 做成 10 万 RO。
```

本文不详细设计 Multishape；仅要求 **Layer 接口可挂接**。

---

## 5. 分域渲染需求（基础 / 导航 / 反馈 / 数据 / 布局 / 主题）

每一域：**场景 → 帧成本风险 → 基座必须提供 → 验收意向**。

### 5.1 基础（Basic）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| 单击/hover 变色 | 全树重绘 | Boundary 缓存；仅该节点 paint++ | paint 次数不随无关子树涨 |
| 图标+文字按钮 | 文本测量贵 | 文本 layout 缓存（同 style/string） | 重复 frame 无重复 shape 风暴 |
| 圆角裁切子内容 | clip 每帧重算 | ClipRRect 层或 boundary 内 clip 进缓存 | 像素正确 + 脏局部 |
| 按压缩放 | 重录整树 | Transform 层合成（少重录） | 动画期 raster 可解释 |
| 禁用态 | 误 layout | 纯 paint 态不 MarkNeedsLayout | layout_count 不动 |

**自定义基础控件清单（作者纪律）：** 可交互热区必须是 RepaintBoundary 或落在最近 Boundary 内；禁止无 Boundary 的整页 `OnPaint` 大画布冒充控件树（海量图另轨）。

### 5.2 导航（Navigation）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| 固定 AppBar + 滚内容 | 顶栏每帧重绘 | **Shell 层** 与 **Content 层** 分离缓存 | 滚动时 shell paint≈0 |
| Tab 切换 | 两页同时全绘 | 页级 boundary/RT；离屏页可 keep 纹理或丢弃策略可配 | 切换 hitch 可预算 |
| Drawer 滑入 | 背后列表矢量重绘 | 背后 bake 或 transform 合成 | 滑入期列表 layout 不风暴 |
| 页 push/pop 转场 | 两棵大树双绘 | 页层 opacity/slide 合成 | compositor-only 方向 |
| 底栏/导航轨 | 与内容抢脏 | 独立 boundary | 内容滚底栏不脏 |

**层模型约定：**

```text
Scaffold-like：
  ShellLayer (AppBar/NavRail/BottomBar)  — 低频
  BodyLayer (Navigator stack / 当前页)   — 中频
  每页 ContentLayer + 可选 NestedScroll
```

### 5.3 反馈（Feedback）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| Modal 遮罩 | 主树全重绘 | Overlay 层；主树 **冻结纹理** 或 skip paint | 打开后 main paint 不涨 |
| Dialog 缩放淡入 | 每帧重录 dialog+主树 | Dialog 自 boundary + 合成动画 | 同左 |
| SnackBar | 底部插入触发布局抖 | Overlay 不强迫 body relayout（或仅 padding 动画层） | layout 局部 |
| Tooltip | 定位错/闪 | 锚点几何（可后 Leader/Follower）；轻量层 | 不脏整表 |
| 多浮层 | z 与 damage 乱 | band 序 + 每 entry damage | hit 与绘序一致 |

**硬规则：** Overlay 可见时，主内容默认 **不得** 因「每帧全清」而矢量重画；若引擎暂不能冻结，必须文档降级为 FullPaint 并计为 **未达标**。

### 5.4 数据（Data）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| 1万+ 行列表 | RO/walk 爆 | **VirtualList 强制**；禁止全挂载 | bind_count ≪ itemCount |
| 滚动 60fps | 每帧重绘所有 cell | cell RepaintBoundary 缓存 + **滚动偏移合成/clip** | p95、hitch、damage |
| 不等高 | 滚动跳 | 前缀和 index↔offset（已有）打磨 | ScrollToIndex 正确 |
| 行 hover | 整表重绘 | 行级 boundary | 单行 damage |
| 表头冻结 | 复杂 | sticky 子层（P2） | 横滚/纵滚头不丢 |
| 复杂 cell | cell 内过重 | cell 内子 boundary 或 bake | cell paint 预算 |
| 复用抖动 | 布局闪 | 回收稳定 size；避免复用时全 layout | layout 平稳 |

**数据域是本设计第一性能战场。** 门禁场景必须进 §11。

### 5.5 布局（Layout）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| 改一处 padding | 整树 layout | **NeedsLayout 局部传播** | layout 节点数可测 |
| 展开折叠 | 连锁 layout+paint | 高度动画优先 clip/层；减少全树 | 动画期 layout 可控 |
| Flex 重算 | 深树多次 layout | 约束缓存/early-out（深化） | 同输入不重复 layout |
| Stack | 绘制序与 hit | 与层序一致 | 单测 |
| 文本固有高 | 每 frame measure | measure 缓存 | 同文同 style 命中缓存 |

**Layout 与 Paint 解耦：** 纯视觉态禁止 `MarkNeedsLayout`；主题密度/字号变更允许 layout，但应走 revision 批量。

### 5.6 主题（Theme）

| 场景 | 风险 | 基座必须 | 验收意向 |
|------|------|----------|----------|
| 切换暗色 | 全树 layout+paint 尖峰 | **theme generation / revision**；依赖子树脏 | 单次切换可预算；非常驻 |
| token 微调 | 无关控件重绘 | 控件声明依赖的 token 键（或整页 cheap） | 脏集可解释 |
| 密度/字号 | 触 layout | 批量 MarkNeedsLayout 一次 flush | 一次切换一次 layout 波 |
| 运行时动态主题 | 每帧读全局 | 只读 snapshot；禁止 paint 里随机数 | 无额外帧抖动 |

**两种实现策略（可选并存）：**

1. **精细：** token → 依赖图 → 脏 paint/layout  
2. **粗但稳：** 整 Shell/Content 一层在切换时重录一次（可接受尖峰），之后 retained  

工业级至少落地 **一种** 并有指标。

---

## 6. 你可能没考虑到的（补全）

以下易在「只想控件渲染性能」时漏掉，但会直接打穿丝滑或可维护性。

### 6.1 帧与呈现

| 项 | 为何重要 |
|----|----------|
| **首帧 / 热身** | 控件库启动白屏与 jank；需 warm-up 与 time-to-first-present 指标 |
| **resize / DPR 变化** | 全缓存失效策略；强制 full present |
| **后台/最小化恢复** | 表面丢失重录 |
| **多 monitor / 可变刷新率** | 120Hz 预算与 hitch 阈值；勿写死 16.7 |
| **Present 合并（SubmitLatest）** | 输入后置后仍影响「跟手」；合并策略要可观测 |

### 6.2 缓存正确性

| 项 | 为何重要 |
|----|----------|
| **缓存失效全集** | 尺寸、DPR、主题 revision、locale（后）、子树结构变化 |
| **图片 decode 异步** | 列表滚动时占位 → 解码完成只脏该 cell |
| **字体回退/热加载** | 测量变了必须丢文本缓存 |
| **滤镜/阴影** | 易逼 SaveLayer 全幅；默认规范：能 bake 进 boundary 则 bake |
| **半透明叠层** | 打断合批与缓存；文档化 cost |

### 6.3 命中与绘制一致（事件后置也要预留）

| 项 | 为何重要 |
|----|----------|
| **Hit 与 transform/clip 一致** | 已有逆 CTM；层路径后必须仍成立 |
| **Overlay 命中序** | 与绘制 z 一致 |
| **滚动中 hit** | 坐标随 offset |

### 6.4 内存与泄漏

| 项 | 为何重要 |
|----|----------|
| **层纹理预算** | Boundary 过多 → VRAM/RSS；需上限与淘汰（LRU/距视口） |
| **Picture/Image 生命周期** | 与 dispose 纪律一致 |
| **列表快速滑** | 缓存 cell 纹理要回收 |

### 6.5 并发与线程

| 项 | 为何重要 |
|----|----------|
| **UI vs raster** | 录制与上传可在 raster；RO 树所有权规则要写死 |
| **禁止 UI 读回 GPU** | 卡顿源 |
| **COW FramePacket** | 已有方向；层纹理句柄跨线程安全 |

### 6.6 可访问性与语义（渲染相邻）

| 项 | 为何重要 |
|----|----------|
| **Semantics 树与绘制边界** | 后置读屏，但 role/label 骨架勿与层模型冲突 |
| **焦点环绘制** | 输入后置，但 ring 是 **paint 层** 需求，宜预留 boundary |

### 6.7 测试与假绿

| 项 | 为何重要 |
|----|----------|
| **禁止降画质装 60fps** | 与母表 J 一致 |
| **CPU 单测 ≠ GPU Present** | 已有 Clear 教训；窗测+GPU 路径必测 |
| **场景 baseline** | 同机同场景；D8 工具可用 |

### 6.8 自定义控件 API 纪律（库作者）

| 项 | 为何重要 |
|----|----------|
| **何时 MarkNeedsPaint vs Layout** | 写进控件作者指南（可附本文 §10） |
| **禁止 paint 里做 IO/解码** | 掉帧 |
| **禁止 paint 里改树结构** | 重入 |
| **测量缓存键** | 文本/图标 |

### 6.9 平台与窗口

| 项 | 为何重要 |
|----|----------|
| **X11/Wayland  damage** | OS damage 可忽略时的引擎自洽 |
| **透明窗口/圆角窗** | 特殊 Clear 策略 |
| **HiDPI 非整数 scale** | 缓存与模糊 |

### 6.10 产品节奏

| 项 | 为何重要 |
|----|----------|
| **FullPaint 默认可开发控件** | 不阻塞 L3 视觉并行 |
| **Retained 作为发布默认** | 门禁达标才切 |
| **双轨指标** | 开发态 vs 发布态勿混谈「已丝滑」 |

---

## 7. 模块落点（实现映射）

| 能力 | 主模块 | 说明 |
|------|--------|------|
| Present 策略 Full/Retained/Hybrid | `ui/embedder` | PipelineOptions |
| FlushLayout 局部 | `ui/rendering` PipelineOwner | 传播规则文档化 |
| Boundary → 缓存 | rendering + scene | Picture 和/或纹理句柄 |
| Record 接线 | rendering.BuildLayerTree 深化 | 脏 RO → PictureLayer |
| Composite 默走 | embedder + scene | 替换/并行 RO 直绘 |
| VirtualList + cell 缓存 | rendering | 与 Boundary 结合 |
| Overlay 冻结主树 | overlay + embedder | 策略位 |
| 主题 revision | theme + rendering | generation 计数 |
| 合成动画 | animation + scene Opacity/Transform | F09 方向 |
| 指标 | scheduler | §11 ID |
| 窗测 | `examples/ui_widget_render_*`（新建，不进 ui/） | 纪律 |

**依赖硬规则不变：** `ui → render → gpu`；禁止 CGO。

---

## 8. 分期实现（建议真源序）

> 可与仓库 Phase 编号交叉；**本文序是控件渲染性能专用**，不自动改写 P7 Ant 皮肤序。

### 阶段 W0 — 正确性（阻塞一切「丝滑」宣称）

| 项 | 交付 | DoD |
|----|------|-----|
| Present 契约 | Clear 与 skip 静态不可并存；策略枚举 | 示例静态内容不再丢；单测 + 窗测 |
| FullPaint 默认 | 产品/示例可开发 | 文档写明性能非最终 |
| 回归 | GPU 路径测试或文档强制窗测 | 禁止仅 CPU 假绿 |

**W0 落地（2026-07-28）：** `embedder.PaintPresentTree` 稳态帧改为 **全树 FlushPaint**（不再默认 CompositeOnly）。`PaintPresentTreeCompositeOnly` 保留给单测/未来 Retained。证据：`present_damage_test` · 各 `ui_render_base_*` 窗测。

### 阶段 W1 — Boundary 缓存 MVP

| 项 | 交付 | DoD |
|----|------|-----|
| 脏 boundary → Picture 或 RT | 至少一种缓存 | hover 单测：仅该 boundary 重录 |
| 尺寸/DPR 失效 | 自动 | 单测 |
| 指标 | re-record 次数 / skip 次数 | JSON 可见 |

### 阶段 W2 — 层 Present 接线

| 项 | 交付 | DoD |
|----|------|-----|
| Pipeline 可选/默认 Record→Composite | 与 FullPaint 可切换 | 同场景 Retained 下 damage≪全屏（有层纹理时） |
| DirtyLayerIDs 驱动 Rasterize | 已有加深 | S4 类门禁 |
| 异步路径不破缓存所有权 | 规则文档 | 无 use-after-free |

### 阶段 W3 — 数据域（列表）

| 项 | 交付 | DoD |
|----|------|-----|
| VirtualList + cell Boundary 缓存 | 默认推荐路径 | itemCount=10k，bind≪N |
| 滚动偏移合成/clip | 减少 cell 重录 | 滚 soak p95/hitch |
| 图片 cell 异步 | 解码完成局部脏 | 滚动不爆 RSS 无故 |

### 阶段 W4 — 壳 / Overlay / 导航

| 项 | 交付 | DoD |
|----|------|-----|
| Shell/Content 分层 | 约定 + 示例 Scaffold | 滚时 shell paint≈0 |
| Overlay 开时主树冻结或 skip | 策略 | Dialog 开 main paint 不涨 |
| 页/Tab boundary | 示例 | 切换 hitch 预算内 |

### 阶段 W5 — 主题与合成动画

| 项 | 交付 | DoD |
|----|------|-----|
| theme revision | API + 脏 | 切暗色可测尖峰一次 |
| Opacity/Transform 合成动画 | 少重录 | 转场 compositor 方向 |
| 文档：控件作者指南 | §10 独立可摘 | 评审用 |

### 阶段 W6 — 发布默认 Retained

| 项 | 交付 | DoD |
|----|------|-----|
| 默认策略切 Retained | 门禁全绿 | §11 场景集通过 |
| FullPaint 仅 debug/回退 | 配置 | 文档 |

**IME/键盘/Focus 产品：** 另相文档/Phase；本 W 序列不阻塞，但 W4 Overlay/命中需保持可扩展。

---

## 9. 与 L3 控件库的接口约定（渲染侧）

L3（Ant 等）**只允许** 依赖本设计已稳定的能力：

```text
允许：RO 组合、Boundary、主题 token、VirtualList、Overlay 插入、动画 Controller
禁止：控件内 import gpu；控件内每帧全屏 Clear；控件内挂载 itemCount 级 child
禁止：为单控件开私有 Present 旁路（破坏帧模型）
```

自定义控件与库控件 **同一套** 纪律；无特权通道。

---

## 10. 控件作者渲染纪律（摘要）

可在 L3 文档中展开；基座评审用：

1. **热更新视觉 → `MarkNeedsPaint`；改尺寸/约束 → `MarkNeedsLayout`。**  
2. **可独立刷新的块 → `SetRepaintBoundary(true)`（并接受缓存义务）。**  
3. **列表 → 只通过 VirtualList（或未来官方 List）宿主。**  
4. **`OnPaint` 禁止 IO、解码、改树、随机布局。**  
5. **动画优先 opacity/transform；避免每 tick 改 layout。**  
6. **重特效（模糊/阴影）默认 bake 进 boundary，避免每帧全幅 SaveLayer。**  
7. **主题色来自 token，不写死；依赖变更靠 revision。**  

---

## 11. 门禁与指标（可证伪）

### 11.1 场景集（窗测，放 `examples/`）

| 场景 ID | 内容 | 主看 |
|---------|------|------|
| WR-S0 | 静态 Scaffold 壳 | 稳态 paint 低 |
| WR-S1 | 单 Button hover | 单 boundary 重录 |
| WR-S2 | VirtualList 10k 滚 | bind、p95、hitch、damage |
| WR-S3 | 开 Modal | main paint 不涨 |
| WR-S4 | Tab/页切换 | hitch、重录页数 |
| WR-S5 | 主题切换一次 | 尖峰一次、之后稳 |
| WR-S6 | 嵌套滚/冻结头（W 后期） | 正确性+帧时 |

### 11.2 指标（接入 MetricsStore / JSON）

| 方向 | 指标 | 门禁意向 |
|------|------|----------|
| 帧 | p95 interval、hitch_rate、vsync_source | 动画/滚动场景达标带 |
| 管线 | layout_count、paint_count、boundary_rerecord、layer_skip | S2：动画不逼 layout |
| 脏 | damage_area_px、present_mode | Retained 下 ≪ 全屏（有证据） |
| 列表 | bind_count | ≪ itemCount |
| 资源 | RSS slope、gpu_ops、cpu_fallback | soak 不爬升、热路径少 fallback |
| 回归 | CompareToBaseline | 同机场景 |

### 11.3 正确性门禁

- GPU Present 路径下静态不丢（W0）  
- Hit 与 transform/clip 一致（已有加深保持）  
- `ui` 无 import `gpu`；无 cgo  

---

## 12. 允许 / 禁止宣称

```text
允许（有 §11 证据）：
  · 控件渲染基座按 Flutter 帧语义设计（Layout/Paint/Composite/Present）
  · Retained 策略下边界脏区与列表虚拟化达到门禁
  · 自定义控件遵守作者纪律即可获得同级渲染模型

禁止：
  · 「已对齐 Flutter 全量 / Material 全组件」
  · 无 Retained 层纹理时宣称工业级 dirty 60fps
  · vsync_source=fallback 时宣称锁显示刷新率
  · 无同机 baseline 的「更顺」
  · 用 FullPaint 开发态指标冒充 Retained 发布态
```

---

## 13. 开放问题（实现前可决议）

| ID | 问题 | 候选 |
|----|------|------|
| Q1 | Boundary 缓存首选 Picture replay 还是 GPU RT？ | MVP Picture+CPU/GPU replay；发布偏 RT blit |
| Q2 | Pipeline 默认何时切 Retained？ | W6 + §11 全绿 |
| Q3 | 主题精细依赖图 vs 整层重录？ | 先整层重录 + revision；再精细化 |
| Q4 | 与 Multishape 动态图同一 Composite？ | 是，Layer 挂接 |
| Q5 | 命令式 RO 上是否引入轻量声明式控件 DSL？ | L3 决定；本设计不依赖 |

---

## 14. 修订

| 版本 | 说明 |
|------|------|
| **1.0** | 初版：可行性、Flutter 对齐、六域渲染、补全项、架构、W0–W6 分期、门禁、宣称；IME/键盘后置 |

---

## 15. 一句话收束

> **可行：** 按 Flutter 的 Layout →（Boundary）Paint/Record → Composite → Present 把 gpui 从「能画的 L1」升到「控件工业级渲染底座」。  
> **关键路径：** 结束半 retained → Boundary 真缓存 → 层 Present → 列表/壳/Overlay 分层 → 主题 revision → 合成动画。  
> **后置：** IME/键盘。  
> **验证：** §11 场景与指标，而不是控件皮肤先堆满。
