# P3.6 批 2：GPU mipmap（2026-11-02 交付，已提交 `e0684738` 未推送）

拍板：D11 render 出真源 / D12 两侧按级精确求和 / D13 条目键带级数 / D14 采样器三处全切 / D15 CPU 兜底统一到一份链 / D16 旧门不动、另开 mip 专测。评估 `batch2-assessment.md`，方案 `batch2-design.md`。

## 已完成（步骤 1–4，代码树内可编译，触及四包测试全绿）

| 步 | 内容 | 关键位置 |
|---|---|---|
| 1 | 真源枚举 `MipmapMode{None,Nearest,Linear}`（零值 None=今日行为逐位不变）；公开口 `DrawImageOptions.UseMipmaps bool → Mipmap MipmapMode`；`QuadDrawOptions`/`AtlasSprite` 同挂；`engine/tex.MipMode` 改别名（其 `Normalized` 仍把零值→Nearest，tex 行为不动） | `render/internal/image/mipmap.go`、`render/context_image.go`、`render/internal/image/draw.go`、`render/m4_quadclass.go`、`render/vertices.go`、`engine/tex/mipmap.go` |
| 2 | 链缓存 `MipChainFor`：按内容代际一条链、single-flight（并发同代际只建一次）、引用持有（在读的被淘汰也不释放）、靠批 1c 的 `AddGenIDWatcher` 在换代/Dispose 时精确归还池；CPU 兜底改吃这份缓存（顺带根除"每次绘制重产链 + 从不 `Release()`"的泄池） | `render/internal/image/mip_cache.go`、`draw.go` |
| 3 | 条目键 `(代际, 级数)`；多级创建 + 逐级上传（取自同一份链）；字节按级求和，与 hal 账同源 | `render/internal/gpu/image_cache.go`、`gpu/hal/mip_math.go`、`gpu/hal/vram_ledger.go` |
| 4 | 采样器按档绑定（`SamplerForFilter` 等三处死代码转正）；`ImageSamplerKey.MipmapLinear bool → Mip MipmapMode`；绑定组缓存判据由 `nearest` 换成完整采样键 | `render/internal/gpu/image_pipeline.go`、`render_session.go`、四个 Queue 口与 24 处调用点 |

端到端证据（正向钉 `TestR1_ImageMipmapUsesGPU`，显存读回）：8× 缩小的噪声图，`None` 离中性 **71.6**（双线性抽到个别纹素=混叠），`Linear` **0.5**（盒式预滤波≈均值）。此前该钉两档逐位相同（=71.6/71.6），正是"公开口撒谎"的实测形态。

## 本批新抓的两处底层洞（都有实测）

1. **`LodMaxClamp=0` 在两个后端含义相反**：gles 把 0 当"不夹上限（32）"（`gpu/gwgpu/gles/sampler.go:46-52` 显式注释），rwgpu 直接透传 0 给 wgpu-native（`gpu/rwgpu/sampler.go`）→ LOD 被夹死在 0 级。后果：**即便有人把 mipmap 接起来，native 后端也会静默只采样第 0 级**，而 GL 后端正常——同一份描述器两种画面，双后端逐位一致的前提直接被破坏。修法：hal 文档定死"0=不夹上限"，rwgpu 与 gles 对齐，图像采样描述器显式写 `LodMinClamp:0, LodMaxClamp:32`。
2. ** variadic 吞参数**：删 `contentDirty` 形参后，`render/vertices_atlas.go:296` 那句 `..., nearest, false, bicubic)` 里残留的 `false` 被 `bicubic ...bool` 吸收，变成"bicubic 永远关"且**编译不报错**。教训：全量切换删参数时，凡结尾是 variadic 的签名必须逐个调用点核对尾参，编译器不兜这个底。

另记一处我自己的方法论错（如实登记）：第一次判"两档相同"用了 `Context.Image()`——headless 下它读的是 CPU 面，不是显存；GPU 行为必须走 `ExportImageBuf` 读回（`context_draw.go:21`）。

## 附带修正

- CPU 链生成 512×512：**17.6ms → 1.71ms**（128² 1.39→0.14ms）。原因：旧 `downsample` 每输出像素调 4 次 `GetRGBA` + 一次 `SetRGBA`（每次一把锁）。新实现逐行读写，**逐位不变**，由 `TestDownsampleByteIdentical` 用原逐像素实现作独立参照对拍（7 种格式 × 7 种尺寸，含 1×1、1×4、4×1、NPOT）。
- 层数公式：`min` 写法（design.md 旧记）订正为 `1+floor(log2(max(w,h)))`，与 Impeller 和仓内两份实现一致；级数与逐级尺寸由 `gpu/hal/mip_math.go` 单点提供，CPU 链、纹理创建、显存账三处共用。
- 池化：链的各级仍走 `GetFromDefault/PutToDefault`；`MipChainFlush`/统计口留给门禁。

## 步骤 5：门禁终局（`batch2/` 各日志，冻结树一轮跑完）

