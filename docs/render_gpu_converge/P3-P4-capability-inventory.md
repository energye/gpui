# P3/P4 能力级覆盖清单（2026-11-10 审计）

> 性质：**只读审计**。上一份是任务级（每个任务有窗/有测试）；本文件下钻到能力/子能力，
> 每行给「主单测 / 主真窗 / 次要载体 / 缺口」。结论先说：**没有全部找全**——已发现
> 一个能力实现缺口（SVG marker）和多项「有实现无单测/无窗卡」的子能力；任务级 ✅ 不等于
> 能力级全覆盖。

## 0 口径

- 能力来源：公开枚举/构造器（`ImageFilterKind` 15 种、`PathEffect` 7 种、`MipmapMode`
  3 档、图片格式、SVG G1–G14、COLRv1 paint、P3.7 六批、P4.1 四子项）+ 各任务 evidence。
- 主单测：该能力的第一责任测试文件/表；主真窗：该能力的第一责任真窗（卡/probe）。
- 次要载体：legacy 回归窗（`ui_render_base_*`）、stress/组合窗（`svg_showcase`）、
  测量窗（`p39b`）、直画/回放两半中的另一半；**不计主覆盖**。
- 未逐项核的格子标「待核」；不把待核当绿。

## 1 P3 能力矩阵

### P3.1 色彩管理 / 线性混合

| 能力 | 主单测 | 主真窗 | 次要 | 结论 |
|---|---|---|---|---|
| BlendSpace SRGB/Linear、固定线性混合、渐变/画笔插值 | `p31_blend_space_test.go`、`p31_fixed_linear_test.go` | `ui_wr_p31_linear_blend` | conformance 台 | ✅ |

### P3.2 自定义 shader

| 能力 | 主单测 | 主真窗 | 次要 | 结论 |
|---|---|---|---|---|
| ShaderEffect 绘制/校验/预乘/泄漏/pipeline cache | `p32_shader_effect_test.go`、`p32_shader_validate_test.go`、`p32_shader_premul_test.go`、`p32_shader_leak_test.go`、`p32_pipeline_cache_test.go` | `ui_wr_p32_shader` | — | ✅ |

### P3.3 富文本

| 能力 | 主单测 | 主真窗 | 次要 | 结论 |
|---|---|---|---|---|
| ParagraphBuilder/逐 run 样式/换行/装饰/CJK | `render/paragraph_test.go`、`paragraph_cjk_test.go`、`ui/rendering/paragraph_style_test.go`、`paragraph_wrap_test.go` | `ui_wr_p33_richtext` | `ui/rendering/paragraph_test.go` | ✅ |

### P3.4 滤镜家族（15 节点 + MaskFilter）

| 能力 | 主单测 | 主真窗（p34_filters） | 次要 | 结论 |
|---|---|---|---|---|
| Blur / BlurXY | `filter_ops_gpu_test.go`、`filter_ops_contract_test.go` | **无卡** | `ui_render_base_filterops` B（BlurXY）、`p38_svg_filter`（SVG fe） | ⚠️ 基座主窗缺卡 |
| DropShadow | `p04_filter_gpu_test.go`、`p1_capability_matrix_closers_test.go` | **无卡** | — | ⚠️ 无窗卡 |
| ColorMatrix | `filter_ops_contract_test.go`、`p1_capability_matrix_closers_test.go` | **无卡** | `p38_svg_filter` feColorMatrix（SVG 语义） | ⚠️ 无基座窗卡 |
| Grayscale | `filter_ops_contract_test.go` | **无卡** | `p38_svg_filter` | ⚠️ 无基座窗卡 |
| Invert | `filter_ops_contract_test.go`、`filter_ops_gpu_test.go` | **无卡** | `ui_render_base_filterops` A | ⚠️ 无基座主窗卡 |
| Erode / Dilate | **无节点级单测**（只有 `internal/filter/morphology_test.go` 核函数） | A 卡（Dilate/Erode 对偶） | — | ⚠️ 有窗无单测 |
| Offset | `paragraph_test.go`（Merge+Displacement 组） | **无卡** | — | ⚠️ 无窗卡/单测归属弱 |
| Merge | `paragraph_test.go` | B 卡 | — | ✅ |
| Displacement | **无基座节点单测**（`svg/filter4b_test.go` 是 SVG fe） | **无卡** | — | ⚠️ CPU 有实现、GPU aux 未接；无基座载体 |
| Lighting | `filter_gpu_p189_test.go` | **无卡** | — | ⚠️ 无窗卡 |
| Tile | `filter_gpu_p189_test.go` | **无卡** | — | ⚠️ 无窗卡 |
| AlphaThreshold | **无单测** | **无卡** | — | ⚠️ 完全缺口 |
| Bloom | `p37_5_bloom_test.go`、`p37_5_bloom_gpu_test.go` | **无 P3.4 卡** | `engine/p37_bloom`（2.5D 层，不同能力） | ⚠️ 基座节点无主窗 |
| Paint.MaskFilter | `p34_gl_maskfilter_fill_test.go`、`p34_paint_filter_gpu_route_test.go` | C 卡 | — | ✅ |
| ApplyMask/PushMaskLayer + blur | 部分（mask 测试） | **无卡** | — | ⚠️ 残留（P3.4 在册） |

