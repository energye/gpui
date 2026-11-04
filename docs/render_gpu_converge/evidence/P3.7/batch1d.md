# 批 1d · Canvas 级 4×4 投影层（并存式，链序拍板 B）（2026-11-04，交付；提交 `1772257b`、未推送）

## 结论

`render.Context` 长出**第二层变换**：2×3 CTM 之外挂一层 4×4 投影层，四动词 `SetProjection/PushProjection/PopProjection/Projection` 进 `Context` + `Canvas`（接口 109→113）+ `recording.Recorder`（落 `ProjectionCommand`，回放逐位一致）。P3.7 六批里「画布级透视」这块地基到位：2.5D 可以 `SetProjection(相机投影)` 后按世界坐标画精灵，窗口级 2×3 变换照常叠在外层；**恒等（零值）投影 ⇒ 全部消费点走历史路径逐位不变**（三窗双后端存量零变化实证见 §五）。

**链序（本批最重要的一个拍板，B）**：`设备点 = deviceMatrix ∘ projection(4×4) ∘ matrix(2×3) ∘ 用户点`——模型几何先过用户矩阵，再过相机投影，最后叠设备缩放。定这条不是偏好而是收口：定稿前图像链按「投影最内」、路径链按 verb 期烘焙后的「投影最外」，同一矩形在非恒等 CTM 下两条链落点差 **3371 像素**（实测）。物理上 B 也与「世界→模型/视图→相机投影→视口」同序。因为本仓路径族在 verb 期就用 `c.matrix` 烘焙几何（`MoveTo` 等），投影**不折进** `totalMatrix()`，改由两个映射器分持：`projDeviceFromUser`（吃用户点：图像/quad/图集/视频/裁剪矩形口）与 `projDeviceFromBaked`（吃已烘焙点：路径/描边/裁剪路径）；纯仿射提升走 `projTotalAffine()` 合成 `deviceMatrix ∘ proj ∘ matrix`，与「把同一份仿射乘进 CTM」等效（钉 3 逐位实证）。

**标准对齐**：Skia `SkM44` 的限定域口径（允许 CTM 带透视，但路径/文字各有额外约束⇒ 并存式 + 显式表态与 Skia 实际形态一致）；2.5D 侧 Godot（`Transform2D` 仿射-only，透视内容引擎算好喂渲染器 = 本批形态）。评估正本 `batch1d-assessment.md`（D-1 投影并入 Push/Pop、D-2 文字跳过+计数、D-3 GPU 纹理/视频 AABB 降级+计数、D-4 录制面随接口合规自然进）。

## 一、改动

