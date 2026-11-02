# P3.6 批 1c：图号=内容版本（播放与消费链，2026-11-02 交付，已提交 `4c313d47` 未推送）

直话：批 1b 只把动图解到 CPU，**显卡路径一帧都没走过**。本批把消费链打通，并按用户拍板 **D7=(B) 严格照 Skia** 把「图像内容身份」这套模型换成上游口径——**图号就是内容版本，像素变了就换号；以号为键的缓存注册监听、按号精确逐出自己那份旧纹理**。评估里推荐的 (A)「稳定图号 + 内容版本号」被否，理由接受：(A) 只满足三条标准里的一条，(B) 同时满足「对齐上游」，并且顺带把评估都没预料到的第四处洞（两族像素源撞号）一并消掉——(A) 修不到它。

拍板记录：**D7=(B)** / **D8=先修坑再接线，组件接线拆批 1d** / **D9=播放链独立第三扇真窗 `ui_wr_p36_animplay`** / **D10=批 1c 先于批 2**。

## 1 D5 反转（显式记账）

| 项 | 旧（D5，批 1b 已入库） | 新（D7=B） |
|---|---|---|
| 图号语义 | 对象身份（原地改写保持不变） | **内容版本**（原地改写必换号） |
| 动图逐帧 | 同 buf + `MarkPixelsDirty` + `ContentDirty` 整幅重写 | 同 buf + `NotifyPixelsChanged` + 监听器广播退役号 → 各家缓存逐出自己那份 |
| 条目/显存不涨的手段 | 键不变 | 变更时精确逐出（+ 可选的尺寸分桶回收） |
| 上游依据 | 我方自有模型 | Skia `SkPixelRef::notifyPixelsChanged` / `getGenerationID` / `addGenIDChangeListener` / `notifyAddedToCache` 原文口径 |

按「接口全量切换制」一次改到位、不留垫片：删 `ImageBuf.MarkPixelsDirty`/`TakeGPUDirty`/`IsGPUDirty`、删 `ImageDrawCommand.ContentDirty`、删 `ImageCache.rewriteImage`；批 1b 两条钉（同代际 + 脏旗标）改写成新契约（`image_anim_p36_test.go`、`export_imagebuf_gen_test.go`，后者由 `export_imagebuf_dirty_test.go` 改名）。

## 2 修掉的四处洞（全部有实测数字，缺一项就不动）

| # | 洞 | 定位 | 修复前实测 | 修复后实测 |
|---|---|---|---|---|
| A | 原地改写的脏旗标是一次性的、不可广播，同 buf 的第二个消费者（另一 session/另一扇窗）纹理永久冻结在第 0 帧 | `buf.go:309-317` 读后即清；`render_session.go:2681` 一 session 一家缓存；`context_image_gpu.go:111,122` 各取一次；可达路径 `ui/rendering/image.go:105 SetImageShared` | 三容器一致：`take#2=[false false false]`，cacheA `uploads=3` vs cacheB `uploads=1` | 新增钉 `TestImageCache_TwoConsumersBothReupload`：6 帧两家各 `uploads=6`、`entries=1/1` |
| B | GPU 图案采样场缓存键只混图号，动图代际恒定 → 逐帧同键命中，永远服务旧采样且不自愈 | `brush_field_cache.go:9-14` + `brush_native.go:556-585` | 逐帧像素 sha uniq=3/3、2/2、3/3，而 `patternFieldSeed` **uniq=1/N** | `patternFieldSeed` 逐帧 **uniq=N/N**（gif 3/3、apng 2/2、webp 3/3）；seed 已含图号，换代际天然分家 |
| C | 图像批合并谓词不比较脏标志，同 commit 内「画→推进帧→再画」时脏帧被干净组长吞 | `image_pipeline.go:1151-1183` + `render_session.go:4311-4319` | 谓词对 (clean 组长, dirty 组员) 实测返回 `true`（端到端后果当时未复现，按纪律只登记不动手） | 换代际后两帧是**两个不同图号**，谓词走既有 `GenerationID 不等 → false` 分支实测 `false`；同代际同几何仍 `true`（合批不退化）。新增钉 `TestImagePipeline_DifferentGenerationNeverMerges` |
| D | `Pixmap` 与 `ImageBuf` 各有独立计数器（都从 1 起）喂同一个缓存键空间 → 构造性撞号，同尺寸异来源命中同一张纹理，第二个来源的像素永不上屏 | `pixmap.go:31` + `buf.go:50` → `image_cache.go:62` | 各造 200 个：**交集 200/200**；同键不同像素两次 `GetOrUpload` → `misses 1→1, hits 0→1`，**返回同一纹理指针** | 单一分配器 `image.NextContentGenID`（`gen_watch.go`）：交集 **0/200**。新增钉 `TestImageCache_KeySpaceDisjointAcrossSources` |

