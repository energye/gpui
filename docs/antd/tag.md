# Tag 标签
> 来源：[Ant Design 6.5.x Tag](https://ant.design/components/tag)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：进行标记和分类的小标签。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

进行标记和分类的小标签。

**Tag** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 多彩标签 | 复现「多彩标签」视觉与布局 |
| 动态添加和删除 | 复现「动态添加和删除」视觉与布局 |
| 可选择标签 | 复现「可选择标签」视觉与布局 |
| 添加动画 | 复现「添加动画」视觉与布局 |
| 图标按钮 | icon 与文本混排 |
| 预设状态的标签 | 复现「预设状态的标签」视觉与布局 |
| 可拖拽标签 | 复现「可拖拽标签」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `color`

- **说明**：标签色
- **类型**：string
- **默认值**：`variant="solid"` 时为 `default`
- **版本**：`solid` 默认颜色: 6.4.0

#### `disabled`

- **说明**：是否禁用标签
- **类型**：boolean
- **默认值**：false
- **版本**：6.0.0

#### `icon`

- **说明**：设置图标
- **类型**：ReactNode
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `variant`

- **说明**：标签变体
- **类型**：`'filled' | 'solid' | 'outlined'`
- **默认值**：`'filled'`
- **版本**：6.0.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |
  | `solid` | 实心填充 |
  | `outlined` | 描边空心 |

#### `bordered`

- **说明**：是否带边框，请使用 `variant="filled"` 替代
- **类型**：boolean
- **默认值**：true

#### `checked`

- **说明**：设置标签的选中状态
- **类型**：boolean
- **默认值**：false

#### `options`

- **说明**：选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style`
- **类型**：`Array`
- **默认值**：-
- **版本**：`className` 和 `style`: 6.4.0

### 1.4 交互视觉状态（实现检查表）

| 状态 | 要求 |
| --- | --- |
| default | 默认色、边框、阴影符合 token |
| hover | 可交互控件需有悬停反馈 |
| active/pressed | 按下态对比或反馈（若适用） |
| focus | 可见 focus ring，键盘可达 |
| disabled | 降对比 + 禁止交互，布局稳定 |
| loading | 指示器 + 通常阻止重复触发 |
| error/warning | 与 status/Form 语义色一致 |

### 1.5 语义化 DOM 与主题

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 用于标记事物的属性和维度。
- 进行分类。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **多彩标签**（`colorful.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **动态添加和删除**（`control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可选择标签**（`checkable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **添加动画**（`animation.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **图标按钮**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **预设状态的标签**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **可拖拽标签**（`draggable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 选中值 |
| `defaultValue` | 非受控默认值 | 初始选中值 |
| `onChange` | 值变化 | 点击标签时触发的回调 |
| `disabled` | 禁用 | 是否禁用标签 |
| `options` | 数据化 options | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` |
| `checked` | 选中布尔 | 设置标签的选中状态 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 多彩标签 | `colorful.tsx` | 否 |
| 动态添加和删除 | `control.tsx` | 否 |
| 可选择标签 | `checkable.tsx` | 否 |
| 添加动画 | `animation.tsx` | 否 |
| 图标按钮 | `icon.tsx` | 否 |
| 预设状态的标签 | `status.tsx` | 否 |
| 自定义关闭按钮 | `customize.tsx` | 是 |
| 可拖拽标签 | `draggable.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
| 禁用标签 | `disabled.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.7 组合关系

- **Form**：录入类注意 `value`/`checked` 与 `valuePropName`。
- **ConfigProvider**：尺寸、主题、locale、空状态、默认 props。
- **App**：message / modal / notification 上下文。
- **浮层**：Modal/Drawer 内注意 `getPopupContainer`。
- **Space / Flex / Grid / Layout**：布局与间距。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

### Tag

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | closeIcon | 自定义关闭按钮。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | false | 4.4.0 | 5.14.0 |
| color | 标签色 | string | `variant="solid"` 时为 `default` | `solid` 默认颜色: 6.4.0 | × |
| disabled | 是否禁用标签 | boolean | false | 6.0.0 | × |
| href | 点击跳转的地址，指定此属性`tag`组件会渲染成 `<a>` 标签 | string | - | 6.0.0 | × |
| icon | 设置图标 | ReactNode | - | onClose | 关闭时的回调（可通过 `e.preventDefault()` 来阻止默认行为） | (e: React.MouseEvent<HTMLElement, MouseEvent>) => void | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | target | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | 6.0.0 | × |
| variant | 标签变体 | `'filled' \| 'solid' \| 'outlined'` | `'filled'` | 6.0.0 | 6.0.0 |
| ~~bordered~~ | 是否带边框，请使用 `variant="filled"` 替代 | boolean | true | - | × |

### Tag.CheckableTag

| 参数     | 说明                 | 类型              | 默认值 | 版本   |
| -------- | -------------------- | ----------------- | ------ | ------ |
| checked  | 设置标签的选中状态   | boolean           | false  |        |
| icon     | 设置图标             | ReactNode         | -      | 5.27.0 |
| onChange | 点击标签时触发的回调 | (checked) => void | -      |        |

### Tag.CheckableTagGroup

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-group), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-group), string> | - | disabled | 禁用选中 | `boolean` | - | options | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` | `Array<{ className?: string; label: ReactNode; style?: CSSProperties; value: string \| number } \| string \| number>` | - | `className` 和 `style`: 6.4.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-group), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-group), CSSProperties> | - | onChange | 点击标签时触发的回调 | `(value: string \| number \| Array<string \| number> \| null) => void` | - 
```js
import { Tag } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `closeIcon` | 自定义关闭按钮。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | false | 4.4.0 |
| `color` | 标签色 | string | `variant="solid"` 时为 `default` | `solid` 默认颜色: 6.4.0 |
| `disabled` | 是否禁用标签 | boolean | false | 6.0.0 |
| `href` | 点击跳转的地址，指定此属性`tag`组件会渲染成 `` 标签 | string | - | 6.0.0 |
| `icon` | 设置图标 | ReactNode | - | — |
| `onClose` | 关闭时的回调（可通过 `e.preventDefault()` 来阻止默认行为） | (e: React.MouseEvent) => void | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | 6.0.0 |
| `variant` | 标签变体 | `'filled' \| 'solid' \| 'outlined'` | `'filled'` | 6.0.0 |
| `bordered` | 是否带边框，请使用 `variant="filled"` 替代 | boolean | true | - |
| `checked` | 设置标签的选中状态 | boolean | false | — |
| `onChange` | 点击标签时触发的回调 | (checked) => void | - | — |
| `defaultValue` | 初始选中值 | `string \| number \| Array \| null` | - | — |
| `multiple` | 多选模式 | `boolean` | - | — |
| `options` | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` | `Array` | - | `className` 和 `style`: 6.4.0 |
| `value` | 选中值 | `string \| number \| Array \| null` | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Tag** 的验收清单：

