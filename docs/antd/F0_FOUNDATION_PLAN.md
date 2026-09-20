# F0 地基第一需求：五部分补齐详细开发计划

> 地位：`ARCHITECTURE.md` §5 中 F0 的展开施工图。本文是第一需求的唯一计划真源。
> 依据：Flutter 源码（commit `2d3957cbfb8`，`packages/flutter/lib/src/`，下文类名后均注 `文件:行号`，均为本机实测 `grep ^class` 所得，不是凭记忆）；
> 产品依据 antd v6.5.1；引擎真源 `ENGINE_UI_WIDGET_RENDER.md`（U5–U16、§2.2 指标族 A–J、§2.5 时长）、`UI_PIXEL_ASSERTION_STANDARD.md`（三证据）、`AGENTS.md`（真窗/测试/文档硬规矩）。
> 旧 `ui/kit/*` 产品实现（含 `flex/grid/space/layout/icon`）全部作废，不作为地基；地基只包现有引擎能力，不写新 GPU 代码。

## 0. 你说的五部分，我理解成这样（先对齐，再看补充）

你要的是：照 Flutter 把“拼组件用的小件”全部补齐，一件一件有源码出处，缺的写清楚在哪建；
五部分（布局、装饰绘制、内容、交互行为、往下传的主题范围）作为**第一需求 F0**，每部分都有**开发任务 + 单元测试 + 独立真窗测试 + 验收标准**，全部绿了后面的按钮才开工。

你没提、但不补后面必返工的，我补了 6 项（§6）：动效体系、语义无障碍、文本编辑/IME、性能契约（重画边界/缓存/虚拟化）、方向语言、测试基建与纪律。每项都说明了“为什么必须现在做”。

## 1. P1 布局（对应 Flutter `basic.dart` + `container.dart` 布局段 + `viewport/scrollable/sliver`）

### 1.1 Flutter 出处（抽查行号，`basic.dart`）

| 小件 | 出处 | 一句话 |
| --- | --- | --- |
| `Directionality` | `basic.dart:171` | 文字方向祖先，RTL 时图标前后互换 |
| `Padding` | `basic.dart:2317` | 内边距包一层 |
| `Align` / `Center` | `basic.dart:2479` / `2567` | 对齐，Center 是 Align 特例 |
| `SizedBox` | `basic.dart:2773` | 定宽高或撑缝 |
| `ConstrainedBox` | `basic.dart:2877` | 加约束（最小最大宽高） |
| `UnconstrainedBox` | `basic.dart:3158` | 去掉父约束（慎用） |
| `FractionallySizedBox` | `basic.dart:3254` | 按父比例取尺寸 |
| `LimitedBox` | `basic.dart:3365` | 无界时给上限 |
| `OverflowBox` / `SizedOverflowBox` | `basic.dart:3426` / `3535` | 允许溢出 |
| `Offstage` | `basic.dart:3623` | 藏起来但还占布局 |
| `AspectRatio` | `basic.dart:3748` | 固定宽高比 |
| `IntrinsicWidth` / `IntrinsicHeight` | `basic.dart:3812` / `3887` | 按内容本征尺寸 |
| `Baseline` | `basic.dart:3914` | 按基线对齐 |
| `Stack` / `Positioned` | `basic.dart:4743` / `4894` | 叠放 + 定位 |
| `Flex` / `Row` / `Column` | `basic.dart:5294` / `5695` / `5887` | 横排竖排主轴交叉轴 |
| `Flexible` / `Expanded` | `basic.dart:5930` / `6021` | Flex 孩子权重 |
| `Wrap` | `basic.dart:6080` | 自动换行排 |
| `Flow` | `basic.dart:6356` | 自定义流式（性能敏感） |
| `CustomSingleChildLayout` / `CustomMultiChildLayout` + `LayoutId` | `basic.dart:2589` / `2678` / `2612` | 自定义布局协议 |
| `ListBody` | `basic.dart:4589` | 纵向依次排（Column 简化前身） |
| `Viewport` / `ShrinkWrappingViewport` | `viewport.dart:56` / `387` | 视口（只画可见区） |
| `Scrollable` | `scrollable.dart:121` | 滚动宿主 |
| `SliverList` / `SliverGrid` / `SliverOpacity` / `KeepAlive` | `sliver.dart:167` / `739` / `1343` / `1652` | 按需建行的长列表协议 |
| `Container` / `DecoratedBox` | `container.dart:246` / `63` | 布局 + 装饰的组合糖（Container 是 Stateless 拼装，不是底层） |

