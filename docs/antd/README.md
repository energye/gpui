# Ant Design 6.5.x → gpui 控件规格文档

> 依据 [Ant Design 6.5.x](https://ant.design/components/overview) 官方文档整理。  
> **每个控件一份 Markdown**，细度对齐 `button.md`（外观 / 功能 / 完整 API / 实现要点）。  
> 用于产品控件的功能规格与验收清单。  
> **渲染架构**见上级 [`ENGINE_FLUTTER_SKIA_ARCH.md`](../ENGINE_FLUTTER_SKIA_ARCH.md)（控件实现后置 P7，禁止绕过 Layer/直触 GPU）。

## 文档结构（与 Button 同级）

每个控件文档统一包含：

1. **控件外观**
   - 基础形态
   - 官方示例对应的外观形态表
   - 外观相关配置逐项说明（含枚举值与视觉语义）
   - 交互视觉状态检查表（default/hover/focus/disabled/loading…）
   - 语义化 DOM 与 Design Token
2. **功能**
   - 使用场景（When To Use）
   - 按官方示例拆解的核心功能清单
   - 行为 API 能力表
   - 示例全表（含 debug 标记）
   - 实例方法 / Ref、FAQ（若有）
   - 与其它控件组合关系
3. **配置（API）**
   - 官方 API 全文表格
   - 导入方式
   - 配置项速查表（从 API 解析）
4. **gpui kit 实现要点**（验收清单）
5. **参考链接**

## 版本

- 目标版本：**antd v6.5.1**
- 源文档：`components/*/index.zh-CN.md`（官方仓库）

## 控件索引

### 通用 General

| 控件 | 文档 |
| --- | --- |
| Button | [button.md](./button.md) |
| FloatButton | [float-button.md](./float-button.md) |
| Icon | [icon.md](./icon.md) |
| Typography | [typography.md](./typography.md) |

### 布局 Layout

| 控件 | 文档 |
| --- | --- |
| Divider | [divider.md](./divider.md) |
| Flex | [flex.md](./flex.md) |
| Grid | [grid.md](./grid.md) |
| Layout | [layout.md](./layout.md) |
| Masonry | [masonry.md](./masonry.md) |
| Space | [space.md](./space.md) |
| Splitter | [splitter.md](./splitter.md) |

### 导航 Navigation

| 控件 | 文档 |
| --- | --- |
| Anchor | [anchor.md](./anchor.md) |
| Breadcrumb | [breadcrumb.md](./breadcrumb.md) |
| Dropdown | [dropdown.md](./dropdown.md) |
| Menu | [menu.md](./menu.md) |
| Pagination | [pagination.md](./pagination.md) |
| Steps | [steps.md](./steps.md) |
| Tabs | [tabs.md](./tabs.md) |

### 数据录入 Data Entry

| 控件 | 文档 |
| --- | --- |
| AutoComplete | [auto-complete.md](./auto-complete.md) |
| Cascader | [cascader.md](./cascader.md) |
| Checkbox | [checkbox.md](./checkbox.md) |
| ColorPicker | [color-picker.md](./color-picker.md) |
| DatePicker | [date-picker.md](./date-picker.md) |
| Form | [form.md](./form.md) |
| Input | [input.md](./input.md) |
| InputNumber | [input-number.md](./input-number.md) |
| Mentions | [mentions.md](./mentions.md) |
| Radio | [radio.md](./radio.md) |
| Rate | [rate.md](./rate.md) |
| Select | [select.md](./select.md) |
| Slider | [slider.md](./slider.md) |
| Switch | [switch.md](./switch.md) |
| TimePicker | [time-picker.md](./time-picker.md) |
| Transfer | [transfer.md](./transfer.md) |
| TreeSelect | [tree-select.md](./tree-select.md) |
| Upload | [upload.md](./upload.md) |

### 数据展示 Data Display

| 控件 | 文档 |
| --- | --- |
| Avatar | [avatar.md](./avatar.md) |
| Badge | [badge.md](./badge.md) |
| Calendar | [calendar.md](./calendar.md) |
| Card | [card.md](./card.md) |
| Carousel | [carousel.md](./carousel.md) |
| Collapse | [collapse.md](./collapse.md) |
| Descriptions | [descriptions.md](./descriptions.md) |
| Empty | [empty.md](./empty.md) |
| Image | [image.md](./image.md) |
| List | [list.md](./list.md) |
| Popover | [popover.md](./popover.md) |
| QRCode | [qr-code.md](./qr-code.md) |
| Segmented | [segmented.md](./segmented.md) |
| Statistic | [statistic.md](./statistic.md) |
| Table | [table.md](./table.md) |
| Tag | [tag.md](./tag.md) |
| Timeline | [timeline.md](./timeline.md) |
| Tooltip | [tooltip.md](./tooltip.md) |
| Tour | [tour.md](./tour.md) |
| Tree | [tree.md](./tree.md) |

### 反馈 Feedback

| 控件 | 文档 |
| --- | --- |
| Alert | [alert.md](./alert.md) |
| Drawer | [drawer.md](./drawer.md) |
| Message | [message.md](./message.md) |
| Modal | [modal.md](./modal.md) |
| Notification | [notification.md](./notification.md) |
| Popconfirm | [popconfirm.md](./popconfirm.md) |
| Progress | [progress.md](./progress.md) |
| Result | [result.md](./result.md) |
| Skeleton | [skeleton.md](./skeleton.md) |
| Spin | [spin.md](./spin.md) |
| Watermark | [watermark.md](./watermark.md) |

### 其他 Other

| 控件 | 文档 |
| --- | --- |
| Affix | [affix.md](./affix.md) |
| App | [app.md](./app.md) |
| BorderBeam | [border-beam.md](./border-beam.md) |
| ConfigProvider | [config-provider.md](./config-provider.md) |
| Util | [util.md](./util.md) |

## 总体进度看板（新会话先看这张表领活）

> 状态一共五种：**未开工** → **首批中** → **首批完**（新 DoD 六条全勾：P0+P1 全合 + showcase 大图 + 官网并排）→ **全完**（官方非 debug 全覆盖）；另有 **重写中**（推翻重写专用，旧状态作废待重做）。
> ⚠ 2026-09-14 起 W0 文档与 W1 代码全推翻重写：本表一切旧“首批完”作废，已统一翻成“重写中”，待办见各行备注。`ui/kit/coverage.go` 还没跟上，在它同步之前以本表为准。
> 每次完工更新对应行；`ui/kit/coverage.go` 同步后两处保持一致。

### W0 地基（先行，阻塞一切，重写中）

| 项 | 状态 | 备注 |
| --- | --- | --- |
| 套件架子 `ui/kit`（一控件一目录 + doc.go） | 重写中 | 旧首批完作废；待按新 DoD 重验目录规矩与 §6.10 签名落点 |
| 示例总窗（左标签右展示 + 临时导航） | 重写中 | 旧首批完作废；待重验独立打开/独立测/独立截图与事件主干接线 |
| 覆盖记录 `ui/kit/coverage.go` | 重写中 | 旧首批完作废；待把本表“重写中”同步过去，同步前以本表为准 |
| 主题全局种子（对齐 antd v6.5.1） | 重写中 | 旧首批完作废；悬停/按压/内边距等 Token 还没跟官网种子对过数，对完才算落定 |
| 浮层通用定位（十二方向/翻转/箭头/外点关/焦点锁） | 重写中 | 旧首批完作废；待按 antd 定位语义逐项重验，只调包不自写一套 |
| Icon（最底层，先做） | 重写中 | 旧 P0 全合作废；待 W0 文档定稿后按新 DoD 重做 |

### W1 基础件（19 个，先按钮标杆签字，再 18 个并行）

> 本节 19 行旧“首批完”全部作废，现统一为重写中；备注里的旧 P0 用例号只备查，不代表现状。开工顺序：W0 文档定稿 → 门禁修好（见下使用建议第 5 条）→ **按钮先单做、官网并排签字（标杆）** → 剩下 18 个并行；不等标杆签字就铺开必返工。

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| Alert [alert.md](./alert.md) | 重写中 | `ui/kit/alert` P0全合（ALT-01…19 + 画廊`alert`页） |
| BorderBeam [border-beam.md](./border-beam.md) | 重写中 | `ui/kit/border-beam` P0全合（BB-01…15 + 画廊`border-beam`页，自有扩展） |
| Button [button.md](./button.md) | 重写中 | `ui/kit/button` P0全合（BTN-01…20/23/24 + 画廊`button`页） |
| Divider [divider.md](./divider.md) | 重写中 | `ui/kit/divider` P0全合（DIV-01…23 + 画廊`divider`页） |
| Flex [flex.md](./flex.md) | 重写中 | `ui/kit/flex` P0全合（FLX-01…18 + 画廊`flex`页） |
| FloatButton [float-button.md](./float-button.md) | 重写中 | `ui/kit/float-button` P0全合（FB-01…26 + 画廊`float-button`页） |
| Grid [grid.md](./grid.md) | 重写中 | `ui/kit/grid` P0全合（GRD-01…19 + 画廊`grid`页） |
| Layout [layout.md](./layout.md) | 重写中 | `ui/kit/layout` P0全合（LAY-01…21 + 画廊`layout`页） |
| Masonry [masonry.md](./masonry.md) | 重写中 | `ui/kit/masonry` P0全合（MAS-01…16 + 画廊`masonry`页） |
| Progress [progress.md](./progress.md) | 重写中 | `ui/kit/progress` P0全合（PRG-01…22 + 画廊`progress`页） |
| Skeleton [skeleton.md](./skeleton.md) | 重写中 | `ui/kit/skeleton` P0全合（SKL-01…18 + 画廊`skeleton`页，先做已合） |
| Space [space.md](./space.md) | 重写中 | `ui/kit/space` P0全合（SPC-01…21 + 画廊`space`页） |
| Spin [spin.md](./spin.md) | 重写中 | `ui/kit/spin` P0全合（SPN-01…18 + 画廊`spin`页，先做已合） |
| Splitter [splitter.md](./splitter.md) | 重写中 | `ui/kit/splitter` P0全合（SPL-01…21 + 画廊`splitter`页） |
| Statistic [statistic.md](./statistic.md) | 重写中 | `ui/kit/statistic` P0全合（STA-01…16 + 画廊`statistic`页） |
| Tag [tag.md](./tag.md) | 重写中 | `ui/kit/tag` P0全合（TAG-01…19 + 画廊`tag`页） |
| Timeline [timeline.md](./timeline.md) | 重写中 | `ui/kit/timeline` P0全合（TL-01…17 + 画廊`timeline`页） |
| Typography [typography.md](./typography.md) | 重写中 | `ui/kit/typography` P0全合（TYP-01…25 + 画廊`typography`页） |
| Watermark [watermark.md](./watermark.md) | 重写中 | `ui/kit/watermark` P0全合（WM-01…14 + 画廊`watermark`页） |

### W2 组合件（20 个并行）

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| Affix [affix.md](./affix.md) | 未开工 | — |
| Avatar [avatar.md](./avatar.md) | 未开工 | — |
| Badge [badge.md](./badge.md) | 未开工 | — |
| Card [card.md](./card.md) | 未开工 | — |
| Carousel [carousel.md](./carousel.md) | 未开工 | — |
| Collapse [collapse.md](./collapse.md) | 未开工 | — |
| Descriptions [descriptions.md](./descriptions.md) | 未开工 | — |
| Empty [empty.md](./empty.md) | 未开工 | — |
| Menu [menu.md](./menu.md) | 未开工 | 被复用底座 |
| Tooltip [tooltip.md](./tooltip.md) | 重写中 | 旧 P0 全合（TIP-01…21）作废；站在按钮和画廊上，W1 落定后连带回归 |
| Popover [popover.md](./popover.md) | 重写中 | 旧 P0 全合（POP-01…19）作废；同上，连带回归 |
| QRCode [qr-code.md](./qr-code.md) | 未开工 | — |
| Radio [radio.md](./radio.md) | 未开工 | — |
| Rate [rate.md](./rate.md) | 未开工 | — |
| Result [result.md](./result.md) | 未开工 | — |
| Segmented [segmented.md](./segmented.md) | 未开工 | — |
| Slider [slider.md](./slider.md) | 未开工 | — |
| Steps [steps.md](./steps.md) | 未开工 | — |
| Switch [switch.md](./switch.md) | 未开工 | — |
| Tabs [tabs.md](./tabs.md) | 未开工 | 做好后回替总窗导航 |

### W3 浮层表单件（18 个并行）

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| Input [input.md](./input.md) | 未开工 | 先做（被复用） |
| Select [select.md](./select.md) | 未开工 | 先做（被复用） |
| Tree [tree.md](./tree.md) | 未开工 | 非勾选先行 |
| TimePicker [time-picker.md](./time-picker.md) | 未开工 | — |
| AutoComplete [auto-complete.md](./auto-complete.md) | 未开工 | 等 Input |
| Mentions [mentions.md](./mentions.md) | 未开工 | 等 Input |
| InputNumber [input-number.md](./input-number.md) | 未开工 | 等 Input |
| ColorPicker [color-picker.md](./color-picker.md) | 未开工 | — |
| Image [image.md](./image.md) | 未开工 | 含预览浮层 |
| Message [message.md](./message.md) | 未开工 | — |
| Notification [notification.md](./notification.md) | 未开工 | — |
| Tour [tour.md](./tour.md) | 未开工 | — |
| Anchor [anchor.md](./anchor.md) | 未开工 | — |
| Dropdown [dropdown.md](./dropdown.md) | 未开工 | — |
| Popconfirm [popconfirm.md](./popconfirm.md) | 未开工 | — |
| Upload [upload.md](./upload.md) | 未开工 | — |
| Drawer 本体 [drawer.md](./drawer.md) | 未开工 | 表单联调延后 W4 |
| Modal 本体 [modal.md](./modal.md) | 未开工 | 表单联调延后 W4 |

### W4 抬升件 + 聚合（11 个）

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| Checkbox [checkbox.md](./checkbox.md) | 未开工 | 先做（表格/穿梭等它） |
| Pagination [pagination.md](./pagination.md) | 未开工 | 先做（表格/列表等它） |
| Calendar 完整头 [calendar.md](./calendar.md) | 未开工 | 降级可先行 |
| Breadcrumb 完整分支 [breadcrumb.md](./breadcrumb.md) | 未开工 | 降级可先行 |
| List [list.md](./list.md) | 未开工 | — |
| Table [table.md](./table.md) | 未开工 | 等 Checkbox/Pagination |
| Transfer [transfer.md](./transfer.md) | 未开工 | 等 Table/List |
| Cascader [cascader.md](./cascader.md) | 未开工 | 等 Select |
| DatePicker [date-picker.md](./date-picker.md) | 未开工 | 等 Input/TimePicker |
| TreeSelect [tree-select.md](./tree-select.md) | 未开工 | 等 Tree + Select |
| Form [form.md](./form.md) | 未开工 | 最后（等 DatePicker） |

### W5 全局（最后，2 个并行）

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| App [app.md](./app.md) | 未开工 | 等消息/弹窗/通知 + 按钮/输入 |
| ConfigProvider [config-provider.md](./config-provider.md) | 未开工 | 等消息/弹窗/通知 + 按钮/输入 |

### 随时可并行

| 控件 | 状态 | 备注 |
| --- | --- | --- |
| Util [util.md](./util.md) | 未开工 | 无 UI，不进 kit |

## 再生生成

- **antd 源码（权威）**：`/home/yanghy/app/projects/ant-design/components/*`
- §1–§5 历史脚本：[`_generate_deep_docs.py`](./_generate_deep_docs.py)（只作骨架参考，重写期间不许整批跑）
- **§6**：**逐个手写**（按钮标杆先签字，签字后再铺开）；`_deepen_sec6.py` 与 `_write_button_depth_sec6.py --force` 已封存，重写期间**不得**再跑整批覆盖，谁跑谁负责找回

手写下一控件流程：读 `index.zh-CN.md` + `style/*.ts` + 组件 TS → 按标杆签字稿的 §6 结构写满 6.1–6.12 → 只替换该文件 `## 6.` 起至文末。

## 使用建议（开发 kit）

1. 实现前读对应 `docs/antd/<name>.md` 的 **§6**（DoD）；§1–§3 为 antd 能力全集参考。  
2. 以 **§6.8 P0+P1** 定实现范围（一次做完，不分期）；**§6.10** 定 Go API；**§6.4 / §6.9** 写自动测试。  
3. 以 **§6.12** 做完成勾选（含独立真窗 + gallery，见下）。  
4. `coverage.go` Notes：P0+P1 已对齐该 md §6；真做不了的单测 Skip 写清平台原因。
5. 门禁先修再开 W1 代码：`ui/kit/acceptance_test.go` 现在还跳过 P1 只查 P0，跟新 DoD 打架，开工前必须改成 P0+P1 同查（只认 N/A 与写清原因的 Skip），修完三处回归全绿才算数。

### 完成定义（全库统一）

控件宣布 **1:1 完成** 须同时满足：

| # | 要求 | 文档锚点 |
| --- | --- | --- |
| 1 | §6.8 **P0+P1** 已实现（官方非 debug 一个不少） | 各控件 §6.8 |
| 2 | §6.9 中 **P0+P1 / L1 / L2** 自动测试通过 | 各控件 §6.9 |
| 3 | L2 Token/度量断言（适用者） | §6.2 |
| 4 | L3 showcase 大图 + 官网并排验收通过（控件可见时） | §6.9 L3 |
| 5 | **独立真窗** `examples/kit/<名>/` 已建，`-auto-only` 按该组件特性流程逐功能验效果全绿 | ACCEPTANCE 第七节第三道门禁 |
| 6 | **示例程序** 已增加/更新 | 见下节 **ui_polish_gallery** |
| 7 | `ui/kit/coverage.go` Notes 已更新 | P0+P1 对齐路径 + Skip 原因 |
| 8 | **人测完才关**：人在真窗按该组件特性流程亲手测完每实例每功能并看效果，§6.9 L4 留验收记录 | ACCEPTANCE 第八节硬规则 |

### 示例程序：`examples/ui_polish_gallery`（强制）

**权威约定（全局只此一处详述）：**

- **路径：** 仓库根目录 [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)（W0 地基新建；落定前 §6.12 第 5/6 条视为悬空）
- **形态：** 单窗口总预览：左标签（组件列表）、右展示（该组件页）。**何时：** 实现某控件 **§6.8 P0+P1** 后、勾选 §6.12 完成前，必须在 gallery **增加或更新**该控件标签页。
- **覆盖范围（一次做完）：** 每个标签页与官方示例库对齐——官方**非 debug** 示例在数量/功能/展示效果/使用方式上一个不少（debug 仅参考，不进数）。用法功能相同、写法按 Go 习惯；浏览器才有的能力（真文件下载、剪贴板等）走宿主映射，真做不了的单测 Skip 写清平台原因，效果相同。
- **P1：** 必须进 gallery，与 P0 一次做完；真做不了的须在 `coverage.go` Notes 写清 Skip 原因。
- **组织：** 一控件一标签；能力用 section 分块（参考 Button 页：Type / Size / Icon / …）。
- **独立可验：** 每个标签须可独立打开、独立测、独立截图；总窗只作平时预览，正式验收以单标签独立断言为准（组合窗不代替单能力）。
- **独立真窗（硬）：** 每组件另有独立目录 `examples/kit/<名>/`（一组件一窗，只摆自己），`go run ./examples/kit/<名>` 打开，`-auto-only` 按该组件自己的特性流程逐功能验效果全绿（一项一效果，挂一项整窗挂）；机器绿只是放行，关闭必须人按同一流程亲手测完看完效果（见 ACCEPTANCE 第八节）。
- **豁免：** 无运行时 UI 的条目（如 **Util** 类型工具）可跳过 gallery，在 Notes 写「无 UI」。  
- **细节实现：** 见 `examples/ui_polish_gallery/catalog.go` 与 [`ui/kit/doc.go`](../../ui/kit/doc.go)（Demos 规则）。

各控件文档 **§6.12** 仅保留短锚点，指向本节，避免 72 份重复长文。

### 官网截图基准（W0 重写落点，全局只此一处详述）

- **来源：** `https://ant.design/components/<名>`，版本锁 6.5.1，只认这一处，不认 Gio。
- **存哪：** `ui/kit/<名>/testdata/official_<名>.png`，跟 showcase 大图放同一目录；截了多大窗口、哪几个示例，一并记入该组件文档 §6.1。
- **谁签字：** 建/大改基线时人眼并排验完，把截图与结论记入该组件测试注释，§6.9 的 L4 行留验收记录；没这条不算完。

## 1:1 产品规格（§6）

**写法约定：** 全部控件 §6 对齐 [Button §6](./button.md) 模板细度（6.1–6.12：度量 Token、状态机规则 ID、chrome、a11y、平台边界、P0/P1、可测用例、Go API、结构、DoD）。  
**依据源码：** `/home/yanghy/app/projects/ant-design/components/<name>/`。  
**再生（重写期间封存，不许跑）：**

上面两条历史脚本重写期间一律不许整批跑，只许单看参考；解封要等 W0 定稿后另行确认。

| 状态 | 说明 |
| --- | --- |
| **样板** | 按钮标杆签字稿（以签字版为准，不自动覆盖） |
| **全库** | 旧 72 份 §6 已作废，待按标杆逐个重写（约 200–260 行/控件；无「见上文」filler） |
| **实现时** | 以该控件 §6 为 DoD；复杂控件可再对照 style/*.ts 补业务专用数字 |

## 说明

- **List** 官方倾向废弃，仍保留完整规格便于兼容。  
- **Icon** 依赖 `@ant-design/icons@6.x`。  
- Table / Form / DatePicker 等 API 极长，文档含官方全文表 + 解析速查。  
- §1–§3 = antd 能力全集；**§6 = gpui 可交付的 1:1 产品需求**（无裁剪；平台做不了的单测 Skip 写清原因）。  
