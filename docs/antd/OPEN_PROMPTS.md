# 每个组件开工话术（新会话直接复制粘贴一句就能开工）

> 怎么用：正常情况只复制**通用话术**那一行就行，`<名>` 换成组件名
> （比如 alert、tag、table）。话术里锁死了七件事，不用再口头补：
> 读 `AGENTS.md`、读该组件 `docs/antd/<名>.md §6`、只认官网 6.5.1、
> 只改 `ui/kit/<名>/` 与 `examples/kit/<名>/`、先写 §6.9 开头测试说明
> 再开工、按特性流程逐功能验效果、四处回归全绿、人亲手点完才关。
> 开工顺序按波次从上往下走，跨波必须等上游（等谁见各文档 §2.7）。
> 下面各组件那 72 行是已填好特性要点的现成版，想省事直接复制也行，
> 跟通用话术效力一样。

## 通用话术（新会话复制这一行，<名>换成组件名）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 <名>：
先读 AGENTS.md，再读 docs/antd/<名>.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/<名>，
不认 Gio。只改 ui/kit/<名>/ 与 examples/kit/<名>/（一组件一窗，只摆自己）。
先在该组件文档 §6.9 开头写清测试说明（特性与官网 demo 一一对应：测什么、
怎么点、看什么效果），再开工；做完 P0+P1（官方非 debug 一个不少），按这份
测试说明逐功能验效果（一项一效果，挂一项整窗挂，不许拿通用点击敷衍）。
跑 ui/kit、画廊门禁、按钮回归、<名>真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/<名> 按同一份说明亲手测完才关。
```

## 各组件现成版（跟通用话术效力一样，直接复制也行）

## W0 地基（1 个，先行）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 icon：
先读 AGENTS.md，再读 docs/antd/icon.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/icon，
不认 Gio。只改 ui/kit/icon/ 与 examples/kit/icon/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、icon 真窗
-auto-only 四处全绿，最后我跑 go run ./examples/kit/icon 亲手点完才关。
```

