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
| `totalBoundaryShowSizeChanger` | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 | — |
| `onChange` | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 | function(page, pageSize) | - | — |
| `onShowSizeChange` | pageSize 变化的回调 | function(current, size) | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Pagination** 的验收清单：

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
- 官方文档：https://ant.design/components/pagination
- 中文文档：https://ant.design/components/pagination-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/pagination
- 驱动 gpui kit：`pagination`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Pagination** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/pagination/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Pagination）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 选中/展开/分页或步骤切换与键盘 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Pagination）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：分页器用于分隔长列表，每次只加载一个页面。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 分页项高 itemSize | **32** | controlHeight |
| itemSize | **32/24/40** | controlHeight* |
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
| `align` | 对齐方式 | start \ | center \ |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `current` | 当前页数 | number | - |
| `defaultCurrent` | 默认的当前页数 | number | 1 |
| `defaultPageSize` | 默认的每页条数 | number | 10 |
| `disabled` | 禁用分页 | boolean | - |
| `hideOnSinglePage` | 只有一页时是否隐藏分页器 | boolean | false |
| `itemRender` | 用于自定义页码的结构，可用于优化 SEO | (page, type: 'page' \ | 'prev' \ |
| `pageSize` | 每页条数 | number | - |
| `pageSizeOptions` | 指定每页可以显示多少条 | number\[] | \[`10`, `20`, `50`, `100`] |
| `responsive` | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | - |
| `showLessItems` | 是否显示较少页面内容 | boolean | false |
| `showQuickJumper` | 是否可以快速跳转至某页 | boolean \ | { goButton: ReactNode } |
| `showSizeChanger` | 是否展示 `pageSize` 切换器 | boolean \ | [SelectProps](/components/select-cn#api) |
| `showTitle` | 是否显示原生 tooltip 页码提示 | boolean | true |
| `showTotal` | 用于显示数据总量和当前数据顺序 | function(total, range) | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
current, pageSize, total
  点页码 ──► onChange(page, pageSize)
  prev/next ──► 夹紧 1..pages
  sizeChanger ──► onShowSizeChange + onChange
  quickJumper Enter ──► 跳页
  disabled ──► 无切换
```

\*itemSize 默认 32。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| PG-S1 | 点第 2 页 | current=2；onChange |
| PG-S2 | 首页点 prev | 不变 |
| PG-S3 | 末页点 next | 不变 |
| PG-S4 | 改 pageSize | 回调 |
| PG-S5 | jumper 输入页码 Enter | 跳转 |
| PG-S6 | disabled | 不切换 |
| PG-S7 | simple | 简化 UI |
| PG-S8 | 项高 middle | 32 |
| PG-S9 | total=0 | 合理空/一页 |
| PG-S10 | showTotal | 文案含总数 |
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
| `onChange` | 必须 |
| `disabled` | 必须 |
| `size` | 必须 |
| 官方主路径示例 | 基本、方向、更多、改变、跳转、尺寸、简洁、受控 |
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
| 其余示例 | 总数, 全部展示, 上一步和下一步, 自定义语义结构的样式和类 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestPagination_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Pagination 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| PG-01 | L1 | NewPagination 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| PG-02 | L1 | 点第 2 页 | current=2；onChange |
| PG-03 | L1 | 首页点 prev | 不变 |
| PG-04 | L1 | 末页点 next | 不变 |
| PG-05 | L1 | 改 pageSize | 回调 |
| PG-06 | L1 | jumper 输入页码 Enter | 跳转 |
| PG-07 | L1 | disabled | 不切换 |
| PG-08 | L1 | simple | 简化 UI |
| PG-09 | L1 | 项高 middle | 32 |
| PG-10 | L1 | total=0 | 合理空/一页 |
| PG-11 | L1 | showTotal | 文案含总数 |
| PG-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-13 | L1 | 复现官方示例「方向」（`align.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-14 | L1 | 复现官方示例「更多」（`more.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-15 | L1 | 复现官方示例「改变」（`changer.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-16 | L1 | 复现官方示例「跳转」（`jump.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-17 | L1 | 复现官方示例「尺寸」（`mini.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-18 | L1 | 复现官方示例「简洁」（`simple.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-19 | L1 | 复现官方示例「受控」（`controlled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| PG-20 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| PG-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| PG-22 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| PG-23 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| PG-24 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| PG-25 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| PG-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewPagination(...) *Pagination

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

同时满足即可宣布 **Pagination 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/pagination.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Pagination 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
