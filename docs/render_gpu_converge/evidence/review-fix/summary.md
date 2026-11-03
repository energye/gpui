# P3.1–P3.6 交付后审查修复批 · 证据汇总（2026-11-03）

> 来源：P3.0–P3.6 收口后的全量源码审查（三路深读 + 定向核查）确认 15 项问题；本批按
> 「复现红 → 对齐标准方案 → 修 → 钉转绿 → 登记」逐条闭环。分支 feat/p2-收敛整合，
> HEAD 基线 18ccc406。

## 第一批：三个 P0

### P0-1 SetAnimatedImage 静态 owned buf 泄漏（ui/rendering/image_anim.go）
- **复现红**：`TestRenderImage_AnimatedReplacesOwnedStatic`（image_anim_test.go）——静态
  owned → SetAnimatedImage → `static.Disposed()==false`。
- **比审查定位更深一层**：真正孤儿点在 `releaseAnimatedExcept(keep)` 的 `im.anim != keep`
  分支——静态模式（anim==nil、keep=a≠nil）时把 `im.Img/ownsImg` 清零却不 Dispose。
  审查点名的 `im.Img = a.Image()` 覆盖是其下游表象。
- **修**：① `releaseAnimatedExcept` 只在真摘掉解码器时动 Img/ownsImg；② 覆盖路径前补
  `im.releaseOwnedExcept(nil)`（双保险，静态 owned 由正本释放口 Dispose）。
- **钉**：探针转正为永久钉（含换装后 Clear 连带释放断言）。`ui/rendering` 包 image 相关
  全绿；整包回归含已知波动（m5_keystroke，批 3 基线 summary.tsv 本就 FAIL、单跑绿）。

### P0-2 ImageCache.Stats() D20 快照竞态（render/internal/gpu/image_cache.go）
- **复现红**：新钉 `TestImageCache_StatsSampledFromAnotherThread_RaceFree`（GPU 环境
  `-race`）——竞态面实测 4 类，比审查点名的 `c.gen` 多三项：
  `entries` map（:320 插 / removeEntry 删 vs Stats len+遍历）、`bySrc` map（watch/unwatch
  不持 retiredMu vs watchedCount）、`lastUploadBytes`（光栅写 vs 快照读）。
- **拍板（用户确认）**：口径 B「全原子计数」——gen/lastUploadBytes 转 atomic，
  Entries/MipEntries/Watched 转镜像原子计数（entries/bySrc 增删唯一站点维护），
  Stats 零锁快照；热路径不加锁、无锁序风险；同题登记台账。
- **漂移防钉**：`TestImageCache_DerivedCountersMatchMaps` 六检查点（上传/mip/换代 drain/
  源释放/purge/终 drain）断言镜像 == map 地面真值。
- **门禁**：`-race ./render/internal/gpu/` 全包绿（0 DATA RACE，30.2s）；D20 对账窗
  `ui_wr_p36_animplay` 验证归真窗轮。

### P0-3 非 ASCII 逐 run 样式系统性错位（render/text + render/paragraph.go）
- **机制坐实**：shaper cluster=rune 下标（shaper_hb.go:105-115/shaper_own.go:613），
  layout.go:441 却加 `seg.Start`（字节）+`part.start`（字节累加）；消费端
  paragraph.go:128（span 边界）与 isSpaceGlyph（:[]rune 索引）按 rune 解读。
  三钉复现红：CJK 三 span 颜色 counts=[2,0,6]、混排字重 face 丢 700、非 ASCII Justify
  不拉伸（gapCount 落空）。
