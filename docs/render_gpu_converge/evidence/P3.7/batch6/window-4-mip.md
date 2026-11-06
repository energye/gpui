批 6 ④ · mip 出口窗 收口账（2026-11-06；设计见同目录 `windows-3456-design.md`，窗 `examples/engine/p37_mip_exit`）
====================================================================================

一、收口对象
------------
台账 756（批 4 mip 档位接线主跳）一行 ☑→✅。renderconv 档位映射
（`tex.Filter↔DrawImageOptions.Mipmap`，批 4 明文未做、待「档位挂引擎哪个对象」
拍板）不在本窗、探针不冒充——窗只断引擎出口 `sprite.AtlasSprite{Filter,Mipmap}`
经 `ToRender()` 逐字段带到渲染命令后的画面效应。

二、五臂判据与实测差值（10 枚探针全绿；容差显式：语义像素 2 / 逐位段 0 / 金图 0）
--------------------------------------------------------------------------------
| 臂 | 断言 | 实测 |
|---|---|---|
| A mip 三档 | `Filter=Bilinear` × `Mipmap∈{None,Nearest,Linear}` 同几何 256→96（2.667x，非整数 LOD≈1.42——整数倍缩小 Nearest 与 Linear 同档可等图）：三档画面两两互异 | `mip_three_levels_differ_on_screen` ✓（内缩 4px 区字节差 none↔nearest 23032 / none↔linear 23032 / nearest↔linear 22036）；native/go 两端同数 |
| | 引擎采样契约口径：nearest/bicubic 图的 mipmap 开关被屏蔽（`EffectiveImageSamplerKey`，`game/tex/testdata/mipmap_cases.json` sampler_contract 冻结）⇒ 档位臂必须走 bilinear 路 | 首版实测抓到：三档全挂 InterpNearest 时两两差 0px——契约行为非引擎洞，臂按契约重建（如实记） |
| B filter 三档 | nearest/bilinear/bicubic × MipmapNone 同几何互异 | `filter_three_levels_differ_on_screen` ✓；首版 8px 规则棋盘配整数 4x 缩小，bilinear 采样对 (4k+1,4k+2) 恒不落格边界 ⇒ nearest≡bilinear 数学必等（实测 0px）——窗纹理设计缺陷，换 LCG 噪声（承接 ui_wr_p36_mipmap 配方，种子 12345）后成立 |
| C 放大段 | 2x 中心点 s=31.75 恰跨 32/32 纹素边界：nearest 精确取纹素绿 (60,160,90)；bilinear 解析混合 0.625 绿+0.375 红 = (112.5,126.25,82.5)；内部点 (X+95,Y+95)（s=47.25）四邻与 bicubic 16 邻全落同格 ⇒ 三档精确纹素色 | `magnify_nearest_exact_texel` / `magnify_bilinear_analytic_boundary_blend` / `magnify_three_filters_exact_interior` 三枚 ✓（首版把 bilinear 断言点也放边界上期望「内部精确」——实测 (112,126,82) 正是解析混合值，行为正确、期望写歪，按公式修正） |
| D 零值老路 | 引擎零值精灵（FilterUnset+MipmapNone）≡ 直构 render 零值精灵逐位 | `zero_value_road_bitwise` diff=0 worst=0 ✓；历史路语义在窗内钉成画面：中心 (880,444)/(1060,444) = (221,134,95) ≡ 0.3×亮线+0.7×格色的 bilinear 解析值（FilterUnset→渲染层归一 bilinear） |
| E 映射面板 | 4 Filter × 3 Mipmap 全组合 `ToRender()` 逐字段携带 + 非法词汇具名拒 + GPU op 有界 | `torender_carries_all_combinations`（12 组合）/ `parse_mipmap_refuses_bogus` / `parse_filter_refuses_bogus` / `paint_gpu_ops_bounded`（gpu_ops=25）✓ |

三、三证据合证
--------------
1. 逻辑探针 10 枚（`probes=10/10`），extra 键含 `mip_pair_diffs`、
   `zero_road_diff_px=0`、`paint_gpu_ops=25`、`mapping_combos=12`。
2. 语义像素 7 枚（终帧快照；期望色/混合值由同一份采样几何公式反推，容差 2）。
3. Golden 逐位 0（fail-closed、mask 指标带 y<32、容差显式 0；native/go 两基线
   逐位相同；播种跑与缺基线跑均判红实测）。

四、H15/H16
-----------
- H15：定帧 240 停机 + 过 gateFreezeAt=120 定格。native 门禁相六轮
  （播种后复核 + 五轮）金图恒 0.0000%（presents 240/241 两形态）。④ 为静态
  内容窗：常驻相（-manual-seconds 6，352 presents）金图同 0——如实记
  （对照 ② 动画窗常驻相 0.0755% 只报数；本窗无墙钟相位可漂）。
- H16 两变异（改码→跑→取值→还原；变异触及引擎码 `engine/sprite/atlas.go`
  的 ToRender 各删一行；还原 `git checkout --` 后 grep 双行核位 + `git diff`
  零命中 + 终验复绿）：
  1. 摘 `Mipmap: s.Mipmap`：`torender_carries_all_combinations` 红
     （(bicubic,linear)→(bicubic,none)）+ `mip_three_levels_differ_on_screen`
     红（三档全等图=回退 None 态）+ **金图 1.9829%**；
  2. 摘 `Filter: s.Filter`：携带红（(bicubic,linear)→(default,linear)）+
     `filter_three_levels_differ_on_screen` 红（三档全等图=回退 bilinear 老路）
     + **金图 1.9669%**。

五、门禁账
----------
见 `window-4-gates.md`；原始跑数见 `window-4-runs.md`。
实现期三处自身口径纠偏（如实记）：① 档位臂挂 nearest 采样路与引擎屏蔽契约相
矛盾（探针 0px 抓红）；② 8px 棋盘 × 整数缩小让 bilinear≡nearest（纹理选型缺
陷，换噪声）；③ bilinear 放大「内部精确」断言点落在纹素边界（公式反推期望混
合值后成立）。三处都是探针把自己的窗设计错误打回，不是绕引擎洞。

六、收口动作
------------
台账 756 ☑→✅（附本窗名+实测差值+收口枚号）；§7 追加批 6 ④ 行；
`batch6-window-plan.md` §5 补交付进展。
