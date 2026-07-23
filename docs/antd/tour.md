# Tour 漫游式引导
> 来源：[Ant Design 6.5.x Tour](https://ant.design/components/tour)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用于分步引导用户了解产品功能的气泡组件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
