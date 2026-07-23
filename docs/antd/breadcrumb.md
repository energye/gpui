# Breadcrumb 面包屑
> 来源：[Ant Design 6.5.x Breadcrumb](https://ant.design/components/breadcrumb)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：显示当前页面在系统层级结构中的位置，并能向上返回。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

显示当前页面在系统层级结构中的位置，并能向上返回。

**Breadcrumb** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 带有图标的 | icon 与文本混排 |
| 带有参数的 | 复现「带有参数的」视觉与布局 |
| 分隔符 | 复现「分隔符」视觉与布局 |
| 带下拉菜单的面包屑 | 复现「带下拉菜单的面包屑」视觉与布局 |
| 独立的分隔符 | 复现「独立的分隔符」视觉与布局 |
| Debug Routes | 复现「Debug Routes」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `dropdownIcon`

- **说明**：自定义下拉图标
- **类型**：ReactNode
- **默认值**：``
- **版本**：6.2.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `title`

- **说明**：名称
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.3.0

#### `type`

- **说明**：标记为分隔符
- **类型**：`separator`
- **默认值**：—
- **版本**：5.3.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `separator` | 官方取值 `separator` |

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

- 当系统拥有超过两级以上的层级结构时；
- 当需要告知用户『你在哪里』时；
- 当需要向上导航的功能时。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带有图标的**（`withIcon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **带有参数的**（`withParams.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **分隔符**（`separator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **带下拉菜单的面包屑**（`overlay.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **独立的分隔符**（`separator-component.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **Debug Routes**（`debug-routes.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 单击事件 |
| `items` | 数据化 items | 路由栈信息（>=5.3.0 推荐使用，旧版请使用 `Breadcrumb.Item` 子组件方式） |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 带有图标的 | `withIcon.tsx` | 否 |
| 带有参数的 | `withParams.tsx` | 否 |
| 分隔符 | `separator.tsx` | 否 |
| 带下拉菜单的面包屑 | `overlay.tsx` | 否 |
| 独立的分隔符 | `separator-component.tsx` | 否 |
| Debug Routes | `debug-routes.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
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

### Breadcrumb

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| dropdownIcon | 自定义下拉图标 | ReactNode | `<DownOutlined />` | 6.2.0 | 6.2.0 |
| items | 路由栈信息（>=5.3.0 推荐使用，旧版请使用 `Breadcrumb.Item` 子组件方式） | [ItemType\[\]](#itemtype) | - | 5.3.0 | × |
| itemRender | 自定义链接函数，和 react-router 配合使用，详见[示例](#use-with-browserhistory) | (route, params, routes, paths) => ReactNode | - | params | 路由的参数 | object | - | separator | 分隔符自定义 | ReactNode | `/` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |

### ItemType

> type ItemType = Omit<[RouteItemType](#routeitemtype), 'title' | 'path'> | [SeparatorType](#separatortype)

### RouteItemType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| className | 自定义类名 | string | - | href | 链接的目的地，不能和 `path` 共用 | string | - | menu | 菜单配置项 | [MenuProps](/components/menu-cn/#api) | - | 4.24.0 |
| onClick | 单击事件 | (e:MouseEvent) => void | - 
### SeparatorType

```ts
const item = {
  type: 'separator', // Must have
  separator: '/',
};
```

| 参数      | 说明           | 类型        | 默认值 | 版本  |
| --------- | -------------- | ----------- | ------ | ----- |
| type      | 标记为分隔符   | `separator` |        | 5.3.0 |
| separator | 要显示的分隔符 | ReactNode   | `/`    | 5.3.0 |

### 和 browserHistory 配合 {#use-with-browserhistory}

和 react-router 一起使用时，默认生成的 url 路径是带有 `#` 的，如果和 browserHistory 一起使用的话，你可以使用 `itemRender` 属性定义面包屑链接。

```jsx
import { Link } from 'react-router';

const items = [
  {
    path: '/index',
    title: 'home',
  },
  {
    path: '/first',
    title: 'first',
    children: [
      {
        path: '/general',
        title: 'General',
      },
      {
        path: '/layout',
        title: 'Layout',
      },
      {
        path: '/navigation',
        title: 'Navigation',
      },
    ],
  },
  {
    path: '/second',
    title: 'second',
  },
];

function itemRender(currentRoute, params, items, paths) {
  const isLast = currentRoute?.path === items[items.length - 1]?.path;

  return isLast ? (
    <span>{currentRoute.title}</span>
  ) : (
    <Link to={`/${paths.join('/')}`}>{currentRoute.title}</Link>
  );
}

return <Breadcrumb itemRender={itemRender} items={items} />;
```

### 导入方式

```js
import { Breadcrumb } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `dropdownIcon` | 自定义下拉图标 | ReactNode | `` | 6.2.0 |
| `items` | 路由栈信息（>=5.3.0 推荐使用，旧版请使用 `Breadcrumb.Item` 子组件方式） | [ItemType\[\]](#itemtype) | - | 5.3.0 |
| `itemRender` | 自定义链接函数，和 react-router 配合使用，详见[示例](#use-with-browserhistory) | (route, params, routes, paths) => ReactNode | - | — |
| `params` | 路由的参数 | object | - | — |
| `separator` | 分隔符自定义 | ReactNode | `/` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `className` | 自定义类名 | string | - | — |
| `dropdownProps` | 弹出下拉菜单的自定义配置 | [Dropdown](/components/dropdown-cn) | - | — |
| `href` | 链接的目的地，不能和 `path` 共用 | string | - | — |
| `path` | 拼接路径，每一层都会拼接前一个 `path` 信息。不能和 `href` 共用 | string | - | — |
| `menu` | 菜单配置项 | [MenuProps](/components/menu-cn/#api) | - | 4.24.0 |
| `onClick` | 单击事件 | (e:MouseEvent) => void | - | — |
| `title` | 名称 | ReactNode | - | 5.3.0 |
| `type` | 标记为分隔符 | `separator` | — | 5.3.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Breadcrumb** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/breadcrumb
- 中文文档：https://ant.design/components/breadcrumb-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/breadcrumb
- 驱动 gpui kit：`breadcrumb`
