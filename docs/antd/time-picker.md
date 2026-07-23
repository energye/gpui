# TimePicker 时间选择框
> 来源：[Ant Design 6.5.x TimePicker](https://ant.design/components/time-picker)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：输入或选择时间的控件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

输入或选择时间的控件。

**TimePicker** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 受控组件 | 复现「受控组件」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 选择确认 | 复现「选择确认」视觉与布局 |
| 禁用 | disabled 灰态与不可点 |
| 选择时分 | 复现「选择时分」视觉与布局 |
| 步长选项 | 复现「步长选项」视觉与布局 |
| 附加内容 | 复现「附加内容」视觉与布局 |
| 12 小时制 | 复现「12 小时制」视觉与布局 |
| 滚动即改变 | 复现「滚动即改变」视觉与布局 |
| 范围选择器 | 复现「范围选择器」视觉与布局 |
| 形态变体 | variant 线框/填充差异 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 前后缀 | 复现「前后缀」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：自定义清除按钮
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：true
- **版本**：5.8.0: 支持对象类型

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：禁用全部操作
- **类型**：boolean
- **默认值**：false

#### `placement`

- **说明**：选择框弹出的位置
- **类型**：`bottomLeft` `bottomRight` `topLeft` `topRight`
- **默认值**：bottomLeft
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |

#### `popupStyle`

- **说明**：弹出层样式对象, 请使用 `styles.popup` 替换
- **类型**：object
- **默认值**：-

#### `prefix`

- **说明**：自定义前缀
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.22.0

#### `size`

- **说明**：输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px
- **类型**：`large` | `medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：设置校验状态
- **类型**：'error' | 'warning' | 'success' | 'validating'
- **默认值**：-
- **版本**：4.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |
  | `success` | 成功绿语义 |
  | `validating` | 官方取值 `validating` |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `suffixIcon`

- **说明**：自定义的选择框后缀图标
- **类型**：ReactNode
- **默认值**：-

#### `use12Hours`

- **说明**：使用 12 小时制，为 true 时 `format` 默认为 `h:mm:ss a`
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `format` | 官方取值 `format` |

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

当用户需要输入一个时间，可以点击标准输入框，弹出时间面板进行选择。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **受控组件**（`value.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **选择确认**（`need-confirm.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **选择时分**（`hide-column.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **步长选项**（`interval-options.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **附加内容**（`addon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **12 小时制**（`12hours.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **滚动即改变**（`change-on-scroll.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **范围选择器**（`range-picker.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **前后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前时间 |
| `defaultValue` | 非受控默认值 | 默认时间 |
| `onChange` | 值变化 | 时间发生变化的回调 |
| `open` | 受控显隐 | 面板是否打开 |
| `onOpenChange` | 显隐变化 | 面板打开/关闭时的回调 |
| `disabled` | 禁用 | 禁用全部操作 |
| `getPopupContainer` | 浮层容器 | 定义浮层的容器，默认为 body 上新建 div |
| `onClear` | 清除 | 点击清除按钮时的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 受控组件 | `value.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 选择确认 | `need-confirm.tsx` | 否 |
| 禁用 | `disabled.tsx` | 否 |
| 选择时分 | `hide-column.tsx` | 否 |
| 步长选项 | `interval-options.tsx` | 否 |
| 附加内容 | `addon.tsx` | 否 |
| 12 小时制 | `12hours.tsx` | 否 |
| 滚动即改变 | `change-on-scroll.tsx` | 否 |
| 彩色弹出层 | `colored-popup.tsx` | 是 |
| 范围选择器 | `range-picker.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 前后缀 | `suffix.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 2.6 FAQ

## FAQ

- [如何在 TimePicker 中使用自定义日期库（如 Moment.js ）](/docs/react/use-custom-date-library#timepicker)

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

---

通用属性参考：[通用属性](/docs/react/common-props)

```jsx
import dayjs from 'dayjs';
import customParseFormat from 'dayjs/plugin/customParseFormat'

dayjs.extend(customParseFormat)
<TimePicker defaultValue={dayjs('13:30:56', 'HH:mm:ss')} />;
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象类型 | 6.4.0 |
| ~~addon~~ | TimePicker 面板底部的附加内容渲染函数，请使用 `renderExtraFooter` 替代 | () => ReactNode | - | - | × |
| cellRender | 自定义单元格的内容 | (current: number, info: { originNode: React.ReactNode, today: dayjs, range?: 'start' \| 'end', subType: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| changeOnScroll | 在滚动时改变选择值 | boolean | false | 5.14.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 默认时间 | [dayjs](http://day.js.org/) | - | disabled | 禁用全部操作 | boolean | false | disabledTime | 不可选择的时间 | [DisabledTime](#disabledtime) | - | 4.19.0 | × |
| format | 展示的时间格式 | string | `HH:mm:ss` | getPopupContainer | 定义浮层的容器，默认为 body 上新建 div | function(trigger) | - | hideDisabledOptions | 隐藏禁止选择的选项 | boolean | false | hourStep | 小时选项间隔 | number | 1 | inputReadOnly | 设置输入框为只读（避免在移动设备上打开虚拟键盘） | boolean | false | minuteStep | 分钟选项间隔 | number | 1 | needConfirm | 是否需要确认按钮，为 `false` 时失去焦点即代表选择 | boolean | - | 5.14.0 | × |
| open | 面板是否打开 | boolean | false | placeholder | 没有值的时候显示的内容 | string \| \[string, string] | `请选择时间` | placement | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | ~~popupClassName~~ | 弹出层类名，请使用 `classNames.popup` 替换 | string | - | ~~popupStyle~~ | 弹出层样式对象, 请使用 `styles.popup` 替换 | object | - | prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| previewValue | 当用户选择时间悬停选项时，输入字段的值会发生临时更改 | false \| hover | hover | 6.0.0 | × |
| renderExtraFooter | 选择框底部显示自定义的内容 | () => ReactNode | - | secondStep | 秒选项间隔 | number | 1 | showNow | 面板是否显示“此刻”按钮 | boolean | - | 4.4.0 | × |
| size | 输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px | `large` \| `medium` \| `small` | - | status | 设置校验状态 | 'error' \| 'warning' \| 'success' \| 'validating' | - | 4.19.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | suffixIcon | 自定义的选择框后缀图标 | ReactNode | - | use12Hours | 使用 12 小时制，为 true 时 `format` 默认为 `h:mm:ss a` | boolean | false | value | 当前时间 | [dayjs](http://day.js.org/) | - | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| onCalendarChange | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | onChange | 时间发生变化的回调 | function(time: dayjs, timeString: string): void | - | onClear | 点击清除按钮时的回调 | () => void | - | 6.5.0 | × |
| onOpenChange | 面板打开/关闭时的回调 | (open: boolean) => void | - 
#### DisabledTime

```typescript
type DisabledTime = (now: Dayjs) => {
  disabledHours?: () => number[];
  disabledMinutes?: (selectedHour: number) => number[];
  disabledSeconds?: (selectedHour: number, selectedMinute: number) => number[];
  disabledMilliseconds?: (
    selectedHour: number,
    selectedMinute: number,
    selectedSecond: number,
  ) => number[];
};
```

注意：`disabledMilliseconds` 为 `5.14.0` 新增。

## 方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

## RangePicker

属性与 DatePicker 的 [RangePicker](/components/date-picker-cn#rangepicker) 相同。还包含以下属性：

| 参数         | 说明                 | 类型                                    | 默认值 | 版本   |
| ------------ | -------------------- | --------------------------------------- | ------ | ------ |
| disabledTime | 不可选择的时间       | [RangeDisabledTime](#rangedisabledtime) | -      | 4.19.0 |
| order        | 始末时间是否自动排序 | boolean                                 | true   | 4.1.0  |

### RangeDisabledTime

```typescript
type RangeDisabledTime = (
  now: Dayjs,
  type = 'start' | 'end',
) => {
  disabledHours?: () => number[];
  disabledMinutes?: (selectedHour: number) => number[];
  disabledSeconds?: (selectedHour: number, selectedMinute: number) => number[];
};
```

### 导入方式

```js
import { TimePicker } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象类型 |
| `addon` | TimePicker 面板底部的附加内容渲染函数，请使用 `renderExtraFooter` 替代 | () => ReactNode | - | - |
| `cellRender` | 自定义单元格的内容 | (current: number, info: { originNode: React.ReactNode, today: dayjs, range?: 'start' \| 'end', subType: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 |
| `changeOnScroll` | 在滚动时改变选择值 | boolean | false | 5.14.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 默认时间 | [dayjs](http://day.js.org/) | - | — |
| `disabled` | 禁用全部操作 | boolean | false | — |
| `disabledTime` | 不可选择的时间 | [DisabledTime](#disabledtime) | - | 4.19.0 |
| `format` | 展示的时间格式 | string | `HH:mm:ss` | — |
| `getPopupContainer` | 定义浮层的容器，默认为 body 上新建 div | function(trigger) | - | — |
| `hideDisabledOptions` | 隐藏禁止选择的选项 | boolean | false | — |
| `hourStep` | 小时选项间隔 | number | 1 | — |
| `inputReadOnly` | 设置输入框为只读（避免在移动设备上打开虚拟键盘） | boolean | false | — |
| `minuteStep` | 分钟选项间隔 | number | 1 | — |
| `needConfirm` | 是否需要确认按钮，为 `false` 时失去焦点即代表选择 | boolean | - | 5.14.0 |
| `open` | 面板是否打开 | boolean | false | — |
| `placeholder` | 没有值的时候显示的内容 | string \| \[string, string] | `请选择时间` | — |
| `placement` | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | — |
| `popupClassName` | 弹出层类名，请使用 `classNames.popup` 替换 | string | - | — |
| `popupStyle` | 弹出层样式对象, 请使用 `styles.popup` 替换 | object | - | — |
| `prefix` | 自定义前缀 | ReactNode | - | 5.22.0 |
| `previewValue` | 当用户选择时间悬停选项时，输入字段的值会发生临时更改 | false \| hover | hover | 6.0.0 |
| `renderExtraFooter` | 选择框底部显示自定义的内容 | () => ReactNode | - | — |
| `secondStep` | 秒选项间隔 | number | 1 | — |
| `showNow` | 面板是否显示“此刻”按钮 | boolean | - | 4.4.0 |
| `size` | 输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px | `large` \| `medium` \| `small` | - | — |
| `status` | 设置校验状态 | 'error' \| 'warning' \| 'success' \| 'validating' | - | 4.19.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `suffixIcon` | 自定义的选择框后缀图标 | ReactNode | - | — |
| `use12Hours` | 使用 12 小时制，为 true 时 `format` 默认为 `h:mm:ss a` | boolean | false | — |
| `value` | 当前时间 | [dayjs](http://day.js.org/) | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onCalendarChange` | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | — |
| `onChange` | 时间发生变化的回调 | function(time: dayjs, timeString: string): void | - | — |
| `onClear` | 点击清除按钮时的回调 | () => void | - | 6.5.0 |
| `onOpenChange` | 面板打开/关闭时的回调 | (open: boolean) => void | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `order` | 始末时间是否自动排序 | boolean | true | 4.1.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **TimePicker** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **15** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/time-picker
- 中文文档：https://ant.design/components/time-picker-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/time-picker
- 驱动 gpui kit：`time-picker`