对应 `rendering/`：`flex.dart`（RenderFlex）、`stack.dart`（RenderStack）、`wrap.dart`、`viewport.dart`、`sliver.dart`、`proxy_box.dart`（RenderPadding/Align/Center/SizedBox/ConstrainedBox等）。

### 1.2 gpui 现状

有地基：`ui/rendering/box.go`（盒子）、`align.go`（对齐）、`absolute.go`（绝对定位）、`constraints.go`（约束）、`viewport.go`（视口）、`scrollable.go`（滚动）、`virtual_list.go`（虚拟列表）。
缺门面：上面 20+ 小件**没有 widget 层可拼装的 Go 门面**；`ui/kit/flex/grid/space/layout` 是 antd 产品写法（旧实现，作废），不是 Flutter 基础件；`ui/kit/internal/prim` 不存在。

### 1.3 要建（`ui/kit/internal/prim/layout.go`，只包 `ui/rendering`，不碰 GPU）

`Pad`、`Center`、`AlignBox`、`Sized`、`Constrained`、`Aspect`、`Offstage`（布局占位但不画）、`Row`/`Column`（Flex 门面，主轴/交叉轴/间距）、`Expanded`/`Flexible`/`Spacer`（权重/空缝）、`Stack`/`Positioned`/`IndexedStack`（叠放/下标切页）、`Wrap`、`CustomLayout`（回调式自定义）、`LayoutBuilder`（按父约束回调，响应断点必备）、`ViewportBox`（滚动态只画可见行，包 `virtual_list.go`）。
`Container` 不单独建——它是“Padding+DecoratedBox+ConstrainedBox 的拼装糖”，用时现场拼，不多一层抽象（和 Flutter 一致）。基础 `Table`（排版 Table，非 antd 产品表）F0-2 明确延后，Descriptions 先用 Row 拼。

### 1.4 单元测试（逐文件跑，一个文件绿了再跑下一个）

| 测试文件 | 断什么 | 数据 |
| --- | --- | --- |
| `layout_test.go` | 三种约束（紧/松/无界）下 `Row/Column/Stack/Wrap` 尺寸与位置；`Expanded` 按权重分；`Positioned` 偏移；`Offstage` 不画但占位 | `testdata/layout_cases.json`（约束输入 + 期望尺寸位置，不许循环生成） |
| `layout_edge_test.go` | 溢出（`OverflowBox` 语义）、无界套 `LimitedBox`、基线对齐 | 同上加溢出用例 |
| 约束矩阵 | 每个布局件 × 紧/松/无界全跑（见 ACCEPTANCE 第一条第 2 款精神） | 同上 |

缺失数据 `t.Skipf` 写清原因，禁止静默假绿；禁止本机绝对路径，临时文件只许 `t.TempDir()`。

### 1.5 真窗（独立窗 `examples/kit_f0_layout/`，只摆布局，不摆产品组件）

- 几何：基线 **1200×800** 逻辑像素（窗可调大小；基线只用于 Golden/证据对齐，布局必须随调大小重排）；`RUN_SECONDS=5`（正确性窗，无持续动画不判 fps）；必做一次调大小（拉宽/拉窄回基线，布局不错位、不断言崩）。
- 内容：左 Row（定宽+权重+间距）、右 Column、右下 Stack 叠放、底 Wrap 换行、长列表虚拟段（只画可见行）；每段带标题说明。
- 指标：§2.2 **A–J 全族必采**（`fps_wall/interval_*/hitch/vsync_source/cpu_ui/cpu_raster/cpu_pct/rss_*` 一个不少，缺字段即 FAIL）；正确性窗 `slope_gate=off` 可在 README 声明（<15s 才许）。
- 像素三证据：逻辑探针（各段子节点尺寸位置导出 JSON）+ 像素断言（中心点采样容差 ≤8/通道，≥3 点）+ Golden（`testdata/showcase_layout.png`，零容差，未声明容差即零容差；差异超限 FAIL 并产差异图）。
- 跑法：先 `-auto-only` 自检全绿，再常驻等人工（收真事件打日志改标题，关窗或 `-manual-seconds` 超时出汇总 JSON）；跑完即关的一次性窗不能代替常驻人工窗。

