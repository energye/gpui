# UI Kit 开发指南 — 底层契约（必读）

> 版本：1.3 | 日期：2026-07-24  
> 状态：**活文档** · 与源码冲突时以源码 + `go test` 为准  
> 底层交付总账：[`UI_FOUNDATION_P0.md`](./UI_FOUNDATION_P0.md) v2.1+  
> **Ant 对齐目标/验收：** [`UI_KIT_ANT_V6_SPEC.md`](./UI_KIT_ANT_V6_SPEC.md)（L1 行为 · L2 Token · L3 本库 golden · L4 人眼）  
> 架构总图：[`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) · 布局：[`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md)  
> 覆盖率表：[`ui/kit/coverage.go`](../ui/kit/coverage.go) · [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)  
> 引擎层 CPU/GPU 正向任务卡（**非控件**）：[`PERF_ENGINE_FORWARD.md`](./PERF_ENGINE_FORWARD.md)

---

## 0. 一句话

```text
kit = 产品 Props + 状态机 + a11y 名 + 组合 primitive
禁止：第二套 Hit / 帧循环 / 每帧 Sync / 魔法 offset / 硬编码色当默认皮
禁止：rebuild 写死度量且无 DefaultXxx + SetXxx（见 §0.1）
禁止：能力只测不展 — 示例必须进 ui_polish_gallery 对应控件分类（见 §2.7）
禁止：值变更整树 rebuild · 静止挂 Ticker · ContinuousRender · 滥开 RepaintBoundary（见 §2.9）
对齐 Ant：见 UI_KIT_ANT_V6_SPEC（行为 + Token + 本库 golden；非浏览器像素哈希）
```

依赖方向（硬）：

```text
业务 → ui/app → ui/kit → ui/primitive → ui/core → render
              ↘ ui/platform（仅 Host 注入）
真窗 Present：ui/app.OwnedPresenter（或 exboot.RunUIDemand 薄包装）
```

---

## 0.1 默认值与配置（**强制 · 全控件**）

> **后面开发的每个 kit 控件必须按本节实现。** Review / PR 可据此拒合。

### 铁律

```text
1. DefaultXxx 常量 = 真实 Ant Design v5 控件的默认度量（padding / gap / width / height / font…）
2. 公开字段 + SetXxx / SetXxxInsets = 业务使用时覆盖
3. rebuild() 只读「解析后的默认或字段」，禁止散落 magic 数字且无 Default/API
4. 色/圆角/控件高优先 Token（th.SizeOr / th.Color）；Token 没有的再写 Default 常量
5. 未调用 Set 时：走 Default（Ant 真默认）
6. 调用 Set(零值) 时：视为显式配置（用 bodyPadSet / padSet 等 flag 区分「未设」与「设为 0」）
```

| 允许 | 禁止 |
|------|------|
| `const DefaultModalPadding = 24` + `SetPadding` | `panel.Padding = All(24)` 且无字段/setter |
| `DefaultTabBodyPadding = {}`（Tabs **壳**无 inset，内容自 pad） | 为「省事」把 Modal/Card 的 Ant 默认改成全 0 |
| 0 表示「用 Default」的 float 字段（`Width float64`） | 业务只能改源码才能调间距 |
| Theme Token 作默认（`TokenControlHeight`） | 硬编码品牌色当唯一默认皮 |

### 实现模板（新控件复制）

```go
// Ant Design Xxx defaults — https://ant.design/components/xxx
const (
    DefaultXxxPadding   = 24.0
    DefaultXxxTitleFont = 16.0
    DefaultXxxGap       = 12.0
)

type Xxx struct {
    // Padding uniform inset (0 → DefaultXxxPadding). Prefer SetPaddingInsets for sides.
    Padding       float64
    TitleFontSize float64 // 0 → DefaultXxxTitleFont
    Gap           float64 // 0 → DefaultXxxGap
    // …
    pad    primitive.EdgeInsets
    padSet bool
}

func (x *Xxx) SetPadding(px float64) {
    x.Padding = px
    x.padSet = false
    x.rebuild()
}

func (x *Xxx) SetPaddingInsets(p primitive.EdgeInsets) {
    x.pad = p
    x.padSet = true // explicit, including all-zero
    x.rebuild()
}

func (x *Xxx) padding() primitive.EdgeInsets {
    if x.padSet {
        return x.pad
    }
    px := DefaultXxxPadding
    if x.Padding > 0 {
        px = x.Padding
    }
    return primitive.All(px)
}

func (x *Xxx) rebuild() {
    // …
    root.Padding = x.padding() // never literal All(24) here
}
```

