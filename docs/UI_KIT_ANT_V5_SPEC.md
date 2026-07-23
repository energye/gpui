# UI Kit · Ant Design v5 对齐规格

> 版本：1.0 | 日期：2026-07-23  
> 状态：**活文档 · kit 产品对齐目标与验收大纲**  
> **实现纪律（必读）：** [`UI_KIT_DEV_GUIDE.md`](./UI_KIT_DEV_GUIDE.md)  
> **底座能力：** [`UI_FOUNDATION_P0.md`](./UI_FOUNDATION_P0.md) · [`LAYOUT_FOUNDATION.md`](./LAYOUT_FOUNDATION.md)  
> **状态权威表：** [`ui/kit/coverage.go`](../ui/kit/coverage.go) · 摘要 [`UI_KIT_COVERAGE.md`](./UI_KIT_COVERAGE.md)  
> **架构：** [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) · **App 壳：** [`UI_APP_SHELL_PLAN.md`](./UI_APP_SHELL_PLAN.md)

---

## 0. 文档定位（三件套）

| 文档 | 管什么 |
|------|--------|
| **本文 SPEC** | 对齐 Ant Design **v5 的什么**（能力、状态、验收类别、分期） |
| [`UI_KIT_DEV_GUIDE.md`](./UI_KIT_DEV_GUIDE.md) | **怎么实现**（primitive 组合、禁止每帧 Sync、Theme hook、Present 路径） |
| [`coverage.go`](../ui/kit/coverage.go) | **做到哪了**（Ready + Notes；改表只改源码） |

与代码冲突时：**以源码与 `go test` 为准**，再回写文档。

---

## 1. 目标

| 项 | 约定 |
|----|------|
| 基线版本 | **Ant Design v5**（桌面主路径） |
| 适用范围 | 当前 `ui/kit` 已有控件与后续同类扩展 |
| **不含** | Pro Components、纯 Web 专属能力、系统原生菜单/托盘（另轨） |
| 底层约束 | 只用 `ui/core` + `ui/primitive` + `ui/layer` + `ui/app`；**禁止** kit 自建第二套渲染/事件/帧循环 |

### 1.1 对齐级别（替代笼统「像素级」）

美观与一致 **必须**做到，但度量分层：

| 级别 | 名称 | 标准 | 验收 |
|------|------|------|------|
| **L1** | 行为 | 与 Ant v5 桌面主路径一致：开合、选中、键盘、禁用、校验、浮层规则等 | Headless / behavior 测 |
| **L2** | Token / 几何 | 色、字号、圆角、间距、控件高度与 **Token 基线**一致 | `ant_style_test` + 控件读 Token |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与 **仓库基线**一致（容差抗 AA） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时签字，**非 CI 哈希官网** |

**明确不做：**

- 与浏览器渲染 ant.design **逐像素哈希**一致（字体/AA/DPR 栈不同，不可达且绑死错误 KPI）。  
- 为「抠官网截图」破坏 `hit == layout == paint` 或引入 per-control 魔法 offset。

**美观怎么保证：** L2 锁设计系统 + L3 锁回归 + L4 建基线人眼。这比假「像素级」更能做出好看的控件。

---

## 2. 全局规范

### 2.1 Theme / Token

- 色、字号、圆角、间距、尺寸一律从 `core.Theme` / `TokenSet` 读取（`themeOf` / ConfigProvider / Tree.Theme）。  
- 默认基线（middle，与源码一致，见 `ant_style_test.go`）：

| Token | 值 |
|-------|-----|
| controlHeight | **32** |
| controlHeightSM | **24** |
| controlHeightLG | **40** |
| fontSize | **14** |
| borderRadius | **6** |
| button paddingInline | **15** |
| control paddingInline | **11** |
| lineWidth | **1** |

- 换肤：`ConfigProvider` 或 `Tree.SetTheme`；控件 `SetThemeHook` 重建 chrome（见 DEV_GUIDE）。  
- 禁止默认硬编码品牌色；单实例覆盖用 `Theme` 字段或 `Style`。

### 2.2 状态矩阵（产品控件）

