# Timeline 时间轴
> 来源：[Ant Design 6.5.x Timeline](https://ant.design/components/timeline)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：垂直展示的时间流信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

垂直展示的时间流信息。

**Timeline** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 变体样式 | variant 线框/填充差异 |
| 等待及排序 | 复现「等待及排序」视觉与布局 |
| 交替展现 | 复现「交替展现」视觉与布局 |
| 水平布局 | 横向布局 |
| 自定义时间轴点 | 自定义渲染/插槽外观 |
| 另一侧时间轴点 | 复现「另一侧时间轴点」视觉与布局 |
| 标题 | 复现「标题」视觉与布局 |
| 标题占比 | 复现「标题占比」视觉与布局 |
| 语义化自定义 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `mode`

- **说明**：通过设置 `mode` 可以改变时间轴和内容的相对位置
- **类型**：`start` | `alternate` | `end`
- **默认值**：`start`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `alternate` | 官方取值 `alternate` |
  | `end` | 逻辑结束侧 |

#### `orientation`

- **说明**：设置时间轴的方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`vertical`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `pending`

- **说明**：指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替
- **类型**：ReactNode
- **默认值**：false

#### `pendingDot`

- **说明**：当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替
- **类型**：ReactNode
- **默认值**：<LoadingOutlined />

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `titleSpan`

- **说明**：设置标题占比空间，为到 dot 中心点距离
- **类型**：number | string
- **默认值**：12

#### `variant`

- **说明**：设置样式变体
- **类型**：`filled` | `outlined`
- **默认值**：`outlined`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |
  | `outlined` | 描边空心 |

#### `color`

- **说明**：指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值
- **类型**：string
- **默认值**：`blue`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `blue` | 官方取值 `blue` |
  | `red` | 官方取值 `red` |
  | `green` | 官方取值 `green` |
  | `gray` | 官方取值 `gray` |

#### `content`

- **说明**：设置内容
- **类型**：ReactNode
- **默认值**：-

#### `dot`

- **说明**：自定义时间轴点，请使用 `icon` 替换
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义节点图标
- **类型**：ReactNode
- **默认值**：-

#### `loading`

- **说明**：设置加载状态
- **类型**：boolean
- **默认值**：false

#### `placement`

- **说明**：自定义节点位置
- **类型**：`start` | `end`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `position`

- **说明**：自定义节点位置，请使用 `placement` 替换
- **类型**：`start` | `end`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `title`

- **说明**：设置标题
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

- 当有一系列信息需按时间排列时，可正序和倒序。
- 需要有一条时间轴进行视觉上的串联时。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **变体样式**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **等待及排序**（`pending.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **交替展现**（`alternate.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **水平布局**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义时间轴点**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **另一侧时间轴点**（`end.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **标题**（`title.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **标题占比**（`title-span.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **语义化自定义**（`semantic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `loading` | 加载中 | 设置加载状态 |
| `items` | 数据化 items | 选项配置 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 变体样式 | `variant.tsx` | 否 |
| 等待及排序 | `pending.tsx` | 否 |
| 最后一个及排序 | `pending-legacy.tsx` | 是 |
| 交替展现 | `alternate.tsx` | 否 |
| 水平布局 | `horizontal.tsx` | 否 |
| 水平布局 | `horizontal-debug.tsx` | 是 |
| 自定义时间轴点 | `custom.tsx` | 否 |
| 另一侧时间轴点 | `end.tsx` | 否 |
| 标题 | `title.tsx` | 否 |
| 标题占比 | `title-span.tsx` | 否 |
| 语义化自定义 | `semantic.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
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

### Timeline

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | items | 选项配置 | [Items](#items)[] | - | mode | 通过设置 `mode` 可以改变时间轴和内容的相对位置 | `start` \| `alternate` \| `end` | `start` | orientation | 设置时间轴的方向 | `vertical` \| `horizontal` | `vertical` | ~~pending~~ | 指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替 | ReactNode | false | ~~pendingDot~~ | 当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替 | ReactNode | &lt;LoadingOutlined /&gt; | reverse | 节点排序 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | titleSpan | 设置标题占比空间，为到 dot 中心点距离 <InlinePopover previewURL="https://mdn.alipayobjects.com/huamei_7uahnr/afts/img/A*1NJISa7bpqgAAAAAR5AAAAgAerJ8AQ/original"></InlinePopover> | number \| string | 12 | variant | 设置样式变体 | `filled` \| `outlined` | `outlined` 
### Items

时间轴的每一个节点。

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| color | 指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值 | string | `blue` |
| content | 设置内容 | ReactNode | - |
| ~~children~~ | 设置内容，请使用 `content` 替换 | ReactNode | - |
| ~~dot~~ | 自定义时间轴点，请使用 `icon` 替换 | ReactNode | - |
| icon | 自定义节点图标 | ReactNode | - |
| ~~label~~ | 设置标签，请使用 `title` 替换 | ReactNode | - |
| loading | 设置加载状态 | boolean | false |
| placement | 自定义节点位置 | `start` \| `end` | - |
| ~~position~~ | 自定义节点位置，请使用 `placement` 替换 | `start` \| `end` | - |
| title | 设置标题 | ReactNode | - |

### 导入方式

```js
import { Timeline } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `items` | 选项配置 | [Items](#items)[] | - | — |
| `mode` | 通过设置 `mode` 可以改变时间轴和内容的相对位置 | `start` \| `alternate` \| `end` | `start` | — |
| `orientation` | 设置时间轴的方向 | `vertical` \| `horizontal` | `vertical` | — |
| `pending` | 指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替 | ReactNode | false | — |
| `pendingDot` | 当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替 | ReactNode | <LoadingOutlined /> | — |
| `reverse` | 节点排序 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `titleSpan` | 设置标题占比空间，为到 dot 中心点距离 | number \| string | 12 | — |
| `variant` | 设置样式变体 | `filled` \| `outlined` | `outlined` | — |
| `color` | 指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值 | string | `blue` | — |
| `content` | 设置内容 | ReactNode | - | — |
| `children` | 设置内容，请使用 `content` 替换 | ReactNode | - | — |
| `dot` | 自定义时间轴点，请使用 `icon` 替换 | ReactNode | - | — |
| `icon` | 自定义节点图标 | ReactNode | - | — |
| `label` | 设置标签，请使用 `title` 替换 | ReactNode | - | — |
| `loading` | 设置加载状态 | boolean | false | — |
| `placement` | 自定义节点位置 | `start` \| `end` | - | — |
| `position` | 自定义节点位置，请使用 `placement` 替换 | `start` \| `end` | - | — |
| `title` | 设置标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Timeline** 的验收清单：

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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/timeline
- 中文文档：https://ant.design/components/timeline-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/timeline
- 驱动 gpui kit：`timeline`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Timeline** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/timeline/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Timeline）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Timeline）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：垂直展示的时间流信息。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `items` | 选项配置 | [Items](#items)[] | - |
| `mode` | 通过设置 `mode` 可以改变时间轴和内容的相对位置 | `start` \ | `alternate` \ |
| `orientation` | 设置时间轴的方向 | `vertical` \ | `horizontal` |
| `reverse` | 节点排序 | boolean | false |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `titleSpan` | 设置标题占比空间，为到 dot 中心点距离 <InlinePopover previewURL="https://… | number \ | string |
| `variant` | 设置样式变体 | `filled` \ | `outlined` |
| `color` | 指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值 | string | `blue` |
| `content` | 设置内容 | ReactNode | - |
| `icon` | 自定义节点图标 | ReactNode | - |
| `loading` | 设置加载状态 | boolean | false |
| `placement` | 自定义节点位置 | `start` \ | `end` |
| `title` | 设置标题 | ReactNode | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
items 时间轴渲染
mode alternate 左右
pending 末尾未完成
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TL-S1 | 3 items | 3 节点 |
| TL-S2 | alternate | 左右交错 |
| TL-S3 | pending | 末尾 pending |
| TL-S4 | reverse | 倒序 |
| TL-S5 | color 点 | 点色 |
| TL-S6 | 自定义 dot | 自定义点 |
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
| 表格/树/列表 | 结构角色与展开/选中态可读 |
| 排序/筛选 | 控件有名 |

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
| `loading` | 必须 |
| `variant` | 必须 |
| `items` | 必须 |
| `title` | 必须 |
| `content` | 必须 |
| `placement` | 必须 |
| `mode` | 必须 |
| `orientation` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | 基本用法、变体样式、等待及排序、交替展现、水平布局、自定义时间轴点、另一侧时间轴点、标题 |
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
| 其余示例 | 标题占比, 语义化自定义, 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTimeline_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Timeline 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TL-01 | L1 | NewTimeline 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TL-02 | L1 | 3 items | 3 节点 |
| TL-03 | L1 | alternate | 左右交错 |
| TL-04 | L1 | pending | 末尾 pending |
| TL-05 | L1 | reverse | 倒序 |
| TL-06 | L1 | color 点 | 点色 |
| TL-07 | L1 | 自定义 dot | 自定义点 |
| TL-08 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-09 | L1 | 复现官方示例「变体样式」（`variant.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-10 | L1 | 复现官方示例「等待及排序」（`pending.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-11 | L1 | 复现官方示例「交替展现」（`alternate.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-12 | L1 | 复现官方示例「水平布局」（`horizontal.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-13 | L1 | 复现官方示例「自定义时间轴点」（`custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-14 | L1 | 复现官方示例「另一侧时间轴点」（`end.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-15 | L1 | 复现官方示例「标题」（`title.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TL-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TL-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TL-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TL-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TL-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TL-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TL-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewTimeline(...) *Timeline

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

同时满足即可宣布 **Timeline 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/timeline.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Timeline 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
