# 第 2 轮交叉复核报告（ROUND2）

> 复核对象：`docs/review/R1–R7` 共 7 份分模块审查文档。
> 复核方式：通读全文 + 回源码抽验 10+ 处关键行号 + `go build ./...` 实测。
> 本轮定位：排查矛盾、归并重复、找漏审、汇总 P0/P1、区分共识与单源待验。

---

## 一句话结论

7 份文档整体可信：本轮抽验的 10 余处关键结论**全部在源码得到证实**（含一处对旧线索的正确推翻）；发现的问题主要是**口径不统一、同一 bug 分散多处、以及一批没人审的模块**，没有发现方向性误判。

---

## 一、矛盾点排查

| # | 矛盾/不一致 | 详情 | 判定 |
|---|---|---|---|
| M1 | **评分口径三种制式** | R1=32/40、R2=25/40、R5=25.5/40、R6=24/40（4 维×10）；R4=24/50（5 维，多一个 Skia 对齐维）；R3=6.4/10、R7≈5.8/10（综合平均）。**无法直接横向比较** | 口径问题，终审统一折算 |
| M2 | 同一 bug 级别漂移 | Stroke damage 无线宽外扩：R5 定 P1、R7 定 P2；Buffer.Map 忙等：R2 定 P1、R7 定 P2。位置完全相同（context.go:1162/1175、map_pending.go:190-204） | 取严，均按 P1 计 |
| M3 | LayerCache 表述冲突 | R3 说 `ui/rendering/layer_cache.go` 是死代码（NewLayerCache 无生产调用者，唯一引用方 tmp_vlprobe 编译不过）；R4 性能节写"每帧整树重建……靠 LayerCache 复用缓解"——若 LayerCache 是死代码此说法不成立。注意仓库里有**三个撞名缓存**（rendering.LayerCache / render/scene cache.go LayerCache / PictureTextureCache），R4 大概率指的是后两者，属措辞歧义非事实错误 | 已回源码验证 R3 正确；R4 措辞需修正 |
| M4 | shaper 默认值注释矛盾 | R6 报"shaper.go 注释说默认 OwnShaper 实际是 HbShaper"。本轮实测确认：第 7 行、29 行注释写 OwnShaper (default)，第 21 行实际 `defaultShaper = NewHbShaper()`——**R6 属实** | R6 正确 |
| M5 | 验收基调差异 | R1 给验收体系打 32/40 较高，其他文档大量 P0——不算矛盾（R1 审的是门禁诚实性，不是引擎正确性），但终审总评时要分开陈述 | 说明即可 |
| M6 | damage 生效层级 | R3 说 embedder 层 PresentFrameAuto 吃 damage 判 Idle；R7 说 OS 级增量呈现整链是死的（PresentWithDamage 丢参数）。两者是**不同层**，合起来才是完整故事：damage 在 GPU 内 scissor 有效、embedder 决策有效、OS present 无效 | 互补，已归并 |

## 二、重复发现归并

| 编号 | 归并后问题 | 出处 | 终定级别 |
|---|---|---|---|
| D1 | Stroke/Fill 脏区不含线宽外扩 | R5-A5(P1) + R7(P2)，context.go:1162/1175，本轮复验 ✅ | P1 |
| D2 | Buffer.Map 忙等阻塞 | R2-C6(P1) + R7(P2) | P1 |
| D3 | UI/raster 双线程无所有权纪律（竞争族） | R3-P0（BoundaryCache + ConsumeNeedsPaint）+ R4-P0（ui/scene 层字段原地写）+ R6-P1（HbShaper XScale）+ R2-P2（swapchain Stats）。四处独立发现，同一根因 | P0 族 |
| D4 | InputRouter 只派发单个命中目标（注释承诺整条路径） | R1-A1(P2) + R3(P2)，input_router.go:142-147 | P2 |
| D5 | tmp 目录破坏构建（tmp_vlprobe、render/tmp_ref_stroke 使 go build ./... 红） | R1+R3+R7 三方独立，本轮 go build 实测复现 ✅ | P1 卫生 |
| D6 | 缓存指纹弱哈希族 | R3 textContentKey 截断 64 字节（boundary_cache.go:428，本轮复验 ✅）+ R6 缓存 key 冲突三处 + R4 Encoding.Hash 漏 brushes | P1 族 |
| D7 | "声明未接线"死代码族 | R2 uniformStructTypes（本轮复验 ✅）+ R3 LayerCache/Mutations + R6 ShapingCache + R7 ClipDepth/OverlapFactor/Auto 路由 | P1 族 |
| D8 | 增量呈现/damage 链断裂总图 | R7 OS 级丢参数（surface.go:312，本轮复验 ✅）+ R3 blit 不报损（textured.go:876-882）+ R5 A5 欠估 + R1 C2 门禁公式估算 | P0~P1 族 |
| D9 | 全帧全量重录性能 | R3-P0（每帧全树重建）+ R4（Path 每次 Clone）+ R6（文本逐字 ShapeUncached） | P0/P2 |

