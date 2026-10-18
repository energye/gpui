# CPU 1% 线重放计划（基线 db852552）

日期：2026-10-18 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/框架代码整理`
状态：**计划文档**。产品代码零改动；本地分支已停在基线 `db852552`（工作区干净）。

## 0. 一句话

以 `db852552`（实测渲染正确、上屏语义对齐标准的点）为基线，把 spin CPU 优化按**刀组**逐组重放；
每一组都必须过「双后端画面 + 示例门禁 + 写批次冲突计数 = 0 + 该刀 CPU A/B 在正确窗上重测」；
任何一组红了或口径不清就停下报告。已被直接实验证死的一刀（`f8acffea` 延迟合并上屏）不在重放范围内，
等「每趟上传区域（slab + 帧内游标 + 旧板延迟退）」改造一起上。

## 1. 为什么以 db852552 为基线（本轮实测依据）

- **基线正确性已验**：`db852552` 上 R4 真窗实测 `box=48% red=0-1% ncol≈1150-1303`、`present_mode=damage_union`（正常）；
  唯一 FAIL 是已知噪声门 `damage_ratio_avg=0.357 > 0.35`（好节点实测也 0.354~0.357，不作判据）。
- **破洞真因（可复现）**：`f8acffea` 的延迟合并上屏把「每 CB 一趟提交/一次冲刷」压成「一帧一趟提交/一次冲刷」，
  而引擎的 per-flush 上传一律写同一段缓冲偏移（`session_sdf_verts/_uniform`、`session_glyph_mask_verts/_uniform_*`、
  `image_vert_buf/_uniform_slab` 等），写批次又只在提交时 drain、重叠区间「后者覆盖前者」：
  - GL：同帧 85 条暂存写 / 9 个 `(buf,off)` 组；合并开只冲刷 13 次（`buf=4` 写 46 次只冲刷 2 次）→ 46 个填充全读最后一次写入，
    层纹理被填成整块红（导出证据 `/tmp/opencode/ab/tgt_on/ord7_exec_tex29_112x112.ppm`）；
  - native：`gpu/webgpu/swapchain_hal.go` 的 `EndFrameMerged` 把延迟的命令缓冲**直接丢掉** → 窗口黑约 4.5s；
  - 开关翻回 false：两端全部正常，且五个示例的冲突普查**全部归零**（r4/c5/pelican/base_text/light2d，spinner 本就无冲突）。
- **收益实测（这就是「不能为了 CPU 数字把渲染搞坏」的量化依据）**：
  | 窗口 | 指标 | 合并开 | 合并关 |
  |---|---|---|---|
  | R4 | `cpu_pct_avg` | 23.01 / 22.97 | 23.54 / 23.48 |
  | R4 | 帧间隔 p50/p95/p99 (ms) | 16.639 / 19.21 / 20.08 | 16.650 / 19.33 / 19.77 |
  | R4 | hitch / fps | 1–2 / 57.6 | 1–2 / 57.7 |
  | spinner | 进程 CPU 时间（12.6s 窗） | 1.85 s | 1.87 s |
  即这一刀收益 ≈0~0.5pt（且开着的那臂画的内容更少），代价是引擎级上传区域改造。

## 2. 基线与边界约定

- 基线提交：`db852552`（实测取证点）；本地 `feat/框架代码整理` 现为 `e375f442`（= 基线 + 一笔「删除一个示例」`examples/ui_polish_gallery`，无引擎代码改动）。
  重放分支以 `e375f442` 为起点；§1/§8 的实测证据均在 `db852552` 上采集，两者引擎代码一致。
- 远端 `origin/feat/框架代码整理` = `f17e76e1`（23 笔重放候选全在，未丢）。**本计划全程不 push、不操作远程仓库。**
- 重置前的工作区资产副本：`/tmp/opencode/backup_cpu1pct_20261018/`（`worktree_vs_HEAD.patch` + `files/`）、
  探针树 `/tmp/gpui-bisect/f8acffea`、本轮 dump `/tmp/opencode/ab/`（含 PPM/PNG/日志/拓查数据）。
- 提交纪律：只提交本组自己改动的文件；提交与推送都要用户确认。

## 3. 刀组清单（重放顺序，代码面已按 git 逐笔核过）

| 组 | 提交 | 代码面 | 机制 | 声称收益 | 风险/备注 |
|---|---|---|---|---|---|
| G0 | 前置（**已落地**） | `gles/write_batch.go`（守卫+纯函数）、`gles/write_batch_overlap_test.go`（新增）、`gles/queue.go`+`queue_linux.go`（`StagedWriteOverlaps`）、`gpu/hal/api.go`（接口）、`metal/noop/webgpu queue`（桩，返 0 并注明理由）、`render/internal/gpu/{render_session,gpu_render_context}.go`、`render/{context.go,context_diag.go,context_damage.go}`、`ui/scheduler/metrics.go`、`ui/embedder/pipeline_app.go` | 同一 drain 内**严格重叠**的暂存写计数（相接不算），一路通到示例 JSON（`staged_write_overlaps`） | 门禁（无收益） | 已落地；含反向验证单测（46 次同 offset → 45） |
| G1 | `c28441d7` | 9 文件：`gles/{adapter,adapter_context,adapter_context_linux,adapter_linux,device_linux,queue,queue_linux,surface}`、`ui/scheduler/metrics.go` | VAO 原子化+无锁快路、UnlockKeepBinding、损伤剪刀 blit、PresentCount 双记修复 | 提交线固定开销下降 | 中；触及 queue/surface 核心 |
| G2 | `5402484d` | 6 文件：`gles/swapchain{,_hal}`、`render/{frame,present_construct,present_frame,present_swapchain}` | damage-aware present tier-1、交换链视图缓存、损伤历史 | 16.99%→16.49%（基本持平） | 中；上屏语义 |
| G3 | `95876e57` | 3 文件：`render/internal/gpu/{image_pipeline,image_rotation_uniform,render_session}` | 旋转走 uniform 矩阵（Impeller RSTransform 同款） | 省 12 顶点重算+一次小上传 | 低 |
| G4a | `15aae17f`（拆） | 上传/后端子集：`gles/{write_batch,state_cache,queue,queue_linux,command_execute,surface,resource_linux,resource_windows,adapter_context_linux}`、`render/internal/gpu/render_session` | 写批次零拷贝暂存、执行期状态缓存扩面、提交路径收口 | 大包（无单刀数字） | **高**；含上传路径核心，必须拆开验 |
| G4b | `15aae17f`（拆） | 平台/场景子集：`ui/platform/x11_linux.go`、`ui/scene/textured.go`（+`gpu/webgpu/swapchain_hal.go`） | x11 提交路径、场景层纹理 | 同上 | 中 |
| G5 | `2128225c` | 7 文件：`gles/{gl/context_linux,surface,write_batch}`、`render/{present_construct,present_frame}`、`ui/platform/x11_linux`、`ui/scheduler/scheduler` | 调度轮询合并 + 损伤复用 | 15s 11.5%→9.1% | 中 |
| G6 | `69a22a85` | 3 文件：`gles/{shader,shader_disk_cache}`、`ui/embedder/pipeline_app` | 等待切片 + 着色器磁盘缓存 | 9.1%→7.1% | 低 |
| G7 | `e1a1b9d4` | 4 文件：`ui/rendering/{absolute,boundary_cache,picture_box}`、`ui/scheduler/metrics` | 边界缓存 CPU 准入 + 复杂度门（对齐 Flutter dl_complexity） | **本刀 A/B 自认作废，必须重测** | 中 |
| G8 | `42929d04` | 26 文件：`gles/{adapter_context*,command*,command_encoder,command_renderpass,default_fb_linux(新),device*,queue_linux,resource*,shader*,state_cache,target_adjust(新)}` + `naga/glsl`、`shader/glsl` 各 5 文件 | GLES 直渲窗口默认帧缓冲（目标坐标调整块+窗口 24/8） | 7.1%→6.4% | **最高**；改上屏语义，必须单独验画面 |
| G9 | `8c386a12` | 5 文件：`gles/{queue_linux,resource_linux,swapchain}`、`render/{present_construct,present_swapchain}` | Linux 窗改非阻塞交换（`eglSwapInterval 0`） | 7.08~7.13%→3.40~3.52% | **高**；交换语义 |
| G10 | `dc2733b6` | 10 文件：`gles/{adapter_context,adapter_context_linux,queue}`、`ui/embedder/{app,pipeline_app}`、`ui/raster/{hub,loop,threadid_linux,threadid_other,threadid_windows}` | 全进程单 GL 线程 + keep 租约守卫（**修** G1/G8 系列的绑定交接冻结画面） | CPU 与干净 HEAD 持平（噪声内） | 中；**必须与它所修的前刀成对走** |
| G11 | `3a4440a8` | 8 文件：`ui/embedder/pipeline_app`、`ui/platform/{poll_yield_linux,vsync_drm_linux,wayland_clipboard_linux,wayland_linux,x11_linux}`、`ui/scheduler/{metrics,scheduler}` | DRM vblank 事件化 + 轮询器停车 | 4.67~7.29%→3.38/3.49%（尺子 2.86~3.00%） | 中高；平台层 |
| G12 | `45867530` | 1 文件：`gles/surface` | 撤回「剪刀常驻」（对齐 Impeller 每 pass 复位） | 7.1% 持平（自撤刀） | 低；可与前一刀合并 |
| G13 | `caf2eec7` | 4 文件：`gles/surface`、`metal/queue`、`noop/queue`、`webgpu/queue` | 三家透调补合并卡点 + blit 注释对齐 | 无收益声明 | 低 |
| A1 | `e92c0015`、`c0815112` | 别线 S90 追车 2.5D：`examples/engine/chase_large` 等（基线里**尚不存在**） | — | — | 单独一步带，避免本地分支缺别线产物 |
| A2 | `05f2b5df`、`e6cea5ad` | 金图基线（R6/R14/C9/R15/C4/C5，分后端）+`.gitignore` | — | — | 必须带；各 C/R 窗门禁凭证 |
| A3 | `f6a99e87` | `examples/kit/spin_min/main.go`（retained 下补不透底色，消旋转白圈） | — | — | 低 |
| X | `45d6cdd1`、`44f71758`、`c3d875d3`、`f17e76e1` | 纯文档/诊断（无产品代码） | — | — | 跳过或当资料引用 |
| ✗ | `f8acffea` | 16 文件（`gles/queue*`、`surface`、`swapchain*`、`hal/api`、`webgpu`、`metal`、`noop`、`render/{context_damage,context_gpu,present_frame,gpu_render_context,render_session}`） | 延迟合并上屏 | ≈0~0.5pt | **剔除**，见 §5 |

顺序说明：G1→G7 的低风险部分先上，G8/G9/G10/G11 是上屏/交换语义高危刀，逐刀单独验画面；
G10 是「前刀引入的冻结画面」的补丁，不能只搬前刀不搬它。

## 4. 每步统一门禁（每组）的 Definition of Done

1. **编译**：`go build ./...`（既有例外：`examples/ui_polish_gallery` 缺 `ui/kit/*`；该目录当前已被删除并 staged，属别线动作、不在本计划范围）。
2. **画面（GL）**：受影响示例真窗抓图（`xwd -id` 是仓库认可的 oracle），与基线留档逐帧比对（至少覆盖：静态区、脏区、
   相位切换帧、关窗前最后一帧）。
3. **画面（native）**：`GPUI_BACKEND=native WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so` 同窗对照（本轮已实测：
   延迟开 = 黑窗 4.5s、延迟关 = 1.5s 起正常，这就是 native 的判据样板）。
4. **示例自检 JSON**：可用字段 `present_mode` / `damage_*` / `boundary_*` / `frame_raster_ms`；
   已知噪声（不作判据）：`fps`、`cpu` 打 0.0，`damage_ratio_avg` 略超 0.35。
5. **上传守卫 = 0（新硬门禁，G0 已落地）**：定义 = 「一次 drain 内，对同一缓冲、**严格重叠**（相接不算）的多次暂存写」。
   判据取两路，任一命中即红 → 停下报告：
   - 示例 JSON `staged_write_overlaps` 必须缺省或 0（非 0 会显式输出）；
   - 运行日志不得出现 `gles: staged write overlap buf=… label=… writes=… overlaps=…`（该诊断行每进程上限 200 行，只在命中时打）。
   跨端：**仅 GLES 计数**（它是树里唯一有合并上传的后端）；`metal`/`noop`/`webgpu` 的 `StagedWriteOverlaps` 恒 0 并注明理由（各自库内/直发暂存，不会把两次引擎写合成一个区域）。
   探针树 `/tmp/gpui-bisect/f8acffea` 的 `GPUI_WBTRACE=1`（`DBGWB flush buf=N label=… writes=M`，`M>1` 即冲突）仍可用于逐趟明细排查。
6. **性能**：该刀自己的 A/B（同机、同窗、同时长、开关前后各 3 次）并记录「尺子口径」；**不给数字不算过**。
7. **停下报告**：任一步红、口径不清、画面异常 → 停下问方向，不带病前进。

## 5. 剔除与前置待办（不在重放范围）

1. `f8acffea` 延迟合并上屏：等「每趟上传区域」改造——每趟 flush 独占缓冲区域（slab + 帧内游标 + 旧板延迟退 +
   指纹跳过按槽位；对齐 wgpu staging belt / Impeller host buffer 语义），改造完再决定是否重开
   （判据：**在正确窗上**实测出 ≥ 目标收益）。
2. 同批契约缺口（与第 1 条一起做）：
   - `gpu/webgpu/swapchain_hal.go::EndFrameMerged` 必须真提交它拿到的命令缓冲（现在直接丢弃）；
   - `gles` 写批次的零拷贝暂存要与 `gpu/hal/api.go` 里 `WriteBuffer`「立即写入（内部创建 staging）」的措辞对齐
     （slab 自有字节零拷贝；调用方 scratch 一律拷贝或显式移交给队列）。
3. **G0 上传守卫已落地**（2026-10-18）：`hal.Queue.StagedWriteOverlaps()` + 示例 JSON `staged_write_overlaps` 字段 + 命中时的 stderr 诊断行；
   纯函数决策表单测 `TestCountStagedWriteOverlaps`（含 46 次同 offset → 45 的反向验证）与 `TestNoteStagedWriteOverlapsCounts` 绿；
   基线复测 0（见 §8 与本次门禁记录）。它是「一刀优化把上传时序搞坏」的通用防线，本轮这条守卫本来就能直接抓到 `f8acffea` 形态。

## 6. 复现命令（基线取证 + 门禁）

```bash
# 基线画面（GL 抓图 oracle）
cd /home/yanghy/app/projects/gogpu/gpui
go build -o /tmp/base_r4 ./examples/ui_wr_r4_composite
SWEEP_SECS=6 SWEEP_STEP=1.0 SWEEP_START=1.2 python3 /tmp/opencode/wsweep3.py base_r4 /tmp/base_r4

# 通用窗口取证（含 native）
CAP_SECS=6 CAP_SHOTS=2 python3 /tmp/opencode/wcap.py <tag> /tmp/base_<ex>            # GL
CAP_BACKEND=native CAP_SECS=6 CAP_SHOTS=2 python3 /tmp/opencode/wcap.py <tag>_nat <bin>  # native

# 写批次冲突普查（探针树；正式版按 G0 落到 gles）
cd /tmp/gpui-bisect/f8acffea
DISPLAY=:1 GPUI_BACKEND=go RUN_SECONDS=6 GPUI_WBTRACE=1 /tmp/opencode/ab/bin/<ex> 2>&1 \
  | grep "DBGWB flush" | grep -v "writes=1$"        # 有输出 = 冲突
```

## 7. 修订表

| 日期 | 内容 |
|---|---|
| 2026-10-18 | 立项：以 db852552 为基线制定重放计划；写入本轮实测（基线画面、破洞真因、收益≈0、五示例冲突普查、native 黑窗）；确定 G0 门禁先行、f8acffea 剔除与两处契约待办 |
| 2026-10-18 | 基线取证留档（§8）：GL+native 各 10 窗抓图签名与示例 JSON 字段入库；记录 `ui_l1_spinner` 基线黑窗（指标绿）与两处噪声门；采集脚本 `/tmp/opencode/baseline_census.py` 可原地复跑对比 |
| 2026-10-18 | G0 落地：上传守卫（严格重叠计数）从 gles 写批次一路接到示例 JSON `staged_write_overlaps`；hal 接口新增 `Queue.StagedWriteOverlaps()`，三后端桩恒 0（注明理由）；新增守卫单测 2 个；目录文档 `docs/RENDER_API_CATALOG.md` 同步；回归：gles/hal/noop/scheduler 绿，webgpu/render/embedder 失败集合与基线一致（既有/抖动），apidoc 绿，基线复测 0 命中 |

## 8. 基线取证留档（2026-10-18，db852552，只读采集）

采集脚本 `/tmp/opencode/baseline_census.py`（可原地复跑做重放后对比）；每窗 `RUN_SECONDS=6`、t=2.0s 与 t=4.5s 两次 `xwd -id` 抓图；
原始证据（PNG/xwd/app.log/JSON 记录）在 `/tmp/opencode/baseline_db852552/`，汇总清单在同目录 `MANIFEST.md`。
G0 落地后的同脚本复跑留档在 `/tmp/opencode/g0_census/`：10 窗签名与基线只有 `ncol` 的动画相位浮动差（`modal/black/red/box` 全等），
守卫命中 0（无 `staged write overlap` 行、JSON 无 `staged_write_overlaps` 字段）。

| 示例 | GL 抓图签名（尺寸 ncol modal black red box） | native 抓图签名 | 备注 |
|---|---|---|---|
| `ui_wr_r4_composite` | 1200x800 ncol=1291 modal=17% black=0% red=1% box=48% / 1200x800 ncol=1114 modal=17% black=0% red=0% box=48% | 1200x800 ncol=1481 modal=17% black=0% red=1% box=48% / 1200x800 ncol=1491 modal=17% black=0% red=0% box=48% | static box=48%（关键判据）；damage_ratio 门为已知噪声 |
| `ui_wr_c5_anim_over_static` | 1200x800 ncol=1929 modal=20% black=0% red=0% box=0% / 1200x800 ncol=1917 modal=20% black=0% red=0% box=0% | 1200x800 ncol=2088 modal=20% black=0% red=0% box=0% / 1200x800 ncol=2049 modal=20% black=0% red=0% box=0% | 6s 窗对示例门禁偏短（要 ≥30s） |
| `ui_wr_c1_boundary_nest` | 1200x800 ncol=830 modal=57% black=0% red=0% box=0% / 1200x800 ncol=875 modal=50% black=0% red=0% box=0% | 1200x800 ncol=1218 modal=57% black=0% red=0% box=0% / 1200x800 ncol=1216 modal=50% black=0% red=0% box=0% |  |
| `ui_wr_c3_list_scroll` | 1200x800 ncol=988 modal=31% black=0% red=0% box=4% / 1200x800 ncol=973 modal=32% black=0% red=0% box=4% | 1200x800 ncol=2073 modal=30% black=0% red=0% box=4% / 1200x800 ncol=2138 modal=32% black=0% red=0% box=4% | 6s 窗异步图未出全（images_loaded=10/24） |
| `ui_wr_c6_savelayer_group` | 1200x800 ncol=1740 modal=30% black=0% red=0% box=0% / 1200x800 ncol=1916 modal=29% black=0% red=0% box=0% | 1200x800 ncol=2233 modal=29% black=0% red=0% box=0% / 1200x800 ncol=2238 modal=29% black=0% red=0% box=0% | 6s 窗偏短（要 ≥10s） |
| `ui_render_pelican` | 1200x700 ncol=12161 modal=6% black=0% red=0% box=0% / 1200x700 ncol=12548 modal=6% black=0% red=0% box=0% | 1200x700 ncol=11810 modal=6% black=0% red=0% box=0% / 1200x700 ncol=11882 modal=6% black=0% red=0% box=0% | 色彩最丰富，判据用 ncol>1e4 |
| `ui_render_base_text` | 800x720 ncol=3813 modal=45% black=0% red=0% box=0% / 800x720 ncol=3811 modal=45% black=0% red=0% box=0% | 800x720 ncol=4453 modal=44% black=0% red=0% box=0% / 800x720 ncol=4462 modal=44% black=0% red=0% box=0% |  |
| `ui_l1_spinner` | 480x320 ncol=3 modal=100% black=100% red=0% box=0% / 480x320 ncol=3 modal=100% black=100% red=0% box=0% | 480x320 ncol=3 modal=100% black=100% red=0% box=0% / 480x320 ncol=3 modal=100% black=100% red=0% box=0% | 基线即全黑（指标绿），既有洞，不作画面判据 |
| `engine/light2d` | 1200x800 ncol=1778 modal=6% black=0% red=0% box=0% / 1200x800 ncol=1785 modal=6% black=0% red=0% box=0% | 1200x800 ncol=1775 modal=6% black=0% red=0% box=0% / 1200x800 ncol=1782 modal=6% black=0% red=0% box=0% |  |
| `kit/spin_min` | 480x320 ncol=201 modal=98% black=98% red=0% box=0% / 480x320 ncol=174 modal=98% black=98% red=0% box=0% | 480x320 ncol=204 modal=99% black=99% red=0% box=0% / 480x320 ncol=203 modal=99% black=99% red=0% box=0% | 近黑底+居中图标，两帧有差（在动） |

**判据用法**：重放后同一脚本再跑一遍，逐窗对比签名——静态窗的 `modal%`/`box%`/`ncol` 应落在同一量级；
`black%` 明显上升、或 `ncol` 掉到个位数，即「画面被搞坏」，无论示例自检是否为绿。
动画相位使 PNG 逐位对比只在静止窗有意义（`ui_l1_spinner` 两帧完全一致、`kit/spin_min` 两帧有差即可作静止/活动判据）。

**基线已存在的异常（记录，不作回归判据）**：
1. `ui_l1_spinner` 基线即全黑（ncol=3，两帧一致）而示例自检全绿（55.8fps / damage_union / boundary_skip=7020）——P3 老示例自检只看指标不看像素，属既有洞；GL 与 native 同表现。
   考证：用 `git archive 3387a429`（该示例最后一次改动、且是基线祖先）取旧树重跑，同样是黑窗（ncol=26~31、black=100%）→ **与 CPU 线无关，早于基线数月**；作为独立既有洞记录，不进重放门禁。
2. `ui_wr_r4_composite` 的 `damage_ratio_avg` 基线就 0.353~0.358（>0.35 报 FAIL），是已记录噪声门，不作判据。
3. c5/c6 的 FAIL 行是采集时长不足（示例要求 ≥30s / ≥10s），c3 是 6s 内异步出图未完成（10/24），均与渲染正确性无关。