- **修（统一 cluster ≡ rune 下标，全文基准发布）**：
  1. `shapeSegments`：段基 `segRuneStart`（RuneCount 累加）+ `facePart.start` rune 累加；
     cluster 平移全程 rune 空间。
  2. `LayoutText`：发布前按段落 rune 基（+\n）rebase，cluster 全文空间（对齐
     Paragraph.StyleForRun 声明的契约；Skia/Impeller positions 即全文 text 索引）。
  3. `wrap.go`：`markBreakOpportunitiesEnhanced` 直接用 cluster 作 rune 下标，删
     `findRuneIndexForCluster` 字节反查（全量切换不留垫片）。
  4. `styledFace.Runs → RunsAt(para, runeBase)`：带段内 rune 基切 span（旧状收到段切片
     按全段边界裁剪，多段段落错位）；layout 优先 `RunsAt` 断言。
  5. **连带洞（探针实测暴露）**：styledFace 把 parts 的 face 覆写回外层 base——
     MultiFace 作 base 时 `ShapeUncached(Source()==nil)` 拒整，CJK 富文本整段零 glyph。
     修为 weight-0 透传 `pr.Face`、weight≠0 才覆写。
  6. ellipsis 标记补段内 cluster（P1-5 一并）。
- **连带修复（复现链暴露的兄弟洞，同属本条）**：`calculateBreakPosition` word 路
  `lastBreak+1` → `lastBreak`：isBreak 是 break-before 语义，旧 +1 把下一 chunk 首字符
  拖进当行使行宽越过 MaxWidth——**ASCII 实测同样越界**（探针 "hello world hello"
  @90 越界 2.01px），属早于本批的真实存量洞，非 rune 改动引入。
- **钉**：三钉转正 `render/paragraph_cjk_test.go`（另含 P1-4/5 钉）；存量钉全绿；
  ui/rendering 整包绿（4 红均在批 3 基线 FAIL 名单 m1_noquadratic/m5_keystroke，单跑绿）。
- **登记（新问题归位，不顺手改）**：
  - `shape_result_cache.go:405-419` layoutGlyphsUncached 与 `ui/rendering/text_layout.go:635,679`
    兜底 glyph 全零 cluster（draw 路 cluster 无消费者，latent）→ 归 P3.8/P4.4 文本专项。
  - CRLF 富文本 span 边界：builder runeRuns 按原文计数 vs layout 归一 \r\n→\n，
    多段 + CRLF 混用时边界偏 1（latent，ASCII 单行不受影响）→ 登记待补。
  - `render/scene` GlyphEntry 不带 cluster（RenderedGlyph.Cluster 恒 0，零非测试读者）→
    按第 12 条存留两问保留待 P3.8。

## 第二批：六个 P1

### P1-4 字重 face 字号硬编码 20.0（render/paragraph.go）
- 复现红：`TestParagraph_WeightFaceFollowsBaseSize`（24px 基准 bold run face size=20）。
- 修：`f.family.Face(f.Face.Size(), WithWeight(w))`。

### P1-5 ellipsis 截断展平丢样式 + MultiFace 静默无标记（render/text/layout.go）
- **复现红**：`TestTruncate_EllipsisSurvivesMultiFace`（tail 非 … GID——
  `ShapeUncached("…", styledFace/MultiFace)` 因 Source()==nil 拒整）；
  `TestTruncate_KeepsPerRunStructure`（截断行单 run 拍扁）；
  `TestParagraph_TruncationEllipsisKeepsStyle`。
- 修（Flutter 语义：省略号继承断点字符样式）：截断不再拍扁成单 run——前缀按原 run
  切片重建 + 标记独立 run；face 取断点 run 的具体字体（`ellipsisFace` 逐级兜底），
  样式取覆盖断点 run 的 Style；line.Glyphs 与 runs 保持双写一致。
- 存量钉 `TestMaxLinesEllipsis` 的 `_ = lastGlyph` 空转改实断言（GID==… + cluster 单调
  段内），收紧后 ASCII 绿（真对齐非放宽）。

### P1-6 DrawParagraphStyled 漏还原 paint.Pattern（render/paragraph.go）
- 复现红：`TestDrawParagraphStyledRestoresPattern`（非 solid 图样调后 nil）。
- 修：快照/还原加 Pattern（SetRGBA 同清 Brush 与 Pattern，两者都在 restore 面）。

### P1-7 GPU blur 核半径钳 ±24 与 CPU 分叉（filter_gpu_graph.go）
- **复现**：σ=9/10/12 实测 diff=1/2/5（既有 GPU==CPU 容差 ≤3）——静态 `for i in -24..24`
  钳核 + Go 侧 `Min(24, ceil(3σ))` 四处。
