# Checkbox 多选框
> 来源：[Ant Design 6.5.x Checkbox](https://ant.design/components/checkbox)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：收集用户的多项选择。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

收集用户的多项选择。

**Checkbox** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 不可用 | disabled 态 |
| 受控的 Checkbox | 复现「受控的 Checkbox」视觉与布局 |
| Checkbox 组 | 复现「Checkbox 组」视觉与布局 |
| 全选 | 复现「全选」视觉与布局 |
| 布局 | 复现「布局」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `checked`

- **说明**：指定当前是否选中
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：失效状态
- **类型**：boolean
- **默认值**：false

#### `indeterminate`

- **说明**：设置 indeterminate 状态，只负责样式控制
- **类型**：boolean
- **默认值**：false

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `title`

- **说明**：选项的 title
- **类型**：`string`
- **默认值**：-

#### `style`

- **说明**：选项的样式
- **类型**：`React.CSSProperties`
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

- 在一组可选项中进行多项选择时；
- 单独使用可以表示两种状态之间的切换，和 `switch` 类似。区别在于切换 `switch` 会直接触发状态改变，而 `checkbox` 一般用于状态标记，需要和提交操作配合。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **受控的 Checkbox**（`controller.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **Checkbox 组**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **全选**（`check-all.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **布局**（`layout.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定选中的选项 |
| `defaultValue` | 非受控默认值 | 默认选中的选项 |
| `onChange` | 值变化 | 变化时的回调函数 |
| `disabled` | 禁用 | 失效状态 |
| `options` | 数据化 options | 指定可选项 |
| `checked` | 选中布尔 | 指定当前是否选中 |
| `indeterminate` | 半选 | 设置 indeterminate 状态，只负责样式控制 |
| `defaultChecked` | 默认选中 | 初始是否选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 受控的 Checkbox | `controller.tsx` | 否 |
| Checkbox 组 | `group.tsx` | 否 |
| 全选 | `check-all.tsx` | 否 |
| 布局 | `layout.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 同行布局 | `debug-line.tsx` | 是 |
| 禁用下的 Tooltip | `debug-disable-popover.tsx` | 是 |
| Group 内勾选框等宽 | `debug-group-width.tsx` | 是 |
| 自定义 lineWidth | `custom-line-width.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 方法 {#methods}

#### Checkbox

| 名称          | 描述                      | 版本   |
| ------------- | ------------------------- | ------ |
| blur()        | 移除焦点                  |        |
| focus()       | 获取焦点                  |        |
| nativeElement | 返回 Checkbox 的 DOM 节点 | 5.17.3 |

### 2.6 FAQ

## FAQ

### 为什么在 Form.Item 下不能绑定数据？ {#faq-form-item-limitations}

Form.Item 默认绑定值属性到 `value` 上，而 Checkbox 的值属性为 `checked`。你可以通过 `valuePropName` 来修改绑定的值属性。

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

#### Checkbox

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| checked | 指定当前是否选中 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultChecked | 初始是否选中 | boolean | false | disabled | 失效状态 | boolean | false | indeterminate | 设置 indeterminate 状态，只负责样式控制 | boolean | false | onChange | 变化时的回调函数 | (e: CheckboxChangeEvent) => void | - | onBlur | 失去焦点时的回调 | function() | - | onFocus | 获得焦点时的回调 | function() | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - 
#### Checkbox.Group

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认选中的选项 | (string \| number)\[] | \[] | name | CheckboxGroup 下所有 `input[type="checkbox"]` 的 `name` 属性 | string | - | value | 指定选中的选项 | (string \| number \| boolean)\[] | \[] | className | 选项的类名 | `string` | - | 5.25.0 |
| style | 选项的样式 | `React.CSSProperties` | - 
##### Option

```typescript
interface Option {
  label: string;
  value: string;
  disabled?: boolean;
}
```

### 方法 {#methods}

#### Checkbox

| 名称          | 描述                      | 版本   |
| ------------- | ------------------------- | ------ |
| blur()        | 移除焦点                  |        |
| focus()       | 获取焦点                  |        |
| nativeElement | 返回 Checkbox 的 DOM 节点 | 5.17.3 |

### 导入方式

```js
import { Checkbox } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultChecked` | 初始是否选中 | boolean | false | — |
| `disabled` | 失效状态 | boolean | false | — |
| `indeterminate` | 设置 indeterminate 状态，只负责样式控制 | boolean | false | — |
| `onChange` | 变化时的回调函数 | (e: CheckboxChangeEvent) => void | - | — |
| `onBlur` | 失去焦点时的回调 | function() | - | — |
| `onFocus` | 获得焦点时的回调 | function() | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 默认选中的选项 | (string \| number)\[] | \[] | — |
| `name` | CheckboxGroup 下所有 `input[type="checkbox"]` 的 `name` 属性 | string | - | — |
| `options` | 指定可选项 | string\[] \| number\[] \| Option\[] | \[] | — |
| `value` | 指定选中的选项 | (string \| number \| boolean)\[] | \[] | — |
| `title` | 选项的 title | `string` | - | — |
| `className` | 选项的类名 | `string` | - | 5.25.0 |
| `style` | 选项的样式 | `React.CSSProperties` | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `nativeElement` | 返回 Checkbox 的 DOM 节点 | — | — | 5.17.3 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Checkbox** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **7** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/checkbox
- 中文文档：https://ant.design/components/checkbox-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/checkbox
- 驱动 gpui kit：`checkbox`
