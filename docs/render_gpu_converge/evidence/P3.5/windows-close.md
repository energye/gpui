# P3.5 真窗收口（五卡能力窗口 + 窗口抓出的四处引擎洞）

日期：2026-11-02　分支：`feat/p2-收敛整合`　基线提交：`b93da030`（批 5 布尔保曲线）
范围：P3.5 五项能力（批 1 conic / 批 2 四角独立 RRect / 批 3 Matrix4 透视 / 批 4 PathEffect / 批 5 布尔保曲线）的**独立真窗**收口（评估表遗留项：五项能力只有原子验证，缺真窗）。

粒度拍板（用户）：**拆四扇独立窗**——一能力一窗，禁止用组合窗代替（AGENTS「每个主能力必须有独立真窗」）。布尔保曲线的真窗已有 `examples/ui_render_base_pathbool`（批 5 收口时改造成保曲线口径 + 金图重录，见 `batch5.md` §5），本轮不再新建第五扇。
人工验收拍板（用户）：**门禁由本会话跑完，常驻窗留给用户开**（命令见 §6）。

## 1. 交付：四扇真窗

全部走 `wrkit.ShellChrome` + `embedder.NewPipelineApp`（`PresentPolicyFullPaint` + WarmUp + 快照），三证据合证（逻辑探针 + 像素探针 + Golden 逐位），金图区间 = `wrkit.ShellContentRects()`（图例 + 身体，顶栏/HUD 不入比对）。

| 窗 | 能力 | 画面 | 逻辑 | 像素 | Golden |
|---|---|---|---|---|---|
| `examples/ui_wr_p35_conic` | 批 1 conic 真 verb | A 精确弧 vs 降二次链 vs 立方近似；B 径向误差 ×20 放大镜（含近似链与圆重合点）；C 权重族 w=0 退化弦…w=1.35 过冲 + 非均匀 Scale 后同一 conic | 8/8 | 10/10 | 逐位 0.0000% |
| `examples/ui_wr_p35_rrect4` | 批 2 四角独立 RRect | A 六态归约（Empty/Rect/Oval/Simple/NinePatch/Complex）；B 单角/多角半径逐一生效 + 与 RoundRect 等价对照；C ClipRRect 分流（uniform 快路 vs 精确路径裁剪） | 6/6 | 9/9 | 逐位 0.0000% |
| `examples/ui_wr_p35_persp` | 批 3 Matrix4 透视 | A 同一后退盘两次贴图：仿射双线性 vs 1/w 透视校正，青色量线同一屏幕高度读不同色带；B 深度阶梯 5 盘，末盘远边在眼后整块拒绝；C 恒等 / 提升仿射（平行四边形）/ 投影（梯形）+ bbox 空角 | 9/9 | 17/17 | 逐位 0.0000% |
| `examples/ui_wr_p35_patheffect` | 批 4 PathEffect | A dash/corner/discrete 三构造器（灰=原线）；B 1D 三形态 + 2D 网格；C Sum 并集 vs Compose 两序 + 密度护栏拒绝位（对照盘同框） | 10/10 | 23/23 | 逐位 0.0000% |

关键不变式（全部由窗自己算出、不由画面反推）：

- conic：w=cos(θ/2) 的精确圆弧对半径误差 ≤1e-9；降二次链在 tol 内（0.1px 预算）；仿射映射保权重逐点 ≤1e-9；四分之一盘面积对 πr²/4 相对误差 ≤1e-6。
- persp：`ProjectQuad` 对闭式解 ≤1e-9 且与逐角 `ProjectPoint` 一致；等权 `Perspective=true` 与仿射腿**逐字节相同**（332800/332800 通道零差异）；两模式在量线处实测差异 45840 通道、uv 位移 0.264；`Affine()` 对投影矩阵必须拒绝；`Invert` 复算 (37,−19,55,1) 误差 0.0。
- patheffect：dash 效果与 `ApplyDash` 逐 verb 逐坐标相同；直角切弧顶点→弧距 6.36px（r=18，六隅全等，r 折半距折半）；1D 盖章数 = 弧长 walk 的 `[offset,length)` 计数（11/11）且首章质心落位误差 0.000px；rotate 章边方向 = 该点切线（Δ=0.000 rad）而 translate 不跟；morph 逐点弧长+法向映射误差 ≤0.000px；Sum=并集且含两输入、Compose 两序不同（118 vs 100 verbs）；效果哈希参数折叠、值形态自定义效果=0（不可键控）；挂载腿驱动栅格且优先于 `Stroke.Dash`（2690 vs 2690，纯 dash 1687）。