- **拍板（用户：按标准对齐、要通用性，不走回退捷径）**：对齐 Impeller 动态半径单 pass
  口径——WGSL 循环界改 uniform `rad` 驱动，四处 Go 钳位去 Min；任意 σ 与 CPU 正本
  `CachedGaussianKernel(half=ceil(3σ))` 同构，无回退无近似分叉。
- **钉**：`TestFilterOpsGPUBlurLargeSigmaMatchesCPU` σ=9/12/20 GPU==CPU ≤3 且
  gpu_ops≥1 无回退；既有 blur 钉全绿（native WGSL 验证动态循环；GL 后端随五目标/
  真窗轮复核，登记见下）。

### P1-8 GPU tile 非零原点（filter_gpu_graph.go mode 12）
- 复现红：`TestFilterOpsGPUTileNonZeroOriginMatchesCPU` diff=255（x0 像素值加进归一化
  uv → ClampToEdge 拖影）。
- 修：`(x0 + u % w0) / size.x`（对齐 CPU 正本 lighting.go:121-123 `x0+mod` 语义）。

### P1-9 GPU lighting 负 z 被 max(z,0.001) 抬正（filter_gpu_graph.go mode 11）
- **复现修正（不采信审查叙述）**：审查称「灯在下方时全亮」——实测凸块场景 GPU/CPU 两侧
  均黑（diff=0），分叉在**凹形高度场缝壁**（法线 xy 投影 × 灯距）：双带留缝场景修复前
  diff=166。
- 修：对齐 CPU 守卫语义——仅 `0 ≤ z < 0.001` 抬到 0.001（防除零），负 z 保留。
- 钉：修复后 diff 稳定 4/255（缝壁 FP32 归一化残差，修复前 166；台面近黑双端锚防双双
  变黑假对齐）；容差显式声明 5 并注记来源。CPU 侧存量 `TestPointDiffuseLightGeometry`
  绿。

### P1-10 P3.1 全量切换漏网（build/vet 非绿）
- 复现：`go build ./...`/`go vet ./render/...` 红即问题结果。
- 实际面比审查大：`examples/ui_render_base_recording` spy 4 方法（FillPath/StrokePath/
  FillRect/StrokeRect）+ `render/recording/backends/raster` 两测试文件 11 处调用点
  （审查点名 2 处）。全部补 `render.BlendSpace` 实参（SRGB 与旧行为等价）。
- 门禁：全仓 build rc=0、`go vet ./render/...` rc=0、raster 测试包绿。
  五目标脚本的 known-broken 名单相应移除 ui_render_base_recording。

## 第三批：P2 规范

- **P2-11 gofmt**：17 文件（含本批新改文件）`gofmt -w render/ ui/ engine/`，
  `gofmt -l` 清零，零行为变化。
- **P2-12 死代码三定夺（过「重复实现？三支柱需求内？」两问）**：
  - `HasDefaultImageCache`（sdf_gpu.go）：collector 改 FromLive 后剩定义+注释引用，零
    调用 → 删；gpu.go 注释随改。
  - `ImageFilterMerge`（filter_ops_gpu.go）：死标记类型，零消费者（仅自注释）→ 删；
    catalog §13 P3.4 行随记。
  - `RegisteredGPUFilterGraphHooks`：**审查「零消费者」判定已过时**——
    `render/internal/gpu/user_shader_fill.go:356` 引擎真调用 +
    `render/p34_gl_maskfilter_fill_test.go:43` GL 路钉 → 保留公开面不动，如实收回删除
    倾向（能力面不裁）。
- **P2-13 catalog 两处滞后**：P3.6 批 1b 动图行「生产消费者为零」→ 更新为 kit 装配
  证据（`ui/kit/anim_image_widget.go:49`、`ui/kit/anim_image_state.go:304`）+
  `ui_wr_p36_animplay` 已交付；P3.1 BlendSpace 行状态按 §0 状态词约定改 🔌
  （已交付、真窗双后端验证、无生产消费者——旧标 🔗 与图例冲突）。
