# L2 框架壳任务计划 — Phase 5（手势 · 焦点 · Overlay · 动画完备）

> **版本：1.0** | 日期：2026-07-27  
> **状态：待实现**  
> **前置：** P4 主门禁已通过 — [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md)  
> **真源：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) §2 L2 · F09 / F13 / F17  
> **大纲父页：** [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md) Phase 5  

---

## 0. 目标与非目标

### 目标

在 **不破坏 P0–P4 管线契约** 的前提下，提供 **控件前底座**：复杂交互不必每个控件自写一套。

| 能力 | 一句话 |
|------|--------|
| **GestureArena** | 识别器竞争；点击 / 拖 / 滚轮不互相打架 |
| **嵌套滚动竞争** | 父子 scroll 谁吃 pointer 有明确规则 |
| **Focus** | 焦点树、Tab 遍历、键盘路由骨架 |
| **Overlay 机制** | 填满 P2 预留 band：插入 / 移除 / 命中序 |
| **Animation 完备** | Curve、状态机；隐式动画默认 compositor-only（F09） |
| **Semantics 最小** | 角色 / 标签骨架（可很薄） |
| **主题/Token 传播（最小）** | 全局配置下发接口（命令式也可） |

### 非目标（本阶段不做）

| 不做 | 放到 |
|------|------|
| Ant Modal / Button / 任意 `docs/antd` 产品 API 与皮肤 | **P7** |
| 完整 a11y 生态（读屏树、平台桥、全量 ARIA） | 后置 |
| 完整 IME / 选区编辑器 | 后置 |
| per-layer GPU RT 池打磨、更细 damage、多窗 HUD | **P6** |
| 改 `ui → render → gpu` 依赖方向 | 永不 |
| 为单个控件开私有 Present 路径 | 永不 |

> **纪律：** 本卡 **排除** `docs/antd/`。P5 只交付 **机制**；P7 再按 antd 波次迁控件。

### 全局约束（继承 P0–P4）

| ID | 约束 |
|----|------|
| G1 | `ui → render → gpu`，**ui 禁止 import gpu** |
| G3 | 逻辑 px、Y-down；物理 = 逻辑 × dpr |
| G4 | 管道 depth、背压、SubmitLatest 不堵 UI |
| G5 | 能力不够 → 改 **render**（或 gpu），不在 ui 复制 GPU |
| G7 | `go test ./ui/...`；动 render 时测 `./render` |
| G8 | Overlay / 动画 **不得** 逼 main 全量矢量 re-paint；脏区模型仍 ∝ 脏层 |
| G9 | 输入路径不经 Wait Present（F08 回归） |

### 建议 baseline（开工前）

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go test ./ui/... -count=1
# 可选：存 spinner / scroll JSON 作回归基线
go run ./examples/ui_l1_spinner 2> /tmp/l1_spinner_p5base.err | tee /tmp/l1_spinner_p5base.json
go run ./examples/ui_l1_scroll  2> /tmp/l1_scroll_p5base.err  | tee /tmp/l1_scroll_p5base.json
```

P5 合入后同场景不得显著回退（layout 空转、`raster_layer_count` 暴涨、IDLE 失效）。

---

## 1. 交付物总览

```text
ui/gestures/                 // 新建：竞技场 + 识别器
  arena.go                   // GestureArena / GestureArenaManager
  recognizer.go              // GestureRecognizer 接口
  tap.go                     // TapGestureRecognizer
  pan.go                     // Pan/DragGestureRecognizer（可接 Scrollable）
  scroll.go                  // 滚轮识别（或并入 pan）
  arena_test.go
  matrix_test.go             // 点击 vs 拖 不冲突矩阵

ui/focus/                    // 新建：焦点树
  node.go                    // FocusNode / Focusable
  manager.go                 // FocusManager：request/blur/Tab
  traversal.go               // 顺序遍历（深度优先 / 显式 order）
  focus_test.go

