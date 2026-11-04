# P3.7 批 3c（绑定与预算契约扩展，D4 加重档）· 实施方案（2026-11-05，前置探针已过）

依据：design.md §4.3（D4 已拍板非推荐加重档）+ `batch3a-design.md` §2.5（SetImage 在材质路
无槽可占，binding 3+ 归 3c）。前置探针（design §4.3「必做」）**已过**：

> **gles 实测（TestP37CProbeGLESMultiTextureBindings，2026-11-05）**：bindings 0..6 三纹理对
> （texture@1/3/5 + sampler@2/4/6）+ 272B uniform（> 旧 256B 预算）经 naga→GLSL 翻译 + GL 编译
> **全链真编译 OK**。gles limits：MaxSampledTexturesPerShaderStage=16、MaxSamplersPerShaderStage=16、
> MaxBindingsPerBindGroup=1000、**MaxUniformBufferBindingSize=65536**、MinUniformBufferOffsetAlignment=256。
> Metal 本机不可验，按标记写明（naga 同一翻译面，登记待验）。

## 1 契约扩展（render/shader_effect.go）

- **纹理对**：声明对 = (texture@奇, sampler@偶+1)——(1,2) 第一对（材质路=精灵纹理，Godot
  TEXTURE/UV；填充路=SetImage 槽 0）、(3,4) 第二对、(5,6) 第三对……对数 N ≤ **包级上限
  `MaxShaderImagePairs = 8`**（8 纹理 + 8 采样器 ≤ 两端 per-stage 16 的实测 limits；包级上限
  之外由管线 ensure 夹设备 limits，具名拒绝）。结构校验：每纹理必须配对紧跟的 sampler；
  孤 sampler / 孤 texture / 越对绑定 ⇒ 具名拒绝；binding(0) 仍恒 uniform。
- **uniform 预算**：块大小 ≤ **`ShaderUniformBudgetMax = 65536`**（包级上限=实测两端
  MaxUniformBufferBindingSize 最小值）；`ShaderUniformBudget = 256` 语义降为「默认槽步长」
  （文档注记）。uniform buffer 实际大小 = uniform 长度向上取整到 256（对齐步长），布局
  MinBindingSize = max(256, uniformSize)。
- **ShaderInterface 扩**：`{UsesImage bool, TexturePairs int, UniformBytes int}`——
  TexturePairs=声明对数（≥1 ⇔ UsesImage），UniformBytes=声明 uniform 块字节数（0=未声明）。
  `ShaderBinding()` 返回值扩（调用方：userShaderCache/材质缓存布局派生唯一来源，L2-P1-3）。
- **SetImage 按槽位**：`SetImageAt(slot int, img *ImageBuf)` + `ImageAt(slot) *ImageBuf`；
  `SetImage/Image` 保持为 slot 0 的别名（接口全量切换制：既有调用方零改动，P3.2 行为不变）。
  槽位语义：slot k = 第 k 对纹理（k=0 即 binding(1)/(2)）。效果内部 `images []*ImageBuf`
  按声明对数预分配长度；SetImageAt(slot≥声明对数) ⇒ 具名拒绝。

## 2 引擎侧布局派生（internal/gpu，L2-P1-3：布局=模块声明派生）

- **材质缓存**：bgl0 从「全局两形」改为**按效果模块派生**（每效果一条 bgl0：uniform@0
  MinBindingSize=max(256,UniformBytes) + 逐对 texture@2k+1/sampler@2k+2）；跨腿共享同一
  bgl0 对象不变。绑定组内容：对 0=精灵纹理+命令 FilterKey 采样器（既有 3a 语义），
  对 1+=SetImageAt 的图 + **线性钳边固定采样器**（填充路 SetImage 同款约定，文档写死）。
- **SetImage 图上传**：材质路按效果持有槽位纹理缓存（slot → tex/view + 上传时 genID），
  genID 变化重传、entry.destroy 全清（照 imageCache 语义的极简版）；每帧上传的池化登记为
  后续优化（同 fillShaderAsImage scratch 池化在册尾巴）。
- **填充路 pass-1**（userShaderCache）：bgl 按效果模块派生（现有实现恒布 uniform@0 +
  条件纹理对——扩为逐对声明派生）；绑定组内容逐对喂 `effect.images[k]`（上传照现有
  per-pass `dualTexCreateTex` 惯例，多对同式）。

## 3 钉（先红后绿）

1. 契约：三对纹理放行、对不齐/孤绑定/超 8 对拒绝、272B uniform 放行、>65536 拒绝、
   ShaderInterface 读数（TexturePairs/UniformBytes）。
