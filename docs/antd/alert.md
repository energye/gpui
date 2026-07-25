# Alert 警告提示
> 来源：[Ant Design 6.5.x Alert](https://ant.design/components/alert)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：警告提示，展现需要关注的信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

警告提示，展现需要关注的信息。

**Alert** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 四种样式 | 复现「四种样式」视觉与布局 |
| 无边框 | bordered 网格线 |
| 可关闭的警告提示 | 复现「可关闭的警告提示」视觉与布局 |
| 含有辅助性文字介绍 | 复现「含有辅助性文字介绍」视觉与布局 |
| 图标 | icon 与文本混排 |
| 顶部公告 | 复现「顶部公告」视觉与布局 |
| 轮播的公告 | 复现「轮播的公告」视觉与布局 |
| 平滑地卸载 | 复现「平滑地卸载」视觉与布局 |
| React 错误处理 | 复现「React 错误处理」视觉与布局 |
| 操作 | 复现「操作」视觉与布局 |
| 自定义标题对齐 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `banner`

- **说明**：是否用作顶部公告
- **类型**：boolean
- **默认值**：false

#### `variant`

- **说明**：警告提示样式变体
- **类型**：`outlined` | `filled`
- **默认值**：`outlined`
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `filled` | 浅底填充 |

#### `classNames`

- **说明**：自定义组件内部各语义化结构的类名。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `closable`

- **说明**：可关闭配置
- **类型**：boolean | [ClosableType](#closabletype) & React.AriaAttributes
- **默认值**：`false`

#### `closeIcon`

- **说明**：（仅支持全局配置）自定义关闭图标
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：警告提示的辅助性文字介绍
- **类型**：ReactNode
- **默认值**：-

#### `errorIcon`

- **说明**：（仅支持全局配置）自定义错误图标
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义图标，`showIcon` 为 true 时有效
- **类型**：ReactNode
- **默认值**：-

#### `infoIcon`

- **说明**：（仅支持全局配置）自定义信息图标
- **类型**：ReactNode
- **默认值**：-

#### `showIcon`

- **说明**：是否显示辅助图标
- **类型**：boolean
- **默认值**：false，`banner` 模式下默认值为 true

#### `styles`

- **说明**：自定义组件内部各语义化结构的内联样式。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `successIcon`

- **说明**：（仅支持全局配置）自定义成功图标
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：警告提示内容
- **类型**：ReactNode
- **默认值**：-

#### `type`

- **说明**：指定警告提示的样式，有四种选择 `success`、`info`、`warning`、`error`
- **类型**：string
- **默认值**：`info`，`banner` 模式下默认值为 `warning`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `warning` | 警告橙语义 |
  | `error` | 错误红语义 |

#### `warningIcon`

- **说明**：（仅支持全局配置）自定义警告图标
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

- 当某个页面需要向用户显示警告的信息时。
- 非浮层的静态展现形式，始终展现，不会自动消失，用户可以点击关闭。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **四种样式**（`style.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **无边框**（`filled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可关闭的警告提示**（`closable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **含有辅助性文字介绍**（`description.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **图标**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **顶部公告**（`banner.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **轮播的公告**（`loop-banner.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **平滑地卸载**（`smooth-closed.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **React 错误处理**（`error-boundary.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **操作**（`action.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义标题对齐**（`custom-title-alignment.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `afterClose` | 关闭后 | 关闭动画结束后触发的回调函数，请使用 `closable.afterClose` 替换 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 四种样式 | `style.tsx` | 否 |
| 无边框 | `filled.tsx` | 否 |
| 可关闭的警告提示 | `closable.tsx` | 否 |
| 含有辅助性文字介绍 | `description.tsx` | 否 |
| 图标 | `icon.tsx` | 否 |
| 顶部公告 | `banner.tsx` | 否 |
| 轮播的公告 | `loop-banner.tsx` | 否 |
| 平滑地卸载 | `smooth-closed.tsx` | 否 |
| React 错误处理 | `error-boundary.tsx` | 否 |
| 自定义图标 | `custom-icon.tsx` | 是 |
| 操作 | `action.tsx` | 否 |
| 自定义标题对齐 | `custom-title-alignment.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| action | 自定义操作项 | ReactNode | - | ~~afterClose~~ | 关闭动画结束后触发的回调函数，请使用 `closable.afterClose` 替换 | () => void | - | banner | 是否用作顶部公告 | boolean | false | variant | 警告提示样式变体 | `outlined` \| `filled` | `outlined` | 6.4.0 | 6.4.0 |
| classNames | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | closable | 可关闭配置 | boolean \| [ClosableType](#closabletype) & React.AriaAttributes | `false` | closeIcon | （仅支持全局配置）自定义关闭图标 | ReactNode | - | × | 5.14.0 |
| description | 警告提示的辅助性文字介绍 | ReactNode | - | errorIcon | （仅支持全局配置）自定义错误图标 | ReactNode | - | × | 6.2.0 |
| icon | 自定义图标，`showIcon` 为 true 时有效 | ReactNode | - | infoIcon | （仅支持全局配置）自定义信息图标 | ReactNode | - | × | 6.2.0 |
| ~~message~~ | 警告提示内容，请使用 `title` 替换 | ReactNode | - | ~~onClose~~ | 关闭时触发的回调函数，请使用 `closable.onClose` 替换 | (e: MouseEvent) => void | - | ~~closeIcon~~ | 自定义关闭图标，请使用 `closable.closeIcon` 替代 | ReactNode | - | - | × |
| ~~closeText~~ | 自定义关闭文案，请使用 `closable.closeIcon` 替代 | ReactNode | - | - | × |
| showIcon | 是否显示辅助图标 | boolean | false，`banner` 模式下默认值为 true | styles | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | successIcon | （仅支持全局配置）自定义成功图标 | ReactNode | - | × | 6.2.0 |
| title | 警告提示内容 | ReactNode | - | type | 指定警告提示的样式，有四种选择 `success`、`info`、`warning`、`error` | string | `info`，`banner` 模式下默认值为 `warning` | warningIcon | （仅支持全局配置）自定义警告图标 | ReactNode | - | × | 6.2.0 |

### ClosableType

| 参数       | 说明                         | 类型                    | 默认值 | 版本 |
| ---------- | ---------------------------- | ----------------------- | ------ | ---- |
| afterClose | 关闭动画结束后触发的回调函数 | function                | -      | -    |
| closeIcon  | 自定义关闭图标               | ReactNode               | -      | -    |
| onClose    | 关闭时触发的回调函数         | (e: MouseEvent) => void | -      | -    |

### Alert.ErrorBoundary

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| description | 自定义错误内容，如果未指定会展示报错堆栈 | ReactNode | {{ error stack }} | title | 自定义错误标题，如果未指定会展示原生报错信息 | ReactNode | {{ error }} 
```js
import { Alert } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `action` | 自定义操作项 | ReactNode | - | — |
| `afterClose` | 关闭动画结束后触发的回调函数，请使用 `closable.afterClose` 替换 | () => void | - | — |
| `banner` | 是否用作顶部公告 | boolean | false | — |
| `variant` | 警告提示样式变体 | `outlined` \| `filled` | `outlined` | 6.4.0 |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `closable` | 可关闭配置 | boolean \| [ClosableType](#closabletype) & React.AriaAttributes | `false` | — |
| `closeIcon` | （仅支持全局配置）自定义关闭图标 | ReactNode | - | × |
| `description` | 警告提示的辅助性文字介绍 | ReactNode | - | — |
| `errorIcon` | （仅支持全局配置）自定义错误图标 | ReactNode | - | × |
| `icon` | 自定义图标，`showIcon` 为 true 时有效 | ReactNode | - | — |
| `infoIcon` | （仅支持全局配置）自定义信息图标 | ReactNode | - | × |
| `message` | 警告提示内容，请使用 `title` 替换 | ReactNode | - | — |
| `onClose` | 关闭时触发的回调函数，请使用 `closable.onClose` 替换 | (e: MouseEvent) => void | - | — |
| `closeText` | 自定义关闭文案，请使用 `closable.closeIcon` 替代 | ReactNode | - | - |
| `showIcon` | 是否显示辅助图标 | boolean | false，`banner` 模式下默认值为 true | — |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `successIcon` | （仅支持全局配置）自定义成功图标 | ReactNode | - | × |
| `title` | 警告提示内容 | ReactNode | - | — |
| `type` | 指定警告提示的样式，有四种选择 `success`、`info`、`warning`、`error` | string | `info`，`banner` 模式下默认值为 `warning` | — |
| `warningIcon` | （仅支持全局配置）自定义警告图标 | ReactNode | - | × |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Alert** 的验收清单：

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
- 官方文档：https://ant.design/components/alert
- 中文文档：https://ant.design/components/alert-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/alert
- 驱动 gpui kit：`alert`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Alert** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/alert/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Alert）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Alert）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：警告提示，展现需要关注的信息。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`fontSize=14`、`padding=16`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/alert/style/index.ts` `prepareComponentToken` / `genBaseStyle` / `genTypeStyle`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认内边距（无 description） | **8 × 12** | `defaultPadding` = `paddingContentVerticalSM(8)` + 固定水平 **12** |
| 有 description 内边距 | **16 × 24** | `withDescriptionPadding` = `paddingMD(16)` × `paddingContentHorizontalLG(24)` |
| 标题字号（无 description） | **14** | `fontSize` |
| 标题字号（有 description） | **16** | `fontSizeLG`；`colorTextHeading` |
| description 字号 | **14** | `fontSize`；色 `colorText` |
| 圆角 | **8** | `borderRadiusLG`（Alert 根用 LG，非 `borderRadius`） |
| 边框线宽 | **1** | `lineWidth`；`variant=filled` / `banner` → 无边框 |
| 图标字号（无 description） | **14** | ≈ `fontSize` / 内置 filled 图标 |
| 图标字号（有 description） | **24** | `withDescriptionIconSize` = `fontSizeHeading3` |
| 图标右边距（无 description） | **8** | `marginXS` |
| 图标右边距（有 description） | **12** | `marginSM` |
| action / close 左边距 | **8** | `marginXS` |
| Focus ring outset（close） | ≈ **1.5px** 可见 | 可调，必须可见 |
| `banner` | 圆角 **0**、边框 **0**、铺满宽 | `banner` 修饰 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| success 底 / 图标 / 边 | `colorSuccess` + 浅底/浅边（`colorSuccessBg` / `Border` 等价） | type=success |
| info 底 / 图标 / 边 | `colorPrimary` + `colorPrimaryBg` / 浅边 | type=info（antd colorInfo≈primary） |
| warning 底 / 图标 / 边 | `colorWarning` + 浅底/浅边 | type=warning；banner 默认 type |
| error 底 / 图标 / 边 | `colorError` + 浅底/浅边 | type=error |
| 标题字 | `colorText`（heading 语义） | 有/无 description 均 |
| description 字 | `colorText` | 非 secondary（antd 正文色） |
| close 图标 | `colorTextTertiary` / hover 更深 | 可交互 |
| `variant=filled` | 同 type 底色，**边框透明** | 6.4.0 |
| `variant=outlined` | type 底 + type 边框色 | 默认 |

禁止硬编码品牌色作为唯一默认皮；浅底/浅边可用 `lighten(语义色)` 回落，但语义色本身须读 Theme。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `action` | 自定义操作项 | ReactNode | - |
| `banner` | 是否用作顶部公告 | boolean | false |
| `variant` | 样式变体 | `outlined` \| `filled` | `outlined` |
| `closable` | 可关闭；对象形态含 `onClose` / `afterClose` / `closeIcon` / aria | boolean \| ClosableType | false |
| `description` | 辅助性文字介绍 | ReactNode | - |
| `icon` | 自定义图标（`showIcon` 有效时） | ReactNode | 按 type 默认图标 |
| `showIcon` | 是否显示辅助图标 | boolean | false；**banner 未显式设时 true** |
| `title` | 警告提示内容（替换已弃用 `message`） | ReactNode | - |
| `type` | `success` \| `info` \| `warning` \| `error` | string | `info`；**banner 未显式设时 `warning`** |

**P1 / 全局配置（本阶段不做实现强制）：** `classNames` / `styles` 语义钩子、`ConfigProvider` 的 `closeIcon` / `*Icon` 全局、ErrorBoundary、平滑卸载动画像素级。

**配置优先级：** 显式 props > 组件默认（含 banner 对 type/showIcon 的默认）> ConfigProvider 全局默认。

**弃用兼容（kit）：** `message` → `title`；顶层 `onClose` / `afterClose` / `closeIcon` → `closable.*`（可保留顶层 setter 作糖）。

### 6.4 交互状态机（L1）

```text
mount ──► type 语义色 + title + description?
         │
         ├─ showIcon ──► 默认/自定义图标
         ├─ banner ──► 无边框、直角、默认 type=warning、默认 showIcon
         ├─ variant=filled ──► 无描边
         └─ action ──► 右侧操作槽
closable ──► 点关闭 ──► onClose ──► Hidden(不可见) ──► afterClose（P0 瞬时，等同关闭后）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| ALT-S1 | type=success/info/warning/error | 四套语义底/边/图标色 |
| ALT-S2 | closable 点关闭 | 触发 onClose；随后不可见（Hidden） |
| ALT-S3 | description | 双行：title(LG) + description |
| ALT-S4 | showIcon | 图标可见；自定义 icon 优先 |
| ALT-S5 | banner | 直角、无边、默认 warning + showIcon |
| ALT-S6 | action 区 | 可挂 Button 等节点 |
| ALT-S7 | variant=filled | 边框宽 0 / 透明 |
| ALT-S8 | onClose PreventDefault（若提供） | 可阻止隐藏（与 Tag 一致可选） |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token；outlined 有 type 边 |
| filled | 同底色、无边 |
| banner | 无边、radius=0；常作顶栏宽铺满 |
| with-description | padding 16×24；图标 24；title 16 |
| close hover/focus | close 可点；focus ring 可见 |
| 主题切换 | 色与间距随 Theme 更新 |

**动效：** 关闭 leave 动画 P1；P0 **瞬时隐藏**。轮播公告（loop-banner）标题可走自定义 `TitleNode` + Ticker 位移，不强制像素级 marquee。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根节点 `role=alert`（或等价 Label/Role） |
| 关闭 | close 为可聚焦 button；默认可键盘激活；`aria-label` 默认 "Close" |
| 不抢焦点 | 展示时不自动抢焦点 |

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
| `type` | success / info / warning / error；banner 默认 warning |
| `variant` | outlined（默认）/ filled |
| `title` | 主文案；`TitleNode` 可自定义（轮播公告） |
| `description` | 辅助文案；双行结构 |
| `showIcon` / `icon` | 默认 false；banner 默认 true；可自定义 icon |
| `banner` | 顶栏样式（无边、直角） |
| `closable` + `onClose` / `afterClose` | 关闭；P0 瞬时隐藏 |
| `action` | 操作槽（可挂 Button） |
| 官方主路径示例 | 基本、四种样式、无边框、可关闭、description、图标、顶部公告、轮播的公告 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | role=alert + close 可操作 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| 关闭 leave 动画像素级 / smooth-closed | 分期 |
| ConfigProvider 全局 closeIcon / *Icon | 分期 |
| ErrorBoundary | React 专用，桌面映射弱 |
| 自定义标题对齐 / style-class 语义 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 完整 action.tsx 矩阵（多按钮列布局精修） | 槽位 P0 已有；复杂矩阵 P1 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestAlert_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1/L3/L4 标记）全部通过** 才可宣称 Alert 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| ALT-01 | L1 | NewAlert 默认创建 | 不崩溃；type=info、variant=outlined、showIcon=false、closable=false |
| ALT-02 | L1 | type=success/info/warning/error | 四套语义底/边/图标色互异 |
| ALT-03 | L1 | closable 点关闭 | onClose；Hidden / Visible=false；afterClose 触发 |
| ALT-04 | L1 | description | 双行结构（title + description 节点） |
| ALT-05 | L1 | showIcon | 图标可见 |
| ALT-06 | L1 | banner | radius=0、无边；未 SetType 时 type=warning；未 SetShowIcon 时 showIcon |
| ALT-07 | L1 | action 区 | 可挂 Button 节点 |
| ALT-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | title + type=success 可布局 |
| ALT-09 | L1 | 复现官方示例「四种样式」（`style.tsx`） | 四 type 均布局 |
| ALT-10 | L1 | 复现官方示例「无边框」（`filled.tsx`） | variant=filled → 无边框 |
| ALT-11 | L1 | 复现官方示例「可关闭的警告提示」（`closable.tsx`） | 四 type + closable 可关 |
| ALT-12 | L1 | 复现官方示例「含有辅助性文字介绍」（`description.tsx`） | 四 type + description |
| ALT-13 | L1 | 复现官方示例「图标」（`icon.tsx`） | showIcon ± description ± closable |
| ALT-14 | L1 | 复现官方示例「顶部公告」（`banner.tsx`） | banner 组合 |
| ALT-15 | L1 | 复现官方示例「轮播的公告」（`loop-banner.tsx`） | banner + TitleNode/长标题可布局 |
| ALT-16 | L2 | 读取 §6.2 关键尺寸/间距 | pad 8×12 / 有 desc 16×24；radius 8；字号 14/16；容差 ±0.5 |
| ALT-17 | L2 | 默认皮颜色 | 语义色走 Theme；非唯一硬编码品牌皮 |
| ALT-18 | L2 | disabled 外观（适用者） | **N/A**：antd Alert 无 disabled；用例断言不适用即可 |
| ALT-19 | L1 | 键盘/焦点主路径（适用者） | close 可聚焦；Enter/Space 可触发关闭 |
| ALT-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| ALT-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| ALT-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
NewAlert(title string) *Alert

// 内容
SetTitle(string) / SetTitleNode(core.Node)
SetMessage(string)            // 兼容别名 → SetTitle（antd message 已弃用）
SetDescription(string) / SetDescriptionNode(core.Node)

// 类型与变体
SetType(AlertType|string)     // info|success|warning|error
SetVariant(AlertVariant)      // outlined|filled
SetBanner(bool)
SetShowIcon(bool)
SetIcon(name string) / SetIconNode(core.Node)

// 关闭
SetClosable(bool)
SetCloseIcon(core.Node)
SetOnClose(func()) / OnClose(func(*AlertCloseEvent))  // 支持 PreventDefault
SetAfterClose(func())
CloseAria / SetCloseAria(string)

// 操作
SetAction(core.Node)

// 主题 / a11y / 挂树
SetTheme(*Theme) · SetFace · SetStyle · SetAriaLabel
Node() · ChromeNode() · CloseNode() · Visible() · IconVisible()
// L2 只读：FontSize / TitleFontSize / PadH / PadV / Radius / LineWidth /
//          Background / IconColor / BorderColor / HasBorder
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Type | `info`；**banner 且未显式 SetType → `warning`** |
| Variant | `outlined` |
| ShowIcon | `false`；**banner 且未显式 SetShowIcon → `true`** |
| Closable | false |
| Banner | false |
| Title | NewAlert 入参 |
| Description / Action / Icon | 空 |
| 关闭后 | Hidden=true（瞬时；afterClose 同步触发） |

### 6.11 结构与绘制分层（实现提示）

```text
Decorated Root  (role=alert)
  └─ Row  (CrossCenter | CrossStart when description)
       ├─ Icon?                 // showIcon
       ├─ Flexible(1) Section   // Column(title, description?)
       ├─ Action?               // 操作槽
       └─ Close Pressable?      // closable
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default / 字段 / Token；禁止裸 magic padding。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 关闭动画 P1；P0 瞬时 ClearChildren + HitTransparent。  
- 轮播标题：`TitleNode` + 可选 Ticker（gallery/loop-banner）。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Alert 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/alert.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Alert 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
