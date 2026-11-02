# P3.6 批 1c 方案：图号=内容版本（D7 拍板 = 严格照 Skia）（2026-11-02）

拍板记录（用户 2026-11-02，见 `batch1c-assessment.md` §4 的候选）：**D7=(B) 逐帧换图号 + 变更监听器**，不采纳我方推荐的 (A) 内容版本号；**D8=先修坑再接线，组件接线拆批 1d**；**D9=播放链独立第三扇真窗 `ui_wr_p36_animplay`**；**D10=批 1c 先于批 2**。

## 0 D5 反转（必须显式记账，不能口头改）

- **被推翻的**：design.md §1.4「动图载体复用同一 `ImageBuf`、代际恒定 + `MarkPixelsDirty` 原地重写」，以及 §1.5 之后落地的 `ContentDirty`/`rewriteImage` 通道。
- **改成的**：图号（`GenerationID`）就是**内容版本**——像素变了就换号（Skia `SkPixelRef::notifyPixelsChanged` 口径）；「连播 N 帧缓存条目与显存不增长」这条目标**不降级**，但达成手段从「键不变」换成「变更时精确逐出 + 尺寸分桶回收」。
- **反转的代价（如实写明）**：批 1b 已交付并入库的公开契约与钉要改语义——`render/image_anim_p36_test.go:290-298`（同 buf 同 genID 逐帧置脏）、`render/export_imagebuf_dirty_test.go:9,32-43`（genID 稳定 + `IsGPUDirty`/`TakeGPUDirty` 一次性）；`AnimatedImage` 的 doc 行 `anim.go:190`（"MarkPixelsDirty rewires one texture per animation instead of one per frame"）要重写；catalog §3.5 与台账要同步删/改口。按「接口全量切换制」同一次提交改到位，不留 shim。
- **为什么用户选 (B) 也站得住**：三条标准口径里 (A) 只满足「双后端可验证」，(B) 同时满足「对齐上游设计」；且 (B) 顺带把评估里没预料到的第四处洞（键空间重叠，见 §1）一并消掉——(A) 修不到它。

## 1 实测抓到的第四处洞（键空间重叠，(B) 的额外收益）

`Pixmap` 与 `ImageBuf` **各有独立计数器**：`render/pixmap.go:31` `nextPixmapGenID`、`render/internal/image/buf.go:50` `nextImageBufGenID`，都从 1 起、都喂同一个 `ImageCache.entries` 键空间（`render/internal/gpu/image_cache.go:62`，注释自称按 `Pixmap.GenerationID()` 键，ADR-014）。

探针实测（`TestP36C_ProbeKeySpace`，scratch 文件 `render/internal/gpu/zz_p36c_probe_test.go`）：各造 200 个 → `ImageBuf 图号 uniq=200，Pixmap 图号 uniq=200，两者交集=200`（**完全重叠**）；同键同尺寸不同来源喂进一个缓存 → `misses 1->1, hits 0->1, entries 1->1`，**两次返回同一纹理指针**——第二个来源的像素永远上不了显卡，画出来是第一个来源的画面。这与动图无关，是既有正确性洞（离屏 Pixmap 与加载图同尺寸即踩），随本批的身份模型统一一并修。

## 2 机制（四件事）

### 2.1 单一图号分配器
`render/internal/image` 出一份进程级单调分配器，`ImageBuf` 与 `Pixmap` 共用 → 键空间不再重叠。上游对照：Skia 全局 `SkGenerateID()` 单一分配器，`SkPixelRef::getGenerationID()` 从中取号。
- 派生键保留：`brush_fill.go:146`（`genID ^ 0xC0A7_C01A_0000_0001`，注释已自认在躲碰撞）、`brush_advanced.go:127`（`genBase + tileIdx`）、`brush_native.go:838`。派生号仍可用，但**缓存条目必须记住"我是从哪个源号派生的"**（见 2.3），否则监听器无法反推要逐出谁。