### 1.6 验收

- 上述单测全绿 + 真窗 `-auto-only` 全绿 + 人工看过（布局不错位、滚动不抖、长列表快滑只建可见行）+ Golden 入库。
- 产品包 `import` 检查：只许调 `prim/behavior/scope` + `ui/theme`，直调 `render/gpu` 即打回。

## 2. P2 装饰绘制（对应 `basic.dart` 装饰段 + `container.dart:63`）

### 2.1 Flutter 出处

| 小件 | 出处 | 一句话 |
| --- | --- | --- |
| `ColoredBox` | `basic.dart:8393` | 纯底色盒 |
| `DecoratedBox` | `container.dart:63` | 底色+边框+圆角+阴影+渐变（一站式） |
| `ClipRect` / `ClipRRect` / `ClipOval` / `ClipPath` | `basic.dart:952` / `1035` / `1217` / `1286` | 矩形/圆角/椭圆/路径裁剪 |
| `Opacity` | `basic.dart:336` | 整层透明（一次合成） |
| `Transform` / `FractionalTranslation` / `RotatedBox` | `basic.dart:1585` / `2186` / `2247` | 矩阵变换/按比例位移/旋转盒 |
| `FittedBox` | `basic.dart:2108` | 按比例缩放塞进父盒 |
| `ShaderMask` / `BackdropFilter` / `ColorFiltered` / `ImageFiltered` | `basic.dart:427` / `636` + painting 滤镜 | 着色/背景模糊/颜色滤镜 |
| `PhysicalModel` / `PhysicalShape` | `basic.dart:1373` / `1473` | 阴影 + 裁剪 + 高度一体 |
| `CustomPaint` | `basic.dart:833` | 给画笔自己画（painter/repaintBoundary 语义） |
| `CompositedTransformTarget/Follower` | `basic.dart:1946` / `2008` | 跟随定位（浮层箭头跟随底座） |

### 2.2 gpui 现状

有：`clip_rrect.go`、`opacity.go`、`transform.go`、`filter_ro.go`、`draw.go`。
缺：统一 `DecoratedBox` 门面（底边圆角阴影渐变一次拼好）、`CustomPaint` 门面（painter + 独立重画标记）、跟随定位门面（给浮层用）。

### 2.3 要建（`prim/decor.go`）

`Colored`、`Decorated`（底/边/圆角/阴影/渐变，参数对标 antd Token，不手写色值）、`ClipRect/RRect/Oval`、`OpacityLayer`（只做整层合成，逐通道 dim 不算对）、`Transformed`、`Fitted`、`CustomPaint` 门面（`Painter func(PaintCtx)` + `repaintBoundary bool`）、`FollowTarget/Follower`（包 overlay 定位，给 D 类浮层用）。

### 2.4 单测

`decor_test.go`：圆角裁剪后命中区一致（Hit ≡ 绘）、透明 0.65 整层合成（混合公式验算，容差 ≤12/通道并附公式）、阴影不扩大命中、渐变只断两端点 + 中点单调。数据入 `testdata/decor_cases.json`。

### 2.5 真窗（`examples/kit_f0_decor/`，基线 1200×800 可调大小，5s）

底边圆角阴影矩阵 + 透明叠加 + 变换旋转 + 裁剪头像 + 自定义画（对勾/圆环）+ 跟随小条；像素断言按 `UI_PIXEL_ASSERTION_STANDARD.md` F0（静态）+ F5（半透明混合公式）+ Golden 入库；指标 A–J 全族。

### 2.6 验收

混合色必须附公式验算；逐通道 dim 洗白、裁剪后点得中画不中（Hit ≠ 绘）即 FAIL。

## 3. P3 内容（对应 `text.dart` + `image.dart` + `icon.dart` + `basic.dart:6496/6733`）

### 3.1 Flutter 出处

