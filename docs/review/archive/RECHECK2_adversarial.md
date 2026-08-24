# RECHECK2 对抗复核 —— 专门攻击 RECHECK 的三个改判

> 复核日期：2026-08-24。方法：假设改判本身错了，逐条找反证。结论只有三种：翻案维持 / 翻案有误恢复原判 / 部分修正。
> 所有行号均为本轮实测，非抄上一轮。

---

## 一、X03（光栅线程原地写共享层树）被判误报 —— **翻案维持**

RECHECK_p0.md:12/25 的翻案理由是「被写的 PictureLayer 每帧新建、pkt 走 channel」。我从三个方向攻击，全部没能击穿：

### 攻击① BuildLayerTree 真的每帧新建全部 PictureLayer 吗？有没有 retained/缓存复用分支？

**答：真的每帧全新建，找不到任何复用分支。**

- `BuildFramePacket`（`ui/rendering/layer_build.go:319-322`）每帧调 `BuildLayerTree(root)`；
- `BuildLayerTree`（layer_build.go:20-25）每帧 `scene.NewLayerBuilder()`；
- `NewLayerBuilder`（`ui/scene/build.go:14-19`）每帧 `NewContainerLayer()` 造新根；
- `AddPicture`（build.go:121-127）每次 `NewPictureLayer()`——`NewPictureLayer`（`ui/scene/layer.go:283-286`）就是 `&PictureLayer{...}` 字面量分配，无池化；
- 全 `ui/scene`、`ui/rendering` 目录 grep `sync.Pool / packetPool / layerPool` 零命中，无对象池；
- 层树里唯一带「缓存」字样的东西是 `CacheKey`（layer.go:265-268）和 BoundaryLayer，但 CacheKey 只是绑给 **纹理缓存**（PictureTextureCache）用的稳定键，**不是 layer 对象本身的复用**——下一帧还是新的 PictureLayer 结构体，只是键相同。

layer_build.go:12 注释说 builder 的 Root「跨帧按指针共享（COW）」，初看像翻案的漏洞。追下去：这个 COW 指的是同一帧内 builder→packet 共享根指针（build.go:138-141「no deep copy」），**不是跨帧复用上一帧的树**——下一帧 BuildLayerTree 又从 NewLayerBuilder 造新树，旧树无人引用。

### 攻击② textured.go:809/844 写 NeedsRaster/Valid 的对象从哪来？

**答：来自本帧新建、经 channel 提交的 pkt，写入只发生在光栅线程拿到 pkt 之后。**

- 生产侧：`pipeline_app.go:921` UI 线程 `BuildFramePacket` 造出新 pkt → `:941` UI 线程内 `RasterizeDirty(pkt)` 先清一轮标志 → `:1078` `SubmitLatest(job)` 经 channel 交给光栅线程；
- 消费侧：光栅线程 job 里 `:1262` `CompositeFramePacketTextured(pkt, ...)` 才是 textured.go:809/844 写 `pl.NeedsRaster=false` 的地方。Go channel 自带 happens-before，两侧不会同时碰同一个 pkt；
- overlay 带（overlay/state.go:209-251）同样每帧 `BuildOverlayBand` 全新建树再 AttachToPacket（pipeline_app.go:935），无跨帧对象。

### 攻击③ 若存在复用路径则 X03 复活——BoundaryCache 返回的 Picture 会不会被塞回层树？

**答：不会，这是最可能翻盘的地方，实测堵死。**

- 全仓库对 layer 的 Picture 赋值只有三处，全是 `ui/scene/layer.go` 自己的定义（`:308 SetPicture`、`:318 Record`），生产代码**零调用**——即没有任何代码把缓存里的 Picture 装进 PictureLayer；
- `BoundaryCache.Store`（`ui/rendering/boundary_cache.go:244+`）把 Picture 存进自己的 `entries` map；它的消费方 `tryReplay`（boundary_cache.go:180+）只在 **paint 走查时**（absolute.go:82、box.go:200）直接把缓存内容画到 DC 上，走的是直绘路径，根本不经层树；层树里 boundary 只是个空壳 `BoundaryLayer`（layer.go:322-330，无 Picture 字段）。

### 判定

三条攻击路线全部失败。X03 原判针对的「共享对象」不存在：被写的 layer 是每帧私有的，交接走 channel。真正跨线程共享的 RO 树脏标志和 BoundaryCache 已归 X02。**翻案维持，X03 保持摘除。**

---

## 二、X05（文字主路径绕过 shaping）被收窄为「仅位图主路」—— **翻案维持（补两处事实修正）**

### 攻击① 默认渲染档位（Auto）实际走哪条路？矢量路径只有显式调用才可达吗？