ui/overlay/                  // 新建：Overlay 机制（非 Ant Modal）
  entry.go                   // OverlayEntry：insert/remove
  state.go                   // OverlayState：栈序、命中
  overlay_test.go

ui/semantics/                // 新建：最小语义（可很薄）
  node.go                    // role / label 骨架
  semantics_test.go

ui/theme/                    // 新建（或 ui/style）：Token 下发最小
  tokens.go                  // 颜色/字号/间距 map 或 struct
  provider.go                // 全局/子树配置读取接口
  theme_test.go

ui/animation/                // 加深（P3 Controller 已有）
  curve.go                   // Linear / EaseInOut / Cubic 等
  status.go                  // dismissed/forward/reverse/completed
  implicit.go                // 隐式 opacity/offset → compositor-only
  controller.go              // 接 Curve + Status（兼容现 API）

ui/rendering/
  scrollable.go              // 迁到 Gesture 识别器驱动（保留薄适配）
  // 可选：Listener / GestureDetector 式 RO 胶水

ui/scene/
  packet.go / build.go       // Overlay band 真填；命中与 paint 序
  compositing.go             // 保持 F13 语义

ui/embedder/                 // 事件分发：指针 → Arena；键 → Focus
  // 或 ui/input 路由包（实现时定名）

examples/
  ui_l2_shell/               // 手势矩阵 + 焦点 Tab + Overlay 浮层烟囱
```

**依赖方向（新增）：**

```text
ui/gestures|focus|overlay|semantics|theme  →  可依赖 platform / rendering / scene / animation
                                           ↛  gpu
L3 kit（P7）→ L2 包 → L1 → render → gpu
```

---

# 分轨任务（建议顺序：A → B → C → D → E → F）

原则：**先手势与输入路由（A/B），再焦点与浮层（C/D），最后动画完备与薄壳（E），示例与回归（F）。**  
每轨可单独 PR；**A 必须先合**（后续轨依赖事件进 Arena）。

---

## 轨 A — GestureArena（对齐 Flutter gestures 思想）

**目标：** 同一 pointer 上多个识别器竞争；**点击与拖不冲突** 有可测矩阵；Scrollable 可迁入 arena，不再手写 ad-hoc。

### A.1 核心模型

```text
PointerDown
  → HitTest 得 path（深→浅或浅→深，文档钉死一种）
  → 路径上各 GestureRecognizer.addPointer
  → GestureArena 开局（每 pointer 一局）

PointerMove / Up / Cancel
  → 所有参赛识别器收事件
  → accept / reject
  → 胜者独占后续（或 team 决议，MVP 单胜者即可）

同帧多 Move
  → 合并为最新点（继承 P4 S6）