## 2. 窗口抓出的四处引擎洞（全部在引擎层修，示例层零绕路）

真窗像素断言把「逻辑指标绿但画面对不上」的三处假绿抓了出来，另有一处 GPU 编排在批 3 门禁的离屏单组场景里不可见、只有真窗的多裁剪组帧才暴露。

### 2.1 `quadArea` 格林公式口径不统一（`render/pathops_area.go`）

- 现象：混合直边+曲边的路径面积被整体抬高（conic 窗四分之一盘探针复现）。
- 根因：`quadArea` 用 ∫x·dy 端点式，`lineArea`/`cubicArea` 用对称式 ½∫(x·dy−y·dx)，两者差一个 ½[x·y] 端点项；纯曲线环闭合成环时互相抵消，混直边就不抵消。
- 修复：`quadArea` 改对称式；口径与 dense 数值积分逐形状对账钉（`render/pathops_area_convention_test.go`：quad/cubic/conic 各自、圆角矩形混合、扇形、多形状批量）。
- 详见同目录 `area-fix.md`（用户拍板：单独提交）。

### 2.2 `Path.RRect` 方角仍发零长立方（`render/path_build.go`）

- 现象：rrect4 窗「只设左下角」卡——`Area()` 说圆了，栅格填出来是方的（角部邻弧覆盖丢失）。
- 根因：四角无条件各发一段 cubic，方角半径为 0 时是退化零长立方，留在栅格边表里污染相邻真实弧的覆盖。Skia `SkPath::addRRect` 只对圆角发曲线，方角是直边相接。
- 修复：`rounded(rad) = rad.X>0 && rad.Y>0` 才发 cubic（四角各自判定）。
- 钉：`TestRRectSquareCornersEmitNoDegenerateCubics`（单圆角→恰好 1 段 cubic、0 退化、四角全圆→10 verb）+ `TestRRectSingleCornerRadiiRasterizeRounded`（CPU/GPU 双后端角部像素：圆角必须留出、方角必须实心）。

### 2.3 `clipPathCoverageMask` 裁剪盒在画面外时越界切（`render/internal/gpu/depth_clip.go`）

- 现象：rrect4 窗 `ClipRRect` 精确路径裁剪腿在裁剪盒整体位于表面对外时 panic（slice bounds out of range）。
- 根因：`x0,x1 := max(0,offX), min(w,offX+mw)` 之后没有判空区间，offX>w 时 `x0>x1` 直接进循环切片。
- 修复：`x0>=x1 || y0>=y1` 时直接返回全零覆盖掩码（盒不在画面上=处处无覆盖，正是正确结果，深度测试随后拒掉全部内容）。
- 钉：`TestClipPathCoverageMaskOffSurfaceBBox`（四个整体出界框断言掩码长度=w*h 且全零 + 擦角不入 panic 且重叠区保留 + 面上对照）。

### 2.4 透视 quad 被按「仿射顶点下标」分进裁剪组，整批错位一格（`render/internal/gpu/{image_pipeline,render_session}.go`）【本轮新洞】

- 现象：persp 窗 B 卡深度阶梯第一盘（far=40）在 GPU 快照里**只剩轮廓描边，贴图整块不见**；把 B 卡绘制顺序换成 `[1,2,3,0,4]`，消失的仍是「命令下标 2」那一格——**消失跟下标走，不跟几何走**。
- 定位链（真数据，非猜测）：临时在 `buildImageResources`/`recordGroupDraws` 打点，拿到本帧 6 条图像命令（1 仿射 + 5 透视）、`perspIdx=[-1 0 1 2 3 4]`、各组 `img=[start,count)` 与 `firstVertex` 绑定；`sliceImageResources` 以 `firstVertex∈[cmdStart*6, cmdEnd*6)` 判归属，而透视槽的 `firstVertex=pIdx*6` 是**另一套下标**（仿射命令 0 混在前面时恰好错开一格）：
  - 组 [0,1) 抓到两条（仿射槽 fv=0 与透视槽 fv=0）；
  - 组 [1,1) 抓到命令 2 的几何、组 [2,1) 抓到命令 3 的几何……每格都用**上一格**的顶点画；
  - 最后一格 `[5,1)` 什么都抓不到，`imageRes=nil`；
  - 于是各透视盘的贴图被提前画到「前一组」的绘制上下文里，被后画的卡背景覆盖——第一盘整块不见，其余四盘因为落在自身背景之后而侥幸可见。
