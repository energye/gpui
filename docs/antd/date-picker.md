# DatePicker 日期选择框
> 来源：[Ant Design 6.5.x DatePicker](https://ant.design/components/date-picker)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：输入或选择日期的控件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
| className | 选择器 className | string | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | dateRender | 自定义日期单元格的内容，5.4.0 起用 `cellRender` 代替 | function(currentDate: dayjs, today: dayjs) => React.ReactNode | - | < 5.4.0 | × |
| cellRender | 自定义单元格的内容 | (current: dayjs, info: { originNode: React.ReactElement,today: DateType, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| components | 自定义面板 | Record<Panel \| 'input', React.ComponentType> | - | 5.14.0 | × |
| defaultOpen | 是否默认展开控制弹层 | boolean | - | disabled | 禁用 | boolean | false | disabledDate | 不可选择的日期 | (currentDate: dayjs, info: { from?: dayjs, type: Picker }) => boolean | - | `info`: 5.14.0 | × |
| ~~dropdownClassName~~ | 弹出日历的 className，请使用 `classNames.popup.root` 替代 | string | - | - | × |
| format | 设置日期格式，为数组时支持多格式匹配，展示以第一个为准。配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。示例：[自定义格式](#date-picker-demo-format) | [formatType](#formattype) | [@rc-component/picker](https://github.com/react-component/picker/blob/f512f18ed59d6791280d1c3d7d37abbb9867eb0b/src/utils/uiUtil.ts#L155-L177) | order | 多选、范围时是否自动排序 | boolean | true | 5.14.0 | × |
| preserveInvalidOnBlur | 失去焦点是否要清空输入框内无效内容 | boolean | false | 5.14.0 | × |
| ~~popupClassName~~ | 额外的弹出日历 className，使用 `classNames.popup.root` 替代 | string | - | 4.23.0 | × |
| getPopupContainer | 定义浮层的容器，默认为 body 上新建 div | function(trigger) | - | inputReadOnly | 设置输入框为只读（避免在移动设备上打开虚拟键盘） | boolean | false | locale | 国际化配置 | object | [默认配置](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | minDate | 最小日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 | × |
| maxDate | 最大日期，同样会限制面板的切换范围 | dayjs | - | 5.14.0 | × |
| mode | 日期面板的状态（[设置后无法选择年份/月份？](/docs/react/faq#当我指定了-datepickerrangepicker-的-mode-属性后点击后无法选择年份月份)） | `time` \| `date` \| `month` \| `year` \| `decade` | - | needConfirm | 是否需要确认按钮，为 `false` 时失去焦点即代表选择。当设置 `multiple` 时默认为 `false` | boolean | - | 5.14.0 | × |
| nextIcon | 自定义下一个图标 | ReactNode | - | 4.17.0 | × |
| open | 控制弹层是否展开 | boolean | - | panelRender | 自定义渲染面板 | (panelNode) => ReactNode | - | 4.5.0 | × |
| picker | 设置选择器类型 | `date` \| `week` \| `month` \| `quarter` \| `year` | `date` | `quarter`: 4.1.0 | × |
| placeholder | 输入框提示文字 | string \| \[string, string] | - | placement | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | ~~popupStyle~~ | 额外的弹出日历样式，使用 `styles.popup.root` 替代 | CSSProperties | {} | prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| prevIcon | 自定义上一个图标 | ReactNode | - | 4.17.0 | × |
| previewValue | 当用户选择日期悬停选项时，输入字段的值会发生临时更改 | false \| hover | hover | 6.0.0 | × |
| presets | 预设时间范围快捷选择, 自 `5.8.0` 起 value 支持函数返回值 | { label: React.ReactNode, value: Dayjs \| (() => Dayjs) }\[] | - | size | 输入框大小，`large` 高度为 40px，`small` 为 24px，默认是 32px | `large` \| `medium` \| `small` | - | status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| style | 自定义输入框样式 | CSSProperties | {} | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | suffixIcon | 自定义的选择框后缀图标 | ReactNode | - | superNextIcon | 自定义 `>>` 切换图标 | ReactNode | - | 4.17.0 | × |
| superPrevIcon | 自定义 `<<` 切换图标 | ReactNode | - | 4.17.0 | × |
| clearIcon | （仅支持全局配置）自定义清除图标 | ReactNode | - | × | 6.4.0 |
| variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | DatePicker: 5.19.0，RangePicker: 5.19.0 |
| onClear | 点击清除按钮时的回调 | () => void | - | 6.5.0 | × |
| onOpenChange | 弹出日历和关闭日历的回调 | function(open) | - | onPanelChange | 日历面板切换的回调 | function(value, mode) | - | ~~onSelect~~ | 选中日期时的回调，请使用 `onCalendarChange` 替代 | function(value) | - | - | × |

### 共同的方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### DatePicker

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultPickerValue | 默认面板日期，每次面板打开时会被重置到该日期 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| defaultValue | 默认日期，如果开始时间或结束时间为 `null` 或者 `undefined`，日期范围将是一个开区间 | [dayjs](https://day.js.org/) | - | format | 展示的日期格式，配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。 | [formatType](#formattype) | `YYYY-MM-DD` | pickerValue | 面板日期，可以用于受控切换面板所在日期。配合 `onPanelChange` 使用。 | [dayjs](https://day.js.org/) | - | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | (mode) => React.ReactNode | - | showTime | 增加时间选择功能 | Object \| boolean | [TimePicker Options](/components/time-picker-cn#api) | showTime.defaultOpenValue | 设置用户选择日期时默认的时分秒，[例子](#date-picker-demo-disabled-date) | [dayjs](https://day.js.org/) | dayjs() | value | 日期 | [dayjs](https://day.js.org/) | - | onOk | 点击确定按钮的回调 | function() | - 
### DatePicker\[picker=year]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | value | 日期 | [dayjs](https://day.js.org/) | - 
### DatePicker\[picker=quarter]

`4.1.0` 新增。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | value | 日期 | [dayjs](https://day.js.org/) | - 
### DatePicker\[picker=month]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | value | 日期 | [dayjs](https://day.js.org/) | - 
### DatePicker\[picker=week]

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认日期 | [dayjs](https://day.js.org/) | - | multiple | 是否为多选 | boolean | false | 5.14.0 |
| renderExtraFooter | 在面板中添加额外的页脚 | (mode) => React.ReactNode | - | value | 日期 | [dayjs](https://day.js.org/) | - | showWeek | DatePicker 下展示当前周 | boolean | true | 5.14.0 |

### RangePicker

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowEmpty | 允许起始项部分为空 | \[boolean, boolean] | \[false, false] | cellRender | 自定义单元格的内容。 | (current: dayjs, info: { originNode: React.ReactElement,today: DateType, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| dateRender | 自定义日期单元格的内容，5.4.0 起用 `cellRender` 代替 | function(currentDate: dayjs, today: dayjs) => React.ReactNode | - | < 5.4.0 | × |
| defaultPickerValue | 默认面板日期，每次面板打开时会被重置到该日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | 5.14.0 | × |
| defaultValue | 默认日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | disabled | 禁用起始项 | \[boolean, boolean] | - | disabledTime | 不可选择的时间 | function(date: dayjs, partial: `start` \| `end`, info: { from?: dayjs }) | - | `info.from`: 5.17.0 | × |
| format | 展示的日期格式，配置参考 [dayjs#format](https://day.js.org/docs/zh-CN/display/format#%E6%94%AF%E6%8C%81%E7%9A%84%E6%A0%BC%E5%BC%8F%E5%8C%96%E5%8D%A0%E4%BD%8D%E7%AC%A6%E5%88%97%E8%A1%A8)。 | [formatType](#formattype) | `YYYY-MM-DD HH:mm:ss` | id | 设置输入框 `id` 属性。 | { start?: string, end?: string } | - | 5.14.0 | × |
| pickerValue | 面板日期，可以用于受控切换面板所在日期。配合 `onPanelChange` 使用。 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | 5.14.0 | × |
| presets | 预设时间范围快捷选择，自 `5.8.0` 起 value 支持函数返回值 | { label: React.ReactNode, value: \[(Dayjs \| (() => Dayjs)), (Dayjs \| (() => Dayjs))] }\[] | - | renderExtraFooter | 在面板中添加额外的页脚 | () => React.ReactNode | - | separator | 设置分隔符 | React.ReactNode | `<SwapRightOutlined />` | showTime | 增加时间选择功能 | Object\|boolean | [TimePicker Options](/components/time-picker-cn#api) | ~~showTime.defaultValue~~ | 请使用 `showTime.defaultOpenValue` | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | \[dayjs(), dayjs()] | 5.27.3 | × |
| showTime.defaultOpenValue | 设置用户选择日期时默认的时分秒，[例子](#date-picker-demo-disabled-date) | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | \[dayjs(), dayjs()] | value | 日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | onCalendarChange | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | onChange | 日期范围发生变化的回调 | function(dates: \[dayjs, dayjs] \| null, dateStrings: \[string, string] \| null) | - | onFocus | 聚焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 | × |
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
| `showWeek` | DatePicker 下展示当前周 | boolean | false | 5.14.0 |
| `value` | 日期 | [dayjs](https://day.js.org/) | - | — |
| `onChange` | 时间发生变化的回调 | function(date: dayjs \| null, dateString: string \| null) | - | — |
| `onOk` | 点击确定按钮的回调 | function() | - | — |
| `tagRender` | 自定义 tag 内容 render，仅在 `multiple` 模式下生效 | (props) => ReactNode | - | 6.4.0 |
| `allowEmpty` | 允许起始项部分为空 | \[boolean, boolean] | \[false, false] | — |
| `id` | 设置输入框 `id` 属性。 | { start?: string, end?: string } | - | 5.14.0 |
| `separator` | 设置分隔符 | React.ReactNode | `` | — |
| `onCalendarChange` | 待选日期发生变化的回调。`info` 参数自 4.4.0 添加 | function(dates: \[dayjs, dayjs], dateStrings: \[string, string], info: { range:`start`\|`end` }) | - | — |
| `onFocus` | 聚焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 |
| `onBlur` | 失焦时回调 | function(event, { range: 'start' \| 'end' }) | - | `range`: 5.14.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **DatePicker** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **25** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/date-picker
- 中文文档：https://ant.design/components/date-picker-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/date-picker
- 驱动 gpui kit：`date-picker`
