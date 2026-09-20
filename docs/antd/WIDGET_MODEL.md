# antd 组件实现模型（Flutter 三件套 Go 版）

> 地位：和 [`ARCHITECTURE.md`](./ARCHITECTURE.md) 一起构成新架构真源。本文是“动手手册”——后面每个组件都按本文的模子写。
> 前置：先读 `ARCHITECTURE.md` §0–§2。旧 `ui/kit` 实现作废，不参照。
> 语言说明：Go 无继承、无 JSX，下面的 `Widget/State/RenderObject` 都是“职责名”，不是要照抄 Dart 类名。

## 1. 一个组件 = kit 包下多个文件（同属 package kit，不单建包）

```text
ui/kit/button_props.go, button_state.go, button_render.go, button_theme.go, button_widget.go → Button
ui/kit/alert_props.go, alert_state.go, … → Alert（同理，一个组件可多个文件，文件名带组件前缀）
```

- 五段缺一段先问：Props（对标 Widget）/ State（对标 State）/ Render（对标 RenderObject，只拼基础件）/ Theme（对标 ButtonStyle）/ Build（对标 build()，组合点）。
- 全库同包，导出名必须带组件前缀（如 `ButtonProps`、`AlertProps`、`BuildButton`），不许裸用 `Props`/`Instance`/`Build`（会重名编不过）。
- 公开 API 只有三样：`<Comp>Props`、`<Comp>Instance`、`Build<Comp>(ctx, Props)`；其余全小写。
- **禁止**：Render 段里出现用户文案、色值、字号数字（全走 Theme 段 + Props）；Props 段里出现悬停/按压判断（那是 State 段的活）；State 段里出现画布调用（那是 Render 段的活）。
- 公开 API 增删改必须同步总账文档并跑目录覆盖检查（AGENTS.md 硬规矩，F0 会把检查命令钉下来）。

基础件与行为件（L1/L2）的位置（F0 落定）：

```text
ui/kit/internal/prim/      Box / Flex(Row/Column) / Stack / Text / Icon / Image /
                           DecoratedBox / ClipRRect / Opacity / Padding / Center / SizedBox …
                           （门面：包住 ui/rendering 现有能力，不写新 GPU 代码）
ui/kit/internal/behavior/  Interactive（悬停按压焦点禁用加载）/ Field（值+校验+受控）/
                           OverlayTrigger（定位+外点关+焦点锁）/ Motion（显隐动画+节拍）
                           （门面：包住 ui/gestures、ui/focus、ui/overlay、ui/animation、ui/scheduler）
ui/kit/internal/scope/     Ctx（见 §5 全字段）+ StateResolver[T] + Provider/Use（按 aspect 订阅）
```

产品包只许 `import` 上面三处 + `ui/theme`；直调 `render`/`gpu`、自写定位算法、手抄焦点逻辑，一律打回。

## 2. Props（配置，对标 Widget：不可变）

```go
// ButtonProps 是“这次调用长什么样”，值语义，创建后不许改；改 = 造一个新的调 Update。
type ButtonProps struct {
    // 内容：文字优先 string，富内容用子 Props（对标 child/children，不许传“画好的图”）。
    Label string
    Icon  *IconProps // L1 基础件的 Props，不是画好的节点
    // 外观档位：原样抄 antd 枚举，不自创名字。
    Type    ButtonType    // primary|dashed|link|text|default（语法糖，见 Theme 段换算）
    Color   ButtonColor   // default|primary|danger|+预设色（antd 6.5.1 PresetColors）
    Variant ButtonVariant // outlined|dashed|solid|filled|text|link（auto=按 Type 推导）
    Size    SizeType      // small|medium|large（默认 medium；middle 仅兼容，见 SizeContext）
    Shape   ButtonShape   // default|circle|round
    Block   bool
    Ghost   bool
    Disabled bool
    Loading  LoadingProp // false | true | {delay, icon}（delay 到才转，不闪一下）
    // 行为回调：点一下干什么，只调回调，不在组件里做业务。
    OnClick func()
    // 样式逃生口：对标 classNames/styles，按语义节点（root/icon/content）分，不许整坨 style 盖掉主题。
    ClassNames map[string]string
    Styles     map[string]string
}

func DefaultButtonProps() ButtonProps { /* 默认值抄 antd：size=medium, ghost=false … */ }
```

