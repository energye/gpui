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

## 3 3e-2 scene 透视 quad 通道（下一枚，同批内）

- `Scene.DrawImageQuad(img *Image, corners [4]render.Point, opts render.QuadDrawOptions)`：**复用 render 的
  `QuadDrawOptions`**（含 `Perspective`/`Weights`），不新造第二套选项类型；`ui_render_base_scene` 的图像入口形态照搬。
- 新 `TagImageQuad = 0x52`：`pathData` 8×float32 四角（场景用户系，交给回放端当前 2×3 变换作用，与 `TagImage` 同处理），
  `drawData` = 图像索引 + 透视标志，权重 4×float32 进 `pathData`（scene 全流 float32 精度口径，注释写清）。
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
- scene 编码无版本号/魔数、纯内存双流 ⇒ 「编码流演进」的兼容靠 tag 穷举绊线（`AppendWithTranslation` default panic）
  与 delta 零发射保证；若将来要落盘，需要另立版本化设计（不在本批）。

## 7 步 3e-1 交付记录（2026-11-04，未提交）

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
