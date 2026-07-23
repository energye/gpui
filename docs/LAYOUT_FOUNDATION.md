# UI 布局底盘：Flutter 约束模型（强制）

> 版本：2.2 | 2026-07-22  
> 收敛：`layoutPaddedChild` · `Flexible` 仅 tight 轴 · `BindTicker` · 去掉默认 CenterContent 噪音  
> **先稳底盘，上层只消费规则，禁止在控件里打补丁抵消引擎错误。**  
> 任务化交付与 kit 纪律：[`UI_FOUNDATION_P0.md`](./UI_FOUNDATION_P0.md) · [`UI_KIT_DEV_GUIDE.md`](./UI_KIT_DEV_GUIDE.md)

## 原则

| 层 | 对齐 |
|----|------|
| 布局 / 命中 / 逻辑绘制坐标 | **Flutter**（同一逻辑像素） |
| 控件外观与交互 token | **Ant Design 5** |
| 光栅 AA | Skia / 浏览器 |

## 不可破的契约

1. **Constraints 单向传递**  
   父给 `min/max`，子返回 `size`，子 `offset` 由父写入。

2. **hit == layout == paint(logical)**  
   `AbsoluteBounds` / `HitTest` / `PaintContext.Origin` 必须一致。  
   UI demand 循环 **paint scale 固定为 1**（见 `exboot`）。  
   每帧 `BeginFrame` 重置 CTM（防止 InvertY 泄漏）。

3. **`Flex` 默认 `CrossStart`**（Row 不再默认竖直居中）  
   **`Flexible` = Expanded + Align.topLeft（默认）**  
   - 自己占满 flex 分配空间  
   - 子在 `(0,0)`，**不**被强制铺满、**不**被居中  
   - 需要铺满时显式 `FillChild=true`（Input 编辑器 / tab body）

4. **`Decorated`**  
   - 定自己的 `Width/Height`  
   - 对子默认只 **cap Max**（Switch 拇指保持 18×18）  
   - `StretchChild=true` 才 tight 铺满（Tab 命中条 / tab body）  
   - **`CenterContent` 默认 false（贴顶左）**；仅 Button/Input/Select chrome `SetCenterContent(true)`

5. **`Slot`**  
   - 空 → 尺寸 0（禁止在松约束下吞掉 MaxWidth）  
   - `ExpandFill` 仅用于需要铺满的宿主（tab body）

6. **脏标记**  
   `MarkNeedsLayout` 必须冒泡；`Flexible`/`Flex` 必须 `LayoutSkipIfClean`，子脏则重排。


## hit == paint 坐标契约（core）

```
Paint:  childPC.Origin = parentOrigin + child.Offset()   // DefaultPaintChildren
Hit:    local          = parentPoint  - child.Offset()   // DefaultHitTest
绝对位置 AbsoluteOffset = Σ Offset（祖先链）
```

因此 **Offset 是唯一真相**：Layout 写 Offset/Size 后，Hit 与 Paint 必须共用，禁止：

1. Layout 用松 `MaxHeight` 算 `offY` 居中，但 `Size.Height` 仍是 content（旧 Pressable bug）
2. Paint 里再用魔法 Y 偏移，而不改 Offset
3. 自定义 HitTest 忽略 `child.Offset`（应走 `DefaultHitTest` 或显式 `p.Sub(offset)`）

回归：`core.AuditHitPaintContract` + `TestAuditHitPaint_TabsNestedControls`。

## 禁止

- **`Flexible`/`Spacer` 只吃 tight 轴**（flex 主轴分配 / CrossStretch）；禁止用松 `MaxHeight` 撑高（Modal footer `Row(Spacer,btn)` 会把按钮 CrossCenter 到中下）

- 在控件里写「魔法 offset」修正点击  
- 默认 2× 超采样混进 layout 坐标  
- `Decorated` 默认 `Min==Max` 撑满子节点  
- `Decorated` / `Flex` 默认竖直居中
- **`Pressable` 用松 `MaxHeight` 居中子节点**（Tabs body 传入大 MaxHeight → 命中在上、绘制在中下）

## 回归

```bash
go test ./ui/primitive/ ./ui/kit/ ./ui/core/ -count=1
go run ./examples/ui_polish_gallery
```

预期：各 Tab 内容**贴右上顶部**；鼠标指哪点哪；Input 占位与光标在框内垂直居中。
