# UI 底层 P0 — 稳住 kit 支撑层

> 版本：2.0 | 日期：2026-07-23  
> 状态：**P0 + F1–F8 + 三项收口已落地**（修饰键 · 浮层自动锚点 · Theme 广播）  
> 关联：[`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md) · [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) · [`UI_APP_SHELL_PLAN.md`](./UI_APP_SHELL_PLAN.md) · [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)

---

## 0. 一句话目标

在 **不扩 Ant 产品控件** 的前提下，把 `ui/kit` 所依赖的底层（`core` / `primitive` / `layer` / `platform` / `app`）补到 **硬契约可回归**，使 kit 只做组合与产品状态机，**禁止再在控件里打补丁抵消引擎错误**。

```text
core 管管道与契约 · primitive 管积木语义 · layer 管双带合成
app 管按需帧与默认 Present · platform 管 SPI
kit 只组合，不发明第二套 Hit / 浮层 / 帧循环
```

---

## 0.1 范围

| 包含（P0） | 不包含 |
|------------|--------|
| 布局/命中/绘制坐标契约门禁 | kit 新 Ant 组件、props 对齐官网 |
| 浮层栈统一（锚点 / 点外关闭 / Trigger delay） | Menu 嵌套、Date Range 完整面板等产品深度 |
| 滚动 + RepaintBoundary 嵌套规则锁死 | Sticky 完整 CSS 语义的「锦上添花」可降级为决策 |
| Editable 选区 / 多行 caret / 剪贴板 SPI | Linux XIM 真 IME（标 P0.4 可选并行，不堵门禁） |
| `ui/app` 默认 Compositor Present 上收 | 多窗口 / ExternalHost / OpenWindow 一站式（仍见 APP_SHELL） |
| 回归测试与唯一真窗路径 | Win/mac 真 Host、局部 damage present |

**原则**（摘自 [`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md)）：

> 先稳底盘，上层只消费规则，禁止在控件里打补丁抵消引擎错误。

---

## 0.2 现状基线（审计 · 2026-07-23）

| 项 | 状态 |
|----|------|
| 包测试 | `go test ./ui/core ./ui/primitive ./ui/layer ./ui/app ./ui/platform` **绿** |
| 按需帧 + 单窗 `Attach`/`Run`/`Pulse` | ✅ Phase 1 |
| 双带合成 `PaintMain`/`PaintOverlays` + `BlitTo` | ✅ `ui/layer` |
| hit==paint 契约工具 | ✅ `AuditHitPaintContract`（覆盖偏窄） |
| 默认 GPU Present | ✅ `ui/app.OwnedPresenter`；exboot 薄包装 |
| AnchoredPopup flip/shift / outside dismiss | ✅ flip/shift + `DismissOnOutside` + Tree registry |
| Trigger `DelayMs` | ✅ Ticker 延迟开 |
| CapClipboard | ✅ **跨平台** `NewSystemClipboard`：Linux xclip/xsel · Win PowerShell · mac pbcopy/pbpaste · 均 memory fallback；三端 Host + Headless + `app.Attach` 桥接 |
| Editable 完整选区 / 多行 caret | ✅ 选区 + 多行 caret 几何 + 点选 + Up/Down/Home/End 行内；选区高亮 |
| Sticky 真语义 | ✅ **决策 B**：Table 固定 Column 头；Notes 已更新 |

权威覆盖率（kit 产品面，**非本 P0 目标**）：[`ui/kit/coverage.go`](../ui/kit/coverage.go) · 70/70 `CovReady` 表示 API+Headless，不等于底层完备。

---

## 1. 依赖方向（硬）

```text
业务 / kit
    ↓ 只组合
primitive → core → render
app → core + platform  （+ layer 仅 Present 路径）
layer → core → render
platform → core（Dispatch）
```

禁止：

- kit 内第二套 Hit / 浮层栈 / 帧循环  
- kit 魔法 `offset` 修正点击（应修 primitive/core）  
- example 发明第二种 retained 合成序（绕过 `Compositor`）

---

## 2. P0 工作包

### P0.1 布局 / 坐标契约硬化

