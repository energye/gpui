# UI 框架总图与规划 — Primitive 组合底座 × Kit 产品面 × Flutter 管线 × render

> 版本：4.5 | 日期：2026-07-23 | **活文档 · 打磨 §12.1 · 测试 §12.2 · 波次执行 §12.3 · 合成带 §4.1**  
> 状态：**控件产品架构以「primitive 组合」为底座**；已含 **Ant 全量组件 → 组合能力反推清单**  
> 入口：[`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md) ✅  
> 引擎：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md) · [`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md)（控件 = **P2 另开轨道**）  
> 覆盖率权威表：[`ui/kit/coverage.go`](../ui/kit/coverage.go) · 摘要 [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)  
> App 壳：[`UI_APP_SHELL_PLAN.md`](./UI_APP_SHELL_PLAN.md)（按需帧已落地；多窗 API 仍后置）

---

## 0. 一句话目标

跨平台桌面 GUI 框架：

| 维度 | 选择 |
|------|------|
| 运行时 | **Flutter 向**管线（树·布局·Hit·Focus·帧）；Go **单树**简化 |
| **组合底座** | **`ui/primitive`**：无产品语义的通用积木（Box / Pressable / Text / EditableText…） |
| **产品控件面** | **`ui/kit`**：由 primitive **组合**出的高级控件；**默认一套对标 Ant Design** |
| 扩展 | 业务/第三方用 primitive 自组任意控件；可另做 kit；Token/Skin 换皮；插件注册 |
| 绘制 | 仅 `render.Context` + `PresentFrame*` |
| 平台 | `ui/platform` SPI；Linux 真测；Win/mac stub→真适配 |

**核心命题**

```text
core 管管道 · primitive 管积木 · kit 管产品话术与默认组装
Ant Design = 某一套 kit API + skin/default 的目标，不是底座
```

**不做**：包名 `ui/antd`；primitive 依赖 kit；Material 默认底座；Web DOM/CSS；core 内 OS 细节；任意矢量局部重绘承诺。

---

## 1. 分层（硬边界）

```
┌──────────────────────────────────────────────────────────────┐
│ L5  App / 业务 / 第三方                                        │
│     组合 primitive · 可选 kit · 自建 mykit · 插件               │
├──────────────────────────────────────────────────────────────┤
│ L4  ui/kit（产品控件面 · 默认可对标 Ant）                        │
│     Button Input Form Modal Table … = primitive 组合 + 产品 API │
├──────────────────────────────────────────────────────────────┤
│ L3c ui/skin/*（Token 解析 + 绘制/组装策略）                      │
│     skin/default 默认可为 Ant 视觉；可换整皮或单 typeID Painter   │
├──────────────────────────────────────────────────────────────┤
│ L3b ui/primitive（★ 组合底层 · 无 Button/Modal 等产品名）         │
│     Box Flex Stack Text Icon Pressable Scroll OverlayPortal…   │
├──────────────────────────────────────────────────────────────┤
│ L3a′ ui/layer（retained 合成 · host 选用）                       │
│     Compositor · LayerCache · main/overlay 双带 BlitTo         │
├──────────────────────────────────────────────────────────────┤
│ L3a ui/core（框架运行时 · 非控件）                               │
│     Node · Layout · Hit · Focus · Frame · OverlayHost          │
│     PaintMain / PaintOverlays · LayerBand · Ticker             │
├──────────────────────────────────────────────────────────────┤
│ L2  ui/platform（跨平台 SPI + Caps）                            │
├──────────────────────────────────────────────────────────────┤
│ L1  render（Context · PresentFrame* · text · lifecycle）       │
└──────────────────────────────────────────────────────────────┘
```

| 层 | 包 | 职责 | 禁止 |
|----|-----|------|------|
| L1 | `render` | 像素与 present | 布局树、控件状态机 |
| L2 | `ui/platform` | 窗口/输入/IME/剪贴板/能力 | 产品控件 API |
| L3a | `ui/core` | 管线与算法 | 具体色值、产品控件名、OS API |
| L3a′ | `ui/layer` | 离屏 base/boundary 合成 | 产品控件、OS API |
| L3b | `ui/primitive` | **通用可组合积木** | 依赖 kit、Ant 专有枚举当唯一 API |
| L3c | `ui/skin/*` | 外观与组装绘制 | 自管 swapchain |
| L4 | `ui/kit` | 产品级控件 API | 硬编码色、绕过 primitive 堆业务绘制、包名 antd |
| L5 | app | 页面与扩展 | 绕过 core 直接 GPU（除 host 引导） |

**依赖方向（只允许向下）**

```text
app → kit → primitive → core → render
 app → primitive → core
 app → layer → core → render     // retained 合成（exboot / 真窗）
      skin → primitive + core
      * → platform SPI
```

禁止：`primitive → kit`；`core → primitive/kit`；`render → ui`；`kit/core/primitive → X11/Win32/AppKit`。

**绘制铁律**：只 `render.Context`；只 `PresentFrame*`；禁止 silent CPU 冒充 GPU；布局/Hit/Focus/IME ≠ ENGINE_GAPS。

---

## 2. 包路径与目录

```text
<module>/ui/core
<module>/ui/layer              # retained 合成（Compositor · LayerCache）
<module>/ui/primitive          # 组合底层（★）
<module>/ui/kit                # 产品面（默认可 Ant 向）
<module>/ui/platform
<module>/ui/platform/linux|windows|darwin
<module>/ui/app                # 按需帧会话入口（见 UI_APP_SHELL_PLAN）
<module>/ui/skin/default       # 默认可 Ant 视觉
<module>/ui/skin/<name>
```

| 规则 | 说明 |
|------|------|
| 禁止 | `ui/antd` |
| 应用默认 | `kit` + `theme.Default()`（内聚 primitive 组装 + default skin） |
| 重度自定义 | 主要 import `primitive`，可不依赖 kit |
| 真窗 present | `ui/app` + `ui/layer.Compositor`（或等价双带 Blit）；见 §4.1 |
| 类型 ID | `primitive.Pressable`、`kit.Button`（稳定字符串，与路径解耦） |

```text
ui/
  core/         node layout hit focus event frame paint theme plugin · OverlayHost
  layer/        compositor cache · main/overlay 双带
  primitive/    box flex stack text icon pressable scroll overlay_portal mask ...
  kit/          button input form modal table menu ...   # 内部只组合 primitive
  app/          Application · Session · demand loop
  platform/     host caps headless ...
  skin/default/ tokens painters ...
```

---

## 3. 设计原则（控件产品）

1. **积木先于产品**：先 primitive 可用，再 kit；kit 不能拥有 primitive 做不到的 Hit/Paint 魔法。  
2. **产品名不进底层**：无 `PrimaryButton` primitive；只有 `Pressable` + 样式映射。  
3. **组合公式**（见 §6）：高级控件 = 布局积木 + 交互积木 + 内容积木 + 产品状态机 + 友好 Props。  
4. **换皮 ≠ 换积木**：Token/Skin 换外观；换产品语义才换 kit 或自建组合。  
5. **Ant 是目标之一**：默认 kit+skin 对标 Ant 能力面与视觉；**允许**零 Ant 只用 primitive。  
6. **扩展四级**：组合 → 项目封装 → 自建 kit 包 → 极少数新 primitive（须评审）。  
7. **单树 + 单次 layout pass**；交互态在 primitive；`loading`/`validate` 默认在 kit。

---

## 4. Flutter 映射（管线参考）

| Flutter | 本框架 |
|---------|--------|
| Widget 配置 | Props / 结构体；不强制不可变每帧新建 |
| Element + RenderObject | **单树 Node**（Layout+Paint+Hit 合并） |
| Row/Column/Stack | **primitive** Flex/Stack（算法可在 core） |
| GestureDetector / InkWell | **primitive.Pressable** |
| EditableText | **primitive.EditableText** |
| Material Button 等 | **kit.*** 组合，非底层 |
| Theme / ThemeExtension | TokenSet + Theme；产品 type→Token 在 kit |
| CustomPainter / 自定义 RO | PainterNode 或实现 Node 全契约 |
| Overlay | primitive.OverlayPortal + OverlayHost + kit Modal 等（合成见 §4.1） |
| RepaintBoundary | primitive.RepaintBoundary / ScrollViewport；Paint 脏隔离 |
| 替换官方 Button 工厂 | 非 Flutter 强项；本框架可选显式 `ReplaceControl` |

**帧顺序**

```text
PumpEvents → Dispatch → flush setState
  → Layout(仅 needsLayout / 约束变化) → Paint
  → PresentFrame*（按需；无 dirty/ticker/expose 则不 Present）
```

**脏区 / 按需（Flutter 向 · 2026-07-22）**

| 机制 | 行为 |
|------|------|
| `MarkNeedsLayout` | 气泡祖先；下次 Layout 重算；约束相同且 clean → early-out |
| `MarkNeedsPaint` | 气泡至 **RepaintBoundary** 停止；仍 `tree.markDirty` 调度帧 |
| `primitive.RepaintBoundary` | 隔离动画控件脏区（对标 Flutter RepaintBoundary） |
| `ScrollViewport` | **默认** `SetRepaintBoundary(true)`；滚动用 `ContentPaintOffset`，不改 child layout Offset |
| `Pressable` | **默认不是** Boundary（Tabs 等大量实例；每 Boundary 每帧 markLive 成本高） |
| `Tree.AddTicker` | ANIMATING；Tick 内 MarkNeedsPaint；**禁止** kit Continuous |
| `FullPaintRequired` | 首帧/resize/expose 全量 paint |
| G2 非承诺 | 矢量 MSAA 直写 swapchain **不**保证区外像素保留；blit 层为正确局部路径 |
| Present 默认 | Phase1 **整窗** present；可选 `PresentFrameDamage*` 须先证明 surface LoadOpLoad（曾试局部 present 导致黑屏，已撤回） |

