# Slider 滑动输入条
> 来源：[Ant Design 6.5.x Slider](https://ant.design/components/slider)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：滑动型输入器，展示当前值和可选范围。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

滑动型输入器，展示当前值和可选范围。

**Slider** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 带输入框的滑块 | 复现「带输入框的滑块」视觉与布局 |
| 带 icon 的滑块 | 复现「带 icon 的滑块」视觉与布局 |
| 自定义提示 | 自定义渲染/插槽外观 |
| 事件 | 复现「事件」视觉与布局 |
| 带标签的滑块 | 复现「带标签的滑块」视觉与布局 |
| 垂直 | 纵向布局 |
| 控制 ToolTip 的显示 | 复现「控制 ToolTip 的显示」视觉与布局 |
| 反向 | 复现「反向」视觉与布局 |
| 范围可拖拽 | 复现「范围可拖拽」视觉与布局 |
| 多点组合 | 复现「多点组合」视觉与布局 |
| 动态增减节点 | 复现「动态增减节点」视觉与布局 |
| 禁用指定滑块 | disabled 灰态与不可点 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：值为 true 时，滑块为禁用状态。该属性也可以是一个数组，用于禁用 range 模式下特定的 handle，例如 `[true, false, true]` 会禁用第一个和第三个 handle。当任意 handle 被禁用时，`editable` 模式将自动禁用
- **类型**：boolean | boolean[]
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `editable` | 官方取值 `editable` |

#### `keyboard`

- **说明**：支持使用键盘操作 handler
- **类型**：boolean
- **默认值**：true
- **版本**：5.2.0+

#### `marks`

- **说明**：刻度标记，key 的类型必须为 `number` 且取值在闭区间 \[min, max] 内，每个标签可以单独设置样式
- **类型**：object
- **默认值**：{ number: ReactNode } or { number: { style: CSSProperties, label: ReactNode } }

#### `max`

- **说明**：最大值
- **类型**：number
- **默认值**：100

#### `orientation`

- **说明**：排列方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `vertical`

- **说明**：值为 true 时，Slider 为垂直方向。与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `handleStyle`

- **说明**：滑块手柄样式，请使用 `styles.handle` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `railStyle`

- **说明**：滑轨样式，请使用 `styles.rail` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `trackStyle`

- **说明**：已选轨道样式，请使用 `styles.track` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `maxCount`

- **说明**：配置 `editable` 时，最大节点数量
- **类型**：number
- **默认值**：-
- **版本**：5.20.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `editable` | 官方取值 `editable` |

#### `autoAdjustOverflow`

- **说明**：是否自动调整弹出位置
- **类型**：boolean
- **默认值**：true
- **版本**：5.8.0

#### `placement`

- **说明**：设置 Tooltip 展示位置。参考 [Tooltip](/components/tooltip-cn)
- **类型**：string
- **默认值**：-
- **版本**：4.23.0

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

当用户需要在数值区间/自定义区间内进行选择时，可为连续或离散值。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带输入框的滑块**（`input-number.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **带 icon 的滑块**（`icon-slider.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义提示**（`tip-formatter.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **事件**（`event.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **带标签的滑块**（`mark.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **垂直**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **控制 ToolTip 的显示**（`show-tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **反向**（`reverse.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **范围可拖拽**（`draggableTrack.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **多点组合**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **动态增减节点**（`editable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **禁用指定滑块**（`disabled-handle.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 设置当前取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] |
| `defaultValue` | 非受控默认值 | 设置初始取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] |
| `onChange` | 值变化 | 当 Slider 的值发生改变时，会触发 onChange 事件，并把改变后的值作为参数传入 |
| `open` | 受控显隐 | 值为 true 时，Tooltip 将会始终显示；否则始终不显示，哪怕在拖拽及移入时 |
| `disabled` | 禁用 | 值为 true 时，滑块为禁用状态。该属性也可以是一个数组，用于禁用 range 模式下特定的 handle，例如 `[true, false, true]` 会禁用第一个和第三个 handle。当任意 handle 被禁用时，`editable` 模式将自动禁用 |
| `getPopupContainer` | 浮层容器 | Tooltip 渲染父节点，默认渲染到 body 上 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 带输入框的滑块 | `input-number.tsx` | 否 |
| 带 icon 的滑块 | `icon-slider.tsx` | 否 |
| 自定义提示 | `tip-formatter.tsx` | 否 |
| 事件 | `event.tsx` | 否 |
| 带标签的滑块 | `mark.tsx` | 否 |
| 垂直 | `vertical.tsx` | 否 |
| 控制 ToolTip 的显示 | `show-tooltip.tsx` | 否 |
| 反向 | `reverse.tsx` | 否 |
| 范围可拖拽 | `draggableTrack.tsx` | 否 |
| 多点组合 | `multiple.tsx` | 否 |
| 动态增减节点 | `editable.tsx` | 否 |
| 禁用指定滑块 | `disabled-handle.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 设置初始取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \| \[number, number] | 0 \| \[0, 0] | disabled | 值为 true 时，滑块为禁用状态。该属性也可以是一个数组，用于禁用 range 模式下特定的 handle，例如 `[true, false, true]` 会禁用第一个和第三个 handle。当任意 handle 被禁用时，`editable` 模式将自动禁用 | boolean \| boolean[] | false | keyboard | 支持使用键盘操作 handler | boolean | true | 5.2.0+ | × |
| dots | 是否只能拖拽到刻度上 | boolean | false | included | `marks` 不为空对象时有效，值为 true 时表示值为包含关系，false 表示并列 | boolean | true | marks | 刻度标记，key 的类型必须为 `number` 且取值在闭区间 \[min, max] 内，每个标签可以单独设置样式 | object | { number: ReactNode } or { number: { style: CSSProperties, label: ReactNode } } | max | 最大值 | number | 100 | min | 最小值 | number | 0 | orientation | 排列方向 | `horizontal` \| `vertical` | `horizontal` | range | 双滑块模式 | boolean \| [range](#range) | false | reverse | 反向坐标轴 | boolean | false | step | 步长，取值必须大于 0，并且可被 (max - min) 整除。当 `marks` 不为空对象时，可以设置 `step` 为 null，此时 Slider 的可选值仅有 `marks`、`min` 和 `max` | number \| null | 1 | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | tooltip | 设置 Tooltip 相关属性 | [tooltip](#tooltip) | - | 4.23.0 | × |
| value | 设置当前取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \| \[number, number] | - | vertical | 值为 true 时，Slider 为垂直方向。与 `orientation` 同时存在，以 `orientation` 优先 | boolean | false | onChangeComplete | 与 `mouseup` 和 `keyup` 触发时机一致，把当前值作为参数传入 | (value) => void | - | onChange | 当 Slider 的值发生改变时，会触发 onChange 事件，并把改变后的值作为参数传入 | (value) => void | - | ~~handleStyle~~ | 滑块手柄样式，请使用 `styles.handle` 替代 | CSSProperties | - | - | × |
| ~~onAfterChange~~ | 与 `mouseup` 和 `keyup` 触发时机一致，请使用 `onChangeComplete` 替代 | (value) => void | - | - | × |
| ~~railStyle~~ | 滑轨样式，请使用 `styles.rail` 替代 | CSSProperties | - | - | × |
| ~~trackStyle~~ | 已选轨道样式，请使用 `styles.track` 替代 | CSSProperties | - | - | × |

### range

| 参数           | 说明                                               | 类型    | 默认值 | 版本   |
| -------------- | -------------------------------------------------- | ------- | ------ | ------ |
| draggableTrack | 范围刻度是否可被拖拽                               | boolean | false  |        |
| editable       | 启动动态增减节点，不能和 `draggableTrack` 一同使用 | boolean | false  | 5.20.0 |
| minCount       | 配置 `editable` 时，最小节点数量                   | number  | 0      | 5.20.0 |
| maxCount       | 配置 `editable` 时，最大节点数量                   | number  | -      | 5.20.0 |

### tooltip

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| autoAdjustOverflow | 是否自动调整弹出位置 | boolean | true | 5.8.0 |
| open | 值为 true 时，Tooltip 将会始终显示；否则始终不显示，哪怕在拖拽及移入时 | boolean | - | 4.23.0 |
| placement | 设置 Tooltip 展示位置。参考 [Tooltip](/components/tooltip-cn) | string | - | 4.23.0 |
| getPopupContainer | Tooltip 渲染父节点，默认渲染到 body 上 | (triggerNode) => HTMLElement | () => document.body | 4.23.0 |
| formatter | Slider 会把当前值传给 `formatter`，并在 Tooltip 中显示 `formatter` 的返回值，若为 null，则隐藏 Tooltip | value => ReactNode \| null | IDENTITY | 4.23.0 |

## 方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 导入方式

```js
import { Slider } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 设置初始取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \| \[number, number] | 0 \| \[0, 0] | — |
| `disabled` | 值为 true 时，滑块为禁用状态。该属性也可以是一个数组，用于禁用 range 模式下特定的 handle，例如 `[true, false, true]` 会禁用第一个和第三个 handle。当任意 handle 被禁用时，`editable` 模式将自动禁用 | boolean \| boolean[] | false | — |
| `keyboard` | 支持使用键盘操作 handler | boolean | true | 5.2.0+ |
| `dots` | 是否只能拖拽到刻度上 | boolean | false | — |
| `included` | `marks` 不为空对象时有效，值为 true 时表示值为包含关系，false 表示并列 | boolean | true | — |
| `marks` | 刻度标记，key 的类型必须为 `number` 且取值在闭区间 \[min, max] 内，每个标签可以单独设置样式 | object | { number: ReactNode } or { number: { style: CSSProperties, label: ReactNode } } | — |
| `max` | 最大值 | number | 100 | — |
| `min` | 最小值 | number | 0 | — |
| `orientation` | 排列方向 | `horizontal` \| `vertical` | `horizontal` | — |
| `range` | 双滑块模式 | boolean \| [range](#range) | false | — |
| `reverse` | 反向坐标轴 | boolean | false | — |
| `step` | 步长，取值必须大于 0，并且可被 (max - min) 整除。当 `marks` 不为空对象时，可以设置 `step` 为 null，此时 Slider 的可选值仅有 `marks`、`min` 和 `max` | number \| null | 1 | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `tooltip` | 设置 Tooltip 相关属性 | [tooltip](#tooltip) | - | 4.23.0 |
| `value` | 设置当前取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \| \[number, number] | - | — |
| `vertical` | 值为 true 时，Slider 为垂直方向。与 `orientation` 同时存在，以 `orientation` 优先 | boolean | false | — |
| `onChangeComplete` | 与 `mouseup` 和 `keyup` 触发时机一致，把当前值作为参数传入 | (value) => void | - | — |
| `onChange` | 当 Slider 的值发生改变时，会触发 onChange 事件，并把改变后的值作为参数传入 | (value) => void | - | — |
| `handleStyle` | 滑块手柄样式，请使用 `styles.handle` 替代 | CSSProperties | - | - |
| `onAfterChange` | 与 `mouseup` 和 `keyup` 触发时机一致，请使用 `onChangeComplete` 替代 | (value) => void | - | - |
| `railStyle` | 滑轨样式，请使用 `styles.rail` 替代 | CSSProperties | - | - |
| `trackStyle` | 已选轨道样式，请使用 `styles.track` 替代 | CSSProperties | - | - |
| `draggableTrack` | 范围刻度是否可被拖拽 | boolean | false | — |
| `editable` | 启动动态增减节点，不能和 `draggableTrack` 一同使用 | boolean | false | 5.20.0 |
| `minCount` | 配置 `editable` 时，最小节点数量 | number | 0 | 5.20.0 |
| `maxCount` | 配置 `editable` 时，最大节点数量 | number | - | 5.20.0 |
| `autoAdjustOverflow` | 是否自动调整弹出位置 | boolean | true | 5.8.0 |
| `open` | 值为 true 时，Tooltip 将会始终显示；否则始终不显示，哪怕在拖拽及移入时 | boolean | - | 4.23.0 |
| `placement` | 设置 Tooltip 展示位置。参考 [Tooltip](/components/tooltip-cn) | string | - | 4.23.0 |
| `getPopupContainer` | Tooltip 渲染父节点，默认渲染到 body 上 | (triggerNode) => HTMLElement | () => document.body | 4.23.0 |
| `formatter` | Slider 会把当前值传给 `formatter`，并在 Tooltip 中显示 `formatter` 的返回值，若为 null，则隐藏 Tooltip | value => ReactNode \| null | IDENTITY | 4.23.0 |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Slider** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **14** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/slider
- 中文文档：https://ant.design/components/slider-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/slider
- 驱动 gpui kit：`slider`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Slider** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/slider/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Slider）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Slider）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：滑动型输入器，展示当前值和可选范围。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 控件高度 middle | **32** | `controlHeight` |
| 控件高度 small | **24** | `controlHeightSM` |
| 控件高度 large | **40** | `controlHeightLG` |
| 字号 middle | **14** | `fontSize` |
| 圆角 | **6** | `borderRadius` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / hover / active | `colorPrimary` + 变体 | 强调、选中、开态 |
| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | status 与反馈 |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 浮层阴影 / 遮罩 | `boxShadowSecondary` / `colorBgMask` | 适用者 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultValue` | 设置初始取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \ | \[number, number] |
| `disabled` | 值为 true 时，滑块为禁用状态。该属性也可以是一个数组，用于禁用 range 模式下特定的 handle，例如… | boolean \ | boolean[] |
| `keyboard` | 支持使用键盘操作 handler | boolean | true |
| `dots` | 是否只能拖拽到刻度上 | boolean | false |
| `included` | `marks` 不为空对象时有效，值为 true 时表示值为包含关系，false 表示并列 | boolean | true |
| `marks` | 刻度标记，key 的类型必须为 `number` 且取值在闭区间 \[min, max] 内，每个标签可以单独设置样式 | object | { number: ReactNode } or { number: { style: CSSProperties, label: ReactNode } } |
| `max` | 最大值 | number | 100 |
| `min` | 最小值 | number | 0 |
| `orientation` | 排列方向 | `horizontal` \ | `vertical` |
| `range` | 双滑块模式 | boolean \ | [range](#range) |
| `reverse` | 反向坐标轴 | boolean | false |
| `step` | 步长，取值必须大于 0，并且可被 (max - min) 整除。当 `marks` 不为空对象时，可以设置 `st… | number \ | null |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `tooltip` | 设置 Tooltip 相关属性 | [tooltip](#tooltip) | - |
| `value` | 设置当前取值。当 `range` 为 false 时，使用 number，否则用 \[number, number] | number \ | \[number, number] |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
value=v ∈ [min,max]
  drag handle ──► onChange(v') ── release ──► onChangeComplete(v')
  click rail ──► 跳到对应值
  range ──► 两 handle；左≤右
  marks 点击 ──► 跳到刻度
  keyboard arrows ──► ±step
  disabled ──► 不响应
```

\*轨厚约 4；handle 命中大于可视。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SLD-S1 | 拖到最大 | value=max；onChange |
| SLD-S2 | min/max 夹紧 | 不能越界 |
| SLD-S3 | step=10 | 只落在 10 的倍数 |
| SLD-S4 | range 两柄 | 数组两值 |
| SLD-S5 | marks 点击 | 到刻度值 |
| SLD-S6 | disabled | 不改 |
| SLD-S7 | 键盘右 | +step |
| SLD-S8 | vertical | 垂直轨 |
| SLD-S9 | tooltip | 拖动时展示值（可配） |
| SLD-S10 | onChangeComplete | 仅松手触发 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | textbox / combobox / spinbutton / listbox 等 |
| 标签 | 与 Form.Item label 或 aria-labelledby 关联 |
| 清除/下拉 | 控件有可访问名称 |
| 错误 | status=error 时暴露 invalid |
| 键盘 | 主路径可选/提交/关闭 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |
| IME/剪贴板/滚动宿主（适用者） | **宿主** | P0 宿主 |
| 浏览器-only API | **映射**或 P1 不做 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `value` | 必须 |
| `defaultValue` | 必须 |
| `onChange` | 必须 |
| `disabled` | 必须 |
| `open` | 必须 |
| `placement` | 必须 |
| `orientation` | 必须 |
| 官方主路径示例 | 基本、带输入框的滑块、带 icon 的滑块、自定义提示、事件、带标签的滑块、垂直、控制 ToolTip 的显示 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| 动画像素级 / 复杂虚拟列表 | 分期 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 反向, 范围可拖拽, 多点组合, 动态增减节点 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSlider_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Slider 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SLD-01 | L1 | NewSlider 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SLD-02 | L1 | 拖到最大 | value=max；onChange |
| SLD-03 | L1 | min/max 夹紧 | 不能越界 |
| SLD-04 | L1 | step=10 | 只落在 10 的倍数 |
| SLD-05 | L1 | range 两柄 | 数组两值 |
| SLD-06 | L1 | marks 点击 | 到刻度值 |
| SLD-07 | L1 | disabled | 不改 |
| SLD-08 | L1 | 键盘右 | +step |
| SLD-09 | L1 | vertical | 垂直轨 |
| SLD-10 | L1 | tooltip | 拖动时展示值（可配） |
| SLD-11 | L1 | onChangeComplete | 仅松手触发 |
| SLD-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-13 | L1 | 复现官方示例「带输入框的滑块」（`input-number.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-14 | L1 | 复现官方示例「带 icon 的滑块」（`icon-slider.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-15 | L1 | 复现官方示例「自定义提示」（`tip-formatter.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-16 | L1 | 复现官方示例「事件」（`event.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-17 | L1 | 复现官方示例「带标签的滑块」（`mark.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-18 | L1 | 复现官方示例「垂直」（`vertical.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-19 | L1 | 复现官方示例「控制 ToolTip 的显示」（`show-tooltip.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SLD-20 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SLD-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| SLD-22 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SLD-23 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| SLD-24 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SLD-25 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SLD-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSlider(...) *Slider

// 配置：对 §6.3 / §3 中 P0 字段提供 SetXxx
// 回调：OnChange / OnClick / OnOpenChange / OnConfirm … 按 API
// 状态：SetDisabled / SetLoading（适用者）
// 主题：SetTheme(*Theme)；Style 可选覆盖
// a11y：SetAriaLabel / 焦点与键盘
// 挂树：Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Disabled | false |
| Size（适用者） | middle / 控件默认 |
| 受控值 | 未 Set 时用 default* 或零值 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Field / Selector
  ├─ prefix?
  ├─ editable / display value
  ├─ clear? / suffix?
  └─ Portal popup? (list/panel)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Slider 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/slider.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Slider 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
