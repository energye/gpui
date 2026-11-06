缺口 4 影响面评估与方案（2026-11-06，开工前停报告版；用户原话已拍档 A，映射语义三决策点按本文推荐执行、提交时逐枚把关）
========================================================================================================================

被测缺口（一句话）：`engine/tex` 有一整套完备的采样档位（`tex.Filter{Near,Far,Mip,MaxAniso}`
+ `Normalized`/`SamplerParams`），但仓里**没有任何一条路把它带进渲染命令**——
`renderconv` 六个转换函数对 `Mip|Filter` 零命中，tex 档位在引擎侧是孤岛。

一、码面事实（本会话逐条重核实，非记忆；复跑命令见 §五）
--------------------------------------------------------

**真源侧（完备无缺）：**
- `engine/tex/mipmap.go:104-109` `Filter{Near, Far Kind; Mip MipMode; MaxAniso uint16}`；
  `:59-68` `MipMode = render.MipmapMode` 别名（D11：render 持真值，tex 不存第二份）；
  `:144-156` `Normalized()` 把非法轴归历史默认；`:204-210` `SamplerParams()` 折算
  `(magLinear, minLinear, mipLinear, aniso)`。
- 批 4 当年登记的「`engine/tex` 无持档位对象」**作废**（总账已改口）：持档位对象就是
  `tex.Filter`。

**转换层（断点所在）：**
- `engine/renderconv/conv.go` 全文 6 函数（Vec2/Color/Mat2D 双向），全目录对 `Mip|Filter`
  **零命中**（grep rc=1 实测）。
- `engine/renderconv/doc.go` 冻结宪法：「逐字段拷贝、不舍入、不钳位、**不给默认**」+
  「只允许加法式变更」⇒ 加 `FilterToRender` 合法；但函数体内**自作归一/默认映射**违反本包宪法。

**两条既有路对照：**
- sprite 路（通）：`engine/sprite/atlas.go:138-141` 精灵直持 `render.InterpolationMode` +
  `render.MipmapMode`，`:266-306` `ToRender/AtlasToRender` 逐字段带到渲染命令（批 4 主跳）。
- tex 路（断）：`tex.Table`/`tex.Filter` 全仓非测试消费者 = **0**（唯一消费者是
  `examples/engine/tex` 演示窗）；引擎帧提交只走 `AtlasToRender`，从不查 tex 表。

**render 侧落点（决定映射上限）：**
- 精灵路 `render/vertices.go:77` `AtlasSprite{Filter, Mipmap}`，**无 aniso 字段**（批 4
  登记「aniso 不进 sprite，不承诺不存在能力」）；单图路 `render/context_image.go:174-179`
  `DrawImageOptions{Interpolation, Mipmap}`，aniso 走内部 `SetPerImageFilter`
  （`render/internal/gpu/image_pipeline.go:1174`），公开口同样不带。
- GPU 采样器 **mag/min 永远同档**（`image_pipeline.go:836-851、1195-1223` 两族描述符全部
  `MagFilter==MinFilter`），全管线无分离采样器 ⇒ `tex.Filter` 的 Near/Far 双轴**没有
  一一对应承载形**。
- 精灵路折叠点：`render/vertices_atlas.go:95/113/130/465-499` 把 `sp.Mipmap` +
  `normalizeAtlasFilter` 的 nearest 位喂进同一条 `ImageDrawCommand`，其 `FilterKey()` =
  `EffectiveImageSamplerKey`（`image_pipeline.go:1154`）——nearest/bicubic 屏蔽 mip/aniso
  （`game/tex/testdata/mipmap_cases.json` "sampler_contract" 冻结）⇒ mip 臂必须走 bilinear 路。

**两扇窗现状：**
- ④ `examples/engine/p37_mip_exit/main.go:27-28` 明文登记「tex.Filter↔AtlasFilter 的
  renderconv 映射不在本窗，探针不冒充」——本步就是兑现这句登记。
- ⑥ `examples/ui_wr_p37_scene` 场景流普通图像口（`render/scene/scene.go:416` `DrawImage`）
  不带任何采样档位；quad 口只带 interp 不带 mip（批 3e-2 明确不编码）；但 `render.Canvas`
  有 `DrawImageEx`（`render/canvas.go:150-151`，带 Interpolation+Mipmap）⇒ ⑥ 面板可窗上
  直画单图路两轴。

二、方案（档 A：tex.Filter 为真源 + renderconv.FilterToRender 纯函数）
----------------------------------------------------------------------

