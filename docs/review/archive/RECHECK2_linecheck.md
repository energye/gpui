# 行号机械核对报告（RECHECK2 · linecheck）

> 日期：2026-08-24。核对对象：`docs/review/RECHECK_SUMMARY.md`（终版台账）+ `docs/review/REPAIR_PLAN.md`（修复计划）中出现的全部「文件:行号」引用。
> 核对方法：逐条用 sed 抽行 + grep 交叉搜索（NewLayerCache 调用方、KeepAlive 计数、`.scale` 写点、clipPath 消费方等），比对「该行附近代码」与「文档所述行为」是否相符。
> 判定口径：**一致** = 行号命中且行为相符；**漂移** = 行号偏了但附近能找到所述代码（给出实际行号）；**失配** = 所述代码不存在或行为不符。

## 一句话结论

**共核对 32 条独立引用（去重后）：一致 26 条、漂移 5 条、失配 1 条（歧义型）、无法核对 1 条（文档没写文件名）。唯一失配是 X05 的 `text.go:1008`——仓库里有两个同名 text.go，台账没写包路径，按 `ui/rendering/text.go` 字面去读就落空（该文件只有 785 行），真正命中的是 `render/text.go:1008`，代码确实存在且行为相符。所以这不是结论可疑，是引用不够精确。**

## 二、全量核对表

### 来自 RECHECK_SUMMARY.md（终版台账）

| # | 引用 | 所述行为 | 判定 | 实际情况 | 备注 |
|---|---|---|---|---|---|
| 1 | gpu/rwgpu/adapter.go:556-571 | stringViewToString 零拷贝 | **一致** | :570 `unsafe.String((*byte)(...), int(sv.Length))` 直接把 C 内存包成 Go string | X01 核心 |
| 2 | gpu/rwgpu/adapter.go:533-546 | 先拷贝四个字段后立即 free | **一致** | :533-543 四个 stringViewToString，:546 procAdapterInfoFreeMembers.Call | free 后 string 仍指 C 内存 |
| 3 | gpu/rwgpu/adapter.go:593-594 | 回调路径复用同一函数 | **一致** | :593-594 callbackStringView 直接 return stringViewToString(...) | 补充证据属实 |
| 4 | （全文件无 KeepAlive） | 无存活保障 | **一致** | grep -c KeepAlive = 0 | 交叉验证 |
| 5 | ui/embedder/pipeline_app.go:1199 | 光栅闭包调用的保留树绘制入口 | **一致** | :1198-1201 PaintPresentLayerTree 定义处，:992 Run 闭包内间接执行 | X02 链第一环 |
| 6 | ui/embedder/pipeline_app.go:997 | 提交给光栅线程的任务闭包 | **一致** | :991-999 raster.FrameJob{Run: func()...} | X02 链第二环 |
| 7 | ui/raster/loop.go:60-84 | 光栅线程 goroutine 循环 | **一致** | :60 `go func()` 起 :84，:64 for-select 消费 jobs | X02 链第三环 |
| 8 | ui/rendering/boundary_cache.go:27-48 | 无锁跨线程读写结构 | **一致** | :27 struct 定义，普通 map+int64 字段，无任何 mutex | X02 第二半 |
| 9 | ui/embedder/pipeline_app.go:656-659 | UI 侧清脏标志位置 | **一致（弱）** | :658 `a.pipe.FlushLayout(vp, true)` 是清脏入口；真正的置 false 在 ui/rendering/pipeline.go:125/:189 | 引用的是入口行 |
| 10 | render/path_boolean.go:62-100 | 逐像素采样 + 2048 截断 | **一致** | :62 maxDim=2048 静默夹断，:71-77 双重 Winding 逐像素扫描 | X04 |
| 11 | render/path_boolean.go（签名） | 无填充规则参数 | **一致** | op switch 里没有 EvenOdd/NonZero 区分，Winding 单一规则 | X04 附注 |
| 12 | text.go:1008 | 矢量档走 text.Shape 完整 shaping | **失配（歧义型）** | `ui/rendering/text.go` 仅 785 行且无任何 Shape 调用；实际命中 **render/text.go:1008** `shaped := text.Shape(s, c.face)` | 见第三节展开 |
| 13 | ui/rendering/layer_build.go:20 | 全树重建入口 | **一致** | :20 BuildLayerTree 定义，每次帧全量 walk | X06 |
| 14 | ui/rendering/layer_build.go:222 | 每层重建时设 CacheKey | **一致** | :222 pl.SetCacheKey(base.EnsureCacheID())（addOwnPicture 内） | X06 |
| 15 | ui/rendering/layer_build.go:242 | 同上（另一分支） | **一致** | :240-242 baseOf 失败时 SetCacheKey(0)，同样每帧重录 | X06 |
| 16 | NewLayerCache 零生产调用 | ui/rendering 层缓存死代码 | **一致** | layer_cache.go:32 定义后全仓零生产调用方；render/scene 另有一套同名缓存（cache.go:70，测试在用） | 与台账「同名另一套非死代码」说法吻合 |
| 17 | gpu/webgpu/surface.go:312 | PresentWithDamage 收了矩形但不用 | **一致** | :312 参数名 `_ []image.Rectangle`，:313 直接转调 s.Present(st) | C1；注释自认 wgpu-native 不支持 |
| 18 | ui/scene/ 下两文件（C5） | 此前写成 ui/rendering 是笔误 | **无法核对** | 台账没给文件名和行号，只能确认 ui/scene 包存在（LayerBuilder/PictureLayer 等） | 建议后续修订时补文件名 |
| 19 | raster 后端 SetClip「只塞不生效」、ClearClip「空函数体」（B12） | clip 洞 | **一致** | render/recording/backends/raster/backend.go:119-130 SetClip 只把路径塞进 ctx 且注释自认没有真 Clip 方法；:133-136 ClearClip 函数体为空 | 台账没写行号，本次补上 |
| 20 | 默认 full_paint 属分期设计（C4） | W6 才全局默认 retained | **一致** | ui/embedder/pipeline_app.go:235「Default remains full_paint until W6 makes retained the global default.」 | 定性修正有代码背书 |
| 21 | ui/platform/wayland_linux.go:1377 | scale 字段只读不写 | **一致** | :1377 `scale float64` 字段声明；grep `.scale` 全文件只有 :1489/:1492 两处读，零写点 | N1 |
| 22 | ui/platform/wayland_linux.go:715 | 同上佐证 | **漂移（弱）** | :715 只是 `host := &wlHost{win: win}` 构造处，没初始化 scale；结论成立主要靠「全仓无写点」 | 行为沾边但不是缩放逻辑本体 |
| 23 | ui/platform/x11_linux.go:404 | 缩放恒为 1 | **一致** | :404 `scale: 1` 硬编码 | N1 |
| 24 | ui/scheduler/scheduler.go:100-116 | vsync 监听 goroutine 无退出 | **一致** | :101 `go func()`，:102 for 无限循环，无 quit/select 退出分支，:113 出错还 time.Sleep 继续 | P2 新增 |
| 25 | ui/platform/wayland_clipboard_linux.go:469-505 | Get 最长阻塞 3 秒 | **一致** | :469 readOffer，:485 deadline=now+3s，:494-499 EAGAIN 忙等轮询（2ms sleep）直到超时 | P2 新增 |