- **P2-14 测试钉空洞补实**：`TestParagraphBuilderSpanSemantics` 收紧（旧 `G!=1 && R!=1`
  逻辑与 + c=14 落错 span，PopStyle 恢复路径零断言）→ 按颜色真值三段断言 + c=17 落
  span3 钉 Pop 恢复；省略号空转断言并入 P1-5 修实。
- **P2-15 深审欠账登记**：P3.1/P3.2、P3.5 逐文件深审本轮配额未做（定向钉全绿 + 机械
  检查通过）——见台账修订行，后续轮补。

## 门禁（2026-11-03 实测）

- apidoc：`go run ./scripts/apidoc` **rc=0**（catalog 两处滞后 + ImageFilterMerge 删项登记后复验）。
- build/vet：全仓 `go build ./...` **rc=0**（linux/amd64）；`go vet ./render/...` **rc=0**。
- 五目标：`build/` 5/5 全绿（linux/amd64·linux/arm64·windows/amd64·darwin/amd64·darwin/arm64，
  各 raw + 分包 + nogpu 三段 exit 0；gpupixel 历史断口已不存在，known-broken 排除成为遗留口径，
  ui_render_base_recording 随 P1-10 移出名单）。
- per-file 主 sweep（正本 34 包，带 lib）：**741 条 = 711 PASS / 21 SKIP_TAGGED / 8 FAIL /
  1 NO_TESTS**；对照批 3 基线（738=706/21/10/1）零新增红，且 m1_noquadratic、pacing 两条
  基线漂移在本轮转绿（FAIL 只减不增）。本批新增钉 3 文件在册全绿
  （render/paragraph_cjk_test.go、render/filter_gpu_p189_test.go、
  render/internal/gpu/image_cache_stats_race_test.go）。
- per-file text 补轮（**新登记**：正本 34 包清单历史不含 `render/text`，为门禁盲区——
  P3.3 交付时该包同样未入列，本批以 extra 口径补）：**123 条 = 118 PASS / 5 SKIP_TAGGED /
  0 FAIL**，含 truncate_p15 新钉与全部 zz 长时件。
- race 组：`-race ./render/internal/gpu/` 全包 **rc=0、0 DATA RACE**（59.8s，最终代码态）；
  `-race ./render/` D20/动图/快照定向 **ok**（image_stats_test 的 ImageCacheSnapshot 转发面
  在册）。批 3 口径的 render 全包 race 债（当时红）由 P0-2 修复覆盖定向面，全包 race 债维持
  批 3 gate-attribution 归因。
- 真窗轮（`windows/window-runs.txt`，XWayland :0、一轮一窗、每轮查 Xwayland 存活）：
  8/8 绿——p34_filters、p36_animplay（D20 对账卡）、p31_linear_blend 双后端
  golden=0.0000%；p33_richtext 首跑 golden diff 2.6131%（双后端同值互证非抖动），
  worktree A/B 行内容取证：旧金图焊在断行 `+1` 吞字缺陷上（"every l|ine"），新画面
  完整词界 + justify 满幅；**用户拍板重采**，重采后 native/go 双后端对同一新金图
  golden=0.0000% 逐位一致（`p33_golden_before/after.png` 对照入证据）。

## 遗留与登记（入台账 K 点表）

- K3：cluster 兜底全零三处（shape_result_cache / ui 估算路）——latent，draw 路零消费者。
- K4：CRLF 富文本 span 边界偏 1——latent。
- K5：scene GlyphEntry 不带 cluster（RenderedGlyph.Cluster 恒 0）——随 P3.8 定夺。
- K6：WGSL 动态循环核 GL 后端复验（本批 native 已验；真窗 p34 双后端过，GL 档复跑在册）。
- 深审欠账：P3.1/P3.2、P3.5 逐文件深审本轮配额未做（定向钉全绿 + 机械检查通过）——已回填台账 §2.2 待办池 **L1–L3**（各行写明未审范围与深审须覆盖的面，取用按 §4.2 载体登记）。


---

