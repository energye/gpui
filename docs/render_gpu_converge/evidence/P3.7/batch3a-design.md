# P3.7 批 3a（真·quad 片元材质骨架）· 实施方案（2026-11-04，**已拍板定稿**）

依据：`batch3a-assessment.md`（本仓 P3.2 实测 + Godot 材质形态取证）+ design.md 已拍板的
D4/D8/D9 + batch3e 改判（材质 item 级为主、3e-3 依赖本批）。
**2026-11-04 用户拍板：D-3a-1=A（引擎模板合成）、D-3a-2=T2（声明即接管）、
D-3a-3=R1（骨架+内部命令字段）、D-3a-4=P1（探针先行，本会话跑）——全部按推荐档。**

## 0 已锁定的拍板（沿用，不重开）

- **D8（已拍板）**：真·quad 片元路，一次绘制、无读回、可合批；弃两趟 RT 读回骨架（`user_shader_fill.go:266-267` 的 WaitIdle+读回在材质路不再出现）。
- **D4（已拍板，非推荐加重档）**：扩绑定槽位 binding 3+、扩 uniform 预算——**归 3c**，本批只把绑定布局写成可扩展形态。
- **D9（已拍板）**：录制 + scene 编码都进——归 3e/3e-3。
- **batch3e 改判（已拍板）**：材质 item 级为主（Godot `rendering_server.h:867`、`renderer_canvas_cull.cpp:1977-1983`），命令级材质不做；3b 的挂点=per-draw 选项（与 tint 同位）。
- **design §10 P3（硬前置）**：premul 双路口径探针在 3a 动代码前跑，结论落文件。

## 1 本轮拍板点（四项，2026-11-04 全部拍板：A / T2 / R1 / P1）

### D-3a-1 组合机制（✅ 拍板 A）

- **A（推荐）引擎模板合成，Godot `#CODE : FRAGMENT` 同构**：管线片元=组合模块——引擎前奏（@group(1) 引擎绑定声明 + rrect_clip_coverage + 输出段）+ 用户源码（`@fragment fn fs_main` 手术为普通函数）+ 引擎 `fs_main` 包装（按 IR 读到的声明前缀传参调用户函数，之后按标准序后乘 `opacity × clip_cov [× tint]`）。组合在 `NewShaderEffect` 期完成，**组合产物重跑 Parse→Lower→Validate 全链**，手术 bug 当场具名拒绝。
  - 可测红利：恒等材质（用户字面 `return textureSample(...)`）与无材质**逐位相同**可做像素钉——包装后乘结构与标准 FS 同序（`textured_quad_persp.wgsl:101` 的 `texel * opacity * in.tint`）。
  - 一份 effect 双路可用：填充路包装以默认值喂参（tint=恒等、hom_w=1）。
  - 代价：源码手术（去 `@fragment`、参数 `@location` 属性、改名）+ 组合模块机器；IR 参数表已验可读（`gpu/shader/ir/ir.go:605-621` `Function.Arguments[]{Name,Type,Binding}`，前提成立）。
- **B（备选）用户模块直当入口**：引擎后处理走「透明度折进逐顶点 tint + 裁剪只走 stencil/depth 腿」。省掉组合机器，但三处豁免：RRect SDF 快腿材质不可用（同组 AA 边与邻批有差）、恒等材质逐位钉做不成（浮点乘序不同，只能 ≤1ulp 声明）、声明了 tint/hom_w 的 effect 不能走填充路（能力面劈叉）。
- 灯光染色共存（批 6 材质窗规格）在 B 下只能靠「透明度折进 tint」，用户声明 tint 即接管灯光——语义上成立但把灯光与 tint 耦死；A 下灯光=tint 走引擎后乘或用户接管（见 D-3a-2）两条路都干净。

### D-3a-2 tint 语义（✅ 拍板 T2）

