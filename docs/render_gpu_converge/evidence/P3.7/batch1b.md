# 批 1b · 透视腿补染色（2026-11-03 开工，2026-11-04 交付收口）

## 结论

`DrawImageQuad` 的 `QuadDrawOptions` 新增 `Tint`，**透视腿与仿射腿同时打通染色**：2.5D「按深度投影的精灵」与「被灯光染色」过去只能选一个，现在可以共存。零结构体=不染色 ⇒ 存量画面逐位不变（真窗双后端金图实测 0.0000%）。

同批附带修一个既有洞：`QueueImageDraw`/`QueueImageDrawTint` 的 variadic 尾参被 `false` 吞，**指定 bicubic 在 quad 仿射腿与图集染色腿一直静默实走双线性**——修法与钉见 §四 N5–N7、§六。

## 一、为什么这是小切口（评估期就核实过）

36B 透视顶点布局的第三个属性本来就是 `@location(2) tint vec4<f32>`（`image_pipeline.go:1248-1258`），`shaders/textured_quad_persp.wgsl:27` 也声明并传进片元——**布局与 shader 早就备好该通道**，缺的只是 Go 侧管道：`buildPerspVerticesInto` 把四个分量写死成 `1,1,1,1`，且 `QueueImageDrawQuadW` 根本没有 tint 形参（注释自陈「this entry never carries tint, blend, or bicubic」）。本批 **WGSL 与顶点布局零改动**。

## 二、改动

| # | 位置 | 内容 |
|---|---|---|
| 1 | `render/m4_quadclass.go:55-57,73-84` | 公开面 `QuadDrawOptions.Tint RGBA` + 新哨兵 `ErrQuadBadTint` + `quadTintFinite` 校验口 |
| 2 | `render/m4_quadimage.go:49-51` | 入口拒绝非有限染色（NaN/Inf 三分量任一）⇒ `ErrQuadBadTint`，不静默画错 |
| 3 | `render/m4_quadimage.go:82-115` | GPU 侧分流：**恒等 tint（零结构体）仍走原 `QueueImageDraw`**（存量逐位不变）；非恒等走 `QueueImageDrawTint`；透视走扩了 tint 形参的 `QueueImageDrawQuadW`。下发口径与图集腿一致：straight 因子按 `tr*ta` premul 存进顶点（`vertices_atlas.go:277-281` 同款） |
| 4 | `render/m4_quadimage.go:171,213,236-252` | CPU 腿 `drawImageQuadCPUSplit` 加 `tint` 形参并在 premul/straight 两分支按图集 CPU 同款公式乘入（`srcA = sa·ta·k`、`srcR = sr·tr·ta·k`…），保证 GPU/CPU 一致 |
| 5 | `render/context_damage.go:259-267` | `gpuContextOps` 接口 `QueueImageDrawQuadW` 加 `tintR,tintG,tintB,tintA`（**接口全量切换制**：唯一实现 + 唯一调用点同批改，不留双路） |
| 6 | `render/internal/gpu/gpu_render_context.go:1442-1495` | 实现填 `cmd.Tint*`（字段本就在 `ImageDrawCommand`，`image_pipeline.go:104-110`） |
| 7 | `render/internal/gpu/image_pipeline.go:1283-1291` | `buildPerspVerticesInto` 由写死白改为取 `cmd.Tint*`，零结构体退回 identity（与仿射孪生体 `:1343-1347` **逐字同式**） |
| 8 | `docs/RENDER_API_CATALOG.md:150` | `QuadDrawOptions{...Tint}` 字段清单 + `ErrQuadBadTint` 哨兵 + 批 1b 语义段（apidoc 首轮 rc=1 抓到缺登记，补齐后 rc=0） |

## 三、能力单测（八枚外部 + 一枚内部，先红后绿）

外部像素级 `render/p37_quad_tint_test.go`（复用 P3.5 批 3 的透视角与色带图，`GPUOps>0` 且 `CPUFallbackOps=0` 前置守卫防「靠 CPU 兜底蒙绿」）：