2. SetImageAt 回环 + ImageAt + 越槽拒绝；SetImage 旧口 = slot 0 等价（P3.2 存量零变化）。
3. 多纹理材质像素钉：精灵纹理 × SetImage 第二纹理在片元混合，逐像素判混合结果
   （判据=第二纹理内容出现在画面 ⇒ SetImage 槽位真绑定到位）+ 恒等材质回归仍逐位。
4. 大 uniform 材质像素钉：272B uniform 驱动颜色（mix_level 逐像素档位）。
5. 填充路多纹理：两纹理 shader 走 SetShader 填充路像素判混合。
负向：N1 摘第二纹理绑定 ⇒ 钉 3 红；N2 摘预算放开 ⇒ 钉 4 红；N3 摘槽位派生 ⇒ 钉 1/3 红；
N4 摘 SetImageAt 槽位校验 ⇒ 钉 1 红。全部「仍在编译」打断。

## 4 口径

- ShaderInterface 返回值扩 = 公开类型字段变更（接口全量切换制，同一提交改完所有消费方：
  userShaderCache + 材质缓存 + 测试）。
- Metal 本机不可验（naga 同面，登记待验）；gles 已实测（探针）。
- 材质腿仍零生产消费者，「已验证待接线」；多纹理材质窗归批 6（D7）。
- per-frame SetImage 上传池化 = 在册尾巴（同 fillShaderAsImage scratch）。

## 5 交付记录（2026-11-05，已提交 `4309c454`、未推送）

**落地清单**：
- `render/shader_effect.go`：契约扩展（纹理对 (1,2)/(3,4)/… 包级 8 对、逐对配对校验、
  uniform 预算 256→`ShaderUniformBudgetMax=65536`）+ `MaxShaderImagePairs`/
  `ShaderUniformBudgetMax` 常量 + `ShaderInterface.TexturePairs/UniformBytes` 派生 +
  `SetImageAt(slot, img)/ImageAt(slot)`（SetImage/Image = slot 0 别名，P3.2 调用方零迁移）+
  `SetUniformBytes` 上限改 65536 + `declaredBindInterface` 替代 `declaresImageBinding`。
- `render/internal/gpu/user_shader.go` + `user_shader_fill.go`：填充路 bgl 按效果派生
  （逐对 texture@2k+1/sampler@2k+2）+ uniform buffer 尺寸按声明（>256B 放开）+ 绑定组逐对
  喂 `effect.ImageAt(k)`（未绑对落 1×1 黑占位）。
- `render/internal/gpu/material_pipeline.go` + `render_session.go`：材质 bgl0 按效果派生
  （全局两形退役）+ uniform 槽按声明尺寸 + **SetImageAt 槽位纹理缓存**（slots 1+，gen 键控
  重传，entry.destroy 全清）+ 绑定组（对 0=精灵纹理+FilterKey 采样器、对 1+=槽位纹理+
  线性采样器、未绑=黑占位）+ 绑定组缓存键含槽位代次。

**探针（必做前置）**：gles 真编译 binding 0..6 三纹理对 + 272B uniform 全链 OK
（`p37_3c_probe_gles_test.go`）；limits 实测 16/16/1000/65536/256。Metal 不可验按标记。

**钉 6 枚全绿**（`render/p37_3c_test.go` 5 + 探针 1）：契约面（三对放行+Interface 读数、
孤绑定/超对拒绝、288B 放行、超 65536 拒绝）、SetImageAt 回环/越槽/旧口别名、多纹理材质
像素（mix_level=1 ≡ 第二纹理，SetImage 槽位真绑定）、大 uniform 像素（272B split=0.5
左右半分）、填充路多纹理像素。**负向 4 条红还原复绿**（`negative.txt`）。

**契约边界钉同步（接口全量切换制）**：`p32_shader_effect_test.go`（257B 现合法、超
ShaderUniformBudgetMax 拒绝）+ `p32_shader_validate_test.go`（288B 现合法、65552B 拒绝）——
两钉锁的是旧 256B 边界，随本批契约变更同步改边界（非回归）。

**门禁**：build/vet rc=0、触及 9 文件 gofmt 净、逐文件触及面 12 文件全绿（p32 五文件 +
3a/3b/3c 三代 + 探针）、`-race` ok 3.7s、conformance 双后端 PASS、apidoc rc=0（目录补
SetImageAt/ImageAt/两常量 + 批 3c 行）、真窗 `ui_wr_p32_shader` golden 0.0000%。

**口径**：材质腿仍零生产消费者，「已验证待接线」；Metal naga 翻译同面待验（按标记）；
SetImage 上传池化=在册尾巴；多纹理材质窗随批 6（D7）。