- **T2（推荐）声明即接管，Godot 同构**：用户片元声明 `tint` 输入 ⇒ tint 应用是用户的事（引擎不再后乘 tint，只后乘 opacity×clip）；未声明 ⇒ 引擎后乘。对标 Godot：`COLOR` 进用户代码前已含 modulate，用户写 `COLOR=...` 即接管（canvas.glsl 模板语义）。
- **T3（备选）引擎必乘**：tint 输入降级为「知情」输入（引擎照样后乘，用户自己再乘会双乘）——无双乘就要在契约文档写死「不许再乘」，脚枪换位不消失。
- 双乘脚枪无论选哪档都要在 `ShaderEffect` 契约文档写死一句。

### D-3a-3 3a 范围切分（✅ 拍板 R1）

- **R1（推荐）骨架 + 内部命令字段**：`ImageDrawCommand`（internal/gpu 内部类型）加 `Material *ShaderEffect` + 材质腿绘制路径 + 内部级像素钉（照批 1a 先例：直接构内部命令真设备测，零公开 API）；3b 只做三个公开入口选项字段 + 合批谓词 + CPU 腿表态。
- **R2（备选）3a 只做管线骨架**：无任何命令字段，骨架经测试缝验证——原子性最纯，但「骨架接得上命令路径」要等 3b 才第一次被真验，风险后移。
- **R3（不推荐）3a 顺手把 3b 公开入口一起做**：违反五个子步各一次提交的批次切分。

### D-3a-4 P3 探针节奏（✅ 拍板 P1）

- **P1（推荐）方案定稿后、3a 动代码前，本会话立即跑**：探针=组合可行性 + premul 双路口径一次实测（§4），落 `batch3a/probe-p3-premul.txt`；探针是诊断代码（测试 harness），不算 3a 实现开工。
- **P2（备选）探针并入 3a 实现批**：省一次往返，但 premul 契约就没了「先测后改」的独立性，违反 design §10「缺证据不动手」。

## 2 组合机制 A 详案（D-3a-1 选 A 时定稿）

### 2.1 组合生成器（`render/shader_effect.go` 新增，纯函数可单测）

输入：用户 WGSL 源 + 路径标志（fill/quad）。输出：组合 WGSL 字符串。结构：

```wgsl
// —— 引擎前奏（quad 路才需要；fill 路无此段）——
struct EngineFSIn {
    @builtin(position) frag_pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) tint: vec4<f32>,
    @location(2) hom_w: f32,
}
@group(1) @binding(0) var<uniform> eng_clip: ClipParams;      // 与标准管线同布局
@group(1) @binding(1) var<uniform> eng_img: EngineImageUniforms; // transform/opacity（顶点用同一块）
fn rrect_clip_coverage(...) -> f32 { ... }                     // 与 textured_quad_persp.wgsl 同款

// —— 用户源码（手术后）——
fn user_fs_main(uv: vec2<f32> [, tint: vec4<f32> [, hom_w: f32]]) -> vec4<f32> { ...原函数体... }

// —— 引擎输出段 ——
@fragment fn fs_main(in: EngineFSIn) -> @location(0) vec4<f32> {
    let user = user_fs_main(in.uv [, in.tint [, in.hom_w]]);   // 按 IR 声明前缀传参
    var out = user * (eng_img.opacity_pad.x * rrect_clip_coverage(in.frag_pos.xy));
    // tint 按 D-3a-2：未声明 ⇒ out = out * in.tint；声明 ⇒ 不再乘
    return out;
}
```

手术规则（全部有组合后重验兜底）：
1. 用户源内 `fs_main` 全量改名 `user_fs_main`（声明与调用点一起，注释内命中无害）；
2. `@fragment` 属性摘除；参数表按 IR 重建（去 `@location(N)` 属性，保名字与类型）；**返回类型的 `@location(0)` 一并摘除**（WGSL：IO 属性只许在入口上；探针首轮实测只摘参数不摘返回 ⇒ naga 判模块无效，见 probe 记录）；
3. 组合产物重跑 `validateUserShader` 全链（Parse→Lower→Validate），任何手术偏差=具名拒绝；
4. fill 路包装：`EngineFSIn` 只有 uv（VS=现有全屏三角 `userShaderVS`），tint/hom_w 以字面默认值传参；quad 路包装：三位置全由材质顶点模块输出。

### 2.2 契约扩展（`validateShaderContract`）

