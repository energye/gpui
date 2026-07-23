# Tree 树形控件
> 来源：[Ant Design 6.5.x Tree](https://ant.design/components/tree)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：多层次的结构列表。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

多层次的结构列表。

**Tree** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 受控操作示例 | 复现「受控操作示例」视觉与布局 |
| 拖动示例 | 复现「拖动示例」视觉与布局 |
| 异步数据加载 | loading 指示与防重复 |
| 可搜索 | 带搜索框外观 |
| 连接线 | 复现「连接线」视觉与布局 |
| 自定义图标 | icon 与文本混排 |
| 目录 | 复现「目录」视觉与布局 |
| 自定义展开/折叠图标 | icon 与文本混排 |
| 虚拟滚动 | 复现「虚拟滚动」视觉与布局 |
| 占据整行 | 复现「占据整行」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `blockNode`

- **说明**：是否节点占据一行
- **类型**：boolean
- **默认值**：false

#### `checkStrictly`

- **说明**：checkable 状态下节点选择完全受控（父子节点选中状态不再关联）
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：将树禁用
- **类型**：boolean
- **默认值**：false

#### `draggable`

- **说明**：设置节点可拖拽，可以通过 `icon: false` 关闭拖拽提示图标
- **类型**：boolean | ((node: DataNode) => boolean) | { icon?: React.ReactNode | false, nodeDraggable?: (node: DataNode) => boolean }
- **默认值**：false
- **版本**：`config`: 4.17.0

#### `height`

- **说明**：设置虚拟滚动容器高度，设置后内部节点不再支持横向滚动
- **类型**：number
- **默认值**：-

#### `icon`

- **说明**：在标题之前插入自定义图标。需要设置 `showIcon` 为 true
- **类型**：ReactNode | (props) => ReactNode
- **默认值**：-

#### `loadData`

- **说明**：异步加载数据
- **类型**：function(node)
- **默认值**：-

#### `loadedKeys`

- **说明**：（受控）已经加载的节点，需要配合 `loadData` 使用
- **类型**：string\[]
- **默认值**：\[]
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `loadData` | 官方取值 `loadData` |

#### `rootStyle`

- **说明**：Tree 最外层样式，请使用 `styles.root` 替代
- **类型**：CSSProperties
- **默认值**：-
- **版本**：4.20.0

#### `showIcon`

- **说明**：控制是否展示 `icon` 节点，没有默认样式
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `icon` | 官方取值 `icon` |

#### `showLine`

- **说明**：是否展示连接线
- **类型**：boolean | { showLeafIcon: ReactNode | ((props: AntTreeNodeProps) => ReactNode) }
- **默认值**：false

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `switcherIcon`

- **说明**：自定义树节点的展开/折叠图标（`showLine` 下不会自动 rotate）
- **类型**：ReactNode | ((props: AntTreeNodeProps) => ReactNode)
- **默认值**：-
- **版本**：renderProps: 4.20.0

#### `switcherLoadingIcon`

- **说明**：自定义树节点的加载图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.20.0

#### `onLoad`

- **说明**：节点加载完毕时触发
- **类型**：function(loadedKeys, {event, node})
- **默认值**：-

#### `title`

- **说明**：标题
- **类型**：ReactNode
- **默认值**：`---`

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

文件夹、组织架构、生物分类、国家地区等等，世间万物的大多数结构都是树形结构。使用 `树控件` 可以完整展现其中的层级关系，并具有展开收起选择等交互功能。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **受控操作示例**（`basic-controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **拖动示例**（`draggable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **异步数据加载**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **可搜索**（`search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **连接线**（`line.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义图标**（`customized-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **目录**（`directory.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义展开/折叠图标**（`switcher-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **虚拟滚动**（`virtual-scroll.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **占据整行**（`block-node.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onSelect` | 选中 | 点击树节点触发 |
| `disabled` | 禁用 | 将树禁用 |
| `treeData` | 树数据 | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（key 在整个树范围内唯一） |
| `loadData` | 异步加载 | 异步加载数据 |
| `virtual` | 虚拟滚动 | 设置 false 时关闭虚拟滚动 |
| `selectedKeys` | 选中 keys | （受控）设置选中的树节点，多选需设置 `multiple` 为 true |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 受控操作示例 | `basic-controlled.tsx` | 否 |
| 拖动示例 | `draggable.tsx` | 否 |
| 异步数据加载 | `dynamic.tsx` | 否 |
| 可搜索 | `search.tsx` | 否 |
| 连接线 | `line.tsx` | 否 |
| 自定义图标 | `customized-icon.tsx` | 否 |
| 目录 | `directory.tsx` | 否 |
| 目录 Debug | `directory-debug.tsx` | 是 |
| 自定义展开/折叠图标 | `switcher-icon.tsx` | 否 |
| 虚拟滚动 | `virtual-scroll.tsx` | 否 |
| Drag Debug | `drag-debug.tsx` | 是 |
| 大数据 | `big-data.tsx` | 是 |
| 占据整行 | `block-node.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
| 多行 | `multiple-line.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 连接线调试 | `line-debug.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### Tree 方法

| 名称 | 说明 |
| --- | --- |
| scrollTo({ key: string \| number; align?: 'top' \| 'bottom' \| 'auto'; offset?: number }) | 虚拟滚动下，滚动到指定 key 条目 |

### 2.6 FAQ

## FAQ

### defaultExpandAll 在异步加载数据时为何不生效？ {#faq-default-expand-all}

`default` 前缀属性只有在初始化时生效，因而异步加载数据时 `defaultExpandAll` 已经执行完成。你可以通过受控 `expandedKeys` 或者在数据加载完成后渲染 Tree 来实现全部展开。

### 虚拟滚动的限制 {#faq-virtual-scroll-limitation}

虚拟滚动通过在仅渲染可视区域的元素来提升渲染性能。但是同时由于不会渲染所有节点，所以无法自动拓转横向宽度（比如超长 `title` 的横向滚动条）。

### `disabled` 节点在树中的关系是什么？ {#faq-disabled-node}

Tree 通过传导方式进行数据变更。无论是展开还是勾选，它都会从变更的节点开始向上、向下传导变化，直到遍历的当前节点是 `disabled` 时停止。因而如果控制的节点本身为 `disabled` 时，那么它只会修改本身而不会影响其他节点。举例来说，一个父节点包含 3 个子节点，其中一个为 `disabled` 状态。那么勾选父节点，只会影响其余两个子节点变成勾选状态。勾选两个子节点后，无论 `disabled` 节点什么状态，父节点都会变成勾选状态。

这种传导终止的方式是为了防止通过勾选子节点使得 `disabled` 父节点变成勾选状态，而用户无法直接勾选 `disabled` 父节点更改其状态导致的交互矛盾。如果你有着自己的传导需求，可以通过 `checkStrictly` 自定义勾选逻辑。

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

### Tree props

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowDrop | 是否允许拖拽时放置在该节点 | ({ dropNode, dropPosition }) => boolean | - | autoExpandParent | 是否自动展开父节点 | boolean | false | blockNode | 是否节点占据一行 | boolean | false | checkable | 节点前添加 Checkbox 复选框 | boolean | false | checkedKeys | （受控）选中复选框的树节点（注意：父子节点有关联，如果传入父节点 key，则子节点自动选中；相应当子节点 key 都传入，父节点也自动选中。当设置 `checkable` 和 `checkStrictly`，它是一个有`checked`和`halfChecked`属性的对象，并且父子节点的选中与否不再关联 | string\[] \| {checked: string\[], halfChecked: string\[]} | \[] | checkStrictly | checkable 状态下节点选择完全受控（父子节点选中状态不再关联） | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultCheckedKeys | 默认选中复选框的树节点 | string\[] | \[] | defaultExpandAll | 默认展开所有树节点 | boolean | false | defaultExpandedKeys | 默认展开指定的树节点 | string\[] | \[] | defaultExpandParent | 默认展开父节点 | boolean | true | defaultSelectedKeys | 默认选中的树节点 | string\[] | \[] | disabled | 将树禁用 | boolean | false | draggable | 设置节点可拖拽，可以通过 `icon: false` 关闭拖拽提示图标 | boolean \| ((node: DataNode) => boolean) \| { icon?: React.ReactNode \| false, nodeDraggable?: (node: DataNode) => boolean } | false | `config`: 4.17.0 | × |
| expandedKeys | （受控）展开指定的树节点 | string\[] | \[] | fieldNames | 自定义节点 title、key、children 的字段 | object | { title: `title`, key: `key`, children: `children` } | 4.17.0 | × |
| filterTreeNode | 按需筛选树节点（高亮），返回 true | function(node) | - | height | 设置虚拟滚动容器高度，设置后内部节点不再支持横向滚动 | number | - | icon | 在标题之前插入自定义图标。需要设置 `showIcon` 为 true | ReactNode \| (props) => ReactNode | - | loadData | 异步加载数据 | function(node) | - | loadedKeys | （受控）已经加载的节点，需要配合 `loadData` 使用 | string\[] | \[] | motion | 自定义树的动画配置 | CSSMotionProps | - | multiple | 支持点选多个节点（节点本身） | boolean | false | ~~rootStyle~~ | Tree 最外层样式，请使用 `styles.root` 替代 | CSSProperties | - | 4.20.0 | × |
| selectable | 是否可选中 | boolean | true | selectedKeys | （受控）设置选中的树节点，多选需设置 `multiple` 为 true | string\[] | - | showIcon | 控制是否展示 `icon` 节点，没有默认样式 | boolean | false | showLine | 是否展示连接线 | boolean \| { showLeafIcon: ReactNode \| ((props: AntTreeNodeProps) => ReactNode) } | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | switcherIcon | 自定义树节点的展开/折叠图标（`showLine` 下不会自动 rotate） | ReactNode \| ((props: AntTreeNodeProps) => ReactNode) | - | renderProps: 4.20.0 | × |
| switcherLoadingIcon | 自定义树节点的加载图标 | ReactNode | - | 5.20.0 | × |
| titleRender | 自定义渲染节点 | (nodeData) => ReactNode | - | 4.5.0 | × |
| treeData | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（key 在整个树范围内唯一） | array&lt;{key, title, children, \[disabled, selectable]}> | - | virtual | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 | × |
| onCheck | 点击复选框触发 | function(checkedKeys, e:{checked: boolean, checkedNodes, node, event, halfCheckedKeys}) | - | onDoubleClick | 双击树节点触发 | function(event, node) | - | onDragEnd | dragend 触发时调用 | function({event, node}) | - | onDragEnter | dragenter 触发时调用 | function({event, node, expandedKeys}) | - | onDragLeave | dragleave 触发时调用 | function({event, node}) | - | onDragOver | dragover 触发时调用 | function({event, node}) | - | onDragStart | 开始拖拽时调用 | function({event, node}) | - | onDrop | drop 触发时调用 | function({event, node, dragNode, dragNodesKeys}) | - | onExpand | 展开/收起节点时触发 | function(expandedKeys, {expanded: boolean, node}) | - | onLoad | 节点加载完毕时触发 | function(loadedKeys, {event, node}) | - | onRightClick | 响应右键点击 | function({event, node}) | - | onSelect | 点击树节点触发 | function(selectedKeys, e:{selected: boolean, selectedNodes, node, event}) | - 
### TreeNode props

| 参数 | 说明 | 类型 | 默认值 | checkable | 当树为 checkable 时，设置独立节点是否展示 Checkbox | boolean | - | disabled | 禁掉响应 | boolean | false | isLeaf | 设置为叶子节点 (设置了 `loadData` 时有效)。为 `false` 时会强制将其作为父节点 | boolean | - | selectable | 设置节点是否可被选中 | boolean | true 
### DirectoryTree props

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| expandAction | 目录展开逻辑，可选：false \| `click` \| `doubleClick` | string \| boolean | `click` |

### 导入方式

```js
import { Tree } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowDrop` | 是否允许拖拽时放置在该节点 | ({ dropNode, dropPosition }) => boolean | - | — |
| `autoExpandParent` | 是否自动展开父节点 | boolean | false | — |
| `blockNode` | 是否节点占据一行 | boolean | false | — |
| `checkable` | 节点前添加 Checkbox 复选框 | boolean | false | — |
| `checkedKeys` | （受控）选中复选框的树节点（注意：父子节点有关联，如果传入父节点 key，则子节点自动选中；相应当子节点 key 都传入，父节点也自动选中。当设置 `checkable` 和 `checkStrictly`，它是一个有`checked`和`halfChecked`属性的对象，并且父子节点的选中与否不再关联 | string\[] \| {checked: string\[], halfChecked: string\[]} | \[] | — |
| `checkStrictly` | checkable 状态下节点选择完全受控（父子节点选中状态不再关联） | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultCheckedKeys` | 默认选中复选框的树节点 | string\[] | \[] | — |
| `defaultExpandAll` | 默认展开所有树节点 | boolean | false | — |
| `defaultExpandedKeys` | 默认展开指定的树节点 | string\[] | \[] | — |
| `defaultExpandParent` | 默认展开父节点 | boolean | true | — |
| `defaultSelectedKeys` | 默认选中的树节点 | string\[] | \[] | — |
| `disabled` | 将树禁用 | boolean | false | — |
| `draggable` | 设置节点可拖拽，可以通过 `icon: false` 关闭拖拽提示图标 | boolean \| ((node: DataNode) => boolean) \| { icon?: React.ReactNode \| false, nodeDraggable?: (node: DataNode) => boolean } | false | `config`: 4.17.0 |
| `expandedKeys` | （受控）展开指定的树节点 | string\[] | \[] | — |
| `fieldNames` | 自定义节点 title、key、children 的字段 | object | { title: `title`, key: `key`, children: `children` } | 4.17.0 |
| `filterTreeNode` | 按需筛选树节点（高亮），返回 true | function(node) | - | — |
| `height` | 设置虚拟滚动容器高度，设置后内部节点不再支持横向滚动 | number | - | — |
| `icon` | 在标题之前插入自定义图标。需要设置 `showIcon` 为 true | ReactNode \| (props) => ReactNode | - | — |
| `loadData` | 异步加载数据 | function(node) | - | — |
| `loadedKeys` | （受控）已经加载的节点，需要配合 `loadData` 使用 | string\[] | \[] | — |
| `motion` | 自定义树的动画配置 | CSSMotionProps | - | — |
| `multiple` | 支持点选多个节点（节点本身） | boolean | false | — |
| `rootStyle` | Tree 最外层样式，请使用 `styles.root` 替代 | CSSProperties | - | 4.20.0 |
| `selectable` | 是否可选中 | boolean | true | — |
| `selectedKeys` | （受控）设置选中的树节点，多选需设置 `multiple` 为 true | string\[] | - | — |
| `showIcon` | 控制是否展示 `icon` 节点，没有默认样式 | boolean | false | — |
| `showLine` | 是否展示连接线 | boolean \| { showLeafIcon: ReactNode \| ((props: AntTreeNodeProps) => ReactNode) } | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `switcherIcon` | 自定义树节点的展开/折叠图标（`showLine` 下不会自动 rotate） | ReactNode \| ((props: AntTreeNodeProps) => ReactNode) | - | renderProps: 4.20.0 |
| `switcherLoadingIcon` | 自定义树节点的加载图标 | ReactNode | - | 5.20.0 |
| `titleRender` | 自定义渲染节点 | (nodeData) => ReactNode | - | 4.5.0 |
| `treeData` | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（key 在整个树范围内唯一） | array<{key, title, children, \[disabled, selectable]}> | - | — |
| `virtual` | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 |
| `onCheck` | 点击复选框触发 | function(checkedKeys, e:{checked: boolean, checkedNodes, node, event, halfCheckedKeys}) | - | — |
| `onDoubleClick` | 双击树节点触发 | function(event, node) | - | — |
| `onDragEnd` | dragend 触发时调用 | function({event, node}) | - | — |
| `onDragEnter` | dragenter 触发时调用 | function({event, node, expandedKeys}) | - | — |
| `onDragLeave` | dragleave 触发时调用 | function({event, node}) | - | — |
| `onDragOver` | dragover 触发时调用 | function({event, node}) | - | — |
| `onDragStart` | 开始拖拽时调用 | function({event, node}) | - | — |
| `onDrop` | drop 触发时调用 | function({event, node, dragNode, dragNodesKeys}) | - | — |
| `onExpand` | 展开/收起节点时触发 | function(expandedKeys, {expanded: boolean, node}) | - | — |
| `onLoad` | 节点加载完毕时触发 | function(loadedKeys, {event, node}) | - | — |
| `onRightClick` | 响应右键点击 | function({event, node}) | - | — |
| `onSelect` | 点击树节点触发 | function(selectedKeys, e:{selected: boolean, selectedNodes, node, event}) | - | — |
| `disableCheckbox` | 禁掉 checkbox | boolean | false | — |
| `isLeaf` | 设置为叶子节点 (设置了 `loadData` 时有效)。为 `false` 时会强制将其作为父节点 | boolean | - | — |
| `key` | 被树的 (default)ExpandedKeys / (default)CheckedKeys / (default)SelectedKeys 属性所用。注意：整个树范围内的所有节点的 key 值不能重复！ | string | (内部计算出的节点位置) | — |
| `title` | 标题 | ReactNode | `---` | — |
| `expandAction` | 目录展开逻辑，可选：false \| `click` \| `doubleClick` | string \| boolean | `click` | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Tree** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **12** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tree
- 中文文档：https://ant.design/components/tree-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tree
- 驱动 gpui kit：`tree`