**问题**：规则已写在 `LAYOUT_FOUNDATION.md`，但回归面偏窄；kit 历史补丁（Switch/Tabs Y/Input 居中）说明契约未成为不可破门禁。

**目标**：任意主路径控件树 Layout 后 `AuditHitPaintContract` 无 issue；新 primitive 默认走 `DefaultHitTest` / `DefaultPaintChildren`。

| # | 任务 | 主要路径 | DoD |
|---|------|----------|-----|
| A1 | 扩大 HitPaint 矩阵 | `ui/core/hit_paint_contract*.go` | 覆盖 Button / Input / Switch / Checkbox / Select 触发器 / Modal footer `Row(Spacer,btn)` / Tabs 嵌套 |
| A2 | Flex / Flexible / Decorated 规则单测锁死 | `ui/primitive/*` · `layout_contract_test.go` | Flexible 只 tight 轴；Decorated 默认不 Center；Pressable 禁止松 MaxHeight 居中子 |
| A3 | 文档化「禁止 kit 魔法 offset」检查清单 | 本文件 §5 + PR 模板可选 | 评审可勾选；发现偏移先查底层 |

**锚点现状**：`core.AuditHitPaintContract` · `TestAuditHitPaint_TabsNestedControls` · `LAYOUT_FOUNDATION.md` 契约 1–6。

---

### P0.2 浮层底座统一（C-Overlay / C-Anchor / C-Trigger）

**问题**：Select / Dropdown / Tooltip / Popover / Cascader 各自灌 `Viewport`、各自关弹层；`Trigger.DelayMs` 未实现；无统一点外关闭。

**目标**：浮层行为在 primitive/core 一次实现，kit 只配 placement / 内容 / 产品 Z。

| # | 任务 | 主要路径 | DoD |
|---|------|----------|-----|
| B1 | Viewport 默认源 | `core.Tree` · `AnchoredPopup` | 未显式设置时用 `Tree.Viewport()`；kit 可覆盖 |
| B2 | 布局后锚点刷新 | `AnchoredPopup` · Portal 生命周期 | 打开期间 Layout/尺寸变化后位置正确（测：锚点移动 / 窗 resize） |
| B3 | flip / shift 最小完备 | `ui/primitive/anchored.go` | 四向 placement 越界可翻；水平/垂直 clamp；单测表驱动 |
| B4 | Outside dismiss | OverlayHost 或 Anchored 策略 | 统一 API（如 `DismissOnOutsidePointer`）；Select/Dropdown Headless 测：点空白关闭 |
| B5 | Trigger 真 delay | `ui/primitive/trigger.go` + Ticker | `DelayMs>0` hover 延迟开；leave grace 可后置但需文档；测 delay 内不开 |
| B6 | （可选）互斥组 | OverlayHost 策略 | 同 group 开新关旧；Tooltip 组可共用 |

**非目标**：箭头几何、复杂碰撞避让库、全屏接管。

**kit 受益**：Tooltip · Popover · Popconfirm · Select · Dropdown · Cascader · ColorPicker 面板。

---

### P0.3 滚动 + Boundary + Sticky 语义

**问题**：`ScrollViewport` 主路径可用且重，但嵌套滚轮、Boundary 脏、Sticky 语义是 kit（Tabs/Table/List/Modal 盖层）最脆区。

**目标**：行为可预测、可测、有文档决策；Modal 双带不被 Scroll 层盖住（已有合成，需锁回归）。

| # | 任务 | 主要路径 | DoD |
|---|------|----------|-----|
| C1 | 嵌套滚轮路由契约 | `tree.DispatchScroll` · `ScrollViewport` | 命中最近可滚视口；边界是否穿透写死 + 测 |
| C2 | Boundary 嵌套脏规则 | `scroll.go` composite-only 路径 | Scroll 内 Spin/Skeleton：子脏不强制整层重栅（现有方向锁成测） |
| C3 | Sticky 决策二选一 | `primitive.Sticky` · `kit.Table` | **决策 A**：Sticky 在 Scroll 内参与 paint+hit；**决策 B**：文档标明 Table 头为「固定 Column 简化」并改 Notes——P0 必须选定并测对应行为 |
| C4 | Tabs bar / 横滚 / 拖条 | 已有 `tabs_*` · `scroll_*` 测 | 全部保持绿；作为 P0 回归基线不得删弱 |

