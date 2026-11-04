# P3.7 批 3e（scene 投影编码）· 实施方案（2026-11-04，拍板后定稿）

依据：`batch3e-assessment.md`（scene 现状 + Godot 2D 命令面取证）+ 本轮三条拍板：
- **两个都做**：4×4 投影口按 Skia/Flutter 归**通用层**（`render/scene`），透视 quad 通道给 **2.5D 正路**（Godot「算好再喂」）；
  文档与钉都要写死「2.5D 玩法不调 scene 的 4×4 口」。
- **材质 item（节点）级为主**：Godot 实测材质挂 item（`rendering_server.h:867`，命令结构体只有 texture）；
  命令级材质若要做，必须标注为超出 Godot 的扩展 ⇒ **材质那半整块后移**（且依赖批 3a），本批不碰。
- **3e 先做投影那半，独立一枚提交**。

链序沿用批 1d 拍板 B：`设备 = deviceMatrix ∘ projection(4×4) ∘ matrix(2×3) ∘ 用户点`。
scene 侧**不自建组合**：投影交给 `render.Context` 的投影层（回放 `dc.Set/Push/PopProjection`），
场景变换继续走 `dc.SetTransform(base×scene)`（`gpu_renderer.go:101`）⇒ 两条链天然同序，
scene 里没有第二套链序可写错。

## 1 单一枚举：投影模式上移到 render 主包

现状：`recording.ProjectionMode{Set,Push,Pop}`（`recording/command.go:265-276`）。scene 也要同一三值 ⇒
不允许多份定义。做法（接口全量切换制，不留 alias 垫片）：
1. `render` 主包新增 `ProjectionMode` + `ProjectionSet/ProjectionPush/ProjectionPop`（放 `context_projection.go`）。
2. `recording` 删除自有类型，`ProjectionCommand.Mode`、`Backend.Projection(mode, m)`、`Recorder` 侧全部改用 `render.ProjectionMode`；
   `CmdProjection` 常量名不动。
3. scene 直接复用 `render.ProjectionMode`。
风险与验收：`go build ./...` 一次改完（原子切换）；`RENDER_API_CATALOG.md` 的 recording 计数行（类型 85、
常量 67、`Backend` 契约 39）与 render 主包计数同步实测重填；`go run ./scripts/apidoc` rc=0。

## 2 3e-1 scene 4×4 投影口（本批这一枚）

| 处 | 改动 | 锚点 |
|---|---|---|
| 编码流 | `Encoding` 新流 `projections []ProjectionEntry{Mode render.ProjectionMode; Matrix [16]float32}`；`NewEncoding/Reset/Clone/Hash/Size/Capacity` 全登记 | `encoding.go:417-461/464/480/798/1188/1264/1274` |
| 发射 | `EncodeProjection(mode, m)`（Set/Push 带矩阵，Pop 只带 mode）；**delta 编码**：与 `lastEncodedProjection` 同值不发（照 `emitAntiAliasIfNeeded` `scene.go:130-135` 的形态）⇒ 不调新动词的存量场景**零新 tag** | 新代码 |
| 合并 | `AppendWithImages`（无 default，必须补 case，否则索引静默错补）与 `AppendWithTranslation`（有 default panic，必须补 case）两处都要 `projections` 拼接 + tag case；`Clone` 同 | `encoding.go:913-950`、`:1041-1122`（panic `:1120`） |
| Scene 状态 | 字段 `projection/hasProjection/projectionStack/lastEncodedProjection`；动词 `SetProjection/PushProjection/PopProjection/Projection`；`Reset` 清栈；**零值→恒等、非有限忽略**（与 `Context.SetProjection` `context_projection.go:25` 及 `recorder_projection.go:21` 同口径） | `scene.go:30-109` |
| Decoder | `Projection() (render.ProjectionMode, render.Matrix4)` | `decoder.go:190` 一带 |
| GPU 回放 | tag case → `dc.SetProjection/PushProjection/PopProjection`（真投影，与 Context 一致） | `gpu_renderer.go:84` switch |
| CPU 回放 | tag case 维护本地投影态；`Matrix4.Affine()` 可表达 ⇒ 乘进逐命令烘焙链；真投影 ⇒ **具名计数 + 跳过该命令**（照批 1d 文字族 fail-closed，禁止静默压扁）；新增 `Renderer.Stats()/ResetStats()` 暴露计数 | `renderer.go:673` switch、`renderer.go:684-709` |
| 边界 | 结构钉：`engine/` 与 `ui/` 对 `render/scene` **零 import**（现状实测 0 命中）⇒「2.5D 不调 scene 的 4×4 口」不再是话术 | 新钉 |