Kit smoke（`ui_kit_m5_smoke`）默认 `Continuous=false` + Ticker。

### 4.1 合成带（Main / Overlay）— 对齐 Flutter Overlay

**问题**：仅把 portal 画进与主树相同的 base RT，再 `CompositeLive` 全部 Boundary，会使 **Tabs `ScrollViewport` 等 main 层画在 Modal mask 之上**（hit 仍 overlays 优先 → 点得中、看起来没盖住）。

**契约（已实现 · `ui/core` + `ui/layer`）**

| 阶段 | API | 内容 |
|------|-----|------|
| 主树 paint | `Tree.PaintMain` | root only · `LayerBandMain` |
| 浮层 paint | `Tree.PaintOverlays` | OverlayHost · `LayerBandOverlay` |
| 主树 base | compositor `mainBase` | 非 deferred 主树像素 |
| 浮层 base | compositor `overlayBase` | Mask / 面板 chrome（透明底） |
| Present | `Compositor.BlitTo` | **mainBase → mainLayers → overlayBase → overlayLayers** |

```text
HitTest（不变）:  overlays 自上而下 → root
Present Z（compositor 路径）:
  [top]    overlayLayers（portal 内 Boundary，若有）
           overlayBase（Modal Mask + 面板矢量）
           mainLayers（ScrollViewport / Spin / Skeleton …）
  [bottom] mainBase
```

**硬规则**

1. 真窗 retained 路径必须用 **`Compositor.Frame` + `BlitTo`**（或等价双 RT / 分带 blit）。  
2. **禁止**假设「单 DC `Tree.Paint` + DeferLayerBlit + 一次 CompositeLive」能正确盖住 Scroll 层。  
3. 浮层必须走 **OverlayPortal → OverlayHost**；禁止 in-tree 假遮罩指望盖住 Boundary 层。  
4. 无 compositor（`GPUI_COMPOSITOR=0`）时单 surface 顺序 root→overlays 仍正确（无 deferred 分轨）。  
5. 细节与注释：`ui/core/doc.go` · `ui/layer/compositor.go`。

**kit 受益**：Modal / Drawer / Message / Tooltip / Select popup 等一切 portal 浮层。

---

## 5. `ui/primitive` — 组合底层规格

### 5.1 准入门槛

同时尽量满足：无产品名；可组合；契约稳定；能力单一；只读 Token 语义键（不写死品牌色）。

### 5.2 统一契约

每个 primitive 是 `core.Node`，并具备：

| 面 | 要求 |
|----|------|
| Layout | 参与 Constraints 或固定 intrinsic |
| Paint | 仅 `PaintContext` → `render.Context` |
| Hit | 可配置：可点 / 穿透 / 透明吞事件 |
| State（交互类） | 至少：`hovered/pressed/focused/disabled` 可查询 |
| Slots | 具名子节点（`child`/`prefix`/…）按需 |
| Style | `Theme.Token` + 可选本地 Override |
| A11y | `Role`/`Label` 字段可空 |
| TypeID | 如 `primitive.Box`，供 Skin 挂接 |

```text
PressableState { Hovered, Pressed, Focused, Disabled }
// 皮肤：state → Token 键 → 绘制；不改 Hit 实现
```

### 5.3 组合架构能力总表（Infra · 非控件名）

> 实现 Ant **全量**组件时，真正要买齐的是下列 **架构能力**。  
> 能力在 `core` 或 `primitive` 中落地；kit 只消费。  
> **优先级**：P0 骨架 → P1 可点可输 → P2 浮层 → P3 数据密度 → P4 复杂选择器/反馈 → P5 增强。

| 能力 ID | 名称 | 说明 | 主要服务 Ant 类 | 优先级 |
|---------|------|------|-----------------|--------|
| **C-Tree** | 节点树/脏标记 | mount、key、setState | 全部 | P0 |
| **C-Constraint** | 约束布局协议 | Constraints 下传、单次 pass | 全部布局 | P0 |
| **C-Flex** | Flex 算法 | Row/Column、gap、flex 因子 | Flex/Space/ToolBar/Form | P0 |
| **C-Stack** | 叠放布局 | 对齐/绝对偏移 | Badge/浮层内结构 | P0 |
| **C-Grid** | 网格布局 | 行列、span、gutter（可简化） | Grid/Form/Calendar/Descriptions | P1–P3 |
| **C-Hit** | 命中与捕获 | 逆 z、clip、pointer capture | 全部交互 | P0 |
| **C-Event** | 指针/键/滚轮路由 | 冒泡、Handled | 全部交互 | P0 |
| **C-Focus** | 焦点环/Tab | 含 **FocusTrap**、**RovingTabindex** | Form/Modal/Menu/Tabs | P1–P2 |
| **C-Paint** | PaintContext | clip、offset、theme | 全部 | P0 |
| **C-Frame** | 帧管线 | layout→paint→present | 全部 | P0 |
| **C-Theme** | Token/Theme | 语义色与尺寸 | ConfigProvider 等价 | P1 |
| **C-Skin** | Painter/Part | typeID 绘制策略 | 换皮 | P1 |
| **C-Overlay** | 浮层栈/Portal | z-order、多浮层、点击外部 | Modal/Drawer/Dropdown/… | P2 |
| **C-Anchor** | 锚点定位 | 贴附触发器、flip/shift、箭头位 | Tooltip/Popover/Dropdown/Select | P2 |
| **C-Scroll** | 视口滚动 | 偏移、裁剪、嵌套滚轮 | List/Table/Menu/长表单 | P2–P3 |
| **C-Sticky** | 吸顶/粘附 | 相对滚动容器 sticky | Table 表头、Affix | P3–P4 |
| **C-Virtual** | 虚拟窗口 | 只构建可见行/节点 | Table/List/Tree/Select | P3 |
| **C-Edit** | 文本编辑+IME | 选区、caret、composition | Input/Mentions/… | P2 |
| **C-Measure** | 文本/子树测量 | intrinsic、ellipsis 判定 | Typography/Table 列 | P1–P2 |
| **C-SelectModel** | 选择模型 | 单选/多选/范围 | Radio/Select/Table/Tree | P2–P3 |
| **C-KeyboardNav** | 方向键导航 | 列表/菜单/表格单元格 | Menu/Select/Table/Tabs | P2–P3 |
| **C-Drag** | 拖拽手势 | 滑块、列宽、Transfer、Upload 排序 | Slider/Splitter/Table | P3–P4 |
| **C-Gesture** | 长按/双击等 | 可选 | 桌面增强 | P4 |
| **C-Motion** | 动画 ticker | 开合、高度、透明度；ReduceMotion | Collapse/Modal/Skeleton | P3–P5 |
| **C-Presence** | 挂载过渡 | enter/leave 后再卸载 | Message/Notification | P3 |
| **C-FormBind** | 字段绑定/校验管线 | name path、rules、错误收集 | Form | P3 |
| **C-Trigger** | 触发控制器 | hover/focus/click 延迟、互斥 | Tooltip/Dropdown | P2 |
| **C-PortalHost** | 根级浮层宿主 | 与 Window 绑定 | 全部浮层 | P2 |
| **C-ScrollSpy** | 滚动侦听/联动 | 可见 section | Anchor | P4 |
| **C-ClipContent** | 溢出省略/裁切 | 单行多行 ellipsis | Typography/Table | P1 |
| **C-IconReg** | 图标注册表 | 名→path | Icon 全站 | P1 |
| **C-Image** | 图像加载/绘制 | 含失败占位 | Image/Avatar/Upload | P2 |
| **C-CanvasPaint** | 自定义矢量绘制 | 进度环、二维码、水印、骨架 | Progress/QR/Watermark | P3–P4 |
| **C-FileHost** | 文件选择服务 | platform 能力 | Upload | P4 |
| **C-Clipbd** | 剪贴板 | 复制粘贴 | Input/Table | P2 |
| **C-NotifyQueue** | 全局队列 | 叠放、duration、maxCount | Message/Notification | P3 |
| **C-Config** | 全局配置下发 | 尺寸、locale、prefix（类 ConfigProvider） | 全部 kit | P1 |
| **C-A11y** | 角色/标签/活区 | 最小集 | 全部 | P2–P5 |
| **C-Plugin** | 插件注册 | 控件/皮肤/服务 | 扩展 | P0–P2 |
| **C-Platform** | SPI+Caps | 跨平台 | 全部 | P0 |

**统计（架构能力）**：上表 **36** 项；P0≈10，P1–P2 补交互与浮层，P3+ 数据与动效。

### 5.4 Primitive 节点清单（可组合积木）

> 与 §5.3 对应：能力是「管道」，Primitive 是「零件」。  
> **推荐实现数**：核心 **28** 种节点（可合并实现，但能力不可丢）。

#### A. 布局

| TypeID | 名称 | 能力依赖 | 优先级 |
|--------|------|----------|--------|
| `primitive.Box` | 盒（尺寸/padding/子） | C-Constraint | P0 |
| `primitive.Flex` | 行/列 | C-Flex | P0 |
| `primitive.Stack` | 叠放 | C-Stack | P0 |
| `primitive.Flexible` | 弹性子/Spacer | C-Flex | P0 |
| `primitive.Grid` | 网格（行列 span） | C-Grid | P1–P3 |
| `primitive.ScrollViewport` | 滚动视口 | C-Scroll | P2 |
| `primitive.Sticky` | 粘性子 | C-Sticky | P3 |
| `primitive.SplitPane` | 可拖分割（底层） | C-Drag+C-Flex | P4 |
| `primitive.Divider` | 分割线（几何） | C-Paint | P1 |