| 小件 | 出处 | 一句话 |
| --- | --- | --- |
| `Text` / `RichText` | `text.dart:497` / `basic.dart:6496` | 单样式 / 多 span 富文本 |
| `DefaultTextStyle` | `text.dart:55` | 文字样式祖先（往下继承） |
| `Image`（+ `FadeInImage` 占位渐显） | `image.dart:343` + `fade_in_image.dart:61` | 位图 + 占位淡入 |
| `RawImage` | `basic.dart:6733` | 直接贴像素（视频帧/纹理） |
| `Icon` / `ImageIcon` / `IconTheme` | `icon.dart:74` + `image_icon.dart:25` + `icon_theme.dart:24` | 图标字体 + 图片图标 + 主题尺寸颜色 |
| `ColorFiltered` / `ImageFiltered` | `color_filter.dart:36` + `image_filter.dart:39` | 逐像素改色 / 图像滤镜（模糊等） |
| `EdgeInsets` / `EdgeInsetsDirectional` | `painting/edge_insets.dart:390` / `742` | 物理边距 / 跟随读写方向边距 |
| `BorderRadius` / `BorderRadiusDirectional` | `painting/border_radius.dart:353` / `603` | 物理圆角 / 跟随方向圆角 |

### 3.2 gpui 现状

有：`text.go`/`paragraph.go`（段落构建布局绘制）、`image.go`/`image_draw.go`（绘制九宫圆角）、系统字体加载。
缺：`DefaultTextStyle` 继承门面、富文本多 span 样式栈门面、图片占位→渐显状态机、图标主题（尺寸颜色随 Ctx）门面；旧 `ui/kit/icon` 作废。

### 3.3 要建（`prim/content.go`）

`Label`（单样式文字）、`RichLabel`（多 span，样式栈）、`TextStyleScope`（DefaultTextStyle 对标）、`Picture`（占位→图→错三态，异步到图只标脏一格）、`GlyphIcon`（图标字体，尺寸颜色走 Ctx）。

### 3.4 单测

`content_test.go`：多 span 换行高度、省略（`maxLines/ellipsis`）、有字体真测 / 无字体启发式（`t.Skipf` 写清）、图片三态流转、图标尺寸跟主题。文字断言用区域级（包围盒内非背景像素数 ≥ 阈值 + Golden），禁止单点采样断字形。

### 3.5 真窗（`examples/kit_f0_content/`，基线 1200×800 可调大小，5s；图片异步段加长 15s 看占位→图）

单行/多行/省略/富文本/图标行/图片三态；出图后主树重录仅一格（`paint_count` 可解释）；A–J 全族 + Golden。

### 3.6 验收

字是真字（有字体字区有墨，无字体宁可空，不画黑条）；图片占位到图只脏一格，多一格即 FAIL。

## 4. P4 交互行为（对应手势/焦点/表单/浮层/滚动/语义/指针七组）

### 4.1 Flutter 出处

| 组 | 小件 | 出处 |
| --- | --- | --- |
| 手势 | `GestureDetector`（点按/长按/双击/悬停/拖） | `gesture_detector.dart:223` |
| 指针 | `Listener`（裸指针）、`MouseRegion`（悬停进出）、`IgnorePointer` / `AbsorbPointer` | `basic.dart:7160` / `7302` / `7637` / `7750` |
| 焦点键盘 | `Focus` / `FocusScope` / `ExcludeFocus` + `FocusNode/FocusScopeNode` | `focus_scope.dart:126/804/962` + `focus_manager.dart:456/1354` |
| 表单 | `Form` / `FormState` / `FormField` / `FormFieldState` | `form.dart:60/241/518/638` |
| 浮层导航 | `Overlay` / `OverlayState` / `OverlayEntry` / `OverlayPortal` | `overlay.dart:479/650/109/1869` |
| 滚动条 | `Scrollable` + `Scrollbar` + `RefreshIndicator`（按需） | `scrollable.dart:121` + material 滚动条 |
| 语义 | `Semantics` / `MergeSemantics` / `ExcludeSemantics` | `basic.dart:7861/8040/8098` |
| 重画边界 | `RepaintBoundary` | `basic.dart:7558` |

### 4.2 gpui 现状

`ui/gestures`、`ui/focus`、`ui/overlay`（十二方向/翻转/箭头/外点关/焦点锁）、`ui/semantics`、`ui/scene`（Layer）全有地基；缺统一行为件。