- `fs_main` 参数面从「不校验」扩为**声明前缀校验**：第一参必须 `uv: vec2<f32>`（@location(0)），之后按序可选 `tint: vec4<f32>`（@location(1)）、`hom_w: f32`（@location(2)）；多余参数/乱序/类型不符/未知位置 ⇒ 具名拒绝。**零参 `fs_main()` 维持今天放行**（`render/p32_shader_leak_test.go:19` 在用，只上 fill 路）。
- 契约文档（`shader_effect.go` 头注释）同步：premul 输出、tint 接管语义（D-3a-2）、材质路 binding(1)/(2)=精灵纹理、SetImage 在材质路的口径（§2.5）。

### 2.3 材质管线族与缓存

- 材质顶点模块（引擎持有，新增 WGSL）：36B 顶点（pos2+uv2+tint4+hom_w1，与 `imagePerspVertexStride=36` 同布局），输出 `@location(0) uv / @location(1) tint / @location(2) hom_w` 三个 varying + 位置按 hom_w 缩放出真 clip-space w（照 `textured_quad_persp.wgsl:77-94` 的 vs_main）；**透视与否是数据（hom_w=1 即仿射），不是管线维度**。
- 缓存：`userShaderCache.entries` 键从 `*ShaderEffect` 扩为 `{effect, path(fill/quad), leg(stencil/depthClip/sdf)}`；每 effect：fill 管线 1 条 + quad 管线 2–3 条（stencil/depthClip 必备，sdf 快腿可选）。`evictEffect` 改为遍历销毁该 effect 全部条目（驱逐钩子契约不变）。
- 绑定布局：@group(0)=用户声明原样（uniform binding(0) + 纹理对 binding(1)/(2)——材质路绑精灵纹理与命令 FilterKey 采样器）；@group(1)=引擎（clip + transform/opacity，与标准管线 @group(1) 形态对齐，`textured_quad_persp.wgsl:48` 先例）。布局按 effect 声明派生不变（L2-P1-3 原则）。
- per-draw 数据走既有 slab 惯例：用户 uniform 256B/槽（`dualTexUniformSlotStride`，`dual_tex_blend.go:276` 先例）+ 引擎 transform/opacity 同槽排布，一次 WriteBuffer，动态偏移或按槽绑定照 slab 既有形态。

### 2.4 绘制路径（3a 内部口径）

- `ImageDrawCommand.Material != nil` ⇒ 该命令走材质腿：组合管线 + 材质顶点缓冲（四角=命令角点经 CTM 烘焙，uv=图集/整图 0..1，tint=命令 tint，hom_w=权重或 1）+ 用户 uniform slab 槽。
- 合批：材质命令与无材质命令互不合批（管线不同天然隔离，谓词在 3b 正式加 `a.Material != b.Material → false` 一行，指针比较）；材质命令之间按现有谓词维度合（同 effect 指针 + 同图 + 同采样 + 同 blend）。材质腿首批只接 Normal 混合（非 Normal 具名拒绝，与透视腿 `ErrQuadUnsupportedBlend` 同口径，扩族归 3b+）。
- CPU 腿：材质命令跳过并计数（沿用 P3.2 `CPUFallbackOps==0` 口径，`render/p32_shader_premul_test.go:68`），无 CPU 求值器写进契约，不假装支持。

### 2.5 SetImage 在材质路的口径（3a 定死，3c 放开）

- 材质路 binding(1)/(2)=精灵自己的纹理+命令采样器（Godot `TEXTURE`/`UV` 同语义）；`effect.SetImage()` 绑的图在材质路**无槽可占**（binding 3+ 归 3c）⇒ 材质路忽略 SetImage 状态，fill 路照旧。契约文档写死，3c 多槽放开后材质路 SetImage 图走 binding(3+)。

## 3 premul 契约（✅ 2026-11-05 探针定论，填充腿修复待拍板）

探针结论（全文 `batch3a/probe-p3-premul.txt`，复跑 `TestP37AProbe*` 六测试全绿）：

1. **组合可行性 ✅**：A 档成立——四类声明前缀组合+重验+native 真编译全过；组合器手术规格两处实测修正
   （返回类型 `@location` 必须一并摘除、参数表闭括号配对扫描），已并入 §2.1。
