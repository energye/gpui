# Grid 栅格
> 来源：[Ant Design 6.5.x Grid](https://ant.design/components/grid)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：24 栅格系统。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

24 栅格系统。

**Grid** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基础栅格 | 复现「基础栅格」视觉与布局 |
| 区块间隔 | 复现「区块间隔」视觉与布局 |
| 左右偏移 | 复现「左右偏移」视觉与布局 |
| 栅格排序 | 复现「栅格排序」视觉与布局 |
| 排版 | 复现「排版」视觉与布局 |
| 对齐 | 复现「对齐」视觉与布局 |
| 排序 | 复现「排序」视觉与布局 |
| Flex 填充 | 复现「Flex 填充」视觉与布局 |
| 响应式布局 | 断点响应式 |
| Flex 响应式布局 | 断点响应式 |
| 其他属性的响应式 | 断点响应式 |
| 栅格配置器 | 复现「栅格配置器」视觉与布局 |
| useBreakpoint Hook | 复现「useBreakpoint Hook」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：垂直对齐方式
- **类型**：`top` | `middle` | `bottom` | `stretch` | `{[key in 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl' | 'xxxl']: 'top' | 'middle' | 'bottom' | 'stretch'}`
- **默认值**：`top`
- **版本**：object: 4.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `middle` | 中尺寸 |
  | `bottom` | 下方 |
  | `stretch` | 官方取值 `stretch` |
  | `{[key in 'xs` | 官方取值 `{[key in 'xs` |
  | `sm` | 官方取值 `sm` |
  | `md` | 官方取值 `md` |
  | `lg` | 官方取值 `lg` |
  | `xl` | 官方取值 `xl` |
  | `xxl` | 官方取值 `xxl` |
  | `xxxl']: 'top` | 官方取值 `xxxl']: 'top` |
  | `stretch'}` | 官方取值 `stretch'}` |

#### `gutter`

- **说明**：栅格间隔，可以写成[字符串CSS单位](https://developer.mozilla.org/zh-CN/docs/Web/CSS/CSS_Values_and_Units)或支持响应式的对象写法来设置水平间隔 { xs: 8, sm: 16, md: 24}。或者使用数组形式同时设置 `[水平间距, 垂直间距]`
- **类型**：number | string | object | array
- **默认值**：0
- **版本**：string: 5.28.0

#### `justify`

- **说明**：水平排列方式
- **类型**：`start` | `end` | `center` | `space-around` | `space-between` | `space-evenly` | `{[key in 'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl' | 'xxxl']: 'start' | 'end' | 'center' | 'space-around' | 'space-between' | 'space-evenly'}`
- **默认值**：`start`
- **版本**：object: 4.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |
  | `center` | 居中 |
  | `space-around` | 官方取值 `space-around` |
  | `space-between` | 官方取值 `space-between` |
  | `space-evenly` | 官方取值 `space-evenly` |
  | `{[key in 'xs` | 官方取值 `{[key in 'xs` |
  | `sm` | 官方取值 `sm` |
  | `md` | 官方取值 `md` |
  | `lg` | 官方取值 `lg` |
  | `xl` | 官方取值 `xl` |
  | `xxl` | 官方取值 `xxl` |
  | `xxxl']: 'start` | 官方取值 `xxxl']: 'start` |
  | `space-evenly'}` | 官方取值 `space-evenly'}` |

#### `wrap`

- **说明**：是否自动换行
- **类型**：boolean
- **默认值**：true
- **版本**：4.8.0

#### `flex`

- **说明**：flex 布局属性。数字类型对应 'flex: n n auto'；字符串类型直接透传（例如纯数字字符串 'n' 对应 'flex: n 1 0'）
- **类型**：string | number
- **默认值**：-

#### `offset`

- **说明**：栅格左侧的间隔格数，间隔内不可以有栅格
- **类型**：number
- **默认值**：0

#### `span`

- **说明**：栅格占位格数，为 0 时相当于 `display: none`
- **类型**：number
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

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

实现与 antd **Grid** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. **基础栅格**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **区块间隔**（`gutter.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **左右偏移**（`offset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **栅格排序**（`sort.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **排版**（`flex.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **对齐**（`flex-align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **排序**（`flex-order.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **Flex 填充**（`flex-stretch.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **响应式布局**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **Flex 响应式布局**（`responsive-flex.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **其他属性的响应式**（`responsive-more.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **栅格配置器**（`playground.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **useBreakpoint Hook**（`useBreakpoint.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基础栅格 | `basic.tsx` | 否 |
| 区块间隔 | `gutter.tsx` | 否 |
| 左右偏移 | `offset.tsx` | 否 |
| 栅格排序 | `sort.tsx` | 否 |
| 排版 | `flex.tsx` | 否 |
| 对齐 | `flex-align.tsx` | 否 |
| 排序 | `flex-order.tsx` | 否 |
| Flex 填充 | `flex-stretch.tsx` | 否 |
| 响应式布局 | `responsive.tsx` | 否 |
| Flex 响应式布局 | `responsive-flex.tsx` | 否 |
| 其他属性的响应式 | `responsive-more.tsx` | 否 |
| 栅格配置器 | `playground.tsx` | 否 |
| useBreakpoint Hook | `useBreakpoint.tsx` | 否 |

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

Ant Design 的布局组件若不能满足你的需求，你也可以直接使用社区的优秀布局组件：

- [react-flexbox-grid](https://roylee0704.github.io/react-flexbox-grid/)
- [react-blocks](https://github.com/whoisandy/react-blocks/)

### Row

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 垂直对齐方式 | `top` \| `middle` \| `bottom` \| `stretch` \| `{[key in 'xs' \| 'sm' \| 'md' \| 'lg' \| 'xl' \| 'xxl' \| 'xxxl']: 'top' \| 'middle' \| 'bottom' \| 'stretch'}` | `top` | object: 4.24.0 | × |
| gutter | 栅格间隔，可以写成[字符串CSS单位](https://developer.mozilla.org/zh-CN/docs/Web/CSS/CSS_Values_and_Units)或支持响应式的对象写法来设置水平间隔 { xs: 8, sm: 16, md: 24}。或者使用数组形式同时设置 `[水平间距, 垂直间距]` | number \| string \| object \| array | 0 | string: 5.28.0 | × |
| justify | 水平排列方式 | `start` \| `end` \| `center` \| `space-around` \| `space-between` \| `space-evenly` \| `{[key in 'xs' \| 'sm' \| 'md' \| 'lg' \| 'xl' \| 'xxl' \| 'xxxl']: 'start' \| 'end' \| 'center' \| 'space-around' \| 'space-between' \| 'space-evenly'}` | `start` | object: 4.24.0 | × |
| wrap | 是否自动换行 | boolean | true | 4.8.0 | × |

### Col

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| flex | flex 布局属性。数字类型对应 'flex: n n auto'；字符串类型直接透传（例如纯数字字符串 'n' 对应 'flex: n 1 0'） | string \| number | - | offset | 栅格左侧的间隔格数，间隔内不可以有栅格 | number | 0 | order | 栅格顺序 | number | 0 | pull | 栅格向左移动格数 | number | 0 | push | 栅格向右移动格数 | number | 0 | span | 栅格占位格数，为 0 时相当于 `display: none` | number | - | xs | `窗口宽度 < 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | sm | `窗口宽度 ≥ 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | md | `窗口宽度 ≥ 768px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | lg | `窗口宽度 ≥ 992px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | xl | `窗口宽度 ≥ 1200px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | xxl | `窗口宽度 ≥ 1600px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | xxxl | `窗口宽度 ≥ 1920px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | 6.3.0 | × |

您可以使用 [主题定制](/docs/react/customize-theme-cn) 修改 `screen[XS|SM|MD|LG|XL|XXL|XXXL]` 来修改断点值（自 5.1.0 起，[codesandbox demo](https://codesandbox.io/s/antd-reproduction-template-forked-dlq3r9?file=/index.js)）。

响应式栅格的断点扩展自 [BootStrap 4 的规则](https://getbootstrap.com/docs/4.0/layout/overview/#responsive-breakpoints)（不包含链接里 `occasionally` 的部分)。

### 导入方式

```js
import { Grid } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 垂直对齐方式 | `top` \| `middle` \| `bottom` \| `stretch` \| `{[key in 'xs' \| 'sm' \| 'md' \| 'lg' \| 'xl' \| 'xxl' \| 'xxxl']: 'top' \| 'middle' \| 'bottom' \| 'stretch'}` | `top` | object: 4.24.0 |
| `gutter` | 栅格间隔，可以写成[字符串CSS单位](https://developer.mozilla.org/zh-CN/docs/Web/CSS/CSS_Values_and_Units)或支持响应式的对象写法来设置水平间隔 { xs: 8, sm: 16, md: 24}。或者使用数组形式同时设置 `[水平间距, 垂直间距]` | number \| string \| object \| array | 0 | string: 5.28.0 |
| `justify` | 水平排列方式 | `start` \| `end` \| `center` \| `space-around` \| `space-between` \| `space-evenly` \| `{[key in 'xs' \| 'sm' \| 'md' \| 'lg' \| 'xl' \| 'xxl' \| 'xxxl']: 'start' \| 'end' \| 'center' \| 'space-around' \| 'space-between' \| 'space-evenly'}` | `start` | object: 4.24.0 |
| `wrap` | 是否自动换行 | boolean | true | 4.8.0 |
| `flex` | flex 布局属性。数字类型对应 'flex: n n auto'；字符串类型直接透传（例如纯数字字符串 'n' 对应 'flex: n 1 0'） | string \| number | - | — |
| `offset` | 栅格左侧的间隔格数，间隔内不可以有栅格 | number | 0 | — |
| `order` | 栅格顺序 | number | 0 | — |
| `pull` | 栅格向左移动格数 | number | 0 | — |
| `push` | 栅格向右移动格数 | number | 0 | — |
| `span` | 栅格占位格数，为 0 时相当于 `display: none` | number | - | — |
| `xs` | `窗口宽度 < 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `sm` | `窗口宽度 ≥ 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `md` | `窗口宽度 ≥ 768px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `lg` | `窗口宽度 ≥ 992px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `xl` | `窗口宽度 ≥ 1200px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `xxl` | `窗口宽度 ≥ 1600px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | — |
| `xxxl` | `窗口宽度 ≥ 1920px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \| object | - | 6.3.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Grid** 的验收清单：

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
- 官方文档：https://ant.design/components/grid
- 中文文档：https://ant.design/components/grid-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/grid
- 驱动 gpui kit：`grid`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Grid** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/grid/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Grid）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 布局参数驱动子项几何正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Grid）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：24 栅格系统。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 栅格列数 | **24** | gridColumns |
| 列数 | **24** | gridColumns |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**布局**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `align` | 垂直对齐方式 | `top` \ | `middle` \ |
| `gutter` | 栅格间隔，可以写成[字符串CSS单位](https://developer.mozilla.org/zh-CN/d… | number \ | string \ |
| `justify` | 水平排列方式 | `start` \ | `end` \ |
| `wrap` | 是否自动换行 | boolean | true |
| `flex` | flex 布局属性。数字类型对应 'flex: n n auto'；字符串类型直接透传（例如纯数字字符串 'n' … | string \ | number |
| `offset` | 栅格左侧的间隔格数，间隔内不可以有栅格 | number | 0 |
| `order` | 栅格顺序 | number | 0 |
| `pull` | 栅格向左移动格数 | number | 0 |
| `push` | 栅格向右移动格数 | number | 0 |
| `span` | 栅格占位格数，为 0 时相当于 `display: none` | number | - |
| `xs` | `窗口宽度 < 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |
| `sm` | `窗口宽度 ≥ 576px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |
| `md` | `窗口宽度 ≥ 768px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |
| `lg` | `窗口宽度 ≥ 992px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |
| `xl` | `窗口宽度 ≥ 1200px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |
| `xxl` | `窗口宽度 ≥ 1600px` 响应式栅格，可为栅格数或一个包含其他属性的对象 | number \ | object |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
Row + Col span/24
gutter 间距
断点 xs…xxl 改 span
```

\*24 栅格。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| GRD-S1 | span=12+12 | 各 50% |
| GRD-S2 | span=8×3 | 各约 33% |
| GRD-S3 | offset=6 | 左空 6 格 |
| GRD-S4 | gutter=16 | 列间隙 |
| GRD-S5 | 响应 md=12 xs=24 | 断点切换 |
| GRD-S6 | wrap | 换行行为 |
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
| 装饰分隔 | 纯装饰可 aria-hidden |
| 拖拽把手 | 可命名；键盘微调 P0/P1 按控件 |

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
| `align` | 必须 |
| `gutter` | 必须 |
| `justify` | 必须 |
| `wrap` | 必须 |
| `flex` | 必须 |
| `offset` | 必须 |
| `order` | 必须 |
| `pull` | 必须 |
| 官方主路径示例 | 基础栅格、区块间隔、左右偏移、栅格排序、排版、对齐、排序、Flex 填充 |
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
| 其余示例 | 响应式布局, Flex 响应式布局, 其他属性的响应式, 栅格配置器 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestGrid_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Grid 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| GRD-01 | L1 | NewGrid 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| GRD-02 | L1 | span=12+12 | 各 50% |
| GRD-03 | L1 | span=8×3 | 各约 33% |
| GRD-04 | L1 | offset=6 | 左空 6 格 |
| GRD-05 | L1 | gutter=16 | 列间隙 |
| GRD-06 | L1 | 响应 md=12 xs=24 | 断点切换 |
| GRD-07 | L1 | wrap | 换行行为 |
| GRD-08 | L1 | 复现官方示例「基础栅格」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-09 | L1 | 复现官方示例「区块间隔」（`gutter.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-10 | L1 | 复现官方示例「左右偏移」（`offset.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-11 | L1 | 复现官方示例「栅格排序」（`sort.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-12 | L1 | 复现官方示例「排版」（`flex.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-13 | L1 | 复现官方示例「对齐」（`flex-align.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-14 | L1 | 复现官方示例「排序」（`flex-order.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-15 | L1 | 复现官方示例「Flex 填充」（`flex-stretch.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| GRD-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| GRD-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| GRD-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| GRD-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| GRD-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| GRD-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| GRD-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewGrid(...) *Grid

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
Layout root
  └─ children with gap/span/handles
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Grid 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/grid.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Grid 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