| 测试 | 判据 |
|---|---|
| `TestP37QuadTintPerspCPUGPUParity` | 四组 tint（含半透明 ta=0.5）下，透视 GPU 与 CPU 在稳定区逐通道 **≤1/255**（容差沿用 P3.5 批 3 声明：f32 clip-w vs f64 CPU） |
| `TestP37QuadTintAffineCPUGPUParity` | 仿射腿同口径 ≤1/255 |
| `TestP37QuadTintActuallyDarkens` | **防「两侧都忽略 tint」的空转绿**：同图同几何，`tint=0.5 灰` 必须把非白底取样点压暗（透视与仿射两腿分别判） |
| `TestP37QuadTintZeroIsIdentity` | 零结构体与显式 `{1,1,1,1}` **逐位相同**（diff=0），钉住存量不变 |
| `TestP37QuadTintRejectsNonFinite` | NaN/Inf 分量返回具名错、不画 |

外部像素级 `render/p37_quad_bicubic_test.go`（批 1b 附带 bicubic 修复的钉；判读一律 `ExportImageBuf`+`GetRGBA`）：

| 测试 | 判据 |
|---|---|
| `TestP37QuadBicubicActuallyEngages` | 仿射腿 bicubic 与 bilinear **逐像素分叉**（实测差异像素 11020）；GPU/CPU 稳定区逐通道 ≤8/255（显式声明，实测 1） |
| `TestP37QuadTintDoesNotChangeSampling` | 同一 bicubic 请求下白染色与零结构体**逐位相同**（采样核不因队列口变档）；且带染色的 bicubic 必须与 bilinear 分叉（防两分支都退双线性） |
| `TestP37AtlasTintBicubicEngages` | 图集腿（GPU 路恒走 `QueueImageDrawTint`）bicubic 与 bilinear 分叉（实测差异像素 11020）+ 白染色不改采样核 |

内部顶点级 `render/internal/gpu/p37_persp_merge_test.go::TestP37PerspVerticesCarryTint`：`buildPerspVerticesInto` 必须把 `cmd.Tint*` 写进 offset 16..27 六个顶点；零结构体写 `1,1,1,1` 且权重/几何不受影响。

> 测试文件自身在同一会话实测纠偏（交付前发现、未跑过就先修判据）：`diffStats` 的 `maxDelta` 更新写在 `d>0→break` 之后（恒 0 死代码）、CPU/GPU 对比通道对误写 `{g1,b2}`、helper `defer out.Dispose()` 会在返回前释放返回图——三处修正后全绿。

## 四、负向对照（七路，逐条实测，日志 `negative.txt`）

| 编号 | 破坏方式 | 结果 |
|---|---|---|
| N1 | 透视打包还原成写死白 | 顶点钉红（`vert[0] tint=[1 1 1 1] want [0.5 0.25 0.75 1]`）+ 透视 GPU/CPU 分叉钉红（最大通道差 127/255/191） |
| N2 | CPU 腿摘 tint | 透视 GPU/CPU 分叉钉红（同上量级） |
| N3 | 仿射腿强制走无 tint 队列口 | 仿射分叉钉红（127）+ 压暗钉红 |
| N4 | 摘掉入口非有限校验 | `非有限 tint={NaN 0 0 1} 未被拒绝` 红 |
| N5 | quad 恒等分支尾参还原 `nearest,false,bicubic`（variadic 吞参原始形态） | **两枚红**：`bicubic 与 bilinear 逐像素相同` + `白染色与零结构体画面不同（changed=11020）`；图集钉绿=隔离正确 |
| N6 | quad 染色分支尾参还原 | **两断言红**：`白染色与零结构体不同` + `带染色的 bicubic 与 bilinear 相同`；其余钉绿=隔离正确 |
| N7 | 图集染色分支尾参还原（`vertices_atlas.go:289`） | 图集钉红：`图集腿 bicubic 与 bilinear 逐像素相同`；其余钉绿=隔离正确 |

N5–N7 验证的是同一既有洞（`QueueImageDraw`/`QueueImageDrawTint` 结尾 `bicubic ...bool` 被按位置多传的 `false` 吞掉）的三个修复位，逐一还原逐一红、还原后复跑全绿。

## 五、门禁（日志 `gates/`、`windows/`；**bicubic 修复并入后全套重跑，本节即终态**）

