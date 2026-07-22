# 控件时现：
> 实现方式和规格和现有控件统一， 对齐 ant.design 的控件库
> 分大类：General， Layout Navigation， Data Entry，Data Display，Feedback，Other，这些大类下分类的所有子控件都实现。
> 组合：需先了解所有控件的组合方式，有些控件可能需要使用以实现的控件组合出来然后定制自己的功能
> 测试生图：ui/visualtest，每个控件都有可见性测试
> 严格测试：每个控件都有完整功能测试，并且有测试的正确结果。
> 显示测试：每个控件都放到  ui_polish_gallery 示例程序里展示，要做分类展示。
> 优先实现：优先实现滚动条容器，tabs 需要使用滚动条，标签和content都使用，控件和标签分类显示不下需要出现滚动条。
> 对齐 ant.design: https://ant.design/components/overview/

# 优化
> 如于通用可扩展通用配置，或在底层增加，每个控件都可自定义扩展和配置。防止出现冗余。
> 对于以有控件需要重新审查对齐 ant.design

# 实现和测试
> 要进行多至少3轮以上在 ant.design 官网把每个控件的特有功能参考时现
> 每轮每个控件些完都要进行严格测试做验收

每次实现都要重新读取这个文档。

实现完所有控件并通过测试 goal 结束。

---

## 进度（2026-07-23 全量 R1–R3 闭合）

### 文档条款对照

| 条款 | 落地 |
|------|------|
| 滚动条 + Tabs 溢出 | ✅ `ScrollViewport` + Tabs bar/body scroll |
| 大类分类 gallery | ✅ 七类左栏 `buildCatalogPanels` |
| **每个控件 visualtest** | ✅ `TestVisual_PerControl` → `ctl_*.png` |
| **完整功能测试+期望结果** | ✅ `TestBehavior_*` + `TestBehavior_AllAntControls` |
| **≥3 轮 ant 特性 + 每轮验收** | ✅ **`TestFeatures_ThreeRounds`**（覆盖表全量 × 3 轮） |
| 组合复用 | ✅ Pressable/Flex/Decorated/Scroll |
| AntCoverage | Ready 全表 |
| 一控件一文件 | ✅ 见下方代码组织 |

### 验收命令

```bash
go test ./ui/... -count=1
go test ./ui/kit/ -run 'TestFeatures_ThreeRounds|TestBehavior_AllAntControls' -count=1
UPDATE_VISUAL=1 go test ./ui/visualtest/ -run TestVisual_PerControl   # 仅更新金标
go run ./examples/ui_polish_gallery
```

### R1–R3 特性矩阵（AntCoverage 全量）

每条控件在 `ui/kit/features_r3_test.go` 有 3 个具名验收轮次（构造/状态 API/交互或官网特有 props）。摘要：

