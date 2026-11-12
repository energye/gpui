# O6a 渲染层缓存/池盘点（2026-11-12）

> 方法：以 P2.4b 预算域表为索引，逐个核对代码里的 map/池上限、清扫与释放；
> 未逐行审计全部 800 文件，标记 `verify` 的项由 O6b 真窗 pprof 复核。
> 判定：**bounded** = 有硬上限/预算；**swept** = 按 live key 或代数清扫；
> **frame-local** = 帧内生命周期；**verify** = 代码未见到明确上限，需真窗复核。

## 1 预算域（`render/budget.go DefaultBudget`，P2.4b 唯一声明）

| 域 | 默认上限 | 运行时落点 | 状态 |
|---|---|---|---|
| process-global | 768MiB | `gpu/hal/vram_ledger.go` | bounded（进程账本） |
| image-cache | 128 条 / 64MiB（多窗公平份额 16/8MiB） | `render/internal/gpu/image_cache.go` | bounded + 多窗 FairSplit |
| picture-cache | 按帧工作集自动 sizing（MinEntries 64 公平地板） | `ui/scene.PictureTextureCache` | bounded（LRU + 工作集） |
| surface-pool | 128MiB | 设备级 surface trio 池 | bounded |
| stencil-scratch | 8 个尺寸桶 | 每 session textureSet | bounded |
| clip-mask | 24 条 / 16MiB / TTL 5s | depth-clip mask cache | bounded + TTL |
| blend-dualtex | 8 条 / 32MiB | 高级混合 BGRA 中间池 | bounded |
| layer-offscreen | 4 条 / 32MiB | 每 context 图层/效果离屏池 | bounded |
| filter-graph | 8 条/尺寸桶 / 128MiB | GPU 滤镜图 RT 池 | bounded |
| glyph-color | 4 页 | 彩色字形图集 | bounded |
| glyph-mask | 16384 条 / 4MiB | R8 字形掩码图集 | bounded |
| video-stage | 8 槽 / 64MiB | 视频纹理池 + 单次上传 | bounded |

## 2 预算外缓存/池（O6 重点）

| 缓存/池 | 位置 | 键/内容 | 上限/清理 | 状态 |
|---|---|---|---|---|
| PictureTextureCache.entries | `ui/scene/textured.go` | 稳定 CacheKey → 双缓冲纹理 | `max` 工作集 + LRU 淘汰；关闭释放 | bounded |
| PictureTextureCache.rerecordBy | 同上 | CacheKey → 原因计数（诊断） | live-key 清扫 + **硬上限 1024** 最老优先（O4） | bounded |
| PictureTextureCache.stable/churn | 同上 | CacheKey 准入计数 | BeginFrame 按 liveKeys 清扫 | swept |
| FilterResultCache | `ui/scene/filter_cache.go` | (filter key,fingerprint) → CPU pixmap+GPU slot | 240 帧未用淘汰 | swept |
| BoundaryCache | `ui/rendering/boundary_cache.go` | boundary 录制 | 120 帧未用 + 每 30 帧扫；可选 maxEntries | swept |
| layoutCache（每 RenderText） | `ui/rendering/layout_cache.go` | (text,face,size,w,lh) → 行 | **4096 条**/表，满则清表 | bounded |
| text.Cache / advanceCache | `render/text/cache.go` / `advance_cache.go` | 通用 LRU | softLimit + 最老淘汰 | bounded |
| 字形布局模板 cache | `glyph_mask_engine.go` / `gpu_text.go` | 布局模板 | `layoutCacheSoft` + 最老淘汰 | bounded |
| 彩色字形 atlas entries/lastUse | `glyph_color_atlas.go` | ColorGlyphKey → region | BudgetGlyphColor 页数 + LRU | bounded |
| brushFieldCache | `render/internal/gpu/brush_field_cache.go` | 笔刷场 | **8 条** + 最老淘汰 | bounded |
| depthClip.geomCache | `depth_clip.go` | 几何哈希 → 顶点集 | **frame-local**，提交后 `releaseFrameGeom` 清 | frame-local |
| shadowCache | `render/shadow_cache.go` | 内容键 → 阴影 tile | **128 条 / 128MiB** + 代数淘汰 | bounded |
| recording.ResourcePool | `render/recording/pool.go` | 录制内路径/图/字体 | 随 Recording 生命周期释放 | per-recording |
| res.Registry | `render/internal/gpu/res/registry.go` | 原生资源槽 | Owner 释放/失效；借用视图按帧 retire | released/swept |
| gpu_shared.msdfAtlas | `gpu_shared.go` | 页号 → 纹理/视图 | 页数受 BudgetGlyphMask | bounded |
| dualTex/filterGraph bgCache | `dual_tex_blend.go` / `filter_gpu_graph.go` | 绑定组 | 资源代次/关闭清理 | verify（O6b pprof） |
| SDF 覆盖 LUT | `render/internal/gpu` | 固定 1024 项 | 固定大小 | fixed |
| scheduler metrics ring | `ui/scheduler/metrics.go` | 帧间隔样本 | 256 环 | fixed |
| hitch_diag spans | `ui/embedder/hitch_diag.go` | 帧跨度 | 128 上限 | bounded |
| text 彩色栅格/emoji 位图 | `render/text/color_raster.go` / `draw_emoji.go` | 键 → 位图 | 软 LRU（`verify` 具体上限） | verify |
| cffBounds / autoHint caches | `render/text/cff_outline.go` / `autohint.go` | 字体解析中间结果 | 随 face/缓存上限（`verify`） | verify |

