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
| --- | --- | --- | --- | --- | --- | --- | --- |
| F0-1 主题与范围 | `ui/theme/seed*.go` + `ui/kit/internal/scope/{ctx,states,resolve}.go` + `ui/kit/scope.go` 公开门面 | `seed_ant_test.go`、`scope` 各 `_test.go` 全绿（逐文件跑） | `examples/kit_f0_scope/`（三套主题一切换全局生效；-auto-only 全绿 + Golden 差异0 + 调大小回基线 + 人工签字） | 🟩已绿 | 本次提交 | 种子与 antd 6.5.1 逐项对；真窗第二跑 3/3 + Golden 0 + resize 回 1200x800 |
| F0-2 布局+装饰 | `ui/kit/internal/prim/{layout,decor}.go` + `ui/kit/prim.go` 公开门面 | `layout_test.go` 约束矩阵、`layout_edge_test.go` 边界、`decor_test.go` 混合公式（逐文件跑全绿） | `examples/kit_f0_layout/`、`kit_f0_decor/`（-auto-only 全绿 + Golden 差异0 + 调大小回基线；F0-1 scope 回归绿） | 🟩已绿 | 本次提交 | layout 3/3探针+bind8/1000+fps57；decor 3/3探针+混合公式+fps采而不判（静态正确性，F0-5再判）；未动他线文件 |
| F0-3 内容 | `ui/kit/internal/prim/content.go` + `ui/kit/prim.go` 内容门面 | `content_test.go`（估算+合并+三态+图标，逐文件跑全绿） | `examples/kit_f0_content/`（-auto-only 全绿 6/6 + Golden 差异0 + 调大小回基线；F0-1/F0-2回归绿） | 🟩已绿 | 本次提交 | 文字三区墨量≥60+图片到图报错各只标脏一格+fps58；未动他线文件 |
| F0-4 交互+触发 | `ui/kit/internal/behavior/{interactive,field,overlay_trigger}.go` + `ui/kit/behavior.go` 公开门面 | 三个 `_test.go`（五态/受控/外点关，逐文件跑全绿） | `examples/kit_f0_interact/`（-auto-only 全绿 15/15 + Golden 差异0 + 调大小回基线；F0-1/F0-2/F0-3回归绿） | 🟩已绿 | 本次提交 | 点击计1/禁用0/防重0+受控只回调+拼写提交一次+四向精确翻转+外点关Esc+焦点锁+环只键盘亮+命中一致+浮层只脏一格+fps58；未动他线文件 |
| F0-5 动效语义性能方向+门禁 | `ui/kit/internal/behavior/{motion,semantics,performance,direction}.go` + `ui/kit/internal/gate/{props,state,wiring,token}.go` + `ui/kit/{behavior,gate}.go` 门面 + `ui/scheduler/ticker.go` 收尾合并修 + `docs/antd/F0_GATE_CHECK.md` 门禁命令 | 五个 `_test.go` + `ticker_late_add_test.go`（逐文件跑全绿）+ 门禁自检（接线ok/自有硬编码ok/构建ok） | `examples/kit_f0_motion/`（-auto-only 全绿 17/17 + Golden 差异0 + 调大小回基线 + 省画retained/damage_union/damage0.09 + 光栅0.22ms；F0-1~F0-4回归绿） | 🟩已绿 | 本次提交 | 转圈真转+省动效全停+波纹点后散+有名有角色+RTL镜像+四门禁自检+fps59.7；底层修在调度器收尾合并处；未动他线文件 |
| F0-6 六窗全绿签字 | 六窗 `main.go` + 三套主题 Golden | — | 六窗 `-auto-only` 全绿 + 人工签字 | ⬜未开 | — | 先决 F0-5；签字不过 F1 不开 |

## 2. F1–F6 组件（每行一组件，源码对不上即假绿）

证据模板（每行通用）：源码 `ui/kit/<前缀>_*.go`（五段齐，多词前缀一律小写下划线）→ 单测 `ui/kit/<前缀>_*_test.go` + `testdata/` → 真窗 `examples/kit/<短横线名>/`（源码下划线、目录短横线，如 `float_button_*.go` ↔ `examples/kit/float-button/`；带限定词的拆窗规则见行备注）→ Golden `testdata/showcase*`。