## W1 基础件（19 个：按钮标杆先签字，再 18 个并行）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 alert：
先读 AGENTS.md，再读 docs/antd/alert.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/alert，
不认 Gio。只改 ui/kit/alert/ 与 examples/kit/alert/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（四套语义色/可关真关/描述双行/图标/顶部公告/操作槽，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、alert 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/alert 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 border-beam：
先读 AGENTS.md，再读 docs/antd/border-beam.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components（自有扩展，
对照 Card/Modal 被装饰容器），不认 Gio。只改 ui/kit/border-beam/ 与
examples/kit/border-beam/（一组件一窗，只摆自己），做完 P0+P1（官方非 debug
一个不少），按该组件 §6.4 特性流程逐功能验效果（一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、border-beam 真窗 -auto-only 四处全绿，
最后我跑 go run ./examples/kit/border-beam 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 button（标杆先签字）：
先读 AGENTS.md，再读 docs/antd/button.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/button，
不认 Gio。只改 ui/kit/button/ 与 examples/kit/button/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（点看回调/边框看有无/悬停按压看变色#4096ff/#0958d9/焦点看环/键盘看激活/
禁用加载看吞事件，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、
button 真窗 -auto-only 四处全绿，最后我跑 go run ./examples/kit/button
亲手点完才关。标杆签字前 W1 其余 18 个不开工。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 divider：
先读 AGENTS.md，再读 docs/antd/divider.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/divider，
不认 Gio。只改 ui/kit/divider/ 与 examples/kit/divider/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（水平竖向/文字方位/虚线/变体，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、divider 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/divider 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 flex：
先读 AGENTS.md，再读 docs/antd/flex.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/flex，
不认 Gio。只改 ui/kit/flex/ 与 examples/kit/flex/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（横竖/对齐/间距/gap，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、
按钮回归、flex 真窗 -auto-only 四处全绿，最后我跑 go run ./examples/kit/flex
亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 float-button：
先读 AGENTS.md，再读 docs/antd/float-button.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/float-button，
不认 Gio。只改 ui/kit/float-button/ 与 examples/kit/float-button/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（点击回调/禁用吞事件/主次色/圆方/分组展开收起/回顶，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、float-button 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/float-button 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 grid：
先读 AGENTS.md，再读 docs/antd/grid.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/grid，
不认 Gio。只改 ui/kit/grid/ 与 examples/kit/grid/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（24 格/间距/偏移/排序/响应式断点，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、grid 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/grid 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 layout：
先读 AGENTS.md，再读 docs/antd/layout.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/layout，
不认 Gio。只改 ui/kit/layout/ 与 examples/kit/layout/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（四区/顶高64/侧宽200/折叠变80/深色/断点自折叠，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、layout 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/layout 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 masonry：
先读 AGENTS.md，再读 docs/antd/masonry.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/masonry，
不认 Gio。只改 ui/kit/masonry/ 与 examples/kit/masonry/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（列分配/响应式/图片/动态更新，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、masonry 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/masonry 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 progress：
先读 AGENTS.md，再读 docs/antd/progress.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/progress，
不认 Gio。只改 ui/kit/progress/ 与 examples/kit/progress/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（线/圈/仪表/进度值/状态色/渐变，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、progress 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/progress 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 skeleton：
先读 AGENTS.md，再读 docs/antd/skeleton.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/skeleton，
不认 Gio。只改 ui/kit/skeleton/ 与 examples/kit/skeleton/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（加载切换/动态/段落行数/圆角/按钮输入头像占位，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、skeleton 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/skeleton 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 space：
先读 AGENTS.md，再读 docs/antd/space.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/space，
不认 Gio。只改 ui/kit/space/ 与 examples/kit/space/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（横竖/对齐/间距档/自动换行/紧凑，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、space 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/space 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 spin：
先读 AGENTS.md，再读 docs/antd/spin.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/spin，
不认 Gio。只改 ui/kit/spin/ 与 examples/kit/spin/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（转圈/尺寸/延迟/容器罩层/提示文案，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、spin 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/spin 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 splitter：
先读 AGENTS.md，再读 docs/antd/splitter.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/splitter，
不认 Gio。只改 ui/kit/splitter/ 与 examples/kit/splitter/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（拖±40/最小最大钳位/松手回调/折叠/竖向/懒提交/不可拖，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、splitter 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/splitter 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 statistic：
先读 AGENTS.md，再读 docs/antd/statistic.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/statistic，
不认 Gio。只改 ui/kit/statistic/ 与 examples/kit/statistic/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（数值/前后缀/精度/分组/倒计时刷新/加载骨架，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、statistic 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/statistic 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tag：
先读 AGENTS.md，再读 docs/antd/tag.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tag，
不认 Gio。只改 ui/kit/tag/ 与 examples/kit/tag/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（可关真关/阻止关闭/勾选切换/单多选/色板/边框，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、tag 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/tag 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 timeline：
先读 AGENTS.md，再读 docs/antd/timeline.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/timeline，
不认 Gio。只改 ui/kit/timeline/ 与 examples/kit/timeline/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（单双色/自定义点/加载点/位置/标签，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、timeline 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/timeline 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 typography：
先读 AGENTS.md，再读 docs/antd/typography.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/typography，
不认 Gio。只改 ui/kit/typography/ 与 examples/kit/typography/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（标题阶梯/语义色/复制/省略展开/编辑提交取消/禁用，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、typography 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/typography 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 watermark：
先读 AGENTS.md，再读 docs/antd/watermark.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/watermark，
不认 Gio。只改 ui/kit/watermark/ 与 examples/kit/watermark/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（单行多行/图片/透明/旋转/间距/继承透传，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、watermark 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/watermark 亲手点完才关。
```

## W2 组合件（20 个并行：提示气泡连带回炉，先回归再往下）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 affix：
先读 AGENTS.md，再读 docs/antd/affix.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/affix，
不认 Gio。只改 ui/kit/affix/ 与 examples/kit/affix/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（吸顶偏移/固定/目标容器，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、
按钮回归、affix 真窗 -auto-only 四处全绿，最后我跑 go run ./examples/kit/affix
亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 avatar：
先读 AGENTS.md，再读 docs/antd/avatar.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/avatar，
不认 Gio。只改 ui/kit/avatar/ 与 examples/kit/avatar/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（文字图片图标/尺寸形状/群组/徽标叠加，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、avatar 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/avatar 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 badge：
先读 AGENTS.md，再读 docs/antd/badge.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/badge，
不认 Gio。只改 ui/kit/badge/ 与 examples/kit/badge/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（计数封顶/小红点/状态点/缎带/偏移，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、badge 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/badge 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 card：
先读 AGENTS.md，再读 docs/antd/card.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/card，
不认 Gio。只改 ui/kit/card/ 与 examples/kit/card/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（标题封面操作/边框阴影/加载骨架/页签，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、card 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/card 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 carousel：
先读 AGENTS.md，再读 docs/antd/carousel.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/carousel，
不认 Gio。只改 ui/kit/carousel/ 与 examples/kit/carousel/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（自动播/箭头点/指示点/拖拽切/竖向，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、carousel 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/carousel 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 collapse：
先读 AGENTS.md，再读 docs/antd/collapse.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/collapse，
不认 Gio。只改 ui/kit/collapse/ 与 examples/kit/collapse/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（展开收起/手风琴/禁用/自定义头/受控，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、collapse 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/collapse 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 descriptions：
先读 AGENTS.md，再读 docs/antd/descriptions.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/descriptions，
不认 Gio。只改 ui/kit/descriptions/ 与 examples/kit/descriptions/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（列数/边框/尺寸/竖向/跨列，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、descriptions 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/descriptions 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 empty：
先读 AGENTS.md，再读 docs/antd/empty.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/empty，
不认 Gio。只改 ui/kit/empty/ 与 examples/kit/empty/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（默认图/简单图/自定义图与文案/底部操作，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、empty 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/empty 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 menu：
先读 AGENTS.md，再读 docs/antd/menu.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/menu，
不认 Gio。只改 ui/kit/menu/ 与 examples/kit/menu/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（选中/展开/横竖内嵌/禁用/弹层，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、menu 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/menu 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tooltip（连带回炉）：
先读 AGENTS.md，再读 docs/antd/tooltip.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tooltip，
不认 Gio。只改 ui/kit/tooltip/ 与 examples/kit/tooltip/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（悬停开/离开关/12 向/聚焦开失焦关/点击开关/退出键关/受控，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、tooltip 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/tooltip 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 popover（连带回炉）：
先读 AGENTS.md，再读 docs/antd/popover.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/popover，
不认 Gio。只改 ui/kit/popover/ 与 examples/kit/popover/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（悬停开/点击切换/聚焦开/右键开/外点关/退出键关/内容按钮可点，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、popover 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/popover 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 qr-code：
先读 AGENTS.md，再读 docs/antd/qr-code.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/qr-code，
不认 Gio。只改 ui/kit/qr-code/ 与 examples/kit/qr-code/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（编码/尺寸/纠错/图标/过期刷新/下载映射，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、qr-code 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/qr-code 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 radio：
先读 AGENTS.md，再读 docs/antd/radio.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/radio，
不认 Gio。只改 ui/kit/radio/ 与 examples/kit/radio/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（选中/组互斥/禁用/按钮样式/键盘，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、radio 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/radio 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 rate：
先读 AGENTS.md，再读 docs/antd/rate.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/rate，
不认 Gio。只改 ui/kit/rate/ 与 examples/kit/rate/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（半星/文案/只读禁用/清除/键盘，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、rate 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/rate 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 result：
先读 AGENTS.md，再读 docs/antd/result.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/result，
不认 Gio。只改 ui/kit/result/ 与 examples/kit/result/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（成功失败警告/图标/标题副标题/操作区，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、result 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/result 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 segmented：
先读 AGENTS.md，再读 docs/antd/segmented.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/segmented，
不认 Gio。只改 ui/kit/segmented/ 与 examples/kit/segmented/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（选中切换/禁用/尺寸/图标/受控，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、segmented 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/segmented 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 slider：
先读 AGENTS.md，再读 docs/antd/slider.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/slider，
不认 Gio。只改 ui/kit/slider/ 与 examples/kit/slider/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（拖动值/范围双柄/刻度/禁用/竖向/键盘，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、slider 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/slider 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 steps：
先读 AGENTS.md，再读 docs/antd/steps.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/steps，
不认 Gio。只改 ui/kit/steps/ 与 examples/kit/steps/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（当前步/状态/点击切换/竖向/小尺寸/进度点，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、steps 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/steps 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 switch：
先读 AGENTS.md，再读 docs/antd/switch.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/switch，
不认 Gio。只改 ui/kit/switch/ 与 examples/kit/switch/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（开合切换/加载中/禁用/文字图标/尺寸/键盘，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、switch 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/switch 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tabs：
先读 AGENTS.md，再读 docs/antd/tabs.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tabs，
不认 Gio。只改 ui/kit/tabs/ 与 examples/kit/tabs/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（切换/新增关页/禁用/位置/尺寸/溢出，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、tabs 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/tabs 亲手点完才关。
```

