# Tour 漫游式引导
> 来源：[Ant Design 6.5.x Tour](https://ant.design/components/tour)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用于分步引导用户了解产品功能的气泡组件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于分步引导用户了解产品功能的气泡组件。

**Tour** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 非模态 | 复现「非模态」视觉与布局 |
| 位置 | placement 方位 |
| 自定义遮罩样式 | mask 层 |
| 自定义指示器 | 自定义渲染/插槽外观 |
| 自定义操作按钮 | 自定义渲染/插槽外观 |
| 自定义高亮区域的样式 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabledInteraction`

- **说明**：禁用高亮区域交互
- **类型**：`boolean`
- **默认值**：`false`
- **版本**：5.13.0

#### `gap`

- **说明**：控制高亮区域的圆角边框和显示间距
- **类型**：`{ offset?: number | [number, number]; radius?: number }`
- **默认值**：`{ offset?: 6 ; radius?: 2 }`
- **版本**：5.0.0 (数组类型的 `offset`: 5.9.0 )
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `{ offset?: number` | 官方取值 `{ offset?: number` |

#### `keyboard`

- **说明**：是否启用键盘快捷行为
- **类型**：boolean
- **默认值**：true
- **版本**：6.2.0

#### `placement`

- **说明**：引导卡片相对于目标元素的位置
- **类型**：`center` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom` `top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight`
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `center` | 居中 |
  | `left` | 左侧 |
  | `leftTop` | 左上 |
  | `leftBottom` | 左下 |
  | `right` | 右侧 |
  | `rightTop` | 右上 |
  | `rightBottom` | 右下 |
  | `top` | 上方 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |
  | `bottom` | 下方 |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |

#### `mask`

- **说明**：是否启用蒙层，也可传入配置改变蒙层样式和填充色
- **类型**：`boolean | { style?: React.CSSProperties; color?: string; }`
- **默认值**：`true`

#### `type`

- **说明**：类型，影响底色与文字颜色
- **类型**：`default` | `primary`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `primary` | 主色强调 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `zIndex`

- **说明**：Tour 的层级
- **类型**：number
- **默认值**：1001
- **版本**：5.3.0

#### `cover`

- **说明**：展示的图片或者视频
- **类型**：`ReactNode`
- **默认值**：-

#### `title`

- **说明**：标题
- **类型**：`ReactNode`
- **默认值**：-

#### `description`

- **说明**：主要描述部分
- **类型**：`ReactNode`
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

常用于引导用户了解产品功能。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **非模态**（`non-modal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义遮罩样式**（`mask.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义指示器**（`indicator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义操作按钮**（`actions-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义高亮区域的样式**（`gap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 步骤改变时的回调，current 为当前的步骤 |
| `open` | 受控显隐 | 打开引导 |
| `getPopupContainer` | 浮层容器 | 设置 Tour 浮层的渲染节点，默认是 body |
| `onFinish` | 提交成功 | 引导完成时的回调 |
| `current` | 当前步骤/页 | 当前处于哪一步 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 非模态 | `non-modal.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 自定义遮罩样式 | `mask.tsx` | 否 |
| 自定义指示器 | `indicator.tsx` | 否 |
| 自定义操作按钮 | `actions-render.tsx` | 否 |
| 自定义高亮区域的样式 | `gap.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |

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

### Tour

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| arrow | 是否显示箭头，包含是否指向元素中心的配置 | `boolean` \| `{ pointAtCenter: boolean}` | `true` | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | closeIcon | 自定义关闭按钮 | `React.ReactNode` | `true` | 5.9.0 | 5.14.0 |
| disabledInteraction | 禁用高亮区域交互 | `boolean` | `false` | 5.13.0 | × |
| gap | 控制高亮区域的圆角边框和显示间距 | `{ offset?: number \| [number, number]; radius?: number }` | `{ offset?: 6 ; radius?: 2 }` | 5.0.0 (数组类型的 `offset`: 5.9.0 ) | × |
| keyboard | 是否启用键盘快捷行为 | boolean | true | 6.2.0 | × |
| placement | 引导卡片相对于目标元素的位置 | `center` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom` `top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight` | `bottom` | onClose | 关闭引导时的回调函数 | `Function` | - | onFinish | 引导完成时的回调 | `Function` | - | mask | 是否启用蒙层，也可传入配置改变蒙层样式和填充色 | `boolean \| { style?: React.CSSProperties; color?: string; }` | `true` | type | 类型，影响底色与文字颜色 | `default` \| `primary` | `default` | open | 打开引导 | `boolean` | - | onChange | 步骤改变时的回调，current 为当前的步骤 | `(current: number) => void` | - | current | 当前处于哪一步 | `number` | - | scrollIntoViewOptions | 是否支持当前元素滚动到视窗内，也可传入配置指定滚动视窗的相关参数 | `boolean \| ScrollIntoViewOptions` | `true` | 5.2.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | indicatorsRender | 自定义指示器 | `(current: number, total: number) => ReactNode` | - | 5.2.0 | × |
| actionsRender | 自定义操作按钮 | `(originNode: ReactNode, info: { current: number, total: number }) => ReactNode` | - | 5.25.0 | × |
| zIndex | Tour 的层级 | number | 1001 | 5.3.0 | × |
| getPopupContainer | 设置 Tour 浮层的渲染节点，默认是 body | `(node: HTMLElement) => HTMLElement` | body | 5.12.0 | × |

### TourStep 引导步骤卡片

| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| target | 获取引导卡片指向的元素，为空时居中于屏幕 | `() => HTMLElement` \| `HTMLElement` | - | closeIcon | 自定义关闭按钮 | `React.ReactNode` | `true` | 5.9.0 |
| cover | 展示的图片或者视频 | `ReactNode` | - | description | 主要描述部分 | `ReactNode` | - | onClose | 关闭引导时的回调函数 | `Function` | - | type | 类型，影响底色与文字颜色 | `default` \| `primary` | `default` | prevButtonProps | 上一步按钮的属性 | `{ children: ReactNode; onClick: Function }` | - 
### 导入方式

```js
import { Tour } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `arrow` | 是否显示箭头，包含是否指向元素中心的配置 | `boolean` \| `{ pointAtCenter: boolean}` | `true` | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `closeIcon` | 自定义关闭按钮 | `React.ReactNode` | `true` | 5.9.0 |
| `disabledInteraction` | 禁用高亮区域交互 | `boolean` | `false` | 5.13.0 |
| `gap` | 控制高亮区域的圆角边框和显示间距 | `{ offset?: number \| [number, number]; radius?: number }` | `{ offset?: 6 ; radius?: 2 }` | 5.0.0 (数组类型的 `offset`: 5.9.0 ) |
| `keyboard` | 是否启用键盘快捷行为 | boolean | true | 6.2.0 |
| `placement` | 引导卡片相对于目标元素的位置 | `center` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom` `top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight` | `bottom` | — |
| `onClose` | 关闭引导时的回调函数 | `Function` | - | — |
| `onFinish` | 引导完成时的回调 | `Function` | - | — |
| `mask` | 是否启用蒙层，也可传入配置改变蒙层样式和填充色 | `boolean \| { style?: React.CSSProperties; color?: string; }` | `true` | — |
| `type` | 类型，影响底色与文字颜色 | `default` \| `primary` | `default` | — |
| `open` | 打开引导 | `boolean` | - | — |
| `onChange` | 步骤改变时的回调，current 为当前的步骤 | `(current: number) => void` | - | — |
| `current` | 当前处于哪一步 | `number` | - | — |
| `scrollIntoViewOptions` | 是否支持当前元素滚动到视窗内，也可传入配置指定滚动视窗的相关参数 | `boolean \| ScrollIntoViewOptions` | `true` | 5.2.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `indicatorsRender` | 自定义指示器 | `(current: number, total: number) => ReactNode` | - | 5.2.0 |
| `actionsRender` | 自定义操作按钮 | `(originNode: ReactNode, info: { current: number, total: number }) => ReactNode` | - | 5.25.0 |
| `zIndex` | Tour 的层级 | number | 1001 | 5.3.0 |
| `getPopupContainer` | 设置 Tour 浮层的渲染节点，默认是 body | `(node: HTMLElement) => HTMLElement` | body | 5.12.0 |
| `target` | 获取引导卡片指向的元素，为空时居中于屏幕 | `() => HTMLElement` \| `HTMLElement` | - | — |
| `cover` | 展示的图片或者视频 | `ReactNode` | - | — |
| `title` | 标题 | `ReactNode` | - | — |
| `description` | 主要描述部分 | `ReactNode` | - | — |
| `nextButtonProps` | 下一步按钮的属性 | `{ children: ReactNode; onClick: Function }` | - | — |
| `prevButtonProps` | 上一步按钮的属性 | `{ children: ReactNode; onClick: Function }` | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Tour** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。
12. **表单专项**：rules、dependencies、scrollToFirstError。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tour
- 中文文档：https://ant.design/components/tour-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tour
- 驱动 gpui kit：`tour`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Tour** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/tour/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Tour）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 开合、遮罩/Esc、placement、确认/取消主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Tour）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于分步引导用户了解产品功能的气泡组件。

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
| `arrow` | 是否显示箭头，包含是否指向元素中心的配置 | `boolean` \ | `{ pointAtCenter: boolean}` |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `closeIcon` | 自定义关闭按钮 | `React.ReactNode` | `true` |
| `disabledInteraction` | 禁用高亮区域交互 | `boolean` | `false` |
| `gap` | 控制高亮区域的圆角边框和显示间距 | `{ offset?: number \ | [number, number]; radius?: number }` |
| `keyboard` | 是否启用键盘快捷行为 | boolean | true |
| `placement` | 引导卡片相对于目标元素的位置 | `center` `left` `leftTop` `leftBottom… | `bottom` |
| `onClose` | 关闭引导时的回调函数 | `Function` | - |
| `onFinish` | 引导完成时的回调 | `Function` | - |
| `mask` | 是否启用蒙层，也可传入配置改变蒙层样式和填充色 | `boolean \ | { style?: React.CSSProperties; color?: string; }` |
| `type` | 类型，影响底色与文字颜色 | `default` \ | `primary` |
| `open` | 打开引导 | `boolean` | - |
| `onChange` | 步骤改变时的回调，current 为当前的步骤 | `(current: number) => void` | - |
| `current` | 当前处于哪一步 | `number` | - |
| `scrollIntoViewOptions` | 是否支持当前元素滚动到视窗内，也可传入配置指定滚动视窗的相关参数 | `boolean \ | ScrollIntoViewOptions` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
open ── step[current] 高亮洞 + 气泡
next/prev ── current'
close ── onClose
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TOU-S1 | 打开 | 洞+气泡 |
| TOU-S2 | 下一步 | current+1 |
| TOU-S3 | 上一步 | current-1 |
| TOU-S4 | 关闭 | onClose |
| TOU-S5 | 受控 current | 外部 |
| TOU-S6 | 末步完成 | 关闭或回调 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| mask | `colorBgMask` 半透明（适用者） |
| panel/popup | 容器底 + 阴影 + 圆角 LG |
| open/close | 动画可关 / reduced-motion |
| disabled 触发 | 触发器禁用皮，不打开 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | dialog / menu / tooltip 等 |
| 焦点 | 打开进入浮层；关闭回触发器（可配） |
| Esc | 关闭（若允许） |
| 标题 | Dialog 必须有可访问名 |
| 遮罩 | 点击策略明确 |

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
| `onChange` | 必须 |
| `type` | 必须 |
| `open` | 必须 |
| `title` | 必须 |
| `placement` | 必须 |
| 官方主路径示例 | 基本、非模态、位置、自定义遮罩样式、自定义指示器、自定义操作按钮、自定义高亮区域的样式、自定义语义结构的样式和类 |
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
| 其余示例 | _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTour_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Tour 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TOU-01 | L1 | NewTour 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TOU-02 | L1 | 打开 | 洞+气泡 |
| TOU-03 | L1 | 下一步 | current+1 |
| TOU-04 | L1 | 上一步 | current-1 |
| TOU-05 | L1 | 关闭 | onClose |
| TOU-06 | L1 | 受控 current | 外部 |
| TOU-07 | L1 | 末步完成 | 关闭或回调 |
| TOU-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-09 | L1 | 复现官方示例「非模态」（`non-modal.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-10 | L1 | 复现官方示例「位置」（`placement.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-11 | L1 | 复现官方示例「自定义遮罩样式」（`mask.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-12 | L1 | 复现官方示例「自定义指示器」（`indicator.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-13 | L1 | 复现官方示例「自定义操作按钮」（`actions-render.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-14 | L1 | 复现官方示例「自定义高亮区域的样式」（`gap.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-15 | L1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TOU-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TOU-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TOU-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TOU-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TOU-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TOU-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TOU-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewTour(...) *Tour

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
Trigger?
  └─ Portal
       ├─ mask?
       └─ panel / popup (+ arrow?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Tour 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/tour.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Tour 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