附带发现（**登记不修**，与批 1c 不同根）：`ImageBuf.Fill` 是逐像素 `SetRGBA`，而 `SetRGBA` 每像素调一次 `InvalidatePremulCache`（拿一次 `premulMu`）→ 512×512 实测 **17.5ms/次**，是探针时间假象的真凶。全仓非测试代码零调用者（公开便捷口），按「标准能力零消费者也保留」不动，成本单独归位到 CPU 图像层待办。

## 3 机制实现（文件:行号）

- `render/internal/image/gen_watch.go`（新）：单一分配器 `NextContentGenID` + `GenWatch`（`Add`/`Fire`）+ `GenWatched` 接口（`GenerationID` + `AddGenIDWatcher`）。回调**不持锁**执行；写明与 Skia 的形态差异（Skia 一次一号，我方注册—注销长期有效，缓存条目插入时注册、删除时注销，覆盖同一窗口）。
- `render/internal/image/buf.go`：`genID` 由单一分配器发；`NotifyPixelsChanged` 换新号并在解锁后广播；`Dispose` 同样广播（号归 0）并清注册表；新增 `AddGenIDWatcher`；删 `gpuDirty` 与三个脏旗标方法；6 处构造点改 `NextContentGenID()`。
- `render/pixmap.go`：`genMu` 保护号与注册表（**顺带修掉既有隐患**：原先 `GenerationID()` 读与 `NotifyPixelsChanged()` 写完全无同步）；删 `nextPixmapGenID`；`FillRect` 的裸改号改走 `NotifyPixelsChanged`（一并广播）；新增 `AddGenIDWatcher`。
- `render/internal/gpu/image_pipeline.go` / `gpu_render_context.go`：`ImageDrawCommand` 删 `ContentDirty`、加 `Src GenWatched` + `SrcGen`（排队时当场取源号，因为像素是在那一刻读的）；四个 Queue 口全量加 `src` 参。
- `render/context_damage.go` + `render/{context_image_gpu,context_draw,m4_quadimage,vertices_atlas}.go` + gpu 侧 14 处调用点：同批全量切换；合成 payload（渐变条、shader 填充）显式传 `nil`，派生键（`pm.GenerationID()^常量`、`genBase+tileIdx`）传真实源。
- `render/internal/gpu/image_cache.go`：条目记 `srcGen`/`unwatch`；`GetOrUpload` 顶端 `drainRetired()`（**监听器可能来自别的线程，只往互斥保护的退役集里记号，缓存本体仍只由光栅线程改**，与 `ImageCache` 既有单线程契约一致）；插入后若源已领先一版则立刻自记退役（自愈）；`removeEntry`/`Destroy` 注销；`Stats` 加 `RetiredGens`/`Watched`/`Creates`。
- 三个原地改写用户切换：`render/internal/image/anim.go:publish`、`render/context_image_buf.go:69,86`（`ExportImageBuf` 复用）、`render/video_bridge.go:178,292,321`（视频 CPU 兜底）。

录制回放安全性（(B) 的最大疑问，先测掉再动）：录制命令存的是资源表里的 **buf 引用**（`recorder_canvas_ops.go:141-146`），回放 `backend.DrawImage(r.resources.GetImage(c.Image), …)`（`recorder.go:289-290`）→ 图号在回放当场重取，不会被钉在第一帧。

## 4 成本实测（512×512，native webgpu，×200 帧）

