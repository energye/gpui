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