## 三、漏审项识别（7 份文档都没覆盖）

| 模块 | 规模 | 状态 |
|---|---|---|
| render/filters、render/raster | init 副作用小包 | 零覆盖（风险低但应记录） |
| ui/focus、ui/semantics、ui/theme、ui/input | 合计约 1900 行 | 零覆盖 |
| gpu/context（window/platform/gesture/scroll 等） | 约 22 文件 | R2 仅一句带过，无论述 |
| ui/platform 窗口层（wayland/x11/win32/appkit） | 大量文件 | 仅 R1 vet 提 unsafe.Pointer、R6 提 IME 两处，clipboard/cursor/vsync/CSD 未系统审 |
| render/integration/ggcanvas | — | 仅 R7 从 damage 接口角度碰过 |
| lib/（wgpu-native 二进制分发与版本管理）、third_party、scripts/ | — | 零覆盖（供应链/构建卫生盲区） |
| 根目录散落产物 | ui_wr_* 二进制 8 个、tmp_ptrgesture/tmp_ptrprobe/tmp_repro_clip/tmp_resize_diag、r4_snapshot.png、out/ | **任何文档都没点名**（比 R1/R7 提到的 tmp_vlprobe 范围更大） |

## 四、P0/P1 汇总总表

### P0（6 项，建议修复顺序即编号顺序）

| 编号 | 模块 | 问题 | 文件:行号 | 来源 | 本轮抽验 |
|---|---|---|---|---|---|
| X01 | gpu/rwgpu | stringViewToString 用 unsafe.String 引用 C 内存未拷贝（use-after-free） | adapter.go:556-571 | R2-C1 | ✅ |
| X02 | ui/embedder+rendering | UI/raster 跨线程竞争：BoundaryCache 无锁读写 + raster 清 UI 脏标志 | boundary_cache.go:27-50; pipeline_app.go:1199 | R3 | 部分✅（行号修正为 1199） |
| X03 | ui/scene | 光栅线程原地写共享层树字段（无同步） | composite.go:77-78; rasterize.go:52-53; textured.go:844 | R4 | 待终审 -race 实测 |
| X04 | render | 路径布尔运算是逐像素采样近似（2048 上限静默截断） | path_boolean.go:62-100 | R5-A3 | 部分✅ |
| X05 | render/text | 主绘制管线绕过 shaping（只有 cmap+advance，连字/kerning/阿拉伯变形全丢） | face.go:258; shape_result_cache.go:379 | R6 | 注释级证据，待行为实测 |
| X06 | ui/rendering | 每帧全树重建+全量重录；增量 LayerCache 是零调用死代码 | layer_build.go; layer_cache.go | R3-F01 | ✅ |

### P1（按修复批次分组）

**批 A（内存/并发安全）**
- R2-C2 acquire 超时锁黑洞 swapchain.go:975-1020
- R4 NextLayerID 非原子 layer.go:24-30
- R6 HbShaper XScale 竞态 shaper_hb.go:67-68

**批 B（错误结果类）**
- R5-A4 clip even-odd 忽略绕向 internal/clip/mask.go:199-224
- R5-A1/A2 Path.Reset 不清 bounds / Append 不合并边界 path.go:178-196（本轮双复验 ✅）
- R5-A5=D1 damage 无描边外扩 context.go:1162/1175 ✅
- R5-A6 focal 渐变忽略 StartRadius gradient_radial.go:130-190
- R5-A7 CPU 渐变不受 CTM 影响
- R6 bidi 半套 segment.go:80-84 + layout.go:266-309
- R6 IME delete_surrounding 静默丢弃 editor.go:345-348（本轮复验 ✅）
- R6 IME preedit 字段错位 wayland_textinput_linux.go:380-393
- R6 变量字体 GPU 路径丢 variations
- R4 render/scene CPU 路径丢图层 alpha renderer.go:716-722
- R4 recording 回放丢字体/clip recorder.go:177,185; backend.go:118-136