规矩：新增参数先查 antd 官网 API 表，有才加；官网有、我们没有的，要么补上，要么单测 `Skip` 写清平台原因（比如浏览器才有的），不许静默少参数。

`type` 是语法糖（抄 `Button.tsx` 的 `ButtonTypeMap`）：`primary=[primary,solid]`、`dashed=[default,dashed]`、`link=[link,link]`、`text=[default,text]`；当 `color/variant` 显式给了，以后者为准。换算只许写在 Theme 段，不许散在 Render 段。

## 3. State（实例，对标 State：现在什么样、怎么变）

```go
// WidgetState 是“当前处境”，不是“配置”：hover/pressed/focused/disabled/loading 全放这。
type WidgetState uint32
const (
    StateHover WidgetState = 1 << iota
    StatePressed
    StateFocused      // 只键盘焦点亮环，鼠标点不亮（:focus-visible）
    StateDisabled     // 全吞事件，焦点环不亮
    StateLoading
)

// ButtonInstance 是一次挂载的活物：Props 快照 + 当前 State + 控制器。
type ButtonInstance struct {
    props ButtonProps
    states WidgetState
    loadingOn time.Time // delay 计时起点
    // …焦点/手势句柄（L2 给的，不自建）
}

func (in *ButtonInstance) Mount(ctx *scope.Ctx)
func (in *ButtonInstance) Update(ctx *scope.Ctx, next ButtonProps) // Props 变了：换快照、重算、标脏
func (in *ButtonInstance) Unmount()
func (in *ButtonInstance) SetState(f func(*WidgetState)) // 唯一改状态的口：改完标脏，重排重画
```

- 状态机（B 类通用，C/D/E 只加不减）：`idle→hover→pressed→idle`；`disabled` 全吞；`loading` 转且防重（点多少次只算一次）；`delay` 未到不转。
- 受控 vs 非受控（C 类）：值有 `Value` + `DefaultValue` + `OnChange` 三件（抄 `FormField`/`Input`）；外面给了 `Value` 就是受控（内部不改，只回调），没给就是非受控（内部存一份）。Render 段只读“当前值”（一个 `Value()` 函数），不问受控非受控。
- 拼写中不调回调（C 类铁律，复核补丁 1）：输入法拼写串（`composing`）只显示不算值，`Value()` 读提交值，`onChange` 只在提交时调一次；计数器、搜索联想、校验全看提交值。
- 下拉键盘（D 类，复核补丁 2）：关着时 Enter/Space/Down 开 → Up/Down 移 → Enter 定 → Esc 关；焦点一直在触发框，选项只高亮不抢焦点。
- 标签收起（C 类，复核补丁 3）：多选标签按 `maxTagCount` 收起 + 省略号，尾标签标收起数，读屏读全量。
- 表格包法（E 类，复核补丁 4）：分页条在外、视口在中、固定列钉视口两侧、虚拟只管体行；横滚头体同滚，固定列不动。
- 弹框宿主（F 类，复核补丁 5）：`App` 级 `ModalHost` 管栈层叠与销毁全部，命令式确认框只调它，不自建浮层栈。
- 校验时机（F 类，复核补丁 6）：`validateTrigger`（改时/失焦/提交）+ 异步防竞态（以后发为准，先到丢弃记日志）。
- 键盘（L2 给）：Tab 进得去、回车空格按得动、Esc 退得出；展示型（Tag、Card）不抢焦点。

## 4. Render（布局绘制，对标 RenderObject：只拼，不画）

```go
// Render 段只干三件事，签名以 ui/rendering 现有接口为准（F0 对齐时钉死）。
type ButtonRenderNode interface {
    Layout(c Constraints) Size   // 约束往下、尺寸往上；只调 L1 排版件
    Paint(p *PaintCtx)           // 只调 L1 绘制件；色值全从 Theme 段 Resolve 来
    HitTest(pt Point) bool       // 只调 L2 手势的命中，不管业务
}
```

