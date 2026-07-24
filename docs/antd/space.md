# Space 间距
> 来源：[Ant Design 6.5.x Space](https://ant.design/components/space)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：设置组件之间的间距。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

设置组件之间的间距。

**Space** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 垂直间距 | 纵向布局 |
| 间距大小 | 不同 size 档位 |
| 对齐 | 复现「对齐」视觉与布局 |
| 自动换行 | 复现「自动换行」视觉与布局 |
| 分隔符 | 复现「分隔符」视觉与布局 |
| 紧凑布局组合 | 复现「紧凑布局组合」视觉与布局 |
| Button 紧凑布局 | 复现「Button 紧凑布局」视觉与布局 |
| 垂直方向紧凑布局 | 纵向布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：对齐方式
- **类型**：`start` | `end` |`center` |`baseline`
- **默认值**：-
- **版本**：4.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |
  | `center` | 居中 |
  | `baseline` | 官方取值 `baseline` |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props: SpaceProps })=> Record
- **默认值**：-

#### `direction`

- **说明**：间距方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`horizontal`
- **版本**：4.1.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `orientation`

- **说明**：间距方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `size`

- **说明**：间距大小
- **类型**：[Size](#size) | [Size\[\]](#size)
- **默认值**：`small`
- **版本**：4.1.0 | Array: 4.9.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props: SpaceProps })=> Record
- **默认值**：-

#### `vertical`

- **说明**：是否垂直，和 `orientation` 同时配置以 `orientation` 优先
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `wrap`

- **说明**：是否自动换行，仅在 `horizontal` 时有效
- **类型**：boolean
- **默认值**：false
- **版本**：4.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |

#### `block`

- **说明**：将宽度调整为父元素宽度的选项
- **类型**：boolean
- **默认值**：false
- **版本**：4.24.0

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

避免组件紧贴在一起，拉开统一的空间。

- 适合行内元素的水平间距。
- 可以设置各种水平对齐方式。
- 需要表单组件之间紧凑连接且合并边框时，使用 Space.Compact（自 `antd@4.24.0` 版本开始提供该组件）。

### 与 Flex 组件的区别 {#difference-with-flex-component}

