# Upload 上传
> 来源：[Ant Design 6.5.x Upload](https://ant.design/components/upload)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：文件选择上传和拖拽上传控件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

文件选择上传和拖拽上传控件。

**Upload** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 点击上传 | 复现「点击上传」视觉与布局 |
| 用户头像 | 复现「用户头像」视觉与布局 |
| 已上传的文件列表 | 复现「已上传的文件列表」视觉与布局 |
| 照片墙 | 复现「照片墙」视觉与布局 |
| 圆形照片墙 | 复现「圆形照片墙」视觉与布局 |
| 完全控制的上传列表 | 复现「完全控制的上传列表」视觉与布局 |
| 拖拽上传 | 复现「拖拽上传」视觉与布局 |
| 粘贴上传 | 复现「粘贴上传」视觉与布局 |
| 文件夹上传 | 复现「文件夹上传」视觉与布局 |
| 手动上传 | 复现「手动上传」视觉与布局 |
| 只上传 png 图片 | 复现「只上传 png 图片」视觉与布局 |
| 图片列表样式 | 复现「图片列表样式」视觉与布局 |
| 自定义预览 | 自定义渲染/插槽外观 |
| 限制数量 | 复现「限制数量」视觉与布局 |
| 上传前转换文件 | 复现「上传前转换文件」视觉与布局 |
| 阿里云 OSS | 复现「阿里云 OSS」视觉与布局 |
| 自定义交互图标和文件信息 | icon 与文本混排 |
| 上传列表拖拽排序 | 复现「上传列表拖拽排序」视觉与布局 |
| 上传前裁切图片 | 复现「上传前裁切图片」视觉与布局 |
| 自定义进度条样式 | 进度条/圈 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `accept`

- **说明**：接受上传的文件类型，详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept)
- **类型**：string | [AcceptObject](#acceptobject)
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用。对于自定义 Upload children 时，请同时将 `disabled` 属性传给 child node，以确保禁用状态的渲染效果保持一致
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `disabled` | 官方取值 `disabled` |

#### `listType`

- **说明**：上传列表的内建样式，支持四种基本样式 `text`, `picture`, `picture-card` 和 `picture-circle`
- **类型**：string
- **默认值**：`text`
- **版本**：`picture-circle`(5.2.0+)
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `text` | 文本/弱样式 |
  | `picture` | 图片列表 |
  | `picture-card` | 照片墙 |
  | `picture-circle` | 圆形照片墙 |

#### `maxCount`

- **说明**：限制上传数量。当为 1 时，始终用最新上传的文件代替当前文件
- **类型**：number
- **默认值**：-
- **版本**：4.10.0

#### `progress`

- **说明**：自定义进度条样式
- **类型**：[ProgressProps](/components/progress-cn#api)（仅支持 `type="line"`）
- **默认值**：{ strokeWidth: 2, showInfo: false }
- **版本**：4.3.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `onPreview`

- **说明**：点击文件链接或预览图标时的回调
- **类型**：function(file)
- **默认值**：-

#### `percent`

- **说明**：上传进度
- **类型**：number
- **默认值**：-

#### `status`

- **说明**：上传状态，不同状态展示颜色也会有所不同
- **类型**：`error` | `done` | `uploading` | `removed`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `done` | 官方取值 `done` |
  | `uploading` | 官方取值 `uploading` |
  | `removed` | 官方取值 `removed` |

#### `format`

- **说明**：接受的文件类型，与原生 input accept 属性相同，支持 MIME 类型、文件扩展名等格式。详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept)
- **类型**：string
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

上传是将信息（网页、文字、图片、视频等）通过网页或者上传工具发布到远程服务器上的过程。

- 当需要上传一个或一些文件时。
- 当需要展现上传的进度时。
- 当需要使用拖拽交互时。

### 2.2 核心功能（按官方示例拆解）

1. **点击上传**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **用户头像**（`avatar.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **已上传的文件列表**（`defaultFileList.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **照片墙**（`picture-card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **圆形照片墙**（`picture-circle.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **完全控制的上传列表**（`fileList.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **拖拽上传**（`drag.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **粘贴上传**（`paste.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **文件夹上传**（`directory.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **手动上传**（`upload-manually.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **只上传 png 图片**（`upload-png-only.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **图片列表样式**（`picture-style.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义预览**（`preview-file.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **限制数量**（`max-count.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **上传前转换文件**（`transform-file.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **阿里云 OSS**（`upload-with-aliyun-oss.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **自定义交互图标和文件信息**（`upload-custom-action-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **上传列表拖拽排序**（`drag-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **上传前裁切图片**（`crop-image.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **自定义进度条样式**（`customize-progress-bar.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
21. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 上传文件改变时的回调，上传每个阶段都会触发该事件。详见 [onChange](#onchange) |
| `disabled` | 禁用 | 是否禁用。对于自定义 Upload children 时，请同时将 `disabled` 属性传给 child node，以确保禁用状态的渲染效果保持一致 |
| `fileList` | 文件列表 | 已经上传的文件列表（受控），使用此参数时，如果遇到 `onChange` 只调用一次的问题，请参考 [#2423](https://github.com/ant-design/ant-design/issues/2423) |
| `customRequest` | 自定义上传 | 通过覆盖默认的上传行为，可以自定义自己的上传实现 |
| `onRemove` | 移除 | 点击移除文件时的回调，返回值为 false 时不移除。支持返回一个 Promise 对象，Promise 对象 resolve(false) 或 reject 时不移除 |
| `onPreview` | 预览 | 点击文件链接或预览图标时的回调 |
| `percent` | 进度值 | 上传进度 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 点击上传 | `basic.tsx` | 否 |
| 用户头像 | `avatar.tsx` | 否 |
| 已上传的文件列表 | `defaultFileList.tsx` | 否 |
| 照片墙 | `picture-card.tsx` | 否 |
| 圆形照片墙 | `picture-circle.tsx` | 否 |
| 完全控制的上传列表 | `fileList.tsx` | 否 |
| 拖拽上传 | `drag.tsx` | 否 |
| 粘贴上传 | `paste.tsx` | 否 |
| 文件夹上传 | `directory.tsx` | 否 |
| 手动上传 | `upload-manually.tsx` | 否 |
| 只上传 png 图片 | `upload-png-only.tsx` | 否 |
| 图片列表样式 | `picture-style.tsx` | 否 |
| 自定义预览 | `preview-file.tsx` | 否 |
| 限制数量 | `max-count.tsx` | 否 |
| 上传前转换文件 | `transform-file.tsx` | 否 |
| 阿里云 OSS | `upload-with-aliyun-oss.tsx` | 否 |
| 自定义显示 icon | `file-type.tsx` | 是 |
| 自定义交互图标和文件信息 | `upload-custom-action-icon.tsx` | 否 |
| 上传列表拖拽排序 | `drag-sorting.tsx` | 否 |
| 上传前裁切图片 | `crop-image.tsx` | 否 |
| 自定义进度条样式 | `customize-progress-bar.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
| Debug Disabled Styles | `debug-disabled.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 服务端如何实现？ {#faq-server-implement}

- 服务端上传接口实现可以参考 [jQuery-File-Upload](https://github.com/blueimp/jQuery-File-Upload/wiki#server-side)。
- 如果要做本地 mock 可以参考这个 [express 的例子](https://github.com/react-component/upload/blob/211979fdaa2c7896b6496df7061a0cfc0fc5434e/server.js)。

### 如何显示下载链接？ {#faq-show-download-link}

请使用 `fileList` 属性设置数组项的 `url` 属性进行展示控制。

### `customRequest` 怎么使用？ {#faq-custom-request}

请参考 。

### 为何 `fileList` 受控时，上传不在列表中的文件不会触发 `onChange` 后续的 `status` 更新事件？ {#faq-filelist-controlled-status}

`onChange` 事件仅会作用于在列表中的文件，因而 `fileList` 不存在对应文件时后续事件会被忽略。请注意，在 `4.13.0` 版本之前受控状态存在 bug 导致不在列表中的文件也会触发。

### `onChange` 为什么有时候返回 File 有时候返回 { originFileObj: File }？ {#faq-on-change-return-type}

历史原因，在 `beforeUpload` 返回 `false` 时，会返回 `File` 对象。在下个大版本我们会统一返回 `{ originFileObj: File }` 对象。当前版本已经兼容所有场景下 `info.file.originFileObj` 获取原 `File` 写法。你可以提前切换。

### 为何有时 Chrome 点击 Upload 无法弹出文件选择框？ {#faq-chrome-file-picker}

与 `antd` 无关，原生上传也会失败。请重启 `Chrome` 浏览器，让其完成升级工作。

相关 `issue`：

- [#48007](https://github.com/ant-design/ant-design/issues/48007)
- [#32672](https://github.com/ant-design/ant-design/issues/32672)
- [#32913](https://github.com/ant-design/ant-design/issues/32913)
- [#33988](https://github.com/ant-design/ant-design/issues/33988)

### 文件夹上传在 Safari 仍然可以选中文件? {#faq-safari-folder-upload}

组件内部是以 `directory`、`webkitdirectory` 属性控制 input 来实现文件夹选择的, 但似乎在 Safari 的实现中，[并不会阻止用户选择文件](https://stackoverflow.com/q/55649945/3040605)。可以通过 `accept` 配置来解决此问题，例如：

```tsx
accept = {
  // 不允许选择任何文件
  format: `.${'n'.repeat(100)}`,
  // 当选择文件夹后，接受所有文件夹内的文件
  filter: () => true,
};
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
| accept | 接受上传的文件类型，详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept) | string \| [AcceptObject](#acceptobject) | - | action | 上传的地址 | string \| (file) => Promise&lt;string> | - | beforeUpload | 上传文件之前的钩子，参数为上传的文件，若返回 `false` 则停止上传。支持返回一个 Promise 对象，Promise 对象 reject 时则停止上传，resolve 时开始上传（ resolve 传入 `File` 或 `Blob` 对象则上传 resolve 传入对象）；也可以返回 `Upload.LIST_IGNORE`，此时列表中将不展示此文件。 **注意：IE9 不支持该方法** | (file: [RcFile](#rcfile), fileList: [RcFile[]](#rcfile)) => boolean \| Promise&lt;File> \| `Upload.LIST_IGNORE` | - | customRequest | 通过覆盖默认的上传行为，可以自定义自己的上传实现 | ( options: [RequestOptions](#request-options), info: { defaultRequest: (option: [RequestOptions](#request-options)) => void; } ) => void | - | defaultRequest: 5.28.0 | 5.27.0 |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | data | 上传所需额外参数或返回上传额外参数的方法 | object\|(file) => object \| Promise&lt;object> | - | defaultFileList | 默认已经上传的文件列表 | object\[] | - | directory | 支持上传文件夹（[caniuse](https://caniuse.com/#feat=input-file-directory)） | boolean | false | disabled | 是否禁用。对于自定义 Upload children 时，请同时将 `disabled` 属性传给 child node，以确保禁用状态的渲染效果保持一致 | boolean | false | fileList | 已经上传的文件列表（受控），使用此参数时，如果遇到 `onChange` 只调用一次的问题，请参考 [#2423](https://github.com/ant-design/ant-design/issues/2423) | [UploadFile](#uploadfile)\[] | - | headers | 设置上传的请求头部，IE10 以上有效 | object | - | iconRender | 自定义显示 icon | (file: UploadFile, listType?: UploadListType) => ReactNode | - | isImageUrl | 自定义缩略图是否使用 &lt;img /> 标签进行显示 | (file: UploadFile) => boolean | [(内部实现)](https://github.com/ant-design/ant-design/blob/4ad5830eecfb87471cd8ac588c5d992862b70770/components/upload/utils.tsx#L47-L68) | itemRender | 自定义上传列表项 | (originNode: ReactElement, file: UploadFile, fileList: object\[], actions: { download: function, preview: function, remove: function }) => React.ReactNode | - | 4.16.0 | × |
| listType | 上传列表的内建样式，支持四种基本样式 `text`, `picture`, `picture-card` 和 `picture-circle` | string | `text` | `picture-circle`(5.2.0+) | × |
| maxCount | 限制上传数量。当为 1 时，始终用最新上传的文件代替当前文件 | number | - | 4.10.0 | × |
| method | 上传请求的 http method | string | `post` | multiple | 是否支持多选文件，`ie10+` 支持。开启后按住 ctrl 可选择多个文件 | boolean | false | name | 发到后台的文件参数名 | string | `file` | openFileDialogOnClick | 点击打开文件对话框 | boolean | true | pastable | 是否支持粘贴文件 | boolean | false | 5.25.0 | × |
| previewFile | 自定义文件预览逻辑 | (file: File \| Blob) => Promise&lt;dataURL: string> | - | progress | 自定义进度条样式 | [ProgressProps](/components/progress-cn#api)（仅支持 `type="line"`） | { strokeWidth: 2, showInfo: false } | 4.3.0 | 6.4.0 |
| showUploadList | 是否展示文件列表, 可设为一个对象，用于单独设定 `extra`(5.20.0+), `showPreviewIcon`, `showRemoveIcon`, `showDownloadIcon`, `removeIcon` 和 `downloadIcon` | boolean \| { extra?: ReactNode \| (file: UploadFile) => ReactNode, showPreviewIcon?: boolean \| (file: UploadFile) => boolean, showDownloadIcon?: boolean \| (file: UploadFile) => boolean, showRemoveIcon?: boolean \| (file: UploadFile) => boolean, previewIcon?: ReactNode \| (file: UploadFile) => ReactNode, removeIcon?: ReactNode \| (file: UploadFile) => ReactNode, downloadIcon?: ReactNode \| (file: UploadFile) => ReactNode } | true | `extra`: 5.20.0, `showPreviewIcon` function: 5.21.0, `showRemoveIcon` function: 5.21.0, `showDownloadIcon` function: 5.21.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | withCredentials | 上传请求时是否携带 cookie | boolean | false | onChange | 上传文件改变时的回调，上传每个阶段都会触发该事件。详见 [onChange](#onchange) | function | - | onDrop | 当文件被拖入上传区域时执行的回调功能 | (event: React.DragEvent) => void | - | 4.16.0 | × |
| onDownload | 点击下载文件时的回调，如果没有指定，则默认跳转到文件 url 对应的标签页 | function(file): void | (跳转新标签页) | onPreview | 点击文件链接或预览图标时的回调 | function(file) | - | onRemove | 点击移除文件时的回调，返回值为 false 时不移除。支持返回一个 Promise 对象，Promise 对象 resolve(false) 或 reject 时不移除 | function(file): boolean \| Promise | - 
## Interface

### RcFile

继承自 [File](https://developer.mozilla.org/zh-CN/docs/Web/API/File)。

| 参数             | 说明                           | 类型   | 默认值 | 版本 |
| ---------------- | ------------------------------ | ------ | ------ | ---- |
| uid              | 唯一标识符，不设置时会自动生成 | string | -      | -    |
| lastModifiedDate | 上次修改文件的日期和时间       | date   | -      | -    |

### UploadFile

继承自 [File](https://developer.mozilla.org/zh-CN/docs/Web/API/File)，附带额外属性用于渲染。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| crossOrigin | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.20.0 |
| name | 文件名 | string | - | - |
| percent | 上传进度 | number | - | - |
| status | 上传状态，不同状态展示颜色也会有所不同 | `error` \| `done` \| `uploading` \| `removed` | - | - |
| thumbUrl | 缩略图地址 | string | - | - |
| uid | 唯一标识符，不设置时会自动生成 | string | - | - |
| url | 下载地址 | string | - | - |

### RequestOptions {#request-options}

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| action | 上传的地址 | string | - | - |
| data | 上传所需额外参数或返回上传额外参数的方法 | Record<string, unknown> | - | - |
| filename | 文件名 | string | - | - |
| file | 文件信息 | [UploadFile](#uploadfile) | - | - |
| withCredentials | 上传请求时是否携带 cookie | boolean | - | - |
| headers | 上传的请求头部 | Record<string, string> | - | - |
| method | 上传请求的 http method | string | - | - |
| onProgress | 上传进度回调 | (event: object, file: UploadFile) => void | - | - |
| onError | 上传失败回调 | (event: object, body?: object) => void | - | - |
| onSuccess | 上传成功回调 | (body: object, fileOrXhr?: UploadFile \| XMLHttpRequest) => void | - | - |

### onChange

> 💡 上传中、完成、失败都会调用这个函数。

文件状态改变的回调，返回为：

```jsx
{
  file: { /* ... */ },
  fileList: [ /* ... */ ],
  event: { /* ... */ },
}
```

1. `file` 当前操作的文件对象。

   ```jsx
   {
      uid: 'uid',      // 文件唯一标识，建议设置为负数，防止和内部产生的 id 冲突
      name: 'xx.png',   // 文件名
      status: 'done' | 'uploading' | 'error' | 'removed' , //  beforeUpload 拦截的文件没有 status 状态属性
      response: '{"status": "success"}', // 服务端响应内容
      linkProps: '{"download": "image"}', // 下载链接额外的 HTML 属性
   }
   ```

2. `fileList` 当前的文件列表。

3. `event` 上传中的服务端响应内容，包含了上传进度等信息，高级浏览器支持。

### AcceptObject

```typescript
{
  format: string;
  filter?: 'native' | ((file: RcFile) => boolean);
}
```

用于配置文件类型接受的规则对象。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| format | 接受的文件类型，与原生 input accept 属性相同，支持 MIME 类型、文件扩展名等格式。详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept) | string | - 
### 导入方式

```js
import { Upload } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `accept` | 接受上传的文件类型，详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept) | string \| [AcceptObject](#acceptobject) | - | — |
| `action` | 上传的地址 | string \| (file) => Promise<string> | - | — |
| `beforeUpload` | 上传文件之前的钩子，参数为上传的文件，若返回 `false` 则停止上传。支持返回一个 Promise 对象，Promise 对象 reject 时则停止上传，resolve 时开始上传（ resolve 传入 `File` 或 `Blob` 对象则上传 resolve 传入对象）；也可以返回 `Upload.LIST_IGNORE`，此时列表中将不展示此文件。 **注意：IE9 不支持该方法** | (file: [RcFile](#rcfile), fileList: [RcFile[]](#rcfile)) => boolean \| Promise<File> \| `Upload.LIST_IGNORE` | - | — |
| `customRequest` | 通过覆盖默认的上传行为，可以自定义自己的上传实现 | ( options: [RequestOptions](#request-options), info: { defaultRequest: (option: [RequestOptions](#request-options)) => void; } ) => void | - | defaultRequest: 5.28.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `data` | 上传所需额外参数或返回上传额外参数的方法 | object\|(file) => object \| Promise<object> | - | — |
| `defaultFileList` | 默认已经上传的文件列表 | object\[] | - | — |
| `directory` | 支持上传文件夹（[caniuse](https://caniuse.com/#feat=input-file-directory)） | boolean | false | — |
| `disabled` | 是否禁用。对于自定义 Upload children 时，请同时将 `disabled` 属性传给 child node，以确保禁用状态的渲染效果保持一致 | boolean | false | — |
| `fileList` | 已经上传的文件列表（受控），使用此参数时，如果遇到 `onChange` 只调用一次的问题，请参考 [#2423](https://github.com/ant-design/ant-design/issues/2423) | [UploadFile](#uploadfile)\[] | - | — |
| `headers` | 设置上传的请求头部，IE10 以上有效 | object | - | — |
| `iconRender` | 自定义显示 icon | (file: UploadFile, listType?: UploadListType) => ReactNode | - | — |
| `isImageUrl` | 自定义缩略图是否使用 <img /> 标签进行显示 | (file: UploadFile) => boolean | [(内部实现)](https://github.com/ant-design/ant-design/blob/4ad5830eecfb87471cd8ac588c5d992862b70770/components/upload/utils.tsx#L47-L68) | — |
| `itemRender` | 自定义上传列表项 | (originNode: ReactElement, file: UploadFile, fileList: object\[], actions: { download: function, preview: function, remove: function }) => React.ReactNode | - | 4.16.0 |
| `listType` | 上传列表的内建样式，支持四种基本样式 `text`, `picture`, `picture-card` 和 `picture-circle` | string | `text` | `picture-circle`(5.2.0+) |
| `maxCount` | 限制上传数量。当为 1 时，始终用最新上传的文件代替当前文件 | number | - | 4.10.0 |
| `method` | 上传请求的 http method | string | `post` | — |
| `multiple` | 是否支持多选文件，`ie10+` 支持。开启后按住 ctrl 可选择多个文件 | boolean | false | — |
| `name` | 发到后台的文件参数名 | string | `file` | — |
| `openFileDialogOnClick` | 点击打开文件对话框 | boolean | true | — |
| `pastable` | 是否支持粘贴文件 | boolean | false | 5.25.0 |
| `previewFile` | 自定义文件预览逻辑 | (file: File \| Blob) => Promise<dataURL: string> | - | — |
| `progress` | 自定义进度条样式 | [ProgressProps](/components/progress-cn#api)（仅支持 `type="line"`） | { strokeWidth: 2, showInfo: false } | 4.3.0 |
| `showUploadList` | 是否展示文件列表, 可设为一个对象，用于单独设定 `extra`(5.20.0+), `showPreviewIcon`, `showRemoveIcon`, `showDownloadIcon`, `removeIcon` 和 `downloadIcon` | boolean \| { extra?: ReactNode \| (file: UploadFile) => ReactNode, showPreviewIcon?: boolean \| (file: UploadFile) => boolean, showDownloadIcon?: boolean \| (file: UploadFile) => boolean, showRemoveIcon?: boolean \| (file: UploadFile) => boolean, previewIcon?: ReactNode \| (file: UploadFile) => ReactNode, removeIcon?: ReactNode \| (file: UploadFile) => ReactNode, downloadIcon?: ReactNode \| (file: UploadFile) => ReactNode } | true | `extra`: 5.20.0, `showPreviewIcon` function: 5.21.0, `showRemoveIcon` function: 5.21.0, `showDownloadIcon` function: 5.21.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `withCredentials` | 上传请求时是否携带 cookie | boolean | false | — |
| `onChange` | 上传文件改变时的回调，上传每个阶段都会触发该事件。详见 [onChange](#onchange) | function | - | — |
| `onDrop` | 当文件被拖入上传区域时执行的回调功能 | (event: React.DragEvent) => void | - | 4.16.0 |
| `onDownload` | 点击下载文件时的回调，如果没有指定，则默认跳转到文件 url 对应的标签页 | function(file): void | (跳转新标签页) | — |
| `onPreview` | 点击文件链接或预览图标时的回调 | function(file) | - | — |
| `onRemove` | 点击移除文件时的回调，返回值为 false 时不移除。支持返回一个 Promise 对象，Promise 对象 resolve(false) 或 reject 时不移除 | function(file): boolean \| Promise | - | — |
| `uid` | 唯一标识符，不设置时会自动生成 | string | - | - |
| `lastModifiedDate` | 上次修改文件的日期和时间 | date | - | - |
| `crossOrigin` | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.20.0 |
| `percent` | 上传进度 | number | - | - |
| `status` | 上传状态，不同状态展示颜色也会有所不同 | `error` \| `done` \| `uploading` \| `removed` | - | - |
| `thumbUrl` | 缩略图地址 | string | - | - |
| `url` | 下载地址 | string | - | - |
| `filename` | 文件名 | string | - | - |
| `file` | 文件信息 | [UploadFile](#uploadfile) | - | - |
| `onProgress` | 上传进度回调 | (event: object, file: UploadFile) => void | - | - |
| `onError` | 上传失败回调 | (event: object, body?: object) => void | - | - |
| `onSuccess` | 上传成功回调 | (body: object, fileOrXhr?: UploadFile \| XMLHttpRequest) => void | - | - |
| `format` | 接受的文件类型，与原生 input accept 属性相同，支持 MIME 类型、文件扩展名等格式。详见 [input accept Attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#accept) | string | - | — |
| `filter` | 文件过滤规则。设置为 `'native'` 时使用浏览器原生过滤行为；设置为函数时可以自定义过滤逻辑，函数返回 `true` 表示接受该文件，返回 `false` 表示拒绝 | `'native'` \| `(file: RcFile) => boolean` | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Upload** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **21** 个，均需可复现。
12. **上传专项**：uploading/done/error/removed 状态机。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/upload
- 中文文档：https://ant.design/components/upload-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/upload
- 驱动 gpui kit：`upload`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Upload** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/upload/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Upload）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Upload）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：文件选择上传和拖拽上传控件。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 控件高度 middle | **32** | `controlHeight` |
| 控件高度 small | **24** | `controlHeightSM` |
| 控件高度 large | **40** | `controlHeightLG` |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `accept` | 接受上传的文件类型，详见 [input accept Attribute](https://developer.m… | string \ | [AcceptObject](#acceptobject) |
| `action` | 上传的地址 | string \ | (file) => Promise&lt;string> |
| `beforeUpload` | 上传文件之前的钩子，参数为上传的文件，若返回 `false` 则停止上传。支持返回一个 Promise 对象，Pr… | (file: [RcFile](#rcfile), fileList: [… | Promise&lt;File> \ |
| `customRequest` | 通过覆盖默认的上传行为，可以自定义自己的上传实现 | ( options: [RequestOptions](#request-… | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `data` | 上传所需额外参数或返回上传额外参数的方法 | object\ | (file) => object \ |
| `defaultFileList` | 默认已经上传的文件列表 | object\[] | - |
| `directory` | 支持上传文件夹（[caniuse](https://caniuse.com/#feat=input-file-di… | boolean | false |
| `disabled` | 是否禁用。对于自定义 Upload children 时，请同时将 `disabled` 属性传给 child n… | boolean | false |
| `fileList` | 已经上传的文件列表（受控），使用此参数时，如果遇到 `onChange` 只调用一次的问题，请参考 [#2423]… | [UploadFile](#uploadfile)\[] | - |
| `headers` | 设置上传的请求头部，IE10 以上有效 | object | - |
| `iconRender` | 自定义显示 icon | (file: UploadFile, listType?: UploadL… | - |
| `isImageUrl` | 自定义缩略图是否使用 &lt;img /> 标签进行显示 | (file: UploadFile) => boolean | [(内部实现)](https://github.com/ant-design/ant-design/blob/4ad5830eecfb87471cd8ac588c5d992862b70770/components/upload/utils.tsx#L47-L68) |
| `itemRender` | 自定义上传列表项 | (originNode: ReactElement, file: Uplo… | - |
| `listType` | 上传列表的内建样式，支持四种基本样式 `text`, `picture`, `picture-card` 和 `p… | string | `text` |
| `maxCount` | 限制上传数量。当为 1 时，始终用最新上传的文件代替当前文件 | number | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
idle ── 选择文件 ──► beforeUpload(file)
                      ├── 返回 false ──► 仍可入 list（antd 行为）或不上传
                      ├── 返回 Promise reject ──► 阻止
                      └── 通过 ──► customRequest / 默认上传
                                      ├── progress ──► onChange
                                      ├── done ──► file.status=done
                                      └── error ──► file.status=error
onRemove ──► 列表移除 + onChange
maxCount 触顶 ──► 不能再选（或替换策略）
disabled ──► 不可选
```

\*桌面 customRequest 为 P0 主路径（无浏览器 Form 默认上传）。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| UPL-S1 | 选择 1 个文件 | `fileList` 增加；`onChange` |
| UPL-S2 | `beforeUpload` 返回 false | 按 antd：列表可有文件但不自动上传 |
| UPL-S3 | `customRequest` 调成功 | status 到 done |
| UPL-S4 | 上传失败 | status=error；可展示 |
| UPL-S5 | `onRemove` | 项消失 |
| UPL-S6 | `maxCount=1` 再选 | 受控替换或拒绝（与实现一致并测） |
| UPL-S7 | `disabled` | 不能选文件 |
| UPL-S8 | `listType=picture-card` | 卡片格布局 |
| UPL-S9 | Drag 区拖入 | 同等 onChange |
| UPL-S10 | `accept` 过滤 | 不接受类型不可入（或宿主过滤） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | textbox / combobox / spinbutton / listbox 等 |
| 标签 | 与 Form.Item label 或 aria-labelledby 关联 |
| 清除/下拉 | 控件有可访问名称 |
| 错误 | status=error 时暴露 invalid |
| 键盘 | 主路径可选/提交/关闭 |

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
| `disabled` | 必须 |
| `status` | 必须 |
| `fileList` | 必须 |
| `percent` | 必须 |
| 官方主路径示例 | 点击上传、用户头像、已上传的文件列表、照片墙、圆形照片墙、完全控制的上传列表、拖拽上传、粘贴上传 |
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
| 其余示例 | 文件夹上传, 手动上传, 只上传 png 图片, 图片列表样式 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestUpload_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Upload 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| UPL-01 | L1 | NewUpload 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| UPL-02 | L1 | 选择 1 个文件 | `fileList` 增加；`onChange` |
| UPL-03 | L1 | `beforeUpload` 返回 false | 按 antd：列表可有文件但不自动上传 |
| UPL-04 | L1 | `customRequest` 调成功 | status 到 done |
| UPL-05 | L1 | 上传失败 | status=error；可展示 |
| UPL-06 | L1 | `onRemove` | 项消失 |
| UPL-07 | L1 | `maxCount=1` 再选 | 受控替换或拒绝（与实现一致并测） |
| UPL-08 | L1 | `disabled` | 不能选文件 |
| UPL-09 | L1 | `listType=picture-card` | 卡片格布局 |
| UPL-10 | L1 | Drag 区拖入 | 同等 onChange |
| UPL-11 | L1 | `accept` 过滤 | 不接受类型不可入（或宿主过滤） |
| UPL-12 | L1 | 复现官方示例「点击上传」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-13 | L1 | 复现官方示例「用户头像」（`avatar.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-14 | L1 | 复现官方示例「已上传的文件列表」（`defaultFileList.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-15 | L1 | 复现官方示例「照片墙」（`picture-card.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-16 | L1 | 复现官方示例「圆形照片墙」（`picture-circle.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-17 | L1 | 复现官方示例「完全控制的上传列表」（`fileList.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-18 | L1 | 复现官方示例「拖拽上传」（`drag.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-19 | L1 | 复现官方示例「粘贴上传」（`paste.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| UPL-20 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| UPL-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| UPL-22 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| UPL-23 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| UPL-24 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| UPL-25 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| UPL-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewUpload(...) *Upload

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
Field / Selector
  ├─ prefix?
  ├─ editable / display value
  ├─ clear? / suffix?
  └─ Portal popup? (list/panel)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Upload 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/upload.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Upload 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
