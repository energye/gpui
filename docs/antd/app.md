# App 包裹组件
> 来源：[Ant Design 6.5.x App](https://ant.design/components/app)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：提供重置样式和提供消费上下文的默认环境。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

提供重置样式和提供消费上下文的默认环境。

**App** 本体几乎无自有外观：默认渲染一个 `div` 包裹层（`component=false` 时不建节点），只提供 antd 重置样式底；视觉即 children 原样，另挂 message / modal / notification 三个浮层的占位容器。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 顶层包 App，子页面拿到 message/modal/notification 上下文并能弹出提示 |
| Hooks 配置 | 给 App 传 message/notification 默认配置，子页面弹出时按该配置生效 |

### 1.4 交互视觉状态（实现检查表）

App 无自有交互视觉态：hover / active / focus ring / disabled / loading / error-warning 皮均标 **N/A**。只验两点：默认重置样式生效，message / modal / notification 占位容器随 children 挂载。

### 1.5 语义化 DOM 与主题

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 提供可消费 React context 的 `message.xxx`、`Modal.xxx`、`notification.xxx` 的静态方法，可以简化 useMessage 等方法需要手动植入 `contextHolder` 的问题。
- 提供基于 `.ant-app` 的默认重置样式，解决原生元素没有 antd 规范样式的问题。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **Hooks 配置**（`config.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| Hooks 配置 | `config.tsx` | 否 |

### 2.6 FAQ

## FAQ

### CSS Var 在 `` 内不起作用 {#faq-css-var-component-false}

请确保 App 的 `component` 是一个有效的 html 标签名，以便在启用 CSS 变量时有一个容器来承载 CSS 类名。如果不设置，则默认为 `div` 标签，如果设置为 `false`，则不会创建额外的 DOM 节点，也不会提供默认样式。

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

> 自 `antd@5.1.0` 版本开始提供该组件。

### App

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| component | 设置渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \| false | div | 5.11.0 | × |
| message | App 内 Message 的全局配置 | [MessageConfig](/components/message-cn/#messageconfig) | - | 5.3.0 | × |
| notification | App 内 Notification 的全局配置 | [NotificationConfig](/components/notification-cn/#notificationconfig) | - | 5.3.0 | × |

### 导入方式

```js
import { App } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `component` | 设置渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \| false | div | 5.11.0 |
| `message` | App 内 Message 的全局配置 | [MessageConfig](/components/message-cn/#messageconfig) | - | 5.3.0 |
| `notification` | App 内 Notification 的全局配置 | [NotificationConfig](/components/notification-cn/#notificationconfig) | - | 5.3.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **App** 的验收清单：

1. **配置面**：覆盖 `component` / `message` / `notification`（§6.3），命名与 antd 一致。
2. **上下文透传**：子级 `UseApp()` 能拿到 message / notification / modal 三件套；嵌套 App 按 §6.3 合并规则生效。
3. **无尺寸/受控/数据驱动/浮层定位/虚拟列表**：N/A（App 不做这些）。
4. **示例矩阵**：官方非 debug 示例约 **2** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/app
- 中文文档：https://ant.design/components/app-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/app
- 驱动 gpui kit：`app`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **App** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/app/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（App）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（App）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：提供重置样式和提供消费上下文的默认环境。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**其他**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `component` | 设置渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \ | false |
| `message` | App 内 Message 的全局配置 | [MessageConfig](/components/message-c… | - |
| `notification` | App 内 Notification 的全局配置 | [NotificationConfig](/components/noti… | - |

**上下文透传语义（源码 `components/app/App.tsx`）：** App 把 `message` / `notification` 与上层 `AppConfigContext` 做浅合并（子级覆盖父级同名字段），再经 `useMessage` / `useNotification` / `useModal` 生成三件套放入 `AppContext`；子组件经 `App.useApp()`（kit 侧 `UseApp()`）消费。`modal` 无全局配置项，直接透传。`component=false` 时不建包裹节点，同时无重置样式与 CSS Var 容器。App 必须在 `ConfigProvider` 之下才能拿到 Design Token；`UseApp` 必须在 App 子树内调用，否则无上下文可用。嵌套 App 按同规则逐层合并，内层优先。

### 6.4 交互状态机（L1）

```text
App 包裹 ──► message/modal/notification 上下文可用
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| APP-S1 | 包裹后经 `UseApp()` 取 message 并 `Success("hi")` | 同一 App 树下 message 队列长度 +1，holder 节点非空 |
| APP-S2 | 经 `UseApp()` 取 modal 并 `Confirm({Title:"t"})` | Modal holder 出现确认框节点，回调可关 |
| APP-S3 | 经 `UseApp()` 取 notification 并 `Open({Title:"t"})` | Notification holder 出现通知节点，可关 |
| APP-S4 | `SetMessageConfig({Duration:9})` 后 Success | 该条 message 的 duration 读到 9；未设字段沿用父级/默认 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 包裹层只留重置样式底，children 原样透出 |
| hover/active/focus/disabled/loading | N/A（App 本体无皮） |
| 主题切换 | 重置样式与浮层容器跟随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰图 | alt 或 aria-hidden |
| 有意义操作 | 复制/关闭/展开有名 |

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
| `component` | 必须 |
| `message` | 必须 |
| `notification` | 必须 |
| 官方主路径示例 | 基本用法、Hooks 配置 |
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

### 6.9 验收用例表（可测）

> 测试名建议：`TestApp_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 App 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| APP-01 | L1 | NewApp 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| APP-02 | L1 | 包裹后经 `UseApp()` 取 message 并 `Success("hi")` | message 队列长度 +1，holder 节点非空 |
| APP-03 | L1 | 经 `UseApp()` 取 modal 并 `Confirm({Title:"t"})` | Modal holder 出现确认框节点，回调可关 |
| APP-04 | L1 | 经 `UseApp()` 取 notification 并 `Open({Title:"t"})` | Notification holder 出现通知节点，可关 |
| APP-05 | L1 | `SetMessageConfig({Duration:9})` 后 Success | 该条 duration 读到 9；未设字段沿用父级/默认 |
| APP-06 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 子页面三件套均可弹；无控制台级错误 |
| APP-07 | L1 | 复现官方示例「Hooks 配置」（`config.tsx`） | message/notification 默认配置生效；无控制台级错误 |
| APP-08 | L2 | 重置样式底跟随 Theme | 换肤后底色/字色走 Token，无写死皮 |
| APP-09 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| APP-10 | L2 | disabled 外观 | **N/A**：App 无 disabled 皮 |
| APP-11 | L1 | 键盘/焦点主路径 | **N/A**：App 本体不可聚焦，焦点在 children/浮层内 |
| APP-12 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| APP-13 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| APP-14 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewApp(children ...core.Node) *App

// 三个具体设置函数（提案签名，不建包，实现可微调命名但语义不可丢）：
func (a *App) SetComponent(tag string, omit bool) // tag="div"；omit=true 对应 component=false，不建包裹节点
func (a *App) SetMessageConfig(cfg AppMessageConfig) // 如 cfg.Duration；与上层 AppConfig 浅合并，子级优先
func (a *App) SetNotificationConfig(cfg AppNotificationConfig) // 同上，子级优先

// 上下文消费（子树内）：UseApp() (message, notification, modal 三件套)
// 挂树：Node() core.Node； holders 经 MessageHolder()/ModalHolder()/NotificationHolder() 可验
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Component | `div` 包裹层；`omit=true` 时不建节点、无重置样式容器 |
| Message / Notification | 空配置，沿用上层 AppConfig / 全局默认 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Display root
  └─ content (+ actions?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **App 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/app.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` App 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
