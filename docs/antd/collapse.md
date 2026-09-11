# Collapse 折叠面板
> 来源：[Ant Design 6.5.x Collapse](https://ant.design/components/collapse)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：可以折叠/展开的内容区域。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

可以折叠/展开的内容区域。

**Collapse** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 折叠面板 | 复现「折叠面板」视觉与布局 |
| 面板尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 手风琴 | 复现「手风琴」视觉与布局 |
| 面板嵌套 | 复现「面板嵌套」视觉与布局 |
| 简洁风格 | 复现「简洁风格」视觉与布局 |
| 自定义面板 | 自定义渲染/插槽外观 |
| 隐藏箭头 | arrow 指示 |
| 额外节点 | 复现「额外节点」视觉与布局 |
| 幽灵折叠面板 | 透明/反色底 |
| 可折叠触发区域 | 复现「可折叠触发区域」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bordered`

- **说明**：带边框风格的折叠面板
- **类型**：boolean
- **默认值**：true

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `expandIcon`

- **说明**：自定义切换图标
- **类型**：(panelProps) => ReactNode
- **默认值**：-

#### `expandIconPlacement`

- **说明**：设置图标位置
- **类型**：`start` | `end`
- **默认值**：`start`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `expandIconPosition`

- **说明**：设置图标位置，请使用 `expandIconPlacement` 替换
- **类型**：`start` | `end`
- **默认值**：-
- **版本**：4.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `ghost`

- **说明**：使折叠面板透明且无边框
- **类型**：boolean
- **默认值**：false
- **版本**：4.4.0

#### `size`

- **说明**：设置折叠面板大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **版本**：5.2.0
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

#### `extra`

- **说明**：自定义渲染每个面板右上角的内容
- **类型**：ReactNode
- **默认值**：-

#### `label`

- **说明**：面板标题
- **类型**：ReactNode
- **默认值**：-

#### `header`

- **说明**：面板标题
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

- 对复杂区域进行分组和隐藏，保持页面的整洁。
- `手风琴` 是一种特殊的折叠面板，只允许单个内容区域展开。

### 2.2 核心功能（按官方示例拆解）

1. **折叠面板**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **面板尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **手风琴**（`accordion.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **面板嵌套**（`mix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **简洁风格**（`borderless.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义面板**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **隐藏箭头**（`noarrow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **额外节点**（`extra.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **幽灵折叠面板**（`ghost.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **可折叠触发区域**（`collapsible.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 切换面板的回调 |
| `items` | 数据化 items | 折叠项目内容 |
| `destroyOnHidden` | 隐藏销毁 | 销毁折叠隐藏的面板 |
| `activeKey` | 激活面板 | 当前激活 tab 面板的 key |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 折叠面板 | `basic.tsx` | 否 |
| 面板尺寸 | `size.tsx` | 否 |
| 手风琴 | `accordion.tsx` | 否 |
| 面板嵌套 | `mix.tsx` | 否 |
| 简洁风格 | `borderless.tsx` | 否 |
| 自定义面板 | `custom.tsx` | 否 |
| 隐藏箭头 | `noarrow.tsx` | 否 |
| 额外节点 | `extra.tsx` | 否 |
| 幽灵折叠面板 | `ghost.tsx` | 否 |
| 可折叠触发区域 | `collapsible.tsx` | 否 |
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

### Collapse

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| accordion | 手风琴模式 | boolean | false | activeKey | 当前激活 tab 面板的 key | string\[] \| string <br/> number\[] \| number | [手风琴模式](#collapse-demo-accordion)下默认第一个元素 | bordered | 带边框风格的折叠面板 | boolean | true | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | collapsible | 所有子面板是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 | × |
| defaultActiveKey | 初始化选中面板的 key | string\[] \| string<br/> number\[] \| number | - | ~~destroyInactivePanel~~ | 销毁折叠隐藏的面板 | boolean | false | destroyOnHidden | 销毁折叠隐藏的面板 | boolean | false | 5.25.0 | × |
| expandIcon | 自定义切换图标 | (panelProps) => ReactNode | - | expandIconPlacement | 设置图标位置 | `start` \| `end` | `start` | - | × |
| ~~expandIconPosition~~ | 设置图标位置，请使用 `expandIconPlacement` 替换 | `start` \| `end` | - | 4.21.0 | × |
| ghost | 使折叠面板透明且无边框 | boolean | false | 4.4.0 | × |
| size | 设置折叠面板大小 | `large` \| `medium` \| `small` | `medium` | 5.2.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onChange | 切换面板的回调 | function | - | items | 折叠项目内容 | [ItemType](#itemtype) | - | 5.6.0 | × |

### ItemType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| classNames | 语义化结构 className | [`Record<header \| body, string>`](#semantic-dom) | - | 5.21.0 |
| collapsible | 是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | extra | 自定义渲染每个面板右上角的内容 | ReactNode | - | key | 对应 activeKey | string \| number | - | showArrow | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true 
### Collapse.Panel

:::warning{title=已废弃}
版本 >= 5.6.0 时请使用 items 方式配置面板。
:::

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| collapsible | 是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 (icon: 4.24.0) |
| extra | 自定义渲染每个面板右上角的内容 | ReactNode | - | header | 面板标题 | ReactNode | - | showArrow | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true 
```js
import { Collapse } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `accordion` | 手风琴模式 | boolean | false | — |
| `activeKey` | 当前激活 tab 面板的 key | string\[] \| string  number\[] \| number | [手风琴模式](#collapse-demo-accordion)下默认第一个元素 | — |
| `bordered` | 带边框风格的折叠面板 | boolean | true | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `collapsible` | 所有子面板是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 |
| `defaultActiveKey` | 初始化选中面板的 key | string\[] \| string number\[] \| number | - | — |
| `destroyInactivePanel` | 销毁折叠隐藏的面板 | boolean | false | — |
| `destroyOnHidden` | 销毁折叠隐藏的面板 | boolean | false | 5.25.0 |
| `expandIcon` | 自定义切换图标 | (panelProps) => ReactNode | - | — |
| `expandIconPlacement` | 设置图标位置 | `start` \| `end` | `start` | - |
| `expandIconPosition` | 设置图标位置，请使用 `expandIconPlacement` 替换 | `start` \| `end` | - | 4.21.0 |
| `ghost` | 使折叠面板透明且无边框 | boolean | false | 4.4.0 |
| `size` | 设置折叠面板大小 | `large` \| `medium` \| `small` | `medium` | 5.2.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onChange` | 切换面板的回调 | function | - | — |
| `items` | 折叠项目内容 | [ItemType](#itemtype) | - | 5.6.0 |
| `children` | body 区域内容 | ReactNode | - | — |
| `extra` | 自定义渲染每个面板右上角的内容 | ReactNode | - | — |
| `forceRender` | 被隐藏时是否渲染 body 区域 DOM 结构 | boolean | false | — |
| `key` | 对应 activeKey | string \| number | - | — |
| `label` | 面板标题 | ReactNode | - | - |
| `showArrow` | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true | — |
| `header` | 面板标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Collapse** 的验收清单：

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
- 官方文档：https://ant.design/components/collapse
- 中文文档：https://ant.design/components/collapse-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/collapse
- 驱动 gpui kit：`collapse`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Collapse** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/collapse/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Collapse）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Collapse）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：可以折叠/展开的内容区域。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/collapse/style/index.ts` · `prepareComponentToken` / `genBaseStyle`（`collapsePanelBorderRadius = borderRadiusLG`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值（antd 种子） | Token / 来源 |
| --- | --- | --- |
| 字号 medium | **14** | `fontSize` |
| 字号 large | **16** | `fontSizeLG` |
| 面板圆角 | **8** | `borderRadiusLG`（`collapsePanelBorderRadius`） |
| 边框线宽 | **1** | `lineWidth` |
| header 内边距 medium | **12 × 16**（上下 × 左右） | `headerPadding` = `paddingSM` `padding`（组件常量；本库 seed `paddingSM` 映射见下） |
| header 内边距 small | **8 × 12**（上下 × 左右；左可 8） | `headerPaddingSM` |
| header 内边距 large | **16 × 24** | `headerPaddingLG` |
| content 内边距 medium | **16 × 16** | `contentPadding`（垂直 `padding`，水平固定 16） |
| content 内边距 small | **12** | `contentPaddingSM` = `paddingSM`（antd 12；kit 常量 12） |
| content 内边距 large | **24** | `contentPaddingLG` = `paddingLG` |
| borderless content 内边距 | **4 / 16 / 16**（上 / 左右 / 下） | `borderlessContentPadding` |
| 箭头区高度 medium | ≈ **fontHeight**（≈22） | `fontSize × lineHeight` |
| 箭头字号 | **12** | `fontSizeIcon` |
| 箭头与标题间距 | **12** | `marginSM`（antd；kit 可用常量 12） |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

> **kit 种子差异：** 本库 `TokenPaddingSM=8`、`TokenPaddingXS=4` 对齐 antd `paddingXS`/`paddingXXS` 阶梯，**不等于** antd Collapse 组件 token 里的 `paddingSM=12`。Collapse 组件度量以本表 **DefaultCollapse\*** 常量为准，再 `SizeOr` 读 Theme 覆盖。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 根 / 头背景（默认 bordered） | `colorFillAlter` → kit **`colorFillSecondary`** | antd `headerBg` |
| 面板 body 背景 | `colorBgContainer` | antd `contentBg` |
| 标题文字 | `colorText` / heading | header 文案 |
| body 文字 | `colorText` | |
| 边框 / 分割 | `colorBorder` | 根框 + item 底边 + body 顶边 |
| borderless body 背景 | transparent | `borderlessContentBg` |
| ghost 根/体 | transparent、无边框 | |
| 禁用头文字 | `colorDisabledText` | collapsible=disabled |
| Focus ring | `colorPrimary` / controlOutline | 键盘可见 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `accordion` | 手风琴：至多一个展开 | bool | false |
| `activeKey` | 受控展开 key（多选为列表；手风琴单 key） | `[]string` / string | — |
| `defaultActiveKey` | 非受控初始展开 | `[]string` / string | —（手风琴文档可默认首项，kit 不强制） |
| `bordered` | 带边框风格 | bool | true |
| `collapsible` | 全局触发区：`header` \| `icon` \| `disabled` | enum | header（整头可点） |
| `destroyOnHidden` | 收起时卸载 body | bool | false |
| `expandIcon` | 自定义箭头（`isActive`） | func | 默认 ▸/▾ 或 Right 旋转 |
| `expandIconPlacement` | 箭头 `start` \| `end` | enum | start |
| `ghost` | 透明无边框 | bool | false |
| `size` | `large` \| `medium` \| `small` | enum | medium（kit=`CollapseMiddle`） |
| `onChange` | 展开 key 变化 | `func([]string)` | — |
| `items` | 面板数据（现代 API） | `[]CollapseItem` | — |
| item.`key` | 对应 activeKey | string | 必填 |
| item.`label` / `header` | 面板标题 | string / Node | — |
| item.`children` | body 内容 | Node | — |
| item.`extra` | 头右侧额外节点（点击不切换） | Node | — |
| item.`showArrow` | 是否显示箭头（false 时 collapsible 不可为 icon） | bool | true |
| item.`collapsible` | 覆盖全局 collapsible | enum | 继承 |
| item.`forceRender` | 收起仍挂载 body | bool | false |
| `classNames` / `styles` | 语义钩子 | — | **P1** |

**配置优先级：** 受控 `activeKey` > 非受控 `defaultActiveKey` / 内部态 > 组件默认 > ConfigProvider（P1）。  
item 级 `collapsible` / `showArrow` 覆盖全局。

### 6.4 交互状态机（L1）

```text
activeKeys 集合（多选）或单 key（accordion）
  点触发区 ──► toggle key；onChange(keys)
  accordion=true ──► 至多保留一个 key
  collapsible=disabled ──► 不可切换（含键盘）
  collapsible=icon ──► 仅箭头可点（需 showArrow）
  collapsible=header ──► 标题+箭头可点；extra 不切换
  受控 activeKey ──► 点击只 onChange，不改内部，直到 SetActiveKey
  destroyOnHidden + 收起 ──► 卸载 body（forceRender 除外）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| COL-S1 | 展开一项 | body 可见；`onChange` 含该 key |
| COL-S2 | accordion 开第二项 | 第一项收起；keys 长度为 1 |
| COL-S3 | collapsible=disabled | 点击/键盘不切换 |
| COL-S4 | ghost | 根/体透明、无边框 |
| COL-S5 | bordered=false | 根无描边；item 保留底部分割 |
| COL-S6 | destroyOnHidden | 收起后 body 不在树（无 forceRender） |
| COL-S7 | 受控 activeKey | 外部 `SetActiveKey` 优先；点击不私自改 |
| COL-S8 | 键盘 | 可聚焦头；Enter/Space 切换 |
| COL-S9 | showArrow=false | 无箭头；不可 collapsible=icon |
| COL-S10 | expandIconPlacement=end | 箭头在标题逻辑尾侧 |
| COL-S11 | extra 点击 | 不触发折叠 |
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
| 根 | `Role=group`（或 region）+ 可选 `AriaLabel` |
| 面板头 | 可 Tab 聚焦（非 disabled）；`Role=button`；展开态可读（aria-expanded 映射 Label/State） |
| 键盘 | 聚焦头上 Enter/Space 切换（与 collapsible 一致） |
| extra | 不抢头的展开语义；独立可点节点自行处理 |

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
| `items` / item `key`·`label`·`children` | 现代数据驱动（兼容 `CollapsePanel`/`Header`/`Content` 别名） |
| `activeKey` / `defaultActiveKey` / `onChange` | 受控 + 非受控 |
| `accordion` | 至多一开 |
| `size` small\|medium\|large | 头/体 padding + large 字号 |
| `bordered` / `ghost` | 边框与幽灵皮 |
| `collapsible` header\|icon\|disabled（含 item 级） | 触发区；disabled 禁键盘 |
| `showArrow` / `expandIcon` / `expandIconPlacement` | 箭头显隐、自定义、起止侧 |
| `extra` | 右侧节点；点击不折叠 |
| `destroyOnHidden` / item `forceRender` | 收起卸载 |
| 官方主路径示例（gallery） | basic / size / accordion / mix / borderless / custom / noarrow / extra |
| 度量 §6.2 | Token / DefaultCollapse\* 断言 |

**口径：** `ghost` 明确在 P0 内（透明无边框皮，与 `bordered=false` 同页对比进 gallery，用例 COL-05 覆盖），P1 列表中的“幽灵折叠面板 gallery 整页”仅指独立整页 demo 的像素级打磨，不代表 ghost 本体推迟。
| a11y §6.6 | 头可聚焦 + 展开态可读 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| 展开/收起高度动画像素级 | P0 瞬时切换 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| ConfigProvider 全局默认 | 分期 |
| 其余示例 | 幽灵折叠面板 gallery 整页、可折叠触发区域三态 demo、自定义语义 style-class、_semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestCollapse_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Collapse 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| COL-01 | L1 | NewCollapse 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| COL-02 | L1 | 展开一项 | 内容可见；onChange |
| COL-03 | L1 | accordion 开第二项 | 第一项收起 |
| COL-04 | L1 | collapsible=disabled | 不可点 |
| COL-05 | L1 | ghost | 无边框背景弱 |
| COL-06 | L1 | bordered=false | 无边框 |
| COL-07 | L1 | destroyOnHidden | 收起卸载 |
| COL-08 | L1 | 受控 activeKey | 外部优先 |
| COL-09 | L1 | 键盘（适用） | 可激活头 |
| COL-10 | L1 | 复现官方示例「折叠面板」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-11 | L1 | 复现官方示例「面板尺寸」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-12 | L1 | 复现官方示例「手风琴」（`accordion.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-13 | L1 | 复现官方示例「面板嵌套」（`mix.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-14 | L1 | 复现官方示例「简洁风格」（`borderless.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-15 | L1 | 复现官方示例「自定义面板」（`custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-16 | L1 | 复现官方示例「隐藏箭头」（`noarrow.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-17 | L1 | 复现官方示例「额外节点」（`extra.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| COL-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| COL-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| COL-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| COL-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| COL-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| COL-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| COL-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
type CollapseItem struct {
  Key, Label string
  LabelNode, Children, Extra, ExpandIcon core.Node
  ShowArrow *bool          // nil → true
  Collapsible CollapseCollapsible // 0=inherit
  ForceRender bool
  // 兼容别名：Header≡Label，Content≡Children（旧 CollapsePanel）
}

type CollapseSize         // Middle | Small | Large   （antd medium|small|large）
type CollapseCollapsible  // Header | Icon | Disabled
type CollapseIconPlacement // Start | End

NewCollapse(items ...CollapseItem) *Collapse
// 兼容：CollapsePanel 类型别名 = CollapseItem；NewCollapse(panels...)

// 数据
SetItems(...CollapseItem) / Items()
// 展开
ActiveKeys() []string
SetActiveKey(keys ...string)          // 受控
SetActive(keys ...string)             // 非受控程序化（兼容旧名；不进 controlled）
SetDefaultActiveKey(keys ...string)
IsActive(key string) bool
// 配置
SetAccordion / SetBordered / SetGhost / SetSize / SetCollapsible
SetDestroyOnHidden / SetExpandIconPlacement
SetExpandIcon(func(isActive bool, item CollapseItem) core.Node)
// 回调
SetOnChange(func(keys []string))  // 字段 OnChange
// 主题 / a11y / 挂树
SetTheme / SetFace / SetStyle / SetAriaLabel
Node() core.Node   // 根身份在 toggle 后保持稳定（ClearChildren 复用 Root）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Accordion | false |
| Bordered | true |
| Ghost | false |
| Size | CollapseMiddle（medium） |
| Collapsible | CollapseCollapsibleHeader |
| ExpandIconPlacement | CollapseIconStart |
| DestroyOnHidden | false |
| ShowArrow（item） | true |
| ActiveKeys | 空（或 defaultActiveKey） |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Decorated root（边框/圆角/headerBg 或 ghost）
  └─ Column items
       └─ item Column
            ├─ header row（Pressable 或分区 icon/title）
            │    expandIcon? · title · extra?
            └─ panel body?（展开或 forceRender；destroyOnHidden 则收起卸）
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；**Root 指针稳定**（ClearChildren）。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 展开动效 P0 瞬时；Ticker 仅当 loading/自定动画需要。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Collapse 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/collapse.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Collapse 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