### 2.2 原地改写 = 换号，dirty 通道退役
- `AnimatedImage.publish()`（`render/internal/image/anim.go:369-372`）：`MarkPixelsDirty()` → `NotifyPixelsChanged()`。
- 另外两个原地改写用户一并切：`render/context_image_buf.go:69,86`（`ExportImageBuf` 复用）、`render/video_bridge.go:178,292,321`（视频 CPU 兜底帧）。
- **退役**：`ImageBuf.MarkPixelsDirty/TakeGPUDirty/IsGPUDirty`（`buf.go:296,309,321`）、`ImageDrawCommand.ContentDirty`（`image_pipeline.go:125-127`、`gpu_render_context.go:1420,1468`）、`ImageCache.rewriteImage`（`image_cache.go:277-320`）与 `GetOrUpload` 的 dirty 分支（`:242-244`）。留 `MarkEphemeral`（genID=0 的单帧纹理，`buf.go:284`，particle 示例在用，与新模型正交）。
- **连带自动闭合**：洞 A（第二消费者冻结）——每帧新号，每家缓存各自 miss 各自上传，没有可被偷走的一次性标志；洞 C（合并吞脏）——同 commit 内「画→推进帧→再画」变成两个**不同图号**，`canMergeImageDraw` 现有的 `a.GenerationID != b.GenerationID → false`（`image_pipeline.go:1160`）天然不合并。两条都要写钉把"自动闭合"钉成"断言闭合"。
- 录制回放安全（这是 (B) 的最大疑问点，先实测掉）：录制 op 存的是**资源表里的 buf 引用**（`render/recording/recorder_canvas_ops.go:141-146` `Image: r.resources.AddImageBuf(img)`），回放 `backend.DrawImage(r.resources.GetImage(c.Image), c.Opts)`（`render/recording/recorder.go:289-290`）→ `tryGPUDrawImage` 当场读 `img.GenerationID()`（`render/context_image_gpu.go:112,123`）。**图号不是录制时定死的整数**，换代际不会把回放钉在第一帧。

### 2.3 图号变更监听（照 Skia `addGenIDChangeListener`）
上游原文（`SkPixelRef.h`）：`getGenerationID()`「Each time the pixels are changed (and notifyPixelsChanged is called), a different generation ID will be returned」；`addGenIDChangeListener()`「This can be used to invalidate caches keyed by SkPixelRef generation ID」；`notifyAddedToCache()`「allows the cache to know automatically those entries can be purged when this pixelref is changed or deleted」。
- 我方形态：`internal/image` 出监听原语（注册返回注销函数，回调带**旧号**），`ImageBuf` 与 `Pixmap` 共用同一原语（不写两份）。`Dispose()` 也发一次（`buf.go:91-114` 把 genID 归零），让缓存立刻甩掉这条来源的注册与条目。
- **API 面**：`render.ImageBuf` 是类型别名（`render/context_image_decode.go`/`context_image_buf.go`），任何导出方法都直接进公开面。监听口以 **`Pixmap` 导出方法 + `internal/image` 包级函数**双形态落地（包级函数不随别名暴露给 render 公开面），公开新增数按实测写 catalog，跑 apidoc 校验。
- 与 Skia 的一处口径差异（写明不藏）：Skia 的监听器是**一次一号**（「you must add a new listener for each generation ID change」），我方按**注册—注销**长期有效、条目删除时注销。效果等价（回调恰好覆盖条目存活期），但形态不同，须在台账标为有意分歧而非"照抄"。

### 2.4 尺寸分桶空闲池（把 D5 想要的性能性质拿回来）
逐帧换号后若不做回收复用，等于每帧 `CreateTexture`+`Destroy`（940MX 上明显是回归，且撞「性能红线」）。
- `GetOrUpload` 插入时注册监听；监听回调 → `removeEntry`（走既有 `retire` 延迟释放：等本次提交完成，`image_cache.go:88-96` 注释即此语义）→ 纹理按 `(w,h,format)` 进**空闲池**；`uploadImage`（`image_cache.go:353-389`）优先从池取同尺寸纹理整幅 `WriteTexture`，取不到才 `CreateTexture`。
- 上游对照：延迟释放＝Flutter `UnrefQueue::Unref`/`DeleteTexture` 批量延后 drain（`engine/src/flutter/flow/skia_gpu_object.h:30-54`，与 `drain_delay_` 一起）；分桶回收＝Skia `GrResourceCache` 按 size/config 分桶复用（Flutter 动图路径每帧新位图新图像对象、不做原地重写，见 `lib/ui/painting/multi_frame_codec.cc:59-113,147-166`，靠这条回收链路兜住显存）。
- **空闲池必须计入预算与显存账**（`usedBytes`、`hal.VramAdd/VramForget`），否则「条目不涨」是假的——池里的纹理是真显存。预算压力（`evictIfNeeded`）要能把池榨干。

