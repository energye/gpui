# Progress 进度条
> 来源：[Ant Design 6.5.x Progress](https://ant.design/components/progress)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：展示操作的当前进度。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

展示操作的当前进度。

**Progress** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 进度条 | 进度条/圈 |
| 进度圈 | 进度条/圈 |
| 小型进度条 | 进度条/圈 |
| 响应式进度圈 | 进度条/圈 |
| 小型进度圈 | 进度条/圈 |
| 动态展示 | 复现「动态展示」视觉与布局 |
| 自定义文字格式 | 自定义渲染/插槽外观 |
| 仪表盘 | 复现「仪表盘」视觉与布局 |
| 分段进度条 | 进度条/圈 |
| 边缘形状 | 复现「边缘形状」视觉与布局 |
| 自定义进度条渐变色 | 进度条/圈 |
| 步骤进度条 | 进度条/圈 |
| 步骤进度圈 | 进度条/圈 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 改变进度数值位置 | placement 方位 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `percent`

- **说明**：百分比
- **类型**：number
- **默认值**：0

#### `railColor`

- **说明**：未完成的分段的颜色
- **类型**：string
- **默认值**：-

#### `showInfo`

- **说明**：是否显示进度数值或状态图标
- **类型**：boolean
- **默认值**：true

#### `status`

- **说明**：状态，可选：`success` `exception` `normal` `active`(仅限 line)
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `exception` | 官方取值 `exception` |
  | `normal` | 官方取值 `normal` |
  | `active` | 官方取值 `active` |

#### `strokeColor`

- **说明**：进度条的色彩
- **类型**：string
- **默认值**：-

#### `strokeLinecap`

- **说明**：进度条的样式
- **类型**：`round` | `butt` | `square`，区别详见 [stroke-linecap](https://developer.mozilla.org/docs/Web/SVG/Attribute/stroke-linecap)
- **默认值**：`round`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `round` | 大圆角/胶囊 |
  | `butt` | 官方取值 `butt` |
  | `square` | 方形 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `trailColor`

- **说明**：未完成的分段的颜色。已废弃，请使用 `railColor`
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `railColor` | 官方取值 `railColor` |

#### `type`

- **说明**：类型，可选 `line` `circle` `dashboard`
- **类型**：string
- **默认值**：`line`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `line` | 线型 |
  | `circle` | 圆形 |
  | `dashboard` | 官方取值 `dashboard` |

#### `size`

- **说明**：进度条的尺寸
- **类型**：number | \[number | string, number] | { width: number, height: number } | "small" | "medium"
- **默认值**："medium"
- **版本**：5.3.0, Object: 5.18.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |

#### `percentPosition`

- **说明**：进度数值位置，传入对象，`align` 表示数值的水平位置，`type` 表示数值在进度条内部还是外部
- **类型**：{ align: string; type: string }
- **默认值**：{ align: \"end\", type: \"outer\" }
- **版本**：5.18.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `align` | 官方取值 `align` |
  | `type` | 官方取值 `type` |

#### `strokeWidth`

- **说明**：圆形进度条线的宽度，单位是进度条画布宽度的百分比
- **类型**：number
- **默认值**：6

#### `gapPlacement`

- **说明**：仪表盘进度条缺口位置
- **类型**：`top` | `bottom` | `start` | `end`
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `gapPosition`

- **说明**：仪表盘进度条缺口位置，请使用 `gapPlacement` 替换
- **类型**：`top` | `bottom` | `left` | `right`
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `left` | 左侧 |
  | `right` | 右侧 |

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

在操作需要较长时间才能完成时，为用户显示该操作的当前进度和状态。

- 当一个操作会打断当前界面，或者需要在后台运行，且耗时可能超过 2 秒时；
- 当需要显示一个操作完成的百分比时。

### 2.2 核心功能（按官方示例拆解）

1. **进度条**（`line.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **进度圈**（`circle.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **小型进度条**（`line-mini.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **响应式进度圈**（`circle-micro.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **小型进度圈**（`circle-mini.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **动态展示**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义文字格式**（`format.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **仪表盘**（`dashboard.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **分段进度条**（`segment.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **边缘形状**（`linecap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义进度条渐变色**（`gradient-line.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **步骤进度条**（`steps.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **步骤进度圈**（`circle-steps.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **改变进度数值位置**（`info-position.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `percent` | 进度值 | 百分比 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 进度条 | `line.tsx` | 否 |
| 进度圈 | `circle.tsx` | 否 |
| 小型进度条 | `line-mini.tsx` | 否 |
| 响应式进度圈 | `circle-micro.tsx` | 否 |
| 小型进度圈 | `circle-mini.tsx` | 否 |
| 动态展示 | `dynamic.tsx` | 否 |
| 自定义文字格式 | `format.tsx` | 否 |
| 仪表盘 | `dashboard.tsx` | 否 |
| 分段进度条 | `segment.tsx` | 否 |
| 边缘形状 | `linecap.tsx` | 否 |
| 自定义进度条渐变色 | `gradient-line.tsx` | 否 |
| 步骤进度条 | `steps.tsx` | 否 |
| 步骤进度圈 | `circle-steps.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 改变进度数值位置 | `info-position.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

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

各类型共用的属性。

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| format | 内容的模板函数 | function(percent, successPercent) | (percent) => percent + `%` | - | × |
| percent | 百分比 | number | 0 | - | × |
| railColor | 未完成的分段的颜色 | string | - | - | × |
| showInfo | 是否显示进度数值或状态图标 | boolean | true | - | × |
| status | 状态，可选：`success` `exception` `normal` `active`(仅限 line) | string | - | - | × |
| strokeColor | 进度条的色彩 | string | - | - | × |
| strokeLinecap | 进度条的样式 | `round` \| `butt` \| `square`，区别详见 [stroke-linecap](https://developer.mozilla.org/docs/Web/SVG/Attribute/stroke-linecap) | `round` | - | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| success | 成功进度条相关配置 | { percent: number, strokeColor: string } | - | - | × |
| ~~trailColor~~ | 未完成的分段的颜色。已废弃，请使用 `railColor` | string | - | - | × |
| type | 类型，可选 `line` `circle` `dashboard` | string | `line` | - | × |
| size | 进度条的尺寸 | number \| \[number \| string, number] \| { width: number, height: number } \| "small" \| "medium" | "medium" | 5.3.0, Object: 5.18.0 | × |

### `type="line"`

| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| steps | 进度条总共步数 | number | - | - |
| rounding | 用于四舍五入数值的函数 | (step: number) => number | Math.round | 5.24.0 |
| strokeColor | 进度条的色彩，传入 object 时为渐变。当有 `steps` 时支持传入一个数组。 | string \| string[] \| { from: string; to: string; direction: string } | - | 4.21.0: `string[]` |
| percentPosition | 进度数值位置，传入对象，`align` 表示数值的水平位置，`type` 表示数值在进度条内部还是外部 | { align: string; type: string } | { align: \"end\", type: \"outer\" } | 5.18.0 |

### `type="circle"`

| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| steps | 进度条总共步数，传入 object 时，count 指步数，gap 指间隔大小。传 number 类型时，gap 默认为 2。 | number \| { count: number, gap: number } | - | 5.16.0 |
| strokeColor | 圆形进度条线的色彩，传入 object 时为渐变 | string \| { number%: string } | - | - |
| strokeWidth | 圆形进度条线的宽度，单位是进度条画布宽度的百分比 | number | 6 | - |

### `type="dashboard"`

| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| steps | 进度条总共步数，传入 object 时，count 指步数，gap 指间隔大小。传 number 类型时，gap 默认为 2。 | number \| { count: number, gap: number } | - | 5.16.0 |
| gapDegree | 仪表盘进度条缺口角度，可取值 0 ~ 295 | number | 75 | - |
| gapPlacement | 仪表盘进度条缺口位置 | `top` \| `bottom` \| `start` \| `end` | `bottom` | - |
| ~~gapPosition~~ | 仪表盘进度条缺口位置，请使用 `gapPlacement` 替换 | `top` \| `bottom` \| `left` \| `right` | `bottom` | - |
| strokeWidth | 仪表盘进度条线的宽度，单位是进度条画布宽度的百分比 | number | 6 | - |

### 导入方式

```js
import { Progress } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `format` | 内容的模板函数 | function(percent, successPercent) | (percent) => percent + `%` | - |
| `percent` | 百分比 | number | 0 | - |
| `railColor` | 未完成的分段的颜色 | string | - | - |
| `showInfo` | 是否显示进度数值或状态图标 | boolean | true | - |
| `status` | 状态，可选：`success` `exception` `normal` `active`(仅限 line) | string | - | - |
| `strokeColor` | 进度条的色彩 | string | - | - |
| `strokeLinecap` | 进度条的样式 | `round` \| `butt` \| `square`，区别详见 [stroke-linecap](https://developer.mozilla.org/docs/Web/SVG/Attribute/stroke-linecap) | `round` | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `success` | 成功进度条相关配置 | { percent: number, strokeColor: string } | - | - |
| `trailColor` | 未完成的分段的颜色。已废弃，请使用 `railColor` | string | - | - |
| `type` | 类型，可选 `line` `circle` `dashboard` | string | `line` | - |
| `size` | 进度条的尺寸 | number \| \[number \| string, number] \| { width: number, height: number } \| "small" \| "medium" | "medium" | 5.3.0, Object: 5.18.0 |
| `steps` | 进度条总共步数 | number | - | - |
| `rounding` | 用于四舍五入数值的函数 | (step: number) => number | Math.round | 5.24.0 |
| `percentPosition` | 进度数值位置，传入对象，`align` 表示数值的水平位置，`type` 表示数值在进度条内部还是外部 | { align: string; type: string } | { align: \"end\", type: \"outer\" } | 5.18.0 |
| `strokeWidth` | 圆形进度条线的宽度，单位是进度条画布宽度的百分比 | number | 6 | - |
| `gapDegree` | 仪表盘进度条缺口角度，可取值 0 ~ 295 | number | 75 | - |
| `gapPlacement` | 仪表盘进度条缺口位置 | `top` \| `bottom` \| `start` \| `end` | `bottom` | - |
| `gapPosition` | 仪表盘进度条缺口位置，请使用 `gapPlacement` 替换 | `top` \| `bottom` \| `left` \| `right` | `bottom` | - |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Progress** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **16** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/progress
- 中文文档：https://ant.design/components/progress-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/progress
- 驱动 gpui kit：`progress`
