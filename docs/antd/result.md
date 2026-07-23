# Result 结果
> 来源：[Ant Design 6.5.x Result](https://ant.design/components/result)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：用于反馈一系列操作任务的处理结果。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于反馈一系列操作任务的处理结果。

**Result** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| Success | 复现「Success」视觉与布局 |
| Info | 复现「Info」视觉与布局 |
| Warning | 复现「Warning」视觉与布局 |
| 403 | 复现「403」视觉与布局 |
| 404 | 复现「404」视觉与布局 |
| 500 | 复现「500」视觉与布局 |
| Error | 复现「Error」视觉与布局 |
| 自定义 icon | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：自定义组件内部各语义化结构的类名。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：6.0.0

#### `extra`

- **说明**：操作区
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义 icon
- **类型**：ReactNode
- **默认值**：-

#### `status`

- **说明**：结果的状态，决定图标和颜色
- **类型**：`success` | `error` | `info` | `warning` | `404` | `403` | `500`
- **默认值**：`info`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |
  | `404` | 官方取值 `404` |
  | `403` | 官方取值 `403` |
  | `500` | 官方取值 `500` |

#### `styles`

- **说明**：自定义组件内部各语义化结构的内联样式。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：6.0.0

#### `title`

- **说明**：title 文字
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

当有重要操作需告知用户处理结果，且反馈内容较为复杂时使用。

### 2.2 核心功能（按官方示例拆解）

1. **Success**（`success.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **Info**（`info.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **Warning**（`warning.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **403**（`403.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **404**（`404.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **500**（`500.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **Error**（`error.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义 icon**（`customIcon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| Success | `success.tsx` | 否 |
| Info | `info.tsx` | 否 |
| Warning | `warning.tsx` | 否 |
| 403 | `403.tsx` | 否 |
| 404 | `404.tsx` | 否 |
| 500 | `500.tsx` | 否 |
| Error | `error.tsx` | 否 |
| 自定义 icon | `customIcon.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| extra | 操作区 | ReactNode | - | icon | 自定义 icon | ReactNode | - | status | 结果的状态，决定图标和颜色 | `success` \| `error` \| `info` \| `warning` \| `404` \| `403` \| `500` | `info` | styles | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| subTitle | subTitle 文字 | ReactNode | - | title | title 文字 | ReactNode | - 
### 导入方式

```js
import { Result } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record \| (info: { props }) => Record | - | 6.0.0 |
| `extra` | 操作区 | ReactNode | - | — |
| `icon` | 自定义 icon | ReactNode | - | — |
| `status` | 结果的状态，决定图标和颜色 | `success` \| `error` \| `info` \| `warning` \| `404` \| `403` \| `500` | `info` | — |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record \| (info: { props }) => Record | - | 6.0.0 |
| `subTitle` | subTitle 文字 | ReactNode | - | — |
| `title` | title 文字 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Result** 的验收清单：

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
- 官方文档：https://ant.design/components/result
- 中文文档：https://ant.design/components/result-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/result
- 驱动 gpui kit：`result`