- 布局：横排竖排（`Flex`）、叠放（`Stack`）、撑满（`Block`=整宽）全用 L1；`gap="small"`=8 这类数字从主题间距来，不手写。
- 绘制：底、边、字、图标、波纹、转圈，全是 L1 基础件；Render 段里不许出现 `render.New*` 直调，缺能力先补 L1（无例外）。
- 合成：转圈、波纹、骨架闪光标独立重画（对标 `RepaintBoundary`）；静止底不动。loading 整钮 `opacity 0.65` 一次合成（整层合成，字不发虚；逐通道 dim 洗白的不算对）。
- 波纹（抄 `WaveEffect`）：点完才散开（0→6、0.4 秒散、2 秒褪净、初始两成深）；颜色跟边框走、无边跟底色；字按钮链按钮/转圈/禁用/关波纹/省动效 = 无波纹。

## 5. Ctx（上下文，对标 InheritedWidget：显式传）

```go
// Ctx 是“往下流的东西”，Build/Layout/Paint 全透传，不读全局。
// 按 aspect 订阅（对标 InheritedModel）：只刷依赖字段，不是全树重跑。
type Ctx struct {
    Theme    ThemeData      // Seed+Alias+各组件主题（各组件 Theme 段的 Resolve 入口）
    Size     SizeType       // ConfigProvider 尺寸（小/中/大）
    Disabled bool           // 整树禁用（DisabledContext）
    Dir      Direction      // ltr|rtl（图标前后跟着走）
    Motion   MotionConfig   // 动效开/关、省动效（reduced-motion 全局关）
    Locale   string         // 语言（空态文案、日期格式）
    PopupContainer TargetRef // 浮层挂载点（getPopupContainer，默认 body；false=当前位置）
    TargetContainer TargetRef // 滚动监听容器（getTargetContainer，Affix/Anchor/表头sticky共用）
    PopupMatchWidth *bool // 下拉同宽（false 关虚拟，见 Select）
    PopupOverflow OverflowMode // 弹层溢出（viewport/scroll）
    RenderEmpty EmptyFn // 空态自定义（componentName→内容）
    HolderRender HolderFn // 静态调用主题（Message/Modal/Notification静态吃主题）
    Variant VariantType // 全局输入形态（outlined/filled/borderless）
}
```

`ConfigProvider`/`App`（F 类）就是“往 Ctx 里放东西”的组件；产品组件用 `scope.Use(ctx)` 取。换肤、整树禁用、尺寸切换 = 换 Ctx 重跑 `Build`，组件代码不动。

## 6. Theme（主题，对标 ButtonStyle：三级盖+按状态取）

```go
// StateResolver 按当前 WidgetState 取值（对标 WidgetStateProperty）。
type StateResolver[T any] interface { Resolve(s WidgetState) T }

// 组件 Token（举例 button，值抄 antd 6.5.1 种子推导，全写死在 Theme 段，不散在 Render 段）。
type ButtonTheme struct {
    Bg     StateResolver[Color]   // solid/filled/outlined/text/link × 各色 × 各状态
    Border StateResolver[Color]
    Text   StateResolver[Color]
    Radius float64 // 跟全局圆角走
    // …
}

// 三级合并（对标 style > themeStyleOf > defaultStyleOf）：这次 Props > Ctx 组件主题 > 全局种子。
// 合并点在 Resolve 内，调用方只传 Ctx 主题 + Props；ClassNames/Styles 逃生口同级合并，不盖主题结构。
func ResolveButton(st ButtonTheme, p ButtonProps, s WidgetState) ResolvedButton
```

- 受控铁律（全件）：所有 `value/open/current/fileList` 受控只回调不回写，非受控才自改；Select 引用不变不更新，Color 用对象保精度，Upload 不在列表忽略，Tour/Modal/Message 的 key/current/open 外部优先。
- 拼写铁律（全输入框）：Input/Search/TextArea/Select搜索/AutoComplete/Mentions/DatePicker/ColorPicker-hex 组字期显示不同步值，`onChange/onSearch` 只提交调；DatePicker `preserveInvalidOnBlur` 与组字期互斥。
- 键盘铁律（三类）：下拉类复用 Select 顺序（关→开→移→定→关，焦点留触发框）；面板类（日期/颜色/时间）开→方向移→Enter定→Esc关；弹窗类（Modal/Drawer/Tour）Tab圈地+Esc+焦点进出；Message 无键盘不抢焦点。

