# Notification 通知提醒框
> 来源：[Ant Design 6.5.x Notification](https://ant.design/components/notification)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：全局展示通知提醒信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

全局展示通知提醒信息。

**Notification** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| Hooks 调用（推荐） | 复现「Hooks 调用（推荐）」视觉与布局 |
| 自动关闭的延时 | 复现「自动关闭的延时」视觉与布局 |
| 带有图标的通知提醒框 | icon 与文本混排 |
| 自定义按钮 | 自定义渲染/插槽外观 |
| 自定义图标 | icon 与文本混排 |
| 位置 | placement 方位 |
| 更新消息内容 | 复现「更新消息内容」视觉与布局 |
| 堆叠 | 复现「堆叠」视觉与布局 |
| 显示进度条 | 进度条/圈 |
| 静态方法（不推荐） | 复现「静态方法（不推荐）」视觉与布局 |
| 自定义进度条颜色 | 语义色/预设色 |
| 自定义语义结构样式 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `actions`

- **说明**：自定义按钮组
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.24.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `closable`

- **说明**：是否显示右上角的关闭按钮
- **类型**：boolean | [ClosableType](#closabletype)
- **默认值**：true

#### `closeIcon`

- **说明**：自定义关闭图标
- **类型**：ReactNode
- **默认值**：true
- **版本**：5.7.0：设置为 null 或 false 时隐藏关闭按钮

#### `description`

- **说明**：通知提醒内容，必选
- **类型**：ReactNode
- **默认值**：-

#### `duration`

- **说明**：默认 4.5 秒后自动关闭，配置为 `0 | false` 则不会自动关闭
- **类型**：number | false
- **默认值**：4.5
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `0` | 官方取值 `0` |

#### `icon`

- **说明**：自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：通知提醒标题
- **类型**：ReactNode
- **默认值**：-
- **版本**：6.0.0

#### `message`

- **说明**：通知提醒标题，请使用 `title` 替换
- **类型**：ReactNode
- **默认值**：-

#### `placement`

- **说明**：弹出位置，可选 `top` | `topLeft` | `topRight` | `bottom` | `bottomLeft` | `bottomRight`
- **类型**：string
- **默认值**：`topRight`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |
  | `bottom` | 下方 |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |

#### `style`

- **说明**：自定义内联样式
- **类型**：[CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794)
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `props`

- **说明**：透传至通知 `div` 上的 props 对象，支持传入 `data-*` `aria-*` 或 `role` 作为对象的属性。需要注意的是，虽然在 TypeScript 类型中声明的类型支持传入 `data-*` 作为对象的属性，但目前只允许传入 `data-testid` 作为对象的属性。 详见 https://github.com/microsoft/TypeScript/issues/28960
- **类型**：Object
- **默认值**：-

#### `bottom`

- **说明**：消息从底部弹出时，距离底部的位置，单位像素
- **类型**：number
- **默认值**：24

#### `getContainer`

- **说明**：配置渲染节点的输出位置
- **类型**：() => HTMLNode
- **默认值**：() => document.body

#### `top`

- **说明**：消息从顶部弹出时，距离顶部的位置，单位像素
- **类型**：number
- **默认值**：24

#### `maxCount`

- **说明**：最大显示数，超过限制时，最早的消息会被自动关闭
- **类型**：number
- **默认值**：-
- **版本**：4.17.0

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

在系统四个角显示通知提醒信息。经常用于以下情况：

- 较为复杂的通知内容。
- 带有交互的通知，给出用户下一步的行动点。
- 系统主动推送。

### 2.2 核心功能（按官方示例拆解）

1. **Hooks 调用（推荐）**（`hooks.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **自动关闭的延时**（`duration.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **带有图标的通知提醒框**（`with-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义按钮**（`with-btn.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义图标**（`custom-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **更新消息内容**（`update.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **堆叠**（`stack.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **显示进度条**（`show-with-progress.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **静态方法（不推荐）**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义进度条颜色**（`progress-color.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构样式**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击通知时触发的回调函数 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| Hooks 调用（推荐） | `hooks.tsx` | 否 |
| 自动关闭的延时 | `duration.tsx` | 否 |
| 带有图标的通知提醒框 | `with-icon.tsx` | 否 |
| 自定义按钮 | `with-btn.tsx` | 否 |
| 自定义图标 | `custom-icon.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 更新消息内容 | `update.tsx` | 否 |
| 堆叠 | `stack.tsx` | 否 |
| 显示进度条 | `show-with-progress.tsx` | 否 |
| 静态方法（不推荐） | `basic.tsx` | 否 |
| 自定义进度条颜色 | `progress-color.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 自定义语义结构样式 | `style-class.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 方法

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

### 2.6 FAQ

## FAQ

### 为什么 notification 不能获取 context、redux 的内容和 ConfigProvider 的 `locale/prefixCls/theme` 等配置？ {#faq-context-redux}

直接调用 notification 方法，antd 会通过 `ReactDOM.render` 动态创建新的 React 实体。其 context 与当前代码所在 context 并不相同，因而无法获取 context 信息。

当你需要 context 信息（例如 ConfigProvider 配置的内容）时，可以通过 `notification.useNotification` 方法会返回 `api` 实体以及 `contextHolder` 节点。将其插入到你需要获取 context 位置即可：

```tsx
const [api, contextHolder] = notification.useNotification();

return (
  
    {/* contextHolder 在 Context1 内，它可以获得 Context1 的 context */}
    {contextHolder}
    
      {/* contextHolder 在 Context2 外，因而不会获得 Context2 的 context */}
    
  
);
```

**异同**：通过 hooks 创建的 `contextHolder` 必须插入到子元素节点中才会生效，当你不需要上下文信息时请直接调用。

> 可通过 [App 包裹组件](/components/app-cn) 简化 `useNotification` 等方法需要手动植入 contextHolder 的问题。

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

### 为什么 `style={{ width: 'max-content' }}` 在 Notification 上不生效？ {#faq-notification-width}

Notification 使用固定宽度布局，以保证堆叠卡片样式的一致性。因此不支持在通知外层节点上使用 `max-content`、`min-content`、`fit-content(...)` 这类 intrinsic width。

如果你需要调整 Notification 的整体宽度，建议通过组件 token `width` 来配置：

```tsx

  

```

如果你只是希望通知内容本身按内容宽度排布，可以在 `title` 或 `description` 里自行渲染 ReactNode，并把 `max-content` 放在内部节点上，而不是放在 Notification 根节点上。

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

- `notification.success(config)`
- `notification.error(config)`
- `notification.info(config)`
- `notification.warning(config)`
- `notification.open(config)`
- `notification.destroy(key?: String)`

config 参数如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 自定义按钮组 | ReactNode | - | 5.24.0 | × |
| ~~btn~~ | 自定义按钮组，请使用 `actions` 替换 | ReactNode | - | - | × |
| className | 自定义 CSS class | string | - | - | 5.7.0 |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | closable | 是否显示右上角的关闭按钮 | boolean \| [ClosableType](#closabletype) | true | - | × |
| closeIcon | 自定义关闭图标 | ReactNode | true | 5.7.0：设置为 null 或 false 时隐藏关闭按钮 | 5.14.0 |
| description | 通知提醒内容，必选 | ReactNode | - | - | × |
| duration | 默认 4.5 秒后自动关闭，配置为 `0 \| false` 则不会自动关闭 | number \| false | 4.5 | - | × |
| showProgress | 显示自动关闭通知框的进度条 | boolean | pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 | × |
| icon | 自定义图标 | ReactNode | - | - | × |
| key | 当前通知唯一标志 | string | - | - | × |
| title | 通知提醒标题 | ReactNode | - | 6.0.0 | × |
| ~~message~~ | 通知提醒标题，请使用 `title` 替换 | ReactNode | - | - | × |
| placement | 弹出位置，可选 `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight` | string | `topRight` | - | × |
| role | 供屏幕阅读器识别的通知内容语义，默认为 `alert`。此情况下屏幕阅读器会立即打断当前正在阅读的其他内容，转而阅读通知内容 | `alert \| status` | `alert` | 5.6.0 | × |
| style | 自定义内联样式 | [CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794) | - | - | 5.7.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onClick | 点击通知时触发的回调函数 | function | - | - | × |
| onClose | 当通知关闭时触发 | function | - | - | × |
| props | 透传至通知 `div` 上的 props 对象，支持传入 `data-*` `aria-*` 或 `role` 作为对象的属性。需要注意的是，虽然在 TypeScript 类型中声明的类型支持传入 `data-*` 作为对象的属性，但目前只允许传入 `data-testid` 作为对象的属性。 详见 https://github.com/microsoft/TypeScript/issues/28960 | Object | - | - | × |

- `notification.useNotification(config)`

config 参数如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| bottom | 消息从底部弹出时，距离底部的位置，单位像素 | number | 24 | closeIcon | 自定义关闭图标 | ReactNode | true | 5.7.0：设置为 null 或 false 时隐藏关闭按钮 | 5.14.0 |
| getContainer | 配置渲染节点的输出位置 | () => HTMLNode | () => document.body | placement | 弹出位置，可选 `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight` | string | `topRight` | showProgress | 显示自动关闭通知框的进度条 | boolean | pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 | × |
| rtl | 是否开启 RTL 模式 | boolean | false | stack | 堆叠模式，超过阈值时会将所有消息收起 | boolean \| `{ threshold: number }` | `{ threshold: 3 }` | 5.10.0 | × |
| top | 消息从顶部弹出时，距离顶部的位置，单位像素 | number | 24 | maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | 4.17.0 | × |

### ClosableType

| 参数      | 说明             | 类型      | 默认值    | 版本 |
| --------- | ---------------- | --------- | --------- | ---- |
| closeIcon | 自定义关闭图标   | ReactNode | undefined | -    |
| onClose   | 当通知关闭时触发 | function  | -         | -    |

### 全局配置

还提供了一个全局配置方法，在调用前提前配置，全局一次生效。

`notification.config(options)`

> 当你使用 `ConfigProvider` 进行全局化配置时，系统会默认自动开启 RTL 模式。(4.3.0+)
>
> 当你想单独使用，可通过如下设置开启 RTL 模式。

```js
notification.config({
  placement: 'bottomRight',
  bottom: 50,
  duration: 3,
  rtl: true,
});
```

#### notification.config

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| bottom | 消息从底部弹出时，距离底部的位置，单位像素 | number | 24 | duration | 默认自动关闭延时，单位秒 | number | 4.5 | pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 |
| getContainer | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLNode | () => document.body | rtl | 是否开启 RTL 模式 | boolean | false | maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | 4.17.0 |

### 导入方式

```js
import { Notification } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `actions` | 自定义按钮组 | ReactNode | - | 5.24.0 |
| `btn` | 自定义按钮组，请使用 `actions` 替换 | ReactNode | - | - |
| `className` | 自定义 CSS class | string | - | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `closable` | 是否显示右上角的关闭按钮 | boolean \| [ClosableType](#closabletype) | true | - |
| `closeIcon` | 自定义关闭图标 | ReactNode | true | 5.7.0：设置为 null 或 false 时隐藏关闭按钮 |
| `description` | 通知提醒内容，必选 | ReactNode | - | - |
| `duration` | 默认 4.5 秒后自动关闭，配置为 `0 \| false` 则不会自动关闭 | number \| false | 4.5 | - |
| `showProgress` | 显示自动关闭通知框的进度条 | boolean | — | 5.18.0 |
| `pauseOnHover` | 悬停时是否暂停计时器 | boolean | true | 5.18.0 |
| `icon` | 自定义图标 | ReactNode | - | - |
| `key` | 当前通知唯一标志 | string | - | - |
| `title` | 通知提醒标题 | ReactNode | - | 6.0.0 |
| `message` | 通知提醒标题，请使用 `title` 替换 | ReactNode | - | - |
| `placement` | 弹出位置，可选 `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight` | string | `topRight` | - |
| `role` | 供屏幕阅读器识别的通知内容语义，默认为 `alert`。此情况下屏幕阅读器会立即打断当前正在阅读的其他内容，转而阅读通知内容 | `alert \| status` | `alert` | 5.6.0 |
| `style` | 自定义内联样式 | [CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794) | - | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onClick` | 点击通知时触发的回调函数 | function | - | - |
| `onClose` | 当通知关闭时触发 | function | - | - |
| `props` | 透传至通知 `div` 上的 props 对象，支持传入 `data-*` `aria-*` 或 `role` 作为对象的属性。需要注意的是，虽然在 TypeScript 类型中声明的类型支持传入 `data-*` 作为对象的属性，但目前只允许传入 `data-testid` 作为对象的属性。 详见 https://github.com/microsoft/TypeScript/issues/28960 | Object | - | - |
| `bottom` | 消息从底部弹出时，距离底部的位置，单位像素 | number | 24 | — |
| `getContainer` | 配置渲染节点的输出位置 | () => HTMLNode | () => document.body | — |
| `rtl` | 是否开启 RTL 模式 | boolean | false | — |
| `stack` | 堆叠模式，超过阈值时会将所有消息收起 | boolean \| `{ threshold: number }` | `{ threshold: 3 }` | 5.10.0 |
| `top` | 消息从顶部弹出时，距离顶部的位置，单位像素 | number | 24 | — |
| `maxCount` | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | 4.17.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Notification** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **12** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/notification
- 中文文档：https://ant.design/components/notification-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/notification
- 驱动 gpui kit：`notification`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Notification** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/notification/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Notification）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Notification）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：全局展示通知提醒信息。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| duration 默认 | **4.5s** | API |
| 宽约 | **384** | 实现/token |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `actions` | 自定义按钮组 | ReactNode | - |
| `className` | 自定义 CSS class | string | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `closable` | 是否显示右上角的关闭按钮 | boolean \ | [ClosableType](#closabletype) |
| `closeIcon` | 自定义关闭图标 | ReactNode | true |
| `description` | 通知提醒内容，必选 | ReactNode | - |
| `duration` | 默认 4.5 秒后自动关闭，配置为 `0 \ | false` 则不会自动关闭 | number \ |
| `showProgress` | 显示自动关闭通知框的进度条 | boolean | — |
| `pauseOnHover` | 悬停时是否暂停计时器 | boolean | true |
| `icon` | 自定义图标 | ReactNode | - |
| `key` | 当前通知唯一标志 | string | - |
| `title` | 通知提醒标题 | ReactNode | - |
| `placement` | 弹出位置，可选 `top` \ | `topLeft` \ | `topRight` \ |
| `role` | 供屏幕阅读器识别的通知内容语义，默认为 `alert`。此情况下屏幕阅读器会立即打断当前正在阅读的其他内容，转而阅… | `alert \ | status` |
| `style` | 自定义内联样式 | [CSSProperties](https://github.com/De… | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
open ──► placement 角落显示
duration ──► 自动关（默认 4.5s）
key 更新 ──► 替换
btn 点击 ──► 业务回调
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| NTF-S1 | open | 可见 |
| NTF-S2 | placement=bottomLeft | 位置在左下 |
| NTF-S3 | duration 到期 | 消失 |
| NTF-S4 | key 更新 | 不新增一条 |
| NTF-S5 | 手动 close | onClose |
| NTF-S6 | 带 btn | 按钮可点 |
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
| 实时区域 | message/notification 用 status 语义等价 |
| 关闭 | 可关控件可操作 |
| 不抢焦点 | 轻提示默认不抢（Modal 例外） |

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
| `title` | 必须 |
| `placement` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | Hooks 调用（推荐）、自动关闭的延时、带有图标的通知提醒框、自定义按钮、自定义图标、位置、更新消息内容、堆叠 |
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
| 其余示例 | 显示进度条, 静态方法（不推荐）, 自定义进度条颜色, 自定义语义结构样式 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestNotification_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Notification 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| NTF-01 | L1 | NewNotification 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| NTF-02 | L1 | open | 可见 |
| NTF-03 | L1 | placement=bottomLeft | 位置在左下 |
| NTF-04 | L1 | duration 到期 | 消失 |
| NTF-05 | L1 | key 更新 | 不新增一条 |
| NTF-06 | L1 | 手动 close | onClose |
| NTF-07 | L1 | 带 btn | 按钮可点 |
| NTF-08 | L1 | 复现官方示例「Hooks 调用（推荐）」（`hooks.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-09 | L1 | 复现官方示例「自动关闭的延时」（`duration.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-10 | L1 | 复现官方示例「带有图标的通知提醒框」（`with-icon.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-11 | L1 | 复现官方示例「自定义按钮」（`with-btn.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-12 | L1 | 复现官方示例「自定义图标」（`custom-icon.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-13 | L1 | 复现官方示例「位置」（`placement.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-14 | L1 | 复现官方示例「更新消息内容」（`update.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-15 | L1 | 复现官方示例「堆叠」（`stack.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| NTF-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| NTF-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| NTF-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| NTF-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| NTF-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| NTF-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| NTF-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewNotification(...) *Notification

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
Host holder or inline
  └─ item (icon + content + close?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Notification 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/notification.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Notification 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
