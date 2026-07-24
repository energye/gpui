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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

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

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 边长 floatButtonSize | **40** | controlHeightLG |
| 边长 | **40** | controlHeightLG |
| 贴边 right/bottom | **24 / 48** | marginLG / marginXXL |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**通用**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `icon` | 自定义图标 | ReactNode | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `content` | 文字及其它内容 | ReactNode | - |
| `tooltip` | 气泡卡片的内容 | ReactNode \ | [TooltipProps](/components/tooltip-cn#api) |
| `type` | 设置按钮类型 | `default` \ | `primary` |
| `shape` | 设置按钮形状 | `circle` \ | `square` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `onClick` | 点击按钮时的回调 | (event) => void | - |
| `href` | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - |
| `htmlType` | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://develop… | `submit` \ | `reset` \ |
| `badge` | 带徽标数字的悬浮按钮（不支持 `status` 以及相关属性） | [BadgeProps](/components/badge-cn#api) | - |
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
受控 open；placement 四向

【BackTop】scrollY < visibilityHeight ──► 隐藏
           scrollY ≥ 400 ──► 显示 ── click ──► 滚到顶
```

\*默认 type=default，shape=circle，边长 40。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| FB-S1 | 点击单钮 | onClick 一次 |
| FB-S2 | disabled | 不触发 |
| FB-S3 | type primary/default | 色正确 |
| FB-S4 | shape circle/square | 圆/方圆角 8 |
| FB-S5 | 边长 | 40×40 |
| FB-S6 | Group trigger=click 开 | 子钮出现 |
| FB-S7 | 受控 open=false | 收起 |
| FB-S8 | placement=left | 子钮在左 |
| FB-S9 | BackTop scroll<400 | 不可见 |
| FB-S10 | BackTop scroll≥400 点击 | 回顶 |
| FB-S11 | badge count | 角标可见 |
| FB-S12 | 仅图标 | 必须 AriaLabel |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | Token 默认皮 |
| hover / active | 可交互反馈 |
| focus | 可见 focus ring |
| checked/selected/active（适用者） | 主色强调 |
| disabled | 降对比；无 hover |
| loading | 指示器；防重复 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | button / checkbox / switch / radio 等与语义一致 |
| 名称 | 可交互必有名；仅图标必须 AriaLabel |
| 焦点 | Tab 可达；ring 可见 |
| 键盘 | Space/Enter 或方向键按角色 |
| 禁用 | 不可激活；读屏可感知（平台支持时） |

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
| `onClick` | 必须 |
| `disabled` | 必须 |
| `type` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `content` | 必须 |
| `placement` | 必须 |
| `trigger` | 必须 |
| `shape` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | 基本、类型、形状、描述、含有气泡卡片的悬浮按钮、浮动按钮组、菜单模式、受控模式 |
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
| 其余示例 | 弹出方向, 可拖拽, 回到顶部, 徽标数 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestFloatButton_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 FloatButton 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| FB-01 | L1 | NewFloatButton 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| FB-02 | L1 | 点击单钮 | onClick 一次 |
| FB-03 | L1 | disabled | 不触发 |
| FB-04 | L1 | type primary/default | 色正确 |
| FB-05 | L1 | shape circle/square | 圆/方圆角 8 |
| FB-06 | L1 | 边长 | 40×40 |
| FB-07 | L1 | Group trigger=click 开 | 子钮出现 |
| FB-08 | L1 | 受控 open=false | 收起 |
| FB-09 | L1 | placement=left | 子钮在左 |
| FB-10 | L1 | BackTop scroll<400 | 不可见 |
| FB-11 | L1 | BackTop scroll≥400 点击 | 回顶 |
| FB-12 | L1 | badge count | 角标可见 |
| FB-13 | L1 | 仅图标 | 必须 AriaLabel |
| FB-14 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-15 | L1 | 复现官方示例「类型」（`type.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-16 | L1 | 复现官方示例「形状」（`shape.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-17 | L1 | 复现官方示例「描述」（`content.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-18 | L1 | 复现官方示例「含有气泡卡片的悬浮按钮」（`tooltip.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-19 | L1 | 复现官方示例「浮动按钮组」（`group.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-20 | L1 | 复现官方示例「菜单模式」（`group-menu.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-21 | L1 | 复现官方示例「受控模式」（`controlled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FB-22 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| FB-23 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| FB-24 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| FB-25 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| FB-26 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| FB-27 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| FB-28 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewFloatButton(...) *FloatButton

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
Pressable
  └─ Decorated chrome
       └─ content (icon/label/indicator)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **FloatButton 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/float-button.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` FloatButton 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
