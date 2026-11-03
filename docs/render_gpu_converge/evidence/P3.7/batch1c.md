# 批 1c · 批绘腿吃「四角+权重」（2026-11-04，交付）

## 结论

`AtlasSprite` 新增 `Perspective/Corners/Weights` 三字段，**2.5D 批量精灵的透视腿打通**：过去「按深度投影的精灵」在批绘入口根本不存在（Depth 只排序不投影、四角无处放、`Projector.Project` 的 scale 全仓被 `_` 丢弃），现在引擎（`engine/camera.Projector.ProjectSpriteQuad`）算好投影四角+齐次权重喂入，render 批绘腿走 1a 合批后的 36B 透视槽（与 quad 透视贴图同槽）、CPU 腿走 `newTriSamplerPerspective` 1/w 有理插值。零值 `Perspective=false` ⇒ 历史仿射路逐位不变（真窗双后端金图实测 0.0000%）。

**职责划分对齐 Godot Sprite3D**（评估 §0，用户拍板 2.5D 优先对齐 Godot）：引擎生成顶点级数据（投影角/UV/逐顶点色）、投影属相机、渲染器只做透视正确插值——本批形态与 Godot 同构，零值仿射路与 Godot 2D canvas 的仿射-only 分层一致。

## 一、改动

| # | 位置 | 内容 |
|---|---|---|
| 1 | `render/vertices.go:77-107` | `AtlasSprite` 加 `Perspective bool` + `Corners [4]Point`（用户空间，CTM 照常应用）+ `Weights [4]float64`（齐次 w 恒有限 >0）；`true` ⇒ 忽略 Dst/Rot/Flip/Pivot（避免半仿射半透视混合解释） |
| 2 | `render/vertices.go:111-125` | 新哨兵 `ErrAtlasQuadDegenerate`/`ErrAtlasQuadBowTie`/`ErrAtlasQuadBadWeights`（按 quad 口命名对齐，拍板 D-1）；非有限角复用 `ErrAtlasNonFinite`、透视+bicubic 复用 `ErrQuadPerspectiveBicubic`（同一语义不造第二份枚举） |
| 3 | `render/vertices_atlas.go` | `atlasPerspValidate`（权重→组合→四角校验序与 DrawImageQuad 一致）+ `atlasDrawable` 透视规则（不读 Dst）+ `DrawAtlasEx` 校验循环扩透视组合 + GPU 循环透视分支（`QueueImageDrawQuadW` 传子矩形 UV/premul tint/`nearest`，bicubic 校验拒绝）+ CPU 腿 `drawAtlasExCPU` 透视采样分支 |
| 4 | `render/depth_r5.go:74-90` | `NewDepthSprite` 早拒绝扩透视校验（`DrawDepthSprites` 经拍平透传 `DrawAtlasEx` 统一生效，排序语义不动） |
| 5 | `render/vertices_atlas.go:20-26` | 老 `DrawAtlas` 文档写明「只读老字段，透视精灵必须走 DrawAtlasEx」（拍板 D-4，行为不改） |
| 6 | `engine/sprite/atlas.go` | `sprite.AtlasSprite` 加同名词三字段（角点承载已投影屏幕角）+ `Validate` 透视早拒（InvalidArg）+ `ToRender` 逐字段透传（`renderconv.Vec2ToRenderPoint`） |
| 7 | `engine/camera/project.go:110-139` | **新出口 `ProjectSpriteQuad(worldRect, depths[4]) → ([4]render.Point, [4]float64, ok)`**——`ProjectQuad` 第一个真消费者（评估期生产消费者=0）；权重=focal+depth；单深度调用=等权「按深度缩放」形态（`DepthToScale` 由此拿到真消费者）；behind-eye/非有限整体拒绝 |
| 8 | `examples/engine/camera/main.go:920-951` | 示例层绕路搬走：两处 `proj.Project(sh, 0)` 丢弃 scale 写死 80×60/60×28 的矩形改为按 scale 缩放（depth=0 ⇒ scale≡1 ⇒ 画面逐位不变，双后端 golden changed=0 实证） |
| 9 | `render/internal/gpu/image_pipeline.go:898-927` | **附带修复既有洞**（见 §三） |
| 10 | `docs/RENDER_API_CATALOG.md:209-211` | `AtlasSprite` 行登记新字段+三哨兵+批 1c 语义；`DrawAtlasEx` 行补批 1c 承载（apidoc 首轮 rc=1 抓到缺登记，补齐后 rc=0） |

