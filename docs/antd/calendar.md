# Calendar 日历
> 来源：[Ant Design 6.5.x Calendar](https://ant.design/components/calendar)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：按照日历形式展示数据的容器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Calendar** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/calendar/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Calendar）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Calendar）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：按照日历形式展示数据的容器。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `cellRender` | 自定义单元格的内容 | function(current: dayjs, info: { pref… | 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \ |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `fullCellRender` | 自定义单元格的内容 | function(current: dayjs, info: { pref… | 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \ |
| `defaultValue` | 默认展示的日期 | [dayjs](https://day.js.org/) | - |
| `disabledDate` | 不可选择的日期，参数为当前 `value`，注意使用时[不要直接修改](https://github.com/an… | (currentDate: Dayjs) => boolean | - |
| `fullscreen` | 是否全屏显示 | boolean | true |
| `showWeek` | 是否显示周数列 | boolean | false |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `headerRender` | 自定义头部内容 | function(object:{value: Dayjs, type: … | 'month', onChange: f(), onTypeChange: f()}) |
| `locale` | 国际化配置 | object | [(默认配置)](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) |
| `mode` | 初始模式 | `month` \ | `year` |
| `validRange` | 设置可以显示的日期 | \[[dayjs](https://day.js.org/), [dayj… | - |
| `value` | 展示日期 | [dayjs](https://day.js.org/) | - |
| `onChange` | 日期变化回调 | function(date: Dayjs) | - |
| `onPanelChange` | 日期面板变化回调 | function(date: Dayjs, mode: string) | - |
| `onSelect` | 选择日期回调，包含来源信息 | function(date: Dayjs, info: { source:… | 'month' \ |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
value ── 选日 ──► onSelect/onChange
mode month|year 切换面板
disabledDate 禁日
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CAL-S1 | 选日 | onSelect |
| CAL-S2 | mode=year | 年面板 |
| CAL-S3 | disabledDate | 禁日不可点 |
| CAL-S4 | fullscreen=false | 卡片模式 |
| CAL-S5 | 受控 value | 外部 |
| CAL-S6 | 切月 | 面板月份变 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 表格/树/列表 | 结构角色与展开/选中态可读 |
| 排序/筛选 | 控件有名 |

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
| `mode` | 必须 |
| 官方主路径示例 | 基本、通知事项日历、跨日期事件、卡片模式、选择功能、农历日历、周数、自定义头部 |
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
| 其余示例 | 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestCalendar_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Calendar 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CAL-01 | L1 | NewCalendar 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| CAL-02 | L1 | 选日 | onSelect |
| CAL-03 | L1 | mode=year | 年面板 |
| CAL-04 | L1 | disabledDate | 禁日不可点 |
| CAL-05 | L1 | fullscreen=false | 卡片模式 |
| CAL-06 | L1 | 受控 value | 外部 |
| CAL-07 | L1 | 切月 | 面板月份变 |
| CAL-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-09 | L1 | 复现官方示例「通知事项日历」（`notice-calendar.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-10 | L1 | 复现官方示例「跨日期事件」（`event-range.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-11 | L1 | 复现官方示例「卡片模式」（`card.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-12 | L1 | 复现官方示例「选择功能」（`select.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-13 | L1 | 复现官方示例「农历日历」（`lunar.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-14 | L1 | 复现官方示例「周数」（`week.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-15 | L1 | 复现官方示例「自定义头部」（`customize-header.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CAL-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| CAL-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| CAL-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| CAL-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| CAL-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| CAL-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| CAL-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewCalendar(...) *Calendar

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
Data view
  ├─ header?
  ├─ body rows/nodes
  └─ pagination/footer?
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Calendar 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/calendar.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Calendar 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
