# Message 全局提示
> 来源：[Ant Design 6.5.x Message](https://ant.design/components/message)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：全局展示操作反馈信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

全局展示操作反馈信息。

**Message** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| Hooks 调用（推荐） | 复现「Hooks 调用（推荐）」视觉与布局 |
| 其他提示类型 | type 预设外观 |
| 修改延时 | 复现「修改延时」视觉与布局 |
| 堆叠 | 复现「堆叠」视觉与布局 |
| 加载中 | loading 指示与防重复 |
| Promise 接口 | 复现「Promise 接口」视觉与布局 |
| 自定义语义结构样式 | 自定义渲染/插槽外观 |
| 更新消息内容 | 复现「更新消息内容」视觉与布局 |
| 静态方法（不推荐） | 复现「静态方法（不推荐）」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `content`

- **说明**：提示内容
- **类型**：ReactNode | config
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `config` | 官方取值 `config` |

#### `duration`

- **说明**：自动关闭的延时，单位秒。设为 0 时不自动关闭
- **类型**：number
- **默认值**：3

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `icon`

- **说明**：自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `style`

- **说明**：自定义内联样式
- **类型**：[CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794)
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `getContainer`

- **说明**：配置渲染节点的输出位置，但依旧为全屏展示
- **类型**：() => HTMLElement
- **默认值**：() => document.body

#### `maxCount`

- **说明**：最大显示数，超过限制时，最早的消息会被自动关闭
- **类型**：number
- **默认值**：-

#### `prefixCls`

- **说明**：消息节点的 className 前缀
- **类型**：string
- **默认值**：`ant-message`
- **版本**：4.5.0

#### `stack`

- **说明**：堆叠模式，超过阈值时会将所有消息收起。折叠状态下仅展示最新的消息
- **类型**：boolean | `{ threshold: number }`
- **默认值**：false
- **版本**：6.4.0

#### `top`

- **说明**：消息距离顶部的位置
- **类型**：string | number
- **默认值**：8

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

- 可提供成功、警告和错误等反馈信息。
- 顶部居中显示并自动消失，是一种不打断用户操作的轻量级提示方式。

### 2.2 核心功能（按官方示例拆解）

1. **Hooks 调用（推荐）**（`hooks.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **其他提示类型**（`other.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **修改延时**（`duration.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **堆叠**（`stack.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **加载中**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **Promise 接口**（`thenable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构样式**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **更新消息内容**（`update.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **静态方法（不推荐）**（`info.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击 message 时触发的回调函数 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| Hooks 调用（推荐） | `hooks.tsx` | 否 |
| 其他提示类型 | `other.tsx` | 否 |
| 修改延时 | `duration.tsx` | 否 |
| 堆叠 | `stack.tsx` | 否 |
| 加载中 | `loading.tsx` | 否 |
| Promise 接口 | `thenable.tsx` | 否 |
| 自定义语义结构样式 | `style-class.tsx` | 否 |
| 更新消息内容 | `update.tsx` | 否 |
| 静态方法（不推荐） | `info.tsx` | 否 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 全局方法

还提供了全局配置和全局销毁方法：

- `message.config(options)`
- `message.destroy()`

> 也可通过 `message.destroy(key)` 来关闭一条消息。

#### message.config

> 当你使用 `ConfigProvider` 进行全局化配置时，系统会默认自动开启 RTL 模式。(4.3.0+)
>
> 当你想单独使用，可通过如下设置开启 RTL 模式。

```js
message.config({
  top: 100,
  duration: 2,
  maxCount: 3,
  rtl: true,
  prefixCls: 'my-message',
});
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| duration | 默认自动关闭延时，单位秒 | number | 3 | getContainer | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLElement | () => document.body | maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | prefixCls | 消息节点的 className 前缀 | string | `ant-message` | 4.5.0 | × |
| rtl | 是否开启 RTL 模式 | boolean | false | stack | 堆叠模式，超过阈值时会将所有消息收起。折叠状态下仅展示最新的消息 | boolean \| `{ threshold: number }` | false | 6.4.0 | × |
| top | 消息距离顶部的位置 | string \| number | 8 
#### 方法

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

### 2.6 FAQ

## FAQ

### 为什么 message 不能获取 context、redux 的内容和 ConfigProvider 的 `locale/prefixCls/theme` 等配置？ {#faq-context-redux}

直接调用 message 方法，antd 会通过 `ReactDOM.render` 动态创建新的 React 实体。其 context 与当前代码所在 context 并不相同，因而无法获取 context 信息。

当你需要 context 信息（例如 ConfigProvider 配置的内容）时，可以通过 `message.useMessage` 方法会返回 `api` 实体以及 `contextHolder` 节点。将其插入到你需要获取 context 位置即可：

```tsx
const [api, contextHolder] = message.useMessage();

return (
  
    {/* contextHolder 在 Context1 内，它可以获得 Context1 的 context */}
    {contextHolder}
    
      {/* contextHolder 在 Context2 外，因而不会获得 Context2 的 context */}
    
  
);
```

**异同**：通过 hooks 创建的 `contextHolder` 必须插入到子元素节点中才会生效，当你不需要上下文信息时请直接调用。

> 可通过 [App 包裹组件](/components/app-cn) 简化 `useMessage` 等方法需要手动植入 contextHolder 的问题。

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

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

组件提供了一些静态方法，使用方式和参数如下：

- `message.success(content, [duration], onClose)`
- `message.error(content, [duration], onClose)`
- `message.info(content, [duration], onClose)`
- `message.warning(content, [duration], onClose)`
- `message.loading(content, [duration], onClose)`

| 参数     | 说明                                        | 类型                | 默认值 |
| -------- | ------------------------------------------- | ------------------- | ------ |
| content  | 提示内容                                    | ReactNode \| config | -      |
| duration | 自动关闭的延时，单位秒。设为 0 时不自动关闭 | number              | 3      |
| onClose  | 关闭时触发的回调函数                        | function            | -      |

组件同时提供 promise 接口。

- `message[level](content, [duration]).then(afterClose)`
- `message[level](content, [duration], onClose).then(afterClose)`

其中 `message[level]` 是组件已经提供的静态方法。`then` 接口返回值是 Promise。

也可以对象的形式传递参数：

- `message.open(config)`
- `message.success(config)`
- `message.error(config)`
- `message.info(config)`
- `message.warning(config)`
- `message.loading(config)`

`config` 对象属性如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| className | 自定义 CSS class | string | - | - | 5.7.0 |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| content | 提示内容 | ReactNode | - | - | × |
| duration | 自动关闭的延时，单位秒。设为 0 时不自动关闭 | number | 3 | - | × |
| icon | 自定义图标 | ReactNode | - | - | × |
| pauseOnHover | 悬停时是否暂停计时器 | boolean | true | - | × |
| key | 当前提示的唯一标志 | string \| number | - | - | × |
| style | 自定义内联样式 | [CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794) | - | - | 5.7.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| onClick | 点击 message 时触发的回调函数 | function | - | - | × |
| onClose | 关闭时触发的回调函数 | function | - | - | × |

### 全局方法

还提供了全局配置和全局销毁方法：

- `message.config(options)`
- `message.destroy()`

> 也可通过 `message.destroy(key)` 来关闭一条消息。

#### message.config

> 当你使用 `ConfigProvider` 进行全局化配置时，系统会默认自动开启 RTL 模式。(4.3.0+)
>
> 当你想单独使用，可通过如下设置开启 RTL 模式。

```js
message.config({
  top: 100,
  duration: 2,
  maxCount: 3,
  rtl: true,
  prefixCls: 'my-message',
});
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| duration | 默认自动关闭延时，单位秒 | number | 3 | getContainer | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLElement | () => document.body | maxCount | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | prefixCls | 消息节点的 className 前缀 | string | `ant-message` | 4.5.0 | × |
| rtl | 是否开启 RTL 模式 | boolean | false | stack | 堆叠模式，超过阈值时会将所有消息收起。折叠状态下仅展示最新的消息 | boolean \| `{ threshold: number }` | false | 6.4.0 | × |
| top | 消息距离顶部的位置 | string \| number | 8 
### 导入方式

```js
import { Message } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `content` | 提示内容 | ReactNode \| config | - | — |
| `duration` | 自动关闭的延时，单位秒。设为 0 时不自动关闭 | number | 3 | — |
| `onClose` | 关闭时触发的回调函数 | function | - | — |
| `className` | 自定义 CSS class | string | - | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `icon` | 自定义图标 | ReactNode | - | - |
| `pauseOnHover` | 悬停时是否暂停计时器 | boolean | true | - |
| `key` | 当前提示的唯一标志 | string \| number | - | - |
| `style` | 自定义内联样式 | [CSSProperties](https://github.com/DefinitelyTyped/DefinitelyTyped/blob/e434515761b36830c3e58a970abf5186f005adac/types/react/index.d.ts#L794) | - | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `onClick` | 点击 message 时触发的回调函数 | function | - | - |
| `getContainer` | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLElement | () => document.body | — |
| `maxCount` | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - | — |
| `prefixCls` | 消息节点的 className 前缀 | string | `ant-message` | 4.5.0 |
| `rtl` | 是否开启 RTL 模式 | boolean | false | — |
| `stack` | 堆叠模式，超过阈值时会将所有消息收起。折叠状态下仅展示最新的消息 | boolean \| `{ threshold: number }` | false | 6.4.0 |
| `top` | 消息距离顶部的位置 | string \| number | 8 | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Message** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **9** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/message
- 中文文档：https://ant.design/components/message-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/message
- 驱动 gpui kit：`message`