> P3.4 任务级「6/6」是**探针数**，不是 15 节点的窗卡覆盖；上表是能力级结论。

### P3.5 路径与几何

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| Conic 曲线 | `p35_conic_effect_test.go` | `ui_wr_p35_conic` | ✅ |
| RRect 四角/退化 | `p35_cliprrect_paths_test.go`、`rrect_test.go`、`cliprrect_consumer_test.go` | `ui_wr_p35_rrect4` | ✅ |
| Matrix4 透视 quad | `matrix4_test.go`、`matrix4_quad_persp_*` | `ui_wr_p35_persp` | ✅ |
| PathEffect 7 种（dash/corner/discrete/1D×3/2D/sum/compose） | `path_effect_test.go`、`path_effect_gpu_test.go`、`recording/path_effect_test.go` | `ui_wr_p35_patheffect`（A/B/C 三卡覆盖全族） | ✅ |
| 布尔路径 | `pathbool`/path op 测试 | `ui_render_base_pathbool` | ✅ |
| 面积/裁剪修正 | `p35_*`、`clip_ops_semantics_test.go` | 随上述窗 | ✅ |

### P3.6 图片格式 / mipmap / 动图

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| 格式嗅探/解码（PNG/JPEG/GIF/WebP）、AVIF/HEIF 具名拒绝 | `image_decode_p36_test.go` | `ui_wr_p36_imgfmt` C 卡 | ✅（逐格式 case 在单测，窗用嗅探卡） |
| EXIF 8 方向 | `image_decode_p36_test.go` | imgfmt A 卡 | ✅ |
| 动图合成（restore-prev/bg/keep-erase）、GIF/APNG/WebP | `image_anim_p36_test.go` | imgfmt B + `p36_animplay` A | ✅ |
| Mipmap None/Nearest/Linear + 显存 | `internal/image/mipmap_test.go`、`mipmap_downsample_test.go` | `ui_wr_p36_mipmap` | ✅ |
| 组件入口（anim image state） | `ui/kit/anim_image_state_test.go` | `ui_wr_p36_animplay` C | ✅ |
| 每格式逐项窗卡 | 单测有 case | **未逐格式开卡**（按 §4.1 一能力一窗，C 卡嗅探+单测足够） | 待核（是否需要在 C 卡列格式矩阵） |

### P3.7 2.5D

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| 透视/合批/腿染色/quad 权重/Canvas 4×4 | `p37_projection_*`、`p37_atlas_persp_test.go`、`p37_quad_*`、`p37_mat_persp_test.go` | `engine/p37_persp_batch` | ✅ |
| 世界高度/z 序 | `p37_draw_order_test.go` + 相关 | `engine/p37_height_zsort` | ✅ |
| 材质（多纹理/256B/描边洞/scene 编码） | `p37_3a..3d_test.go`、`p37_6_*` | `engine/p37_material` | ✅ |
| mip 档位接线 | `p37_*` / sprite 测试 | `engine/p37_mip_exit` | ✅ |
| 泛光（RGBA8 近似） | `p37_5_bloom_test.go`、`p37_5_bloom_gpu_test.go` | `engine/p37_bloom` | ✅（真 float GPU 归 M5，已登记） |
| 场景流 | `p37_6_*` | `engine/25d_basis` | ✅ |

