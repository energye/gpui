# 老能力与 Flutter 对齐审计（复用前必查）

> 目的：框架里已有的东西，能直接拿来拼组件的才拿；名字像但做法不一样的，先修底层再用，不许在产品里绕。
> 方法：逐个跟 Flutter 源码比（类名后注出处），再看 gpui 本体（包名后注文件:行号，均为本机实测）。
> 处置只三档：**直接复用** / **包一层复用**（F0 门面）/ **退回底层**（走底层 skill 修，不在 `ui/kit` 里手写）。

## 1. 动画：部分对齐（能转，但曲线不全）

- Flutter：`animation/curves.dart`（`Curves.linear/decelerate/ease/easeIn/easeOut/easeInOut/elasticInOut/bounce…` 二十多种 + `Cubic/Interval/Threshold/SawTooth/Flipped`）+ `AnimationController`（前进后退循环绑 vsync）+ `implicit_animations.dart:604` 起自动画件 + `transitions.dart` 过渡件。
- gpui：`ui/animation/controller.go:11`（0 到 1 走节拍器，`SetCurve`）+ `curve.go:45`（只有 `Linear/EaseInOut/EaseIn/EaseOut` 四条）+ `ui/scheduler/ticker.go`（节拍注册表）。
- 结论：节拍器和控制器是对的，但曲线只有 4 条（缺 `decelerate/bounce/elastic/cubic/interval` 等），没有补间（Tween 插值）和自动画件。
- 处置：**包一层复用 + 底层按需补曲线**。F0 的动效件先用现有 4 条跑悬停波纹转圈；组件要的 `decelerate` 这类常用曲线，退回底层一次补（带单测），不许在产品里手写缓动函数。

## 2. 手势：部分对齐（点按拖动滚动通，长按双击缩放待查）

- Flutter：`gesture_detector.dart:223`（点按/双击/长按/横竖拖/平移/缩放全回调）+ 竞技场（谁赢谁收）+ `RawGestureDetector:1318` 自拼。
- gpui：`ui/gestures/arena.go:24`（竞技场）+ `tap.go:11` + `pan.go:9` + `scroll.go:13`（滚轮）+ `dispatcher.go:16` + 多点测试（`arena_test.go:251` 双点互不干扰）。
- 结论：点按拖动滚动的架子是对的；长按、双击、捏合缩放有没有 recognizer，本次只看到点按与平移文件，要用时先查，无则退回底层补。
- 处置：**包一层复用**。F0 交互件只调现有识别器；缺的长按缩放走底层补，不在组件里算距离时间。

## 3. 焦点键盘：部分对齐（节点和环有，跑焦点的规矩缺）

- Flutter：`focus_manager.dart:456`（节点）+ `focus_scope.dart:126/804`（焦点盒/范围）+ `focus_traversal.dart`（Tab 下一个去哪三种策略）+ `shortcuts.dart`/`actions.dart`（按键翻成意图再动手）。
- gpui：`ui/focus/manager.go:7` + `node.go:41` + `ring.go:8`（焦点环矩形）。
- 结论：节点和环是对的；跑焦点的分组策略、快捷键翻意图那套没见到。
- 处置：**包一层复用 + 缺的补底层**。F0 先保 Tab 进回车动 Esc 退；弹窗圈地、按键直调函数的快捷键，退回底层补。

## 4. 浮层：基本对齐（定位翻转有关闭，差对照表）

- Flutter：`overlay.dart:479/109/1869`（浮层栈/条目/新式跟随）+ `CompositedTransformTarget/Follower`（`basic.dart:1946/2008` 箭头跟随）。
- gpui：`ui/overlay/placement.go:266`（`Resolve`）+ `:90`（`FlipPlacement`）+ `entry.go:61`（条目）+ `state.go:191`（命中栈）。
- 结论：定位翻转条目命中四件是对的；十二方向名字跟 antd 对不上、箭头跟随尺寸，要一张对照表钉死。
- 处置：**包一层复用**。对照表进 F0 附录，算法不动。

