# antd 组件库需求架构（Flutter 对齐版）

> 地位：组件库新架构唯一真源。后附实现模型见 [`WIDGET_MODEL.md`](./WIDGET_MODEL.md)。
> 版本：v2（2026-09-20，flat kit：一组件可多文件同属 package kit，导出名带前缀）｜ 产品依据：antd **v6.5.1**（`components/*/index.zh-CN.md`）｜ 引擎依据：[`ENGINE_FLUTTER_SKIA_ARCH.md`](../ENGINE_FLUTTER_SKIA_ARCH.md)
>
> **推翻声明**：`ui/kit` 下现有全部组件实现作废，不作为新架构的依据，也不做迁移参照。
> 各 `docs/antd/<名>.md` 的 **§1 外观 / §2 功能 / §3 API** 保留为产品真值（照着 antd 官网抄的，长什么样、有哪些参数，以它们为准）；
> 各文档 **§6 实现要点/验收清单（旧 DoD、旧 P0 用例号、旧 Go 签名、旧波次备注）全部冻结作废**，后续按本文 + `WIDGET_MODEL.md` 重写 §6。
> `docs/antd/README.md` 看板与 `ui/kit/coverage.go` 的旧状态（首批完/重写中）同样冻结，仅备查，不指导开工。

## 0. 一句话：新架构长什么样

Flutter 做组件库的核心就三句话，后面所有 antd 组件都照这三句做：

1. **三件套分家**：`Widget（配置）` / `Element+State（实例和状态）` / `RenderObject（布局和绘制）` 各干各的，不许混在一起。
   出处：`flutter/packages/flutter/lib/src/widgets/framework.dart`（`Widget` / `StatelessWidget` / `StatefulWidget` / `State` / `RenderObjectWidget`），`rendering/object.dart` + `box.dart`（`RenderObject` / `RenderBox`）。
2. **小件拼大件**：大组件不自己画，全用基础小件拼出来（比如按钮 = 外框 + 横排 + 图标 + 文字 + 水波）。
   出处：`widgets/basic.dart`（`Row`/`Column`/`Stack`/`Container`），`material/button_style_button.dart`（`ButtonStyleButton` 拼 `Material` + `InkWell` + `Row`）。
3. **主题往下流**：颜色字号圆角全从主题往下传，组件只写“没给就用主题的”，状态不同值不同（悬停一个色、按下一个色、禁用一个色）。
   出处：`material/theme.dart`（`Theme.of(context)`），`material/button_style.dart`（`ButtonStyle` + `WidgetStateProperty` 按状态取值，三级合并：自己写的 > 主题里的 > 默认的）。

Go 里没有继承和 JSX，所以映射成这样（细节全在 `WIDGET_MODEL.md`）：

| Flutter | gpui 新模型（Go） | 放哪 |
| --- | --- | --- |
| `Widget`（不可变配置） | `<Comp>Props` 结构体（值拷贝，不许改，如 `ButtonProps`） | `ui/kit/button_*.go`（package kit，一个组件可多个文件，文件名带前缀） |
| `Element` + `State`（实例、生命周期） | `<Comp>Instance` + `Mount/Update/Unmount` + `SetState` | 同上 State 段 |
| `RenderObject`（布局绘制） | Render 段（`Layout` / `Paint` / `HitTest`，只拼基础件） | 同上 Render 段 |
| `InheritedWidget`（主题下传） | `Ctx` 结构体（主题、尺寸、禁用、方向、动效、语言、挂载点、空态、静态配置，显式传参） | `ui/kit/internal/scope` + 各组件 Theme 段（L3） |
| `build()`（组合点） | `Build<Comp>(ctx, Props)`（如 `BuildButton`） | 同上 Build 段 |
| `ButtonStyle` 三级合并 | `Resolve<Comp>` 函数（自己写的 > 组件主题 > 全局种子） | 同上 Theme 段 |
| `WidgetStateProperty`（按状态取值） | `StateResolver[T]`（按 `hover/pressed/focused/disabled/loading` 取值） | `ui/kit/internal/scope` |
| `Row/Column/Stack/Container/Text/Icon` | 基础件门面（包住现有 `ui/rendering`，不写新 GPU 代码） | `ui/kit/internal/prim` |
| 手势/焦点/浮层/动效/语义 | 行为件（包住现有 `ui/gestures`、`ui/focus`、`ui/overlay`、`ui/animation`、`ui/scheduler` + 语义包） | `ui/kit/internal/behavior` |