## 3 3e-2 scene 透视 quad 通道（✅ 2026-11-04 已交付，交付记录见 §8）

- `Scene.DrawImageQuad(img *Image, corners [4]render.Point, opts render.QuadDrawOptions)`：**复用 render 的
  `QuadDrawOptions`**（含 `Perspective`/`Weights`），不新造第二套选项类型；`ui_render_base_scene` 的图像入口形态照搬。
- 新 `TagImageQuad = 0x52`：`pathData` 8×float32 四角 + 权重 4×float32 + 不透明度 1×float32（共 13 个），
  `drawData` = 图像索引 + 位标志（bit0=透视）+ 采样档（共 3 个 uint32）。**交付时按 `TagImage` 的真实约定改定**：
  四角在录制期就用 `currentTransform` 烘焙好（`TagImage` 是把 `currentTransform.Multiply(transform)` 烘进编码流，
  不是留给回放端再乘），回放臂把 CTM 复位到 base；本条原写法「交给回放端当前 2×3 变换作用」会让人二次施加。
- 四角凸包进 `RecordCommandBounds`（damage 追踪，`encoding.go:778`）。
- GPU 回放 → `dc.DrawImageQuad(buf, corners, opts)`（`render/m4_quadimage.go`），**权重/眼后哨兵由 render 判**
  （`ErrQuadBehindEye` 等），scene 不重复校验 ⇒ 不出现第二套判据。
- CPU 回放：仿射可表达（平行四边形）⇒ 复用现有仿射 blit（`renderer.go:843/1144`）；真透视 ⇒ 计数跳过。
- 登记：2.5D 侧（`engine/sprite`/`camera`）本批**不自动改走** scene quad（它们已有批 1c/1d 的直通腿），
  接线归批 6 真窗。

## 4 钉与负向（先红后绿）

3e-1 钉（新档 `render/scene/p37_3e_projection_test.go`）：
1. 恒等零发射：不调投影动词编码逐字节/逐位与改前**完全一致**（tag 数、`Hash()`、stream 长度全等）。
2. Set/Push/Pop 编码与解码回环：模式与 16 元矩阵取回一致；零值当恒等（不激活）。
3. GPU 回放 ≡ 直接画：同一画面走「scene 编码 + `RenderScene`」与「`dc.SetProjection` + 直接画」像素**逐位相同**。
4. 链序：scene 里投影 + 场景变换叠合后的落点 ≡ `device ∘ proj ∘ matrix ∘ user`（与 `render` 侧既有链序钉同判据）。
5. 合并：含投影的编码过 `Append/AppendWithTranslation` 后投影条目不丢、索引不串（`projections` 流长度与 tag 计数对齐）。
6. CPU 腿表态：仿射提升态 ⇒ 画得对；真投影态 ⇒ **不画且计数 +1**（不许画出错位画面）。
7. 边界钉：`engine/`、`ui/` 对 `render/scene` 零 import（结构性，go/parser 扫目录）。

3e-1 负向（逐条拆自己写的判据，必红）：摘 Scene 动词发射、`Hash` 不进 projections（LayerCache 串键）、
`AppendWithTranslation` 漏 case（应 panic 而非静默）、CPU 把真投影静默压扁成仿射（计数应为正却 0）、
零值归一化摘掉（Push 后恒等被当已装）。

## 5 门禁（按批口径）

`go build ./...`、`go vet ./render ./render/scene ./render/recording`、`gofmt -l`（本批文件净）、
scene/recording 两包全测试单跑、`render` 投影族定向（`TestP37Projection*`）复跑不回退、
五窗像素（批 2 那五扇）+ `ui_render_base_scene` 与四扇 render_scene* 示例跑通、
主 sweep 逐文件对基线只减不增、`go run ./scripts/apidoc` rc=0、
目录同步（`RENDER_API_CATALOG.md`：render 主包 +`ProjectionMode`/三常量、scene 新方法与 tag、recording 类型迁移）。