## W3 浮层表单件（18 个并行：输入选择先行）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 input：
先读 AGENTS.md，再读 docs/antd/input.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/input，
不认 Gio。只改 ui/kit/input/ 与 examples/kit/input/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（输入回写/前后缀/清除/计数/密码显隐/禁用/状态，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、input 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/input 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 select：
先读 AGENTS.md，再读 docs/antd/select.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/select，
不认 Gio。只改 ui/kit/select/ 与 examples/kit/select/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（展开选值/搜索过滤/多选标签/清除/禁用/状态，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、select 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/select 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tree：
先读 AGENTS.md，再读 docs/antd/tree.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tree，
不认 Gio。只改 ui/kit/tree/ 与 examples/kit/tree/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（展开收起/选中/勾选半选/禁用/拖拽/搜索，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、tree 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/tree 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 time-picker：
先读 AGENTS.md，再读 docs/antd/time-picker.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/time-picker，
不认 Gio。只改 ui/kit/time-picker/ 与 examples/kit/time-picker/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（选时分秒/禁用项/格式化/清除/弹层开合，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、time-picker 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/time-picker 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 auto-complete：
先读 AGENTS.md，再读 docs/antd/auto-complete.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/auto-complete，
不认 Gio（等 input 就绪）。只改 ui/kit/auto-complete/ 与 examples/kit/auto-complete/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（输入联想/选中回填/清除/自定义选项，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、auto-complete 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/auto-complete 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 mentions：
先读 AGENTS.md，再读 docs/antd/mentions.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/mentions，
不认 Gio（等 input 就绪）。只改 ui/kit/mentions/ 与 examples/kit/mentions/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（@触发/选中插入/受控值/禁用，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、mentions 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/mentions 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 input-number：
先读 AGENTS.md，再读 docs/antd/input-number.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/input-number，
不认 Gio（等 input 就绪）。只改 ui/kit/input-number/ 与 examples/kit/input-number/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（加减/键盘上下/精度步长/越界钳位/禁用，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、input-number 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/input-number 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 color-picker：
先读 AGENTS.md，再读 docs/antd/color-picker.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/color-picker，
不认 Gio。只改 ui/kit/color-picker/ 与 examples/kit/color-picker/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（选色/透明/预设/受控值/清除/禁用，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、color-picker 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/color-picker 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 image：
先读 AGENTS.md，再读 docs/antd/image.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/image，
不认 Gio。只改 ui/kit/image/ 与 examples/kit/image/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（预览开合/缩放旋转/多图切换/占位失败，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、image 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/image 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 message：
先读 AGENTS.md，再读 docs/antd/message.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/message，
不认 Gio。只改 ui/kit/message/ 与 examples/kit/message/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（成功失败警告加载/时长自动关/手动关/队列，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、message 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/message 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 notification：
先读 AGENTS.md，再读 docs/antd/notification.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/notification，
不认 Gio。只改 ui/kit/notification/ 与 examples/kit/notification/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（四态/自动关/手动关/位置/操作按钮，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、notification 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/notification 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tour：
先读 AGENTS.md，再读 docs/antd/tour.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tour，
不认 Gio。只改 ui/kit/tour/ 与 examples/kit/tour/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（上一步下一步/跳过/定位/遮罩，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、tour 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/tour 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 anchor：
先读 AGENTS.md，再读 docs/antd/anchor.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/anchor，
不认 Gio。只改 ui/kit/anchor/ 与 examples/kit/anchor/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（锚点跳转/高亮跟随/偏移/滚动容器，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、anchor 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/anchor 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 dropdown：
先读 AGENTS.md，再读 docs/antd/dropdown.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/dropdown，
不认 Gio。只改 ui/kit/dropdown/ 与 examples/kit/dropdown/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（点击悬停展开/选中回调/禁用项/多级/箭头，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、dropdown 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/dropdown 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 popconfirm：
先读 AGENTS.md，再读 docs/antd/popconfirm.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/popconfirm，
不认 Gio。只改 ui/kit/popconfirm/ 与 examples/kit/popconfirm/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（确认取消/位置/图标/受控，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、popconfirm 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/popconfirm 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 upload：
先读 AGENTS.md，再读 docs/antd/upload.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/upload，
不认 Gio。只改 ui/kit/upload/ 与 examples/kit/upload/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（选文件映射/列表/进度/删除/拖拽/限制，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、upload 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/upload 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 drawer：
先读 AGENTS.md，再读 docs/antd/drawer.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/drawer，
不认 Gio（表单联调延后 W4）。只改 ui/kit/drawer/ 与 examples/kit/drawer/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（四向/开合/遮罩关/退出键关/尺寸，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、drawer 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/drawer 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 modal：
先读 AGENTS.md，再读 docs/antd/modal.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/modal，
不认 Gio（表单联调延后 W4）。只改 ui/kit/modal/ 与 examples/kit/modal/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（确定取消/遮罩关/退出键关/居中/确认框，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、modal 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/modal 亲手点完才关。
```

## W4 抬升件 + 聚合（11 个：多选分页先行）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 checkbox：
先读 AGENTS.md，再读 docs/antd/checkbox.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/checkbox，
不认 Gio。只改 ui/kit/checkbox/ 与 examples/kit/checkbox/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（勾选半选/组互斥全选/禁用/键盘，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、checkbox 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/checkbox 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 pagination：
先读 AGENTS.md，再读 docs/antd/pagination.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/pagination，
不认 Gio。只改 ui/kit/pagination/ 与 examples/kit/pagination/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（翻页/尺寸切换/跳页/总数/禁用，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、pagination 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/pagination 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 calendar：
先读 AGENTS.md，再读 docs/antd/calendar.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/calendar，
不认 Gio。只改 ui/kit/calendar/ 与 examples/kit/calendar/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（年月切/日选/禁用日/农历假日/受控，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、calendar 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/calendar 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 breadcrumb：
先读 AGENTS.md，再读 docs/antd/breadcrumb.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/breadcrumb，
不认 Gio。只改 ui/kit/breadcrumb/ 与 examples/kit/breadcrumb/（一组件一窗，
只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能
验效果（层级跳转/分隔符/下拉菜单/溢出，一项一效果，挂一项整窗挂），跑 ui/kit、
画廊门禁、按钮回归、breadcrumb 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/breadcrumb 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 list：
先读 AGENTS.md，再读 docs/antd/list.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/list，
不认 Gio（等 checkbox/pagination 就绪）。只改 ui/kit/list/ 与 examples/kit/list/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（分页/加载更多/空态/操作项，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、list 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/list 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 table：
先读 AGENTS.md，再读 docs/antd/table.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/table，
不认 Gio（等 checkbox/pagination 就绪）。只改 ui/kit/table/ 与 examples/kit/table/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（排序筛选/分页/勾选/展开行/空态/固定列，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、table 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/table 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 transfer：
先读 AGENTS.md，再读 docs/antd/transfer.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/transfer，
不认 Gio（等 table/list 就绪）。只改 ui/kit/transfer/ 与 examples/kit/transfer/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（左右互传/全选/搜索/禁用，一项一效果，挂一项整窗挂），
跑 ui/kit、画廊门禁、按钮回归、transfer 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/transfer 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 cascader：
先读 AGENTS.md，再读 docs/antd/cascader.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/cascader，
不认 Gio（等 select 就绪）。只改 ui/kit/cascader/ 与 examples/kit/cascader/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（多级展开/选中回填/搜索/禁用/清除，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、cascader 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/cascader 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 date-picker：
先读 AGENTS.md，再读 docs/antd/date-picker.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/date-picker，
不认 Gio（等 input/time-picker 就绪）。只改 ui/kit/date-picker/ 与
examples/kit/date-picker/（一组件一窗，只摆自己），做完 P0+P1（官方非 debug
一个不少），按该组件 §6.4 特性流程逐功能验效果（选日/范围/时间/禁用日/
格式化/清除，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、
date-picker 真窗 -auto-only 四处全绿，最后我跑 go run ./examples/kit/date-picker
亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 tree-select：
先读 AGENTS.md，再读 docs/antd/tree-select.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/tree-select，
不认 Gio（等 tree+select 就绪）。只改 ui/kit/tree-select/ 与
examples/kit/tree-select/（一组件一窗，只摆自己），做完 P0+P1（官方非 debug
一个不少），按该组件 §6.4 特性流程逐功能验效果（树选/多选标签/搜索/禁用/
清除，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、tree-select
真窗 -auto-only 四处全绿，最后我跑 go run ./examples/kit/tree-select 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 form：
先读 AGENTS.md，再读 docs/antd/form.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/form，
不认 Gio（等 date-picker 就绪，最后做）。只改 ui/kit/form/ 与 examples/kit/form/
（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4
特性流程逐功能验效果（布局/提交校验/联动/禁用/尺寸/错误提示，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、form 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/form 亲手点完才关。
```