```

| 类型 | 职责 |
|------|------|
| `GestureArena` | 一局竞争：成员、resolve、sweep |
| `GestureArenaManager` | 按 `pointerID`（MVP 可单指 = 1）管理多局 |
| `GestureRecognizer` | `AddPointer` / `HandleEvent` / `Accept` / `Reject` / `Dispose` |
| `TapGestureRecognizer` | 位移阈值内 Up → onTap；超阈 reject |
| `PanGestureRecognizer` | 超 thr → onPanStart/Update/End；与 Tap 竞争 |
| `Eager / Lazy` | MVP：拖超 thr 时 pan accept、tap reject |

** thr 建议（逻辑 px，可配）：**

| 常量 | 建议默认 | 用途 |
|------|----------|------|
| `kTouchSlop` | 8 | 区分 tap / pan |
| `kTapTimeout` | 300ms | 可选；超时仍可只靠 Up |

### A.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| A1 | `GestureRecognizer` 接口 + `GestureArena` 单局 resolve | 单测：两成员一 accept 一 reject |
| A2 | `GestureArenaManager`：Down 开局、Up/Cancel 收局 | 单测生命周期无泄漏成员 |
| A3 | `TapGestureRecognizer`：slop 内 tap 成功 | 单测 |
| A4 | `PanGestureRecognizer`：超 thr 胜出；onUpdate 最新点 | 单测 + 合并 Move |
| A5 | **冲突矩阵**：同 path 上 Tap+Pan — 小动= tap，大动= pan 且无双触发 | `matrix_test` 硬断言 |
| A6 | 滚轮：`PointerScroll` 直达 scroll 识别器或 Viewport（可不进竞技） | 单测 offset |
| A7 | 胶水：HitTest path → 自动 `AddPointer`（测试 helper 或 embedder） | 单测 path 顺序 |
| A8 | `Scrollable` 改为持有/委托 Pan（或并存适配期，标 Deprecated） | S6 回归仍绿 |

### A.3 单测门禁（手势矩阵）

| 测试名（建议） | 断言 |
|----------------|------|
| `TestArena_TapWinsWithinSlop` | 位移 &lt; thr → onTap=1，onPanStart=0 |
| `TestArena_PanWinsBeyondSlop` | 位移 ≥ thr → onPanStart=1，onTap=0 |
| `TestArena_RejectDoesNotFire` | reject 后 Up 不触发 tap |
| `TestArena_LatestMoveOnly` | 同帧 10 次 Move 只体现最后点 delta |
| `TestArena_ScrollWheelBypassesTap` | 滚轮改 offset，不产生 tap |

```bash
go test ./ui/gestures -run 'TestArena_' -count=1
```

---

## 轨 B — 嵌套滚动竞争（P4 完整项）

**目标：** 父子 Viewport/Scrollable 同时可滚时，**谁吃 drag** 有文档 + 单测；不再「父子各滚一半」或双消耗。

### B.1 规则草案（MVP，写进包注释）

```text
默认（类似 Flutter scrollable 最小集）：
1. 命中最深可滚子先认领 pointer（加入 arena）
2. 子在边界且继续同向拖 → 子 reject / 父 claim（边界移交）
3. 子未到边界 → 子独占，父不滚动
4. 滚轮：默认最深可滚消费；到边界可冒泡（可配）

非目标：完整 multi-axis、Android overscroll glow、平台差异像素级对齐
```

### B.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| B1 | 嵌套两层 Viewport + 双 Scrollable/Pan | 结构可测 |
| B2 | 子未到顶/底：只子 offset 变 | 单测 |
| B3 | 子已到顶且继续下拉：父开始滚（或明确「不移交」开关） | 单测钉一种默认 |
| B4 | 文档：`scroll_competition.md` 段或 `gestures` 包注释 | 与测一致 |
| B5 | 回归 S5/S6：单层行为不变 | 绿 |

### B.3 门禁

| 项 | 标准 |
|----|------|
| 子未边界 50 次 Move | 父 `ScrollOffset` 不变 |
| 子到边界后再 50 次 | 父 offset 单调变化；无双倍 delta |

---

## 轨 C — Focus（焦点树 · Tab · 键盘路由骨架）

**目标：** 输入类控件（P7）可依赖的焦点底座；**焦点环绘制不破坏脏区模型**。

### C.1 模型

| 概念 | 说明 |
|------|------|
| `FocusNode` | 可聚焦实体；`RequestFocus` / `Unfocus` / `HasFocus` |
| `FocusManager` | 主窗一个；维护 primary focus、历史 |
| `FocusOrder` | MVP：树深度优先；可选显式 `TabIndex` |
| 键盘路由 | `EventKey` → 先给 primary focus 的 `OnKey`；未处理再 Tab 遍历 |
| 焦点环 | 可选 `onFocusChange` → 目标 `MarkNeedsPaint`；**仅**该 Boundary/节点脏 |

**不做：** 完整 Shortcuts/Actions 体系、平台 IME 绑定、读屏焦点同步。

### C.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| C1 | `FocusNode` + `FocusManager.RequestFocus/Blur` | 单测唯一 primary |
| C2 | 注册/注销：节点 detach 自动 blur | 单测无悬空 primary |
| C3 | Tab / Shift+Tab 遍历可聚焦列表 | 单测顺序 |
| C4 | 键盘：Space/Enter 回调挂在 node（`OnKey`） | 单测消费 |
| C5 | 焦点变化 → 仅相关节点 NeedsPaint；layout 不整树 | 计数断言 |
| C6 | 与 HitTest：PointerDown 可 `RequestFocus` 命中可聚焦者 | 单测 |
| C7 | （可选）焦点环 paint helper：outset rect，**不**全屏 saveLayer | 预算内 |

### C.3 门禁

| 项 | 标准 |
|----|------|
| 10 节点 Tab 一圈 | 顺序稳定；回到起点 |
| 焦点在 Boundary 内切换 | `layout_count` 不因焦点涨；paint 限于脏节点 |
| 无焦点时 Key | 不 panic；可 no-op |

```bash
go test ./ui/focus -run 'TestFocus_' -count=1
```

---

## 轨 D — Overlay 机制（填满 F13 band）

**目标：** **机制** 可用：插入 / 移除 / 栈序 / 命中；**不是** Ant Modal 皮肤。

### D.1 模型

```text
FramePacket
  Root     = main band（应用树）
  Overlay  = overlay band（P2 已 EnsureOverlayBand）