- Space 为内联元素提供间距，其本身会为每一个子元素添加包裹元素用于内联对齐。适用于行、列中多个子元素的等距排列。
- Flex 为块级元素提供间距，其本身不会添加包裹元素。适用于垂直或水平方向上的子元素布局，并提供了更多的灵活性和控制能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`base.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **垂直间距**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **间距大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **对齐**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自动换行**（`wrap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **分隔符**（`separator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **紧凑布局组合**（`compact.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **Button 紧凑布局**（`compact-buttons.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **垂直方向紧凑布局**（`compact-button-vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `base.tsx` | 否 |
| 垂直间距 | `vertical.tsx` | 否 |
| 间距大小 | `size.tsx` | 否 |
| 对齐 | `align.tsx` | 否 |
| 自动换行 | `wrap.tsx` | 否 |
| 分隔符 | `separator.tsx` | 否 |
| 紧凑布局组合 | `compact.tsx` | 否 |
| Button 紧凑布局 | `compact-buttons.tsx` | 否 |
| 垂直方向紧凑布局 | `compact-button-vertical.tsx` | 否 |
| 调试 Input 前置/后置标签 | `compact-debug.tsx` | 是 |
| 紧凑布局嵌套 | `compact-nested.tsx` | 是 |
| 多样的 Child | `debug.tsx` | 是 |
| Flex gap 样式 | `gap-in-line.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义主题 | `component-token.tsx` | 是 |

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

### Space

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 对齐方式 | `start` \| `end` \|`center` \|`baseline` | - | 4.2.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props: SpaceProps })=> Record<[SemanticDOM](#semantic-dom), string> | - | ~~direction~~ | 间距方向 | `vertical` \| `horizontal` | `horizontal` | 4.1.0 | × |
| orientation | 间距方向 | `vertical` \| `horizontal` | `horizontal` | size | 间距大小 | [Size](#size) \| [Size\[\]](#size) | `small` | 4.1.0 \| Array: 4.9.0 | 5.6.0 |
| ~~split~~ | 设置分隔符, 请使用 `separator` 替换 | ReactNode | - | 4.7.0 | × |
| separator | 设置分隔符 | ReactNode | - | - | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props: SpaceProps })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | vertical | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - | × |
| wrap | 是否自动换行，仅在 `horizontal` 时有效 | boolean | false | 4.9.0 | × |

### Size

`'small' | 'medium' | 'large' | number`

### Space.Compact

需要表单组件之间紧凑连接且合并边框时，使用 Space.Compact，支持的组件有：

- Button
- AutoComplete
- Cascader
- DatePicker
- Input/Input.Search
- InputNumber
- Select
- TimePicker
- TreeSelect

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| block | 将宽度调整为父元素宽度的选项 | boolean | false | 4.24.0 |
| ~~direction~~ | 指定排列方向 | `vertical` \| `horizontal` | `horizontal` | 4.24.0 |
| orientation | 指定排列方向 | `vertical` \| `horizontal` | `horizontal` | vertical | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - |

### Space.Addon

> 自 antd@5.29.0 版本开始提供该组件。

用于在紧凑布局中创建自定义单元格。

| 参数     | 说明       | 类型      | 默认值 | 版本   |
| -------- | ---------- | --------- | ------ | ------ |
| children | 自定义内容 | ReactNode | -      | 5.29.0 |

### 导入方式

```js
import { Space } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 对齐方式 | `start` \| `end` \|`center` \|`baseline` | - | 4.2.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props: SpaceProps })=> Record | - | — |
| `direction` | 间距方向 | `vertical` \| `horizontal` | `horizontal` | 4.1.0 |
| `orientation` | 间距方向 | `vertical` \| `horizontal` | `horizontal` | — |
| `size` | 间距大小 | [Size](#size) \| [Size\[\]](#size) | `small` | 4.1.0 \| Array: 4.9.0 |
| `split` | 设置分隔符, 请使用 `separator` 替换 | ReactNode | - | 4.7.0 |
| `separator` | 设置分隔符 | ReactNode | - | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props: SpaceProps })=> Record | - | — |
| `vertical` | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - |
| `wrap` | 是否自动换行，仅在 `horizontal` 时有效 | boolean | false | 4.9.0 |
| `block` | 将宽度调整为父元素宽度的选项 | boolean | false | 4.24.0 |
| `children` | 自定义内容 | ReactNode | - | 5.29.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Space** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **10** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/space
- 中文文档：https://ant.design/components/space-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/space
- 驱动 gpui kit：`space`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Space** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/space/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Space）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 布局参数驱动子项几何正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Space）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：设置组件之间的间距。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| Space size small | **8** | paddingXS |
| Space size middle | **16** | padding |
| Space size large | **24** | paddingLG |
| size small 默认 | **8** | paddingXS |
| middle/large | **16 / 24** | padding / paddingLG |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**布局**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `align` | 交叉轴对齐 | `start` \| `end` \| `center` \| `baseline` | 水平未设时 **center**；垂直未设时无强制 |
| `orientation` | 间距方向（优先于 `vertical` / 废弃 `direction`） | `vertical` \| `horizontal` | `horizontal` |
| `vertical` | 是否垂直；与 `orientation` 同时配置以 `orientation` 优先 | boolean | false |
| `size` | 间距大小（预设或数字或 `[列, 行]`） | `small` \| `medium`/`middle` \| `large` \| `number` \| `[Size, Size]` | **`small`（8）** |
| `wrap` | 是否自动换行，**仅 horizontal** 有效 | boolean | false |
| `separator` | 子项之间的分隔节点（装饰可 aria-hidden） | Node | - |
| `children` | 子节点 | Node… | - |
| `classNames` / `styles` | 语义钩子 | — | P1 |
| **Space.Compact** |  |  |  |
| `block` | Compact 宽度铺满父级 | boolean | false |
| `orientation` / `vertical` | Compact 排列方向 | 同 Space | horizontal |
| `size` | 下传给紧凑子控件的尺寸档（Button 等） | small/middle/large | middle |
| **Space.Addon** | Compact 内自定义单元格 | children | — |

**配置优先级（通用）：** 受控 props > 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。  
**方向解析：** `orientation`（若 Set）> `vertical` 糖 > 默认 horizontal（与 antd `useOrientation` 一致）。

### 6.4 交互状态机（L1）

```text
子项等距；Compact 合并边框
```