**合成硬规则**（已实现，P0 验收必查）：

```text
Present Z: mainBase → mainLayers → overlayBase → overlayLayers
真窗 retained 必须 Compositor.Frame + BlitTo
```

见 MAP §4.1 · `ui/core/doc.go` · `ui/layer/compositor.go`。

---

### P0.4 编辑内核 + 剪贴板（C-Edit / C-Clipbd）

**问题**：全站 Input/TextArea/Form 依赖 `EditableText`；选区与多行 caret 弱；剪贴板仅 Caps 名。

**目标**：Headless 下编辑主路径完备；平台剪贴板有 SPI + 至少一种实现（Headless 假实现即可测）。

| # | 任务 | 主要路径 | DoD |
|---|------|----------|-----|
| D1 | 选区（anchor/focus） | `ui/primitive/editable.go` | 拖选 / Shift+方向键；删除/输入作用于选区 |
| D2 | 多行 caret 几何 | 同上 | `Multiline=true` 时 caret 行/列正确（非 first-line only） |
| D3 | 单行跟光标水平滚动 | 同上 | 超长文本 caret 始终在可视 clip 内 |
| D4 | CapClipboard SPI | `ui/platform` | 接口 `Clipboard` 或 Host 可选能力；Headless 内存实现 |
| D5 | Editable Copy/Paste/Cut | editable + 快捷键 | Ctrl/Cmd+C/V/X（按 Caps）；Headless 测往返 |
| D6 | IME 契约锁死 | `HandleIME` · Headless `InjectIME` | preedit 可见、commit、End 清空；**Linux XIM 真窗为可选并行，不堵 P0 关门** |

**已有可复用**：`CaretLocalPos` · `HandleIME` · Headless `CapIME`（见 `platform/ime.go`）。

---

### P0.5 `ui/app` 默认 Present 上收

**问题**：双带合成正确路径在 `examples/exboot`；业务/smoke 易抄错导致「点得中 Modal、看起来没盖住」。

**目标**：唯一推荐真窗路径在 `ui/app`（或 `ui/app` 子文件），exboot 变薄包装。

| # | 任务 | 主要路径 | DoD |
|---|------|----------|-----|
| E1 | `DefaultPresent` / `PresentOwned` | `ui/app`（新文件可 `present.go`） | BeginFrame → `Compositor.Frame` → `BlitTo` → `PresentFrame*`；处理 resize / full paint |
| E2 | Session 接 Theme / Scale / Size | `Session` + Present | 从 Host 取 Size/Scale；Tree Viewport 同步 |
| E3 | 官方 example 迁路径 | `ui_kit_*` / `ui_polish_gallery` / shell | 不再内联第二套 compositor 循环；或一行调用 DefaultPresent |
| E4 | 文档 | `UI_APP_SHELL_PLAN` §0.1 更新 | 标明「默认 Present ✅」；OpenWindow 仍可 ⏳ |

**非目标**：`OpenWindow` 一站式建窗、多 Session、ExternalHost（仍属 APP_SHELL Phase 2/3）。

---

## 3. 建议执行顺序

```text
A 契约硬化 ──┬──► B 浮层底座 ──► kit 浮层族可去补丁
             │
             ├──► C 滚动/Boundary（与 B 可并行）
             │
             ├──► E 默认 Present（与 A 紧耦合，建议 A 后立刻 E）
             │
             └──► D 编辑/剪贴板（可与 B/C 并行）
```

**推荐迭代切片：**

| 迭代 | 内容 | 出口 |
|------|------|------|
| **P0-a** | A1–A3 + E1–E4 | 契约门禁 + 唯一真窗 Present |
| **P0-b** | B1–B5 + C1–C2 + C4 | 浮层/滚动可预期 |
| **P0-c** | C3 决策落地 + D1–D5 | 表单/列表底盘 |
| **P0-d** | D6 加固 + 全量回归 + 文档收口 | P0 关门 |

---

## 4. 分层责任（防回流）