#### B. 内容

| TypeID | 名称 | 能力依赖 | 优先级 |
|--------|------|----------|--------|
| `primitive.Text` | 文本 | C-Measure C-ClipContent | P0 |
| `primitive.RichText` | 多样式 span | C-Measure | P3 |
| `primitive.Icon` | 图标 | C-IconReg | P1 |
| `primitive.Image` | 图像 | C-Image | P2 |
| `primitive.Canvas` | 即时矢量绘制 | C-CanvasPaint | P3 |

#### C. 交互

| TypeID | 名称 | 能力依赖 | 优先级 |
|--------|------|----------|--------|
| `primitive.Pressable` | 可点（含 hover/press；可选 focus） | C-Hit C-Event C-Focus? | P0 |
| `primitive.Focusable` | 仅焦点壳（若未并入 Pressable） | C-Focus | P1 |
| `primitive.HitTarget` | 热区扩大 | C-Hit | P1 |
| `primitive.Draggable` | 拖拽源/手柄 | C-Drag | P3 |
| `primitive.OverlayPortal` | 传送到浮层栈 | C-Overlay C-PortalHost | P2 |
| `primitive.AnchoredPopup` | 锚点弹出层 | C-Anchor C-Overlay C-Trigger | P2 |
| `primitive.Mask` | 遮罩/点击关闭 | C-Overlay | P2 |

#### D. 输入

| TypeID | 名称 | 能力依赖 | 优先级 |
|--------|------|----------|--------|
| `primitive.EditableText` | 编辑内核+IME | C-Edit C-Focus C-Clipbd | P2 |
| `primitive.CaretLayer` | 光标/选区绘制（可内嵌） | C-Edit | P2 |

#### E. 结构 / 装饰 / 数据辅助

| TypeID | 名称 | 能力依赖 | 优先级 |
|--------|------|----------|--------|
| `primitive.Slot` | 具名插槽 | — | P1 |
| `primitive.Decorated` | 背景边框圆角阴影 | C-Theme C-Skin | P1 |
| `primitive.Clip` | 裁剪 | C-Paint | P0 |
| `primitive.PainterNode` | 自定义 Paint 回调 | C-Paint | P1 |
| `primitive.VirtualList` | 虚拟列表窗口 | C-Virtual C-Scroll | P3 |
| `primitive.SelectionScope` | 选择模型作用域 | C-SelectModel | P2 |
| `primitive.FocusScope` | 焦点子树/trap | C-Focus | P2 |
| `primitive.KeyboardNav` | 方向键容器（可作 mixin） | C-KeyboardNav | P2 |

#### F. 明确不进 primitive

一切 **产品名**：Button、Modal、Form、Table、Select、DatePicker、Message…  
一律在 **kit** 用上表积木组合。

### 5.5 布局算法与事件（摘要）

- Constraints 下传；Flex/Stack/Grid 单次 pass。  
- 指针：Hit → capture → 冒泡；Click=同目标 down/up。  
- 滚轮：最近 ScrollViewport。键盘：焦点节点 + KeyboardNav 容器。

### 5.6 统计总览（primitive 组合架构）

| 类别 | 数量 | 说明 |
|------|------|------|
| 架构能力（§5.3） | **36** | 管道级 |
| Primitive 节点（§5.4） | **28** | 可合并实现，能力不可删 |
| 其中 P0 必做节点 | **9** | Box Flex Stack Flexible Text Pressable Clip + core 管线 |
| P1 补齐 | **+7** | Decorated Slot Icon Focus HitTarget Divider Painter… |
| P2 浮层+输入 | **+7** | Overlay Anchored Mask Editable Scroll FocusScope… |
| P3+ 密度/拖拽/绘制 | **+5** | Virtual Grid Sticky Drag Canvas Split… |
| Ant 组件覆盖（§5.7） | **70+** | 见全量表；桌面可砍子集 |

---

## 5.7 Ant Design 全量组件 → 组合依赖清单

> 来源：Ant Design Components Overview（General / Layout / Navigation / Data Entry / Data Display / Feedback / Other）。  
> **组合列**只写 **primitive + 架构能力**，不写产品实现细节。  
> **Pri** = 建议实现优先级（与桌面主路径相关；非官网权重）。  
> **✓** = 可用已列 primitive 组合；**△** = 需对应 P3+ 能力；**○** = 后置/可降级。

### 5.7.1 General

| Ant 组件 | 主要组合（primitive / 能力） | Pri |
|----------|------------------------------|-----|
| **Button** | Pressable + Decorated + Flex + Icon? + Text；C-Focus 键盘 | P0–P1 |
| **FloatButton.Group** | Stack/Box 定位 + Pressable 组 + 可选 Overlay | P4 |
| **Icon** | Icon + C-IconReg | P1 |
| **Typography** | Text/RichText + C-ClipContent + C-Measure；可复制 C-Clipbd | P1–P3 |

### 5.7.2 Layout

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **Divider** | Divider / PainterNode | P1 |
| **Flex** | Flex | P0 |
| **Grid** (Row/Col) | Grid 或 Flex 模拟 | P1–P3 |
| **Layout** (Header/Sider/Content/Footer) | Flex/Box 嵌套 + 可选 Sticky | P1 |
| **Space** | Flex + gap | P0 |
| **Space.Compact** | Flex 无缝 + 子 Decorated 拼边 | P2 |
| **Splitter** | SplitPane + Draggable + Flex | P4 |
| **Masonry** | 自定义布局算法（后）/ Canvas 级 | ○ |

### 5.7.3 Navigation

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **Anchor** | ScrollViewport 外 + C-ScrollSpy + Pressable 链 | P4 |
| **Breadcrumb** | Flex + Pressable/Text + Separator | P2 |
| **Dropdown** | Pressable + AnchoredPopup + 菜单列表(KeyboardNav) | P2–P3 |
| **Menu** | Flex/Scroll + Pressable 项 + Nested + C-KeyboardNav + SelectionScope | P3 |
| **Pagination** | Flex + Pressable + EditableText(页码)? | P3 |
| **Steps** | Flex + Icon/Text + 状态 Decorated | P3 |
| **Tabs** | Flex + Pressable + 指示条 + C-KeyboardNav；内容 Stack/Box | P3 |

### 5.7.4 Data Entry

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **AutoComplete** | EditableText + AnchoredPopup + VirtualList/Filtering | P3 |
| **Cascader** | AnchoredPopup + 多列 Scroll/KeyboardNav + SelectionScope | P4 |
| **Checkbox** | Pressable + Decorated(指示) + Text；Group=SelectionScope | P2 |
| **ColorPicker** | Pressable + AnchoredPopup + Canvas/面板 + Editable | P4 |
| **DatePicker** | Editable/显示 + AnchoredPopup + **CalendarGrid(Grid)** + KeyboardNav | P4 |
| **Form** | Flex/Grid 布局 + **C-FormBind** + 子 Field | P3 |
| **Form.Item** | Flex + Label Text + 控件 Slot + 错误 Text | P3 |
| **Input** | Decorated + Flex + EditableText + Affix Slot | P2 |
| **Input.TextArea** | 同上多行 EditableText + 可 Resize(后) | P2 |
| **Input.Search/Password/OTP** | Input 组合 + Pressable 图标 | P2–P3 |
| **InputNumber** | EditableText(过滤) + Pressable 步进 | P3 |
| **Mentions** | EditableText + 触发检测 + AnchoredPopup 列表 | P4 |
| **Radio** | Pressable + Decorated + SelectionScope(Group) | P2 |
| **Rate** | Flex + 多个 Pressable/Icon + 半选 hit | P3 |
| **Select** | Pressable 展示 + AnchoredPopup + VirtualList + KeyboardNav + 多选 Tag | P3 |
| **Slider** | Box 轨道 + Draggable 拇指 + C-Drag + 可选 Mark | P3 |
| **Switch** | Pressable + Decorated 滑动块（可 Motion） | P2 |
| **TimePicker** | 同 Date 面板列 Scroll + Editable | P4 |
| **Transfer** | 双 VirtualList/List + Pressable + SelectionScope | P4 |
| **TreeSelect** | Select 壳 + Tree 面板（见 Tree） | P4 |
| **Upload** | Pressable + **C-FileHost** + List 项 + Image? + Drag 区 | P4 |

### 5.7.5 Data Display

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **Avatar** | Box 圆裁剪 + Image/Text/Icon | P2 |
| **Badge** | Stack + 角标 Box/Text | P2 |
| **Calendar** | Grid 日期 + Pressable 格 + 面板头 | P4 |
| **Card** | Decorated + Flex 头/体/尾 + Slot | P2 |
| **Carousel** | Clip + 横滑 Stack/偏移 + Pressable 指示；可 Motion | P4 |
| **Collapse** | Pressable 头 + 高度展开(C-Motion) + Clip | P3 |
| **Descriptions** | Grid/Flex 标签值对 | P3 |
| **Empty** | Flex + Image/Icon + Text | P2 |
| **Image** | Image + 预览 OverlayPortal + 可选画廊 | P2–P4 |
| **List** | Scroll + 项 Flex；可 VirtualList | P3 |
| **Popover** | AnchoredPopup + Trigger(hover/click) + 内容 Slot | P2 |
| **QRCode** | Canvas 绘码 | P4 |
| **Segmented** | Flex + Pressable + SelectionScope + Decorated 滑块 | P3 |
| **Statistic** | Flex + Text 层级 | P3 |
| **Table** | **Grid/列 Flex** + **Sticky 头** + **Virtual 行** + 排序 Pressable + 可选列拖宽 Draggable + SelectionScope | P3–P4 |
| **Tag** | Decorated + Text + 可选关闭 Pressable | P2 |
| **Timeline** | Flex/Stack 轴线 + 点 + 内容 | P3 |
| **Tooltip** | AnchoredPopup + C-Trigger 延迟 + Text | P2 |
| **Tour** | Mask 镂空(C-Canvas/Clip) + AnchoredPopup + 步骤状态 | P5 |
| **Tree** | 缩进 Flex + Expand Pressable + 可选 Virtual + SelectionScope + KeyboardNav | P3–P4 |