2. **quad 材质腿 ✅ 逐位**：组合管线 + `BlendStatePremultiplied` 直出 = premul-src-over 白底解析值
   max=0 diff=0；恒等材质 ≡ 无材质逐位（跨包 digest `d2e2090f6c6057ac` 双边一致）。
3. **双路不一致 ❌，根因=填充腿 premul 交接反向（新洞，实测到行号）**：`fillShaderAsImage`
   （`user_shader_fill.go:86`）`premulToStraightRGBA` 去预乘后交给按 premul 解释的下游
   （图像腿 level0 原样上传 `image_cache.go:553-570` + Context 无视图 flush 的层合成
   `context_layer_blend.go:174` 的 u8 premul src-over）——去预乘被 premul 解释原样抵消，
   只剩 u8 舍入与 **uint8 回绕**（生产输出 ≡ `uint8(c_s+255-a_s)`，24×24 全域 576/576 精确吻合，
   max=213）。d9 钉实测值 126 = 382 mod 256，被 128±3 容差掩盖；其注释「等价于 50% 纯红合成」
   的数学正确值 (255,128,128)，断言与自身注释矛盾。
   旁证：兄弟函数 `fillPaintFilteredAsImage` 交的就是 premul 字节；图像腿全部既有消费者走 premul。
   影响面：仅 a<255 的填充 shader 像素错（不透明恒等不受影响）；`ui_wr_p32_shader` 两卡均
   alpha=1 ⇒ 金图预计零变化。
4. **契约定稿（quad 腿按此实现）**：用户 shader 一律输 premul；quad 材质路 premul 直出 +
   `BlendStatePremultiplied`（探针逐位实证）；**填充路应交 premul**（摘 `premulToStraightRGBA`，
   与兄弟函数/图像腿/quad 腿四方对齐）——该行为修复**停在拍板节点**（2026-11-05 两问未答，
   保守分支登记不修）：问 1=填充腿洞现在修 vs 先登记后修；问 2=d9 期望同步修正 vs 不动。
   拍板后：填充腿修复 + d9 期望改 (255,128,128) + 真窗复跑 + `TestP37AProbeFillLegGradient`
   改回 `p37AProbeGradOverWhite` 期望（现状回绕行为已先钉在测试里，防无感漂移）。

## 4 P3 探针（批 3a 前置，P1 档时本会话跑）

1. **组合可行性**：挑两份代表 WGSL（uv-only 零参 + uv/tint/hom_w 全前缀），组合后过 `validateUserShader` + 双后端真编译（webgpu native + gles）。
2. **premul 双路**：同一 shader（输出半透明非平凡 premul 色，带 alpha 梯度防纯色假绿）分别走「填充路读回」与「quad 材质路直出混合」，同几何同背景逐像素对拍；差异像素数与最大通道差落盘。
3. **恒等材质 ≡ 无材质**（A 档红利验证）：用户字面返回纹理采样的材质 vs 不挂材质的同一 quad，逐位对拍。
4. 产物：`evidence/P3.7/batch3a/probe-p3-premul.txt`（含双后端、差异统计、结论一句）。

## 5 钉与负向（先红后绿，D-3a-1/2/3 定稿后细化枚号）

外部钉（`render/p37_3a_*_test.go`）：
1. 契约扩展：uv-only 放行、tint/hom_w 前缀放行、乱序/多余/类型不符拒绝（具名错）。
2. 组合回环：组合产物过 Parse→Lower→Validate；手术摘掉（拿未组合源直建管线）⇒ 钉红。
3. 恒等材质 ≡ 无材质：双后端逐位（A 档；B 档改 ≤1ulp 声明）。
4. 材质 quad ≡ 手写组合管线直绘：同 WGSL 同 uniform 同四角，逐像素相同。
5. 引擎后乘序：装 clip + opacity<1 的材质画面与「用户输出×(opacity×clip)×tint」手算一致（判据用不可交换的数值对）。
6. 存量 P3.2 行为不回退：`p32_*_test.go` 全绿 + `ui_wr_p32_shader` 真窗复跑（填充路换组合模块后逐位不变）。
7. CPU 腿表态：材质命令跳过且计数（不静默画错）。