OverlayState
  entries []OverlayEntry   // 后插入者更靠上（paint 后画、命中先测）

OverlayEntry
  builder / root RenderObject 或 Layer 子树
  onRemove
```

| 规则 | 说明 |
|------|------|
| Paint 序 | main 全量合成后画 overlay（或 raster 已 walk Overlay） |
| 命中序 | **先** overlay（顶→底），未命中再 main |
| 插入/移除 | 只脏 overlay 相关层；**禁止** 为弹层强制 main 全树 MarkNeedsPaint |
| 多 entry | 栈：后进先命中 |

### D.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| D1 | `OverlayEntry` + `OverlayState.Insert/Remove` | 单测栈深 |
| D2 | `BuildLayerTree` / packet：真实 `pkt.Overlay` 非空壳（有 entry 时） | 单测 Walk 含 overlay 子 |
| D3 | 命中：点在浮层上不落到 main | 单测 |
| D4 | 移除 entry 后命中恢复 main | 单测 |
| D5 | 脏区：仅 overlay entry 动画/变色时 main `DirtyLayerIDs` 不无故暴涨 | 计数或集合断言 |
| D6 | embedder/路由：指针先 overlay HitTest | 与 D3 联调 |
| D7 | （可选）barrier 全屏入口：半透明 ColorBox + 吃 pointer；仍走 overlay band | 单测吃事件 |

### D.3 门禁

| 项 | 标准 |
|----|------|
| 1 overlay + main Spinner | Spinner 仍可 CompositeOnly；overlay 自有 dirty |
| 插入/删除 100 次 | 无泄漏 entry；Focus/Arena 无悬空 |
| 依赖 | overlay 包 ↛ gpu |

```bash
go test ./ui/overlay -count=1
go test ./ui/scene -run 'Overlay|Band' -count=1
```

---

## 轨 E — Animation 完备 · Semantics · Theme（薄）

### E.1 Animation（F09 完整）

P3 已有 `animation.Controller`（0..1、Ticker、完成卸注册）。P5 补齐：

| # | 任务 | DoD |
|---|------|-----|
| E1 | `Curve` 接口 + `Linear` / `EaseInOut`（至少 2 条） | 单测 t→y 边界 0/1 |
| E2 | `Controller` 接 Curve；`Status`：dismissed/forward/completed（reverse 可选） | 单测状态迁移 |
| E3 | 完成 / Stop → 自动卸 Ticker（**F17 回归**） | 单测 registry 空 |
| E4 | **隐式动画 MVP**：`AnimatedOpacity` 或 helper — 改 opacity 走 `MutSetOpacity` / compositor dirty，**默认不**整子树 re-raster | 单测 ClassifyDirty |
| E5 | 文档：默认动画路径 = compositor-only；例外才 saveLayer | 包注释 |

**门禁（F09）：**

```text
隐式 opacity 动画 30 帧：
  · layout_count 不增（或仅首帧）
  · CompositorDirty 有、Raster 脏层不因 opacity 每帧全量
  · 结束后 Ticker 卸掉 → 可回 IDLE（接 S0）