按控件适用子集实现：

`default` / `hover` / `active` / `focus` / `disabled` / `loading` / `selected` / `open` / `checked` / `indeterminate` / `error` / `warning`

- 交互态（hover/active/focus/disabled）优先来自 **primitive**（Pressable 等）。  
- loading / validate / error 由 **kit 状态机**叠加。  
- 更新 chrome：挂 `OnStateChange`，**禁止**新增「主循环必须每帧 SyncState」。

### 2.3 浮层

所有弹层（Modal / Drawer / Dropdown / Select / Tooltip / Popover / Popconfirm / Message / Tour…）：

| 要求 | 说明 |
|------|------|
| Portal | `OverlayPortal`，禁止 in-tree 假遮罩指望盖住 Boundary |
| 合成 | 真窗必须 `OwnedPresenter` 双带（MAP §4.1） |
| outside dismiss | 默认；`OnDismiss` 同步产品 `Open` |
| ESC / focus trap | Modal/Drawer 等已有 FocusScope 路径 |
| z-order | `Portal.ZOrder` |
| 锚点 | `AnchoredPopup` + AnchorNode；**Layout 后自动刷新**，勿依赖每帧 `Sync()`（已 Deprecated） |

### 2.4 输入类

| 要求 | 说明 |
|------|------|
| 键盘 / 选区 / 清空 / 禁用 / 只读 / maxLength | Editable + kit 壳 |
| IME | Headless 可测 composition；**Linux 真窗无 CapIME**（正式降级，见 `platform/ime.go`） |
| 拖选 / Shift+扩展 | 底层已交付 |
| 撤销/重做 | **后置**（Editable 无历史栈前勿写「必须」） |
| 密码掩码 | 需在 primitive/kit 显式实现后再验收 |

### 2.5 动画

- 必须 `Tree.AddTicker` + demand frame。  
- **禁止** product 控件 `ContinuousRender=true`。  
- 停止后注销 ticker，不再空转。

### 2.6 桌面 vs Web（Ant 差异）

| Web Ant | 本库桌面 |
|---------|----------|
| DOM focus / outline | `FocusRing` / Decorated 焦点边 |
| `<input type=file>` | `CapFile` / 注入 Picker |
| 浏览器滚动条 | `ScrollViewport` + 可选 bar |
| 系统深色 | 业务 `SetTheme`；Host Caps 后接 |
| 嵌套路由/SSR | 不适用 |

行为对齐 **桌面主路径**，不追 Web 专属 API。

---

## 3. 分期（Wave）

与 `coverage.go` **Notes** 对齐：Ready ≠ 全 props。

| Wave | 目标 | 代表控件 |
|------|------|----------|
| **A · 主路径打磨** | L1+L2 扎实；关键态 L3 | Button、Input、Checkbox/Radio/Switch、Form 基、Modal/Drawer、Select、Tabs、Message |
| **B · 密度与导航** | 表/树/菜单深挖 | Table、List、Tree、Menu（嵌套）、Pagination、Dropdown、Transfer、Cascader |
| **C · 长尾 / Notes** | 补 Notes 或标 WontFix | Date **range**、Upload **host dialog**、Image **GPU**、QR **codec**、ColorPicker 全盘、真 Affix sticky 等 |

每 Wave 结束：更新 `coverage.go` Notes + 跑 §5 验收命令。

---

## 4. 组件需求（能力大纲）

下列为 **对齐目标清单**。实现时须再拆 **Props 表**（可写在控件旁 `// Ant:` 注释或本文件附录）。依赖未齐的 primitive 时，**先补底座或标后置**，禁止 kit 内复制 Hit/浮层。

### 4.1 通用

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Button** | default/primary/dashed/text/link、danger、loading、block、icon；尺寸与 hover/active/focus | ghost 若未齐则 Notes |
| **FloatButton** | 圆/方、图标、偏移、组 | 定位用布局，非系统 always-on-top |
| **Icon** | 注册表、色/尺寸 token | Path/SVG later |
| **Typography** | Title/Paragraph/Text；层级、次要、截断 | `Text.Ellipsis` / MaxLines |