包内钉：缓存键变体维度（同 effect 两腿两管线、驱逐全清）、slab 槽位对账、材质命令不合批进无材质批。

负向：摘组合后乘（用户输出直出）⇒ 钉 5 红；摘 premul 混合系数 ⇒ 探针/钉 4 红；摘参数校验 ⇒ 钉 1 红；摘变体键 ⇒ 两腿串用钉红；摘 CPU 计数 ⇒ 钉 7 红。

## 6 门禁（按批口径，3a 为子步只跑触及包）

`go build ./...`、`go vet ./render ./render/internal/gpu`、触及文件 `gofmt` 净、
`render` + `render/internal/gpu` 定向（P3.2 存量 + P3.7 3a 新钉）`-count=1` 单跑、
`-race`（ShaderEffect 生命周期触及共享数据，必跑）、conformance 双后端、
真窗 `ui_wr_p32_shader` 复跑（存量金图逐位零变化）、`go run ./scripts/apidoc` rc=0、
目录同步（`ShaderEffect` 契约描述、组合机制注记；若 D-3a-3 选 R1 加 `ImageDrawCommand` 材质注记——internal 类型不进公开目录，只在 P3.2 行补说明）。
逐文件 sweep 与五目标交叉构建按批口径留批 3 收尾。

## 7 登记（不假装做完）

- 材质 3a 后仍零生产消费者（消费者=3b 入口/批 6 窗），「已验证待接线」，不标生产。
- Godot canvas 材质允许用户动顶点（`#CODE : VERTEX`，canvas.glsl:274）；本批 D8 拍板引擎持顶点级、用户只写片元——差距登记，扩展另议。
- 填充路 scratch RT 池化仍是在册尾巴（`user_shader_fill.go:172-176` 自认），本批不顺手做。
- gles 上 binding 4+ 的 naga 翻译实测归 3c（design §4.3 前置）；3a 的组合模块只需 group(0)/group(1) 两级，与既有管线同深。
- Metal 本机不可验，按标记写明。

## 8 批 3a 交付记录（2026-11-05，未提交）

**前置：填充腿 premul 校准修正（探针 verdict 落地，用户拍板「可以，开3a」= 按推荐档执行）**：
`fillShaderAsImage` 摘 `premulToStraightRGBA`（premul 字节直交图像链，与兄弟函数
`fillPaintFilteredAsImage`/图像腿 level0 契约对齐）；d9 契约钉期望同步修正 (255,128,128)
（对齐其自身注释语义，旧 128±3 锁的是回绕值 126）；`premulToStraightRGBA` 删除（生产零调用者）。
验证：生产填充腿 vs quad 腿解析期望 **0 像素差**（修复前 max=213）；真窗 `ui_wr_p32_shader`
双后端 golden 0.0000%/220800px（两卡 shader 均不透明，跟踪快照逐位未变）；契约文档
（shader_effect.go 头注）改写 premul-native 叙述。

**3a 骨架落地（R1：零公开入口，内部命令字段 + 内部像素钉）**：
- `render/shader_compose.go`（新）：`fragmentParams`（声明前缀校验：逐位 @location(i)、
  类型 vec2/vec4/f32、零参合法、名字自由）+ `rewriteUserFragment`（fs_main→gpui_user_fs_main
  全量改名、摘 @fragment 与参数/返回 IO 属性、配对括号扫描）+ `composeAndValidate`
  （fill/quad 两变体组合 + 组合产物重跑 Parse→Lower→Validate）。引擎段：quad 带
  @group(1) 引擎逐绘制块（transform+opacity，字节=ImageUniforms）+ @group(2) 标准裁剪
  （同一 clipBindLayout 对象）+ RRect SDF 覆盖；fill 无引擎段（coverage 走 mask 链）。
  T2 落地：组合期决定 tint 归属（声明了 tint ⇒ 输出段不再乘）。
- `render/shader_effect.go`：`NewShaderEffect` 构造期组合两变体并重验（手术偏差当场具名拒绝）；
  `ShaderEffect.ComposedFillWGSL()/ComposedQuadWGSL()` 新读口（目录 §3.16 新节，apidoc rc=0）；
  契约文档写死声明前缀 + T2 接管语义。填充路 pass-1（user_shader.go）切到组合源（接口全量切换）。
