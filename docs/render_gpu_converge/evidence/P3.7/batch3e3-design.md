# P3.7 批 3e-3（材质编码：录制面 + scene item 级）· 实施方案（2026-11-05，design.md §4.5 D9 已拍板；材质粒度按 batch3e 拍板=item 级）

依据：design.md §4.5（D9 录制+scene 都进）+ batch3e-design.md §6（材质依赖 3a，粒度 item 级）+
3a–3d 已交付（`ShaderEffect` 全链在位）。Godot 对标：`canvas_item_set_material`（item 级，
`rendering_server.h:867`；`renderer_canvas_cull.cpp:1977-1983` 实现）——scene 流内等价物 =
**粘滞状态命令**（item 的命令在流内连续，SetMaterial 在 item 边界切换 = item 级语义）。

## 1 scene 半（编码流）

- 新 `TagMaterial Tag = 0x04`（state 组，TagProjection 0x03 之后；跨层粘滞，与 TagSetAntiAlias
  同族——layer 边界不复位，文档写死）。payload：uint32 材质索引（0xFFFFFFFF=nil 清除）。
- **材质注册表**：`Scene.materials []*render.ShaderEffect`（照 images 注册表形态）；
  `Encoder.EncodeMaterial(idx)` / `Decoder.Material() (idx, ok)`；0xFFFFFFFF 解码为 nil。
- **delta 去重**（照 3e-1 投影形态）：`Scene.lastEmittedMaterial` 指针比较——同 effect 不重发；
  不调 SetMaterial 的存量场景零新 tag（Hash 不漂）。
- **Hash**：材质注册表进 `Hash()`（E5b 教训：缓存键必须吞尽新状态）。*ShaderEffect 是
  指针对象——给它加 `gen uint64`（SetUniformBytes/SetImageAt bump）+ 公开 `Gen() uint64`，
  Hash 吞 `ptr^gen`（uniform 改动后 LayerCache 必失效；照 ImageBuf.GenerationID 先例）。
- **Scene 动词**：`SetMaterial(e *render.ShaderEffect)`（nil 清除）；Push/Pop 不设（Godot
  canvas item 材质无 push/pop 语义，最小面）。
- **GPU 回放**（`gpu_renderer.go`）：`case TagMaterial` → 维护 `r.currentMaterial`；
  `TagImage` resolveImage → `DrawImageEx` opts.Material；`TagImageQuad` → opts.Material；
  `TagFill`（paint 构造点）→ `paint.Shader = r.currentMaterial`（Godot item 材质作用于
  fill/stroke 全部命令）。回放结束复位（照投影回放末尾显式复位防泄漏）。
- **CPU 瓦片腿**：`case TagMaterial` → 维护 currentMaterial；image/quad/fill 命令遇
  currentMaterial != nil ⇒ **整笔拒画 + `Renderer.MaterialSkips` 计数**（公开查询口，照
  `ProjectionDegrades` 形态；无 CPU 求值器 = P3.2 契约，禁止静默裸画）。
- **合并两路**：`AppendWithImages`（无 default，必须补 case + 材质注册表并集抬索引，照
  images 抬索引先例）+ `AppendWithTranslation`（有 default panic，必须补 case）。
- 边界钉延续：engine/ui 对 render/scene 零引用。

## 2 录制半（design §4.5 第一半）

- `FillPathCommand.Shader` / `StrokePathCommand.Shader *render.ShaderEffect`（命令结构最小
  新增，录制期取值、回放只读——批 4 PathEffect 先例）。
- `Recorder`：状态字段 `shader` + `recorderState.shader` 快照（save/restore 携带）+
  `SetShader(e *render.ShaderEffect)`（镜像 Context.SetShader）；Fill/Stroke 命令构造时
  抓 `r.shader`。
- `Backend.FillPath/StrokePath` 签名 +shader 尾参（**接口全量切换制**：Backend 接口 +
  CanvasBackend + raster 后端 + 测试桩一次改完，方法数不变）。
- `DrawImageCommand.Opts.Material` 随 3b 自动随行（零工作，实测确认）。

## 3 钉（先红后绿，`render/scene/p37_3e3_material_test.go` + `render/recording/p37_3e3_material_test.go`）

scene 面：
1. 恒等零发射：不调 SetMaterial 的存量场景 tag 数/Hash/流长全等（E1 同款）。
2. 编码回环：SetMaterial 编码/解码（索引回环、0xFFFFFFFF=nil、delta 去重不重发、跨 effect
   重发）、Hash 不漂 + uniform 改动后 gen bump ⇒ Hash 变（LayerCache 防线）。
3. GPU 回放 ≡ 手写：SetMaterial + DrawImage（材质=恒等采样）回放 ≡ 手写 DrawImageEx
   Material 逐位；TagFill 回放 paint.Shader 生效（渐变填充）。
4. 合并：AppendWithImages/AppendWithTranslation 材质注册表并集抬索引 + TagMaterial 照抄
   不丢（AppendWithTranslation 漏 case ⇒ panic 绊线）。
5. CPU 表态：currentMaterial != nil ⇒ image/fill 拒画 + MaterialSkips 计数（非零断言 +
   对照组 nil 材质必画）。
6. 边界结构钉：engine/ui 对 render/scene 零引用（延续）。
录制面：
7. SetShader 状态录制：Fill/Stroke 命令携带 shader；save/restore 快照；回放（CanvasBackend）
   paint.Shader 生效像素钉；DrawImage 命令 Opts.Material 自动随行（实测断言）。
负向：N1 摘 delta 去重 ⇒ 钉 2 红（Hash 漂/重复 tag）；N2 摘 Hash 吞 gen ⇒ 钉 2 红
（uniform 改动 LayerCache 陈旧）；N3 摘合并 case ⇒ 钉 4 红（panic 绊线/丢索引）；
N4 摘 CPU 拒画 ⇒ 钉 5 红（静默裸画）；N5 摘录制 shader 抓取 ⇒ 钉 7 红。

## 4 口径

- item 级=流内粘滞（Godot item 命令连续的等价物）；命令级材质不做（batch3e 拍板）。
- scene 仍零生产消费者，「已验证待接线」；材质腿随 3a–3d 全链在位。
- GPU 回放材质路走 3b 的 tryGPUDrawImage 材质分支 + 3a 的材质管线（零新 GPU 面）。
- Hash 吞指针^gen：ShaderEffect.Close 后重放 → 材质注册表仍持引用（Close 语义=GPU 资源
  驱逐+拒新用，回放遇到 closed effect ⇒ CPU 腿拒画计数 / GPU 腿材质腿拒画——照 3a
  buildImageResources Closed 拒绝口径）。