### 5.7.6 Feedback

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **Alert** | Decorated + Flex + Icon + Text + 关闭 Pressable | P2 |
| **Drawer** | OverlayPortal + Mask + 侧栏 + FocusScope 陷阱 + Esc；**合成 §4.1**；Z=400 | P3 |
| **Message** | PortalHost + **C-NotifyQueue** + 项 Decorated；C-Presence | P3 |
| **Modal** | OverlayPortal + Mask + 居中面板 + FocusScope 陷阱 + Esc + 脚 Button；**合成须 §4.1 双带**；MaskClosable 默认 true | P3 |
| **Notification** | 同 Message 角位置队列 | P3 |
| **Popconfirm** | Popover + 确认/取消 Pressable | P3 |
| **Progress** | Canvas/Box 条或环 | P3 |
| **Result** | Flex + Icon + Typography + 操作 Slot | P3 |
| **Skeleton** | Canvas/Box 占位 + 可选 Motion 闪烁 | P3 |
| **Spin** | Stack 遮罩 + Canvas/Icon 旋转(C-Motion) | P2–P3 |
| **Watermark** | 顶层 Canvas 铺贴（不抢 Hit 或穿透） | P4 |

### 5.7.7 Other

| Ant 组件 | 主要组合 | Pri |
|----------|----------|-----|
| **Affix** | Sticky 或 滚动监听改 Offset | P4 |
| **App** | 根：Theme/Config + PortalHost + Notify 挂载点 | P1 |
| **ConfigProvider** | C-Config + C-Theme 下发 | P1 |
| **BorderBeam 等装饰** | Canvas/Motion 特效 | ○ |
| **Util**（hooks 等） | 非节点；对等于服务/工具包 | P2+ |

### 5.7.8 覆盖率与缺口统计（2026-07-23 · 以源码为准）

| 项 | 数量/结论 |
|----|-----------|
| 权威表 | **`ui/kit/coverage.go` → `AntCoverage()`**（`go test ./ui/kit -run TestAntCoverageTable`） |
| 表项 / `CovReady` | **70 / 70**（状态列全 ready） |
| CovPartial / Primitive / Later | **0**（残差写在 **Notes**，非 Status） |
| Notes 残差（能力未齐） | Menu 嵌套 later；DatePicker range later；Upload host dialog later；Image GPU texture later；QR codec later；Table virtual+sticky head 基线 |
| 摘要文档 | [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)（须与 coverage.go 同步） |
| 历史规划表（§5.7 各节 Pri） | 仍作**组合依赖参考**；**不**再当作落地状态表 |
| Primitive / C-* 能力 | 主路径已补 OverlayPortal、Mask、Scroll、FocusScope、VirtualList 等；细节见 §5.3–§5.4 与源码 |
| 合成 | 浮层 Z 序见 **§4.1**（双带）；勿用旧「单 base + 全层 CompositeLive」 |

### 5.7.9 实现分层建议（由清单反推）

```text
层 0  core 管线：C-Tree…C-Frame C-Hit C-Event
层 1  布局积木：Box Flex Stack Flexible Clip Text Pressable
层 2  观感：Decorated Icon Slot Theme Skin Divider
层 3  输入：Focus EditableText
层 4  浮层：PortalHost OverlayPortal Mask AnchoredPopup Trigger
层 5  列表密度：Scroll VirtualList SelectionScope KeyboardNav
层 6  结构复杂：Grid Sticky Draggable SplitPane FormBind
层 7  绘制增强：Canvas Motion Presence NotifyQueue
层 8  平台：FileHost 等 Caps
```

**kit 只允许使用已存在层的积木组合**；缺积木时先加 primitive/能力，禁止在 kit 内复制一套 Hit/浮层。

---

## 6. 组合扩展模型（产品怎么长出来）

### 6.1 公式

```text
高级控件 =
    布局 primitive（Flex/Box/Stack）
  + 交互 primitive（Pressable/Focusable/Editable/Overlay）
  + 内容 primitive（Text/Icon/Image）
  + 装饰（Decorated / Skin Painter）
  + 产品状态机（loading、validate… 在 kit）
  + 产品 Props（业务友好 API）
```

### 6.2 示例组装

**kit.Button**

```text
Pressable (disabled||loading, onClick)
  └─ Decorated (type/danger/size → Token)
       └─ Flex(Row, gap)
            Icon? · Text(label) · Loading?
```

**kit.Input**

```text
Focusable
  └─ Decorated (border/status Token)
       └─ Flex
            Prefix · EditableText · Suffix/Clear
```

**kit.Modal**

```text
OverlayPortal(Z=500)          // 进 OverlayHost；合成见 §4.1 双带
  └─ FocusScope
       └─ modalLayer
            Mask(fullscreen) · Decorated(panel)
                 Flex(Column): title · content · footer(kit.Button)
```

Mask 尺寸 = `Modal.Viewport` 或 `Tree.Viewport()`。**MaskClosable** 默认 true。  
**Esc 关闭**：已接线（FocusScope.OnEscape → OnCancel + close）。  
Present 必须 `Compositor` 双带，否则 main 的 Scroll 层会盖住 mask。

**业务自研（非 Ant）** 如工具条、时间线、芯片输入：只组合 primitive，或自建 `mykit`，**不改 core**。

### 6.3 扩展四级

| 级 | 做法 | 何时 |
|----|------|------|
| L1 | 页面内组合 primitive/kit | 日常 |
| L2 | 项目 `func PrimaryBtn(...)` | 规范与埋点 |
| L3 | 自建 kit 包 | 多应用设计体系 |
| L4 | 新 primitive | 现有积木组合不能表达；需评审 |

### 6.4 自定义「已有产品控件」入口

| 优先级 | 入口 | 说明 |
|--------|------|------|
| 1 | kit **Props / Slots** | 单实例 |
| 2 | **Token** / SetTheme | 品牌色、暗色、密度 |
| 3 | **Skin / 按 typeID Painter** / Part 槽 | 换形态，类型名不变 |
| 4 | 包装组合 | 项目默认行为 |
| 5 | 显式 **`ReplaceControl("kit.Button")`** | 全局换实现；须契约测试 |
| 6 | **Plugin** | 打包 Token+Skin+可选 Replace |

决策：能 Props/Token 不写 Skin；能 Painter 不 Replace；禁止静默覆盖；绘制仍只走 render。

业务 **更推荐**：直接组合 **primitive** 做新 UI，而不是 Replace 内置 Button。

---

## 7. `ui/kit` — 产品控件面（默认可对标 Ant）

Kit = **行为 + 产品 Props + 状态机 + a11y 名**；内部 **只组合 primitive**。  
默认语义/状态矩阵/视觉目标可对标 **Ant Design**；包名仍为 `kit`。

### 7.1 批次（实现路径）

| 批次 | 产品控件 | 主要依赖的 primitive | 里程碑 |
|------|----------|----------------------|--------|
| **K0 示范** | SimpleButton / SimpleField（可无完整 Ant props） | Pressable Text Box Editable | M1 |
| **B0** | Box 布局容器封装、Text、Icon、**Button** | Box Flex Text Icon Pressable Decorated | M1 |
| **B1** | Input TextArea Checkbox Switch Radio | Focusable Editable Decorated | M2 |
| **B2** | Form FormItem Select | B1 + 校验状态机 | M3 |
| **B3** | Modal Drawer message | OverlayPortal Pressable | M3 |
| **B4** | List Table Scroll | ScrollViewport 虚拟化 | M4 |
| **B5** | Menu Dropdown Tabs Pagination | Overlay Flex Pressable | M4–M5 |

### 7.2 状态矩阵（产品控件）

default / hover / active / focus / disabled；（表单）error/warning；（Button）loading。  
其中 hover/active/focus/disabled **来自 primitive 状态**；error/loading 由 kit 叠加。

### 7.3 Props 子集（用户层 · 对标 Ant 可简化）

**Button**：Type(default/primary/dashed/text/link)、Size、Danger、Disabled、Loading、Block、Icon、Label、OnClick。键盘 Space/Enter。  

**Input**：Value、Placeholder、Disabled、ReadOnly、MaxLength、AllowClear、Prefix/Suffix、Size、Status、OnChange、OnPressEnter。  

**Form/FormItem**：Layout、OnFinish、Name、Label、Required、Rules、ValidateStatus。  

**Modal**：Open、Title、Content、OnOk/OnCancel、MaskClosable、Width、Viewport；FocusScope 陷阱 + **Esc**；合成见 §4.1。  

**Table/List**：Columns/DataSource/RowKey/Loading/虚拟高度等基础集；排序固定列后置。  

**Menu/Tabs/Pagination/Dropdown**：基础选中与 OnChange；完整 Ant 网页生态非目标。  

每控件文档：**与 ant.design 对照 + 桌面差异**。未列 props = 后置或 WontFix。

### 7.4 Kit 实现标准

1. **仅组合 primitive**（+ 少量 kit 子组件）  
2. Token/Skin 驱动外观  
3. 状态矩阵完整  
4. 布局参与 Constraints  
5. Hit/焦点/键盘正确  
6. 只经 render.Context  
7. 可被 Skin/Replace 扩展  
8. 平台中立  
9. 单测：组合结构 + 状态；契约测试供 Replace 后回归  
10. 差异文档  

