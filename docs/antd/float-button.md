# FloatButton 悬浮按钮
> 来源：[Ant Design 6.5.x FloatButton](https://ant.design/components/float-button)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：悬浮于页面上方的按钮。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

悬浮于页面上方的按钮。

**FloatButton** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 类型 | type 预设外观 |
| 形状 | 复现「形状」视觉与布局 |
| 描述 | 复现「描述」视觉与布局 |
| 含有气泡卡片的悬浮按钮 | card 风格容器 |
| 浮动按钮组 | 复现「浮动按钮组」视觉与布局 |
| 菜单模式 | 复现「菜单模式」视觉与布局 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 弹出方向 | 复现「弹出方向」视觉与布局 |
| 可拖拽 | 复现「可拖拽」视觉与布局 |
| 回到顶部 | 复现「回到顶部」视觉与布局 |
| 徽标数 | Badge 叠加 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `icon`

- **说明**：自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `content`

- **说明**：文字及其它内容
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：请使用 `content` 代替
- **类型**：ReactNode
- **默认值**：-

#### `type`

- **说明**：设置按钮类型
- **类型**：`default` | `primary`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `primary` | 主色强调 |

#### `shape`

- **说明**：设置按钮形状
- **类型**：`circle` | `square`
- **默认值**：`circle`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `circle` | 圆形 |
  | `square` | 方形 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：按钮是否禁用
- **类型**：boolean
- **默认值**：-
- **版本**：6.4.0

#### `placement`

- **说明**：自定义菜单弹出位置
- **类型**：`top` | `left` | `right` | `bottom`
- **默认值**：`top`
- **版本**：5.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `left` | 左侧 |
  | `right` | 右侧 |
  | `bottom` | 下方 |

#### `duration`

- **说明**：回到顶部所需时间（ms）
- **类型**：number
- **默认值**：450

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

- 用于网站上的全局功能；
- 无论浏览到何处都可以看见的按钮。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **形状**（`shape.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **描述**（`content.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **含有气泡卡片的悬浮按钮**（`tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **浮动按钮组**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **菜单模式**（`group-menu.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **受控模式**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **弹出方向**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **可拖拽**（`draggable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **回到顶部**（`back-top.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **徽标数**（`badge.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击按钮时的回调 |
| `open` | 受控显隐 | 受控展开，需配合 trigger 一起使用 |
| `onOpenChange` | 显隐变化 | 展开收起时的回调，需配合 trigger 一起使用 |
| `disabled` | 禁用 | 按钮是否禁用 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 类型 | `type.tsx` | 否 |
| 形状 | `shape.tsx` | 否 |
| 描述 | `content.tsx` | 否 |
| 含有气泡卡片的悬浮按钮 | `tooltip.tsx` | 否 |
| 浮动按钮组 | `group.tsx` | 否 |
| 菜单模式 | `group-menu.tsx` | 否 |
| 受控模式 | `controlled.tsx` | 否 |
| 弹出方向 | `placement.tsx` | 否 |
| 可拖拽 | `draggable.tsx` | 否 |
| 回到顶部 | `back-top.tsx` | 否 |
| 徽标数 | `badge.tsx` | 否 |
| 调试小圆点使用 | `badge-debug.tsx` | 是 |
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

> 自 `antd@5.0.0` 版本开始提供该组件。

### 共同的 API

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| icon | 自定义图标 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | content | 文字及其它内容 | ReactNode | - | ~~description~~ | 请使用 `content` 代替 | ReactNode | - | tooltip | 气泡卡片的内容 | ReactNode \| [TooltipProps](/components/tooltip-cn#api) | - | TooltipProps: 5.25.0 | × |
| type | 设置按钮类型 | `default` \| `primary` | `default` | shape | 设置按钮形状 | `circle` \| `square` | `circle` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onClick | 点击按钮时的回调 | (event) => void | - | href | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | target | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | htmlType | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | 5.21.0 | × |
| badge | 带徽标数字的悬浮按钮（不支持 `status` 以及相关属性） | [BadgeProps](/components/badge-cn#api) | - | 5.4.0 | × |
| disabled | 按钮是否禁用 | boolean | - | 6.4.0 | × |

### FloatButton.Group

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| shape | 设置包含的 FloatButton 按钮形状 | `circle` \| `square` | `circle` | trigger | 触发方式（有触发方式为菜单模式） | `click` \| `hover` | - | open | 受控展开，需配合 trigger 一起使用 | boolean | - | closeIcon | 自定义关闭按钮 | React.ReactNode | `<CloseOutlined />` | placement | 自定义菜单弹出位置 | `top` \| `left` \| `right` \| `bottom` | `top` | 5.21.0 | × |
| onOpenChange | 展开收起时的回调，需配合 trigger 一起使用 | (open: boolean) => void | - | onClick | 点击按钮时的回调（仅在菜单模式中有效） | (event) => void | - | 5.3.0 | × |

### FloatButton.BackTop

| 参数             | 说明                               | 类型              | 默认值       | 版本 |
| ---------------- | ---------------------------------- | ----------------- | ------------ | ---- |
| duration         | 回到顶部所需时间（ms）             | number            | 450          |      |
| target           | 设置需要监听其滚动事件的元素       | () => HTMLElement | () => window |      |
| visibilityHeight | 滚动高度达到此参数值才出现 BackTop | number            | 400          |      |
| onClick          | 点击按钮的回调函数                 | () => void        | -            |      |

### 导入方式

```js
import { FloatButton } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `icon` | 自定义图标 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `content` | 文字及其它内容 | ReactNode | - | — |
| `description` | 请使用 `content` 代替 | ReactNode | - | — |
| `tooltip` | 气泡卡片的内容 | ReactNode \| [TooltipProps](/components/tooltip-cn#api) | - | TooltipProps: 5.25.0 |
| `type` | 设置按钮类型 | `default` \| `primary` | `default` | — |
| `shape` | 设置按钮形状 | `circle` \| `square` | `circle` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onClick` | 点击按钮时的回调 | (event) => void | - | — |
| `href` | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | — |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | — |
| `htmlType` | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | 5.21.0 |
| `badge` | 带徽标数字的悬浮按钮（不支持 `status` 以及相关属性） | [BadgeProps](/components/badge-cn#api) | - | 5.4.0 |
| `disabled` | 按钮是否禁用 | boolean | - | 6.4.0 |
| `trigger` | 触发方式（有触发方式为菜单模式） | `click` \| `hover` | - | — |
| `open` | 受控展开，需配合 trigger 一起使用 | boolean | - | — |
| `closeIcon` | 自定义关闭按钮 | React.ReactNode | `` | — |
| `placement` | 自定义菜单弹出位置 | `top` \| `left` \| `right` \| `bottom` | `top` | 5.21.0 |
| `onOpenChange` | 展开收起时的回调，需配合 trigger 一起使用 | (open: boolean) => void | - | — |
| `duration` | 回到顶部所需时间（ms） | number | 450 | — |
| `visibilityHeight` | 滚动高度达到此参数值才出现 BackTop | number | 400 | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **FloatButton** 的验收清单：

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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/float-button
- 中文文档：https://ant.design/components/float-button-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/float-button
- 驱动 gpui kit：`float-button`
