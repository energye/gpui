# UI Kit 开发指南 — 底层契约（必读）

> 版本：1.1 | 日期：2026-07-23  
> 状态：**活文档** · 与源码冲突时以源码 + `go test` 为准  
> 底层交付总账：[`UI_FOUNDATION_P0.md`](./UI_FOUNDATION_P0.md) v2.1+  
> **Ant 对齐目标/验收：** [`UI_KIT_ANT_V5_SPEC.md`](./UI_KIT_ANT_V5_SPEC.md)（L1 行为 · L2 Token · L3 本库 golden · L4 人眼）  
> 架构总图：[`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) · 布局：[`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md)  
> 覆盖率表：[`ui/kit/coverage.go`](../ui/kit/coverage.go) · [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)

---

## 0. 一句话

```text
kit = 产品 Props + 状态机 + a11y 名 + 组合 primitive
禁止：第二套 Hit / 帧循环 / 每帧 Sync / 魔法 offset / 硬编码色当默认皮
对齐 Ant：见 UI_KIT_ANT_V5_SPEC（行为 + Token + 本库 golden；非浏览器像素哈希）
```

依赖方向（硬）：

```text
业务 → ui/app → ui/kit → ui/primitive → ui/core → render
              ↘ ui/platform（仅 Host 注入）
真窗 Present：ui/app.OwnedPresenter（或 exboot.RunUIDemand 薄包装）
```

---

## 1. 底层已交付能力（kit 可直接依赖）

| 能力 | 用法摘要 | 源码 |
|------|----------|------|
| 布局契约 | hit == layout == paint；Flexible 只 tight 轴；Decorated 默认不居中 | `LAYOUT_FOUNDATION` · `primitive/*` |
| 脏 / 按需帧 | `MarkNeedsLayout` / `MarkNeedsPaint` / `AddTicker`；禁止 Continuous 动画控件 | `core` · `app` |
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

### 2.7 测试

- [ ] Headless：布局稳定、状态回调、Hit、（若有）打开/关闭浮层  
- [ ] 不引入第二套 present 循环  
- [ ] 覆盖率变更只改 `coverage.go`，再同步 `UI_KIT_COVERAGE.md` 数字/Notes  

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
| 新底层契约 | 本文 §1–§4 + `UI_FOUNDATION_P0` |
| 新/改覆盖率 | `coverage.go` + `UI_KIT_COVERAGE.md` |
| Present / 壳 | `UI_APP_SHELL_PLAN` |
| 布局铁律 | `LAYOUT_FOUNDATION` |

---

**维护：** kit 开发默认读本文；底层实现细节与历史交付见 `UI_FOUNDATION_P0.md`。
