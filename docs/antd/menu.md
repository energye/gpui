# Menu 导航菜单
> 来源：[Ant Design 6.5.x Menu](https://ant.design/components/menu)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：为页面和功能提供导航的菜单列表。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

为页面和功能提供导航的菜单列表。

**Menu** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 顶部导航 | 复现「顶部导航」视觉与布局 |
| 内嵌菜单 | 复现「内嵌菜单」视觉与布局 |
| 缩起内嵌菜单 | 复现「缩起内嵌菜单」视觉与布局 |
| 菜单项提示 | 复现「菜单项提示」视觉与布局 |
| 只展开当前父级菜单 | 展开/折叠指示 |
| 垂直菜单 | 纵向布局 |
| 主题 | light/dark 或主题色 |
| 子菜单主题 | light/dark 或主题色 |
| 切换菜单类型 | type 预设外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 自定义弹出框 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `expandIcon`

- **说明**：自定义展开图标
- **类型**：ReactNode | `(props: SubMenuProps & { isSubMenu: boolean }) => ReactNode`
- **默认值**：-
- **版本**：4.9.0

#### `inlineCollapsed`

- **说明**：inline 时菜单是否收起状态
- **类型**：boolean
- **默认值**：-

#### `mode`

- **说明**：菜单类型，现在支持垂直、水平、和内嵌模式三种
- **类型**：`vertical` | `horizontal` | `inline`
- **默认值**：`vertical`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |
  | `inline` | 内联紧凑 |

#### `overflowedIndicator`

- **说明**：用于自定义 Menu 水平空间不足时的省略收缩的图标
- **类型**：ReactNode
- **默认值**：``

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `theme`

- **说明**：主题颜色
- **类型**：`light` | `dark`
- **默认值**：`light`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `light` | 浅色主题 |
  | `dark` | 深色主题 |

#### `danger`

- **说明**：展示错误状态样式
- **类型**：boolean
- **默认值**：false

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `extra`

- **说明**：额外节点
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.21.0

#### `icon`

- **说明**：菜单图标
- **类型**：ReactNode
- **默认值**：-

#### `label`

- **说明**：菜单项标题
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：设置收缩时展示的悬浮标题
- **类型**：string
- **默认值**：-

#### `popupClassName`

- **说明**：子菜单样式，`mode="inline"` 时无效
- **类型**：string
- **默认值**：-

#### `onTitleClick`

- **说明**：点击子菜单标题
- **类型**：function({ key, domEvent })
- **默认值**：-

#### `dashed`

- **说明**：是否虚线
- **类型**：boolean
- **默认值**：false

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

导航菜单是一个网站的灵魂，用户依赖导航在各个页面中进行跳转。一般分为顶部导航和侧边导航，顶部导航提供全局性的类目和功能，侧边导航提供多级结构来收纳和排列网站架构。

更多布局和导航的使用可以参考：[通用布局](/components/layout-cn)。

### 2.2 核心功能（按官方示例拆解）

1. **顶部导航**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **内嵌菜单**（`inline.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **缩起内嵌菜单**（`inline-collapsed.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **菜单项提示**（`tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **只展开当前父级菜单**（`sider-current.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **垂直菜单**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **主题**（`theme.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **子菜单主题**（`submenu-theme.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **切换菜单类型**（`switch-mode.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义弹出框**（`custom-popup-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击 MenuItem 调用此函数 |
| `onSelect` | 选中 | 被选中时调用 |
| `onDeselect` | 取消选中 | 取消选中时调用，仅在 multiple 生效 |
| `onOpenChange` | 显隐变化 | SubMenu 展开/关闭的回调 |
| `disabled` | 禁用 | 是否禁用 |
| `items` | 数据化 items | 菜单内容 |
| `selectedKeys` | 选中 keys | 当前选中的菜单项 key 数组 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 顶部导航 | `horizontal.tsx` | 否 |
| 顶部导航（dark） | `horizontal-dark.tsx` | 是 |
| 内嵌菜单 | `inline.tsx` | 否 |
| 缩起内嵌菜单 | `inline-collapsed.tsx` | 否 |
| 菜单项提示 | `tooltip.tsx` | 否 |
| 只展开当前父级菜单 | `sider-current.tsx` | 否 |
| 垂直菜单 | `vertical.tsx` | 否 |
| 主题 | `theme.tsx` | 否 |
| 子菜单主题 | `submenu-theme.tsx` | 否 |
| 切换菜单类型 | `switch-mode.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Style debug | `style-debug.tsx` | 是 |
| v4 版本 Menu | `menu-v4.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| Extra Style debug | `extra-style.tsx` | 是 |
| Extra 折叠调试 | `extra-collapsed-debug.tsx` | 是 |
| 自定义弹出框 | `custom-popup-render.tsx` | 否 |
| 折叠菜单 icon 对齐 | `collapsed-icon-debug.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 为何 Menu 的子元素会渲染两次？ {#faq-render-twice}

Menu 通过[二次渲染](https://github.com/react-component/menu/blob/f4684514096d6b7123339cbe72e7b0f68db0bce2/src/Menu.tsx#L543)收集嵌套结构信息以支持 HOC 的结构。合并成一个推导结构会使得逻辑变得十分复杂，欢迎 PR 以协助改进该设计。

### 在 Flex 布局中，Menu 没有按照预期响应式省略菜单？ {#faq-flex-layout}

Menu 初始化时会先全部渲染，然后根据宽度裁剪内容。当处于 Flex 布局中，你需要告知其预期宽度为响应式宽度（[在线 Demo](https://codesandbox.io/s/ding-bu-dao-hang-antd-4-21-7-forked-5e3imy?file=/demo.js)）：

```jsx

  Some Content
  

```

## Semantic DOM

## 主题变量（Design Token）{#design-token}

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

### Menu

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | defaultOpenKeys | 初始展开的 SubMenu 菜单项 key 数组 | string\[] | - | defaultSelectedKeys | 初始选中的菜单项 key 数组 | string\[] | - | expandIcon | 自定义展开图标 | ReactNode \| `(props: SubMenuProps & { isSubMenu: boolean }) => ReactNode` | - | 4.9.0 | 5.15.0 |
| forceSubMenuRender | 在子菜单展示之前就渲染进 DOM | boolean | false | inlineCollapsed | inline 时菜单是否收起状态 | boolean | - | inlineIndent | inline 模式的菜单缩进宽度 | number | 24 | items | 菜单内容 | [ItemType\[\]](#itemtype) | - | 4.20.0 | × |
| mode | 菜单类型，现在支持垂直、水平、和内嵌模式三种 | `vertical` \| `horizontal` \| `inline` | `vertical` | multiple | 是否允许多选 | boolean | false | openKeys | 当前展开的 SubMenu 菜单项 key 数组 | string\[] | - | overflowedIndicator | 用于自定义 Menu 水平空间不足时的省略收缩的图标 | ReactNode | `<EllipsisOutlined />` | selectable | 是否允许选中 | boolean | true | selectedKeys | 当前选中的菜单项 key 数组 | string\[] | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> | - | subMenuCloseDelay | 用户鼠标离开子菜单后关闭延时，单位：秒 | number | 0.1 | subMenuOpenDelay | 用户鼠标进入子菜单后开启延时，单位：秒 | number | 0 | tooltip | 配置 inline 折叠时的 MenuItem 悬浮提示，设为 `false` 可关闭 | false \| TooltipProps | - | 6.3.0 | × |
| theme | 主题颜色 | `light` \| `dark` | `light` | triggerSubMenuAction | SubMenu 展开/关闭的触发行为 | `hover` \| `click` | `hover` | onClick | 点击 MenuItem 调用此函数 | function({ key, keyPath, domEvent, itemData }) | - | onDeselect | 取消选中时调用，仅在 multiple 生效 | function({ key, keyPath, selectedKeys, domEvent, itemData }) | - | onOpenChange | SubMenu 展开/关闭的回调 | function(openKeys: string\[]) | - | onSelect | 被选中时调用 | function({ key, keyPath, selectedKeys, domEvent, itemData }) | - | popupRender | 自定义子菜单的弹出框 | (node: ReactElement, props: { item: SubMenuProps; keys: string[] }) => ReactElement | - 
> 更多属性查看 [@rc-component/menu](https://github.com/react-component/menu#api)

### ItemType

> type ItemType = [MenuItemType](#menuitemtype) | [SubMenuType](#submenutype) | [MenuItemGroupType](#menuitemgrouptype) | [MenuDividerType](#menudividertype);

#### MenuItemType

| 参数     | 说明                     | 类型      | 默认值 | 版本   |
| -------- | ------------------------ | --------- | ------ | ------ |
| danger   | 展示错误状态样式         | boolean   | false  |        |
| disabled | 是否禁用                 | boolean   | false  |        |
| extra    | 额外节点                 | ReactNode | -      | 5.21.0 |
| icon     | 菜单图标                 | ReactNode | -      |        |
| key      | item 的唯一标志          | string    | -      |        |
| label    | 菜单项标题               | ReactNode | -      |        |
| title    | 设置收缩时展示的悬浮标题 | string    | -      |        |

#### SubMenuType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| children | 子菜单的菜单项 | [ItemType\[\]](#itemtype) | - | icon | 菜单图标 | ReactNode | - | label | 菜单项标题 | ReactNode | - | popupOffset | 子菜单偏移量，`mode="inline"` 时无效 | \[number, number] | - | theme | 设置子菜单的主题，默认从 Menu 上继承 | `light` \| `dark` | - 
#### MenuItemGroupType

定义类型为 `group` 时，会作为分组处理:

```ts
const groupItem = {
  type: 'group', // Must have
  label: 'My Group',
  children: [],
};
```

| 参数     | 说明         | 类型                              | 默认值 | 版本 |
| -------- | ------------ | --------------------------------- | ------ | ---- |
| children | 分组的菜单项 | [MenuItemType\[\]](#menuitemtype) | -      |      |
| label    | 分组标题     | ReactNode                         | -      |      |

#### MenuDividerType

菜单项分割线，只用在弹出菜单内，需要定义类型为 `divider`：

```ts
const dividerItem = {
  type: 'divider', // Must have
};
```

| 参数   | 说明     | 类型    | 默认值 | 版本 |
| ------ | -------- | ------- | ------ | ---- |
| dashed | 是否虚线 | boolean | false  |      |

### 导入方式

```js
import { Menu } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `defaultOpenKeys` | 初始展开的 SubMenu 菜单项 key 数组 | string\[] | - | — |
| `defaultSelectedKeys` | 初始选中的菜单项 key 数组 | string\[] | - | — |
| `expandIcon` | 自定义展开图标 | ReactNode \| `(props: SubMenuProps & { isSubMenu: boolean }) => ReactNode` | - | 4.9.0 |
| `forceSubMenuRender` | 在子菜单展示之前就渲染进 DOM | boolean | false | — |
| `inlineCollapsed` | inline 时菜单是否收起状态 | boolean | - | — |
| `inlineIndent` | inline 模式的菜单缩进宽度 | number | 24 | — |
| `items` | 菜单内容 | [ItemType\[\]](#itemtype) | - | 4.20.0 |
| `mode` | 菜单类型，现在支持垂直、水平、和内嵌模式三种 | `vertical` \| `horizontal` \| `inline` | `vertical` | — |
| `multiple` | 是否允许多选 | boolean | false | — |
| `openKeys` | 当前展开的 SubMenu 菜单项 key 数组 | string\[] | - | — |
| `overflowedIndicator` | 用于自定义 Menu 水平空间不足时的省略收缩的图标 | ReactNode | `` | — |
| `selectable` | 是否允许选中 | boolean | true | — |
| `selectedKeys` | 当前选中的菜单项 key 数组 | string\[] | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `subMenuCloseDelay` | 用户鼠标离开子菜单后关闭延时，单位：秒 | number | 0.1 | — |
| `subMenuOpenDelay` | 用户鼠标进入子菜单后开启延时，单位：秒 | number | 0 | — |
| `tooltip` | 配置 inline 折叠时的 MenuItem 悬浮提示，设为 `false` 可关闭 | false \| TooltipProps | - | 6.3.0 |
| `theme` | 主题颜色 | `light` \| `dark` | `light` | — |
| `triggerSubMenuAction` | SubMenu 展开/关闭的触发行为 | `hover` \| `click` | `hover` | — |
| `onClick` | 点击 MenuItem 调用此函数 | function({ key, keyPath, domEvent, itemData }) | - | — |
| `onDeselect` | 取消选中时调用，仅在 multiple 生效 | function({ key, keyPath, selectedKeys, domEvent, itemData }) | - | — |
| `onOpenChange` | SubMenu 展开/关闭的回调 | function(openKeys: string\[]) | - | — |
| `onSelect` | 被选中时调用 | function({ key, keyPath, selectedKeys, domEvent, itemData }) | - | — |
| `popupRender` | 自定义子菜单的弹出框 | (node: ReactElement, props: { item: SubMenuProps; keys: string[] }) => ReactElement | - | — |
| `danger` | 展示错误状态样式 | boolean | false | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `extra` | 额外节点 | ReactNode | - | 5.21.0 |
| `icon` | 菜单图标 | ReactNode | - | — |
| `key` | item 的唯一标志 | string | - | — |
| `label` | 菜单项标题 | ReactNode | - | — |
| `title` | 设置收缩时展示的悬浮标题 | string | - | — |
| `children` | 子菜单的菜单项 | [ItemType\[\]](#itemtype) | - | — |
| `popupClassName` | 子菜单样式，`mode="inline"` 时无效 | string | - | — |
| `popupOffset` | 子菜单偏移量，`mode="inline"` 时无效 | \[number, number] | - | — |
| `onTitleClick` | 点击子菜单标题 | function({ key, domEvent }) | - | — |
| `dashed` | 是否虚线 | boolean | false | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Menu** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **11** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/menu
- 中文文档：https://ant.design/components/menu-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/menu
- 驱动 gpui kit：`menu`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Menu** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/menu/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Menu）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 选中/展开/分页或步骤切换与键盘 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Menu）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：为页面和功能提供导航的菜单列表。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 菜单项高 itemHeight | **40** | controlHeightLG |
| itemHeight | **40** | controlHeightLG |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `defaultOpenKeys` | 初始展开的 SubMenu 菜单项 key 数组 | string\[] | - |
| `defaultSelectedKeys` | 初始选中的菜单项 key 数组 | string\[] | - |
| `expandIcon` | 自定义展开图标 | ReactNode \ | `(props: SubMenuProps & { isSubMenu: boolean }) => ReactNode` |
| `forceSubMenuRender` | 在子菜单展示之前就渲染进 DOM | boolean | false |
| `inlineCollapsed` | inline 时菜单是否收起状态 | boolean | - |
| `inlineIndent` | inline 模式的菜单缩进宽度 | number | 24 |
| `items` | 菜单内容 | [ItemType\[\]](#itemtype) | - |
| `mode` | 菜单类型，现在支持垂直、水平、和内嵌模式三种 | `vertical` \ | `horizontal` \ |
| `multiple` | 是否允许多选 | boolean | false |
| `openKeys` | 当前展开的 SubMenu 菜单项 key 数组 | string\[] | - |
| `overflowedIndicator` | 用于自定义 Menu 水平空间不足时的省略收缩的图标 | ReactNode | `<EllipsisOutlined />` |
| `selectable` | 是否允许选中 | boolean | true |
| `selectedKeys` | 当前选中的菜单项 key 数组 | string\[] | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) ,… | (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> |
| `subMenuCloseDelay` | 用户鼠标离开子菜单后关闭延时，单位：秒 | number | 0.1 |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
mount ──► mode=vertical|horizontal|inline 渲染 items
             ├── 点 item ──► selectedKeys + onClick/onSelect
             ├── SubMenu ──► openKeys 展开（inline 内嵌 / 其它 popup）
             ├── inlineCollapsed ──► 窄栏；sub 变 popup
             ├── disabled item ──► 不可选
             ├── theme dark/light ──► 色板
             └── 键盘 ↑↓ Enter / Esc（popup）──► 移动/激活/关闭
```

\*itemHeight 默认 controlHeightLG=40。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| MNU-S1 | 点菜单项 | `onClick`；`selectedKeys` 含其 key |
| MNU-S2 | 打开 SubMenu | `openKeys` 更新；子项可见 |
| MNU-S3 | 受控 selectedKeys | 外部优先 |
| MNU-S4 | disabled 项点击 | 不选中 |
| MNU-S5 | `mode` 切换 | 布局变为水平/直/inline |
| MNU-S6 | `theme=dark` | 深底浅字 |
| MNU-S7 | `inlineCollapsed=true` | 宽度收窄；图标可见 |
| MNU-S8 | 项高度 | ≈40 |
| MNU-S9 | 键盘 Enter 在项上 | 激活同点击 |
| MNU-S10 | 多选 multiple（若开） | 可多 selectedKeys |
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
| 角色 | navigation / menu / tablist 等 |
| 当前 | aria-current / selected |
| 键盘 | 方向键与激活 |

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
| `onClick` | 必须 |
| `disabled` | 必须 |
| `onOpenChange` | 必须 |
| `items` | 必须 |
| `children` | 必须 |
| `title` | 必须 |
| `mode` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | 顶部导航、内嵌菜单、缩起内嵌菜单、菜单项提示、只展开当前父级菜单、垂直菜单、主题、子菜单主题 |
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
| 其余示例 | 切换菜单类型, 自定义语义结构的样式和类, 自定义弹出框, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestMenu_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Menu 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| MNU-01 | L1 | NewMenu 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| MNU-02 | L1 | 点菜单项 | `onClick`；`selectedKeys` 含其 key |
| MNU-03 | L1 | 打开 SubMenu | `openKeys` 更新；子项可见 |
| MNU-04 | L1 | 受控 selectedKeys | 外部优先 |
| MNU-05 | L1 | disabled 项点击 | 不选中 |
| MNU-06 | L1 | `mode` 切换 | 布局变为水平/直/inline |
| MNU-07 | L1 | `theme=dark` | 深底浅字 |
| MNU-08 | L1 | `inlineCollapsed=true` | 宽度收窄；图标可见 |
| MNU-09 | L1 | 项高度 | ≈40 |
| MNU-10 | L1 | 键盘 Enter 在项上 | 激活同点击 |
| MNU-11 | L1 | 多选 multiple（若开） | 可多 selectedKeys |
| MNU-12 | L1 | 复现官方示例「顶部导航」（`horizontal.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-13 | L1 | 复现官方示例「内嵌菜单」（`inline.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-14 | L1 | 复现官方示例「缩起内嵌菜单」（`inline-collapsed.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-15 | L1 | 复现官方示例「菜单项提示」（`tooltip.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-16 | L1 | 复现官方示例「只展开当前父级菜单」（`sider-current.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-17 | L1 | 复现官方示例「垂直菜单」（`vertical.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-18 | L1 | 复现官方示例「主题」（`theme.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-19 | L1 | 复现官方示例「子菜单主题」（`submenu-theme.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| MNU-20 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| MNU-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| MNU-22 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| MNU-23 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| MNU-24 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| MNU-25 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| MNU-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewMenu(...) *Menu

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
Nav root
  └─ items / panels / connectors
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Menu 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/menu.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Menu 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
