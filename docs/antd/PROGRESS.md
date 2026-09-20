# 组件库实现进度台账（唯一状态真源）

> 地位：本表是“干到哪了”的唯一答案。`ARCHITECTURE.md` §5 的波次和 `F0_FOUNDATION_PLAN.md` §7–§8 只写“要干什么”，“干完没”只认本表。
> 更新纪律：代码绿了表没绿 = 没干完。每一波/每个组件/每个大件完成时，必须在同一批里把本表对应行的状态、证据、验证 commit 一起更新，不许只改代码不改表。
> 当前全表 ⬜：旧代码已清（`c79a141`），F0-1 尚未开工。

## 状态图例

| 符号 | 意思 | 怎么才算 |
| --- | --- | --- |
| ⬜未开 | 没动手 | 源码证据不存在 |
| 🟡进行中 | 正在写 | 部分证据有了，单测或真窗还没全绿 |
| 🟩已绿 | 单测+真窗+Golden 全绿 | 三证据齐，待签字 |
| ✅已签字 | 人工看过签字 | 签字记录进本表备注 |

## 源码对照方法（新会话先跑这三条，不猜）

```text
1. 地基落了没：git ls-files ui/kit/internal ui/theme examples/kit_f0_* | sort
2. 某组件落了没：git ls-files ui/kit/<前缀>_* examples/kit/<名> | sort
3. 单测真绿没：go test ./ui/kit/ -run 'Test<组件>' -count=1（逐文件跑，禁止一次全量）
```

状态 ✅ 的行，其“验证 commit”列必须能 `git show <commit> --stat` 看到源码+单测+真窗+本表四样，缺一样就是假绿。

## 1. F0 地基（前一波不绿后一波不开）

| 步骤 | 源码证据（必须存在） | 单测证据 | 真窗证据 | 状态 | 验证 commit | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| F0-1 主题与范围 | `ui/theme/seed*.go` + `ui/kit/internal/scope/{ctx,states,resolve}.go` | `seed_ant_test.go`、`scope` 各 `_test.go` 全绿 | `examples/kit_f0_scope/`（三套主题一切换全局生效） | ⬜未开 | — | 种子与 antd 6.5.1 逐项对，差一处即挂 |
| F0-2 布局+装饰 | `ui/kit/internal/prim/{layout,decor}.go` | `layout_test.go` 约束矩阵、`decor_test.go` 混合公式 | `examples/kit_f0_layout/`、`kit_f0_decor/`（必做调大小回基线） | ⬜未开 | — | 先决 F0-1 |
| F0-3 内容 | `ui/kit/internal/prim/content.go` | `content_test.go`（真字区域级断言） | `examples/kit_f0_content/`（图片三态只脏一格） | ⬜未开 | — | 先决 F0-2 |
| F0-4 交互+触发 | `ui/kit/internal/behavior/{interactive,field,overlay_trigger}.go` | 三个 `_test.go`（五态/受控/外点关） | `examples/kit_f0_interact/`（焦点环只键盘亮） | ⬜未开 | — | 先决 F0-3 |
| F0-5 动效语义性能方向+门禁 | `behavior/motion.go` + 语义/性能/方向门面 + 中央门禁改查四件 | 各 `_test.go` + 门禁自检 | `examples/kit_f0_motion/`（转圈真转+省动效全停） | ⬜未开 | — | 先决 F0-4 |
| F0-6 六窗全绿签字 | 六窗 `main.go` + 三套主题 Golden | — | 六窗 `-auto-only` 全绿 + 人工签字 | ⬜未开 | — | 先决 F0-5；签字不过 F1 不开 |

## 2. F1–F6 组件（每行一组件，源码对不上即假绿）

证据模板（每行通用）：源码 `ui/kit/<前缀>_*.go`（五段齐）→ 单测 `ui/kit/<前缀>_*_test.go` + `testdata/` → 真窗 `examples/kit/<名>/`（基线 1200×800 可调+指标 A–J+三证据）→ Golden `testdata/showcase*`。

