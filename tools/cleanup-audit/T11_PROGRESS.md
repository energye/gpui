# render/video_direct.go 2026-10-07 18:03
RUN t11_split_generic.py rc=1   File "/home/yanghy/app/projects/gogpu/gpui/tools/cleanup-audit/t11_split_generic.py", line 15, in <module> /     assert n not in NAME2FILE, "duplicate map " + n / AssertionError: duplicate map Stats
RED 执行失败
RUN t11_prune_imports.py rc=0 BUILD_OK round=0
TOUCHED=['render/context.go', 'render/context_construct.go', 'render/context_diag.go', 'render/context_draw.go', 'render/context_output.go', 'render/context_path.go', 'render/context_style.go', 'render/context_text.go', 'render/context_transform.go', 'render/present_construct.go', 'render/present_frame.go', 'render/present_swapchain.go', 'render/present_target.go', 'render/software.go', 'render/software_blend.go', 'render/software_construct.go', 'render/software_fill.go', 'render/software_stroke.go', 'render/text.go', 'render/text_dispatch.go', 'render/text_entry.go', 'render/text_font.go', 'render/text_gpu.go', 'render/text_outline.go']
ROUND1 residual_funcs=41 gofmt_dirty=False
RED 第一轮: 有残留函数或格式未收敛
ROUND2 missing=[] multi_in_cut={'Close': ['render/context_construct.go', 'render/sdf_accelerator.go', 'render/video_direct.go', 'render/video_direct.go', 'render/video_direct.go', 'render/path.go', 'render/present_frame.go', 'render/path_builder.go', 'render/integration/ggcanvas/canvas.go', 'render/internal/gpu/gpu_texture.go', 'render/internal/gpu/vello_accelerator.go', 'render/internal/gpu/backend.go', 'render/internal/gpu/backend.go', 'render/internal/gpu/text_pipeline.go', 'render/internal/gpu/text_pipeline.go', 'render/internal/gpu/renderer.go', 'render/internal/gpu/sdf_gpu.go', 'render/internal/gpu/gpu_shared.go', 'render/internal/gpu/gpu_render_context.go', 'render/internal/gpu/vello_compute.go', 'render/internal/gpu/pipeline.go', 'render/internal/gpu/atlas.go', 'render/internal/gpu/memory.go', 'render/internal/parallel/pool.go', 'render/internal/parallel/tile_grid.go', 'render/internal/parallel/rasterizer.go', 'render/surface/path.go', 'render/surface/image_surface.go', 'render/surface/gpu_surface.go', 'render/scene/renderer.go', 'render/scene/path.go', 'render/text/source.go'], 'Acquire': ['render/video_direct.go', 'render/video_direct.go', 'render/internal/gpu/texture_pool.go', 'render/internal/gpu/res/cache.go', 'render/internal/gpu/res/registry.go'], 'Release': ['render/video_direct.go', 'render/video_direct.go', 'render/internal/gpu/depth_clip.go', 'render/internal/gpu/gpu_types.go', 'render/internal/gpu/texture_pool.go', 'render/internal/gpu/shaders.go', 'render/internal/gpu/res/cache.go', 'render/internal/gpu/res/registry.go', 'render/internal/image/mipmap.go'], 'Stats': ['render/video_direct.go', 'render/video_direct.go', 'render/video_direct.go', 'render/pixmap_pool.go', 'render/internal/gpu/pipeline_cache_core.go', 'render/internal/gpu/tile.go', 'render/internal/gpu/sparse_strips_gpu.go', 'render/internal/gpu/sparse_strips_gpu.go', 'render/internal/gpu/render_session.go', 'render/internal/gpu/texture_pool.go', 'render/internal/gpu/image_cache.go', 'render/internal/gpu/path_geometry_cache.go', 'render/internal/gpu/path_geometry_cache.go', 'render/internal/gpu/path_geometry_cache.go', 'render/internal/gpu/path_geometry_cache.go', 'render/internal/gpu/memory.go', 'render/scene/cache.go', 'render/scene/renderer.go', 'render/text/glyph_mask_atlas.go', 'render/text/glyph_cache.go', 'render/text/subpixel.go', 'render/text/msdf/atlas.go', 'render/text/msdf/atlas.go', 'render/text/cache/shaping.go'], 'destroy': ['render/video_direct.go', 'render/video_direct.go', 'render/internal/gpu/texture.go', 'render/internal/gpu/convex_renderer.go', 'render/internal/gpu/stencil_renderer.go'], 'AcquireForFrame': ['render/video_direct.go', 'render/video_direct.go']}
RED 第二轮: 丢失或本刀内重复
ROUND3 build_rc=0
ROUND3 vet_new=[]
TESTS pat=TestVideo|TestVideoBridge|TestVideoTextu rc=0
ROUND3 passed=0 fails=[] known=['TestStrokeExpansion_ScaledDashedRect', 'TestStrokeString_DifferentFromFill']
VERDICT=RED

