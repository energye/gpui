# Breadcrumb 面包屑
> 来源：[Ant Design 6.5.x Breadcrumb](https://ant.design/components/breadcrumb)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：显示当前页面在系统层级结构中的位置，并能向上返回。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Breadcrumb** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/breadcrumb/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Breadcrumb）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 选中/展开/分页或步骤切换与键盘 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Breadcrumb）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：显示当前页面在系统层级结构中的位置，并能向上返回。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design Breadcrumb componentToken + 本库 Theme 默认** 为准（`scale=1`，种子：`fontSize=14`、`marginXS=8`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。源码：`components/breadcrumb/style/index.ts` `prepareComponentToken`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 | **14** | `fontSize` |
| 图标字号 | **14** | `iconFontSize` ← `fontSize` |
| 分隔符左右间距 | **8** | `separatorMargin` ← antd `marginXS`（kit 回落 `TokenMarginSM`/8；**非** kit `TokenMarginXS`=4） |
| 链接水平内边距 | **4** | `paddingXXS` |
| 链接圆角 | **4** | `borderRadiusSM` |
| 链接行高盒 | ≈ **fontHeight**（≈22） | 行盒；P0 以字号+pad 近似 |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

> Breadcrumb **无** size 阶梯（small/middle/large）；§6 通用模板的 controlHeight 不适用本控件。

#### 6.2.2 颜色 Token（语义 · Breadcrumb）

| 用途 | Token / 组件 Token | 备注 |
| --- | --- | --- |
| 项文字 / 链接默认 | `itemColor` / `linkColor` ← `colorTextDescription`（kit：`colorTextSecondary`） | 非主色 |
| 链接 hover | `linkHoverColor` ← `colorText` | 背景 `colorBgTextHover` |
| 末项 | `lastItemColor` ← `colorText` | 强调当前页 |
| 分隔符 | `separatorColor` ← `colorTextDescription` | 与项同级次 |
| 禁用（适用者） | `colorTextDisabled` | 无 hover 高亮 |

禁止硬编码品牌色（如 `#1890ff`）作为唯一默认皮；链接色走 description/text，**不是** `colorPrimary`。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `dropdownIcon` | 自定义下拉图标 | ReactNode | `<DownOutlined />` |
| `items` | 路由栈信息（>=5.3.0 推荐使用，旧版请使用 `Breadcrumb.Item` 子组件方式） | [ItemType\[\]](#itemtype) | - |
| `itemRender` | 自定义链接函数，和 react-router 配合使用，详见[示例](#use-with-browserhistory) | (route, params, routes, paths) => Rea… | - |
| `params` | 路由的参数 | object | - |
| `separator` | 分隔符自定义 | ReactNode | `/` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `className` | 自定义类名 | string | - |
| `dropdownProps` | 弹出下拉菜单的自定义配置 | [Dropdown](/components/dropdown-cn) | - |
| `href` | 链接的目的地，不能和 `path` 共用 | string | - |
| `path` | 拼接路径，每一层都会拼接前一个 `path` 信息。不能和 `href` 共用 | string | - |
| `menu` | 菜单配置项 | [MenuProps](/components/menu-cn/#api) | - |
| `onClick` | 单击事件 | (e:MouseEvent) => void | - |
| `title` | 名称 | ReactNode | - |
| `type` | 标记为分隔符 | `separator` | — |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
items 渲染 · separator ·
末项通常非链接
href/项点击 ──► 导航/回调
```

\*separatorMargin=8。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| BC-S1 | 三项普通项 | 两 separator（项间自动插入） |
| BC-S2 | 自定义 separator（根 `separator`） | 可见且替换默认 `/` |
| BC-S3 | 链接项点击（`href`/`path`/`Link`/`OnClick`） | 回调/路由 |
| BC-S4 | 末项 | 非链接强调（`lastItemColor`）；无自动 separator 尾随 |
| BC-S5 | 带 `menu` 的项 | 可下拉（P0，组合 Dropdown） |
| BC-S6 | separator 间距 | ≈8（`separatorMargin`） |
| BC-S7 | `type: 'separator'` 独立分隔项 | 使用项级 `separator` 文案；根 `separator=""` 时不自动插 sep |
| BC-S8 | `params` 替换 title/path 中 `:key` | 展示替换后文案 |
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
| 角色 | navigation / menu / tablist 等 |
| 当前 | aria-current / selected |
| 键盘 | 方向键与激活 |

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
| `items` / `title` | 数据驱动路由栈（`BreadcrumbItem`） |
| `type: 'separator'` | 独立分隔项（`SeparatorType`） |
| `separator` | 根级分隔符，默认 `/`；可设空串关闭自动 sep |
| `params` | title/path 中 `:key` 替换 |
| `href` / `path` / `Link` | 链接项；path 逐层拼接为 `#/a/b` |
| `onClick` | 项级 + 根级回调 |
| `menu` | 项级下拉二选一：A 依赖 `kit.Dropdown` 底座（hover 触发）；B 无底座时降级为不可展开、点击只走 `OnClick`/`OnMenuClick`，所选分支写入 coverage Notes |
| `dropdownIcon` | 自定义下拉图标（默认 chevron-down） |
| `itemRender` | 自定义项内容钩子（浏览器History 映射） |
| 图标项 | `Icon` / `IconNode` / `TitleNode` |
| 官方主路径示例 | **基本**、**带有图标的**、**带有参数的**、**分隔符**、**带下拉菜单的面包屑**、**独立的分隔符**、**Debug Routes**（legacy `routes`→items+menu 映射） |
| 度量 §6.2 | Token 断言（字号 14、separatorMargin 8、色走 Token） |
| a11y §6.6 | `role=navigation`；末项 `aria-current=page`；链接可聚焦/键盘激活 |
| §6.9 中 L1/L2 **非 P1** 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic `classNames` / `styles` 深度 | 官方 `style-class.tsx` / `_semantic.tsx` |
| `dropdownProps` 全量透传 | 分期 |
| 动画像素级 / 链接色 transition | 分期 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| component-token / 官网逐像素哈希 | 分期 |
| ConfigProvider 全局 breadcrumb 默认 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestBreadcrumb_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Breadcrumb 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| BC-01 | L1 | NewBreadcrumb 默认创建 | 不崩溃；separator=`/`；role=navigation |
| BC-02 | L1 | 三项普通项 | 两 separator（BC-S1） |
| BC-03 | L1 | 自定义 separator `>` | 可见（BC-S2） |
| BC-04 | L1 | 链接项点击 | OnClick 回调（BC-S3） |
| BC-05 | L1 | 末项 | 非链接；`aria-current=page`；lastItem 色（BC-S4） |
| BC-06 | L1 | 带 menu 的项 | Dropdown 可开（BC-S5） |
| BC-07 | L1 | separator 间距 | ≈8（BC-S6） |
| BC-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 四项；中间链接可点；末项非链接 |
| BC-09 | L1 | 复现官方示例「带有图标的」（`withIcon.tsx`） | Icon/Title 混排；链接可点 |
| BC-10 | L1 | 复现官方示例「带有参数的」（`withParams.tsx`） | `:id` → 替换后标题（BC-S8） |
| BC-11 | L1 | 复现官方示例「分隔符」（`separator.tsx`） | 根 separator=`>` |
| BC-12 | L1 | 复现官方示例「带下拉菜单的面包屑」（`overlay.tsx`） | menu 项可开下拉 |
| BC-13 | L1 | 复现官方示例「独立的分隔符」（`separator-component.tsx`） | type=separator；根 separator 空（BC-S7） |
| BC-14 | L1 | 复现官方示例「Debug Routes」（`debug-routes.tsx`） | routes→items；children→menu |
| BC-15 | P1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | semantic classNames/styles（§6.8 P1） |
| BC-16 | L2 | 读取 §6.2 关键尺寸/间距 | fontSize=14、separatorMargin=8（±0.5） |
| BC-17 | L2 | 默认皮颜色 | item/link/last/sep 走 Theme Token；无硬编码品牌色 |
| BC-18 | L2 | 项 Disabled（适用者） | 禁用色；不可点；无 hover 高亮 |
| BC-19 | L1 | 键盘/焦点主路径 | 链接可聚焦；Enter/Space 激活；Focus ring 可见 |
| BC-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差；可另测） |
| BC-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| BC-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。旧 `NewBreadcrumb(...string)` / `Items []string` **删除**。

```text
type BreadcrumbItem struct {
  Type      string      // "" | "separator"
  Title     string
  TitleNode core.Node   // 优先于 Title（图标混排）
  Icon      string      // registry 名
  IconNode  core.Node
  Href      string
  Path      string
  Link      bool        // antd href:'' → 显式链接（空串仍是链接）
  Menu      []MenuItem
  OnClick   func()
  Separator string      // type=separator 时的文案；空 → 根 separator 或 "/"
  Key       string
  Disabled  bool
}

NewBreadcrumb(items ...BreadcrumbItem) *Breadcrumb
BreadcrumbTitles(titles ...string) []BreadcrumbItem   // 便捷：纯文案项
BreadcrumbFromRoutes(routes ...BreadcrumbItem) []BreadcrumbItem // legacy routes：children→menu、breadcrumbName→title

// 配置
SetItems / SetSeparator / SetParams / SetDropdownIcon / SetDropdownIconNode
SetItemRender / SetOnClick / SetOnMenuClick
SetFace / SetTheme / SetAriaLabel
// 查询（测试 / a11y）
Node / ChromeNode
SeparatorCount / ItemCount / ResolvedSeparator / ResolvedSeparatorMargin
ResolvedFontSize / ItemColor / LinkColor / LastItemColor / SeparatorColor
ItemPressable(i) / ItemDropdown(i) / DisplayTitle(i)
// a11y：root role=navigation；末项 aria-current=page
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Separator | `/`（`SetSeparator("")` 显式空 → 关闭自动 sep） |
| Params | nil / 空 map |
| DropdownIcon | `chevron-down` |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
nav (Flex, role=navigation)
  └─ ol (Flex row wrap, gap via separator margin)
       ├─ item (Text | Pressable link | Dropdown+overlay-link)
       ├─ separator (Text, marginInline=separatorMargin)
       └─ …
```

- 组合 `ui/primitive` + `ui/core` + `kit.Icon`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index（选 Dropdown 底座分支时 menu 走底座；降级分支无浮层）；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion（P0 瞬时开合即可）。

### 6.12 完成定义（DoD）

同时满足即可宣布 **Breadcrumb 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/breadcrumb.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Breadcrumb 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
