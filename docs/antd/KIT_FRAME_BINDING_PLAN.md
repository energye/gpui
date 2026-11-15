# 组件框架绑定支撑计划（KIT_FRAME_BINDING）

> 地位：F0 地基的收尾批。补齐组件基座对 Flutter 标准缺的四件框架能力，之后控件层（F1–F6 74 项）才是纯组合活。
> 日期：2026-11-16。依据：源码逐层核对（gpui 本仓）+ Flutter 标准锚点（`/home/yanghy/app/projects/gogpu/flutter/packages/flutter/lib/src/`）。
> 性质：先支撑后控件（ARCHITECTURE.md §5 波次序不变，本批是 F0→F1/F2 之间的前置收口）。现有已签字组件不迁移、不参照，新契约以本文为准。

## 0. 核对结论（为什么是这四件）

基座两半边核对结果：

- **渲染半边：符合标准，不动。** 约束传播（`rendering/constraints.go` 四值 BoxConstraints）、布局协议（`RenderBox.Layout` + relayout boundary + constraints 早期退出）、重绘协议（`MarkNeedsPaint` + RepaintBoundary 三规则 paint walk）、帧管线（PipelineOwner + layer_build 分层重建）——与 Flutter RenderObject 层逐点同构。
- **交互与重建半边：缺四件。** 全部不在渲染层（渲染层没缺东西），分布在 kit 层与 focus/embedder 缝上。

| # | 缺件 | Flutter 标准锚点 | gpui 现状（源码证据） | 归属 |
|---|---|---|---|---|
| 1 | Element 绑定层 | `framework.dart:3570` `Element` 持 renderObject，setState→rebuild→diff→自动标脏 | kit 全部 Instance 不持渲染节点；`SetState`（qrcode_state.go:135）只改字段不触发重画；唯一手工焊点是 anim_image（Instance 持 node + 帧到点 `AdvanceFrame`+标脏） | kit 层 |
| 2 | InheritedWidget 依赖登记 | `framework.dart:1718` 系 `dependOnInheritedWidgetOfExactType`：订阅登记 + 变更只刷依赖者 | `scope.ChangedAspects(before, after)` 纯函数 diff 已有，但 Provider/Use 未实现、无订阅登记、全仓零调用 | kit 层 |
| 3 | 手势竞技场自动接线 | `gestures/binding.dart:477-508` `dispatchEvent` 沿命中路径逐 target `handleEvent`，recognizer 在其中自动 `arena.add` | routePointer（input_router.go:561-580）沿命中路径分发**已有且对齐**；但 router 全文 0 处 arena——竞技场本体（gestures/arena.go）完整却无人驱动；kit `Interactive.JoinArena`（interactive.go:146）与 `Scrollable.JoinPointer`（scrollable.go:213）全靠窗侧手动喂；F0 interact 窗退化为窗侧手写矩形命中（main.go:259-278 `BehaviorContains`） | kit + embedder 缝 |
| 4 | FocusScope 树 | `widgets/focus_scope.dart:106+` FocusNode 是树节点（scope 层级 + traversal 策略 + 弹窗圈地） | `focus/node.go` FocusNode 是平面结构体（无 parent/children/scope 字段）；FocusManager 是平表（Register/Unregister/RequestFocus）；无 traversal 策略实现 | ui/focus |

## 1. 修复件定义与验收标准

### 件 1：Element 绑定层（kit，最先做）

**契约**（对齐 Element 三职责：持绑定 / 管生命周期 / 转标脏）：

```go
// ui/kit/internal/scope（或 kit 包级，动工时定稿）
// WidgetNode = Element 的 Go 版：一个组件实例与它的渲染根节点的活绑定。
type WidgetNode struct {
    Inst   Instance            // 组件实例（实现 mountable 接口）
    Root   rendering.RenderObject // Build 产出的组件根节点
    Aspects []scope.Aspect     // 订阅的 Ctx aspects（件 2 消费）
}

// Instance 统一最小接口（五段里的 State 段实现它）：
type Instance interface {
    Mount(ctx ScopeCtx, root rendering.RenderObject)   // 挂树：宿主注入根节点引用
    Update(ctx ScopeCtx, next Props)                   // props/ctx 变：diff+标脏
    Unmount()
    Node() rendering.RenderObject                      // Build 产出
}
```