# render/video_direct.go 2026-10-07 18:06
RUN t11_split_generic.py rc=0 video_texpool.go funcs=10 / UNMAPPED=[] / DONE
RUN t11_prune_imports.py rc=0 round=0 pruned=10 / round=1 pruned=5 / BUILD_OK round=2
TOUCHED=['render/context.go', 'render/context_construct.go', 'render/context_diag.go', 'render/context_draw.go', 'render/context_output.go', 'render/context_path.go', 'render/context_style.go', 'render/context_text.go', 'render/context_transform.go', 'render/present_construct.go', 'render/present_frame.go', 'render/present_swapchain.go', 'render/present_target.go', 'render/software.go', 'render/software_blend.go', 'render/software_construct.go', 'render/software_fill.go', 'render/software_stroke.go', 'render/text.go', 'render/text_dispatch.go', 'render/text_entry.go', 'render/text_font.go', 'render/text_gpu.go', 'render/text_outline.go', 'render/video_backend.go', 'render/video_bridge.go', 'render/video_direct.go', 'render/video_planepool.go', 'render/video_texpool.go']
ROUND1 residual_funcs=0 gofmt_dirty=False
ROUND2 missing=[] multi_in_cut={}
ROUND3 build_rc=0
ROUND3 vet_new=[]
TESTS pat=TestVideoBackendQueryBothCopyModes|TestV rc=0
ROUND3 passed=0 fails=[] known=['TestStrokeExpansion_ScaledDashedRect', 'TestStrokeString_DifferentFromFill']
RED 单测零通过: 用例名可能没命中
VERDICT=RED