### 4.3 要建（`ui/kit/internal/behavior/` 四件）

- `interactive.go`：悬停/按压/焦点/禁用/加载五态机（idle→hover→pressed→idle，禁用全吞，加载防重），只包手势 + 焦点，不碰绘制。
- `field.go`：值 + `DefaultValue` + `OnChange` + 校验（受控/非受控，抄 `FormField`），`Value()` 给 render 读。
- `overlay_trigger.go`：方向/翻转/箭头/外点关/焦点锁/Esc（只调 `ui/overlay`，不自写算法；`Draggable/DragTarget` 按 `drag_target.dart:174/619` 语义，F0 只做触发器，拖放源与落点延后到 E 类长列表）。
- `motion.go`：见 §6.1（显隐动画 + 节拍器，波纹转圈走这里；`AnimatedContainer/Padding/Align/Opacity…` 在 `implicit_animations.dart:604/902/994/1841`，`Fade/Slide/Scale` 在 `transitions.dart`）。

### 4.4 单测

`interactive_test.go`（五态流转、禁用全吞、加载防重、键盘 Tab/回车空格/Esc）、`field_test.go`（受控只回调不改值、非受控内部存、校验错提示）、`overlay_trigger_test.go`（外点关、Esc 关、焦点锁不外泄）。状态机缺一态即 FAIL。

### 4.5 真窗（`examples/kit_f0_interact/`，基线 1200×800 可调大小，15s 看焦点与浮层）

可点块（悬停按压变色）+ 禁用块（点不动）+ 加载块（防重）+ 输入框（受控/非受控）+ 浮层触发（十二方向抽查 4 向 + 翻转 + 外点关 + Esc）+ 焦点走查（Tab 全程、焦点环只键盘亮）。
指标 A–J 全族；Hit ≡ 绘（点哪高亮哪）；浮层开后主树 `paint_count` 不涨（对标 R8）。

### 4.6 验收

鼠标点不亮焦点环（亮了即 FAIL，`:focus-visible` 语义）；禁用可点中、加载可重入、外点关不掉、焦点锁外泄，任一即 FAIL。

## 5. P5 下传与主题（对应 `framework.dart` 三件套 + `media_query.dart` + `theme.dart` + `widget_state.dart`）

### 5.1 Flutter 出处

| 机制 | 出处 | 一句话 |
| --- | --- | --- |
| `InheritedWidget` | `framework.dart:1855` | 包住 child 往下传，`updateShouldNotify` 为真才通知 |
| `InheritedModel`（按需订阅版） | `widgets/inherited_model.dart:121`（不在 framework.dart 内） | 按 aspect 只刷订阅者（`MediaQuery` 就是它） |
| `RenderObjectWidget` / `Single/MultiChild` | `framework.dart:1893` / `1958` / `1994` | 布局绘制挂载点 |
| `MediaQuery` / `MediaQueryData` | `media_query.dart:1276` / `203` | 窗口尺寸/边距/方向往下传 |
| `Theme` / `ThemeData` / `Theme.of` | `material/theme.dart:50` | 全局主题往下传 |
| `ButtonStyle`（三级合并） | `material/button_style.dart:162` | 自己写的 > 主题里的 > 默认的 |
| `WidgetState` / `WidgetStateProperty` | `widget_state.dart:168` / `821` | 按悬停按压焦点禁用等状态取值 |
| `EdgeInsets` / `BorderRadius` / `TextStyle` | `painting/edge_insets.dart` / `border_radius.dart` / `text_style.dart:468` | 间距圆角字形基础类型 |
| `ColorScheme` | `material/color_scheme.dart:128`（不在 painting 内，实测 painting 无此类） | Material3 语义色板，给主题和组件默认色统一取色口 |
| `DefaultTextStyle` / `IconTheme` / `DefaultAssetBundle` | `text.dart:55` + icon/theme + `basic.dart:7044` | 文字/图标/资源祖先 |

### 5.2 gpui 现状

`ui/theme` 有种子 + 别名（`tokens.go`）；`ARCHITECTURE.md` 的 `Ctx` + `StateResolver` 还是纸面；`scope` 包不存在。

### 5.3 要建（`ui/kit/internal/scope/` + `ui/theme` 补数）

