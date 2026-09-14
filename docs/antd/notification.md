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
| `open` / `success` / `info` / `warning` / `error` | 类型打开 | 按类型打开一条（`type` sugar 带语义图标） |
| `title` / `description` | 主文案 | 标题 + 必选内容 |
| `duration` | 自动关闭 | 默认 4.5；`0` 常驻 |
| `key` | 同条更新 | 同 key 替换，不新增 |
| `onClose` | 关闭回调 | 关闭时触发 |
| `destroy` | 销毁 | `destroy(key?)` 关一条或清空 |
| `placement` | 弹出位置 | 6 方位，默认 `topRight` |
| `actions` | 按钮组 | 自定义按钮 |
| `stack` | 堆叠 | 默认阈值 3 |
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
| 语义结构调试 | `_semantic.tsx` | 是 |

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

- **依赖等级**：L3（浮层：四角堆叠池 + z-index 定位）。
- **等谁**：浮层定位（placement 独立队列）、Button（`actions` 按钮组）、Icon（类型图标）。
- **文件归属**：`ui/kit/notification/`。
- **组合**：经 App 上下文消费；ConfigProvider 下发 `placement`/`duration`/`maxCount` 全局默认。
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
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - |  | 6.0.0 |
| closable | 是否显示右上角的关闭按钮 | boolean \| [ClosableType](#closabletype) | true | - | × |
| closeIcon | 自定义关闭图标 | ReactNode | true | 5.7.0：设置为 null 或 false 时隐藏关闭按钮 | 5.14.0 |
| description | 通知提醒内容，必选 | ReactNode | - | - | × |
| duration | 默认 4.5 秒后自动关闭，配置为 `0 \| false` 则不会自动关闭 | number \| false | 4.5 | - | × |
| showProgress | 显示自动关闭通知框的进度条 | boolean |  | 5.18.0 | × |
| pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 | × |
| icon | 自定义图标 | ReactNode | - | - | × |
| key | 当前通知唯一标志 | string | - | - | × |
| title | 通知提醒标题 | ReactNode | - | 6.0.0 | × |
| ~~message~~ | 通知提醒标题，请使用 `title` 替换 | ReactNode | - | - | × |
| placement | 弹出位置，可选 `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight` | string | `topRight` | - | × |
| role | 供屏幕阅读器识别的通知内容语义，默认为 `alert`。此情况下屏幕阅读器会立即打断当前正在阅读的其他内容，转而阅读通知内容 | `alert \| status` | `alert` | 5.6.0 | × |
| style | 自定义内联样式 | [CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794) | - | - | 5.7.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - |  | 6.0.0 |
| onClick | 点击通知时触发的回调函数 | function | - | - | × |
| onClose | 当通知关闭时触发 | function | - | - | × |
| props | 透传至通知 `div` 上的 props 对象，支持传入 `data-*` `aria-*` 或 `role` 作为对象的属性。需要注意的是，虽然在 TypeScript 类型中声明的类型支持传入 `data-*` 作为对象的属性，但目前只允许传入 `data-testid` 作为对象的属性。 详见 https://github.com/microsoft/TypeScript/issues/28960 | Object | - | - | × |

- `notification.useNotification(config)`

config 参数如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| bottom | 消息从底部弹出时，距离底部的位置，单位像素 | number | 24 |  | × |
| closeIcon | 自定义关闭图标 | ReactNode | true | 5.7.0：设置为 null 或 false 时隐藏关闭按钮 | 5.14.0 |
| getContainer | 配置渲染节点的输出位置 | () => HTMLNode | () => document.body |  | × |
| placement | 弹出位置，可选 `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight` | string | `topRight` |  | × |
| showProgress | 显示自动关闭通知框的进度条 | boolean |  | 5.18.0 | × |
| pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 | × |
| rtl | 是否开启 RTL 模式 | boolean | false |  | × |
| stack | 堆叠模式，超过阈值时会将所有消息收起 | boolean \| `{ threshold: number }` | `{ threshold: 3 }` | 5.10.0 | × |
| top | 消息从顶部弹出时，距离顶部的位置，单位像素 | number | 24 |  | × |
| maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | 4.17.0 | × |

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
| bottom | 消息从底部弹出时，距离底部的位置，单位像素 | number | 24 |  |
| duration | 默认自动关闭延时，单位秒 | number | 4.5 |  |
| pauseOnHover | 悬停时是否暂停计时器 | boolean | true | 5.18.0 |
| getContainer | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLNode | () => document.body |  |
| rtl | 是否开启 RTL 模式 | boolean | false |  |
| maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | 4.17.0 |

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

1. **配置面**：覆盖 §6.8 P0+P1 字段（与 §6.8 打架以 §6.8 为准）；P1 必须实现且命名与官网一致。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：§6.8 P0+P1 示例均需可复现（官方非 debug 一个不少）。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/notification
- 中文文档：https://ant.design/components/notification-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/notification
- 驱动 gpui kit：`notification`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Notification** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/notification/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Notification）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Notification）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：全局展示通知提醒信息。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

数值对齐 antd `components/notification/style`（`prepareComponentToken` / `notification.ts`）：

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| duration 默认 | **4.5s** | API |
| 宽 | **384** | componentToken `width` |
| 字号正文 | **14** | `fontSize` |
| 标题字号 | **16** | `fontSizeLG` |
| 圆角 | **8** | `borderRadiusLG` |
| 边框线宽 | **1** | `lineWidth`（默认皮可 0 边框 + 阴影语义） |
| 内边距 | **20 × 24** | `paddingMD` × `paddingContentHorizontalLG` / `paddingLG` |
| 图标尺寸 | **24** | `fontSizeLG × lineHeightLG` |
| 关闭钮约 | **22** | `controlHeightLG × 0.55` |
| 列表项间距 | **16** | `margin`（notificationMarginBottom） |
| 边缘 inset | **24** | `marginLG`（notificationMarginEdge）；`top`/`bottom` API 默认 24 |
| stack 阈值 | **3** | `stack.threshold` 默认 |
| Focus ring outset | ≈ **1.5px** 可见 | 关闭钮等可聚焦控件 |

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
open ──► placement 角落独立队列显示（默认 topRight，边缘 inset 24）
duration（默认 4.5s ±0.2s）──► 自动关；0/false 常驻
key 更新 ──► 同 key 替换，不新增
maxCount 超限 ──► 丢最旧一条
stack 超 threshold（默认 3）──► 折叠只展最新
actions 点击 ──► 业务回调；手动 close ──► onClose
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| NTF-S1 | open | 右上角（默认 topRight）可见，宽 384 |
| NTF-S2 | placement=bottomLeft | 左下角队列位置，边缘 inset 24 |
| NTF-S3 | duration 到期（4.5s ±0.2s） | 消失；duration=0 常驻 |
| NTF-S4 | key 更新 | 同 key 替换，不新增一条 |
| NTF-S5 | 手动 close | onClose 触发并移除 |
| NTF-S6 | 带 actions | 按钮可点，回调透出 |
| NTF-S7 | maxCount=2 后连发 3 条 | 最旧被丢弃，剩 2 条 |
| NTF-S8 | stack 超 threshold=3 | 折叠只展最新，其余收起 |
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
| 角色 | 卡片 `role=alert`（默认打断），可配 `role=status` 非打断 |
| 命名 | 每卡名=title＋description（description 必选，空测试失败） |
| 键盘 | Esc 关闭当前卡（适用时）；actions 内按钮保留自身键盘能力 |
| 焦点环 | 关闭按钮/actions 按钮聚焦时 ring 可见；轻提示默认不抢焦点 |
| 遮罩 | 无遮罩；多卡堆叠朗读顺序与视觉顺序一致 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 类型打开（open/success/info/warning/error）+ key 更新替换 + destroy | **对等** | P0 L1 |
| 六角堆叠池（`top/topLeft/topRight/bottom/bottomLeft/bottomRight`，默认 `topRight`；边缘 inset 24，`top`/`bottom` API 各默认 24） | **对等**：P0 先验 topRight 与 bottomLeft 两点，全引擎后补 | P0 L1 |
| 自动关闭（默认 **4.5s**，`0/false` 常驻；容差 ±0.2s，虚拟时钟断言） | **对等** | P0 L1 |
| 堆叠上限（`maxCount` 超限丢最旧；`stack.threshold` 默认 3 超量折叠） | **对等**：队列写实 | P0 L1 |
| `actions` 按钮组 + `onClick`/`onClose` + `closable` | **对等** | P0 L1 |
| 尺寸/色 Token（宽 384、内边距 20×24、圆角 8、图标 24） | **对等** | P0 L2 |
| `showProgress` 进度条/`pauseOnHover` 精确计时 | **P1 必做** | P1 |
| 静态方法全局单例 | **映射**：`useNotification`/App holder 为 P0 | P1 |
| Semantic classNames/styles 深度 | kit 语义钩子 | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

> 前置（增量规格，另起文档）：6 方位独立堆叠池 placement 引擎（每方位独立队列与偏移）与 `role=status` 非打断语义；P0 先验 topRight（默认）与 bottomLeft 两点主路径（NTF-02 / NTF-03），全引擎后补。

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `Open` / `Info` / `Success` / `Error` / `Warning` | hooks 对等 API；`Node()` 作 contextHolder |
| `title` / `description` | 主文案；`title` 取代已弃用 `message` |
| `placement` | `top` \| `topLeft` \| `topRight` \| `bottom` \| `bottomLeft` \| `bottomRight`；默认 `topRight` |
| `duration` | 默认 4.5s；`0` 常驻 |
| `key` 更新 | 同 key 替换，不新增 |
| `icon` / 类型图标 | 自定义 `IconName`；type sugar 带语义图标 |
| `actions` | 自定义按钮组（Go：`[]NotificationAction`） |
| `closable` / 手动 close | 默认 true；关闭触发 `onClose` |
| `onClick` / `onClose` | 必须 |
| `stack` + threshold | 默认阈值 3；超过折叠 |
| `Destroy` / `maxCount` / `top` / `bottom` | 全局配置子集 |
| 官方主路径示例 | Hooks 调用（推荐）、自动关闭的延时、带有图标的通知提醒框、自定义按钮、自定义图标、位置、更新消息内容、堆叠 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | role=`alert`（可 `status`）；轻提示不抢焦点；关闭可操作 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| `showProgress` / 进度条色 | 显示进度条、自定义进度条颜色 |
| `pauseOnHover` 精确计时 | 悬停暂停 |
| 静态方法全局单例 | `notification.success` 静态路径（不推荐） |
| semantic classNames/styles 深度 | 自定义语义结构样式 |
| 动画像素级 / 复杂虚拟列表 stack 阴影 | P1 必做 |
| 浏览器-only API（getContainer/prefixCls/RTL） | P1 必做 |
| debug 示例 | 不验收（仅参考） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestNotification_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Notification 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| NTF-01 | L1 | NewNotification 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| NTF-02 | L1 | open | 可见 |
| NTF-03 | L1 | placement=bottomLeft | 位置在左下 |
| NTF-04 | L1 | duration 到期 | 消失 |
| NTF-05 | L1 | key 更新 | 不新增一条 |
| NTF-06 | L1 | 手动 close | onClose |
| NTF-07 | L1 | 带 btn | 按钮可点 |
| NTF-08 | L1 | 复现「Hooks 调用」（`hooks.tsx`）：经 holder `Open({Title, Description})` | 右上角出现宽 384 卡片，标题+内容齐 |
| NTF-09 | L1 | 复现「自动关闭的延时」（`duration.tsx`）：`duration=0` 与默认各发一条 | 前者常驻，后者 4.5s±0.2s 消失 |
| NTF-10 | L1 | 复现「带有图标的通知提醒框」（`with-icon.tsx`）：success/info/warning/error 各一条 | 4 条图标语义色互异 |
| NTF-11 | L1 | 复现「自定义按钮」（`with-btn.tsx`）：带 `actions` 发一条并点按钮 | 按钮回调触发，卡片不误关 |
| NTF-12 | L1 | 复现「自定义图标」（`custom-icon.tsx`）：设 `IconName` 发一条 | 自定义图标替换默认类型图标 |
| NTF-13 | L1 | 复现「位置」（`placement.tsx`）：`topRight` 与 `bottomLeft` 各发一条 | 两卡分属右上/左下队列，互不串池 |
| NTF-14 | L1 | 复现「更新消息内容」（`update.tsx`）：同 key 发两次不同 description | 卡片仍 1 张，内容为第二次文案 |
| NTF-15 | L1 | 复现「堆叠」（`stack.tsx`）：threshold=3 下连发 5 条 | 只展最新 + 折叠，其余收起 |
| NTF-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| NTF-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| NTF-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| NTF-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| NTF-20 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| NTF-21 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| NTF-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API（含历史 `Message.Notification` 兼容路径）；语义对齐 antd `useNotification` + `api.open/config`。

```text
NewNotification() *Notification

// 全局 / holder
SetTheme(*Theme) / SetFace / SetStyle
SetDuration(seconds) / SetPlacement / SetTop / SetBottom
SetMaxCount / SetStack / SetStackThreshold / SetClosable
Node() core.Node                 // OverlayPortal contextHolder，挂 app root
AttachTicker(*Tree) / Tick(dt)   // duration 自动关；无私有帧循环

// 打开
Open(NotificationConfig) *NotificationHandle
Info|Success|Error|Warning(NotificationConfig) *NotificationHandle
Destroy(keys ...string)          // 空 = 全清

// NotificationConfig P0 字段
//   Type, Title, Description, Key, IconName
//   Duration + DurationSet（0 = 常驻）
//   Placement, Closable + ClosableSet
//   Actions []NotificationAction{Label, Primary, OnClick}
//   OnClick, OnClose, Style, Role ("alert"|"status")

// 只读 / 测试
Count() / Items() / VisibleItems()
PlacementOf(key) / ItemBox(key)  // 布局断言（可选）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Duration | **4.5** |
| Placement | **topRight** |
| Top / Bottom | **24** |
| Closable | **true** |
| Stack | false（`SetStack(true)` 后 threshold=**3**） |
| Role | **alert** |
| Width | **384** |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
NotificationHolder（四角六方位独立堆叠池；与 Message 顶栏单队列区分）
  ├─ 6 队列：top/topLeft/topRight/bottom/bottomLeft/bottomRight（默认 topRight；边缘 inset 24 ±0.5px；各池独立不串池）
  └─ 队列内纵向卡片（宽 384 ±0.5px；内边距 20×24；圆角 8；图标 24）
       ├─ 卡片（icon + title + description 必选 + close + actions；role=alert 默认打断，可配 status）
       ├─ 入队：同 placement 追加；同 key 替换不新增（NTF-S4）；maxCount 超限丢最旧（NTF-S7）
       ├─ stack 超 threshold（默认 3）折叠只展最新（NTF-S8）；计时 duration 默认 4.5s ±0.2s（虚拟时钟），0/false 常驻
       └─ 交互：手动 close → onClose；actions 点击透业务回调不误关；Esc 关当前卡（适用时）
```

- 与 Message 区分：Notification 为四角六方位独立池重卡片（含 title/description/actions/close，可打断）；Message 为顶部居中单队列轻条（无分栏、无 actions、不抢焦点，见 [message.md §6.11](./message.md#611-结构与绘制分层实现提示)）。

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Notification 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/notification.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Notification 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
