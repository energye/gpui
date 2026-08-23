# R4 场景层审查：ui/scene、ui/io、ui/overlay + render/scene、render/recording、render/surface、render/svg

> 审查依据：`docs/review/.material_scene.txt`（约 41 万字符前人调查记录）+ 本轮对 5 条关键线索的逐条源码核实（文件:行号均已复核）。核实结论与材料不一致处已如实标注（见「线索核实」一节，其中 stamp 一条被推翻）。

## 直话结论

**这套场景层现在不能算工业级。** 生产在用的 `ui/scene` 主链路（真窗验证过的那条）基本能用，但底下埋着跨线程数据竞争和几处语义偏差；`render/scene` 功能最全却有两个正确性洞（CPU 回放丢图层 alpha、圆角矩形绕过裁剪）；`render/recording`、`render/surface`、`render/svg` 三个包全仓库**零消费者**（连示例都没接 surface/svg），属于原型半成品。最大的问题不是某个 bug，而是**同一件事做了三套**：ui/scene.Picture、render/scene.Encoding、render/recording.Command 三种录制格式并存，互不相通。

---

## 一、线索核实结果（本轮逐条复核）

| # | 线索 | 结论 | 证据 |
|---|------|------|------|
| 1 | layerIDGen 非原子 | **成立** | `ui/scene/layer.go:24-30`，`layerIDGen++` 裸加，注释自认 "not concurrency-hardened" |
| 2 | 跨线程共享可变层树 | **成立** | `ui/scene/composite.go:77-78`、`rasterize.go:52-53`、`textured.go:844` 在合成/光栅侧直接写 `t.NeedsRaster=false; t.Picture.Valid=true`；`packet.go:53-55` Root 按指针共享（注释写 "shared / COW"，实际无任何 COW 协议），UI 线程下一帧重建/写同字段 → 数据竞争 |
| 3 | render/svg\|surface\|recording 无生产消费者 | **成立且更严重** | 全仓 grep `gpui/render/surface`、`gpui/render/svg` 零命中（连 examples 都没有）；recording 只有 `examples/render_recording/main.go` + 自测 + `render/s3c_m3_gpu_gate_test.go`。surface/svg 是彻底孤儿包 |
| 4 | textured.go dc 共享线程安全疑点 | **部分成立** | 缓存 map 有 `c.mu` 保护，但 record 路径在持锁期间做 GPU 提交（FlushGPUWithView），且 dc 本身的 Push/Pop/SetDamageTracking 无独立同步，依赖"单光栅线程"这一未强制约定 |
| 5 | Picture 跨线程修改风险 | **成立**（与 #2 同根） | PictureLayer 字段双线程读写无同步，见 #2 |
| — | **stamp 永不自增（LRU 失效）** | **推翻** | 前人记录称 `c.stamp` 从不自增导致 LRU 退化为随机淘汰。实测 `ui/scene/textured.go:304` 的 `BeginFrame` 中有 `c.stamp++`（"advance the LRU clock once per composite frame"），LRU 时钟正常。此线索**不成立** |

---

## 二、正确性 / 隐藏 Bug

### ui/scene（生产主链路）

