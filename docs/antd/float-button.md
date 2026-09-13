# FloatButton 悬浮按钮
> 来源：[Ant Design 6.5.x FloatButton](https://ant.design/components/float-button)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：悬浮于页面上方的按钮。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

悬浮于页面上方的按钮。

**FloatButton** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 类型 | type 预设外观 |
| 形状 | 复现「形状」视觉与布局 |
| 描述 | 复现「描述」视觉与布局 |
| 含有气泡卡片的悬浮按钮 | card 风格容器 |
| 浮动按钮组 | 复现「浮动按钮组」视觉与布局 |
| 菜单模式 | 复现「菜单模式」视觉与布局 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 弹出方向 | 复现「弹出方向」视觉与布局 |
| 可拖拽 | 复现「可拖拽」视觉与布局 |
| 回到顶部 | 复现「回到顶部」视觉与布局 |
| 徽标数 | Badge 叠加 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `icon`

- **说明**：自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `content`

- **说明**：文字及其它内容
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：请使用 `content` 代替
- **类型**：ReactNode
- **默认值**：-

#### `type`

- **说明**：设置按钮类型
- **类型**：`default` | `primary`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `primary` | 主色强调 |

#### `shape`

- **说明**：设置按钮形状
- **类型**：`circle` | `square`
- **默认值**：`circle`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `circle` | 圆形 |
  | `square` | 方形 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：按钮是否禁用
- **类型**：boolean
- **默认值**：-
- **版本**：6.4.0

#### `placement`

- **说明**：自定义菜单弹出位置
- **类型**：`top` | `left` | `right` | `bottom`
- **默认值**：`top`
- **版本**：5.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `left` | 左侧 |
  | `right` | 右侧 |
  | `bottom` | 下方 |

#### `duration`

- **说明**：回到顶部所需时间（ms）
- **类型**：number
- **默认值**：450

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

- 用于网站上的全局功能；
- 无论浏览到何处都可以看见的按钮。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **形状**（`shape.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **描述**（`content.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **含有气泡卡片的悬浮按钮**（`tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **浮动按钮组**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **菜单模式**（`group-menu.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **受控模式**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **弹出方向**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **可拖拽**（`draggable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **回到顶部**（`back-top.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **徽标数**（`badge.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击按钮时的回调 |
| `open` | 受控显隐 | 受控展开，需配合 trigger 一起使用 |
| `onOpenChange` | 显隐变化 | 展开收起时的回调，需配合 trigger 一起使用 |
| `disabled` | 禁用 | 按钮是否禁用 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 类型 | `type.tsx` | 否 |
| 形状 | `shape.tsx` | 否 |
| 描述 | `content.tsx` | 否 |
| 含有气泡卡片的悬浮按钮 | `tooltip.tsx` | 否 |
| 浮动按钮组 | `group.tsx` | 否 |
| 菜单模式 | `group-menu.tsx` | 否 |
| 受控模式 | `controlled.tsx` | 否 |
| 弹出方向 | `placement.tsx` | 否 |
| 可拖拽 | `draggable.tsx` | 否 |
| 回到顶部 | `back-top.tsx` | 否 |
| 徽标数 | `badge.tsx` | 否 |
| 调试小圆点使用 | `badge-debug.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |

### 2.7 组合关系

- **依赖等级**：**L1 无依赖基础件**；单钮/组/BackTop 均不依赖组内其他 10 件，可先行实现。
- **等谁**：无；`tooltip` 字符串 P0 自绘，完整 Tooltip/xBadge 叠加为 P1，不同文件并行安全。
- **文件归属**：`ui/kit/float-button/`（只改自己文件，并行安全）。
- **ConfigProvider**：尺寸、主题、locale、默认 props。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

> 自 `antd@5.0.0` 版本开始提供该组件。

### 共同的 API

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| icon | 自定义图标 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | content | 文字及其它内容 | ReactNode | - | ~~description~~ | 请使用 `content` 代替 | ReactNode | - | tooltip | 气泡卡片的内容 | ReactNode \| [TooltipProps](/components/tooltip-cn#api) | - | TooltipProps: 5.25.0 | × |
| type | 设置按钮类型 | `default` \| `primary` | `default` | shape | 设置按钮形状 | `circle` \| `square` | `circle` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onClick | 点击按钮时的回调 | (event) => void | - | href | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | target | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | htmlType | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | 5.21.0 | × |
| badge | 带徽标数字的悬浮按钮（不支持 `status` 以及相关属性） | [BadgeProps](/components/badge-cn#api) | - | 5.4.0 | × |
| disabled | 按钮是否禁用 | boolean | - | 6.4.0 | × |

### FloatButton.Group

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| shape | 设置包含的 FloatButton 按钮形状 | `circle` \| `square` | `circle` | trigger | 触发方式（有触发方式为菜单模式） | `click` \| `hover` | - | open | 受控展开，需配合 trigger 一起使用 | boolean | - | closeIcon | 自定义关闭按钮 | React.ReactNode | `<CloseOutlined />` | placement | 自定义菜单弹出位置 | `top` \| `left` \| `right` \| `bottom` | `top` | 5.21.0 | × |
| onOpenChange | 展开收起时的回调，需配合 trigger 一起使用 | (open: boolean) => void | - | onClick | 点击按钮时的回调（仅在菜单模式中有效） | (event) => void | - | 5.3.0 | × |

### FloatButton.BackTop

| 参数             | 说明                               | 类型              | 默认值       | 版本 |
| ---------------- | ---------------------------------- | ----------------- | ------------ | ---- |
| duration         | 回到顶部所需时间（ms）             | number            | 450          |      |
| target           | 设置需要监听其滚动事件的元素       | () => HTMLElement | () => window |      |
| visibilityHeight | 滚动高度达到此参数值才出现 BackTop | number            | 400          |      |
| onClick          | 点击按钮的回调函数                 | () => void        | -            |      |

### 导入方式

```js
import { FloatButton } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `icon` | 自定义图标 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `content` | 文字及其它内容 | ReactNode | - | — |
| `description` | 请使用 `content` 代替 | ReactNode | - | — |
| `tooltip` | 气泡卡片的内容 | ReactNode \| [TooltipProps](/components/tooltip-cn#api) | - | TooltipProps: 5.25.0 |
| `type` | 设置按钮类型 | `default` \| `primary` | `default` | — |
| `shape` | 设置按钮形状 | `circle` \| `square` | `circle` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onClick` | 点击按钮时的回调 | (event) => void | - | — |
| `href` | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | — |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | — |
| `htmlType` | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | 5.21.0 |
| `badge` | 带徽标数字的悬浮按钮（不支持 `status` 以及相关属性） | [BadgeProps](/components/badge-cn#api) | - | 5.4.0 |
| `disabled` | 按钮是否禁用 | boolean | - | 6.4.0 |
| `trigger` | 触发方式（有触发方式为菜单模式） | `click` \| `hover` | - | — |
| `open` | 受控展开，需配合 trigger 一起使用 | boolean | - | — |
| `closeIcon` | 自定义关闭按钮 | React.ReactNode | `` | — |
| `placement` | 自定义菜单弹出位置 | `top` \| `left` \| `right` \| `bottom` | `top` | 5.21.0 |
| `onOpenChange` | 展开收起时的回调，需配合 trigger 一起使用 | (open: boolean) => void | - | — |
| `duration` | 回到顶部所需时间（ms） | number | 450 | — |
| `visibilityHeight` | 滚动高度达到此参数值才出现 BackTop | number | 400 | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **FloatButton** 的验收清单：

1. **配置面**：`type`/`shape`/`icon`/`content`/`tooltip`（字符串）/`disabled`/`loading`；Group `trigger`/`open`/`placement`/`closeIcon`；BackTop/draggable/badge 按 P1 分期。
2. **几何**：边长 40、circle r=20、square r=8、组间距 16（§6.2，±0.5px）。
3. **交互**：单击 1 次、禁用吞事件、菜单外点关闭、受控 open（§6.4 FB-S1…S13）。
4. **无障碍**：单钮/trigger 可聚焦命名，仅图标必须 AriaLabel（§6.6）。
5. **示例矩阵**：P0 按 §6.8（9 例）、余下按 P1 分期；与 §4 例数打架以 §6.8 为准。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/float-button
- 中文文档：https://ant.design/components/float-button-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/float-button
- 驱动 gpui kit：`float-button`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **FloatButton** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/float-button/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（FloatButton）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（FloatButton）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：悬浮于页面上方的按钮。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/float-button/style/index.ts`（`floatButtonSize = controlHeightLG` 等）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 边长 `floatButtonSize` | **40** | `controlHeightLG` |
| 图标边长 `floatButtonIconSize` | **18** | `fontSizeIcon×1.5`（回落 12×1.5） |
| content 字号 | **12** | `fontSizeSM` |
| 竖直 padding | **4** | `paddingXXS`（回落 `paddingXS`） |
| 圆角 `shape=circle` | **边长/2**（20） | 几何推导 |
| 圆角 `shape=square` | **8** | `borderRadiusLG` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |
| 贴边 insetInlineEnd / insetBlockEnd | **24 / 48** | `marginLG` / `marginXXL`（布局提示，非 OS 置顶） |
| Group 子钮间距 | **16** | `padding` |

> FloatButton **无** small/middle size 档；边长固定为 `controlHeightLG`（与 antd `size="large"` Button 同源）。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| `type=primary` 填充 / hover / active | `colorPrimary` + 变体 | solid |
| `type=default` 底 / 边 / 字 | `colorBgContainer` / `colorBorder` / `colorText` | outlined |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 浮层阴影 | `boxShadowSecondary`（若 Theme 有） | individual 悬浮 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**通用**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `icon` | 自定义图标 | ReactNode | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `content` | 文字及其它内容 | ReactNode | - |
| `tooltip` | 悬停气泡：字符串透传为 Tooltip 标题（P0）；完整 `TooltipProps`（位置/箭头/延时）为 P1；悬停示意、不抢主点击，disabled 时不弹 | ReactNode \ | [TooltipProps](/components/tooltip-cn#api) |
| `type` | 设置按钮类型 | `default` \ | `primary` |
| `shape` | 设置按钮形状 | `circle` \ | `square` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `onClick` | 点击按钮时的回调 | (event) => void | - |
| `href` | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - |
| `htmlType` | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://develop… | `submit` \ | `reset` \ |
| `badge` | 徽标叠加：`count/dot/overflowCount` 角标右上叠加、不抢主点击；不支持 `status/text/title/children`（源码 `FloatButton.tsx` omit） | [BadgeProps](/components/badge-cn#api) | - |
| `disabled` | 按钮是否禁用 | boolean | - |
| `trigger` | 触发方式（有触发方式为菜单模式） | `click` \ | `hover` |
| `open` | 受控展开，需配合 trigger 一起使用 | boolean | - |
| `closeIcon` | 自定义关闭按钮 | React.ReactNode | `<CloseOutlined />` |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
【单钮】
mount ──► default ──hover/press──► click(onClick)
disabled ──► 吞点击
badge 叠层不抢主点击

【Group 无 trigger】子钮常显
【Group + trigger】closed ──trigger──► open 子钮 + closeIcon
受控 open；placement 四向；`trigger=click` 时组外点击走 overlay 外点关闭（document capture 监听，不在组内则收起，源码 `FloatButtonGroup.tsx`）

【BackTop】scrollY < visibilityHeight ──► 隐藏
           scrollY ≥ 400 ──► 显示 ── click ──► 滚到顶
```

\*默认 type=default，shape=circle，边长 40。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| FB-S1 | 点击单钮 | onClick 一次 |
| FB-S2 | disabled | 不触发 |
| FB-S3 | type primary/default | 色正确 |
| FB-S4 | shape circle/square | circle 半径=边长/2；square 圆角=`borderRadiusLG`(8) |
| FB-S5 | 边长 | 40×40 |
| FB-S6 | Group trigger=click 开 | 子钮出现 |
| FB-S7 | 受控 open=false | 收起 |
| FB-S8 | placement=left | 子钮在左 |
| FB-S9 | BackTop scroll<400 | 不可见（**P1**） |
| FB-S10 | BackTop scroll≥400 点击 | 回顶（**P1**） |
| FB-S11 | badge count | 角标可见（**P1**） |
| FB-S12 | 仅图标 | 必须 AriaLabel |
| FB-S13 | loading=true（**kit 自增，antd 6.5 无此 API**） | spinner 示意 + 吞 `onClick` 防重复（Ticker 驱动） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 部位 | 规则 |
| --- | --- |
| 单钮 `type=default` | 容器底 `colorBgContainer` + 边框 `colorBorder`，字/图标 `colorText` |
| 单钮 `type=primary` | 实心 `colorPrimary` 底，反白字/图标；hover/active 走变体 |
| `shape=circle` | 圆形，半径=边长/2（20±0.5px） |
| `shape=square` | 方形，圆角=`borderRadiusLG`（8±0.5px） |
| `content` 文字钮 | 字号 12（`fontSizeSM`），竖直 padding 4 |
| Group 常显 | 子钮纵向排布，间距 16（`padding`） |
| Group 菜单 open | 子钮按 placement 四向展开，trigger 切 closeIcon |
| `tooltip` 字符串 | 悬停气泡文案（P0）；不抢主点击，disabled 时不弹 |
| `disabled` | 禁用底+禁用字；无 hover 高亮 |
| `loading`（kit 自增） | spinner 示意；吞重复 click（FB-S13） |


**动效：** 菜单展开/BackTop 入场 P0 可用瞬时切换，须尊重 reduced-motion。


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 单钮/trigger 为 `button`；子钮菜单容器可标 `menu`，子钮为 `menuitem` |
| 名称 | 单钮默认识别名 = `content` 文案；仅图标时**必须** `AriaLabel` |
| 焦点 | Tab 可聚焦单钮与 trigger；Focus ring 可见（§6.2） |
| 键盘 | Space/Enter 激活单钮；菜单 open 时 Esc 收起 |
| 禁用 | disabled 不触发激活；读屏可感知（平台支持时） |
| 徽标 | badge 角标纯装饰，不进 Tab 序 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 单钮点击/禁用/键盘/type/shape/content | **对等** | P0 L1 |
| 边长 40/圆角/色 Token（§6.2） | **对等** | P0 L2 |
| Group 菜单 trigger/placement/受控 open/外点关闭 | **对等**（外点关闭走 overlay 监听） | P0 L1 |
| `tooltip` 字符串 | **对等**；完整 `TooltipProps`（位置/箭头/延时）P1 | P0/P1 |
| `href`/`target` 跳转 | **映射**为打开 URL 回调，非 `<a>` 导航 | P1 |
| `htmlType` submit/reset | **映射**：由上层 Form 解释，只抛事件 | P1 |
| BackTop `target` 滚动宿主/`visibilityHeight` | **映射**宿主滚动观测 | P1 |
| `draggable` 拖拽 | **映射**宿主拖拽 | P1 |
| `badge.status` 等 Badge 全量 | 不支持（源码 omit），仅 count/dot | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `onClick` | 必须 |
| `disabled` | 必须 |
| `loading`（kit 自增，antd 无） | bool；Ticker spinner + 吞重复 click（见 FB-S13） |
| `type` | `default` \| `primary`（默认 **default**） |
| `shape` | `circle` \| `square`（默认 **circle**） |
| `icon` | 必须；无 icon 且无 content 时用默认图标 |
| `content` | 文字说明（antd `content`；旧 `description` 弃用） |
| `tooltip` | 悬停气泡文案（字符串 P0；完整 `TooltipProps` P1；不抢主点击，disabled 不弹） |
| `open` / `onOpenChange` | Group 菜单模式受控/非受控 |
| `trigger` | Group：`click` \| `hover`；无 trigger = 子钮常显 |
| `placement` | Group 菜单：`top` \| `left` \| `right` \| `bottom`（默认 top） |
| `closeIcon` | 菜单展开时 trigger 图标（默认 close） |
| Group 外点关闭 | **P0**：`trigger=click` 菜单开时组外点击收起（走 overlay 外点关闭，document capture，源码 `FloatButtonGroup.tsx`） |
| 官方主路径示例 | 基本、类型、形状、描述(content)、气泡、浮动按钮组、菜单模式、受控模式 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 **非 P1** 的 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| `badge` | 徽标叠加（`count/dot` 右上，不抢主点击；不支持 `status/text/title/children`） |
| `FloatButton.BackTop` | 回顶（visibilityHeight=400） |
| 可拖拽 `draggable` | 分期 |
| 弹出方向完整 demo 动画 | placement 四向 **行为** 属 P0；入场动画像素级属 P1 |
| `href` / `target` / `htmlType` | 桌面映射 |
| semantic classNames/styles 深度 | 分期 |
| ConfigProvider 全局默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

**15 例→P0/P1 剪裁对应表**（§2.4 全量；P0=§6.8 主路径 9 例，余下 4 例 P1，2 例 debug 不计）：

| 示例 | 裁剪 | 原因 |
| --- | --- | --- |
| 基本/类型/形状/描述/气泡卡片/浮动按钮组/菜单模式/受控模式/弹出方向 | P0 | 主路径行为 + gallery 必备（弹出方向仅四向行为，动画 P1） |
| 可拖拽 | P1 | 宿主拖拽映射分期 |
| 回到顶部 | P1 | BackTop 滚动宿主分期 |
| 徽标数 | P1 | badge 叠加分期 |
| 自定义语义结构的样式和类 | P1 | semantic 深度 |
| badge-debug/render-panel | 不计 | 内部调试/面板预览 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestFloatButton_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 FloatButton 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| FB-01 | L1 | NewFloatButton 默认创建 | type=default，shape=circle，可点；不崩溃 |
| FB-02 | L1 | 点击单钮 | onClick 一次 |
| FB-03 | L1 | disabled | 不触发 |
| FB-04 | L1 | type primary/default | 色正确（Token） |
| FB-05 | L1 | shape circle/square | circle r=size/2；square r=8 |
| FB-06 | L1 | 边长 | 40×40（±0.5） |
| FB-07 | L1 | Group trigger=click 开 | 子钮出现 |
| FB-08 | L1 | 受控 open=false | 收起 |
| FB-09 | L1 | placement=left | 子钮在左 |
| FB-10 | L1 **P1** | BackTop scroll<400 | 不可见 |
| FB-11 | L1 **P1** | BackTop scroll≥400 点击 | 回顶 |
| FB-12 | L1 **P1** | badge count | 角标可见 |
| FB-13 | L1 | 仅图标 | 必须 AriaLabel |
| FB-14 | L1 | 挂 `basic.tsx`（默认单钮，点击 1 次） | `onClick` 触发 1 次；边长 40±0.5px |
| FB-15 | L1 | 挂 `type.tsx`（default + primary 并存） | 两钮底色分别为容器底与 `colorPrimary`，字色分别为正文色与反白 |
| FB-16 | L1 | 挂 `shape.tsx`（circle + square） | circle 半径 20±0.5px，square 圆角 8±0.5px |
| FB-17 | L1 | 挂 `content.tsx`（文字钮） | content 文案可见，字号 12±0.5px |
| FB-18 | L1 | 挂 `tooltip.tsx`（悬停） | 悬停出现气泡文案；主点击仍触发 1 次 |
| FB-19 | L1 | 挂 `group.tsx`（无 trigger） | 全部子钮常显，纵向间距 16±0.5px |
| FB-20 | L1 | 挂 `group-menu.tsx`（trigger=click/hover） | 触发后子钮出现；组外点击收起，`onOpenChange` 各触发 1 次 |
| FB-21 | L1 | 挂 `controlled.tsx`（SetOpen(true/false)） | open 状态与设置值一致，子钮显隐跟随 |
| FB-22 | L2 | 读取 §6.2 关键尺寸 | 边长 40、square r=8（±0.5） |
| FB-23 | L2 | 默认皮颜色 | 走 Theme Token（非硬编码品牌色） |
| FB-24 | L2 | disabled 外观 | 禁用色；无 hover 高亮 |
| FB-25 | L1 | 键盘/焦点主路径 | Focus ring 可见；Enter/Space 激活 |
| FB-26 | L1 | loading=true | 不触发 onClick；有 spinner |
| FB-27 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| FB-28 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| FB-29 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

> **P0 测试范围：** FB-01…FB-09、FB-13…FB-26（L1/L2）。FB-10…12、FB-29 为 P1；FB-27/28 为 L3/L4。

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约，实现可微调命名但语义不可丢。

```text
// —— 单钮 ——
NewFloatButton() *FloatButton

SetType(ButtonType)              // default | primary（其它 Type 回落 default）
SetShape(FloatButtonShape)       // circle | square
SetIcon(name string)             // empty + empty content → 默认图标
SetContent(string)               // antd content；替代 description
SetTooltip(string)               // 悬停气泡；empty 清除
SetDisabled(bool)
SetLoading(bool)                 // Ticker spinner
SetOnClick(func())
SetAriaLabel(string)             // 仅图标必填
SetTheme(*Theme) / SetFace / Style
Node() core.Node
ChromeNode() core.Node
AttachTicker(*core.Tree)         // loading

// —— 组（FloatButton.Group）——
NewFloatButtonGroup(children ...*FloatButton) *FloatButtonGroup
Add(*FloatButton) / SetChildren(...*FloatButton)
SetTrigger(FloatButtonTrigger)   // none | click | hover
SetPlacement(FloatButtonPlacement) // top|bottom|left|right；默认 top
SetOpen(bool)                    // 受控
SetDefaultOpen(bool)             // 非受控初始
SetOnOpenChange(func(bool))
SetType / SetShape / SetIcon / SetCloseIcon(string)
SetDisabled(bool)
SetOnClick(func())               // trigger 点击（菜单模式）
Open() bool
Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Type | **default**（antd 6.5） |
| Shape | **circle** |
| Disabled / Loading | false |
| Icon | 空且无 content 时 → 默认图标（kit：`info`） |
| Content / Tooltip / AriaLabel | 空 |
| Group.Trigger | none（子钮常显） |
| Group.Placement | top |
| Group.Open | false（菜单模式）；无 trigger 时子钮始终可见 |
| 受控 open | 仅 `SetOpen` 后为受控 |

### 6.11 结构与绘制分层（实现提示）

```text
// —— 单钮 ——
Pressable（命中 40×40，hover/press/focus、Space/Enter）
  └─ Decorated（circle r=20 / square r=8，底/边走 type Token）
       └─ Row(gap)：Icon? · Content(12px)? · badge 角标叠层（不抢点击）

// —— 组 ——
FloatButtonGroup root（Column，gap=16）
  ├─ trigger 钮（菜单模式；开时切 closeIcon）
  └─ 子钮 × N（placement 四向排布；受控 open 驱动显隐）

// —— BackTop（P1）——
宿主滚动观测 + 单钮；scrollY≥400 显示，点击回顶（duration=450ms）
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **FloatButton 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（FB-01…09、FB-13…26）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字：边长 40、square r=8）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见；FB-27）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/float-button.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` FloatButton 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
