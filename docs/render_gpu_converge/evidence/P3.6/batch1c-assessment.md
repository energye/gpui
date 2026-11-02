# P3.6 批 1c 影响面评估：动图播放与消费链（2026-11-02，方向待拍板）

直话：批 1b 只把动图做到「解码器交付帧 i 的像素 + 元数据」这一层，**GPU 消费链一步没走过**。评估实测发现：D5 拍的「稳定代际 + 原地重写」这条路上有三处底层洞，其中**两处已用探针复现出数字化后果**（多 session 冻结、Pattern 采样场永久过期），第三处是合并谓词的契约缺口（谓词实测为 `true`，触发需要一帧内改写时序）。好消息是 D5 想要的核心性质——**连播 N 帧缓存条目与显存不增长**——实测成立，只是还没有钉。

探针文件（scratch，未提交，正式钉落地后删除）：`render/internal/gpu/zz_p36c_probe_test.go`
复现：`WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go test ./render/internal/gpu/ -run '^TestP36C_Probe$' -count=1 -v`

## 1 实测数据（2026-11-02，本机 linux-amd64，940MX，native webgpu）

三个容器各跑一遍（`gif_restore_prev.gif` 3 帧、`apng_plays3.png` 2 帧、`webp_anim_lossy.webp` 3 帧，画布均 40×30）：

| 观测量 | gif | apng | webp | 说明 |
|---|---|---|---|---|
| 逐帧 `GenerationID` | 全同（1 个值） | 全同 | 全同 | D5 契约成立 |
| 逐帧像素 sha256（前 6 字节） | 3/3 互不相同 | 2/2 | 3/3 | 内容确实在变 |
| **`imageCache` Entries / usedBytes** | **1 / 4800 恒定** | 1 / 4800 | 1 / 4800 | ✅ 组 4 要的「条目不涨」 |
| `hal.VramLiveBytes/Count` | 0→9600 / 0→2（两个缓存各一张） | 同 | 同 | ✅ 一帧一张，不随帧数涨 |
| Destroy 后 | 0 bytes / 0 count | 0/0 | 0/0 | ✅ 资源回收干净 |
| 单消费者连播 uploads | **3**（=帧数） | 2 | 3 | ✅ 每帧真重传 |
| **第二消费者连播 uploads** | **1**（停在首帧） | 1 | 1 | ❌ 洞 A |
| **逐帧 `patternFieldSeed`** | **uniq=1/3** | uniq=1/2 | uniq=1/3 | ❌ 洞 B |
| `canMergeImageDraw(clean 组长, dirty 组员)` | **true** | true | true | ❌ 洞 C |

## 2 三处洞（机制 + 行号 + 复现）

### 洞 A：原地重写的脏标志是一次性的，第二个消费者永远看不到
- 机制：`ImageBuf.TakeGPUDirty()`（`render/internal/image/buf.go:309-317`）**读后即清**；`ImageCache` 是 per-session（`render/internal/gpu/render_session.go:2681` 一 session 一家）。绘制入口每次自行取一次（`render/context_image_gpu.go:111`、`:122`）。于是同一 `ImageBuf` 被两个 session 画时，第一家拿走 dirty，第二家每帧拿到 `false` → `GetOrUpload` 走纯命中分支（`image_cache.go:239-250`）不重传 → **它的纹理永久停在第一次上传的那一帧**。
- 实测：探针里 `take#2 = [false false false]`，cacheB `uploads=1` 而 cacheA `uploads=3`（三容器一致）。
- 可达性（不是假想场景）：`ui/rendering/image.go:105 SetImageShared` 就是公开支持的「一 buf 多节点共享」；两扇窗、或「窗口 + 离屏 picture target」各有一 session。多显示器/缩略图墙同一动图必然踩。
- 归属：**批 1b 之前就存在**（`MarkPixelsDirty` 的注释写明它是给 `ExportImageBuf` 复用建的，`buf.go:294-296`；`image_cache.go:276` 同），动图只是第一个必然触发的消费者。按 AGENTS「洞修在引擎层」，归批 1c。

