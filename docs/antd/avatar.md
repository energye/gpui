# Avatar 头像
> 来源：[Ant Design 6.5.x Avatar](https://ant.design/components/avatar)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用来代表用户或事物，支持图片、图标或字符展示。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用来代表用户或事物，支持图片、图标或字符展示。

**Avatar** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 类型 | type 预设外观 |
| 自动调整字符大小 | 不同 size 档位 |
| 带徽标的头像 | Badge 叠加 |
| Avatar.Group | 复现「Avatar.Group」视觉与布局 |
| 响应式尺寸 | 不同 size 档位的高宽/字号/内边距 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `alt`

- **说明**：图像无法显示时的替代文本
- **类型**：string
- **默认值**：-

#### `gap`

- **说明**：字符类型距离左右两侧边界单位像素
- **类型**：number
- **默认值**：4
- **版本**：4.3.0

#### `icon`

- **说明**：设置头像的自定义图标
- **类型**：ReactNode
- **默认值**：-

#### `shape`

- **说明**：指定头像的形状
- **类型**：`circle` | `square`
- **默认值**：`circle`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `circle` | 圆形 |
  | `square` | 方形 |

#### `size`

- **说明**：设置头像的大小
- **类型**：number | `large` | `medium` | `small` | { xs: number, sm: number, ...}
- **默认值**：`medium`
- **版本**：4.7.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `src`

- **说明**：图片类头像的资源地址或者图片元素
- **类型**：string | ReactNode
- **默认值**：-
- **版本**：ReactNode: 4.8.0

#### `srcSet`

- **说明**：设置图片类头像响应式资源地址
- **类型**：string
- **默认值**：-

#### `draggable`

- **说明**：图片是否允许拖动
- **类型**：boolean | `'true'` | `'false'`
- **默认值**：true

#### `onError`

- **说明**：图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为
- **类型**：() => boolean
- **默认值**：-

#### `max`

- **说明**：设置最多显示相关配置
- **类型**：`{ count?: number; style?: CSSProperties; popover?: PopoverProps }`
- **默认值**：-
- **版本**：5.18.0

#### `maxCount`

- **说明**：已废弃，请使用 `max={{ count: number }}`
- **类型**：number
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

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

实现与 antd **Avatar** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自动调整字符大小**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **带徽标的头像**（`badge.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **Avatar.Group**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **响应式尺寸**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 类型 | `type.tsx` | 否 |
| 自动调整字符大小 | `dynamic.tsx` | 否 |
| 带徽标的头像 | `badge.tsx` | 否 |
| Avatar.Group | `group.tsx` | 否 |
| 隐藏情况下计算字符对齐 | `toggle-debug.tsx` | 是 |
| 响应式尺寸 | `responsive.tsx` | 否 |
| 图片不存在时 | `fallback.tsx` | 是 |
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

### Avatar

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| alt | 图像无法显示时的替代文本 | string | - | gap | 字符类型距离左右两侧边界单位像素 | number | 4 | 4.3.0 | × |
| icon | 设置头像的自定义图标 | ReactNode | - | shape | 指定头像的形状 | `circle` \| `square` | `circle` | size | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.7.0 | × |
| src | 图片类头像的资源地址或者图片元素 | string \| ReactNode | - | ReactNode: 4.8.0 | × |
| srcSet | 设置图片类头像响应式资源地址 | string | - | draggable | 图片是否允许拖动 | boolean \| `'true'` \| `'false'` | true | crossOrigin | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.17.0 | × |
| onError | 图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为 | () => boolean | - 
> Tip：你可以设置 `icon` 或 `children` 作为图片加载失败的默认 fallback 行为，优先级为 `icon` > `children`

### Avatar.Group 

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| max | 设置最多显示相关配置 | `{ count?: number; style?: CSSProperties; popover?: PopoverProps }` | - | 5.18.0 |
| ~~maxCount~~ | 已废弃，请使用 `max={{ count: number }}` | number | - | ~~maxPopoverTrigger~~ | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `hover` \| `focus` \| `click` | `hover` | size | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.8.0 |
| shape | 设置头像的形状 | `circle` \| `square` | `circle` | 5.8.0 |

### 导入方式

```js
import { Avatar } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `alt` | 图像无法显示时的替代文本 | string | - | — |
| `gap` | 字符类型距离左右两侧边界单位像素 | number | 4 | 4.3.0 |
| `icon` | 设置头像的自定义图标 | ReactNode | - | — |
| `shape` | 指定头像的形状 | `circle` \| `square` | `circle` | — |
| `size` | 设置头像的大小 | number \| `large` \| `medium` \| `small` \| { xs: number, sm: number, ...} | `medium` | 4.7.0 |
| `src` | 图片类头像的资源地址或者图片元素 | string \| ReactNode | - | ReactNode: 4.8.0 |
| `srcSet` | 设置图片类头像响应式资源地址 | string | - | — |
| `draggable` | 图片是否允许拖动 | boolean \| `'true'` \| `'false'` | true | — |
| `crossOrigin` | CORS 属性设置 | `'anonymous'` \| `'use-credentials'` \| `''` | - | 4.17.0 |
| `onError` | 图片加载失败的事件，返回 false 会关闭组件默认的 fallback 行为 | () => boolean | - | — |
| `max` | 设置最多显示相关配置 | `{ count?: number; style?: CSSProperties; popover?: PopoverProps }` | - | 5.18.0 |
| `maxCount` | 已废弃，请使用 `max={{ count: number }}` | number | - | — |
| `maxPopoverPlacement` | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `top` \| `bottom` | `top` | — |
| `maxPopoverTrigger` | 已废弃，请使用 `max={{ popover: PopoverProps }}` | `hover` \| `focus` \| `click` | `hover` | — |
| `maxStyle` | 已废弃，请使用 `max={{ style: CSSProperties }}` | CSSProperties | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Avatar** 的验收清单：

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

---
## 5. 参考链接
- 官方文档：https://ant.design/components/avatar
- 中文文档：https://ant.design/components/avatar-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/avatar
- 驱动 gpui kit：`avatar`