---

## 8. Theme / Skin / Token

| 概念 | 含义 | 默认 |
|------|------|------|
| **TokenSet** | 色/字号/间距/圆角/动效数据 | 可对齐 Ant Token 语义键 |
| **Skin** | typeID → Painter；Part 槽；组装策略 | `skin/default` 可 Ant 视觉 |
| **Theme** | TokenSet + Skin + Dark + Density + Scale | `Default()` = 亮色 Ant 向 + default skin |

### 8.1 Token 最小键（示意 · 默认可 Ant 色）

**色**：colorPrimary、Success/Warning/Error、colorText*、colorBg*、colorBorder*、colorPrimaryHover/Active、colorBgMask、colorLink。  
**型**：fontSize*、lineHeight、controlHeight*、padding*、borderRadius*、motionDuration*、boxShadow*。  
**状态映射**：hover/active/focus/disabled/error → 上表派生键。

Kit 的 `type=primary` → 读 `colorPrimary` 等；**另一设计体系**换映射表即可。

### 8.2 Skin 挂接

- primitive：`primitive.Decorated`、`primitive.Pressable` 的默认绘制。  
- kit：`kit.Button` 等组装级 Painter（可选；也可纯组合 + Decorated）。  
- `OverridePainter(typeID)` 只换某一类。

---

## 9. Core / Platform API（摘要）

### 9.1 Node（单树）

`Layout(Constraints) Size` · `Paint(*PaintContext)` · `HitTest(Offset) Node` · 脏标记 · Mount/Unmount · 可选 Stateful.SetState。

### 9.2 PaintContext

`DC *render.Context` · Origin · Clip · Scale · Theme；禁止另开 CPU 位图作最终帧。

### 9.3 PluginHost

`Register/Replace Control|Skin|TokenSet|Service`；同名 Register 失败；Replace 显式。  
插件不握 GPU device、不 import OS backend。

### 9.4 Host SPI + Caps

Size/ScaleFactor、指针键鼠文本、IME、剪贴板、光标、DarkMode/ReduceMotion/FontScale、Present、Surface lifecycle。  
Headless 必有。无 Cap 则降级（如无 composition 仅 TextInput）。  
Win/mac：M0 stub 可编译；真适配 M6。

与 `gpu/context.EventSource` / `PlatformProvider`：platform 适配翻译，core 不直连。

---

## 10. 用户层用法示意

```go
// 路径 1：产品面（默认可 Ant 向）
kit.Button("OK", kit.TypePrimary(), kit.OnClick(save))

// 路径 2：直接组合积木（任意自定义，不限 Ant）
primitive.Pressable(OnClick(save),
    primitive.Decorated(TokenBgPrimary,
        primitive.FlexRow(gap,
            primitive.Icon("check"),
            primitive.Text("OK"),
        ),
    ),
)

// 路径 3：自建 kit
mykit.ToolButton(...) // 内部同样只组合 primitive
```

```go
// 宿主帧
host.PumpEvents()
tree.Dispatch(ev)
tree.Frame() // layout → paint → present
```

---

## 11. 用户层能力评估（现代 GUI · 目标态）

| 能力 | 目标 | 用户怎么做 | 里程碑 |
|------|------|------------|--------|
| 组合式底层积木 | ● | import primitive | M0–M2 |
| 产品级常用控件 | ● | kit（可 Ant 向） | M1–M5 |
| 不限 Ant 的自定义 | ● | 组合 primitive / 自建 kit | M1+ |
| 主题/暗色/品牌色 | ● | Token | M1–M3 |
| 换展示形态 | ● | Skin/Painter | M1–M2 |
| 表单/浮层/表/虚拟列表 | ● | kit B2–B4 | M3–M4 |
| IME/快捷键/剪贴板 | ● | platform+kit | M1–M3 |
| 跨平台同一 API | ● | kit/primitive 不写 OS | M0 |
| 跨平台实现 | 分期 | Linux→Win/mac | M0/M6 |
| 多窗口/DnD/原生菜单 | ○ 后置 | — | 后 |
| 测试 | ● | Headless + 单测 | M0 |

**不能指望**：与 ant.design 网页 100% props；首日 Win/mac 原生打磨；DOM/CSS；引擎完美局部重绘。

**综合（规划）**：底座是 **可组合 primitive 框架**；Ant 向 kit 是 **第一套产品皮肤与 API**，不是唯一形态。

---

## 12. 里程碑与任务（按底座优先）

### M0 — core + 最小 primitive + 真窗闭环

| 任务 | 产出 |
|------|------|
| core：§5.3 P0 能力（Tree Constraint Flex Stack Hit Event Paint Frame） | 可测管线 |
| primitive：Box Flex Stack Flexible Text Pressable Clip | 可组合 |
| platform：SPI + Headless + Linux | 双宿主 |
| Plugin Registry 空跑 | 扩展位 |
| smoke：Pressable+Text 点击变色 | `examples/ui_core_smoke` |

**DoD**：Linux 真窗可点可画 present；Headless 测 layout/hit；**不**依赖 kit/Ant。对照 §5.7 仅覆盖 Button 级组合预备。

### M1 — §5.9 层2 观感 + kit B0 ✅（2026-07-21）

Decorated Slot Icon Focus HitTarget Divider Theme/Skin Token；kit.Button/Text/Icon；组合示例。

| 产出 | 路径 |
|------|------|
| Theme/Token/Skin/Focus | `ui/core`（`theme.go` `skin.go` `focus.go`） |
| P1 primitives | `ui/primitive` — Decorated Slot Icon Focusable HitTarget Divider PainterNode |
| skin/default | `ui/skin/default` |
| kit B0 | `ui/kit` — Button Text Icon |
| smoke | `examples/ui_kit_smoke` |
| 单测 | `go test ./ui/core ./ui/primitive ./ui/kit` |

### M2 — 层3–4 输入+浮层 + B1 ✅（2026-07-21）

EditableText、IME、Input 系、Checkbox/Radio/Switch、OverlayPortal/Mask/AnchoredPopup/Trigger、Tooltip/Popover 基、ScrollViewport。

| 产出 | 路径 |
|------|------|
| OverlayHost / Scroll / IME 事件 | `ui/core`（`overlay.go` `event_scroll_ime.go` · Tree 扩展） |
| P2 primitives | EditableText ScrollViewport OverlayPortal Mask AnchoredPopup Trigger |
| kit B1 | Input TextArea Checkbox Radio Switch Tooltip Popover |
| smoke | `examples/ui_kit_b1_smoke` |
| 单测 | `go test ./ui/kit`（m2_test 等） |


### M3 — 层5–6 部分 + B2/B3 ✅（2026-07-21）

FormBind、SelectionScope、KeyboardNav、VirtualList 起步、Modal/Drawer/message 队列、Select/Menu/Tabs 基。

| 产出 | 路径 |
|------|------|
| FormModel / SelectionModel / KeyboardNav / NotifyQueue | `ui/core` |
| VirtualList · FocusScope | `ui/primitive` |
| kit B2/B3 | Form FormItem Select Menu Tabs Modal Drawer MessageHost |
| smoke | `examples/ui_kit_b2_smoke` |
| 单测 | `ui/kit/m3_test.go` |


### M4 — Table/Tree/复杂选择 + B4/B5 ✅（2026-07-21）

Grid Sticky Draggable、Table/List/Tree、Cascader/Transfer 等按需、Pagination/Dropdown 完善。

| 产出 | 路径 |
|------|------|
| LayoutGrid / Grid / Sticky / Draggable / SplitPane | `ui/core/layout_grid.go` · `ui/primitive/grid.go` |
| kit B4/B5 | Table List Tree Pagination Dropdown Transfer Cascader |
| smoke | `examples/ui_kit_b3_smoke` |
| 单测 | `ui/kit/m4_test.go` |


### M5 — 层7 动效/a11y + 打磨 ✅（2026-07-21）

Motion Presence Canvas 增强、Tour/Skeleton 等、A11y、density。

| 产出 | 路径 |
|------|------|
| Clock / Anim / Ease / ReduceMotion | `ui/core/motion.go` · Tree.TickClock |
| A11y Role/Label/Live on NodeBase | `ui/core/node.go` · kit.CollectA11y |
| Canvas · Motion · Presence · ProgressRing | `ui/primitive/motion.go` |
| Skeleton · Spin · Progress · Tour · Density | `ui/kit/feedback.go` |
| smoke | `examples/ui_kit_m5_smoke` |
| 单测 | `ui/kit/m5_test.go` |


### M6 — 壳与跨平台 ✅（2026-07-21）

示例迁 kit、Win/mac、golden、§5.7 覆盖率对照更新。

| 产出 | 路径 |
|------|------|
| `NewHost` 工厂 · Win/mac API stub | `ui/platform/factory.go` · `windows_host*.go` · `darwin_host*.go` |
| GPU present | **Linux only**（`GPUPresentReady`）；Win/mac stub 不握 swapchain |
| Golden layout/paint/hit | `ui/kit/golden_test.go` |
| Ant §5.7 覆盖率表 | `ui/kit/coverage.go` · `AntCoverage()` |
| kit 壳示例 | `examples/ui_kit_shell`（Header/Sider/Table/Modal/Message） |

**说明**：Win/mac 为可编译 SPI stub（合成事件），真 HWND/AppKit 适配仍后置；不改引擎主线。

### 12.1 主路径视觉与输入打磨（需求清单）