**联动规则**：
- `SetState(f)` 收口：改完状态位后，若状态影响几何（layout 类）→ `root.MarkNeedsLayout()`；仅外观 → `root.MarkNeedsPaint()`。Instance 声明自己属于哪类（`NeedsLayoutOnChange bool` 或按状态位表）。
- Render 段产出的节点树由 Build 一次组装，Instance 持引用；Update 只改节点属性（SetFixedWidth/SetBackground 等已有 setter），**不重建节点树**（Flutter Element 复用 RenderObject 同语义）。

**验收**：单测——SetState 后 `Root.NeedsPaint()==true`（或 NeedsLayout）；Update 改 props 后节点属性变且无新节点分配；负向——Instance 未 Mount 时 SetState 不 panic（挂树前静默）。钉子窗：F2 Button 标杆（见 §2）。

### 件 2：InheritedWidget 依赖登记（kit，可与件 1 并行）

**契约**（对齐 InheritedModel.aspectOf 语义）：

```go
// ui/kit/internal/scope
// Provider 注册组件的 aspect 订阅；Use 读值并登记依赖。
// 宿主持注册表；SetCtx 时按 ChangedAspects 通知订阅者 Update。
type Registry struct { /* aspect → []订阅者 */ }
func (r *Registry) Subscribe(node *WidgetNode, aspects []scope.Aspect)
func (r *Registry) Unsubscribe(node *WidgetNode)
func (r *Registry) SetCtx(next ScopeCtx)  // 内部走 ChangedAspects，只通知订阅了变更 aspect 的 node
```

**验收**：单测——订阅 theme 的 node 在 SetCtx(仅 locale 变) 不被通知；SetCtx(theme 变) 被通知且 Update 收到新 Ctx；Unsubscribe 后不再通知；嵌套 Provider（子作用域覆盖）行为钉死。负向——摘掉 aspect 过滤（全通知）测试红。

### 件 3：手势竞技场自动接线（kit 契约 + embedder 一处收口）

**契约**（对齐 GestureBinding.dispatchEvent → handleEvent → arena.add 链）：

- 节点侧：声明参与手势的节点实现一个可选接口（如 `GestureMember interface { ArenaMember() gestures.GestureRecognizer }`）；挂树时不动作（对齐 Flutter：recognizer 在事件期 join arena，不在挂树期）。
- 路由侧：`routePointer` 已有命中路径向上遍历（input_router.go:575-580）——在遍历同一循环里，对 PointerDown 事件把路径上的 GestureMember 自动 `arena.add`（down 开场 → move 竞争 → up close/sweep 裁决，GestureArenaManager 语义逐字对齐 arena.dart:71-157 add/close/sweep）。
- 竞技场管理器统一持有：InputRouter 持一个 `GestureArenaManager`（现在 Scrollable 自持一份、Interactive 一份的碎片化收掉）。
- **窗侧手写命中判断作废**：F0 interact 窗的 `BehaviorContains` 矩形命中是竞技场断线时代的补丁，件 3 完成后改走标准链（该窗回归改为验证行为不变，不是保留补丁）。

**验收**：单测——同路径两个 recognizer（tap + scroll）同 down，arena 裁决只一方赢（对齐 Flutter 竞技场规则）；Interactive 挂树后零手动接线即可收 down/up → pressed/tap 状态机走通；负向——摘掉自动 add 测试红。真窗回归：f0_interact 15 探针全绿且窗侧 OnPointer 手写命中删除。

### 件 4：FocusScope 树（ui/focus 补）

**契约**（对齐 focus_scope.dart 树语义）：

```go
// FocusScopeNode：scope 层级（Modal 圈地 = 子 scope 挡住外部 traversal）。
type FocusScope struct {
    Parent   *FocusScope
    Children []*FocusScope
    Nodes    []*FocusNode  // 本 scope 直接持有
    Trap     bool          // 圈地：traversal 不越界（Modal/Drawer）
}
// FocusManager 增：当前 scope 栈、traversal 策略口（顺序遍历 = 默认策略）。
```

**验收**：单测——嵌套 scope 内 Tab 不越界（Trap）、出 scope 回到父级链、scope 销毁焦点回落父 scope 焦点；负向——平表模式（现行为）回归不破（存量 ime/textinput 窗全绿）。

## 2. 实施顺序与关门

```
件1 Element绑定 ─┬─→ 件3 竞技场接线 ──→ F2 Button 标杆（绑定契约的验收窗）
件2 Inherited ───┘                        （标杆签字五样 + 本文件1/2/3 验收全过）
件4 FocusScope ──→ 独立线，F4 浮层波前必须绿
```

