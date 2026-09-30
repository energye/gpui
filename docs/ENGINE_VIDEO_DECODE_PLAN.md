# 视频解码模块立项 — ffmpeg 后端（purego 绑定 libgpui_ffmpeg，独立根模块）

> **版本：v0.15 VR2查漏补缺** | 日期：2026-09-12 | **状态：VW0双窗绿；VR2四段齐全（VR2a开工中）**
> **性质：** `gpui` 项目里的**独立根模块 `gpui/video`**（与 `ui` / `render` / `gpu` 同级，不放在渲染层），需求 + 计划 + 真窗验收唯一真源。
> **范围（ffmpeg 后端）：解码全走自建单文件 `libgpui_ffmpeg`（ffmpeg 7.1.5，LGPL 2.1）：`video/ffmpeg` 经 purego 绑定，demux + 解码 + sws 转 RGBA 全用 ffmpeg；`Player` 只做队列/时钟/搜进度编排。只剩 ffmpeg 后端。**
> **ffmpeg 库位置：** `gpui/lib/ffmpeg/<平台>/`（6 构建进仓：linux-x64/arm64/386/arm + win-x64/arm64；mac 待补）；对照源码仍在上级 `gogpu/ffmpeg` 只读。
> **并读：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)（`ui → render → gpu`）· [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)（**禁止 CGO**）· [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) · [`UI_PIXEL_ASSERTION_STANDARD.md`](./UI_PIXEL_ASSERTION_STANDARD.md)（画面断言）· [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（真窗纪律参照）。

---

## 0. 用户要求（固化 · 不可删改义）

| # | 要求 |
|---|------|
| V-U1 | 第一阶段只要常用格式里的最小闭环：**MP4 盒子 + H.264 编码**，别的盒子（MKV/AVI/MOV/WebM）和别的编码（H.265/VP9/AV1）一律后置。 |
| V-U2 | 第一阶段**只要画面**，不要声音、不要字幕。音频（AAC/Opus）、音画对齐后置（本期只做画面，声音见 §12 VW6、字幕见 §12 P3 行，不算永久不做）。 |
| V-U3 | **ffmpeg 后端，不自己重写解码**：`video/ffmpeg` 经 purego 绑定 `lib/ffmpeg` 下的自建单文件库（13 功能模块 + 数据 + 变参 Go 版：format_demux/codec_encode/packet/frame/scale_color/resample_audio/filter_graph/device_io/dict_opt/buffer_mem/error_log/media_desc/crypto_hash_misc/data_const/variadic_go；so 导出 991 个公开函数全绑 + 16 个数据全包 + 4 个 C 变参全有 Go 拼串版，见 `doc.go`）；**禁止 `import "C"`**，`CGO_ENABLED=0` 必须能编译。**`video` 核心禁止 import `ui` / `render` / `gpu`**（根模块独立性，合规门禁见 §7）；上墙显示走桥接（`video` 只出帧，消费侧一律经 `render` 层转成 `render/ImageBuf` 再 `DrawImage`，见 §4；`video` 不出 GPU 句柄/显存指针，消费侧不直调 `gpu` 包，GPU 上传归 `render` 所有）。**解码含硬解一律走 ffmpeg 通路**：软解走 `decode.go` 高层口，硬解走 `libavutil/hwcontext` 通路（VA-API/NVDEC/VideoToolbox/MediaCodec 按平台，帧回传后软解对照），`video` 不直连显卡接口。 |
| V-U4 | 直接用 ffmpeg 解码（对照源码仅定位问题，不再自研解码器）；**播放链路全走 ffmpeg 包（含硬件加速）**。 |
| V-U5 | 做为 `gpui` 里的**独立根模块 `gpui/video`**：不放在渲染层；解码细节藏在模块内，对外只给干净的打开、取帧、跳进度、关闭接口，并按组件消费设计（状态与事件齐备，`ui/kit` 视频播放组件直接包 `video` 能力，见 §4.5）；显示侧由消费侧经 `render` 层桥接进现有图片链路。 |
| V-U6 | **每个主能力必须有独立真实窗口验收**（`examples/video_vr_*/`，GPU Present），另设组合窗做集成；单测不能单独标完成。 |
| V-U7 | 每个真窗必须同时具备：① 可 `go run` 的 GPU 窗；② **stderr + JSON 指标**，不达标 `FAIL:` + `exit 1`；③ **README 写明人眼可见效果**。 |
| V-U8 | 阶段状态全程可查：主表、组合表、分期表、修订表四处同步，禁止口头标完成。 |

**术语（大白话）：**

| 词 | 意思 |
|---|------|
| 封装 / 盒子 | MP4 这个文件外壳，里面装着视频流、时间表、元信息。 |
| 编码 / 算法 | 盒子里的压缩算法，H.264 就是算法。 |
| YUV | 解码器吐出来的常用颜色存法，和屏幕要的 RGBA 长得不一样，需要转一下。 |
| 时间戳（PTS） | 每一帧该在什么时候显示，跳进度、控速全靠它。 |
| 关键帧 | 不靠别人、自己就能解出完整画面的帧，跳进度只能落到关键帧。 |
| 真窗 | 真正弹一个 GPU 窗口跑起来的验收程序，不是只跑单测。 |

---

## 1. 目标与非目标

### 1.1 目标（第一阶段做完时长啥样）

```text
打开一个手机拍的 MP4 → ffmpeg 读出宽高/帧率/时长
  → ffmpeg 后台一帧一帧解出画面并转成 RGBA → Player 只做队列/时钟/搜进度编排
  → 经 render 层当一张图送上墙（render/ImageBuf + DrawImage，不直调 gpu）
  → 能播、能停、能按时间跳、坏文件能报错不崩
```

### 1.2 非目标（第一阶段明确不做）

- 音频解码与播放、音画对齐、变速不变调（声音是下一期模块，见 §12 VW6；字幕见 §12 P3）。
- H.265 / VP8 / VP9 / AV1、MKV / AVI / MOV / WebM、网络流、直播、低延迟。
- GPU 里直接解码（不绕过 `render` 直调 `gpu`，见 §4）；硬解一律走 ffmpeg hwcontext 通路，见 V-U3 与 §12 S4。
- 倒放、逐帧步进之外的花式播放模式（可预留接口，不实现）。

---

## 2. 主能力 §VR 与单能力真窗（1 : 1）

> **规则（V-U6）：** 每一行必须有独立目录 `examples/video_vr_<id>/`，禁止多项挤一个 main 就关闭多项。
> **窗口一律 1200×800**（客户区逻辑像素）；**运行时间 ≥5s**，关闭时取 §2.5 关闭用时长。
> 关闭证据：代码 + 对应单能力真窗绿 +（若该波次含组合）相关组合窗绿 + 回写本文状态。

| ID | 能力 | 单能力真窗包名 | 窗口 | 关闭用 RUN_SECONDS | 指标门禁（须 FAIL） | 可见效果（README 必写） | 波次 | 状态 |
|----|------|----------------|------|--------------------|---------------------|-------------------------|------|------|
| **VR0** | MP4 拆盒：读出视频轨、宽高、帧率、时长、关键帧目录表 | `video_vr0_demux` | 1200×800 | 5 | §2.2 全族 + `track_found=1`、`keyframe_count≥1`、`duration_ms>0` | 左边文件信息面板，右边关键帧位置条，坏盒子报可读错 | VW0 | 🟩真窗绿 2026-09-11（120样本/30fps/1280x720/4关键帧，presents=295，track=1/kf=4/dur=4000ms，坏错可读；VW0待VR1） |
| **VR1** | H.264 参数集与帧切分：参数集解析、帧边界切分、丢参数能报错（§2.9 F1/F2/F15） | `video_vr1_params` | 1200×800 | 5 | §2.2 全族 + `sps_ok=1`、`pps_ok=1`、`frames_split≥1` + 档位等级全认 | 参数面板 + 切分出的帧数，缺参数的文件明确报错不断言崩 | VW0 | 🟩真窗绿 2026-09-12（合成4帧High/3.1与真片前8采样8帧High/4.2双路，presents≈295，缺参数坏流可读；VR0回归仍绿） |
| **VR2** | H.264 画面解码全功能：§2.9 F3–F15/F18–F20 能出正确 YUV（§2.8 全档） | `video_vr2_decode` | 1200×800 | 15 | §2.2 全族 + `frames_decoded≥N`、`yuv_ready=1`、§2.8 每档 + §2.9 每行差异在容差内 | 同一帧与对照图并排，肉眼一致，差异率进门禁 | VW1 | 🟩真窗绿 2026-09-13（§6.1四段全绿 + 真窗 RUN15 绿：15 帧差异 0/上屏 880；VW1待VR3） |
| **VR3** | 颜色转换：YUV 转自有 RGBA 帧，色偏可门禁 | `video_vr3_color` | 1200×800 | 5 | §2.2 全族 + `color_diff_per_channel≤容差`、灰阶/肤色块不断言崩 | 色卡 + 灰阶条，转完和理论色对得上 | VW1 | 🟩真窗绿 2026-09-13（11组向量色差0/真I帧转色/上屏295，容差3，见§6 VR3行） |
| **VR4** | 播放管线：后台解 + 帧队列 + 按时间戳显示 + 播/停（§2.8 全档能播） | `video_vr4_play` | 1200×800 | 30 | §2.2 全族 + 动画窗 fps 门禁（§2.2.2）+ `dropped_old_frames≥0` 可解释 + 音画位空跑（无音频不判） | 视频在窗里正常播，暂停真停，队列水位 HUD 可见 | VW2 | 🟩真窗绿 2026-09-13（解码15/显示215/零丢/直播125/上屏1740，fps57.5/p95 18.8毫秒，见§6 VR4行） |
| **VR5** | 跳进度：按时间跳到最近关键帧再往前解到目标帧 | `video_vr5_seek` | 1200×800 | 15 | §2.2 全族 + `seek_ok=1`、`seek_landing_delta_ms` 在预算内、跳后 3s 内画面恢复 | 进度条 + 跳完画面对得上，不黑屏不花屏 | VW2 | 🟩真窗绿 2026-09-13（四跳全绿/双IDR第二GOP前解4/差0/恢复≤14毫秒/直播84帧10跳/上屏877，见§6 VR5行） |
| **VR6** | 容错：坏盒子、花屏流、缺参数、截断文件，报错不崩不卡死（含 §2.9 F17/F20） | `video_vr6_fault` | 1200×800 | 15 | §2.2 全族 + `fault_cases_pass==total`、坏输入零崩溃、错误信息可读 | 坏文件列表逐个过，每个给出人话错误 | VW2 | 🟩真窗绿 2026-09-13（12例全过/花屏隔离2帧剩8帧播完/直播76/上屏870，见§6 VR6行） |
| **VR7** | 性能与内存极限优化：1080p 稳播、零热分配、帧池复用、内存封顶、GC 可控 | `video_vr7_perf` | 1200×800 | 60 | §2.2 全族 + §2.7 极限门禁（解码耗时/分配/池命中/GC/内存封顶/CPU 全 FAIL） | 同一片 60s 稳播，CPU/内存/分配/GC 四曲线 HUD 可见 | VW3 | 🟩真窗绿 2026-09-13（五站全过/直播309/分配89B/帧/池99.7%/零泄漏/峰值423MB<512MB/斜率负/GCp99 0.2毫秒/CPU28.6%/卡顿0/上屏3438/fps57.1/p95 21.5毫秒/冷解码p95 28.6毫秒，见§6 VR7行） |
| **VR8** | gpui 真窗内嵌播放：视频做为 UI 场景里的一块，与面板/文字/浮层同屏合成 | `video_vr8_embed` | 1200×800 | 30 | §2.2 全族 + 动画窗 fps 门禁 + `embed_ok=1` + 缩放/叠加后画面不断言崩 | 视频区正常播，旁边面板文字不闪，浮层盖上视频不坏 | VW3 | 🟩真窗绿 2026-09-13（冷解码5/直播151/内嵌1/两尺寸同源/裁剪透明浮层齐/改尺寸走完/尾MAD124/上屏1737/fps57.4/p95 19.8毫秒，见§6 VR8行） |
| **VR9** | 扩展注册机制：容器/解码/颜色全走注册表，加新编码不用改核心 | `video_vr9_registry` | 1200×800 | 15 | §2.2 全族 + `registry_cases_pass==total` + 不支持的格式报人话错（零崩溃） | 同一片走注册表播出；演示第二个解码器可插拔；不支持的文件明确报错 | VW3 | 🟩真窗绿 2026-09-13（注册8/8/同片播完/灰桩灰像素/三坏例人话/直播76/上屏866/fps57.4/p95 18.5毫秒，见§6 VR9行） |

**命名纪律：** 包名与上表一一对应；新增 VR 必须同步新增一行 + 一个包 + 窗口/时长两列，禁止复用他包关闭新 VR。

### 2.1 Present 与显示策略

| 策略 | 含义 | 默认 |
|------|------|------|
| `video_frame_as_image` | 解一帧 → 转 RGBA → 经 `render` 层 `DrawImage` / GPU 纹理上传上墙（上传归 `render` 所有，消费侧不直调 `gpu` 包） | **默认** |
| `damage_present` | 只有新帧到了才提交 Present，没新帧不空刷 | 播放窗默认 |
| `embed_in_ui_scene` | 视频帧做为 UI 场景里的一个矩形节点，与面板/文字/浮层一起走层合成；缩放走 `DrawImage` 目标矩形，裁剪走现有裁剪，透明叠加走不透明度（VR8 验收；`ui/kit` 视频播放组件沿此策略，见 §4.5） | UI 内嵌默认 |
| 禁止 | 没解出帧就拿上一帧冒充通过门禁；降分辨率偷过性能门禁；示例层绕过模块直调解码内部上墙 | — |

### 2.2 真窗硬性指标（参照 UI 真窗 A–J · 全族必采）

> 每个 `video_vr_*` / `video_vc_*` 结束时必须输出一族不少的 JSON 块（无数据填 `null`/`0` + `unavailable` 原因，禁止默默省略）。不满足硬 FAIL 则 `exit 1`。

| 族 | 回答什么 | 真窗必采字段（JSON） | 硬 FAIL 默认 |
|----|----------|----------------------|--------------|
| A 帧时 | 稳不稳 | `fps_wall`、`interval_avg_ms`、`interval_p50_ms`、`interval_p95_ms`、`hitch_count`、`hitch_rate_per_min`、`vsync_source`、`target_hz` | 动画/播放窗按 §2.2.2 的 60 档判；必须输出 `vsync_source` |
| B 管线 | 解码跟得上显示吗 | `decode_ms_avg`、`decode_ms_p95`、`queue_depth_avg`、`queue_depth_max`、`dropped_old_frames`、`alloc_per_frame_B`、`pool_hit_pct` | 队列持续顶满且丢帧失控 → FAIL；解码 p95 超预算 → FAIL（预算写 README）；稳态每帧分配超 §2.7 → FAIL；池命中低于 §2.7 → FAIL |
| C 播放 | 进度对吗 | `frames_decoded`、`frames_shown`、`clock_drift_ms`、`seek_ok`、`seek_landing_delta_ms` | 跳进度落点超预算 → FAIL；时钟漂移持续拉大 → FAIL |
| D CPU | 谁吃 CPU | `cpu_pct_avg`、`cpu_ui_pct`、`cpu_raster_pct`、`cpu_decode_pct` | 长窗（≥60s）超预算 → FAIL（默认建议单核折算 >85% 且持续，具体写 README；VR7/VC2 按 §2.7 收紧） |
| E 内存 | 漏不漏、封顶吗 | `rss_start_kb`、`rss_end_kb`、`rss_peak_kb`、`rss_slope_kb_per_min`、`mem_cap_kb`、`gc_pauses_ms_p99`、`heap_alloc_MB` | soak/压力窗 slope 超预算 → FAIL（默认建议 >30000 KB/min，VR7/VC2 按 §2.7 收紧）；峰值超 `mem_cap_kb` → FAIL；GC 停顿超 §2.7 → FAIL |
| F GPU | 提交/回退 | `gpu_ops`、`cpu_fallback_ops`、`last_cpu_fallback` | 热路径无故回退暴涨 → FAIL |
| G 解码 | 解得对吗 | `sps_ok`、`pps_ok`、`frames_split`、`yuv_ready`、`color_diff_per_channel`、`pixel_golden_diff_pct` | 任一正确性字段不达标 → FAIL |
| H 首帧 | 出画面快吗 | `time_to_first_frame_ms` | 超 README 预算 → FAIL（默认建议同机 <2000ms） |
| I 回归 | 变差？ | 支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` | 超容差 → FAIL |
| J 合规 | 干净？ | 无 `import "C"`、无新增第三方依赖、`go vet` 过 | 违规直接拒合入 |

#### 2.2.2 帧时门禁（播放类窗）

| 窗类型 | `target_hz` | 硬 FAIL |
|--------|-------------|---------|
| 持续播放（VR4、VR8、VC0、VC2 等） | 60 | 有效 FPS <55 或 `interval_p95_ms > 22` |
| 长跑（≥60s） | 60 | 另判 `hitch_rate_per_min` 超 README 预算（默认建议 ≤5） |
| 正确性窗（VR0/VR1/VR3 等） | — | 跑满关闭用时长且 JSON 全族齐即可，不硬卡 fps，但必须输出 `vsync_source` |

### 2.7 性能/资源/内存极限优化（硬 · 全模块生效，VR7 验收）

> 原则：正确是底线，省是本事。热路径一律按零分配设计，内存有封顶、队列有界、GC 可观测，任何一期都不接受“先跑通以后再优化”。

| 项 | 硬要求 | 门禁（不满足 → FAIL） |
|----|--------|----------------------|
| 热路径零分配 | 拆包→解码→转色→入队的稳态循环不准每帧新开大内存；帧缓冲（YUV/RGBA/比特流工作区）走池复用，尺寸不变只借不建 | 稳态 `alloc_per_frame_B` 超预算 → FAIL（预算写各窗 README，VR7 最严）；基准测试分配次数回归超标 → FAIL |
| 帧池复用 | YUV 帧池 + RGBA 帧池 + 工作区池三池独立；借还配对，泄漏可查；分辨率切换才换池，平时零增长 | `pool_hit_pct` 低于预算 → FAIL；借出未还（池泄漏）→ FAIL |
| 队列有界 | 帧队列定长（默认个位数），满了丢最旧并计数，禁止无界堆积吃内存 | 队列无界增长或顶满无计数 → FAIL |
| 内存封顶 | 解码器总占用（池 + 参考帧 + 队列）有配置上限，超了走淘汰/降级并如实上报，禁止静默涨内存 | 峰值超 `mem_cap_kb` → FAIL；超限无上报 → FAIL |
| GC 可控 | 大缓冲复用不断、短命小对象不进热循环；GC 停顿与堆大小进 JSON | `gc_pauses_ms_p99` 超预算 → FAIL；堆持续爬升（slope 失控）→ FAIL |
| 解码耗时 | 1080p 稳播是硬线，p95 进门禁；热点必须有 profile 数据，不拍脑袋优化 | `decode_ms_p95` 超 README 预算 → FAIL；热点无数据就宣称优化 → 打回 |
| CPU 预算 | 长跑 CPU 有封顶，解码线程占比可解释，禁止双 0 装绿、禁止降画质换 CPU | 超预算 → FAIL；降分辨率/降画质偷过 → FAIL（见 §8） |
| 长跑证据 | VR7 60s + VC2 120s 双证据，CPU/内存/分配/GC 四曲线 HUD 可见，基线可回归 | 任一曲线失控 → FAIL；无基线谈更顺 → 打回 |
| 跨窗连带 | 优化不准破坏正确性：每次极限优化必须重跑 VR2/VR3 正确性门禁（差异率/色偏） | 正确性回退 → FAIL，优化回滚 |

```text
热循环 = 每帧必走的代码；冷路径 = 打开/跳进度/报错/切换分辨率，允许分配但须收敛
池语义 = 借（acquire）→ 用 → 还（release），禁止还两次、禁止用后不还
封顶语义 = 上限是配置不是建议，超限行为（淘汰谁/丢哪帧/报什么）必须写进 README
```

### 2.8 清晰度矩阵（常用档全要能解析能播 · 硬）

> 每一档都要过“解析出宽高 + 解出正确画面 + 能播”，档位越高性能预算越松，但正确性一视同仁。竖屏按短边归档（例如 720×1280 算 720p 档）。

| 档 | 典型尺寸（横屏，竖屏短边同档） | 本期要求 | 性能口径 |
|----|-------------------------------|----------|----------|
| 480p | 854×480 | 解析 + 解码正确 + 能播 | 必须流畅，`decode_ms_p95` 最严 |
| 720p | 1280×720 | 解析 + 解码正确 + 能播 | 必须流畅，预算次严 |
| 1080p | 1920×1080 | 解析 + 解码正确 + 能播（VR7 主线） | 稳播硬线，按 §2.7 门禁 |
| 1440p | 2560×1440 | 解析 + 解码正确 + 能播 | 正确性同线，耗时/CPU 预算按像素数放宽（README 写清倍数关系） |
| 2160p（4K） | 3840×2160 | 解析 + 解码正确 + 能播 | 正确性同线，允许跟不上时丢旧帧保时间线（`dropped_old_frames` 如实计数，不算失败）；耗时/CPU/内存封顶按档写预算 |

规则：只测 1080p 就宣称全档通过直接判 FAIL；每档至少一片真片源（见 §7）；内存封顶 `mem_cap_kb` 按档配置，4K 封顶最大但必须有顶；桥接上墙时大帧缩放走显示目标矩形，不准在解码侧偷降分辨率（见 §8）。

### 2.9 H.264 能力矩阵（全走 ffmpeg，不自研 · 硬）

> 本期编码只有 H.264 一个，解析和处理全走 `video/ffmpeg`（demux + 解码 + sws 转 RGBA 全用 ffmpeg，见 V-U3/V-U4）。下表每一行都要有真窗或 ffmpeg 对等证据；凡是真片子里会出现的，以 ffmpeg 输出为真值对齐；罕见制式按 ffmpeg 同行为准报人话错。档位缩写：B=基础档、M=主档、H=高档。

| # | 功能点 | 说人话 | 本期要求 | 验收落哪 |
|---|--------|--------|----------|----------|
| F1 | 档位 B/M/H（8 位 4:2:0） | 手机、相机、软件压出来的片基本都是这三个档 | 全做，解对 | VR1（认出档位）+ VR2（各档至少一片解对） |
| F2 | 等级 1–5.2（含高清高码率） | 等级是片子的上限声明，超了说明播放器带不动 | 全认；超限报人话错，不断言崩 | VR1（读等级）+ VR6（超限错） |
| F3 | I 帧 / P 帧 / B 帧（含连续 B 帧与重排序） | B 帧要等后面的帧到了才能解，显示顺序和解码顺序不一样 | 全做；重排序正确，时间戳对得上 | VR2（B 帧片）+ VR4（播出来不跳帧） |
| F4 | IDR 瞬时刷新 + 随机接入 | 切片、跳进度、断流重进都靠 IDR 重新开始 | 全做；IDR 后参考帧清空，跳进度落 IDR 正确 | VR2 + VR5 |
| F5 | 熵编码两套（简单那套 + 高压缩那套） | B/M 档常用简单那套，H 档常用高压缩那套 | 两套全做 | VR2（两套各至少一片） |
| F6 | 帧内预测全部块尺寸 | 同一帧里自己猜自己，分大中小三种块 | 全做 | VR2 |
| F7 | 帧间分区从大到小（最小到小块） | 前后帧找相似块，越小越精细也越贵 | 全做到最小块；分区错了花屏，所以花屏回归必备 | VR2 + VR6 |
| F8 | 四分之一像素运动精度 + 多参考帧 + 加权预测 | 淡入淡出、遮挡、镜头晃动全靠这三件 | 全做 | VR2（含淡入淡出片） |
| F9 | 整数变换两套块尺寸 + 量化 + 缩放列表 | 压缩的核心数学，H 档多一套大块 | 全做；缩放列表缺失按默认，错了色块分层 | VR2 + VR3 |
| F10 | 去方块滤波（含开关与边界强度） | 去掉块状马赛克，关了也能看但有块 | 全做；开关两种都验，边界强度错了边缘发虚 | VR2 |
| F11 | 4:2:0 全范围/有限范围 + 宽高比 + 裁剪窗口 | 有的片黑更黑、有的带黑边裁掉、有的像素不是正方形 | 全做；转色和显示尺寸都对 | VR3 + VR0（宽高比/裁剪） |
| F12 | 隔行（场编码两套模式） | 老电视信号和部分采集卡的片，一帧分两场 | 解析认出 + 解对；确属罕见制式才允许报不支持且必须人话 | VR2（含隔行片）/ VR6（不支持分支） |
| F13 | 补充信息（含显示时序、缓冲周期、用户数据） | 片子附带的小纸条，管显示时序和码率控制 | 时序类全做（显示顺序/延时对）；用户数据透传不丢；不认识的纸条跳过不崩 | VR2 + VR4 + VR6 |
| F14 | 显示/传输参数（含时序、宽高比、码率平滑） | 片头说明书里的播放参数 | 全认并生效（时序进时钟，宽高比进显示） | VR0 + VR4 |
| F15 | MP4 里的 H.264 存放两套写法 + 参数集带内带外 + 合成时间偏移 | 同一编码在 MP4 里有两种打包法，B 帧显示顺序靠偏移表 | 全做；两种打包法都播对，B 帧不提前不延后 | VR0 + VR2 + VR4 |
| F16 | 编辑列表与旋转矩阵 | 有的片头尾留白要跳过、手机竖拍要旋转 | 全做；时长与方向对 | VR0 + VR8（竖拍片方向对） |
| F17 | 容错工具（条带组、任意顺序、冗余片、数据分区） | 标准里的抗丢包件，现在真片极少见 | 检测到必须报人话错（不乱解、不崩）；出现即进 VR6 用例 | VR6 |
| F18 | 特殊片（跳过片、空帧） | 极小占位的占位片 | 全做，时钟不漂 | VR2 + VR4 |
| F19 | 流尾（序列结束/流结束、填充字节） | 片尾的结束标记和对齐填充 | 全认，播完有结束状态 | VR4 |
| F20 | 错误隐藏（参考帧丢了怎么糊过去） | 网盘坏片、截断片总有几帧是坏的 | 坏帧隔离 + 报错 + 继续播，不整片花、不崩 | VR6 |

规则：本表“本期要求”里的“全做/全认”一律指走 ffmpeg 对齐（ffmpeg 能解我们就能播），不自研解码器。F1–F16、F18–F20 是真片高频区，以 ffmpeg 输出为真值对齐，报不支持算 FAIL；F12 隔行与 F17 老容错件如确属罕见制式才允许报不支持，但必须同时满足三条：① 报出是哪一个工具、② 不崩不卡、③ 进 VR6 用例留档，且与 ffmpeg 同行为一致。只测无 B 帧片就宣称 H.264 全功能直接判 FAIL。

### 2.5 真窗几何与运行时长

| 项 | 值 |
|----|-----|
| 客户区大小 | 1200×800 逻辑像素，全部 `video_vr_*` / `video_vc_*` |
| 最小运行 | `RUN_SECONDS ≥ 5`，`<5` → FAIL |
| 关闭用时长 | 见 §2 主表关闭用列；组合窗取所覆盖各 VR 的最大值（见 §3） |

```bash
# 标准跑法（所有 video 真窗）
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/video_vr0_demux
RUN_SECONDS=30 go run ./examples/video_vr4_play
RUN_SECONDS=30 go run ./examples/video_vr8_embed
RUN_SECONDS=15 go run ./examples/video_vr9_registry
RUN_SECONDS=60 go run ./examples/video_vr7_perf
```

### 2.6 真窗复杂度基线（防拿色块蒙混）

| 项 | 要求 |
|----|------|
| 信息面板 | 文件信息 + 解码状态 + 帧计数，三者缺一不可 |
| LiveHUD | 实时 fps / 解码耗时 / 队列水位 / 内存四项可见 |
| Legend | ≥5 行，说明画面里每块东西是啥、预期是啥样 |
| Extra | 实现策略、边界、失败条件六维写清（正确/脏/缓存/边界/失败/可见） |
| JSON | 经统一上报函数输出 §2.2 全族，不手拼字段 |
| 像素证据 | 按 `UI_PIXEL_ASSERTION_STANDARD.md` 做画面断言 + Golden 对比（见 §7） |

---

## 3. 组合真窗（集成 · 不代替 §2）

> 组合窗同样 1200×800、`RUN_SECONDS≥5`；关闭用时长取所覆盖各 VR 关闭用最大值。只做集成回归，不能用组合窗代替单能力窗关闭 VR。

| 组合 ID | 覆盖的主能力 | 真窗包名 | 关闭用 RUN_SECONDS | 要证明的集成效果 | 状态 |
|---------|--------------|----------|--------------------|------------------|------|
| VC0 | VR0+VR1+VR2+VR3+VR4 | `video_vc0_fullplay` | 30 | 从打开文件到播出画面全链路，一片播完不黑不花 | 🟩真窗绿 2026-09-13（720p五站全过/解码5显示5零丢播完/首尾MAD非黑/直播151/上屏1741/fps57.8，见§6 VC0行） |
| VC1 | VR4+VR5+VR6 | `video_vc1_seek_fault` | 30 | 边播边跳 + 坏文件混入，跳后恢复、坏文件报错不影响好文件 | 🟩真窗绿 2026-09-13（跳3/3+坏例5/5/直播166跳17恢复≤15毫秒/上屏1793/fps59.4，见§6 VC1行） |
| VC2 | VR4+VR7 | `video_vc2_soak` | 120 | 循环播 120s，CPU/内存平稳，无泄漏无崩 | 🟩真窗绿 2026-09-13（五站全过/直播608/分配89B/帧/池99.8%/零泄漏/峰值413MB<512MB/斜率22049KB/分<30000/GCp99 0.3毫秒/CPU29.8%/卡顿2.0/分/上屏6856/fps57.1/p95 20.0毫秒，见§6 VC2行） |
| VC3 | VR8+VR9+VR4 | `video_vc3_embed_extend` | 30 | 视频嵌在 UI 真窗里播 + 全走注册表 + 不支持格式可读报错，三者同屏 | 🟩真窗绿 2026-09-13（三合一1/注册8/8/内嵌1/直播151/改尺寸走完/尾MAD124/上屏1723/fps57.0/p95 19.6毫秒，见§6 VC3行） |

---

## 4. 架构与落点（独立根模块 · 不进渲染层）

```text
gpui/video（本立项的新根模块：拆盒/解码/颜色/队列/时钟/API/注册表/数据源）
  │ 只出自有帧（宽高 + 像素 + 时间戳），不 import ui/render/gpu，
  │ 不出 GPU 句柄/显存指针；对外接口按组件消费设计（见 §4.5）
  ▼（消费侧一律走 render：video.Frame → render.ImageBuf → DrawImage → Present）
ui/kit video_*（视频播放组件，包 video 能力；画画只调 prim/render 门面，不碰 gpu 包）
ui（以后要播视频的控件都走 kit 视频组件）→ render → gpu（老链路不动，GPU 上传归 render 所有）
```

流式生产级（2026-09-14）：`video/io_source.go` 数据源抽象（本地文件/内存/http(s) Range分块缓存，Range优先、禁Range回退全量，file://清洗）+ `video/player_open.go` + `video/player_playback.go` + `video/player_seek.go` 双路径（小片≤64帧且预估≤256MB全量缓冲保小片逐位一致，大片只解头（重排延迟+1帧）快开、后台按采样流边解边播、VUI重排延迟出显示序、有界队列永不涨到片长、搜进度世代号作废在途帧后落键前解、循环到尾重绕重解戳递增、URL全链路Range不断流）。秒开证据：720p约330毫秒/1080p约740毫秒（含全量小片5帧解码，大片只付头帧钱）；网络：httptest Range单测锁（206+两路GET）。

真窗上墙路径（VR4/VR8/VC0/VC2/VC3 走这条）：

```text
video.Player 取帧 → 转 RGBA 字节 → 消费侧（真窗/kit 组件）包成 render.ImageBuf
  → render 层 DrawImage / GPU 纹理上传 → Present（上传归 render，消费侧不直调 gpu）
```

**禁止：** `video` import `ui` / `render` / `gpu`；`video` 出 GPU 句柄/显存指针/显卡纹理；消费侧（`examples` 真窗、`ui/kit` 组件）直调 `gpu` 包传视频帧；`ui` 直调解码内部；解码模块直调 `gpu`；示例层绕过模块自己拼解码；`render` 反向依赖 `video`（渲染层不认视频，桥接只活在消费侧）。

### 4.1 包结构（计划）

```text
video/                        # 根模块对外 API（package video 不变）：打开/取帧/跳进度/关闭 + 播放器编排
video/player_types.go         # 哨兵错误 + Info/Stats/Options + Player 结构体
video/player_open.go          # 打开/关闭（OpenFile/OpenWithSource/Close）
video/player_playback.go      # 播放（Poll/暂停/速率/Stats/池借还）
video/player_seek.go          # 跳进度（SeekTo/SeekFast/SeekBy/关键帧/单步）
video/backend_ffmpeg.go       # ffmpeg 后端接线（后台单线程拥有全部 ffmpeg 调用）
video/registry_probe.go       # 能力查询 + 壳/编码探测
video/fault_classify.go       # 坏输入分类（Classify）
video/mem_pool.go             # 定长池 + 内存封顶
video/io_source.go            # 数据源（本地文件/内存/HTTP Range）
video/sync_audio.go           # 音画同步数学 + Player 声音接线（AudioQueue/PollAudio/ProbeAudio，主钟声领）
video/ffmpeg/                 # purego 绑定自建单文件 libgpui_ffmpeg（13 功能模块全导出，上层直接调结构体方法）
#  format_demux.go  拆盒/IO（FormatContext/Stream/IOContext）   codec_encode.go  编解码/码流滤镜（CodecContext/Codec/Parser/BitStreamFilter）
#  packet.go 包（Packet）   frame.go 帧（Frame）   scale_color.go 转色（Scaler/Image）   resample_audio.go 音频重采样（Resampler/AudioFifo）
#  filter_graph.go 滤镜图（FilterGraph/FilterContext/Filter/Sink/Source）   device_io.go 设备（DeviceList）
#  dict_opt.go 字典选项（Dictionary/OptObject）   buffer_mem.go 内存缓冲（Mem/Buffer/Fifo/BPrint）
#  error_log.go 报错日志版本（Library/Log/Math/Clock/Cpu）   media_desc.go 媒体描述（MediaDesc）
#  crypto_hash_misc.go 杂项（HWDevice/Crypto/Muxer/Prober/Samples/Util）
#  decode.go 高层解码器（Open/Next/SeekTo/Close，RGBA）   audio_decode.go 高层声音（OpenAudio/Next/SeekTo/Close，48kHz 立体声 float）
video/clock/                  # 帧队列 + 时间戳时钟 + 播/停/跳进度调度（与容器/解码解耦，只认接口）
video/testdata/               # 小测试片源与生成脚本（大文件不进仓库，见 §7）
ui/kit/video_*.go            # 视频播放组件（扁平，不建子包，见 §4.5）：video_props.go（片源/播控/字幕开关）/video_state.go（播/停/缓冲/错状态机）/video_render.go（画幅矩形与浮层排版计算）/video_widget.go（BuildVideo 唯一入口 + Ctx 订阅）/video_theme.go（尺寸与色板）；画画只调 prim/render 门面，不碰 gpu 包
examples/video_vr*/           # 单能力真窗（§2，内含 clock.Frame → render.ImageBuf 桥接）
examples/video_vc*/           # 组合真窗（§3，同上桥接）
```

### 4.2 对照 ffmpeg 的看图（只学思路）

| ffmpeg 那块 | 对应我们哪块 | 第一阶段看什么 |
|-------------|--------------|----------------|
| 拆盒 + 解码 + 转色 | `video/ffmpeg`（demux/解码/sws 转 RGBA，原生；`video/backend_ffmpeg.go` 接线） | 以 ffmpeg 输出为真值：盒子嵌套、目录表、关键帧位置、时间戳换算；解码各档输出；sws 转 RGBA 的精度、灰阶肤色不断层 |
| 能力问询 + 坏输入分类 | `video/registry_probe.go` + `video/fault_classify.go` | 问完再开，坏片说人话 |
| 播放器编排 | `video/player_*.go` + `video/clock` | 按时间戳显示、跟不上丢旧帧、跳进度落关键帧 |
| 播放调度思想 | `video/clock` | 按时间戳显示、跟不上丢旧帧、跳进度落关键帧 |

### 4.3 ffmpeg 库信息（自建单文件 · 进 gpui 仓库）

| 项 | 值 |
|---|-----|
| 库是啥 | 自建单文件 `libgpui_ffmpeg`（ffmpeg 7.1.5，LGPL 2.1），`video/ffmpeg` 经 purego 绑定后直解（demux/解码/sws 转 RGBA） |
| 从哪编 | ffmpeg 7.1 官方源码 + 双版本配方（`tools/ffmpeg-recipe` 唯一真源：基础版只管看片 + 高级版加写文件，两档都不碰 GPL，LGPL 2.1 不变），本机 docker 与 Actions 同一套脚本（`tools/ffmpeg/` + `.github/workflows/ffmpeg-dual.yml`） |
| 放哪了 | `gpui/lib/ffmpeg/<平台>/`（linux-x64/arm64/386/arm + win-x64/arm64 共 6 构建进 `git`；mac `.dylib` 待补）；对照源码仍在上级 `gogpu/ffmpeg` 只读；双版本：基础版 `libgpui_ffmpeg.(so|dll)` 默认加载（只管看片），高级版 `libgpui_ffmpeg_full.(so|dll)` 显式指定才加载（`GPUI_FFMPEG_VARIANT=full`，加写文件：复用+编码+烧字）；配方唯一真源 `tools/ffmpeg-recipe`，构建脚本 `tools/ffmpeg/`（本机 docker 与 Actions 同一套），workflow `.github/workflows/ffmpeg-dual.yml` |
| 许可 | LGPL 2.1（自建库 LGPL 合规；禁止把 GPL 代码拷进 `gpui`，只链二进制 + 自己写的 purego 绑定） |

第一阶段要翻的具体位置：

| ffmpeg 位置 | 看啥 | 对应我们哪个 VR |
|-------------|------|-----------------|
| `libavformat/mov.c` 附近 + `libavformat/isom.h` | MP4 盒子嵌套、目录表、关键帧位置、时间戳换算 | VR0 |
| `doc/demuxers.texi` 里 mp4/mov 章节 | 盒子行为的文字说明，代码看不懂时先看这里 | VR0 |
| `libavcodec/h264*`（参数集、帧切分、熵解码、预测、反变换、滤波） | H.264 主流程与边界处理 | VR1、VR2 |
| `libavcodec/h264*` 里的补充信息/显示参数/滤波各文件（按功能名找，不背文件名） | §2.9 F10/F13/F14 的对照实现 | VR2 |
| `libavformat/mov.c` 里的 avcC/合成偏移/编辑列表/旋转分支 | §2.9 F15/F16 的盒子侧对照 | VR0 |
| `libavcodec/get_bits.h`、`golomb.h` 附近 | 比特流读法与指数哥伦布解码，只定位问题，不重写读位器 | VR1、VR2 |
| `libswscale/`（`yuv2rgb.c`、`input.c`、`output.c`）+ `doc/libswscale.texi` | YUV 转 RGBA 公式与精度 | VR3 |
| `libavutil/`（`mem`、`log`、`error`、`rational` 思想） | 内存、报错、分数时间戳思想，只学不搬 | VR6、clock |
| `doc/examples/demux_decode.c`、`decode_video.c` | 从拆包到解码的调用顺序，帮你理清管线 | VR4 |
| `tests/fate/` 里 mp4/h264 相关用例名 | 坏文件长啥样，帮你补 VR6 用例思路 | VR6 |

使用纪律：

```text
允许：看思路、看流程、看边界条件、看测试用例名，只定位问题
禁止：复制任何 .c/.h 进 gpui；按 ffmpeg 代码逐行翻译；自己设计 Go 解码结构重写解码器；
      把 gogpu/ffmpeg 加进 gpui 的 go.mod / 构建 / CI
更新：浅拷贝不会自动跟新版，需要对照新版时手动 git pull；更新后在本节同步提交号与日期
```

### 4.4 扩展性设计（加新编码不用改核心 · 硬）

> 目标：以后加 H.265 / VP9 / AV1、新盒子（MKV/WebM）、新采样格式、音频，只加新包 + 注册，不改核心流程。这是 VR9 验收的对象。

| 扩展点 | 接口思想（只认接口不认实现） | 以后加东西咋加 | 第一阶段做到啥 |
|--------|------------------------------|----------------|----------------|
| 容器（拆盒） | 探测（看头认盒子）+ 打开 + 取视频包 + 跳关键帧 + 关闭 | 新增容器走 ffmpeg 能力（ffmpeg 已支持则直接复用注册，不自研拆盒；播放器不动） | MP4 经 ffmpeg；再加一个探测失败的可读错 |
| 视频解码 | 按编码名注册；输入压缩包，输出 YUV 帧；支持刷尾、关闭 | 新增编码走 ffmpeg 能力（ffmpeg 已支持则直接复用注册，不自研解码器；核心照旧取帧） | H.264 经 ffmpeg；再加一个演示用桩解码器，证明能插拔 |
| 颜色转换 | 按输入采样格式注册转换器，转出 RGBA | 新增采样走 ffmpeg sws 能力（不自研转色；核心照旧取 RGBA） | 最常见格式先行，别的报不支持 |
| 帧与时钟 | 帧只带宽高、格式、时间戳；队列和时钟不认具体编码 | 音频以后另起队列，时钟再做对齐，视频侧不动 | 视频单队列 + 时间戳时钟 |
| 播放器编排 | 打开文件 → 选容器 → 选解码 → 解 → 转色 → 队列 → 按戳显示 | 加新格式不碰编排，只加注册 | 全链路走注册表，不写死 MP4/H.264 |
| 能力查询 | 能问出现在支持哪些盒子/编码/采样格式 | UI 层先问再开，不支持的给人话提示 | 查询接口 + 不支持时的人话错（零崩溃） |

规则：核心流程禁止写死任何一个盒子名、编码名、采样格式名；写死了 VR9 直接判 FAIL。播放器 API 第一版冻结，加新能力只加注册与新包，不 break 老接口。

### 4.5 视频播放组件设计（`ui/kit` video_* · 硬）

> 目标：`video` 是能力层（解码出帧），`ui/kit` video_* 是组件层（拿来即用的播放控件）。能力层不知道组件存在，组件层包住能力层，渲染一律走 `render`（见 §4），三层单向依赖：`ui/kit video_*` → `video`（取帧/播控）→ 消费侧经 `render` 上墙。

| 组件文件 | 放什么 | 不放什么 |
|----------|--------|----------|
| `video_props.go` | 片源（文件/内存/URL）、自动播、循环、静音、音量、变速、目标矩形模式（充满/包含/拉伸）、封面、字幕轨开关（P3 预留，见 V-U2） | 解码细节、ffmpeg 符号 |
| `video_state.go` | 播/停/缓冲中/播完/出错状态机 + 队列水位/时钟漂移透出（读 `video.Stats` 真值，不自算） | 自己另起解码线程、自己算时间戳 |
| `video_render.go` | 画幅矩形计算（缩放只走显示目标矩形，解码分辨率不动，沿 §2.1 `embed_in_ui_scene`）、控制条/浮层排版、进度条缩略图取帧位（P1 预留） |逐像素拷贝上墙（走 `render.ImageBuf` + `DrawImage`，上传归 `render`）|
| `video_widget.go` | `BuildVideo` 唯一入口 + Ctx 订阅（主题/语言/方向，照 kit 惯例）+ 事件回调（首帧/播完/出错/缓冲变化） | 绕过 `video` 直调 `video/ffmpeg` |
| `video_theme.go` | 尺寸/色板/字号默认值（kit 主题走 Ctx，不写死） | 用户文案硬编码（引擎层硬编码纪律） |

规则：① 组件只调 `video` 公开 API（打开/取帧/跳进度/播控/状态事件），`video/ffmpeg` 符号不出组件；② 组件与示例一样是消费侧，帧转 `render.ImageBuf` 后只调 `prim`/`render` 门面画画，不直引 `gpu` 包（合规见 §7）；③ 组件行为（状态机/动效/绘制）只活在 `ui/kit/video_*` 里，示例只做摆位与演示数据（组件示例边界纪律）；④ 验收另起独立 kit 真窗 `examples/kit_video`（走 kit 真窗纪律），不占用 `video_vr_*` 名额，不代替任何 VR。

---

## 5. 分期（VW 关闭 = 单窗全集 + 组合窗）

| W | 状态 | 必须绿的单能力窗 | 必须绿的组合窗 | 说明 |
|---|------|------------------|----------------|------|
| VW0 | 🟩双窗绿（VR0+VR1，无组合窗） | VR0、VR1 | — | 先能拆盒、切帧，画面还没出来也算数 |
| VW1 | 🟩双绿（VR2+VR3，无组合窗）2026-09-13 | VR2、VR3 | — | 第一帧正确解出来、颜色对 |
| VW2 | 🟩收口 2026-09-13（VR4+VR5+VR6单窗齐+VC0+VC1组合齐） | VR4、VR5、VR6 | VC0、VC1 | 能播、能跳、坏文件不崩 |
| VW3 | 🟩收口 2026-09-13（VR7+VR8+VR9单窗齐+VC2+VC3组合齐） | VR7、VR8、VR9 | VC2、VC3 | 1080p 稳播 + gpui 真窗内嵌 + 扩展注册收口 |

```text
VW0 → VW1 → VW2 → VW3
```

**熔断：** 无单能力真窗标完成；用组合窗代替单窗关闭 VR；降分辨率偷过性能门禁；坏文件崩溃当小事跳过；热循环分配超标、无界队列、超封顶静默涨内存（§2.7 任一条即熔断）；只测无 B 帧片宣称 H.264 全功能、高频功能报不支持（§2.9 即熔断）。

---

## 6. 需求细节（每个 VR 做到啥算完）

- **VR0 拆盒：** 能读常见手机/相机导出的 MP4；§2.8 每档给出视频轨宽高（横竖屏都认）、帧率、时长；给出关键帧目录；盒子坏了报人话错（缺头、截断、找不到视频轨分开报）。
- **VR1 参数与切分：** 参数集丢了、错了能检出；一帧的边界切不错；B/M/H 三档与等级 1–5.2 全认（§2.9 F1/F2），MP4 两种打包法与带内带外参数集都认（F15）；高档特性明确报不支持而不是乱解的口子只留给 F17 老容错件，且必须人话报错。
- **VR2 解码：** §2.8 五档片全走 ffmpeg 解出连续正确画面；§2.9 F3–F15/F18–F20 以 ffmpeg 输出为真值对齐（B 帧重排序、IDR、两套熵编码、全尺寸预测、最小分区、四分之一精度/多参考/加权、两套变换、缩放列表、滤波开关、范围/宽高比/裁剪、时序信息、编辑列表/旋转、跳过片、流尾）。体量大按 §6.1 四段验收，段段有 ffmpeg 对等证据，VR2 主格等四段全绿加真窗才翻。

### 6.1 VR2 分期（段段有专有测试与状态格 · 硬）

> 每段翻绿标准 = 引擎落点代码 + 专有测试全绿（含真值来源） + 状态格写日期与证据。VR2 主格（§2）与 VW1（§5）等四段全绿加 `video_vr2_decode` 真窗（RUN 15）才翻，任何一段没绿都不许宣称解码完成。

| 段 | 范围 | 引擎落点 | 专有测试（文件+真值+门禁） | 状态 |
|----|------|----------|---------------------------|------|
| VR2a I帧帧内 | F4（IDR部分）、F5简单熵编码、F6帧内预测、F9 4x4变换与默认缩放、F10滤波开关 | `video/ffmpeg` 全走 ffmpeg | ffmpeg 对等：I-only 片与 ffmpeg 输出逐字节一致 | ✅引擎绿（VR2a-1✅、VR2a-2✅，I-only 片与 ffmpeg 输出逐字节一致（2026-09-13）） |
| VR2b P帧帧间 | F3（I/P部分）、F4参考清空与随机接入、F7帧间分区、F8四分之一精度与单参考、F18跳过片、参考帧管理 | `video/ffmpeg` 全走 ffmpeg | ffmpeg 对等：混合片与静止片各 5 帧逐字节一致 | ✅引擎绿（混合片与静止片各 5 帧与 ffmpeg 输出逐字节一致（2026-09-12）） |
| VR2c 高档 | F1（主档与高档各至少一片解对）、F5高压缩熵编码、F6 8x8 帧内余量、F9 8x8 变换与缩放列表 | `video/ffmpeg` 全走 ffmpeg | ffmpeg 对等：三高档片各 5 帧逐字节一致 | ✅引擎绿（三高档片各 5 帧与 ffmpeg 输出逐字节一致（2026-09-13）） |
| VR2d B帧收尾 | F3（B 帧与重排序，显示序对表）、F8多参考与加权、F11解码侧标记透出（含宽高比与全/有限范围）、F12隔行、F13时序类小纸条、F14显示时序、§2.8 五档矩阵、真窗 `video_vr2_decode`（RUN 15） | `video/ffmpeg` 全走 ffmpeg；真窗桥接走 `render` | ffmpeg 对等：B 帧片解码序与显示序双断言 + 五档片逐字节一致 + 真窗绿 | ✅引擎绿（B 帧片与五档片与 ffmpeg 输出逐字节一致（2026-09-13），真窗 `video_vr2_decode` RUN15 绿：15 帧差异 0/上屏 880） |

#### 6.1.1 VR2a（全走 ffmpeg）

#### 6.1.2 VR2b（全走 ffmpeg）

#### 6.1.3 VR2c（全走 ffmpeg）

#### 6.1.4 VR2d（全走 ffmpeg）

> VR2d 明细现全走 `video/ffmpeg`，以 ffmpeg 输出为真值。

- **VR6 容错**✅**：** 落地根 `video` 的 `Classify` 归口（坏盒/缺参数/截断/超限等级/档位/颜色/坏片分桶，人话+层+工具，`errors.Is` 可判）与播放器隔离（坏帧跳过+解码器重置+尾继续播，截断样本可读错，开门拦截，好片零误伤）；门禁手表11项绿；真窗 `video_vr6_fault`（RUN15）绿：12例全过/花屏隔离2帧剩8帧/直播76/上屏870/fps57.3/p95 18.7毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg`绿、vet过）。
- **VR3 颜色**✅**：** 走 ffmpeg sws 转 RGBA（有限+全范围，601/709按片内信号切换；热路径零分配由 ffmpeg 侧保证，我方只做队列/时钟编排）；门禁11组向量（黑白灰三原色肤色灰阶）逐字节零差异，独立浮点公式交叉最大差1；真片交叉（I帧首帧 vs ffmpeg 4.4默认swscale：9216像素R/B≤2、G≤3、均值0.47，真窗容差定3并写进README）；真窗 `video_vr3_color`（RUN5）绿：11组色差0/真I帧1帧/上屏295/§2.2全族51键。状态——已解决 2026-09-13，回归（`video/ffmpeg`绿、全仓构建与vet过）。
- **VR4 播放**✅**：** 落地 `video/clock`（有界队列定长阻塞背压+暂停时钟+追帧计数）与根 `video` 播放器（容器戳排序+后台供帧+按戳显示+暂停真停+末帧同拍结束+循环戳递增+坏片人话错）；门禁手表6项绿（顺序/暂停/追帧/循环/坏片/合规，循环让调度用 `Gosched` 保稳）；真窗 `video_vr4_play`（RUN30）绿：三档解码15/显示215/零丢/直播125/上屏1740、fps57.5/p95 18.8毫秒/§2.2全族51键。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg` 全绿、全仓构建与vet过；注：`CGO_ENABLED=0 go run` 下别线 `render/internal/gpu` 有未使用变量致构建失败，默认CGO跑窗不受影响，非本线未动）。
- **VR5 跳进度**✅**：** 落地根 `video` 的 `SeekTo`（经 ffmpeg 原生 seek 到目标；队列排空重填+时钟重锚+暂停保持，非循环窗；循环+跳留给 VC1）与 `clock.Queue.Clear`；门禁手表7项绿（双IDR第二GOP前解4/第一GOP前解3/跨戳差100/单键片/20连跳/循环拒绝/暂停跳，落点预算500毫秒）；新小片 `video/testdata/vr5_seek.mp4`（96x96/10帧/双IDR，前解4远小于全片10）；真窗 `video_vr5_seek`（RUN15）绿：四跳全绿/差0/恢复≤14毫秒/直播84帧10跳（含播完绕回走跳）/上屏877/fps57.9/p95 17.8毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg` 全绿、全仓vet过）。
- **VR6 容错：** 坏盒子、花屏流、截断尾、缺参数，每个都有用例；全部做到报错不崩，错误信息能定位到哪一层。
- **VC1 边播边跳+坏文件**✅**：** 新建组合真窗 `examples/video_vc1_seek_fault`（1200×800，RUN30）：左上三跳（好片双IDR跳第二GOP/第一GOP/跨戳，落点预算500毫秒+跳后即时恢复），左下五坏例（缺文件/非MP4/截断尾/坏帧隔离，人话+层+工具，好片不受影响），右边好片直播（10秒跳1800/20秒跳600/播完绕回走跳，每跳下一拍即恢复）；动画60档判。RUN30实跑绿：跳3/3+坏例5/5/直播166跳17恢复≤15毫秒/上屏1793/fps59.4/p95 17.5毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg`绿、vet过）。只做集成回归，不代替单能力窗。注：首跑曾被别线未跟踪 `ui/overlay` 坏文件卡编译（`Rect.X` 系旧字段名），与本线无关未动，重跑即过，备查。
- **VC0 全链路**✅**：** 新建组合真窗 `examples/video_vc0_fullplay`（1200×800，RUN30）：同一片 720p 出五站数——VR0 拆盒（1280x720/5fps/5采样/关键帧≥1/时长>0）、VR1 参数（Main/等级/切5帧/IDR≥1）、VR2 解码（5帧+显示序递增，以 ffmpeg 输出为真值）、VR3 转色（首尾帧非黑≥2，走 ffmpeg sws）、VR4 播放（解码=显示=采样5/零丢/播到 Ended），任一站红整窗红；右边同片循环直播 30 秒不黑不花；动画 60 档判。RUN30 实跑绿：五站全过/直播151/上屏1741/fps57.8/p95 18.7毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg`绿、vet过）。只做集成回归，不代替单能力窗。
- **VR7 性能极限**✅**：** 落地 `video/mem_pool.go` 帧池（借还/泄漏/封顶可查）加播放器后台供帧 + 真窗 `examples/video_vr7_perf`（RUN60）：左五站（拆盒1920x1080/参数/冷解码5帧递增/首尾非黑播完零丢/预估<512MB封顶+池预热）+右同片1080p循环直播+底部四曲线HUD；上墙走 `render/ImageBuf` + `DrawImage`，显示缩到480x270走显示目标矩形（解码仍全1080p，§2.8显示侧缩放）；门禁手表（稳态≤2048B/帧/Poll零分配/池命中≥90%/泄漏0/峰值≤512MB/斜率≤30000KB/分/GCp99≤10毫秒/CPU≤85%/卡顿≤50/分/fps≥55+p95≤22毫秒/冷解码p95≤50毫秒/首帧≤2000毫秒）全绿；RUN60实跑绿：解码5/显示309/分配89B/帧/池99.7%/泄漏0/峰值423MB/斜率负/GCp99 0.2毫秒/CPU28.6%/卡顿0/上屏3438/fps57.1/p95 21.5毫秒/冷解码p95 28.6毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`池+稳态+容错+播放+跳进度绿/`clock`/`video/ffmpeg`绿、CGO_ENABLED=0构建+vet过、VR2/VR3正确性重跑零回退）。只做极限验收，不代替任何正确性窗。
- **VR8 真窗内嵌**✅**：** 新建组合真窗 `examples/video_vr8_embed`（1200×800，RUN30）：同片720p冷解码到Ended（解码=显示=采样5/零丢/递增/首尾MAD非黑）+右同片循环直播（主480x270+同源小窗240x135共享帧，缩放只走显示目标矩形，解码仍全720p；主窗包圆角裁剪+整组透明，半透明浮层盖一角，面板文字同屏；10秒缩360x202、20秒还原，改后直播继续涨才算改尺寸过）；动画60档判。RUN30实跑绿：冷解码5/直播151/内嵌1/缩放裁剪透明浮层改尺寸全1/尾MAD124/上屏1737/fps57.4/p95 19.8毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`池+稳态+容错绿/`video/ffmpeg` 绿、vet+CGO构建过）。只做内嵌验收，不代替任何解码/播放窗；`video`本体不碰`render`/`gpu`，桥接走 `render`（见§4）。
- **VR9 扩展**✅**：** 落地 `video/registry_probe.go`（容器探测+解码注册+能力查询，容器/编码各注册一次，播放器全走注册表不再写盒子/编码/采样名字，`TestNoHardcodedNames` 熔断锁，`Classify` 补注册表桶）加 `registry_probe_test.go`（能力/同片播完/灰桩可插拔/三坏例/无写死6项）加真窗 `examples/video_vr9_registry`（RUN15）：探测mp4/h264+能力表齐+同片走注册表播完+窗侧灰桩（96x96灰像素）+坏盒/坏编码/坏采样三人话错零崩溃（8/8）+右同片循环直播；动画60档判。RUN15实跑绿：注册8/8/播出5/直播76/上屏866/fps57.4/p95 18.5毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video`/`clock`/`video/ffmpeg` 全绿、VR4窗RUN15绿、VR5窗RUN15绿、vet+CGO构建过）。只做扩展验收，不代替任何解码/播放窗。
- **VC2 长跑**✅**：** 新建组合真窗 `examples/video_vc2_soak`（1200×800，RUN120，VR7同构）：左五站（拆盒1920x1080/参数/冷解码5帧递增/首尾非黑播完零丢/预估<512MB封顶+池预热）+右同片1080p循环直播120秒+底部四曲线HUD；上墙走 `render/ImageBuf` + `DrawImage`，显示缩到480x270（解码仍全1080p）；门禁沿VR7手表但卡顿走§2.2.2长跑默认线（≤5/分钟，不沿用VR7首跑宽松值）。RUN120实跑绿：五站全过/直播608/长跑播稳1/分配89B/帧/池99.8%/零泄漏/峰值413MB<512MB/斜率22049KB/分<30000/GCp99 0.3毫秒/CPU29.8%/卡顿2.0/分/上屏6856/fps57.1/p95 20.0毫秒/冷解码p95 22.6毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video/ffmpeg` 绿、vet+CGO构建过、VR2/VR3零回退）。只做长跑集成回归，不代替任何单能力窗。注：斜率22049含UI渲染/HUD的120秒堆涨，另按 `video` 稳态口径分开算。
- **VC3 三合一**✅**：** 新建组合真窗 `examples/video_vc3_embed_extend`（1200×800，RUN30，引擎零改动）：左注册表8例（能力探测+同片播完+窗侧灰桩96x96+坏盒/坏编码/坏采样三人话错零崩溃）+左内嵌冷门禁（同片720p冷解码到Ended，解码=显示=采样5/零丢/递增/首尾MAD非黑）+右同片循环直播（主480x270+同源小窗240x135共享帧，缩放只走显示目标矩形；主窗包圆角裁剪+整组透明，半透明浮层盖一角，面板文字同屏；10秒缩360x202、20秒还原，改后直播继续涨）；动画60档判。RUN30实跑绿：三合一1/注册8/8/内嵌1/缩放裁剪透明浮层改尺寸全1/直播151/尾MAD124/上屏1723/fps57.0/p95 19.6毫秒/§2.2全族齐。状态——已解决 2026-09-13，回归（`video/ffmpeg`/`video`注册表绿、VR2/VR3零回退、vet+CGO构建过）。只做组合集成回归，不代替任何单能力窗；`video`本体不碰`render`/`gpu`，桥接走 `render`（见§4）。

---

## 7. 测试计划

| 层 | 内容 | 要求 |
|----|------|------|
| 单测 | ffmpeg 解析处理输出对齐（demux/解码/sws/audio）+ 队列时钟/播放编排/容错分类 | `video` 根 + `video/ffmpeg` + `video/clock` 有单测，坏输入用例必有，regression 进 CI |
| 基准 | 每个热路径有 `Benchmark`（多档规模 + `-benchmem` 分配断言） | 分配次数/每操作字节超标 → FAIL；优化前后比值回归进 CI |
| 画像 | 热点期 `pprof`（CPU/堆/分配）留档关键结论 | 无数据宣称优化 → 打回；画像证明热点与优化手段对得上 |
| 真窗 | §2 每个 VR 独立窗 + §3 组合窗 | 1200×800、关闭用时长、§2.2 全族 JSON、`FAIL:` + `exit 1`、README 可见效果 |
| 画面证据 | 像素断言 + Golden | 按 `UI_PIXEL_ASSERTION_STANDARD.md` 做，逻辑指标绿不代表画面对 |
| 片源管理 | `video/testdata/` 只放小文件，§2.8 每档至少一片 + §2.9 每行至少一例 | 大片、长片不进仓库；放生成脚本（用小图连成测试流/短片，每档都生成；工具覆盖用合成码流 + 标准一致性向量思路），CI 现场生成；4K 长跑片本地手工验 |
| 合规 | 无 CGO（`import "C"` 空）、`CGO_ENABLED=0 go build ./...` 过、根模块零反向依赖、消费侧视频帧只走 `render` | `CGO_ENABLED=0 go build ./...` 过；`grep import "C"` 空；`video` 禁止 import `ui`/`render`/`gpu`（单测锁，参照 `TestNoGPUImport` 做法）；消费侧（`examples` 真窗、`ui/kit` video_*）视频帧禁止直调 `gpu` 包，一律 `render.ImageBuf` + `DrawImage`（kit 组件画画只调 prim/render 门面）；`go vet` 过；第三方只认两项：Go 侧 `purego` 绑定 + `lib/ffmpeg` 下自建 `libgpui_ffmpeg` 二进制（LGPL 2.1） |
| 文档 | 公开 API 有目录账 | 新增/改动 `video` 公开 API 时，同步本文件 §10 修订行 |

---

## 8. 宣称纪律

```text
允许：某 VR/VC 在对应真窗 + §2.2 全族指标证据下描述已达标
禁止：无单能力真窗关闭 VR；省略 CPU/内存/帧时只报业务字段；
      用小分辨率冒充 1080p 达标；降画质通过性能门禁；无基线谈更顺；
      热循环每帧新开大内存；无界队列堆帧；超内存封顶静默上涨；
      先跑通以后再优化（§2.7 不接受分期拖欠）
```

---

## 9. 开放问题

| ID | 问题 | 倾向 |
|----|------|------|
| Q1 | 首批测试片定哪几个（手机/相机/软件导出各一） | 立项后先定片单再开工 VW0 |
| Q2 | H.264 先到哪个档位（基础档先行还是直接主档） | 不分先行：§2.9 定了 B/M/H 全做，VR1 认档位、VR2 各档解对（已决，关闭本问题） |
| Q3 | YUV 存法放哪（`video` 自有还是借 `render/ImageBuf`） | `video` 自有帧类型（YUV/RGBA 自带），只在消费侧转 `ImageBuf`，`video` 本体不依赖 `render`；消费侧一律走 `render`（`ui/kit` video_* 同理，见 §4/§4.5） |
| Q4 | 大片源与长跑片怎么管（不进仓库） | 生成脚本 + 本地大片手工验，不进 git |

---

## 10. 修订

| 版本 | 说明 |
|------|------|
| v0.1 立项 | 建文档：范围锁 MP4+H.264、只要画面、纯 Go 无第三方、对照 `gogpu/ffmpeg` 自研；主表 VR0–VR7、组合 VC0–VC2、分期 VW0–VW3 全 ⬜；真窗 1200×800 + 关闭用时长 + A–J 全族门禁；独立模块落 `render/video`，复用 `ImageBuf`/`DrawImage`/GPU 上传，不破 `ui → render → gpu` 与禁止 CGO。 |
| v0.2 立项 | 补 §4.3 参考库信息：下载地址与浅拷贝命令、存放 `gogpu/ffmpeg`（与 `gpui` 同级、不进仓库）、当时提交号与大小、第一阶段对照文件表、使用与更新纪律。 |
| v0.3 立项 | 加真窗播放与扩展性：§2.1 增 `embed_in_ui_scene`；新增 VR8（gpui 真窗内嵌播放）与 VR9（扩展注册机制）及 VC3；§4.1 包结构注册化、新增 §4.4 扩展点表与硬规则；VW3 收口 VR7+VR8+VR9 与 VC2+VC3；§6 补 VR8/VR9 完成线。 |
| v0.4 立项 | 改独立根模块：落点 `render/video` 改 `gpui/video`（与 `ui`/`render`/`gpu` 同级，不进渲染层）；`video` 核心禁止 import `ui`/`render`/`gpu`，自有帧类型，显示经真窗桥接转 `ImageBuf`；§4/§4.1/§4.4/§6/§7/§9 同步，合规加反向依赖单测锁。 |
| v0.5 立项 | 性能极限化：VR7 改极限优化能力；B/E 族加分配/池命中/封顶/GC 字段与 FAIL 线；新增 §2.7（热路径零分配、三池复用、有界队列、内存封顶、GC 可控、长跑双证据）；§6/§7/§8 同步（基准+画像进 CI，热循环分配与无界队列直接判 FAIL）。 |
| v0.6 立项 | 定声音后置：本期只要画面（V-U2），声音模块下一期再立项；新增 §11（范围预览、独立成篇、启动条件），§1.2 同步指向；本期只给时钟留扩展口，不做音频。 |
| v0.7 立项 | 加清晰度矩阵：新增 §2.8（480p/720p/1080p/1440p/2160p 五档全要解析能播，竖屏按短边归档；正确性同线、性能预算按档放宽）；VR0/VR1/VR2/VR4 行与 §6 同步；片源每档至少一片；内存封顶按档配置。 |
| v0.8 立项 | 加 H.264 全功能矩阵：新增 §2.9（F1–F20：三档位、等级、B 帧重排序、IDR、两套熵编码、全尺寸预测、最小分区、亚像素/多参考/加权、变换量化、滤波、范围宽高比裁剪、隔行、补充信息、显示参数、MP4 双打包/偏移、编辑旋转、老容错件、跳过片、流尾、错误隐藏）；VR1/VR2/VR6 行与 §6/§7/Q2/§4.3 同步；高频区全解对，老件三条件才许报不支持。 |








| v0.121 旧自研清零+全走ffmpeg | 纯文档（零代码改动）：规范部分（§0–§9/§11–§12，不含§10历史）删旧 Go 自研实现描述，全统一为 ffmpeg 口径——解析处理（demux/解码/sws/swr/seek/线程/转码）全走 `video/ffmpeg`，上屏全走 `render`（`render/ImageBuf` + `DrawImage`，不直调 `gpu`）；§6.1.1–6.1.4 明细清零；§12 S1/S1b/S2/S3/A1/V2 行按 ffmpeg 重写（S1/S2/S3 关🟩，A1/V2 保持🟨走 ffmpeg 通路）；§12.2 顺序17/18 同步；§11.6 口径改 ffmpeg 原生；§10 历史原样保留备查；§2/§5 VW0–VW3状态不动，其余行不动。 |
| v0.120 render直通+kit组件架构 | 纯文档（零代码改动）：V-U2 补后置去向（声音见 VW6、字幕见 P3，不算永久不做）+ V-U3/V-U5/§2.1/§4/§4.1/§7/Q3 同步 render 直通口径（`video` 只出帧不出 GPU 句柄，消费侧一律 `render.ImageBuf` + `DrawImage`，消费侧禁直调 `gpu` 包，上传归 `render` 所有）+ 新增 §4.5 kit 视频组件设计（能力层 `video` 与组件层 `ui/kit video_*` 单向依赖，五文件分工：props/state/render/widget/theme，只调 `video` 公开 API，只调 prim/render 门面画画，验收走独立 `examples/kit_video` 不占 VR 名额）+ §11.6 S4/S5 按 render 直通重写（先钉本机 Linux + Intel 核显 VA-API 一台目标机，硬解帧回传 CPU 可读帧再交 `render` 上传；纹理直传由 `render` 出接口）+ §12 新增 S4/K1 两行 + §12.2 新增顺序 26（K1 组件化，可独立并行；S1b 基线数字保留对照不删）；§2/§5 VW0–VW3状态不动，其余行不动。 |
| v0.119 慢放记录移除+ffmpeg硬解方向 | 纯文档（零代码改动）：删 §6.2 慢放四项 + §6.3 治慢放方案 + §10 v0.78–v0.81 四行 + §11.6 之 09-19 复测段 + §10 v0.95 行（慢放问题不再留记录；v0.82 S1-VR7D 计时口径与 S1b 复测数字保留，S1b 去掉已删 v0.95 的引用）；V-U3/V-U4 口径同步（播放链路全走 ffmpeg 包，解码含硬解一律走 ffmpeg hwcontext 通路，`video` 不直连显卡接口）+ §12 S4 行改按新方向（先定接口再施工）；§2/§5 VW0–VW3状态不动，其余行不动。 |
| v0.118 ffmpeg全量与Actions修好 | 全量 full 与构建链修好（只动构建本线，真窗不动）：full 补齐烧字三滤镜（drawtext/subtitles/ass）+ openh264 编码（`--enable-encoder=libopenh264` 要点名，只开库不出件）+ ass 编码，门禁 6 项全查（写盒+三滤镜+openh264+ass，缺一即 FAIL，之前只查写盒）；备料 7 件全齐（fribidi 坏包换好、libass/ openh264 用 git 重打、expat 新加给 fontconfig 用，坏包直接 exit 3）；脚本 8 处真修（pkg-config 真名 freetype2/harfbuzz/fontconfig/fribidi、configure 加 `--pkg-config-flags=--static` 否则静态 fontconfig 误判不存在、meson 交叉加 cross-file（arm64/arm/386/w64）+ `--libdir=lib` 钉死否则 .pc 找不到、openssl 交叉不递 CC 只递前缀+make 盖 CC/AR、freetype 等 autoconf 交叉递 `--host`、386 内核头软链接、openh264 装完删 .so 只留 .a）；win-arm64 拆真 ARM64（llvm-mingw 的 clang，之前复用 x86_64 前缀是错的，已拆开，zlib 另起 w64arm）；workflow 可直接跑（ffmpeg 验包+版本号、配方唯一真源、外库两步走、功能门禁按目标目录找文件、base 按文件名找非 full、许可门禁、mac 真编不再占位、EXPAT_VER 补齐、need-only 不再 `|| true` 掩错）；`§4.3`/`.gitignore`（darwin 4 格）同步；门禁（linux-x64 full 48MB 六门禁全过 + GPL 零混入 + `TestRemuxSplitWithSubtitles`/`TestVariantRouting` 绿）；9-26 追记：本机 6 目标 base 全出（/tmp/ffwork/out：386/arm/arm64/x64 新 base 解码器 111 对老仓 41，win-x64 与 win-arm64 真机格式对：x64 为 PE32+ x86-64、arm64 为 PE32+ Aarch64，经容器内对应 nm 门禁）+ full 已出 3 个（linux-x64 48MB 六门禁全过、win-x64 11MB 写盒过、win-arm64 10MB 写盒过，arm64/386/arm 的 full 外库重编中）；build-deps 再修 3 处（harfbuzz 关 tests/utilities/docs 只留静态库，之前交叉编测试二进制链宿主 x86_64 libz.so 炸；fribidi 关 tests/docs/bin 同理；fontconfig 的 LDFLAGS 指自建 zlib-$TAG，之前 fc-cache 链宿主 libz 炸；zlib 装完补对应 ranlib，之前 w64arm 的 .a 索引不对致 configure 认不出 zlib）；workflow 再修 5 处（linux-win 与 mac 加 setup-go 1.25，否则 recipe 跑不起；编单文件去掉 `RECIPE_FLAGS` 环境透传，只走挂载的 recipe 文件，之前大串易截断；win 门禁用镜像内对应 nm 查，runner 本机无 win nm；win full 门禁与 build-one.sh 同口径暂只查写盒；base 门禁由不存在的 `av_decoder_iterate` 改 `avcodec_find_decoder`，mac/base 找文件排除 *full*，删掉从未使用的 dispatch target 开关）；build-one.sh 再修 2 处（`PKG_CONFIG_LIBDIR`/DETSFX 补 `zlib-$TAG`：freetype2 的 `.pc` 写了 `Requires: zlib`，缺了 libass 传递依赖解析炸，实测 arm64/arm 挂在 libass 检测上；CFG 加 `--pkg-config=pkg-config`：交叉前缀下 configure 找 `<前缀>pkg-config` 找不到就回退 false，外库检测全灭，实测 arm64/arm 料全备好却报 `libass not found`）；9-26 本机 12 产物全齐（6 base 全换新：解码器 111 对老仓 41；6 full：linux-x64 46MB、386 45MB、arm64 47MB、arm 41MB 六门禁全过，win-x64 11MB、win-arm64 9.6MB 写盒过，GPL 全干净）；§2/§5 VW0–VW3状态不动。 |
| v0.117 ffmpeg转封装 | 转封装写文件链路（只动 `video/ffmpeg` 本线，真窗不动）：修 2 个签名真 bug（`WriteHeader` 按头文件回 `int` 错误码，之前按指针；`AvioOpen/Open2` 按头文件回 `int` + url 走 Go 字符串，之前回指针）+ 新最小访问器（`FormatContext.NbStreams/StreamAt/SetPb`、`Stream.Index/CodecPar/TimeBase`、`Packet.StreamIndex/SetStreamIndex/IsKey/DurationMs`，偏移按 7.1 头文件：`fmtPb=32/streamIndex=8/codecpar=16`、`pktFlags=40`）+ `CodecIDMovText=94213` + `AllocOutputContext2` 的 format_name 走 Go 字符串 + `AVIOFlagRead/Write` 常量；新 `remux_split_test.go`（1080p 原片切 3 段写 `t.TempDir()`：读包搬包 + 挂 `mov_text` 字幕轨 + 重开验时长±3秒/字幕轨/1920x1080 解出帧；原片 18M 没进仓、输出不进仓，缺 full 库或缺片就 Skip）；门禁（`TestRemuxSplitWithSubtitles` full 版绿 + `video/ffmpeg` 逐文件绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.116 ffmpeg双版本 | 双版本库链路（只动本线）：配方唯一真源 `tools/ffmpeg-recipe`（基础版解码 96+拆盒 186+切帧 28+过滤 12+滤镜 38+协议 17，全原生；高级版写盒 181 显式点名 + 原生编码 179 显式点名 + 烧字 drawtext/subtitles/ass，外库 freetype/harfbuzz/fontconfig/fribidi/libass/openh264 全静态，GPL 三个一个不开，LGPL 不变；教训：`--enable-muxer=all` 在 `--disable-everything` 后是空操作，实测零复用器，必须逐个点名）+ 构建脚本 `tools/ffmpeg/`（本机 docker 与 Actions 同一套：Dockerfile+build-deps.sh+build-one.sh；缺料容错：pkg-config 找不到的外库自动丢对应开关及连带滤镜，本次编“缺啥少啥”full 不卡死）+ workflow `.github/workflows/ffmpeg-dual.yml`（6 目标×2 版本 + GPL 门禁）+ Go 装载（`lib.go`：默认基础版旧名不动，高级版同目录 `_full` 后缀，`GPUI_FFMPEG_VARIANT=full` 才进，`GPUI_FFMPEG_PATH` 指哪加载哪；新 `Variant/IsFull/libRelNameFull` + `TestVariantRouting` 选路门禁）+ `.gitignore` 放行 6 个 `_full` 产物 + README/§4.3 同步；门禁（`vet` + `CGO_ENABLED=0` + `gofmt` + `video/ffmpeg` 逐文件绿 + 本地 docker 两套独立编出 linux-x64 双产物并存验过：基础版 14MB + 高级版 16MB 写盒 180 可列 + 字幕/音视频编码在 + GPL 零混入）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.115 ffmpeg说明补齐 | 739 个偏门方法全补中文说明（只动 `video/ffmpeg` 本线，真窗不动）：`crypto_hash_misc` 391 + `format_demux` 98 + `media_desc` 88 + `filter_graph` 64 + `resample_audio` 37 + `codec_encode` 35 + `scale_color` 18；每句说明含干什么、对哪个 ffmpeg 函数、参数、返回、nil 会不会崩（`go doc` 直接出）；README 用法约定同步一句；V-U3 口径句同步（13 功能模块 + 数据 + 变参 Go 版，变参已可用不再写跳过）；门禁（`video/ffmpeg` 逐文件绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.114 ffmpeg变参收尾 | 4 个变参全有 Go 拼串版可用（只动 `video/ffmpeg` 本线，真窗不动）：新 `variadic_go.go`（`Asprintf/Util.Strlcatf/IOContext.Printf/Log.Logf/Log.Once/BPrint.BprintfF`；Go 里 `fmt.Sprintf` 拼好再调不带变参的兄弟函数）+ 顺手修 `CloseDynBuf` 少回长度（`avio_close_dyn_buf` 回值是字节数，之前丢了只回错，`Printf` 收尾要用）+ 新 `TestVariadicGo` 钉死（整数/字符串/浮点全对 + 动态流收尾内容对 + 日志只验不崩）+ README/`doc.go` 同步口径（1011=991 函数+16 数据+4 变参全可用；直调证据：purego `...any` 只是逐个放寄存器，浮点必错 1.5 变 0.0，libc `snprintf` 实测）；门禁（`video/ffmpeg` 逐文件绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.113 ffmpeg数据收尾 | 16 个数据符号全包（只动 `video/ffmpeg` 本线，真窗不动）：`lib.go` 存 so 句柄 + 新 `data_const.go`（函数走 `RegisterLibFunc`、数据走 `Dlsym` 取地址再读，vet 过 `unsafeptr` 写法同 `SetGoCallback`）+ 10 个 `const int` 上下文大小（`Crypto.AesSize` 等 9 个 + `Util.TreeNodeSize`，实测 aes=288/camellia=280/cast5=140/tea=68/twofish=4276/md5=88/ripemd=128/sha=120/sha512=208/tree=32）+ 6 个 `const char[]` 版本串（`Library.Codec/Device/Filter/Format/Util/SwrFfversion`，全是 FFmpeg version 7.1.5）+ 新 `TestDataConst` 钉死（数字>0 + 版本串含 FFmpeg）+ README 对照表重建（13 节→14 节，函数注册 1005 行去重 1002 个 + 数据 16 个共 1021 行，`nm -D` 双向零差；覆盖口径同步 1011=991 函数+16 数据+4 变参跳过）+ `doc.go` 同步口径；4 个变参（`av_asprintf/avio_printf/av_log_once/av_strlcatf`，purego 表达不了）保持 Go 替代写法；门禁（`video/ffmpeg` 全绿 + `TestA2SoundHead` 抽查绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.112 video收敛 | 回归 + 保守收敛（只动 `video` 本线，真窗不动）：22 个测试文件逐文件全绿（3 个已知红归位：VR7-T 预算记 S1 攻坚、`TestS2PlayerSeekParity` 跳转时序竞态归跳转线、`TestStreamHTTPFullPlay` 少帧/companions 偶发抖动）；删 10 个空壳方法（`startAudioLoop/clearAudioQueue/closeAudioQueue/waitAudioLoop/wakeAudioLoop/parkAudioLocked/startAudioClockAtSeek/pauseAudioClock/resumeAudioClock/streamAudioDrained`，Go AAC 退役残留，零调用）+ `AudioQueue` 旧队 5 个方法（`Push/Pushes/MaxDepth/DepthAvg/Drained/Drain`，`PushRealtime` 是唯一生产路径，`Clear` 跳转清队保留）+ 过期 `video-only` 注释 3 处（`doc.go` 两处 + `AudioInfo` 头）；`ComputeTargetDelay/SelectMaster/AudioStepMs` 数学门面保留（门禁钉死，对外语义不动）；门禁（A2 全 7 项复验 + 抽查 15 项绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.111 声画跟随+音量 | 高级功能两件（只动 `video` 本线，真窗不动）：① 变速声音跟随 — `SetRate` 本就双钟同走（clock 缩放 + 按当前 due 重锚），声音侧无需另修，`TestA2RateKeepsSoundSync` 钉死（2x 照出 10 帧单调 + 回 1x 不倒退）；② 音量静音 — 新 `SetVolume`（0..4，非法拒收，默认 1）+ `SetMuted`（暂停出声不断解， reopen 同步续流），`TestA2VolumeAndMute` 钉死（双播放器同跳 8000 按戳配对逐采样 0.5x + 静音零出声 + 非法值拒收）；教训（`masterDue` 曾加声音限速 `audio+80`，与后台一画泵一块形成回路把管线卡成涓流，`TestA2SoundHead` 能量 0 抓现行，已退回纯墙钟，双方独立丢旧帧，声领走 `ComputeTargetDelay` 窗侧）；新问题归位（跳转线）：`TestS2PlayerSeekParity` 首帧 20200≠20000，两边对照都抖（音频前 2/3 红、音频后 1/3 红，同一死法，worktree 干净对照），系跳转时序竞态非本线，不在本线修；门禁（A2 全 7 项 + `ffmpeg`/`clock` + 播放跳转流注册 B 帧池逐文件绿 + `vet` + `CGO_ENABLED=0`；VR7-T 预算红系已知 S1 项）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.110 声音接通 | 音频解码接线（只动 `video` + `video/ffmpeg` 本线，真窗不动）：新 `video/ffmpeg/audio_decode.go`（`OpenAudio` 开最佳音轨 + `Next` 取 48kHz 立体声 float PCM + `SeekTo`/`Close`，swr 按 swresample.h 文档例配 `in/out_chlayout` + 采样格式/率，重采样经已绑定的 `Resampler` 方法，零新注册；单测 4 项绿：oceans AAC 身份/5 帧能量递增/跳转落点/无声片诚实无音轨）+ `Player` 接声音（`openFFmpeg` 另开一路音频、后台同线程每帧泵一块、`takeFFSeek` 同目标跳转、`Close` 同收；`HasAudio`/`Master`/`PollAudio`/`ProbeAudio` 由桩转实，`AudioQueue.PushRealtime` 满则丢最老永不卡画面，`AVDiffMs` 取末声音减末画面；A2 单测由 video-only 改有声：基线断言 AAC 48kHz 立体声 + 新 `TestA2SoundHead`（236 画面/231 声音双单调）+ `TestA2SoundSeeks`（四跳回声+跳转后声音续流），静音片 fallback 不动）；顺手修正 `codec_id.h` 音频 id（MP3=86017/AAC=86018/AC3=86019/Opus=86076，之前整体错位 1， oceans 实测抓现行）+ `swr_alloc_set_opts2` 格式参改 `int32` + 音频偏移注记进 `types.go`；门禁（`video`/`ffmpeg`/`clock` 全绿 + `vet` + `CGO_ENABLED=0` + `gofmt` 干净；VR7-T 预算红系已知 S1 项非本次）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.109 API审计修 | 按审计修四件（只动 `video/ffmpeg` 本线）：① `avio_alloc_context` 补齐 7 参（后 3 回调可传 nil，之前少 3 参是真 bug，未被播放器调用故未爆）；② 39 个枚举/值类型签名改 `int32`/`string`（`GetPixFmt` 等回指针改回值，`-1` 哨兵不再被当指针，冒烟验过 rgba/s16/420P 三平面/cuda/色度下采样）；③ 重复注册 18 组（`lib.go` 解码直连 14 + `crypto` 的 `sws_*`/`swscale_*` 21 套搬家到 `scale_color`，`av_buffer_unref` 搬进 `buffer_mem`，4 个无用老 raw 删除）；④ 回调接通（`NewLogCallback` + `Log.SetGoCallback`，`purego.NewCallback` 跳板，约 2000 上限）；`swscale_*` 三件返回值顺手改 `string`/`uint32`；README 对照表同步（crypto 删 21 重复行 + buffer 补 unref 行，去重后 1002 行）；门禁（`video/...` 构建 + `vet` + `CGO_ENABLED=0` + `decode_test.go` 绿 + 枚举冒烟一次性全过后删除）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.108 ffmpeg包README | 新 `video/ffmpeg/README.md`（大白话：包是干什么的、两层用法、库文件在哪、13 模块一览、用法约定、覆盖口径、快速上手、全部 API 对照 13 节共 1022 行 Go 入口→ffmpeg 函数，折叠默认收起）+ 补齐 8 个已注册未包方法的 Go 入口（`Log.DefaultCallback/FormatLine/FormatLine2`、`Math.AddStable/RescaleDelta/CompareMod`、`Clock.IsMonotonic`、`Cpu.ForceCount`，注册数不动）；门禁（`video/...` 构建 + `vet` + `CGO_ENABLED=0` 构建 + `decode_test.go` 绿全过）；§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.107 ffmpeg全API导出 | `video/ffmpeg` 按功能切 13 模块、so 里 991 个公开符号全绑成 Go 友好结构体方法（首参持有者指针→接收器方法、C int 返回 keep 名单留 int32 否则转 error、nil-safe；`decode.go` 高层 Open/Next/SeekTo/Close 不动，仍是播放唯一入口）；补 8 个漏网（`avutil_version/configuration/license`、`avsubtitle_free`、`av_crc`、`av_adler32_update`、`av_strndup`、`av_int_list_length_for_size`）+ `doc.go` 注记 4 个 C 变参主动跳过（`av_asprintf/avio_printf/av_log_once/av_strlcatf`，purego 表达不了，给替代写法）；门禁（`nm -D` 对注册表双向零差 + 新符号一次性冒烟全过 + `decode_test.go` 绿 + `video/...` 构建 + `vet` + `CGO_ENABLED=0` 构建过）；§0 V-U3/§4.1/§10 同步本行；§2/§5 VW0–VW3状态不动。 |
| v0.106 按功能改名 | 结构性规范调整（零逻辑改动，`git mv` 保历史）：`player.go` 拆四件（types/open/playback/seek，SetRate 归播放，ffDecoder 别名归后端）+ `ff_player/registry/fault/pool/source` 按功能改名 + `a2_sync/a2_player/audio_stub` 三合一 `sync_audio.go` + 22 个测试文件归组改名（player_*/playback_*/mem_*/gate_*/sync_audio/backend）；熔断锁改扫 `player_*.go` 四文件 + 坏输入换新路径 + `doc.go` 重写 ffmpeg 口径；门禁（19 文件绿 + 3 项已知抖动复现：S2 搜进度偶发错位、VR7-T 预算红、HTTP 全播偶发少帧 + `clock/ffmpeg` 绿 + `vet` + `CGO_ENABLED=0` + 19 真窗示例全构建过）；§4 包结构表同步新名；§2/§5 状态不动。 |
| v0.105 收敛删Go重写 | 收敛优化（只动本线，git 历史可恢复）：删 `video/mp4/h264/h265/aac/color` + 根 `bframe/s2_player/seek_index/v2_headers/audio` + A1 两测试，根包瘦为 ffmpeg 口径（`registry/fault/player/a2_player` + `audio_stub`），测试/示例逐 ffmpeg 口径改完（`registry/fault/b1/vr3/vr5/vr6/a2` 等 + 19 真窗示例）；门禁（`video` 根播搜容错注册稳态流/S2/S6/B/VR3-6/A2/B1 逐文件绿 + S8 索引测试随包删除 + 示例全 `go build` 过 + `vet` 过；已知抖动：S2 偶发首现错位、VR7-T 预算红记 S1 攻坚、HTTP 全播偶发少帧，见遗留）；待办（t-vr7d-base、t-audio-ffmpeg 延续）；§2/§5 VW0–VW3状态不动。 |
| v0.104 ffmpeg后端切换 | 解码切 ffmpeg 后端：新包`video/ffmpeg`（purego 绑定自建单文件 `libgpui_ffmpeg` 7.1.5，demux/codec/packet/frame/sws/swr/hwdevice/dict 约 40+ 接口，偏移按 7.1 头文件钉死）+ 新文件`video/ff_player.go`（`openFFmpeg/ffDecodeLoop/takeFFSeek/seekFFmpeg`：后台单线程拥有全部 ffmpeg 调用，Next+SeekTo 握手，队列=clock.Queue，循环 SeekTo(0)+epoch，S6 池借还）+ `video/player.go` 最小钩子（OpenFile/OpenWithSource 走 ffmpeg，SeekTo/SeekFast 分支，内存源 spool 临时 .mp4）；旧 Go `mp4/h264/aac` 留仓标 deprecated，播放器不再驱动；门禁（`video/ffmpeg` 解码 3 帧 + `ff_play` 播搜关 + `video` 根播放/搜进度/容错/注册/稳态/流/S2/S6/S8/B/VR3-7/A1/A2/B1 逐文件绿 + `clock/color/ffmpeg/aac` 绿 + `vet` + `CGO_ENABLED=0` 构建过）全过；口径（标题/V-U3/V-U4/§4.3/§7 合规同步本行：第三方只认 purego + `lib/ffmpeg` 自建二进制 LGPL 2.1）；待办（t-vr7d-base：VR7-D 需重定含 sws 的新基线，暂 log-only；t-audio-ffmpeg：音频解码未接线，A2 门禁暂 video-only）；§2/§5 VW0–VW3状态不动。 |

---

## 11. 生产级差距总表（现状 → 生产级 · 只记录差距，不改 VW0–VW3 已关状态）

> 性质：本节是 2026-09-14 基于生产级目标（任意大小/时长片源）盘点的差距清单，只做现状与缺口记录，不改变 §2/§3/§5 已关闭状态，不新增施工承诺。每一行都写清“生产级长什么样、我们现在在哪”。后继施工按 §12 分期认领，认领时再立门禁。

### 11.1 片源能打开（盒子）

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| B1 | 分段存放 MP4（手机录像/监控/直播回放主流写法，边录边存）能播 | 空 moov+moof 已拼表可播（2026-09-15，B1 门禁两片绿，见 §12 B1 行）；无表无段仍报 `ErrFragmented` 旧桶 | 主体已通；余中途换参/sidx 定时/mfra 直跳，非常见手机片不挡 |
| B2 | MKV / WebM / MOV / AVI / FLV / TS 常用盒子能播 | 只有 MP4 一种（注册表已留插槽，未注册新盒子） | 每加一种需拆盒+门禁，无施工 |
| B3 | 网站视频 HLS / DASH（切小片轮着下、网差降清晰度）能播 | 无切片拉取、无码率自适应 | 整块缺失 |
| B4 | 旋转/横竖屏/变帧率/章节信息全认 | 只认宽高帧率时长；旋转/编辑列表仅透出，未全生效 | 显示方向与时长仍有缺口 |

#### 11.1 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **B1 分段 MP4（moof）**：ffmpeg 在 `libavformat/mov.c`。认盒子走 `mov_read_moof`（记 `fragment.moof_offset`，调 `update_frag_index`，再递归吃 `traf`）；拼参数走 `mov_read_tfhd` / `mov_read_trex`（默认采样参数）/ `mov_read_tfdt`（本段起始解码时间）/ `mov_read_trun`（拼本段采样表）；段内跳转 `mov_read_seek` → `mov_seek_stream` → `mov_seek_fragment`（先 `search_frag_timestamp` 定位段，再按采样时间戳下钻）。分段 MP4 全走 ffmpeg demux（空 moov+moof 由 ffmpeg 拼表，见 §12 B1 行门禁两片绿）：段索引即 ffmpeg 统一采样表；`video/io_source.go` 本来就是 Range 分块，段包按段偏移拉。基线 `video/testdata/b1_ffmpeg.json`（同片同机：5 帧 B 帧片 + 100 帧 I/P 长片，ffprobe/benchmark/yuv 全入库）+ 门禁 `video/playback_frag_test.go` 4 项绿（头/像素逐字节/播到尾零丢单调 Ended/跳转 floor）。余：中途换参（avcC 切换）/mfra 直跳 + 真窗复验另开；未跑真窗故 §12 B1 行标 🟨。
- **B2 新盒子（MKV/WebM/MOV/AVI/FLV/TS）**：ffmpeg 各有一个独立 demux，只认接口（探测+打开+取包+跳转+关闭），我们 §4.4 注册表同构，直接对号入座。MKV/WebM 在 `libavformat/matroskadec.c`（入口 `matroska_read_header/packet/seek/close`，上下文 `MatroskaDemuxContext`，`ebml_parse` / `ebml_parse_nest` 自描述解析，`matroska_parse_block/cluster/track` 拼包，`matroska_parse_cues` / `matroska_add_index_entries` 建跳转索引，靠 `Cluster+Block/SimpleBlock` 拼包、`Cues+SeekHead` 跳转）；FLV 在 `libavformat/flvdec.c`（`flv_read_header/packet/seek/close` + `FLVContext`，纯 Tag 流顺序读，`flv_queue_extradata` 攒参数，跳转靠关键帧文件偏移线性找）；AVI 在 `libavformat/avidec.c`（`avi_read_header/packet/seek/close` + `AVIContext`，RIFF 分块，`avi_load_index` / `avi_read_idx1` / `guess_ni_flag` 索引表驱动，兼容非交织）；TS 在 `libavformat/mpegts.c`（`mpegts_read_header/packet/close`，无独立 `read_seek`，靠通用 seek + `seek_back` / `mpegts_get_pcr` 回退；`handle_packets` / `handle_packet` / `parse_pcr`，`mpegts_open_section/pes/pcr_filter`，188 字节包过滤 PAT/PMT 重组 PES，PCR 做时钟）。MOV 与 MP4 同文件（`mov.c`），最顺。要做：每种盒子单独一包，实现探测+打开+取包+跳转+关闭后 `RegisterContainer` 注册，播放器不动；先啃 MOV（复用现有 MP4 解析大半），再 FLV/AVI（索引简单），再 MKV（EBML 自描述最费），TS 最后（无全局索引，配 N3 直播一起啃）。验：每种盒子一片真片播到 `Ended` + 跳转 + 坏盒人话错。
- **B3 HLS/DASH**：ffmpeg 在 `libavformat/hls.c` + `libavformat/dashdec.c`，模型是“列表拉取 + 分片队列 + 按带宽/错误换版本”。HLS 结构 `HLSContext` → `variant`（`bandwidth`）→ `playlist` → `segment`（URL/时长/序号）；`hls_read_header` 调 `parse_playlist` 解析 m3u8 主/子列表，`select_cur_seq_no` 定首序号，`open_input` + `reload_playlist` 轮询刷直播列表，`hls_read_packet` 按 `cur_seq_no` 顺序消费，换码率走 `recheck_discard_flags`（按带宽/错误屏蔽版本）+ `update_variant_timing`，跳转 `hls_read_seek` 全列表重算序号再重开。DASH 结构 `DASHContext` → Period/AdaptationSet → `representation`（`bandwidth/id/url_template`）；`parse_manifest*` 边解析 XML 边建，`resolve_content_path` / `ff_dash_fill_tmpl_params` 展开 `$Bandwidth$/$Number$/$Time$`，`calc_cur_seg_no` / `calc_max_seg_no` 定号，`open_input` / `update_init_section` 拉 init+media，音/视/字幕各维护一组 representation，可 `reopen_demux_for_component` 开子 demux，`copy_init_section` 复用初始化段，跳转 `dash_seek` 按 `SegmentTimeline/duration` 算号（`dry_run` 预探被屏蔽流）。要做：新包（列表解析 + 分片队列 + 版本选择），复用 `Source` Range 拉分片，播放器加“换版本不断流”（排空旧队、重锚时钟、复用初始化段）；本地先用 `httptest` 喂 m3u8/MPD 夹具。验：网好播高清、网差自动降档不断、直播列表能跟、跳转不卡死。
- **B4 旋转/编辑列表/章节/变帧率**：ffmpeg 里旋转是 `mov.c` 读 `tkhd` 矩阵 + `displaymatrix` 侧数据透给显示（解码不管旋转）；编辑列表 `elst` 参与时长与起跳换算；章节是跳转表（`seek_chapter` 走同一 `stream_seek`）；变帧率靠 `stts/ctts` 逐采样时间戳（我们已有 `ctts`）。旋转/编辑列表/章节全走 ffmpeg（旋转透给消费侧做显示旋转，解码侧不转；`elst` 进时长与首帧起跳；章节跳走 ffmpeg）。验：竖拍片在 VR8 里方向对、留白片时长对、章节跳落点对。

### 11.2 画面能解（视频编码）

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| V1 | H.264 等级放宽到 4K 档（6.x），高等级片能开 | 等级卡 1–5.2，超限 fail fast | 等级上限是“打不开 4K”级缺口 |
| V2 | H.265 / VP9 / AV1 新编码能解 | 盒子认 hvc1/hev1、hvcC 解析、注册 h265 名、播报人话错不崩（2026-09-17，见§12 V2行）；独立真窗仍缺 | 头通，独立真窗仍缺 |
| V3 | 10 位深色 / HDR 片颜色不错 | 只有 8 位 `yuv420p`，10 位与 HDR 无管线 | 颜色管线缺口 |
| V4 | 隔行片能看（老采集卡/电视信号） | F12 只认出并报人话错，不解 | 罕见制式三条件留档，未解码 |

#### 11.2 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **V1 等级放宽（5.2 → 6.x，4K）**：ffmpeg 不卡等级（等级只是上限声明，`h264dec.c` 按 `SPS` 实际参数解，超限由内存与性能自然约束）。等级只做“声明校验 + 解前预估”（按宽高×队列算字节，超 `mem_cap_kb` 报装不下而不是报等级错，见 §12 S7 行）；解码全走 ffmpeg，ffmpeg 不卡等级。验：4K 片能打开、超内存片报装不下。
- **V2 新编码（H.265/VP9/AV1）**：ffmpeg 每个编码独立一包，各自注册：H.264 `libavcodec/h264dec.c`（`ff_h264_decoder`，`h264_decode_init/end`，`FF_CODEC_DECODE_CB(h264_decode_frame)` 一包一帧）；HEVC `libavcodec/hevc/hevcdec.c`（`ff_hevc_decoder`，`hevc_decode_init/free`，`FF_CODEC_RECEIVE_FRAME_CB(hevc_receive_frame)` 拉帧模型，适配延迟/多帧缓存）；VP9 `libavcodec/vp9.c`（`ff_vp9_decoder`，`FF_CODEC_DECODE_CB(vp9_decode_frame)`）；AV1 `libavcodec/av1dec.c`（`ff_av1_decoder`，`av1_decode_init/free/flush`，`RECEIVE_FRAME_CB` 拉帧）。一包一帧（DECODE_CB）与拉帧（RECEIVE_FRAME_CB）是两种消费契约。要做：新增编码走 ffmpeg 能力（ffmpeg 已支持 H.265/VP9/AV1 则直接复用，`video/ffmpeg` 绑定 + `video/registry_probe.go` 探测，不自研解码器；播放器不动）。顺序建议 H.265 → VP9 → AV1；每种独立门禁（ffmpeg 输出对齐 + 真窗）。验：新编码片播到 `Ended`。
- **V3 10 位/HDR**：ffmpeg 转色 `libswscale`（见 §11.6 S1）与颜色侧数据分离：10 位先解成 10 位平面，再经 dither 下到 8 位上屏（`format.c:fmt_dither` 选 Bayer 矩阵，执行在 `uops_tmpl.c`），HDR 另有色调映射（`tonemap` filter）。转色全走 ffmpeg sws。要做：10 位平面经 ffmpeg sws 下到 8 位上屏（dither 走 ffmpeg）→ HDR 加色调映射（后置，先报人话错再实现）。验：10 位片颜色不断层、SDR 片输出与 ffmpeg 一致。
- **V4 隔行**：ffmpeg 是真解（场解码 + 去隔行 filter `yadif/bwdif` 在 `libavfilter`）。隔行全走 ffmpeg（场解码 + 去隔行 filter `yadif/bwdif` 走 ffmpeg）。验：隔行片播出来无梳齿、逐行片与 ffmpeg 一致。

### 11.3 声音能出（现在是零）

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| A1 | AAC / MP3 / Opus 等声音能解 | 无声道、无解码 | 整块为零，最大单项缺口 |
| A2 | 音画对上（声音做主时钟，画面快了等、慢了丢） | 只有画面自己的钟，变速只动画面 | 时钟架构缺口 |
| A3 | 变速不变调、音量、静音、声道切换 | 无 | 整块缺失 |
| A4 | 系统声音输出（各系统各一套接口） | 无 | 宿主桥接缺失 |

#### 11.3 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **A1 声音解码（AAC 先行、Opus 随后）**：ffmpeg 在 `libavcodec/aac/aacdec.c`（`ff_aac_decoder` / `ff_aac_fixed_decoder`，`ff_aac_decode_init`，`FF_CODEC_DECODE_CB(aac_decode_frame)` 内调 `aac_decode_frame_int`；裸流预切 `aac_parser.c`）与 `libavcodec/opus/dec.c`（`ff_opus_decoder`，`opus_decode_init`，`FF_CODEC_DECODE_CB(opus_decode_packet)` 内调 `opus_decode_frame` 按 TOC 切包）。要做：声音解码全走 ffmpeg（`video/ffmpeg/audio_decode.go`：开最佳音轨 + 取 48kHz 立体声 float PCM + 跳转/关闭，重采样走 ffmpeg swr，不自研 AAC/Opus）。验：有声 MP4 能拆出音轨、解出 PCM、坏音频流人话错。
- **A2 音画对齐**：ffmpeg 在 `fftools/ffplay.c`，三钟同构 `Clock{pts/serial/speed/paused/last_updated}`（音/视/外），默认音频做主钟（`get_master_sync_type` / `get_master_clock` 按 `av_sync_type` + 流存在性选主）；读时间统一 `get_clock`（暂停则冻结），写 `set_clock` / `set_clock_at`（seek/队空校准），变速只改 `Clock.speed` 并重基准（`set_clock_speed`），防漂 `sync_clock_to_slave`，外钟按纳包速度微调（`check_external_clock_speed`）；视频同步在 `video_refresh`（按 `diff/sync_threshold` 丢/重显，算留多久 `compute_target_delay` / `vp_duration`，显示后 `update_video_pts` 刷视频钟）；音频同步在 `sdl_audio_callback` → `audio_decode_frame`（非音频主时 `swr` 补偿/丢样）。我们 `video/clock` 只有视频单钟（`Clock` + `Queue`，变速已接画面 `Rate`）。要做：音频另起队列（PCM 帧队列），`Clock` 一式两份（音钟主、视钟从），`Poll` 侧抄 `video_refresh`（超前等、落后丢，阈值写 README），seek 时双队同 `serial`（抄 `stream_seek` + `packet_queue_flush` + 新 serial 语义，我们已有 `generation` 世代号，直接对号）。验：声画漂移收敛不发散、seek 后双队同 serial、无声片回落视频主钟。
- **A3 变速不变调/音量/声道**：ffmpeg 变速不变调不在重采样库，在 `libavfilter/af_atempo.c`（`tempo` 选项，WSOLA 切片平移 + 重叠混合，`yae_curr_frag` / `yae_prev_frag` / `yae_clear` 管 fragment 环）；重采样在 `libswresample/swresample.c`（`SwrContext`，`swr_alloc/swr_init/swr_convert/swr_get_delay`，核 `resample.c`，汇编 `x86/resample.asm`）；声道矩阵 `rematrix.c`，抖动 `dither.c`。画面变速走 `video/clock`（rate 0.25–4x），声音侧走 ffmpeg（重采样走 swr，变速不变调走 `atempo` 语义，声道混音 + 音量/静音走 ffmpeg filter，不自研）。验：2x 人声不变尖、声道切换不断流、静音零爆音。
- **A4 系统输出**：ffmpeg 走 SDL 回调（`sdl_audio_callback` 拉 PCM，缺声回合落）。我们纪律是 `video` 本体不碰系统句柄（§4 同 `gpu` 禁令），桥接在宿主侧。要做：宿主侧（examples/平台层）对接系统音频输出（各系统各一套），`video` 只出 PCM 帧 + 时间戳 + 水位；断设备（拔耳机）报人话错并暂停不断线程。验：有声窗播出声、插拔不崩。

### 11.4 网络和直播

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| N1 | 缓冲水位状态机（水多就播、水少就转圈提示） | 只有 Range 分块缓存，无水位上报 | 体验缺口 |
| N2 | 断网重试、超时、弱网恢复 | 断一下就死，无重试 | 稳定性缺口 |
| N3 | 无限时长直播（帧目录不全读内存） | 帧目录一次全读内存，跳转逐个扫描 | “播不长”级缺口，见 S8 |

#### 11.4 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **N1 缓冲水位**：ffmpeg 在 `ffplay.c` 的 `PacketQueue`（`nb_packets/size/duration` 三维水位，判空/满/serial）+ `FrameQueue`（`pictq/sampq/subpq` 三实例，`rindex/windex/size/max_size/keep_last` 环形语义）+ `read_thread` 按水位启停 demux。要做：我们 `clock.Queue`（cap 默认 4，`DefaultCap`）加水位上报（包数/字节/时长三维，`Stats` 透出），播放器加三态机（播/转圈缓冲/恢复），阈值写 README；小样 HUD 接水位条（VR4 窗已有水位 HUD 雏形，照抄）。验：弱网先转圈后恢复，水位 JSON 全程有数。
- **N2 断网重试**：ffmpeg 在 `libavformat/avio.c`（`avio` 重连 + 超时选项）+ demux 层 `open_input` 重开。要做：我们 `video/io_source.go`（Range 分块）加超时 + 次数封顶重试 + 人话错（超时/404/500 分开，`player_stream_test.go` 已有 500 快败雏形）；重试时播放器不死（停钟、保队、重开后续流）。验：断网 3 秒恢复不断播、404 秒报可读错。
- **N3 无限时长**：ffmpeg 的 TS（`mpegts.c`）本来就无全局索引（188 字节包过滤 PAT/PMT 重组 PES，PCR 做时钟，seek 靠 `seek_back` 回退），HLS/DASH（见 B3）按序号消费不限总长。跳转走 ffmpeg 原生 seek（ longtime 片不全读内存）；直播模式不限总长（时长未知、水位驱动）。验：2 小时片打开毫秒级、跳转对数级、直播 1 小时内存不涨。

### 11.5 播放操作（手感）

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| P1 | 上次进度记忆、缩略图预览（拖进度条冒小图） | 无 | 功能缺口 |
| P2 | AB 循环、截图、逐帧手感打磨 | AB 循环/截图无；`StepFrame` 暂停单步已有雏形 | 功能缺口 |
| P3 | 字幕（内外挂 srt/ass 开关渲染） | V-U2 明确后置，无 | 整块缺失 |

#### 11.5 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **P1 进度记忆/缩略图**：ffmpeg 进度记忆在 cli 侧（`ffplay.c` 无内置，靠 `-ss` 起播 + 外层存）；缩略图在 `libavfilter/vf_thumbnail.c`（按帧差异挑缩略图）+ `vf_framestep.c`（按 N 步抽帧），真截图靠 `ffmpeg -ss/-frames:v` 或 ffplay 按 `s` 写 PNG（无专用 snapshot filter）。要做：进度记忆放小样/宿主侧（`PositionMs` 已有，存盘即可）；缩略图用 `StepFrame` 暂停单步 + 定时抽帧（`framestep` 语义抄），拖条浮层显示小图（先低分再高清）。验：二次打开续播、拖条小图与落点一致。
- **P2 AB 循环/截图/逐帧**：ffmpeg AB 循环是 seek 组合（`seek_chapter` + 同一 `stream_seek`，AB 两点来回跳）；截图是抽帧写文件；逐帧是 `step_to_next_frame`（暂停则 resume 再置 `step=1`，`video_refresh` 消费放一帧即停）。我们 `SeekBy/Next/PrevKeyframe/StepFrame/PositionMs/Seeking` 已有（v0.43），AB 只差两点来回跳 + 小样按钮；截图只差抽帧写 PNG（`Frame.Pix` 已有 RGBA，直接落盘）。验：AB 两点循环无缝、截图与画面逐位一致、单步一帧一停。
- **P3 字幕**：ffmpeg 在 `libavfilter/vf_subtitles.c` 一个文件 cover 两个 filter（`ass` + `subtitles`，`allfilters.c:ff_vf_subtitles` 注册；`init_subtitles` 开 track，`ass_render_frame` 合成，`filter_frame` 叠字幕，走 libass 烧录）。要做：字幕全走 ffmpeg（srt/ass 解析 + `subtitles`/`ass` 烧录走 ffmpeg filter，见 full 版 drawtext/subtitles/ass 门禁；显示走 VR8 内嵌浮层同路，时间戳对齐视频钟）；开关/轨道切换/样式走 ffmpeg。验：内外挂字幕时间对、开关不闪、ass 样式基本对。

### 11.6 跑得快（实测根因）

> 性能口径：解码与转色全走 ffmpeg（ffmpeg 原生 SIMD/多线程/HW 加速，不自研快车道）；`video` 只做队列/时钟编排，上屏走 `render`（见§4）。缺口如下：

| # | 生产级手段（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| S1 | CPU 算图快车道 | 走 ffmpeg 原生（sws/dsp 汇编已自带，不自研 Go 汇编） | 无缺口（ffmpeg 侧已覆盖） |
| S2 | 多核一起解 | 走 ffmpeg 原生线程（`thread_count`，不自研 Go 并行） | 无缺口（ffmpeg 侧已覆盖） |
| S3 | 熵解码加速 | 走 ffmpeg 原生（查表 + 汇编已自带） | 无缺口（ffmpeg 侧已覆盖） |
| S4 | 显卡/芯片硬解（4K60 正道） | V-U3 禁 CGO、`video` 禁 import 显卡包 | 全走 ffmpeg hwcontext 通路（先钉一台目标机：本机 Linux + Intel 核显 VA-API，再定接口；软解对照回落不断）；硬解帧须回传为 CPU 可读帧再交 `render` 上传，`video` 不出 GPU 句柄，消费侧不直调 `gpu`，见 §4/§4.5 |
| S5 | 零拷贝上屏（解完直接是显卡纹理） | CPU 逐像素搬运上墙 | 显示通路缺口 |

#### 11.6 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **S1 快车道**：ffmpeg 原生（转色 `libswscale` + dsp 汇编已自带，不自研 Go 汇编；dither 在 10 位下 8 位时走 ffmpeg）。验：与 ffmpeg 同片输出一致。
- **S2 多线程**：ffmpeg 原生（`libavcodec/pthread_frame.c` + `pthread_slice.c`，`thread_count` 开，不自研 Go 并行）。验：与 ffmpeg 同片输出一致。
- **S3 熵解码**：ffmpeg 原生（查表 + 汇编已自带）。验：与 ffmpeg 同片输出一致。
- **S4 硬解**：ffmpeg 抽象在 `libavutil/hwcontext.h/.c`（`av_hwdevice_ctx_create` 建设备，`av_hwframe_ctx_alloc` + `av_hwframe_transfer_data/map/get_buffer` 走帧），后端每家一个文件（`hwcontext_cuda/vaapi/qsv/d3d11va/d3d12va/dxva2/videotoolbox/mediacodec/opencl/vulkan/drm/vdpau/amf`），cli 接线 `fftools/ffmpeg_hw.c`（`hw_device_init_from_string/type` 解析 `-hwaccel`，全局 `HWDevice` 表供编解码/filter 取）。我们 V-U3 禁 CGO、`video` 禁 import 显卡包。要做：先钉一台目标机（本机 Linux + Intel 核显，先接 VA-API；NVDEC/VideoToolbox/MediaCodec 按平台随后一家一家加），再定 render 直通接口——`video` 只出“可硬解码的包 + 时间戳”，硬解帧一律经 ffmpeg 回传（`transfer_data`/`map`）为 CPU 可读帧后走现有 `video.Frame → render.ImageBuf → DrawImage` 上墙，`video` 不出 GPU 句柄/显存指针，消费侧（真窗/kit 组件）不直调 `gpu` 包建纹理（见 §4/§4.5）；exact 门禁对硬解放宽为容差比对 + 软解对照（同帧硬解与软解差分布写死，超出即 FAIL）。验：目标机上 4K60 满帧 + 回落软解不断 + 软硬差分布达标。
- **S5 零拷贝上屏**：ffmpeg 是 `hwcontext` 帧直接贴（DRM prime fd / D3D 纹理互操作，`map` 而非 `memcpy`）。我们 `video.Frame → render.ImageBuf → DrawImage` 全程 CPU 搬运。要做：纹理直传也只能由 `render` 层出接口（解码输出/硬解句柄由 `render` 建纹理，`video` 与消费侧不碰 `gpu` 包，显示矩形只传坐标，VR8 的“缩放只走显示矩形”纪律不变，kit 组件同理见 §4.5）；软解先做 PBO/持久映射（仍一次拷贝但不经 Go 堆），硬解再做真零拷。验：4K 上屏耗时降幅 + 大小窗同源一致。

### 11.7 装得下、播得完（任意大小/时长）

| # | 生产级要求（说人话） | 我们现在 | 差距 |
|---|---------------------|----------|------|
| S6 | 转色/帧缓冲池化（不每帧新分配大内存） | `video/mem_pool.go` 帧池（借还/泄漏/封顶可查）+ ffmpeg 直 scale 进池缓冲 | 池稳态零增长 + 同片峰值采数 + 四窗复验绿（见 §12 S6 行） |
| S7 | 内存封顶按档配置、超了报“装不下”不爆内存 | 已接上（2026-09-14晚）：五档上限写死（480p128MB/720p256MB/1080p512MB/1440p1024MB/4K2048MB，短边归档）+ 打开前解前预估拦（小片全量/流式有界两路，超限秒报装不下进mem-over-cap桶）+ 播中淘汰上报（有界队列丢旧计数+池evictions上报+Stats带cap/estimate/evictions） | 见 §12 S7 行 |
| S8 | 帧目录流式化（不全读内存）+ 跳转二分查 + 分段索引 | 跳转走 ffmpeg 原生 seek（ longtime 片不全读内存） | 跳转已对数级；2 小时毫秒开仍缺（留 N3/B1），见 §12 S8 行 |
| S9 | 长稳（几小时不漏内存、不越播越慢） | 最长证据 VC2 120s 循环 | 无长稳 soak 证据 |

#### 11.7 深度解析（对齐 ffmpeg，只学思路不搬代码）

- **S6 池化接上**：ffmpeg 是 `libavutil` 池 + `av_buffer_pool`（帧池借还）+ `hwframe_ctx` 帧池，`ffplay.c` 的 `FrameQueue` 环形复用。我们 `video/mem_pool.go` 帧池（借还/泄漏/封顶可查），ffmpeg 直 scale 进池缓冲（`Poll` 消费/队列丢弃/关窗回收归还，分辨率切换重建）。验（对等线见 §12 S6 行）：暖机后 miss 零增长 + 同片峰值≤maxrss + 与 ffmpeg 输出一致。
- **S7 封顶接线**：ffmpeg 是 `-max_alloc` 单块上限（`fftools/opt_common.c:1249 opt_max_alloc` → `libavutil/mem.c:76-77 av_max_alloc`，`:102 av_malloc`/`:158 av_realloc` 拒超限）+ 帧池上限（`libavutil/buffer.h:266 av_buffer_pool_init` + `buffer.c:390 av_buffer_pool_get` 借还复用）+ 有界队列（`fftools/ffplay.c:126 VIDEO_PICTURE_QUEUE_SIZE 3` + `:129 FRAME_QUEUE_SIZE` + `:705 max_size` 封顶 + `:751 peek_writable` 满则等 + `:789 frame_queue_next` 归还，另 `:1694/:1843 framedrop` 丢旧保时间线）。我们已接上（2026-09-14晚）：`video/mem_pool.go:MemCapKBFor` 五档上限（`EstimateLiveBytes` 按宽高×(YUV+RGBA)×(参考16+队列4+备3)+工作区重算写死，短边归档）+ `video/player_open.go:OpenFile/OpenWithSource` + `video/backend_ffmpeg.go:openFFmpeg` 解前预估拦（缓冲/流式两路，超限 `ErrMemOverCap` 人话 + `video/fault_classify.go:KindMemOverCap` 入桶）+ 播中 `video/clock/queue.go` 有界（`Push` 满则等 + `PollDue` 丢旧计 `Dropped`）+ `Stats.MemCapKB/EstimateB/PoolEvictions` 上报（健康 evictions 0，不静默）。验（对等线见 §12 S7 行 + `video/testdata/vr7m_ffmpeg.json` + `video/gate_perf_mem_test.go`）：超限秒报装不下、播中封顶不爆。
- **S8 目录流式化**：ffmpeg 普通 MP4 用 `stsc/stts/ctts/stco` 索引（`mov.c` 常驻内存，量小），frag 用段索引（B1），TS/HLS 不用全局索引（N3）。 longtime 片不全读内存。跳转走 ffmpeg 原生 seek（`libavformat/seek.c` 二分 + `mov.c` 先定段后下钻 + `ffplay.c:stream_seek` 拨针语义，不自研索引）。验（见 §12 S8 行）：落点与 ffmpeg 一致。
- **S9 长稳 soak**：ffmpeg 靠 FATE + 长跑 CI。我们最长 VC2 120s。要做：4 小时循环 soak（内存斜率/GC/CPU/漂移四曲线 + `rss_slope` 门禁沿 §2.7）， valuable 在 nightly 不在合入；先把 S6–S8 接完再跑，否则测出漏了也定位不清。验：4 小时斜率不超预算 + 首尾帧 exact 抽检一致。

### 11.8 小结：到“任意大小/时长”先补哪三项

1. **能打开**：B1 分段 MP4 + S8 目录流式化 + V1 等级放宽（否则大片长片直接拒播）。
2. **不断不爆**：S6 池化接上 + S7 封顶接线（否则大帧抖、小机爆）。
3. **跟得上**：S1/S2/S3 走 ffmpeg 原生（不自研；否则高分高帧跟不上，先上 S4 硬解，见 §12 S4 行）。

---

## 12. 后续分期（未启动 · 不算承诺，每项独立开工）

> 状态：**草案**。VW0–VW3 已收口；以下分期除特别标注外未开工、无门禁、无时间承诺，只做候选清单。**每项可单独在新会话开工**（单独立项：范围+门禁+真窗），不反写 §2/§3/§5。
> 状态图例：⬜ 未启动（可单独开工） · 🟨 进行中 · 🟩 已完成（真窗绿+回归绿+回写本文）。
>
> 对等验证总规则（硬 · 2026-09-14 起）：**每一项开工必须立 ffmpeg 对等验证**，三件缺一不开工：① 对等点（ffmpeg 哪个文件/函数/行为，对我们哪段代码）；② 对比物（同片 + 同命令 + 同指标，基线数写进门禁）；③ 通过线（逐字节零差异，或差值预算写死）。只比可比的：ffplay 带显示和声音，端到端帧率不直接对，只对解码段；内存只对比趋势与封顶（Go GC 与 ffmpeg 池架构不同，绝对值不对线）。
>
> 实现对齐硬纪律（硬 · 2026-09-14 起）：**先看 ffmpeg 再动手，不问用户要路线**。① 开工前必须翻对照库（上级 `gogpu/ffmpeg` 只读，不搬代码）：每项写清 ffmpeg 哪个文件/哪个函数/哪几行，对我们哪个文件/哪个函数；找不到对等点不开工，找到后把路径行号写进门禁注释与基线。② 路线问题（怎么切、开几个工人、常驻还是现开、小图走不走并行、池怎么借还）一律先抄 ffmpeg：转色抄 `libswscale/swscale.c` 分段（按行切段、各写各行、至少一行一段）与 `utils.c:ff_sws_thread_exec` 分片线程思想；线程数抄解码默认（`libavcodec/pthread_frame.c`：核数加一封顶 `MAX_AUTO_THREADS`）；池抄 `libavutil` 池 + `av_buffer_pool` 借还（稳态零新分配）。ffmpeg 就是这么干的就照着干，不问用户。③ 只有 ffmpeg 也没答案（两种做法都对，或 Go 特有无对等如 GC/堆绝对值见 VR7-G 行）才允许问用户，问时必须同时贴出已查的 ffmpeg 文件与行为、为什么没答案，否则打回。④ 防写错：改完必须过三道——输出与改前逐位一致（向量/exact 锁）、对等门禁（同片同机同命令基线）、所属回归逐文件绿；一道不过不标绿，不进真窗。
> 本机尺子（`ffmpeg 4.4.2`，同仓片源，2026-09-14晚更新）：长片 320x240x200 同片门禁基线为 `vr7t_ffmpeg.json`（rgba max 0.126秒/200帧 = 0.63ms/帧、yuv 参考 0.54ms/帧、maxrss 约 52MB，6 次取最大保守值）；我方同片转色段追测约 0.41ms/帧（p95 约 0.58ms，`Stats` 的 `decode_ms` 口径，池命中 98.5%），编译后门禁二进制同片约 15.7MB ≤ 52MB：转色段与内存段均已追平；解码与转色全走 ffmpeg（见 S1/S2/S3 行）。
>
> 对比清单（每项开工从这里领自己的三件，领完写进该项门禁）：解码正确性对解码器 YUV 输出（`gen_vr2.sh` 的 `.yuv`，逐字节零差异，VR2 已有）；头信息对 `ffprobe` + `trace_headers`（逐项一致，VR1 已有）；转色对 `libswscale`（向量单测零差异 + 窗容差，VR3 已有）；池化语义对 `av_buffer_pool` 借还 + `FrameQueue` 留末帧复用（S6 已收尾：CPU 件 `s6_pool_test.go` 稳态零增长 + 门禁 `vr7t_ffmpeg_test.go` 转色 0.41≤0.63 + 同片峰值 15.7≤52MB，窗级四窗复验绿）；解码耗时对 `ffmpeg -benchmark -i 片 -f null -` 的 utime/帧（差值预算待 S1/S2 立）；转色段耗时对 `ffmpeg -benchmark -pix_fmt rgba -f null -` 的 utime/帧（S6 已达 0.41≤0.63，S1 再压）；内存对 `bench: maxrss` 同片峰值与斜率（同片峰值 S6 已采 15.7≤52MB，长稳趋势待 S7/S9 立）。
>
> 禁自定数（硬 · 2026-09-14 起）：**所有分数线只认 ffmpeg 来源**，不再自己定数。存量自定线（§2.2/§2.7 各窗预算与 §12 各行旧说明）逐项替换为 ffmpeg 对等线，换完之前一律标注“自定（待替换）”，不冒充对等；新开工项直接执行本条。
> S6 对等线改写（旧自定线作废）：① 池行为：删“命中≥90%”，换暖机（首个队列深 + 2 帧）后 miss 零增长（对 `av_buffer_pool` 稳态零新分配；2026-09-14晚：miss 3 全在暖机，稳态 197/197 中，已达）；② 转色耗时：换我方转色段（`Stats` 的 `decode_ms` 口径）≤ `ffmpeg -benchmark -pix_fmt rgba -f null -` 同片 utime/帧（门禁基线 0.63ms/帧；2026-09-14晚常驻多核后追测 0.41ms/帧，已达，早前单线程 0.66ms 作废，不记 S1）；③ 内存：换同片 RSS 峰值 ≤ `bench: maxrss`（基线约 52MB 含转；2026-09-14晚：编译后门禁二进制同片 15.7MB ≤ 52MB，已采数；长稳趋势仍按 VR7-M 只看封顶不涨，交 S7/S9）。
>
> #### 12.1 存量线替换总表（VW0–VW3 全量 · 硬 · 2026-09-14 起）
>
> 顺序：按窗号从小往大换，VR0→VR1→VR2→VR3→VR4→VR5→VR6→VR7→VR8→VR9，最后换组合 VC0/VC1/VC2/VC3。每换一扇重跑该窗真窗 + 所属回归，换完回写该行说明，VW0–VW3 已关状态不动（换线是复验追记，不直接判旧窗 FAIL）。
> Go 特有无对等项（GC 停顿绝对值、堆绝对值、CPU 绝对值、hitch 绝对值）：ffmpeg 无此维度，不定绝对数，只留趋势门（不涨、不爆、不丢），见 VR7-G 行。
>
> | 窗 | 存量自定线（位置，待替换） | ffmpeg 对等（文件/行为） | 对比物 + 通过线（同片同命令） |
> |----|---------------------------|--------------------------|-------------------------------|
> | VR0 拆盒 | `track_found=1`、`keyframe_count≥1`、`duration_ms>0`（§2 主表） | `ffprobe` 流/格式 + `libavformat/mov.c` 盒子语义 | `ffprobe -v error -select_streams v:0 -show_entries stream=width,height,avg_frame_rate,codec_name,profile,level,duration,nb_frames -of default=nw=1` + `-show_entries format=duration` + `-skip_frame nokey -show_entries frame=pict_type` 数关键帧：宽高/帧率/时长/关键帧数逐项一致才过 |
> | VR1 参数 | `sps_ok=1`、`pps_ok=1`、`frames_split≥1`、档位等级全认（§2 主表） | `ffprobe` 头 + `ffmpeg -v trace` 头追踪（`libavcodec/h264*` 参数集） | 同片 `ffprobe` 档位/等级/宽高 + `ffmpeg -v trace -i 片 -f null -` 的 SPS/PPS/帧数：档位、等级、宽高、切分帧数逐项一致才过 |
> | VR2 解码 | `frames_decoded≥N`、`yuv_ready=1`、差异容差（§2 主表 + §2.9） | 解码器 YUV 输出（`gen_vr2.sh` 即 `ffmpeg -i 片 -f rawvideo -pix_fmt yuv420p`） | 已是 ffmpeg 线，保留：每帧与 `.yuv` 逐字节零差异；1440p/4K 本地片同样逐字节对，不定容差 |
> | VR3 颜色 | `color_diff_per_channel≤3`（§2 主表 + §6 VR3 行，自定 3） | `libswscale`（`yuv2rgb.c`/`input.c`/`output.c`）默认转色输出 | `ffmpeg -v error -i 片 -pix_fmt rgba -f rawvideo` 同帧输出：11 组向量逐字节零差异保留；真片差分布与 ffmpeg 查表噪声一致（R/B P99≤2、G P99≤3），删自定 3，改写进窗 README |
> | VR4 播放 | fps≥55、p95≤22ms（§2.2.2，自定）；`dropped_old_frames` 可解释、时钟漂移（§2.2 C） | `ffplay.c` 的 `video_refresh`（`diff`/`sync_threshold`/`compute_target_delay` 丢帧保时间线，`update_video_pts`） | 端到端 fps 不对线；对解码段：同片 `ffmpeg -benchmark` utime/帧 ≤16.6ms 则我方须零丢满帧，否则允许按 ffplay 语义丢旧保时间线，但丢数 = 应丢数自洽 + 显示序单调 + Ended 才过 |
> | VR5 跳进度 | 落点预算 500ms、恢复≤14ms（§2 主表 + §6 VR5 行，自定） | `ffmpeg -ss` 落点 + `ffplay.c` 的 `stream_seek`（拨针 + 顺手丢帧 + 新 serial） | 同片 `ffmpeg -ss 时间 -i 片 -frames:v 1` 落点：我方落点须为目标前覆盖关键帧且差值 ≤ ffmpeg 落点差；跳后首现帧 PTS = 落点，恢复耗时只记录不设自定线 |
> | VR6 容错 | `fault_cases_pass==total`、人话错（§2 主表，自定用例集） | `ffmpeg -v error -i 坏片 -f null -` 退出行为 + `tests/fate` 坏文件思路 | 同坏片逐个对：ffmpeg 报错我方须报同层桶（`Classify`），ffmpeg 能播完我方须播完且隔离帧数 ≤ ffmpeg 花帧数；三条件罕见项（F12/F17）保留但须写清 ffmpeg 同行为 |
> | VR7-D 解码耗时 | `decode_ms_p95` 超 README 预算 FAIL（§2.2 B + §2.7，自定） | `ffmpeg -hide_banner -benchmark -i 片 -f null -` 的 utime/帧 | 同片 utime/帧即上限：我方 `decode_ms_p95`（`Stats` 口径）≤ 该数才过；未达记 S1/S2 攻坚，不自降预算 |
> | VR7-T 转色耗时 | 含在解码耗时内（自定） | `ffmpeg -hide_banner -benchmark -pix_fmt rgba -f null -` 的 utime/帧 | 同 S6 第②条：我方转色段 ≤ 该数（本机长片 0.60ms/帧）才过 |
> | VR7-M 内存封顶 | 峰值≤512MB、`mem_cap_kb`（§2.2 E + §2.7，自定数） | `bench: maxrss` 同片峰值 + `-max_alloc`/帧池上限思想 | 同片 `maxrss` 为参照：峰值只看趋势与封顶（Go GC 不对绝对值），超 `mem_cap_kb` 的 `mem_cap_kb` 本身改按 `EstimateDecoderBytes` 逐档重算后写死，不拍数 |
> | VR7-A 分配 | `alloc_per_frame_B` 预算（VR7 2048B/帧，自定） | `libavutil/mem` 池 + `av_buffer_pool` 稳态零新分配 | 删字节预算，换 S6 第①条：暖机后 miss 零增长 + 稳态大内存零新分配才过 |
> | VR7-P 池命中 | `pool_hit_pct` 预算（90%，自定） | 同上 + `FrameQueue` 留末帧复用 | 删 90%，换 S6 第①条（已达） |
> | VR7-G GC/堆/CPU/hitch/首帧/冷解码 | GCp99≤10ms、斜率≤30000KB/min、CPU≤85%、卡顿≤50/min、首帧≤2000ms、冷解码p95≤50ms（全自定） | ffmpeg 无此维度（Go 运行时特有） | 不定绝对数，只留趋势门：GC/堆/CPU/斜率只看不涨不爆（长稳曲线），首帧/冷解码只记录同机毫秒数，不设线；同片 ffmpeg 能实时则我方长稳须不丢帧 |
> | VR8 内嵌 | `embed_ok=1` + fps 门禁（§2 主表，自定） | 同 VR4 + 显示矩形缩放（`libswscale` 缩放语义只走显示侧） | 正确性沿 VR2/VR3 线，fps 沿 VR4 线；缩放/裁剪/透明只许走显示目标矩形，解码侧分辨率与 VR2 exact 一致 |
> | VR9 注册 | `registry_cases_pass==total`（§2 主表，自定用例集） | `allfilters.c`/解码器注册思想（可插拔，不定数） | 同片走注册表输出与直连逐位一致；不支持格式与 ffmpeg 退出行为一致（人话错零崩溃）；用例数只记录不设线 |
> | VC0/VC1/VC2/VC3 | 沿单窗预算（自定） | 沿各覆盖 VR 的对等线 | 不单设线：五站/三跳/长跑/三合一逐站复用对应 VR 行通过线；VC2 120s 斜率只留趋势门（不涨不爆），绝对值不对线 |

| 项 | 归属 | 内容（认领 §11 缺口） | 状态 | 说明 |
|----|------|----------------------|------|------|
| S6 | VW4 能装下 | 池化接上（ffmpeg 直 scale 进 `video/mem_pool.go` 帧池借还） | 🟩 已完成 | 收尾绿（2026-09-14晚，见 `video/gate_perf_mem_test.go`）：①池稳态零增长（200帧命中98.5%/miss仅3暖机/关窗零泄漏）②转色走 ffmpeg（0.41≤0.63门禁绿）③同片峰值15.7MB≤52MB已采数（长稳趋势交S7/S9）；四窗复验绿（VR2差0/VR3向量0+md5/VR4零丢/VR7六十秒分配88B池99.7%峰值499MB<512MB） |
| S7 | VW4 能装下 | 内存封顶接线（解前预估 + 超限报装不下 + 播中淘汰上报） | 🟩 已完成 | 五档上限（480p128/720p256/1080p512/1440p1024/4K2048MB，短边归档，`EstimateLiveBytes` 重算写死）+ 解前两路拦（`ErrMemOverCap`+`KindMemOverCap`桶）+ 播中有界淘汰上报（`Dropped`+`PoolEvictions`+`Stats`三字段）；对等 `libavutil/mem.c:76-77 av_max_alloc`+`buffer.h:266/buffer.c:390`池+`ffplay.c:126/129/705/751/789`有界队列（见§11.7 S7）；基线 `video/testdata/vr7m_ffmpeg.json`（同片bench maxrss 50228kB参照，档live重算表）+ 门禁 `video/gate_perf_mem_test.go`（档表自洽+同片Ended+单调+零挤出+8K秒报）绿；四窗复验绿（VR2差0/VR3向量0+md5/VR4零丢/VR7六十秒分配91B池99.7%峰值496MB<512MB零泄漏）；`video`根/`video/ffmpeg`/`video/clock`逐包绿，`vet`+`CGO_ENABLED=0`构建过；VR7-D未达（12.2>8.8ms）系旧缺口（现走 ffmpeg 原生，见 S1/S2 行），改前改后同红与S7无关 |
| S8 | VW4 能装下 | 目录流式化（段表 + 二分，小片快路不动；页读留 N3/B1） | 🟩 已完成 | 查表半落 + 真窗复验绿（2026-09-15，引擎/门禁/数据/接口零改）：五片逐位一致 + 步数对数级（bound 20/24/26，实测 10/11/12）+ 快路不动+ 真窗 `examples/video_s8_index` RUN15绿（索引18/18零坏/前解最大4远小于全片200/直播77/跳2/恢复≤80ms/上屏867/fps57.0/p95 17.5ms/卡顿3/vsync true/首帧94.4ms/峰值276388KB<1GB/§2.2全族齐）+ 常驻30秒绿（backend x11/真事件55/上屏1743/直播152/跳2/门禁18/18）；目录页读 + 2小时毫秒开仍缺（留 N3/B1，另开，不挡本项） |
| B1 | VW4 能装下 | 分段 MP4（moof 段索引 + 段内 seek 下钻） | 🟩 已完成 | 初落 + 收敛 + 真窗复验绿（2026-09-15，引擎/门禁/数据/接口零改）：基线 `video/testdata/b1_ffmpeg.json`（5 帧 B 帧片 + 100 帧 I/P 长片）+ 门禁 `video/playback_frag_test.go` 4 项绿（头2/像素105帧逐字节/播到尾2片零丢单调 Ended/跳转5次 floor）+ 真窗 `examples/video_b1_frag` RUN15绿（分段114/114零坏/差异0/直播76/上屏886/fps58.3/p95 18.0ms/hitch0/vsync true/首帧105.9ms/峰值276MB<1024MB/§2.2全族齐，`go vet`过）；ffmpeg 对等仍绿；余中途换参/mfra 直跳另开，不挡主体 |
| S1 | VW5 跟得上 | 快车道（全走 ffmpeg 原生） | 🟩 已完成 | ffmpeg 原生覆盖（sws/dsp 汇编已自带） |
| S1b | VW5 跟得上 | 解码主体（全走 ffmpeg） | 🟩 已完成 | ffmpeg 原生覆盖 |
| S2 | VW5 跟得上 | 多线程（全走 ffmpeg 原生线程） | 🟩 已完成 | ffmpeg 原生覆盖（`thread_count`）：门禁绿 + 四窗复验绿（VR2差0/VR3向量0/VR4零丢/VR7分配89B池99.7%/上屏3417）；`Options.S2Parallel` 兼容保留；S2关🟩 |
| S3 | VW5 跟得上 | 熵解码（全走 ffmpeg 原生） | 🟩 已完成 | ffmpeg 原生覆盖 |
| A1 | VW6 有声音 | 声音解码（全走 `video/ffmpeg/audio_decode.go` + swr） | 🟨 进行中 | ffmpeg 通路已通（48kHz 立体声 float PCM + 波形门禁，见 `video/ffmpeg/audio_decode.go` + `video/sync_audio_test.go`）；真窗另开 |
| A2 | VW6 有声音 | 音画对齐（PCM 队列 + 第二时钟 + 双队同 serial） | 🟩 已完成 | 落地绿（2026-09-16，见 `video/sync_audio_test.go`）：声领画随（有声片音频主钟/静音回落视频）+ 双队同序号（一次seek双针同搬）+ 变速暂停双钟同走 + 基线`video/testdata/a2_ffmpeg.json` + 门禁`video/sync_audio_test.go` 5项绿 + 真窗`examples/video_a2_sync` RUN15两连绿（对齐8/8/直播111/音约458/差≤170/跳3/恢复≤273ms/上屏约690） |
| A4 | VW6 有声音 | 宿主音频桥接（系统输出 + 插拔处理） | 🟩 已完成 | 落地绿（2026-09-16，见 `examples/video_a4_sink`）：宿主侧桥接（`examples/video_a4_sink`，`video`零改动，§4纪律）+ 对等`ffplay.c`音频三件（见源码头注释）+ 基线`video/testdata/a4_ffmpeg.json`（同A2门禁片，LC/44.1k/双声道/217包/ASC121056e500/320x240，阈值与差预算沿A2线；本地声卡s16le 2ch 44100Hz，paplay优先/aplay兜底/WAV链；手写Pulse线协议不抄SDL未做，见基线peer_note）+ 门禁`examples/video_a4_sink`6项绿（壳1/向量11/链1/泵1/备1/拔线1，headless可跑，无喇叭诚实跳过不装绿）+ 真窗`examples/video_a4_sink` RUN15复验绿（2026-09-16：探针15/15/播134/音583/主audio/差≤39/拔线杀→停→恢/上屏567/`gpu_backend=integrated`零回退/`display_backend=x11`/§2.2全族齐；报告新增诚实遥测键：队列水位/追帧丢帧/时钟漂移/转色耗时/音频编解水位走播放器真值，构建光栅耗时与present策略走调度快照真值，门禁线零改，`go vet`过）；§2/§5 VW0–VW3状态不动，其余§12行不动（A1/A2未动：A2门禁5项抽查仍绿） |
| V2 | VW7 片源广 | 新编码（全走 ffmpeg） | 🟨 进行中 | ffmpeg 通路（盒子认 hvc1/hev1 + 注册 h265 名 + 播报人话错，见 `video/ffmpeg` + `video/registry_probe_test.go`）；像素/B/独立真窗按 ffmpeg 输出验收，另开 |
| B2 | VW7 片源广 | 新盒子（MOV 先行，其一 + 独立门禁） | ⬜ 未启动 | 可独立并行 |
| B3 | VW7 片源广 | HLS/DASH（列表 + 分片队列 + 版本选择） | ⬜ 未启动 | 可独立并行 |
| P3 | VW7 片源广 | 字幕（srt 先行 + 烧录 + 开关） | ⬜ 未启动 | 可独立并行 |
| S4 | VW5 跟得上 | 硬解（先钉目标机：本机 Linux + Intel 核显 VA-API，再定 render 直通接口；硬解帧回传 CPU 可读帧再交 `render` 上传，见 §11.6 S4） | ⬜ 未启动 | 接口先行；S5 纹理直传（render 出接口）随后另开 |
| K1 | VW8 组件化 | 视频播放组件（`ui/kit` video_*，包 `video` 能力，渲染只走 `render`，见 §4.5） | ⬜ 未启动 | 可独立并行；验收走独立 kit 真窗 `examples/kit_video`，不占 VR 名额，不代替任何 VR |

> #### 12.2 推进顺序与进度（维护表 · 硬 · 2026-09-14 起）
>
> 走法（三步，缺一不算完）：① 同片同机采 ffmpeg 基线数入库 → ② 改该窗测试断言为对等线 → ③ 实现追平 + 真窗复验回写。每推完一步更新本表状态（图例同 §12）；§12.1 是每扇的对照细则，本表是排到哪的进度。
>
> | 顺序 | 阶段 | 步骤 | 对应 | 状态 | 进展 |
> |------|------|------|------|------|------|
> | 1 | 重测换线（VW0–VW3 复验） | VR0 重测 + 换线 + 复验 | §12.1 VR0 行 | 🟩 已完成 | A小步（2026-09-14）：基线 `video/testdata/vr0_ffprobe.json`（ffprobe 4.4.2：v0.49 起以 vr_oceans 960x400/23.976fps/1116采样/28关键帧 + vr_f42906 852x480/24fps/812采样/5关键帧 + 720p 常跑为准；ENERGY 片已不在仓内见 v0.49）+ 门禁 ffmpeg 对等门禁（VR0）（宽高/编解码映射/profile系/level/帧率±0.02/流对轨与格式对头各±50ms/采样与关键帧精确）三片绿 + 真窗 `video_vr0_demux` 三路 RUN5 全绿（合成键4/4000ms，oceans 键28/46546ms，f42906 键5/33833ms，坏错均可读，VR0-1 窗坏例已改砸首盒）+ `video` 回归绿；§2 VR0 行不动 |
> | 2 | 重测换线（VW0–VW3 复验） | VR1 重测 + 换线 + 复验 | §12.1 VR1 行 | 🟩 已完成 | B小步（2026-09-14）：基线 `video/testdata/vr1_ffprobe.json`（ffprobe 4.4.2 + ffmpeg 4.4.2 `-v trace`：与 VR0 同三片 oceans 960x400/Constrained Baseline/30/视频1116帧 + f42906 852x480/High/30/视频812帧 + 720p 1280x720/Main/31/5帧常跑；SPS/PPS 去重各 1，trace 原始各 2 行系找流与解码双初始化；帧数对视频流，f42906 总 2269/oceans 总 3301 均含音频不对总数）+ 门禁 ffmpeg 对等门禁（VR1）（档位走 avcC[1]/等级走 avcC[3]/宽高精确/参数集 1-1/切分帧数精确，66 系兼容 Constrained Baseline 拼写，引擎零改动）三片绿 + 真窗 `video_vr1_params` 三路 RUN5 全绿（合成 High/3.1 切 4 帧上屏 297，oceans Baseline/3.0 前 8 采样切 8 帧上屏 296，f42906 High/3.0 前 8 采样切 8 帧上屏 296，缺参数坏流可读）+ 回归（`video`/`video/ffmpeg` 绿含 VR0 parity 仍绿、`vet` 过）；§2 VR1 行不动 |
> | 3 | 重测换线（VW0–VW3 复验） | VR2 重测 + 换线 + 复验 | §12.1 VR2 行 | 🟩 已完成 | C小步（2026-09-14）：基线 `video/testdata/vr2_ffmpeg.json`（ffmpeg 4.4.2：`gen_vr2.sh` 十四片，帧数/尺寸/档位等级/yuv 字节+md5 全入库）+ 门禁 ffmpeg 对等门禁（VR2） 十四片绿（档位走 avcC[1]/等级走 avcC[3]/宽高精确/解码序 POC/逐字节零差异，小片双打包、大片 AVCC，引擎零改动；高档三片首次进像素门禁）+ 真窗 `video_vr2_decode` RUN15 绿（解码 15/差异 0/上屏 881）+ 回归（`video`/`video/ffmpeg` 绿含 VR0/VR1 parity 仍绿、`vet` 过）；§2 VR2 行不动 |
> | 4 | 重测换线（VW0–VW3 复验） | VR3 重测 + 换线 + 复验 | §12.1 VR3 行 | 🟩 已完成 | C2小步（2026-09-14）：基线 `video/testdata/vr3_ffmpeg.json`（ffmpeg 4.4.2：与窗同三片 b_intra 96x96/Constrained Baseline/10 + 480p裁边854x480/Main/22 + 720p/Main/31，各5帧/5fps，rgba逐帧md5+差分布最大2/3/2且R/B P99≤2/G P99≤3，向量11组零差异，旧自定3作废）+ 门禁 `video/vr3_ffmpeg_test.go`（流身份走avcC/宽高精确/播到Ended/显示序单调PTS递增/逐帧md5锁死，引擎零改动）三片绿 + 真窗 `video_vr3_color` RUN5 绿（向量差0/三片15帧md5对上/上屏295/§2.2全族齐）+ 回归（`video`/`color`/`h264`/`mp4`/`clock`逐文件绿、`vet`+CGO构建过）；§2 VR3 行不动 |
> | 5 | 重测换线（VW0–VW3 复验） | VR4 重测 + 换线 + 复验 | §12.1 VR4 行 | 🟩 已完成 | D小步（2026-09-14）：基线 `video/testdata/vr4_ffmpeg.json`（ffmpeg 4.4.2：与窗同三片 B帧96x96/Main/10 + 480p 854x480/Main/22 + 720p/Main/31，各 5帧/5fps，utime/帧取多次最大 1.0/2.6/3.8ms，全≤16.6ms 故零丢满帧；端到端 fps 不对线）+ 门禁 `video/vr4_ffmpeg_test.go`（手表播到 Ended：宽高/预算自洽/5/5 零丢/显示序单调 PTS 递增/Ended，引擎零改动）三片绿 + 真窗 `video_vr4_play` RUN30 绿（三档解码 15/显示 90/零丢/直播 126/上屏 1734/fps57.3/p95 18.8毫秒/§2.2 全族齐）+ 回归（`video`/`clock`/`mp4`/`color`/`h264` 逐文件绿、`vet` 过；`stream_long_test.go` 偶发抖动单例重跑即过，属别线未动）；§2 VR4 行不动 |
> | 6 | 重测换线（VW0–VW3 复验） | VR5 重测 + 换线 + 复验 | §12.1 VR5 行 | 🟩 已完成 | E小步（2026-09-14）：基线 `video/testdata/vr5_ffmpeg.json`（ffmpeg 4.4.2：与窗同三片 双IDR96x96/Main/10/10帧 + B帧96x96/Main/10/5帧 + 480p裁边854x480/Main/22/5帧，三片.mp4已进仓；对等语义：我方落覆盖帧floor、ffmpeg -ss落天花板ceiling，格点上同帧差0，格点间各差一格如vr5 1.3秒我方1.2秒差100对ffmpeg 1.4秒差100，通过线为我方差值≤ffmpeg差值；时间坐标差elst偏移ffmpeg从0起、我方vr5/bframes首帧400毫秒480p首帧200毫秒，基线以shift_ms换算；恢复耗时只记录不设线）+ 新门禁 `video/vr5_ffmpeg_test.go` 三片五跳绿（流身份走avcC档位等级/宽高精确、落点键差前解精确、Stats对齐、跳后下拍首现PTS等于落点，引擎零改动）+ 真窗 `video_vr5_seek` RUN15绿（四跳全绿差0/直播84帧10跳恢复≤15毫秒/上屏883/§2.2全族齐）+ 回归（`video`搜进度/VR4对等/S6池/播放/流/容错/注册绿、`clock`/`mp4`/`color`绿、`h264`十二文件绿含VR1/VR2 parity仍绿、`vet`+CGO构建过；注：`stream_long_test.go`的HTTP整播偶发多丢、SeekBackward偶发首现未落，单例重跑即过，属别线流式长片抖动未动）；§2 VR5行不动 |
> | 7 | 重测换线（VW0–VW3 复验） | VR6 重测 + 换线 + 复验 | §12.1 VR6 行 | 🟩 已完成 | F小步（2026-09-14）：基线 `video/testdata/vr6_ffmpeg.json`（ffmpeg 4.4.2：文件6例缺文件exit1无此文件+junk71B Invalid data exit1+截断尾3514B error reading header/End of file exit1+花屏6002B split错exit0解9/10缺dts3变哈希1/2/4受影响4+等级60片3714B exit0解5/5哈希与好片一致+好片5/5，四冻结坏文件进仓字节锁死；单元9例缺参数/F17四路/F20两路/F12三路/超档位各记ffmpeg同文件行号与日志串；通过线ffmpeg报错同层桶、播完则Ended且隔离≤受影响，花屏隔离2≤4，等级分歧待V1，F12/F17三条件留档）+ 新门禁 `video/vr6_ffmpeg_test.go` 六文件七单元十三子项绿（字节锁死/基线自洽/同层桶/播到Ended/隔离数/ profile等级分桶，引擎零改动）+ 真窗 `video_vr6_fault` RUN15绿（12/12/花屏隔离2帧剩8帧/直播76/上屏878/fps57.8/p95 18.6毫秒/§2.2全族齐）+ 回归（`video`容错/对等VR4/VR5/VR6/播放/搜进度/注册/S6池/稳态/流/长片/合规逐文件绿、`clock`/`mp4`/`color`绿、`h264`十二文件绿含VR1/VR2 parity仍绿、`vet`+CGO构建过）；§2 VR6行不动 |
> | 8 | 重测换线（VW0–VW3 复验） | VR7 重测 + 换线 + 复验 | §12.1 VR7 各行 | ⬜ 未启动 | – |
> | 9 | 重测换线（VW0–VW3 复验） | VR8 重测 + 换线 + 复验 | §12.1 VR8 行 | ⬜ 未启动 | – |
> | 10 | 重测换线（VW0–VW3 复验） | VR9 重测 + 换线 + 复验 | §12.1 VR9 行 | ⬜ 未启动 | – |
> | 11 | 重测换线（VW0–VW3 复验） | VC0/VC1 复验（沿单窗线） | §12.1 VC 行 | ⬜ 未启动 | – |
> | 12 | 重测换线（VW0–VW3 复验） | VC2/VC3 复验（沿单窗线，斜率只留趋势） | §12.1 VC 行 | ⬜ 未启动 | – |
> | 13 | VW4 能装下 | S6 收尾（池稳态零增长 + 转色追平 + 内存采数 + 四窗复验） | §12 S6 行 | 🟩 已完成 | 三线全达：池已达/转色0.41≤0.63已达/内存15.7≤52已采数（2026-09-14晚，见 `video/gate_perf_mem_test.go`） |
> | 14 | VW4 能装下 | S7 封顶接线 | §12 S7 行 | 🟩 已完成 | 五档上限+解前两路拦+播中淘汰上报全接上（2026-09-14晚，见§12 S7行）：基线 `video/testdata/vr7m_ffmpeg.json` + 门禁 `video/gate_perf_mem_test.go` 绿 + 四窗复验绿（VR2/VR3/VR4/VR7） |
> | 15 | VW4 能装下 | S8 查表半落 + 真窗复验（B1 已🟩见 B1 行，后续页读另开） | §12 S8 行 | 🟩 已完成 | S8 索引落地 + 真窗复验绿（见 S8 行：RUN15 18/18零坏 + 常驻30秒55真事件）；B1 分段已 🟩（真窗RUN15绿，见 B1 行） |
> | 16 | VW5 跟得上 | S1 快车道 | §12 S1 行 | 🟨 进行中 | 转色双核落地+收敛+插值核绿+去块核绿（见S1行）+桌面收尾2026-09-15（引擎零改）：VR7-D闲时10连5绿5红认账红+三核/VR2十四片逐文件绿+VR7真窗RUN60绿，三笔未全绿故仍🟨不标🟩，与 S2 同批文件，一前一后 |
> | 17 | VW5 跟得上 | S2 多线程 | §12 S2 行 | 🟩 已完成 | ffmpeg 原生线程 + 门禁绿 + 四窗复验绿（见S2行） |
> | 18 | VW5 跟得上 | S3 熵解码 | §12 S3 行 | 🟩 已完成 | ffmpeg 原生覆盖 |
> | 19 | VW6 有声音 | A1 AAC 先行 | §12 A1 行 | 🟨 进行中 | 落地2绿（真PCM+波形门禁，见 `video/ffmpeg/audio_decode_test.go` + `video/sync_audio_test.go`；未跑真窗故不标🟩） |
> | 20 | VW6 有声音 | A2 音画对齐 | §12 A2 行 | 🟩 已完成 | 落地绿（2026-09-16，见 `video/sync_audio_test.go`）：门禁5项绿 + 真窗RUN15两连绿（8/8，差≤170ms） |
> | 21 | VW6 有声音 | A4 宿主音频桥接 | §12 A4 行 | 🟩 已完成 | 宿主桥接+门禁绿+真窗RUN15两连绿（见A4行；引擎零改，A2抽查仍绿） |
> | 22 | VW7 片源广 | V2 H.265 头已通 | §12 V2 行 | 🟨 进行中 | 盒子+hvcC+注册+门禁+VR9真窗RUN15绿（见V2行；像素/独立真窗另开） |
> | 23 | VW7 片源广 | B2 新盒子（MOV 先行） | §12 B2 行 | ⬜ 未启动 | 可独立并行 |
> | 24 | VW7 片源广 | B3 HLS/DASH | §12 B3 行 | ⬜ 未启动 | 可独立并行 |
> | 25 | VW7 片源广 | P3 字幕 | §12 P3 行 | ⬜ 未启动 | 可独立并行 |
> | 26 | VW8 组件化 | K1 视频播放组件（`ui/kit` video_*，验收走独立 kit 真窗） | §12 K1 行 | ⬜ 未启动 | 可独立并行（`video` 接口须先齐：打开/取帧/播控/状态事件；S1b 基线数字保留对照，不删） |

---

## 13. 声音模块（原 §11 · 本期不做，并入 VW6 候选）

> 状态：**未立项**。本期（VW0–VW3）只做画面，声音等后继分期单独立项成篇，不在本文件里施工。原 §11 暂定范围（AAC 先行/Opus 随后、复用音频轨、音频另起队列、宿主侧桥接、独立声表+组合窗验收）整体并入 §12 VW6 候选，原文保留本节备查，不算本期承诺。

---

## 14. L2 绑定缺口填补分期（video/ffmpeg Go 包装逐模块补全 · 硬）

## 14. L2 绑定缺口填补分期（video/ffmpeg Go 包装逐模块补全 · 硬）

> 本节是可直接执行的工作单：新会话从头读到尾就能开工，一轮一轮往下走，每轮结束自动提交一次（含本表状态格更新）。
>
> ### 14.0 开工环境
>
> - 仓库：/home/yanghy/app/projects/gogpu/gpui，分支 feat/ime-x11；C 头在 /home/yanghy/app/projects/gogpu/ffmpeg（libavutil/libavformat/libavfilter/libavcodec，只读不改）。
> - 背景：L1 符号存在已全（1002 个绑定名 base/full 双过 + 16 数据符号 + 4 变参包装）。L2 可调用从约 150 个起步往上填。
> - 起手三件事：`git status --short`（认清别线改动，绝不动）→ `git log --oneline -3`（确认基线）→ 跑一遍 §14.3 缺口脚本（确认本轮起点数）。
>
> ### 14.1 轮次表（维护表 · 硬 · 每填完一轮把状态格改成 🟩 已完成 + 落点提交号）
>
> | 轮次 | 模块 | 缺口 | 难度 | 打法 / 落点 | 探针名 | 状态 |
> |------|------|------|------|-------------|--------|------|
> | L2-0 | 三层验证基线 | — | — | da78fef4：L1 新建 symbol_cover_test.go + wrap_cover_test 加 5 batch；修 ChannelFromString（回指针改回 int32）、ChannelLayoutCheck（ret<0 改 ret==0） | TestWrapCover/TestWrapCoverMath | 🟩 已完成 |
> | L2-1 | error_log（11）+ media_desc（85） | 96 | 中 | ce472837：修 18 处签名错位 + 注释同步 | TestWrapCoverErrorLog/TestWrapCoverMediaDesc | 🟩 已完成 |
> | L2-2 | format_demux（80） | 80 | 中 | faf04504：修 12 处签名错位 + decode.go 加 RawFormatCtx 只读口 | TestWrapCoverDemux/TestWrapCoverDemuxFree | 🟩 已完成 |
> | L2-3 | filter_graph（55） | 55 | 中 | 859b0689：修 8 处签名错位（buffer->scale->sink 真链搭法见 feat_setup_test.go openFeatChain） | TestWrapCoverFilter | 🟩 已完成 |
> | L2-4 | codec_encode（47） | 47 | 中 | ef6a958d：修 11 处签名错位 + decode.go 加 CodecID/IsOpen/CodecCtx 只读口；剩 3 个（字幕编解码、硬解参数）要真字幕上下文/真硬解设备，见 L2-13 | TestWrapCoverCodec | 🟩 已完成 |
> | L2-5 | frame（15）+ packet（15） | 30 | 最容易 | 签名全对得上零修改，只加 TestWrapCoverFramePacket（真帧真包 + 空槽数组边数据 + 字典打包解包一轮游） | TestWrapCoverFramePacket | 🟩 已完成 |
> | L2-6 | buffer_mem（36） | 36 | 容易 | 7d56a54d：36 签名全对零修改，只修注释（Realloc/Free 谁吃旧块、InitForBuffer 被 Finalize 吃掉外部、BPrint 回调跳板签名）+ TestWrapCoverBufferMem 清零 | TestWrapCoverBufferMem | 🟩 已完成 |
> | L2-7 | device_io（7）+ decode（1） | 8 | 容易 | 6daed3cb：修 3 处签名错位（ListDevices 3 参改 2 参回设备数，ListInputSources/ListOutputSinks 2 参补 4 参回设备数）+ TestWrapCoverDeviceIO 清零 | TestWrapCoverDeviceIO | 🟩 已完成 |
> | L2-8 | dict_opt（36） | 36 | 中 | 46b3378b：修 4 处签名错位（FieldPtr/ChildClassIterate 改传 AVClass 表，Serialize 补 2 个分隔符，Find/Find2 unit 空串改指针）+ SetDict/SetDict2 标消费语义 + TestWrapCoverDictOpt 清零 | TestWrapCoverDictOpt | 🟩 已完成 |
> | L2-9 | scale_color（33）+ resample_audio（27） | 60 | 中 | 236251e1：修 6 处签名错位（ImageCheckSize2 补 log_offset，BufferSize 传参顺序，Alloc/BytesPerSample 枚举改 int32，BuildMatrix2 步长/编码改值类型，Space/读写回采样数）+ TestWrapCoverScaleResample 清 56 缺口 | TestWrapCoverScaleResample | 🟩 已完成 |
> | L2-10 | crypto 查表算术批 | 约60 | 容易 | 226bf2e9：修 31 处签名错位（AVInteger 按值传扁平 L0-L7、大整数/有理数/对数回 int32、枚举改 int32、DispositionFromString 改 string 回 int32、FindDefaultStreamIndex 回 int32）+ TestWrapCoverCryptoTables 清 60 缺口 | TestWrapCoverCryptoTables | 🟩 已完成 |
> | L2-11 | crypto 字符串内存批 | 约60 | 容易 | 5429afcd：修 20 处签名错位（回0/1的是非判断非出错：Match/Strstart/Stristart/FilenameNumberTest/FindInfoTag/Sscanf/Strcasecmp/Strncasecmp 改回 int32，Base64Decode 回字节数 int32，Calloc 改 nmemb/size uintptr，GetFrameFilename2/FileMap 改 error，Assert0Fpu/Bprintf/Log 改无回值，Strdup 改名 s，Timegm 回 int64，Utf8Decode 回 int32）+ TestWrapCoverCryptoStr 清 56 缺口 | TestWrapCoverCryptoStr | 🟩 已完成 |
> | L2-12 | crypto 哈希加密批 | 约150 | 中 | 59744628：修 3 处签名错位（HashGetSize 回字节数 int32、HmacAlloc 类型改枚举 int32、BlowfishInit 注明 key_len 是字节数）+ TestWrapCoverCryptoHash 清 82 缺口 | TestWrapCoverCryptoHash | 🟩 已完成 |
> | L2-13 | crypto 硬件批 + codec 剩 3 个 | 约120 | 难 | cd41c765：修 5 处签名错位（HwdeviceCtxAlloc/HwframeCtxCreateDerived/HwframeTransferGetFormats/VdpauBindContext/AvcodecGetHwFramesParameters 枚举改 int32，VdpauBindContext 改 error）+ TestWrapCoverCryptoHw 清 32 缺口 | TestWrapCoverCryptoHw | 🟩 已完成 |
> | L2-14 | 图像采样批（22） | 22 | 容易 | 9f2c809d：修 22 处签名错位（Read/WriteImageLine 行宽 int32 改指针数组，Samples 6 个行宽改 *int32、采样格式改枚举 int32，Image 像素格式改枚举 int32、行宽尺寸改指针、CopyPlaneUcFrom 行宽改 uintptr、FillMaxPixsteps 改无回值、FillColor 颜色改指针）+ TestWrapCoverImageSamples 清 22 缺口；另删 L2-13 探针 AvcodecEncodeSubtitle 野调用（full 新 so 下约 1/6 崩，编码真路延 L2-17） | TestWrapCoverImageSamples | 🟩 已完成 |
> | L2-15 | 变换解析批（32） | 32 | 中 | 9a8e2910：修 22 处签名错位（变换计算释放改无回值，Dct/Rdft/Tx类型改枚举int32，Expr计数器改*uint32、表达式改Go string，Lzo改int32+长度*int32，Vorbis/Ac3/Adts改回值+类型化槽）+ TestWrapCoverXformParse 清 32 缺口（FFT/DCT/MDCT/RDFT冲激向量+TX新路+表达式11/Vorbis真包576，全C实测；另钉inverse=0不建反变换路、RDFT类型2/3回nil） | TestWrapCoverXformParse | 🟩 已完成 |
> | L2-16 | 边数据批（36） | 36 | 中 | afd76a12：修 18 处签名错位（size_t槽改*uintptr，边数据/编码器/IAMF参数类型改枚举int32，StreamAddSideData改int32，StreamGetCodecTimebase改按值回AVRational）+ TestWrapCoverSideData 清 36 缺口（Cpb40/检测框1040/Vivid1636/加密包37与34/参数144/提示80/真帧挂边数据/真流时基0/1轮转，全C实测；钉Downmix与Clone传nil崩、base无复用器延full） | TestWrapCoverSideData | 🟩 已完成 |
> | L2-17 | 复用读写批（28） | 28 | 中 | 8ca5bd72：修 10 处签名错位（FindInputFormat 短名改 Go string，FmtCtxGetDurationEstimationMethod 改回 int32 枚举，InitPacket 改无回值，ProbeInputBuffer/2 的 url 改 Go string，ProbeInputFormat2/3 分槽改 *int32，SetOptionsString 三串改 Go string，GetOutputTimestamp 两槽改 *int64，TransferInternalStreamTimingInfo 改 int32 回 int32）+ TestWrapCoverMuxRW 清 28 缺口（三迭代 Demux182/Filter44/Mux180 + mp4 真盒真流探测节目包帧暂停 SDP + full 真写路裸帧帧归 C 脱钩 + WriteTrailer 真收尾出片，全 C 实测；钉 ProbeInputFormat2 等分回空按设计、裸盒 WriteTrailer 会崩走真路、ENOSYS 裸帧 C 照吃 Go 侧脱钩） | TestWrapCoverMuxRW | 🟩 已完成 |
> | L2-18 | 线程树设备批（42） | 42 | 难 | 408a6e22：修 7 处签名错位（BmgGet 输出槽改 *float64，ChromaLocationFromName 改回枚举加 error，DefaultGetCategory 改回 int32，JniSetJavaVm 改回 error，ParseCpuCaps 字串改 Go string，SwriAudioConvertAlloc 两格式改枚举 int32，ThreadMessageQueueNbElems 改回条数 int32）+ TestWrapCoverThreadMisc 真路 42 件（VDPAU/BMG/CRC/分类名/DV/SAD/包打双日志/采样直拷/消息队列发收/执行器同步跑任务/手搭 ResampleContext 走 DSP 双表/手造 24 字节 va_list 走 Vbprintf（断言回写内容）与 Vlog，全 C 实测；传 Go nil 进 purego 在 trampoline 里即崩，实测为凭；缺口归零） | TestWrapCoverThreadMisc | 🟩 已完成 |
>
> ### 14.2 每轮标准动作（S1–S7，一轮走完才算完）
>
> - S1 拉缺口：跑 §14.3 脚本，记下本模块起点数。
> - S2 查头文件：逐个缺口函数对照 ffmpeg 头文件定 C 语义（参数类型、返回值含义、NULL 能不能传），先写在纸上再动代码，禁止凭印象写断言。
> - S3 影响面评估：`grep -rn "\.函数名(" video/ examples/` 查外部调用者，无调用者才能改签名；有调用者先停下问用户。
> - S4 修签名 + 注释：类型错位直接改（外面没人调就是安全的）；注释同步写清（收指针的不传 NULL、要真对象还是零值能用、谁拥有释放权）。
> - S5 加探针：新测试函数名见上表，一次点全名；断言答案必须能在 C 源码里找到出处；崩溃类只验守卫不走真路，真路走真对象。
> - S6 跑验证：§14.4 四条命令全绿（base 整包 + full 子集，一个 FAIL 都不行）。
> - S7 提交：`git status` 对照，只 `git add` 本轮自己文件（禁止 `add .`/`add -A`/`commit -am`），提交信息格式 `video/ffmpeg L2补全<模块>：修N处签名错位，加<探针名>探针清M缺口`，本表状态格改 🟩 + 落点提交号，**代码和状态格同一次提交**。
>
> ### 14.3 缺口脚本（现拉，不凭印象）
>
> ```
> cd /home/yanghy/app/projects/gogpu/gpui && python3 - <<'EOF'
> import re, glob, os
> wrap_re = re.compile(r'^func (?:\([^)]+\) )?([A-Z][A-Za-z0-9]+)\s*\(')
> wrappers = {}
> for f in glob.glob('video/ffmpeg/*.go'):
>     if f.endswith('_test.go'): continue
>     for line in open(f):
>         m = wrap_re.match(line)
>         if m: wrappers.setdefault(m.group(1), os.path.basename(f))
> used = set()
> for f in glob.glob('video/ffmpeg/*_test.go'):
>     for m in re.finditer(r'[A-Za-z][A-Za-z0-9]+\(', open(f).read()):
>         used.add(m.group(0)[:-1])
> gap = {n: mod for n, mod in wrappers.items() if n not in used}
> print(f"wrappers={len(wrappers)} gap={len(gap)}")
> from collections import Counter
> for mod, n in Counter(gap.values()).most_common():
>     print(f"{n:4d} {mod}")
> EOF
> ```
>
> ### 14.4 四条验证（gpui 目录下跑，shell 经常停在 ffmpeg 仓库，go test 前一定先 cd；长命令套 timeout 防卡死，卡住直接杀了查行号再改）
>
> ```
> cd /home/yanghy/app/projects/gogpu/gpui
> go vet ./video/ffmpeg/
> timeout 100 go test ./video/ffmpeg/ -run 'TestWrapCover|TestSymbolCover|TestDataConst|TestVariadicGo' -count=1
> timeout 300 go test ./video/ffmpeg/ -count=1
> timeout 200 env GPUI_FFMPEG_VARIANT=full go test ./video/ffmpeg/ -run 'TestWrapCover|TestSymbolCover' -count=1
> ```
>
> ### 14.5 已知坑位（前人踩过，新会话先读再动手，注释里同步写清）
>
> - 往只读 AVIO 流写字会在 C 的 flush_buffer 里转圈出不来（卡 40 分钟）：写类探针只许调在写流上，读流只验读和状态。
> - C 里直接解指针的函数一律不传 NULL：Handshake（解 opaque）、ProtocolGetClass/PrintStringArray（strcmp/解数组）、二次 FindStreamInfo（内部状态）、二次 avcodec_open2（状态机）、GetHwConfig/DefaultGetFormat/字幕编解码（直接解 ctx）。野路不走，真路给真对象，守卫只验 nil 接收器。
> - BSF 链吃掉过滤器所有权：Append 进去的壳别再 Free，不然 double free。
> - 小包描述表是 NULL_IF_CONFIG_SMALL 裁过的：档次表不在里面，只验越界回空。
> - base 版无复用器：GuessFormat 回空符合预期，真断言只在 full 版跑；mp4 默认视频编码无 x264 版是 MPEG4（12），见 movenc.c。
> - 顺时针设 90 度读出来是 -90（读的是逆时针角）；ParseRatio 的 max 传 0 会压成 0/1，传 1001000；`go vet` 报 `possible misuse of unsafe.Pointer`（如整数转指针）必须改掉。
> - 调不通的（要真硬件/真上下文）用 `t.Skipf` 注明原因跳过，禁止静默假绿；`t.TempDir()` 放临时文件，禁止写死绝对路径。