# 追加：D 批（L1–L3 深审产出的 12 条修复，2026-11-03）

深审三报告（L1=P3.1、L2=P3.2、L3=P3.5，`deep-audit-l1l3/`）去重出 13 条 → 并 D5/D12 后
按用户拍板「P0×3 + P1×10 一批修、允许重采、逐条停报」执行 D1–D12。逐条纪律：复现红→标准方案→钉绿→负向对照；
证据逐条落 `deep-audit-l1l3/D8|D9|D10|D12-*.md`，过程登记 `findings-register.md`。

- **D1（L2-P0-1）shader 泄漏+Close 空操作**：`ShaderEffect.Close` 幂等 + `Closed()` 判别口 +
  `Register/UnregisterShaderEffectEvictor` 广播驱逐（RegisterPurgeEvictable 同族）；
  `userShaderCache.evictEffect`、`gpu_shared` 注册/反注册/abandon 补漏。钉：`p32_shader_leak_test.go`
  （6 轮句柄/字节零涨、closed 不回灌）。
- **D2（L2-P0-2）shader 失败=画实心色+假绿**：CPU 落点 shader 绘制改静默跳过（doFill 尾判）；
  `fillShaderAsImage` 补 lazy ensureGPU（进程首 GPU op=shader 接住）；管线建前拒 closed/nil。
  钉：`TestP32ShaderLazyFirstGPUOp`、`TestShaderPaintCPUSkips` 收紧。
- **D3（L3-A1）PathEffect 原型 conic GPU 描边 panic**：vello 加速路 `HasConics→ExpandConicsToQuads(0.1)`
  前置降换（与 CPU 两腿同防线）；effect 输出 conic 同降。钉：`p35_conic_effect_test.go`。
- **D4（L1-P1-1）DrawPaint/layer 路由 Linear 静默回退**：`Layer.blendSpace` 全链贯通
  （pushLayerSurface→compositeLayer/queueLayerAdvancedGPU→QueueAdvancedLayerComposite→dual-tex/CPU resolve），
  `BlendLinear` 按 `linear()&&IsAdvancedBlendMode` 门控。钉：`TestP31DrawPaintLinearGPUDomain`（CPU 档由 D5 钉守）。
- **D5（L1-P1-2）CPU 固定功能 Linear 换算式分叉**：`software_blend` 固定模式 Linear 一律降级 sRGB 域
  （对齐「降级契约」）；`p31_fixed_linear_test.go` 12 固定模式逐个钉降级值。
- **D6（L1-P1-3∪L2-P3-9）状态泄漏**：`withPaintFrozen` 值拷贝冻结（DrawImageEx CPU 腿/ShaderMask），
  Push/Pop 环境态不再被绘制动词改写；ShaderMask 补 BlendSpace 存取还原、ambient shader 不再劫持遮罩矩形。
  钉：`d6_state_leak_test.go` 三测。
- **D7（L1-P1-4）降级契约断言无判别力**：单测补判别对（Advanced 模式组）；真窗 C 卡改 25% 半透明黑
  （正确降级 153 vs 误线性 179）；p31 金图按拍板重采，双后端 0.0000%。
- **D8（L2-P1-3）pipeline 缓存首绘毒化**：BGL 从「首绘实时值」改**模块声明派生**（`ShaderInterface/ShaderBinding()`），
  uniform 布局=契约预算 256、bind 绑满 buffer，声明未绑走缓存级 1×1 黑占位（Dawn default-bindings 同法）；
  负向对照 A–D 逐腿归因（如实登记：布局腿与绑满腿须成对）。钉：`p32_pipeline_cache_test.go` 4 条。
- **D9（L2-P1-4）shader 输出 alpha 语义**：标定矩阵证实合成链按 straight 解释 pixelData——
  拍板（两轮，含实测翻案回报）：契约=premul 输出（Flutter 官方）+ 引擎 pass1 后 `premulToStraightRGBA`
  去预乘喂 straight 链；文档三处统一。钉：`p32_shader_premul_test.go`（A<1 行为+CPU 跳过）。
  **新洞登记**：兄弟路 `fillMaskedAsImage` 同样 premul→straight 双乘（GPU 半透明填充实测 (255,127,127)，
  CPU 正确）——归 L2-P2-7/P3-7 同族待单开。