为什么旧实现必须扔：旧 `ui/kit/button/button.go` 是一个文件里又管参数、又管悬停按压状态机、又直接调 `render` 画、又自己算颜色，一共几千行；
Flutter 要求这四件事分属四个地方。旧代码不是“写得不好”，是“分家方式”从根上就不对，修不好，只能按新模子重写。

## 1. 分层（只许往下调，不许往上、跨层、返头）

```text
L0 主题地基：种子 Token → 别名 Token → 组件 Token规范（只住 ui/theme；各组件 Theme 段归 L3，实现本规范）
        ↓ 只许往下用（import 只许往下；Ctx 是运行时 L4→L3 数据流，L3 只 import scope 读，不算返头）
L1 基础件：Box / Flex(横排竖排) / Stack / Text / Icon / Image / DecoratedBox(底色边框圆角阴影) / Clip / Opacity …（ui/kit/internal/prim，包 ui/rendering）
        ↓
L2 行为件：可交互（悬停按压焦点禁用加载） / 表单字段（值+校验+受控） / 浮层触发（定位+外点关+焦点锁） / 动效（显隐动画+节拍器）（ui/kit/internal/behavior，包 ui/gestures、ui/focus、ui/overlay、ui/animation、ui/scheduler）
        ↓
L3 产品组件：70+ 个 antd 组件（ui/kit/button_*.go、alert_*.go…，同属 package kit，一个组件可多个文件，只许拼 L1+L2，不许直调 render/gpu）
        ↓
L4 组合与全局：Form+Table+Modal 联动、App、ConfigProvider（ui/kit/app_*.go、config_provider_*.go…，同属 package kit，往 Ctx 里放东西）
```

硬规矩（违反即打回）：

- 窗口/事件/资源归引擎层（`ui/embedder` + 窗侧 shell）：窗口创建、事件循环、事件分发、后台冻结、资源释放全归引擎；kit 只收回调（点按/悬停/焦点/值变），`Mount/Update/Unmount` 由窗侧调（见 `WIDGET_MODEL.md` §3），组件不自建窗口、不自起常驻 goroutine。

- 产品组件（L3）**禁止** `import gpu`，**禁止**直调 `render` 画布，只能 import `prim/behavior/scope` + `ui/theme`；`render` 公开 API 只许 `prim` 调，L3 要句柄走 `prim` 转调。
- 产品组件**禁止**自写布局、手势识别、焦点管理、浮层定位算法，一律调 L1/L2；不够用先补 L1/L2，再给产品用（能力在实际所在位置实现，见 AGENTS.md）。
- 引擎层（`ui/*`、`render`、`gpu`）**禁止**写死用户文案、业务色、中文空格这类东西；默认值走主题或 Props，见 AGENTS.md 引擎硬编码纪律。
- 用户文案、颜色、尺寸一律 `Props` 传进来或 `Ctx` 主题里取，不许在 Render 段里写死。

## 2. 主题（三级，和 Flutter/antd 同构）

Flutter 是 `ThemeData（全局） → ButtonTheme（组件） → Button.style（这次调用）` 三级，后者盖住前者；
antd 是 `Seed（种子） → Alias（别名） → Component（组件 Token）` 三级（`components/theme/`）。
新架构照抄这三级：

```text
Seed（种子，原样抄 antd 6.5.1 默认浅色）：主色 #1677ff、成功/告警/错误色、文字/背景/边框层级、字号、间距、圆角、控件高、阴影、遮罩
  → Alias（别名，派生）：悬停色、按压色、焦点环（宽3、色#91caff、偏1，只键盘焦点亮）、透明度（loading 整层 0.65 一次合成）
  → Component（组件 Token，各组件 Theme 段）：比如 button 的各 variant/color 在各状态下的底/边/字色
```

- 取值顺序永远是：**这次调用的 Props > Ctx 里的组件主题 > 全局种子默认值**（对标 `ButtonStyle` 的 `style > themeStyleOf > defaultStyleOf`）。整树禁用与单组件禁用是或关系（见 `WIDGET_MODEL.md` §6 禁用铁律），尺寸/形态/动效是显式优先（同上）。
- 状态相关的值全用 `StateResolver[T]` 存（比如“底色”不是一个颜色，是“默认什么色、悬停什么色、按下什么色、禁用什么色”一张表），Render 段只许 `Resolve(当前状态)`，不许写 `if hover … else …` 散装判断。
- `ConfigProvider` 对标 Flutter 的 `InheritedWidget`（`SizeContext`、`DisabledContext`、`ConfigContext` 在 `components/config-provider/`）：主题、尺寸、禁用、方向、动效、语言、挂载点（popup/target容器）、同宽溢出（popupMatch/Overflow）、空态（renderEmpty）、静态配置（holderRender）、形态变体（variant），全部放进 `Ctx` 往下传，子组件用 `scope.Use(ctx)` 取，不许读全局变量。L2 以本节5件（gestures/focus/overlay/animation/scheduler）为基准，textinput/textbuffer/io 走 Field/图片管线（见 F0），不再各自枚举。
- 深色、紧凑、品牌色换肤 = 换一套 Seed/Component Token 重跑 `Resolve`，组件代码不动。