### 壳 vs 产品控件

| 类型 | 默认策略 | 示例 |
|------|----------|------|
| **产品控件** | Ant 真实 chrome（有 pad/gap/宽） | Card 24 · Modal 24 · Drawer 24 · Form item 24 |
| **布局壳** | 可为 0；内容节点自己 pad | `DefaultTabBodyPadding={}` · 需要时 `SetBodyPadding` |

### 已落地范例（抄作业）

| 控件 | Default | API |
|------|---------|-----|
| Tabs | `DefaultTabBodyPadding` · width/ink/pad* | `SetBodyPadding` · `SetTabWidth` · … |
| Card | `DefaultCardPadding=24` 等 | `SetPadding` · `SetPaddingInsets` · `SetTitleFontSize` · gaps |
| Modal | `DefaultModalPadding=24` · `DefaultModalWidth=520` | `SetPadding` · `SetPaddingInsets` · `SetWidth` |
| Drawer | `DefaultDrawerPadding=24` · `DefaultDrawerWidth=378` | `SetPadding` · `SetPaddingInsets` · `SetWidth` |
| Form | `DefaultFormItemGap=24` · Field/Error gap | `SetItemGap` · `FieldGap`/`ErrorGap` 字段 |
| Button / Input | Size ladder + Theme Token | `SetSize` · `Style` · `SetFixedSize` |
| Space | gap → TokenMarginSM | `SetSize` · `SetWrap` |

### 验收

- [ ] 存在 `Default*` 常量（或明确走 Theme Token，注释写清）  
- [ ] 存在字段 + `Set*`；Insets 类支持「显式全 0」  
- [ ] `rebuild` 无裸 `All(16)` / `Gap = 24`（除非只出现在 Default 常量定义处）  
- [ ] Headless：`默认几何` + `Set 后几何变化` 至少各一条  
- [ ] 业务用法：`NewXxx` 即 Ant 默认；`SetXxx` 再定制  

```go
// 业务侧
m := kit.NewModal("确认")           // Ant 默认
m.SetPadding(16)                   // 使用时覆盖
m.SetWidth(640)
```

---

## 1. 底层已交付能力（kit 可直接依赖）

| 能力 | 用法摘要 | 源码 |
|------|----------|------|
| 布局契约 | hit == layout == paint；Flexible 只 tight 轴；Decorated 默认不居中 | `LAYOUT_FOUNDATION` · `primitive/*` |
| 脏 / 按需帧 | `MarkNeedsLayout` / `MarkNeedsPaint` / `AddTicker`；禁止 Continuous 动画控件；**写控件正向优化见 §2.9** | `core` · `app` · 本文 §2.9 |
| 双带合成 | Modal 等 portal 必须 Compositor 路径；`OwnedPresenter` | `layer` · `app/present.go` |
| 浮层 | `OverlayPortal` + `AnchoredPopup`；**Layout 后自动锚点**；点外关闭 | `anchored.go` · `Tree.Layout` |
| 滚动 | `ScrollViewport`；嵌套 **到边穿透**；`TrapWheel` 锁内层 | `scroll.go` |
| 编辑 | 选区 / 拖选 / Shift+点 / 多行 caret / 剪贴板快捷键 | `editable.go` |
| 文本 | `MaxWidth` · `MaxLines` · `Ellipsis` | `text.go` |
| Theme | `themeOf` / ConfigProvider / Tree.Theme / **SetTheme 广播** | `theme_resolve.go` · `core` |
| 修饰键 | `ev.Shift/Ctrl/Alt/Meta`（Linux 真窗已填；Headless `Inject*Mods`） | `platform` |
| Cursor / IME 位 | `app.Attach` 自动桥接 Host | `app/app.go` |