- **D10（L2-P1-5）入口校验只到语法**：`validateShaderContract` 复用同趟 Lower 产物——fs_main 必须存在且
  vec4<f32> 返回、槽位表 0/1/2 固定（契约外直拒）、sampler 缺配对纹理直拒、uniform 预算 256 用
  `ir.TypeSize` 真测、全类型池扫 f64；对齐 Flutter `FragmentProgram.forShader` 编译期拒绝。
  钉：`p32_shader_validate_test.go` 8 条（含正例护栏）；负向对照 A/D 各红。
- **D11（L3-A2）透视降级残顶点**：`buildImageResources` 重排——降级决定（`degradePerspectiveCmds`）
  移到指纹+上传之前，`packPerspectiveQuads` 只管幸存透视束；白盒指纹一致性与最终字节对账。
  测试缝 `GPUI_TEST_PERSP_PIPELINE_FAIL`（仅测试注入）。钉：`d11_persp_degrade_test.go`；p35_persp 双后端 0.0000%。
- **D12（L3-A3）ClipRRect 双路不等价**：方向经两拍（B→F）：解析腿从「仅旋转」扩到「轴对齐非等比=
  椭圆真形、旋转等比=角度解析」，快路守卫维持严格均匀容差（存量像素逐位钉 F2）；
  `RRectClip.RadiusY/Angle` + `ellipseCornerDist`（牛顿投影+隐函数定号）+ bounds 像素中心余量 +
  谓词 mask 级路由 + 条目哈希补全。钉：`p35_cliprrect_paths_test.go` F1/F2/F3；负向摘腿旋转跳回 66/255。
  **存量登记**：p35_rrect4 go 后端金图 0.0038%（29px 1-LSB）与 HEAD 基线差异集合逐点相同=非本批引入；
  栅格 cubic 近似残差 ≤67/255（解析=真几何）。
- **存量 SIGABRT 对账**：D8 修后整包仍在 `TestContextFrameDamage_MultipleFillsSeparateRects` 同签名崩
  → 与 user shader 非同族，改登记为独立存量（vello accelerator 管线家族）待单开。

## D 批门禁（2026-11-03 实测，跑完补数）

- apidoc：`go run ./scripts/apidoc` rc=0（6 新符号入 catalog P3.2 行：Closed/注册对/ShaderInterface/ShaderBinding/ShaderUniformBudget）。
- build/vet：`go build ./...`、`go vet ./render/...`（含 internal/clip、internal/gpu）rc=0。
- 五目标：`build/` 5/5 全绿（D 批态重跑 2026-11-03 18:30，各 raw+分包+nogpu 三段 exit 0；`build/*.txt` 已刷新）。
- race：`-race ./render/internal/gpu/` 全包 **ok**（100.6s，最终代码态，0 DATA RACE）；
  `-race -run Shader|Blend|Layer|Clip|D12|DeviceScale ./render/` 定向两组 **ok**（49.3s/9.0s）。
- per-file：正本 34 包全量重扫（最终代码态 2026-11-03，`per-file-tests-dbatch/`）**750 = 719 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS**；
  对基线 741=711/21/8/1：新增 9 条 PASS 全为本批新钉（在册全绿），FAIL 名单差异仅
  `ui/rendering/m1_noquadratic_test.go` 一枚——**单跑复核为性能比例门禁抖动**（同码 `-count=3` 得
  15.7/8.8/10.6，阈值 15；D 批未触及 ui/rendering 与 BoxesForRange 任何路径），非新增逻辑红，
  登记 `findings-register.md`；`render/internal/gpu/` 整包 ok（20.2s、40.9s 两次）。
- 真窗：p31 双后端 0.0000%（重采后）、p32 双后端 0.0000%、p33 go 0.0000%、p35_rrect4 native 0.0000%
  （go 0.0038% 存量登记）、p35_persp 双后端 0.0000%——明细 `windows/window-runs.txt`。