| 门禁 | 结果 |
|---|---|
| `go run ./scripts/apidoc` | 首轮 **rc=1**（缺 `ErrQuadBadTint` 登记）→ 补 catalog 后 **rc=0**；catalog 增补 bicubic 修正注记后复跑仍 **rc=0** |
| `go build ./...` / `go vet ./render ./render/internal/gpu` | rc=0 / rc=0 |
| `-race ./render/internal/gpu` 整包 | **绿**（38.5s） |
| `-race ./render -run 'TestP37\|TestQuad\|TestM1\|TestD[0-9]\|TestR1'` | **绿**（7.8s）。注：`./render` **不加过滤**的单进程整包跑仍崩在 `conv.rs:519 invalid bind group layout`——与在册 **M4** 逐字相同（`TestContextFrameDamage_MultipleFillsSeparateRects`，`targeted-render.txt` 头部栈可查），非本批引入 |
| `go test ./render/internal/gpu` 整包 | 绿（23.4s） |
| conformance 双后端 | `TestConformanceDualBackend` **PASS**（`TestConformanceInner` SKIP=内层由双后端驱动，既有口径） |
| 五目标 cross build ×{默认,nogpu} | **5/5 全绿**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64；每目标 per-package + nogpu + raw 三项全 `# exit: 0`，脚本 `gates/build-gate.sh`） |
| `tools/per-file-tests.sh` 逐文件对基线 752=722/21/8/1 | **754 = 723 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS**：+2 全为本批新测试文件（`p37_quad_tint` / `p37_quad_bicubic`，均 PASS）；FAIL 九枚 = 基线八枚逐名相同 + `s6_9_heavy_budget_test`（在册墙钟 p50 贴线漂移族，本轮独立 3/3 红、**干净树 HEAD worktree 同命令同 3/3 红 ⇒ 环境红非本批引入**，与 P3.5 批 3/P3.6 批 3 的两树红绿互摆记录同族）⇒ **零新增红**。日志 `gates/per-file-tests/`（`*.log` 不入库，`summary.tsv` 入库，照 989f4ff8 先例） |
| **存量金图零变化（实测，不留到批 6）** | `examples/ui_wr_p35_persp` 双后端**各两连 verify**（一轮一窗、RunFor 到期 → app.Close() → win.Close() 正常关闭、每轮前后查 Xwayland 存活）：native 与 go 均 **logic 9/9 / pixel 17/17 / 金图 0.0000%** / 763584px（seeded=false，比对既有金图非重采），Xwayland 全程存活。**上会话「bicubic 修复会让该窗画面变化 ⇒ 金图重采」的预判实测收回**：窗内可见 quad 全走 InterpNearest（`blitPlate:536`），第 413 行 affine+bicubic 是 L9 逻辑探针的离屏一笔（只查错误码、不上屏不进金图）；A/B 差异图为空集，金图不重采。`gpu_ops` 每帧 ops 143.4 与上会话完全一致（差异纯帧数抖动）。日志 `windows/summary.txt` |

## 六、口径与登记

- **生产消费者**：`DrawImageQuad` 侧的 tint 是组件/引擎可用的能力口，但 2.5D 批量精灵走的是批绘腿（`AtlasSprite`/`DepthSprite`），其 tint 承载与投影四角要等 **批 1c** ⇒ 本批仍属「已验证待接线」，不当接线标绿。
- **附带修复的既有洞（同批交付，非新能力）**：`QueueImageDraw`/`QueueImageDrawTint` 结尾 `bicubic ...bool` 的 variadic 形态下，两处既有调用（`m4_quadimage.go` 恒等分支、`vertices_atlas.go:289` 染色分支）按位置写成 `nearest, false, bicubic`，实现读 `bicubic[0]` ⇒ 指定 bicubic 静默实走双线性（与 P3.6 批 2 在册「variadic 吞参数」同族，纪律原文：删形参时凡结尾 variadic 的签名必须逐调用点数尾参）。主图像路 `DrawImageEx`（`context_image_gpu.go:126`）写法正确不受影响。修法=全量切换到 `nearest, bicubic`（不留双路），三修复位各配一枚红过的新钉（N5–N7）；catalog `DrawImageQuad` 行加注。
- **上会话遗留拍板「persp 金图按拍板重采」的执行结果**：预判（画面会变）被本会话实测推翻（见 §五末行），按「核实后发现没问题的，如实收回判断」处理——金图不重采，四轮 verify 日志照实落 `windows/`。
- 登记不做：透视下的 blend 混合模式（`ErrQuadUnsupportedBlend` 保留）与 bicubic（`ErrQuadPerspectiveBicubic` 保留）；`QuadDrawOptions` 其余字段不动；录制/scene 侧的 tint 携带按 D9 归批 3e（本批 DrawImageQuad 未新增命令类型，`QuadDrawOptions` 是值拷贝直通，录制面已有的 quad 命令随批 3e 一并扩）。
- 与 M6 无交集（不触 `SetEffectSurface`/`ExportImageBuf`）。
