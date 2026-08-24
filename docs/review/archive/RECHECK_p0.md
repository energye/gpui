# P0 六项结论复核报告（RECHECK）

> 复核方式：逐条回源码定位（原审查行号有偏差的已重新定位），只读码不改码。
> 复核日期：2026-08-23。判定口径：**成立**（问题属实）/ **部分成立**（主体属实但表述或范围要修）/ **误报**（不成立）。

## 总表

| 编号 | 原结论 | 复核判定 | 关键证据（文件:行号） | 备注 |
|---|---|---|---|---|
| X01 | stringViewToString 用 unsafe.String 引用 C 内存未拷贝（use-after-free） | **成立** | gpu/rwgpu/adapter.go:556-571（:570 显式长度分支返回指向 C 内存的字符串）；消费点 adapter.go:533-543；:546 立即 procAdapterInfoFreeMembers 释放；对照真拷贝版 cStringAt :574-588 | 全 rwgpu 无一处 KeepAlive 保护这几个字段；且回调路径 callbackStringView(:593-594) 也复用此函数，同样暴露 |
| X02 | 光标线程闭包清 UI 脏标志 + BoundaryCache 无锁，跨线程竞争 | **成立** | ui/embedder/pipeline_app.go:1199（ConsumeNeedsPaint 在 draw 闭包内）→ :997 SubmitLatest → ui/raster/loop.go:60-84（LockOSThread 光栅协程执行 job.Run）；UI 侧写者 node.go:343-347 MarkNeedsPaint / :310 MarkNeedsLayout（普通 bool 无原子）；BoundaryCache 无锁 map：ui/rendering/boundary_cache.go:27-48；UI 线程 Clear：pipeline_app.go:656-659（EventResize）；raster 侧 tryReplay/Store：box.go:200/217、absolute.go:82/108（都在 paint 闭包=光栅线程跑） | pipeline.go:176-182 只保护 owner.paintDirty 一个字段，逐节点 needsPaint 是裸 bool |
| X03 | ui/scene/rasterize.go 原地写共享层树字段无锁 | **误报（按原文表述）** | 写入点确实存在：ui/scene/rasterize.go:50-53、ui/scene/textured.go:809/844；但被写的 PictureLayer 每帧新建：ui/rendering/layer_build.go:20（NewLayerBuilder）+ :218/:238 AddPicture，RecordInto 每帧重录（picture.go:405-413 清空重录）；pkt 经 channel（SubmitLatest，loop.go:128-152）交给光栅线程，channel 自带 happens-before | 对象不跨帧共享，两侧也不并发碰同一个 pkt（UI 线程统计都在提交前 ：861-865）。真正共享的是 RO 树和 BoundaryCache——那已归入 X02 |
| X04 | 布尔运算是逐像素采样近似、2048 静默截断、恒按 NonZero | **成立** | render/path_boolean.go:62-68（maxDim=2048 直接砍边界，无告警）；:70-100 双重循环逐像素 `Winding()!=0` 判内外；函数签名只有 (a,b,op)，全程无填充规则参数 | EvenOdd 路径必错：自交图形按奇偶规则该挖空的区域，NonZero 口径判成实心（绕向同向时），结果多出一块 |
| X05 | 文字主路径绕过 shaping（只有 cmap+advance） | **部分成立** | 绕过属实：render/internal/gpu/gpu_text.go:179 `face.Glyphs(s)` → render/text/face.go:160-220 iterGlyphs 仅 cmap+advance；GlyphMask 引擎同理走 text.LayoutGlyphs（shape_result_cache.go:375-379 注释自认「no GSUB/GPOS」）；**但有例外**：轮廓/矢量分支 drawStringAsOutlines → textOutlinePath → text.Shape（render/text.go:1008）走全局 shaper（text/shaper.go:58-72），有完整 shaping；DrawShapedGlyphs（text.go:210）吃外部预整形结果 | 准确说法：GPU 位图两条主路（MSDF + GlyphMask，含默认 Auto 档）绕过 shaping；矢量/轮廓档和显式预整形接口不绕。连字/kerning 在位图主路上确实丢 |
| X06 | 每帧全树重建 + rendering.LayerCache 死代码 | **成立** | 每帧重建：ui/embedder/pipeline_app.go:843 BuildFramePacket → layer_build.go:20 新建 builder → :222/:242 每叶无条件 RecordInto 重录；NewLayerCache（ui/rendering/layer_cache.go:31）全仓库 grep 仅定义处+文档，零生产调用；唯一引用方 tmp_vlprobe/main.go:76 调用不存在的 owner.LayerCache()，go vet 实测仍编译失败（本轮复现） | 注意撞名：render/scene/cache.go:70 另有一个 NewLayerCache(maxSizeMB)，测试在用，是另一套东西；死代码特指 ui/rendering 这份 |

## 逐条大白话说明