\*默认 size=small=8。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SPC-S1 | 默认三子 | 横向 gap8 |
| SPC-S2 | size=large | gap24 |
| SPC-S3 | vertical | 纵向 |
| SPC-S4 | wrap | 换行 |
| SPC-S5 | separator | 分隔可见 |
| SPC-S6 | Compact 双 Button | 中间无双边框缝 |
| SPC-S7 | align | 对齐 |
| SPC-S8 | size=16 数字 | 16px |
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
| 装饰分隔 | 纯装饰可 aria-hidden |
| 拖拽把手 | 可命名；键盘微调 P0/P1 按控件 |

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
| `size` | 必须 |
| `children` | 必须 |
| `orientation` | 必须 |
| 官方主路径示例 | 基本用法、垂直间距、间距大小、对齐、自动换行、分隔符、紧凑布局组合、Button 紧凑布局 |
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
| 其余示例 | 垂直方向紧凑布局, 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSpace_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Space 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SPC-01 | L1 | NewSpace 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SPC-02 | L1 | 默认三子 | 横向 gap8 |
| SPC-03 | L1 | size=large | gap24 |
| SPC-04 | L1 | vertical | 纵向 |
| SPC-05 | L1 | wrap | 换行 |
| SPC-06 | L1 | separator | 分隔可见 |
| SPC-07 | L1 | Compact 双 Button | 中间无双边框缝 |
| SPC-08 | L1 | align | 对齐 |
| SPC-09 | L1 | size=16 数字 | 16px |
| SPC-10 | L1 | 复现官方示例「基本用法」（`base.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-11 | L1 | 复现官方示例「垂直间距」（`vertical.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-12 | L1 | 复现官方示例「间距大小」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-13 | L1 | 复现官方示例「对齐」（`align.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-14 | L1 | 复现官方示例「自动换行」（`wrap.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-15 | L1 | 复现官方示例「分隔符」（`separator.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-16 | L1 | 复现官方示例「紧凑布局组合」（`compact.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-17 | L1 | 复现官方示例「Button 紧凑布局」（`compact-buttons.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SPC-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SPC-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| SPC-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SPC-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| SPC-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SPC-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SPC-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。旧 `SetSize(float64)` 改为 `SetSizePx`；新增 Compact / Addon。

```text
// —— Space ——
NewSpace(children ...core.Node) *Space

SetOrientation(SpaceOrientation)   // horizontal | vertical；优先于 Vertical
SetVertical(bool)                  // 糖；orientation 已 Set 时忽略
SetAlign(SpaceAlign)               // auto|start|end|center|baseline
SetSize(SpaceSize)                 // small|middle|large 预设
SetSizePx(float64)                 // 数字间距（含显式 0）
SetSizeXY(col, row float64)        // antd size={[col,row]}；主轴/列间距优先写入 Gap
SetWrap(bool)                      // 仅 horizontal 生效
SetSeparator(func() core.Node)     // 每个子项间隙调用一次（避免同 Node 挂多次）
SetChildren(...core.Node) / Add / ClearChildren
SetTheme(*Theme)
SetAriaLabel(string)               // 可选布局名；分隔符纯装饰不抢焦点
SetExpandMax(bool)                 // block 级铺满父宽（antd style display:flex vs 默认 inline-flex）；Compact block 嵌套时需开
Node() core.Node
ChromeNode() core.Node
ResolvedGap() float64
EffectiveOrientation() SpaceOrientation
IsVertical() bool

// —— Space.Compact ——
NewSpaceCompact(children ...core.Node) *SpaceCompact
SetOrientation / SetVertical / SetBlock / SetSize(ButtonSize 等档)
Add / SetChildren / Node()
// 行为：gap ≈ -lineWidth 叠边；子 Button 中间项圆角清零（无逐角 radius 时的 P0 近似）

// —— Space.Addon ——
NewSpaceAddon(children ...core.Node) *SpaceAddon
SetChildren(...core.Node) / SetChild / Add
SetSize(ButtonSize)                // small|middle|large → height/pad/radius
SetDisabled(bool)
SetTheme(*Theme)
Node() core.Node
// 结构：Decorated(h=controlHeight) → Flex(Row, CrossCenter, gap0) → children
// Compact.AddAddon 下传 Size/Theme（antd useCompactItemContext）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Orientation | horizontal |
| Vertical | false |
| Size | **small（8）** — 非 middle |
| Align | auto → 水平 **center**；垂直 start |
| Wrap | false |
| Separator | nil |
| Compact.block | false |
| Compact.size | middle（子控件档） |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Layout root
  └─ children with gap/span/handles
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Space 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/space.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Space 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