## 3. 布局、绘制、合成、手势、焦点、语义、动效、浮层（每行一句，各归其位）

| 能力 | Flutter 样子 | gpui 落点 | 产品组件只做 |
| --- | --- | --- | --- |
| 布局 | 约束往下、尺寸往上（`BoxConstraints`）；`Flex`/`Stack` 管排版 | `ui/rendering` 的约束与排版（已有），L1 包一层门面 | 选横排还是竖排、间距多大，不自己算坐标 |
| 绘制 | `RenderObject.paint` 经 `PaintingContext` 录制，不直触 GPU | `render` 画布（Skia 式），经 L1 调用 | 拼基础件，不碰画布 |
| 合成 | `Layer` 树 + `RepaintBoundary`（动画部分独立重画） | `ui/scene` 的 Layer（已有） | 给转圈、波纹、骨架闪光标“独立重画”，静止部分不动 |
| 手势 | `GestureDetector`（点按、长按、悬停、拖） | `ui/gestures`，L2 包一层 | 声明“点一下干什么、悬停干什么”，不自己算命中 |
| 焦点键盘 | `FocusNode/FocusScope`，Tab 进、回车空格式激活、Esc 退 | `ui/focus`，L2 包一层 | 声明“能不能聚焦、键怎么反应”，焦点环用统一数字 |
| 语义无障碍 | `Semantics` 树（名字、角色、状态给读屏） | 语义包（现有语义/新增，引擎侧补） | 每个可交互件给名字、角色、禁用/选中状态 |
| 动效 | 显式（转圈、波纹，`Ticker` 驱动）+ 隐式（悬停淡入淡出，`Animated*`） | `ui/animation` + `ui/scheduler` 节拍，L2 包一层 | 点完才有波纹（0→6、0.4 秒散、2 秒褪净）、转圈走节拍真转、尊重省动效开关 |
| 浮层 | `Overlay` + 十二方向定位 + 翻转 + 箭头 + 外点关 + 焦点锁 | `ui/overlay`（唯一实现，各组件只调） | 选方向、给内容，不管定位算法 |

## 4. 组件分类（70+ 个一次归位，后面按类套模子）

分类只看“行为像谁”，不看 antd 原来的“通用/布局/导航”分组（那是文档目录，不是实现模型）：

| 类 | 像 Flutter 的谁 | 模子 | 成员（antd 名） |
| --- | --- | --- | --- |
| A 基础显示 | `StatelessWidget` 为主 | Props + 纯拼装，无状态或只读状态 | Typography、Divider、Tag、Badge、Avatar、Card、Empty、Result、Statistic、Descriptions、Timeline、QRCode、Image（展示态）、Alert、Icon（L1 图标） |
| B 基础交互 | `<Comp>Instance` + `StateResolver` | Props + 实例（hover/pressed/focus/disabled/loading）+ 状态查表 | Button、Switch、Checkbox、Radio、Rate、Slider、Segmented、FloatButton |
| C 输入受控 | `FormField` + `Controller` | B 的全部 + 值/校验/受控非受控/输入法 | Input（含搜索/密码/多行/OTP）、InputNumber、Select、AutoComplete、Mentions、Cascader、TreeSelect、DatePicker、TimePicker、ColorPicker、Upload |
| D 浮层触发 | `Overlay` + `FocusTrap` | B/C 的触发件 + L2 浮层触发器（定位/外点关/焦点锁/Esc） | Tooltip、Popover、Popconfirm、Dropdown、Menu（弹出态）、Modal、Drawer、Message、Notification、Tour |
| E 结构导航 | `Flex`/`Sliver`/`Scroll` | L1 排版件的组合 + 滚动/虚拟化宿主（长列表只画可见行） | Layout、Grid、Flex、Space、Splitter、Affix、Anchor、Breadcrumb、Pagination、Steps、Tabs、Calendar、Table、List、Tree、Transfer、Collapse、Carousel、Spin（容器态）、Skeleton（容器态）、Watermark、Progress、Masonry |
| F 全局作用域 | `InheritedWidget` | 往 Ctx 放东西，不画东西 | ConfigProvider、App、Theme（种子切换）、Locale |