1. **配置面**：覆盖 API 表常用字段；冷门字段可分期但命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：官方非 debug 示例约 **9** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tag
- 中文文档：https://ant.design/components/tag-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tag
- 驱动 gpui kit：`tag`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Tag** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/tag/`（`index.zh-CN.md` + `style/index.ts` + `presetCmp.ts` + `statusCmp.ts` + CheckableTag / CheckableTagGroup）。  
> **修订说明（2026-07-25）**：修正 §6.2 误用 controlHeight 档（Tag 无 size 档；高约 22）；拆清 Tag / CheckableTag / CheckableTagGroup 配置；P0 补 `color`/`closable`/`onClose`/`bordered` 遗漏；§6.10 写明 Go 契约。

### 6.1 对齐级别定义（Tag）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 关闭、Checkable 切换、Group 选中、禁用、键盘激活 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 字号/圆角/padding/边线与色皮走 Theme + §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Tag）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例（customize / component-token / disabled 文档页）不计入 P0 验收。  

> 控件说明：进行标记和分类的小标签。

### 6.2 度量与 Design Token（L2 基线）

数值以 **antd Tag `prepareToken` / `prepareComponentToken` + 本库 Theme 默认** 为准（`scale=1`，种子 `fontSize=14`、`fontSizeSM=12`、`borderRadiusSM=4`、`lineWidth=1`）。实现必须通过 Token / Default 常量读取；下表为回落。

#### 6.2.1 几何与组件 Token（Tag **无** size 档）

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 `tagFontSize` | **12** | `fontSizeSM` |
| 行高内容区 | **≈20** | `lineHeightSM × fontSizeSM`（antd `tagLineHeight`） |
| 控件视觉高度 | **≈22** | `height: auto`；demo `control.tsx` 写死 22 对齐 Input small |
| 水平 padding | **7** | `tagPaddingHorizontal(8) − lineWidth` |
| 垂直 padding | **≈1** | 配合行高凑 ~22（实现 Default） |
| 图标尺寸 `tagIconSize` | **≈10–12** | `fontSizeIcon − 2×lineWidth` |
| 图标/文字间距 | **≈7** | 同 `paddingInline` |
| 关闭图标左间距 | **≈3** | `paddingXXS − lineWidth`（≈3） |
| 圆角 | **4** | `borderRadiusSM` |
| 边框线宽 | **1** | `lineWidth` |
| CheckableTagGroup 间距 | **8** | `paddingXS` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见（可交互态） |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token / 来源 | 备注 |
| --- | --- | --- |
| 默认底 `defaultBg` | `colorFillTertiary` on `colorBgContainer` | 约 `#fafafa` / DisabledBg 近似 |
| 默认字 `defaultColor` | `colorText` | |
| 默认边 | `colorBorder` | `outlined` / 默认有边路径 |
| 默认 solid 底 | 近黑 `#000000e0` | `variant=solid` 且无 color |
| solid 字 | `colorTextInverse` | |
| filled 底 | defaultBg | **无边** |
| Checkable 未选 hover | `colorPrimary` 字 + `colorFillSecondary` 底 | |
| Checkable 选中 | `colorPrimary` 底 + inverse 字 | hover → PrimaryHover |
| status success/error/warning | Theme 语义色 + 浅底 | processing → primary |
| 预设色板 blue/red/… | antd PresetColors 表 | 用户显式 `color=` 时使用 |
| 自定义 `#hex` | 解析为 solid/filled/outlined 三套派生 | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |

禁止硬编码品牌色作为 **唯一默认皮**（无 color 时必须走 Theme）。

### 6.3 关键配置与语义

分类：**数据展示**。三层 API（与 antd 一致）：

#### Tag

| 配置 | 说明 | 默认 |
| --- | --- | --- |
| children / label | 标签文案 | — |
| `color` | 预设色名 / status / `#hex` / 空 | 空 |
| `variant` | `filled` \| `solid` \| `outlined` | **`filled`** |
| `closable` / `closeIcon` | 关闭 | false |
| `onClose` | 关闭回调；可 preventDefault 阻止隐藏 | — |
| `icon` | 前导图标 | — |
| `disabled` | 禁用 | false |
| `bordered` | **弃用**：`false` → `variant=filled` | true（历史） |
| `onClick` | 整标签点击 | — |

#### Tag.CheckableTag

| 配置 | 说明 | 默认 |
| --- | --- | --- |
| `checked` | 选中（受控） | false |
| `defaultChecked` | 非受控初值 | false |
| `onChange` | `(checked) => void` | — |
| `icon` | 前导图标 | — |
| `disabled` | 禁用 | false |

#### Tag.CheckableTagGroup

| 配置 | 说明 | 默认 |
| --- | --- | --- |
| `options` | `string` 或 `{label,value,icon?,disabled?}` | — |
| `value` | 受控选中（单/多） | — |
| `defaultValue` | 非受控初值 | — |
| `multiple` | 多选 | false |
| `onChange` | 选中变化 | — |
| `disabled` | 整组禁用 | false |

**配置优先级：** 受控 `value`/`checked` > 显式 `default*` > 组件默认 > ConfigProvider（P1）。

### 6.4 交互状态机（L1）

