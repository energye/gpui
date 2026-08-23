# P1 复核报告（批 A 并发类 + 批 B 算错结果类）

> 复核方式：逐条回源码读实现，判定「成立 / 误报 / 部分成立」。日期：2026-08-23。
> 行号为复核当日实际行号。

## 总表

| 编号 | 原结论 | 判定 | 证据（文件:行号） | 备注 |
|---|---|---|---|---|
| A1 | acquire 超时恢复有洞，hung 下 reconfigureThrottled 受 500ms 限流可能连续失败、无升级策略 | 成立 | gpu/webgpu/swapchain.go:946(minInterval=500ms)、953-954(限流直接报错)、1009-1021(hung 门每 ≥500ms 才试一次 reconfigureThrottled；失败仅返回 ErrAcquireTimeout) | hung 路径与限流共用同一 lastReconfig/pendingReconfigure，无重试计数、无放弃重建/降级策略；若 Configure() 持续失败则每帧空转。属真实恢复洞 |
| A2 | 图层 ID 生成非原子 | 成立 | ui/scene/layer.go:21-30(`layerIDGen++` 普通 uint64 自增，NextLayerID 无锁无 atomic) | 注释自认 "not concurrency-hardened beyond atomic-ish P2 use"。当前 UI 单线程使用下不炸，但作为公共 API 是隐患 |
| A3 | HbShaper 竞态：XScale 共享变量 | 成立 | render/text/shaper_hb.go:67-68(Shape 内直写 `hbf.hbf.XScale/YScale`)、40-44(mu 只护 faces map)、110-118(getOrCreateHbFont RLock 返回共享 *hbFont) | 同一 FontSource 不同 size 的两个 goroutine 同时 Shape 会互踩 XScale，且 hb.Buffer 的 Shape 也并发作用于同一 hbf。mu 完全没盖住这段 |
| B1 | clip 忽略绕向，交点两两配对按 even-odd | 成立 | render/internal/clip/mask.go:199-224(AA 版 sortFloats 后 `i += 2` 两两配对)、291-325(非 AA rasterizeScanline 注释写 non-zero 却同样两两配对 even-odd)；edge.dir 字段(175/190/192)算出后从未用于配对 | dir 字段是死代码。自交多边形/双叠子路径在 clip mask 下会出镂空错误 |
| B2 | Path.Reset 不清 boundsValid | 成立 | render/path.go:178-185(Reset 不动 boundsValid)、169-177(Clear 正确置 false)、90-97(expandBounds 首点只在 !boundsValid 时重置 min/max) | Reset 复用后旧 bounds 残留，新路径 Bounds() 会并入旧坐标范围 → Fill/Stroke damage(trackDamage(path.Bounds()), context.go:1162/1175)虚大 |
| B3 | Path.Append 不合并边界 | 成立 | render/path.go:187-196(Append 直接 append verbs/coords，只搬 other.current/start) | other 的 boundsValid/boundsMin* 完全未并入 p，拼接后 Bounds() 缺 other 部分 |
| B4 | damage 不含描边外扩 | 成立 | render/context.go:1162、1175(Stroke 前 trackDamage(c.path.Bounds()))；path.Bounds() 是几何 AABB(path.go:115-125)，无 lineWidth/2、miter、dasher 外扩；doStroke/doStrokePreserve 无补充 damage | 粗描边场景 OS 合成器增量 present 会留残影 |
| B5 | focal 渐变忽略 StartRadius | 成立 | render/gradient_radial.go:129-192(computeTFocal 全文只用 EndRadius 解射线-圆交，189-191 gradientT=pointDist/intersectDist 未映射 StartRadius)；对比 computeTSimple:120-125 用了 StartRadius | focal 情况下 t=0 不落在 StartRadius 边界，起始色带错位 |
| B6 | CPU 渐变不受 CTM | 成立 | render/software.go:531、597、984(paint.ColorAt 设备像素坐标 x+0.5/y+0.5)；paint.go:188-198 ColorAt 直通 Brush.ColorAt；brush.go:160-167 brushPattern 直通；Paint.SetBrush(paint.go:156-165)/SetFillBrush(context.go:905-915) 均不注入逆 CTM；paint 上只有 TransformScale(context.go:2413) 用于 dash/线宽，无数标变换 | 渐变几何坐标按用户空间给、采样却用设备空间，任何 scale/translate 变换下渐变跟着设备坐标走而非图形走，与 Skia/Cairo 行为不符 |
| B7 | bidi 半套：压平 0/1 且段序不重排 | 成立 | render/text/segment.go:80-84(level 只有 0/1，RTL run 全标 1，嵌套层级丢失)；layout.go:276-281(段内 RTL 重排 ReorderRTLShapedGlyphs，但 runs 保持逻辑顺序 createLine:315-337 顺序拼接，无 UAX#9 L2 视觉级 run 重排) | 混排段落（如英文夹 RTL）整行顺序仍是逻辑序，RTL 段应整体移位/镜像的视觉规则缺失；嵌套 embedding 也全被拍平 |
| B8 | IME delete_surrounding 被丢 | 成立 | ui/platform/wayland_textinput_linux.go:411-426(delete_surrounding 编码为 IMEKind=2、IMEStart=-before 负数)；ui/input/fromplatform.go:39-52(原样透传)；ui/textinput/editor.go:345-348(IMECaretMove 分支要求 `Start>=0 && End>=0`，负 Start 静默丢弃) | 平台层注释承诺"Start<0 为删除请求"，editor 层没有对应分支，协议请求整条被吞 |
| B9 | preedit 字段错位 | 成立 | ui/platform/wayland_textinput_linux.go:380-395(preedit_string(text, commit, index)：index=caret 字节偏移被塞进 IMEStart，commit 标志(uint32 0/1)被塞进 IMEEnd) | editor.caretFromIME(editor.go:353-359) 把 End 当 caret 读（End≥0 取 End）——而 wayland 送来的 End 是 commit 标志恒 0/1；caret 信息实际在 Start 里没人读。字段语义完全拧了 |
| B10 | 可变字体 GPU 路径丢 variations | 成立 | render/internal/gpu/glyph_mask_engine.go:673-697(rasterizeGlyph 只收 parsed/gid/size/hinting，调 RasterizeHinted/RasterizeAliased 无 variations 参数)；render/text/glyph_mask_rasterizer.go:167-176(RasterizeHinted→ExtractOutlineHinted，非 Var 版本)；gpu_text.go:200-204(MSDF GlyphKey{FontID,GlyphID,Size} 无 varHash)+210(ExtractOutline 非 Var)；gpu_text.go:430-442(computeFontID 只哈希字体名+NumGlyphs，不含 variations) | 对照 CPU 路径 draw.go:260-261 走 ExtractOutlineHintedVar 带 gvar；GPU 两条管线（glyph-mask 与 MSDF）都拿默认实例轮廓，wght=700 画出来是 regular。shape 层缓存键倒是有 varHash(shape_result_cache.go:344)，但光栅化层丢了 |
| B11 | scene CPU tile 丢 alpha | 成立 | render/scene/renderer.go:717-720(TagPushLayer `_ = alpha`，注释自认 TODO per-layer alpha blending)、722(TagPopLayer TODO) | PushLayer 的 alpha 直接扔掉，CPU tile 回放时图层不透明度失效 |
| B12 | recording 回放丢字体/clip | 部分成立 | recorder.go:173-178(DrawTextCommand 回放传 face=nil，注释"Font face lookup would need additional handling")、recorder.go:143-159(Save/Restore/SetClip 有回放转发)；backends/raster/backend.go:119-131(SetClip 只设 path+fillRule，注释承认 render.Context 无 Clip 方法，实际没有调用任何裁剪生效接口)、backend.go:134-137(ClearClip 空函数体) | 「丢字体」成立：face=nil 落到 backend.DrawText(nil)。「丢 clip」要拆开看：命令录制/回放分发层(recorder.go:149-158)没丢 SetClip，但 raster 后端的 SetClip/ClearClip 是空转实现——clip 在最终渲染确实不生效。综合判部分成立（问题真实存在，但位置在后端实现层而非回放层） |
| B13 | 缓存指纹弱哈希族 | 成立 | ui/rendering/boundary_cache.go:420-443(textContentKey 手写 k*31 累加且 `i<64` 截断前 64 字节，长文本共享前缀即同 key)；render/scene/encoding.go:752-800(Encoding.Hash 逐字段 FNV 但 brushes 切片不在哈希范围内——tags/pathData/drawData/textData/transforms 四类，drawData 只存 brushIdx 序号)；render/scene/text.go:155-163 与 render/text.go:1010-1016(scene/CPU glyph cacheKey 用 OutlineCacheKey{FontID,GID,Size,Hinting}，未填 VariationHash 字段——该字段存在但三处生产代码均无人赋值) | 三处都核实：①textContentKey 确实 64 字节截断+弱乘法哈希；②Encoding.Hash 确实不含 brushes 内容（改刷子颜色不改 hash）；③glyph 缓存键确实漏 variations。注意 boundary_cache 在 ui/rendering 包不在 render/scene |

## 判定与原结论不同的条目展开

### B12 recording 回放丢字体/clip —— 部分成立

原结论把两件事捆在一起说「recorder.go:177,185; backend.go:118-136」，回源码后要拆开：

**丢字体：成立。**
`render/recording/recorder.go:173-178`：

```go
case DrawTextCommand:
    brush := r.resources.GetBrush(c.Brush)
    // Font face lookup would need additional handling
    backend.DrawText(c.Text, c.X, c.Y, nil, brush)
```

录制的 DrawTextCommand 回放时 face 恒传 nil，注释自己承认没做。StrokeTextCommand(:179-183) 同样 nil。raster 后端拿 nil face 后文字要么画不出要么走默认字体，回放结果和原始绘制不一致。

**丢 clip：问题真实，但不在回放分发层，在后端实现层。**
回放分发本身没丢：`recorder.go:149-158` 对 SetClipCommand / ClearClipCommand / ClipRoundRectCommand 都有转发，测试(recorder_test.go:462-471)也验证了 SetClip 命令会被录制。

真正的问题在 raster 后端：
- `render/recording/backends/raster/backend.go:119-131` SetClip 只把 path 塞进 ctx 并设 FillRule，注释写"render.Context doesn't have a direct Clip method, so we use ClipPreserve behavior by setting path and letting fill/stroke respect it"——但后续 FillPath(:139-148) 会 `b.ctx.SetPath` 覆盖掉这条 clip path，所以 clip 从未真正生效；
- `backend.go:134-137` ClearClip 是空函数体，注释承认没做。

结论：现象（回放后 clip 不生效、文字丢字形）成立；定性为「回放丢 clip」不准确，准确说法是 **SetClip 命令有录制有分发，但 raster 后端没有可用的裁剪落地通道**。修的位置应是后端（或给 render.Context 补 Clip 能力），不是 replay 循环。
