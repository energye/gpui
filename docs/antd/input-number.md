# InputNumber 数字输入框
> 来源：[Ant Design 6.5.x InputNumber](https://ant.design/components/input-number)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：通过鼠标或键盘，输入范围内的数值。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

通过鼠标或键盘，输入范围内的数值。

**InputNumber** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 不可用 | disabled 态 |
| 高精度小数 | 复现「高精度小数」视觉与布局 |
| 格式化展示 | 复现「格式化展示」视觉与布局 |
| 键盘行为 | 复现「键盘行为」视觉与布局 |
| 鼠标滚轮 | 复现「鼠标滚轮」视觉与布局 |
| 形态变体 | variant 线框/填充差异 |
| 拨轮 | 复现「拨轮」视觉与布局 |
| 超出边界 | 复现「超出边界」视觉与布局 |
| 前缀/后缀 | 复现「前缀/后缀」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 聚焦 | 复现「聚焦」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `controls`

- **说明**：是否显示增减按钮，也可设置自定义箭头图标
- **类型**：boolean | { upIcon?: React.ReactNode; downIcon?: React.ReactNode; }
- **默认值**：-

#### `disabled`

- **说明**：禁用
- **类型**：boolean
- **默认值**：false

#### `keyboard`

- **说明**：是否启用键盘快捷行为
- **类型**：boolean
- **默认值**：true

#### `max`

- **说明**：最大值
- **类型**：number
- **默认值**：[Number.MAX_SAFE_INTEGER](https://developer.mozilla.org/zh-CN/docs/Web/JavaScript/Reference/Global_Objects/Number/MAX_SAFE_INTEGER)

#### `status`

- **说明**：设置校验状态
- **类型**：'error' | 'warning'
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `prefix`

- **说明**：带有前缀图标的 input
- **类型**：ReactNode
- **默认值**：-

#### `suffix`

- **说明**：带有后缀图标的 input
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.20.0

#### `size`

- **说明**：输入框大小
- **类型**：`large` | `medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `stringMode`

- **说明**：字符值模式，开启后支持高精度小数。同时 `onChange` 将返回 string 类型
- **类型**：boolean
- **默认值**：false
- **版本**：4.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `onChange` | 官方取值 `onChange` |

#### `mode`

- **说明**：展示输入框或拨轮
- **类型**：`'input' | 'spinner'`
- **默认值**：`'input'`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `input` | 官方取值 `input` |
  | `spinner` | 官方取值 `spinner` |

#### `variant`

- **说明**：形态变体
- **类型**：`outlined` | `borderless` | `filled` | `underlined`
- **默认值**：`outlined`
- **版本**：5.13.0 | `underlined`: 5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `borderless` | 无边框 |
  | `filled` | 浅底填充 |
  | `underlined` | 底边线形态 |

#### `bordered`

- **说明**：是否带边框，请使用 `variant` 替代
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

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

当需要获取标准数值时。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **高精度小数**（`digit.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **格式化展示**（`formatter.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **键盘行为**（`keyboard.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **鼠标滚轮**（`change-on-wheel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **拨轮**（`spinner.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **超出边界**（`out-of-range.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **前缀/后缀**（`presuffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **聚焦**（`focus.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前值 |
| `defaultValue` | 非受控默认值 | 初始值 |
| `onChange` | 值变化 | 变化回调 |
| `disabled` | 禁用 | 禁用 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 前置/后置标签 | `addon.tsx` | 是 |
| 不可用 | `disabled.tsx` | 否 |
| 高精度小数 | `digit.tsx` | 否 |
| 格式化展示 | `formatter.tsx` | 否 |
| 键盘行为 | `keyboard.tsx` | 否 |
| 鼠标滚轮 | `change-on-wheel.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 拨轮 | `spinner.tsx` | 否 |
| 禁用步进按钮 hover | `disabled-hover-debug.tsx` | 是 |
| Filled Debug | `filled-debug.tsx` | 是 |
| Borderless 高度对齐 | `borderless-height-debug.tsx` | 是 |
| 超出边界 | `out-of-range.tsx` | 否 |
| 前缀/后缀 | `presuffix.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 聚焦 | `focus.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 图标按钮 | `controls.tsx` | 是 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 覆盖组件样式 | `debug-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### Ref

| 名称 | 说明 | 参数 | 版本 |
| --- | --- | --- | --- |
| blur() | 移除焦点 | - | nativeElement | 获取原生 DOM 元素 | - | 5.17.3 |

### 2.6 FAQ

## FAQ

### 为何受控模式下，`value` 可以超出 `min` 和 `max` 范围？ {#faq-controlled-range}

在受控模式下，开发者可能自行存储相关数据。如果组件将数据约束回范围内，会导致展示数据与实际存储数据不一致的情况。这使得一些如表单场景存在潜在的数据问题。

### 为何动态修改 `min` 和 `max` 让 `value` 超出范围不会触发 `onChange` 事件？ {#faq-dynamic-range-change}

`onChange` 事件为用户触发事件，自行触发会导致表单库误以为变更来自用户操作。我们以错误样式展示超出范围的数值。

### 为何 `onBlur` 等事件获取不到正确的 value？ {#faq-onblur-value}

InputNumber 的值由内部逻辑封装而成，通过 `onBlur` 等事件获取的 `event.target.value` 仅为 DOM 元素的 `value` 而非 InputNumber 的实际值。例如通过 `formatter` 或者 `decimalSeparator` 更改展示格式，DOM 中得到的就是格式化后的字符串。你总是应该通过 `onChange` 获取当前值。

### 为何 `changeOnWheel` 无法控制鼠标滚轮是否改变数值？ {#faq-change-on-wheel}

> 不建议使用 `type` 属性

InputNumber 组件允许你使用 input 元素的所有属性最终透传至 input 元素，当你传入 `type="number"` 时 input 元素也会添加这个属性，这会使 input 元素触发原生特性（允许鼠标滚轮改变数值），从而导致 `changeOnWheel` 无法控制鼠标滚轮是否改变数值。

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
| ~~addonAfter~~ | 带标签的 input，设置后置标签，请使用 Space.Compact 替换 | ReactNode | - | 4.17.0 | × |
| ~~addonBefore~~ | 带标签的 input，设置前置标签，请使用 Space.Compact 替换 | ReactNode | - | 4.17.0 | × |
| changeOnBlur | 是否在失去焦点时，触发 `onChange` 事件（例如值超出范围时，重新限制回范围并触发事件） | boolean | true | 5.11.0 | × |
| changeOnWheel | 允许鼠标滚轮改变数值 | boolean | - | 5.14.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| controls | 是否显示增减按钮，也可设置自定义箭头图标 | boolean \| { upIcon?: React.ReactNode; downIcon?: React.ReactNode; } | - | decimalSeparator | 小数点 | string | - | - | × |
| placeholder | 占位符 | string | - | defaultValue | 初始值 | number | - | - | × |
| disabled | 禁用 | boolean | false | - | × |
| formatter | 指定输入框展示值的格式 | function(value: number \| string, info: { userTyping: boolean, input: string }): string | - | keyboard | 是否启用键盘快捷行为 | boolean | true | max | 最大值 | number | [Number.MAX_SAFE_INTEGER](https://developer.mozilla.org/zh-CN/docs/Web/JavaScript/Reference/Global_Objects/Number/MAX_SAFE_INTEGER) | - | × |
| min | 最小值 | number | [Number.MIN_SAFE_INTEGER](https://developer.mozilla.org/zh-CN/docs/Web/JavaScript/Reference/Global_Objects/Number/MIN_SAFE_INTEGER) | - | × |
| parser | 指定从 `formatter` 里转换回数字的方式，和 `formatter` 搭配使用 | function(string): number | - | - | × |
| precision | 数值精度，配置 `formatter` 时会以 `formatter` 为准 | number | - | - | × |
| readOnly | 只读 | boolean | false | - | × |
| status | 设置校验状态 | 'error' \| 'warning' | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| prefix | 带有前缀图标的 input | ReactNode | - | suffix | 带有后缀图标的 input | ReactNode | - | 5.20.0 | × |
| size | 输入框大小 | `large` \| `medium` \| `small` | - | - | × |
| step | 每次改变步数，可以为小数 | number \| string | 1 | - | × |
| stringMode | 字符值模式，开启后支持高精度小数。同时 `onChange` 将返回 string 类型 | boolean | false | 4.13.0 | × |
| mode | 展示输入框或拨轮 | `'input' \| 'spinner'` | `'input'` | value | 当前值 | number | - | - | × |
| variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| onChange | 变化回调 | function(value: number \| string \| null) | - | - | × |
| onPressEnter | 按下回车的回调 | function(e) | - | - | × |
| onStep | 点击上下箭头、键盘、滚轮的回调 | (value: number, info: { offset: number, type: 'up' \| 'down', emitter: 'handler' \| 'keydown' \| 'wheel' }) => void | - | 4.7.0 | × |
| ~~bordered~~ | 是否带边框，请使用 `variant` 替代 | boolean | true | - | × |

## Ref

| 名称 | 说明 | 参数 | 版本 |
| --- | --- | --- | --- |
| blur() | 移除焦点 | - | nativeElement | 获取原生 DOM 元素 | - | 5.17.3 |

### 导入方式

```js
import { InputNumber } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `addonAfter` | 带标签的 input，设置后置标签，请使用 Space.Compact 替换 | ReactNode | - | 4.17.0 |
| `addonBefore` | 带标签的 input，设置前置标签，请使用 Space.Compact 替换 | ReactNode | - | 4.17.0 |
| `changeOnBlur` | 是否在失去焦点时，触发 `onChange` 事件（例如值超出范围时，重新限制回范围并触发事件） | boolean | true | 5.11.0 |
| `changeOnWheel` | 允许鼠标滚轮改变数值 | boolean | - | 5.14.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `controls` | 是否显示增减按钮，也可设置自定义箭头图标 | boolean \| { upIcon?: React.ReactNode; downIcon?: React.ReactNode; } | - | — |
| `decimalSeparator` | 小数点 | string | - | - |
| `placeholder` | 占位符 | string | - | — |
| `defaultValue` | 初始值 | number | - | - |
| `disabled` | 禁用 | boolean | false | - |
| `formatter` | 指定输入框展示值的格式 | function(value: number \| string, info: { userTyping: boolean, input: string }): string | - | — |
| `keyboard` | 是否启用键盘快捷行为 | boolean | true | — |
| `max` | 最大值 | number | [Number.MAX_SAFE_INTEGER](https://developer.mozilla.org/zh-CN/docs/Web/JavaScript/Reference/Global_Objects/Number/MAX_SAFE_INTEGER) | - |
| `min` | 最小值 | number | [Number.MIN_SAFE_INTEGER](https://developer.mozilla.org/zh-CN/docs/Web/JavaScript/Reference/Global_Objects/Number/MIN_SAFE_INTEGER) | - |
| `parser` | 指定从 `formatter` 里转换回数字的方式，和 `formatter` 搭配使用 | function(string): number | - | - |
| `precision` | 数值精度，配置 `formatter` 时会以 `formatter` 为准 | number | - | - |
| `readOnly` | 只读 | boolean | false | - |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `prefix` | 带有前缀图标的 input | ReactNode | - | — |
| `suffix` | 带有后缀图标的 input | ReactNode | - | 5.20.0 |
| `size` | 输入框大小 | `large` \| `medium` \| `small` | - | - |
| `step` | 每次改变步数，可以为小数 | number \| string | 1 | - |
| `stringMode` | 字符值模式，开启后支持高精度小数。同时 `onChange` 将返回 string 类型 | boolean | false | 4.13.0 |
| `mode` | 展示输入框或拨轮 | `'input' \| 'spinner'` | `'input'` | — |
| `value` | 当前值 | number | - | - |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onChange` | 变化回调 | function(value: number \| string \| null) | - | - |
| `onPressEnter` | 按下回车的回调 | function(e) | - | - |
| `onStep` | 点击上下箭头、键盘、滚轮的回调 | (value: number, info: { offset: number, type: 'up' \| 'down', emitter: 'handler' \| 'keydown' \| 'wheel' }) => void | - | 4.7.0 |
| `bordered` | 是否带边框，请使用 `variant` 替代 | boolean | true | - |
| `(option?: { preventScroll?: boolean, cursor?: 'start' | 'end' | 'all' })` | 获取焦点 | — | — | cursor - 5.22.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **InputNumber** 的验收清单：

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

---
## 5. 参考链接
- 官方文档：https://ant.design/components/input-number
- 中文文档：https://ant.design/components/input-number-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/input-number
- 驱动 gpui kit：`input-number`