- **顺序依据**：件 3 的自动 join 需要件 1 的节点契约（recognizer 挂在组件根节点上）；件 2 与件 1 同层无依赖可并行；件 4 完全独立（ui/focus 包内），但 F4 类（Modal 圈地）依赖它，排在 F4 前收口即可。
- **关门标准**：每件单测逐文件绿 + 对应负向红还原 + 存量窗回归（f0 六窗 + ime 族）零回归；件 1/2/3 联合验收窗 = F2 Button 标杆（不是独立新窗——标杆窗就是这三件的真窗证据）。
- **文档同步**：件 1/2 落地后改 `WIDGET_MODEL.md` §3/§5（State 段挂树规则 + Ctx Provider 语义）与 `F0_GATE_CHECK.md`（接线四查增「必须走 WidgetNode 绑定」）；本文随实施逐件回填状态。

## 3. 状态台账

| 件 | 状态 | 单测 | 验收窗 | 日期 |
|---|---|---|---|---|
| 1 Element 绑定 | ✅ 落地（`ui/kit/internal/scope/bind.go`：`Instance` 框架面四方法 + `WidgetNode`（Attach/Mount/MarkStateDirty/Unmount）+ `DirtyTable.Classify/Diff`；Update 因 Go 无协变留在各组件具体类型） | `scope/bind_test.go` 9 用例：SetState→Root.NeedsPaint/Layout、DirtyNone 不脏、未挂树/卸载后静默、挂载幂等、Diff 异或语义、nil 根焊点 panic、组合位最强类 | F2 Button 标杆 | 2026-11-16 |
| 2 Inherited 登记 | ✅ 落地（`ui/kit/internal/scope/registry.go`：`Registry.Subscribe/Unsubscribe/SetCtx`，内部走 `ChangedAspects` 只刷依赖者，通知按订阅序；`NotifyCtx` 转发 UpdateCtx + MarkNeedsPaint） | `scope/registry_test.go` 7 用例：theme 订阅者对 locale-only 变更零通知、Unsubscribe 后不通知、无变更静默、多 aspect、resubscribe 换表、双订阅者各通知一次、nil 安全；负向验证=摘掉 aspect 过滤三测红→还原绿 | F2 Button 标杆 | 2026-11-16 |
| 3 竞技场接线 | ✅ 落地（路由侧 `ui/embedder/input_router.go`：InputRouter 持统一 `GestureArenaManager`，routePointer 命中路径循环内对 PointerDown 自动 `JoinPointer`（deepest-first）→ Route → CloseArena，逐字对齐 Dispatcher.HandleDown / GestureBinding.dispatchEvent；附 MouseTracker 语义的悬停退出合成——move 时对脱离命中路径的 handler 合成 PointerLeave。节点侧 `ui/kit/internal/behavior/interactive_region.go`：`InteractiveRegion`（Listener 的 Go 版，`*RenderBox` 内嵌包裹 + Init 先于 AddChild 的 promotion 焊法 + HitTest 自命中解析回 region）组合 Interactive 机器，JoinPointer→tap 入竞技场、OnPointer 自驱 hover/pressed。窗侧手写命中删除：f0_interact 三机器改挂 Region、Input 走 PipelineOptions.Input、OnEvent 指针/手动喂键整段删、hit_match 改断言真树 HitTest 命中 region） | `interactive_region_test.go` 5 用例（自动竞技场 tap/出 slop 拒/禁用吞/hover 进出含退出合成/slop 拒）+ 负向验证（摘自动 join→tap 红→还原绿） | f0_interact 真窗 15/15 探针全绿 + Golden 重播种两跑逐位 0.0000%（2026-11-16）+ 嵌入器/手势/kit/rendering 回归绿 | 2026-11-16 |
| 4 FocusScope 树 | ✅ 落地（`ui/focus/scope.go`：`FocusScope` 树节点（Parent/Children/Nodes/Trap）+ `FocusNode.scope` 字段（nil=平表旧路径零改动）+ Manager traversal scope 约束——Trap 内 Tab 循环不越界、非 Trap 出尾上浮父链、嵌套取最近 trap、Detach 销毁焦点回落（blur 兜底）+ Add 重挂自动搬家） | `focus/scope_test.go` 7 用例（Trap 圈地/非 Trap 上浮/嵌套内圈/销毁回落+注销/重挂搬家/平表逐字节回归/scoped+flat 混居）+ 负向验证（摘 trap 约束→圈地测试红→还原绿）；focus 全量 + embedder/kit 回归绿（ime/textinput 存量路径不受影响——无 scope 节点走原路径） | F4 浮层波前收口（Modal/Drawer 圈地消费方接线待 F4） | 2026-11-16 |