```

### E.2 Semantics 最小

| # | 任务 | DoD |
|---|------|-----|
| E6 | `SemanticsNode`：`Role` / `Label` 字段；可挂到 RO 或并行树 | 单测读写 |
| E7 | 遍历导出扁平列表（调试/测试用） | 单测 |
| E8 | **不**接系统读屏；文档标明骨架 | 注释 |

### E.3 主题 / Token 传播（最小）

| # | 任务 | DoD |
|---|------|-----|
| E9 | `theme.Tokens`：Primary / Surface / FontSize 等最小集 | 编译 |
| E10 | `Provider`：全局 default + 可选 Override 闭包/栈 | 单测读取 |
| E11 | **不**做 Ant Token 全表、不做 CSS 选择器 | — |

---

## 轨 F — 示例 · 指标 · 回归 · 文档

| # | 任务 | DoD |
|---|------|-----|
| F1 | `examples/ui_l2_shell`：Tap/Pan 区 + Tab 焦点环 + 一键 Insert Overlay | 真窗可跑（有 DISPLAY） |
| F2 | 指标（可选 JSON 字段）：`gesture_arena_depth`、`focus_id`、`overlay_count` | 字段稳定或测试计数 |
| F3 | 回归：`go test ./ui/...`；S0/S2/S5/S6 相关不挂 | 全绿 |
| F4 | 更新 CLOSEOUT / OUTLINE / README：P5 细卡链接与状态 | 文档 |
| F5 | depcheck：新包均无 import gpu | `TestNoGPUImport` 绿 |
| F6 | （可选）程序化无窗：shell 场景的 arena+focus+overlay 集成测 | 单测 |

---

## 2. 场景门禁总表（P5 关门）

| ID | 场景 | 通过标准 |
|----|------|----------|
| **G-TapPan** | 点击与拖矩阵 | 互斥触发；见轨 A |
| **G-NestScroll** | 嵌套滚动 | 边界内外责任清晰；见轨 B |
| **G-Focus** | Tab 顺序 + 键 | 唯一 primary；脏区局部 |
| **G-Overlay** | 浮层命中与 band | 先 overlay 后 main；main 不被迫全量 |
| **G-Anim** | 隐式 opacity | compositor-only；结束 IDLE |
| **S0 回归** | 空闲 | 仍可阻塞；无僵尸 Ticker |
| **S2 回归** | Spinner | layout 不涨；脏层仍小 |
| **S5/S6 回归** | 列表/拖滚 | bind 上界；Scrollable/Arena 兼容 |
| **依赖** | depcheck | ui 无 import gpu |

```bash
go test ./ui/... -count=1
go test ./ui/gestures ./ui/focus ./ui/overlay ./ui/animation ./ui/semantics ./ui/theme -count=1
go test ./ui/rendering -run 'TestS5_|TestS6_|TestViewport_' -count=1
# 有 DISPLAY：
go run ./examples/ui_l2_shell
go run ./examples/ui_l1_spinner   # 回归
go run ./examples/ui_l1_scroll    # 回归
```

---

## 3. F 项勾选（P5）

| ID | 内容 | 轨 | 状态 |
|----|------|-----|------|
| **F09** | 动画默认 compositor-only（完备） | E | ⬜ |
| **F13** | Overlay band **填满**（不再仅预留） | D | ⬜ |
| **F17** | Ticker/Animation 结束卸注册（加深） | E | ⬜ |
| 手势竞技 | GestureArena + 矩阵 | A | ⬜ |
| 嵌套滚动 | 竞争规则 | B | ⬜ |
| 焦点壳 | Focus 树 + Tab | C | ⬜ |
| Semantics/Theme | 最小骨架 | E | ⬜ |
| 回归 F01–F08/F14 等 | 不回退 | F | ⬜ |

---

## 4. PR 切片建议

| PR | 内容 | 合并前提 |
|----|------|----------|
| **P5a** | 轨 A GestureArena + Tap/Pan 矩阵 | `TestArena_*` 绿 |
| **P5b** | 轨 B 嵌套滚动 + Scrollable 迁 arena | 嵌套测 + S6 绿 |
| **P5c** | 轨 C Focus | `TestFocus_*` 绿 |
| **P5d** | 轨 D Overlay 机制 + packet 真 band | overlay 测绿 |
| **P5e** | 轨 E Curve/Status/隐式动画 + semantics/theme | F09 相关测绿 |
| **P5f** | 轨 F 示例 + 文档勾选 + 全量回归 | `go test ./ui/...` + 烟囱 |

**禁止：**

- 单 PR 混入 `docs/antd` 控件实现或 P7 皮肤  
- 单 PR 大改 P6 多窗 / RasterCache  
- 为 Overlay 绕过 Layer 直接全屏 Present  

---

## 5. 风险

| 风险 | 缓解 |
|------|------|
| Arena 与 ad-hoc Scrollable 双路径行为分叉 | P5a/b 明确迁移；S6 必回归 |
| 焦点环每帧全树 paint | 环放 Boundary；只 MarkNeedsPaint 局部 |
| Overlay 插入导致 main Dirty 全集 | D5 断言；entry 自建 Boundary |
| 隐式动画误走 Picture 重录 | E4 走 MutSetOpacity；ClassifyDirty 测 |
| 嵌套滚动规则与直觉不符 | B 轨文档钉默认；提供「不移交」开关 |
| 范围膨胀成 a11y/IME/Ant | 非目标表 + PR 禁止清单 |
| 事件路由侵入 embedder 过深 | 先测 helper；再最小改 PipelineApp |

---

## 6. 完成定义（P5 Done）

- [ ] 轨 A–F 主路径（可选真窗键鼠细节可后补，但单测矩阵必须齐）  
- [ ] G-TapPan / G-Focus / G-Overlay / G-Anim 单测硬断言绿  
- [ ] 嵌套滚动默认规则有测（B）  
- [ ] `go test ./ui/...` 绿；ui 无 gpu import  
- [ ] S0/S2/S5/S6 回归绿  
- [ ] `examples/ui_l2_shell` 可演示三件套（手势/焦点/浮层）  
- [ ] 文档勾选；OUTLINE 挂「P5 细卡已开」  
- [ ] **无** antd 控件合入本阶段 Done 条件  

## 6.1 实现状态（开工后维护）

| 项 | 状态 |
|----|------|
| GestureArena + Tap/Pan | ⬜ |
| 嵌套滚动竞争 | ⬜ |
| FocusManager + Tab | ⬜ |
| OverlayState 真 band | ⬜ |
| Curve / 隐式 compositor 动画 | ⬜ |
| Semantics / Theme 骨架 | ⬜ |
| ui_l2_shell | ⬜ |
| 回归 P0–P4 门禁 | ⬜ |

---

## 7. 与 P6 / P7 的边界

```text
P5 做完 → 复杂交互有壳，P7 控件「不必自写手势/焦点/浮层」
P6     → 合成效率、多窗、HUD（不挡 P5 关门）
P7     → docs/antd 按波次迁入；只依赖 L1+L2；禁止直触 GPU
```

P7 触发条件（大纲原句）：L2 手势 / 焦点 / Overlay **至少可用** —— 即本卡 A+C+D 主路径绿即可开 P7 波次细卡，不必等 E 主题完美。

---

## 8. 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-27 | 首版 P5 细卡：A–F 分轨、手势矩阵、Overlay 机制、排除 antd |