- 标准依据：Skia/Flutter 的显示列表按录制顺序 + 各自的 clip 设备绘制，绘制命令与其所属裁剪组是一对一的；本仓既有约定也是「组 = 该命令区间的裁剪状态」。修法因此是**按命令区间分派**，而不是给透视单开一条分派路。
- 修复（最小、单一真源）：`imageDrawCall` 加 `cmdStart/cmdCount`（该次 draw 覆盖的图像命令区间），两处生产者（`buildImageResources` 的仿射槽/透视槽、`buildGPUTextureResources` 的合并槽）都填上；`sliceImageResources` 改按 `cmdStart/cmdCount` 判定，顶点缓冲语义不变（仿射仍 `firstVertex=i*6`，透视仍 `pIdx*6` + 独立 36B 缓冲）。仿射/GPU 纹理路径命令区间与顶点区间历来重合，行为逐字节不变；只有混入透视命令的帧被纠正。
- 钉：
  - `render/internal/gpu/perspective_quad_test.go: TestSliceImageResourcesGroupsByCommandIndex`——混合命令列表按组切片，逐组核对落入的 draw；含「合并仿射槽只在整段命令入组时落入」与「透视命令不得被前一格抓走」。
  - `render/internal/gpu/image_batch_s41_test.go: TestS41_SliceImageResourcesByCommandRange`（原 `…ByVertexRange`，命令区间显式化，语义不变）。
  - `render/matrix4_quad_persp_batch_test.go: TestQuadPerspectiveLadderEveryPlateLands`——一仿射 + 五透视多 quad 帧离屏重放，逐盘量线色带对闭式解，GPU/CPU 两腿同断言。
  - 真窗 `ui_wr_p35_persp` 像素探针 `B_plate0_far40_band2`（修复前 FAIL：读到卡底色；修复后 ok）。
- 顺带钉住的批 3 不变式（同一窗逻辑探针）：等权 ≡ 仿射逐字节、`ProjectQuad` ≡ 闭式、behind-eye 整 quad 拒绝、非法权重/bicubic 哨兵错。

## 3. 负向对照（证明钉是真钉）

摘掉修复 → 钉必须红；还原 → 必须绿。

| 轮 | 操作 | 结果 |
|---|---|---|
| 1 | `sliceImageResources` 还原为「按 firstVertex 判组」 | `TestSliceImageResourcesGroupsByCommandIndex` **红**：`group [0,1) = [0 0], want [0]`（透视 draw 漏进前一格，正是真窗现象的机读版） |
| 2 | 还原本次修复 | **绿**（同一条用例，无其他改动） |
| 3 | 真窗 `ui_wr_p35_persp` 修复前/后各一轮 | 修复前 `B_plate0_far40_band2` FAIL（GPU 快照只剩描边），`GOGPU_RENDER_MODE=cpu` 同窗 17/17 全绿（证明洞在 GPU 编排不在几何/效果本身）；修复后 GPU 17/17 + 金图逐位 |

## 4. 门禁

