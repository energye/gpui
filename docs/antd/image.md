# Image 图片
> 来源：[Ant Design 6.5.x Image](https://ant.design/components/image)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：可预览的图片。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

可预览的图片。

**Image** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 渐进加载 | loading 指示与防重复 |
| 容错处理 | 复现「容错处理」视觉与布局 |
| 多张图片预览 | 复现「多张图片预览」视觉与布局 |
| 相册模式 | 复现「相册模式」视觉与布局 |
| 自定义预览图片 | 自定义渲染/插槽外观 |
| 受控的预览 | 复现「受控的预览」视觉与布局 |
| 自定义工具栏 | 自定义渲染/插槽外观 |
| 自定义预览内容 | 自定义渲染/插槽外观 |
| 预览遮罩 | mask 层 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 嵌套 | 复现「嵌套」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `alt`

- **说明**：图像描述
- **类型**：string
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `fallback`

- **说明**：加载失败容错地址
- **类型**：string
- **默认值**：-

#### `height`

- **说明**：图像高度
- **类型**：string | number
- **默认值**：-

#### `placeholder`

- **说明**：加载占位，支持 ReactNode 或配置对象
- **类型**：[PlaceholderType](#placeholdertype)
- **默认值**：-

#### `preview`

- **说明**：预览参数，为 `false` 时禁用
- **类型**：boolean | [PreviewType](#previewtype)
- **默认值**：true

#### `src`

- **说明**：图片地址
- **类型**：string
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `width`

- **说明**：图像宽度
- **类型**：string | number
- **默认值**：-

#### `onError`

- **说明**：加载错误回调
- **类型**：(event: Event) => void
- **默认值**：-

#### `percent`

- **说明**：进度值
- **类型**：number
- **默认值**：-

#### `cover`

- **说明**：自定义预览遮罩
- **类型**：React.ReactNode | [CoverConfig](#coverconfig)
- **默认值**：-
- **版本**：CoverConfig v6.0 开始支持

#### `getContainer`

- **说明**：指定预览挂载的节点，但依旧为全屏展示，false 为挂载在当前位置
- **类型**：string | HTMLElement | (() => HTMLElement) | false
- **默认值**：-

#### `mask`

- **说明**：预览遮罩效果
- **类型**：boolean | { enabled?: boolean, blur?: boolean, closable?: boolean }
- **默认值**：true
- **版本**：mask.closable: 6.4.0

#### `maskClassName`

- **说明**：缩略图遮罩类名，请使用 `classNames.cover` 替换
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `classNames.cover` | 官方取值 `classNames.cover` |

#### `onOpenChange`

- **说明**：预览打开状态变化的回调
- **类型**：(visible: boolean) => void
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

- 需要展示图片时使用。
- 加载显示大图或加载失败时容错处理。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **渐进加载**（`placeholder.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **容错处理**（`fallback.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **多张图片预览**（`preview-group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **相册模式**（`preview-group-visible.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义预览图片**（`previewSrc.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **受控的预览**（`controlled-preview.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义工具栏**（`toolbarRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义预览内容**（`imageRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **预览遮罩**（`mask.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **嵌套**（`nested.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 切换预览图的回调 |
| `open` | 受控显隐 | 是否显示预览 |
| `onOpenChange` | 显隐变化 | 预览打开状态变化的回调 |
| `items` | 数据化 items | 预览数组 |
| `current` | 当前步骤/页 | 当前预览图的 index |
| `percent` | 进度值 | 进度值 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 渐进加载 | `placeholder.tsx` | 否 |
| 容错处理 | `fallback.tsx` | 否 |
| 多张图片预览 | `preview-group.tsx` | 否 |
| 相册模式 | `preview-group-visible.tsx` | 否 |
| 自定义预览图片 | `previewSrc.tsx` | 否 |
| 受控的预览 | `controlled-preview.tsx` | 否 |
| 自定义工具栏 | `toolbarRender.tsx` | 否 |
| 自定义预览内容 | `imageRender.tsx` | 否 |
| 预览遮罩 | `mask.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义预览文本 | `preview-mask.tsx` | 是 |
| 自定义预览遮罩位置 | `coverPlacement.tsx` | 是 |
| 嵌套 | `nested.tsx` | 否 |
| 多图预览时顶部进度自定义 | `preview-group-top-progress.tsx` | 是 |
| 自定义组件 Token | `component-token.tsx` | 是 |
| 在渲染函数中获取图片信息 | `preview-imgInfo.tsx` | 是 |

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

### Image

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| alt | 图像描述 | string | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | fallback | 加载失败容错地址 | string | - | height | 图像高度 | string \| number | - | placeholder | 加载占位，支持 ReactNode 或配置对象 | [PlaceholderType](#placeholdertype) | - | preview | 预览参数，为 `false` 时禁用 | boolean \| [PreviewType](#previewtype) | true | src | 图片地址 | string | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | width | 图像宽度 | string \| number | - | onError | 加载错误回调 | (event: Event) => void | - 
其他属性见 [&lt;img>](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/img#Attributes)

### PlaceholderType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| progress | 进度配置，设置为 `true` 显示渐变动画，设置 `{ percent: number }` 显示进度，`render` 自定义渲染 | boolean \| [ImageProgressConfig](#imageprogressconfig) | - 
| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| percent | 进度值 | number | - 
### PreviewType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| actionsRender | 自定义工具栏渲染 | (originalNode: React.ReactElement, info: ToolbarRenderInfoType) => React.ReactNode | - | cover | 自定义预览遮罩 | React.ReactNode \| [CoverConfig](#coverconfig) | - | CoverConfig v6.0 开始支持 |
| focusTrap | 预览打开时是否在预览内捕获焦点 | boolean | true | 6.4.0 |
| ~~destroyOnClose~~ | 关闭预览时销毁子元素，已移除，不再支持 | boolean | false | getContainer | 指定预览挂载的节点，但依旧为全屏展示，false 为挂载在当前位置 | string \| HTMLElement \| (() => HTMLElement) \| false | - | mask | 预览遮罩效果 | boolean \| { enabled?: boolean, blur?: boolean, closable?: boolean } | true | mask.closable: 6.4.0 |
| ~~maskClassName~~ | 缩略图遮罩类名，请使用 `classNames.cover` 替换 | string | - | minScale | 最小缩放倍数 | number | 1 | open | 是否显示预览 | boolean | - | scaleStep | `1 + scaleStep` 为缩放放大的每步倍数 | number | 0.5 | styles | 自定义语义化结构样式 | Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | ~~visible~~ | 是否显示，请使用 `open` 替换 | boolean | - | onTransform | 预览图 transform 变化的回调 | { transform: [TransformType](#transformtype), action: [TransformAction](#transformaction) } | - 
### PreviewGroup

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | items | 预览数组 | string[] \| { src: string, crossOrigin: string, ... }[] | - 
### PreviewGroupType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| actionsRender | 自定义工具栏渲染 | (originalNode: React.ReactElement, info: ToolbarRenderInfoType) => React.ReactNode | - | countRender | 自定义预览计数内容 | (current: number, total: number) => React.ReactNode | - | current | 当前预览图的 index | number | - | getContainer | 指定预览挂载的节点，但依旧为全屏展示，false 为挂载在当前位置 | string \| HTMLElement \| (() => HTMLElement) \| false | - | mask | 预览遮罩效果 | boolean \| { enabled?: boolean, blur?: boolean, closable?: boolean } | true | mask.closable: 6.4.0 |
| ~~maskClassName~~ | 缩略图遮罩类名，请使用 `classNames.cover` 替换 | string | - | maxScale | 最大放大倍数 | number | 50 | open | 是否显示预览 | boolean | - | styles | 自定义语义化结构样式 | Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | ~~toolbarRender~~ | 自定义工具栏，请使用 `actionsRender` 替换 | (originalNode: React.ReactElement, info: ToolbarRenderInfoType) => React.ReactNode | - | onOpenChange | 预览打开状态变化回调，额外携带当前预览图索引 | (visible: boolean, info: { current: number }) => void | - | onTransform | 预览图 transform 变化的回调 | { transform: [TransformType](#transformtype), action: [TransformAction](#transformaction) } | - 
## Interface

### TransformType

```typescript
{
  x: number;
  y: number;
  rotate: number;
  scale: number;
  flipX: boolean;
  flipY: boolean;
}
```

### TransformAction

```typescript
type TransformAction =
  | 'flipY'
  | 'flipX'
  | 'rotateLeft'
  | 'rotateRight'
  | 'zoomIn'
  | 'zoomOut'
  | 'close'
  | 'prev'
  | 'next'
  | 'wheel'
  | 'doubleClick'
  | 'move'
  | 'dragRebound'
  | 'reset';
```

### ToolbarRenderInfoType

```typescript
{
  icons: {
    flipYIcon: React.ReactNode;
    flipXIcon: React.ReactNode;
    rotateLeftIcon: React.ReactNode;
    rotateRightIcon: React.ReactNode;
    zoomOutIcon: React.ReactNode;
    zoomInIcon: React.ReactNode;
  };
  actions: {
    onActive?: (index: number) => void; // 5.21.0 之后支持
    onFlipY: () => void;
    onFlipX: () => void;
    onRotateLeft: () => void;
    onRotateRight: () => void;
    onZoomOut: () => void;
    onZoomIn: () => void;
    onReset: () => void; // 5.17.3 之后支持
    onClose: () => void;
  };
  transform: TransformType,
  current: number;
  total: number;
  image: ImgInfo
}
```

### ImgInfo

```typescript
{
  url: string;
  alt: string;
  width: string | number;
  height: string | number;
}
```

### CoverConfig

```typescript
type CoverConfig = {
  coverNode?: React.ReactNode; // 自定义遮罩元素
  placement?: 'top' | 'bottom' | 'center'; // 设置预览遮罩显示的位置
};
```

### 导入方式

```js
import { Image } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `alt` | 图像描述 | string | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `fallback` | 加载失败容错地址 | string | - | — |
| `height` | 图像高度 | string \| number | - | — |
| `placeholder` | 加载占位，支持 ReactNode 或配置对象 | [PlaceholderType](#placeholdertype) | - | — |
| `preview` | 预览参数，为 `false` 时禁用 | boolean \| [PreviewType](#previewtype) | true | — |
| `src` | 图片地址 | string | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `width` | 图像宽度 | string \| number | - | — |
| `onError` | 加载错误回调 | (event: Event) => void | - | — |
| `progress` | 进度配置，设置为 `true` 显示渐变动画，设置 `{ percent: number }` 显示进度，`render` 自定义渲染 | boolean \| [ImageProgressConfig](#imageprogressconfig) | - | — |
| `percent` | 进度值 | number | - | — |
| `render` | 自定义渲染，接收默认的进度 UI 和百分比 | (progress: React.ReactNode, percent: number) => React.ReactNode | - | — |
| `actionsRender` | 自定义工具栏渲染 | (originalNode: React.ReactElement, info: ToolbarRenderInfoType) => React.ReactNode | - | — |
| `closeIcon` | 自定义关闭 Icon | React.ReactNode | - | — |
| `cover` | 自定义预览遮罩 | React.ReactNode \| [CoverConfig](#coverconfig) | - | CoverConfig v6.0 开始支持 |
| `focusTrap` | 预览打开时是否在预览内捕获焦点 | boolean | true | 6.4.0 |
| `destroyOnClose` | 关闭预览时销毁子元素，已移除，不再支持 | boolean | false | — |
| `forceRender` | 强制渲染预览图，已移除，不再支持 | boolean | - | — |
| `getContainer` | 指定预览挂载的节点，但依旧为全屏展示，false 为挂载在当前位置 | string \| HTMLElement \| (() => HTMLElement) \| false | - | — |
| `imageRender` | 自定义预览内容 | (originalNode: React.ReactElement, info: { transform: [TransformType](#transformtype), image: [ImgInfo](#imginfo) }) => React.ReactNode | - | — |
| `mask` | 预览遮罩效果 | boolean \| { enabled?: boolean, blur?: boolean, closable?: boolean } | true | mask.closable: 6.4.0 |
| `maskClassName` | 缩略图遮罩类名，请使用 `classNames.cover` 替换 | string | - | — |
| `maxScale` | 最大缩放倍数 | number | 50 | — |
| `minScale` | 最小缩放倍数 | number | 1 | — |
| `movable` | 预览图片大于视口时是否可拖拽移动 | boolean | true | — |
| `open` | 是否显示预览 | boolean | - | — |
| `rootClassName` | 预览图的根 DOM 类名，会同时作用在图片和预览层最外侧 | string | - | — |
| `scaleStep` | `1 + scaleStep` 为缩放放大的每步倍数 | number | 0.5 | — |
| `toolbarRender` | 自定义工具栏，请使用 `actionsRender` 替换 | (originalNode: React.ReactElement, info: Omit) => React.ReactNode | - | — |
| `visible` | 是否显示，请使用 `open` 替换 | boolean | - | — |
| `onOpenChange` | 预览打开状态变化的回调 | (visible: boolean) => void | - | — |
| `onTransform` | 预览图 transform 变化的回调 | { transform: [TransformType](#transformtype), action: [TransformAction](#transformaction) } | - | — |
| `onVisibleChange` | 当 `visible` 发生改变时的回调，请使用 `onOpenChange` 替换 | (visible: boolean, prevVisible: boolean) => void | - | — |
| `items` | 预览数组 | string[] \| { src: string, crossOrigin: string, ... }[] | - | — |
| `countRender` | 自定义预览计数内容 | (current: number, total: number) => React.ReactNode | - | — |
| `current` | 当前预览图的 index | number | - | — |
| `onChange` | 切换预览图的回调 | (current: number, prevCurrent: number) => void | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Image** 的验收清单：

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
- 官方文档：https://ant.design/components/image
- 中文文档：https://ant.design/components/image-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/image
- 驱动 gpui kit：`image`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Image** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/image/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Image）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 开合、遮罩/Esc、placement、确认/取消主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Image）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：可预览的图片。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 middle | **14** | `fontSize` |
| 圆角 | **6** | `borderRadius` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / hover / active | `colorPrimary` + 变体 | 强调、选中、开态 |
| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | status 与反馈 |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 浮层阴影 / 遮罩 | `boxShadowSecondary` / `colorBgMask` | 适用者 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `alt` | 图像描述 | string | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `fallback` | 加载失败容错地址 | string | - |
| `height` | 图像高度 | string \ | number |
| `placeholder` | 加载占位，支持 ReactNode 或配置对象 | [PlaceholderType](#placeholdertype) | - |
| `preview` | 预览参数，为 `false` 时禁用 | boolean \ | [PreviewType](#previewtype) |
| `src` | 图片地址 | string | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `width` | 图像宽度 | string \ | number |
| `onError` | 加载错误回调 | (event: Event) => void | - |
| `progress` | 进度配置，设置为 `true` 显示渐变动画，设置 `{ percent: number }` 显示进度，`ren… | boolean \ | [ImageProgressConfig](#imageprogressconfig) |
| `percent` | 进度值 | number | - |
| `render` | 自定义渲染，接收默认的进度 UI 和百分比 | (progress: React.ReactNode, percent: … | - |
| `actionsRender` | 自定义工具栏渲染 | (originalNode: React.ReactElement, in… | - |
| `closeIcon` | 自定义关闭 Icon | React.ReactNode | - |
| `cover` | 自定义预览遮罩 | React.ReactNode \ | [CoverConfig](#coverconfig) |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
展示 src
  点图 preview ──► 全屏预览
  Esc/遮罩 ──► 关预览
  加载失败 ──► fallback
  PreviewGroup ──► 左右切换
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| IMG-S1 | 显示 src | 可见图 |
| IMG-S2 | 打开预览 | 全屏层 |
| IMG-S3 | 关闭预览 | 层消失 |
| IMG-S4 | 失败 fallback | fallback 图 |
| IMG-S5 | Group 下一张 | 切换 |
| IMG-S6 | preview=false | 点不预览 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| mask | `colorBgMask` 半透明（适用者） |
| panel/popup | 容器底 + 阴影 + 圆角 LG |
| open/close | 动画可关 / reduced-motion |
| disabled 触发 | 触发器禁用皮，不打开 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | dialog / menu / tooltip 等 |
| 焦点 | 打开进入浮层；关闭回触发器（可配） |
| Esc | 关闭（若允许） |
| 标题 | Dialog 必须有可访问名 |
| 遮罩 | 点击策略明确 |

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
| `onChange` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `items` | 必须 |
| `percent` | 必须 |
| 官方主路径示例 | 基本用法、渐进加载、容错处理、多张图片预览、相册模式、自定义预览图片、受控的预览、自定义工具栏 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| 动画像素级 / 复杂虚拟列表 | 分期 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 自定义预览内容, 预览遮罩, 自定义语义结构的样式和类, 嵌套 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestImage_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Image 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| IMG-01 | L1 | NewImage 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| IMG-02 | L1 | 显示 src | 可见图 |
| IMG-03 | L1 | 打开预览 | 全屏层 |
| IMG-04 | L1 | 关闭预览 | 层消失 |
| IMG-05 | L1 | 失败 fallback | fallback 图 |
| IMG-06 | L1 | Group 下一张 | 切换 |
| IMG-07 | L1 | preview=false | 点不预览 |
| IMG-08 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-09 | L1 | 复现官方示例「渐进加载」（`placeholder.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-10 | L1 | 复现官方示例「容错处理」（`fallback.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-11 | L1 | 复现官方示例「多张图片预览」（`preview-group.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-12 | L1 | 复现官方示例「相册模式」（`preview-group-visible.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-13 | L1 | 复现官方示例「自定义预览图片」（`previewSrc.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-14 | L1 | 复现官方示例「受控的预览」（`controlled-preview.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-15 | L1 | 复现官方示例「自定义工具栏」（`toolbarRender.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| IMG-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| IMG-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| IMG-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| IMG-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| IMG-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| IMG-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| IMG-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewImage(...) *Image

// 配置：对 §6.3 / §3 中 P0 字段提供 SetXxx
// 回调：OnChange / OnClick / OnOpenChange / OnConfirm … 按 API
// 状态：SetDisabled / SetLoading（适用者）
// 主题：SetTheme(*Theme)；Style 可选覆盖
// a11y：SetAriaLabel / 焦点与键盘
// 挂树：Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Disabled | false |
| Size（适用者） | middle / 控件默认 |
| 受控值 | 未 Set 时用 default* 或零值 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Trigger?
  └─ Portal
       ├─ mask?
       └─ panel / popup (+ arrow?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Image 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/image.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Image 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
