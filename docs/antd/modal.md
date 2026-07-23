# Modal 对话框
> 来源：[Ant Design 6.5.x Modal](https://ant.design/components/modal)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：展示一个对话框，提供标题、内容区、操作区。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

展示一个对话框，提供标题、内容区、操作区。

**Modal** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 异步关闭 | 复现「异步关闭」视觉与布局 |
| 自定义页脚 | 自定义渲染/插槽外观 |
| 遮罩 | mask 层 |
| 加载中 | loading 指示与防重复 |
| 自定义页脚渲染函数 | 自定义渲染/插槽外观 |
| 使用 hooks 获得上下文 | 复现「使用 hooks 获得上下文」视觉与布局 |
| 国际化 | 复现「国际化」视觉与布局 |
| 手动更新和移除 | 复现「手动更新和移除」视觉与布局 |
| 自定义位置 | placement 方位 |
| 自定义页脚按钮属性 | 自定义渲染/插槽外观 |
| 自定义渲染对话框 | 自定义渲染/插槽外观 |
| 自定义模态的宽度 | 自定义渲染/插槽外观 |
| 静态方法 | 复现「静态方法」视觉与布局 |
| 静态确认对话框 | 复现「静态确认对话框」视觉与布局 |
| 销毁确认对话框 | 复现「销毁确认对话框」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `centered`

- **说明**：垂直居中展示 Modal
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义 Modal 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `closable`

- **说明**：是否显示右上角的关闭按钮
- **类型**：boolean | [ClosableType](#closabletype)
- **默认值**：true

#### `closeIcon`

- **说明**：自定义关闭图标。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮
- **类型**：ReactNode
- **默认值**：<CloseOutlined />

#### `footer`

- **说明**：底部内容，当不需要默认底部按钮时，可以设为 `footer={null}`
- **类型**：ReactNode | (originNode: ReactNode, extra: { OkBtn: React.FC, CancelBtn: React.FC }) => ReactNode
- **默认值**：(确定取消按钮)
- **版本**：renderFunction: 5.9.0

#### `getContainer`

- **说明**：指定 Modal 挂载的节点，但依旧为全屏展示，`false` 为挂载在当前位置
- **类型**：HTMLElement | () => HTMLElement | Selectors | false
- **默认值**：document.body

#### `keyboard`

- **说明**：是否支持键盘 esc 关闭
- **类型**：boolean
- **默认值**：true

#### `mask`

- **说明**：遮罩效果
- **类型**：boolean | `{enabled: boolean, blur: boolean, closable?: boolean}`
- **默认值**：true
- **版本**：mask.closable: 6.3.0

#### `okType`

- **说明**：确认按钮类型
- **类型**：string
- **默认值**：`primary`

#### `style`

- **说明**：可用于设置浮层的样式，调整浮层位置等
- **类型**：CSSProperties
- **默认值**：-

#### `styles`

- **说明**：用于自定义 Modal 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `loading`

- **说明**：显示骨架屏
- **类型**：boolean
- **默认值**：—
- **版本**：5.18.0

#### `title`

- **说明**：标题
- **类型**：ReactNode
- **默认值**：-

#### `width`

- **说明**：宽度
- **类型**：string | number | [Breakpoint](/components/grid-cn#col)
- **默认值**：520
- **版本**：Breakpoint: 5.23.0

#### `zIndex`

- **说明**：设置 Modal 的 `z-index`
- **类型**：number
- **默认值**：1000
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `z-index` | 官方取值 `z-index` |

#### `onCancel`

- **说明**：点击遮罩层或右上角叉或取消按钮的回调
- **类型**：function(e)
- **默认值**：-

#### `content`

- **说明**：内容
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义图标
- **类型**：ReactNode
- **默认值**：<ExclamationCircleFilled />

#### `disabled`

- **说明**：关闭图标是否禁用
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

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

需要用户处理事务，又不希望跳转页面以致打断工作流程时，可以使用 `Modal` 在当前页面正中打开一个浮层，承载相应的操作。

另外当需要一个简洁的确认框询问用户时，可以使用 [`App.useApp`](/components/app-cn/) 封装的语法糖方法。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **异步关闭**（`async.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义页脚**（`footer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **遮罩**（`mask.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **加载中**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义页脚渲染函数**（`footer-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **使用 hooks 获得上下文**（`hooks.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **国际化**（`locale.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **手动更新和移除**（`manual.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义位置**（`position.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义页脚按钮属性**（`button-props.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义渲染对话框**（`modal-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义模态的宽度**（`width.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **静态方法**（`static-info.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **静态确认对话框**（`confirm.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **销毁确认对话框**（`confirm-router.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `open` | 受控显隐 | 对话框是否可见 |
| `disabled` | 禁用 | 关闭图标是否禁用 |
| `loading` | 加载中 | 显示骨架屏 |
| `destroyOnHidden` | 隐藏销毁 | 关闭时销毁 Modal 里的子元素 |
| `onCancel` | 取消 | 点击遮罩层或右上角叉或取消按钮的回调 |
| `onOk` | 确定 | 点击确定回调 |
| `afterClose` | 关闭后 | Modal 完全关闭后的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 异步关闭 | `async.tsx` | 否 |
| 自定义页脚 | `footer.tsx` | 否 |
| 遮罩 | `mask.tsx` | 否 |
| 加载中 | `loading.tsx` | 否 |
| 自定义页脚渲染函数 | `footer-render.tsx` | 否 |
| 使用 hooks 获得上下文 | `hooks.tsx` | 否 |
| 国际化 | `locale.tsx` | 否 |
| 手动更新和移除 | `manual.tsx` | 否 |
| 自定义位置 | `position.tsx` | 否 |
| 调试使用 | `dark.tsx` | 是 |
| 自定义页脚按钮属性 | `button-props.tsx` | 否 |
| 自定义渲染对话框 | `modal-render.tsx` | 否 |
| 自定义模态的宽度 | `width.tsx` | 否 |
| 静态方法 | `static-info.tsx` | 否 |
| 静态确认对话框 | `confirm.tsx` | 否 |
| 销毁确认对话框 | `confirm-router.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 嵌套弹框 | `nested.tsx` | 是 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 控制弹框动画原点 | `custom-mouse-position.tsx` | 是 |
| 线框风格 | `wireframe.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 为什么 Modal 方法不能获取 context、redux 的内容和 ConfigProvider `locale/prefixCls/theme` 等配置？ {#faq-context-redux}

直接调用 Modal 方法，antd 会通过 `ReactDOM.render` 动态创建新的 React 实体。其 context 与当前代码所在 context 并不相同，因而无法获取 context 信息。

当你需要 context 信息（例如 ConfigProvider 配置的内容）时，可以通过 `Modal.useModal` 方法会返回 `modal` 实体以及 `contextHolder` 节点。将其插入到你需要获取 context 位置即可：

```tsx
const [modal, contextHolder] = Modal.useModal();

return (
  <Context1.Provider value="Ant">
    {/* contextHolder 在 Context1 内，它可以获得 Context1 的 context */}
    {contextHolder}
    <Context2.Provider value="Design">
      {/* contextHolder 在 Context2 外，因而不会获得 Context2 的 context */}
    </Context2.Provider>
  </Context1.Provider>
);
```

**异同**：通过 hooks 创建的 `contextHolder` 必须插入到子元素节点中才会生效，当你不需要上下文信息时请直接调用。

> 可通过 [App 包裹组件](/components/app-cn) 简化 `useModal` 等方法需要手动植入 contextHolder 的问题。

#### 方法

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

### 2.6 FAQ

## FAQ

### 为什么 Modal 关闭时，内容不会更新？ {#faq-content-not-update}

Modal 在关闭时会将内容进行 memo 从而避免关闭过程中的内容跳跃。也因此如果你在配合使用 Form 有关闭时重置 `initialValues` 的操作，请通过在 effect 中调用 `resetFields` 来重置。

### 为什么 Modal 方法不能获取 context、redux 的内容和 ConfigProvider `locale/prefixCls/theme` 等配置？ {#faq-context-redux}

直接调用 Modal 方法，antd 会通过 `ReactDOM.render` 动态创建新的 React 实体。其 context 与当前代码所在 context 并不相同，因而无法获取 context 信息。

当你需要 context 信息（例如 ConfigProvider 配置的内容）时，可以通过 `Modal.useModal` 方法会返回 `modal` 实体以及 `contextHolder` 节点。将其插入到你需要获取 context 位置即可：

```tsx
const [modal, contextHolder] = Modal.useModal();

return (
  
    {/* contextHolder 在 Context1 内，它可以获得 Context1 的 context */}
    {contextHolder}
    
      {/* contextHolder 在 Context2 外，因而不会获得 Context2 的 context */}
    
  
);
```

**异同**：通过 hooks 创建的 `contextHolder` 必须插入到子元素节点中才会生效，当你不需要上下文信息时请直接调用。

> 可通过 [App 包裹组件](/components/app-cn) 简化 `useModal` 等方法需要手动植入 contextHolder 的问题。

### 静态方法如何设置 prefixCls ？ {#faq-set-prefix-cls}

你可以通过 [`ConfigProvider.config`](/components/config-provider-cn#configproviderconfig-4130) 进行设置。

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
| afterClose | Modal 完全关闭后的回调 | function | - | cancelButtonProps | cancel 按钮 props | [ButtonProps](/components/button-cn#api) | - | cancelText | 取消按钮文字 | ReactNode | `取消` | centered | 垂直居中展示 Modal | boolean | false | classNames | 用于自定义 Modal 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | closable | 是否显示右上角的关闭按钮 | boolean \| [ClosableType](#closabletype) | true | - | 5.16.0 |
| closeIcon | 自定义关闭图标。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | &lt;CloseOutlined /> | confirmLoading | 确定按钮 loading | boolean | false | ~~destroyOnClose~~ | 关闭时销毁 Modal 里的子元素 | boolean | false | destroyOnHidden | 关闭时销毁 Modal 里的子元素 | boolean | false | 5.25.0 | × |
| ~~focusTriggerAfterClose~~ | 对话框关闭后是否需要聚焦触发元素。请使用 `focusable.focusTriggerAfterClose` 替代 | boolean | true | 4.9.0 | × |
| footer | 底部内容，当不需要默认底部按钮时，可以设为 `footer={null}` | ReactNode \| (originNode: ReactNode, extra: { OkBtn: React.FC, CancelBtn: React.FC }) => ReactNode | (确定取消按钮) | renderFunction: 5.9.0 | × |
| forceRender | 强制渲染 Modal | boolean | false | focusable | 对话框内焦点管理的配置 | `{ trap?: boolean, focusTriggerAfterClose?: boolean }` | - | 6.2.0 | 6.4.0 |
| getContainer | 指定 Modal 挂载的节点，但依旧为全屏展示，`false` 为挂载在当前位置 | HTMLElement \| () => HTMLElement \| Selectors \| false | document.body | keyboard | 是否支持键盘 esc 关闭 | boolean | true | mask | 遮罩效果 | boolean \| `{enabled: boolean, blur: boolean, closable?: boolean}` | true | mask.closable: 6.3.0 | 6.0.0，mask.closable: 6.3.0 |
| ~~maskClosable~~ | 点击蒙层是否允许关闭。请使用 `mask.closable` 替代。 | boolean | true | - | × |
| modalRender | 自定义渲染对话框 | (node: ReactNode) => ReactNode | - | 4.7.0 | × |
| okButtonProps | ok 按钮 props | [ButtonProps](/components/button-cn#api) | - | okText | 确认按钮文字 | ReactNode | `确定` | okType | 确认按钮类型 | string | `primary` | style | 可用于设置浮层的样式，调整浮层位置等 | CSSProperties | - | styles | 用于自定义 Modal 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | loading | 显示骨架屏 | boolean | scrollLock | 弹窗打开时是否锁定body滚动 | boolean | true | 6.5.0 | × |
| title | 标题 | ReactNode | - | open | 对话框是否可见 | boolean | - | width | 宽度 | string \| number \| [Breakpoint](/components/grid-cn#col) | 520 | Breakpoint: 5.23.0 | × |
| wrapClassName | 对话框外层容器的类名 | string | - | zIndex | 设置 Modal 的 `z-index` | number | 1000 | onCancel | 点击遮罩层或右上角叉或取消按钮的回调 | function(e) | - | onOk | 点击确定回调 | function(e) | - | afterOpenChange | 打开和关闭 Modal 时动画结束后的回调 | (open: boolean) => void | - | 5.4.0 | × |

#### 注意

- `<Modal />` 默认关闭后状态不会自动清空，如果希望每次打开都是新内容，请设置 `destroyOnHidden`。
- `<Modal />` 和 Form 一起配合使用时，设置 `destroyOnHidden` 也不会在 Modal 关闭时销毁表单字段数据，需要设置 `<Form preserve={false} />`。
- `Modal.method()` RTL 模式仅支持 hooks 用法。

### Modal.method()

包括：

- `Modal.info`
- `Modal.success`
- `Modal.error`
- `Modal.warning`
- `Modal.confirm`

以上均为一个函数，参数为 object，具体属性如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| afterClose | Modal 完全关闭后的回调 | function | - | 4.9.0 |
| ~~autoFocusButton~~ | 指定自动获得焦点的按钮。请使用 `focusable.autoFocusButton` 替代 | null \| `ok` \| `cancel` | `ok` | cancelText | 设置 Modal.confirm 取消按钮文字 | string | `取消` | className | 容器类名 | string | - | closeIcon | 自定义关闭图标 | ReactNode | undefined | 4.9.0 |
| content | 内容 | ReactNode | - | footer | 底部内容，当不需要默认底部按钮时，可以设为 `footer: null` | ReactNode \| (originNode: ReactNode, extra: { OkBtn: React.FC, CancelBtn: React.FC }) => ReactNode | - | renderFunction: 5.9.0 |
| getContainer | 指定 Modal 挂载的 HTML 节点，false 为挂载在当前 dom | HTMLElement \| () => HTMLElement \| Selectors \| false | document.body | keyboard | 是否支持键盘 esc 关闭 | boolean | true | ~~maskClosable~~ | 点击蒙层是否允许关闭。请使用 `mask.closable` 替代。 | boolean | false | - |
| scrollLock | 弹窗打开时是否锁定body滚动 | boolean | true | 6.5.0 |
| okButtonProps | ok 按钮 props | [ButtonProps](/components/button-cn#api) | - | okType | 确认按钮类型 | string | `primary` | title | 标题 | ReactNode | - | wrapClassName | 对话框外层容器的类名 | string | - | 4.18.0 |
| zIndex | 设置 Modal 的 `z-index` | number | 1000 | onOk | 点击确定回调，参数为关闭函数，若返回 promise 时 resolve 为正常关闭, reject 为不关闭 | function(close) | - 
### ClosableType

| 参数       | 说明                   | 类型      | 默认值    | 版本 |
| ---------- | ---------------------- | --------- | --------- | ---- |
| afterClose | Modal 完全关闭后的回调 | function  | -         | -    |
| closeIcon  | 自定义关闭图标         | ReactNode | undefined | -    |
| disabled   | 关闭图标是否禁用       | boolean   | false     | -    |
| onClose    | 弹窗关闭即时调用       | Function  | undefined | -    |

```jsx
const modal = Modal.info();

modal.update({
  title: '修改的标题',
  content: '修改的内容',
});

// 在 4.8.0 或更高版本中，可以通过传入函数的方式更新弹窗
modal.update((prevConfig) => ({
  ...prevConfig,
  title: `${prevConfig.title}（新）`,
}));

modal.destroy();
```

- `Modal.destroyAll`

使用 `Modal.destroyAll()` 可以销毁弹出的确认窗（即上述的 `Modal.info`、`Modal.success`、`Modal.error`、`Modal.warning`、`Modal.confirm`）。通常用于路由监听当中，处理路由前进、后退不能销毁确认对话框的问题，而不用各处去使用实例的返回值进行关闭（`modal.destroy()` 适用于主动关闭，而不是路由这样被动关闭）

```jsx
import { browserHistory } from 'react-router';

// router change
browserHistory.listen(() => {
  Modal.destroyAll();
});
```

### Modal.useModal()

当你需要使用 Context 时，可以通过 `Modal.useModal` 创建一个 `contextHolder` 插入子节点中。通过 hooks 创建的临时 Modal 将会得到 `contextHolder` 所在位置的所有上下文。创建的 `modal` 对象拥有与 [`Modal.method`](#modalmethod) 相同的创建通知方法。

```jsx
const [modal, contextHolder] = Modal.useModal();

React.useEffect(() => {
  modal.confirm({
    // ...
  });
}, []);

return <div>{contextHolder}</div>;
```

`modal.confirm` 返回方法：

- `destroy`：销毁当前窗口
- `update`：更新当前窗口
- `then`：Promise 链式调用，支持 `await` 操作。该方法为 Hooks 仅有

```tsx
//点击 `onOk` 时返回 `true`，点击 `onCancel` 时返回 `false`
const confirmed = await modal.confirm({ ... });
```

### 导入方式

```js
import { Modal } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `afterClose` | Modal 完全关闭后的回调 | function | - | — |
| `cancelButtonProps` | cancel 按钮 props | [ButtonProps](/components/button-cn#api) | - | — |
| `cancelText` | 取消按钮文字 | ReactNode | `取消` | — |
| `centered` | 垂直居中展示 Modal | boolean | false | — |
| `classNames` | 用于自定义 Modal 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `closable` | 是否显示右上角的关闭按钮 | boolean \| [ClosableType](#closabletype) | true | - |
| `closeIcon` | 自定义关闭图标。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | <CloseOutlined /> | — |
| `confirmLoading` | 确定按钮 loading | boolean | false | — |
| `destroyOnClose` | 关闭时销毁 Modal 里的子元素 | boolean | false | — |
| `destroyOnHidden` | 关闭时销毁 Modal 里的子元素 | boolean | false | 5.25.0 |
| `focusTriggerAfterClose` | 对话框关闭后是否需要聚焦触发元素。请使用 `focusable.focusTriggerAfterClose` 替代 | boolean | true | 4.9.0 |
| `footer` | 底部内容，当不需要默认底部按钮时，可以设为 `footer={null}` | ReactNode \| (originNode: ReactNode, extra: { OkBtn: React.FC, CancelBtn: React.FC }) => ReactNode | (确定取消按钮) | renderFunction: 5.9.0 |
| `forceRender` | 强制渲染 Modal | boolean | false | — |
| `focusable` | 对话框内焦点管理的配置 | `{ trap?: boolean, focusTriggerAfterClose?: boolean }` | - | 6.2.0 |
| `getContainer` | 指定 Modal 挂载的节点，但依旧为全屏展示，`false` 为挂载在当前位置 | HTMLElement \| () => HTMLElement \| Selectors \| false | document.body | — |
| `keyboard` | 是否支持键盘 esc 关闭 | boolean | true | — |
| `mask` | 遮罩效果 | boolean \| `{enabled: boolean, blur: boolean, closable?: boolean}` | true | mask.closable: 6.3.0 |
| `maskClosable` | 点击蒙层是否允许关闭。请使用 `mask.closable` 替代。 | boolean | true | - |
| `modalRender` | 自定义渲染对话框 | (node: ReactNode) => ReactNode | - | 4.7.0 |
| `okButtonProps` | ok 按钮 props | [ButtonProps](/components/button-cn#api) | - | — |
| `okText` | 确认按钮文字 | ReactNode | `确定` | — |
| `okType` | 确认按钮类型 | string | `primary` | — |
| `style` | 可用于设置浮层的样式，调整浮层位置等 | CSSProperties | - | — |
| `styles` | 用于自定义 Modal 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `loading` | 显示骨架屏 | boolean | — | 5.18.0 |
| `scrollLock` | 弹窗打开时是否锁定body滚动 | boolean | true | 6.5.0 |
| `title` | 标题 | ReactNode | - | — |
| `open` | 对话框是否可见 | boolean | - | — |
| `width` | 宽度 | string \| number \| [Breakpoint](/components/grid-cn#col) | 520 | Breakpoint: 5.23.0 |
| `wrapClassName` | 对话框外层容器的类名 | string | - | — |
| `zIndex` | 设置 Modal 的 `z-index` | number | 1000 | — |
| `onCancel` | 点击遮罩层或右上角叉或取消按钮的回调 | function(e) | - | — |
| `onOk` | 点击确定回调 | function(e) | - | — |
| `afterOpenChange` | 打开和关闭 Modal 时动画结束后的回调 | (open: boolean) => void | - | 5.4.0 |
| `autoFocusButton` | 指定自动获得焦点的按钮。请使用 `focusable.autoFocusButton` 替代 | null \| `ok` \| `cancel` | `ok` | — |
| `className` | 容器类名 | string | - | — |
| `content` | 内容 | ReactNode | - | — |
| `focusable.autoFocusButton` | 指定自动获得焦点的按钮 | null \| `ok` \| `cancel` | `ok` | 6.2.0 |
| `icon` | 自定义图标 | ReactNode | <ExclamationCircleFilled /> | — |
| `disabled` | 关闭图标是否禁用 | boolean | false | - |
| `onClose` | 弹窗关闭即时调用 | Function | undefined | - |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Modal** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **17** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/modal
- 中文文档：https://ant.design/components/modal-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/modal
- 驱动 gpui kit：`modal`