- `ctx.go`：`Ctx` 全字段（见 `WIDGET_MODEL.md` §5），显式传参，`Provider/Use` 按 aspect 订阅（对标 InheritedModel），不读全局。
- `states.go`：`WidgetState`（hover/pressed/focused/disabled/loading/selected/error）+ `StateResolver[T]`（`Resolve(states)`，Render 段只调它，不写散装 if）。
- `resolve.go`：三级合并（这次 Props > Ctx 组件主题 > 全局种子）。
- `ui/theme` 补数：Seed/Alias 跟 antd 6.5.1 默认浅色逐项对数（主色/文字/背景/边框、字号、间距 8、圆角、控件高、阴影、遮罩；悬停=调色板[5]、按压=[7]，以发布包为准）；焦点环全库统一（宽 3、色 #91caff、偏 1、只键盘亮）。

### 5.4 单测

`scope_test.go`（换 Ctx 重跑 Resolve 值跟着变、组件代码不动）、`states_test.go`（各状态组合取值跟官网调色板对）、`seed_test.go`（种子值与 antd 发布包逐项对，差一处即 FAIL）。

### 5.5 真窗（`examples/kit_f0_scope/`，基线 1200×800 可调大小，5s）

同一按钮 × 三套主题（默认/换肤/紧凑）+ 整树禁用开关 + 尺寸切换；一切换全局生效；A–J 全族 + Golden（三套主题各一张）。

### 5.6 验收

换肤要改组件代码、整树禁用漏一件、尺寸切换要逐个调，任一即 FAIL。

## 6. 你没提、但必须一起做的补充（不做后面必返工）

1. **动效体系**（出处 `implicit_animations.dart:604` 起 `AnimatedContainer/Padding/Align/Opacity…` + `transitions.dart` + `scheduler`）：显式（转圈波纹走 `Ticker` 真转）+ 隐式（悬停淡入淡出）+ 曲线时长 Token + 省动效总开关。验收：转圈不转、波纹按住就有（应点完才散）、省动效不停，FAIL。
2. **语义无障碍**（出处 `basic.dart:7861`）：每个可交互件有名（读屏读得出）、有角色（按钮/输入/开关）、有状态（禁用/选中）；最小点 44px、对比度、焦点环（见 P4）。验收：读屏树缺名、对比度不够，FAIL。
3. **文本编辑/IME**（出处 `editable_text.dart` + `textinput` + 引擎 `ENGINE_TEXT_X11/WAYLAND_IME_REQUIREMENT.md`）：控制器（值/选区/光标）、占位、校验时机；X11/Wayland 输入法走引擎既有链路，F0 只留接口 + Skip 写清。验收：光标定位靠猜、无 Skip 硬过，FAIL。
4. **性能契约**（出处 `basic.dart:7558` + 引擎 R3/R4/R7/R14）：转圈波纹骨架标独立重画、静止不动；缓存预算满了踢最老；长列表只建可见行（`bind_count≪item_count`）；首帧有内容；后台冻结。验收：静止帧还在全量重画、滚动掉帧（`fps<55`）、RSS 爬升超预算，FAIL。
5. **方向语言**（出处 `basic.dart:171` + `localizations.dart`）：`Dir` 进 Ctx，图标前后、分页箭头跟着走；空态文案走 Locale。验收：切 RTL 图标不换边，FAIL。
6. **测试基建与纪律**：`testdata/` 管数据、逐文件跑回归、`<5s` 不得关闭、人工常驻窗 + `-auto-only` 双轨、Golden 零容差（放宽必须声明依据并产差异图）、公开 API 改同步总账 + 目录覆盖检查、跨平台格一次标齐（X11 真机 + `/gpui-wayland-nested` 嵌套 Wayland 双证据）、`CGO_ENABLED=0` 可构建、产品禁直调 `gpu`（编译期 `depcheck` 卡）。

## 6.5 七个大件（与 F0 并行单独立项，不挡小组件）