## 5. 主题：部分对齐（种子有，缺全量对数）

- Flutter：`material/theme.dart:50`（主题下发）+ `color_scheme.dart:128`（语义色板）+ `button_style.dart:162`（三级合并）+ `widget_state.dart:821`（按状态取值）。
- gpui：`ui/theme/tokens.go:61`（种子结构体）+ `:280`（主色悬停按压底边四派生已对 `#4096ff/#0958d9/#e6f4ff/#91caff`）。
- 结论：结构是对的；种子是否跟 antd 6.5.1 默认浅色逐项对完（字号间距圆角控件高阴影遮罩），本次只抽到主色四派生，全量待 F0-1 对数。
- 处置：**包一层复用 + F0-1 对数**。对不上的数不许开工。

## 6. 文字排版：部分对齐（能显示能测，复杂混排待验）

- Flutter：`Text: text.dart:497` + `RichText: basic.dart:6496` + `ParagraphBuilder` + 整形换行省略。
- gpui：`ui/rendering/text.go:32`（`RenderText`）+ `paragraph.go:51`（`ParagraphBuilder`）+ 测量缓存（`text.go:571`）。
- 结论：单样式多样式两件都在；双向混排、复杂整形、中日韩回落、省略最大行数，以排版单测和 `ui_wr_ime_r2_textlayout`（文本系，非 widget 主表 R 号）为准，能过即用，不过退回文本域补。
- 处置：**包一层复用**，F0-3 以测试为准。

## 7. 层合成：基本对齐

- Flutter：`Offset/Opacity/Transform/Clip/BackdropFilter/PhysicalModel/Picture` 层树。
- gpui：`ui/scene/layer.go`（`Offset:74`/`Opacity:89`/`ClipRect:104`/`ClipRRect:120`/`Transform:138`/`ColorFilter:185`/`ImageFilter:237`/`Backdrop:248`/`Picture:272`/`Boundary:334`）。
- 结论：层种类齐，独立重画边界有（`basic.dart:7558` 对 `Boundary:334`）。
- 处置：**直接复用**。

## 8. 语义无障碍：不对齐（只有点，缺树）

- Flutter：`Semantics: basic.dart:7861` + 合并/排除/阻断 + 语义动作。
- gpui：`ui/semantics/node.go:22`（角色名标签）+ `:56`（拍平），两个单测。
- 结论：只能存读屏节点，缺合并排除、缺动作（点按调值）、缺与焦点联动。
- 处置：**退回底层补**后再用。F0 先保每个可交互件有名有角色；合并排除与动作走底层补。

## 9. 调度节拍：基本对齐

- Flutter：调度绑 vsync，无信号 16.67ms 回落，帧分建布局绘制合成。
- gpui：`ui/scheduler/ticker.go` + `metrics.go`（帧计数分位）+ 真窗 `vsync_source`（回落不许称锁 60Hz）。
- 处置：**直接复用**。

## 10. 滚动虚拟列表：基本对齐

- Flutter：`Scrollable:121` + `Viewport:56` + `SliverList:167` 按需建行。
- gpui：`ui/rendering/scrollable.go` + `viewport.go` + `virtual_list.go`，`ui_wr_r7` 已证只建可见行。
- 处置：**直接复用**。

## 处置总表（语义有名角色先行，合并排除动作退回，见 F0 §6.2）

| 能力 | 结论 | F0 做法 |
| --- | --- | --- |
| 层合成/调度/滚动虚拟 | 基本对齐，直接复用 | 门面薄包一层就用 |
| 浮层/手势点按拖滚/焦点节点/文字 | 基本或部分对齐，包一层复用 | 缺对照表和策略的补表补策略，不动算法 |
| 动画曲线/长按缩放/焦点遍历快捷键/语义树/种子全量 | 部分或不对齐 | 退回底层按 AGENTS.md 修，产品层不绕 |

退回的活按“能力在哪层修哪层”走，修完带回归（相关 `*_test.go` 逐个跑 + 对应 `ui_wr_*` 真窗重放），再回 F0。
