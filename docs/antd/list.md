# List 列表
> 来源：[Ant Design 6.5.x List](https://ant.design/components/list)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：最基础的列表展示，可承载文字、列表、图片、段落。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 默认横排 |
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

- **依赖等级 L2 组合件**：数据容器 + 同库 `Pagination`（切页）、`Spin`（loading 遮罩）、`Empty`（空态 `emptyText`）、`Card`（grid 项容器示例）；`grid` 断点列数经 `ViewportWidth` 注入。
- **Item**：`List.Item`（actions/extra）+ `List.Item.Meta`（avatar/title/description）为内置子组件，同文件实现；`itemLayout` 决定 extra/actions 位置（LST-S9）。
- **废弃冻结**：官方 List 已废弃（继任 Listy 另起规格），kit P0 冻结当前主路径不追新。
- **文件归属**：`ui/kit/list/`（`list.go` + `item.go`/`meta.go`，复用 `ui/kit/pagination`、`ui/kit/spin`、`ui/kit/empty`）。
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
| `itemLayout` | 设置 `List.Item` 布局，设置成 `vertical` 则竖直样式显示，默认横排 | `horizontal` \| `vertical` | `horizontal` | — |
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **List** 的验收清单：

1. **配置面**：覆盖 §6.8 P0+P1 字段（dataSource/renderItem/rowKey/itemLayout/size/loading/split/bordered/header/footer/loadMore/pagination/grid/locale/meta）；semantic 深度 P1。
2. **视觉态**：bordered/split/itemLayout 双布局/loading 遮罩/empty/头尾/grid/分页 position-align（§6.4 LST-S1~S10，§6.5）。
3. **尺寸态**：small / default / large（项 8·16/12·0/16·24，§6.2）。
4. **受控/非受控**：分页受控（page/pageSize + onChange）与非受控默认页；dataSource 为父级数据。
5. **数据驱动**：dataSource + renderItem + rowKey（缺省 `key` 或 `list-item-{index}`）。
6. **无障碍**：List/List.Item 分表（§6.6）；空态朗读名、加载忙态、项内控件各自键盘。
7. **RTL**：extra/actions/分页 align start/end 镜像；grid 流向镜像。
8. **浮层**：无自带浮层；分页弹层随同库 Pagination。
9. **性能**：分页切片 + 按需重建；虚拟列表 P1（复用 `ui/rendering` VirtualList，本地直喂）。
10. **主题**：Token 化（§6.2 行线 Split/头尾 transparent）；支持 reduced-motion（瞬时）。
11. **示例矩阵**：§6.8 P0 **8** 例（simple/basic/loadmore/vertical/pagination/grid/responsive/infinite-load）；`virtual-list/拖拽排序4例` 归 P1。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/list
- 中文文档：https://ant.design/components/list-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/list
- 驱动 gpui kit：`list`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **List** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/list/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（List）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（List）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：最基础的列表展示，可承载文字、列表、图片、段落。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

数值对齐 antd `components/list/style`（`prepareComponentToken`）：

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 项内边距 default | **12 0** | `itemPadding` ← `paddingContentVertical 0` |
| 项内边距 small | **8 16** | `itemPaddingSM` ← `paddingContentVerticalSM paddingContentHorizontal` |
| 项内边距 large | **16 24** | `itemPaddingLG` ← `paddingContentVerticalLG paddingContentHorizontalLG` |
| 头/尾/项横向内边距 | **24** | `paddingLG` |
| 空文本内边距 | **16** | `emptyTextPadding` ← `padding` |
| 头/尾底色 | transparent | `headerBg` / `footerBg` |
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
| `bordered` | 是否展示边框 | boolean | false |
| `dataSource` | 列表数据源 | any\[] | - |
| `footer` | 列表底部 | ReactNode | - |
| `grid` | 列表栅格配置 | [object](#list-grid-props) | - |
| `header` | 列表头部 | ReactNode | - |
| `itemLayout` | 设置 `List.Item` 布局，设置成 `vertical` 则竖直样式显示，默认横排 | `horizontal` \| `vertical` | `horizontal` |
| `loading` | 当卡片内容还在加载中时，可以用 `loading` 展示一个占位 | boolean \ | [object](/components/spin-cn#api) ([更多](https://github.com/ant-design/ant-design/issues/8659)) |
| `loadMore` | 加载更多 | ReactNode | - |
| `locale` | 默认文案设置，目前包括空数据文案 | object | {emptyText: `暂无数据`} |
| `pagination` | 对应的 `pagination` 配置，设置 false 不显示 | boolean \ | object |
| `renderItem` | 当使用 dataSource 时，可以用 `renderItem` 自定义渲染列表项 | (item: T, index: number) => ReactNode | - |
| `rowKey` | 当 `renderItem` 自定义渲染列表项有效时，自定义每一行的 `key` 的获取方式 | `keyof` T \ | (item: T) => `React.Key` |
| `size` | list 的尺寸 | `default` \ | `large` \ |
| `split` | 是否展示分割线 | boolean | true |
| `position` | 指定分页显示的位置 | `top` \ | `bottom` \ |
| `align` | 指定分页对齐的位置 | `start` \ | `center` \ |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
dataSource.map(renderItem) ──► rowKey 去重
pagination ──► 切页（pageSize 切分 dataSource）
loading ──► Spin 遮罩（isLoading 时 body 占位 minHeight 53）
[] ──► Empty（emptyText）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| LST-S1 | 2 项 dataSource | 2 行 |
| LST-S2 | pagination 翻页 | 按 pageSize 切分；`onChange(page, pageSize)`；position `bottom`/`top`/`both`，align 默认 `end` |
| LST-S3 | loading=true | Spin 遮罩；加载中不重复触发翻页/加载更多 |
| LST-S4 | 空数组 | Empty，内边距 16（`emptyTextPadding`） |
| LST-S5 | split=false | 无分割线 |
| LST-S6 | bordered | 1px 边框 + 圆角 |
| LST-S7 | grid | 按 column/断点列数排栅格，gutter 间隔 |
| LST-S8 | header/footer/loadMore | 可见；loadMore 在列表尾 |
| LST-S9 | itemLayout=vertical | extra 放右侧，actions 放底部；horizontal 则 extra 在最右、actions 同行 |
| LST-S10 | size small/default/large | 项内边距 8 16 / 12 0 / 16 24（§6.2） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| bordered | 外框 1px `colorBorder`＋圆角 6；头/尾/项横向 pad 24；分割线仍受 `split` 控制 |
| split | 默认 true 画 `colorSplit` 行线；split=false 全列表无行线；grid 栅格不画行线 |
| itemLayout | horizontal：extra 最右、actions 同行尾；vertical：extra 右侧、actions 底部 |
| loading | Spin 遮罩 body（minHeight 53）；加载中不重复触发翻页/loadMore |
| empty | 空数组走 Empty 文案（默认「暂无数据」），内边距 16 |
| header/footer/loadMore | 头/尾通栏；loadMore 居尾部按钮区；grid 下仍通栏占整行 |
| grid | 按 column/断点列数排，gutter 间距；bordered 网格卡片各自圆角 |
| pagination | position bottom/top/both，align 默认 end；换页只切 dataSource 片 |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

**List（容器）：**

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `role=list`；grid 栅格仍为 list，不用表格角色 |
| 命名 | `AriaLabel` 可设；空态名=`locale.emptyText`（默认「暂无数据」） |
| 键盘 | 本体无统一键盘操作；分页/loadMore 按各自控件处理 |
| 加载 | loading 朗读“加载中”且不重复触发；翻页后焦点建议落新页首项或分页控件 |

**List.Item（项）：**

| 项 | 要求 |
| --- | --- |
| 角色 | 每项 `role=listitem`；grid 项同 |
| 命名 | 名=title＋description（无 title 用首行文本）；extra/actions 各自命名 |
| 键盘 | 纯展示项不抢焦点；项内按钮/链接按各自语义 Enter/Space 激活 |
| 焦点环 | 分页、loadMore、项内可点控件聚焦时 ring 可见（outset≈1.5px） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1 / §6.4 LST-S1~S10） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2 项12·0/8·16/16·24） | **对等** | P0 L2 |
| `pagination` 切页（position/align） | **组合**：复用同库 `Pagination`；缺失时 P0 自绘简易页码 | P0 L1 |
| `grid` 断点列数（xs…xxxl+gutter） | **对等**（`ViewportWidth` 注入） | P0 L1 |
| 虚拟列表 `virtual-list` | **P1**：本地数据源直喂，复用 `ui/rendering` VirtualList | P1 |
| 拖拽排序 4 示例（dnd-kit 手势） | **P1**：缺手势重排语义，先补手势再跟进 | P1 |
| Semantic classNames/styles（actions/extra） | kit 浅钩子；深度 P1 | P0 浅 / P1 深 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

> 废弃策略：官方 List 已废弃（下个 major 移除，继任者 Listy 另起规格）；kit 侧 P0+P1 全跟 List 主路径，不冻结范围，不追 Listy。

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `dataSource` / `renderItem` / `rowKey` | 数据驱动；`rowKey` 取键，缺省用 `key` 字段或 `list-item-{index}` |
| `itemLayout` | `horizontal`（默认）/ `vertical`（LST-S9） |
| `size` | small / default / large → 项内边距见 §6.2（LST-S10） |
| `loading` | Spin 遮罩（LST-S3） |
| `split` / `bordered` | 分割线（默认 true）/ 边框 |
| `header` / `footer` / `loadMore` | 头尾与加载更多（LST-S8） |
| `pagination` | 切页 + position/align（LST-S2） |
| `grid` | column / gutter / xs…xxxl 断点列数（LST-S7） |
| `locale.emptyText` | 空数据文案（LST-S4） |
| `title` / `description` / `avatar` / `extra` / `actions` | Item.Meta 主字段（extra/actions 位置随 itemLayout，LST-S9） |
| 官方主路径示例 | 简单列表、基础列表、加载更多、竖排列表样式、分页设置、栅格列表、响应式的栅格列表、滚动加载 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | P1 必做 |
| 动画像素级 | P1 必做（P0 瞬时切换） |
| 虚拟列表（`virtual-list.tsx`） | P1：换本地数据源（`VirtualList data` 直喂，不经 List `dataSource`），复用现有 `ui/rendering` VirtualList |
| 拖拽排序 4 示例 | P1：缺手势重排语义（官方基于 dnd-kit 拖拽手势实现排序；kit 侧须先补手势重排语义） |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| debug 示例 | 不验收（仅参考） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestList_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 List 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| LST-01 | L1 | NewList 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| LST-02 | L1 | 2 项 dataSource | 2 行 |
| LST-03 | L1 | pagination | 翻页回调 |
| LST-04 | L1 | loading | 遮罩 |
| LST-05 | L1 | 空数组 | Empty |
| LST-06 | L1 | split=false | 无分割线 |
| LST-07 | L1 | bordered | 边框 |
| LST-08 | L1 | grid | 栅格项 |
| LST-09 | L1 | header/footer | 可见 |
| LST-10 | L1 | 复现官方示例「简单列表」（`simple.tsx`） | bordered + header/footer + 三档 size（default/small/large）可布局 |
| LST-11 | L1 | 复现官方示例「基础列表」（`basic.tsx`） | horizontal + `Item.Meta`（avatar/title/description）4 项可布局 |
| LST-12 | L1 | 复现官方示例「加载更多」（`loadmore.tsx`） | `loadMore` 按钮 + 初始化 Spin 遮罩可布局；点击加载追加 |
| LST-13 | L1 | 复现官方示例「竖排列表样式」（`vertical.tsx`） | `itemLayout=vertical` + extra 右侧 + actions 底部 + 分页 pageSize=3 |
| LST-14 | L1 | 复现官方示例「分页设置」（`pagination.tsx`） | position top/bottom/both × align start/center/end 切页可测 |
| LST-15 | L1 | 复现官方示例「栅格列表」（`grid.tsx`） | `grid={gutter:16,column:4}` + 项内 Card 可布局 |
| LST-16 | L1 | 复现官方示例「响应式的栅格列表」（`responsive.tsx`） | `grid={xs:1,sm:2,…}` 断点列数随视口切换 |
| LST-17 | L1 | 复现官方示例「滚动加载」（`infinite-load.tsx`） | 滚动触底追加 + Spin 遮罩；加载中不重复触发 |
| LST-18 | L2 | 读取 §6.2 关键尺寸/间距 | 项12·0/8·16/16·24、头尾横24、空文16（±0.5px） |
| LST-19 | L2 | 默认皮颜色 | 行线 `colorSplit`、头尾 transparent、空文次级色；无硬编码品牌色 |
| LST-20 | L2 | disabled 外观 | **不适用**（List 无 disabled API；加载中用 Spin 遮罩）— 跳过 |
| LST-21 | L1 | 键盘/焦点 | 本体无统一键盘；分页/loadMore/项内可点控件按各自语义可聚焦（§6.6） |
| LST-22 | L1 | itemLayout=vertical 带 extra/actions | extra 右侧、actions 底部；horizontal 对照组位置正确 |
| LST-23 | L2 | size small/large 项内边距 | 8 16 / 16 24（±0.5px） |
| LST-24 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| LST-25 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| LST-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
// 类型
ListItemLayout = horizontal | vertical  // 默认 horizontal
ListSize       = small | default | large // 默认 default

// 列表项（P0 字段，对齐 List.Item / Item.Meta）
type ListItem struct {
  Key         string
  Title       string
  Description string
  Avatar      core.Node  // 图标节点（可选）
  Extra       core.Node  // 额外内容（vertical 右侧 / horizontal 最右）
  Actions     []core.Node
}

NewList(items ...ListItem) *List

// 数据与布局
SetDataSource([]ListItem)
SetRenderItem(func(item ListItem, index int) core.Node)
SetRowKey(func(item ListItem) string)
SetItemLayout(ListItemLayout)
SetSize(ListSize)
SetBordered(bool)               // 默认 false
SetSplit(bool)                  // 默认 true
SetLoading(bool)
SetHeader(core.Node) / SetFooter(core.Node) / SetLoadMore(core.Node)
SetEmptyText(string)
// 分页：false=不显示；position bottom|top|both（默认 bottom），align start|center|end（默认 end）
SetPagination(*ListPagination)
SetOnPageChange(func(page, pageSize int))
// 栅格：column / gutter / xs…xxxl 断点列数
SetGrid(ListGridConfig)
// 主题 / a11y / 挂树
SetTheme(*Theme)
SetAriaLabel(string)
Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Size | default |
| ItemLayout | horizontal |
| Split | true |
| Bordered / Loading | false |
| Pagination | false（不显示） |
| EmptyText | `暂无数据`（随 locale） |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Data view
  ├─ header?
  ├─ body rows/nodes
  └─ pagination/footer?
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **List 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/list.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` List 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