### P3.8 SVG / 彩色字形 / Picture

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| G1–G5 文档/pAR/单位/继承/use/symbol/嵌套 svg | `p38_doc_test.go`、`c4*_test.go` | `ui_wr_p38_svg_doc`（+ structure 卡 1/4） | ✅ |
| G6–G10 渐变/clip/mask/image/pattern | `p38_paint_test.go`、`p38_clip/mask/image/pattern*_test.go` | `ui_wr_p38_svg_paint`（+ structure 卡 3/4） | ✅ |
| G11 text/tspan/textPath | `p38_text_test.go`、`filter4b4_test.go` | `ui_wr_p38_svg_text` | ✅ |
| G12 SVG filter（fe 子集 8 原语） | `p38_filter_test.go`、`filter4b*_test.go` | `ui_wr_p38_svg_filter` | ✅（按声明子集，不是全部 fe） |
| G13 嵌套 svg | `p38_doc_test.go` | svg_doc | ✅ |
| G14 switch / a=g / foreignObject/SMIL/script 具名拒绝 | `c4_test.go`、switch 测试 | structure 卡 1 | ✅（switch/a） |
| **G14 marker** | **无实现**（`render/svg` 只有 feature 字符串，无 Marker 解析/渲染） | **无** | ❌ **能力缺口（D2 声明要做）** |
| 彩色字形 CBDT/sbix/SVG/COLRv1 静态 | `color_font_adapter_test.go`、`color_glyph_test.go`、`colrv1_test.go`、`color_font_e2e_test.go` | `ui_wr_p38_color_font` 四卡 | ✅ |
| Picture 嵌套/环/深度 | `p37_6_scene_replay_test.go`、picture 测试 | `ui_wr_p38_picture` | ✅ |

> `ui_wr_p38_svg_showcase` 是全面压测/对照窗（**次要**，不是单项主载体）；
> `ui_render_base_svg` 是 legacy 回归（次要）。

### P3.9 / P3.9a / P3.9b

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| MeshPatch Coons 12/TensorProduct 16 | `mesh_patch_test.go`、`mesh_patch_gpu_test.go` | `ui_wr_p39_meshgradient` | ✅ |
| MeshGradient 网格/Bilinear/CatmullRom | `mesh_gradient_test.go`、`mesh_gradient_gpu_test.go` | `ui_wr_p39a_meshgradient_grid` | ✅ |
| 1x/4x 性能测量 | —（测量，N/A） | `ui_wr_p39b_msaa_anim` | ✅/N/A |

## 2 P4 能力矩阵

### P4.1

| 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|
| ① MSAA even-odd + go4x 修复 | `clip_evenodd_msaa_test.go`、`gles_msaa_resolve_test.go` | `ui_wr_p41_evenodd_msaa` | ✅ |
| ② 分数裁剪 AA | `clip_fractional_test.go` | `ui_wr_p41_fractional_clip` | ✅ |
| ③ mesh paint alpha/blend | `vertices_paint_blend_test.go`（4 case） | `ui_wr_p41_mesh_paint_blend` A/C/D | ⚠️ B（Plus/Porter-Duff GPU）FAIL；全族未覆盖 |
| ④ DrawShadow 设备空间抬升 | `drawshadow_projection_test.go`（锁 CPU） | `ui_wr_p41_drawshadow` | ✅ 语义；GPU #4 待复验 |
| DrawVertices/DrawMesh live 回放 | **无专门 round-trip** | mesh 窗 live 已实测（op 计数） | ⚠️ 测试缺口 |
| ③ 全 Porter-Duff 族 | **无全族表**（只有 Plus/Multiply/alpha/solid） | 同上 B 卡 | ⚠️ 全族覆盖缺口 |