`renderconv` 新增（additive，render/ 零行）：

    func FilterToRender(f tex.Filter) (render.InterpolationMode, render.MipmapMode, error)

映射语义（三个决策点，推荐口径=「全具名拒绝 + Mip 透传」）：

| 输入 | 处置 | 依据 |
|---|---|---|
| Near==Far==Nearest | `InterpNearest` | 同档唯一无损形 |
| Near==Far==Linear | `InterpBilinear` | 同上 |
| Near≠Far（混合轴） | **具名拒绝** `ErrFilterMixedAxes` | GPU 无分离采样器，静默选一边=renderconv 宪法「不给默认」+ H22/H23 吞开关教训 |
| 含 KindUnknown | **具名拒绝** `ErrFilterUnknownKind` | 「未选」不可映射；要默认调用方显式 `Normalized()` |
| Mip | **原样透传不归一**（MipUnknown→`MipmapNone` 保历史单层路） | 批 4 登记「归一语义与单档历史路不兼容」的本体：tex 零值=未选（Normalized→MipNearest），sprite 零值=关（H16 钉守住）；转换不偷语义 |
| MaxAniso∈{0,1} | 放行（0=关，tex 自身语义） | |
| MaxAniso>1 | **具名拒绝** `ErrFilterAnisoUnsupported` | 精灵/单图公开路均无 aniso 承载字段；静默吞开关=假装档位生效 |

**真窗证据（不许只靠单测标绿）：**
- ④ `p37_mip_exit` 加臂 F「tex 驱动面板」：三格 tex.Filter → FilterToRender →
  `sprite.AtlasSprite{Filter,Mipmap}` → `ToRender()`，与既有枚举臂**同几何同源图**做
  窗上跨臂逐位等价（F1≡b0、F2≡a1、F3≡a2，diff=0）——「转换忠实携带」的最强像素证据；
  另加两枚拒绝牌（混合轴、aniso>1）+ 像素探针。
- ⑥ `ui_wr_p37_scene` 加面板 E「tex 档位（单图路）」：窗上 `DrawImageEx` 直画三格
  tex 驱动 (Interpolation,Mipmap)，离屏腿做 tex 路≡枚举路逐位 + 两两互异；
  scene 流不编码 mip 的口径如实登记（quad 口只带 interp）。

**生产接线口径（如实）**：`FilterToRender` 落地后消费者仍是 ④/⑥ 真窗面板（原子验证），
引擎内部提交路查 `tex.Table` 的生产接线属后续批（「已验证待接线是正常中间态」口径不变）。

三、影响面
----------
- **改动面**：`engine/renderconv` 新文件（+1 公开函数 +3 哨兵错）；④/⑥ 各一臂 +
  探针/README；金图两窗重播种（先删基线→播种→双后端复验→稳定轮→cmp）。
- **不碰**：`render/` 零行、现有 6 转换零改、sprite/tex 包零改 → 存量逐位路零风险；
  `RENDER_API_CATALOG` 不涉（renderconv 是 engine 层包，apidoc 照跑确认无越界）。
- **切换面**：新函数无存量调用方，无「同笔切换」面。
- **约束照旧**：mip 臂走 bilinear 路（nearest/bicubic 契约屏蔽）；金图 fail-closed；
  ④⑥ 门禁相 8 秒（③ 的 20 秒口径只属臂 G CPU 回退腿）。

四、验收标准
------------
1. 红测先行：`engine/renderconv` 新测试先编译红（函数不存在）→ 实现后绿；
   拒绝三态 + 透传语义各有断言。
2. ④：探针 10→12 逻辑（跨臂逐位等价 3 对 + 拒绝三态）+ 语义像素 7→8（拒绝牌）；
   双后端金图 0.0000%。
3. ⑥：探针 12→14 逻辑 + 语义像素按面板落点增补；双后端金图 0.0000%。
4. 门禁：build/vet/gofmt/apidoc rc=0；触及包逐文件回归全绿。

五、复核命令（任何人可重跑）
----------------------------
    grep -rn "Mip\|Filter" engine/renderconv/*.go        # 开工前零命中 = 缺口原始判据
    grep -n "type Filter struct" engine/tex/mipmap.go    # 持档位对象
    grep -n "EffectiveImageSamplerKey" render/internal/gpu/image_pipeline.go
    grep -rn "tex\.Filter\|tex\.NewTable" --include=*.go . | grep -v _test | grep -v examples/engine/tex | grep -v "^./engine/tex"
    #   ↑ 非测试生产消费者 = 0（tex 路孤岛判据）
