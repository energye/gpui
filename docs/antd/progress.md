# Progress 进度条
> 来源：[Ant Design 6.5.x Progress](https://ant.design/components/progress)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：展示操作的当前进度。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Progress** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/progress/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Progress）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Progress）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：展示操作的当前进度。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落（对照 `components/progress/utils.ts` + `style/index.ts`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| line 轨高 medium | **8** | `TokenProgressHeight` / `getSize(…,'line')` |
| line 轨高 small | **6** | size=`small` |
| line 轨圆角 | **半高**（胶囊） | antd `lineBorderRadius`≈100 → clamp 半高 |
| line 与 info 间距 | **8** | `marginXS` |
| circle / dashboard 默认边长 | **120** | `getSize` medium |
| circle / dashboard small 边长 | **60** | size=`small` |
| circle 线宽（相对边长 %） | **6** | `strokeWidth` 默认；且 ≥ `3/size*100` |
| dashboard 默认 gapDegree | **75** | `Circle.tsx` |
| dashboard 默认 gapPlacement | **bottom** | |
| 环内文字字号 | ≈ `size*0.15+6` | antd circleStyle.fontSize |
| 字号（line info） | **14** | `fontSize` |
| 线宽 fallback（无父约束） | **160** | kit 桌面默认（antd 为 100% 宽） |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 进度填充 defaultColor | `colorPrimary` | status=normal/active |
| 成功填充 | `colorSuccess` | status=success 或 percent≥100 自动 |
| 异常填充 | `colorError` | status=exception |
| 轨道 remainingColor / rail | `colorFillSecondary` | `railColor` 可覆盖 |
| 文本 / 环心文字 | `colorText` / `colorTextSecondary` | success/exception 时 info 跟 status 色 |
| 边框 / 容器底 | `colorBorder` / `colorBgContainer` | active 闪光近似用 bgContainer |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `format` | 内容的模板函数 | function(percent, successPercent) | (percent) => percent + `%` |
| `percent` | 百分比 | number | 0 |
| `railColor` | 未完成的分段的颜色 | string | - |
| `showInfo` | 是否显示进度数值或状态图标 | boolean | true |
| `status` | 状态，可选：`success` `exception` `normal` `active`(仅限 line) | string | - |
| `strokeColor` | 进度条的色彩 | string | - |
| `strokeLinecap` | 进度条的样式 | `round` \ | `butt` \ |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `success` | 成功进度条相关配置 | { percent: number, strokeColor: string } | - |
| `type` | 类型，可选 `line` `circle` `dashboard` | string | `line` |
| `size` | 进度条的尺寸 | number \ | \[number \ |
| `steps` | 进度条总共步数 | number | - |
| `rounding` | 用于四舍五入数值的函数 | (step: number) => number | Math.round |
| `percentPosition` | 进度数值位置，传入对象，`align` 表示数值的水平位置，`type` 表示数值在进度条内部还是外部 | { align: string; type: string } | { align: \"end\", type: \"outer\" } |
| `strokeWidth` | 圆形进度条线的宽度，单位是进度条画布宽度的百分比 | number | 6 |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
percent=p ∈ [0,100] ──► 线填充宽 / 圆弧扫角 = p%
status 显式 ∈ {normal, active, exception, success}
  未设且 percent≥100 ──► 自动 success（antd progressStatus）
  未设且 percent<100 ──► normal
type=line|circle|dashboard ──► 线 / 全环 / 缺口环（dashboard gapDegree 默认 75）
showInfo=false ──► 无百分比与状态图标
format 设 ──► info 文案走 format(percent, successPercent)；覆盖默认 `%` 与成功/异常图标
status=active（仅 line）──► 轨上闪光/扫光；Ticker 驱动；非 line 忽略动画
size=small|medium|number ──► 线高 6|8；环边长 60|120|自定义
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| PRG-S1 | percent=50 type=line | 轨填充约一半 |
| PRG-S2 | percent=100 且未显式 status | 自动 success（色+图标） |
| PRG-S3 | status=exception | 错误色；info 为异常图标/色 |
| PRG-S4 | type=circle | 环形；默认边长 120 |
| PRG-S5 | showInfo=false | 无百分比数字与状态图标 |
| PRG-S6 | size=medium type=line | 线高 ≈8 |
| PRG-S7 | type=circle 默认 size | 边长 ≈120 |
| PRG-S8 | type=dashboard | 缺口环；默认 gapDegree=75 |
| PRG-S9 | status=active type=line | Ticker 扫光；SetPercent 不整树 rebuild |
| PRG-S10 | format 自定义 | info 显示 format 返回值 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| normal | 填充 primary；info 为 `p%`（或 format） |
| active（line） | 同 normal 色 + 扫光层（P0 近似，非像素级 CSS keyframes） |
| success | 填充 success；无 format 时 line 用成功图标、circle 用中心勾 |
| exception | 填充 error；无 format 时 line 用异常图标、circle 用中心叉 |
| showInfo=false | 无 info 节点 |
| 主题切换 | 色与间距随 Theme / Token 更新 |

**动效：** `status=active` 的扫光跟 Host Ticker；P0 允许简化；尊重 reduced-motion 时可静止。percent 变更优先改几何/脏区，**禁止**无结构变化时整树 rebuild。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| role | 根节点 `progressbar` |
| 值 | `aria-valuenow`≈percent；`valuemin=0` `valuemax=100`（Label/Value 字段承载） |
| 名 | `SetAriaLabel` 或默认 `"{percent} percent"` / format 文案 |
| 焦点 | **非**键盘操作控件；不抢焦点、无强制 tabIndex |
| 实时 | 值变更更新 accessible name；不必 live region |

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
| `size` | small / medium / 自定义 SizePx |
| `type` | line / circle / dashboard |
| `status` | normal / exception / active(line) / success；空=自动 |
| `percent` | 0..100；SetPercent 不整树 rebuild |
| `showInfo` / `format` | 默认 true；自定义文案 |
| `type=dashboard` + gapDegree | 默认 gap 75 / placement bottom |
| `status=active`（line） | Ticker 扫光近似 |
| 官方主路径示例 | 进度条、进度圈、小型进度条、响应式进度圈、小型进度圈、动态展示、自定义文字格式、仪表盘 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | role=progressbar + 值名 |
| §6.9 中 L1/L2 P0 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| `steps` 线/圈分段 | 分期 |
| `strokeLinecap` butt/square、渐变 `strokeColor` object | 分期 |
| `percentPosition` inner/outer/align | 分期 |
| `success.percent` 双色分段 | 分期 |
| semantic classNames/styles 深度 | 分期 |
| 动画像素级 active keyframes | 分期 |
| ConfigProvider 全局 progress 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 分段进度条、边缘形状、自定义渐变、步骤进度条/圈、尺寸全矩阵、info-position |

### 6.9 验收用例表（可测）

> 测试名建议：`TestProgress_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关 L1/L2 用例全部通过** 才可宣称 Progress 完成 1:1 主路径。  
> L3/L4 与 P1 不阻塞本阶段 DoD（可另测）。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| PRG-01 | L1 | NewProgress 默认创建 | type=line、size=medium、showInfo=true、status 空（自动）、percent 入参 |
| PRG-02 | L1 | percent=50 type=line | 轨填充约一半（FillRatio≈0.5） |
| PRG-03 | L1 | percent=100 未设 status | effectiveStatus=success |
| PRG-04 | L1 | status=exception | 填充/info 用 error 色 |
| PRG-05 | L1 | type=circle | 环形节点；默认边长≈120 |
| PRG-06 | L1 | showInfo=false | 无百分比数字节点 |
| PRG-07 | L1 | size=medium type=line | 线高≈8 |
| PRG-08 | L1 | type=circle 默认 | 边长≈120 |
| PRG-09 | P1 | steps 线 | 分段显示（本阶段不做） |
| PRG-10 | L1 | 复现「进度条」`line.tsx` | 30/50 active/70 exception/100/50 hideInfo 可构建 |
| PRG-11 | L1 | 复现「进度圈」`circle.tsx` | 75 / 70 exception / 100 可构建 |
| PRG-12 | L1 | 复现「小型进度条」`line-mini.tsx` | size=small 线高≈6 |
| PRG-13 | L1 | 复现「响应式进度圈」`circle-micro.tsx` | size 小数 + strokeWidth + format 可构建 |
| PRG-14 | L1 | 复现「小型进度圈」`circle-mini.tsx` | size=80 三环可构建 |
| PRG-15 | L1 | 复现「动态展示」`dynamic.tsx` | SetPercent ±10 不整树 rebuild 根 |
| PRG-16 | L1 | 复现「自定义文字格式」`format.tsx` | format 返回值出现在 info |
| PRG-17 | L1 | 复现「仪表盘」`dashboard.tsx` | type=dashboard + gapDegree 可构建 |
| PRG-18 | L2 | §6.2 关键尺寸 | line 8/6、circle 120/60、info gap≈8（±0.5） |
| PRG-19 | L2 | 默认皮颜色 | 走 Theme Token；无硬编码品牌蓝/绿/红为唯一皮 |
| PRG-20 | — | disabled（不适用） | Progress **无** disabled；跳过 |
| PRG-21 | L1 | a11y 主路径 | role=progressbar；Label/值反映 percent；非焦点控件 |
| PRG-22 | L3 | 关键态 golden | 与仓库基线一致（另轨） |
| PRG-23 | L4 | 与 ant.design 并排 | 人眼签字（另轨） |
| PRG-24 | P1 | steps/gradient/linecap/percentPosition 等 | Notes 标明；本阶段不做 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约，实现可微调命名但语义不可丢。

```text
NewProgress(percent float64) *Progress

// 形态
SetType(ProgressType)              // line | circle | dashboard
SetSize(ProgressSize)              // small | medium（预设）
SetSizePx(float64)                 // 自定义：line 高或 circle 边长；0=用 Size 预设
SetWidth(float64)                  // line 轨宽；0=父约束或 fallback 160
SetStrokeWidth(float64)            // circle 线宽（相对边长 %）；0=默认 6
SetGapDegree(float64)              // dashboard 缺口角度；未设 dashboard 默认 75
SetGapPlacement(ProgressGapPlacement) // top|bottom|start|end；dashboard 默认 bottom

// 值与状态
SetPercent(float64)                // clamp 0..100；禁止无结构变化时 rebuild 根
SetStatus(ProgressStatus)          // ""=自动 | normal|exception|active|success
SetShowInfo(bool)                  // 默认 true
SetFormat(func(percent, successPercent float64) string) // nil=默认

// 色（可选覆盖；零值/未 Set 走 Token）
SetStrokeColor(render.RGBA)
SetRailColor(render.RGBA)          // 亦兼容 trailColor 语义

// 主题 / a11y / 挂树
SetTheme(*core.Theme)
SetAriaLabel(string)
Node() core.Node
AttachTicker(*core.Tree)           // status=active 扫光；OnMount 亦可自动绑
EffectiveStatus() ProgressStatus   // 含 percent≥100 自动 success
LineHeight() / CircleSize() float64
FillRatio() float64                // 0..1，测填充
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Type | line |
| Size | medium |
| SizePx / Width / StrokeWidth | 0（走预设 / fallback） |
| Status | 空（自动：≥100→success，否则 normal） |
| ShowInfo | true |
| Format | nil → `"{percent}%"`；success/exception 且无 format 时用状态图标 |
| GapDegree | dashboard 未设 → 75 |
| GapPlacement | dashboard → bottom |
| Percent | NewProgress 入参（clamp） |

### 6.11 结构与绘制分层（实现提示）

```text
progressHost（非 RepaintBoundary；OnMount 绑 Ticker）
  └─ line:
       Row(gap=marginXS) CrossCenter
         progressTrack（Layout 定宽高；Paint rail+fill+active 扫光）
         info Text?（outer end）
  └─ circle|dashboard:
       Stack / 定边长盒
         progressRing Canvas（轨弧 + 进度弧；dashboard 留 gapDegree）
         居中 info Text?（% / format / 勾叉）
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；`SetPercent` / active Tick 只改脏画与 info 文案。  
- 命中区域与布局盒一致（`hit == layout == paint`）；**禁止** Progress 根 `RepaintBoundary`（ScrollViewport 嵌套会留洞）。  
- active 扫光跟随 Host Tick；非 active 不挂 Ticker。

### 6.12 完成定义（DoD）

同时满足即可宣布 **Progress 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 相关 L1/L2**（PRG-01…08、10…19、21）测试通过；PRG-09/20/22–24 不阻塞。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若仓库已有 Progress golden 则保持）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Progress 页按 §6.8 P0 官方示例重铺；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/progress.md` §6；P1 显式列出。  


---

**本章用法**：实现 `ui/kit` Progress 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
