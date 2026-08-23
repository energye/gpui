# R3 审查：ui/rendering + ui/embedder 分层架构

> 依据：`docs/ENGINE_FLUTTER_SKIA_ARCH.md`、`docs/ENGINE_ARCH_OVERVIEW.md` 两轮深入调查 + 本轮 5 条关键线索源码复核（文件:行号均已核实）。语言为大白话。

## 一句话直话结论

**骨架是真的 Flutter 味，能跑能测，但现在这套分层撑不起工业级组件库**——文档承诺的三大件（Layer COW 复用、默认 retained 合成、compositor-only 动画）一个都没真正兑现；更严重的是存在跨线程裸奔数据竞争和两条绘制路径结果不一致的隐藏 bug。补齐 TOP5 之后有戏，现在不行。

---

## 维度一：架构对齐度（诚实性）

总评：**骨架对、承诺虚**。RO 树 / RepaintBoundary / RelayoutBoundary / 双线程提交（UI 建 packet、raster 线程 SubmitLatest）/ vsync 节奏都是真的；但三个 F 承诺打折：

1. **F01「Layer COW，禁全树 deep copy」——名不副实（P0）**
   - `ui/rendering/layer_build.go:10-20` 注释说 "Root is shared by pointer across frames (COW)"，但 `BuildFramePacket` 每帧调用 `BuildLayerTree` → `scene.NewLayerBuilder()` 新建整棵层树，且 `addLeafPicture`/`addOwnContent` 每帧无条件 `RecordInto` 重录所有叶子 Picture 的 ops（layer_build.go:243、260-299）。所谓 COW 只是"共享指针"，实际是**每帧全树重建 + 全量重录显示列表**，CPU 端 O(树) 成本每帧都付。
   - 专为修这个问题写的增量 `LayerCache`（`ui/rendering/layer_cache.go`）是**死代码**：全仓库无生产调用者；唯一引用方 `tmp_vlprobe/main.go:76` 调用已不存在的 `owner.LayerCache()` 和 `BuildFramePacketCached`，`go vet` 直接编译失败（本轮已实测复现）。注释里自己承认 "Without this, BuildLayerTree allocated ~4-5MB/s allocation"，却没接上。

2. **F02/F04「静态层纹理复用」——存在但要手动开（P1）**
   - 默认 present policy 是 `full_paint`（`ui/embedder/pipeline_app.go:365-371`，测试 `present_policy_test.go:12` 钉死），retained 路径要示例显式 `SetPresentPolicy(retained)`（pipeline_app.go:175-185 注释自认 "Default remains full_paint until W6"）。文档 §6.3 说 P3 起"静态层纹理复用/禁止每帧无条件重传"，默认值直接违背。retained 纹理路径本身（`ui/scene/textured.go` 双缓冲 slot、LRU、延迟释放、EnsureCapacity）工程质量不错，这部分是兑现的。

3. **F09「动画默认 compositor-only」——断头路（P1）**
   - `ui/scene/compositing.go:11` 的 `ClassifyDirty` 只有测试在调；`ui/scene/packet.go:57-67` 的 `Mutations`/`CompositorDirtyIDs` 字段没有任何 embedder 生产者填充或消费者应用；`ui/animation/implicit.go:29,53` 的 AnimatedOpacity 产出 Mutation 后没人接。真实的 spinner 动画走的是 MarkNeedsPaint → 重录，不是 compositor-only。另外 `needsCompositing` 位算完没人消费（node.go:218 起），纯摆设（P2）。

4. 兑现得好的：depcheck 编译期依赖纪律、damage 度量、嵌套边界隔离（R3 内脏不外溢）、shell/content 分区计数——这些有真窗和指标背书，可信。

## 维度二：正确性 / 隐藏 Bug

1. **【P0】UI 线程与 raster 线程裸奔数据竞争（两处，本轮坐实）**
   - **BoundaryCache 无锁跨线程读写 map**：结构体（boundary_cache.go:27-50）没有任何 mutex；`Clear()` 由 UI 线程在 EventResize 时调用（`ui/embedder/pipeline_app.go:658`、1302），而 `BeginFrame`/`tryReplay`/`Store` 在 raster 线程的 draw 回调里跑（paintPresentTreeWithOpts）。并发 map 读写可直接 fatal panic。
   - **raster 线程清 UI 线程的脏标志**：`ConsumeNeedsPaint()` 在 raster 线程遍历整棵 RO 树清 needsPaint（pipeline_app.go:1193-1197），同时 UI 线程 ticker/事件可能正在 MarkNeedsPaint 写同一个无原子保护的 bool——既是 data race，也可能**吞掉一次有效失效**（丢帧甚至永久不更新）。live paint 路径 FlushPaint 也在 raster 线程遍历活树并 clearPaintDirty，同样裸奔。文档 §5.1 自己写着"Raster 线程禁止读可变 RO 树"，实现直接违反自家真源。

