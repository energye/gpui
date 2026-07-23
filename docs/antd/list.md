# List 列表
> 来源：[Ant Design 6.5.x List](https://ant.design/components/list)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：最基础的列表展示，可承载文字、列表、图片、段落。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

最基础的列表展示，可承载文字、列表、图片、段落。

**List** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 简单列表 | 复现「简单列表」视觉与布局 |
| 基础列表 | 复现「基础列表」视觉与布局 |
| 加载更多 | loading 指示与防重复 |
| 竖排列表样式 | 复现「竖排列表样式」视觉与布局 |
| 分页设置 | 分页器外观 |
| 栅格列表 | 复现「栅格列表」视觉与布局 |
| 响应式的栅格列表 | 断点响应式 |
| 滚动加载 | loading 指示与防重复 |
| 拖拽排序 | 复现「拖拽排序」视觉与布局 |
| 拖拽排序（拖拽手柄） | 复现「拖拽排序（拖拽手柄）」视觉与布局 |
| 栅格拖拽排序 | 复现「栅格拖拽排序」视觉与布局 |
| 栅格拖拽排序（拖拽手柄） | 复现「栅格拖拽排序（拖拽手柄）」视觉与布局 |
| 滚动加载无限长列表 | loading 指示与防重复 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bordered`

- **说明**：是否展示边框
- **类型**：boolean
- **默认值**：false

#### `footer`

- **说明**：列表底部
- **类型**：ReactNode
- **默认值**：-

#### `header`

- **说明**：列表头部
- **类型**：ReactNode
- **默认值**：-

#### `itemLayout`

- **说明**：设置 `List.Item` 布局，设置成 `vertical` 则竖直样式显示，默认横排
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `List.Item` | 官方取值 `List.Item` |
  | `vertical` | 垂直排布 |

#### `loading`

- **说明**：当卡片内容还在加载中时，可以用 `loading` 展示一个占位
- **类型**：boolean | [object](/components/spin-cn#api) ([更多](https://github.com/ant-design/ant-design/issues/8659))
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `loading` | 官方取值 `loading` |

#### `loadMore`

- **说明**：加载更多
- **类型**：ReactNode
- **默认值**：-

#### `size`

- **说明**：list 的尺寸
- **类型**：`default` | `large` | `small`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `small` | 小尺寸（更紧凑） |

#### `position`

- **说明**：指定分页显示的位置
- **类型**：`top` | `bottom` | `both`
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `both` | 官方取值 `both` |

#### `align`

- **说明**：指定分页对齐的位置
- **类型**：`start` | `center` | `end`
- **默认值**：`end`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `center` | 居中 |
  | `end` | 逻辑结束侧 |

#### `gutter`

- **说明**：栅格间隔
- **类型**：number
- **默认值**：0

#### `actions`

- **说明**：列表操作组，根据 `itemLayout` 的不同，位置在卡片底部或者最右侧
- **类型**：Array<ReactNode>
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `itemLayout` | 官方取值 `itemLayout` |

#### `classNames`

- **说明**：语义化结构 className
- **类型**：[`Record`](#semantic-dom)
- **默认值**：-
- **版本**：5.18.0

#### `extra`

- **说明**：额外内容，通常用在 `itemLayout` 为 `vertical` 的情况下，展示右侧内容; `horizontal` 展示在列表元素最右侧
- **类型**：ReactNode
- **默认值**：-

#### `styles`

- **说明**：语义化结构 style
- **类型**：[`Record`](#semantic-dom)
- **默认值**：-
- **版本**：5.18.0

#### `avatar`

- **说明**：列表元素的图标
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：列表元素的描述内容
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：列表元素的标题
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

最基础的列表展示，可承载文字、列表、图片、段落，常用于后台数据展示页面。

:::warning{title=废弃提示}
List 组件已经进入废弃阶段，将于下个 major 版本移除。
:::

### 2.2 核心功能（按官方示例拆解）

1. **简单列表**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **基础列表**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **加载更多**（`loadmore.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **竖排列表样式**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **分页设置**（`pagination.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **栅格列表**（`grid.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **响应式的栅格列表**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **滚动加载**（`infinite-load.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **拖拽排序**（`drag-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **拖拽排序（拖拽手柄）**（`drag-sorting-handler.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **栅格拖拽排序**（`grid-drag-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **栅格拖拽排序（拖拽手柄）**（`grid-drag-sorting-handler.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **滚动加载无限长列表**（`virtual-list.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `loading` | 加载中 | 当卡片内容还在加载中时，可以用 `loading` 展示一个占位 |
| `dataSource` | 数据源 | 列表数据源 |
| `pagination` | 分页 | 对应的 `pagination` 配置，设置 false 不显示 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 简单列表 | `simple.tsx` | 否 |
| 基础列表 | `basic.tsx` | 否 |
| 加载更多 | `loadmore.tsx` | 否 |
| 竖排列表样式 | `vertical.tsx` | 否 |
| 分页设置 | `pagination.tsx` | 否 |
| 栅格列表 | `grid.tsx` | 否 |
| 测试栅格列表 | `grid-test.tsx` | 是 |
| 响应式的栅格列表 | `responsive.tsx` | 否 |
| 滚动加载 | `infinite-load.tsx` | 否 |
| 拖拽排序 | `drag-sorting.tsx` | 否 |
| 拖拽排序（拖拽手柄） | `drag-sorting-handler.tsx` | 否 |
| 栅格拖拽排序 | `grid-drag-sorting.tsx` | 否 |
| 栅格拖拽排序（拖拽手柄） | `grid-drag-sorting-handler.tsx` | 否 |
| 滚动加载无限长列表 | `virtual-list.tsx` | 否 |
| 自定义组件 token | `component-token.tsx` | 是 |
| Spin 加载状态调试 | `spin-debug.tsx` | 是 |

### 2.6 FAQ

## FAQ {#faq}

### List 组件废弃后，有替代方案吗？ {#faq-listy-replacement}

在 Ant Design v6 中，我们将推出一个全新的 Listy 组件作为 List 的继任者。

Listy 内置虚拟滚动能力，并更加强调灵活的布局控制，旨在帮助开发者根据不同业务场景更高效地实现自定义列表。

目前，底层实现 rc-listy 已基本开发完成，正在等待核心维护者的评审与后续调整。

Ant Design v6 将基于 rc-listy 正式提供 Listy 组件。

相关链接：

- Pull Request: [PR #54182](https://github.com/ant-design/ant-design/pull/54182)
- RFC 讨论: [Discussion #54458](https://github.com/ant-design/ant-design/discussions/54458)

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

另外我们封装了 [ProList](https://procomponents.ant.design/components/list)，在 `antd` List 之上扩展了更多便捷易用的功能，比如多选，展开等功能，使用体验贴近 Table，欢迎尝试使用。

### List

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| bordered | 是否展示边框 | boolean | false | dataSource | 列表数据源 | any\[] | - | footer | 列表底部 | ReactNode | - | grid | 列表栅格配置 | [object](#list-grid-props) | - | header | 列表头部 | ReactNode | - | itemLayout | 设置 `List.Item` 布局，设置成 `vertical` 则竖直样式显示，默认横排 | string | - | loading | 当卡片内容还在加载中时，可以用 `loading` 展示一个占位 | boolean \| [object](/components/spin-cn#api) ([更多](https://github.com/ant-design/ant-design/issues/8659)) | false | loadMore | 加载更多 | ReactNode | - | locale | 默认文案设置，目前包括空数据文案 | object | {emptyText: `暂无数据`} | pagination | 对应的 `pagination` 配置，设置 false 不显示 | boolean \| object | false | renderItem | 当使用 dataSource 时，可以用 `renderItem` 自定义渲染列表项 | (item: T, index: number) => ReactNode | - | rowKey | 当 `renderItem` 自定义渲染列表项有效时，自定义每一行的 `key` 的获取方式 | `keyof` T \| (item: T) => `React.Key` | `"key"` | size | list 的尺寸 | `default` \| `large` \| `small` | `default` | split | 是否展示分割线 | boolean | true 
### pagination

分页的配置项。

| 参数     | 说明               | 类型                         | 默认值   |
| -------- | ------------------ | ---------------------------- | -------- |
| position | 指定分页显示的位置 | `top` \| `bottom` \| `both`  | `bottom` |
| align    | 指定分页对齐的位置 | `start` \| `center` \| `end` | `end`    |

更多配置项，请查看 [`Pagination`](/components/pagination-cn)。

### List grid props

| 参数   | 说明                 | 类型   | 默认值 | 版本  |
| ------ | -------------------- | ------ | ------ | ----- |
| column | 列数                 | number | -      |       |
| gutter | 栅格间隔             | number | 0      |       |
| xs     | `<576px` 展示的列数  | number | -      |       |
| sm     | `≥576px` 展示的列数  | number | -      |       |
| md     | `≥768px` 展示的列数  | number | -      |       |
| lg     | `≥992px` 展示的列数  | number | -      |       |
| xl     | `≥1200px` 展示的列数 | number | -      |       |
| xxl    | `≥1600px` 展示的列数 | number | -      |       |
| xxxl   | `≥1920px` 展示的列数 | number | -      | 6.3.0 |

### List.Item

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 列表操作组，根据 `itemLayout` 的不同，位置在卡片底部或者最右侧 | Array&lt;ReactNode> | - | classNames | 语义化结构 className | [`Record<actions \| extra, string>`](#semantic-dom) | - | 5.18.0 | 5.18.0 |
| extra | 额外内容，通常用在 `itemLayout` 为 `vertical` 的情况下，展示右侧内容; `horizontal` 展示在列表元素最右侧 | ReactNode | - | styles | 语义化结构 style | [`Record<actions \| extra, CSSProperties>`](#semantic-dom) | - | 5.18.0 | 5.18.0 |

### List.Item.Meta

| 参数        | 说明               | 类型      | 默认值 | 版本 |
| ----------- | ------------------ | --------- | ------ | ---- |
| avatar      | 列表元素的图标     | ReactNode | -      |      |
| description | 列表元素的描述内容 | ReactNode | -      |      |
| title       | 列表元素的标题     | ReactNode | -      |      |

### 导入方式

```js
import { List } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `bordered` | 是否展示边框 | boolean | false | — |
| `dataSource` | 列表数据源 | any\[] | - | — |
| `footer` | 列表底部 | ReactNode | - | — |
| `grid` | 列表栅格配置 | [object](#list-grid-props) | - | — |
| `header` | 列表头部 | ReactNode | - | — |
| `itemLayout` | 设置 `List.Item` 布局，设置成 `vertical` 则竖直样式显示，默认横排 | string | - | — |
| `loading` | 当卡片内容还在加载中时，可以用 `loading` 展示一个占位 | boolean \| [object](/components/spin-cn#api) ([更多](https://github.com/ant-design/ant-design/issues/8659)) | false | — |
| `loadMore` | 加载更多 | ReactNode | - | — |
| `locale` | 默认文案设置，目前包括空数据文案 | object | {emptyText: `暂无数据`} | — |
| `pagination` | 对应的 `pagination` 配置，设置 false 不显示 | boolean \| object | false | — |
| `renderItem` | 当使用 dataSource 时，可以用 `renderItem` 自定义渲染列表项 | (item: T, index: number) => ReactNode | - | — |
| `rowKey` | 当 `renderItem` 自定义渲染列表项有效时，自定义每一行的 `key` 的获取方式 | `keyof` T \| (item: T) => `React.Key` | `"key"` | — |
| `size` | list 的尺寸 | `default` \| `large` \| `small` | `default` | — |
| `split` | 是否展示分割线 | boolean | true | — |
| `position` | 指定分页显示的位置 | `top` \| `bottom` \| `both` | `bottom` | — |
| `align` | 指定分页对齐的位置 | `start` \| `center` \| `end` | `end` | — |
| `column` | 列数 | number | - | — |
| `gutter` | 栅格间隔 | number | 0 | — |
| `xs` | `<576px` 展示的列数 | number | - | — |
| `sm` | `≥576px` 展示的列数 | number | - | — |
| `md` | `≥768px` 展示的列数 | number | - | — |
| `lg` | `≥992px` 展示的列数 | number | - | — |
| `xl` | `≥1200px` 展示的列数 | number | - | — |
| `xxl` | `≥1600px` 展示的列数 | number | - | — |
| `xxxl` | `≥1920px` 展示的列数 | number | - | 6.3.0 |
| `actions` | 列表操作组，根据 `itemLayout` 的不同，位置在卡片底部或者最右侧 | Array<ReactNode> | - | — |
| `classNames` | 语义化结构 className | [`Record`](#semantic-dom) | - | 5.18.0 |
| `extra` | 额外内容，通常用在 `itemLayout` 为 `vertical` 的情况下，展示右侧内容; `horizontal` 展示在列表元素最右侧 | ReactNode | - | — |
| `styles` | 语义化结构 style | [`Record`](#semantic-dom) | - | 5.18.0 |
| `avatar` | 列表元素的图标 | ReactNode | - | — |
| `description` | 列表元素的描述内容 | ReactNode | - | — |
| `title` | 列表元素的标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **List** 的验收清单：

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
- 官方文档：https://ant.design/components/list
- 中文文档：https://ant.design/components/list-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/list
- 驱动 gpui kit：`list`
