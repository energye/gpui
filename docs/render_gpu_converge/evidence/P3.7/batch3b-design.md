# P3.7 批 3b（图像/批绘公开入口挂材质）· 实施方案（2026-11-05，design.md §4.2 已在 P3.7 拍板内，本文=影响面实测 + 落地口径）

依据：`batch3a-design.md` §8 交付记录（3a 骨架 `0fac5835`）+ design.md §4.2（D4/D8/D9 已拍板）。
本批公开面：三个选项字段 + 一枚哨兵 + 一个可选接口方法 + 一个计数字段；其余全在 3a 骨架上接线。

## 1 三条公开入口 → ImageDrawCommand 的流（全部实测到行号）

| 入口 | GPU 队列点 | CPU 分派点 | 材质路由 |
|---|---|---|---|
| `DrawImageEx`（`context_image_draw.go:95`） | `tryGPUDrawImage`（`context_image_gpu.go:47`）三分支：投影槽 `QueueImageDrawQuadW:123` / blend `QueueImageDrawBlend:145` / 普通 `QueueImageDraw:155` | 返回 false 后的投影 CPU 腿 `:129`、bicubic CPU、pattern 路 | GPU：material + Normal 且无投影层 ⇒ `QueueImageDrawMaterial`；material + (投影层 / blend≠Normal / bicubic) ⇒ 返 false；**CPU skip 收口在 `DrawImageEx` 内 tryGPUDrawImage 返 false 之后一处**（覆盖全部 CPU 分支） |
| `DrawImageQuad`（`m4_quadimage.go:23`） | `:117` `QueueImageDrawQuadW`（透视槽）/ `:131` `QueueImageDraw` / `:145` `QueueImageDrawTint` | `:152` `drawImageQuadCPUSplit` | 校验期拒绝：material+Perspective、material+bicubic ⇒ 新哨兵 `ErrQuadMaterialUnsupported`；material+画布投影层（perspLeg 被置真）⇒ 同哨兵；material+Normal ⇒ `QueueImageDrawMaterial`（tint 走 36B 顶点通道）；CPU 分支 material ⇒ skip+计数 |
| `DrawAtlas`（`vertices_atlas.go:27`）/ `DrawAtlasEx`（`:310`） | `:86`/`:428` QuadW（透视槽）、`:103`/`:448` QueueImageDraw、`:439` Tint | `:43`/`:121` `drawAtlasCPU`、`:466` `drawAtlasExCPU` | 精灵级：sprite.Material + (sp.Perspective / 投影层) ⇒ 跳该精灵+计数（画家序：behind-eye 同款，不炸整批）；sprite.Material ⇒ `QueueImageDrawMaterial`（图集子矩形 UV 即 Godot TEXTURE/UV 语义；tint 走顶点通道）；两条 CPU 路对 material 精灵 skip+计数 |

老口 `DrawAtlas` 的「只读老字段」契约（批 1c D-4）扩展一行：**Material 在两口都生效**
（nil 恒等 ⇒ 老调用方逐位不变成立；忽略已设 Material=静默裸画，违反非静默纪律）。

## 2 公开面（+3 字段 +1 哨兵 +1 可选接口方法 +1 计数字段）

- `DrawImageOptions.Material` / `QuadDrawOptions.Material` / `AtlasSprite.Material *ShaderEffect`：
  per-draw 选项（与 tint 同位，Godot item 级 `canvas_item_set_material` 形态）；零值 nil = 无材质
  = 存量逐位不变。文档写死：无 CPU 求值器（P3.2 契约），CPU 模式跳过并计入
  `RenderPathStats.MaterialSkips`（**绝不记 CPUFallbackOps、绝不裸画**）；
  Material+Perspective/bicubic/画布投影层 = 具名拒绝或精灵级跳过（扩展登记归批 6/3c）。
- `ErrQuadMaterialUnsupported`：quad 口 material+Perspective / material+bicubic 的具名拒绝
  （同 `ErrQuadPerspectiveBicubic` 的「组合无变体即拒绝」风格）；图集校验共用。
- `gpuContextOps.QueueImageDrawMaterial`（可选能力方法，照 Tint/Blend/QuadW 的
  「断言 + 不可断言走 CPU skip」既有形态）：参数=QueueImageDrawTint 全形（材质顶点通道恒带
  tint，36B 布局既有）；实现=提取 `newImageDrawCmd` 共享构造（Blend 版本调用它）+ `cmd.Material`
  赋值。**不动 QueueImageDraw 既有签名**（约 10 处调用点零波及）。
- `RenderPathStats.MaterialSkips int`：材质拒绝/跳过计数（CPU 模式、精灵级透视组合、
  眼后等全部走这里），非静默纪律的观测面。

## 3 引擎侧接线（3a 已就位，本批零改动）

合批谓词 `a.Material != b.Material`（3a）、材质腿 buildImageResources/RecordDraws（3a）、
三类具名拒绝（3a；本批公开入口在其**上游**拦截，拒绝面从批次错误变为入口错误/精灵跳过）。

## 4 钉（先红后绿，`render/p37_3b_test.go`）