- 全局种子（`ui/theme`）F0 跟 antd 6.5.1 默认浅色逐项对数（主色/文字/背景/边框、字号、间距、圆角、控件高、内外边距、阴影、遮罩）；悬停/按压派生色跟官网调色板对（`hover=palette[5]、active=palette[7]`，以发布的包为准，不凭记忆）。
- 焦点环全库统一：宽 3、色 `#91caff`、偏 1、只键盘亮、禁用不亮（抄 `genFocusOutline` + `alias.ts`）；各组件只写“圆按钮环是圆的”这类特例。
- 转圈图标：开口圆环、无圆点、走节拍真转。

## 7. 开工清单（按类勾，缺一个不开工）

- A 显示类：Props 表 + 拼装图 + Token 表；无障碍名字；三主题截图。
- B 交互类：A 的全部 + 状态机单测（idle→hover→pressed→idle、禁用全吞、loading 防重）+ 键盘（Tab/回车空格/Esc）+ 焦点环 + 波纹/转圈 + StateResolver 全状态截图。
- C 输入类：B 的全部 + 受控/非受控 + 校验 + 值变化回调 + 输入法占位（暂缓要 Skip 写清）+ 读屏（名字/值/禁用）。
- D 浮层类：触发件（B/C）+ 十二方向/翻转/箭头/外点关/焦点锁/Esc + 浮层内容独立真窗。
- E 结构类：排版拼装 + 长列表虚拟化（只画可见行，滚动帧率门禁）+ 组合联调（Form/Table/分页）。
- F 全局类：Ctx 读写 + 换肤/禁用/尺寸全局生效 + 整树回归。

## 8. Button 骨架（标杆，新 Button 第一个按这个写）

```go
package kit

func BuildButton(ctx *scope.Ctx, p ButtonProps) *ButtonInstance {
    in := &ButtonInstance{props: p}
    // 1. 主题：三级合并出各状态色（Theme 段）。
    res := ResolveButton(ctx.Theme.Button, p, in.states)
    // 2. 拼装：DecoratedBox(底边圆角) > Flex(横排：Icon+Label) + Motion(波纹/转圈)（全 L1/L2）。
    // 3. 接线：Interactive(点按/悬停)+Focus(键盘)+Semantics(名字/角色/状态)（全 L2）。
    _ = res
    return in
}
```

标杆签字看五样：三件套分家了没、主题三级走了没、状态解析用了没、L1/L2 接了没（无直调 render/gpu、无自写定位）、三证据（单测状态机 + 真窗点一遍 + Golden）齐了没。五样缺一样，后面 70 个不开工。

## 9. 测试（沿用三证据，断言对象换成新模子）

- 单测：Props 默认值（跟官网默认值逐项对）、`Resolve` 各状态值（跟官网调色板对）、状态机流转、受控/非受控（C 类）、约束矩阵（三种约束全跑，见 ACCEPTANCE）。
- 真窗：独立窗（`examples/kit/<名>/`，一组件一窗，只摆自己，官网 demo 非 debug 一个不少；段顺序、标题、描述原文、行列、间距跟官网走；高出滚 `RenderViewport`，滚时清悬停防 stuck）；`-auto-only` 按该组件测试说明逐功能验效果（一项一效果，挂一项整窗挂）。
- Golden：`testdata/showcase_<名>.png`（全 variant/color/size/shape 一张摆完）+ 三主题截图；字是真字（有字体字区有墨，无字体宁可空，不画黑条）；容差显式声明。
- 测试数据一律 `testdata/`，临时文件只许 `t.TempDir()`，不许硬编码本机路径、不许循环生成标准数据（AGENTS.md 硬规矩）。

## 10. 旧代码处理（只一句话：删了重写，不迁移）

F0 定稿后，各组件开工第一步是清空 `ui/kit/<名>/*.go`（`testdata/` 旧图仅备查看效果，不做回归基准），按 §1–§8 在 `ui/kit/<名>_*.go` 重建五段；`examples/kit/<名>/` 同理只摆自己。`coverage.go` 与 README 旧看板冻结不再更新，新进度按 `ARCHITECTURE.md` §5 的 F0–F6 另起表。
