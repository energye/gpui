# Badge 徽标数
> 来源：[Ant Design 6.5.x Badge](https://ant.design/components/badge)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：图标右上角的圆形徽标数字。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

图标右上角的圆形徽标数字。

**Badge** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 独立使用 | 复现「独立使用」视觉与布局 |
| 封顶数字 | 复现「封顶数字」视觉与布局 |
| 讨嫌的小红点 | 复现「讨嫌的小红点」视觉与布局 |
| 动态 | 复现「动态」视觉与布局 |
| 可点击 | 复现「可点击」视觉与布局 |
| 自定义位置偏移 | placement 方位 |
| 大小 | 不同 size 档位 |
| 状态点 | 复现「状态点」视觉与布局 |
| 多彩徽标 | Badge 叠加 |
| 缎带 | 复现「缎带」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `color`

- **说明**：自定义小圆点的颜色
- **类型**：string
- **默认值**：-

#### `count`

- **说明**：展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `dot`

- **说明**：不展示数字，只有一个小红点
- **类型**：boolean
- **默认值**：false

#### `offset`

- **说明**：设置状态点的位置偏移
- **类型**：\[number, number]
- **默认值**：-

#### `overflowCount`

- **说明**：展示封顶的数字值
- **类型**：number
- **默认值**：99

#### `showZero`

- **说明**：当数值为 0 时，是否展示 Badge
- **类型**：boolean
- **默认值**：false

#### `size`

- **说明**：在设置了 `count` 的前提下有效，设置小圆点的大小
- **类型**：`medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：设置 Badge 为状态点
- **类型**：`success` | `processing` | `default` | `error` | `warning`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `processing` | 进行中 |
  | `default` | 默认中性外观 |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `text`

- **说明**：在设置了 `status` 的前提下有效，设置状态点的文本
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip
- **类型**：string | null | false
- **默认值**：-
- **版本**：6.5.0

#### `placement`

- **说明**：缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动
- **类型**：`start` | `end`
- **默认值**：`end`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

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

一般出现在通知图标或头像的右上角，用于显示需要处理的消息条数，通过醒目视觉形式吸引用户处理。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **独立使用**（`no-wrapper.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **封顶数字**（`overflow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **讨嫌的小红点**（`dot.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **动态**（`change.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **可点击**（`link.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义位置偏移**（`offset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **状态点**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **多彩徽标**（`colorful.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **缎带**（`ribbon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 独立使用 | `no-wrapper.tsx` | 否 |
| 封顶数字 | `overflow.tsx` | 否 |
| 讨嫌的小红点 | `dot.tsx` | 否 |
| 动态 | `change.tsx` | 否 |
| 可点击 | `link.tsx` | 否 |
| 自定义位置偏移 | `offset.tsx` | 否 |
| 大小 | `size.tsx` | 否 |
| 状态点 | `status.tsx` | 否 |
| 多彩徽标 | `colorful.tsx` | 否 |
| 缎带 | `ribbon.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Ribbon Debug | `ribbon-debug.tsx` | 是 |
| 各种混用的情况 | `mix.tsx` | 是 |
| 自定义标题 | `title.tsx` | 是 |
| 多彩徽标支持 count 显示 Debug | `colorful-with-count-debug.tsx` | 是 |
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

### Badge

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| color | 自定义小圆点的颜色 | string | - | count | 展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | dot | 不展示数字，只有一个小红点 | boolean | false | offset | 设置状态点的位置偏移 | \[number, number] | - | overflowCount | 展示封顶的数字值 | number | 99 | showZero | 当数值为 0 时，是否展示 Badge | boolean | false | size | 在设置了 `count` 的前提下有效，设置小圆点的大小 | `medium` \| `small` | - | - | × |
| status | 设置 Badge 为状态点 | `success` \| `processing` \| `default` \| `error` \| `warning` | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | text | 在设置了 `status` 的前提下有效，设置状态点的文本 | ReactNode | - | title | 设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip | string \| null \| false | - | 6.5.0 | × |

### Badge.Ribbon

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | color | 自定义缎带的颜色 | string | - | placement | 缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动 | `start` \| `end` | `end` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | text | 缎带中填入的内容 | ReactNode | - 
### 导入方式

```js
import { Badge } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `color` | 自定义小圆点的颜色 | string | - | — |
| `count` | 展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dot` | 不展示数字，只有一个小红点 | boolean | false | — |
| `offset` | 设置状态点的位置偏移 | \[number, number] | - | — |
| `overflowCount` | 展示封顶的数字值 | number | 99 | — |
| `showZero` | 当数值为 0 时，是否展示 Badge | boolean | false | — |
| `size` | 在设置了 `count` 的前提下有效，设置小圆点的大小 | `medium` \| `small` | - | - |
| `status` | 设置 Badge 为状态点 | `success` \| `processing` \| `default` \| `error` \| `warning` | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `text` | 在设置了 `status` 的前提下有效，设置状态点的文本 | ReactNode | - | — |
| `title` | 设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip | string \| null \| false | - | 6.5.0 |
| `placement` | 缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动 | `start` \| `end` | `end` | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Badge** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **12** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/badge
- 中文文档：https://ant.design/components/badge-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/badge
- 驱动 gpui kit：`badge`