## 3 代表窗矩阵（O6b：每能力至少一扇，120s）

| 能力 | 代表窗 | 备注 |
|---|---|---|
| base/idle/VRAM | `ui_base_idle`、`ui_base_vram_cap`、`ui_base_batch_cap`、`ui_base_multiwin_cap` | multiwin 8 窗需隔离显示 |
| 文本 | `ui_wr_p33_richtext`、`ui_text_m5_anycase`、`ui_wr_p38_color_font` | m4_virtual 已 300s |
| 图片 | `ui_wr_r10_async_image`、`ui_wr_p36_mipmap`、`ui_wr_p36_animplay` | 动图/多级 mip |
| 滤镜 | `ui_wr_p34_filters`、`ui_wr_r20_filter` | 16 节点/滤镜子树缓存 |
| 图层/合成 | `ui_wr_c2_retained_scene`、`ui_wr_r18_savelayer`、`ui_wr_r4_composite` | retained/离屏层 |
| 2.5D | `ui_wr_p37_scene`、`ui_wr_p37_mat_persp` | 透视/材质/泛光 |
| 视频 | `video_vc2_soak`、`video_4k_play`、`video_retained_golden` | 4K 帧池 |
| HDR | `ui_wr_m5_hdr` | RGBA16F/tonemap |
| SVG | `ui_wr_p38_svg_showcase`、`ui_wr_p38_svg_filter` | 解析/滤镜 |
| mesh | `ui_wr_p39_meshgradient`、`ui_wr_p39a_meshgradient_grid` | 顶点网格 |
| MSAA | `ui_wr_p39b_msaa_anim`、`ui_wr_p41_evenodd_msaa` | 1x/4x |
| 几何 | `ui_wr_p35_conic`、`ui_wr_p35_patheffect`、`ui_wr_p35_persp`、`ui_wr_p35_rrect4` | 路径/透视 |

## 4 分批与门禁

- **O6b（本批）**：先跑 7 个核心能力代表窗 × 120s（native，单窗串行）：
  `ui_base_idle`、`ui_wr_p33_richtext`、`ui_wr_r10_async_image`、`ui_wr_p34_filters`、
  `ui_wr_c2_retained_scene`、`ui_wr_p37_scene`、`video_vc2_soak`。
  判据：RSS 120s steady（线 30000）+ pprof（有增长才抓）+ 窗自身 gate 结果；
  金图/hitch 既有环境红与本次内存结论分开记。
- **O6b-2（待批）**：剩余能力窗 + go 抽检（每能力至少一档 go）。
- **O6c（待批）**：2h soak 选 2 扇（建议 `ui_text_m4_virtual` + `video_vc2_soak`），
  需隔离显示；时长与窗清单请用户确认后再跑。
- **O6d**：22 窗金图 + O5 回归 + 台账/证据收口。
