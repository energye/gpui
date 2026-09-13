# Message 全局提示
> 来源：[Ant Design 6.5.x Message](https://ant.design/components/message)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：全局展示操作反馈信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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
| `open` / `success` / `error` / `info` / `warning` / `loading` | 类型打开 | 按类型打开一条，返回句柄（promise `then` 等关闭） |
| `duration` | 自动关闭 | 秒；`0` 常驻；默认 3 |
| `key` | 同条更新 | 同 key 再次 open 只更新内容，不新增 |
| `onClose` | 关闭回调 | 关闭时触发 |
| `destroy` | 销毁 | `destroy(key?)` 关一条或清空 |
| `maxCount` | 最大显示数 | 超限丢最旧 |
| `stack` | 堆叠折叠 | 超阈值收起，只展最新 |
| `top` | 顶部偏移 | 默认 8 |
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
| 语义结构调试 | `_semantic.tsx` | 是 |

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

- **依赖等级**：L3（浮层：顶部居中队列 + 堆叠定位）。
- **等谁**：浮层定位（holder 队列）、Icon/Progress（类型图标与 loading 环）。
- **文件归属**：`ui/kit/message/`。
- **组合**：经 App 上下文消费（`UseApp`）；ConfigProvider 下发 `top`/`duration`/`maxCount` 全局默认。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Message** 的验收清单：

1. **配置面**：覆盖 §6.8 P0 字段（与 §6.8 打架以 §6.8 为准）；P1 可分期但命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：§6.8 P0 示例均需可复现（官方非 debug 主路径）。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/message
- 中文文档：https://ant.design/components/message-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/message
- 驱动 gpui kit：`message`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Message** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/message/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Message）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Message）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：全局展示操作反馈信息。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。
源码：`components/message/style/index.ts`（`contentPadding` 由 `controlHeightLG/fontSize/lineHeight` 派生）+ notice 共享样式（圆角 `borderRadiusLG=8`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| duration 默认 | **3s** | API |
| 顶部偏移 top | **8** | API（`getPlacementOffsetStyle`） |
| 内容区 padding | **≈9 × 12** | `contentPadding` = `(controlHeightLG − fontSize×lineHeight)/2` × `paddingSM` |
| 最大宽 | `max-content` / `maxWidth 100%` | 水平居中不定宽 |
| 字号 middle | **14** | `fontSize` |
| 圆角 | **8** | `borderRadiusLG`（notice 共享样式） |
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
| `content` | 提示内容 | ReactNode \| config |
| `duration` | 自动关闭的延时，单位秒。设为 0 时不自动关闭 | number | 3 |
| `onClose` | 关闭时触发的回调函数 | function | - |
| `className` | 自定义 CSS class | string | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `icon` | 自定义图标 | ReactNode | - |
| `pauseOnHover` | 悬停时是否暂停计时器 | boolean | true |
| `key` | 当前提示的唯一标志 | string \| number |
| `style` | 自定义内联样式 | [CSSProperties](https://github.com/De… | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `onClick` | 点击 message 时触发的回调函数 | function | - |
| `getContainer` | 配置渲染节点的输出位置，但依旧为全屏展示 | () => HTMLElement | () => document.body |
| `maxCount` | 最大显示数，超过限制时，最早的消息会被自动关闭 | number | - |
| `prefixCls` | 消息节点的 className 前缀 | string | `ant-message` |
| `rtl` | 是否开启 RTL 模式 | boolean | false |
| `stack` | 堆叠模式，超过阈值时会将所有消息收起。折叠状态下仅展示最新的消息 | boolean \| `{ threshold: number }` |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
Message.success(content) ──► 顶栏入队显示
             ├── duration 到期 ──► 离场销毁
             ├── duration=0 ──► 常驻直至 close/destroy
             ├── 同 key 再次 open ──► 更新内容
             └── maxCount ──► 超限丢最旧
```

\*默认 duration=3s。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| MSG-S1 | success 调用 | 可见成功条 |
| MSG-S2 | duration=0.1 短时 | 0.1s±0.05s（虚拟时钟 Tick 推进）后自动消失 + `onClose` 恰一次 |
| MSG-S3 | duration=0 | 不自动关（虚拟时钟推进任意步仍在，直至 close/destroy） |
| MSG-S4 | 同 key 更新 | 仍一条 |
| MSG-S5 | 连续多条 | 顶部居中纵向堆叠（`top=8` 起排） |
| MSG-S6 | destroy | 清空 |
| MSG-S7 | error/warning/info/loading | 图标类型正确 |
| MSG-S8 | `maxCount=2` 后连发 3 条 | 最旧一条被丢弃，队列剩 2 条 |
| MSG-S9 | `stack` 开启超 threshold | 折叠只展最新一条 + 计数，其余收起 |
| MSG-S10 | `pauseOnHover=true` 悬停 | 悬停期间剩余时长不变（虚拟时钟断言：推进 N 步仍不关），移开后恢复计时到期关 |

**可断言补充（MSG-S2/S3/S10，虚拟时钟）：** `duration` 默认 3s，测试一律用虚拟时钟 Tick 推进断言，不依赖真实时钟；短时 `duration=0.1` 容差 ±0.05s，默认 3s 容差 ±0.2s；`duration=0` 常驻分支推进任意步仍在；`pauseOnHover=true` 时悬停条 `剩余时长` 快照不变，移开后继续递减到期触发 `onClose`/`Then` 恰一次。
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
| 角色 | 条目 `role=status`，`aria-live=polite`（轻提示不断言 assertive） |
| 命名 | 每条名=content 文案（`format` 无，此处即内容本身） |
| 键盘 | 无键盘操作；轻提示默认不抢焦点 |
| 焦点环 | 不适用（条目本身不可聚焦；带 `onClick` 可点条目需可聚焦并 ring 可见） |
| 遮罩 | 无遮罩；多条堆叠朗读顺序与视觉顺序一致 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 类型打开（success/error/info/warning/loading）+ duration/key/onClose/destroy | **对等** | P0 L1 |
| 顶部居中队列（`top=8`，`getPlacementOffsetStyle`） | **对等**：holder 顶栏入队，`top` 可配 | P0 L1 |
| 堆叠队列（`maxCount` 超限丢最旧；`stack` 超阈值折叠只展最新，threshold 默认 3） | **对等**：队列写实，先进先出丢弃 + 折叠计数 | P0 L1 |
| 同 key 更新内容不新增 + promise `then(afterClose)` | **对等**：`MessageHandle.Then` 映射 | P0 L1 |
| `pauseOnHover` 悬停暂停计时 | **对等**（虚拟时钟可测） | P0 L1 |
| 尺寸/色 Token（内容区 ≈9×12、圆角 8） | **对等** | P0 L2 |
| 入场/离场动画 | **近似**或瞬时 | P1 |
| 全局静态方法脱离上下文（`ReactDOM.render` 语义） | **映射**：`useMessage`/App holder 为 P0，裸静态仅便利用法 | P1 |
| Semantic classNames/styles 深度 | kit 语义钩子 | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `onClick` | 必须 |
| `content` | 必须 |
| `icon` | 类型图标必须；自定义图标名/节点提供浅覆盖 |
| `duration` / `key` / `onClose` | 自动关闭、常驻、同 key 更新与关闭回调 |
| `destroy` / `maxCount` / `stack` / `top` | 清空、超限丢最旧、折叠堆叠、顶部偏移 |
| 官方主路径示例 | Hooks 调用（推荐）、其他提示类型、修改延时、堆叠、加载中、Promise 接口、自定义语义结构样式、更新消息内容 |
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
| 静态方法全局上下文 | `message.useMessage` / App 模式为 P0；全局静态方法仅保留便利用法 |
| 其余示例 | 静态方法（不推荐）, `_semantic.tsx` |

### 6.9 验收用例表（可测）

> 测试名建议：`TestMessage_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Message 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| MSG-01 | L1 | NewMessage 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| MSG-02 | L1 | success 调用 | 可见成功条 |
| MSG-03 | L1 | duration=0.1 短时 | 会自动消失 |
| MSG-04 | L1 | duration=0 | 不自动关 |
| MSG-05 | L1 | 同 key 更新 | 仍一条 |
| MSG-06 | L1 | 连续多条 | 堆叠 |
| MSG-07 | L1 | destroy | 清空 |
| MSG-08 | L1 | error/warning/info/loading | 图标类型正确 |
| MSG-09 | L1 | 复现「Hooks 调用」（`hooks.tsx`）：经 holder 发 `Success("hi")` | 顶栏出现成功条，holder 非空；`duration` 默认 3s |
| MSG-10 | L1 | 复现「其他提示类型」（`other.tsx`）：success/error/info/warning/loading 各发一条 | 5 条图标语义互异，loading 条带旋转指示 |
| MSG-11 | L1 | 复现「修改延时」（`duration.tsx`）：发 `duration=10` 与 `duration=0` 各一条 | 前者 10s 后消失，后者常驻直至 destroy |
| MSG-12 | L1 | 复现「堆叠」（`stack.tsx`）：threshold=3 下连发 5 条 | 只展最新 + 折叠计数，旧条收起 |
| MSG-13 | L1 | 复现「加载中」（`loading.tsx`）：`Loading("Action...", 0)` 后调 `Destroy` | 常驻loading条出现后被手动关闭，无残留 |
| MSG-14 | L1 | 复现「Promise 接口」（`thenable.tsx`）：`Success(...).Then(afterClose)` | 关闭后 `afterClose` 恰触发一次 |
| MSG-15 | L1 | 复现「自定义语义结构样式」（`style-class.tsx`） | 浅 Style 覆盖 root 圆角/色生效，不崩 |
| MSG-16 | L1 | 复现「更新消息内容」（`update.tsx`）：同 key 发两次不同 content | 队列仍 1 条，内容为第二次文案 |
| MSG-17 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| MSG-18 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| MSG-19 | L2 | 自定义 style-class 主路径 | 浅 Style 覆盖 root/icon/title 色与圆角；semantic 深度 P1 |
| MSG-20 | L1 | onClick 主路径 | 点击 message 触发 onClick；轻提示不抢焦点 |
| MSG-21 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| MSG-22 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| MSG-23 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewMessage() *Message

Open(MessageConfig) *MessageHandle
Info(content string, duration ...float64) *MessageHandle
Success(content string, duration ...float64) *MessageHandle
Error(content string, duration ...float64) *MessageHandle
Warning(content string, duration ...float64) *MessageHandle
Loading(content string, duration ...float64) *MessageHandle
Destroy(key ...string)
SetDuration(seconds float64)
SetTop(px float64)
SetMaxCount(n int)
SetStack(enabled bool)
SetStackThreshold(n int)

// 配置：MessageConfig 覆盖 content/type/icon/duration/key/style/onClick/onClose
// 回调：onClick / onClose；MessageHandle.Then(afterClose) 映射 promise 主路径
// 状态：loading 作为 MessageType，由 Ticker 驱动图标
// 主题：SetTheme(*Theme)；Style 可选覆盖
// a11y：实时区域 status；默认不抢焦点
// 挂树：Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Duration | 3s |
| Top | 8px |
| Type | info |
| MaxCount | 0（不限制；SetMaxCount 后超限丢最旧） |
| Stack | false；开启后 threshold 默认 3（来源：官方 `stack.tsx` 示例初始值 `useState(3)`，与 notification 默认阈值对齐） |
| PauseOnHover | true；可测：悬停期间关闭计时冻结（虚拟时钟断言剩余时长不变），移开后恢复计时 |

### 6.11 结构与绘制分层（实现提示）

```text
MessageHolder（顶栏居中单队列；与 Notification 六角独立池区分）
  └─ 纵向堆叠列（top=8 起排 ±0.5px；max-content/100% 不定宽；圆角 8）
       ├─ 条目（icon + content；role=status，aria-live=polite；默认不抢焦点）
       ├─ 入队：新条追加列尾；同 key 更新内容不新增（MSG-S4）
       ├─ maxCount 超限丢最旧 FIFO（MSG-S8）；stack 超 threshold（默认 3）折叠只展最新 + 计数（MSG-S9）
       ├─ 计时：每条 duration 默认 3s（±0.2s，虚拟时钟 Tick 推进）；duration=0 常驻；到期离场销毁 + onClose/Then(afterClose) 恰一次
       └─ 悬停冻结：pauseOnHover=true 时悬停条剩余时长不变，移开恢复（MSG-S10）
```

- 与 Notification 区分：Message 为顶部居中单队列轻提示（无 title/description 分栏、无 actions、无六角池）；Notification 为四角六方位独立堆叠池卡片（见 [notification.md §6.11](./notification.md#611-结构与绘制分层实现提示)）。

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Message 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/message.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Message 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