## 3 钉（三证据合证，先红后绿）

| # | 钉 | 断言 |
|---|---|---|
| P1 | 键空间不重叠 | 各造 N 个 `ImageBuf`/`Pixmap`，两号集交集=0（探针数字转正式钉） |
| P2 | 原地改写=换号 | `DecodeFrame(i)` 逐帧 `GenerationID` 互不相同；buf 指针不变；premul 缓存失效 |
| P3 | **连播 N 帧四路平** | `Entries`、`usedBytes`、`hal.VramLiveBytes`、`VramLiveCount` 恒定；**且池稳定后 `CreateTexture` 次数=0**（计数器/包装口实测） |
| P4 | 多消费者不再冻结 | 同一动图两个 `ImageCache`：各 `uploads=N`、逐帧像素读回/对参照一致（旧行为下第二家 `uploads=1`，探针实测） |
| P5 | 派生键按源号逐出 | `brush_fill` XOR / `brush_advanced` tile 派生条目在源号变更后被清 |
| P6 | 采样场缓存不涨 | 动图作 `ImagePattern` 连播，`fieldCache` 条目 ≤ 8 且逐帧服务新采样（旧行为 seed uniq=1/N 永久过期，探针实测） |
| P7 | 批合并天然分家 | 同 commit 内「画→换帧→再画」两条命令不同号 → `canMergeImageDraw=false`、两次上传都发生（洞 C 闭合） |
| P8 | 号不复用 | 换号后旧号再也取不到条目（单一分配器单调），旧纹理不逃逸（`Destroy`/压测后 `VramLive*` 归零） |
| P9 | 批 1b 既有钉改语义 | `image_anim_p36_test.go`、`export_imagebuf_dirty_test.go` 按新契约改写（不许留反向钉） |
| P10 | 存量金图零变化 | 本批不改像素输出；`video_bridge`/`ExportImageBuf` 两个迁移用户各跑一遍既有像素证据 |

负向两路（红→还原绿）：①摘监听注册 → P3/P4 应红（条目涨/第二家冻结）；②摘空闲池 → P3 的 `CreateTexture` 计数应红。
门禁全套沿用批 1a/1b/1c 评估 §6：apidoc、vet 触及三包、定向、`-race -count=2`（监听表跨线程 + Dispose 与光栅线程竞争，`buf.go:20-27` 的锁序 mu→premulMu 必须保持）、conformance 双后端、五目标×{默认,nogpu}、逐文件只减不增、金图零变化。

## 4 实施切分（每步可独立验证，禁止半新半旧）

1. 单一分配器 + 监听原语（P1、注册/注销表本身）；
2. 原地改写换号 + dirty 通道退役（三用户全量切换 + `ImageDrawCommand`/`gpu_render_context` 签名同步 + P2/P9）；
3. 缓存按源号逐出 + 尺寸分桶空闲池 + 预算/显存记账（P3/P4/P5/P8）；
4. `fieldCache`/派生键复核 + 洞 C 闭合钉（P6/P7）；
5. 文档收口：catalog（删 3 口 + 新增监听口 + `ImageBuf` 语义变更）、design.md §1.4/§1.5 反转行、台账批 1c 状态与修订行、`batch1c.md` 证据与日志；
6. 全套门禁 + 提交前停报告（含 `video_bridge`/`ExportImageBuf` 跨模块连带面）。

批 1c **不含**组件接线（D8 拆批 1d）与真窗（D9 的 `ui_wr_p36_animplay` 归批 3，design.md §3 需新增该窗并把「帧推进期间显存与条目数对账卡」从 mipmap 窗改到该窗）。