## W5 全局（最后，2 个并行）

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 app：
先读 AGENTS.md，再读 docs/antd/app.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/app，
不认 Gio（等 message/modal/notification+button/input 就绪）。只改 ui/kit/app/ 与
examples/kit/app/（一组件一窗，只摆自己），做完 P0+P1（官方非 debug 一个不少），
按该组件 §6.4 特性流程逐功能验效果（上下文/消息弹窗通知透传，一项一效果，
挂一项整窗挂），跑 ui/kit、画廊门禁、按钮回归、app 真窗 -auto-only
四处全绿，最后我跑 go run ./examples/kit/app 亲手点完才关。
```

```text
按 AGENTS.md 与 docs/antd/ACCEPTANCE.md 第七节三道门禁+第八节硬规则开工 config-provider：
先读 AGENTS.md，再读 docs/antd/config-provider.md §6（以 §6 为 DoD，§1–§3 只参考），
版本冻 antd 6.5.1，只认 /home/yanghy/app/projects/ant-design/components/config-provider，
不认 Gio（等 message/modal/notification+button/input 就绪）。只改
ui/kit/config-provider/ 与 examples/kit/config-provider/（一组件一窗，只摆自己），
做完 P0+P1（官方非 debug 一个不少），按该组件 §6.4 特性流程逐功能验效果
（主题语言尺寸全局透传，一项一效果，挂一项整窗挂），跑 ui/kit、画廊门禁、
按钮回归、config-provider 真窗 -auto-only 四处全绿，最后我跑
go run ./examples/kit/config-provider 亲手点完才关。
```
