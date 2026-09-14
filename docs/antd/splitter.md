# Splitter 分隔面板
> 来源：[Ant Design 6.5.x Splitter](https://ant.design/components/splitter)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：自由切分指定区域  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

自由切分指定区域

**Splitter** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 垂直方向 | 纵向布局 |
| 可折叠 | 复现「可折叠」视觉与布局 |
| 可折叠图标显示 | icon 与文本混排 |
| 多面板 | 复现「多面板」视觉与布局 |
| 复杂组合 | 复现「复杂组合」视觉与布局 |
| 延迟渲染模式 | 复现「延迟渲染模式」视觉与布局 |
| 自定义样式 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 双击重置 | 复现「双击重置」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `collapsible`

- **说明**：`motion` 是否开启折叠动画，`icon` 自定义折叠图标
- **类型**：`{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `motion` | 官方取值 `motion` |
  | `icon` | 官方取值 `icon` |

#### `collapsibleIcon`

- **说明**：折叠图标
- **类型**：`{start?: ReactNode; end?: ReactNode}`
- **默认值**：-
- **版本**：6.0.0

#### `draggerIcon`

- **说明**：拖拽图标
- **类型**：`ReactNode`
- **默认值**：-
- **版本**：6.0.0

#### `layout`

- **说明**：布局方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `orientation`

- **说明**：布局方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `vertical`

- **说明**：排列方向，与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：`false`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `onResize`

- **说明**：面板大小变化回调
- **类型**：`(sizes: number[]) => void`
- **默认值**：-

#### `defaultSize`

- **说明**：初始面板大小，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `max`

- **说明**：最大阈值，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `min`

- **说明**：最小阈值，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `size`

- **说明**：受控面板大小，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

### 1.4 交互视觉状态（实现检查表）

| 状态 | 要求 |
| --- | --- |
| default | 默认色、边框、阴影符合 token |
| hover | 可交互控件需有悬停反馈 |
| active/pressed | 按下态对比或反馈（若适用） |
| focus | 可见 focus ring，键盘可达 |
| error/warning | 与 status/Form 语义色一致 |

### 1.5 语义化 DOM 与主题

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 可以水平或垂直地分隔区域。
- 当需要自由拖拽调整各区域大小。
- 当需要指定区域的最大最小宽高时。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **受控模式**（`control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **垂直方向**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可折叠**（`collapsible.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **可折叠图标显示**（`collapsibleIcon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **多面板**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **复杂组合**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **延迟渲染模式**（`lazy.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义样式**（`customize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **双击重置**（`reset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onResizeStart` / `onResize` / `onResizeEnd` | 拖拽三段回调 | 起拖 / 尺寸变化（`lazy` 时拖中不发） / 松手提交，参数均为 `sizes: number[]` |
| `size` / `defaultSize` | 受控/非受控 | `size` 已设为受控（须配 `onResize` 回写，否则面板不动）；`defaultSize` 为初值 |
| `min` / `max` | 夹紧阈值 | px 或百分比，拖拽与折叠恢复均受夹紧 |
| `collapsible` | 折叠 | 面板 `size→0`，空间让给邻面板；触发 `onCollapse(collapsed[], sizes[])` |
| `resizable` | 禁拖 | `false` 时不可拖（仍可折叠）；条无 spinner，cursor 默认 |
| `lazy` | 延迟渲染 | 拖中仅画预览线，松手一次提交 `size` |
| `destroyOnHidden` | 隐藏销毁 | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 |
| `onDraggerDoubleClick` | 双击条 | 双击拖拽条回调 `(index)` |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `size.tsx` | 否 |
| 受控模式 | `control.tsx` | 否 |
| 垂直方向 | `vertical.tsx` | 否 |
| 可折叠 | `collapsible.tsx` | 否 |
| 可折叠图标显示 | `collapsibleIcon.tsx` | 否 |
| 多面板 | `multiple.tsx` | 否 |
| 复杂组合 | `group.tsx` | 否 |
| 延迟渲染模式 | `lazy.tsx` | 否 |
| 自定义样式 | `customize.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 双击重置 | `reset.tsx` | 否 |
| 标签页中嵌套 | `nested-in-tabs.tsx` | 否 |
| 调试 | `debug.tsx` | 是 |
| 尺寸混合 | `size-mix.tsx` | 是 |

> debug 标记依据：`debug.tsx` 文件名含 debug；`size-mix.tsx` 无独立文档 md，为 `size.tsx` 的辅助源文件，标 debug；`nested-in-tabs.tsx` 为正经 Tabs 嵌套场景 demo（文件名无 debug），标非 debug。

### 2.7 组合关系

- **依赖等级**：**L1 无依赖基础件**；拖拽条+面板纯布局交互，不依赖组内其他 10 件，可先行实现。
- **等谁**：无；面板内容由业务挂节点（不同文件，并行安全）。
- **文件归属**：`ui/kit/splitter/`（只改自己文件，并行安全；拖拽把手为本件自有，保留）。
- **ConfigProvider**：主题（条 Token）、默认 props。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

> Splitter 组件需要通过子元素计算面板大小，因而其子元素仅支持 `Splitter.Panel`。

### Splitter

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| collapsible | `motion` 是否开启折叠动画，`icon` 自定义折叠图标 | `{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }` | - | 6.4.0 | × |
| ~~collapsibleIcon~~ | 折叠图标 | `{start?: ReactNode; end?: ReactNode}` | - | 6.0.0 | × |
| destroyOnHidden | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 | `boolean` | `false` | 6.4.0 | × |
| draggerIcon | 拖拽图标 | `ReactNode` | - | 6.0.0 | × |
| ~~layout~~ | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - | × |
| lazy | 延迟渲染模式 | `boolean` | `false` | 5.23.0 | × |
| onCollapse | 展开-收起时回调 | `(collapsed: boolean[], sizes: number[]) => void` | - | 5.28.0 | × |
| orientation | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| vertical | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | - | × |
| onDraggerDoubleClick | 双击拖拽条回调 | `(index: number) => void` | - | 6.3.0 | × |
| onResize | 面板大小变化回调 | `(sizes: number[]) => void` | - | - | × |
| onResizeEnd | 拖拽结束回调 | `(sizes: number[]) => void` | - | - | × |
| onResizeStart | 开始拖拽之前回调 | `(sizes: number[]) => void` | - | - | × |

### Panel

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| collapsible | 快速折叠 | `boolean \| { start?: boolean; end?: boolean; showCollapsibleIcon?: boolean \| 'auto' }` | `false` | showCollapsibleIcon: 5.27.0 |
| defaultSize | 初始面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| destroyOnHidden | 折叠时（size 为 0）销毁面板内容，覆盖 Splitter 的 `destroyOnHidden` | `boolean` | - | 6.4.0 |
| max | 最大阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| min | 最小阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| resizable | 是否开启拖拽伸缩 | `boolean` | `true` | - |
| size | 受控面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |

### 导入方式

```js
import { Splitter } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `collapsible` | `motion` 是否开启折叠动画，`icon` 自定义折叠图标 | `{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }` | - | 6.4.0 |
| `collapsibleIcon` | 折叠图标 | `{start?: ReactNode; end?: ReactNode}` | - | 6.0.0 |
| `destroyOnHidden` | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 | `boolean` | `false` | 6.4.0 |
| `draggerIcon` | 拖拽图标 | `ReactNode` | - | 6.0.0 |
| `layout` | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - |
| `lazy` | 延迟渲染模式 | `boolean` | `false` | 5.23.0 |
| `onCollapse` | 展开-收起时回调 | `(collapsed: boolean[], sizes: number[]) => void` | - | 5.28.0 |
| `orientation` | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | — |
| `onDraggerDoubleClick` | 双击拖拽条回调 | `(index: number) => void` | - | 6.3.0 |
| `onResize` | 面板大小变化回调 | `(sizes: number[]) => void` | - | - |
| `onResizeEnd` | 拖拽结束回调 | `(sizes: number[]) => void` | - | - |
| `onResizeStart` | 开始拖拽之前回调 | `(sizes: number[]) => void` | - | - |
| `defaultSize` | 初始面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `max` | 最大阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `min` | 最小阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `resizable` | 是否开启拖拽伸缩 | `boolean` | `true` | - |
| `size` | 受控面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Splitter** 的验收清单：

1. **配置面**：`orientation`/`vertical` + Panel（size/defaultSize/min/max/resizable/collapsible）+ `lazy` + 回调（onResize/Start/End/onCollapse）+ `destroyOnHidden`（§6.3）。
2. **几何**：条可视 2、命中 6、把手 20（§6.2，±0.5px）；命中≥可视，`hit==layout==paint`。
3. **交互**：拖拽改尺寸±0.5px（min/max 夹紧），lazy 拖中预览松手提交，受控只发回调（§6.4 SPL-S1…S9，§6.11）。
4. **无障碍**：条可聚焦命名，方向键微调；折叠按钮可键盘（§6.6）。
5. **示例矩阵**：P0 按 §6.8（8 例）、余下 P1 一次做完；与 §4 例数打架以 §6.8 为准。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/splitter
- 中文文档：https://ant.design/components/splitter-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/splitter
- 驱动 gpui kit：`splitter`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Splitter** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/splitter/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Splitter）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 布局参数驱动子项几何正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Splitter）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：自由切分指定区域

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

数值来自 antd `components/splitter/style` `prepareComponentToken` + 全局种子。

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 拖拽可视条宽 `splitBarSize` | **2** | 组件 Token（kit `DefaultSplitBarSize`） |
| 拖拽命中区 `splitTriggerSize` | **6** | 组件 Token（kit `DefaultSplitTriggerSize`）；**命中 ≥ 可视** |
| 拖拽把手标识高 `splitBarDraggableSize` | **20** | 组件 Token（kit `DefaultSplitBarDraggableSize`） |
| 折叠按钮（水平条） | 宽≈`fontSizeSM`、高≈`controlHeightSM` | Token 回落 12×24 |

> 面板内容区无字号/圆角自有 chrome（子项自理）；条聚焦 ring 见 §6.5/§6.6。

> **布局约定（gpui）：** 面板 `sizes` 之和 = 容器主轴长度（与 antd 一致；条在边界上叠加命中区，不挤占 flex 百分比基数）。`hit == layout == paint`：条节点布局盒 = `splitTriggerSize`，居中压在相邻面板接缝上。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 拖拽条 hover 底 | `colorBgTextHover` | 对应 antd `controlItemBgHover` |
| 拖拽条 active 底 | `colorBgTextActive` | 对应 antd active hover 阶 |
| 拖拽把手填充 | `colorFillSecondary` / `colorTextSecondary` | antd `colorFill` 近似 |
| 主色 / lazy 预览 | `colorPrimary` @ 低透明 | lazy 拖动预览线 |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | 折叠图标 |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用（不可拖） | 无把手 spinner；cursor default | `resizable=false` |
| Focus ring | `controlOutline` / primary 描边 | 条可聚焦时 |

禁止硬编码品牌色（如 `#1677ff`）作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**布局**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `collapsible` | `motion` 是否开启折叠动画，`icon` 自定义折叠图标 | `{ motion?: boolean; icon?: { start?:… | - |
| `destroyOnHidden` | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 | `boolean` | `false` |
| `draggerIcon` | 拖拽图标 | `ReactNode` | - |
| `lazy` | 延迟渲染模式 | `boolean` | `false` |
| `onCollapse` | 展开-收起时回调 | `(collapsed: boolean[], sizes: number… | - |
| `orientation` | 布局方向 | `horizontal` \ | `vertical` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` |
| `onDraggerDoubleClick` | 双击拖拽条回调 | `(index: number) => void` | - |
| `onResize` | 面板大小变化回调 | `(sizes: number[]) => void` | - |
| `onResizeEnd` | 拖拽结束回调 | `(sizes: number[]) => void` | - |
| `onResizeStart` | 开始拖拽之前回调 | `(sizes: number[]) => void` | - |
| `defaultSize` | 初始面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \ | string` |
| `max` | 最大阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \ | string` |
| `min` | 最小阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \ | string` |

**配置优先级：** 面板 `size`（已设=受控，须配 `onResize` 回写）> `defaultSize` 初值 > 均分；`orientation` > `vertical` 糖；面板 `destroyOnHidden` 覆盖全局。

### 6.4 交互状态机（L1）

```text
drag handle ──► 面板 size 变（夹紧 min/max）+ onResize（lazy 时拖中仅预览）
release ──► onResizeEnd（lazy 时于此提交 size）
collapsible ──► 折叠/恢复 + onCollapse
keyboard on bar ──► 方向键微调相邻面板
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SPL-S1 | 拖动条 +40px（非 lazy，容器 600） | 相邻面板尺寸变化 +40±0.5px，`onResize` 触发且载荷和=容器长 |
| SPL-S2 | 目标尺寸低于 min（如 min=100，拖到 60） | 面板钳在 100±0.5px |
| SPL-S3 | 目标尺寸高于 max（如 max=400，拖到 450） | 面板钳在 400±0.5px |
| SPL-S4 | 松手 | `onResizeEnd` 触发 1 次，载荷=当前 `PanelSizes` |
| SPL-S5 | 折叠（collapsible） | 折叠面板 size→0±0.5px，邻面板增同量，`onCollapse` 触发 |
| SPL-S6 | vertical=true | 条水平，面板上下分；拖动改高度±0.5px |
| SPL-S7 | 命中条（trigger 6，可视条 2） | 条布局盒宽 6±0.5px≥可视 2px，居中压缝 |
| SPL-S8 | lazy=true 拖中 | 拖中面板几何不变，松手一次提交 `size` 并触发 `onResizeEnd` |
| SPL-S9 | resizable=false | 拖动不改尺寸；仍可折叠（若 collapsible） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 条可视宽 2±0.5px，命中宽 6±0.5px，把手标识高 20±0.5px（§6.2） |
| hover/active | 条底走 `colorBgTextHover`/`Active`，把手填充变化 |
| focus | 条聚焦 ring 可见（§6.2 outset 约 1.5px） |
| resizable=false | 无把手 spinner，cursor 默认，不可拖 |
| 主题切换 | 条色与面板间隙随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 拖拽条 | 可聚焦命名；方向键微调相邻面板（步进约容器 1% 或 4px）；Focus ring 可见 |
| 折叠按钮 | 可聚焦命名；Enter/Space 触发折叠/恢复 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 条度量（§6.2 bar 2/trigger 6/把手 20） | **对等** | P0 L2 |
| `orientation`/`vertical` 与 `lazy`/`resizable` | **对等** | P0 L1 |
| `collapsible` 折叠动画 | P0 瞬时可关；像素级动画 P1 | P0 L1/P1 |
| `draggerIcon`/`collapsibleIcon` 自定义图标 | **对等** | P0 L1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `orientation` / `vertical` | 必须；`orientation` 优先于 `vertical` 糖 |
| `Panel.size` / `defaultSize`（px 或百分比） | 必须；`size` 已设为受控（须配 `onResize` 回写），`defaultSize` 为初值 |
| `Panel.min` / `max`（px 或百分比） | 必须；拖拽与折叠恢复均受夹紧 |
| `Panel.resizable` | 必须；false 时不可拖（仍可折叠），默认 true |
| `Panel.collapsible`（+两侧/图标显隐） | 必须；折叠 size→0，空间让给邻面板 |
| `lazy` | 必须；拖中仅画预览线，松手一次提交 |
| `onResize` / `onResizeStart` / `onResizeEnd` | 必须；载荷均为 `sizes: number[]` |
| `onCollapse` | 必须；载荷 `(collapsed[], sizes[])` |
| `destroyOnHidden`（全局/面板覆盖） | 必须；折叠 size 为 0 时销毁面板内容 |
| 官方主路径示例 | 基本用法、受控模式、垂直方向、可折叠、可折叠图标显示、多面板、复杂组合、延迟渲染模式 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | P1 必做 |
| 动画像素级 / 复杂虚拟列表 | P1 必做 |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| debug 示例 | 不验收（仅参考） |
| 其余示例 | 自定义样式, 自定义语义结构的样式和类, 双击重置, 标签页中嵌套, _semantic.tsx |

**14 例→P0/P1 范围对应表**（§2.4 全量；P0+P1=§6.8 非debug 8+4 例一次做完，2 例 debug 不验收（仅参考））：

| 示例 | 范围 | 原因 |
| --- | --- | --- |
| 基本用法/受控模式/垂直方向/可折叠/可折叠图标/多面板/复杂组合/延迟渲染 | P0 | 拖拽+折叠+受控+lazy 主路径，gallery 必备 |
| 自定义样式 | P1 | 条/面板样式覆盖深度 |
| 自定义语义结构的样式和类 | P1 | semantic 深度 |
| 双击重置 | P1 | `onDraggerDoubleClick` 整页（回调能力已验） |
| 标签页中嵌套 | P1 | Tabs 宿主组合页 |
| debug/size-mix | 不计 | 内部调试/辅助源文件（依据见 §2.4） |
| _semantic.tsx | P1 | semantic 深度 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSplitter_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Splitter 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SPL-01 | L1 | NewSplitter 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SPL-02 | L1 | 拖动条 +40px（非 lazy，容器 600） | 相邻面板变化 +40±0.5px，`onResize` 触发 |
| SPL-03 | L1 | 目标低于 min（如 min=100，拖到 60） | 面板钳在 100±0.5px |
| SPL-04 | L1 | 目标高于 max（如 max=400，拖到 450） | 面板钳在 400±0.5px |
| SPL-05 | L1 | 松手 | `onResizeEnd` 触发 1 次 |
| SPL-06 | L1 | 折叠（collapsible） | 折叠面板 size→0±0.5px，`onCollapse` 触发 |
| SPL-07 | L1 | vertical=true | 条水平，面板上下分；拖动改高度±0.5px |
| SPL-08 | L1 | 命中条（trigger 6，可视 2） | 条布局盒宽 6±0.5px≥可视 2px |
| SPL-09 | L1 | 挂 `size.tsx`（双面板 50/50，容器 600，拖 +40px） | 面板变为 340/260±0.5px，`onResize` 载荷和=600 |
| SPL-10 | L1 | 挂 `control.tsx`（受控 size，拖后父回写） | 回写前面板几何不变；回写后跟随新值±0.5px |
| SPL-11 | L1 | 挂 `vertical.tsx`（vertical=true，拖 +40px） | 面板高度变化 +40±0.5px，条水平 |
| SPL-12 | L1 | 挂 `collapsible.tsx`（折叠左面板） | 左面板 size→0±0.5px，右面板增同量，`onCollapse` 1 次 |
| SPL-13 | L1 | 挂 `collapsibleIcon.tsx`（双图标折叠） | 两侧折叠按钮均可点，各触发 `onCollapse` 1 次 |
| SPL-14 | L1 | 挂 `multiple.tsx`（三面板，拖中间条 +40px） | 中间/右侧面板变化 ±40±0.5px，两侧和=容器长 |
| SPL-15 | L1 | 挂 `group.tsx`（嵌套 Splitter） | 内外条各拖 20px，互不干扰，载荷各自为政 |
| SPL-16 | L1 | 挂 `lazy.tsx`（lazy=true，拖 +40px 不松手） | 拖中面板几何不变；松手后一次提交，`onResizeEnd` 1 次 |
| SPL-17 | L2 | 读取 §6.2 条 2/命中 6/把手 20 | 与表内数字一致（±0.5px） |
| SPL-18 | L2 | 默认皮颜色 | 条色走 `colorBgTextHover`/`Active`，无硬编码品牌色 |
| SPL-19 | L2 | resizable=false 外观 | 无把手 spinner，cursor 默认，不可拖 |
| SPL-20 | L1 | 键盘/焦点主路径 | 条可聚焦，方向键微调±步进，Focus ring 可见 |
| SPL-21 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| SPL-22 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| SPL-23 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 **breaking** 旧 `NewSplitter(first, second)` / `Ratio` / `SetRatio` 双栏 API。  
> 实现可微调命名，但下列语义不可丢。

```text
// 尺寸：px 或百分比（antd number | 'xx%'）
DimPx(px float64) SplitterDim
DimPercent(pct float64) SplitterDim   // 0..100，如 40 → "40%"
ParseDim(s string) (SplitterDim, error)

NewSplitterPanel(child core.Node) *SplitterPanel
  SetChild(core.Node)
  SetMin / SetMax / SetSize / SetDefaultSize(SplitterDim)
  SetMinPx / SetMaxPx / SetSizePx / SetDefaultSizePx(float64)
  SetMinPercent / SetMaxPercent / SetSizePercent / SetDefaultSizePercent(float64)
  SetResizable(bool)                  // 默认 true
  SetCollapsible(bool)                // 两侧均可折
  SetCollapsibleSides(start, end bool)
  SetShowCollapsibleIcon(mode)        // auto | always | never
  SetDestroyOnHidden(bool)

NewSplitter(panels ...*SplitterPanel) *Splitter
  SetPanels(...*SplitterPanel)
  SetOrientation(horizontal|vertical) // 优先于 Vertical 糖
  SetVertical(bool)
  SetLazy(bool)                       // 拖中不改 size，松手提交
  SetDestroyOnHidden(bool)            // 全局；面板可覆盖
  SetCollapsibleMotion(bool)          // P0 可瞬时；P1 动画
  SetWidth / SetHeight(float64)       // 可选固定盒；0 → 填父
  SetTheme(*Theme)
  SetAriaLabel(string)

  // 状态 / 回调
  PanelSizes() []float64              // 当前 px（需先 Layout）
  SetPanelSizesPx([]float64)          // 写回非受控 inner 或驱动受控
  OnResize / OnResizeStart / OnResizeEnd func([]float64)
  OnCollapse func(collapsed []bool, sizes []float64)
  CollapseAt(barIndex int, side start|end)

  // 挂树
  Node() core.Node
  ChromeNode() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Orientation | horizontal |
| Vertical | false |
| Lazy / DestroyOnHidden | false |
| Panel.Resizable | true |
| Panel.Collapsible | false |
| Panel size | 均分剩余空间（`autoPtgSizes`） |
| splitBarSize / trigger / spinner | 2 / 6 / 20 |
| 受控 | 任一面板 `size` 已 Set → 受控路径；否则 `defaultSize` + 拖动写 inner |

### 6.11 结构与绘制分层（实现提示）

```text
splitterRoot（自定义 Layout：按 PanelSizes 摆面板 + 条）
  ├─ panelHost[i]（内容；size=0 时 hidden / 可 destroy）
  └─ bar[i]（Draggable + 折叠 Pressable；布局盒 = triggerSize）
       └─ 可视 2px 轨 + 可选 20px spinner + 折叠图标
```

- 组合 `ui/primitive`（Box / Draggable / Pressable）+ `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default / 字段 / Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）；条盒 ≥ 可视条。  
- lazy 预览线可叠在 root 上画；折叠动画 P0 瞬时，Ticker 仅用于 loading 类控件（本控件无 loading）。  
- 键盘：条可聚焦；方向键微调为 P0 主路径（步进 ≈ 容器 1% 或 4px）。

**受控判定**：任一面板 `size` 已 Set 即进入受控路径（`defaultSize` 只作初值）；受控下拖拽/折叠只发回调，不写内部 `sizes`。

```text
非受控：drag ──► 内 sizes 更新 + onResize ──► release ──► onResizeEnd
受控：  drag ──► onResize(sizes')（内部不动）──► 父调 SetPanelSizesPx ──► 下次 Layout 生效
        release ──► onResizeEnd（同上，须父回写才稳定）
```

### 6.12 完成定义（DoD）

同时满足即可宣布 **Splitter 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/splitter.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Splitter 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