| # | 位置 | 内容 |
|---|---|---|
| 1 | `render/context_projection.go`（新文件） | 四动词 + 两个映射器（`projDeviceFromUser`/`projDeviceFromBaked`）+ `projMapQuad`/`projTransformPath`（先展平再逐点映射）+ `projTotalAffine`（唯一「2×3 能否表达」判定源）+ `projTotalMatrix4`（4×4 精确逆，ClipBounds 用）+ `withProjectedPathScope` + `projTextBlocked` + `projRestore`/`ProjectionStats`/`ResetProjectionStats` |
| 2 | `render/context.go:63-70` | 投影层状态（`projection`/`projectionActive`/`projectionStack`）+ **七枚具名计数**（`projClipDegrades`/`projPathExpands`/`projTextSkips`/`projGPUTexDegrades`/`projBrushDegrades`/`projMiscDegrades`/`projInterpDegrades`） |
| 3 | `render/context_layer_pass.go` | `Push/Pop` 与投影层同拍保存/恢复（拍板 D-1，Skia SkM44 风格）——**存归一化快照**（见 §三之 2） |
| 4 | `render/m4_quadimage.go` | `DrawImageQuad` 吃画布投影：用户四角经全链映射、权重齐次合成 `w = w_proj × w_own`、任一角眼后整 quad 拒绝（新哨兵 `ErrQuadBehindEye`）、bicubic 降 bilinear + 计数 |
| 5 | `render/context_image_gpu.go` | `tryGPUDrawImage` 非恒等投影 ⇒ 走 `QueueImageDrawQuadW` 透视槽；GPU 纹理/视频三口按 D-3 投影四角 AABB 降级 + 计数；眼后 ⇒ 不画且**返回 true**（返回 false 会让 video bridge 兜底再画一遍未投影画面） |
| 6 | `render/context_image_draw.go` | `DrawImageEx` CPU 兜底改走 `drawImageQuadCPUSplit` 透视腿；blend 非 Normal ⇒ 老 pattern 路 + 计数（图样采样是 2×3 的登记近似）；眼后显式拒绝 |
| 7 | `render/vertices_atlas.go`（三口） | 老 `DrawAtlas`、`DrawAtlasEx` GPU/CPU 两腿：仿射精灵在投影下**自动升格**为透视 quad；透视精灵权重与投影权重齐次合成；**喂投影口的必须是客户空间角点**（设备角会被用户矩阵乘两遍） |
| 8 | `render/context_draw.go` | `doFill/doStroke` 拆出 `*Unprojected` 变体：投影生效 ⇒ 路径展平+逐点映射成设备路径，以 identity 矩阵走既有管线（Skia 先变换后造型）+ 计数；眼后整笔拒绝 |
| 9 | `render/clip_op.go` `render/context_clip.go` | 裁剪五点的投影表态：矩形 ⇒ 投影直边 quad 路径裁剪（保直线⇒精确）；RRect/圆角 ⇒ 展平逐点映射；两条分析快路在投影下门拒绝（落精确路径，计数在上层） |
| 10 | `render/text_dispatch.go` `render/text_entry.go` `render/paragraph.go` | 文字七个公开入口 fail-closed：投影生效即不画 + 计数（链序 B 下文字族还没接映射器，宁可少画也不画「半条链吃投影」的错位画面；接映射器登记后续专项） |
| 11 | `render/brush_device_space.go` | 设备空间刷烘焙：表态排在恒等早退**之前**（否则恒等 CTM 下降级无人知晓，见 §三之 1），设备矩阵取 `projTotalAffine()` |
| 12 | `render/context_misc_draw.go` | `ClipBounds` 走 4×4 精确逆映射回用户空间；`DrawShadow` 抬升/模糊比例在真透视下按 2×3 + 计数 |
| 13 | `render/vertices_mesh.go` | `DrawVertices/DrawMesh` 顶点逐点投影映射（直线仍直，几何精确；逐顶点色仿射插值登记近似）+ `drawDeviceTriangle` 设备三角三层中性化（见 §三之 3） |
| 14 | `render/canvas.go` | `Canvas` 接口 109→113（四动词）；两实现（`*Context`/`Recorder`）同批原子切换（接口全量切换制） |
| 15 | `render/recording/*` | `ProjectionCommand{Mode,Proj}` + `CmdProjection` + `ProjectionMode{Set,Push,Pop}`；`Backend` 契约 38→39（+`Projection(mode,m)`）；`Recorder` 133→137、`CanvasBackend` 44→45；stub/spy/示例后端同批补齐（`examples/ui_render_base_recording/scene.go`） |
| 16 | `render/matrix4.go` | `Matrix4Homography(m, wx, wy, w1)` 地面单应性构造器（W 行写 `m[12..15]`，只动 W 行不覆盖平移）；`ErrQuadBehindEye` 入 quad 哨兵族 |
| 17 | 文档 | `docs/RENDER_API_CATALOG.md`：§0 两包计数、§3 头 225 方法、新 §3.15 投影族（含链序与表态表）、§1 `Matrix4`/quad 行、§6.3 录制面五行、§8 Canvas 113/MaskCanvas 118、§7.2 未接线行、§9 错误变量行、§11 附录 B 命令枚举回填、§13 P3.7 规划行转正；`go run ./scripts/apidoc` rc=0 |

**没动的东西（有意）**：`render/scene` 公开面（投影编码归批 3e）；WGSL 与 36B/32B 顶点布局（复用 1a/1b/1c）；文字 mesh warp 与文字族接映射器（登记）；GPU 纹理/视频的 quad 形态（队列口无 quad 形参，登记）。

## 二、能力单测（外部十五枚 + 内部三枚，先红后绿）

外部 `render/p37_projection_test.go`（十枚，判据全部从投影输出反推、GPU 走 `ExportImageBuf+GetRGBA`）：栈语义逐值 / **投影层 ≡ 1c quad 透视腿逐位一致**（同一数学两条入口）/ 眼后 `ErrQuadBehindEye` / `DrawImageEx` GPU-CPU parity（实测最大通道差 1，容差显式 ≤1/255）/ **图集仿射精灵升格 ≡ 1c 批绘透视腿逐位一致** + AABB 空角判别器 / 恒等逐位同旧 / 裁剪降级 ≡ 直接路径裁剪 / 路径填充 ≡ 投影 quad 填充 / 文字 fail-closed 不画（对照路 719 黑像素 vs 投影路 0）/ 录制含投影命令回放 ≡ 直接画逐位。

