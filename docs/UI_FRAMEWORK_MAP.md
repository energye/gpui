# UI 框架总图与规划 — Primitive 组合底座 × Kit 产品面 × Flutter 管线 × render

> 版本：4.1 | 日期：2026-07-21 | **活文档 · M0–M5 已落地**  
> 状态：**控件产品架构以「primitive 组合」为底座**；已含 **Ant 全量组件 → 组合能力反推清单**  
> 入口：[`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md) ✅  
> 引擎：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md) · [`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md)（控件 = **P2 另开轨道**）

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
│     Box Flex Stack Text Icon Pressable Focusable EditableText… │
├──────────────────────────────────────────────────────────────┤
│ L3a ui/core（框架运行时 · 非控件）                               │
│     Node 树 · 布局算法 · Hit · Focus · Event · Frame · Plugin  │
├──────────────────────────────────────────────────────────────┤
│ L2  ui/platform（跨平台 SPI + Caps）                            │
├──────────────────────────────────────────────────────────────┤
│ L1  render（Context · PresentFrame* · text · lifecycle）        │
└──────────────────────────────────────────────────────────────┘
```

| 层 | 包 | 职责 | 禁止 |
|----|-----|------|------|
| L1 | `render` | 像素与 present | 布局树、控件状态机 |
| L2 | `ui/platform` | 窗口/输入/IME/剪贴板/能力 | 产品控件 API |
| L3a | `ui/core` | 管线与算法 | 具体色值、产品控件名、OS API |
| L3b | `ui/primitive` | **通用可组合积木** | 依赖 kit、Ant 专有枚举当唯一 API |
| L3c | `ui/skin/*` | 外观与组装绘制 | 自管 swapchain |
| L4 | `ui/kit` | 产品级控件 API | 硬编码色、绕过 primitive 堆业务绘制、包名 antd |
| L5 | app | 页面与扩展 | 绕过 core 直接 GPU（除 host 引导） |

**依赖方向（只允许向下）**

```text
app → kit → primitive → core → render
 app → primitive → core          // 允许：业务直接组合积木
      skin → primitive + core    // 皮肤不依赖 kit 类型名时可只挂 typeID
      * → platform SPI           // 不直连 OS
```

禁止：`primitive → kit`；`core → primitive/kit`；`render → ui`；`kit/core/primitive → X11/Win32/AppKit`。

**绘制铁律**：只 `render.Context`；只 `PresentFrame*`；禁止 silent CPU 冒充 GPU；布局/Hit/Focus/IME ≠ ENGINE_GAPS。

---

## 2. 包路径与目录

```text
<module>/ui/core
<module>/ui/primitive          # 组合底层（★）
<module>/ui/kit                # 产品面（默认可 Ant 向）
<module>/ui/platform
<module>/ui/platform/linux|windows|darwin
<module>/ui/skin/default       # 默认可 Ant 视觉
<module>/ui/skin/<name>
```

| 规则 | 说明 |
|------|------|
| 禁止 | `ui/antd` |
| 应用默认 | `kit` + `theme.Default()`（内聚 primitive 组装 + default skin） |
| 重度自定义 | 主要 import `primitive`，可不依赖 kit |
| 类型 ID | `primitive.Pressable`、`kit.Button`（稳定字符串，与路径解耦） |

```text
ui/
  core/         node layout hit focus event frame paint theme plugin services edit/
  primitive/    box flex stack text icon pressable focusable editable scroll slot decorated ...
  kit/          button input form modal table menu ...   # 内部只组合 primitive
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
| Overlay | primitive.OverlayPortal + kit Modal 等 |
| 替换官方 Button 工厂 | 非 Flutter 强项；本框架可选显式 `ReplaceControl` |

**帧顺序**

```text
PumpEvents → Dispatch → flush setState
  → Layout(root) → Paint → PresentFrame*
```

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
| **Drawer** | OverlayPortal + 滑入面板 Box + Mask + FocusTrap | P3 |
| **Message** | PortalHost + **C-NotifyQueue** + 项 Decorated；C-Presence | P3 |
| **Modal** | OverlayPortal + Mask + 居中面板 + FocusTrap + 脚 Button 组合 | P3 |
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

### 5.7.8 覆盖率与缺口统计

| 项 | 数量/结论 |
|----|-----------|
| Ant 组件条目（含组） | **约 70+**（overview 列表） |
| 仅用 P0–P2 能力可组合（主路径桌面） | Button Icon Flex Space Typography(基) Divider Layout(基) Input 系 Checkbox Radio Switch Tag Card Alert Tooltip Popover Dropdown 基 Modal/Drawer 基 Spin Avatar Badge Empty… **~30** |
| 强依赖 P3+（虚拟/表/树/复杂选择/队列） | Table List Tree Select Menu Tabs Form Cascader Transfer DatePicker… **~25** |
| 强依赖平台/后置 | Upload(FileHost) Tour ColorPicker 完整 QR Masonry… **~10+** |
| **Primitive 侧缺口（相对旧 §5 仅 15 项）** | 必须补：**AnchoredPopup、Mask、Grid、VirtualList、Draggable、SplitPane、Canvas、Sticky、FocusScope、SelectionScope、RichText、Divider、Notify 级 PortalHost 能力** |
| **架构能力缺口** | 必须补：**C-Anchor C-Overlay C-Virtual C-Drag C-FormBind C-KeyboardNav C-NotifyQueue C-Motion C-Sticky C-ScrollSpy C-FileHost C-SelectModel** |

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
OverlayPortal(fullscreen)
  └─ Stack
       Pressable(mask) · Decorated(panel)
            Flex(Column): title · content · footer(Buttons)
```

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

**Modal**：Open、Title、Content、OnOk/OnCancel、MaskClosable、Keyboard Esc、Width；focus trap。  

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


### M6 — 壳与跨平台

示例迁 kit、Win/mac、golden、§5.7 覆盖率对照更新。

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
| 代码 | ✅ **M0–M5 已落地**（2026-07-21） |

**判定：M0–M5 已实现。** 跨平台/示例迁 kit/golden 走 M6。

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

1. ~~冻结 v4.0 分层与 primitive 清单。~~  
2. ~~脚手架：`ui/core` + `ui/primitive` + `ui/platform`（Headless）+ smoke。~~ ✅  
3. ~~M1：`ui/kit` 与 `skin/default`。~~ ✅  
4. ~~M2：EditableText、IME、Input 系、Overlay/Mask/AnchoredPopup、ScrollViewport。~~ ✅  
5. ~~M3：FormBind、SelectionScope、Modal/Drawer、VirtualList 起步。~~ ✅  
6. ~~M4：Grid/Sticky/Table/List/Tree、Pagination 等。~~ ✅  
7. ~~M5：Motion/Presence/Canvas 增强、A11y 最小集。~~ ✅  
8. M6：Win/mac stub→真适配、示例迁 kit、golden、§5.7 覆盖率。  
9. 实现偏差回写本文。

---

## 16. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-21 | 1.x–3.1 | 总图合并、Ant 默认、API/Token/Props、自定义入口 |
| 2026-07-21 | **4.0** | **架构升级**：`ui/primitive` 为组合底层；kit 仅产品面（可对标 Ant）；组合公式/清单/扩展四级；里程碑改为底座优先 |
| 2026-07-21 | **4.1** | **Ant 全量反推**：§5.3 架构能力 36 项；§5.4 primitive 28 种；§5.7 全组件组合表与覆盖率/缺口统计；里程碑对齐分层 0–8 |