| 大件 | 对标 | 需求 | 验收 |
| --- | --- | --- | --- |
| G1 弹窗宿主 ModalHost | App 级弹框栈 | 栈层叠/zIndex1000/嵌套/命令式 confirm+contextHolder/update/destroy/destroyAll/Promise/焦点圈地回焦自动聚焦/mask合并/memo销毁/路由/scrollLock，holderRender 同批落地 | 嵌套三层不乱序，关一层只关一层，静态调用吃得到主题 |
| G2 消息队列层 | NoticeList/MessageHost | taskQueue/GlobalHolder+三路全局+pauseOnHover冻结+maxCount vs stack双机制+top8+key更新/destroy+Promise恰一次；Notification 加四角独立池+进度条+悬停暂停+固定宽 | 超数丢最旧，悬停冻结计时，同 key 只更新不新增 |
| G3 漫游打洞 Tour | RCTour 委托 | mask+gap offset6/radius2+圆角+target矩形注入跟随滚+scrollIntoView+洞区交互开关+语义mask+zIndex上下文+焦点管理+1001盖1000+mask=false仍绘洞 | 洞跟着目标走，洞区点不透，步骤切换面板重定位 |
| G4 日期引擎 | GenerateConfig 可插拔 | 解析格式化多格式+locale/周起始/佛历+disabled矩阵+Range/多选/order/预设函数+受控面板mode/pickerValue | 闰年周起始预设范围全对，换日期库只换实现不改组件 |
| G5 颜色模型 | AggregationColor | 渐变多stop+cleared+双回调onChange/Complete+受控对象精度+format/disabled三开关+panelRender+饱和面板拖拽 | 字符串来回转不丢精度，拖中只回调拖完才提交 |
| G6 上传语义 | Upload 状态机 | LIST_IGNORE+beforeUpload三返回+受控忽略+uid补齐+maxCount替换截断+defaultRequest+预览管线+宿主三件（选文件/拖放/粘贴） | 受控列表不在列表忽略，大文件队列不卡主树 |
| G7 二维码编码库 | 二维码版本纠错掩码 | 版本/纠错等级/掩码全套纯 Go，canvas/svg 双通道一套 CustomPaint 实现 | 与官网同值扫出同码，等级掩码逐项对 |

## 7. 波次与依赖（F0 内顺序，前一波不绿后一波不开；状态只认 PROGRESS.md §1）

| 波 | 内容 | 先决 |
| --- | --- | --- |
| F0-1 | P5 主题与范围（种子对数 + `scope` 做实：ctx/states/resolve 全量 + Provider/Use 按 aspect 订阅 + depcheck/Token 编译期卡） | — |
| F0-2 | P1 布局 + P2 装饰（+Spacer/IndexedStack/LayoutBuilder/基础 Table去留 + Flexible/约束4件/内在3件去留 + 边界件约束 + 补间Tween随曲线补） | F0-1（尺寸圆角色值全从主题来） |
| F0-3 | P3 内容（+TextStyleScope/Picture/EditableText行 + IME 文字门：双向混排/回落/省略不过退回） | F0-2（文字图片都躺在布局盒里） |
| F0-4 | P4 交互 + P5 表单浮层触发（+遍历快捷键/手势前置门/IME控制器接口 + Follow搬本波 + field/IME不倒挂） | F0-3（触发器要摆内容） |
| F0-5 | §6 动效语义性能方向 + 门禁收尾（有名角色先行，合并排除动作退回；中央门禁改查 Props/State/接线/Token） | F0-4 |
| F0-6 | 六个真窗全绿（含 motion 窗规格：转圈波纹Ticker+省动效开关）+ 三套主题 Golden 入库 + 人工签字 | F0-5 |

## 8. 交付清单（F0 做完必须有这些文件；每项状态只认 PROGRESS.md §1，源码对不上即假绿）

```text
ui/kit/internal/prim/{layout,decor,content}.go + 各 _test.go + testdata/*.json
ui/kit/internal/behavior/{interactive,field,overlay_trigger,motion}.go + 各 _test.go
ui/kit/internal/scope/{ctx,states,resolve}.go + 各 _test.go
ui/theme/seed 对数补丁 + seed_ant_test.go
examples/kit_f0_{layout,decor,content,interact,scope,motion}/（各 main.go + README可见效果 + testdata/showcase*.png）
docs/antd/F0_SIGNOFF.md（六窗 -auto-only 全绿 + 人工签字 + Golden 差异率）
```

门禁没改完、签字没留痕，F1（Button 标杆）不开工。