| 该在底层做 | 禁止沉到底层 / 禁止 kit 重做 |
|------------|------------------------------|
| hit==layout==paint | Button Type/Danger 等产品 props |
| Portal / Anchor / Outside dismiss | Modal 标题文案、footer 产品布局 |
| Scroll / Boundary / Sticky 语义决策 | Table 排序 UI、列配置 |
| Editable + Clipboard SPI | Form 校验文案与 Ant 校验时机 |
| Ticker 机制 | Message 图标皮肤 |
| 默认 Compositor Present | 业务页面路由 |

**信号：若 kit 在算全局坐标、自己点外关闭、自己 BeginFrame——底层又欠债，应退回本 P0。**

---

## 5. 验收标准（P0 完成）

| 场景 | 期望 |
|------|------|
| HitPaint 矩阵 | CI 必绿；覆盖 §2 P0.1 控件树 |
| 静态 5s | present 次数不随时间线性涨（沿用 APP_SHELL） |
| Modal + Tabs Scroll | 有/无 compositor 路径：mask 盖住 main Scroll 层；点 mask 可关（MaskClosable） |
| Select/Dropdown | 打开后点空白关闭（统一底层策略）；锚点在触发器下/flip 正确 |
| Tooltip hover delay | `DelayMs` 生效（>0 时） |
| Input 选区 + 粘贴 | Headless：选区删除、剪贴板往返 |
| 真窗入口 | 新 example ≤15 行到 `Run`；**必须** `DefaultPresent`（或文档等价 API） |
| 回归 | `go test ./ui/core ./ui/primitive ./ui/layer ./ui/app ./ui/platform ./ui/kit` 绿；`ui_polish_gallery` 手操清单不回退 |

### 5.1 明确不作为 P0 关门条件

- Menu 嵌套子菜单  
- DatePicker 跨月 Range UI  
- Upload 系统文件框  
- Image GPU 纹理 / QR 真编解码  
- Linux XIM 真窗 composition（有 Headless 契约即可关门；真窗为增强）  
- 多窗口 / ExternalHost  

---

## 6. 源码锚点速查

| 主题 | 路径 |
|------|------|
| 布局契约 | `docs/LAYOUT_FOUNDATION.md` · `ui/core/hit_paint_contract.go` |
| 脏/帧/Ticker | `ui/core/node.go` · `ticker.go` · `tree.go` |
| 浮层宿主 | `ui/core/overlay.go` |
| 锚点 / Trigger / Mask / Portal | `ui/primitive/anchored.go` · `trigger.go` · `mask.go` · `overlay_portal.go` |
| 滚动 / Boundary | `ui/primitive/scroll.go` · `repaint_boundary.go` |
| Sticky / Grid / Drag | `ui/primitive/grid.go` |
| 编辑 | `ui/primitive/editable.go` |
| 合成 | `ui/layer/compositor.go` |
| 按需帧入口 | `ui/app/app.go` |
| **默认 Present（P0.5）** | `ui/app/present.go` · `OwnedPresenter` / `PaintCompositorFrame` |
| exboot 薄包装 | `examples/exboot/demand_ui.go` → 调 `app.NewOwnedPresenter` |
| IME Caps | `ui/platform/ime.go` · `caps.go` |
| HitPaint 矩阵 | `ui/core/hit_paint_contract_test.go`（Tabs + MainPath + Modal footer） |
| 布局契约测 | `ui/primitive/layout_contract_test.go` |

---

## 7. 与其它文档的挂钩

