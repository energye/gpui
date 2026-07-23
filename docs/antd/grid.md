# Grid 栅格
> 来源：[Ant Design 6.5.x Grid](https://ant.design/components/grid)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：24 栅格系统。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