### 4.2 布局

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Divider** | 水平/垂直、文案、虚线 | — |
| **Flex** | gap、justify、align、wrap | 用 primitive.Flex |
| **Grid** | span/gutter | simplified；完整 24 栅格+断点 = Wave B/C |
| **Layout** | header/sider/content/footer | Flex 嵌套 |
| **Space** | 间距、wrap、size | Flex+gap |
| **Splitter** | 拖拽、min/max | SplitPane/Draggable |

### 4.3 导航

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Anchor** | 滚动 spy、高亮、点击定位 | Notes：spy 基线 |
| **Breadcrumb** | 分隔、点击 | — |
| **Dropdown** | placement、outside、键盘 | 浮层规范 |
| **Menu** | 选中、分组、键盘 | **nested later**（Notes） |
| **Pagination** | 翻页、跳转、尺寸 | — |
| **Steps** | current、status、方向 | — |
| **Tabs** | line/card、滚动、ink、切换 | bar/body ScrollViewport |

### 4.4 数据录入

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **AutoComplete** | 过滤、键盘选 | — |
| **Cascader** | 多级、搜索 | 懒加载可后置 |
| **Checkbox** | checked/indeterminate/group | — |
| **ColorPicker** | 预设/面板 | Notes：swatches 基线 |
| **DatePicker** | 选日、清空 | **range later** |
| **Form** | label、校验、提交 | FormModel |
| **Input** | 前后缀、clear、IME、回车 | 密码掩码另开 |
| **InputNumber** | min/max/step/precision | — |
| **Mentions** | @ 触发 | 薄封装基线 |
| **Radio** | 组互斥 | — |
| **Rate** | 星级、半星 | — |
| **Select** | 单/多、搜索、清空 | 虚拟/多选标签分期 |
| **Slider** | 拖拽、step、marks | — |
| **Switch** | 动画、尺寸 | Ticker |
| **TimePicker** | 时分、格式 | 简化面板可 Notes |
| **Transfer** | 穿梭、搜索 | — |
| **TreeSelect** | 树选 | 薄封装基线 |
| **Upload** | 列表、删除 | **host dialog later**；Picker 注入 |

### 4.5 数据展示

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Avatar** | 图/字/图标 fallback | — |
| **Badge** | count、dot | — |
| **Calendar** | 月视图、选日 | — |
| **Card** | head/body/actions | — |
| **Carousel** | 切页、指示 | 复杂手势后置 |
| **Collapse** | 手风琴 | — |
| **Descriptions** | 网格 label/value | — |
| **Empty** | 空态 | — |
| **Image** | 占位、失败 | **GPU texture later** |
| **List** | 列表、空态 | 虚拟固定行高 |
| **Popover** | 标题+内容、placement | 浮层规范 |
| **QRCode** | 展示 | **codec later** |
| **Segmented** | 分段选中 | — |
| **Statistic** | 数值展示 | — |
| **Table** | 排序、选择、虚拟 body | **固定表头**（非 in-scroll sticky） |
| **Tag** | 关闭、色 | Ellipsis 已接 label |
| **Timeline** | 条目、pending | — |
| **Tooltip** | hover 延迟、placement | Trigger DelayMs |
| **Tour** | 步骤、mask、ESC | — |
| **Tree** | 展开、选择 | 半选/虚拟分期 |

### 4.6 反馈

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Alert** | 四态、可关 | — |
| **Drawer** | mask、ESC、trap | 双带合成 |
| **Message** | 队列、duration | NotifyQueue + Ticker |
| **Modal** | mask、footer、ESC、焦点 | 双带；MaskClosable |
| **Popconfirm** | 确认/取消 | — |
| **Progress** | line/环、status | — |
| **Result** | 状态页 | — |
| **Skeleton** | active | Ticker |
| **Spin** | tip、delay | Ticker；Boundary |
| **Watermark** | 铺贴 | 简化可 Notes |

### 4.7 其他