> **需求名**：主路径视觉与输入打磨  
> **英文 / Epic**：`polish: visual chrome + IME` · `M5 polish — default skin & input`  
> **背景**：M0–M6 **功能主路径已落地**，kit 控件多为灰盒/可辨认级，观感简陋（圆角边框、指示器、状态色等）；IME 事件模型有骨架，真宿主与体验需补完。  
> **目标**：主路径 **观感自然可用** + **输入（含 IME）可用**；**不是** Ant 全库像素级、不是推倒重做。  
> **原则**：先磨 **共用绘制链**，再磨指示器与基础控件；IME **可并行**，不与圆角绑同一 PR。M4+ 新控件必须走已打磨的 Decorated，禁止再分叉手绘边框。

#### 范围

| 包含 | 不包含（本需求外） |
|------|-------------------|
| `PaintContext` 圆角 fill/stroke 质量 | Table/Tree 高密度像素精修 |
| `Decorated` + `skin/default` 单路径 | 全量 Ant 长尾组件 |
| Checkbox / Radio / Switch 指示器 | 复杂动效体系（可仅最小） |
| Button / Input 主状态观感与 Token | 真 Win32/AppKit present |
| 焦点环最小可见 | 多端视觉矩阵 CI |
| Linux IME composition→上屏主路径 | 推倒 core 架构 |

#### A. 共用绘制（优先）

| # | 任务 | 主要路径 | 完成标准（DoD） |
|---|------|----------|-----------------|
| A1 | 圆角 Fill/Stroke 质量 | `ui/core/paint.go`（`FillLocalRoundRect` / `StrokeLocalRoundRect`） | 1px 边框清晰；fill 与 stroke 半径对齐；常见尺寸不糊、无双边/明显锯齿 |
| A2 | Decorated 绘制单路径 | `ui/primitive/decorated.go` · `ui/skin/default/skin.go` | skin 与节点 default **同一套** fill+stroke 顺序与 Token 解析；Button/Input/面板同源 |
| A3 | Token 圆角/边框/控件高度 | `ui/core/theme.go` · kit 读 Token | 主路径少魔法数；`borderRadius*` / `controlHeight*` 生效 |

#### B. 指示器与基础控件

| # | 任务 | 主要路径 | 完成标准（DoD） |
|---|------|----------|-----------------|
| B1 | Checkbox | `ui/kit/checkbox.go`（及共享绘制 helper） | 小圆角方框；勾几何居中；选中/半选/禁用可辨且不别扭 |
| B2 | Radio | `ui/kit` 对应实现 | **真圆** + 内点居中；组内选中正确 |
| B3 | Switch | 若已有实现 | 轨道/滑块几何稳定；开/关/禁用可辨 |
| B4 | Button | `ui/kit/button.go` | 高度/padding/圆角齐 Token；primary/default/disabled/hover 正确 |
| B5 | Input | `ui/kit/input.go` | 边框/焦点态可读；placeholder 可辨 |
| B6 | 焦点环最小集 | Focus 绘制或 Decorated 扩展 | 键盘聚焦时主路径控件可见 focus ring |

#### C. 输入 / IME（可与 A/B 并行）

| # | 任务 | 主要路径 | 完成标准（DoD） |
|---|------|----------|-----------------|
| C1 | Host → IME 事件 | `ui/platform/linux_*` → `Tree.DispatchIME` | 真窗 composition 能进 core（非仅单测假事件） |
| C2 | Editable preedit + commit | `ui/primitive/editable.go` · kit Input | 预编辑可见；commit 写入值；End 清理 preedit |
| C3 | IME 候选位置 | `SetIMEPosition` / caret 几何 | Caps 支持时候选靠近 caret；不支持则文档/Caps 标明 |
| C4 | 单行输入回归 | smoke 或手工清单 | 中文拼写→上屏→删除/退格主路径通过 |

#### D. 主路径走查（收口）

| # | 任务 | 完成标准（DoD） |
|---|------|-----------------|
| D1 | 固定 gallery / smoke 页 | 含 Button（多 Type）· Input · Checkbox/Radio · 可选 Form/Modal；真窗可对照 |
| D2 | 风格一致性抽查 | 主路径无「每控件一套手绘边框」；均走 Decorated/Token |
| D3 | 范围确认 | 本需求 **不做** Table 像素精修、全 Ant、复杂动效、Win/mac 真适配 |

#### E. 验收（本需求完成）

- [x] 共用 chrome 无「明显锯齿 / 双边 / 圆角崩」 — W1 `roundrect_fill_stroke` + Decorated 单路径  
- [x] Checkbox/Radio（及已做 Switch）指示器自然可辨 — W2 visual scenarios  
- [x] Button/Input 主状态观感一致、可读 — W3 visual + gallery  
- [x] Linux 下 IME 主路径可用（或正式写清 Caps 降级范围） — **正式 Caps 降级**：`LinuxHost` **无 CapIME**（XIM 未接）；Headless `CapIME` + `InjectIME` 覆盖 composition 序列；Latin 真窗 `XLookupString`→EventText。见 `ui/platform/ime.go`、`examples/ui_polish_gallery/README.md`  
- [x] 主路径控件统一 Decorated + Token，无分叉画法 — W2/W3  
- [x] gallery/smoke 走查通过 — `examples/ui_polish_gallery`（手操 #1–4 可做；#5 按降级说明；#6 Modal 入口）  

#### 建议执行顺序

```text
A1 → A2 → A3 → B1/B2/B3 → B4/B5 → B6
              ↘ C1 → C2 → C3 → C4（并行）
                    → D1–D3 收口 → E 验收
```

#### 状态

| 项 | 状态 |
|----|------|
| 功能主路径 M0–M6 | ✅ 已落地（见 §12） |
| 本打磨需求 | ✅ **W1–W4 已执行**（2026-07-21）；IME 真窗 composition 为 Caps 降级 |
| 测试方案 | 见 **§12.2**（三轨：逻辑 / 视觉回归 / 人工走查） |

### 12.2 主路径打磨 — 测试方案

> 配套 §12.1。目标：**AI 可写、CI 不脆、人能判观感、动态与 IME 可验收**。  
> 对标 Ant Design / Flutter = **设计语言与交互正确**，**不是**与官网 PNG 逐像素一致。  
> 现有 `ui/kit/golden_test.go` 的「非白像素计数」仅作冒烟；打磨阶段以本方案为准升级。

#### 12.2.1 目标拆分（勿混测）

| 问题 | 回答方式 |
|------|----------|
| 行为对不对？ | 轨 1 逻辑单测 |
| 这版有没有画坏？ | 轨 2 部件图回归（**本库基线**） |
| 像不像 Ant、好不好看？ | 轨 3 人眼 + 固定 gallery（非 CI 对齐官网） |
| 动态 / IME 手感？ | 真窗短清单 + 可选关键帧导出 |

#### 12.2.2 三轨架构

```text
轨 1  逻辑契约     — CI 必跑
轨 2  视觉回归     — 小画布部件 PNG + 容差；CI 建议跑 / 可夜间
轨 3  人工走查     — 打磨 PR 必做；真窗 + 短脚本
```

##### 轨 1 — 逻辑契约

**不截图。** 改动至少覆盖（Headless / `go test`）：

- 布局：固定 Constraints 下 Size/Offset；连续两次 layout 稳定  
- 状态：hover / press / checked / disabled / focus 与回调  
- Hit：盒内/外命中  
- IME：可灌 `IMECompositionEvent` 序列 → preedit / commit / 清空  

**通过**：`go test ./ui/core ./ui/primitive ./ui/kit ./ui/platform`（及后续 `./ui/visualtest`）必绿。  
**不负责**：观感。

##### 轨 2 — 视觉回归（部件「证件照」）

**核心**：每个关键外观 = 固定尺寸小画布，CPU `render.Context` → 位图，与仓库 **自有基线** 比对（**不比** ant.design 网站图）。

**场景约束**

- 白底或棋盘格；`scale=1`；固定 Theme  
- 尽量无字，或固定测试串/字体；优先只测 chrome（圆角、边框、勾、点、按钮块）  
- 基线路径建议：`ui/visualtest/testdata/visual/<id>.png`（或等价）  
- 实现建议包：`ui/visualtest`（harness + compare + scenarios）

**第一期场景表（最低集）**

| ID | 内容 | 建议尺寸 |
|----|------|----------|
| `roundrect_fill_stroke` | r=6、border=1、填充+描边 | 64×64 |
| `button_primary` | primary 块（可无字或单字） | 120×40 |
| `button_default` | default 块 | 120×40 |
| `checkbox_off` / `checkbox_on` / `checkbox_indeterminate` | 指示器 | 32×32 |
| `radio_off` / `radio_on` | 真圆+内点 | 32×32 |
| `input_idle` / `input_focus` | 输入框轮廓 | 200×32 |

**比对规则**

1. 与本库基线比，禁止依赖外网截图。  
2. **容差**：如每通道 ≤2，或 diff 像素占比阈值（抗锯齿）；禁止默认「PNG 严格相等」作为唯一标准。  
3. 失败写出 `actual.png` + `diff.png`（差异高亮）。  
4. 有意改观感：显式更新基线（如 `UPDATE_VISUAL=1`），PR 说明改了什么。  

**与 Ant 的关系**：仅在 **建立或大改基线时** 人眼并排 ant.design；满意后 `UPDATE` 进库。CI 只锁「我们已认可的样子」。

**不做（保证可行）**：全窗口 GPU 像素 CI 优先；与官网实时截图比对；动画全程逐帧视频 diff。

##### 轨 3 — 人工走查（打磨棚）

**入口**：新建 `examples/ui_polish_gallery`（或扩展现有 smoke，须一屏可对照）。

**建议分区**：Button 多 Type · Input 多态 · Checkbox/Radio · 可选 Modal/Form。

**手操清单（打磨 PR 必勾，约 5～10 分钟）**

