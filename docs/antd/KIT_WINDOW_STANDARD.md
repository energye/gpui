# kit 真窗自说明标准（一窗一种 + 官网页形态 + 四件套）

> 地位：AGENTS.md「kit 示例窗自说明」硬规矩的详述。适用范围：**仅 `examples/kit/*`**；
> kit 窗与其他模块示例（engine/render/video）窗形态不同，其他模块不受此管。
> 窗形态样板：`examples/kit/icon/`（已签字）；首个四件套模板窗：`examples/kit/divider/`。
> 立项 2026-11-16，窗形态改版 2026-11-17（官网页形态 + retained 脏区渲染）。

## 1. 窗形态（icon 窗样板，硬）

- **白底整窗、无壳**：无标题栏、无图例、无 HUD、无左右分区布局；整窗就是
  该组件的官方示例页——和 antd 官网组件文档页同构。
- **每段四件自上而下堆叠**：

```text
┌─ 白底整窗（滚动视口） ─────────────────────────┐
│ 段标题（13px 深色，抄官网 demo 标题）              │
│ 描述原文（11px 灰色，抄官网 demo 描述）            │
│ 怎么用：Go 调用一行（等宽代码块底）                │
│ [实物演示：真组件按该 demo 参数摆，禁色块]          │
│ （官网 demo 内的段落文字照抄，段间距用真实 Token）   │
│ …下一段…                                      │
└──────────────────────────────────────┘
```

- **效果说明**并入段描述：官网描述原文 + 「对了应该看到什么」写进窗 README
  对照表（§2），不上窗——窗上只保留官网页本来就有的文字。
- 长内容走**滚动视口**（滚轮/拖拽整页滚动，官网页面同款）；基线快照停滚动 0 位。
- 指标（fps/paint/skip/damage）**只采 JSON 不上屏**（wrgate report）。
- **渲染模式（硬）**：`PresentPolicyRetained` 保留/脏区策略——内容不变不触发
  重画（boundary_skip 只增、脏路径必录）；禁止 `PresentPolicyFullPaint` 持续
  全屏重画；门禁按静态窗判法（ACCEPTANCE 第二节：paint 稳住、skip 只增、
  rerecord 有脏必录），不按 60Hz 帧率门。

## 2. 证据与门禁

- **Golden 整窗逐位**：无壳后整窗内容都是组件页内容（说明文字随页入 Golden）；
  基线 `testdata/showcase_<名>_base.png`，0 容差，重播种人工过目留痕。
- **探针↔效果说明对照表进窗 README**（一段一行：段名 ↔ 探针名 ↔ 效果说明摘要）；
  说明写不到探针、探针验不到说明即打回。
- 逻辑探针用组件实例的只读访问器断言几何/状态（不依赖滚动位置，全段覆盖）；
  像素探针+Golden 覆盖滚动 0 位可见区。
- 文字全部真字：所有 `RenderText` 经 `wrkit.FaceAt` 挂 face（无 face 启发式
  路径 CJK 丢字、拉丁落等宽回退——divider 窗已踩过，禁止再犯）。

## 3. 说明 UI 件清单（只许用这些拼）

| 件 | 用谁 |
| --- | --- |
| 段标题/描述/段落文字 | `prim.Label`（挂 face，字号/颜色写死在窗侧官网页样式值） |
| 怎么用代码块 | 等宽：`prim` TextStyle.Family（theme `FontFamilyCode`）+ `ColorCodeBg` 底 |
| 面板底 | `prim.DecoratedBox`（底/边/圆角） |
| 长内容滚动 | `rendering.NewRenderViewport` + `NewScrollable`（滚轮/拖拽） |

说明所需基础能力缺了先补 prim/组件层，不许示例绕。

## 4. 回填清单（九窗，2026-11-17 更新为含窗形态改造）

已签字存量窗按本标准补「怎么用」+ **改造成无壳官网页形态 + retained 渲染**，
回填后 Golden 重播种（人工过目留痕）：

| # | 窗 | 回填内容 |
| --- | --- | --- |
| 1 | `examples/kit/icon/` | 形态已是样板（白底整窗）；补「怎么用」代码行 |
| 2 | `examples/kit/typography/` | 拆壳改官网页形态 + 补「怎么用」+ retained |
| 3 | `examples/kit/f0-scope/` | 同上 |
| 4 | `examples/kit/f0-layout/` | 同上 |
| 5 | `examples/kit/f0-decor/` | 同上 |
| 6 | `examples/kit/f0-content/` | 同上 |
| 7 | `examples/kit/f0-interact/` | 同上 |
| 8 | `examples/kit/f0-motion/` | 同上 |
| 9 | `examples/kit/f0-theme/` | 同上（三套主题 Golden 重播种） |

回填不改组件行为、不重摆实物演示；每窗回填完在 PROGRESS.md 对应行备注记一笔。

## 5. 新窗验收增项

- 新组件真窗（自 Divider 起）按本标准建；验收加两条：**四件套齐全**
  （缺「是什么/怎么用/效果说明」任一即挂）+ **retained 生效**
  （静态跑 paint 不涨、boundary_skip 只增，出现持续全屏重画即挂）。
- 窗 README 增两节：段模板对照说明（§1）+ 探针↔效果说明对照表（§2）。