### 洞 B：GPU 图案采样场缓存的键只认代际，不认内容
- 机制：`brushFieldCacheKey{seed, geom, w, h}`（`render/internal/gpu/brush_field_cache.go:9-14`），`seed` 混的是 `img.GenerationID()` + 子矩形 + 变换 + opacity（`render/internal/gpu/brush_native.go:556-585`）。动图代际恒定、几何恒定 → **seed 逐帧全同**（实测 uniq=1/N）→ `fieldCache.get` 命中直接返回上一帧的 `ColorAt` 采样字节（`brush_field_cache.go:26-36`，条目原地复用、容量 8，所以既不涨也不会被淘汰掉）。
- 后果：动图作为 `ImagePattern` 源做 GPU 填充时，画面冻结在第 0 帧且**不会自愈**（与洞 A 的「一次 commit 内可自愈」不同级）。
- 归属：同洞 A，属 D5 设计留下的口径盲区——我方所有 GPU 侧缓存都以代际为**内容身份**，而 D5 恰好把代际降级成了**对象身份**。

### 洞 C：图像批合并谓词不看 `ContentDirty`，脏帧可能被同代际的干净组长吞掉
- 机制：`canMergeImageDraw` 比较 GenerationID/采样档/opacity/viewport/尺寸（`render/internal/gpu/image_pipeline.go:1151-1183`），**不比较 `ContentDirty`**；而 flush 时只有组长 `cmds[i]` 进 `GetOrUpload`，组员并进同一次 multi-quad draw（`render/internal/gpu/render_session.go:4311-4319`）。
- 触发形状：同一次 commit 内「先画 → 推进一帧 → 再画同一张」→ 前者 clean 当组长、后者 dirty 被并进去，`PixelData` 无人消费；本 commit 画面少一帧，下一个 commit 自愈。
- 实测边界：谓词返回值 `true` 已实测（探针末行）；**「脏内容真被丢掉」这条端到端后果尚无像素级复现**，故按 AGENTS「缺复现不动手」，本项只登记 + 待补一条定向复现钉，不预先改行为。

## 3 上游口径对照（这是拍板的关键分歧点）

- **Skia：原地改像素 = 换代际 + 通知缓存清理**。`SkPixelRef.h` 原文：`getGenerationID()`「Returns a non-zero, unique value corresponding to the pixels in this pixelref. **Each time the pixels are changed (and notifyPixelsChanged is called), a different generation ID will be returned.**」；`notifyPixelsChanged()`「Call this if you have changed the contents of the pixels」；并有 `addGenIDChangeListener()`「This can be used to invalidate caches keyed by SkPixelRef generation ID」与 `notifyAddedToCache()`「allows the cache to know automatically those entries can be purged when this pixelref is changed or deleted」。（本机 Skia 未随 flutter 树 vendored，取上游头文件原文；消费者侧的 mutability 三态 `kMutable/kTemporarilyImmutable/kImmutable` 同文件。）
  → 也就是说 Skia 的「缓存条目不涨」不是靠**键不变**，而是靠**变更时精确逐出旧条目**。
- **Flutter 动图路径：每帧一个新位图 + 一个新图像对象**，不做原地重写：`lib/ui/painting/multi_frame_codec.cc:59-113` 每帧 `SkBitmap bitmap` 新 `tryAllocPixels` → `GetPixels` → `:147-166` 起新 `DlImage`/Impeller 纹理，靠 Dart 侧 dispose + `SkiaUnrefQueue` 回收。
- **我方 D5（已拍且批 1b 已钉）**：`*AnimatedImage` 复用同一 `ImageBuf`、代际恒定 + `MarkPixelsDirty` 原地重写（`render/internal/image/anim.go:369-372`），批 1b 已把它钉进 `render/image_anim_p36_test.go:290-298`（同 buf、同 genID、逐帧置脏）。理由是真源实测：预算硬门 128 条/64 MiB（`render/budget.go:136`），逐帧新代际会让条目与显存单调增长。

分歧就一句话：**Skia 把「内容版本」编码进代际 ID，再用监听器逐出旧条目；D5 把「内容版本」编码进一个独立标志，但标志是 one-shot 的、不能广播。** 三个洞全部出在后者的「不能广播」上。

## 4 候选方案（待拍板 D7）

