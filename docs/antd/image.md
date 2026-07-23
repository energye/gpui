# Image 图片
> 来源：[Ant Design 6.5.x Image](https://ant.design/components/image)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：可预览的图片。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
