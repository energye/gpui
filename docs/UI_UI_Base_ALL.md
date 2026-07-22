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

# 代码组织
1. 每个控件单独一个文件
2. 通用函数放到大分类的通用文件里

每次实现都要重新读取这个文档。

实现完所有控件并通过测试 goal 结束。

---

## 进度（2026-07-23 R3 行为矩阵 + CapFile/Scroll-spy）

### 文档条款对照

| 条款 | 落地 |
|------|------|
| 滚动条 + Tabs 溢出 | ✅ `ScrollViewport` + Tabs bar/body scroll |
| 大类分类 gallery | ✅ 七类左栏 `buildCatalogPanels` |
| **每个控件 visualtest** | ✅ `TestVisual_PerControl` → `ctl_*.png` 一控件一基线 |
| **完整功能测试+期望结果** | ✅ `TestBehavior_*` + **`TestBehavior_AllAntControls`（覆盖表全量）** |
| 组合复用 | ✅ Pressable/Flex/Decorated/Scroll 组合 |
| AntCoverage | Ready 全表，Later/Primitive/Partial=0 |

### 验收命令

```bash
go test ./ui/... -count=1
UPDATE_VISUAL=1 go test ./ui/visualtest/ -run TestVisual_PerControl   # 仅更新金标
go run ./examples/ui_polish_gallery
```

### 本轮（R3）加深的 ant 特性

| 控件 | R1 | R2 | R3 |
|------|----|----|-----|
| Upload | 按钮+FileName | **Picker 注入（CapFile 契约）+ 取消保留** | 宿主 Linux 真对话框（可选） |
| Anchor | 链接列表 | **ScrollTarget + SectionOffsets + SyncFromScroll** | 连续 scroll 事件绑定位（gallery） |
| DatePicker | Input+Calendar | **SelectDay / Value / OnChange** | range / showTime（later） |
| Image | 占位渐变 | **SetSrc + SetPixels 采样绘制** | GPU 纹理 / 解码器 |
| QRCode | 伪模块 | **SetText/SetSize + finder/timing 图案** | 真 QR 编解码 |
| Calendar | 月网格 | 选中日高亮 + 月导航 | 全量 locale |

### 测试入口

- `ui/visualtest/per_control_test.go` — 每控件 `ctl_<name>`
- `ui/kit/behavior_acceptance_test.go` — 深交互期望结果（含 CapFile/DatePicker/Anchor）
- `ui/kit/behavior_all_ant_test.go` — **AntCoverage 全量行为门禁**
- `ui/kit/catalog_coverage_test.go` — 覆盖表 + 构造器布局
- `ui/platform/file_pick.go` — `FilePicker` / `CapFile`

### 能力分层（诚实）

- **深交互 + 期望结果**：Button/Input/Checkbox/Switch/Tabs/Scroll/Modal/Select/Slider/Rate/Upload(Picker)/DatePicker(SelectDay)/Anchor(scroll-spy)…
- **基线 Ready**：Catalog 其余控件均有状态 API、覆盖表条目、全量行为子测试、独立视觉金标
- **仍浅 / 平台后续**：宿主真文件对话框、真图解码+GPU 纹理、真 QR 编解码、Form/Table/Cascader 官网高级 props、Date range

### Goal 状态

文档要求「所有控件实现 + 测试通过」：构造/覆盖/全量行为/视觉金标已绿；**≥3 轮 ant 官网特性**在深交互子集已闭合 R1–R3 骨架，平台级编解码与 host dialog 为显式 later，不阻塞 Ready 表。


---

## 代码组织（强制）

1. **每个控件单独一个文件**：`ui/kit/<control>.go`（如 `button.go`、`modal.go`、`date_picker.go`）。
2. **控件私有类型/方法同文件**：如 `FormItem`→`form.go`，`MenuItem`→`menu.go`，`sliderHost`→`slider.go`。
3. **大类通用函数**放到分类公共文件：
   - `general_common.go` — General
   - `layout_common.go` — Layout
   - `navigation_common.go` — Navigation
   - `entry_common.go` — Data Entry（`formatNum`/`containsFold`/`daysInMonth`/…）
   - `display_common.go` — Data Display（`alertIcon`/`alertColor`/…）
   - `feedback_common.go` — Feedback
   - `other_common.go` — Other / App（Density、`ApplyDensity`）
4. 跨类 chrome token：`ant_chrome.go`。已删除混合袋文件：`catalog_rest.go`、`display_extra.go`、`entry_extra.go`、`feedback_extra.go`、`layout_extra.go`、`feedback.go`（内容已拆入单控件文件）。
