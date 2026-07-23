# Switch 开关
> 来源：[Ant Design 6.5.x Switch](https://ant.design/components/switch)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：使用开关切换两种状态之间。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

使用开关切换两种状态之间。

**Switch** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 不可用 | disabled 态 |
| 文字和图标 | icon 与文本混排 |
| 两种大小 | 不同 size 档位 |
| 加载中 | loading 指示与防重复 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `checked`

- **说明**：指定当前是否选中
- **类型**：boolean
- **默认值**：false

#### `checkedChildren`

- **说明**：选中时的内容
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `loading`

- **说明**：加载中的开关
- **类型**：boolean
- **默认值**：false

#### `size`

- **说明**：开关大小，可选值：`medium` `small`
- **类型**：`'medium'` | `'small'`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `unCheckedChildren`

- **说明**：非选中时的内容
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

- 需要表示开关状态/两种状态之间的切换时；
- 和 `checkbox` 的区别是，切换 `switch` 会直接触发状态改变，而 `checkbox` 一般用于状态标记，需要和提交操作配合。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **文字和图标**（`text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **两种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **加载中**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | `checked` 的别名 |
| `defaultValue` | 非受控默认值 | `defaultChecked` 的别名 |
| `onChange` | 值变化 | 变化时的回调函数 |
| `onClick` | 点击 | 点击时的回调函数 |
| `disabled` | 禁用 | 是否禁用 |
| `loading` | 加载中 | 加载中的开关 |
| `checked` | 选中布尔 | 指定当前是否选中 |
| `defaultChecked` | 默认选中 | 初始是否选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 文字和图标 | `text.tsx` | 否 |
| 两种大小 | `size.tsx` | 否 |
| 加载中 | `loading.tsx` | 否 |
| 自定义组件 Token | `component-token.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 2.6 FAQ

## FAQ

### 为什么在 Form.Item 下不能绑定数据？ {#faq-binding-data}

Form.Item 默认绑定值属性到 `value` 上，而 Switch 的值属性为 `checked`。你可以通过 `valuePropName` 来修改绑定的值属性。

```tsx | pure

  

```

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
| checked | 指定当前是否选中 | boolean | false | checkedChildren | 选中时的内容 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultChecked | 初始是否选中 | boolean | false | defaultValue | `defaultChecked` 的别名 | boolean | - | 5.12.0 | × |
| disabled | 是否禁用 | boolean | false | loading | 加载中的开关 | boolean | false | size | 开关大小，可选值：`medium` `small` | `'medium'` \| `'small'` | `medium` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | unCheckedChildren | 非选中时的内容 | ReactNode | - | value | `checked` 的别名 | boolean | - | 5.12.0 | × |
| onChange | 变化时的回调函数 | function(checked: boolean, event: Event) | - | onClick | 点击时的回调函数 | function(checked: boolean, event: Event) | - 
## 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 导入方式

```js
import { Switch } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false | — |
| `checkedChildren` | 选中时的内容 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultChecked` | 初始是否选中 | boolean | false | — |
| `defaultValue` | `defaultChecked` 的别名 | boolean | - | 5.12.0 |
| `disabled` | 是否禁用 | boolean | false | — |
| `loading` | 加载中的开关 | boolean | false | — |
| `size` | 开关大小，可选值：`medium` `small` | `'medium'` \| `'small'` | `medium` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `unCheckedChildren` | 非选中时的内容 | ReactNode | - | — |
| `value` | `checked` 的别名 | boolean | - | 5.12.0 |
| `onChange` | 变化时的回调函数 | function(checked: boolean, event: Event) | - | — |
| `onClick` | 点击时的回调函数 | function(checked: boolean, event: Event) | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Switch** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **6** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/switch
- 中文文档：https://ant.design/components/switch-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/switch
- 驱动 gpui kit：`switch`