**未交付（勿在 kit 文档宣称已支持）：**

- 真窗 CJK IME composition（Linux 无 CapIME；Headless 可测）  
- Win/mac 真窗口 + GPU Present  
- Sticky 进 Scroll 真吸顶（Table = 固定表头 Column）  
- VirtualList 动态行高 · Icon SVG Path · 多窗 API  

---

## 2. 新控件实现清单（PR 自检）

### 2.0 默认值与配置（§0.1 · **必过**）

- [ ] `Default*` 对齐 Ant v5 真默认（或注释写明 Theme Token）  
- [ ] 公开字段 + `SetXxx` / `SetXxxInsets`（Insets 能显式全 0）  
- [ ] `rebuild` **无**无 API 的 magic padding/gap/width  
- [ ] Headless：默认几何 + Set 后变化  
- [ ] 业务：`New` = 默认；使用时 `Set` 定制  

### 2.1 组合

- [ ] 只组合 `ui/primitive`（+ 少量已有 kit 子件）  
- [ ] 无 `ui/platform` / X11 / GPU device（Upload 注入 `Picker` 除外）  
- [ ] 绘制只经 `PaintContext` → `render.Context`  

### 2.2 布局 / 命中

- [ ] 不写魔法 Y offset 修点击  
- [ ] 子节点用 `DefaultHitTest` / `DefaultPaintChildren` 或显式 `Offset` 一致  
- [ ] `Flexible`/`Spacer` 不用松 `MaxHeight` 撑高（Modal footer 教训）  
- [ ] 需要居中时显式 `SetCenterContent(true)` / `FillChild`  

### 2.3 状态

- [ ] hover/press/focus 来自 `Pressable` / `Focusable` / `EditableText`  
- [ ] chrome 更新挂 **`OnStateChange`**，**禁止**新增「主循环必须每帧 SyncState」  
- [ ] 动画用 `Tree.AddTicker` + `MarkNeedsPaint`，**禁止** `ContinuousRender`  
- [ ] 渲染正向优化见 **§2.9**（patch vs rebuild · Boundary · Ticker 生命周期）  

### 2.4 浮层

- [ ] Modal/Drawer/Dropdown/Select/Tooltip/Popover/Message → **Portal + Anchored/Mask**  
- [ ] 打开：`SetOpen` + `UpdateAnchorFromNode`（或 AnchorNode）  
- [ ] **不要**依赖每帧 `Sync()`；Layout 会 `RefreshOpenGeometry`  
- [ ] `DismissOnOutside` + `OnDismiss` 同步产品 `Open` 字段  
- [ ] Z 序用 `Portal.ZOrder`；合成走 app Present 双带  

### 2.5 Theme

```go
func (c *MyControl) theme() *core.Theme {
    var n core.Node
    if c.Root != nil {
        n = c.Root
    }
    return themeOf(c.Theme, n) // 字段 > ConfigProvider > Tree > DefaultTheme
}
```

- [ ] rebuild 末尾：`Root.SetThemeHook(func(*core.Theme) { c.rebuild() })`  
- [ ] 色值走 Token（`th.Color(core.Token…)`），禁止默认硬编码品牌色  
- [ ] 单实例覆盖用 `Theme` 字段或 `Style`，不用 fork Decorated 画法  

### 2.6 文本 / 列表

- [ ] 长文：`Text.Ellipsis` + `MaxWidth` / `MaxLines`  
- [ ] 虚拟列表：**固定行高** `VirtualList` only  
- [ ] Table 头：固定 Column，勿当 CSS sticky  

### 2.7 真窗示例（`examples/ui_polish_gallery` · **必过**）

> **每个可演示的能力都必须出现在 gallery 对应控件分类页里**，禁止只写单测、不写可见示例。

```text
能力落地 = Headless 测 + coverage Notes + gallery 对应 Tab 的 demo section
```