### X01 内存洞 —— 成立
`stringViewToString` 的显式长度分支（adapter.go:570）就是拿 `unsafe.String` 包了一下 C 内存指针，一个字节都没拷贝。调用方 `Adapter.Info()` 把这四个字符串填进结构体后，紧接着 ：546 就调 `procAdapterInfoFreeMembers` 把 C 内存还回去了，之后谁再读 `info.Vendor` 这些字段就是在读已释放内存。同文件的 `cStringAt`（:574-588）才是正确做法——先 `unsafe.Slice` 再 `string(...)` 真拷贝；作者会写对的版本，只是这条分支漏了。rwgpu 其他地方到处都是 `runtime.KeepAlive`，唯独这里没有任何时序保护，实锤。

### X02 线程竞争 —— 成立
链路完整坐实：`pipe.ConsumeNeedsPaint()`（pipeline_app.go:1199）写在 retained 帧的 draw 闭包里，这个闭包经 ：997 `SubmitLatest` 进了 `ui/raster/loop.go` 的专用 OS 线程执行；而 UI 线程上事件/ticker 回调随时可能调 `MarkNeedsPaint`（node.go:343-347）写同一个节点的裸 bool——一边清一边写，没有原子也没有锁。BoundaryCache 更直接：entries 就是个普通 map（boundary_cache.go:27），光栅线程在 paint 闭包里 tryReplay/Store（box.go:200/217），UI 线程在 EventResize 里 Clear（pipeline_app.go:658），并发读写 map 可以直接 fatal。唯一加了锁的 `owner.paintDirty` 只是冰山一角。

### X03 共享缓存条目原地写 —— 误报（按原文表述）
写入代码确实存在（rasterize.go:50-53、textured.go:844 都会把 `NeedsRaster=false`、`Valid=true`），但被写的这些 PictureLayer **不是跨帧共享的**：`BuildLayerTree` 每帧 `NewLayerBuilder` 全新建树（layer_build.go:20），叶子每帧重新 RecordInto，上一帧的对象下一帧就没人引用了。pkt 从 UI 线程交到光栅线程走的是 channel（SubmitLatest），Go 的 channel 自带内存同步保证。所以「光栅线程原地改共享数据」这个说法不成立——真正跨线程裸奔的是 RO 树脏标志和 BoundaryCache，那两处已经算在 X02 头里了。建议把 X03 从 P0 里摘掉或并入 X02 表述。

### X04 布尔假货 —— 成立
三点全部属实：① ：70-100 就是一个 y 外层 x 内层的双重循环，每个像素点问一次 `Winding()!=0`，纯逐像素采样近似，输出全是 1 像素高的矩形条；② ：62-68 超过 2048 直接把边界砍掉，没有任何告警或返回值提示，大路径右侧/下侧内容静默丢失；③ 函数签名里根本没有填充规则参数，一律按 NonZero 口径判内外。EvenOdd 路径必错：比如一个顺时针自交的五角星类图形，奇偶规则中间应该镂空，NonZero 判出来是实心，布尔结果凭空多一块。

### X05 文字主路径绕过 shaping —— 部分成立
原结论对了一多半但要加限定：MSDF 引擎（gpu_text.go:179 `face.Glyphs`）和 GlyphMask 引擎（glyph_mask_engine.go:190/197 走 `text.LayoutGlyphs`，注释自己写着「cmap + advance; no GSUB/GPOS」）这两条 GPU 位图主路确实完全不做 shaping，连字、kerning、阿拉伯变形全丢。但仓库里有不绕的分支：矢量/轮廓路径 `textOutlinePath` 调的是 `text.Shape`（text.go:1008 → shaper.go:58），走完整全局 shaper（GSUB/GPOS 都在）；`DrawShapedGlyphs` 也支持外部先整好形再画。所以准确说法是「位图主路绕过、矢量档不绕」。考虑到默认 Auto 档优先走 GPU 位图，生产画面上的文字主要还是受影响的那条路，P0 定级维持合理。

### X06 每帧重建 + LayerCache 死代码 —— 成立
两个说法都对上了：① `BuildFramePacket`（layer_build.go:317）每帧调 `BuildLayerTree` → `NewLayerBuilder()` 新建整棵树，且每个叶子 ：222/:242 无条件 `RecordInto` 重录显示列表（RecordInto 会清空旧 ops 重来，picture.go:405-413），所谓 COW 只是共享个根指针；② `ui/rendering/layer_cache.go` 的 `NewLayerCache` 全仓库只有定义和文档提到，零生产调用者；唯一想用它的人 tmp_vlprobe/main.go:76 调的方法根本不存在，`go vet` 到今天还是红的（本轮实测复现）。有个容易踩的坑要说明：render/scene 包里还有个同名 `NewLayerCache(maxSizeMB)`，那是另一套东西、测试在用——死代码特指 ui/rendering 这一份增量构建缓存。

## 结论速览

- **维持 P0**：X01、X02、X04、X06（四条完全成立）。
- **维持但改表述**：X05 改为「GPU 位图主路（MSDF/GlyphMask）绕过 shaping，矢量/轮廓档走完整 shaper」。
- **建议降级/并入**：X03 属误报（写入对象每帧新建、channel 交接自带同步），其有效成分（RO 树脏标志、BoundaryCache 跨线程访问）已在 X02 中覆盖。