| 测项 | 数字 |
|---|---|
| 只 `WriteTexture`（旧 `rewriteImage` 的形态） | **848µs/帧** |
| 建+写+销（新模型的 miss 形态） | **684µs/帧** |
| 只建不写就销（纯 CreateTexture+View+Destroy） | **10.7µs/帧** |
| 命中（无变化） | 0.1µs/帧 |

结论：**换号模型相对原地重写没有可测的每帧退化**——每帧必付的是「把新像素送上显卡」这 0.85ms，建+销只占 1.3%。故本批**不接** `res.Cache` 尺寸分桶池（那是仓里已有的标准池实现，接它换来的收益是 10.7µs/帧，不值当在本批动热路径）；登记为后续优化项，口径写在这里以免被当成已做。

连播期间的资源形状（40×30 三容器 ×N 帧实测）：`Entries` 峰值 1、`usedBytes` 恒 4800、`hal.VramLiveBytes/Count` 不随帧数涨、双 `Destroy` 后归 0/0。

## 5 钉与门禁

正式钉（逐条对应 §1 表的实测数字，先红后绿）：

| 钉 | 位置 | 断言 |
|---|---|---|
| P1 键空间不重叠 | `render/internal/gpu/image_cache_generation_test.go:TestImageCache_KeySpaceDisjointAcrossSources` | 各造 200 个 `ImageBuf`/`Pixmap`，图号交集=0（修复前实测 200/200） |
| P2 原地改写=换号且可广播 | `render/image_anim_p36_test.go:TestAnimatedFrameAdvanceKeepsOneBuffer` | buf 指针不变、逐帧图号互不相同、每帧都向监听者报退役号 |
| P3 连播 N 帧四路平 | `TestImageCache_RetiredGenerationReclaimsEntry` | 连播 8 帧 `Entries`=1、`usedBytes` 不变、`Watched`=1、`RetiredGens`≥8；Dispose+排空后 0/0/0 |
| P4 多消费者不冻结 | `TestImageCache_TwoConsumersBothReupload` | 两缓存各 `uploads`=帧数（修复前第二家恒 1），各家 `Entries`=1 |
| P5 派生键随源号退役 | `TestImageCache_DerivedKeyReleasesWithSource` | `genID^常量` 形态条目在源号变更后被逐出（旧「不比脏」的吞脏路径连带闭合） |
| P7 批合并天然分家 | `TestImagePipeline_DifferentGenerationNeverMerges` | 异号不合并、**同号仍合并**（合批能力不退化） |
| P8 号不复用且单调 | `TestImageGenerationNeverReused` | 50 次改写号全不同且严格递增 |
| P9 批 1b 旧钉改语义 | `render/export_imagebuf_gen_test.go`（由 `_dirty_test.go` 改名）、`render/image_anim_p36_test.go` | 不留反向钉；`Dispose` 亦广播退役 |
| P11 池复用换号 | `TestPixmapPoolRecycleRetiresGeneration` | `pixmapPool` 回收给下一层时同一对象必换新号并广播 |
| P6 采样场不过期 | §1 表 B 行探针实测 | 动图逐帧 `patternFieldSeed` uniq=N/N（修复前 uniq=1/N 永久服务旧采样） |
| P10 存量金图零变化 | 主/补充逐文件回归 + 各真窗金图 | 与基线逐条同名单（下表） |

负向对照（`negative.txt`，全部红→还原绿）：**N1** 摘掉缓存监听注册 → P3/P4/P5 三钉红（8 帧后 `entries` 涨到 9；两消费者 6/6 变 6/1 语义失效；派生键剩 2 条），还原绿；**N2** `Pixmap` 用回独立计数器 → P1 红（交集 200），还原绿；**N3** 动图 `publish` 不回写号 → P2 红（退役号 0 个），还原绿。

门禁终局（冻结树一次跑完，日志均在本目录）：