```text
Tag
  mount ──► visible
    closable 点关 ──► OnClose(e)
                      ├─ e.PreventDefault() ──► 仍可见
                      └─ 默认 ──► Hidden
    disabled ──► 吞 close / click
    OnClick（适用）──► 触发一次

CheckableTag
  mount ──► checked?
    click/Space/Enter ──► toggle ──► OnChange(next)
    disabled ──► 不切换

CheckableTagGroup
  click option ──► 单选：设 value；多选：toggle 成员
                ──► OnChange
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TAG-S1 | `color`=预设/status | 对应色皮 |
| TAG-S2 | closable 点关 | OnClose；默认可隐藏 |
| TAG-S3 | OnClose PreventDefault | 不隐藏 |
| TAG-S4 | Checkable 点击 | 切换 checked + OnChange |
| TAG-S5 | Group 单选/多选 | value / values 正确 |
| TAG-S6 | `bordered=false` 或 `variant=filled` | 无边框 |
| TAG-S7 | `variant=outlined` | 有边 |
| TAG-S8 | `variant=solid` | 实心底 + 反白字 |
| TAG-S9 | icon | 文案前图标 |
| TAG-S10 | 自定义 `#hex` | 派生底/边/字 |
| TAG-S11 | disabled | 禁用色；不触发 close/change |
| TAG-S12 | Checkable 键盘 | Space/Enter 切换 |

### 6.5 视觉 chrome 规则（L2 摘要）

| variant | 默认（无 color） | 有预设/status | 自定义 hex |
| --- | --- | --- | --- |
| `filled` | defaultBg 底、**无边**、defaultColor 字 | light 底、色字、无边 | 浅派生底、色字 |
| `solid` | 近黑底、inverse 字、无边 | dark 底、inverse 字 | 原色底、inverse 字 |
| `outlined` | 浅底、border、defaultColor | light 底、lightBorder、色字 | 浅底 + 色边 + 色字 |

| 态 | 规则 |
| --- | --- |
| default | 上表 |
| hover / active | Checkable / 可点：弱反馈；close icon hover 加深 |
| focus | 可交互：可见 focus ring |
| checked（Checkable） | primary 实心 |
| disabled | 禁用底/字；无 hover |
| processing + spin icon | Icon Ticker 旋转 |

**动效：** 入场/退场（animation.tsx）P0 允许瞬时增删；像素级动画 P1。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 静态 Tag：无强制；Checkable：checkbox；close：button |
| 名称 | 文案即名；close 默认 “Close” |
| 焦点 | Checkable / close / 可点 Tag：Tab 可达；ring 可见 |
| 键盘 | Space/Enter 激活 |
| 禁用 | 不可激活 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 关闭 / Checkable / Group / color / variant / icon / disabled | **对等** | P0 L1 |
| 几何 §6.2 / 默认皮 Token | **对等** | P0 L2 |
| 预设全色板 + status 三 variant | **对等** | P0 |
| `href`/`target` 真导航 | **映射** OnClick | P1 |
| motion 入场退场像素 | **瞬时** 或 P1 | P1 |
| dnd-kit 拖拽库 | **组合** gallery 重排示意 | P0 示意 / P1 完整 |
| semantic classNames/styles | kit 浅钩子 | P1 |
| ConfigProvider 全局 tag | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| Tag：`label`/`value`、`color`、`variant`、`closable`/`onClose`、`icon`、`disabled`、`bordered`（兼容） | 主标签 |
| CheckableTag：`checked`/`defaultChecked`/`onChange`/`icon`/`disabled` | 可选择 |
| CheckableTagGroup：`options`/`value`/`defaultValue`/`multiple`/`onChange`/`disabled` | 组选 |
| 官方主路径示例 | 基本、多彩、动态添加删除、可选择、添加动画（瞬时）、图标、预设状态、可拖拽（组合示意） |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 / style-class demo | 分期 |
| href/target 真链接、ConfigProvider 全局 | 分期 |
| motion 入场退场像素级、完整 dnd-kit | 分期 |
| debug：customize / component-token / disabled 文档页 | 分期 |
| 官网逐像素哈希 | 不做 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTag_PRD_<ID>`。  
> **P0 相关用例（TAG-01–TAG-19）全部通过** 才可宣称 Tag 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TAG-01 | L1 | NewTag 默认创建 | variant=filled；非 closable；非 disabled |
| TAG-02 | L1 | color=red/预设 | 色皮非默认灰 |
| TAG-03 | L1 | closable 关闭 | OnClose；默认 Hidden |
| TAG-04 | L1 | Checkable 切换 | OnChange；checked 翻转 |
| TAG-05 | L1 | bordered=false 或 variant=filled | 无边（BorderWidth=0） |
| TAG-06 | L1 | icon | 前图标存在 |
| TAG-07 | L1 | 自定义 `#f50` | 背景/边/字派生 |
| TAG-08 | L1 | 复现 basic.tsx | 普通/closable 路径可建 |
| TAG-09 | L1 | 复现 colorful.tsx | 预设 × 三 variant + 自定义色可建 |
| TAG-10 | L1 | 复现 control.tsx | 动态增删标签 |
| TAG-11 | L1 | 复现 checkable.tsx | Checkable + Group 单/多选 |
| TAG-12 | L1 | 复现 animation.tsx | 增删（瞬时可） |
| TAG-13 | L1 | 复现 icon.tsx | Tag/Checkable 带 icon |
| TAG-14 | L1 | 复现 status.tsx | status × variant |
| TAG-15 | L1 | 复现 draggable.tsx | 列表可重排（组合） |
| TAG-16 | L2 | 读取 §6.2 关键尺寸 | font≈12、radius≈4、padH≈7、高≈20–24 |
| TAG-17 | L2 | 默认皮颜色 | 走 Theme；无硬编码品牌色当默认 |
| TAG-18 | L2 | disabled 外观 | 禁用色；不触发 close/change |
| TAG-19 | L1 | Checkable 键盘 | Focus ring；Space/Enter 切换 |
| TAG-20 | L3 | 关键态 golden | 基线（可后补） |
| TAG-21 | L4 | 与 ant.design 并排 | 人眼签字 |
| TAG-22 | P1 | §6.8 P1 任一能力 | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API。