| 波 | 组件 | 源码前缀 | 状态 | 验证 commit | 备注 |
| --- | --- | --- | --- | --- | --- |
| F1 | Icon | `icon_` | ⬜未开 | — | 最先落地，全库引用 |
| F1 | Typography | `typography_` | ⬜未开 | — | 剪贴板宿主后补 |
| F1 | Divider | `divider_` | ⬜未开 | — | — |
| F1 | Tag | `tag_` | ⬜未开 | — | — |
| F1 | Badge | `badge_` | ⬜未开 | — | — |
| F1 | Avatar | `avatar_` | ⬜未开 | — | 组popover走D类 |
| F1 | Card | `card_` | ⬜未开 | — | — |
| F1 | Empty | `empty_` | ⬜未开 | — | — |
| F1 | Result | `result_` | ⬜未开 | — | — |
| F1 | Statistic | `statistic_` | ⬜未开 | — | 倒计时走 Motion 计时 |
| F1 | Descriptions | `descriptions_` | ⬜未开 | — | — |
| F1 | Timeline | `timeline_` | ⬜未开 | — | — |
| F1 | Image（展示态） | `image_` | ⬜未开 | — | 预览浮层走D类 |
| F1 | Alert | `alert_` | ⬜未开 | — | — |
| F2 | Button（标杆） | `button_` | ⬜未开 | — | 五样签字缺一不开后续 |
| F2 | Switch | `switch_` | ⬜未开 | — | — |
| F2 | Checkbox | `checkbox_` | ⬜未开 | — | — |
| F2 | Radio | `radio_` | ⬜未开 | — | — |
| F2 | Rate | `rate_` | ⬜未开 | — | 半区命中 L2 加 |
| F2 | Slider | `slider_` | ⬜未开 | — | 拖动吸附六项 |
| F2 | Segmented | `segmented_` | ⬜未开 | — | — |
| F2 | FloatButton | `float_button_` | ⬜未开 | — | — |
| F3 | Input | `input_` | ⬜未开 | — | OTP 多框联动 |
| F3 | InputNumber | `input_number_` | ⬜未开 | — | — |
| F3 | Select | `select_` | ⬜未开 | — | 虚拟行高耦合+门禁 |
| F3 | AutoComplete | `auto_complete_` | ⬜未开 | — | — |
| F3 | Mentions | `mentions_` | ⬜未开 | — | — |
| F3 | Cascader | `cascader_` | ⬜未开 | — | 懒加载异步 |
| F3 | TreeSelect | `tree_select_` | ⬜未开 | — | 依赖树 |
| F3 | DatePicker | `date_picker_` | ⬜未开 | — | 依赖 G4 |
| F3 | TimePicker | `time_picker_` | ⬜未开 | — | 依赖 G4 |
| F3 | ColorPicker | `color_picker_` | ⬜未开 | — | 依赖 G5 |
| F3 | Upload | `upload_` | ⬜未开 | — | 依赖 G6 |
| F4 | Tooltip | `tooltip_` | ⬜未开 | — | 定位底座先行 |
| F4 | Popover | `popover_` | ⬜未开 | — | — |
| F4 | Popconfirm | `popconfirm_` | ⬜未开 | — | — |
| F4 | Dropdown | `dropdown_` | ⬜未开 | — | 多级延后 |
| F4 | Menu（弹出态） | `menu_` | ⬜未开 | — | — |
| F4 | Modal | `modal_` | ⬜未开 | — | 依赖 G1 |
| F4 | Drawer | `drawer_` | ⬜未开 | — | 依赖 G1 |
| F4 | Message | `message_` | ⬜未开 | — | 依赖 G2 |
| F4 | Notification | `notification_` | ⬜未开 | — | 依赖 G2 |
| F4 | Tour | `tour_` | ⬜未开 | — | 依赖 G3 |
| F5 | Layout | `layout_` | ⬜未开 | — | — |
| F5 | Grid | `grid_` | ⬜未开 | — | — |
| F5 | Flex | `flex_` | ⬜未开 | — | — |
| F5 | Space | `space_` | ⬜未开 | — | — |
| F5 | Splitter | `splitter_` | ⬜未开 | — | 拖拽分栏 |
| F5 | Affix | `affix_` | ⬜未开 | — | 滚动宿主链路 |
| F5 | Anchor | `anchor_` | ⬜未开 | — | 同上 |
| F5 | Breadcrumb | `breadcrumb_` | ⬜未开 | — | — |
| F5 | Pagination | `pagination_` | ⬜未开 | — | — |
| F5 | Steps | `steps_` | ⬜未开 | — | — |
| F5 | Tabs | `tabs_` | ⬜未开 | — | 滑动线+溢出 |
| F5 | Calendar | `calendar_` | ⬜未开 | — | 依赖 G4 |
| F5 | Table | `table_` | ⬜未开 | — | 标准包法+虚拟 |
| F5 | List | `list_` | ⬜未开 | — | — |
| F5 | Tree | `tree_` | ⬜未开 | — | 勾选拖拽虚拟 |
| F5 | Transfer | `transfer_` | ⬜未开 | — | — |
| F5 | Collapse | `collapse_` | ⬜未开 | — | 高度动画 |
| F5 | Carousel | `carousel_` | ⬜未开 | — | 定时切先行 |
| F5 | Spin | `spin_` | ⬜未开 | — | — |
| F5 | Skeleton | `skeleton_` | ⬜未开 | — | — |
| F5 | Watermark | `watermark_` | ⬜未开 | — | 平铺+守卫 |
| F5 | Progress | `progress_` | ⬜未开 | — | — |
| F5 | Masonry | `masonry_` | ⬜未开 | — | 瀑布分配 |
| F5 | Form（联调） | `form_` | ⬜未开 | — | 校验时机+联动 |
| F6 | ConfigProvider | `config_provider_` | ⬜未开 | — | Ctx 6 件 |
| F6 | App | `app_` | ⬜未开 | — | 依赖 G1/G2 |
| F6 | Theme | `theme_` | ⬜未开 | — | 换肤 |
| F6 | Locale | `locale_` | ⬜未开 | — | 文案表 |