### 来自 REPAIR_PLAN.md（修复计划）

| # | 引用 | 所述行为 | 判定 | 实际情况 | 备注 |
|---|---|---|---|---|---|
| 26 | adapter.go:574-588 | cStringAt 是真拷贝（改照此办） | **一致** | :574-588 边界扫描后 `string(b[:n])` 生成新 Go string | 1.1 修法参照物有效 |
| 27 | adapter.go:593-594（callbackStringView） | 修复需覆盖回调路径 | **一致** | 同 #3 | 重复引用 |
| 28 | tmp_vlprobe、render/tmp_ref_stroke | 探针目录待删 | **一致** | 两目录当前都存在（ls 实测）；tmp_vlprobe 编译失败与台账 X06 补充吻合 | 1.2/1.3 前提成立 |
| 29 | shaper.go:7 | 注释说默认 OwnShaper，实为假注释 | **一致** | :7 写「OwnShaper ... (default, ADR-048)」，:21 实际 `defaultShaper = NewHbShaper()`；SetShaper(:29) 也写「reset to the default OwnShaper」 | 1.4 改口必要性属实 |
| 30 | RENDER_API_CATALOG §2 布尔行 | 写「自交路径也支持」与事实冲突 | **一致** | docs/RENDER_API_CATALOG.md:70 该行原文如此；EvenOdd 必错则此话不实 | 4.4 改口必要性属实 |
| 31 | pipeline_app.go:1199（ConsumeNeedsPaint） | 清脏标志消费点 | **漂移** | 实际调用在 **:1280**（presentPacketTextured 内，:1235 起的函数）；:1199 附近是 PaintPresentLayerTree | 偏约 80 行，代码存在 |
| 32 | cache/shaping.go（3.2 架子已在） | ShapingCache 已有架子 | **漂移（路径笔误）** | 实际在 **render/text/cache/shaping.go**（含 _test/_bench）；「cache/shaping.go」少了前缀 | 文件存在，路径不全 |
| 33 | context.go:1162/1175 | Fill/Stroke 的 damage 上报缺线宽外扩 | **漂移** | :1162/:1175 只是 Fill/Stroke 的注释行；实际上报点是 **:1167/:1180** 的 `c.trackDamage(c.path.Bounds())`——用路径包围盒、不含描边外沿，描述的问题真实存在；线宽外扩逻辑本体在 expandStrokeToPathSpace（context.go:2511 处调用） | 偏 5 行，问题成立 |
| 34 | render/path.go:178-185 | Reset 未清 boundsValid | **一致** | :180-185 函数体恰好没有 `boundsValid = false`（对比 ：174-175 Clear 里有） | 5.1 前提属实 |
| 35 | render/path.go:189-196 | Append 未合并边界 | **一致** | :189-196 只搬 verbs/coords/current/start，全程未碰 bounds | 5.1 前提属实 |
| 36 | gpu/webgpu/surface.go:312 | 同 #17 | **一致** | 同 #17 | 重复引用 |
| 37 | examples/ui_wr_c2_retained_scene/main.go:63-67 | 门禁硬编码几何常数 | **一致** | :63-66 sumRatio() 里 `100*100 + 3*hotSize² + 2*picBox² + 1200*72` 全是写死的数 | 5.3 属实 |
| 38 | render_session.go:5368-5373 | MSAA 中帧 pass 用 LoadOpLoad 致 UB | **漂移** | :5368-5373 命中的是 encodeSubmitSurfaceGrouped 函数定义头（属中帧提交路径没错）；loadOp 决策本体在 **:4827-4831**（`if s.frameRendered { colorLoadOp = LoadOpLoad }`），:4902 注释自认「Subsequent mid-frame flushes will use LoadOpLoad」 | 偏约 540 行，问题成立 |

