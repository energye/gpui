# Pagination 分页
> 来源：[Ant Design 6.5.x Pagination](https://ant.design/components/pagination)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：分页器用于分隔长列表，每次只加载一个页面。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

分页器用于分隔长列表，每次只加载一个页面。

**Pagination** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 方向 | 复现「方向」视觉与布局 |
| 更多 | 复现「更多」视觉与布局 |
| 改变 | 复现「改变」视觉与布局 |
| 跳转 | 复现「跳转」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 简洁 | 复现「简洁」视觉与布局 |
| 受控 | 复现「受控」视觉与布局 |
| 总数 | 复现「总数」视觉与布局 |
| 全部展示 | 复现「全部展示」视觉与布局 |
| 上一步和下一步 | 复现「上一步和下一步」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：对齐方式
- **类型**：start | center | end
- **默认值**：-
- **版本**：5.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `center` | 居中 |
  | `end` | 逻辑结束侧 |

#### `classNames`

- **说明**：自定义组件内部各语义化结构的类名。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：禁用分页
- **类型**：boolean
- **默认值**：-

#### `responsive`

- **说明**：当 size 未指定时，根据屏幕宽度自动调整尺寸
- **类型**：boolean
- **默认值**：-

#### `simple`

- **说明**：当添加该属性时，显示为简单分页
- **类型**：boolean | { readOnly?: boolean }
- **默认值**：-

#### `size`

- **说明**：组件尺寸
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：自定义组件内部各语义化结构的内联样式。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `total`

- **说明**：数据总数（条目数，非总页数）；决定总页数 `ceil(total/pageSize)` 与页码项数量
- **类型**：number
- **默认值**：0
- **外观影响**：`total=0` 只展示第 1 页；总数变化时页码列表长度与省略号（`jump-prev/jump-next`）跟着变

#### `pageSize` / `pageSizeOptions`

- **说明**：每页条数 / 可选的每页条数档位；`showSizeChanger` 打开时用下拉切换
- **类型**：number / number[]
- **默认值**：10 / `[10, 20, 50, 100]`
- **外观影响**：`pageSize` 越小页码项越多；切换 `pageSize` 后当前页夹紧到 `1..pages`，`onChange`/`onShowSizeChange` 回调

#### `showTotal`

- **说明**：总数文案渲染函数 `showTotal(total, [start, end]) => string`，展示在分页行起始侧
- **类型**：function(total, range)
- **默认值**：-
- **外观影响**：有则多一段次级文本（如“共 50 条”）；无则不占位，页码行左对齐起点变化

#### `showQuickJumper`

- **说明**：快速跳转输入框；`Enter` 跳页
- **类型**：boolean | { goButton: ReactNode }
- **默认值**：false
- **外观影响**：打开时行尾多一个输入框（+可选 Go 按钮），行宽增加

#### `showSizeChanger`

- **说明**：每页条数切换器；未显式设置时 `total > totalBoundaryShowSizeChanger(50)` 默认为 true
- **类型**：boolean
- **默认值**：auto（见 boundary）
- **外观影响**：打开时行尾多一个 Select，行宽增加；`total≤50` 默认不展示

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

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。Pagination 语义节点：`root`（根行容器，flex 布局/对齐/换行）、`item`（页码项，尺寸/边框/背景/悬停/激活态）。函数形态与 `_semantic.tsx` 深度定制为 P1。
- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 当加载/渲染所有数据将花费很多时间时；
- 可切换页码浏览数据。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **方向**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **更多**（`more.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **改变**（`changer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **跳转**（`jump.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **尺寸**（`mini.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **简洁**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **受控**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **总数**（`total.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **全部展示**（`all.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **上一步和下一步**（`itemRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 |
| `disabled` | 禁用 | 禁用分页 |
| `current` | 当前步骤/页 | 当前页数 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 方向 | `align.tsx` | 否 |
| 更多 | `more.tsx` | 否 |
| 改变 | `changer.tsx` | 否 |
| 跳转 | `jump.tsx` | 否 |
| 尺寸 | `mini.tsx` | 否 |
| 简洁 | `simple.tsx` | 否 |
| 受控 | `controlled.tsx` | 否 |
| 总数 | `total.tsx` | 否 |
| 全部展示 | `all.tsx` | 否 |
| 上一步和下一步 | `itemRender.tsx` | 否 |
| 线框风格 | `wireframe.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 变体 Debug | `variant-debug.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.7 组合关系

- **依赖等级**：L2（导航：组合 Select 下拉 + Input 跳页，无浮层定位）。
- **等谁**：等 `kit.Select`（`showSizeChanger` 切换器底座）与输入框（`showQuickJumper` 跳页）就绪；纯页码模式可先行。
- **文件归属**：`ui/kit/pagination/`。
- **组合**：`showSizeChanger` 复用 Select 语义（`pageSizeOptions` 档位），`showQuickJumper` 复用输入框 Enter 跳页；`showTotal` 文案由上层注入。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

```jsx
<Pagination onChange={onChange} total={50} />
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 对齐方式 | start \| center \| end | - | 5.19.0 | × |
| classNames | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | current | 当前页数 | number | - | defaultCurrent | 默认的当前页数 | number | 1 | defaultPageSize | 默认的每页条数 | number | 10 | disabled | 禁用分页 | boolean | - | hideOnSinglePage | 只有一页时是否隐藏分页器 | boolean | false | itemRender | 用于自定义页码的结构，可用于优化 SEO | (page, type: 'page' \| 'prev' \| 'next', originalElement) => React.ReactNode | - | pageSize | 每页条数 | number | - | pageSizeOptions | 指定每页可以显示多少条 | number\[] | \[`10`, `20`, `50`, `100`] | responsive | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | - | showLessItems | 是否显示较少页面内容 | boolean | false | showQuickJumper | 是否可以快速跳转至某页 | boolean \| { goButton: ReactNode } | false | showSizeChanger | 是否展示 `pageSize` 切换器 | boolean \| [SelectProps](/components/select-cn#api) | - | SelectProps: 5.21.0 | 4.21.0，SelectProps: 5.21.0 |
| showTitle | 是否显示原生 tooltip 页码提示 | boolean | true | showTotal | 用于显示数据总量和当前数据顺序 | function(total, range) | - | simple | 当添加该属性时，显示为简单分页 | boolean \| { readOnly?: boolean } | - | size | 组件尺寸 | `large` \| `medium` \| `small` | `medium` | styles | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | total | 数据总数 | number | 0 | totalBoundaryShowSizeChanger | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 | onChange | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 | function(page, pageSize) | - | onShowSizeChange | pageSize 变化的回调 | function(current, size) | - 
### 导入方式

```js
import { Pagination } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 对齐方式 | start \| center \| end | - | 5.19.0 |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `current` | 当前页数 | number | - | — |
| `defaultCurrent` | 默认的当前页数 | number | 1 | — |
| `defaultPageSize` | 默认的每页条数 | number | 10 | — |
| `disabled` | 禁用分页 | boolean | - | — |
| `hideOnSinglePage` | 只有一页时是否隐藏分页器 | boolean | false | — |
| `itemRender` | 用于自定义页码的结构，可用于优化 SEO | (page, type: 'page' \| 'prev' \| 'next', originalElement) => React.ReactNode | - | — |
| `pageSize` | 每页条数 | number | - | — |
| `pageSizeOptions` | 指定每页可以显示多少条 | number\[] | \[`10`, `20`, `50`, `100`] | — |
| `responsive` | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | - | — |
| `showLessItems` | 是否显示较少页面内容 | boolean | false | — |
| `showQuickJumper` | 是否可以快速跳转至某页 | boolean \| { goButton: ReactNode } | false | — |
| `showSizeChanger` | 是否展示 `pageSize` 切换器 | boolean \| [SelectProps](/components/select-cn#api) | - | SelectProps: 5.21.0 |
| `showTitle` | 是否显示原生 tooltip 页码提示 | boolean | true | — |
| `showTotal` | 用于显示数据总量和当前数据顺序 | function(total, range) | - | — |
| `simple` | 当添加该属性时，显示为简单分页 | boolean \| { readOnly?: boolean } | - | — |
| `size` | 组件尺寸 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `total` | 数据总数 | number | 0 | — |
| `totalBoundaryShowSizeChanger` | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 | 6.2.0 |
| `onChange` | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 | function(page, pageSize) | - | — |
| `onShowSizeChange` | pageSize 变化的回调 | function(current, size) | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Pagination** 的验收清单：

1. **配置面**：覆盖 API 表全部字段；冷门字段必须实现且命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：P0 按 §6.8 逐例对照表（10 例主路径），余下 P1 一次做完（`itemRender` 自定义页码、`style-class`；3 个 debug 仅参考不验收）。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/pagination
- 中文文档：https://ant.design/components/pagination-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/pagination
- 驱动 gpui kit：`pagination`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Pagination** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/pagination/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Pagination）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点页码/prev-next 夹紧/size 切换/jumper 跳页/simple 简化与禁用 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Pagination）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：分页器用于分隔长列表，每次只加载一个页面。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 分页项高 middle（`itemSize`） | **32** | 组件 token `itemSize` = `controlHeight`（±0.5px） |
| 分页项高 small（`itemSizeSM`） | **24** | 组件 token `itemSizeSM` = `controlHeightSM`（±0.5px） |
| 分页项高 large（`itemSizeLG`） | **40** | 组件 token `itemSizeLG` = `controlHeightLG`（±0.5px） |
| 字号 middle | **14** | `fontSize` |
| 字号 small | **12** | `fontSizeSM` |
| 字号 large | **16** | `fontSizeLG` |
| 圆角 | **6** | `borderRadius`（small→SM=4，large→LG=8，±0.5px） |
| 边框线宽 | **1** | `lineWidth` |
| 项间距 | **8** | ≈ `marginXS` 节奏（±0.5px） |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 激活项底（`itemActiveBg`） | `colorBgContainer` | 当前页底色 |
| 页码链接底（`itemLinkBg`） | `colorBgContainer` | 普通项底色（`bordered.ts`） |
| 激活禁用底（`itemActiveBgDisabled`） | `controlItemBgActiveDisabled` | 禁用激活项回落 |
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
| `align` | 对齐方式 | `start` \| `center` \| `end` | — |
| `current` | 当前页数（受控） | number | — |
| `defaultCurrent` | 默认的当前页数（非受控） | number | 1 |
| `defaultPageSize` | 默认的每页条数（非受控） | number | 10 |
| `disabled` | 禁用分页 | boolean | false |
| `hideOnSinglePage` | 只有一页时是否隐藏分页器 | boolean | false |
| `pageSize` | 每页条数（受控） | number | — |
| `pageSizeOptions` | 指定每页可以显示多少条 | number[] | `[10, 20, 50, 100]` |
| `responsive` | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | — |
| `showLessItems` | 是否显示较少页面内容 | boolean | false |
| `showQuickJumper` | 是否可以快速跳转至某页 | boolean | false |
| `showSizeChanger` | 是否展示 `pageSize` 切换器；未设时 `total > totalBoundaryShowSizeChanger` 默认为 true | boolean | auto（见 boundary） |
| `showTotal` | 用于显示数据总量和当前数据顺序 | `func(total, range[2]) string` | — |
| `simple` | 简单分页；`readOnly` 时当前页只读 | boolean \| `{ readOnly?: boolean }` | false |
| `size` | 组件尺寸 | `large` \| `medium` \| `small` | `medium` |
| `total` | **数据总数**（条目数，非总页数） | number | 0 |
| `totalBoundaryShowSizeChanger` | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 |
| `onChange` | 页码或 `pageSize` 改变 | `func(page, pageSize)` | — |
| `onShowSizeChange` | `pageSize` 变化 | `func(current, size)` | — |

**配置优先级（通用）：** 受控 props（`current`/`pageSize`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
current, pageSize, total
  点页码 ──► onChange(page, pageSize)
  prev/next ──► 夹紧 1..pages
  sizeChanger ──► onShowSizeChange + onChange
  quickJumper Enter ──► 跳页
  disabled ──► 无切换
```

\*itemSize 默认 32（±0.5px）；`PageCount=ceil(total/pageSize)`，total=0 时 PageCount=1。

**触发条件 + 容差（可断言）：**

- 点页：点页码 → `current=page` + `OnChange(page,pageSize)`；prev/next 夹紧 `1..pages`，越界不变（PG-S2/S3）。
- size：切 pageSize → `OnShowSizeChange(current,size)` + `OnChange`，current 夹紧到新 pages；未显式设 `showSizeChanger` 时 `total>50` 自动展示（`totalBoundaryShowSizeChanger=50`，6.2.0）。
- jumper：输入页码 Enter 跳转，越界夹紧；`simple.readOnly` 时输入只读不可跳。
- 隐藏：`hideOnSinglePage=true` 且 pages=1 时整树隐藏；`showLessItems` 减少页码项数。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| PG-S1 | 点第 2 页 | current=2；onChange(page=2,pageSize) |
| PG-S2 | 首页点 prev | 不变，无回调 |
| PG-S3 | 末页点 next | 不变，无回调 |
| PG-S4 | 改 pageSize | OnShowSizeChange + OnChange，current 夹紧 |
| PG-S5 | jumper 输入页码 Enter | 跳转并夹紧；越界到首/末 |
| PG-S6 | disabled | 不切换，无回调 |
| PG-S7 | simple | 简化 UI（prev + 输入/文案 + next） |
| PG-S8 | 项高 middle | 32（±0.5px） |
| PG-S9 | total=0 | PageCount=1，展示第 1 页 |
| PG-S10 | showTotal | 文案含总数与范围（`showTotal(total,start,end)`） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 页码项高 32/24/40（±0.5px），字 14/12/16；项底 `itemLinkBg` + 边 `colorBorder` + 圆角 6（small 4/large 8） |
| active | 当前页底 `itemActiveBg` + 主色字/边；禁用激活底 `itemActiveBgDisabled` |
| hover/focus | 项 hover 边/字强调 + focus ring 可见（outset ≈1.5px） |
| disabled | 整树降对比不可点；激活项用禁用底 |
| simple | prev/next + 当前/总数文案（或只读输入），无页码列表 |
| jumper/changer | 输入框 + Go（若配）/ Select 档位行尾对齐，行宽随之增加 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `navigation`；页码列为 list，单项为 button（当前页 `aria-current=page`） |
| 当前 | 当前页 `aria-current=page` + `itemActiveBg` 双通道；读屏报“第 X 页共 Y 页” |
| 键盘 | 页码 Tab 可聚焦 ring 可见；Enter/Space 跳页；jumper 输入 Enter 跳转 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 真实映射（gpui 侧落点） | 级别 |
| --- | --- | --- |
| 点页/夹紧/回调（§6.4 PG-S1~S6） | **对等**：`SetCurrent/SetPageSize` + `OnChange/OnShowSizeChange` | P0 L1 |
| 页项度量（§6.2 itemSize 32/24/40） | **对等** | P0 L2 |
| `showSizeChanger` 下拉切换 | **对等**：桌面 Select 映射（`pageSizeOptions` 档位，默认 10/20/50/100；`total>50` 自动展示） | P0 L1 |
| `showQuickJumper` 输入跳页 | **对等**：桌面输入框 Enter 跳页（`goButton` 可配 P1） | P0 L1 |
| `simple` 简洁分页 | **对等**：prev + 文案/只读输入 + next | P0 L1 |
| `responsive` 按宽自适应 | **对等**：`SetViewportWidth` 映射，小宽切 small | P0 L1 |
| 滚动宿主 | **映射**：分页行 `align`（start/center/end）由父布局宿主决定；弹层无，全树随容器滚动 | P0 宿主 |
| `showTitle` 原生 tooltip | P1 不做（桌面无原生 title） | P1 |
| `itemRender` 自定义页码结构 | P1 必做（二期 `SetItemRender(page,kind)`，SEO 语义 P1） | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `onChange` | 必须 |
| `current` / `defaultCurrent` / `total` / `pageSize` / `defaultPageSize` | 必须；`total` 为条目数，`PageCount=ceil(total/pageSize)` |
| `showTotal` | 必须；`SetShowTotal(func(total, start, end int) string)`，起始侧总数文案 |
| `showQuickJumper` | 必须；输入页码 `Enter` 跳转 |
| `showSizeChanger` / `pageSizeOptions` / `totalBoundaryShowSizeChanger` | 必须；未显式设置时 `total>50` 自动展示切换器 |
| `hideOnSinglePage` / `showLessItems` | 必须 |
| `simple` | 必须 |
| `disabled` | 必须 |
| `size` | 必须 |
| `itemRender` 自定义页码结构 | 二期接口形状：`SetItemRender(func(page int, kind ItemKind) core.Node)`，本期可用默认节点；SEO 优化语义 P1 |
| 官方主路径示例（P0，10 例） | 基本（`basic.tsx`）、方向（`align.tsx`，5.19.0）、更多（`more.tsx` 省略号）、改变（`changer.tsx` changer+回调）、跳转（`jump.tsx` jumper）、尺寸（`mini.tsx` small）、简洁（`simple.tsx`）、受控（`controlled.tsx`）、总数（`total.tsx`）、全部展示（`all.tsx` 三件套同屏） |
| 度量 §6.2 | Token 断言（含 `itemSize/itemActiveBg/itemLinkBg`，高 32/24/40±0.5） |
| a11y §6.6 | 当前页 `aria-current=page`；键盘 Enter 跳页 |
| §6.9 中 L1/L2 用例 | 测试通过 |

**逐例 P0/P1 对照表**（§2.4 全量；P0+P1=§6.8 非debug 10+5 例一次做完）：

| 示例 | 范围 | 原因 |
| --- | --- | --- |
| 基本 / 方向 / 更多 / 改变 / 跳转 / 尺寸 / 简洁 / 受控 / 总数 / 全部展示 | P0 | 页码+省略+changer+jumper+总数主路径 |
| 上一步和下一步（`itemRender.tsx`） | P1 | `itemRender` 自定义结构 P1必做（二期形状见 P0） |
| 自定义语义结构的样式和类（`style-class.tsx`）/ `_semantic.tsx` | P1 | semantic 深度 |
| 线框风格（`wireframe.tsx`，debug）/ 组件 Token（`component-token.tsx`，debug）/ 变体 Debug（`variant-debug.tsx`，debug） | P1 | 调试页，不验收 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | P1 必做（`style-class.tsx` / `_semantic.tsx`，见逐例表） |
| 动画像素级 / 复杂虚拟列表 | P1 必做 |
| 浏览器-only API 或桌面无等价项 | P1 必做（`showTitle` 原生 tooltip、`itemRender` SEO 语义） |
| debug 示例 | 不验收（仅参考；`wireframe`/`component-token`/`variant-debug`，见逐例表） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestPagination_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Pagination 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| PG-01 | L1 | NewPagination 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| PG-02 | L1 | 点第 2 页 | current=2；OnChange(2,pageSize) |
| PG-03 | L1 | 首页点 prev | 不变，无回调 |
| PG-04 | L1 | 末页点 next | 不变，无回调 |
| PG-05 | L1 | 改 pageSize | OnShowSizeChange + OnChange，current 夹紧 |
| PG-06 | L1 | jumper 输入页码 Enter | 跳转并夹紧越界 |
| PG-07 | L1 | disabled=true 后点页码 | 不切换，无回调；整树禁用色 |
| PG-08 | L1 | simple=true | 仅 prev + 文案/输入 + next，无页码列表 |
| PG-09 | L1 | 项高 middle | 32（±0.5px） |
| PG-10 | L1 | total=0 | PageCount=1，展示第 1 页 |
| PG-11 | L1 | showTotal 注入 | 文案含总数与范围（start/end 正确） |
| PG-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | total=50 默认页码可点切，OnChange 页码正确 |
| PG-13 | L1 | 复现官方示例「方向」（`align.tsx`） | `align=center/end` 行对齐可断言 |
| PG-14 | L1 | 复现官方示例「更多」（`more.tsx`） | total=500 时省略号（jump-prev/next）可见可点 |
| PG-15 | L1 | 复现官方示例「改变」（`changer.tsx`） | 切 pageSize 档回调双调，current 夹紧 |
| PG-16 | L1 | 复现官方示例「跳转」（`jump.tsx`） | jumper 输入页码 Enter 跳转，越界夹紧 |
| PG-17 | L1 | 复现官方示例「尺寸」（`mini.tsx`） | size=small 项高 24（±0.5px），字 12 |
| PG-18 | L1 | 复现官方示例「简洁」（`simple.tsx`） | simple 行无页码列表，prev/next 可切 |
| PG-19 | L1 | 复现官方示例「受控」（`controlled.tsx`） | 受控 current/pageSize 外部优先，回调后父级回写才变 |
| PG-19b | L1 | 复现官方示例「总数」（`total.tsx`） | `showTotal` 文案含总数与范围 |
| PG-19c | L1 | 复现官方示例「全部展示」（`all.tsx`） | `showSizeChanger`+`showQuickJumper`+`showTotal` 同屏可构建 |
| PG-20 | L2 | 读取 §6.2 关键尺寸/间距 | 高 32/24/40、圆角 6（small 4/large 8）±0.5px |
| PG-21 | L2 | 默认皮颜色 | 当前页 `itemActiveBg`+主色字；普通项 `itemLinkBg`；无硬编码 |
| PG-22 | L2 | disabled 外观 | 整树禁用色；激活项用 `itemActiveBgDisabled`；无 hover |
| PG-23 | L1 | 键盘/焦点主路径 | 页码 Tab 聚焦 ring 可见；Enter/Space 跳页；jumper Enter 跳转 |
| PG-24 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| PG-25 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| PG-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API（旧 `NewPagination(totalPages)`、`Total`=总页数、`OnChange(page int)`、`SetPage`/`SetTotalPages` 全部废弃）。  
> 以下为 **产品需求层** 契约；命名可微调但语义不可丢。

```text
NewPagination() *Pagination   // total=0, current=1, pageSize=10（antd 默认）

// 分页状态（total = 数据条数，非总页数）
SetTotal(n int)
SetCurrent(page int)                 // 受控 current
SetDefaultCurrent(page int)          // 非受控初始
SetPageSize(n int)                   // 受控 pageSize
SetDefaultPageSize(n int)
PageCount() int                      // ceil(total/pageSize)，至少 1（total=0 → 1）

// P0 配置
SetDisabled(bool)
SetSize(PaginationSize)              // small | middle | large → itemH 24/32/40
SetAlign(PaginationAlign)            // start | center | end
SetSimple(bool) / SetSimpleReadOnly(bool)
SetShowQuickJumper(bool)
SetShowSizeChanger(bool)             // 显式；未调时走 totalBoundary 自动
SetHideOnSinglePage(bool)
SetShowLessItems(bool)
SetPageSizeOptions([]int)            // 默认 10/20/50/100
SetTotalBoundaryShowSizeChanger(n)   // 默认 50
SetShowTotal(fn func(total, start, end int) string)

// 回调
SetOnChange(func(page, pageSize int))
SetOnShowSizeChange(func(current, size int))

// 主题 / a11y / 挂树
SetTheme(*Theme) · SetFace(text.Face) · SetAriaLabel(string)
Node() core.Node · ChromeNode() core.Node
// 测试钩：ItemPressable(kind, page) — kind=page|prev|next|jump-prev|jump-next
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Current / DefaultCurrent | 1 |
| PageSize / DefaultPageSize | 10 |
| Total | 0（PageCount=1，展示第 1 页） |
| Disabled | false |
| Size | middle（item 高 32） |
| Align | start |
| ShowQuickJumper | false |
| ShowSizeChanger | auto：`total > 50` 时 true，否则 false；`SetShowSizeChanger` 后固定 |
| Simple | false |
| HideOnSinglePage | false |
| PageSizeOptions | 10, 20, 50, 100 |
| totalBoundaryShowSizeChanger | 50 |

### 6.11 结构与绘制分层（实现提示）

```text
Nav（role=navigation，align start/center/end 由父布局定）
  ├─ showTotal 文案（起始侧次级色）
  ├─ prev/next + 页码项 Pressable（高 32/24/40，当前页 itemActiveBg）
  │    └─ jump-prev/jump-next 省略节点（... 可点展）
  ├─ showSizeChanger Select（pageSizeOptions 档位）
  └─ showQuickJumper 输入框（Enter 跳页；simple 下为当前/总数文案）
```

- 组合 `ui/primitive` + `ui/core` + Select/输入框底座，禁止第二套事件/帧循环。
- 无浮层（`showTitle` 原生 tooltip 不做）；`rebuild()` 只读 Default/字段/Token。
- 命中区域与布局盒一致（`hit == layout == paint`）；项间距 8 用 Flex gap。
- 无动画需求；尊重 reduced-motion。

### 6.12 完成定义（DoD）

同时满足即可宣布 **Pagination 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/pagination.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Pagination 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