| 门禁 | 结果 |
|---|---|
| `scripts/apidoc` | **rc=0**（`apidoc.txt`；批 2 行已入 catalog §3.5，§0 计数 go/ast 重测 152/192/219/18/197/28） |
| `go vet` 触及八包（`render`、`internal/image`、`internal/gpu`、`recording`、`ui/io`、`gpu/hal`、`gpu/rwgpu`、`engine/tex`） | **全部无输出=干净**（`vet.txt`） |
| 定向 | `./render` ok、`./render/internal/image` ok、`./render/internal/gpu` ok（三档端到端 + 级间选择 + 级数/字节 + 采样描述器 + 链缓存 + 逐位对拍，`targeted.txt`） |
| conformance 双后端 | **ok**（`conformance.txt`） |
| `-race -count=2` | `internal/image`、`recording`、`ui/io`、`engine/tex`、`internal/gpu`、`render` 六组 ok；**`gpu/hal` 三条红已归因为该包测试非 `-count=2` 安全**（干净树 `82ab30ef` A/B 同红同线号，根因：`registry_test.go:66` 全局注册不撤销 + 同进程二次枚举适配器为 0），改按 `-race -count=1` 判据 → **PASS、FAIL 行数 0**（`race/race.txt`、`race/hal-count1.txt`、`race-attribution.md`） |
| 五目标 ×{默认,nogpu} | **10/10 绿**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64；`build/`、`build-gate.log`；口径同批 1c：排除三处存量坏包） |
| 主逐文件回归（34 包 **734** 条） | **PASS 704 / SKIP_TAGGED 21 / NO_TESTS 1 / FAIL 8** |
| 与批 1c 基线对账（731 条 = 701/21/1/8） | **FAIL 名单逐条相同、零新增红**；条目 731→734 与 PASS 701→704 的差恰为本批新增三个测试文件（`mip_cache_test.go`、`mipmap_downsample_test.go`、`image_cache_mip_test.go`）（`baseline-compare.txt`） |
| 补充逐文件（80 条） | **PASS 72 / SKIP_TAGGED 2 / FAIL 6**，6 枚与基线逐名相同（P3.1 线 raster 存量坏包） |
| 存量金图零变化 | 主/补充两轮 FAIL 名单与基线逐名相同 ⇒ 无金图/像素回归。原因如实写明：链已统一（D1c 的 CPU 兜底与 GPU 同源），但**现无任何生产调用方把档位设成非 `None`**，故逐位不变是构造性的；档位真生效的像素证据在上面的显存读回钉与批 3 真窗 `ui_wr_p36_mipmap`。 |
| 负向四路 | 全部红→绿成立，逐测试判定（`negative.txt`、`mutate.py`）：N1 级数恒 1、N2 建多级只填 level0、N3 摘按档绑采样器（只有 `TestR1_MipLevelBlendSelect` 能判——第一轮它全绿，据此补钉）、N4 显存账退回 level0 |

## 我自己制造的一次污染与修复（不藏着）

门禁脚本是照批 1c 的脚本复制改写的，`build-gate.sh` 里的 `OUT=` 只替换了 `batch1b` 这种旧写法，**没覆盖到 `batch1c/build`** —— 结果批 2 的五目标构建把**批 1c 已入库的 `batch1c/build/*.txt` 覆盖了**（时间戳 21:40、head `a1567cc9`）。

处置：① 把这 5 份产物搬到 `batch2/build/` 并把头部标签改回真话（`P3.6 batch 2 build gate`、`批 2 working tree`）；② `git checkout -- batch1c/build/` 还原批 1c 的原文件（已核对头部回到 `19:34:56 / head 4d4dffdf`）；③ 修 `batch2/build-gate.sh` 的 `OUT` 与标题，使下一次重跑写到自己目录。教训：照抄门禁脚本时，凡硬编码路径要逐行核对，`sed` 的旧写法匹配不到就是静默不替换——和批 2 那条 variadic 吞参数是同一类"不报错的错"。

顺带把门禁脚本的组级记账也修了：`race rc=$?` 记的是块内最后一条命令的 rc，原先会被读成"整组 race 过了"，现改为逐组各记判定（`run-gates.sh` 注释与 `race-attribution.md`）。

## 剩余与归位（不在批 2 做）

1. 真窗 `ui_wr_p36_mipmap`（含三档并排摩尔/混叠可见差、CPU/GPU 双档同源、级数与字节对账卡）→ **批 3**，与 `ui_wr_p36_imgfmt`、`ui_wr_p36_animplay` 同批收口。
2. 生产接线（mip 档位从 `engine/tex.Filter` 经 `SamplerParams` 落到绘制口 + `ui/rendering` 图像节点）→ **批 1d**；在那之前 mip 与动图同为「已验证待接线」，台账不标生产。
3. `gpu/hal` 两条测试待办（注册不撤销、适配器枚举二次为 0）→ 归 hal 线，本批不顺手改别线测试，已在 `race-attribution.md` 写清行号与修法。
4. 动图逐帧重产链 ≈2.9ms/帧（512²）→ 差量上传/按需产链专项（容器已带帧矩形与 `DisposalRect`）。
5. `Pixmap` 源的 mip（暂退单级）、gles `glGenerateMipmap` 原生产链、`engine/tex` 与 `internal/image` 的层数公式仍各一份（tex 不 import hal，去重需先定层级）→ 均登记不偷跑。