| 门禁 | 结果 | 日志 |
|---|---|---|
| apidoc 机器校验 | **rc=0**（本轮零公开 API 变化，四处修复全在包内非导出符号/行为） | `apidoc.txt` |
| `go vet`（render / render/internal/gpu / 四扇窗包，共 6 包） | **6× rc=0** | `vet.txt` |
| 定向 | `./render`（面积口径/RRect/conic/布尔/Matrix4/透视 quad 组）ok；`./render/internal/gpu`（透视/分片/depth_clip/图像批）命中 **54 条 PASS** ok | `targeted.txt` |
| conformance 双后端 | **PASS**（`TestConformanceDualBackend` 单用例内部对拍 CPU/GPU 两腿） | `conformance.txt` |
| `CGO_ENABLED=1 -race -count=2` | `render/internal/gpu` **ok** 61.59s、`render/recording` **ok** 1.81s；`./render` 整包 = 批 4 已登记的既有崩溃（HEAD `b93da030` worktree 同命令同一处 `src/conv.rs:519 invalid bind group layout` 非展开 panic 崩溃 → 非本批引入；门禁口径逐文件分进程时该包内所有文件按下表逐名绿/落基线红名单） | `race/race.txt`、`race/race-render-full.txt`、`race/race-render-ab-head.txt` |
| 主逐文件（39 包清单） | **698 PASS / 21 SKIP_TAGGED / 9 FAIL / 1 NO_TESTS**（729 行 = 批 5 基线 727 行 + 本轮新增 `render/pathops_area_convention_test.go`、`render/matrix4_quad_persp_batch_test.go` 两枚均 PASS；消失行 0） | `per-file-tests/`、`per-file-tests-run.log` |
| 补充分包 | **72 PASS / 2 SKIP_TAGGED / 6 FAIL**，6 枚全为 HEAD 既有坏包 `render/recording/backends/raster`（P3.1 线债，本批未动） | `per-file-tests-extra/` |
| 五目标构建（per-package + nogpu，CGO_ENABLED=0） | **5/5 目标各 2/2 绿**（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64；三份 HEAD 既有坏包照旧排除：`ui_wr_t_stress`/`scene/internal/gpupixel`/`ui_render_base_recording`） | `build/*.txt`、`build-gate-run.log` |
| 负向对照三组（摘修复→钉红→还原绿） | 全红→还原绿 | `negative-control/README.md`、`race/race-render-ab-head.txt`（第 3 组=HEAD A/B） |
| 四扇窗 seed + verify | 逻辑 8/6/9/10 全绿、像素 10/9/17/23 全绿、Golden `seeded=false` 复跑逐位 **0.0000%** | `{conic,rrect4,persp,patheffect}-verify.{json,log}` |

## 5. 逐文件红名对账（基线 = 批 5 的 9 枚）

本轮 9 枚红名：`render/context_misc_draw_gpu`、`render/filter_present_coherence`、`render/rasterizer_tile_value`、
`render/s6_9_heavy_budget`、`render/text_transform_gpu`、`render/video_direct_upload`、`ui/application/app_test`、
`ui/rendering/m5_keystroke_g1`、`ui/scene/internal/gpupixel/blit_rrect_corner`。

- 与批 5 名单差两枚，同族互换：基线的 `ui/rendering/m1_noquadratic` 本轮**转绿**，`render/s6_9_heavy_budget` 本轮**复现红**。
  两者都是批 3/4/5 已按同机 clean-HEAD A/B 定性的**墙钟 p50 比率门贴线漂移**（亚毫秒 p50 上的 ±10% 门比计时噪声还紧，测试文件自己的注释即此）。
  本轮 standalone 复跑三连：`s6_9` 第 1 轮 ok、第 2/3 轮 FAIL，失败项 `H04_TextRows40 ratio=1.858`（s6.0=0.56ms vs s6.9=1.03ms，绝对量 0.47ms）；
  `ui/rendering` 的 `TestM1|TestM5` 同命令复跑 ok。→ 环境时序，非逻辑回归。
- 其余 7 枚与基线逐名一致（`rasterizer_tile_value`/`text_transform_gpu`/`video_direct_upload`/`context_misc_draw_gpu`/`filter_present_coherence`/`app_test`/`blit_rrect_corner`）。
- 五目标构建 5/5 各 2/2 绿（`build-gate-run.log`；脚本副本 `build-gate-windows.sh` 首版把 `ROOT` 的相对层数算错，产物落进 `docs/docs/…` 且因 `go list ./...` 空集而虚报绿——已改正路径重跑，本报告数字来自重跑后的 `build/`，误产物已删）。
- **零新增逻辑红**；金图类文件 `render/internal/gpu/golden_test.go`、`golden_software_test.go` 本轮均 PASS
  ——四处引擎修复里只有透视分组会影响画面，而它只改「透视命令落进哪个裁剪组」，存量窗/金图无透视贴图，故逐位不变（四窗复跑 0.0000% 即此证据）。

## 6. 人工验收窗（常驻，等用户开）

```bash
cd /home/yanghy/app/projects/gogpu/gpui
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_p35_conic        # 光标沿精确弧扫掠
go run ./examples/ui_wr_p35_rrect4
go run ./examples/ui_wr_p35_persp        # 青色量线沿盘高扫掠（两盘同高读不同色带）
go run ./examples/ui_wr_p35_patheffect   # 静态，常驻等看
```

一轮一窗、WM_DELETE 正常关闭、每轮查显示服务（本会话四窗验后 `Xwayland` 进程计数恒为 1，无异常）。