```text
// --- Tag ---
NewTag(label string) *Tag
SetLabel / SetValue(string)
SetColor(nameOrHex string)           // 预设 | status | #hex | ""
SetColorRGBA(render.RGBA)
SetVariant(TagVariant)               // filled|solid|outlined
SetBordered(bool)                    // 弃用兼容
SetClosable(bool)
SetCloseIcon(core.Node)
SetIcon(name string) / SetIconNode(core.Node)
SetDisabled(bool)
SetOnClick(func())
OnClose func(*TagCloseEvent)         // PreventDefault 阻止隐藏
SetTheme / SetFace / SetStyle / SetAriaLabel
Node() / ChromeNode() / CloseNode()
Hidden bool / Visible()

// --- CheckableTag ---
NewCheckableTag(label string) *CheckableTag
SetChecked / SetDefaultChecked / Checked()
SetOnChange(func(bool))
SetIcon / SetIconNode / SetDisabled / SetLabel
Node() / ChromeNode()

// --- CheckableTagGroup ---
NewCheckableTagGroup(opts ...TagOption) *CheckableTagGroup
// TagOption{ Label, Value, Icon, IconNode, Disabled }
SetOptions / SetMultiple
SetValue(string) / SetValues([]string)
SetDefaultValue / SetDefaultValues
Value() / Values()
SetOnChange(func(string))
SetOnChangeMulti(func([]string))
SetDisabled
Node()
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Variant | filled |
| Closable / Disabled / Hidden / Checked | false |
| Color | 空（默认皮） |
| Group Multiple | false |

### 6.11 结构与绘制分层（实现提示）

```text
Tag:
  [Pressable?]
    └─ Decorated chrome
         └─ Row(Icon? · Label · ClosePressable?)

CheckableTag:
  Pressable
    └─ Decorated
         └─ Row(Icon? · Label)

CheckableTagGroup:
  Flex wrap gap=paddingXS
    └─ CheckableTag × N
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- processing spin 用 Icon Ticker。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Tag 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（TAG-01–TAG-19）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（可后补）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：覆盖 **§6.8 P0** 主路径。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/tag.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Tag 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