**批 C（死链/死代码/一致性）**
- R7 OS 增量呈现断链 surface.go:310-314（本轮复验 ✅）
- R7 MSAA 中帧 LoadOpLoad 读已 discard 内存 render_session.go:5370-5373
- R7 Auto 选管线死逻辑 pipeline_mode.go:60/65
- R3 F02 默认 full_paint pipeline_app.go:365-371
- R3 F09 compositor-only 动画断头 packet.go:57-67
- R3 双路径不一致（Transform 旋转中心/viewport 滚动/spinner retained 消失）
- R2-C3/C4 WGSL uniform 数组步长与 MatrixStride 不合规 lower.go; backend.go:1622-1635
- R2-C5=D7 uniformStructTypes 死代码 ✅
- R4 Encoding.Hash 漏 brushes encoding.go:751-800

**批 D（性能/资源）**
- R2-C6=D2 Map 忙等 map_pending.go:190-204
- R5 渐变逐像素 pow gradient.go:98-133
- R6 fallback 字体按 rune×size 重复整读文件 fontscan_fallback.go:100-127
- R6 缓存 key 冲突族 =D6
- R3 damage blit 不报损 textured.go:876-882
- R1 C2 组合窗门禁公式估算 main.go:63-66

**批 E（工程卫生）**
- D5 tmp 目录破坏构建（go build ./... 实测红）
- 根目录散落二进制与探针目录（漏审新发现）
- shaper.go 默认值注释矛盾（M4）等假注释

## 五、共识与分歧

**共识（多文档独立印证 + 本轮抽验，高可信）：**
1. tmp 目录破坏构建（R1+R3+R7+本轮实测）；
2. UI/raster 线程纪律系统性缺失（R2/R3/R4/R6 四方独立发现不同点位）；
3. damage/增量呈现链从 OS 级到脏区估计全线有问题（R1/R3/R5/R7 四份互补）；
4. 缓存指纹弱哈希是系统性模式（R3/R4/R6）；
5. "声明未接线"半成品模式普遍（R2/R3/R6/R7 各报多处，uniformStructTypes 本轮坐实）；
6. 注释与行为不符是文化性问题（R1 C2、R2 C1、R4 stamp 推翻案、R6 M4、R7 多处）。

**分歧/单源待验（进第 3 轮抽验清单）：** 见下节。

## 六、第 3 轮终审抽验清单（12 条，证据链最弱者优先）

1. X03：ui/scene 光栅线程原地写是否真与 UI 线程并行（跑 `-race` 真窗）。
2. X02 后半：ConsumeNeedsPaint 是否确在 raster 线程调用（pipeline_app.go:1199 线程归属）。
3. X05：DrawString GPU 主路径是否真绕过 HbShaper（行为级验证，不只看注释）。
4. X04：布尔运算 EvenOdd 配对错误与 2048 截断（构造自交路径实测）。
5. R7 MSAA LoadOpLoad UB：4x MSAA 真机开 damage 复现闪块。
6. R7 Wayland FifoRelaxed 命令缓冲释放时序（作者自标"未证实的定时炸弹"）。
7. R3 viewport 滚动平移丢失 + spinner retained 消失（作者自标"静态推断，需真窗钉死"）。
8. R2-C2 acquire 锁黑洞：故障注入验证死锁恢复路径。
9. R5-A7 CPU 渐变是否真的不受 CTM 影响（旋转坐标系画渐变对比）。
10. R4 render/scene CPU tile 丢 alpha（renderer.go:716 `_ = alpha`）像素对照。
11. R1 C2 公式门禁：改场景参数验证门禁自动变绿。
12. R5-A6 focal 渐变 StartRadius 是否真未参与 computeTFocal（细读 gradient_radial.go:130-190）。

---

*第 2 轮完。下一轮（第 3 轮终审）：对上述 12 条做源码级复核 + 补审漏审模块的快速扫描，产出 ROUND3_final_review.md。*
