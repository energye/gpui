# Segmented 分段控制器
> 来源：[Ant Design 6.5.x Segmented](https://ant.design/components/segmented)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用于展示多个选项并允许用户选择其中单个选项。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于展示多个选项并允许用户选择其中单个选项。

**Segmented** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 垂直方向 | 纵向布局 |
| Block 分段选择器 | 宽度撑满父级 |
| 胶囊形状 | 复现「胶囊形状」视觉与布局 |
| 不可用 | disabled 态 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 自定义渲染 | 自定义渲染/插槽外观 |
| 动态数据 | 复现「动态数据」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 设置图标 | icon 与文本混排 |
| 只设置图标 | icon 与文本混排 |
| 配合 name 使用 | 复现「配合 name 使用」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `block`

- **说明**：将宽度调整为父元素宽度的选项
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `orientation`

- **说明**：排列方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `size`

- **说明**：控件尺寸
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `vertical`

- **说明**：排列方向，与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：`false`
- **版本**：5.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `shape`

- **说明**：形状
- **类型**：`default` | `round`
- **默认值**：`default`
- **版本**：5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `round` | 大圆角/胶囊 |

#### `icon`

- **说明**：分段项的显示图标
- **类型**：ReactNode
- **默认值**：-

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

- 用于展示多个选项并允许用户选择其中单个选项；
- 当切换选中选项时，关联区域的内容会发生变化。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **垂直方向**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **Block 分段选择器**（`block.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **胶囊形状**（`shape.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **受控模式**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义渲染**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **动态数据**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **设置图标**（`with-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **只设置图标**（`icon-only.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **配合 name 使用**（`with-name.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前选中的值 |
| `defaultValue` | 非受控默认值 | 默认选中的值 |
| `onChange` | 值变化 | 选项变化时的回调函数 |
| `disabled` | 禁用 | 是否禁用 |
| `options` | 数据化 options | 数据化配置选项内容 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 垂直方向 | `vertical.tsx` | 否 |
| Block 分段选择器 | `block.tsx` | 否 |
| 胶囊形状 | `shape.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 受控模式 | `controlled.tsx` | 否 |
| 自定义渲染 | `custom.tsx` | 否 |
| 动态数据 | `dynamic.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 设置图标 | `with-icon.tsx` | 否 |
| 只设置图标 | `icon-only.tsx` | 否 |
| 配合 name 使用 | `with-name.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 受控同步模式 | `controlled-two.tsx` | 是 |
| 统一高度 | `size-consistent.tsx` | 是 |
| 自定义组件 Token | `componentToken.tsx` | 是 |

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

> 自 `antd@4.20.0` 版本开始提供该组件。

### Segmented

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| block | 将宽度调整为父元素宽度的选项 | boolean | false | classNames | 用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 默认选中的值 | string \| number | `options` 首项的值 | disabled | 是否禁用 | boolean | false | onChange | 选项变化时的回调函数 | function(value: string \| number) | options | 数据化配置选项内容 | string\[] \| number\[] \| SegmentedItemType\[] | [] | orientation | 排列方向 | `horizontal` \| `vertical` | `horizontal` | size | 控件尺寸 | `large` \| `medium` \| `small` | `medium` | styles | 用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> | - | vertical | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | 5.21.0 | × |
| value | 当前选中的值 | string \| number | shape | 形状 | `default` \| `round` | `default` | 5.24.0 | × |
| name | Segmented 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string 
### SegmentedItemType

| 属性 | 描述 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| className | 自定义类名 | string | - | icon | 分段项的显示图标 | ReactNode | - | tooltip | 分段项的工具提示 | string \| [TooltipProps](../tooltip/index.zh-CN.md#api) | - 
### 导入方式

```js
import { Segmented } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `block` | 将宽度调整为父元素宽度的选项 | boolean | false | — |
| `classNames` | 用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `defaultValue` | 默认选中的值 | string \| number | `options` 首项的值 | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `onChange` | 选项变化时的回调函数 | function(value: string \| number) | — | — |
| `options` | 数据化配置选项内容 | string\[] \| number\[] \| SegmentedItemType\[] | [] | — |
| `orientation` | 排列方向 | `horizontal` \| `vertical` | `horizontal` | — |
| `size` | 控件尺寸 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | 5.21.0 |
| `value` | 当前选中的值 | string \| number | — | — |
| `shape` | 形状 | `default` \| `round` | `default` | 5.24.0 |
| `name` | Segmented 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string | — | 5.23.0 |
| `className` | 自定义类名 | string | - | — |
| `icon` | 分段项的显示图标 | ReactNode | - | — |
| `label` | 分段项的显示文本 | ReactNode | - | — |
| `tooltip` | 分段项的工具提示 | string \| [TooltipProps](../tooltip/index.zh-CN.md#api) | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Segmented** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **13** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/segmented
- 中文文档：https://ant.design/components/segmented-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/segmented
- 驱动 gpui kit：`segmented`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Segmented** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/segmented/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Segmented）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Segmented）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于展示多个选项并允许用户选择其中单个选项。

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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `block` | 将宽度调整为父元素宽度的选项 | boolean | false |
| `classNames` | 用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `defaultValue` | 默认选中的值 | string \ | number |
| `disabled` | 是否禁用 | boolean | false |
| `onChange` | 选项变化时的回调函数 | function(value: string \ | number) |
| `options` | 数据化配置选项内容 | string\[] \ | number\[] \ |
| `orientation` | 排列方向 | `horizontal` \ | `vertical` |
| `size` | 控件尺寸 | `large` \ | `medium` \ |
| `styles` | 用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) ,… | (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` |
| `value` | 当前选中的值 | string \ | number |
| `shape` | 形状 | `default` \ | `round` |
| `name` | Segmented 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退… | string | — |
| `className` | 自定义类名 | string | - |
| `icon` | 分段项的显示图标 | ReactNode | - |
| `label` | 分段项的显示文本 | ReactNode | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
options 单选
点 option ──► value + onChange
block ──► 均分撑满
disabled 项 ──► 不可点
```

\*高≈controlHeight。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SEG-S1 | 切换选项 | onChange |
| SEG-S2 | block | 撑满宽 |
| SEG-S3 | disabled 项 | 不可选 |
| SEG-S4 | size | 高度档 |
| SEG-S5 | 受控 value | 外部优先 |
| SEG-S6 | 仅图标 option | 可显示 |
| SEG-S7 | 键盘（适用） | 可切换 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | Token 默认皮 |
| hover / active | 可交互反馈 |
| focus | 可见 focus ring |
| checked/selected/active（适用者） | 主色强调 |
| disabled | 降对比；无 hover |
| loading | 指示器；防重复 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | button / checkbox / switch / radio 等与语义一致 |
| 名称 | 可交互必有名；仅图标必须 AriaLabel |
| 焦点 | Tab 可达；ring 可见 |
| 键盘 | Space/Enter 或方向键按角色 |
| 禁用 | 不可激活；读屏可感知（平台支持时） |

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
| `options` | 必须 |
| `orientation` | 必须 |
| `shape` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | 基本、垂直方向、Block 分段选择器、胶囊形状、不可用、受控模式、自定义渲染、动态数据 |
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
| 其余示例 | 三种大小, 设置图标, 只设置图标, 配合 name 使用 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSegmented_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Segmented 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SEG-01 | L1 | NewSegmented 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SEG-02 | L1 | 切换选项 | onChange |
| SEG-03 | L1 | block | 撑满宽 |
| SEG-04 | L1 | disabled 项 | 不可选 |
| SEG-05 | L1 | size | 高度档 |
| SEG-06 | L1 | 受控 value | 外部优先 |
| SEG-07 | L1 | 仅图标 option | 可显示 |
| SEG-08 | L1 | 键盘（适用） | 可切换 |
| SEG-09 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-10 | L1 | 复现官方示例「垂直方向」（`vertical.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-11 | L1 | 复现官方示例「Block 分段选择器」（`block.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-12 | L1 | 复现官方示例「胶囊形状」（`shape.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-13 | L1 | 复现官方示例「不可用」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-14 | L1 | 复现官方示例「受控模式」（`controlled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-15 | L1 | 复现官方示例「自定义渲染」（`custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-16 | L1 | 复现官方示例「动态数据」（`dynamic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEG-17 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SEG-18 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| SEG-19 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SEG-20 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| SEG-21 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SEG-22 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SEG-23 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSegmented(...) *Segmented

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
Pressable
  └─ Decorated chrome
       └─ content (icon/label/indicator)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Segmented 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/segmented.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Segmented 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
