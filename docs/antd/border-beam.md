# BorderBeam 边框流光
> 来源：[Ant Design 6.5.x BorderBeam](https://ant.design/components/border-beam)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：为容器边框提供持续流动的装饰性高亮效果。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

为容器边框提供持续流动的装饰性高亮效果。

**BorderBeam** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基础用法 | 复现「基础用法」视觉与布局 |
| 鼠标悬浮时显示 | 复现「鼠标悬浮时显示」视觉与布局 |
| 自定义容器 | 自定义渲染/插槽外观 |
| 渐变色 | 渐变填充 |
| 动画时长 | 复现「动画时长」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 线宽 | 复现「线宽」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `color`

- **说明**：流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间
- **类型**：`string | { color: string; percent: number }[]`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `percent` | 官方取值 `percent` |

#### `duration`

- **说明**：流光完成一圈动画的时间，单位秒
- **类型**：number
- **默认值**：6
- **版本**：6.5.0

#### `lineWidth`

- **说明**：流光线宽，数字类型按像素处理
- **类型**：`number | string`
- **默认值**：`1px`
- **版本**：6.5.0

#### `outset`

- **说明**：流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0`
- **类型**：`number | string`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `0` | 官方取值 `0` |

#### `size`

- **说明**：流光可见段的尺寸，数字类型按像素处理
- **类型**：`number | string`
- **默认值**：100
- **版本**：6.5.0

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

- 需要强化某个容器的视觉关注度，但又不希望引入业务状态语义时。
- 适合登录面板、推荐卡片、AI 模块、重点 CTA 区域等场景。
- 它是装饰性效果，不应替代焦点态、校验态或业务状态边框。

### 2.2 核心功能（按官方示例拆解）

1. **基础用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **鼠标悬浮时显示**（`hover.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义容器**（`custom-container.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **渐变色**（`customized-color.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **动画时长**（`duration.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **线宽**（`line-width.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基础用法 | `basic.tsx` | 否 |
| 鼠标悬浮时显示 | `hover.tsx` | 否 |
| 自定义容器 | `custom-container.tsx` | 否 |
| 渐变色 | `customized-color.tsx` | 否 |
| 动画时长 | `duration.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 线宽 | `line-width.tsx` | 否 |
| 不规则圆角 | `non-uniform-radius.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 开启减少动态效果后会怎样？ {#faq-reduced-motion}

`BorderBeam` 会将流光视为装饰效果。当命中 `prefers-reduced-motion: reduce` 时，组件会隐藏 beam 效果。

### `color` 中的 `percent` 表示什么？ {#faq-color-percent}

`percent` 表示渐变停靠点的输入位置，取值范围为 `0 ~ 100`。组件会将这些停靠点映射到可见 beam 段内，并为尾部透明过渡保留空间，以保持流光尾迹连续可见。

### 为什么 `BorderBeam` 没有效果？ {#faq-not-working}

`BorderBeam` 需要通过 `children` 获取实际 DOM 节点，并将流光层插入到该节点中。请确保被包裹的内容是原生 DOM 元素，或是正确透传 `ref` 到 DOM 的 React 组件，否则组件无法定位真实容器，也就无法渲染流光效果。

流光层使用 `position: absolute` 定位，因此被索引到的 DOM 节点还需要提供定位上下文，通常可以为它设置 `position: relative`。`BorderBeam` 不会主动检测或修正子节点的定位样式。

为保证性能，`children` 是否可以插入以及其定位信息会在初始化时判断，后续不会持续监听子节点结构或定位样式变化。

### 如何让流光边框跟随容器圆角？ {#faq-radius}

`BorderBeam` 会在初始化时读取实际容器的计算后 `border-radius`。这个能力更适合 `Card` 这类单容器子节点场景；若子节点结构较复杂，建议直接把圆角写在实际容器根节点上，以获得更稳定的结果。

为保证性能，圆角计算完成后不会持续重新测量。后续由尺寸、祖先样式或子节点内部状态引起的圆角变化，不保证自动重新同步。动画轨迹在运行时可能会做内部平滑处理。

例如：

```tsx
const radius = 24;

  
;
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

### BorderBeam

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| children | 装饰内容 | `ReactNode` | - | 6.4.0 | × |
| color | 流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间 | `string \| { color: string; percent: number }[]` | - | 6.4.0 | × |
| duration | 流光完成一圈动画的时间，单位秒 | number | 6 | 6.5.0 | × |
| lineWidth | 流光线宽，数字类型按像素处理 | `number \| string` | `1px` | 6.5.0 | × |
| outset | 流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0` | `number \| string` | - | 6.4.0 | × |
| size | 流光可见段的尺寸，数字类型按像素处理 | `number \| string` | 100 | 6.5.0 | × |

### 导入方式

```js
import { BorderBeam } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `children` | 装饰内容 | `ReactNode` | - | 6.4.0 |
| `color` | 流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间 | `string \| { color: string; percent: number }[]` | - | 6.4.0 |
| `duration` | 流光完成一圈动画的时间，单位秒 | number | 6 | 6.5.0 |
| `lineWidth` | 流光线宽，数字类型按像素处理 | `number \| string` | `1px` | 6.5.0 |
| `outset` | 流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0` | `number \| string` | - | 6.4.0 |
| `size` | 流光可见段的尺寸，数字类型按像素处理 | `number \| string` | 100 | 6.5.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **BorderBeam** 的验收清单：

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
- 官方文档：https://ant.design/components/border-beam
- 中文文档：https://ant.design/components/border-beam-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/border-beam
- 驱动 gpui kit：`border-beam`