| 波 | 组件 | 源码前缀 | 真窗目录 | 状态 | 验证 commit | 备注 |
| --- | --- | --- | --- | --- | --- |
| F1 | Icon | `icon_` | `examples/kit/icon/` | ⬜未开 | — | 最先落地，全库引用 |
| F1 | Typography | `typography_` | `examples/kit/typography/` | ⬜未开 | — | 剪贴板宿主后补 |
| F1 | Divider | `divider_` | `examples/kit/divider/` | ⬜未开 | — | — |
| F1 | Tag | `tag_` | `examples/kit/tag/` | ⬜未开 | — | — |
| F1 | Badge | `badge_` | `examples/kit/badge/` | ⬜未开 | — | — |
| F1 | Avatar | `avatar_` | `examples/kit/avatar/` | ⬜未开 | — | 组popover走D类 |
| F1 | Card | `card_` | `examples/kit/card/` | ⬜未开 | — | — |
| F1 | Empty | `empty_` | `examples/kit/empty/` | ⬜未开 | — | — |
| F1 | Result | `result_` | `examples/kit/result/` | ⬜未开 | — | — |
| F1 | Statistic | `statistic_` | `examples/kit/statistic/` | ⬜未开 | — | 倒计时走 Motion 计时 |
| F1 | Descriptions | `descriptions_` | `examples/kit/descriptions/` | ⬜未开 | — | — |
| F1 | Timeline | `timeline_` | `examples/kit/timeline/` | ⬜未开 | — | — |
| F1 | Image（展示态） | `image_` | `examples/kit/image/`（展示一窗；预览浮层走 D 类另验，不另起窗） | ⬜未开 | — | 预览浮层走D类 |
| F1 | Alert | `alert_` | `examples/kit/alert/` | ⬜未开 | — | — |
| F2 | Button（标杆） | `button_` | `examples/kit/button/` | ⬜未开 | — | 五样签字缺一不开后续 |
| F2 | Switch | `switch_` | `examples/kit/switch/` | ⬜未开 | — | — |
| F2 | Checkbox | `checkbox_` | `examples/kit/checkbox/` | ⬜未开 | — | — |
| F2 | Radio | `radio_` | `examples/kit/radio/` | ⬜未开 | — | — |
| F2 | Rate | `rate_` | `examples/kit/rate/` | ⬜未开 | — | 半区命中 L2 加 |
| F2 | Slider | `slider_` | `examples/kit/slider/` | ⬜未开 | — | 拖动吸附六项 |
| F2 | Segmented | `segmented_` | `examples/kit/segmented/` | ⬜未开 | — | — |
| F2 | FloatButton | `float_button_` | `examples/kit/float-button/` | ⬜未开 | — | — |
| F3 | Input | `input_` | `examples/kit/input/` | ⬜未开 | — | OTP 多框联动 |
| F3 | InputNumber | `input_number_` | `examples/kit/input-number/` | ⬜未开 | — | — |
| F3 | Select | `select_` | `examples/kit/select/` | ⬜未开 | — | 虚拟行高耦合+`bind_count` 门禁；popupMatch=false 关虚拟、无障碍降级（语义/焦点节点） |
| F3 | AutoComplete | `auto_complete_` | `examples/kit/auto-complete/` | ⬜未开 | — | — |
| F3 | Mentions | `mentions_` | `examples/kit/mentions/` | ⬜未开 | — | — |
| F3 | Cascader | `cascader_` | `examples/kit/cascader/` | ⬜未开 | — | 懒加载异步（树数据，依赖 E 类 Tree 异步小件） |
| F3 | TreeSelect | `tree_select_` | `examples/kit/tree-select/` | ⬜未开 | — | 依赖 E 类 Tree（勾选联动+异步） |
| F3 | DatePicker | `date_picker_` | `examples/kit/date-picker/` | ⬜未开 | — | 依赖 G4 |
| F3 | TimePicker | `time_picker_` | `examples/kit/time-picker/` | ⬜未开 | — | 依赖 G4 |
| F3 | ColorPicker | `color_picker_` | `examples/kit/color-picker/` | ⬜未开 | — | 依赖 G5 |
| F3 | Upload | `upload_` | `examples/kit/upload/` | ⬜未开 | — | 依赖 G6 |
| F4 | Tooltip | `tooltip_` | `examples/kit/tooltip/` | ⬜未开 | — | 定位底座先行 |
| F4 | Popover | `popover_` | `examples/kit/popover/` | ⬜未开 | — | — |
| F4 | Popconfirm | `popconfirm_` | `examples/kit/popconfirm/` | ⬜未开 | — | — |
| F4 | Dropdown | `dropdown_` | `examples/kit/dropdown/` | ⬜未开 | — | 多级延后 |
| F4 | Menu（弹出态） | `menu_` | `examples/kit/menu/`（内嵌+弹层同一窗分段，弹层挂载另验） | ⬜未开 | — | — |
| F4 | Modal | `modal_` | `examples/kit/modal/` | ⬜未开 | — | 依赖 G1 |
| F4 | Drawer | `drawer_` | `examples/kit/drawer/` | ⬜未开 | — | 依赖 G1 |
| F4 | Message | `message_` | `examples/kit/message/` | ⬜未开 | — | 依赖 G2 |
| F4 | Notification | `notification_` | `examples/kit/notification/` | ⬜未开 | — | 依赖 G2 |
| F4 | Tour | `tour_` | `examples/kit/tour/` | ⬜未开 | — | 依赖 G3 |
| F5 | Layout | `layout_` | `examples/kit/layout/` | ⬜未开 | — | — |
| F5 | Grid | `grid_` | `examples/kit/grid/` | ⬜未开 | — | — |
| F5 | Flex | `flex_` | `examples/kit/flex/` | ⬜未开 | — | — |
| F5 | Space | `space_` | `examples/kit/space/` | ⬜未开 | — | — |
| F5 | Splitter | `splitter_` | `examples/kit/splitter/` | ⬜未开 | — | 拖拽分栏 |
| F5 | Affix | `affix_` | `examples/kit/affix/` | ⬜未开 | — | 滚动宿主链路（与 Anchor/表头 sticky 共用，见 Table） |
| F5 | Anchor | `anchor_` | `examples/kit/anchor/` | ⬜未开 | — | 同上滚动宿主链路 |
| F5 | Breadcrumb | `breadcrumb_` | `examples/kit/breadcrumb/` | ⬜未开 | — | — |
| F5 | Pagination | `pagination_` | `examples/kit/pagination/` | ⬜未开 | — | — |
| F5 | Steps | `steps_` | `examples/kit/steps/` | ⬜未开 | — | — |
| F5 | Tabs | `tabs_` | `examples/kit/tabs/` | ⬜未开 | — | 滑动线+溢出 |
| F5 | Calendar | `calendar_` | `examples/kit/calendar/` | ⬜未开 | — | 依赖 G4 |
| F5 | Table | `table_` | `examples/kit/table/` | ⬜未开 | — | 标准包法+虚拟；sticky 与 Affix/Anchor 共滚动宿主链路 |
| F5 | List | `list_` | `examples/kit/list/` | ⬜未开 | — | — |
| F5 | Tree | `tree_` | `examples/kit/tree/` | ⬜未开 | — | 勾选拖拽虚拟 |
| F5 | Transfer | `transfer_` | `examples/kit/transfer/` | ⬜未开 | — | — |
| F5 | Collapse | `collapse_` | `examples/kit/collapse/` | ⬜未开 | — | 高度动画 |
| F5 | Carousel | `carousel_` | `examples/kit/carousel/` | ⬜未开 | — | 定时切先行 |
| F5 | Spin | `spin_` | `examples/kit/spin/` | ⬜未开 | — | — |
| F5 | Skeleton | `skeleton_` | `examples/kit/skeleton/` | ⬜未开 | — | — |
| F5 | Watermark | `watermark_` | `examples/kit/watermark/` | ⬜未开 | — | 平铺+守卫 |
| F5 | Progress | `progress_` | `examples/kit/progress/` | ⬜未开 | — | — |
| F5 | Masonry | `masonry_` | `examples/kit/masonry/` | ⬜未开 | — | 瀑布分配 |
| F5 | Form（联调） | `form_` | `examples/kit/form/` | ⬜未开 | — | 校验时机+联动 |
| F1 | QRCode | `qrcode_` | `examples/kit/qr-code/` | ⬜未开 | — | 依赖 G7 编码库（G7 无独立消费窗，随本行签字） |
| F5 | BorderBeam（自有扩展） | `border_beam_` | `examples/kit/border-beam/` | ⬜未开 | — | 自有 Token，不占 antd 对齐口 |
| F6 | Util（无 UI） | —（不进 kit，按类型对照） | —（无窗，单测即证据） | ⬜未开 | — | 纯函数移植 |
| F6 | ConfigProvider | `config_provider_` | `examples/kit/config-provider/` | ⬜未开 | — | Ctx 6 件 |
| F6 | App | `app_` | `examples/kit/app/` | ⬜未开 | — | 依赖 G1/G2 |
| F6 | Theme | `theme_` | `examples/kit/theme/` | ⬜未开 | — | 换肤 |
| F6 | Locale | `locale_` | `examples/kit/locale/` | ⬜未开 | — | 文案表 |

