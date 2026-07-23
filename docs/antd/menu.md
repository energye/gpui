# Menu 导航菜单
> 来源：[Ant Design 6.5.x Menu](https://ant.design/components/menu)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：为页面和功能提供导航的菜单列表。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