## 6 登记（不假装做完）

- scene 至今零生产消费者 ⇒ 交付后仍是「已验证待接线」，台账不得标生产。
- 材质编码（含 item 级 `scene.Node` 材质挂载）后移，随 3a–3d；`design.md` §4.5 的「材质随保留命令编码」一句
  按 Godot 改判为 item 级为主（本批落文档）。
- **GPU 腿的 quad 拒画没有具名计数**（3e-2 发现）：`render.Context.DrawImageQuad` 对自交/退化/眼后/坏权重返回哨兵错，scene 回放臂按本文件既有风格（图索引越界、字体缺失都是静默跳过）把错吞掉 ⇒ 画面是「一格不画」（`p37_3e_quad_test.go` 的蝴蝶形钉实测），但 CPU 腿那一侧的同类拒画有 `Renderer.ProjectionDegrades` 计数，两侧观测面不对称。接线（批 6）前补一个与 CPU 腿同名的 GPU 侧具名计数，或把两腿的表态统一到同一个读口。
- scene 编码无版本号/魔数、纯内存双流 ⇒ 「编码流演进」的兼容靠 tag 穷举绊线（`AppendWithTranslation` default panic）
  与 delta 零发射保证；若将来要落盘，需要另立版本化设计（不在本批）。

## 7 步 3e-1 交付记录（2026-11-04，已提交 `1a84bd80`、未推送）

**做了什么**：`ProjectionMode`（Set/Push/Pop）从 `render/recording` 上移到 `render` 主包（scene 与 recording 共用一份，
不留 alias 垫片；`Backend` 契约 39 与 `Recorder` 137 方法数不变）；`Matrix4` 加 `Elements()`/`Matrix4FromElements`
成对公开存取口（float32 编码流要完整往返一份矩阵，不能去摸未导出字段）；`render/scene` 得到 4×4 投影口：
`Scene.SetProjection/PushProjection/PopProjection/Projection` + 新 `TagProjection`（**与绘制同流**、粘滞状态、
Set 按流当前值 delta 去重）+ `Decoder.Projection()` + GPU 回放转交 `render.Context` 同名动词（链序仍只在 render 一处）
+ CPU 瓦片腿显式表态（仿射可表达 ⇒ 折进烘焙链精确画；真投影 ⇒ 整笔拒画 + `Renderer.ProjectionDegrades` 计数）。

**钉 7 枚**：外部 `p37_3e_projection_test.go` 四枚（恒等零发射 + Hash 不漂 + Hash 鉴别 / Set-Push-Pop 编码回环 +
空栈 Pop no-op + 非有限忽略 / GPU 回放 ≡ 直接画 + "装投影与不装必须不同"的反向鉴别 / 合并三口不吞流且不注入复位）
+ 边界结构钉一枚（`engine/`、`ui/` 对 `render/scene` 零引用）；包内 `p37_3e_cpu_test.go` 三枚（仿射提升照画且不计数、
真投影拒画必计数、`cpuProjectionState` 三态不含糊）。

**负向 E1/E2b/E3b/E4/E5b/E6/E7 全红**（首轮两条无效变异如实记在 `batch3e/negative.txt`）；
E3b 顺带证明既有穷举绊线在位：漏登记新 tag 会 `panic: unhandled tag 0x03 (Projection)`，不会静默错并。

**被自己的钉打回两次**（改实现不改判据）：
① 投影 tag 起初写进根 `s.encoding`，而绘制写进 `currentEncoding()` 的基层编码 ⇒ `Scene.Encoding()` 拉平时根 tag
被排到全部内容之前，"先画后设"成了"先设后画"；同时我照抄变换流的绝对性，在合并边界补发 `Set(恒等)`，
把父投影整段取消（实测画面差 2350 像素）。正解：与绘制同流同序、合并逐条照抄、投影是粘滞状态不复位。
② Scene 侧与流侧各写一道去重 ⇒ 流侧那道走不到（E2 首轮"仍绿"暴露）；且 `NewEncoding` 没把 `lastEncodedProjection`
初始化成恒等，零值 `Matrix4` 被当成一个真投影值，第一次 `SetProjection(恒等)` 白发一枚 tag（Hash 漂，
会被 LayerCache 串键）。两处改完 E2b 转红、Hash 不漂钉复跑绿。

