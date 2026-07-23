# Spin 加载中
> 来源：[Ant Design 6.5.x Spin](https://ant.design/components/spin)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：用于页面和区块的加载中状态。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于页面和区块的加载中状态。

**Spin** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 各种大小 | 不同 size 档位 |
| 卡片加载中 | loading 指示与防重复 |
| 自定义描述文案 | 自定义渲染/插槽外观 |
| 延迟 | 复现「延迟」视觉与布局 |
| 自定义指示符 | 自定义渲染/插槽外观 |
| 进度 | 进度条/圈 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 全屏 | 复现「全屏」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `delay`

- **说明**：延迟显示加载效果的时间（防止闪烁）
- **类型**：number (毫秒)
- **默认值**：-

#### `description`

- **说明**：可以自定义描述文案
- **类型**：ReactNode
- **默认值**：-
- **版本**：6.3.0

#### `fullscreen`

- **说明**：显示带有 `Spin` 组件的背景
- **类型**：boolean
- **默认值**：false
- **版本**：5.11.0

#### `indicator`

- **说明**：加载指示符
- **类型**：ReactNode
- **默认值**：-

#### `percent`

- **说明**：展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度
- **类型**：number | 'auto'
- **默认值**：-
- **版本**：5.18.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `auto` | 官方取值 `auto` |

#### `size`

- **说明**：组件大小，可选值为 `small` `medium` `large`
- **类型**：string
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `spinning`

- **说明**：是否为加载中状态
- **类型**：boolean
- **默认值**：true

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `tip`

- **说明**：当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description`
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

页面局部处于等待异步数据或正在渲染过程时，合适的加载动效会有效缓解用户的焦虑。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **各种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **卡片加载中**（`nested.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义描述文案**（`tip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **延迟**（`delayAndDebounce.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义指示符**（`custom-indicator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **进度**（`percent.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **全屏**（`fullscreen.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `percent` | 进度值 | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 |
| `spinning` | 是否旋转 | 是否为加载中状态 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 各种大小 | `size.tsx` | 否 |
| 卡片加载中 | `nested.tsx` | 否 |
| 自定义描述文案 | `tip.tsx` | 否 |
| 延迟 | `delayAndDebounce.tsx` | 否 |
| 自定义指示符 | `custom-indicator.tsx` | 否 |
| 进度 | `percent.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 全屏 | `fullscreen.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 方法

### 静态方法

- `Spin.setDefaultIndicator(indicator: ReactNode)`

  你可以自定义全局默认 Spin 的元素。

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
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | delay | 延迟显示加载效果的时间（防止闪烁） | number (毫秒) | - | description | 可以自定义描述文案 | ReactNode | - | 6.3.0 | × |
| fullscreen | 显示带有 `Spin` 组件的背景 | boolean | false | 5.11.0 | × |
| indicator | 加载指示符 | ReactNode | - | percent | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 | number \| 'auto' | - | 5.18.0 | × |
| size | 组件大小，可选值为 `small` `medium` `large` | string | `medium` | spinning | 是否为加载中状态 | boolean | true | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | ~~tip~~ | 当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description` | ReactNode | - | ~~wrapperClassName~~ | 包装器的类属性。已废弃，请使用 `classNames.root` | string | - 
### 静态方法

- `Spin.setDefaultIndicator(indicator: ReactNode)`

  你可以自定义全局默认 Spin 的元素。

### 导入方式

```js
import { Spin } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `delay` | 延迟显示加载效果的时间（防止闪烁） | number (毫秒) | - | — |
| `description` | 可以自定义描述文案 | ReactNode | - | 6.3.0 |
| `fullscreen` | 显示带有 `Spin` 组件的背景 | boolean | false | 5.11.0 |
| `indicator` | 加载指示符 | ReactNode | - | — |
| `percent` | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 | number \| 'auto' | - | 5.18.0 |
| `size` | 组件大小，可选值为 `small` `medium` `large` | string | `medium` | — |
| `spinning` | 是否为加载中状态 | boolean | true | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `tip` | 当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description` | ReactNode | - | — |
| `wrapperClassName` | 包装器的类属性。已废弃，请使用 `classNames.root` | string | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Spin** 的验收清单：

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
- 官方文档：https://ant.design/components/spin
- 中文文档：https://ant.design/components/spin-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/spin
- 驱动 gpui kit：`spin`
