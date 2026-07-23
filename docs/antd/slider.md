# Slider 滑动输入条
> 来源：[Ant Design 6.5.x Slider](https://ant.design/components/slider)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：滑动型输入器，展示当前值和可选范围。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