**门禁**：`go build ./...` rc=0；`go vet ./render/scene ./render/recording ./render` rc=0；本批所触文件 `gofmt` 净
（`render/context_layer.go` 等 HEAD 外在脏按约定未碰）；`render/scene` 全包 ok 0.14s、`render/recording/...`（含 raster）ok、
`render` 投影族/盖序钉/深度族 ok、`engine/camera` `engine/sprite` ok；`go run ./scripts/apidoc` rc=0。
目录实测同步：主包 153/194→**154/195**、常量 198→**201**、`Matrix4` 方法 8→9、`Context` 225 与 `Canvas` 113 不变；
recording `go/ast` 186→**183**（类型 84、常量 64）；scene 顶层 143→**144**、`Scene` 方法 34→38、`Renderer` +2、`Encoding` +1。
真窗 `examples/ui_render_base_scene` 双后端 rc=0、Xwayland 每轮前后 1：`parity=1.4376%`（窗内判据 ok、两臂同值）、
`golden 0.0000%`（存量零变化）；另四扇 `render_*scene*` 示例无 `-auto-only`（不能强杀）⇒ 本批以编译 + 包内测覆盖，
补自检登记为示例层工程债。

**口径**：scene 仍零生产消费者（边界钉保证 `engine/`、`ui/` 不接），本步是"标准能力到位"，不得标生产；
3e-2（quad+权重通道，2.5D 正路）与 3e-3（材质编码，依赖 3a–3d，粒度按 item 级）在后面。

## 8 步 3e-2 交付记录（2026-11-04，已提交 `1b287b4d`、未推送）

**做了什么**：给 `render/scene` 补上 2.5D 的正路——四角 + 齐次权重的「算好再喂」quad 通道，形态照 Godot 2D：
命令面不带任何投影参数，透视由调用方把四角与 1/w 算好再提交（对标 `canvas_item_add_polygon` /
`add_triangle_array`；`transform_2d.h:58` 的 2D 变换只有 `Vector2 columns[3]`）。落地五处：
`TagImageQuad`=0x52（pathData 13 个 float32：四角 8 + 权重 4 + 不透明度 1；drawData 3 个 uint32：图像索引 +
位标志 bit0=透视 + 采样档）、`Encoding.EncodeImageQuad`、`Decoder.ImageQuad`（越界按剩余量推进、绝不 panic）、
`Scene.DrawImageQuad`（角点在录制期用 `currentTransform` 烘焙，与 `TagImage` 同约定；blend/mipmap/tint 明确不编码，
登记在注释里）、GPU 回放直接转交 `Context.DrawImageQuad`（scene 侧不再算第二套透视，抽出共用的
`sceneImageBuf`）、CPU 瓦片腿只吃「平行四边形 = 一个仿射」那一类（`quadAffine` 按图像像素空间→画布的约定折仿射，
先除宽高），梯形与带权 quad 一律整笔拒画并复用 `Renderer.ProjectionDegrades` 计数；
两条合并路径（`Append` / `AppendWithTranslation`）只挪 8 个角点 float，权重与不透明度一格不动，图像索引两路都抬。

**钉 11 枚（另含 2 条子钉）= 外部 5 + 包内 6**：
外部 `p37_3e_quad_test.go`：①逐值回环 + 位移合并只挪角点 + 跨注册表抬索引（含 host 自己那格不许被抬）；
②回放 ≡ 手写 `Context.DrawImageQuad`（逐像素相同，direct/replay 各 968 像素）+ 反向鉴别「透视开/关必须成片不同」
（实测差 784 像素，阈值定 ≥100，防止纯色图假绿）；③场景变换烘焙 ≡ 等价手写角点；④不调 quad 的存量场景零发射；⑤自交（蝴蝶形）quad 被 `Context` 哨兵错拒掉后必须一格不画（错数据可以吞，错画面不行）。
包内 `p37_3e_cpu_test.go`：⑤平行四边形 quad ≡ 等价仿射 `DrawImage`（逐字节相同）；⑥梯形拒画 + 计数；
⑦平行四边形但带权重 ⇒ 同样拒画计数（对照组不透视必须画得出）；⑧真投影下 quad 拒画计数；
⑨图像与 quad 在仿射提升投影下的落点 ≡ 等价手写仿射/手写角点；⑩链序两条子钉
（`tag_transform_site` / `tag_projection_site`，各自只被自己那处反向打断打红）。