1. 三入口恒等材质 ≡ 无材质逐位（DrawImageEx / DrawImageQuad（含 tint）/ DrawAtlasEx 同子矩形）。
2. 区分钉（防假绿）：uv-gradient 材质 ≡ 同 shader 走填充路（同矩形同 uv 映射，两条生产腿互证）
   且 ≠ 无材质画面（成片差异 ≥ 阈值）。
3. CPU skip：`GOGPU_RENDER_MODE=cpu` 三入口 material 绘制 → 像素不变 + `MaterialSkips` 计数 +
   `CPUFallbackOps==0`（skip ≠ fallback 口径）。
4. 拒绝面：quad material+Perspective / +bicubic ⇒ `ErrQuadMaterialUnsupported`；图集校验同。
5. 零值不变：nil 材质路径由既有全套回归 + 真窗承担。
负向：N1 摘入口映射（material 不进队列）⇒ 钉 2 红；N2 摘 CPU skip ⇒ 钉 3 红；N3 摘计数 ⇒ 钉 3 红；
N4 摘拒绝面 ⇒ 钉 4 红。全部「仍在编译」的打断。

## 5 口径

- 材质腿本批起有公开入口但**零生产消费者**（ui/2.5D 未接线），「已验证待接线」不标生产。
- engine/sprite.AtlasSprite 的 Material 透传（ToRender）随批 6 2.5D 接线（登记，本批不动 engine）。
- Material+Perspective / +bicubic / +画布投影层的**开启**归批 6 按需（组合顶点机已就绪，
  只差权重透传与钉）；3c = binding 3+ 多纹理 + SetImage 在材质路的槽位。

## 6 交付记录（2026-11-05，已提交 `3af42eea`、未推送）

**落地清单**：
- 公开面：三入口 `Material *ShaderEffect` 字段（`context_image.go` / `m4_quadclass.go` / `vertices.go`，
  文档写死 per-draw 语义 + Godot TEXTURE/UV + CPU skip 口径 + 拒绝组合）+ `ErrQuadMaterialUnsupported`
  （`m4_quadclass.go` 哨兵块）+ `RenderPathStats.MaterialSkips`（`context.go`，字段注释写死
  「绝不记 CPUFallbackOps」）。
- internal：`gpuContextOps.QueueImageDrawMaterial`（`context_damage.go`，Tint/Blend/QuadW 同族
  **必备方法**）+ 实现（`gpu_render_context.go`：`newImageDrawCmd` 从 QueueImageDrawBlend 提取共享
  构造，材质变体 `cmd.Material` 赋值；QueueImageDraw 既有签名与约 10 处调用点零波及）。
- 三口路由：`tryGPUDrawImage`（材质+投影/blend/bicubic ⇒ 返 false；材质+Normal ⇒ 队列）+
  `DrawImageEx`（tryGPUDrawImage 返 false 后统一 CPU skip 收口，一处覆盖全部分支）+
  `DrawImageQuad`（校验期拒绝 + GPU 臂投影组合守卫 + CPU skip）+ `DrawAtlas`/`DrawAtlasEx`
  （精灵级：材质+透视槽 ⇒ 跳+计数；材质+仿射 ⇒ 队列；两条 CPU 路逐精灵跳+计数）+
  老口 D-4 契约扩展一行（Material 两口都生效）。

**首跑钉抓到的洞（判据先红后绿）**：`sliceImageResources` 切片丢 `materialVertBuf` ⇒
裁剪组切片帧的材质命令 firstVertex（材质索引）落 32B 仿射缓冲读错区间，整格消失
（Identity 钉实测 768 通道差=全白 vs 有图）；修复=切片携带材质缓冲（`render_session.go`）。
测试自身两处结构错如实记（CPU 钉快照位置错位、stale 变量），判据不改。

**钉 4 枚全绿**（`render/p37_3b_test.go`）：三入口恒等材质 ≡ 无材质**逐位**（含 quad tint 臂、
图集同子矩形）；uv-gradient 区分钉（材质 ≡ 填充路 ±1 且 ≠ 无材质 ≥100）；CPU 模式三跳口径
（MaterialSkips=3、DrawImageEx/Quad 无 fallback 冒充、图集整批 fallback=既有语义分离断言、
像素不动）；拒绝面 5 例。**负向 4 条红还原复绿**（`negative.txt`；N4 单层变异被第二层兜底
判无效后双层同摘才红）。

**门禁**：`go build ./...` rc=0；`go vet ./render` rc=0；触及 11 文件 gofmt 净；逐文件触及面
14 文件全绿（3a/3b/probe/p32/quad 三件/atlas 两件/persp merge/image_cache_generation/depth_r5）；
`-race` ok 2.9s；conformance 双后端 PASS；apidoc rc=0；真窗 `ui_wr_p32_shader` golden 0.0000% +
`ui_wr_p35_persp` logic 9/9 pixel 17/17 golden 0.0000%/763584px seeded=false（quad 路零变化），
Xwayland 每轮 1。

**口径**：材质公开入口零生产消费者，「已验证待接线」不标生产；engine/sprite.AtlasSprite 的
Material 透传随批 6；材质+透视/bicubic/投影层的开启归批 6 按需（组合顶点机已就绪）。