2. **【P1】两条绘制路径结果不一致（本轮新发现并坐实）**
   - **Transform 旋转中心不同**：即时 paint 路径绕盒子中心转（`ui/rendering/transform.go:102-113`，translate 到 cx,cy → rotate → 回移）；retained 层路径 `layer_build.go:68,79` PushTransform 后由合成 CTM 绕节点左上角原点旋转（textured.go pushCompositeCTM）。同一棵树 rot≠0 时两种模式画出来的位置不一样，HitTest 还只跟即时路径一致（transform.go:166-192）。
   - **视口滚动平移丢失**：即时 paint 用 `-scrollY` 平移内容（`ui/rendering/viewport.go:294-299`）；而层树的 viewport 分支（layer_build.go:122-140）只推 boundary+clipRect，**没有携带 -scroll 平移**，VirtualList kept cell 又被显式清脏按内容坐标 replay（virtual_list.go:408-413）。静态推断 retained 纹理模式下滚动位置无法表达。C4 真窗过了说明有别的因素兜底（比如滚出窗口的新 cell 重录产生 damage），但这是靠巧合不是靠设计，必须真窗复现钉死。
   - 同类：RenderSpinner 不在 `recordLeafContent` 的分支里（layer_build.go:263-299 只处理 ColorBox/Text/Image），叶子 spinner 在 retained 路径得到空 Picture 但 CacheKey 非零 → phase2 因 Ops 为空直接跳过不画，**spinner 在 retained 模式下会消失**（潜在，需 retained 窗口验证）。

3. **【P1】damage 只报"本帧重录"的层，纯位移 blit 不报损（本轮坐实证据链）**
   - `ui/scene/textured.go:876-882`：DamageRects 仅当 `tex.RecordedThisFrame(CacheKey)` 才追加；blit 走 `DrawGPUTexture`（render/context_image.go:806-822）→ `recordGPUOp`（context.go:180）只加计数器，**不 TrackDamageRect**。
   - 后果：内容没变只是位置变了的层（慢速滚动时 kept cell 正是这种，virtual_list.go:409-413 明确清脏让它们平移重放）零 damage rect → PresentFrameAuto 判 Idle 丢帧 → 屏幕冻结直到恰好有新 cell 挂载。目前被"滚动总会挂新 cell"掩盖，属踩雷型隐患。

4. **【P1】缓存指纹弱哈希**：`textContentKey` 只哈希文本前 64 字节（boundary_cache.go:420-444，`i < len && i < 64`），长文本尾部变化 → key 相同 → 放旧图陈旧画面。colorKey 浮点截断 + XOR 组合同理可碰撞。

5. **【P1】Transform 子 PaintContext 丢字段**：transform.go:114-123 手工 new PaintContext 只带 DC/Origin/Scale/PaintVisits，丢掉 CompositeOnly/BoundaryCache/LayerBudget/saveLayerDepth 等——transform 子树内 composite-only 跳过与边界缓存全部失效。另 transform.go:99 有一个恒假的死条件（P3）。

6. **【P2】MarkNeedsLayout 把 needsPaint 一路标到 relayout boundary，不停在中间 RepaintBoundary**（node.go:308-331）：布局变化击穿重绘隔离，过度失效（Flutter 是停在 relayout boundary 并只在受影响时 markNeedsPaint）。保守正确但白费重录。

7. **【P2】InputRouter 注释说"命中路径上每个控件都收 OnPointer"，实现只派发最深层目标**（input_router.go:142-147，`_ = band; _ = entry` 直接丢弃）——手势冒泡模型没做，注释骗人。

8. **【P2】overlay band 每帧无条件 BuildLayerTree 重录所有 entry**（state.go:199-234），且 entry 无裁剪、命中测试用 AABB，视觉溢出与命中不一致。

9. 【P3】RenderBox.Layout 把所有子 offset 覆盖成 Pad（box.go:52-55），多子/手工 SetOffset 会被冲掉；SetOffset 本身不标脏（node.go:126）；childless AbsoluteBox 自图与空叶图共用 CacheKey（重复 damage rect，无害但浪费）；forceFullPresent=3 计数 hack；legacy App.Run 阻塞式 Submit 违反自家 §5.3。

## 维度三：性能