**3e-2 把 3e-1 的 CPU 腿两条真漏打了出来**（判据先红后绿，原始红记在 `batch3e/negative-3e2.txt` 附段）：
①`TagImage` 那条完全没吃仿射可表达的投影层 ⇒ 与 GPU 腿分叉，实测与等价手写仿射差 900 字节；
②`TagTransform`/`TagProjection` 两处复合写成 `raw.Multiply(proj)`，把投影放到了场景矩阵**内侧**，
违反拍板 B 的 `device = deviceMatrix ∘ projection ∘ matrix ∘ user`（`context_projection.go:342` 的
`D.Mul(P).Mul(M)` 才是真源）——3e-1 的包内钉用恒等场景矩阵测，恒等与任何序都可交换，所以当时照不出。
现四处（722/751/765/995）统一到「投影在矩阵之外」，恒等投影时 `Multiply` 逐位退化回旧值，存量画面零变化。

**负向 15 条全红**（N1–N14 由 `batch3e/negative-3e2.py` 可直接复跑；N15 是跨层打断，见下方单记）：编码丢权重、位移挪权重、位移/非位移两条不抬索引、
回放丢透视位、回放取不到图、录制不烘焙、平行四边形判据放松、拒画不计数、透视位被忽略、图像不折投影、
quad 角点不叠投影、两处链序各打一处（并验另一条子钉仍绿，证明两条子钉真的分得开）。
首轮有四条变异（N1/N5/N6/N10）是因为把变量整个删掉导致 `declared and not used` 编译断裂——编译断裂不算鉴别，
已改成仍编译的等价打断（`float32(1+w*0)`、`perspective && false`、`imageIndex+999`）复跑，条条落在行为红上；
另有 N3/N12 首轮打到同名重复锚点（`+= imageOffset` 的 `TagImage` 分支、两处复合点里不被流覆盖的那一处），
如实记为无效变异后按窗口限定 + `nth` 重做。

**门禁**：`go build ./...` rc=0；`go vet ./render/scene ./render ./render/recording` rc=0；所触文件 `gofmt` 净
（`render/context_layer.go`、`render/internal/clip/stack.go` 的 HEAD 外在脏按约定未碰）；`render/scene` 全包 ok 0.31s、
`-race`（CGO_ENABLED=1）ok 2.29s、`render/recording` + `backends/raster` ok、`./render -run "Projection|P37|Quad|Depth"` ok；
`go run ./scripts/apidoc` rc=0。`./render` 整包在本机 **对称红**：`src/conv.rs:519` 的 wgpu-native 非展开 panic
→ SIGABRT，今日在干净 HEAD 工作树用同一 `LD_LIBRARY_PATH`/`WGPU_NATIVE_PATH` A/B 过，两边同点崩、同样无 `--- FAIL` 名单，
与批 2 登记的存量环境红同一条，不是本步引入。真窗 `examples/ui_render_base_scene` 双后端 rc=0：
`golden 0.0000%`（627200 px，`pixel_golden_first_run=false` ⇒ 与存量金图逐位比过）、A/B parity 1.4376% 与 3e-1 同值、
logic 21/21、point 17/17、ink 1/1、Xwayland 每轮前后各 1（`batch3e/windows-3e2.txt`）。
目录实测同步：scene 顶层 143→**145**（+`TagProjection`/`TagImageQuad`）、`Scene` 方法 34→**39**、
`Encoding` +2、`Decoder` +2、`Renderer` +2；枚举速查行补进两枚新 tag。

**口径与留账**：scene 仍零生产消费者（边界钉保证 `engine/`、`ui/` 不接），quad 通道同属"标准能力到位"，不得标生产。
材质（Godot 的 item 级 `canvas_item_set_material`）留 3e-3，2.5D 场景级接线仍随批 6；
逐文件全量 sweep 与五目标交叉构建按批口径留到批 3 收尾一次性跑。