- `render/internal/gpu/material_pipeline.go`（新）：会话级材质缓存（同 imageCache 所有权，
  独立会话/测试可用）——bgl 全局两形（用户有/无纹理对）跨腿共享（同标准腿一个 uniformLayout
  的形态）、按效果 module+256B uniform 槽+双腿管线（stencil/depthClip 与标准腿同 DS 状态）；
  Close 驱逐经 RegisterShaderEffectEvictor（session id，Destroy 注销）。
- `image_pipeline.go`：`ImageDrawCommand.Material` 内部字段 + 合批谓词
  `a.Material != b.Material → false`（指针比较）+ `imageDrawCall` 材质字段；
  `RecordDraws` 材质分支（组合管线 × 腿、@group(0) 用户 / @group(1) 引擎 / @group(2) clipBG、
  36B 顶点缓冲）。`render_session.go`：buildImageResources 材质分支（拒绝面：Material+Perspective /
  非 Normal / Closed 三类具名错；仿射发射 hom_w=1 复用 buildPerspVerticesInto；引擎 slab 256B
  步长先于槽循环就绪；材质组 0/1 绑定组缓存）+ 生命周期（Destroy 清缓存/缓冲/注销钩子）。
- 3a 口径：材质命令仿射 + Normal；Material+Perspective、非 Normal、关后效果三类具名拒绝
  （扩展归 3b/3c）；SetImage 图在材质路无槽（binding 3+ 归 3c）；CPU 求值器不存在
  （公开入口 3b 在队列前拒绝）。

**钉 10 枚（render 4 + internal 5 子钉 + 探针 1 翻正）全绿**：
- 契约面 `render/p37_3a_test.go`：合法前缀三态放行 + 组合产物引擎锚点/手术锚点 +
  fill 无引擎裁剪段；全前缀调用面（三参传齐 + T2 接管无引擎乘 tint）；uv-only 调用面
  （只传 uv + 引擎后乘 tint）；拒绝面五例（乱序/vec3/四参/缺 location/位置错）全部具名
  contract 错。
- 管线面 `render/internal/gpu/p37_3a_material_test.go`（一台设备一个会话五子钉）：
  恒等材质 ≡ 标准腿**逐位**（同 pass 双区域读回 diff=0）；tint+opacity 引擎块 ≡ 标准腿
  **逐位**；合批形态（同 effect 两张合 1 draw vertCnt=12、材质/无材质/异 effect 三槽）；
  拒绝面三类；Close 驱逐（ensure 双腿→entries 在→Close→entries 清→ensure 拒）。
- 探针 `p37_3a_probe_test.go`：填充腿钉翻正为 premul 契约（0 像素差）；QueueDrain/
  FillShaderOnOwnTarget 隔离段同步 premul 语义。
- 负向 5 条全红还原复绿（`batch3a/negative.txt`；两枚无效变异如实记：N5 首轮环境红
  + 锚点过期；「摘 RecordDraws 材质分支」判结构性存在不冒充崩红）。

**门禁**：`go build ./...` rc=0；`go vet ./render ./render/internal/gpu` rc=0；触及 13 文件
gofmt 净；逐文件门禁触及面 14 文件全绿（p32 五文件 + p37_3a 三文件 + persp/图集/图缓存/
合批 + glyph）；`-race` internal/gpu 材质+探针族 ok 3.6s；conformance 双后端 PASS；
`go run ./scripts/apidoc` rc=0（目录 §3.16 新节 + 两方法登记）；真窗 `ui_wr_p32_shader`
双后端 golden 0.0000%/220800px（材质机器落地后终态复跑，跟踪快照逐位未变）、
Xwayland 每轮前后 1。

**口径**：材质腿 3a 后仍零生产消费者（消费者=3b 公开入口/批 6 窗），「已验证待接线」，
不标生产。3b=公开入口三处 + CPU 腿队列前拒绝；3c=binding 3+ 多纹理 + uniform 预算放开；
3d=描边快路吃 shader + 具名错误族；3e-3=scene 材质编码（item 级）。