**没动的东西（有意）**：WGSL、36B/32B 顶点布局、1a 合批谓词、`degradePerspectiveCmds` 降级语义（图集透视精灵降级后几何保持、UV 失透视校正，与 quad 路同）、仿射腿打包与发射逐字节、批 4 的 `ToRender` 补 `Mipmap`（登记不属本批）。

## 二、能力单测（外部七枚 + 内部一枚 + engine 两枚，先红后绿）

外部像素级 `render/p37_atlas_persp_test.go`（几何一律从 `ProjectorMatrix4.ProjectQuad` 同一份投影输出反推，GPU 读回走 `ExportImageBuf+GetRGBA`）：

| 测试 | 判据 |
|---|---|
| `TestP37AtlasPerspCPUGPUParity` | GPU vs CPU 真采样在梯形内部稳定区逐通道 ≤1/255（实测 1） |
| `TestP37AtlasPerspTrapezoidUV` | 同梯形同图源，透视批绘 vs 仿射 quad 在错切区必分叉（实测差异格 63、最大通道差 128——防 weights 被忽略退仿射的空转绿）+ AABB 两空角必须仍白底（四角承载几何、Dst 被忽略） |
| `TestP37AtlasPerspTintDarkens` | 透视腿 tint 通道在图集精灵生效（压暗方向判定，差异格 75） |
| `TestP37AtlasPerspZeroIsIdentity` | `Perspective=false` 时新字段全部被忽略（塞 NaN/Inf/负权重与不塞逐位相同） |
| `TestP37AtlasPerspSentinels` | 非有限角/零面积/自交/权重 0/负/NaN/透视+bicubic 七路具名拒绝 + 错误后不画；`NewDepthSprite` 坏权重早拒绝 |
| `TestP37DepthSpritesCarryPersp` | 深度口透视精灵与直接 `DrawAtlasEx` 逐格一致 + 排序序保持 |
| `TestP37ImageMixedOrderDraws` | **既有洞修复钉**：透视/仿射 × quad/图集四序全上屏（见 §三） |

内部顶点级 `render/internal/gpu/p37_persp_merge_test.go::TestP37AtlasPerspMerges`：图集透视命令（子矩形 UV+tint）与 quad 透视命令同图必合（1a 谓词不看 UV/tint）；真设备 3 张图集透视精灵 → 1 次 Draw（`LastImageBatchStats=(1,3)`、跨度 18、36B 缓冲）；子矩形 UV 逐角进 36B 顶点。

engine 侧：`engine/camera/camera_project_test.go::TestProjectSpriteQuadExit`（出口角点逐点 == Project、权重 == focal+depth、单深度 == DepthToScale 缩放矩形、behind-eye/NaN 整体拒绝）+ `engine/sprite/atlas_test.go::TestAtlasSpritePerspectiveToRender`（ToRender 逐字段透传、非透视时新字段惰性、Validate 坏角/坏权重早拒）。

## 三、附带修复既有洞：RecordDraws 顶点缓冲切回（混批丢绘制）

**现象**：一帧内透视图像命令在前、仿射图像命令在后（quad 路与图集路同病）⇒ **两笔全部不上屏**；仿射在前则正常。调试变体矩阵实测（同图同几何）：[透视,仿射] → 梯形=(255,255,255)、仿射=(255)（双丢）；[仿射,透视] → 全部正常。

**根因**（`render/internal/gpu/image_pipeline.go:912`）：`RecordDraws` 的顶点缓冲切换写反——`wantBuf := curBuf; if dc.vertBuf != nil { wantBuf = dc.vertBuf }`，仿射 drawCall 的 `vertBuf` 是 nil ⇒ 透视绘制切到 36B 透视缓冲后，仿射绘制**切不回** 32B 仿射缓冲，其 `firstVertex`（仿射缓冲索引）落在透视缓冲里越界 ⇒ wgpu 编码失效 ⇒ 整个 render pass 两笔全丢。修复=仿射显式切回 `res.vertBuf`（一行，注释写明形态）。

**归位与定性**：HEAD `2d0c284b` 干净树 worktree A/B 同命令同红 ⇒ **既有洞，非本批引入**；1a 单测只到 `buildImageResources` 层（该层对混排本就正确，内部调试实测两序各出 2 drawCall 且缓冲/偏移全对）未覆盖编码段，p35 窗全透视、stage_chase 全仿射、存量真窗无人混排故从未暴露。真实 2.5D 场景（透视精灵与仿射 UI/精灵任意序混排）必炸，是批 1c 批绘透视腿的直接前置，归本线本批修。修复钉=`TestP37ImageMixedOrderDraws`（修复前「透视在前」两序红，修复后四序绿）。

