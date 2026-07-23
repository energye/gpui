# Calendar 日历
> 来源：[Ant Design 6.5.x Calendar](https://ant.design/components/calendar)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：按照日历形式展示数据的容器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

按照日历形式展示数据的容器。

**Calendar** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 通知事项日历 | 复现「通知事项日历」视觉与布局 |
| 跨日期事件 | 复现「跨日期事件」视觉与布局 |
| 卡片模式 | card 风格容器 |
| 选择功能 | 复现「选择功能」视觉与布局 |
| 农历日历 | 复现「农历日历」视觉与布局 |
| 周数 | 复现「周数」视觉与布局 |
| 自定义头部 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `fullscreen`

- **说明**：是否全屏显示
- **类型**：boolean
- **默认值**：true

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `mode`

- **说明**：初始模式
- **类型**：`month` | `year`
- **默认值**：`month`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `month` | 月 |
  | `year` | 年 |

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

当数据是日期或按照日期划分时，例如日程、课表、价格日历等，农历等。目前支持年/月切换。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **通知事项日历**（`notice-calendar.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **跨日期事件**（`event-range.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **卡片模式**（`card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **选择功能**（`select.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **农历日历**（`lunar.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **周数**（`week.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义头部**（`customize-header.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 展示日期 |
| `defaultValue` | 非受控默认值 | 默认展示的日期 |
| `onChange` | 值变化 | 日期变化回调 |
| `onSelect` | 选中 | 选择日期回调，包含来源信息 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 通知事项日历 | `notice-calendar.tsx` | 否 |
| 跨日期事件 | `event-range.tsx` | 否 |
| 卡片模式 | `card.tsx` | 否 |
| 选择功能 | `select.tsx` | 否 |
| 农历日历 | `lunar.tsx` | 否 |
| 周数 | `week.tsx` | 否 |
| 自定义头部 | `customize-header.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 如何在 Calendar 中使用自定义日期库 {#faq-customize-date-library}

参考 [使用自定义日期库](/docs/react/use-custom-date-library#calendar)。

### 如何给日期类组件配置国际化？ {#faq-set-locale-date-components}

参考 [如何给日期类组件配置国际化](/components/date-picker-cn#%E5%9B%BD%E9%99%85%E5%8C%96%E9%85%8D%E7%BD%AE)。

### 为什么时间类组件的国际化 locale 设置不生效？ {#faq-locale-not-working}

参考 FAQ [为什么时间类组件的国际化 locale 设置不生效？](/docs/react/faq#为什么时间类组件的国际化-locale-设置不生效)。

### 如何仅获取来自面板点击的日期？ {#faq-get-date-panel-click}

`onSelect` 事件提供额外的来源信息，你可以通过 `info.source` 来判断来源：

```tsx
 {
    if (source === 'date') {
      console.log('Panel Select:', source);
    }
  }}
/>
```

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

**注意**：Calendar 部分 locale 是从 value 中读取，所以请先正确设置 dayjs 的 locale。

```jsx
// 默认语言为 en-US，所以如果需要使用其他语言，推荐在入口文件全局设置 locale
// import dayjs from 'dayjs';
// import 'dayjs/locale/zh-cn';
// dayjs.locale('zh-cn');

<Calendar cellRender={cellRender} onPanelChange={onPanelChange} onSelect={onSelect} />
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| cellRender | 自定义单元格的内容 | function(current: dayjs, info: { prefixCls: string, originNode: React.ReactElement, today: dayjs, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | ~~dateFullCellRender~~ | 自定义渲染日期单元格，返回内容覆盖单元格，>= 5.4.0 请用 `fullCellRender` | function(date: Dayjs): ReactNode | - | < 5.4.0 | × |
| fullCellRender | 自定义单元格的内容 | function(current: dayjs, info: { prefixCls: string, originNode: React.ReactElement, today: dayjs, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| defaultValue | 默认展示的日期 | [dayjs](https://day.js.org/) | - | disabledDate | 不可选择的日期，参数为当前 `value`，注意使用时[不要直接修改](https://github.com/ant-design/ant-design/issues/30987) | (currentDate: Dayjs) => boolean | - | fullscreen | 是否全屏显示 | boolean | true | showWeek | 是否显示周数列 | boolean | false | 5.23.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | headerRender | 自定义头部内容 | function(object:{value: Dayjs, type: 'year' \| 'month', onChange: f(), onTypeChange: f()}) | - | locale | 国际化配置 | object | [(默认配置)](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | mode | 初始模式 | `month` \| `year` | `month` | validRange | 设置可以显示的日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | value | 展示日期 | [dayjs](https://day.js.org/) | - | onChange | 日期变化回调 | function(date: Dayjs) | - | onPanelChange | 日期面板变化回调 | function(date: Dayjs, mode: string) | - | onSelect | 选择日期回调，包含来源信息 | function(date: Dayjs, info: { source: 'year' \| 'month' \| 'date' \| 'customize' }) | - | `info`: 5.6.0 | × |

### 导入方式

```js
import { Calendar } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `cellRender` | 自定义单元格的内容 | function(current: dayjs, info: { prefixCls: string, originNode: React.ReactElement, today: dayjs, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dateFullCellRender` | 自定义渲染日期单元格，返回内容覆盖单元格，>= 5.4.0 请用 `fullCellRender` | function(date: Dayjs): ReactNode | - | < 5.4.0 |
| `fullCellRender` | 自定义单元格的内容 | function(current: dayjs, info: { prefixCls: string, originNode: React.ReactElement, today: dayjs, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 |
| `defaultValue` | 默认展示的日期 | [dayjs](https://day.js.org/) | - | — |
| `disabledDate` | 不可选择的日期，参数为当前 `value`，注意使用时[不要直接修改](https://github.com/ant-design/ant-design/issues/30987) | (currentDate: Dayjs) => boolean | - | — |
| `fullscreen` | 是否全屏显示 | boolean | true | — |
| `showWeek` | 是否显示周数列 | boolean | false | 5.23.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `headerRender` | 自定义头部内容 | function(object:{value: Dayjs, type: 'year' \| 'month', onChange: f(), onTypeChange: f()}) | - | — |
| `locale` | 国际化配置 | object | [(默认配置)](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | — |
| `mode` | 初始模式 | `month` \| `year` | `month` | — |
| `validRange` | 设置可以显示的日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | — |
| `value` | 展示日期 | [dayjs](https://day.js.org/) | - | — |
| `onChange` | 日期变化回调 | function(date: Dayjs) | - | — |
| `onPanelChange` | 日期面板变化回调 | function(date: Dayjs, mode: string) | - | — |
| `onSelect` | 选择日期回调，包含来源信息 | function(date: Dayjs, info: { source: 'year' \| 'month' \| 'date' \| 'customize' }) | - | `info`: 5.6.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Calendar** 的验收清单：

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
- 官方文档：https://ant.design/components/calendar
- 中文文档：https://ant.design/components/calendar-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/calendar
- 驱动 gpui kit：`calendar`