- **(A) 内容与身份分离：`ImageBuf` 加单调 `contentStamp`，消费者各记 lastStamp**（推荐）
  - `MarkPixelsDirty` 改成 `genID` 不变 + `contentStamp++`；`TakeGPUDirty` 换成 `ContentStamp()`；`ImageDrawCommand` 带 stamp，`ImageCache` 条目记 `lastStamp`（≠ 就 `rewriteImage`）→ 洞 A 结构性闭合（N 个 session 各自独立判定，互不偷）。
  - `fieldCache` 键混入 stamp（洞 B 闭合）；`canMergeImageDraw` 加 stamp 比较（洞 C 连带闭合，组长组员 stamp 不同即不合并）。
  - 代价：批 1b 那条「同 genID」钉**不变**（代际仍是身份），新增的是版本维度；公开面 +1 查询口（`ImageBuf.ContentStamp()`）；`gpuDirty` 的既有消费者（`context_image_gpu.go:111,122`、`image_cache.go:239-253`、`export_image_buf` 复用路）按接口全量切换制一次改完，不留 shim。
  - 与上游的关系：**语义对齐**（原地改写必须可被任意数量的消费者察觉，正是 Skia `notifyPixelsChanged` + listener 要实现的目的），**机制按我方缓存模型选**（版本号 vs 换 ID + 监听器）。理由：换 ID 路线要把 listener 挂到每一家以代际为键的缓存（imageCache、fieldCache、后续 atlas），并把批 1b 的「同 genID」硬钉整条推翻，收敛面反而更大。
- **(B) 严格照 Skia：动图逐帧 `NotifyPixelsChanged()` 换代际 + 建 `genID` 变更监听器，各缓存按通知逐出旧条目**
  - 最贴上游；但要新写一套 listener 注册/派发 + 挂接所有代际键缓存，且**直接推翻 D5 与批 1b 已交付的公开契约与钉**（`anim.go:369-372`、`image_anim_p36_test.go:290-298`），预算 128 条在 60fps 下要靠逐出精度才不涨。工作量大一个数量级。
- **(C) 只做动图专属补丁（如给 `AnimatedImage` 单独加"每帧重画"入口绕开共享缓存键）**
  - 写下来是为了明确**不选**：这就是 AGENTS 禁止的「在分层上打补丁绕过底层口径」，洞 A/B 还在。

## 5 批 1c 范围（待拍板 D8/D9/D10）

- **必做**：GPU 端到端消费链的正式钉（组 4 升格：连播 N 帧 Entries/usedBytes/`VramLiveBytes`/`VramLiveCount` 四路不涨 + 每帧真重传 + Destroy 归零，双后端）；洞 A/B 的修复与其定向复现钉；洞 C 的复现补测（复现成立才改行为）；`-race`（动图帧状态跨线程 + 多 session 共享）。
- **待拍 D8**：是否本批就把**三支柱生产接线**点亮——`ui/rendering.RenderImage` 加动图入口 + `ui/kit` 侧控制动词（节奏走 `ui/scheduler.Ticker`，`ui/scheduler/ticker.go:26`、`ui/scheduler/scheduler.go:491`；引擎不持节奏，与 Flutter 把播放放 framework 一致，design.md §5 已登记）。做＝批 1c 变两半；不做＝批 1c 只算「已验证待接线」，生产消费仍为零。
- **待拍 D9**：真窗归属。design.md §3 把「帧推进期间显存与条目数对账卡」写进了 **mipmap 窗** `ui_wr_p36_mipmap`（批 3）。若批 1c 独立成能力，按 AGENTS「每主能力独立真窗」可能要第三扇 `ui_wr_p36_animplay`；否则并进 `ui_wr_p36_imgfmt` 的帧序列卡。
- **待拍 D10**：与批 2（GPU mipmap）的先后。交集实测在 `rewriteImage` 只传 `MipLevel: 0`（`image_cache.go:302`）而创建钉死 `MipLevelCount: 1`（`:362,377`）——批 2 若先做，动图原地重写必须连带重产 mip 链；批 1c 先做则批 2 直接覆盖这条。

## 6 门禁（沿用批 1a/1b 全套）

apidoc rc=0、`go vet` 触及三包 rc=0、定向 `-count=1`、`CGO_ENABLED=1 -race -count=2`、conformance 双后端、五目标构建 ×{默认,nogpu}、逐文件回归只减不增（新增红必须 clean HEAD A/B 归因）、存量金图零变化。