| 控件 | 需求要点 | 底座注意 |
|------|----------|----------|
| **Affix** | offset | Sticky 简化，真吸附后置 |
| **ConfigProvider** | theme 继承 | **真节点** + SetTheme 广播 |
| 复合内嵌 | 共享 Token | 禁止分叉硬编码色 |

---

## 5. 自动验收体系

### 5.1 三轨（与现网一致）

| 轨 | 内容 | 主要文件 |
|----|------|----------|
| **行为** | 事件、状态、回调、键盘、滚动、焦点 | `behavior_all_ant_test.go`、控件专项 `*_test.go` |
| **样式 / Token** | 尺寸、色、圆角、间距 | `ant_style_test.go` |
| **Golden** | 关键态部件图，本库基线 + 容差 | `golden_test.go` · `ui/visualtest` |

### 5.2 强制覆盖（按类）

| 类型 | 至少覆盖 |
|------|----------|
| 全控件 | 覆盖表一行；behavior **≥1** 显式 case（Wave A 主控件建议 ≥3） |
| Overlay | portal 稳定、mask、outside dismiss、ESC、focus trap、z-order（有则测） |
| 输入 | 输入/清空/禁用/只读/maxLength；IME 用 Headless；选区/拖选按能力 |
| 动画 | ticker 注册/注销、推进、停止后无空转 |
| 数据密度 | 虚拟固定行高、分页、排序、选择稳定 |

### 5.3 Golden 规则

1. 固定 Theme、固定测试字体、`scale=1`。  
2. 优先 chrome（可无字或固定单字）。  
3. 与 **本库** `testdata` 比，容差抗 AA；禁止默认「严格 PNG 相等」为唯一标准。  
4. 有意改观感：显式更新基线，PR 说明。  
5. **不对** ant.design 网站实时截图做 CI。

### 5.4 命令

```bash
go test ./ui/kit -run TestAntCoverageTable -v
go test ./ui/core ./ui/primitive ./ui/layer ./ui/app ./ui/platform ./ui/kit -count=1
# 观感（可选）
go run ./examples/ui_polish_gallery
```

---

## 6. 完成标准（DoD）

### 6.1 控件级

- [ ] `coverage.go` 有对应行；能力残差写 **Notes**  
- [ ] 当期 Wave 的 L1 行为测通过  
- [ ] Token/几何不破坏 `ant_style` 基线  
- [ ] 若有视觉变更：golden 更新或 polish 人眼签字  
- [ ] 只复用 token + primitive；遵守 DEV_GUIDE  

### 6.2 库级（里程碑）

- [ ] 公开控件均在覆盖表  
- [ ] Wave A 主路径 L1+L2 达标，关键态 L3 有基线  
- [ ] 统一字体与 scale 下，主路径与 Ant **气质一致**（L4）  
- [ ] 无第二套 present/事件；无新增每帧 Sync 硬依赖  

---

## 7. 每控件实现模板（复制到 PR / 注释）

```text
## kit.Xxx
- Ant: https://ant.design/components/xxx
- Wave: A | B | C
- Props: （名称 / 默认 / 后置）
- States: （适用子集）
- Primitive: （Pressable / Flex / AnchoredPopup / …）
- Notes: （与 coverage 同步）
- Tests: behavior_… / style_… / golden_…
```

---

## 8. 参考

### 8.1 仓库

- `ui/kit/behavior_all_ant_test.go`  
- `ui/kit/ant_style_test.go`  
- `ui/kit/golden_test.go`  
- `ui/kit/coverage.go`  
- `ui/core/theme.go`  
- `ui/primitive/decorated.go` · `editable.go` · `anchored.go` · `scroll.go` · `text.go`  
- `ui/layer/compositor.go`  
- `ui/app/app.go` · `present.go`  

### 8.2 Ant Design v5

- 组件总览：https://ant.design/components/overview/  
- 主题定制：https://ant.design/docs/react/customize-theme-cn/  
- Component Token：https://ant.design/docs/react/demo/component-token/  

---

## 9. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-23 | 初版：自对齐需求稿修订；去掉「官网像素哈希」；加 L1–L4、Wave、桌面差异、与 DEV_GUIDE/Foundation/coverage 挂钩 |
