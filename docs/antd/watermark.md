# Watermark 水印
> 来源：[Ant Design 6.5.x Watermark](https://ant.design/components/watermark)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：给页面的某个区域加上水印。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

给页面的某个区域加上水印。

**Watermark** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 多行水印 | 复现「多行水印」视觉与布局 |
| 图片水印 | 复现「图片水印」视觉与布局 |
| 自定义配置 | 自定义渲染/插槽外观 |
| Modal 与 Drawer | 复现「Modal 与 Drawer」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `width`

- **说明**：水印的宽度，`content` 的默认值为自身的宽度
- **类型**：number
- **默认值**：120
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `content` | 官方取值 `content` |

#### `height`

- **说明**：水印的高度，`content` 的默认值为自身的高度
- **类型**：number
- **默认值**：64
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `content` | 官方取值 `content` |

#### `zIndex`

- **说明**：追加的水印元素的 z-index
- **类型**：number
- **默认值**：999

#### `content`

- **说明**：水印文字内容
- **类型**：string | [WatermarkText](#watermarktext) | (string | [WatermarkText](#watermarktext))[]
- **默认值**：-
- **版本**：WatermarkText: 6.5.0

#### `font`

- **说明**：文字样式
- **类型**：[Font](#font)
- **默认值**：[Font](#font)

#### `gap`

- **说明**：水印之间的间距
- **类型**：\[number, number\]
- **默认值**：\[100, 100\]

#### `offset`

- **说明**：水印距离容器左上角的偏移量，默认为 `gap/2`
- **类型**：\[number, number\]
- **默认值**：\[gap\[0\]/2, gap\[1\]/2\]

#### `text`

- **说明**：单行文字内容
- **类型**：string
- **默认值**：-
- **版本**：6.5.0

#### `color`

- **说明**：字体颜色
- **类型**：[CanvasFillStrokeStyles.fillStyle](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/fillStyle)
- **默认值**：rgba(0,0,0,.15)

#### `fontSize`

- **说明**：字体大小
- **类型**：number
- **默认值**：16

#### `fontFamily`

- **说明**：字体类型
- **类型**：string
- **默认值**：sans-serif

#### `fontStyle`

- **说明**：字体样式
- **类型**：`none` | `normal` | `italic` | `oblique`
- **默认值**：normal
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `none` | 官方取值 `none` |
  | `normal` | 官方取值 `normal` |
  | `italic` | 官方取值 `italic` |
  | `oblique` | 官方取值 `oblique` |

#### `textAlign`

- **说明**：指定文本对齐方向
- **类型**：[CanvasTextAlign](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/textAlign)
- **默认值**：`center`
- **版本**：5.10.0

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

- 页面需要添加水印标识版权时使用。
- 适用于防止信息盗用。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **多行水印**（`multi-line.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **图片水印**（`image.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义配置**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **Modal 与 Drawer**（`portal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onRemove` | 移除 | 水印因 DOM 变更被移除时触发的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 多行水印 | `multi-line.tsx` | 否 |
| 图片水印 | `image.tsx` | 否 |
| 自定义配置 | `custom.tsx` | 否 |
| Modal 与 Drawer | `portal.tsx` | 否 |
| Table 固定列 | `debug.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 处理异常图片水印 {#faq-invalid-image}

当使用图片水印且图片加载异常时，可以同时添加 `content` 防止水印失效（自 5.2.3 开始支持）。

```typescript jsx

  

```

### 从 5.18.0 版本后，为什么添加了 `overflow: hidden` 样式？ {#faq-overflow-hidden}

在之前版本，用户可以通过开发者工具将容器高度设置为 0 来隐藏水印，为了避免这种情况，我们在容器上添加了 `overflow: hidden` 样式。当容器高度变化时，则内容也一同被隐藏。你可以通过覆盖样式来修改这个行为：

```tsx

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

> 自 `antd@5.1.0` 版本开始提供该组件。

### Watermark

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| width | 水印的宽度，`content` 的默认值为自身的宽度 | number | 120 | height | 水印的高度，`content` 的默认值为自身的高度 | number | 64 | inherit | 是否将水印传导给弹出组件如 Modal、Drawer | boolean | true | 5.11.0 | × |
| rotate | 水印绘制时，旋转的角度，单位 `°` | number | -22 | zIndex | 追加的水印元素的 z-index | number | 999 | image | 图片源，建议导出 2 倍或 3 倍图，优先级高 (支持 base64 格式) | string | - | content | 水印文字内容 | string \| [WatermarkText](#watermarktext) \| (string \| [WatermarkText](#watermarktext))[] | - | WatermarkText: 6.5.0 | × |
| font | 文字样式 | [Font](#font) | [Font](#font) | gap | 水印之间的间距 | \[number, number\] | \[100, 100\] | offset | 水印距离容器左上角的偏移量，默认为 `gap/2` | \[number, number\] | \[gap\[0\]/2, gap\[1\]/2\] | onRemove | 水印因 DOM 变更被移除时触发的回调 | `() => void` | - | 6.0.0 | × |

### WatermarkText

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| font | 自定义单行文字样式 | [Font](#font) | - | 6.5.0 |
| text | 单行文字内容 | string | - | 6.5.0 |

### Font

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| color | 字体颜色 | [CanvasFillStrokeStyles.fillStyle](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/fillStyle) | rgba(0,0,0,.15) | fontWeight | 字体粗细 | `normal` \| `lighter` \| `bold` \| `bolder` \| number | normal | fontStyle | 字体样式 | `none` \| `normal` \| `italic` \| `oblique` | normal 
### 导入方式

```js
import { Watermark } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `width` | 水印的宽度，`content` 的默认值为自身的宽度 | number | 120 | — |
| `height` | 水印的高度，`content` 的默认值为自身的高度 | number | 64 | — |
| `inherit` | 是否将水印传导给弹出组件如 Modal、Drawer | boolean | true | 5.11.0 |
| `rotate` | 水印绘制时，旋转的角度，单位 `°` | number | -22 | — |
| `zIndex` | 追加的水印元素的 z-index | number | 999 | — |
| `image` | 图片源，建议导出 2 倍或 3 倍图，优先级高 (支持 base64 格式) | string | - | — |
| `content` | 水印文字内容 | string \| [WatermarkText](#watermarktext) \| (string \| [WatermarkText](#watermarktext))[] | - | WatermarkText: 6.5.0 |
| `font` | 文字样式 | [Font](#font) | [Font](#font) | — |
| `gap` | 水印之间的间距 | \[number, number\] | \[100, 100\] | — |
| `offset` | 水印距离容器左上角的偏移量，默认为 `gap/2` | \[number, number\] | \[gap\[0\]/2, gap\[1\]/2\] | — |
| `onRemove` | 水印因 DOM 变更被移除时触发的回调 | `() => void` | - | 6.0.0 |
| `text` | 单行文字内容 | string | - | 6.5.0 |
| `color` | 字体颜色 | [CanvasFillStrokeStyles.fillStyle](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/fillStyle) | rgba(0,0,0,.15) | — |
| `fontSize` | 字体大小 | number | 16 | — |
| `fontWeight` | 字体粗细 | `normal` \| `lighter` \| `bold` \| `bolder` \| number | normal | — |
| `fontFamily` | 字体类型 | string | sans-serif | — |
| `fontStyle` | 字体样式 | `none` \| `normal` \| `italic` \| `oblique` | normal | — |
| `textAlign` | 指定文本对齐方向 | [CanvasTextAlign](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/textAlign) | `center` | 5.10.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Watermark** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **5** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/watermark
- 中文文档：https://ant.design/components/watermark-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/watermark
- 驱动 gpui kit：`watermark`
