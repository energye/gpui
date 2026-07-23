# Alert 警告提示
> 来源：[Ant Design 6.5.x Alert](https://ant.design/components/alert)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：警告提示，展现需要关注的信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
