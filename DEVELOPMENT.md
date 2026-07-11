# GPUI 开发计划

> 本文档记录开发阶段规划，随开发进展更新。
> 最后更新: 2026-07-11

---

## 总体目标

基于 gg 渲染引擎 + LCL 窗口系统，构建跨平台 GPU 加速的 GUI 框架，对标 ant.design 级别的控件体系。

### 架构分层

```
┌─────────────────────────────────────────────────────┐
│  GUI Controls (Button, Input, Modal, Table, ...)    │  ← 目标
├─────────────────────────────────────────────────────┤
│  Effect System + State Machine + Event System       │  ← 待建
├─────────────────────────────────────────────────────┤
│  FramePump (帧调度基座)                              │  ← ✅ 已完成
├─────────────────────────────────────────────────────┤
│  gg 渲染引擎 (Context, Scene, Text, Filters)        │  ← ✅ 已有
├─────────────────────────────────────────────────────┤
│  LCL + OpenGL (窗口系统 + GL 上下文)                 │  ← ✅ 已有
└─────────────────────────────────────────────────────┘
```

### 设计原则

- **机制与策略分离**: FramePump 只提供帧调度机制，不管上面的 Effect/Widget 怎么实现
- **gogpu 模式移植**: Invalidator(chan 合并) + AnimationController(引用计数) + RunOnMainThreadSync(主线程投递)
- **onPaint 约束**: onPaint 永远不直接调用 Invalidate，所有重绘通过 FramePump goroutine 中转
- **纯 Go 无 CGo**: 通过 purego 调用原生 API

---

## 阶段规划

### Phase 1: FramePump 基座 ⬅️ 当前阶段

**状态**: ✅ 已完成

**目标**: 实现 goroutine 驱动的帧调度器，替代 atomic CAS 方案。

**子项**:

| # | 任务 | 状态 | 文件 |
|---|------|------|------|
| 1.1 | FramePump 核心 (chan 合并 + goroutine) | ✅ | `ui/frame_pump.go` |
| 1.2 | AnimationController (引用计数) | ✅ | `ui/frame_pump.go` |
| 1.3 | AnimationToken (生命周期句柄) | ✅ | `ui/frame_pump.go` |
| 1.4 | TGPUControl 集成 | ✅ | `ui/control.go` |
| 1.5 | Window 统一入口 | ✅ | `ui/window.go` |

**关键提醒**:
- FramePump goroutine 使用 `RunOnMainThreadSync` 投递 Invalidate，确保请求不丢失
- `requestCh` 是 buffered(1) channel，多次调用合并为一次
- `AnimationToken.Stop()` 是幂等的，安全多次调用
- `onPaint` 结尾通过 `pump.RequestRedraw()` 续帧，不直接调用 `Invalidate()`

---

### Phase 2: 帧时间与动画基础

**状态**: 📋 待开始

**目标**: 提供 delta time 和基础动画原语，为 Effect System 打基础。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 2.1 | Delta Time 计算 | 📋 | onPaint 中计算 dt，传递给 onUpdate 回调 |
| 2.2 | 帧率限制 (60fps cap) | 📋 | 可选的 minFrameDuration 节流 |
| 2.3 | Easing 函数库 | 📋 | ease-in, ease-out, ease-in-out, spring |
| 2.4 | Tween 引擎 | 📋 | 属性插值: float, RGBA, Vec2 |
| 2.5 | FPS 计数器 | 📋 | 诊断用，可选 |

**关键提醒**:
- 参考 gogpu 的 `onUpdate(dt float64)` 模式，dt 单位为秒
- dt 需要 clamp (参考 gogpu 的 0.066s 上限)，防止长时间 idle 后动画跳帧
- Easing 函数是 Effect System 的基础原语 (hover 渐变、focus ring 动画都依赖它)
- Tween 引擎需要与 FramePump 的帧节奏对齐

---

### Phase 3: Effect System

**状态**: 📋 待开始

**目标**: 实现可组合的视觉效果系统，支撑控件的 hover/press/focus 动画。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 3.1 | Effect 接口定义 | 📋 | `Update(dt) bool`, `Priority() int` |
| 3.2 | HoverEffect | 📋 | 颜色渐变、阴影变化 |
| 3.3 | PressEffect | 📋 | 缩放、颜色加深 |
| 3.4 | FocusEffect | 📋 | focus ring 渐变 |
| 3.5 | TransitionEffect | 📋 | 通用属性过渡 (opacity, color, position) |
| 3.6 | EffectChain | 📋 | 多个 Effect 组合、优先级排序 |

**关键提醒**:
- 每个 Effect 通过 `pump.RequestRedraw()` 请求帧，FramePump 自动合并
- Effect 需要持有控件的引用，以便修改控件状态 (hovered, pressed 等)
- `RGBA.Lerp()` 和 `SolidBrush.Lerp()` 已有，可直接用于颜色过渡
- Spring 物理效果可能需要独立的 dt 追踪 (不依赖帧率)

---

### Phase 4: Event System

**状态**: 📋 待开始

**目标**: 桥接 LCL 事件到控件树，实现事件分发和命中测试。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 4.1 | 事件类型定义 | 📋 | MouseEvent, KeyEvent, FocusEvent |
| 4.2 | LCL 事件桥接 | 📋 | 从 LCL 回调转换为内部事件 |
| 4.3 | 事件分发器 | 📋 | 控件树遍历、冒泡/捕获 |
| 4.4 | 命中测试 | 📋 | 基于 shape.Contains() 的控件树遍历 |
| 4.5 | 焦点管理 | 📋 | Tab 遍历、焦点环 |
| 4.6 | 手势识别 | 📋 | 长按、双击、拖拽 |