### P4.2 / P4.3 / P4.4

| 任务 | 能力 | 主单测 | 主真窗 | 结论 |
|---|---|---|---|---|
| P4.2 | GPU/CPU 模糊一致 + 渐变 ramp 域 | 模糊/图已有对照测试；**ramp 域无专测** | 无新窗（复用 p34_filters / p38_svg_paint / p31_linear_blend） | ⬜ 未开工；范围需重划 |
| P4.3 | 阴影模糊裁剪 + 跨帧缓存 | `shadow_cache_test.go`、`p43_shadow_filter_source_test.go`、`p43_tier5_draw_order_test.go`、`p43_upload_plate_probe_test.go` | `ui_wr_p41_drawshadow` | ✅ 阴影专项 |
| P4.3 | mask 池化 / backdrop 损伤区 / 抖动 | **无** | **无**（复用 stress/backdrop/filter 窗） | ⬜ 未做（先测） |
| P4.4 | COLRv1 Var/CPAL | **无**（colrv1_test 只有默认实例静态） | **无**（扩 p38_color_font 卡） | ⬜ 未开工 |

## 3 重复性判定

**真主载体（唯一）**：每行第一列的「主单测/主真窗」。下列不算重复：
- **不同层同原语**：p35_persp（render 基座 Matrix4）vs p37_persp_batch（2.5D 相机）；
  p34_filters（基座 ImageFilter）vs p38_svg_filter（SVG fe 语义）；p37_bloom（2.5D 泛光）
  vs P3.4 ImageFilterBloom；p39/p39a（mesh 几何）vs p41_mesh（paint alpha/blend）。
- **直画 + 回放两半**：`render/*_test.go` 与 `render/recording/*_test.go` 同能力配对，不按两个能力计。
- **次要载体**：`ui_wr_p38_svg_showcase`（组合压测）、`ui_wr_p39b_msaa_anim`（测量）、
  `ui_render_base_*`（legacy 回归）、stress/soak（性能）——不计主覆盖，也不删。

**真重复/需防双计**：
- P3.4 的基座滤镜：`ui_render_base_filterops` 不能再当 P3.4 主窗；主窗应是 p34_filters（扩展）。
- P3.8 的 `svg_showcase` 不能当单项 SVG 能力主窗；主窗是 doc/paint/text/filter/structure。
- 工作区未提交的 `p37_mat_persp_bisect/fullseq/record_test.go` 是别线，不计覆盖。

## 4 缺口清单（按优先级）

1. **P3.8 G14 marker 未实现**（D2 声明要做；`render/svg` 无 Marker 解析/渲染/测试/窗）——能力缺口，不只是测试缺口。
2. **P4.1 #3 全 Porter-Duff 族**：只有 Plus/Multiply 等 4 case；GPU 路径 B 卡 FAIL。
3. **P3.4 基座节点**：AlphaThreshold 无单测无窗；Displacement 基座无单测/GPU aux 未接；Erode/Dilate 只有窗卡无节点单测；Offset/Lighting/Tile/ColorMatrix/DropShadow/Grayscale/Blur/BlurXY/Invert 无 p34 主窗卡；Bloom 无 P3.4 主窗。
4. **P4.2**：ramp 域无专测；模糊/图已有对照，先复跑再定。
5. **P4.4**：Var/CPAL 无单测/窗。
6. **P4.3 其余**：无单测/窗，先量。
7. **DrawVertices/DrawMesh 回放**：无专门 round-trip 测试。
8. **live 帧**：无通用断言（P4.1 四扇 hook 已删；mesh 只有 op 计数）；需共享 harness 或 replay parity 兜底。
9. **P3.6 每格式窗卡**、**P3.8 G-list 逐项窗卡**：标「待核」，不直接标绿。

## 5 下一步

- 先把本清单交给用户确认范围（marker 是否补实现、P3.4 节点是否全部加 p34 卡、P4.2 范围）。
- 再按 §4 优先级排：marker（能力缺口）→ P4.1 #3 全族 → P3.4 节点补测/补卡 → P4.2 复跑 → P4.4 → P4.3 量测 → 回放/live 通用设施。