每个类在 `WIDGET_MODEL.md` 里各有一个清单（必须接哪些 L1/L2、状态有哪些、测试重点看什么），开工照单抓药，不许自创。

## 5. 依赖与开工顺序（新波次，旧 W0–W5 作废）

旧 W0–W5 是按“谁被谁复用”排的，不管“地基稳不稳”，这正是返工的根。新波次按 Flutter 地基顺序排，前一波不绿，后一波不开工：

| 波 | 做什么 | 内容 | 绿的标准 |
| --- | --- | --- | --- |
| F0 | 主题与范围 | Seed/Alias 全量对数（跟 antd 6.5.1 种子逐项对）、`Ctx` + `StateResolver` + L1/L2 空架子先立 + G1–G7 七个大件并行单独立项（见 F0 §6.5，不挡小组件） | 主题单测全绿（种子值与官网一致），空架子可 `import` |
| F1 | 基础显示 A | Icon（最底层，多方引用）→ Typography/Divider/Tag/Badge/Avatar/Card/Empty/Result… | 每个有独立真窗（只摆自己），截图 Golden 入库 |
| F2 | 基础交互 B | Button 先单做签字定标杆 → Switch/Checkbox/Radio/Rate/Slider/Segmented/FloatButton | Button 官网并排人眼签字，其余状态机单测 + 真窗点一遍 |
| F3 | 输入受控 C | Input/Select 先（被复用最多）→ 其余输入件 | 受控/非受控/校验/键盘/输入法逐项真测 |
| F4 | 浮层触发 D | Tooltip 先（定位底座验证）→ Popover/Dropdown/Menu/Modal/Drawer/Message/Notification/Tour | 十二方向/翻转/外点关/焦点锁/Esc 逐项真测 |
| F5 | 结构导航 E | 布局排版先 → Table/List/Tree/Transfer（虚拟化）→ Calendar/Form 联调 | 长列表只画可见行（滚动帧率门禁），Form 联调全链路 |
| F6 | 全局 F | ConfigProvider/App/Theme 切换/Locale | 换肤、整树禁用、尺寸切换一处改全局生效 |

Button 仍是第一个标杆（和原来一样），但标杆的内容变了：原来只验“按钮像不像”，现在要验“**三件套 + 主题三级 + 状态解析 + L1/L2 接线**”这套模子行不行。模子不行，后面 70 个全返工，所以 F2 的 Button 必须先签字。

## 6. 文档结构（以后每个组件文档都长这样）

- §1 外观、§2 功能、§3 API：保持现状，照 antd 官网抄，是产品真值，不动。
- §6 实现：冻结范围只限工程部分——旧 Go 签名（New+Set 系）、旧用例号（BTN-/FRM- 系）、旧分层（§6.1/§6.11 L1–L4）、旧 gallery 路径（`examples/ui_polish_gallery`）、§2.7 子目录写法（`ui/kit/<名>/`，一律视为 flat 前缀写法，以 `WIDGET_MODEL.md` §1 为准）、§4 旧纪律（Default+Set+rebuild）；§6 的产品增量（度量/状态/Token/平台边界）继续有效。开工某组件前先按 `WIDGET_MODEL.md` §7 新模板重写其 §6 工程部分，重写完才开工。
- §6 新模板只写四件事——Props 表（哪些参数、默认值、走哪个 Token）、State 表（哪些状态、谁驱动）、拼装图（用了哪几个 L1/L2）、Token 表（组件 Token 名和值）。不写用例号、不写旧 Go 签名（新签名按 `<Comp>Props/Instance/Build<Comp>` 模板生成）。
- 看板：`README.md` 的旧进度表冻结；新进度唯一认 `PROGRESS.md`（每步状态+源码证据+验证 commit），本文 §5 只写顺序不写状态。

## 7. 验收方向（三证据不变，门禁要按新模型改）

像素三证据（逻辑探针 + 像素断言 + Golden）不变，见 `ACCEPTANCE.md` 与 `UI_PIXEL_ASSERTION_STANDARD.md`。
但三道机器门禁（中央/画廊/独立真窗）是按旧 §6（P0 用例号、旧签名）写的，新模型落地后必须改成查四件事：Props 是否全覆盖、State 是否全流转、是否只调 L1/L2（禁直调 render/gpu、禁自写定位）、Token 是否全走主题。门禁改完之前，F0–F1 不开工（否则一边写一边改门禁，必返工）。
