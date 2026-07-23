# Statistic 统计数值
> 来源：[Ant Design 6.5.x Statistic](https://ant.design/components/statistic)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：展示统计数值。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

展示统计数值。

**Statistic** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 单位 | 复现「单位」视觉与布局 |
| 动画效果 | 复现「动画效果」视觉与布局 |
| 在卡片中使用 | card 风格容器 |
| 计时器 | 复现「计时器」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义 Statistic 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `loading`

- **说明**：数值是否加载中
- **类型**：boolean
- **默认值**：false
- **版本**：4.8.0

#### `prefix`

- **说明**：设置数值的前缀
- **类型**：ReactNode
- **默认值**：-

#### `styles`

- **说明**：用于自定义 Statistic 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `suffix`

- **说明**：设置数值的后缀
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：数值的标题
- **类型**：ReactNode
- **默认值**：-

#### `valueStyle`

- **说明**：设置数值区域的样式，请使用 `styles.content` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `type`

- **说明**：计时类型，倒计时或者正计时
- **类型**：`countdown` | `countup`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `countdown` | 官方取值 `countdown` |
  | `countup` | 官方取值 `countup` |

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

- 当需要突出某个或某组数字时。
- 当需要展示带描述的统计类数据时使用。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **单位**（`unit.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **动画效果**（`animated.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **在卡片中使用**（`card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **计时器**（`timer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 数值内容 |
| `onChange` | 值变化 | 倒计时时间变化时触发 |
| `loading` | 加载中 | 数值是否加载中 |
| `onFinish` | 提交成功 | 倒计时完成时触发 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 单位 | `unit.tsx` | 否 |
| 动画效果 | `animated.tsx` | 否 |
| 在卡片中使用 | `card.tsx` | 否 |
| 计时器 | `timer.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

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

#### Statistic

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义 Statistic 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | decimalSeparator | 设置小数点 | string | `.` | formatter | 自定义数值展示 | (value) => ReactNode | - | groupSeparator | 设置千分位标识符 | string | `,` | loading | 数值是否加载中 | boolean | false | 4.8.0 | × |
| precision | 数值精度 | number | - | prefix | 设置数值的前缀 | ReactNode | - | styles | 用于自定义 Statistic 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> | - | suffix | 设置数值的后缀 | ReactNode | - | title | 数值的标题 | ReactNode | - | value | 数值内容 | string \| number | - | ~~valueStyle~~ | 设置数值区域的样式，请使用 `styles.content` 替代 | CSSProperties | - 
#### Statistic.Countdown <Badge type="error">Deprecated</Badge>

<Antd component="Alert" title="版本 >= 5.25.0 时请使用 Statistic.Timer 作为替代方案。" type="warning" banner="true"></Antd>

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| format | 格式化倒计时展示，参考 [dayjs](https://day.js.org/) | string | `HH:mm:ss` | suffix | 设置数值的后缀 | ReactNode | - | value | 数值内容 | number | - | onFinish | 倒计时完成时触发 | () => void | - 
#### Statistic.Timer 

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| type | 计时类型，倒计时或者正计时 | `countdown` \| `countup` | - | prefix | 设置数值的前缀 | ReactNode | - | title | 数值的标题 | ReactNode | - | valueStyle | 设置数值区域的样式 | CSSProperties | - | onChange | 倒计时时间变化时触发 | (value: number) => void | - 
```js
import { Statistic } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义 Statistic 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `decimalSeparator` | 设置小数点 | string | `.` | — |
| `formatter` | 自定义数值展示 | (value) => ReactNode | - | — |
| `groupSeparator` | 设置千分位标识符 | string | `,` | — |
| `loading` | 数值是否加载中 | boolean | false | 4.8.0 |
| `precision` | 数值精度 | number | - | — |
| `prefix` | 设置数值的前缀 | ReactNode | - | — |
| `styles` | 用于自定义 Statistic 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `suffix` | 设置数值的后缀 | ReactNode | - | — |
| `title` | 数值的标题 | ReactNode | - | — |
| `value` | 数值内容 | string \| number | - | — |
| `valueStyle` | 设置数值区域的样式，请使用 `styles.content` 替代 | CSSProperties | - | — |
| `format` | 格式化倒计时展示，参考 [dayjs](https://day.js.org/) | string | `HH:mm:ss` | — |
| `onFinish` | 倒计时完成时触发 | () => void | - | — |
| `onChange` | 倒计时时间变化时触发 | (value: number) => void | - | — |
| `type` | 计时类型，倒计时或者正计时 | `countdown` \| `countup` | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Statistic** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **6** 个，均需可复现。
12. **表单专项**：rules、dependencies、scrollToFirstError。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/statistic
- 中文文档：https://ant.design/components/statistic-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/statistic
- 驱动 gpui kit：`statistic`