- **P0｜跨线程共享可变状态**：如上表 #2/#5。raster 线程合成时原地改共享层的 `NeedsRaster/Picture.Valid`，UI 线程同时建下一帧树并复用 LayerCache 里同一批对象 → Go 内存模型下的真数据竞争，race detector 必报。修法：把这类状态移出共享层（放 FramePacket 侧），或引入代际/fence 同步。
- **P1｜NextLayerID 非原子**（`ui/scene/layer.go:24-30`）：目前只有 UI 线程调用所以没炸，但这是靠约定不是靠机制；一旦 overlay/raster 任何一侧并发取 ID 就是脏 ID。
- **P2｜空矩形裁剪语义反了**（`ui/scene/composite.go:100-122`）：`ClipRectLayer/ClipRRectLayer` 当 W/H≤0 时 case 不 return，落到默认分支**不裁剪直接画子树**。Skia/Flutter 的语义是空 clip = 子树全部裁掉。折叠到零尺寸的控件内容会漏出来。
- **P2｜OpacityLayer 非隔离快路径**（`ui/scene/composite.go:162-172`）：用 `PushLayer(BlendNormal, op)` 逐笔乘 alpha，子图形重叠处透明度叠两次，结果比 Flutter saveLayer 组隔离更暗。注释自己承认只对"不重叠 UI"正确——但引擎无法保证这一点。ColorFilter/BackdropFilter 用了 `PushLayerIsolated`，说明隔离能力是有的，opacity 没用上。
- **P2｜Picture 边界负坐标截断**（`ui/scene/picture.go:374`）：`image.Rect(int(r.minX), ...)` 对 minX 向零截断（-10.5→-10），min 侧该用 floor；注释说 ceil 保守但只对 max 生效。负坐标内容（文字上伸、越界路径）左/上边可能丢 1px。
- **P2｜stats 光栅假成功**（`ui/scene/rasterize.go:52-53`）：`RasterizeDirtyToContext` 在 dc==nil 的统计模式下也置 `Valid=true`、清脏标记——没画却说画了，后续真实合成会跳过该层。
- **P2｜io 解码池会卡 UI 线程**（`ui/io/decode.go:29,49,66`）：jobs 通道缓冲 64，满了 `p.jobs <-` 阻塞提交方；worker 无 recover（解码 panic 直接杀进程）、无 Close/shutdown。
- **P3｜raster loop Done 发送可挂死**（`ui/raster/loop.go:107-109`）：`job.Done <- err` 阻塞发送后 close，若调用方传无缓冲通道又弃收，整个光栅线程挂死；且 send 后 close，第二次接收读到零值 nil error，易误判成功。
- **P3｜overlay Remove 匹配绕**（`ui/overlay/state.go:65-83`）：按指针 OR id 匹配，跨 State 传 Entry 可能删错条目；`ui/overlay/paint.go:37` 还有 `var _ = rendering.Point{}` 死代码。

### render/scene（编码器最完整，但有两个洞）

- **P1｜CPU tile 渲染器丢图层 alpha/blend**（`render/scene/renderer.go:716-722`）：`TagPushLayer` 分支 `_ = alpha // TODO`，PopLayer 也是空转。PushLayer(alpha=0.5) 在 CPU 路径按不透明画，GPU 路径正常 → **同一 Scene 双后端输出不一致**，违反录制回放一致性。
- **P1｜Hash 不含 brushes**（`render/scene/encoding.go:751-800`）：Hash 只算 tags/pathData/drawData/textData/transforms，brushes 流（颜色所在）不参与。几何不变只换色 → hash 相同 → `LayerCache.Get(hash)`（cache.go 按 hash 取）命中旧颜色像素。当前 Hash 仅测试/bench 在调，属**埋雷**，接线即炸。
- **P2｜圆角矩形绕过裁剪**（`render/scene/renderer.go:704`）：`TagFillRoundRect` 画到基础 `pm` 而非带裁剪掩码的 `activePM`（相邻 TagStroke 就用的是 activePM）。clip 区域内圆角矩形不被裁。
- **P2｜DamageTracker ID = 命令序号**：插入/删除一个图形全体 ID 错位 → 大面积假脏；同位置换色的漏检方向也存在。
- **P3｜AppendWithTranslation 对未知 tag 直接 panic**（encoding.go）：新 tag 上线即崩生产，fail-fight 太硬；flattenLayers 会静默 pop 未闭合的 layer，掩盖用户栈不平衡 bug。

### render/recording（未接线）

- **P1｜回放保真度破**：`recorder.go:177,185` DrawText/StrokeText 回放一律传 `nil` face（字体丢失，StrokeText 静默降级为填充）；`backends/raster/backend.go:118-136` SetClip 实际不裁剪（注释自认 Context 没有 Clip 接口），ClearClip 空函数——**录了 clip 回放等于没录**。
- **P3｜FinishRecording 直接交出内部 commands 切片**（`recorder.go:94-101`），"Recorder 不应再用"只是文档约定，复用即污染所谓不可变的 Recording；registry 重复注册 panic，与 surface registry 静默覆盖约定相反。