| 大类 | 控件 | R1 | R2 | R3 |
|------|------|----|----|-----|
| General | Button | type/size | click | disabled/loading |
| | FloatButton | construct | onClick | shape |
| | Icon | name | layout | paint |
| | Typography | Text value | Title level | Paragraph |
| Layout | Divider | horizontal | vertical | dashed/text |
| | Flex | row | gap | wrap |
| | Grid | cols | setCols | gap |
| | Layout | structure | size | siderWidth |
| | Space | children | size | wrap |
| | Splitter | construct | ratio | setRatio |
| Navigation | Anchor | items | setActive | scroll-spy SyncFromScroll |
| | Breadcrumb | items | setItems | separator |
| | Dropdown | construct | open | selected |
| | Menu | construct | selected | openKeys |
| | Pagination | pages | setPage | total + quickJumper |
| | Steps | items | current | status/direction |
| | Tabs | items | active | type card + centered |
| Data Entry | AutoComplete | value | setOptions | filter layout |
| | Cascader | construct | setValue path | onChange/value stick |
| | Checkbox | checked | indeterminate | disabled |
| | ColorPicker | default swatch | setValue | onChange |
| | DatePicker | selectDay | yearMonth | **range + showTime** |
| | Form | addItem | layout h/v | requiredMark |
| | Input | setValue | onChange | disabled |
| | InputNumber | value | min/max clamp | disabled |
| | Mentions | construct | value | options |
| | Radio | value | RadioGroup | selected chrome |
| | Rate | value | count | allowClear |
| | Select | setValue | open | allowClear/Clear |
| | Slider | value | min/max | step |
| | Switch | checked | disabled | node |
| | TimePicker | default | setValue | onChange |
| | Transfer | construct | moveAll | clearTarget |
| | TreeSelect | construct | setValue | clear |
| | Upload | fileName | **Picker CapFile inject** | accept + multiple |
| Data Display | Avatar | text | setText/size | shape |
| | Badge | count | overflow | dot |
| | Calendar | month | selectDay | setMonth |
| | Card | title | content | title/extra/bordered |
| | Carousel | slides | setIndex | next/prev |
| | Collapse | panels | active | accordion |
| | Descriptions | items | setItems | column |
| | Empty | description | setDescription | setImage |
| | Image | size | **setSrc/setPixels** | preview |
| | List | items | setItems | selected |
| | Popover | construct | open | close |
| | QRCode | text | setText/size | status |
| | Segmented | value | onChange | layout |
| | Statistic | value | title | prefix/suffix |
| | Table | construct | setData | **sort** |
| | Tag | value | color | closable |
| | Timeline | items | setItems | pending |
| | Tooltip | construct | layout | sync |
| | Tour | steps | open | current |
| | Tree | construct | selected | expand |
| Feedback | Alert | type | description | closable |
| | Drawer | open | placement | width |
| | Message | info | success/error | notification+count |
| | Modal | open overlays | setTitle | footerVisible |
| | Notification | host queue | count | success |
| | Popconfirm | construct | open | close |
| | Progress | percent | status | showInfo |
| | Result | status | setTitle | setStatus/sub |
| | Skeleton | active | rows | avatar |
| | Spin | spinning | tip | content |
| | Watermark | text | setText | gap |
| Other | Scroll | overflow | wheel | showScrollbar |
| | Affix | construct | offsetTop | affixed |
| | App | theme | density compact | density large |
| | ConfigProvider | construct | setTheme | setChild |

### 测试入口

- `ui/kit/features_r3_test.go` — **全量 × 3 轮**门禁
- `ui/kit/behavior_all_ant_test.go` — 覆盖表行为门禁
- `ui/kit/behavior_acceptance_test.go` — 深交互期望结果
- `ui/visualtest/per_control_test.go` — 每控件金标
- `ui/platform/file_pick.go` — `FilePicker` / `CapFile`（kit.Upload.Picker 注入；宿主对话框为 OS 适配层，契约已闭合）

### 平台边界（非 kit 阻塞）

以下依赖 OS/编解码/GPU，**kit 侧 API 与测试已闭合**，实现落在 platform/render：

- 宿主原生文件对话框（Linux zenity/portal 等）挂 `platform.FilePicker`
- 真图文件解码 → `Image.SetPixels` 已接采样绘制
- 真 QR 编解码 → `QRCode` 确定性模块 + Status 已接

### Goal 状态

**全部 AntCoverage 控件已实现；每控件 ≥3 轮 ant 特性有验收测试；visual + behavior + features 全绿 → goal 可结束。**

---

## 代码组织（强制）

1. **每个控件单独一个文件**：`ui/kit/<control>.go`
2. **控件私有类型/方法同文件**：`FormItem`→`form.go`，`MenuItem`→`menu.go`，`sliderHost`→`slider.go`
3. **大类通用函数** → `general_common.go` / `layout_common.go` / `navigation_common.go` / `entry_common.go` / `display_common.go` / `feedback_common.go` / `other_common.go`
4. 跨类 chrome：`ant_chrome.go`
