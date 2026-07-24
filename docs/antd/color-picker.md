# ColorPicker 颜色选择器
> 来源：[Ant Design 6.5.x ColorPicker](https://ant.design/components/color-picker)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于选择颜色。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于选择颜色。

**ColorPicker** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 触发器尺寸大小 | 不同 size 档位的高宽/字号/内边距 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 渐变色 | 渐变填充 |
| 渲染触发器文本 | 复现「渲染触发器文本」视觉与布局 |
| 禁用 | disabled 灰态与不可点 |
| 禁用透明度 | disabled 灰态与不可点 |
| 清除颜色 | 语义色/预设色 |
| 自定义触发器 | 自定义渲染/插槽外观 |
| 自定义触发事件 | 自定义渲染/插槽外观 |
| 颜色编码 | 语义色/预设色 |
| 预设颜色 | 语义色/预设色 |
| 自定义面板 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：允许清除选择的颜色
- **类型**：boolean
- **默认值**：false

#### `children`

- **说明**：颜色选择器的触发器
- **类型**：React.ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `defaultValue`

- **说明**：颜色默认的值
- **类型**：[ColorType](#colortype)
- **默认值**：-

#### `defaultFormat`

- **说明**：颜色格式默认的值
- **类型**：`rgb` | `hex` | `hsb`
- **默认值**：`hex`
- **版本**：5.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `rgb` | 官方取值 `rgb` |
  | `hex` | 官方取值 `hex` |
  | `hsb` | 官方取值 `hsb` |

#### `disabled`

- **说明**：禁用颜色选择器
- **类型**：boolean
- **默认值**：-

#### `disabledAlpha`

- **说明**：禁用透明度
- **类型**：boolean
- **默认值**：-
- **版本**：5.8.0

#### `disabledFormat`

- **说明**：禁用选择颜色格式
- **类型**：boolean
- **默认值**：-
- **版本**：5.22.0

#### `format`

- **说明**：颜色格式
- **类型**：`rgb` | `hex` | `hsb`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `rgb` | 官方取值 `rgb` |
  | `hex` | 官方取值 `hex` |
  | `hsb` | 官方取值 `hsb` |

#### `mode`

- **说明**：选择器模式，用于配置单色与渐变
- **类型**：`'single' | 'gradient' | ('single' | 'gradient')[]`
- **默认值**：`single`
- **版本**：5.20.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `single` | 官方取值 `single` |
  | `gradient` | 官方取值 `gradient` |
  | `('single` | 官方取值 `('single` |
  | `gradient')[]` | 官方取值 `gradient')[]` |

#### `presets`

- **说明**：预设的颜色
- **类型**：[PresetColorType](#presetcolortype)
- **默认值**：-

#### `placement`

- **说明**：弹出窗口的位置
- **类型**：同 `Tooltips` 组件的 [placement](/components/tooltip-cn/#api) 参数设计
- **默认值**：`bottomLeft`

#### `showText`

- **说明**：显示颜色文本
- **类型**：boolean | `(color: Color) => React.ReactNode`
- **默认值**：-
- **版本**：5.7.0

#### `size`

- **说明**：设置触发器大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **版本**：5.7.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `trigger`

- **说明**：颜色选择器的触发模式
- **类型**：`hover` | `click`
- **默认值**：`click`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `hover` | 悬停触发 |
  | `click` | 点击触发 |

#### `value`

- **说明**：颜色的值
- **类型**：[ColorType](#colortype)
- **默认值**：-

#### `onChange`

- **说明**：颜色变化的回调
- **类型**：`(value: Color, css: string) => void`
- **默认值**：-

#### `onChangeComplete`

- **说明**：颜色选择完成的回调，通过 `onChangeComplete` 对 `value` 受控时拖拽不会改变展示颜色
- **类型**：`(value: Color) => void`
- **默认值**：-
- **版本**：5.7.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `onChangeComplete` | 官方取值 `onChangeComplete` |
  | `value` | 官方取值 `value` |

#### `onFormatChange`

- **说明**：颜色格式变化的回调
- **类型**：`(format: 'hex' | 'rgb' | 'hsb') => void`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `(format: 'hex` | 官方取值 `(format: 'hex` |
  | `rgb` | 官方取值 `rgb` |
  | `hsb') => void` | 官方取值 `hsb') => void` |

#### `toHexString`

- **说明**：转换成 `hex` 格式颜色字符串，返回格式如：`#1677ff`
- **类型**：`() => string`
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `hex` | 官方取值 `hex` |

#### `toHsbString`

- **说明**：转换成 `hsb` 格式颜色字符串，返回格式如：`hsb(215, 91%, 100%)`
- **类型**：`() => string`
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `hsb` | 官方取值 `hsb` |

#### `toRgbString`

- **说明**：转换成 `rgb` 格式颜色字符串，返回格式如：`rgb(22, 119, 255)`
- **类型**：`() => string`
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `rgb` | 官方取值 `rgb` |

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

当用户需要自定义颜色选择的时候使用。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`base.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **触发器尺寸大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **受控模式**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **渐变色**（`line-gradient.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **渲染触发器文本**（`text-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **禁用透明度**（`disabled-alpha.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **清除颜色**（`allowClear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义触发器**（`trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义触发事件**（`trigger-event.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **颜色编码**（`format.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **预设颜色**（`presets.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义面板**（`panel-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 颜色的值 |
| `defaultValue` | 非受控默认值 | 颜色默认的值 |
| `onChange` | 值变化 | 颜色变化的回调 |
| `open` | 受控显隐 | 是否显示弹出窗口 |
| `onOpenChange` | 显隐变化 | 当 `open` 被改变时的回调 |
| `disabled` | 禁用 | 禁用颜色选择器 |
| `destroyOnHidden` | 隐藏销毁 | 关闭后是否销毁弹窗 |
| `onClear` | 清除 | 清除的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `base.tsx` | 否 |
| 触发器尺寸大小 | `size.tsx` | 否 |
| 受控模式 | `controlled.tsx` | 否 |
| 渐变色 | `line-gradient.tsx` | 否 |
| 渲染触发器文本 | `text-render.tsx` | 否 |
| 禁用 | `disabled.tsx` | 否 |
| 禁用透明度 | `disabled-alpha.tsx` | 否 |
| 清除颜色 | `allowClear.tsx` | 否 |
| 自定义触发器 | `trigger.tsx` | 否 |
| 自定义触发事件 | `trigger-event.tsx` | 否 |
| 颜色编码 | `format.tsx` | 否 |
| 预设颜色 | `presets.tsx` | 否 |
| 预设渐变色 | `presets-line-gradient.tsx` | 是 |
| 自定义面板 | `panel-render.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Pure Render | `pure-panel.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 关于颜色赋值的问题 {#faq-color-assignment}

颜色选择器的值同时支持字符串色值和选择器生成的 `Color` 对象，但由于不同格式的颜色字符串互相转换会有精度误差问题，所以受控场景推荐使用选择器生成的 `Color` 对象来进行赋值操作，这样可以避免精度问题，保证取值是精准的，选择器也可以按照预期工作。

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

> 自 `antd@5.5.0` 版本开始提供该组件。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| :-- | :-- | :-- | :-- | :-- | --- |
| allowClear | 允许清除选择的颜色 | boolean | false | arrow | 配置弹出的箭头 | `boolean \| { pointAtCenter: boolean }` | true | children | 颜色选择器的触发器 | React.ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 颜色默认的值 | [ColorType](#colortype) | - | defaultFormat | 颜色格式默认的值 | `rgb` \| `hex` \| `hsb` | `hex` | 5.9.0 | × |
| disabled | 禁用颜色选择器 | boolean | - | disabledAlpha | 禁用透明度 | boolean | - | 5.8.0 | × |
| disabledFormat | 禁用选择颜色格式 | boolean | - | 5.22.0 | × |
| ~~destroyTooltipOnHide~~ | 关闭后是否销毁弹窗 | `boolean` | false | 5.7.0 | × |
| destroyOnHidden | 关闭后是否销毁弹窗 | `boolean` | false | 5.25.0 | × |
| format | 颜色格式 | `rgb` \| `hex` \| `hsb` | - | mode | 选择器模式，用于配置单色与渐变 | `'single' \| 'gradient' \| ('single' \| 'gradient')[]` | `single` | 5.20.0 | × |
| open | 是否显示弹出窗口 | boolean | - | presets | 预设的颜色 | [PresetColorType](#presetcolortype) | - | placement | 弹出窗口的位置 | 同 `Tooltips` 组件的 [placement](/components/tooltip-cn/#api) 参数设计 | `bottomLeft` | panelRender | 自定义渲染面板 | `(panel: React.ReactNode, extra: { components: { Picker: FC; Presets: FC } }) => React.ReactNode` | - | 5.7.0 | × |
| showText | 显示颜色文本 | boolean \| `(color: Color) => React.ReactNode` | - | 5.7.0 | × |
| size | 设置触发器大小 | `large` \| `medium` \| `small` | `medium` | 5.7.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | trigger | 颜色选择器的触发模式 | `hover` \| `click` | `click` | value | 颜色的值 | [ColorType](#colortype) | - | onChange | 颜色变化的回调 | `(value: Color, css: string) => void` | - | onChangeComplete | 颜色选择完成的回调，通过 `onChangeComplete` 对 `value` 受控时拖拽不会改变展示颜色 | `(value: Color) => void` | - | 5.7.0 | × |
| onFormatChange | 颜色格式变化的回调 | `(format: 'hex' \| 'rgb' \| 'hsb') => void` | - | onOpenChange | 当 `open` 被改变时的回调 | `(open: boolean) => void` | - | onClear | 清除的回调 | `() => void` | - | 5.6.0 | × |

#### ColorType

```typescript
type ColorType =
  | string
  | Color
  | {
      color: string;
      percent: number;
    }[];
```

#### PresetColorType

```typescript
type PresetColorType = {
  label: React.ReactNode;
  defaultOpen?: boolean;
  key?: React.Key;
  colors: ColorType[];
};
```

### Color

| 参数 | 说明 | 类型 | 版本 |
| :-- | :-- | :-- | :-- |
| toCssString | 转换成 CSS 支持的格式 | `() => string` | 5.20.0 |
| toHex | 转换成 `hex` 格式字符，返回格式如：`1677ff` | `() => string` | - |
| toHexString | 转换成 `hex` 格式颜色字符串，返回格式如：`#1677ff` | `() => string` | - |
| toHsb | 转换成 `hsb` 对象  | `() => ({ h: number, s: number, b: number, a: number })` | - |
| toHsbString | 转换成 `hsb` 格式颜色字符串，返回格式如：`hsb(215, 91%, 100%)` | `() => string` | - |
| toRgb | 转换成 `rgb` 对象  | `() => ({ r: number, g: number, b: number, a: number })` | - |
| toRgbString | 转换成 `rgb` 格式颜色字符串，返回格式如：`rgb(22, 119, 255)` | `() => string` | - |

### 导入方式

```js
import { ColorPicker } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 允许清除选择的颜色 | boolean | false | — |
| `arrow` | 配置弹出的箭头 | `boolean \| { pointAtCenter: boolean }` | true | — |
| `children` | 颜色选择器的触发器 | React.ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 颜色默认的值 | [ColorType](#colortype) | - | — |
| `defaultFormat` | 颜色格式默认的值 | `rgb` \| `hex` \| `hsb` | `hex` | 5.9.0 |
| `disabled` | 禁用颜色选择器 | boolean | - | — |
| `disabledAlpha` | 禁用透明度 | boolean | - | 5.8.0 |
| `disabledFormat` | 禁用选择颜色格式 | boolean | - | 5.22.0 |
| `destroyTooltipOnHide` | 关闭后是否销毁弹窗 | `boolean` | false | 5.7.0 |
| `destroyOnHidden` | 关闭后是否销毁弹窗 | `boolean` | false | 5.25.0 |
| `format` | 颜色格式 | `rgb` \| `hex` \| `hsb` | - | — |
| `mode` | 选择器模式，用于配置单色与渐变 | `'single' \| 'gradient' \| ('single' \| 'gradient')[]` | `single` | 5.20.0 |
| `open` | 是否显示弹出窗口 | boolean | - | — |
| `presets` | 预设的颜色 | [PresetColorType](#presetcolortype) | - | — |
| `placement` | 弹出窗口的位置 | 同 `Tooltips` 组件的 [placement](/components/tooltip-cn/#api) 参数设计 | `bottomLeft` | — |
| `panelRender` | 自定义渲染面板 | `(panel: React.ReactNode, extra: { components: { Picker: FC; Presets: FC } }) => React.ReactNode` | - | 5.7.0 |
| `showText` | 显示颜色文本 | boolean \| `(color: Color) => React.ReactNode` | - | 5.7.0 |
| `size` | 设置触发器大小 | `large` \| `medium` \| `small` | `medium` | 5.7.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `trigger` | 颜色选择器的触发模式 | `hover` \| `click` | `click` | — |
| `value` | 颜色的值 | [ColorType](#colortype) | - | — |
| `onChange` | 颜色变化的回调 | `(value: Color, css: string) => void` | - | — |
| `onChangeComplete` | 颜色选择完成的回调，通过 `onChangeComplete` 对 `value` 受控时拖拽不会改变展示颜色 | `(value: Color) => void` | - | 5.7.0 |
| `onFormatChange` | 颜色格式变化的回调 | `(format: 'hex' \| 'rgb' \| 'hsb') => void` | - | — |
| `onOpenChange` | 当 `open` 被改变时的回调 | `(open: boolean) => void` | - | — |
| `onClear` | 清除的回调 | `() => void` | - | 5.6.0 |
| `toCssString` | 转换成 CSS 支持的格式 | `() => string` | — | 5.20.0 |
| `toHex` | 转换成 `hex` 格式字符，返回格式如：`1677ff` | `() => string` | — | - |
| `toHexString` | 转换成 `hex` 格式颜色字符串，返回格式如：`#1677ff` | `() => string` | — | - |
| `toHsb` | 转换成 `hsb` 对象 | `() => ({ h: number, s: number, b: number, a: number })` | — | - |
| `toHsbString` | 转换成 `hsb` 格式颜色字符串，返回格式如：`hsb(215, 91%, 100%)` | `() => string` | — | - |
| `toRgb` | 转换成 `rgb` 对象 | `() => ({ r: number, g: number, b: number, a: number })` | — | - |
| `toRgbString` | 转换成 `rgb` 格式颜色字符串，返回格式如：`rgb(22, 119, 255)` | `() => string` | — | - |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **ColorPicker** 的验收清单：

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
- 官方文档：https://ant.design/components/color-picker
- 中文文档：https://ant.design/components/color-picker-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/color-picker
- 驱动 gpui kit：`color-picker`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **ColorPicker** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/color-picker/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（ColorPicker）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（ColorPicker）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于选择颜色。

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
| `allowClear` | 允许清除选择的颜色 | boolean | false |
| `arrow` | 配置弹出的箭头 | `boolean \ | { pointAtCenter: boolean }` |
| `children` | 颜色选择器的触发器 | React.ReactNode | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultValue` | 颜色默认的值 | [ColorType](#colortype) | - |
| `defaultFormat` | 颜色格式默认的值 | `rgb` \ | `hex` \ |
| `disabled` | 禁用颜色选择器 | boolean | - |
| `disabledAlpha` | 禁用透明度 | boolean | - |
| `disabledFormat` | 禁用选择颜色格式 | boolean | - |
| `destroyOnHidden` | 关闭后是否销毁弹窗 | `boolean` | false |
| `format` | 颜色格式 | `rgb` \ | `hex` \ |
| `mode` | 选择器模式，用于配置单色与渐变 | `'single' \ | 'gradient' \ |
| `open` | 是否显示弹出窗口 | boolean | - |
| `presets` | 预设的颜色 | [PresetColorType](#presetcolortype) | - |
| `placement` | 弹出窗口的位置 | 同 `Tooltips` 组件的 [placement](/compone… | `bottomLeft` |
| `panelRender` | 自定义渲染面板 | `(panel: React.ReactNode, extra: { co… | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
开面板 ── 调色 ── onChange
松手/完成 ── onChangeComplete
format hex/rgb/hsb
disabledAlpha 无透明
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CP-S1 | 改色 | onChange |
| CP-S2 | format=hex | 字符串 #rrggbb |
| CP-S3 | presets 点选 | 到预设色 |
| CP-S4 | clear | 空 |
| CP-S5 | disabledAlpha | 无 alpha 滑条 |
| CP-S6 | disabled | 不打开 |
| CP-S7 | showText | 展示文本 |
| CP-S8 | 受控 value | 外部 |
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
| `size` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `children` | 必须 |
| `placement` | 必须 |
| `trigger` | 必须 |
| `allowClear` | 必须 |
| `mode` | 必须 |
| 官方主路径示例 | 基本使用、触发器尺寸大小、受控模式、渐变色、渲染触发器文本、禁用、禁用透明度、清除颜色 |
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
| 其余示例 | 自定义触发器, 自定义触发事件, 颜色编码, 预设颜色 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestColorPicker_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 ColorPicker 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CP-01 | L1 | NewColorPicker 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| CP-02 | L1 | 改色 | onChange |
| CP-03 | L1 | format=hex | 字符串 #rrggbb |
| CP-04 | L1 | presets 点选 | 到预设色 |
| CP-05 | L1 | clear | 空 |
| CP-06 | L1 | disabledAlpha | 无 alpha 滑条 |
| CP-07 | L1 | disabled | 不打开 |
| CP-08 | L1 | showText | 展示文本 |
| CP-09 | L1 | 受控 value | 外部 |
| CP-10 | L1 | 复现官方示例「基本使用」（`base.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-11 | L1 | 复现官方示例「触发器尺寸大小」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-12 | L1 | 复现官方示例「受控模式」（`controlled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-13 | L1 | 复现官方示例「渐变色」（`line-gradient.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-14 | L1 | 复现官方示例「渲染触发器文本」（`text-render.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-15 | L1 | 复现官方示例「禁用」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-16 | L1 | 复现官方示例「禁用透明度」（`disabled-alpha.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-17 | L1 | 复现官方示例「清除颜色」（`allowClear.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CP-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| CP-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| CP-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| CP-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| CP-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| CP-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| CP-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewColorPicker(...) *ColorPicker

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

同时满足即可宣布 **ColorPicker 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/color-picker.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` ColorPicker 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