### render/surface（未接线）

- **P2｜混色公式直通/预乘混用**（`image_surface.go:330-333`）：`outR=(src.R*srcA + dstR*dstA*invSrcA)/outA` 最后除以 outA 得到的是**直通 alpha**值，却存进 Go 语义为**预乘**的 image.RGBA；src 又来自 `RGBA()`（本就是预乘）再乘一次 srcA。半透明叠加处颜色偏亮/发灰，AA 边缘出脏边。opaque-on-opaque 快路径没事，恰好掩盖了这条。
- **P2｜expandStroke 无 cap/join/miter/dash**：折线拐角自相交出豁口；描边质量远低于 render 包内部已有的描边器（又是一处重复造轮子）。
- **P3｜Close 后 Image() 返回 nil 不检查**（NPE 风险）；GPUSurface 声称 SupportsResize 却没实现 Resize。

### render/svg（未接线）

- **P2｜fill="none" 继承被黑填充覆盖**（`render/svg/renderer.go:186-191`）：祖先设 `fill="none"`、元素自身无 fill/stroke 属性时，`shouldFill=false` 但兜底逻辑看的是原始字符串 `a.Fill==""&&a.Stroke==""` → 强制 hasFill=true → 黑填充。违反 SVG 规范也违反自家文档"none 保留"承诺（现有测试只盖了带 stroke 的元素，没盖住这条路）。
- **P3｜百分比单位错解**（`parser.go:114`）：`50%` 剥掉 % 当 50 用；属性解析错误一律静默吞成 0；defs/use/gradients/clipPath 不支持且静默跳过（url(#grad) 填充落黑）；非均匀拉伸渲染、不支持 preserveAspectRatio。

---

## 三、与 Skia 录制回放语义对齐度

- **ui/scene.Picture 不是 SkPicture**：op 只有 6 种（矩/path 填充描边、文字、图片），坐标在录制时就烤死为绝对坐标，**没有 save/restore/matrix/clip/saveLayer 状态栈**；SkPicture 录的是完整 canvas 状态序列，可在任意 CTM 下重放。这里重放正确性完全依赖外层 walk 先摆好 CTM（纯平移假设由 restrictive() 把关）。RasterExtra 闭包更是直接逃逸出显示列表。定位应是"受约束的命令列表"，文档别拿 SkPicture 类比。
- **saveLayer 语义**：filter 类走 PushLayerIsolated（对齐）；opacity 走非隔离快路径（不对齐，见 P2 条）；CPU tile 路径干脆不实现图层（P1 条）。三处三种行为。
- **render/recording** 号称 Cairo 式录制回放，但字体、clip 两项核心状态在回放侧丢失，达不到 Skia "replay == 原绘制" 的底线。
- render/scene.Encoding 双流结构最接近 Vello/Skia 字节流思路，是对齐度最高的一套——偏偏生产 UI 链路没用它。

## 四、性能

- recordWith 对 clip/旋转/缩放下的层一律分配**全窗口纹理**（`textured.go:371` allocEntry(id, c.width, c.height)），EnsureCapacity 可无限扩容：64 个上限 × 全窗 RGBA × dpr² × 双缓冲 ≈ GB 级显存风险，且没有字节预算（对比 render/scene LayerCache 有 64MB 预算）。滚动列表里每个视口内图片层都是全窗纹理。
- evictForNew 兜底可能驱逐本帧在用条目（`textured.go:627` 注释自认）→ 抖动重录。
- Encoding.Hash O(n) 全流扫描当缓存键、registerImage O(n²) 线性查重、measureTextBounds 在光栅线程逐字形迭代——量级尚可但都在热路径上。
- 每帧整树重建 + Path 深拷贝（FillPath 每次 Clone），靠 LayerCache 复用缓解，未消除。

## 五、资源占用

- 显存无预算（见上）；纹理缓存双缓冲 + 延迟释放队列（上限 max×3）设计是对的。
- recording ResourcePool 每次绘制克隆 Path 入池、无去重（文档声称有），长录制内存线性膨胀。
- io 解码池 worker 永生、任务不可取消，解码大图无法中断。

## 六、可读性 / 冗余（重点）

**三套显示列表并存，互不复用**：
1. `ui/scene.Picture`（结构体 op，float64 绝对坐标）——生产在用；
2. `render/scene.Encoding`（Vello 式双流字节编码，含 transform/clip/layer/text glyph run/damage/缓存全套）——只有 filters/internal/gpu 和示例消费，**ui/ 全家不 import 它**；
3. `render/recording.Command`（Cairo 式类型化命令）——仅一个示例 + 测试。

外加 `render/surface`（自带一套劣化版 Path/stroke 类型系统）和 `render/svg` 也全是孤儿。**处置建议**：
- `render/svg`：唯一还有独立价值的包（图标加载），补齐 fill="none"/% 单位/错误上报后可接线给 ui 用；
- `render/recording`：与 render/scene.Encoding 功能重叠，回放保真度又不达标，建议**裁撤**，向量导出需求挂在 Encoding 迭代器上实现；
- `render/surface`：ImageSurface 与 render 包 CPU 渲染重复、GPUSurface 是壳，建议**裁撤**或降级为 internal 试验田；
- `render/scene` 与 `ui/scene` 二选一收敛：长期应让 ui/scene 的 Picture 下沉到 Encoding（拿到 clip/transform/渐变/图层能力），短期至少冻结 render/scene 新功能、把两个 P1 洞补掉。

---

## 七、能否支撑工业级使用？

**不能，分层说：**
- `ui/scene` 主链路：受控场景（单 UI 线程 + GPU 路径 + 真窗覆盖的能力点）可用，接近工业级；但跨线程竞争（P0）、空 clip 反语义、opacity 非隔离三件事不修，不能对外承诺。
- `render/scene`：设计最好、能力最全，但 CPU/GPU 回放不一致 + Hash 雷 + 圆角绕裁剪，处于"高潜力、未达标"。
- `render/recording / surface / svg`：原型级，svg 差一步可用，另两个建议砍。

---

## 评分表（各 10 分）

| 维度 | 得分 | 理由 |
|------|:---:|------|
| 正确性/隐藏Bug | 5 | 主链路真窗绿，但 P0 竞争 + 多处语义偏差 + 未接线包各藏雷 |
| Skia 录制回放对齐 | 4 | Picture 非状态机、opacity 隔离缺失、CPU 图层 TODO、recording 回放丢字体/clip |
| 性能 | 6 | 纹理缓存/双流编码底子好；全窗纹理、O(n) hash、每帧深拷贝拖后腿 |
| 资源占用 | 5 | 显存无字节预算是硬伤；延迟释放/池化设计合格 |
| 可读性/收敛 | 4 | 三套显示列表 + 三个孤儿包，认知和维护成本翻倍 |
| **合计** | **24/50** | |

## TOP5 必修清单

1. **[P0] 斩断 ui/scene 跨线程原地写**：`composite.go:77-78`、`rasterize.go:52-53`、`textured.go:844` 不再改共享层字段，脏状态移入 FramePacket 或加代际同步；顺带给 NextLayerID 换 atomic（`layer.go:24`）。
2. **[P1] render/scene CPU 路径补齐图层合成**：`renderer.go:716-722` 实现 PushLayer alpha/blend，`renderer.go:704` 圆角矩形改画 activePM——消除 CPU/GPU 回放分歧。
3. **[P1] Encoding.Hash 补 brushes 流**（`encoding.go:751`）：防 LayerCache 换色撞键，接线前必须修。
4. **[P1] 明确 recording/surface 去留**：要么补齐 SetClip/face 回放（`backend.go:118`、`recorder.go:177`），要么从公开 API 裁撤——不能以现状留在 render 公开面上。
5. **[P2] 语义对齐两连修**：空 ClipRect 改为裁空（`composite.go:100`）；OpacityLayer 重叠场景走 PushLayerIsolated（`composite.go:162`）；顺手修 svg fill="none" 继承（`renderer.go:186`）与 picture 负坐标 floor（`picture.go:374`）。