## 3. G1–G7 大件（与 F0 并行，不挡小组件）

| 大件 | 源码证据 | 状态 | 验证 commit | 备注 |
| --- | --- | --- | --- | --- |
| G1 弹窗宿主 | `ui/kit/modal_host_*.go` + Modal/Drawer 接入 | ⬜未开 | — | 栈层叠+命令式+焦点圈地 |
| G2 消息队列 | `ui/kit/notice_queue_*.go` + Message/Notification 接入 | ⬜未开 | — | 超数丢最旧+悬停冻结 |
| G3 漫游打洞 | `ui/kit/tour_mask_*.go` | ⬜未开 | — | 洞跟目标+点不透 |
| G4 日期引擎 | `ui/kit/date_engine_*.go` | ⬜未开 | — | 可换实现 |
| G5 颜色模型 | `ui/kit/color_model_*.go` | ⬜未开 | — | 来回转不丢精度 |
| G6 上传语义 | `ui/kit/upload_*.go`（语义+宿主三件） | ⬜未开 | — | 大文件不卡主树 |
| G7 二维码编码库 | `ui/kit/qrcode_*.go` | ⬜未开 | — | 同值扫出同码 |

## 4. 新会话定位流程（照做就不丢）

1. 读本表，看到哪行不是 ⬜。
2. 跑源码对照三条命令，抽查该行证据文件真存在、单测真绿。
3. 报出“已完成到哪、下一项是什么”，等用户确认再动手。
4. 完成一项就更新本表同行（状态+验证 commit），同一批提交。