**答：默认确实走位图主路，但「矢量路径仅显式调用可达」这个隐含说法不准确——旋转/斜切文本在默认档会被自动路由进矢量分支。**

- `selectTextStrategy`（`render/text.go:535-564`）：Auto 时先查 `needsOutlineTransform()`（旋转/剪切/非均匀缩放，text.go:567 起）→ 命中即返回 TextModeVector；再查 `shouldUseGlyphMask()`（text.go:574-594）→ GPU 位图；否则留在 Auto 走 MSDF/CPU 位图；
- `dispatchText`（text.go:110-152）：GlyphMask/MSDF/Auto 三条位图支路的兜底全是位图引擎，只有 Vector 支路调 `drawStringAsOutlines` → `text.Shape`（text.go:1008 附近，shaper.go:58）；
- 全仓库（examples + ui）grep `SetTextMode` **零调用**——没人显式设档位，默认 Auto 就是生产档位。

所以准确的影响面是：**静止横平文本（UI 绝大多数）丢 shaping；旋转/斜切文本自动获得完整 shaping**。这比 RECHECK 写的「矢量/轮廓档（暗示显式）」覆盖面略大，但不改变结论方向。

### 攻击② DrawShapedGlyphs 是孤岛接口吗？

**答：不是孤岛，有生产消费者，但该消费者吃的数据本来就是整过形的——它不是绕 shaping 的受害链。**

- 唯一生产调用点：`render/scene/gpu_renderer.go:247`（`resolveText`，场景 TagText 回放）；
- 上游数据源：`Scene.DrawText`（`render/scene/text.go:456-461`）在编码成 TagText **之前**就调了 `text.Shape` 全整形，再把 ShapedGlyph 编码进场景文件；回放端 DrawShapedGlyphs 只是忠实消费这些已整形字形；
- 注意：`Scene.DrawText` 本身在 ui/examples 里也没找到生产调用者（场景文本编码链目前主要是录制/回放管线在用），所以这条「不绕的支路」离 UI 主画面更远。

### 判定

「部分成立」的定性**维持**：默认画面主体（静态位图文本）确实绕过 shaping，P0 定级合理；矢量分支与预整形接口真实存在且不绕。两处补充修正：① 矢量分支不止显式档，变换文本会自动进入；② DrawShapedGlyphs 有生产消费者（gpu_renderer.go:247），不算孤岛，但其上游 Scene.DrawText 已自带整形。**翻案维持，表述按上述两点加严。**

---

## 三、C4（默认 full_paint）被放宽为「分期设计非漏接」—— **翻案维持**

翻案依据是 pipeline_app.go:176/888 注释自述「W6 才全局默认 retained」。攻击点是查真源：当前进度到没到 W6？

### 核实 docs/ENGINE_UI_WIDGET_RENDER.md

- §5 分期表（:492-499 行区域）：**W0 ✅ · W1 ✅ · W2 ✅ · W3 ✅ · W4 🔄（C4 返工中）· W5 ⬜ · W6 ⬜**。W6 的定义 = 「R14、默认 retained + C9、C11 + 回归 C0–C5」，状态 ⬜ 未开始；
- §10 修订表末行（:642）：「W0 ✅ · W1 ✅ · W2 ✅ · W3 ✅。……默认 Present 仍 full_paint 至 W6」——真源自己就把 full_paint 默认声明为 W6 前的正常状态；
- §2.7 附近策略表（:102）：「retained：W2 窗可选（SetPresentPolicy）；W6 再作默认」。

### 判定

当前进度停在 W3/W4，离 W6 还差两个整期，「retained 尚非全局默认」完全符合分期计划，「分期设计非漏接」的辩护**成立**。翻案维持。

附一条预警（不改判定）：W6 一旦按 §5 宣称完成而默认仍是 full_paint，本条应立即恢复为问题并重新定级——这个改判是有保质期的，保质期到 W6 收口为止。

---

## 总表

| 条目 | 上轮改判 | 本轮对抗结论 | 关键证据 |
|---|---|---|---|
| X03 | 误报摘除 | **翻案维持** | build.go:121-127 每帧 NewPictureLayer、零池化；SetPicture 生产零调用；BoundaryCache 不回流层树 |
| X05 | 收窄为部分成立 | **翻案维持（两点加严）** | text.go:535-564 默认 Auto→位图；变换文本自动走 Shape；DrawShapedGlyphs 有消费者 gpu_renderer.go:247 |
| C4 | 分期设计非漏接 | **翻案维持（有保质期）** | 真源 §5：W6 ⬜ 未开始，当前 W3✅/W4🔄 |
