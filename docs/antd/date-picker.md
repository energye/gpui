# DatePicker 日期选择框
> 来源：[Ant Design 6.5.x DatePicker](https://ant.design/components/date-picker)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：输入或选择日期的控件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

输入或选择日期的控件。

**DatePicker** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 范围选择器 | 复现「范围选择器」视觉与布局 |
| 多选 | 多选标签/勾选外观 |
| 选择确认 | 复现「选择确认」视觉与布局 |
| 切换不同的选择器 | 复现「切换不同的选择器」视觉与布局 |
| 日期格式 | 复现「日期格式」视觉与布局 |
| 日期时间选择 | 复现「日期时间选择」视觉与布局 |
| 格式对齐 | 复现「格式对齐」视觉与布局 |
| 日期限定范围 | 复现「日期限定范围」视觉与布局 |
| 禁用 | disabled 灰态与不可点 |
| 不可选择日期和时间 | 复现「不可选择日期和时间」视觉与布局 |
| 允许留空 | 空状态插画/文案 |
| 选择不超过一定的范围 | 复现「选择不超过一定的范围」视觉与布局 |
| 预设范围 | 复现「预设范围」视觉与布局 |
| 额外的页脚 | 复现「额外的页脚」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 定制单元格 | 复现「定制单元格」视觉与布局 |
| 定制面板 | 复现「定制面板」视觉与布局 |
| 外部使用面板 | 复现「外部使用面板」视觉与布局 |
| 佛历格式 | 复现「佛历格式」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 形态变体 | variant 线框/填充差异 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 弹出位置 | placement 方位 |
| 前后缀 | 复现「前后缀」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：自定义清除按钮
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：true
- **版本**：5.8.0: 支持对象类型

#### `bordered`

- **说明**：是否带边框，请使用 `variant` 替代
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：禁用
- **类型**：boolean
- **默认值**：false

#### `mode`

- **说明**：日期面板的状态（[设置后无法选择年份/月份？](/docs/react/faq#当我指定了-datepickerrangepicker-的-mode-属性后点击后无法选择年份月份)）
- **类型**：`time` | `date` | `month` | `year` | `decade`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `time` | 时间 |
  | `date` | 日 |
  | `month` | 月 |
  | `year` | 年 |
  | `decade` | 官方取值 `decade` |

#### `nextIcon`

- **说明**：自定义下一个图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.17.0

#### `picker`

- **说明**：设置选择器类型
- **类型**：`date` | `week` | `month` | `quarter` | `year`
- **默认值**：`date`
- **版本**：`quarter`: 4.1.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `date` | 日 |
  | `week` | 周 |
  | `month` | 月 |
  | `quarter` | 季 |
  | `year` | 年 |

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

- **说明**：额外的弹出日历样式，使用 `styles.popup.root` 替代
- **类型**：CSSProperties
- **默认值**：{}

#### `prefix`

- **说明**：自定义前缀
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.22.0

#### `prevIcon`

- **说明**：自定义上一个图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.17.0

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
- **类型**：'error' | 'warning'
- **默认值**：-
- **版本**：4.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `style`

- **说明**：自定义输入框样式
- **类型**：CSSProperties
- **默认值**：{}

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `suffixIcon`

- **说明**：自定义的选择框后缀图标
- **类型**：ReactNode
- **默认值**：-

#### `superNextIcon`

- **说明**：自定义 `>>` 切换图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.17.0

#### `superPrevIcon`

- **说明**：自定义 `<<` 切换图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.17.0

#### `clearIcon`

- **说明**：（仅支持全局配置）自定义清除图标
- **类型**：ReactNode
- **默认值**：-

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

#### `showTime`

- **说明**：增加时间选择功能
- **类型**：Object | boolean
- **默认值**：[TimePicker Options](/components/time-picker-cn#api)

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

当用户需要输入一个日期，可以点击标准输入框，弹出日期面板进行选择。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **范围选择器**（`range-picker.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **多选**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **选择确认**（`needConfirm.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **切换不同的选择器**（`switchable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **日期格式**（`format.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **日期时间选择**（`time.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **格式对齐**（`mask.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **日期限定范围**（`date-range.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **不可选择日期和时间**（`disabled-date.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **允许留空**（`allow-empty.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **选择不超过一定的范围**（`select-in-range.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **预设范围**（`preset-ranges.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **额外的页脚**（`extra-footer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **定制单元格**（`cell-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **定制面板**（`components.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **外部使用面板**（`external-panel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **佛历格式**（`buddhist-era.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
21. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
22. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
23. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
24. **弹出位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
25. **前后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 日期 |
| `defaultValue` | 非受控默认值 | 默认日期，如果开始时间或结束时间为 `null` 或者 `undefined`，日期范围将是一个开区间 |
| `onChange` | 值变化 | 时间发生变化的回调 |
| `onSelect` | 选中 | 选中日期时的回调，请使用 `onCalendarChange` 替代 |
| `open` | 受控显隐 | 控制弹层是否展开 |
| `onOpenChange` | 显隐变化 | 弹出日历和关闭日历的回调 |
| `disabled` | 禁用 | 禁用 |
| `getPopupContainer` | 浮层容器 | 定义浮层的容器，默认为 body 上新建 div |
| `onOk` | 确定 | 点击确定按钮的回调 |
| `onClear` | 清除 | 点击清除按钮时的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 范围选择器 | `range-picker.tsx` | 否 |
| 多选 | `multiple.tsx` | 否 |
| 多选 Debug | `multiple-debug.tsx` | 是 |
| 选择确认 | `needConfirm.tsx` | 否 |
| 切换不同的选择器 | `switchable.tsx` | 否 |
| 日期格式 | `format.tsx` | 否 |
| 日期时间选择 | `time.tsx` | 否 |
| 格式对齐 | `mask.tsx` | 否 |
| 日期限定范围 | `date-range.tsx` | 否 |
| 禁用 | `disabled.tsx` | 否 |
| 不可选择日期和时间 | `disabled-date.tsx` | 否 |
| 允许留空 | `allow-empty.tsx` | 否 |
| 选择不超过一定的范围 | `select-in-range.tsx` | 否 |
| 预设范围 | `preset-ranges.tsx` | 否 |
| 额外的页脚 | `extra-footer.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 定制单元格 | `cell-render.tsx` | 否 |
| 定制面板 | `components.tsx` | 否 |
| 外部使用面板 | `external-panel.tsx` | 否 |
| 佛历格式 | `buddhist-era.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Filled Debug | `filled-debug.tsx` | 是 |
| 弹出位置 | `placement.tsx` | 否 |
| 受控面板 | `mode.tsx` | 是 |
| 自定义日期范围选择 | `start-end.tsx` | 是 |
| 前后缀 | `suffix.tsx` | 否 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| suffixIcon | `suffixIcon-debug.tsx` | 是 |

> debug 标记依据：以源码 `index.zh-CN.md` 代码演示区 `debug` 属性为准；`mode.tsx`（受控面板）、`start-end.tsx`（自定义日期范围选择）在源码中带 `debug` 标记，属受控面板调试内部用例故标 debug。

### 2.5 实例方法 / Ref

#### 方法

### 共同的方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 2.6 FAQ

## FAQ

### 当我指定了 DatePicker/RangePicker 的 mode 属性后，点击后无法选择年份/月份？ {#faq-mode-cannot-select}

请参考[常见问答](/docs/react/faq#当我指定了-datepickerrangepicker-的-mode-属性后点击后无法选择年份月份)

### 为何日期选择年份后返回的是日期面板而不是月份面板？ {#faq-year-to-date-panel}

当用户选择完年份后，系统会直接切换至日期面板，而非显式提供月份选择。这样做的设计在于用户只需进行一次点击即可完成年份修改，无需再次点击进入月份选择界面，从而减少了用户的操作负担，同时也避免需要额外感知月份的记忆负担。

### 如何在 DatePicker 中使用自定义日期库（如 Moment.js ）？ {#faq-custom-date-library}

请参考[《使用自定义日期库》](/docs/react/use-custom-date-library#datepicker)

### 为什么时间类组件的国际化 locale 设置不生效？ {#faq-locale-not-work}

参考 FAQ [为什么时间类组件的国际化 locale 设置不生效？](/docs/react/faq#为什么时间类组件的国际化-locale-设置不生效)。

### 如何修改周的起始日？ {#faq-week-start-day}

请使用正确的[语言包](/docs/react/i18n-cn)（[#5605](https://github.com/ant-design/ant-design/issues/5605)），或者修改 dayjs 的 `locale` 配置：

```js
import dayjs from 'dayjs';

import 'dayjs/locale/zh-cn';

import updateLocale from 'dayjs/plugin/updateLocale';

dayjs.extend(updateLocale);
dayjs.updateLocale('zh-cn', {
  weekStart: 0,
});
```

### 为何使用 `panelRender` 时，原来面板无法切换？ {#faq-panel-render-switch}

当你通过 `panelRender` 动态改变层级结构时，会使得原本的 Panel 被当做新的节点删除并创建。这使得其原本的状态会被重置，保持结构稳定即可。详情请参考 [#27263](https://github.com/ant-design/ant-design/issues/27263)。

### 如何理解禁用时间日期？ {#faq-disabled-date-time}

欢迎阅读博客[《为什么禁用日期这么难？》](/docs/blog/picker-cn)了解如何使用。

### 2.7 组合关系

- **依赖等级 L3**：浮层件 + 表单件；触发器 Input 壳 + 日历面板 Portal；先做 `input`/`time-picker` 再做本件。
- **Form**：日期值 `value`（`DateValue` 对齐 dayjs），Range 二元组，`status` 由 Item 下发。
- **ConfigProvider**：locale（周起始/佛历/文案）、size/variant 全局默认。
- **浮层**：Modal/Drawer 内注意 `getPopupContainer`；`placement` 四角。
- **文件归属**：`ui/kit/date-picker/`（触发器 + 日历面板 + Range/时间列）。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

日期类组件包括以下五种形式。

- DatePicker
- DatePicker\[picker="month"]
- DatePicker\[picker="week"]
- DatePicker\[picker="year"]
- DatePicker\[picker="quarter"] (4.1.0 新增)
- RangePicker

### 国际化配置

默认配置为 en-US，如果你需要设置其他语言，推荐在入口处使用我们提供的国际化组件，详见：[ConfigProvider 国际化](https://ant.design/components/config-provider-cn/)。

如有特殊需求（仅修改单一组件的语言），请使用 locale 参数，参考：[默认配置](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json)。

```jsx
// 默认语言为 en-US，如果你需要设置其他语言，推荐在入口文件全局设置 locale
// 确保还导入相关的 dayjs 文件，否则所有文本的区域设置都不会更改（例如范围选择器月份）
import locale from 'antd/locale/zh_CN';
import dayjs from 'dayjs';

import 'dayjs/locale/zh-cn';

dayjs.locale('zh-cn');

<ConfigProvider locale={locale}>
  <DatePicker defaultValue={dayjs('2015-01-01', 'YYYY-MM-DD')} />
</ConfigProvider>;
```

:::warning
在搭配 Next.js 的 App Router 使用时，注意在引入 dayjs 的 locale 文件时加上 `'use client'`。这是由于 Ant Design 的组件都是客户端组件，在 RSC 中引入 dayjs 的 locale 文件将不会在客户端生效。
:::

### 共同的 API

以下 API 为 DatePicker、 RangePicker 共享的 API。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象类型 | 6.4.0 |
| ~~bordered~~ | 是否带边框，请使用 `variant` 替代 | boolean | true | - | × |
| className | 选择器 className | string | - | - | DatePicker: 5.7.0，RangePicker: 5.11.0 |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | - | DatePicker: 5.25.0，RangePicker: 5.25.0 |
| dateRender | 自定义日期单元格的内容，5.4.0 起用 `cellRender` 代替 | function(currentDate: dayjs, today: dayjs) => React.ReactNode | - | < 5.4.0 | × |
| cellRender | 自定义单元格的内容 | (current: dayjs, info: { originNode: React.ReactElement,today: DateType, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| components | 自定义面板 | Record<Panel \| 'input', React.ComponentType> | - | 5.14.0 | × |
| defaultOpen | 是否默认展开控制弹层 | boolean | - | - | × |
| disabled | 禁用 | boolean | false | - | × |
| disabledDate | 不可选择的日期 | (currentDate: dayjs, info: { from?: dayjs, type: Picker }) => boolean | - | `info`: 5.14.0 | × |
| ~~dropdownClassName~~ | 弹出日历的 className，请使用 `classNames.popup.root` 替代 | string | - | - | × |
| format | 设置日期格式，为数组时支持多格式匹配，展示以第一个为准。配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。示例：[自定义格式](#date-picker-demo-format) | [formatType](#formattype) | [@rc-component/picker](https://github.com/react-component/picker/blob/f512f18ed59d6791280d1c3d7d37abbb9867eb0b/src/utils/uiUtil.ts#L155-L177) | - | × |
| order | 多选、范围时是否自动排序 | boolean | true | 5.14.0 | × |
| preserveInvalidOnBlur | 失去焦点是否要清空输入框内无效内容 | boolean | false | 5.14.0 | × |
| ~~popupClassName~~ | 额外的弹出日历 className，使用 `classNames.popup.root` 替代 | string | - | 4.23.0 | × |
| getPopupContainer | 定义浮层的容器，默认为 body 上新建 div | function(trigger) | - | - | × |
| inputReadOnly | 设置输入框为只读（避免在移动设备上打开虚拟键盘） | boolean | false | - | × |
| locale | 国际化配置 | object | [默认配置](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | - | × |
| minDate | 最小日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 | × |
| maxDate | 最大日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 | × |
| mode | 日期面板的状态（[设置后无法选择年份/月份？](/docs/react/faq#当我指定了-datepickerrangepicker-的-mode-属性后点击后无法选择年份月份)） | `time` \| `date` \| `month` \| `year` \| `decade` | - | - | × |
| needConfirm | 是否需要确认按钮，为 `false` 时失去焦点即代表选择。当设置 `multiple` 时默认为 `false` | boolean | - | 5.14.0 | × |
| nextIcon | 自定义下一个图标 | ReactNode | - | 4.17.0 | × |
| open | 控制弹层是否展开 | boolean | - | - | × |
| panelRender | 自定义渲染面板 | (panelNode) => ReactNode | - | 4.5.0 | × |
| picker | 设置选择器类型 | `date` \| `week` \| `month` \| `quarter` \| `year` | `date` | `quarter`: 4.1.0 | × |
| placeholder | 输入框提示文字 | string \| \[string, string] | - | - | × |
| placement | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | - | × |
| ~~popupStyle~~ | 额外的弹出日历样式，使用 `styles.popup.root` 替代 | CSSProperties | {} | - | × |
| prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| prevIcon | 自定义上一个图标 | ReactNode | - | 4.17.0 | × |
| previewValue | 当用户选择日期悬停选项时，输入字段的值会发生临时更改 | false \| hover | hover | 6.0.0 | × |
| presets | 预设时间范围快捷选择, 自 `5.8.0` 起 value 支持函数返回值 | { label: React.ReactNode, value: Dayjs \| (() => Dayjs) }\[] | - | - | × |
| size | 输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px | `large` \| `medium` \| `small` | - | - | × |
| status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| style | 自定义输入框样式 | CSSProperties | {} | - | DatePicker: 5.7.0，RangePicker: 5.11.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | - | DatePicker: 5.25.0，RangePicker: 5.25.0 |
| suffixIcon | 自定义的选择框后缀图标 | ReactNode | - | - | DatePicker: 6.3.0，RangePicker: 6.4.0 |
| superNextIcon | 自定义 `>>` 切换图标 | ReactNode | - | 4.17.0 | × |
| superPrevIcon | 自定义 `<<` 切换图标 | ReactNode | - | 4.17.0 | × |
| clearIcon | （仅支持全局配置）自定义清除图标 | ReactNode | - | × | 6.4.0 |
| variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | DatePicker: 5.19.0，RangePicker: 5.19.0 |
| onClear | 点击清除按钮时的回调 | () => void | - | 6.5.0 | × |
| onOpenChange | 弹出日历和关闭日历的回调 | function(open) | - | - | × |
| onPanelChange | 日历面板切换的回调 | function(value, mode) | - | - | × |
| ~~onSelect~~ | 选中日期时的回调，请使用 `onCalendarChange` 替代 | function(value) | - | - | × |

### 共同的方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### DatePicker

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultPickerValue | 默认面板日期，每次面板打开时会被重置到该日期 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| defaultValue | 默认日期，如果开始时间或结束时间为 `null` 或者 `undefined`，日期范围将是一个开区间 | [dayjs](https://day.js.org/) | - | - |
| format | 展示的日期格式，配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。 | [formatType](#formattype) | `YYYY-MM-DD` | - |
| pickerValue | 面板日期，可以用于受控切换面板所在日期。配合 `onPanelChange` 使用。 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | (mode) => React.ReactNode | - | - |
| showTime | 增加时间选择功能 | Object \| boolean | [TimePicker Options](/components/time-picker-cn#api) | - |
| showTime.defaultOpenValue | 设置用户选择日期时默认的时分秒，[例子](#date-picker-demo-disabled-date) | [dayjs](https://day.js.org/) | dayjs() | - |
| showWeek | DatePicker 下展示当前周 | boolean | false | 5.14.0 |
| value | 日期 | [dayjs](https://day.js.org/) | - | - |
| onOk | 点击确定按钮的回调 | function() | - | - | 
### DatePicker\[picker=year]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | - |
| multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | - |
| value | 日期 | [dayjs](https://day.js.org/) | - | - |
### DatePicker\[picker=quarter]

`4.1.0` 新增。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | - |
| multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | - |
| value | 日期 | [dayjs](https://day.js.org/) | - | - |
### DatePicker\[picker=month]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | - |
| multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | - |
| value | 日期 | [dayjs](https://day.js.org/) | - | - |
### DatePicker\[picker=week]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | - |
| multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | (mode) => React.ReactNode | - | - |
| value | 日期 | [dayjs](https://day.js.org/) | - | - |
| showWeek | DatePicker 下展示当前周（week 变体） | boolean | true | 5.14.0 |

### RangePicker

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowEmpty | 允许起始项部分为空 | \[boolean, boolean] | \[false, false] | - | × |
| cellRender | 自定义单元格的内容。 | (current: dayjs, info: { originNode: React.ReactElement,today: DateType, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| dateRender | 自定义日期单元格的内容，5.4.0 起用 `cellRender` 代替 | function(currentDate: dayjs, today: dayjs) => React.ReactNode | - | < 5.4.0 | × |
| defaultPickerValue | 默认面板日期，每次面板打开时会被重置到该日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | 5.14.0 | × |
| defaultValue | 默认日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | - | × |
| disabled | 禁用起始项 | \[boolean, boolean] | - | - | × |
| disabledTime | 不可选择的时间 | function(date: dayjs, partial: `start` \| `end`, info: { from?: dayjs }) | - | `info.from`: 5.17.0 | × |
| format | 展示的日期格式，配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。 | [formatType](#formattype) | `YYYY-MM-DD HH:mm:ss` | - | × |
| id | 设置输入框 `id` 属性。 | { start?: string, end?: string } | - | 5.14.0 | × |
| pickerValue | 面板日期，可以用于受控切换面板所在日期。配合 `onPanelChange` 使用。 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | 5.14.0 | × |
| presets | 预设时间范围快捷选择，自 `5.8.0` 起 value 支持函数返回值 | { label: React.ReactNode, value: \[(Dayjs \| (() => Dayjs)), (Dayjs \| (() => Dayjs))] }\[] | - | - | × |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | - | × |
| separator | 设置分隔符 | React.ReactNode | `<SwapRightOutlined />` | - | 6.3.0 |
| showTime | 增加时间选择功能 | Object\|boolean | [TimePicker Options](/components/time-picker-cn#api) | - | × |
| ~~showTime.defaultValue~~ | 请使用 `showTime.defaultOpenValue` | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | \[dayjs(), dayjs()] | 5.27.3 | × |
| showTime.defaultOpenValue | 设置用户选择日期时默认的时分秒，[例子](#date-picker-demo-disabled-date) | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | \[dayjs(), dayjs()] | - | × |
| value | 日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | - | × |
| onCalendarChange | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | - | × |
| onChange | 日期范围发生变化的回调 | function(dates: \[dayjs, dayjs] \| null, dateStrings: \[string, string] \| null) | - | - | × |
| onFocus | 聚焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 | × |
| onBlur | 失焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 | × |

#### formatType

```typescript
import type { Dayjs } from 'dayjs';

type Generic = string;
type GenericFn = (value: Dayjs) => string;

export type FormatType =
  | Generic
  | GenericFn
  | Array<Generic | GenericFn>
  | {
      format: string;
      type?: 'mask';
    };
```

注意：`type` 定义为 `5.14.0` 新增。

### 导入方式

```js
import { DatePicker } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象类型 |
| `bordered` | 是否带边框，请使用 `variant` 替代 | boolean | true | - |
| `className` | 选择器 className | string | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dateRender` | 自定义日期单元格的内容，5.4.0 起用 `cellRender` 代替 | function(currentDate: dayjs, today: dayjs) => React.ReactNode | - | < 5.4.0 |
| `cellRender` | 自定义单元格的内容 | (current: dayjs, info: { originNode: React.ReactElement,today: DateType, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 |
| `components` | 自定义面板 | Record | - | 5.14.0 |
| `defaultOpen` | 是否默认展开控制弹层 | boolean | - | — |
| `disabled` | 禁用 | boolean | false | — |
| `disabledDate` | 不可选择的日期 | (currentDate: dayjs, info: { from?: dayjs, type: Picker }) => boolean | - | `info`: 5.14.0 |
| `dropdownClassName` | 弹出日历的 className，请使用 `classNames.popup.root` 替代 | string | - | - |
| `format` | 设置日期格式，为数组时支持多格式匹配，展示以第一个为准。配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。示例：[自定义格式](#date-picker-demo-format) | [formatType](#formattype) | [@rc-component/picker](https://github.com/react-component/picker/blob/f512f18ed59d6791280d1c3d7d37abbb9867eb0b/src/utils/uiUtil.ts#L155-L177) | — |
| `order` | 多选、范围时是否自动排序 | boolean | true | 5.14.0 |
| `preserveInvalidOnBlur` | 失去焦点是否要清空输入框内无效内容 | boolean | false | 5.14.0 |
| `popupClassName` | 额外的弹出日历 className，使用 `classNames.popup.root` 替代 | string | - | 4.23.0 |
| `getPopupContainer` | 定义浮层的容器，默认为 body 上新建 div | function(trigger) | - | — |
| `inputReadOnly` | 设置输入框为只读（避免在移动设备上打开虚拟键盘） | boolean | false | — |
| `locale` | 国际化配置 | object | [默认配置](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | — |
| `minDate` | 最小日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 |
| `maxDate` | 最大日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 |
| `mode` | 日期面板的状态（[设置后无法选择年份/月份？](/docs/react/faq#当我指定了-datepickerrangepicker-的-mode-属性后点击后无法选择年份月份)） | `time` \| `date` \| `month` \| `year` \| `decade` | - | — |
| `needConfirm` | 是否需要确认按钮，为 `false` 时失去焦点即代表选择。当设置 `multiple` 时默认为 `false` | boolean | - | 5.14.0 |
| `nextIcon` | 自定义下一个图标 | ReactNode | - | 4.17.0 |
| `open` | 控制弹层是否展开 | boolean | - | — |
| `panelRender` | 自定义渲染面板 | (panelNode) => ReactNode | - | 4.5.0 |
| `picker` | 设置选择器类型 | `date` \| `week` \| `month` \| `quarter` \| `year` | `date` | `quarter`: 4.1.0 |
| `placeholder` | 输入框提示文字 | string \| \[string, string] | - | — |
| `placement` | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | — |
| `popupStyle` | 额外的弹出日历样式，使用 `styles.popup.root` 替代 | CSSProperties | {} | — |
| `prefix` | 自定义前缀 | ReactNode | - | 5.22.0 |
| `prevIcon` | 自定义上一个图标 | ReactNode | - | 4.17.0 |
| `previewValue` | 当用户选择日期悬停选项时，输入字段的值会发生临时更改 | false \| hover | hover | 6.0.0 |
| `presets` | 预设时间范围快捷选择, 自 `5.8.0` 起 value 支持函数返回值 | { label: React.ReactNode, value: Dayjs \| (() => Dayjs) }\[] | - | — |
| `size` | 输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px | `large` \| `medium` \| `small` | - | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `style` | 自定义输入框样式 | CSSProperties | {} | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `suffixIcon` | 自定义的选择框后缀图标 | ReactNode | - | — |
| `superNextIcon` | 自定义 `>>` 切换图标 | ReactNode | - | 4.17.0 |
| `superPrevIcon` | 自定义 `<<` 切换图标 | ReactNode | - | 4.17.0 |
| `clearIcon` | （仅支持全局配置）自定义清除图标 | ReactNode | - | × |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onClear` | 点击清除按钮时的回调 | () => void | - | 6.5.0 |
| `onOpenChange` | 弹出日历和关闭日历的回调 | function(open) | - | — |
| `onPanelChange` | 日历面板切换的回调 | function(value, mode) | - | — |
| `onSelect` | 选中日期时的回调，请使用 `onCalendarChange` 替代 | function(value) | - | - |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `defaultPickerValue` | 默认面板日期，每次面板打开时会被重置到该日期 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| `defaultValue` | 默认日期，如果开始时间或结束时间为 `null` 或者 `undefined`，日期范围将是一个开区间 | [dayjs](https://day.js.org/) | - | — |
| `disabledTime` | 不可选择的时间 | function(date) | - | — |
| `multiple` | 是否为多选，不支持 `showTime` | boolean | false | 5.14.0 |
| `pickerValue` | 面板日期，可以用于受控切换面板所在日期。配合 `onPanelChange` 使用。 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| `renderExtraFooter` | 在面板中添加额外的页脚 | (mode) => React.ReactNode | - | — |
| `showNow` | 显示当前日期时间的快捷选择 | boolean | - | — |
| `showTime` | 增加时间选择功能 | Object \| boolean | [TimePicker Options](/components/time-picker-cn#api) | — |
| `showTime.defaultValue` | 请使用 `showTime.defaultOpenValue` | [dayjs](https://day.js.org/) | dayjs() | 5.27.3 |
| `showTime.defaultOpenValue` | 设置用户选择日期时默认的时分秒，[例子](#date-picker-demo-disabled-date) | [dayjs](https://day.js.org/) | dayjs() | — |
| `showWeek`（DatePicker 单体） | DatePicker 下展示当前周 | boolean | false | 5.14.0 |
| `showWeek`（picker=week 变体） | DatePicker 下展示当前周 | boolean | true | 5.14.0 |
| `value` | 日期 | [dayjs](https://day.js.org/) | - | — |
| `onChange` | 时间发生变化的回调 | function(date: dayjs \| null, dateString: string \| null) | - | — |
| `onOk` | 点击确定按钮的回调 | function() | - | — |
| `tagRender` | 自定义 tag 内容 render，仅在 `multiple` 模式下生效 | (props) => ReactNode | - | 6.4.0 |
| `allowEmpty` | 允许起始项部分为空 | \[boolean, boolean] | \[false, false] | — |
| `id` | 设置输入框 `id` 属性。 | { start?: string, end?: string } | - | 5.14.0 |
| `separator` | 设置分隔符 | React.ReactNode | `<SwapRightOutlined />` | 6.3.0 |
| `onCalendarChange` | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | — |
| `onFocus` | 聚焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 |
| `onBlur` | 失焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **DatePicker** 的验收清单：

1. **配置面**：覆盖 API 表全部字段；冷门字段必须实现且命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：官方非 debug **25** 个：P0 **8**（§6.8 主路径）+ P1 **17**（禁用/预设/面板定制等）；debug 7 个不验收。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/date-picker
- 中文文档：https://ant.design/components/date-picker-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/date-picker
- 驱动 gpui kit：`date-picker`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **DatePicker** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/date-picker/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（DatePicker）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（DatePicker）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：输入或选择日期的控件。

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

**面板 Token（对齐 `components/date-picker/style/token.ts` → `initPanelComponentToken` + `initPickerPanelToken`，默认种子 `controlHeightSM=24`/`controlHeightLG=40`/`paddingXXS=4`）：**

| Token | 默认值 | 说明 |
| --- | --- | --- |
| `cellWidth` | **36**（`controlHeightSM×1.5`） | 日期单元格宽 |
| `cellHeight` | **24**（`controlHeightSM`） | 日期单元格高 |
| `textHeight` | **40**（`controlHeightLG`） | 单元格文本行高 |
| `withoutTimeCellHeight` | **66**（`controlHeightLG×1.65`） | 年/季/月/周单元格高 |
| `timeColumnWidth` | **56**（`controlHeightLG×1.4`） | 时间列宽（时/分/秒各一列） |
| `timeColumnHeight` | **224**（`28×8`） | 时间列高（可见约 8 行） |
| `timeCellHeight` | **28** | 时间单行高 |
| `pickerYearMonthCellWidth` | **60**（`controlHeightLG×1.5`） | 年/月单元格宽 |
| `pickerQuarterPanelContentHeight` | **56**（`controlHeightLG×1.4`） | 季度面板内容高 |
| `pickerCellPaddingVertical` | **6**（`paddingXXS + paddingXXS/2`） | 单元格纵向边距 |
| `pickerCellBorderGap` | **2** | 单元格间隙（range 背景断开用） |
| `presetsWidth` / `presetsMaxWidth` | **120** / **200** | 预设快捷区宽（P1） |

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
| `value` / `defaultValue` | 受控 / 非受控值；Range 为二元组；`multiple` 为数组 | dayjs \| [dayjs, dayjs] \| dayjs[] | — |
| `onChange` | 值变更（date, dateString） | function | — |
| `picker` | 选择器类型 | `date` \| `week` \| `month` \| `quarter` \| `year` | `date` |
| `format` | 展示/解析格式；字符串或以 `{format,type:'mask'}` 声明 mask | string \| FormatType | 见 antd 默认表 |
| `showTime` | 增加时分秒（可与 `needConfirm` 同用） | boolean \| object | false |
| `needConfirm` | 需点确认才提交 | boolean | false（showTime 时 antd 默认 true，kit P0 以显式字段为准） |
| `multiple` | 多选日期（仅 DatePicker，非 Range） | boolean | false |
| `allowClear` | 清除按钮 | boolean | true |
| `disabled` | 整控件禁用 | boolean | false |
| `disabledDate` | 不可选日期 | `(current) => boolean` | — |
| `size` | 控件高度档 | `large` \| `middle` \| `small` | `middle` |
| `variant` | 形态 | `outlined` \| `filled` \| `borderless` \| `underlined` | `outlined` |
| `status` | 校验态 | `error` \| `warning` | — |
| `open` / `defaultOpen` / `onOpenChange` | 弹层受控 / 非受控 | boolean / function | closed |
| `placement` | 弹层四角 | `bottomLeft` \| `bottomRight` \| `topLeft` \| `topRight` | `bottomLeft` |
| `placeholder` | 空值占位 | string \| [string,string] | locale 默认 |
| `order` | Range/多选是否自动排序 | boolean | true |
| `minDate` / `maxDate` | 面板可切换范围下界/上界 | dayjs | — |
| `mode` | **受控面板模式**（time/date/month/year/decade） | PanelMode | 由 `picker` 推导（**P1**） |

**RangePicker：** 与 DatePicker 共享上表；值类型为 `[start, end]`；无 `multiple`。

**配置优先级（通用）：** 受控 props（`value`/`open`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
closed ── open ──► 面板（picker=date/week/month/quarter/year）
             ├── 点日/周/月/季/年 ──► onChange(value, string) ──► 常关闭
             ├── Range：点起 + 点止 ──► onChange([start,end], [s,e])；order 时有序
             ├── multiple：点选切换集合；面板常保持打开
             ├── showTime ──► 选日后进/同屏时分秒；可选 needConfirm → OK 才提交
             ├── needConfirm ──► 预览选择；OK 提交 / 取消或外点丢弃
             ├── disabledDate(current)=true ──► 不可点
             ├── allowClear ──► 空值 + onChange(nil)
             └── Esc/外点 ──► 关闭（needConfirm 未确认则丢弃预览）
```

\*值类型桌面用 `DateValue`（日历日 + 可选时分秒）；语义对齐 dayjs。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| DP-S1 | 打开并选一天 | `onChange` 一次；输入框展示 format |
| DP-S2 | Range 选起止 | 两值有序（`order=true`） |
| DP-S3 | `disabledDate` 禁今天 | 今天不可选 |
| DP-S4 | `picker=month` | 月面板；选月 |
| DP-S5 | `allowClear` | 清空 |
| DP-S6 | `showTime` | 可选出时分秒 |
| DP-S7 | 受控 value | 外部优先 |
| DP-S8 | 格式 format | 展示字符串匹配 |
| DP-S9 | 禁用 | 打不开或不响应 |
| DP-S10 | size 高度 | 24/32/40 |
| DP-S11 | `needConfirm` | 选日不立即 onChange；Confirm 后一次 |
| DP-S12 | `multiple` | 多日集合；再点取消 |

**时间列算法（`showTime`）：** 选日后时间列与日期面板同屏（或次屏），三列时/分/秒各宽 `timeColumnWidth=56`、行高 `timeCellHeight=28`、列高 `timeColumnHeight=224`（超高列内滚动）；时分秒分别步进选择，`showTime.defaultOpenValue` 为打开面板时的时间初值；日期切换不重置已选时分秒，切回同日保留。

**确认流算法（`needConfirm`）：** 选日/选时只写预览值（输入框可配 `previewValue=hover` 实时预览），不发 `onChange`；点 OK（`onOk`）一次性提交预览并关面板，点取消/外点/Esc 丢弃预览；`multiple=true` 时 antd 默认 `needConfirm=false`（多选点选即进集合），kit 同此默认。

**mask 算法（`format type=mask`）：** `format={format, type:'mask'}` 时输入框按格式占位对齐（如 `YYYY-MM-DD` 占 10 格），数字键顺序填入年月日格、非数字键忽略，退格按格回退；失焦时按 `format` 解析，合法则提交 `onChange`，非法按 `preserveInvalidOnBlur=false` 清空（默认）。

**`disabledTime` 去向：** 单体 `DatePicker` 的 `disabledTime(date)`（返回禁用的时/分/秒集合）随 `showTime` 进 P0（时分秒列中禁行不可点）；`RangePicker` 按 `partial=start|end` 区分两端 + `info.from` 联动的深度形态列 P1，P0 的 Range 仅支持整控件 `disabled` 与按日 `disabledDate`。

**周编号规则：** `picker=week` 选中整周（值取周起始日），周编号按 ISO 周（周一起算的年第几周）；周起始跟随 locale（`zh-cn` 周一、`en` 周日），kit P0 允许固定周一起算并在 Notes 注明，完整 locale 周起始为 P1。
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 触发器 Input 壳（outlined 默认）+ 日历图标后缀；Token 色 |
| hover | 触发器边框/底强调；日期格 hover 底 `cellHoverBg` |
| focus | 触发器**可见** focus ring + 主色边 |
| disabled | 触发器降对比；面板禁日只读（`cellBgDisabled`） |
| status=error/warning | 触发器语义色边框；面板链路不变 |
| 弹层 open | 日历格 36×24 + 时间列 56×224（行 28）；选中主色底；Range 跨格背景不断线（间隙 2） |

**variant 矩阵（`variant` × chrome，L2，触发器壳与 Input 同规则）：**

| variant | 填充 | 边框 | focus | 备注 |
| --- | --- | --- | --- | --- |
| `outlined`（默认） | `colorBgContainer` | 1px `colorBorder` 全边框 | 主色边 + 可见 ring | 默认 |
| `filled` | `colorFillAlter` 浅底 | 无/弱边框 | 主色边 + ring | 浅底形态 |
| `borderless` | 透明 | 无 | 仅 ring 可见 | 无 chrome |
| `underlined` | 透明 | 仅底边 1px `colorBorder` | 底边走主色 | 底边线形态 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 触发器 `textbox`（只读可聚焦）+ 面板 `grid` + 日期 `gridcell`（`aria-selected`/`aria-disabled`）；时间列 `listbox` |
| 标签 | 与 Form.Item label 或 `AriaLabel` 关联；Range 起止各有名 |
| 清除 | 清除钮（allowClear 默认 true）有可访问名；空值时隐藏 |
| 错误 | `status=error` 时触发器暴露 invalid + 错误文案关联 |
| 键盘 | Enter/Space 开层；方向键移格；Enter 选中；Esc 关层；`needConfirm` 下 OK 才提交 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 选日/Range/多选/确认/时间列主路径 | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2：格 36×24/时间列 56×224） | **对等** | P0 L2 |
| 日历 locale（周起始/佛历/年月文案跟随 `locale`，P0 允许周一起算并注明） | **映射** locale 包 | P0 简 / P1 全 |
| 受控 `mode` 面板/`minDate`/`maxDate`/`presets`/`cellRender`/`panelRender` | P1 必做 | P1 |
| `getPopupContainer` 宿主挂载 + `placement` 四角 | **映射**宿主容器 | P0 宿主 |
| Semantic classNames/styles + ConfigProvider 全局默认 | kit 语义钩子，随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

**实现顺序（一次做完，不分期）：单日 → Range → showTime → multiple/needConfirm。** 先跑通单日 `DatePicker`（选日/open/format/禁用/弹层），再叠 Range 二元组，然后 showTime 时间列，最后 multiple 多选与 needConfirm 确认流；后一期以前一期用例全绿为前提。

| 配置 / 能力 | 说明 |
| --- | --- |
| `value` / `defaultValue` / `onChange` | 单选 / Range 二元组 / multiple 数组 |
| `picker` | `date` \| `week` \| `month` \| `quarter` \| `year` |
| `format` + mask 形态 | 展示串；`type=mask` 时占位对齐（输入深度可简） |
| `showTime` | 时分秒可选出 |
| `needConfirm` | 确认后才提交 |
| `multiple` | 多选日期 |
| RangePicker | `NewRangePicker` 或 `SetRange(true)` |
| `disabled` / `disabledDate` | 整控件 / 单日禁选 |
| `size` / `variant` / `status` | 高度档 / 形态 / 校验态 |
| `open` / `defaultOpen` / `onOpenChange` | 弹层 |
| `placement` | 四角 |
| `allowClear` | 清除 |
| 官方主路径示例 | 基本、范围选择器、多选、选择确认、切换不同的选择器、日期格式、日期时间选择、格式对齐 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| 受控 `mode` 面板（time/date/month/year/decade） | P1 必做 |
| `minDate`/`maxDate` 面板切换钳制、`presets`、`cellRender` | P1 必做 |
| semantic classNames/styles 深度 | P1 必做 |
| 动画像素级 / 虚拟长列表 / 输入 mask 逐键编辑深度 | P1 必做 |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| debug 示例 | 不验收（仅参考） |
| 其余示例（P1，逐例去向） | 日期限定范围（`date-range.tsx`，minDate/maxDate）、禁用（`disabled.tsx`）、不可选择日期和时间（`disabled-date.tsx`，disabledDate/disabledTime）、允许留空（`allow-empty.tsx`）、选择不超过一定的范围（`select-in-range.tsx`）、预设范围（`preset-ranges.tsx`，presets）、额外的页脚（`extra-footer.tsx`，renderExtraFooter）、三种大小（`size.tsx`）、定制单元格（`cell-render.tsx`，cellRender）、定制面板（`components.tsx`，components 定制）、外部使用面板（`external-panel.tsx`）、佛历格式（`buddhist-era.tsx`，locale 佛历）、自定义状态（`status.tsx`）、形态变体（`variant.tsx`）、自定义语义结构的样式和类（`style-class.tsx`，semantic 深度）、弹出位置（`placement.tsx`）、前后缀（`suffix.tsx`，prefix/suffixIcon） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestDatePicker_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 DatePicker 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| DP-01 | L1 | NewDatePicker 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| DP-02 | L1 | 打开并选一天 | `onChange` 一次；输入框展示 format |
| DP-03 | L1 | Range 选起止 | 两值有序 |
| DP-04 | L1 | `disabledDate` 禁今天 | 今天不可选 |
| DP-05 | L1 | `picker=month` | 月面板；选月 |
| DP-06 | L1 | `allowClear` | 清空 |
| DP-07 | L1 | `showTime` | 可选出时分秒 |
| DP-08 | L1 | 受控 value | 外部优先 |
| DP-09 | L1 | 格式 format | 展示字符串匹配 |
| DP-10 | L1 | 禁用 | 打不开或不响应 |
| DP-11 | L1 | size 高度 | 24/32/40 |
| DP-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | picker 切换 date/week/month/quarter/year 可创建；主路径可选 |
| DP-13 | L1 | 复现官方示例「范围选择器」（`range-picker.tsx`） | NewRangePicker 选起止；可选 showTime/picker 变体 |
| DP-14 | L1 | 复现官方示例「多选」（`multiple.tsx`） | multiple 多日；size 档可建 |
| DP-15 | L1 | 复现官方示例「选择确认」（`needConfirm.tsx`） | 选日不立即 onChange；Confirm 后一次 |
| DP-16 | L1 | 复现官方示例「切换不同的选择器」（`switchable.tsx`） | SetPicker 切换后面板类型正确 |
| DP-17 | L1 | 复现官方示例「日期格式」（`format.tsx`） | format 展示串匹配 |
| DP-18 | L1 | 复现官方示例「日期时间选择」（`time.tsx`） | showTime 选出含时分 |
| DP-19 | L1 | 复现官方示例「格式对齐」（`mask.tsx`） | FormatMask 时 format 生效且可选日 |
| DP-20 | L2 | 读取 §6.2 关键尺寸/间距 | 高度 24/32/40、圆角 6、线宽 1（±0.5） |
| DP-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| DP-22 | L2 | disabled 外观 | 禁用色；无 hover 高亮 |
| DP-23 | L1 | 键盘/焦点主路径 | Focus ring 可见；Enter/Space 开；Esc 关 |
| DP-24 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| DP-25 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| DP-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。值类型 `DateValue` 对齐 dayjs 日历语义。

```text
// 值
type DateValue struct { Year, Month, Day, Hour, Minute, Second int; Valid, HasTime bool }

NewDatePicker() *DatePicker
NewRangePicker() *DatePicker   // Range=true

// 值
SetValue(DateValue) / GetValue() DateValue
SetDefaultValue(DateValue)
SetRangeValue(start, end DateValue) / GetRangeValue() (start, end DateValue)
SetMultiValue([]DateValue) / GetMultiValue() []DateValue
Clear()
SelectDate(DateValue)          // 程序选日（尊重 needConfirm / disabledDate）
SelectRange(start, end DateValue)
Confirm()                      // needConfirm OK
CancelPending()

// 形态 / 行为
SetPicker(DatePickerPicker)    // date|week|month|quarter|year
SetFormat(string)
SetFormatMask(bool)            // format type=mask
SetShowTime(bool)
SetNeedConfirm(bool)
SetMultiple(bool)
SetRange(bool)
SetOrder(bool)                 // default true
SetDisabledDate(func(DateValue) bool)
SetDisabled(bool)
SetSize(InputSize)
SetVariant(InputVariant)
SetStatus(InputStatus)
SetOpen(bool) / SetDefaultOpen(bool) / IsOpen() bool
SetPlacement(DatePickerPlacement)
SetAllowClear(bool)
SetPlaceholder(string) / SetRangePlaceholder(start, end string)
SetTheme(*Theme) / SetFace(text.Face)
SetAriaLabel(string)

// 回调
OnChange / OnChangeRange / OnChangeMulti
OnOpenChange / OnOk / OnClear

// 面板 / 展示
PanelYearMonth() (year int, month time.Month)
SetPanelMonth(year int, month time.Month)
DisplayText() string
FormatValue(DateValue) string
Popup() / TriggerShell() / Panel() / Node()
HandleKey(*core.KeyEvent)
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Picker | date |
| Size | middle（高 32） |
| Variant | outlined |
| Status | none |
| Disabled / Open / Multiple / Range / ShowTime / NeedConfirm / FormatMask | false |
| AllowClear | true |
| Order | true |
| Format | 随 picker：`YYYY-MM-DD` / `YYYY-wo` / `YYYY-MM` / `YYYY-[Q]Q` / `YYYY`；showTime 时追加 ` HH:mm:ss` |
| Placement | bottomLeft |
| 受控值 | 未 Set 时用 default* 或空 |

### 6.11 结构与绘制分层（实现提示）

```text
Column (Wrap)
  ├─ Pressable trigger (Decorated field)
  │    ├─ display text / range dual / multi summary
  │    ├─ clear?
  │    └─ suffix calendar icon
  └─ AnchoredPopup
       └─ panel (Decorated)
            ├─ header (prev/next · title)
            ├─ body (day|week|month|quarter|year grid)
            ├─ time columns? (showTime)
            └─ footer? (needConfirm OK)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **DatePicker 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/date-picker.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` DatePicker 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
