# Tabs 标签页
> 来源：[Ant Design 6.5.x Tabs](https://ant.design/components/tabs)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：选项卡切换组件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

选项卡切换组件。

**Tabs** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 禁用 | disabled 灰态与不可点 |
| 居中 | 复现「居中」视觉与布局 |
| 图标 | icon 与文本混排 |
| 指示条 | 复现「指示条」视觉与布局 |
| 滑动 | 复现「滑动」视觉与布局 |
| 附加内容 | 复现「附加内容」视觉与布局 |
| 大小 | 不同 size 档位 |
| 位置 | placement 方位 |
| 卡片式页签 | card 风格容器 |
| 新增和关闭页签 | 复现「新增和关闭页签」视觉与布局 |
| 自定义新增页签触发器 | 自定义渲染/插槽外观 |
| 自定义页签头 | 自定义渲染/插槽外观 |
| 可拖拽标签 | 复现「可拖拽标签」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `animated`

- **说明**：是否使用动画切换 Tabs
- **类型**：boolean| { inkBar: boolean, tabPane: boolean }
- **默认值**：{ inkBar: true, tabPane: false }

#### `centered`

- **说明**：标签居中展示
- **类型**：boolean
- **默认值**：false
- **版本**：4.4.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `hideAdd`

- **说明**：是否隐藏加号图标，在 `type="editable-card"` 时有效
- **类型**：boolean
- **默认值**：false

#### `indicator`

- **说明**：自定义指示条的长度和对齐方式
- **类型**：{ size?: number | (origin: number) => number; align: `start` | `center` | `end`; }
- **默认值**：-
- **版本**：5.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `center` | 居中 |
  | `end` | 逻辑结束侧 |

#### `size`

- **说明**：大小，提供 `large` `medium` 和 `small` 三种大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
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

#### `tabBarStyle`

- **说明**：tab bar 的样式对象
- **类型**：CSSProperties
- **默认值**：-

#### `tabPlacement`

- **说明**：页签位置，可选值有 `top` `end` `bottom` `start`
- **类型**：`top` | `end` | `bottom` | `start`
- **默认值**：`top`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `end` | 逻辑结束侧 |
  | `bottom` | 下方 |
  | `start` | 逻辑起始侧 |

#### `tabPosition`

- **说明**：页签位置，可选值有 `top` `right` `bottom` `left`，请使用 `tabPlacement` 替换
- **类型**：`top` | `right` | `bottom` | `left`
- **默认值**：`top`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `right` | 右侧 |
  | `bottom` | 下方 |
  | `left` | 左侧 |

#### `type`

- **说明**：页签的基本样式，可选 `line`、`card` `editable-card` 类型
- **类型**：`line` | `card` | `editable-card`
- **默认值**：`line`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `line` | 线型 |
  | `card` | 卡片型 |
  | `editable-card` | 可编辑卡片型 |

#### `closeIcon`

- **说明**：自定义关闭图标，在 `type="editable-card"` 时有效。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮
- **类型**：ReactNode
- **默认值**：-

#### `disabled`

- **说明**：禁用某一项
- **类型**：boolean
- **默认值**：false

#### `icon`

- **说明**：选项卡头部图标元素
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.12.0

#### `closable`

- **说明**：是否显示选项卡的关闭按钮，在 `type="editable-card"` 时有效
- **类型**：boolean
- **默认值**：true

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

提供平级的区域将大块内容进行收纳和展现，保持界面整洁。

Ant Design 依次提供了三级选项卡，分别用于不同的场景。

- 卡片式的页签，提供可关闭的样式，常用于容器顶部。
- 既可用于容器顶部，也可用于容器内部，是最通用的 Tabs。
- [Radio.Button](/components/radio-cn/#radio-demo-radiobutton) 可作为更次级的页签来使用。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **居中**（`centered.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **图标**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **指示条**（`custom-indicator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **滑动**（`slide.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **附加内容**（`extra.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **卡片式页签**（`card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **新增和关闭页签**（`editable-card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义新增页签触发器**（`custom-add-trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义页签头**（`custom-tab-bar.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **可拖拽标签**（`custom-tab-bar-node.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 切换面板的回调 |
| `disabled` | 禁用 | 禁用某一项 |
| `items` | 数据化 items | 配置选项卡内容 |
| `destroyOnHidden` | 隐藏销毁 | 被隐藏时是否销毁 DOM 结构 |
| `activeKey` | 激活面板 | 当前激活 tab 面板的 key |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 禁用 | `disabled.tsx` | 否 |
| 居中 | `centered.tsx` | 否 |
| 图标 | `icon.tsx` | 否 |
| 指示条 | `custom-indicator.tsx` | 否 |
| 滑动 | `slide.tsx` | 否 |
| 附加内容 | `extra.tsx` | 否 |
| 大小 | `size.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 卡片式页签 | `card.tsx` | 否 |
| 新增和关闭页签 | `editable-card.tsx` | 否 |
| 卡片式页签容器 | `card-top.tsx` | 是 |
| 自定义新增页签触发器 | `custom-add-trigger.tsx` | 否 |
| 自定义页签头 | `custom-tab-bar.tsx` | 否 |
| 可拖拽标签 | `custom-tab-bar-node.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 动画 | `animated.tsx` | 是 |
| 嵌套 | `nest.tsx` | 是 |
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

### Tabs

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| activeKey | 当前激活 tab 面板的 key | string | - | addIcon | 自定义添加按钮，设置 `type="editable-card"` 时有效 | ReactNode | `<PlusOutlined />` | 4.4.0 | 5.14.0 |
| animated | 是否使用动画切换 Tabs | boolean\| { inkBar: boolean, tabPane: boolean } | { inkBar: true, tabPane: false } | centered | 标签居中展示 | boolean | false | 4.4.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultActiveKey | 初始化选中面板的 key，如果没有设置 activeKey | string | `第一个面板的 key` | hideAdd | 是否隐藏加号图标，在 `type="editable-card"` 时有效 | boolean | false | indicator | 自定义指示条的长度和对齐方式 | { size?: number \| (origin: number) => number; align: `start` \| `center` \| `end`; } | - | 5.13.0 | 5.13.0 |
| items | 配置选项卡内容 | [TabItemType](#tabitemtype) | [] | 4.23.0 | × |
| more | 自定义折叠菜单属性 | [MoreProps](#moreprops) | { icon: `<EllipsisOutlined />` , trigger: 'hover' } | removeIcon | 自定义删除按钮，设置 `type="editable-card"` 时有效 | ReactNode | `<CloseOutlined />` | 5.15.0 | 5.15.0 |
| ~~popupClassName~~ | 更多菜单的 `className`, 请使用 `classNames.popup` 替换 | string | - | 4.21.0 | × |
| renderTabBar | 替换 TabBar，用于二次封装标签头 | (props: DefaultTabBarProps, DefaultTabBar: React.ComponentClass) => React.ReactElement | - | size | 大小，提供 `large` `medium` 和 `small` 三种大小 | `large` \| `medium` \| `small` | `medium` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | tabBarExtraContent | tab bar 上额外的元素 | ReactNode \| {left?: ReactNode, right?: ReactNode} | - | object: 4.6.0 | × |
| tabBarGutter | tabs 之间的间隙 | number | - | tabBarStyle | tab bar 的样式对象 | CSSProperties | - | tabPlacement | 页签位置，可选值有 `top` `end` `bottom` `start` | `top` \| `end` \| `bottom` \| `start` | `top` | ~~tabPosition~~ | 页签位置，可选值有 `top` `right` `bottom` `left`，请使用 `tabPlacement` 替换 | `top` \| `right` \| `bottom` \| `left` | `top` | ~~destroyInactiveTabPane~~ | 被隐藏时是否销毁 DOM 结构，使用 `destroyOnHidden` 代替 | boolean | false | destroyOnHidden | 被隐藏时是否销毁 DOM 结构 | boolean | false | 5.25.0 | × |
| type | 页签的基本样式，可选 `line`、`card` `editable-card` 类型 | `line` \| `card` \| `editable-card` | `line` | onChange | 切换面板的回调 | (activeKey: string) => void | - | onEdit | 新增和删除页签的回调，在 `type="editable-card"` 时有效 | (action === 'add' ? event : targetKey, action) => void | - | onTabClick | tab 被点击的回调 | (key: string, event: MouseEvent) => void | - | onTabScroll | tab 滚动时触发 | ({ direction: `left` \| `right` \| `top` \| `bottom` }) => void | - | 4.3.0 | × |

> 更多属性查看 [@rc-component/tabs](https://github.com/react-component/tabs#tabs)

### TabItemType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| closeIcon | 自定义关闭图标，在 `type="editable-card"` 时有效。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | - | destroyOnHidden | 被隐藏时是否销毁 DOM 结构 | boolean | false | 5.25.0 |
| disabled | 禁用某一项 | boolean | false | key | 对应 activeKey | string | - | icon | 选项卡头部图标元素 | ReactNode | - | 5.12.0 |
| children | 选项卡内容元素 | ReactNode | - 
### MoreProps

| 参数                                         | 说明           | 类型      | 默认值 | 版本 |
| -------------------------------------------- | -------------- | --------- | ------ | ---- |
| icon                                         | 自定义折叠图标 | ReactNode | -      |      |
| [DropdownProps](/components/dropdown-cn#api) |                |           |        |      |

### 导入方式

```js
import { Tabs } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `activeKey` | 当前激活 tab 面板的 key | string | - | — |
| `addIcon` | 自定义添加按钮，设置 `type="editable-card"` 时有效 | ReactNode | `` | 4.4.0 |
| `animated` | 是否使用动画切换 Tabs | boolean\| { inkBar: boolean, tabPane: boolean } | { inkBar: true, tabPane: false } | — |
| `centered` | 标签居中展示 | boolean | false | 4.4.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultActiveKey` | 初始化选中面板的 key，如果没有设置 activeKey | string | `第一个面板的 key` | — |
| `hideAdd` | 是否隐藏加号图标，在 `type="editable-card"` 时有效 | boolean | false | — |
| `indicator` | 自定义指示条的长度和对齐方式 | { size?: number \| (origin: number) => number; align: `start` \| `center` \| `end`; } | - | 5.13.0 |
| `items` | 配置选项卡内容 | [TabItemType](#tabitemtype) | [] | 4.23.0 |
| `more` | 自定义折叠菜单属性 | [MoreProps](#moreprops) | { icon: `` , trigger: 'hover' } | — |
| `removeIcon` | 自定义删除按钮，设置 `type="editable-card"` 时有效 | ReactNode | `` | 5.15.0 |
| `popupClassName` | 更多菜单的 `className`, 请使用 `classNames.popup` 替换 | string | - | 4.21.0 |
| `renderTabBar` | 替换 TabBar，用于二次封装标签头 | (props: DefaultTabBarProps, DefaultTabBar: React.ComponentClass) => React.ReactElement | - | — |
| `size` | 大小，提供 `large` `medium` 和 `small` 三种大小 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `tabBarExtraContent` | tab bar 上额外的元素 | ReactNode \| {left?: ReactNode, right?: ReactNode} | - | object: 4.6.0 |
| `tabBarGutter` | tabs 之间的间隙 | number | - | — |
| `tabBarStyle` | tab bar 的样式对象 | CSSProperties | - | — |
| `tabPlacement` | 页签位置，可选值有 `top` `end` `bottom` `start` | `top` \| `end` \| `bottom` \| `start` | `top` | — |
| `tabPosition` | 页签位置，可选值有 `top` `right` `bottom` `left`，请使用 `tabPlacement` 替换 | `top` \| `right` \| `bottom` \| `left` | `top` | — |
| `destroyInactiveTabPane` | 被隐藏时是否销毁 DOM 结构，使用 `destroyOnHidden` 代替 | boolean | false | — |
| `destroyOnHidden` | 被隐藏时是否销毁 DOM 结构 | boolean | false | 5.25.0 |
| `type` | 页签的基本样式，可选 `line`、`card` `editable-card` 类型 | `line` \| `card` \| `editable-card` | `line` | — |
| `onChange` | 切换面板的回调 | (activeKey: string) => void | - | — |
| `onEdit` | 新增和删除页签的回调，在 `type="editable-card"` 时有效 | (action === 'add' ? event : targetKey, action) => void | - | — |
| `onTabClick` | tab 被点击的回调 | (key: string, event: MouseEvent) => void | - | — |
| `onTabScroll` | tab 滚动时触发 | ({ direction: `left` \| `right` \| `top` \| `bottom` }) => void | - | 4.3.0 |
| `closeIcon` | 自定义关闭图标，在 `type="editable-card"` 时有效。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | - | — |
| `disabled` | 禁用某一项 | boolean | false | — |
| `forceRender` | 被隐藏时是否渲染 DOM 结构 | boolean | false | — |
| `key` | 对应 activeKey | string | - | — |
| `label` | 选项卡头部文字元素 | ReactNode | - | — |
| `icon` | 选项卡头部图标元素 | ReactNode | - | 5.12.0 |
| `children` | 选项卡内容元素 | ReactNode | - | — |
| `closable` | 是否显示选项卡的关闭按钮，在 `type="editable-card"` 时有效 | boolean | true | — |
| `[DropdownProps](/components/dropdown-cn#api)` | — | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Tabs** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **15** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tabs
- 中文文档：https://ant.design/components/tabs-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tabs
- 驱动 gpui kit：`tabs`