| 门禁 | 结果 |
|---|---|
| `scripts/apidoc` | **rc=0**（`AddGenIDWatcher` 逐名登记，`apidoc.txt`） |
| `go vet` 触及五包（`./render`、`internal/image`、`internal/gpu`、`recording`、`ui/io`） | **全部无输出=干净**（`vet.txt`） |
| 定向 | `./render`、`./render/internal/image`、`./render/internal/gpu` 三包 ok（`targeted.txt`） |
| conformance 双后端 | **ok**（`conformance.txt`） |
| `-race -count=2` | `internal/image`、`render/recording`、`ui/io`、`internal/gpu` 四组 ok，`./render` 本批钉 ok（`race/race.txt`） |
| 五目标交叉编译 ×{默认,nogpu} | **10/10 绿**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64；`build/`、`build-gate.log`；口径同批 1b：排除 §6 三处存量坏包） |
| 主逐文件回归（34 包 731 条，分进程逐文件） | **PASS 701 / SKIP_TAGGED 21 / NO_TESTS 1 / FAIL 8**（`per-file-tests/summary.tsv`） |
| 补充逐文件（80 条） | **PASS 72 / SKIP_TAGGED 2 / FAIL 6**，6 枚与基线逐名相同（P3.1 线 raster 存量坏包） |
| 与批 1b 对账 | **零新增红**：8 枚 FAIL 逐条同测试、同报错首行（比对输出 `baseline-compare.txt`）；比存档基线**少两枚**——`ui/io/decode_t4_test.go`（批 1b 存档那份 summary 跑在修复之前，账本口径本就是修复后 9 枚）与 `render/s6_9_heavy_budget_test.go`（`H03_PathStrokeCloud` 帧时比 1.253 撞线，本次复跑三连绿，**判为负载抖动，不记作能力改善**） |

`go/ast` 主包公开面实测：**类型 151 · 顶层函数 192 · Context 方法 219 · PresentTarget 方法 18 · 常量 194 · 变量 28**，与批 1b 一致（本批新增是方法级 `AddGenIDWatcher`×2、删除是方法级三个，顶层计数不动）；`Canvas` 109 不变。

脚本与探针：`run-gates.sh`、`build-gate.sh`、`compare-baseline.sh`；探针归档 `probes/gpu_probe_test.go.txt`、`probes/pool_probe_test.go.txt`（scratch 不入库，留源码可复现）。

## 6 存量红与归属（非本批引入，实测对 HEAD 核过）

| 位置 | 现象 | 归属 |
|---|---|---|
| `examples/ui_wr_t_stress/main.go:38,39` | 引 `ui/kit/button`、`ui/kit/icon`，两目录不存在 → 包构建失败 | 组件线遗留（`git log` 最近触及 ce3c0f55，与本批无关） |
| `render/recording/backends/raster/backend_coverage_test.go:24` | `Backend.FillRect` 少 `render.BlendSpace` 参 | **P3.1 线** `76f8723f` 给契约加了 `space` 参，未同步该测试 |
| `examples/ui_render_base_recording/scene.go:470` | `recordingSpyBackend` 的 `FillPath` 少 `BlendSpace` → 不满足 `recording.Backend` | 同上 |
| `ui/scene/internal/gpupixel` | 无非测试 go 文件 | 其他线空目录（AGENTS：空目录不得标绿） |

`git show HEAD:` 逐条核过：以上四处在我动手前即如此，本批不顺手改（属别线交付面），已在此登记待归位。

## 7 显式不做与归属

- 组件/渲染层动图入口与节奏（`ui/rendering.RenderImage` + `ui/kit` 控制动词 + `ui/scheduler.Ticker`）→ **批 1d**（D8 拍板拆分）。批 1c 交付后动图仍是「已验证待接线」，生产消费者为零，不得标生产。
- 第三扇真窗 `ui_wr_p36_animplay`（D9 新增；design.md §3 已把「帧推进期间显存与条目数对账卡」从 mipmap 窗改到本窗）→ **批 3**。
- `res.Cache` 尺寸分桶池：本批按实测**不做**（收益 10.7µs/帧），登记为后续优化。
- 动图帧只改一小块却整幅 `WriteTexture`：容器本来就带 `DisposalRect`/帧矩形，差量上传是真正的收益点（0.85ms/帧那个大头），归后续专项，不在本批偷跑。
- `ImageBuf.Fill` 每像素加锁的成本：登记（§2 附带发现），不在本批改。
- GPU mipmap（批 2）：`MipmapMode` 全量切换与逐层上传仍在批 2；本批改的是身份模型，不动采样档，`rewriteImage` 只传 `MipLevel: 0` 那条限制随 `rewriteImage` 一起消失。