**关键提醒**:
- LCL 已有完整的事件回调 (OnMouseMove, OnMouseDown 等)，需要桥接
- `scene/shape.go` 已有 `Contains()` 方法 (Rect, Circle, Ellipse, RoundRect)
- 事件分发需要考虑 z-order 和裁剪区域
- 焦点管理需要与 Effect System 联动 (FocusEffect)

---

### Phase 5: State Machine

**状态**: 📋 待开始

**目标**: 为控件提供状态管理和状态转换驱动的动画。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 5.1 | ControlState 定义 | 📋 | hovered, pressed, focused, disabled, loading |
| 5.2 | 状态转换规则 | 📋 | 事件 → 状态变更 → Effect 触发 |
| 5.3 | 状态驱动渲染 | 📋 | 根据状态选择渲染路径 |
| 5.4 | 响应式状态 (可选) | 📋 | 信号/订阅模式 |

**关键提醒**:
- 状态转换是 Effect System 的触发源 (hover → HoverEffect.Start())
- 状态变更后需要 `pump.RequestRedraw()` 触发重绘
- 参考 React 的 useState / Flutter 的 StatefulWidget 模式

---

### Phase 6: Layout System

**状态**: 📋 待开始

**目标**: 实现 Flexbox 布局引擎，支持自动布局和响应式设计。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 6.1 | Box Model | 📋 | padding, margin, border |
| 6.2 | Flexbox 子集 | 📋 | flexDirection, justifyContent, alignItems |
| 6.3 | 约束求解器 | 📋 | min/max size, intrinsic size |
| 6.4 | 布局缓存 | 📋 | 脏标记、增量布局 |

**关键提醒**:
- 布局系统是纯 CPU 计算，不涉及渲染
- 参考 Yoga (Flexbox) / Taffy 的算法
- 需要与 Widget 树集成

---

### Phase 7: Theme System

**状态**: 📋 待开始

**目标**: 实现 Design Token 系统，支持主题切换和暗色模式。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 7.1 | Design Token 定义 | 📋 | colors, typography, spacing, shadows |
| 7.2 | Theme 结构体 | 📋 | Light/Dark 主题包 |
| 7.3 | 主题切换 | 📋 | 运行时切换、动画过渡 |
| 7.4 | 平台适配 | 📋 | DarkMode(), FontScale() 查询 |

**关键提醒**:
- gogpu 已有 `DarkMode()`, `FontScale()`, `HighContrast()` 等平台查询
- `render/color.go` 已有 `RGBA.Lerp()` 用于主题切换的颜色过渡
- Design Token 应该是静态的，不依赖运行时状态

---

### Phase 8: Widget System

**状态**: 📋 待开始

**目标**: 实现控件树和基础控件集。

**子项**:

| # | 任务 | 状态 | 说明 |
|---|------|------|------|
| 8.1 | Widget 接口 | 📋 | Render, Layout, HitTest, HandleEvent |
| 8.2 | Widget 树 | 📋 | 父子关系、遍历、脏传播 |
| 8.3 | Button 控件 | 📋 | 对标 ant.design Button |
| 8.4 | Input 控件 | 📋 | 对标 ant.design Input |
| 8.5 | Label 控件 | 📋 | 文本显示 |
| 8.6 | Container 控件 | 📋 | 布局容器 |
| 8.7 | 更多控件... | 📋 | Modal, Dropdown, Table, Tree 等 |

**关键提醒**:
- `widget/widget.go` 已有占位接口，需要扩展
- 控件需要组合 Phase 3-7 的所有能力
- 控件应该是可组合的 (Button 内嵌 Icon + Text + Spinner)

---

## 状态图例

| 符号 | 含义 |
|------|------|
| ✅ | 已完成 |
| 🔨 | 进行中 |
| 📋 | 待开始 |
| ⏸️ | 暂停 |
| ❌ | 已取消 |

---

## 依赖关系

```
Phase 1 (FramePump)
    ↓
Phase 2 (帧时间/动画基础)
    ↓
Phase 3 (Effect System) ← 依赖 Phase 2 的 Easing/Tween
    ↓
Phase 4 (Event System) ← 与 Phase 3 联动 (事件触发 Effect)
    ↓
Phase 5 (State Machine) ← 依赖 Phase 3 + 4
    ↓
Phase 6 (Layout System) ← 独立，但需要 Phase 5 的状态
    ↓
Phase 7 (Theme System) ← 独立
    ↓
Phase 8 (Widget System) ← 依赖所有前置 Phase
```

---

## 参考库

| 库 | 用途 | 参考点 |
|---|------|--------|
| gogpu | 平台层模式 | Invalidator, AnimationController, onUpdate(dt), EventSource |
| gg | 渲染引擎 | Context, Scene, Text, Filters, Color |
| ant.design | 控件设计 | 组件 API、交互规范、视觉标准 |
| Yoga | 布局算法 | Flexbox 约束求解 |
| Flutter | 架构参考 | Widget 树、状态管理、渲染管线 |

---

## 变更日志

| 日期 | 变更 |
|------|------|
| 2026-07-11 | 初始版本，Phase 1 已完成 |