| 要求 | 说明 |
|------|------|
| **分类** | 与左侧 rail 控件一致：Button 能力 → `General · Button`；Flex shrink → `Layout · Flex`；勿塞进无关 Tab |
| **结构** | 优先 `demoPage` + `demoSection`（Ant 文档页形态）；旧 `panel` 逐步替换 |
| **内容** | 覆盖本控件 API 主路径（默认 + 重要 props）；与 [ant.design 组件页](https://ant.design/components/overview) 分区对齐 |
| **新能力** | 例如 FlexShrink、Wrap、Ghost → 加在 **本控件** 页新 section，并更新 section 说明文案 |
| **禁止** | 只在 `*_test.go` / 文档描述能力、gallery 无入口；把示例堆在 `main.go` 杂项区 |

```bash
# 手操验收
go run ./examples/ui_polish_gallery
# 切到对应 Tab，确认新 section 可见、可点
```

### 2.8 测试

- [ ] Headless：布局稳定、状态回调、Hit、（若有）打开/关闭浮层  
- [ ] 不引入第二套 present 循环  
- [ ] 覆盖率变更只改 `coverage.go`，再同步 `UI_KIT_COVERAGE.md` 数字/Notes  

### 2.9 渲染正向优化（写控件 · **必读**）

> **目标：** 在不减能力、不降观感的前提下，让每帧工作量只覆盖「真的变了」的部分。  
> **边界：** 本节是 **kit / primitive 组合层** 正向优化。GPU pass / submit / atlas 等引擎刀见 [`PERF_ENGINE_FORWARD.md`](./PERF_ENGINE_FORWARD.md)，**禁止**在控件里直接改 present / device。

#### 一句话

```text
值/色/角度变 → patch + MarkNeedsPaint
结构/尺寸变 → rebuild（或局部换子）+ MarkNeedsLayout
动画中 → AddTicker，Tick 内只 paint；静止 → Remove / Tick 返回 false
持续自驱动动画（Spin/Skeleton）→ 可开 RepaintBoundary
海量实例 / 嵌 Scroll 的低频控件 → 默认不要 Boundary
长列表 → VirtualList（固定行高）；禁止一次挂满万行节点
```

#### 铁律

| 允许 | 禁止 |
|------|------|
| `SetPercent` / 拇指位移 **复用根节点**，只改属性 + dirty | 每次值变更 `rebuild()` 拆掉整棵子树 |
| hover/press 走 `OnStateChange` → chrome 局部 paint | 主循环每帧 `SyncState` / 每帧 `rebuild` |
| `Tree.AddTicker` + Tick 内 `MarkNeedsPaint` | `ContinuousRender` / 假动画刷帧 |
| 动画结束 `RemoveTicker` 或 `Tick` 返回 `false` | 静止仍挂 Ticker（整树一直 ANIMATING） |
| Spin / Skeleton：`SetRepaintBoundary(true)` | 给每个 Button / Pressable 开 Boundary（Tabs 海量实例会炸） |
| Progress 等嵌在 Scroll 下的低频控件：**不要** Boundary | 嵌套 Boundary 在 `SkipRepaintBoundaries` 路径下「打洞」丢像素 |
| 长表/菜单：`VirtualList` 固定行高 + 可视窗口 | 全量 `for` 生成所有行节点 |
| 文本度量缓存（如 Switch label face cache） | Paint / 每帧 `Glyphs`/`Advance` 重复量字 |
| 相等值 early-return（`if p.Percent == v { return }`） | 无变化仍 `MarkNeeds*` 刷帧 |

#### 脏标记怎么选

| 变化类型 | 调用 | 说明 |
|----------|------|------|
| 仅颜色 / 同尺寸文案 / 旋转角 / 进度填充色 | **`MarkNeedsPaint`** | 不抬 layout；气泡到最近 RepaintBoundary 停 |
| 宽高 / padding / 子节点增删 / 换结构 | **`MarkNeedsLayout`**（隐含 paint） | 气泡祖先；约束未变可 early-out |
| 滚动偏移（ScrollViewport 路径） | 改 `ContentPaintOffset` + paint | **禁止**为滚动改 child layout Offset |
| 主题切换 | `SetThemeHook` → `rebuild` | 由 Tree 广播；勿轮询 Theme |

```go
// 好：值更新 patch（Progress.SetPercent 模式）
func (p *Progress) SetPercent(v float64) {
    if p.Percent == v && p.bar != nil {
        return // 无变化不 dirty
    }
    p.Percent = v
    p.bar.Width = w * (v / 100)
    p.bar.MarkNeedsLayout() // 填充宽变了
    p.bar.MarkNeedsPaint()
    p.Root.MarkNeedsPaint()
    // 禁止：p.rebuild() 重建 Root
}

// 好：结构/尺寸变更才 rebuild
func (b *Button) SetSize(s ControlSize) {
    b.Size = s
    b.rebuild() // 子节点几何阶梯变了
}

// 好：动画 Tick 只 paint
func (s *Spin) Tick(dt float64) bool {
    if !s.Spinning {
        return false // 自动从 Tree 摘掉
    }
    s.angle += dt * 1.2
    s.ring.MarkNeedsPaint()
    return true
}
```

#### rebuild vs patch（何时拆树）

| 场景 | 策略 | 抄作业 |
|------|------|--------|
| 进度 / Slider 值 / Switch 拇指位置 / loading 角 | **patch** 字段 + dirty | `progress.go` `SetPercent` · `switch` thumb |
| Size / 有无 icon / 有无 spinner 子节点 / 换列 | **rebuild** | `button` `SetLoading`（子列表变） |
| Theme Token 变 | **rebuild**（hook） | `SetThemeHook(func…{ c.rebuild() })` |
| 表数据局部更新 | 尽量改行节点或 `MarkNeedsLayout` body，避免整表清子 | `table` `SetData` |
| 虚拟列表窗口滑动 | 只重建 **可见窗口** 子节点 | `primitive.VirtualList` |

**硬规则：** 公开 setter 若只改视觉值，**根节点指针必须稳定**（Headless 可测：`SetX` 前后 `Node()` 同一指针，见 `TestProgress`）。

#### RepaintBoundary 策略（隔离 paint 脏）

```text
Boundary 的代价 = 每层离屏 RT + 合成 + 每帧 markLive
收益 = 脏停在边界内，祖先 / Scroll 内容层不必整层重栅格
```

| 控件类型 | 是否 Boundary | 原因 |
|----------|---------------|------|
| `ScrollViewport` | **是**（primitive 默认） | 滚动高频 paint；内容偏移不改 layout |
| `Spin` / `Skeleton` | **是** | 持续 Tick 自驱动；脏停在本层 |
| `Progress` / 多数静态 chrome | **否** | 更新低频；嵌 Scroll 时 Boundary 易「打洞」丢绘 |
| `Pressable` / Button | **默认否** | gallery/Tabs 成十上百实例，默认 Boundary 成本过高 |
| Modal / Drawer 等 portal | 走 **Overlay 带**，不靠 main 内 Boundary 盖层 | 见框架图 §4.1 双带合成 |

```go
// Spin / Skeleton 模板
type spinHost struct {
    primitive.RepaintBoundary // 或 rebuild 末尾 h.SetRepaintBoundary(true)
    spin *Spin
}
// Progress 明确不要：
// p.Root.SetRepaintBoundary(true)  // 禁止（见 progress.go 注释）
```

#### Ticker 生命周期

```text
挂载且需要动画 → life.attach / AddTicker
Tick：改状态 + MarkNeedsPaint；不需要则 return false
停动画 / Unmount → RemoveTicker 或 still=false
禁止：用 Continuous / 定时器绕过 Tree 帧调度
```

| 检查项 | 做法 |
|--------|------|
| 仅 loading 时转 | `SetLoading(true)` 才 `AddTicker`；false 时 `RemoveTicker` |
| 卸载不再 Tick | `OnUnmount` / `stillMounted` 守卫（Spin/Skeleton `tickerLifecycle`） |
| Tick 成本 | **禁止** Tick 内 `rebuild()`；只动角度/相位字段 |

#### 列表 · 文本 · Paint 热路径

| 主题 | 正向做法 | 禁止 |
|------|----------|------|
| 长列表 | `VirtualList` **固定行高** + overscan | 动态行高假装虚拟；一次 mount 全部行 |
| 长文 | `Text.Ellipsis` + `MaxWidth`/`MaxLines` | 无界文本每帧全量 layout |
| 量字 | rebuild 时缓存 face/advance（Switch `labelPaintCache`） | Paint 路径反复 `Glyphs` |
| Paint | 只读已 layout 几何 + 画 | Paint 里 `ClearChildren` / 建新节点 / 同步 IO |
| 字符串 | 脏时再 `fmt`；热路径可复用 buffer | 每帧 `Sprintf` 仅用于无变化展示 |
| 滚动 | 交给 `ScrollViewport`；嵌套到边穿透已内建 | 手写第二套 clip/offset 与 Hit 不一致 |

#### 按需帧（与宿主契约）

```text
kit Continuous = false（永远）
有 MarkNeeds* / AddTicker → 宿主才 Present
无 dirty 且无 ticker → 不 Present（省 GPU/CPU）
```

example / 业务：`exboot.RunUIDemand` / `OwnedPresenter`，**禁止**为「动画好写」打开 Continuous。

#### PR 自检（渲染性能）

- [ ] 值类 setter：**不**整树 rebuild；根 `Node()` 指针稳定（有测）  
- [ ] 相等值 early-return，不无故 `MarkNeeds*`  
- [ ] chrome 状态用 `OnStateChange`，无「必须每帧 Sync」  
- [ ] 动画仅 Ticker；停/卸干净；Tick 内只 paint  
- [ ] Boundary 有书面理由（持续自驱动 **或** 高频局部）；非默认滥开  
- [ ] 嵌 Scroll 的低频控件 **未** 误开 Boundary（对照 Progress）  
- [ ] 长列表用 `VirtualList` 或明确 Notes「非虚拟 / 数据量上限」  
- [ ] Paint/Tick 路径无新建子树、无同步读盘、无每帧重测字（除非文案刚变）  
- [ ] 未引入 Continuous / 第二套帧循环 / 手写 Compositor present  

#### 已落地范例（抄作业）

| 模式 | 源码 |
|------|------|
| 值 patch + 根稳定 | `ui/kit/progress.go` `SetPercent` · `TestProgress` |
| 状态 chrome 自动 | `button.go` `OnStateChange` → `SyncState` |
| 拇指/标签 paint-only | `switch.go` thumb / label `MarkNeedsPaint` + 量字缓存 |
| 持续动画 + Boundary + lifecycle | `spin.go` · `skeleton.go` |
| 明确 **不要** Boundary | `progress.go` 注释 + `TestProgress_NotRepaintBoundary` |
| 虚拟窗口 | `ui/primitive/virtual_list.go` |
| Scroll 隔离 | `ui/primitive/scroll.go`（默认 Boundary + ContentPaintOffset） |

---

## 3. 标准宿主用法（example / 应用）

```go
// 真窗（Linux GPU 主路径）
res := exboot.RunUIDemand(exboot.UIDemandConfig{
    Host: host, Tree: tree, SC: sc, DC: dc, Device: dev,
    Theme: kit.DefaultTheme(),
    Continuous: false, // kit 必须 false
})
// RunUIDemand 内部 = app.New + OwnedPresenter（双带）+ 按需帧
```

```go
// Headless / 单测
host := platform.NewHeadless(800, 600)
tree := core.NewTree(root)
tree.SetTheme(kit.DefaultTheme()) // 可选
a := app.New(app.Options{DisableRenderThread: true})
a.Attach(host, tree, nil) // 自动桥接 Clipboard / Cursor / IMEPosition
a.Pulse()
```

```go
// 换肤
cp := kit.NewConfigProvider(darkTheme, page.Node())
tree.SetRoot(cp.Node())
// 或
tree.SetTheme(darkTheme) // 广播 themeHook → Button/Input/Select 等 rebuild
```

**禁止：** 在 example 里 `layer.NewCompositor` 手写 present（`present_path_test` 会拦 UI smoke）。

---

## 4. API 速查（kit 常用底层）

### 4.1 浮层

| API | 作用 |
|-----|------|
| `primitive.NewAnchoredPopup(content)` | 默认 `DismissOnOutside=true` |
| `popup.UpdateAnchorFromNode(trigger)` | 设 AnchorNode + 矩形 |
| `popup.SetOpen(bool)` | 开关；Layout 后自动几何 |
| `popup.OnDismiss` | 点外关闭后同步产品 `Open` |
| `popup.RefreshOpenGeometry` | Tree.Layout 自动调用 |
| `core.Tree.RegisterOutsideDismiss` | 一般不必手调（Anchored 已管） |

`Select`/`Dropdown`/`Popover` 的 **`Sync()` 已 Deprecated** — 仅异常布局后强制重算时使用。

### 4.2 编辑

| API | 作用 |
|-----|------|
| 拖选 | 默认 capture 拖动 |
| `ev.Shift` 点选 | 扩展选区 |
| `tree.SetClipboard` / Attach 桥接 | Ctrl/Cmd+C/X/V/A |
| `CaretLocalPos` | IME 候选位置（有 CapIME 时） |

### 4.3 滚动

| API | 作用 |
|-----|------|
| 默认 | 内层滚不动时 **穿透** 外层 |
| `TrapWheel=true` | 锁在内层（浮层列表可选） |

### 4.4 Theme

| API | 作用 |
|-----|------|
| `themeOf(field, node)` | kit 统一解析 |
| `ConfigProvider` | 子树 ambient（**真节点**，`Node()` 返回自身） |
| `Tree.SetTheme` | 全局 + 广播 hooks |
| `Node.SetThemeHook` | 控件 rebuild 注册 |

### 4.5 修饰键

| 来源 | 行为 |
|------|------|
| Linux 真窗 | X `state` → Event 修饰键 |
| Headless | `InjectPointerMods` / `InjectKeyMods` |
| 控件 | 只读 `ev.Shift` / `ev.Ctrl` / `ev.Meta` |

---

## 5. 覆盖率与 Notes

- 权威：`ui/kit/coverage.go` → `AntCoverage()`  
- `CovReady` = **有 API + Headless 测过**，≠ Ant 网页 100% props  
- 残差写 **Notes**（Menu nested、Date range、Upload dialog、Image GPU、QR codec、Table 固定头…）  
- 改表：只改 `coverage.go`，再改 `UI_KIT_COVERAGE.md` 快照  

```bash
go test ./ui/kit -run TestAntCoverageTable -v
```

---

## 6. 回归命令（改 kit / 底层后）

```bash
go test ./ui/core ./ui/primitive ./ui/layer ./ui/app ./ui/platform ./ui/kit -count=1
# 真窗观感（可选）
go run ./examples/ui_polish_gallery
```

---

## 7. 文件布局（kit 包内）

见 `ui/kit/doc.go`：一控件一文件；`ant_chrome.go` Token 色；`theme_resolve.go` 的 `themeOf`；分类 `*_common.go`。

---

## 8. 变更时同步

| 改动 | 更新 |
|------|------|
| 新/改控件 chrome | 本文 **§0.1** + §2.0 自检；Default 对齐 Ant，API 可覆盖 |
| 新/改控件渲染路径 | 本文 **§2.9** 自检（脏粒度 · rebuild/patch · Boundary · Ticker） |
| 新底层契约 | 本文 §1–§4 + `UI_FOUNDATION_P0` |
| 新/改覆盖率 | `coverage.go` + `UI_KIT_COVERAGE.md` |
| Present / 壳 | `UI_APP_SHELL_PLAN` |
| 布局铁律 | `LAYOUT_FOUNDATION` |
| 引擎 CPU/GPU 正向刀 | `PERF_ENGINE_FORWARD`（**不含** kit 控件层） |

---

**维护：** kit 开发默认读本文；底层实现细节与历史交付见 `UI_FOUNDATION_P0.md`；写控件时渲染正向优化以 **§2.9** 为准。
