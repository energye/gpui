# App 包裹组件
> 来源：[Ant Design 6.5.x App](https://ant.design/components/app)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：提供重置样式和提供消费上下文的默认环境。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

提供重置样式和提供消费上下文的默认环境。

**App** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| Hooks 配置 | 复现「Hooks 配置」视觉与布局 |

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
实现 gpui kit 版 **App** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **2** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/app
- 中文文档：https://ant.design/components/app-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/app
- 驱动 gpui kit：`app`