## 3. G1–G7 大件（与 F0 并行，不挡小组件）

| 大件 | 源码证据 | 状态 | 验证 commit | 备注 |
| --- | --- | --- | --- | --- |
| G1 弹窗宿主 | `ui/kit/modal_host_*.go` + `examples/kit/modal-host/`（Modal/Drawer 接入待 F4） | ✅已签字 | 本次提交 | 栈1000/1010/1020+单关保序+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看 |
| G2 消息队列 | `ui/kit/notice_queue_*.go` + `examples/kit/notice-queue/`（Message/Notification 接入待 F4） | ✅已签字 | 本次提交 | 顶8/宽384/角24+maxCount丢最旧+同key更新+悬停冻结+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看（占位结构已确认，最终像素待F4） |
| G3 漫游打洞 | `ui/kit/tour_mask_*.go` + `examples/kit/tour-mask/`（Tour 接入待 F4） | ✅已签字 | 本次提交 | 洞跟目标+gap6/r2+面板520/z1001+受控current+mask关仍绘洞+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看（占位结构已确认，最终像素待F4） |
| G4 日期引擎 | `ui/kit/date_engine_*.go` + `examples/kit/date-engine/`（DatePicker/Calendar 接入待 F3/F5） | ✅已签字 | 本次提交 | 闰年周起始全对+多格式首个展示+佛历+disabled矩阵+Range排序+多选+预设函数+受控面板+可换实现；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看 |
| G5 颜色模型 | `ui/kit/color_model_*.go` + `examples/kit/color-model/`（ColorPicker 接入待 F3） | ✅已签字 | 本次提交 | HSB来回±1+受控对象精度+拖中回调松手提交+渐变stops排序+三开关+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看（占位结构已确认，最终像素待F3） |
| G6 上传语义 | `ui/kit/upload_*.go` + `examples/kit/upload/`（Upload 接入待 F3） | ✅已签字 | 本次提交 | uid补齐+LIST_IGNORE+beforeUpload三返回+受控忽略+maxCount替换截断+默认Tick渐进+业务三回调+宿主三件+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看 |
| G7 二维码编码库 | `ui/kit/qrcode_*.go` + `examples/kit/qrcode/`（QRCode 接入待 F5） | ✅已签字 | 本次提交 | skip2纯Go后端可换+白边剥离绘制层加+boost自动抬升+M升H实数+四态cover+刷新一次+静态吃主题；真窗第二跑3/3+Golden 0+resize回1200x800+人工已看 |

## 4. 新会话定位流程（照做就不丢）

1. 读本表，看到哪行不是 ⬜。
2. 跑源码对照三条命令，抽查该行证据文件真存在、单测真绿。
3. 报出“已完成到哪、下一项是什么”，等用户确认再动手。
4. 完成一项就更新本表同行（状态+验证 commit），同一批提交。