外部 `render/p37_projection_chain_test.go`（五枚，链序与观测面）：网腿只许映射一次（双投影指纹 1072 vs 一次映射 9920）/ 眼后 CPU 不画（0 vs 控制路 1024）/ 仿射提升 ≡ 乘进 CTM（路径 1240=1240、图像 2400=2400，逐位）/ 图像·路径·图集三链落点同序（非恒等 CTM 下 20 格点按链序 B 手算，三臂 0 错）/ 对外计数读口（恒等全零、四处 bicubic 逐一计数、复位交回旧值）。

内部 `render/p37_projection_internal_test.go`（三枚）：`projTotalAffine` 语义（恒等逐位 / 提升合成 / 真透视必须拒）/ 七具名计数逐项咬合 / **零值投影永不激活守卫**（Push-Pop 后照旧画得出蓝、`SetProjection(零值)` 当关闭、Pop 原样恢复、空栈 no-op）。

## 三、本轮抓到并修掉的三个洞（都带反证）

1. **刷烘焙降级被恒等早退遮蔽**：`applyDeviceSpaceBrush` 先 `totalMatrix().IsIdentity()` 早退，投影守卫排在后面 ⇒ 恒等 CTM 下非仿射投影的降级静默发生、计数恒 0。计数钉红（`p37_projection_internal_test.go:99`）→ 设备矩阵改取 `projTotalAffine()`，表态排在恒等早退之前。
2. **投影快照把画布污染成全零矩阵（存量级）**：`Push()` 同拍存的是 `c.projection` 原值，零值 `Matrix4` 的 `IsIdentity()` 为 false ⇒ 任何一次 `Push/Pop` 后上下文都变成「投影已激活 + 装着全零矩阵」，之后每次绘制被映射到空、画面画白。老状态钉 `TestD6DrawImageExNoAmbientLeak` 先叫（HEAD 绿 / 工作树红），在 HEAD 干净副本逐组回灌二分定位到 `context_layer_pass.go` 的 Push hunk。修复 = 存归一化快照（`Projection()`）+ `projRestore` 统一维护激活位 + `SetProjection(零值)` 按文档口径当恒等关闭 + Recorder 同改；新钉 `TestP37ProjectionZeroValueNeverActive` 直咬机制，反证见 `negative.txt` N10。
3. **CPU 网腿双投影**：`drawDeviceTriangle` 收到的已是设备三角，却在真透视分支里中性化矩阵后仍调 `c.Fill()` ⇒ `doFill` 沿投影再映一遍。实测画面（1072 像素）与「手算两次映射」指纹**逐位相同**、与一次映射的期望（9920 像素）差 10992。修复 = CTM/设备矩阵/投影层三层一起中性化，坐标一次都不再映射（提升态同规则，反证 N8）。

另两处如实收回：① 曾报「眼后图形 CPU 兜底画出 1762 像素」——复测证明那是我自己一处映射器笔误（把 baked 映射器用在用户点上）造成的画面位移，不是眼后拒绝缺失；显式拒绝保留为「不断言的防护」（N6 摘掉后全绿，不变量由下游 `doFill` 拒绝兜住）。② `projTotalMatrix4` 一度按旧链序写，改回 `deviceMatrix ∘ proj ∘ matrix`。

## 四、负向对照（十枚，最终代码上逐条红再还原）

`docs/render_gpu_converge/evidence/P3.7/batch1d/negative.txt`：N1 摘投影映射（8 钉红）/ N2 摘升格 / N3 摘文字跳过 / N4 摘路径映射 / N5 摘裁剪降级 / N6 摘眼后拒绝（**红钉 0 = 如实记为不断言的防护**）/ N7 摘链序 / N8 摘网腿中性化 / N9 摘图集用户角（唯一咬点是钉 4 图集臂）/ N10 摘投影快照归一化。每条改完都 `cp` 还原并 `md5sum` 逐字节核验、全仓 `grep -rn "// NEG" render/*.go` 零残留。

## 五、门禁与真窗（全绿，逐项可溯源）