## 四、负向对照（四路 + 修复钉，逐条实测，日志 `negative.txt`）

| 编号 | 破坏方式 | 结果 |
|---|---|---|
| N1 | GPU 图集透视腿退回仿射队列口 | 梯形 UV 判别钉红（透视=仿射逐格相同） |
| N2 | CPU 腿摘透视采样器 | GPU/CPU parity 红（分叉 128 > 1） |
| N3 | GPU 下发等权 | parity 红（129）+ 判别钉红 |
| N4 | 摘权重校验 | 哨兵钉红（负/NaN/NewDepthSprite 三处 got nil） |

四路还原后复跑八枚全绿。修复钉负向见 §三（修复前混批丢绘制实测）。

## 五、真窗与离屏证据（`windows/`）

| 窗 | 结果 |
|---|---|
| `examples/engine/sprite`（DrawAtlasEx+DrawDepthSprites 直接消费者窗） | 双后端 `-auto-only`：native/go 均 parity=0.0000%、golden=0.0000%（56000px）、batch_calls=1、depth_sorted=1、cpu_fallback=0 ⇒ **存量金图逐位零变化实测成立**（零值=老路被像素证据钉住） |
| `examples/engine/camera`（示例绕路搬走窗） | 双后端 `-auto-only`：probes 全 ok、golden **changed=0**（wrote=false 比对既有金图）⇒ scale 捡回后画面逐位不变（scale≡1@depth0 解析恒等被实证） |
| 批 6 透视批绘窗 | 按 D7 归批 6（本批离屏钉为主，PLAN §9 口径） |

## 六、门禁（日志 `gates/`、`windows/`）

| 门禁 | 结果 |
|---|---|
| `go run ./scripts/apidoc` | 首轮 **rc=1**（缺三哨兵登记）→ 补 catalog 后 **rc=0** |
| `go build ./...` + `go vet` 触及包 | rc=0 / rc=0（render、internal/gpu、engine/camera、engine/sprite、examples/engine/camera） |
| conformance 双后端 | PASS |
| `-race ./render/internal/gpu` 整包 | 绿（33.6s） |
| `-race ./render -run 'TestP37\|TestQuad\|TestM1\|TestD[0-9]\|TestR1\|TestAtlas'` | 绿（9.0s） |
| `go test ./render` 整包单进程 | 撞在册 **M4** 同签名（conv.rs:519），照例按逐文件口径判 |
| `go test ./render/internal/gpu` 整包 | 绿 |
| 五目标 cross build ×{默认,nogpu} | **10/10**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64；每目标 raw+per-pkg+nogpu 全 `# exit: 0`，脚本 `gates/build-gate.sh`） |
| `tools/per-file-tests.sh` 逐文件对基线 754=723/21/9/1 | **755 = 724 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS**：+1=本批新测试文件 `p37_atlas_persp`（PASS）；FAIL 九枚=基线八枚 + 一枚同族互换（`s6_9` 转绿、`m1_paint` 转红：墙钟比率门贴线 314.87 vs ≤300，standalone 3/3 绿 + 干净树 HEAD 3/3 绿 ⇒ 环境红非本批引入，与 s6_9 在册两树互摆记录同族）⇒ **零新增红**。日志 `gates/per-file-tests/`（`*.log` 不入库、`summary.tsv` 入库，照 989f4ff8 先例） |

## 七、口径与登记

- **生产消费者**：本批后 `ProjectQuad`/`DepthToScale` 拿到第一个真消费者（camera 出口），示例层绕路（丢 scale 写死尺寸）已搬走；但 2.5D 窗与 stage_chase 仍走仿射——批量精灵透视的规模化接线在批 2（盖序承载）与批 6（透视批绘窗），本批状态=**已验证待接线**，不得当接线标绿。
- 登记不做：批 4 的 `ToRender` 补 `Mipmap`；老 `DrawAtlas` 行为变更（D-4 文档写明）；engine 出口的规模化消费者改造（stage_chase 接投影归批 2/批 6）。
- 与 M6 无交集；记录里新增无新池项（混批洞已在本批修掉并带钉）。
