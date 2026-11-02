# 批 1d D18 实测：内容世代（图号）该不该进重绘边界的内容指纹

结论先说：**不该进**。D18 当初的前提（「不标脏的保留边界会静默重放上一帧」）实测不成立——
录制里的图像命令存的是缓冲**指针**，重放时按当前像素取图，所以画面天生跟手；把图号并进
指纹只会让烘焙了动图的边界**每帧作废重录**，实测有开销、零收益。因此本轮不改
`ui/rendering/boundary_cache.go` 的行为，只补回归钉。

## 1. 复现与定位

- 场景：`AbsoluteBox`（重绘边界）内挂一个非边界的 `RenderImage`，图像缓冲原地推进一帧
  （`Fill` + `NotifyPixelsChanged`，指针不变、图号变），全程不给渲染节点标脏。
- 判定路径：`BoundaryCache.tryReplay`（`ui/rendering/boundary_cache.go:295`）比对
  `contentKeyOf` → `absoluteContentKey`（:561）→ `imageContentKey`（:649），当前只看
  指针 + 状态 + 尺寸 + 占位色，看不到图号。
- 未修复前的第一版钉（断言「必须重录」）红在这里：
  `frame2 rerecord=0 want ≥1`——但同时**像素断言是绿的**，画面显示的正是新帧。这一条
  就是推翻 D18 前提的直接证据。

## 2. 三项实测

仪器：`probes/d18_boundary_rerecord_cost_test.go`（每帧新建 400×400/700×700 光栅上下文，
逐帧原地推进 + 不标脏；`-count=1`）。两个指纹各跑一遍，A/B 对照。

| 树形 | 帧数 | 只看指针 | 图号进指纹 | 结构差 |
|---|---|---|---|---|
| 400×400，边界内 24 色块 + 1 动图 | 200 | avg 4.45 / 4.58 ms，best 3.81 ms | avg 4.66 / 4.68 / 4.87 ms，best 3.92–3.98 ms | rerecord/frame 0.00 → **1.00**，skip 1.00 → 0.00 |
| 400×400，边界内 12 行文字 + 1 动图 | 120 | avg 24.78 ms，best 21.53 ms | avg 24.94 ms，best 21.13 ms | 同上；差异落在噪声内（**该仪器被光栅化主导，无效**） |
| 700×700，边界内 25 子节点 | 150 | avg 8.97 ms，best 7.67 ms | avg 9.14 ms，best 7.70 ms | +1.8% |
| 700×700，边界内 200 子节点 | 150 | avg 12.84 ms，best 11.13 ms | avg 13.66 ms，best 11.38 ms | **+6.4%** |

读法：两种情况下「光栅化绘制」都只走一遍；差别是多出来的**录制 pass**（整棵边界子树再
描述一遍 + 分配 Recorder 操作 + 旧条目垃圾）。子节点越多越贵，且动画期间重绘边界对动图
内容彻底不再起作用。

## 3. 「换缓冲不标脏」有没有可达通路（D18 的另一个假设）

全仓 `ui/ engine/ examples/` 扫下来，渲染节点上的图像缓冲只能从
`SetImage` / `SetImageShared` 进来，两者都按 R10 契约标脏（`ui/rendering/image.go:105`
等）；`RenderImage.Img` 字段没有任何引擎层直接赋值点（唯一命中是
`examples/ui_wr_c10_soak/main.go:137` 的读）。也就是说指针级指纹的盲区没有可达的错误画面
通路，剩下的只是「万一以后有人绕开 setter」的假设场景。

## 4. 反向对照（证明钉真的会红）

- NC-A｜把图号并进指纹（`imageContentKey` 加 `k ^= gen * 0x9E3779B97F4A7C15`）→
  钉红：`frame2 rerecord=1 skip=0 want rerecord=0, skip≥1`。像素那条依旧绿，再次印证
  画面跟手与指纹无关。
- NC-B（**无效对照，如实记录**）｜第一次改的是 `recordOwnContent` 的 `*RenderImage` 分支
  （:703），这棵树走的是 `recordAbsoluteOwnContent` 的子节点分支，变异没生效，钉仍绿。
- NC-B2（有效）｜改 `recordAbsoluteOwnContent` 的图像分支为
  `drawImageOn(dc, t.Img.Clone(), ...)`（录制时存像素快照）→ 钉红：
  `frame2 pixel=(35,35) RGBA{0xdc,0x1e,0x1e} want the new (green) frame`。
  这正是钉要守的方向：哪天录制改成快照像素，第一帧之后就会红。

## 5. 落地

- 行为不改：`imageContentKey` 保持 HEAD 原样（`git diff` 为空）。
- 只加防护：`ui/rendering/boundary_cache_contentgen_test.go` →
  `TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame`，同时钉两件事——画面跟最新帧、
  这一帧仍走重放（rerecord=0 / skip≥1），外加第三条「换缓冲必须重录」的对照。
- 待用户确认 D18 收回后，动图的正确失效通路按 D19 走：推进帧时由组件标脏，不靠指纹。
