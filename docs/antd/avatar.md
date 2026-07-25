# Avatar 头像
> 来源：[Ant Design 6.5.x Avatar](https://ant.design/components/avatar)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用来代表用户或事物，支持图片、图标或字符展示。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用来代表用户或事物，支持图片、图标或字符展示。

**Avatar** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 类型 | type 预设外观 |
| 自动调整字符大小 | 不同 size 档位 |
| 带徽标的头像 | Badge 叠加 |
| Avatar.Group | 复现「Avatar.Group」视觉与布局 |
| 响应式尺寸 | 不同 size 档位的高宽/字号/内边距 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `alt`

- **说明**：图像无法显示时的替代文本
- **类型**：string
- **默认值**：-

#### `gap`

- **说明**：字符类型距离左右两侧边界单位像素
- **类型**：number
- **默认值**：4
- **版本**：4.3.0

#### `icon`

- **说明**：设置头像的自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `shape`

- **说明**：指定头像的形状
- **类型**：`circle` | `square`
- **默认值**：`circle`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `circle` | 圆形 |
  | `square` | 方形 |

#### `size`

- **说明**：设置头像的大小
- **类型**：number | `large` | `medium` | `small` | { xs: number, sm: number, ...}
- **默认值**：`medium`
- **版本**：4.7.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `src`

- **说明**：图片类头像的资源地址或者图片元素
- **类型**：string | ReactNode
- **默认值**：-
- **版本**：ReactNode: 4.8.0

#### `srcSet`

- **说明**：设置图片类头像响应式资源地址
- **类型**：string
- **默认值**：-

#### `draggable`

- **说明**：图片是否允许拖动
- **类型**：boolean | `'true'` | `'false'`
- **默认值**：true

#### `onError`

- **说明**：图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为
- **类型**：() => boolean
- **默认值**：-

#### `max`

- **说明**：设置最多显示相关配置
- **类型**：`{ count?: number; style?: CSSProperties; popover?: PopoverProps }`
- **默认值**：-
- **版本**：5.18.0

#### `maxCount`

- **说明**：已废弃，请使用 `max={{ count: number }}`
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

实现与 antd **Avatar** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自动调整字符大小**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **带徽标的头像**（`badge.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **Avatar.Group**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **响应式尺寸**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 类型 | `type.tsx` | 否 |
| 自动调整字符大小 | `dynamic.tsx` | 否 |
| 带徽标的头像 | `badge.tsx` | 否 |
| Avatar.Group | `group.tsx` | 否 |
| 隐藏情况下计算字符对齐 | `toggle-debug.tsx` | 是 |
| 响应式尺寸 | `responsive.tsx` | 否 |
| 图片不存在时 | `fallback.tsx` | 是 |
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

### Avatar

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| alt | 图像无法显示时的替代文本 | string | - | gap | 字符类型距离左右两侧边界单位像素 | number | 4 | 4.3.0 | × |
| icon | 设置头像的自定义图标 | ReactNode | - | shape | 指定头像的形状 | `circle` \| `square` | `circle` | size | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.7.0 | × |
| src | 图片类头像的资源地址或者图片元素 | string \| ReactNode | - | ReactNode: 4.8.0 | × |
| srcSet | 设置图片类头像响应式资源地址 | string | - | draggable | 图片是否允许拖动 | boolean \| `'true'` \| `'false'` | true | crossOrigin | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.17.0 | × |
| onError | 图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为 | () => boolean | - 
> Tip：你可以设置 `icon` 或 `children` 作为图片加载失败的默认 fallback 行为，优先级为 `icon` > `children`

### Avatar.Group 

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| max | 设置最多显示相关配置 | `{ count?: number; style?: CSSProperties; popover?: PopoverProps }` | - | 5.18.0 |
| ~~maxCount~~ | 已废弃，请使用 `max={{ count: number }}` | number | - | ~~maxPopoverTrigger~~ | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `hover` \| `focus` \| `click` | `hover` | size | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.8.0 |
| shape | 设置头像的形状 | `circle` \| `square` | `circle` | 5.8.0 |

### 导入方式

```js
import { Avatar } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `alt` | 图像无法显示时的替代文本 | string | - | — |
| `gap` | 字符类型距离左右两侧边界单位像素 | number | 4 | 4.3.0 |
| `icon` | 设置头像的自定义图标 | ReactNode | - | — |
| `shape` | 指定头像的形状 | `circle` \| `square` | `circle` | — |
| `size` | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.7.0 |
| `src` | 图片类头像的资源地址或者图片元素 | string \| ReactNode | - | ReactNode: 4.8.0 |
| `srcSet` | 设置图片类头像响应式资源地址 | string | - | — |
| `draggable` | 图片是否允许拖动 | boolean \| `'true'` \| `'false'` | true | — |
| `crossOrigin` | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.17.0 |
| `onError` | 图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为 | () => boolean | - | — |
| `max` | 设置最多显示相关配置 | `{ count?: number; style?: CSSProperties; popover?: PopoverProps }` | - | 5.18.0 |
| `maxCount` | 已废弃，请使用 `max={{ count: number }}` | number | - | — |
| `maxPopoverPlacement` | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `top` \| `bottom` | `top` | — |
| `maxPopoverTrigger` | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `hover` \| `focus` \| `click` | `hover` | — |
| `maxStyle` | 已废弃，请使用 `max={{ style: CSSProperties }}` | CSSProperties | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Avatar** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **6** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/avatar
- 中文文档：https://ant.design/components/avatar-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/avatar
- 驱动 gpui kit：`avatar`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Avatar** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/avatar/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Avatar）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Avatar）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用来代表用户或事物，支持图片、图标或字符展示。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/avatar/style/index.ts` → `prepareComponentToken` / `genBaseStyle` / `genGroupStyle`。

#### 6.2.1 尺寸档位（containerSize）

| size（antd） | kit 枚举 | 边长 | 字号 textFontSize | 图标字号 iconFontSize | square 圆角 |
| --- | --- | --- | --- | --- | --- |
| `small` | `AvatarSmall` | **24**（`controlHeightSM`） | **14**（`fontSize`） | **14**（`fontSize`） | **4**（`borderRadiusSM`） |
| `medium`（默认） | `AvatarMiddle` | **32**（`controlHeight`） | **14**（`fontSize`） | **18**（≈`(fontSizeLG+fontSizeXL)/2`） | **6**（`borderRadius`） |
| `large` | `AvatarLarge` | **40**（`controlHeightLG`） | **14**（`fontSize`） | **24**（≈`fontSizeHeading3`） | **8**（`borderRadiusLG`） |
| 自定义 number | `AvatarSizeCustom` + px | 自定义 | 自定义 number 时字号 **18**（非 icon）；icon 时 **size/2** | 同左 | 同档 radius 或自定义 |

#### 6.2.2 其它几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认 size | **32** | `containerSize` = `controlHeight` |
| `gap`（字符左右留白） | **4** | 产品默认；`SetGap` 覆盖 |
| 边框线宽 | **1** | `lineWidth`；默认色 transparent（组内见 groupBorder） |
| circle 圆角 | **size/2** | 圆形 |
| square 圆角 | 见上表 | `borderRadius` 档 |
| Group 重叠 | **−8** | `groupOverlapping` = `−marginXS`（antd seed marginXS=8；本库 DefaultAvatarGroupOverlapping） |
| Group 间距（popover 内） | **4** | `groupSpace` = `marginXXS`（库无 Token 时回落 4） |
| Group 边框色 | 容器底 | `groupBorderColor` ≈ `colorBgContainer` |
| Focus ring outset | ≈ **1.5px** 可见 | 可交互/可聚焦时；装饰默认 HitDefer 可不聚焦 |
| Loading 指示 | 边长 ≈ size×0.45 | Ticker 旋转环 |

#### 6.2.3 颜色 Token（语义）

| 用途 | Token / 回落 | 备注 |
| --- | --- | --- |
| 默认填充 `avatarBg` | `colorTextPlaceholder` ≈ **`colorTextQuaternary`**（A≈0.25 灰） | 非 primary；禁止默认用品牌蓝 |
| 默认字/图标 `avatarColor` | **`colorTextLightSolid` / `colorTextInverse`** | 浅色字 |
| 图片态底 | **透明** | `src` 成功时 |
| 边框 | transparent（单）/ 组内 `groupBorderColor` | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 适用者 |
| Style 覆盖 | `Style.Background` / `Style.Text` | 官方 type demo 自定义色 |

禁止硬编码品牌色（`#1677FF`）作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `children` / text | 字符头像内容 | string | — |
| `alt` | 图像无法显示时的替代文本 | string | — |
| `gap` | 字符类型距离左右两侧边界单位像素 | number | **4** |
| `icon` | 设置头像的自定义图标（kit：图标名或 Node） | string / Node | — |
| `shape` | 指定头像的形状 | `circle` \| `square` | **`circle`** |
| `size` | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| 响应式 map | **`medium`** |
| `src` | 图片类头像资源（kit：路径/URL 标签；可 `SetPixels` 像素） | string | — |
| `srcSet` | 响应式资源 | string | —（P1 宿主解码） |
| `draggable` | 图片是否允许拖动 | bool | true（桌面映射为标志） |
| `crossOrigin` | CORS | string | —（P1） |
| `onError` | 图片加载失败；返回 false 关闭默认 fallback | `func() bool` | — |
| `onClick` | 点击（可选交互） | `func()` | — |
| `loading` | 加载中 | bool | false |
| `disabled` | 禁用 | bool | false |
| Group `max.count` | 最多显示个数，溢出 `+N` | int | — |
| Group `max.style` | 溢出头 Style | Style | — |
| Group `size` / `shape` | 组内默认 size/shape 上下文 | 同 Avatar | medium / circle |

**内容优先级（antd）：**  
`src` 成功图 → 否则 `icon` → 否则 `children` 字（失败 fallback：`icon` > `children`）。

**配置优先级（通用）：** 受控 props > 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认（ConfigProvider 为 P1）。

### 6.4 交互状态机（L1）

```text
mount
  │
  ├─ src 非空 ── load OK ──► image 态（透明底 + 图）
  │                │ fail
  │                └─ onError?  ── return false ──► 保持 image 占位（调用方自处理）
  │                              └─ else ──► fallback: icon > children 字
  ├─ 无 src + icon ──► icon 态
  └─ 无 src + 无 icon ──► string 态（字符；gap 触发 scale 适配）

Group:
  children N, max.count=M (M<N) ──► 显示前 M 个 + 溢出 Avatar("+N-M")
  groupOverlapping 负边距叠压
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| AV-S1 | `src` 成功（或 `SetImageOK(true)` / 有 Pixels） | 图态；底透明 |
| AV-S2 | `src` 失败（`NotifyImageError` 或加载失败） | 调用 `onError`；非 false 则回退字/icon |
| AV-S3 | 字头像 | 显示字符；长串按 gap 缩小 |
| AV-S4 | `shape=square` | 圆角 = borderRadius 档（非 50%） |
| AV-S5 | `size=large` | 边长 40 |
| AV-S6 | Group `max.count=2` 且 3 个头 | 可见 2 + `+1` 溢出头 |
| AV-S7 | 默认 size | 边长 32（medium） |
| AV-S8 | `size=small` | 边长 24 |
| AV-S9 | 自定义 `size=64` | 边长 64；icon 字号 32 |
| AV-S10 | `SetLoading(true)` | 显示 spinner（Ticker）；可与内容叠加 |
| AV-S11 | `SetGap` 变化 | 重新计算字符 scale |
| AV-S12 | Group 继承 size/shape | 子项未显式设置时用组上下文 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 填充 `avatarBg`、字色 `avatarColor`；circle 50% / square radius 档 |
| image | 底透明；内容 cover 铺满 |
| icon | 图标居中；字号见 §6.2.1 |
| string | 字符居中；scale ≤ 1 使 `advance ≤ size − 2×gap` |
| hover/active/focus | 默认为展示控件（HitDefer）；若设 OnClick/可聚焦则 focus ring 可见 |
| disabled / loading | 禁用色；loading 旋转指示（Ticker） |
| 主题切换 | 色与间距随 Theme 更新 |
| Group | 子项负 margin 叠压；子边框 `groupBorderColor` |

**动效：** 字符 scale / 入场可瞬时；loading 旋转须可关或尊重 reduced-motion；P0 允许固定转速。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰默认 | 默认 `HitDefer`、非 Tab 序；`aria-hidden` 等价（Decorative） |
| 图 | `alt` 有则作为可访问名；无则装饰 |
| 有 OnClick / 可聚焦 | 角色 button 或等价；`SetAriaLabel` 可命名；Space/Enter 触发 |
| 溢出 `+N` | 文本即名称（如 `+1`） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 字符自动 scale（gap） | **对等**（布局后量字宽） | P0 L1 |
| Avatar.Group max + 溢出 | **对等**（`+N`） | P0 L1 |
| Group max.popover 展开隐藏项 | **近似**：P0 可只显示 `+N`；Popover 为 P1 | P0 溢出 / P1 popover |
| 真网络解码 `src` URL | **宿主**：P0 提供 `SetSrc` + `SetPixels` / `SetImageOK` / `NotifyImageError` 注入 | P0 状态机 / P1 自动 HTTP |
| `srcSet` / `crossOrigin` / `draggable` 浏览器语义 | 标志或 P1 | P1 |
| 响应式 size map（xs/sm/…） | **对等**：`SetResponsiveSize` + `SetBreakpoint` 注入当前档 | P0 L1 |
| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `size` small/medium/large + 自定义 number | 必须 |
| `shape` circle \| square | 必须 |
| `icon`（图标名 / Node） | 必须 |
| `children` / text 字符 + `gap` 自动 scale | 必须 |
| `src` 状态机 + `onError` + fallback | 必须（像素/OK 可注入） |
| `alt` / `SetAriaLabel` | 必须 |
| `loading`（Ticker） / `disabled` | 必须 |
| `Style` 背景/字色覆盖 | 必须 |
| Avatar.Group：`size`/`shape` 上下文、`max.count`、溢出 `+N`、叠压 | 必须 |
| 响应式 size map + 当前 breakpoint 注入 | 必须 |
| 官方主路径示例 | 基本、类型、自动调整字符大小、带徽标的头像、Avatar.Group、响应式尺寸 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| Group `max.popover` 展开隐藏头像 | 分期 |
| 真 HTTP 解码 `src` / `srcSet` / `crossOrigin` | 分期 |
| `draggable` 桌面拖拽 | 分期 |
| semantic classNames/styles 深度 | 分期 |
| ConfigProvider 全局 avatar 默认 | 分期 |
| 动画像素级 / debug 示例 / 官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestAvatar_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Avatar 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| AV-01 | L1 | NewAvatar 默认创建 | 不崩溃；shape=circle；size=middle；gap=4；非 disabled/loading |
| AV-02 | L1 | src 成功（`SetSrc`+`SetImageOK(true)` 或 `SetPixels`） | 图态；ContentMode=image |
| AV-03 | L1 | src 失败（`NotifyImageError`） | 调用 onError；回退字/icon |
| AV-04 | L1 | 字头像 `SetText("U")` | 显示字符；ContentMode=text |
| AV-05 | L1 | shape=square | 方；圆角≈6（middle） |
| AV-06 | L1 | size=large | 边长 40±0.5 |
| AV-07 | L1 | Group max=2 三个头 | 可见 2 + 溢出 `+1` |
| AV-08 | L1 | 默认 size | 边长 32±0.5 |
| AV-09 | L1 | 复现官方示例「基本」（`basic.tsx`） | circle/square × 多档 size 可布局 |
| AV-10 | L1 | 复现官方示例「类型」（`type.tsx`） | icon / 字 / src / Style 自定义色可布局 |
| AV-11 | L1 | 复现官方示例「自动调整字符大小」（`dynamic.tsx`） | 长串 scale&lt;1 或字宽适配 gap |
| AV-12 | L1 | 复现官方示例「带徽标的头像」（`badge.tsx`） | Badge 包 Avatar 可布局 |
| AV-13 | L1 | 复现官方示例「Avatar.Group」（`group.tsx`） | 组与 max 溢出可布局 |
| AV-14 | L1 | 复现官方示例「响应式尺寸」（`responsive.tsx`） | SetResponsiveSize + breakpoint 改边长 |
| AV-15 | L2 | 读取 §6.2 关键尺寸/间距 | small24 / middle32 / large40；字号/圆角表内数字 ±0.5 |
| AV-16 | L2 | 默认皮颜色 | 填充非 primary 硬编码；走 quaternary/inverse Token |
| AV-17 | L2 | disabled 外观 | 禁用色；无 hover 高亮 |
| AV-18 | L1 | OnClick + 键盘 | 可聚焦时 Space/Enter 触发；Focus ring 可开 |
| AV-19 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| AV-20 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| AV-21 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
// ── Avatar ──
NewAvatar(text string) *Avatar          // children 字；可空
NewAvatarIcon(iconName string) *Avatar  // icon 便捷构造

type AvatarSize int // AvatarSmall | AvatarMiddle | AvatarLarge | AvatarSizeCustom
type AvatarShape int // AvatarCircle | AvatarSquare

// 配置
SetText(s string)
SetIcon(name string)                 // 空清除；优先于字（无图时）
SetIconNode(n core.Node)             // 自定义图标节点
SetShape(AvatarShape)
SetSize(AvatarSize)                  // 预设档
SetSizePx(px float64)                // 自定义 number → AvatarSizeCustom
SetResponsiveSize(m AvatarResponsiveSize) // xs/sm/md/lg/xl/xxl
SetBreakpoint(bp string)             // 当前断点键，驱动响应式
SetGap(px float64)                   // 默认 4
SetSrc(src string)                   // 进入图态意图；重置 isImgExist
SetSrcSet(s string)                  // 存字段；P1 解码
SetPixels(w, h int, rgba []byte)     // 注入位图 → 图成功
SetImageOK(ok bool)                  // 宿主标记解码结果
NotifyImageError()                   // 触发 onError + 默认 fallback
SetAlt(s string)
SetDraggable(v bool)
SetCrossOrigin(s string)
SetOnError(fn func() bool)
SetOnClick(fn func())
SetLoading(v bool)                   // Ticker spinner
SetDisabled(v bool)
SetTheme(*core.Theme)
SetStyle(Style)                      // Background / Text 等
SetFace(text.Face)
SetAriaLabel(s string)
SetInteractive(v bool)               // 可聚焦 + 点击

// 查询
ContentMode() AvatarContent          // image | icon | text
ResolvedSize() float64
ChromeNode() core.Node
Node() core.Node
AttachTicker(*core.Tree)

// ── Avatar.Group ──
NewAvatarGroup(children ...*Avatar) *AvatarGroup
SetSize / SetSizePx / SetShape       // 组上下文
SetMaxCount(n int)                   // max.count；0=不限制
SetMaxStyle(Style)                   // 溢出头样式
Add / SetChildren
Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Shape | circle |
| Size | middle → 32 |
| Gap | 4 |
| Disabled / Loading | false |
| Draggable | true |
| 默认皮 | Token quaternary 底 + inverse 字 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Avatar
  Decorated root (fixed size × size; radius; bg; clip via overflow hidden equiv)
    └─ content: image | icon | scaled text
    └─ loading spinner? (overlay / replace)

AvatarGroup
  Row / inline-flex
    └─ Avatar… (marginInlineStart = groupOverlapping for i>0)
    └─ overflow Avatar("+N")?
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；值变更避免无谓整树销毁（Root 身份尽量稳定）。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- loading 旋转跟随 Host Tick；静止不挂 Ticker。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Avatar 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/avatar.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Avatar 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**本节相对原稿的修订（实现前）：**  
- **§6.2**：补全 antd `prepareComponentToken` 尺寸/字号/图标字号/组叠压硬数字；默认皮改为 placeholder/inverse 而非 primary。  
- **§6.3 / §6.4**：内容优先级、fallback、Group 规则 ID（AV-S8–S12）。  
- **§6.7 / §6.8**：响应式 size、src 宿主注入、Group popover 划入 P1；P0 清单对齐官方 6 个非 debug 示例。  
- **§6.9 / §6.10**：用例可测化；完整 Go API 契约（含 AvatarGroup）。
