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
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | — | × |
| ~~dateFullCellRender~~ | 自定义渲染日期单元格，返回内容覆盖单元格，>= 5.4.0 请用 `fullCellRender` | function(date: Dayjs): ReactNode | - | < 5.4.0 | × |
| fullCellRender | 自定义单元格的内容 | function(current: dayjs, info: { prefixCls: string, originNode: React.ReactElement, today: dayjs, range?: 'start' \| 'end', type: PanelMode, locale?: Locale, subType?: 'hour' \| 'minute' \| 'second' \| 'meridiem' }) => React.ReactNode | - | 5.4.0 | × |
| defaultValue | 默认展示的日期 | [dayjs](https://day.js.org/) | - | — | × |
| disabledDate | 不可选择的日期，参数为当前 `value`，注意使用时[不要直接修改](https://github.com/ant-design/ant-design/issues/30987) | (currentDate: Dayjs) => boolean | - | — | × |
| fullscreen | 是否全屏显示 | boolean | true | — | × |
| showWeek | 是否显示周数列 | boolean | false | 5.23.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | — | × |
| headerRender | 自定义头部内容 | function(object:{value: Dayjs, type: 'year' \| 'month', onChange: f(), onTypeChange: f()}) | - | — | × |
| locale | 国际化配置 | object | [(默认配置)](https://github.com/ant-design/ant-design/blob/master/components/date-picker/locale/example.json) | — | × |
| mode | 初始模式 | `month` \| `year` | `month` | — | × |
| validRange | 设置可以显示的日期 | \[[dayjs](https://day.js.org/), [dayjs](https://day.js.org/)] | - | — | × |
| value | 展示日期 | [dayjs](https://day.js.org/) | - | — | × |
| onChange | 日期变化回调 | function(date: Dayjs) | - | — | × |
| onPanelChange | 日期面板变化回调 | function(date: Dayjs, mode: string) | - | — | × |
| onSelect | 选择日期回调，包含来源信息 | function(date: Dayjs, info: { source: 'year' \| 'month' \| 'date' \| 'customize' }) | - | `info`: 5.6.0 | × |

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
源码：`components/calendar/style/index.ts`（`prepareComponentToken` + `dateValueHeight` / `weekHeight` / `dateContentHeight`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 middle | **14** | `fontSize` |
| 小字号（周头 / 次级） | **12** | `fontSizeSM` |
| 圆角（根） | **6** | `borderRadius` |
| 卡片模式圆角 | **8** | `borderRadiusLG`（`fullscreen=false`） |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |
| year 选择宽 | **80** | 组件 token `yearControlWidth` |
| month 选择宽 | **70** | 组件 token `monthControlWidth` |
| 迷你面板内容高 | **256** | 组件 token `miniContentHeight` |
| 日期值行高 | **24** | `controlHeightSM`（`dateValueHeight`） |
| 周头行高 | **18** | `controlHeightSM × 0.75`（`weekHeight`） |
| 全屏日期内容区高 | ≈ **92** | `(fontHeightSM+marginXS)×3 + lineWidth×2` |
| 卡片模式单元格 | **≈24–28** 方格 | 对齐 picker mini cell |
| 全屏日期单元格最小高 | **≥ dateValueHeight + content** | 可放 cellRender 内容 |
| Header 垂直 pad | **12** | `paddingSM` |
| 面板 body 垂直 pad | **8** | `paddingXS` |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / 今日强调 / 选中字 | `colorPrimary` + 变体 | 今日边框、选中日期值 |
| 选中单元格底 | `controlItemBgActive` / `colorPrimaryBg` | `itemActiveBg` |
| hover 单元格底 | `controlItemBgHover` / `colorBgTextHover` | |
| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | cellRender Badge 等 |
| 文本 / 次级 / 三级 | `colorText` / `Secondary` / `Tertiary` | 周头、农历副文 |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | fullBg / fullPanelBg |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 他月日期 | 文本 tertiary / 降透明 | `!in-view` |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（kit 映射） | 默认 |
| --- | --- | --- | --- |
| `value` | 受控展示/选中日期 | `DateValue` | —（未控时用 default / 今天） |
| `defaultValue` | 非受控默认日期 | `DateValue` | 今天 |
| `mode` | 面板模式 | `month` \| `year` | `month` |
| `fullscreen` | 全屏 vs 卡片 | `bool` | `true` |
| `showWeek` | 周数列 | `bool` | `false` |
| `disabledDate` | 禁选日 | `func(DateValue) bool` | — |
| `validRange` | 可显示日期闭区间 | `[2]DateValue` | — |
| `cellRender` | 单元格附加内容 | `func(DateValue, CellInfo) Node` | — |
| `fullCellRender` | 整格覆盖渲染 | `func(DateValue, CellInfo) Node` | — |
| `headerRender` | 自定义头部 | `func(HeaderConfig) Node` | 默认年/月选择 + mode |
| `onChange` | 值变化（与旧值不同日） | `func(DateValue)` | — |
| `onSelect` | 选中（含 source） | `func(DateValue, source)` | — |
| `onPanelChange` | 面板年月或 mode 变化 | `func(DateValue, mode)` | — |
| `disabled` / `loading` | 整表禁用 / 加载 | `bool` | `false` |
| `classNames` / `styles` | 语义节点 | — | **P1** |
| `locale` | 国际化文案 | object | **P1**（P0 英文简写） |

**配置优先级（通用）：** 受控 props（`value`）> 显式非受控 `default*` > 组件默认（今天）> ConfigProvider 全局默认（P1）。

**mode ↔ 面板：**

| `mode` | 面板内容 | 选中语义 |
| --- | --- | --- |
| `month` | 日期格（日面板） | 选日 → source=`date` |
| `year` | 12 月格（月面板） | 选月 → source=`month`，并切回 `month` 模式（antd 行为） |

### 6.4 交互状态机（L1）

```text
mount ──► value=defaultValue|today · mode=month · panel=value 年月
  │
  ├─ 点日期格 ──► (若 !disabledDate && !disabled && !loading)
  │                 onSelect(date, source=date)
  │                 非受控：value=date；受控：仅回调，等 SetValue
  │                 若日变化：onChange(date)
  │                 若跨月/年：onPanelChange + 同步 panel
  │
  ├─ 点月份格（mode=year）──► onSelect(monthDate, source=month)
  │                            切 mode→month；更新 panel 年月
  │
  ├─ 头：改年 / 改月 ──► panel 变；onPanelChange；可触发 onSelect(source=year|month|customize)
  ├─ 头：mode 切换 ──► mode 变；onPanelChange(value, mode)
  │
  └─ SetDisabled / SetLoading ──► 吞选择；loading 可挂 Ticker 指示
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CAL-S1 | 点可选日 | 触发 `onSelect`（source=date）；非受控更新 value；日变则 `onChange` |
| CAL-S2 | `mode=year` | 展示 12 月格；点月后回 `month` 面板 |
| CAL-S3 | `disabledDate` / `validRange` 外 | 不可选；不触发 onSelect/onChange |
| CAL-S4 | `fullscreen=false` | 卡片几何（圆角 LG、迷你单元格、内容高≈256） |
| CAL-S5 | 受控 `value` | 交互只回调；展示值以 `SetValue` 为准 |
| CAL-S6 | 切月/切年（头或跨月选日） | `panelYear/panelMonth` 变；`onPanelChange` |
| CAL-S7 | `showWeek=true` | 日期面板左侧周数列（ISO 周） |
| CAL-S8 | `cellRender` / `fullCellRender` | 附加内容 / 整格替换生效 |
| CAL-S9 | `headerRender` | 替换默认头；`onChange`/`onTypeChange` 可用 |
| CAL-S10 | `loading` / 整表 `disabled` | 不触发选择；loading 有指示 |

**日期算法（对齐 `generateCalendar.tsx` + `Header.tsx`）：**

| 项 | 规则 |
| --- | --- |
| 周编号 | `showWeek=true` 时日期面板左侧加周数列；周编号按 ISO 周计算（周一起算的年第几周，与 rc-picker 周列一致） |
| 周起始 | 周头 Su…Sa 的起始日跟随 locale（`dayjs.locale` 的 `weekStart`，如 `zh-cn` 为周一、`en` 为周日）；kit P0 允许固定周一起算并在 Notes 注明，完整 locale 周起始为 P1 |
| `validRange` × `disabledDate` 组合 | 最终禁选 = 越界 ∪ 回调：`mergedDisabled(date) = (validRange && (date < range[0] \|\| date > range[1])) \|\| disabledDate?.(date)`；越界日与 `disabledDate=true` 的日都不触发 `onSelect`/`onChange`；Header 年/月下拉的可选项同样按 `validRange` 裁剪（年列取 `range[0].year … range[1].year`，月列按当前年裁起止月并在切年时钳制月份） |

**Header 前置依赖：** 默认头部由年 `Select` + 月 `Select` + 模式 `Radio.Group/Button` 组成（`Header.tsx`：`YearSelect`/`MonthSelect` 用 Select，`ModeSwitch` 用 Radio Group），kit 实现 Calendar 默认头前置依赖同库 Select 与 Radio.Button；若两者任一缺失，默认头降级为 `headerRender` 注入或纯文本年月，P0 用例 CAL-07/CAL-15 按降级路径验收并在 Notes 注明。

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token；容器底 `colorBgContainer` |
| 今日 | 日期值强调 / 边框 `colorPrimary`（全屏顶边或 mini 描边） |
| 选中 | 底 `itemActiveBg`；日期值可用 `colorPrimary` |
| hover | 可交互格 `controlItemBgHover` |
| 他月 | 降对比（tertiary） |
| disabled 日 / 整表 | 禁用色；无 hover 高亮 |
| loading | 指示器 + 吞选择 |
| 主题切换 | 色与间距随 Theme 更新 |

**动效：** 面板切换 P0 瞬时；尊重 reduced-motion。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `grid`（或等价）；日/月格可激活 |
| 名称 | `AriaLabel` 可设；格有可读日期名 |
| 焦点 | 可聚焦格 Focus ring 可见（§6.2） |
| 键盘 | 聚焦格 Space/Enter 选中（主路径） |
| 选中态 | 读屏可感知 selected（若平台支持） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1 / §6.4） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 头 Select/Radio 外观 | **近似**（kit Select + Radio.Button） | P0 |
| 农历算法库 | **映射**：业务 `fullCellRender` 注入；kit 不内置农历 | P0 钩子 |
| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |
| locale 完整文案表 | **分期**（P0 英文简写） | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `value` / `defaultValue` / 受控 | `DateValue`；默认今天 |
| `onChange` / `onSelect` / `onPanelChange` | 含 source |
| `mode` month\|year | 日面板 / 月面板 |
| `fullscreen` | 全屏默认 true；卡片 false |
| `showWeek` | 周数列 |
| `disabledDate` / `validRange` | 禁日 |
| `cellRender` / `fullCellRender` | 通知/跨日事件/农历钩子 |
| `headerRender` | 自定义头部 |
| `disabled` / `loading`（Ticker） | 状态 |
| 官方主路径示例 | 基本、通知事项日历、跨日期事件、卡片模式、选择功能、农历日历、周数、自定义头部 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | style-class / _semantic |
| 完整 locale 文案 / 周起始可配 | 分期 |
| 内置农历算法 | 业务注入即可 |
| 动画像素级 | 分期 |
| ConfigProvider 全局 calendar 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestCalendar_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Calendar 完成 1:1 主路径。  
> L3/L4（CAL-20/21）与 P1（CAL-22）本阶段不强制进 `TestCalendar_PRD_*`。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CAL-01 | L1 | NewCalendar 默认创建 | 不崩溃；mode=month、fullscreen=true、value≈今天 |
| CAL-02 | L1 | 选日 | onSelect(source=date)；非受控 value 更新；onChange |
| CAL-03 | L1 | mode=year | 年/月面板；Mode()=year |
| CAL-04 | L1 | disabledDate | 禁日不可点；无回调 |
| CAL-05 | L1 | fullscreen=false | IsFullscreen=false；卡片几何 |
| CAL-06 | L1 | 受控 value | 选日只回调；Value 待 SetValue |
| CAL-07 | L1 | 切月 | 面板月份变；onPanelChange |
| CAL-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 可挂 onPanelChange；布局成功 |
| CAL-09 | L1 | 复现官方示例「通知事项日历」（`notice-calendar.tsx`） | cellRender 注入成功 |
| CAL-10 | L1 | 复现官方示例「跨日期事件」（`event-range.tsx`） | defaultValue + cellRender |
| CAL-11 | L1 | 复现官方示例「卡片模式」（`card.tsx`） | fullscreen=false |
| CAL-12 | L1 | 复现官方示例「选择功能」（`select.tsx`） | 受控 value + onSelect |
| CAL-13 | L1 | 复现官方示例「农历日历」（`lunar.tsx`） | fullCellRender 钩子 |
| CAL-14 | L1 | 复现官方示例「周数」（`week.tsx`） | showWeek 两侧布局 |
| CAL-15 | L1 | 复现官方示例「自定义头部」（`customize-header.tsx`） | headerRender 替换头 |
| CAL-16 | L2 | 读取 §6.2 关键尺寸/间距 | 字号/圆角/yearWidth/monthWidth/miniH 等 ±0.5 |
| CAL-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| CAL-18 | L2 | disabled 外观 | 禁用色；禁日无 hover 高亮 |
| CAL-19 | L1 | 键盘/焦点主路径 | 可聚焦格 Focus ring；Enter/Space 选中 |
| CAL-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| CAL-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| CAL-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
NewCalendar() *Calendar

// 值 / 面板
SetValue(DateValue)                 // 受控写；不触发 OnChange
SetDefaultValue(DateValue)          // 非受控初始
GetValue() DateValue
SetMode(CalendarMode)               // month | year
Mode() CalendarMode
SetPanelMonth(year, month)          // 1..12；触发 onPanelChange
PanelYear() / PanelMonth() int
SelectDate(DateValue)               // 程序选日（走 onSelect/onChange 规则）
SelectDay(day int)                  // 当前 panel 月内选日

// 形态
SetFullscreen(bool)                 // 默认 true
SetShowWeek(bool)
SetDisabledDate(func(DateValue) bool)
SetValidRange(start, end DateValue)
SetCellRender(func(DateValue, CalendarCellInfo) core.Node)
SetFullCellRender(func(DateValue, CalendarCellInfo) core.Node)
SetHeaderRender(func(CalendarHeaderConfig) core.Node)

// 状态
SetDisabled(bool)
SetLoading(bool)                    // Ticker 指示
SetControlled(bool)

// 回调
SetOnChange(func(DateValue))
SetOnSelect(func(DateValue, CalendarSelectSource))
SetOnPanelChange(func(DateValue, CalendarMode))

// 主题 / 覆盖 / a11y
SetTheme(*Theme) / Theme 字段
SetFace(text.Face)
Style 可选覆盖
SetAriaLabel(string)

// 挂树
Node() core.Node                    // Root 身份跨 rebuild 稳定
AttachTicker(*core.Tree)            // loading
HeaderNode() / BodyNode()           // 测试钩子
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Value | `defaultValue` 或 **今天** |
| Mode | `month` |
| Fullscreen | `true` |
| ShowWeek | `false` |
| Disabled / Loading / Controlled | `false` |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Flex Column (Root, role=grid)          ← 身份稳定
  ├─ Header（默认：年 Select · 月 Select · mode Radio）
  │     或 HeaderRender 节点
  └─ Body
        ├─ 周头行（Su…Sa；可选周列占位）
        └─ mode=month: 6×7 日格（+ 周数列）
           mode=year:  4×3 月格
        日格 = Pressable → 默认 inner | FullCellRender | + CellRender 内容
```

- 组合 `ui/primitive` + 少量 kit（Select / Radio / Button）；禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；Root 不清毁身份。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- Loading 用 `Tree.AddTicker` + `MarkNeedsPaint`；禁止 ContinuousRender。  

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