| 门禁 | 结果 | 证据 |
|---|---|---|
| apidoc 覆盖校验 | 首轮 **rc=1** 抓到 12 个符号缺登记（`Matrix4Homography`/`ErrQuadBehindEye`/四动词 + `recording` 五符号）→ 补 §3.15/§0/§1/§6.3/§8/§9/§11/§13 → **rc=0** | `go run ./scripts/apidoc` |
| build + vet | `go build ./...` rc=0；`go vet ./render ./render/recording ./engine/camera ./engine/sprite` rc=0；gofmt 触及文件零脏（`context_layer.go`/`internal/clip/stack.go` 是 HEAD 既有脏，未碰） | 本会话实测 |
| 五目标交叉编译 | **10/10 绿**：linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64 × {默认, `-tags nogpu`} 全 exit 0；包数 linux 335/331、windows 341/337、darwin 333/329（比 1c 各 +1 = `examples/ui_render_base_recording` 从「已知坏」收回纳入正检，它在批 1d 补齐 `Backend.Projection` spy） | `gates/build/*.txt` + `gates/build-gate.sh` |
| `-race` | `./render/internal/gpu` 整包 ok 30.4s；`./render -run 'TestP37\|TestQuad\|TestM1\|TestD[0-9]\|TestR1\|TestAtlas'` ok 10.2s | `race.txt` |
| conformance 双后端 | `GPUI_BACKEND=native` PASS 0.85s；`GPUI_BACKEND=go` PASS 0.77s（`TestConformanceDualBackend`） | `conformance.txt` |
| 逐文件 sweep（34 包） | **758 = 726 PASS / 21 SKIP_TAGGED / 10 FAIL / 1 NO_TESTS**（基线 755=724/21/9/1；+3 全为本批新测试文件且三枚 PASS） | `gates/per-file-tests/` + `per-file-tests-run.log` |
| 新增 FAIL 定性 | 两枚，均**非本批引入**：①`mem_window_x11_linux`（`BeginFrame: hal: surface outdated`，sweep 串行长跑 load 6.6~9.8 时出现；本树单跑 3/3 绿 + 纯净 HEAD 3/3 绿）②`s6_9_heavy_budget`（H02/U05 p50 贴线 ~3%；空闲机整档复跑本树仍红，**纯净 HEAD 同族同样红**：HEAD run1 H02=7.75/门限 7.71、H06=0.40/0.31，run3 H06=0.34 ⇒ 台账在案的墙钟漂移族，批 1b 已记同名先例） | `gates/new-fails-ab.txt` |
| FAIL 名单换血 | −`ui/rendering/m1_paint`（回绿）＋上两枚 ⇒ 净 +1；与基线同族的互换照批 1b/1c 先例处理 | `gates/per-file-tests/summary.tsv` |
| 三窗双后端 | 六轮 rc 全 0、Xwayland 每轮前后均存活：**p35_persp golden 0.0000%（logic 9/9、pixel 17/17、cpu_fallback_ops=0）**、**sprite parity 0.0000%**、**camera golden changed=0**——恒等护栏的像素实证 | `windows-summary.txt` |
| 负向对照 | **十枚**在最终代码上逐条红再还原；七文件 `md5sum` 逐字节核验 + `grep -rn "// NEG" render/*.go` 零残留；还原后十八枚钉复跑 ok | `negative.txt` |

## 六、口径与登记

- 公开面：`Canvas` 109→113；`Context` 导出方法 219→225（+四动词 +`ProjectionStats`/`ResetProjectionStats`）；主包类型 154（+`ProjectionStats`）、顶层函数 197（+`Matrix4Homography`）、变量 33（+`ErrQuadBehindEye`）、常量 198；`recording` 顶层声明 98→100 / 符号名 180→186、`Backend` 38→39、`Recorder` 133→137、`CanvasBackend` 44→45。全部数字在 HEAD `b28ff695` 干净工作树与最终工作树各测一次做差（`negative.txt`/本节按此标注）。
- 生产口径：**已验证待接线**——投影四动词现仓零调用者；2.5D 场景级接线（地面/平面共用投影）随批 6 透视批绘窗承接，`render/scene` 投影编码归批 3e。
- 登记不做（后续专项）：文字族接投影映射器（提升态可精确跟上，现在 fail-closed）；透视文字 mesh warp；GPU 纹理/视频的透视 quad 形态；网腿逐顶点色的透视正确插值；批 6 前若要让真窗上报降级计数，读口已在（`ProjectionStats`）。