### 统计

| 判定 | 条数 | 占比 |
|---|---|---|
| 一致 | **26** | 81% |
| 漂移 | **5** | 16%（#22 弱相关、#31 偏80行、#32 路径笔误、#33 偏5行、#38 偏540行） |
| 失配 | **1** | 3%（#12 歧义型，结论仍成立） |
| 无法核对 | 1 | 文档未给文件名（#18） |

## 三、失配条目展开：X05 的「text.go:1008」

**现象**：台账写「矢量档 text.go:1008 走 text.Shape 完整 shaping」。仓库里有三个 text.go：

| 文件 | 行数 | 有无 Shape 调用 |
|---|---|---|
| ui/rendering/text.go | 785 | 无（grep `\.Shape(` 零命中） |
| render/text.go | 1143 | :1008 `shaped := text.Shape(s, c.face)` |
| render/scene/text.go | 587 | 无 |

按字面最可能的理解（渲染层 widget 文本 = ui/rendering/text.go）去查，:1008 落空、整个文件也没有 shaping 调用——这就是「失配」。换到 render/text.go:1008，则完全命中：GPU 矢量文本路径在这里调 `text.Shape` 做完整 shaping，并用 OutlineCacheKey 建轮廓缓存。

**初步判断：这是「引用不带包路径」造成的伪失配，不是结论可疑。** X05 的核心论断（位图主路 MSDF/GlyphMask 无 shaping、矢量档有 shaping、默认 Auto 档走位图主路所以仍是 P0）不受影响，:1008 这个证据本身是真实的。建议后续修订台账时把这类引用补全为 `render/text.go:1008`，避免下一个人按同名文件白查一趟。

## 四、其他说明

1. **漂移不影响任何结论**：5 条漂移全部是「行号偏了、代码还在、行为相符」（最大偏 540 行，最小偏 5 行），其中两条（#31、#38）偏得比较远，动手修之前按本表实际行号找即可。
2. **B12 本次补上了精确位置**：render/recording/backends/raster/backend.go:119-130（SetClip 塞路径但不产生真裁剪，注释自认）+ :133-136（ClearClip 空函数体）。台账原话「有录有发、洞在后端」与代码吻合。
3. **核对边界**：本轮是静态读码比对「代码形态」与「文字描述」是否相符，不含运行时复现（如 -race、真窗实测）；X01/X02 的「危害性」判断依赖调用链推断，本轮只验证了链路每一环的代码确实存在且形态与描述一致。
4. X03（误报摘除项）与 C5 的具体文件名在文档里没有行号级引用，前者无可核对象（摘除正确与否依赖 X02 已核实的链路），后者已在上表标注「无法核对」。