| 文档 | 关系 |
|------|------|
| [`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md) | P0.1 的规则源；本文件补 **门禁与任务** |
| [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) | 架构总图；P0 不改分层，只补管道完备度 |
| [`UI_APP_SHELL_PLAN.md`](./UI_APP_SHELL_PLAN.md) | P0.5 推进「默认 Present」；多窗仍后置 |
| [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md) | kit 覆盖率；**P0 不改 Ready 计数**，底层稳后 kit 去补丁 |
| [`S5_WIDGET_ENTRY.md`](./S5_WIDGET_ENTRY.md) | 控件入口条件；P0 完成后 kit 打磨更安全 |

---

## 8. 状态板

| 包 | 状态 | 备注 |
|----|------|------|
| P0.1 契约 | ✅ P0-a | HitPaint 主路径矩阵；Spacer/CenterContent/Pressable 契约测 |
| P0.2 浮层 | ✅ P0-b | Viewport 默认 Tree；flip/shift；OutsideDismiss；Trigger DelayMs |
| P0.3 滚动/Sticky | ✅ P0-c | 嵌套滚轮测；Sticky **决策 B**（Table 固定头） |
| P0.4 编辑/剪贴板 | ✅ | 选区+多行 caret+剪贴板 SPI；Linux xclip；真窗 XIM 后置 |
| P0.5 默认 Present | ✅ P0-a | `ui/app/present.go`；exboot 委托 OwnedPresenter |
| P0 关门 | ✅ 主门禁 | 可选增强：拖选、XIM、系统剪贴板无 xclip 时纯 X11 |

### 8.1 P0-a 交付记录（2026-07-23）

| 项 | 产出 |
|----|------|
| A1 | `TestAuditHitPaint_MainPathControls` · `TestAuditHitPaint_ModalFooterSpacer` |
| A2 | `TestSpacerDoesNotTakeLooseMaxHeight` · `TestDecoratedCenterContentDefaultOff` · `TestPressableChildTopLeftUnderLooseColumn` · `TestRowSpacerButtonsStayTop` |
| E1–E3 | `OwnedPresenter` / `UseCompositor` / `PaintCompositorFrame`；exboot 改为薄包装 |
| 验证 | `go test ./ui/app ./ui/core ./ui/primitive ./ui/layer ./ui/platform ./ui/kit` 绿；`ui_kit_smoke` / `ui_polish_gallery` 可编译 |

### 8.2 P0-b 交付记录（2026-07-23）

| 项 | 产出 |
|----|------|
| B1 | `AnchoredPopup.resolveViewport` → `Tree.Viewport()` |
| B2 | `AnchorNode` + Layout 刷新 |
| B3 | 四向 flip + clamp；`TestAnchoredPopupFlipBottomToTop` |
| B4 | `Tree.RegisterOutsideDismiss`；`DismissOnOutside` 默认 true；`TestOutsideDismiss*` |
| B5 | `Trigger.DelayMs` + Ticker；`TestTriggerHoverDelay` |
| 验证 | `go test ./ui/core ./ui/primitive ./ui/kit` 绿 |

### 8.3 P0-c/d 交付记录（2026-07-23）

| 项 | 产出 |
|----|------|
| C1 | `TestNestedScrollWheelHitsInner` · `TestNestedScrollWheelOnOuter` |
| C3 | **决策 B**：Table 固定 Column 头；`coverage.go` Notes 更新 |
| D1 | Shift+方向键选区；Backspace/Delete 删选区；插入替换选区 |
| D2–D3 | **多行 caret 几何**、点选、Up/Down、行内 Home/End、选区高亮；`editable_multiline_test.go` |
| D4–D5 | `core`/`platform` Clipboard；Headless + Linux xclip；`app.Attach` 桥接 |
| D6 | 既有 Headless IME 契约保留（未扩 XIM） |
| 验证 | `go test ./ui/core ./ui/primitive ./ui/platform ./ui/kit ./ui/app` 绿 |

### 8.4 F1 拖选交付记录（2026-07-23）

| 项 | 产出 |
|----|------|
| Pointer 修饰键 | `core.PointerEvent.Shift/Ctrl/...`；`platform.Dispatch` 映射 |
| 拖选 | Down 置 anchor+dragging；Move 更新 Cursor；Up/Cancel 结束（树 capture 保证移出仍收到 Move） |
| Shift+click | 已聚焦时保持 SelAnchor、只移 Cursor；不进入 drag |
| 失焦 | 清除 dragging |
| 测试 | `editable_drag_test.go`（单行拖选 / Shift 扩展 / 多行 / 折叠 / Cancel） |
| 验证 | `go test ./ui/primitive ./ui/core ./ui/platform ./ui/kit` 绿 |

### 8.5 F2–F4 交付记录（2026-07-23）

| 项 | 产出 |
|----|------|
| **F3** 浮层 kit 回归 | `Select`/`Dropdown`/`Popover`：`OnDismiss` 同步 `Open`；`floating_dismiss_test.go` |
| **F4** Present 路径 | 全部 `ui_*` 真窗 smoke 走 `RunUIDemand`；`present_path_test.go` 锁清单；app 包注释 |
| **F2** Cursor 桥接 | `app.Attach` → `Tree.SetOnCursor` → `CursorHost`；Headless/Win/mac `SetCursor`；`cursor_bridge_test.go` |
| 验证 | `go test ./ui/app ./ui/platform ./ui/kit ./ui/primitive ./ui/core ./ui/layer` 绿 |

### 8.6 F6 Text 省略 / measure（2026-07-23）

| 项 | 产出 |
|----|------|
| `primitive.Text` | `Ellipsis` · `MaxLines` · `MaxWidth`；单行截断 `…`；多行 soft-wrap + 末行省略 |
| Paint | `PushClipLocal` 防溢出绘制 |
| kit | `kit.Text` SetEllipsis/MaxWidth/MaxLines；`Paragraph` 默认 MaxLines=8；`Tag` label MaxWidth+Ellipsis |
| 测试 | `text_ellipsis_test.go` |
| 验证 | `go test ./ui/primitive -run Text` 绿 |

### 8.7 F7 嵌套滚动边界（2026-07-23）

| 项 | 产出 |
|----|------|
| 语义 | **默认 chain-at-edge**：内层无法吸收 delta（到顶/底或无 overflow）时不设 `Handled`，事件冒泡到祖先 |
| `TrapWheel` | `true` 时始终 `Handled`（锁死内层，如某些浮层列表） |
| API | `ScrollViewport.TrapWheel` · `MaxScroll()` |
| 测试 | `scroll_nested_wheel_test.go`（中段不穿透、顶/底穿透、Trap、无 overflow 穿透） |
| 验证 | `go test ./ui/primitive -run NestedScroll` 绿 |

### 8.8 F5 IME 桥接 + F8 Theme 下发（2026-07-23）

| 项 | 产出 |
|----|------|
| **F8 Theme** | `ThemeProvider` / `ResolveTheme` / `ThemeOrDefault`；`Tree.SetTheme`；`ConfigProvider` 真节点；kit `themeOf` 全控件 |
| **F5 IME** | `Tree.SetOnIMEPosition`；Focus/Key/IME 后推送 caret；`app.Attach` 桥接 `IMEPositioner`；Linux **仍无 CapIME**（正式降级见 `platform/ime.go`） |
| 测试 | `theme_resolve_test.go` · `ime_position_test.go` |
| 验证 | `go test ./ui/core ./ui/kit ./ui/app` 绿 |

### 8.9 三项收口（修饰键 · 浮层 · Theme 广播 · 2026-07-23）

| 项 | 产出 |
|----|------|
| **① 修饰键** | Linux X11 `state@80` → `Event.Shift/Ctrl/Alt/Meta`（键/指针/滚轮）；`ParseModifierState` 跨平台 helper；Headless `Inject*Mods`；`Dispatch` 已映射 |
| **② 浮层锚点** | `OpenGeometryRefresher` + `Tree.Layout` 后 `refreshOpenGeometry`；`AnchoredPopup.RefreshOpenGeometry`；kit `Sync` **Deprecated** |
| **③ Theme 广播** | `SetTheme` → epoch + `ThemeObserver`/`themeHook` 遍历 + FullPaint；`NotifyThemeChanged`；Button/Input `SetThemeHook`→rebuild；ConfigProvider.SetTheme 同源 |
| 测试 | `modifiers_test` · `dispatch_mods_test` · `theme_broadcast_test` · anchor layout test |
| 跨平台 | ① Host 填字段；②③ 纯 core/primitive/kit，Win/mac 自动继承 |

---

## 9. 开工检查清单（复制到 PR）

```text
[ ] 只改 core/primitive/layer/app/platform（或 exboot 变薄）；无新 Ant 产品控件
[ ] 新增/扩展契约测（HitPaint / 浮层 / 滚动 / 编辑）
[ ] 无 kit 魔法 offset；无第二套帧循环
[ ] Present 路径：Compositor 双带或显式 Headless
[ ] go test ./ui/core ./ui/primitive ./ui/layer ./ui/app ./ui/platform ./ui/kit
[ ] 更新本文 §8 状态板（若关闭某包）
```
