# Rate 评分
> 来源：[Ant Design 6.5.x Rate](https://ant.design/components/rate)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于对事物进行评分操作。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于对事物进行评分操作。

**Rate** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 半星 | 复现「半星」视觉与布局 |
| 文案展现 | 复现「文案展现」视觉与布局 |
| 只读 | 复现「只读」视觉与布局 |
| 清除 | 复现「清除」视觉与布局 |
| 其他字符 | 复现「其他字符」视觉与布局 |
| 自定义字符 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：是否允许再次点击后清除
- **类型**：boolean
- **默认值**：true

#### `count`

- **说明**：star 总数
- **类型**：number
- **默认值**：5

#### `disabled`

- **说明**：只读，无法进行交互
- **类型**：boolean
- **默认值**：false

#### `keyboard`

- **说明**：支持使用键盘操作
- **类型**：boolean
- **默认值**：true
- **版本**：5.18.0

#### `size`

- **说明**：星星尺寸
- **类型**：'small' | 'medium' | 'large'
- **默认值**：'medium'
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

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

- 对评价进行展示。
- 对事物进行快速的评级操作。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **半星**（`half.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **文案展现**（`text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **只读**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **清除**（`clear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **其他字符**（`character.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义字符**（`character-function.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前数，受控值 |
| `defaultValue` | 非受控默认值 | 默认值 |
| `onChange` | 值变化 | 选择时的回调 |
| `disabled` | 禁用 | 只读，无法进行交互 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 半星 | `half.tsx` | 否 |
| 文案展现 | `text.tsx` | 否 |
| 只读 | `disabled.tsx` | 否 |
| 清除 | `clear.tsx` | 否 |
| 其他字符 | `character.tsx` | 否 |
| 自定义字符 | `character-function.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

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

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 是否允许再次点击后清除 | boolean | true | allowHalf | 是否允许半选 | boolean | false | character | 自定义字符 | ReactNode \| (RateProps) => ReactNode | &lt;StarFilled /> | function(): 4.4.0 | × |
| count | star 总数 | number | 5 | defaultValue | 默认值 | number | 0 | disabled | 只读，无法进行交互 | boolean | false | keyboard | 支持使用键盘操作 | boolean | true | 5.18.0 | × |
| size | 星星尺寸 | 'small' \| 'medium' \| 'large' | 'medium' | tooltips | 自定义每项的提示信息 | [TooltipProps](/components/tooltip-cn#api)[] \| string\[] | - | value | 当前数，受控值 | number | - | onBlur | 失去焦点时的回调 | function() | - | onChange | 选择时的回调 | function(value: number) | - | onFocus | 获取焦点时的回调 | function() | - | onHoverChange | 鼠标经过时数值变化的回调 | function(value: number) | - | onKeyDown | 按键回调 | function(event) | - 
## 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 导入方式

```js
import { Rate } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 是否允许再次点击后清除 | boolean | true | — |
| `allowHalf` | 是否允许半选 | boolean | false | — |
| `character` | 自定义字符 | ReactNode \| (RateProps) => ReactNode | <StarFilled /> | function(): 4.4.0 |
| `count` | star 总数 | number | 5 | — |
| `defaultValue` | 默认值 | number | 0 | — |
| `disabled` | 只读，无法进行交互 | boolean | false | — |
| `keyboard` | 支持使用键盘操作 | boolean | true | 5.18.0 |
| `size` | 星星尺寸 | 'small' \| 'medium' \| 'large' | 'medium' | — |
| `tooltips` | 自定义每项的提示信息 | [TooltipProps](/components/tooltip-cn#api)[] \| string\[] | - | — |
| `value` | 当前数，受控值 | number | - | — |
| `onBlur` | 失去焦点时的回调 | function() | - | — |
| `onChange` | 选择时的回调 | function(value: number) | - | — |
| `onFocus` | 获取焦点时的回调 | function() | - | — |
| `onHoverChange` | 鼠标经过时数值变化的回调 | function(value: number) | - | — |
| `onKeyDown` | 按键回调 | function(event) | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Rate** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/rate
- 中文文档：https://ant.design/components/rate-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/rate
- 驱动 gpui kit：`rate`
