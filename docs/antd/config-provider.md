# ConfigProvider 全局化配置
> 来源：[Ant Design 6.5.x ConfigProvider](https://ant.design/components/config-provider)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：为组件提供统一的全局化配置。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

为组件提供统一的全局化配置。

**ConfigProvider** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 国际化 | 复现「国际化」视觉与布局 |
| 方向 | 复现「方向」视觉与布局 |
| 组件尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 主题 | light/dark 或主题色 |
| 自定义波纹 | 自定义渲染/插槽外观 |
| 静态方法 | 复现「静态方法」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `componentDisabled`

- **说明**：设置 antd 组件禁用状态
- **类型**：boolean
- **默认值**：-
- **版本**：4.21.0

#### `componentSize`

- **说明**：设置 antd 组件大小
- **类型**：`small` | `medium` | `large`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `direction`

- **说明**：设置文本展示方向。 [示例](#config-provider-demo-direction)
- **类型**：`ltr` | `rtl`
- **默认值**：`ltr`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `ltr` | 官方取值 `ltr` |
  | `rtl` | 官方取值 `rtl` |

#### `iconPrefixCls`

- **说明**：设置图标统一样式前缀
- **类型**：string
- **默认值**：`anticon`
- **版本**：4.11.0

#### `prefixCls`

- **说明**：设置统一样式前缀
- **类型**：string
- **默认值**：`ant`

#### `renderEmpty`

- **说明**：自定义组件空状态。参考 [空状态](/components/empty-cn)
- **类型**：function(componentName: string): ReactNode
- **默认值**：-

#### `theme`

- **说明**：设置主题，参考 [定制主题](/docs/react/customize-theme-cn)
- **类型**：[Theme](/docs/react/customize-theme-cn#theme)
- **默认值**：-
- **版本**：5.0.0

#### `variant`

- **说明**：设置全局输入组件形态变体
- **类型**：`outlined` | `filled` | `borderless`
- **默认值**：-
- **版本**：5.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `filled` | 浅底填充 |
  | `borderless` | 无边框 |

#### `disabled`

- **说明**：是否禁用水波纹效果
- **类型**：boolean
- **默认值**：false

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

实现与 antd **ConfigProvider** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. **国际化**（`locale.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **方向**（`direction.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **组件尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **主题**（`theme.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义波纹**（`wave.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **静态方法**（`holderRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `disabled` | 禁用 | 是否禁用水波纹效果 |
| `virtual` | 虚拟滚动 | 设置 `false` 时关闭虚拟滚动 |
| `getPopupContainer` | 浮层容器 | 弹出框（Select, Tooltip, Menu 等等）渲染父节点，默认渲染到 body 上。 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 国际化 | `locale.tsx` | 否 |
| 方向 | `direction.tsx` | 否 |
| 组件尺寸 | `size.tsx` | 否 |
| 主题 | `theme.tsx` | 否 |
| 自定义波纹 | `wave.tsx` | 否 |
| 静态方法 | `holderRender.tsx` | 否 |
| 前缀 | `prefixCls.tsx` | 是 |
| 获取配置 | `useConfig.tsx` | 是 |
| 警告 | `warning.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 为什么 message.info、notification.open 或 Modal.confirm 等方法内的 ReactNode 无法继承 ConfigProvider 的属性？比如 `prefixCls` 和 `theme`。 {#faq-message-inherit}

静态方法是使用 ReactDOM.render 重新渲染一个 React 根节点上，和主应用的 React 节点是脱离的。我们建议使用 useMessage、useNotification 和 useModal 来使用相关方法。原先的静态方法在 5.0 中已被废弃。

### 2.6 FAQ

## FAQ

### 如何增加一个新的语言包？ {#faq-add-locale}

参考[《增加语言包》](/docs/react/i18n#%E5%A2%9E%E5%8A%A0%E8%AF%AD%E8%A8%80%E5%8C%85)。

### 为什么时间类组件的国际化 locale 设置不生效？ {#faq-locale-not-work}

参考 FAQ [为什么时间类组件的国际化 locale 设置不生效？](/docs/react/faq#为什么时间类组件的国际化-locale-设置不生效)。

### 配置 `getPopupContainer` 导致 Modal 报错？ {#faq-get-popup-container}

相关 issue：

当如下全局设置 `getPopupContainer` 为触发节点的 parentNode 时，由于 Modal 的用法不存在 `triggerNode`，这样会导致 `triggerNode is undefined` 的报错，需要增加一个[判断条件](https://github.com/afc163/feedback-antd/commit/3e4d1ad1bc1a38460dc3bf3c56517f737fe7d44a)。

```diff
  triggerNode.parentNode}
+  getPopupContainer={node => {
+    if (node) {
+      return node.parentNode;
+    }
+    return document.body;
+  }}
 >
   
 
```

### 为什么 message.info、notification.open 或 Modal.confirm 等方法内的 ReactNode 无法继承 ConfigProvider 的属性？比如 `prefixCls` 和 `theme`。 {#faq-message-inherit}

静态方法是使用 ReactDOM.render 重新渲染一个 React 根节点上，和主应用的 React 节点是脱离的。我们建议使用 useMessage、useNotification 和 useModal 来使用相关方法。原先的静态方法在 5.0 中已被废弃。

### Vite 生产模式打包后国际化 locale 设置不生效？ {#faq-vite-locale-not-work}

相关 issue：[#39045](https://github.com/ant-design/ant-design/issues/39045)

由于 Vite 生产模式下打包与开发模式不同，cjs 格式的文件会多一层，需要 `zhCN.default` 来获取。推荐 Vite 用户直接从 `antd/es/locale` 目录下引入 esm 格式的 locale 文件。

### prefixCls 优先级(前者被后者覆盖) {#faq-prefixcls-priority}

1. `ConfigProvider.config({ prefixCls: 'prefix-1' })`
2. `ConfigProvider.config({ holderRender: (children) => {children} })`
3. `message.config({ prefixCls: 'prefix-3' })`

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

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| componentDisabled | 设置 antd 组件禁用状态 | boolean | - | 4.21.0 |
| componentSize | 设置 antd 组件大小 | `small` \| `medium` \| `large` | - | direction | 设置文本展示方向。 [示例](#config-provider-demo-direction) | `ltr` \| `rtl` | `ltr` | getTargetContainer | 配置 Affix、Anchor 滚动监听容器。 | `() => HTMLElement \| Window \| ShadowRoot` | () => window | 4.2.0 |
| iconPrefixCls | 设置图标统一样式前缀 | string | `anticon` | 4.11.0 |
| locale | 语言包配置，语言包可到 [antd/locale](https://unpkg.com/antd/locale/) 目录下寻找 | object | - | popupOverflow | Select 类组件弹层展示逻辑，默认为可视区域滚动，可配置成滚动区域滚动 | 'viewport' \| 'scroll' <InlinePopover previewURL="https://user-images.githubusercontent.com/5378891/230344474-5b9f7e09-0a5d-49e8-bae8-7d2abed6c837.png"></InlinePopover> | 'viewport' | 5.5.0 |
| prefixCls | 设置统一样式前缀 | string | `ant` | theme | 设置主题，参考 [定制主题](/docs/react/customize-theme-cn) | [Theme](/docs/react/customize-theme-cn#theme) | - | 5.0.0 |
| variant | 设置全局输入组件形态变体 | `outlined` \| `filled` \| `borderless` | - | 5.19.0 |
| virtual | 设置 `false` 时关闭虚拟滚动 | boolean | - | 4.3.0 |
| warning | 设置警告等级，`strict` 为 `false` 时会将废弃相关信息聚合为单条信息 | { strict: boolean } | - | 5.10.0 |
| ~~autoInsertSpaceInButton~~ | Button 自动空格配置，请使用 `button={{ autoInsertSpace: boolean }}` 替代 | boolean | - | - |
| ~~dropdownMatchSelectWidth~~ | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean | - | - |

### ConfigProvider.config() {#config}

设置 `Modal`、`Message`、`Notification` 静态方法配置，只会对非 hooks 的静态方法调用生效。

```tsx
ConfigProvider.config({
  // 5.13.0+
  holderRender: (children) => (
    <ConfigProvider
      prefixCls="ant"
      iconPrefixCls="anticon"
      theme={{ token: { colorPrimary: 'red' } }}
    >
      {children}
    </ConfigProvider>
  ),
});
```

### ConfigProvider.useConfig()  {#useconfig}

获取父级 `Provider` 的值，如 `DisabledContextProvider`、`SizeContextProvider`。

```jsx
const {
  componentDisabled, // 5.3.0+
  componentSize, // 5.3.0+
} = ConfigProvider.useConfig();
```

| 返回值 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| componentDisabled | antd 组件禁用状态 | boolean | - | 5.3.0 |
| componentSize | antd 组件大小状态 | `small` \| `medium` \| `large` | - | 5.3.0 |

### 组件配置 {#component-config}

以下配置项用于设置对应组件的通用属性或全局效果配置，具体 API 见链接：

- `affix`：[Affix](/components/affix-cn#api)（自 6.0.0 起支持）
- `alert`：[Alert](/components/alert-cn#api)（自 5.7.0 起支持）
- `anchor`：[Anchor](/components/anchor-cn#api)（自 6.0.0 起支持）
- `app`：[App](/components/app-cn#api)（自 6.3.0 起支持）
- `avatar`：[Avatar](/components/avatar-cn#api)（自 5.7.0 起支持）
- `badge`：[Badge](/components/badge-cn#api)（自 5.7.0 起支持）
- `borderBeam`：[BorderBeam](/components/border-beam-cn#api)（自 6.4.0 起支持）
- `breadcrumb`：[Breadcrumb](/components/breadcrumb-cn#api)（自 5.7.0 起支持）
- `button`：[Button](/components/button-cn#api)（自 5.6.0 起支持）
- `calendar`：[Calendar](/components/calendar-cn#api)（自 6.0.0 起支持）
- `card`：[Card](/components/card-cn#api)（自 5.14.0 起支持）
- `cardMeta`：[Card.Meta](/components/card-cn#cardmeta)（自 6.0.0 起支持）
- `carousel`：[Carousel](/components/carousel-cn#api)（自 5.7.0 起支持）
- `cascader`：[Cascader](/components/cascader-cn#api)（自 5.13.0 起支持）
- `checkbox`：[Checkbox](/components/checkbox-cn#api)（自 6.0.0 起支持）
- `collapse`：[Collapse](/components/collapse-cn#api)（自 5.15.0 起支持）
- `colorPicker`：[ColorPicker](/components/color-picker-cn#api)（自 6.3.0 起支持）
- `datePicker`：[DatePicker](/components/date-picker-cn#api)（自 5.7.0 起支持）
- `rangePicker`：[RangePicker](/components/date-picker-cn#rangepicker)（自 5.11.0 起支持）
- `descriptions`：[Descriptions](/components/descriptions-cn#api)（自 5.23.0 起支持）
- `divider`：[Divider](/components/divider-cn#api)（自 5.10.0 起支持）
- `drawer`：[Drawer](/components/drawer-cn#api)（自 5.10.0 起支持）
- `dropdown`：[Dropdown](/components/dropdown-cn#api)（自 5.11.0 起支持）
- `empty`：[Empty](/components/empty-cn#api)（自 5.23.0 起支持）
- `flex`：[Flex](/components/flex-cn#api)（自 5.10.0 起支持）
- `floatButton`：[FloatButton](/components/float-button-cn#api)（自 6.0.0 起支持）
- `floatButtonGroup`：[FloatButton.Group](/components/float-button-cn#floatbuttongroup)（自 5.16.0 起支持）
- `form`：[Form](/components/form-cn#api)（自 4.8.0 起支持）
- `image`：[Image](/components/image-cn#api)（自 5.14.0 起支持）
- `input`：[Input](/components/input-cn#input)（自 4.2.0 起支持）
- `inputNumber`：[InputNumber](/components/input-number-cn#api)（自 5.19.0 起支持）
- `otp`：[Input.OTP](/components/input-cn#inputotp)（自 6.0.0 起支持）
- `inputPassword`：[Input.Password](/components/input-cn#inputpassword)（自 6.4.0 起支持）
- `inputSearch`：[Input.Search](/components/input-cn#inputsearch)（自 6.4.0 起支持）
- `textArea`：[Input.TextArea](/components/input-cn#inputtextarea)（自 5.15.0 起支持）
- `layout`：[Layout](/components/layout-cn#api)（自 5.7.0 起支持）
- `list`：[List](/components/list-cn#api)（自 5.7.0 起支持）
- `masonry`：[Masonry](/components/masonry-cn#api)（自 6.0.0 起支持）
- `menu`：[Menu](/components/menu-cn#api)（自 5.15.0 起支持）
- `mentions`：[Mentions](/components/mentions-cn#api)（自 5.13.0 起支持）
- `message`：[Message](/components/message-cn#api)（自 5.7.0 起支持）
- `modal`：[Modal](/components/modal-cn#api)（自 5.10.0 起支持）
- `notification`：[Notification](/components/notification-cn#api)（自 5.14.0 起支持）
- `pagination`：[Pagination](/components/pagination-cn#api)（自 6.0.0 起支持）
- `progress`：[Progress](/components/progress-cn#api)（自 5.7.0 起支持）
- `radio`：[Radio](/components/radio-cn#api)（自 6.0.0 起支持）
- `rate`：[Rate](/components/rate-cn#api)（自 5.7.0 起支持）
- `result`：[Result](/components/result-cn#api)（自 6.0.0 起支持）
- `ribbon`：[Badge.Ribbon](/components/badge-cn#badgeribbon)（自 6.0.0 起支持）
- `skeleton`：[Skeleton](/components/skeleton-cn#api)（自 6.0.0 起支持）
- `segmented`：[Segmented](/components/segmented-cn#api)（自 6.0.0 起支持）
- `select`：[Select](/components/select-cn#api)（自 5.13.0 起支持）
- `slider`：[Slider](/components/slider-cn#api)（自 5.23.0 起支持）
- `switch`：[Switch](/components/switch-cn#api)（自 6.0.0 起支持）
- `space`：[Space](/components/space-cn#api)（自 5.6.0 起支持）
- `splitter`：[Splitter](/components/splitter-cn#api)（自 5.21.0 起支持）
- `spin`：[Spin](/components/spin-cn#api)（自 5.20.0 起支持）
- `statistic`：[Statistic](/components/statistic-cn#api)（自 6.0.0 起支持）
- `steps`：[Steps](/components/steps-cn#api)（自 5.10.0 起支持）
- `table`：[Table](/components/table-cn#api)（自 6.2.0 起支持）
- `tabs`：[Tabs](/components/tabs-cn#api)（自 5.14.0 起支持）
- `tag`：[Tag](/components/tag-cn#api)（自 5.14.0 起支持）
- `timeline`：[Timeline](/components/timeline-cn#api)（自 6.0.0 起支持）
- `timePicker`：[TimePicker](/components/time-picker-cn#api)（自 5.13.0 起支持）
- `tour`：[Tour](/components/tour-cn#api)（自 5.14.0 起支持）
- `tooltip`：[Tooltip](/components/tooltip-cn#api)（自 6.1.0 起支持）
- `popover`：[Popover](/components/popover-cn#api)（自 5.23.0 起支持）
- `popconfirm`：[Popconfirm](/components/popconfirm-cn#api)（自 5.23.0 起支持）
- `qrcode`：[QRCode](/components/qr-code-cn#api)（自 6.0.0 起支持）
- `transfer`：[Transfer](/components/transfer-cn#api)（自 5.7.0 起支持）
- `tree`：[Tree](/components/tree-cn#api)（自 6.0.0 起支持）
- `treeSelect`：[TreeSelect](/components/tree-select-cn#api)（自 5.19.0 起支持）
- `typography`：[Typography](/components/typography-cn#api)（自 6.4.0 起支持）
- `upload`：[Upload](/components/upload-cn#api)（自 5.27.0 起支持）
- `watermark`：[Watermark](/components/watermark-cn#api)（自 6.0.0 起支持）
- `wave`：[WaveConfig](#waveconfig)（自 5.8.0 起支持）

### WaveConfig

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| disabled | 是否禁用水波纹效果 | boolean | false | triggerType | 触发水波纹效果的事件 | `click` \| `pointerdown` \| `pointerup` \| `mousedown` \| `mouseup` | `click` | 6.4.0 |

### 导入方式

```js
import { ConfigProvider } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `componentDisabled` | 设置 antd 组件禁用状态 | boolean | - | 4.21.0 |
| `componentSize` | 设置 antd 组件大小 | `small` \| `medium` \| `large` | - | — |
| `csp` | 设置 [Content Security Policy](https://developer.mozilla.org/zh-CN/docs/Web/HTTP/CSP) 配置 | { nonce: string } | - | — |
| `direction` | 设置文本展示方向。 [示例](#config-provider-demo-direction) | `ltr` \| `rtl` | `ltr` | — |
| `getPopupContainer` | 弹出框（Select, Tooltip, Menu 等等）渲染父节点，默认渲染到 body 上。 | `(trigger?: HTMLElement) => HTMLElement \| ShadowRoot` | () => document.body | — |
| `getTargetContainer` | 配置 Affix、Anchor 滚动监听容器。 | `() => HTMLElement \| Window \| ShadowRoot` | () => window | 4.2.0 |
| `iconPrefixCls` | 设置图标统一样式前缀 | string | `anticon` | 4.11.0 |
| `locale` | 语言包配置，语言包可到 [antd/locale](https://unpkg.com/antd/locale/) 目录下寻找 | object | - | — |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。`false` 时会关闭虚拟滚动 | boolean \| number | - | 5.5.0 |
| `popupOverflow` | Select 类组件弹层展示逻辑，默认为可视区域滚动，可配置成滚动区域滚动 | 'viewport' \| 'scroll' | 'viewport' | 5.5.0 |
| `prefixCls` | 设置统一样式前缀 | string | `ant` | — |
| `renderEmpty` | 自定义组件空状态。参考 [空状态](/components/empty-cn) | function(componentName: string): ReactNode | - | — |
| `theme` | 设置主题，参考 [定制主题](/docs/react/customize-theme-cn) | [Theme](/docs/react/customize-theme-cn#theme) | - | 5.0.0 |
| `variant` | 设置全局输入组件形态变体 | `outlined` \| `filled` \| `borderless` | - | 5.19.0 |
| `virtual` | 设置 `false` 时关闭虚拟滚动 | boolean | - | 4.3.0 |
| `warning` | 设置警告等级，`strict` 为 `false` 时会将废弃相关信息聚合为单条信息 | { strict: boolean } | - | 5.10.0 |
| `autoInsertSpaceInButton` | Button 自动空格配置，请使用 `button={{ autoInsertSpace: boolean }}` 替代 | boolean | - | - |
| `dropdownMatchSelectWidth` | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean | - | - |
| `disabled` | 是否禁用水波纹效果 | boolean | false | — |
| `showEffect` | 自定义水波纹效果 | (node: HTMLElement, info: { className, token, component }) => void | - | — |
| `triggerType` | 触发水波纹效果的事件 | `click` \| `pointerdown` \| `pointerup` \| `mousedown` \| `mouseup` | `click` | 6.4.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **ConfigProvider** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **6** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/config-provider
- 中文文档：https://ant.design/components/config-provider-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/config-provider
- 驱动 gpui kit：`config-provider`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **ConfigProvider** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/config-provider/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（ConfigProvider）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（ConfigProvider）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：为组件提供统一的全局化配置。

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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**其他**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `componentDisabled` | 设置 antd 组件禁用状态 | boolean | - |
| `componentSize` | 设置 antd 组件大小 | `small` \ | `medium` \ |
| `csp` | 设置 [Content Security Policy](https://developer.mozilla.or… | { nonce: string } | - |
| `direction` | 设置文本展示方向。 [示例](#config-provider-demo-direction) | `ltr` \ | `rtl` |
| `getPopupContainer` | 弹出框（Select, Tooltip, Menu 等等）渲染父节点，默认渲染到 body 上。 | `(trigger?: HTMLElement) => HTMLEleme… | ShadowRoot` |
| `getTargetContainer` | 配置 Affix、Anchor 滚动监听容器。 | `() => HTMLElement \ | Window \ |
| `iconPrefixCls` | 设置图标统一样式前缀 | string | `anticon` |
| `locale` | 语言包配置，语言包可到 [antd/locale](https://unpkg.com/antd/locale/)… | object | - |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。`false` 时会关闭虚拟滚动 | boolean \ | number |
| `popupOverflow` | Select 类组件弹层展示逻辑，默认为可视区域滚动，可配置成滚动区域滚动 | 'viewport' \ | 'scroll' <InlinePopover previewURL="https://user-images.githubusercontent.com/5378891/230344474-5b9f7e09-0a5d-49e8-bae8-7d2abed6c837.png"></InlinePopover> |
| `prefixCls` | 设置统一样式前缀 | string | `ant` |
| `renderEmpty` | 自定义组件空状态。参考 [空状态](/components/empty-cn) | function(componentName: string): Reac… | - |
| `theme` | 设置主题，参考 [定制主题](/docs/react/customize-theme-cn) | [Theme](/docs/react/customize-theme-c… | - |
| `variant` | 设置全局输入组件形态变体 | `outlined` \ | `filled` \ |
| `virtual` | 设置 `false` 时关闭虚拟滚动 | boolean | - |
| `warning` | 设置警告等级，`strict` 为 `false` 时会将废弃相关信息聚合为单条信息 | { strict: boolean } | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
Provider 注入 theme/locale/size
子控件 rebuild 读上下文
嵌套内层覆盖外层
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CFG-S1 | theme 改 primary | 子 Button 主色变 |
| CFG-S2 | componentSize=small | 子 Input 高 24 |
| CFG-S3 | locale | 文案变 |
| CFG-S4 | 嵌套覆盖 | 内层胜出 |
| CFG-S5 | direction=rtl | 布局镜像（适用） |
| CFG-S6 | getPopupContainer | 浮层挂载点 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰图 | alt 或 aria-hidden |
| 有意义操作 | 复制/关闭/展开有名 |

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
| `disabled` | 必须 |
| `variant` | 必须 |
| 官方主路径示例 | 国际化、方向、组件尺寸、主题、自定义波纹、静态方法 |
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

### 6.9 验收用例表（可测）

> 测试名建议：`TestConfigProvider_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 ConfigProvider 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CFG-01 | L1 | NewConfigProvider 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| CFG-02 | L1 | theme 改 primary | 子 Button 主色变 |
| CFG-03 | L1 | componentSize=small | 子 Input 高 24 |
| CFG-04 | L1 | locale | 文案变 |
| CFG-05 | L1 | 嵌套覆盖 | 内层胜出 |
| CFG-06 | L1 | direction=rtl | 布局镜像（适用） |
| CFG-07 | L1 | getPopupContainer | 浮层挂载点 |
| CFG-08 | L1 | 复现官方示例「国际化」（`locale.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-09 | L1 | 复现官方示例「方向」（`direction.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-10 | L1 | 复现官方示例「组件尺寸」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-11 | L1 | 复现官方示例「主题」（`theme.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-12 | L1 | 复现官方示例「自定义波纹」（`wave.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-13 | L1 | 复现官方示例「静态方法」（`holderRender.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| CFG-14 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| CFG-15 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| CFG-16 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| CFG-17 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| CFG-18 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| CFG-19 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| CFG-20 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewConfigProvider(...) *ConfigProvider

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
Display root
  └─ content (+ actions?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **ConfigProvider 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/config-provider.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` ConfigProvider 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
