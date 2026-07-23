# QRCode 二维码
> 来源：[Ant Design 6.5.x QRCode](https://ant.design/components/qr-code)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：能够将文本转换生成二维码的组件，支持自定义配色和 Logo 配置。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

能够将文本转换生成二维码的组件，支持自定义配色和 Logo 配置。

**QRCode** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 带 Icon 的例子 | 复现「带 Icon 的例子」视觉与布局 |
| 不同的状态 | 复现「不同的状态」视觉与布局 |
| 自定义状态渲染器 | 自定义渲染/插槽外观 |
| 自定义渲染类型 | 自定义渲染/插槽外观 |
| 自定义尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 自定义颜色 | 语义色/预设色 |
| 下载二维码 | 复现「下载二维码」视觉与布局 |
| 纠错比例 | 复现「纠错比例」视觉与布局 |
| 高级用法 | 复现「高级用法」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `type`

- **说明**：渲染类型
- **类型**：`canvas | svg`
- **默认值**：`canvas`
- **版本**：5.6.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `canvas` | 官方取值 `canvas` |
  | `svg` | 官方取值 `svg` |

#### `icon`

- **说明**：二维码中图片的地址（目前只支持图片地址）
- **类型**：string
- **默认值**：-

#### `size`

- **说明**：二维码大小
- **类型**：number
- **默认值**：160

#### `iconSize`

- **说明**：二维码中图片的大小
- **类型**：number | { width: number; height: number }
- **默认值**：40
- **版本**：5.19.0

#### `color`

- **说明**：二维码颜色
- **类型**：string
- **默认值**：`#000`

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `bgColor`

- **说明**：二维码背景颜色
- **类型**：string
- **默认值**：`transparent`
- **版本**：5.5.0

#### `marginSize`

- **说明**：留白（安静区）大小（单位为模块数），`0` 表示无留白
- **类型**：number
- **默认值**：`0`
- **版本**：6.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `0` | 官方取值 `0` |

#### `bordered`

- **说明**：是否有边框
- **类型**：boolean
- **默认值**：true

#### `status`

- **说明**：二维码状态
- **类型**：`active | expired | loading | scanned`
- **默认值**：`active`
- **版本**：scanned: 5.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `active` | 官方取值 `active` |
  | `expired` | 官方取值 `expired` |
  | `loading` | 官方取值 `loading` |
  | `scanned` | 官方取值 `scanned` |

#### `statusRender`

- **说明**：自定义状态渲染器
- **类型**：(info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode
- **默认值**：-
- **版本**：5.20.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

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

当需要将文本转换成为二维码时使用。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`base.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带 Icon 的例子**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **不同的状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义状态渲染器**（`customStatusRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义渲染类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义尺寸**（`customSize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义颜色**（`customColor.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **下载二维码**（`download.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **纠错比例**（`errorlevel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **高级用法**（`Popover.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 扫描后的文本 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `base.tsx` | 否 |
| 带 Icon 的例子 | `icon.tsx` | 否 |
| 不同的状态 | `status.tsx` | 否 |
| 自定义状态渲染器 | `customStatusRender.tsx` | 否 |
| 自定义渲染类型 | `type.tsx` | 否 |
| 自定义尺寸 | `customSize.tsx` | 否 |
| 自定义颜色 | `customColor.tsx` | 否 |
| 下载二维码 | `download.tsx` | 否 |
| 纠错比例 | `errorlevel.tsx` | 否 |
| 高级用法 | `Popover.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.6 FAQ

## FAQ

### 关于二维码纠错等级 {#faq-error-correction-level}

纠错等级也叫纠错率，就是指二维码可以被遮挡后还能正常扫描，而这个能被遮挡的最大面积就是纠错率。

通常情况下二维码分为 4 个纠错级别：`L级` 可纠正约 `7%` 错误、`M级` 可纠正约 `15%` 错误、`Q级` 可纠正约 `25%` 错误、`H级` 可纠正约`30%` 错误。并不是所有位置都可以缺损，像最明显的三个角上的方框，直接影响初始定位。中间零散的部分是内容编码，可以容忍缺损。当二维码的内容编码携带信息比较少的时候，也就是链接比较短的时候，设置不同的纠错等级，生成的图片不会发生变化。

> 有关更多信息，可参阅相关资料：[https://www.qrcode.com/zh/about/error_correction](https://www.qrcode.com/zh/about/error_correction.html)

### ⚠️⚠️⚠️ 二维码无法扫描？ {#faq-cannot-scan}

若二维码无法扫码识别，可能是因为链接地址过长导致像素过于密集，可以通过 size 配置二维码更大，或者通过短链接服务等方式将链接变短。

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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| :-- | :-- | :-- | :-- | :-- | --- |
| value | 扫描后的文本 | `string \| string[]` | - | `string[]`: 5.28.0 | × |
| type | 渲染类型 | `canvas \| svg` | `canvas` | 5.6.0 | × |
| icon | 二维码中图片的地址（目前只支持图片地址） | string | - | - | × |
| size | 二维码大小 | number | 160 | - | × |
| iconSize | 二维码中图片的大小 | number \| { width: number; height: number } | 40 | 5.19.0 | × |
| color | 二维码颜色 | string | `#000` | - | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| bgColor | 二维码背景颜色 | string | `transparent` | 5.5.0 | × |
| marginSize | 留白（安静区）大小（单位为模块数），`0` 表示无留白 | number | `0` | 6.2.0 | × |
| bordered | 是否有边框 | boolean | true | - | × |
| errorLevel | 二维码纠错等级 | `'L' \| 'M' \| 'Q' \| 'H'` | `M` | - | × |
| boostLevel | 如果启用，自动提升纠错等级，结果的纠错级别可能会高于指定的纠错级别 | `boolean` | true | 5.28.0 | × |
| status | 二维码状态 | `active \| expired \| loading \| scanned` | `active` | scanned: 5.13.0 | × |
| statusRender | 自定义状态渲染器 | (info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode | - | 5.20.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |

### StatusRenderInfo

```typescript
type StatusRenderInfo = {
  status: QRStatus;
  locale: Locale['QRCode'];
  onRefresh?: () => void;
};
```

### 导入方式

```js
import { QRCode } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `value` | 扫描后的文本 | `string \| string[]` | - | `string[]`: 5.28.0 |
| `type` | 渲染类型 | `canvas \| svg` | `canvas` | 5.6.0 |
| `icon` | 二维码中图片的地址（目前只支持图片地址） | string | - | - |
| `size` | 二维码大小 | number | 160 | - |
| `iconSize` | 二维码中图片的大小 | number \| { width: number; height: number } | 40 | 5.19.0 |
| `color` | 二维码颜色 | string | `#000` | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `bgColor` | 二维码背景颜色 | string | `transparent` | 5.5.0 |
| `marginSize` | 留白（安静区）大小（单位为模块数），`0` 表示无留白 | number | `0` | 6.2.0 |
| `bordered` | 是否有边框 | boolean | true | - |
| `errorLevel` | 二维码纠错等级 | `'L' \| 'M' \| 'Q' \| 'H'` | `M` | - |
| `boostLevel` | 如果启用，自动提升纠错等级，结果的纠错级别可能会高于指定的纠错级别 | `boolean` | true | 5.28.0 |
| `status` | 二维码状态 | `active \| expired \| loading \| scanned` | `active` | scanned: 5.13.0 |
| `statusRender` | 自定义状态渲染器 | (info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode | - | 5.20.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **QRCode** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **11** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/qr-code
- 中文文档：https://ant.design/components/qr-code-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/qr-code
- 驱动 gpui kit：`qr-code`