# render/video_direct.go (manual completion 2026-10-07)
41 funcs -> video_backend/texpool/planepool/bridge; ROUND1/2/3 GREEN
(hand-run: residual 0, recv-qualified unique, build+vet clean,
video batch 19 PASS, path batch only 2 known-fail, deps+examples build ok,
gofmt clean). VERDICT=GREEN
# T1.1 队列重建
QUEUED accelerator.go lines=650 funcs=10 groups=1 tests=20
REVIEW-PASS adapter_policy.go lines=276 funcs=9 无需拆分
REVIEW-PASS backend.go lines=108 funcs=4 无需拆分
REVIEW-PASS blendmode.go lines=34 funcs=0 无需拆分
REVIEW-PASS brush.go lines=174 funcs=12 无需拆分
REVIEW-PASS brush_custom.go lines=249 funcs=11 无需拆分
REVIEW-PASS clip_op.go lines=85 funcs=2 无需拆分
REVIEW-PASS color.go lines=240 funcs=14 无需拆分
REVIEW-PASS coverage_filler.go lines=71 funcs=2 无需拆分
REVIEW-PASS curve.go lines=22 funcs=0 无需拆分
REVIEW-PASS curve_cubicbez.go lines=207 funcs=12 无需拆分
REVIEW-PASS curve_line.go lines=86 funcs=10 无需拆分
REVIEW-PASS curve_quadbez.go lines=162 funcs=9 无需拆分
REVIEW-PASS curve_rect.go lines=61 funcs=5 无需拆分
REVIEW-PASS dash.go lines=196 funcs=9 无需拆分
REVIEW-PASS depth_r5.go lines=169 funcs=6 无需拆分
REVIEW-PASS device_provider.go lines=78 funcs=5 无需拆分
REVIEW-PASS doc.go lines=70 funcs=0 无需拆分
REVIEW-PASS filter_ops.go lines=37 funcs=0 无需拆分
REVIEW-PASS filter_ops_cpu.go lines=282 funcs=15 无需拆分
QUEUED filter_ops_gpu.go lines=697 funcs=21 groups=2 tests=0
REVIEW-PASS frame.go lines=354 funcs=13 无需拆分
REVIEW-PASS gradient.go lines=194 funcs=7 无需拆分
REVIEW-PASS gradient_linear.go lines=93 funcs=5 无需拆分
REVIEW-PASS gradient_radial.go lines=202 funcs=9 无需拆分
REVIEW-PASS gradient_sweep.go lines=157 funcs=8 无需拆分
REVIEW-PASS lcd_layout.go lines=37 funcs=0 无需拆分
REVIEW-PASS logger.go lines=97 funcs=5 无需拆分
REVIEW-PASS m4_dither.go lines=92 funcs=4 无需拆分
REVIEW-PASS m4_extensions.go lines=38 funcs=0 无需拆分
REVIEW-PASS m4_quadclass.go lines=145 funcs=10 无需拆分
REVIEW-PASS m4_quadimage.go lines=301 funcs=8 无需拆分
REVIEW-PASS m4_trisampler.go lines=49 funcs=2 无需拆分
REVIEW-PASS mask.go lines=220 funcs=15 无需拆分
REVIEW-PASS matrix.go lines=212 funcs=15 无需拆分
REVIEW-PASS nine_patch.go lines=120 funcs=1 无需拆分
REVIEW-PASS oom_exit.go lines=73 funcs=2 无需拆分
REVIEW-PASS oom_purge.go lines=95 funcs=4 无需拆分
REVIEW-PASS options.go lines=106 funcs=5 无需拆分
REVIEW-PASS paint.go lines=304 funcs=15 无需拆分
REVIEW-PASS painter.go lines=73 funcs=3 无需拆分
REVIEW-PASS path.go lines=29 funcs=0 无需拆分
REVIEW-PASS path_boolean.go lines=118 funcs=2 无需拆分
REVIEW-PASS path_build.go lines=262 funcs=16 无需拆分
REVIEW-PASS path_builder.go lines=158 funcs=14 无需拆分
REVIEW-PASS path_metrics.go lines=218 funcs=10 无需拆分
REVIEW-PASS path_ops.go lines=15 funcs=0 无需拆分
REVIEW-PASS path_query.go lines=78 funcs=5 无需拆分
REVIEW-PASS path_svg.go lines=692 funcs=28 内聚单域无需拆分
REVIEW-PASS path_verb.go lines=134 funcs=8 无需拆分
REVIEW-PASS pathops_area.go lines=81 funcs=4 无需拆分
REVIEW-PASS pathops_bbox.go lines=187 funcs=9 无需拆分
REVIEW-PASS pathops_subpath.go lines=576 funcs=14 内聚单域无需拆分
REVIEW-PASS pathops_winding.go lines=204 funcs=11 无需拆分
REVIEW-PASS pattern.go lines=32 funcs=2 无需拆分
REVIEW-PASS pipeline_mode.go lines=92 funcs=2 无需拆分
REVIEW-PASS pixmap.go lines=486 funcs=26 内聚单域无需拆分
REVIEW-PASS pixmap_pool.go lines=234 funcs=14 无需拆分
REVIEW-PASS point.go lines=96 funcs=13 无需拆分
REVIEW-PASS present.go lines=128 funcs=8 无需拆分
REVIEW-PASS present_construct.go lines=584 funcs=8 内聚单域无需拆分
REVIEW-PASS present_frame.go lines=442 funcs=11 内聚单域无需拆分
REVIEW-PASS present_swapchain.go lines=453 funcs=17 内聚单域无需拆分
REVIEW-PASS present_swapchain_gl.go lines=30 funcs=1 无需拆分
REVIEW-PASS present_swapchain_nogl.go lines=23 funcs=1 无需拆分
REVIEW-PASS rasterizer_mode.go lines=76 funcs=1 无需拆分
REVIEW-PASS renderer.go lines=22 funcs=0 无需拆分
REVIEW-PASS sample_count.go lines=58 funcs=2 无需拆分
REVIEW-PASS sdf.go lines=113 funcs=6 无需拆分
REVIEW-PASS sdf_accelerator.go lines=366 funcs=19 无需拆分
QUEUED shape_detect.go lines=737 funcs=13 groups=1 tests=21
REVIEW-PASS shapes.go lines=35 funcs=1 无需拆分
REVIEW-PASS shared_budget.go lines=149 funcs=11 无需拆分
REVIEW-PASS shared_recovery.go lines=180 funcs=3 无需拆分
REVIEW-PASS solver.go lines=215 funcs=9 无需拆分
REVIEW-PASS stroke.go lines=158 funcs=17 无需拆分
REVIEW-PASS stroke_user_expand.go lines=140 funcs=3 无需拆分
REVIEW-PASS vec.go lines=139 funcs=20 无需拆分
REVIEW-PASS vertices.go lines=119 funcs=0 无需拆分
REVIEW-PASS vertices_atlas.go lines=486 funcs=11 内聚单域无需拆分
QUEUED vertices_mesh.go lines=534 funcs=14 groups=3 tests=0
REVIEW-PASS video_backend.go lines=157 funcs=6 无需拆分
REVIEW-PASS video_bridge.go lines=356 funcs=11 无需拆分
QUEUED video_planepool.go lines=431 funcs=14 groups=3 tests=0
REVIEW-PASS video_texpool.go lines=301 funcs=10 无需拆分
REVIEW-PASS vram_ledger_forward.go lines=23 funcs=2 无需拆分

# T1.1 render 顶层收尾（手动验收）
- accelerator.go: 过（650 行 10 函数，单域内聚加速器开关，不拆）
- shape_detect.go: 过（737 行 13 函数，单域内聚形状识别，不拆）
- 其余顶层文件：队列规则 REVIEW-PASS（400 行内或单域小文件）
- T1.1 render 顶层关账：context/software/text/present/video/curve/filter/m4/path/path_ops/vertices 共 11 刀已拆验收，其余记通过