| # | 操作 | 看什么 |
|---|------|--------|
| 1 | 静态浏览 | 圆角、1px 边、间距是否像控件 |
| 2 | 鼠标扫 Button/Checkbox | hover/press 干净 |
| 3 | Tab 走焦 | focus 环可见 |
| 4 | 点 Checkbox/Radio | 选中圆滑、居中 |
| 5 | Linux 中文输入 | 预编辑、上屏、退格 |
| 6 | 开一次 Modal | 遮罩、焦点、Esc |

可选：快捷键导出当前帧 PNG，PR 附 2～3 张关键图。  
**Flutter**：只作交互/桌面化参照，不对标 Material 皮。  
**Ant**：设计语言参照，不要求像素哈希一致。

##### 动态效果

| 类型 | 测法 |
|------|------|
| Hover/Press/Focus | 轨 1 状态断言 + 可选每态一帧部件图；真窗手操 |
| 选中切换 | 单测 + 真窗点选 |
| Modal 开合 | 逻辑焦点 trap；开/关关键帧；动效真窗看 |
| Motion | 真窗；或 `TickClock` 抽 t=0 / 中 / 末 三帧（可选进轨 2） |
| IME | 见下小节 |

##### IME 子轨（与纯视觉并行）

| 级别 | 内容 |
|------|------|
| 自动 | Headless composition → preedit/commit 断言 |
| **必过人工** | Linux 真窗：拼音→选字→上屏→删除 |
| 可选 | caret / 候选位置眼看 |

无真窗 IME 通过，§12.1 C/E 中「输入可用」不得勾完成（除非正式记录 Caps 降级）。

#### 12.2.3 AI 改代码固定工作流

```text
改绘制/控件
  → go test ./ui/...
  → go test ./ui/visualtest/...     # 部件图 diff（落地后）
  → 失败看 diff.png，修到绿或有意 UPDATE 基线
  → 开 polish gallery 跑手操 1–6
  → 需要时并排 ant.design 一眼
  → 合入
```

**对 AI / 实现者约束**

1. 先改 `paint` / `Decorated`，再改单个 kit。  
2. 每个外观改动应对应 **≥1 个 scenario ID**（轨 2）。  
3. **禁止**仅用「非白像素数量」作为观感验收。  
4. 更新 visual 基线必须在 PR 说明观感变更。  

#### 12.2.4 与 §12.1 任务映射

| §12.1 | 主测法 |
|-------|--------|
| A1 圆角 stroke | 轨 2 `roundrect_*` + 轨 3 |
| A2 Decorated | 多控件同帧一致性 + 轨 2 button/input |
| A3 Token | 换 Token 后部件图/布局断言 |
| B1–B3 指示器 | 轨 2 checkbox/radio ROI + 轨 3 |
| B4–B5 Button/Input | 轨 2 多状态 + 轨 3 |
| B6 焦点环 | 轨 1 focus + 轨 3 Tab |
| C1–C4 IME | IME 子轨 |
| D/E 走查验收 | 轨 3 清单 + 轨 1/2 绿 |

#### 12.2.5 分期落地

| 阶段 | 交付 |
|------|------|
| 波次 1 | `ui/visualtest` harness + 第一期场景表 + 容差 diff；gallery 入口 + 手操清单 |
| 波次 2 | Checkbox/Radio/Button/Input 全进 scenario；IME 自动+真窗条目写入 checklist |
| 常态 | 新主路径控件合并前至少 1 张部件图；大改皮肤才批量 UPDATE 基线 |

#### 12.2.6 本方案成功标准

1. 破坏圆角/勾形的改动 → 轨 2 失败并出 diff。  
2. 行为回归 → 轨 1 失败。  
3. 打磨 PR 无轨 3 手操 → 不算 §12.1 完成。  
4. 团队以 **本库 visual 基线** 为 CI 真相；Ant/Flutter 仅人眼参照。  

#### 12.2.7 状态

| 项 | 状态 |
|----|------|
| 方案入文档 | ✅ §12.2 |
| `ui/visualtest` 落地 | ✅ W1–W3 scenarios + 基线 |
| `examples/ui_polish_gallery` | ✅ W3/W4 |

### 12.3 打磨执行波次（给 AI 的精确工作包）

> **总 Goal**：完成 §12.1 E 验收 + §12.2.6 成功标准。  
> **用法**：每个 AI 会话 **只执行一个波次**；会话开头粘贴该波「AI 只做 / 禁止」全文。上一波 DoD 全绿再开下一波。  
> **总约束（每波都适用）**：只改 `ui/**` 与必要 `examples/ui_polish_*`（或文档标明的 smoke）；不改引擎主线大重构；不讨论/重写架构；禁止仅用「非白像素计数」作观感验收；做完必须跑通本波测试并汇报文件列表与结果。

#### 全局禁止（所有波次）

| 禁止 | 说明 |
|------|------|
| 改 `render/` / `gpu/` 大行为（除非 paint 调用缺 API 且最小补丁） | 打磨在 UI 层 |
| 做 Table/Tree 像素精修、全 Ant 长尾 | 范围外 |
| 真 Win32/AppKit present | 后置 |
| 推倒 `core` 树模型 / 换包结构 | 不做 |
| 同一会话跨多个波次「顺便做完」 | 易失控；除非用户明确 `连续 W1–W4` |
| 静默大规模 UPDATE 全部 golden | 有意更新须在汇报中说明 ID |

---

#### W1 — 视觉测试基建 + 圆角描边

| 项 | 内容 |
|----|------|
| **波次 ID** | `W1` |
| **标题** | `ui/visualtest` + `PaintContext` 圆角 fill/stroke |
| **依赖** | 无（可直接开） |
| **对应清单** | §12.1 **A1**；§12.2 轨 2 波次 1 前半 |

**AI 只做**

1. 新建 `ui/visualtest`（或文档等价路径）：  
   - harness：创建 `render.Context` → 挂最小树或直接画 → `Image()`  
   - compare：与 `testdata/visual/<id>.png` 容差比对；失败写 `actual.png` / `diff.png`（或测试输出目录）  
   - 环境变量或测试 flag 更新基线（如 `UPDATE_VISUAL=1`）  
2. 实现并固定第一期场景 **至少一个**：`roundrect_fill_stroke`（64×64，r=6，border=1，填充+描边）。  
3. 修改 `ui/core/paint.go` 中 `FillLocalRoundRect` / `StrokeLocalRoundRect`，使圆角与 1px 描边对齐、不糊、无双边（满足 A1 DoD）。  
4. 将通过后的 `roundrect_fill_stroke` 基线入库。  
5. `go test` 覆盖：`./ui/core`、`./ui/visualtest`（新包）。

**AI 禁止做**

- 不改 Checkbox/Radio/Button/Input 业务逻辑（W2/W3）  
- 不接 IME / platform 宿主（W4）  
- 不做 gallery 全页（W3 可建最小入口则仅允许空壳，本波不强制）  
- 不扩 scenario 表到 button/checkbox（那些是 W2/W3）  
- 不「优化」无关 primitive  

**完成标准（DoD）**

- [ ] `go test ./ui/visualtest` 对 `roundrect_fill_stroke` 绿  
- [ ] 破坏 stroke inset/radius 会导致测试失败（机制有效）  
- [ ] `go test ./ui/core` 绿  
- [ ] 汇报：改动文件、是否 UPDATE 基线、未做项  

**本波交付物路径（预期）**

- `ui/visualtest/*.go`  
- `ui/visualtest/testdata/visual/roundrect_fill_stroke.png`（或等价）  
- `ui/core/paint.go`  

---

#### W2 — Decorated 单路径 + 指示器

| 项 | 内容 |
|----|------|
| **波次 ID** | `W2` |
| **标题** | Decorated/skin 统一 + Checkbox/Radio/Switch 指示器 |
| **依赖** | **W1 DoD 已满足** |
| **对应清单** | §12.1 **A2、A3、B1、B2、B3** |

**AI 只做**

1. 合并/对齐 `ui/primitive/decorated.go` 与 `ui/skin/default` 的绘制：fill→stroke 顺序、Token 解析、半径与边框宽度 **单一实现路径**（A2）。  
2. 主路径圆角/边框/控件高度尽量读 Token（A3）；去掉 kit 指示器上无必要的魔法数（合理默认可保留）。  
3. 打磨 **Checkbox** 指示器：小圆角方框、勾居中、on/off/indeterminate/disabled 可辨（B1）。  
4. 打磨 **Radio**：真圆 + 内点居中、组行为不回归（B2）。  
5. 若仓库已有 **Switch**：轨道/滑块几何与开关态（B3）；**无则跳过并在汇报写明**。  
6. 为轨 2 增加 scenario（在 W1 harness 上扩展）：  
   - `checkbox_off` / `checkbox_on` / `checkbox_indeterminate`  
   - `radio_off` / `radio_on`  
   - （可选）`switch_off` / `switch_on`  
7. `go test ./ui/primitive ./ui/kit ./ui/visualtest ./ui/core` 绿。

**AI 禁止做**

- 不系统改 Button/Input 的 Type 色板与高度体系（W3）  
- 不做 focus ring 专项（W3 B6）  
- 不接 IME（W4）  
- 不改 Table/Modal/Form 结构  
- 不为「更好看」引入第二套绘制 API 绕过 Decorated  

**完成标准（DoD）**

- [ ] Decorated 与 skin/default 无两套矛盾逻辑（或一处委托另一处，单一真源）  
- [ ] checkbox/radio（及已有 switch）visual scenario 绿  
- [ ] kit 相关单测绿；指示器真窗点选不崩（若本波无法真窗，须说明 + Headless 状态测绿）  
- [ ] 汇报：文件列表、基线 ID、跳过的 B3 与否  

