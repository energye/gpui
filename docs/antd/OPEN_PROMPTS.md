# 开工话术（新会话只用这一句，所有控件通用）

## 通用话术（复制这一段，把 `<名>` 换成组件名）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 <名>：
先读 AGENTS.md，再读 docs/antd/<名>.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/<名>，
不认 Gio。只改 ui/kit/<名>/ 与 examples/kit/<名>/（一组件一窗，只摆自己）。
先按测试七类（外观形状/点按反馈/开关显隐/值变化/键盘焦点/布局位置/特殊映射）
把官网 demo 拆成 §6.9 开头测试说明（特性与官网一一对应：测什么、怎么点、
看什么效果），再开工；做完 P0+P1（官方非 debug 一个不少），按这份说明逐功能
验效果（一项一效果，挂一项整窗挂，不许拿通用点击敷衍）。跑 ui/kit 中央门禁、
画廊门禁、按钮回归、<名>真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/<名> 按同一份说明亲手测完才关。
```

用法举例：开提醒就把上面四处 `<名>` 全换成 `alert`，开表格就全换成 `table`。

## 开工顺序（按波次从上往下，跨波等上游）

- W0：图标先行（最底层，多方引用）。
- W1：按钮标杆先签字，再剩下 18 个并行；标杆签字前其余不开工。
- W2：20 个并行；提示、气泡连带回炉，先回归再往下。
- W3：输入、选择先，再做自动完成/提及/数字输入/时间选择；抽屉/弹窗本体涉表单部分延后 W4。
- W4：多选/分页先 → 表格/列表 → 穿梭；树+选择就绪 → 树形下拉/级联；输入+时间就绪 → 日期 → 表单最后。
- W5：应用、配置最后。各文档 §2.7“等谁”行为准。

## 组件速查（特性要点以各文档 §6.8/§6.4 为准，本文不重复，避免腐烂）

| 组件 | 波次 | 文档 |
| --- | --- | --- |
| icon | W0 | docs/antd/icon.md |
| alert | W1 | docs/antd/alert.md |
| border-beam | W1 | docs/antd/border-beam.md |
| button | W1 | docs/antd/button.md |
| divider | W1 | docs/antd/divider.md |
| flex | W1 | docs/antd/flex.md |
| float-button | W1 | docs/antd/float-button.md |
| grid | W1 | docs/antd/grid.md |
| layout | W1 | docs/antd/layout.md |
| masonry | W1 | docs/antd/masonry.md |
| progress | W1 | docs/antd/progress.md |
| skeleton | W1 | docs/antd/skeleton.md |
| space | W1 | docs/antd/space.md |
| spin | W1 | docs/antd/spin.md |
| splitter | W1 | docs/antd/splitter.md |
| statistic | W1 | docs/antd/statistic.md |
| tag | W1 | docs/antd/tag.md |
| timeline | W1 | docs/antd/timeline.md |
| typography | W1 | docs/antd/typography.md |
| watermark | W1 | docs/antd/watermark.md |
| affix | W2 | docs/antd/affix.md |
| avatar | W2 | docs/antd/avatar.md |
| badge | W2 | docs/antd/badge.md |
| card | W2 | docs/antd/card.md |
| carousel | W2 | docs/antd/carousel.md |
| collapse | W2 | docs/antd/collapse.md |
| descriptions | W2 | docs/antd/descriptions.md |
| empty | W2 | docs/antd/empty.md |
| menu | W2 | docs/antd/menu.md |
| tooltip | W2 | docs/antd/tooltip.md |
| popover | W2 | docs/antd/popover.md |
| qr-code | W2 | docs/antd/qr-code.md |
| radio | W2 | docs/antd/radio.md |
| rate | W2 | docs/antd/rate.md |
| result | W2 | docs/antd/result.md |
| segmented | W2 | docs/antd/segmented.md |
| slider | W2 | docs/antd/slider.md |
| steps | W2 | docs/antd/steps.md |
| switch | W2 | docs/antd/switch.md |
| tabs | W2 | docs/antd/tabs.md |
| input | W3 | docs/antd/input.md |
| select | W3 | docs/antd/select.md |
| tree | W3 | docs/antd/tree.md |
| time-picker | W3 | docs/antd/time-picker.md |
| auto-complete | W3 | docs/antd/auto-complete.md |
| mentions | W3 | docs/antd/mentions.md |
| input-number | W3 | docs/antd/input-number.md |
| color-picker | W3 | docs/antd/color-picker.md |
| image | W3 | docs/antd/image.md |
| message | W3 | docs/antd/message.md |
| notification | W3 | docs/antd/notification.md |
| tour | W3 | docs/antd/tour.md |
| anchor | W3 | docs/antd/anchor.md |
| dropdown | W3 | docs/antd/dropdown.md |
| popconfirm | W3 | docs/antd/popconfirm.md |
| upload | W3 | docs/antd/upload.md |
| drawer | W3 | docs/antd/drawer.md |
| modal | W3 | docs/antd/modal.md |
| checkbox | W4 | docs/antd/checkbox.md |
| pagination | W4 | docs/antd/pagination.md |
| calendar | W4 | docs/antd/calendar.md |
| breadcrumb | W4 | docs/antd/breadcrumb.md |
| list | W4 | docs/antd/list.md |
| table | W4 | docs/antd/table.md |
| transfer | W4 | docs/antd/transfer.md |
| cascader | W4 | docs/antd/cascader.md |
| date-picker | W4 | docs/antd/date-picker.md |
| tree-select | W4 | docs/antd/tree-select.md |
| form | W4 | docs/antd/form.md |
| app | W5 | docs/antd/app.md |
| config-provider | W5 | docs/antd/config-provider.md |
| util | 随时 | docs/antd/util.md |