- **每帧全树重建 + 全量 RecordInto 是最大 CPU 开销**（见维度一第 1 条，P0）。纹理缓存只省了 GPU 光栅化，CPU 端录制成本一分不少，大树下这就是主线程瓶颈。
- SubtreeNeedsPaint 在多个 paint 入口逐节点调用（box.go:91,121,133 等），最坏 O(n·depth)；Flutter 用显式 dirty list 保 O(dirty)。
- 度量走全树：TreeMeasureCacheStats / CountRepaintBoundaries / CountPictureOps / ConsumeNeedsPaint 每帧整树扫（pipeline_app.go:843-851、1193）——热路径里的纯观测开销，P2。
- 文本 wrapLines 在 Layout 和 Paint 各算一遍、无行级缓存（text.go:640-732），默认 full_paint 下等于每帧全文重排，P2。
- VirtualList 变高行 rowH 查找 O(mounted²)（virtual_list.go:495-505），窗口小，P3。

## 维度四：资源占用

- PictureTextureCache 工程好（双缓冲、延迟释放、LRU），但 EnsureCapacity 只涨不缩（textured.go），受限子树按全窗尺寸记纹理，密集场景 GPU 内存膨胀风险，P2。
- BoundaryCache 分代驱逐合理；LayerCache 死代码占着维护心智，P3 清理。
- tmp_vlprobe 编译失败目录留在仓库里破坏 `go build ./...`，P2 卫生问题。

## 维度五：可读性 / 冗余

- 三套并行缓存体系（BoundaryCache Picture 级 / PictureTextureCache GPU 级 / LayerCache 死代码），recordOwnContent 与 recordLeafContent 画法重复易漂移（boundary_cache.go 与 layer_build.go 各一份 ColorBox/Text/Image 绘制），P2。
- baseOf 类型 switch 硬编码全部 RO 类型（node.go:383-408），新增类型忘改就静默断掉脏传播链，扩展性陷阱 P2。
- 坐标约定混用：FillRect 用 pc.Abs 平移、FillPath 用 DC.Push+Translate、transform 用 OriginX=0 技巧——读代码费劲，P3。
- 加分项：注释诚实、可溯源、几乎每个坑都有注释解释为什么，这在同类代码里少见。

---

## 能否支撑工业级 GUI 组件库？

**现状不能，底子可以。** 缺的三块硬骨头：① 真 retained 层树（增量构建，接通 LayerCache 或重写脏路径录制）；② 线程所有权纪律（UI/raster 对 RO 树和缓存的读写边界要么加锁要么单线程化）；③ compositor-only 动画闭环（Mutations 从生产到合成接通，或者诚实地把 F09 承诺降级）。这三块补完，加上 TOP5 清单，才够格谈工业级。

## 评分表（各 10 分）

| 维度 | 得分 | 说明 |
|---|---|---|
| 架构对齐度（诚实性） | 6.5 | 骨架真；F01/F02/F09 三大承诺均打折，死代码冒充已实现 |
| 正确性 | 6 | 跨线程竞争 ×2、双路径不一致 ×3、弱指纹、damage 洞 |
| 性能 | 5.5 | 每帧全树重建+重录是地基性开销；多处 O(N) 观测进热路径 |
| 资源占用 | 7 | 纹理缓存工程扎实；EnsureCapacity 不缩 + 全窗纹理是隐患 |
| 可读性 / 冗余 | 7 | 注释优秀；三套缓存并存、baseOf switch、坐标混用扣分 |
| **综合** | **6.4** | 中型组件库雏形合格，工业级差距集中在 TOP5 |

## TOP5 必修清单

1. **【P0·性能/诚实】终结每帧全树重建+全量重录**：接通或重写增量 LayerCache（layer_build.go:20 全量 appendNode；layer_cache.go 死代码），删掉编译失败的 tmp_vlprobe。
2. **【P0·正确】线程安全纪律**：BoundaryCache 加锁或随帧移交（boundary_cache.go:27-50 vs pipeline_app.go:658/1302）；ConsumeNeedsPaint/FlushPaint 的 RO 脏标志读写移回 UI 帧边界或改原子（pipeline_app.go:1193-1197）；跑 `-race` 压测钉死。
3. **【P1·正确】统一双路径语义**：Transform 旋转中心（transform.go:102-113 vs layer_build.go:79）、viewport 滚动平移（viewport.go:294-299 vs layer_build.go:122-140）、Spinner 叶子在 recordLeafContent 补分支（layer_build.go:263-299）——各写一条真窗断言两种 present policy 出像素一致。
4. **【P1·正确】补 damage 洞 + 强指纹**：纯位移 blit 也报新旧两个矩形（textured.go:876-882）；textContentKey 全文哈希替换 64 字节截断（boundary_cache.go:428-430）。
5. **【P1·诚实】compositor-only 动画闭环二选一**：接通 Mutations 生产→packet→合成消费（packet.go:57、compositing.go:11、implicit.go:29），或在 ENGINE_UI_WIDGET_RENDER.md §10 把 F09 承诺降级为"仅隐式动画 API 预留"；同时默认 present policy 要么 W6 兑现 retained 默认化，要么文档改口。