**本波交付物路径（预期）**

- `ui/primitive/decorated.go` · `ui/skin/default/*`  
- `ui/kit/checkbox.go` 及 radio/switch 对应文件  
- `ui/visualtest/testdata/visual/checkbox_*.png` · `radio_*.png`  

---

#### W3 — Button / Input / 焦点环 + Gallery

| 项 | 内容 |
|----|------|
| **波次 ID** | `W3` |
| **标题** | Button/Input 观感、focus ring、polish gallery |
| **依赖** | **W2 DoD 已满足** |
| **对应清单** | §12.1 **B4、B5、B6、D1**（D2 预检） |

**AI 只做**

1. **Button**：高度/padding/圆角对齐 Token；primary/default/disabled/hover（及已有 Type）色与边可读（B4）。  
2. **Input**：边框、焦点态、placeholder 可读；与 Decorated 同源（B5）。  
3. **焦点环最小集**：键盘/程序聚焦时主路径 Button/Input（及可聚焦指示器）有可见 focus 反馈（B6）；不实现完整 a11y 树扩展。  
4. 轨 2 scenario：`button_primary`、`button_default`、`input_idle`、`input_focus`（尺寸按 §12.2 表）。  
5. 新增或扩展 **`examples/ui_polish_gallery`**（若坚持复用 smoke，须在同一示例内固定分区且文档写路径）：  
   - 一屏可见：Button 多 Type、Input、Checkbox/Radio、可选简单 Modal 入口  
   - README 或示例头注释写明 §12.2 手操清单 #1–4（#5–6 可在 W4 补 IME/Modal 说明）  
6. `go test ./ui/kit ./ui/visualtest` 绿；gallery 须能 `go run`（Linux 有显示时）。

**AI 禁止做**

- 不实现 IME composition 真宿主接线（W4）  
- 不精修 Table/Tree/Select 下拉像素  
- 不做暗色主题大改（除非 Button/Input 状态必须的 Token 微调）  
- 不把 gallery 做成完整 Ant 组件浏览器  

**完成标准（DoD）**

- [ ] button/input 相关 visual scenario 绿  
- [ ] gallery 可运行且覆盖 B4–B6 可视项  
- [ ] 手操清单 #1–4 可在 gallery 完成（作者自检或汇报步骤）  
- [ ] 汇报：截图可选、基线是否 UPDATE  

**本波交付物路径（预期）**

- `ui/kit/button.go` · `ui/kit/input.go` · focus 相关最小改动  
- `examples/ui_polish_gallery/`（或文档登记的等价路径）  
- `ui/visualtest/testdata/visual/button_*.png` · `input_*.png`  

---

#### W4 — IME 输入闭环 + 走查收口

| 项 | 内容 |
|----|------|
| **波次 ID** | `W4` |
| **标题** | Linux IME 主路径 + §12.1 D/E 收口 |
| **依赖** | **W3 DoD 已满足**（IME 子项可与 W3 后半并行，但收口勾选必须 W3 已完成） |
| **对应清单** | §12.1 **C1–C4、D2、D3、E** |

**AI 只做**

1. **C1**：Linux platform host 将 IME composition 事件送入 `Tree.DispatchIME`（或现有等价 API）；无能力则 **Caps 标明** 并实现可测降级说明。  
2. **C2**：`EditableText` / Input：preedit 展示、commit 写入、End 清理 preedit。  
3. **C3**：在 Caps 支持时 `SetIMEPosition`（或等价）贴 caret；不支持则文档/Caps 注释写清。  
4. **C4**：单行回归——自动：composition 序列单测；说明真窗中文步骤。  
5. gallery/smoke 补充 IME 说明与手操 #5–6（Modal 若已有则验收焦点/Esc）。  
6. **D2/D3/E**：对照 §12.1 E 勾选清单，在本文或 PR 中把已完成项标为完成；未完成项列出阻塞原因。  
7. 全量：`go test ./ui/...` 绿。

**AI 禁止做**

- 不回头大改 W1 圆角算法「为了更好看」除非 IME 无关回归必须  
- 不做 Win/mac IME  
- 不做多行 TextArea 富文本  
- 不新增长尾 Ant 组件  
- 不以「单测假事件绿」单独宣称 IME 完成（须真窗路径或正式 Caps 降级）  

**完成标准（DoD）**

- [ ] IME：真窗主路径可用 **或** Caps/文档正式降级且自动测覆盖降级路径  
- [ ] C 序列单测绿  
- [ ] §12.1 E 验收项全部勾选或标注阻塞  
- [ ] §12.2.6 成功标准可声称满足（visualtest 能挡坏图、手操清单跑过）  
- [ ] 汇报：E 勾选表、降级说明、残留债  

**本波交付物路径（预期）**

- `ui/platform/linux*` · `ui/core/event_scroll_ime.go` / `tree.go` 接线  
- `ui/primitive/editable.go` · `ui/kit/input.go`  
- 文档 §12.1 E / §12.3 波次状态更新  

---

#### 波次状态总表

| 波次 | 状态 | 说明 |
|------|------|------|
| W1 | ✅ 2026-07-21 | visualtest + paint 圆角 |
| W2 | ✅ 2026-07-21 | Decorated + 指示器 |
| W3 | ✅ 2026-07-21 | Button/Input/focus + gallery |
| W4 | ✅ 2026-07-21 | IME Caps 降级 + E 收口；Headless composition 测绿 |

（真窗 CJK CapIME/XIM 为残留债，见 `ui/platform/ime.go`。）

#### 给 AI 的会话首条模板

```text
你只执行 docs/UI_FRAMEWORK_MAP.md §12.3 波次 W?（把 ? 换成 1/2/3/4）。
严格遵守该波「AI 只做 / AI 禁止做 / DoD」。
先 Read §12.1、§12.2 相关条与本波涉及源文件，再改代码。
完成后按该波 DoD 自检并汇报：文件列表、测试命令与结果、基线是否 UPDATE、未做项。
不要进入其他波次。
```

---

## 13. 轨道与衔接

| 项 | 关系 |
|----|------|
| 引擎 G1–G3 | 并行；不挡 M0 |
| S5 | 已放行控件层 |
| gpu/context · exboot | platform 适配起点 |
| 历史 widget/ | 已删；不恢复为 core |
| 示例 antd_* | 视觉参考；M6 迁 **kit + primitive** |

**默认决策**

1. 包：`core` / **`primitive`** / `kit` / `platform` / `skin/default`；禁 `antd`。  
2. 底座 = primitive 组合；kit 只组合不魔法。  
3. 默认产品面可对标 Ant；非强制。  
4. 单树；Flex+Stack；Pressable 为交互核心。  
5. Register 不覆盖；Replace 显式。  
6. 测试：primitive 行为测 + kit 状态矩阵 + Headless。  
7. 跨平台：API 先、Linux 真、他端 stub→M6。

---

## 14. 开发就绪

| 闸门 | 结果 |
|------|------|
| 架构（primitive 底座） | ✅ 本文 v4.0 |
| 引擎入口 S5 | ✅ |
| M0 可开工 | ✅ 先 core+primitive，**不**先铺 Ant 全表 |
| 代码 | ✅ **M0–M6 主路径已落地**（2026-07-21） |

**判定：M0–M6 主路径已实现**（Linux 真窗 present；Win/mac SPI stub；kit 壳 + golden + 覆盖率表）。后续为真 Win/mac 适配与 Ant 长尾组件。

### 代码位置（M0–M1）

| 包/示例 | 路径 |
|---------|------|
| core | `ui/core` — Node/Constraints/Flex·Stack/Hit/Event/Paint/Tree/Plugin/**Theme·Token·Skin·Focus** |
| primitive | `ui/primitive` — P0 + **Decorated Slot Icon Focusable HitTarget Divider PainterNode** |
| platform | `ui/platform` — Caps/Host/Headless/Linux X11 薄适配 |
| skin/default | `ui/skin/default` — Ant light tokens + MapSkin |
| kit | `ui/kit` — B0–B3（Button…Form/Select/Tabs/Modal/Drawer/Message） |
| smoke | `ui_core_smoke` · `ui_kit_smoke` · `ui_kit_b1_smoke` · **`ui_kit_b2_smoke`** |
| 单测 | `go test ./ui/core ./ui/platform ./ui/primitive ./ui/kit` |

**未做（有意）**：Table/Tree 完整、虚拟列、复杂 DatePicker 等（M4+）、引擎主线改动。

---

## 15. 立即下一步

1. ~~M0–M6 主路径落地。~~ ✅  
2. **按 §12.3 波次执行打磨**：先 **W1** → W2 → W3 → W4（每波独立会话，见波次模板）。  
3. 每波结束对照该波 DoD；全部完成后勾选 §12.1 E 并更新 §12.3 波次状态表。  
4. 后置：真 Win32/AppKit present、Ant 长尾、visual 基线扩展。

---

## 16. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 1.x–3.1 | 总图合并、Ant 默认、API/Token/Props、自定义入口 |
| 2026-07-21 | **4.0** | **架构升级**：`ui/primitive` 为组合底层；kit 仅产品面（可对标 Ant） |
| 2026-07-21 | **4.1** | **Ant 全量反推**：能力/primitive/全组件组合表 |
| 2026-07-21 | **4.2** | **§12.1** 主路径视觉与输入打磨需求正文 |
| 2026-07-21 | **4.3** | **§12.2** 打磨测试方案：三轨逻辑/部件 PNG/人工走查、IME 子轨、AI 工作流 |
| 2026-07-21 | **4.4** | **§12.3** 执行波次 W1–W4：每波 AI 只做/禁止/DoD/交付物/会话模板 |
