# 视频模块立项 — ffmpeg（含硬解）+ render 经 hal 直传 + kit 视频组件（v2.1 收敛）

> **版本：v2.1 收敛** | 日期：2026-09-30 | **状态：新架构唯一真源，旧版作废**
> **性质：** `gpui` 里的视频整机立项：解码 + 上墙 + 播放组件一次做完。需求、计划、验收只认这一份。
> **范围：** 解析处理全走 ffmpeg（含硬解，软解只做回落）；上墙走 `render` 新增的视频直传路，经 `hal` 通用口调显卡，应用配哪套后端视频就跟哪套；播放控件做成 `ui/kit video_*` 组件。
> **一句话链路：** `video`（ffmpeg 解出 RGBA）→ `render`（视频纹理直传 + 缩放合成）→ `hal`（通用显卡口）→ `webgpu` / `gwgpu/gles`（两套实现，调用方不点名）。
> **旧版处理：** v1.x（含 VW0–VW3、VR0–VR9、§11–§14）整体作废，只留一条：历史实现与数据以 git 历史为准（`git log -- docs/ENGINE_VIDEO_DECODE_PLAN.md` 可查）。现有 `video/`、`video/ffmpeg/decode.go`、`video/backend_ffmpeg.go`、各 `examples/video/video_*` 桥接代码允许按本架构推翻重写，不保结构兼容，只保对外行为（打开、取帧、跳进度、关闭、状态事件）语义不变。
> **并读：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)（`ui → render → gpu`）· [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)（禁止 CGO）· [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) · [`UI_PIXEL_ASSERTION_STANDARD.md`](./UI_PIXEL_ASSERTION_STANDARD.md)（画面断言三证据）· [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md)（真窗纪律）。

---

## 0. 用户要求（固化 · 不可删改）

| # | 要求 |
|---|------|
| V-U1 | 没有旧自研：解析处理全走 ffmpeg，不保留、不新增 Go 自研解码、拆盒、转色、重采样。 |
| V-U2 | 硬解就在本期：解码默认走硬解，硬解不可用才回落软解，不留“以后再做硬解”。 |
| V-U3 | ffmpeg 后端：`video/ffmpeg` 经 purego 绑定自建 `libgpui_ffmpeg`（ffmpeg 7.1.5，LGPL 2.1，base/full 两变体，门禁走 base，高级滤镜/转码随 full）；禁止 `import "C"`，`CGO_ENABLED=0` 必须能编译。 |
| V-U4 | `video` 是干净的能力层：只出帧（宽高 + RGBA + 时间戳），不引用 `ui` / `render` / `gpu`（合规单测锁保持）；不出显卡句柄、显存指针、显卡纹理。 |
| V-U5 | 上墙只认 `render`：消费侧（真窗、kit 组件）不直调 `gpu` 包。`render` 内部经 `hal` 通用口调显卡，允许 `render` 按需给 `hal` 加可选能力口，两套后端同步实现。 |
| V-U6 | 后端自动跟随：视频不另建显卡设备，只借当前窗口那套设备；应用配 WebGPU 那套就跟 WebGPU，配纯 Go GL 那套就跟 GL。分支只看通用能力，不写后端名字。 |
| V-U7 | 大片一次达标：2K、4K 在本期直接过门禁，不分“先小片、后大片”。 |
| V-U8 | 做成组件：`ui/kit video_*` 是拿来即用的播放控件，包住 `video` 能力，画画只调 `prim` / `render` 门面。 |
| V-U9 | 每个主能力有独立真窗验收，单测不能单独标完成；可见图形必须逻辑探针 + 像素断言 + Golden 三证据，缺一个不算绿。 |
| V-U10 | 现有代码可推翻：为达到上面目标，`video` 与桥接层允许重写；`render` 允许新增视频直传文件；`hal` 只允许加可选口，不改现有口。 |

**术语（大白话）：**

| 词 | 意思 |
|---|------|
| 硬解 | 用显卡或芯片自带的解码器解视频，又快又省 CPU；解完的帧先拿回内存再上墙，本期就这么走。 |
| 软解 | 用 CPU 跑 ffmpeg 解码，只当硬解不可用时的回落。 |
| 直传 | 视频帧不进通用图片缓存，有自己独立的显卡纹理，每帧原地重写，不重建。 |
| 跟随 | 视频不自己选显卡，用窗口已经建好的那套，窗口是哪套视频就是哪套。 |
| 回落 | 直传或硬解走不通时，自动降到老路（通用 `DrawImage`、软解），保证有画面，同时记一笔数。 |

---

## 1. 目标与非目标

### 1.1 做完时长啥样

```text
打开一个 MP4（手机拍、2K、4K 都行）
  → ffmpeg 用硬解解出画面，硬解不行自动换软解，帧拿回内存转成 RGBA
  → Player 只做队列、时钟、跳进度编排（Ids/Stats/事件齐备）
  → render 视频直传：同一路视频同一块显卡纹理，每帧原地重写
  → 显卡按显示矩形缩放合成（解码分辨率不动），没新帧不重传
  → kit 视频组件包住上面全部，业务方只调 BuildVideo
  → 能播、能停、能跳、能变速静音循环，坏文件报人话错不崩
```

声音已有通路本期不断线：有声片照旧出声（音量/静音/变速透传），无声片一帧不少播。

### 1.2 非目标（本期明确不做）

- 新编码（H.265 / VP9 / AV1）、新盒子（MKV/WebM）、网络自适应码流：走 ffmpeg 能力复用，不在本期立门禁，真有片子只验“能开能报”，不按主能力验收。
- 外挂字幕渲染：`video/ffmpeg` 的滤镜能力保留，组件只留开关占位。烧录字幕（滤镜烧进画面）属于 ffmpeg 转码能力，随高级版库提供，不在组件验收。
- 硬解帧零拷贝直贴（显卡内存直接当纹理用，不经内存）：本期只做“硬解 + 拿回内存 + 直传”，零拷贝只在 `hal` 留可选口占位，不验收。
- 倒放：不支持，真倒解不存在，接口也不预留。
- 音频现状：`video` 已有声音通路（`video/ffmpeg/audio_decode.go` 解出 48kHz 立体声 float PCM + `video/sync_audio.go` 音画同步，主钟声领），本期保留不断线；组件只接音量/静音/变速透传，不新立声音门禁。

---

## 2. 架构总览（分层铁律 · 硬）

```text
video/                    只管出帧：ffmpeg（硬解优先）→ RGBA → 队列/时钟/跳进度
  │ 不引用 ui/render/gpu；不出显卡句柄；线程：ffmpeg 调用只在播放器后台解码线程（音视频各走自己的打开）
  ▼ Player 公开口：OpenFile / OpenWithSource / Info / Poll / SeekTo / SeekFast / Pause / SetRate / Stats / Close
  音频口（老口不清零）：PollAudio / SetVolume+Volume / SetMuted+Muted
render/                   唯一能碰显卡的地方：视频直传纹理 + 上传 + 缩放合成 + Present
  │ 只认 hal.xxx + types.xxx + gpucontext 桥；创建那一句才选后端，之后无感知
  ▼ hal.Device / hal.Queue / hal.Texture（通用口）
gpu/webgpu ─┐ 同步实现 hal，后端二选一，调用方不点名
gpu/gwgpu/* ─┘ 第一扇窗建设备，后面窗口共用（AcquireSharedPresentDevice 那套）
ui/kit video_*            组件层：调 video 拿帧，调 render 画画，不碰 gpu 包
```

| 层 | 能干 | 禁止 |
|---|---|---|
| `video`（含 `video/ffmpeg`、`video/clock`） | demux、硬解软解、sws 转 RGBA、队列时钟跳进度、封顶分类 | 引用 `ui` / `render` / `gpu`；出显卡句柄；自己建显卡设备 |
| `render`（含新增视频直传） | 建视频纹理、写纹理、画四边形、查当前后端、回落通用路 | 出现 `webgpu.Xxx` / `gwgpu` / `gles.` 具体类型（创建入口与注册行除外，合入前 grep 必查） |
| `hal` | 通用显卡口（建纹理、写纹理、画、查能力） | 加视频专用强依赖口；本期只允许加可选能力口 |
| `ui/kit video_*` | 播控状态机、画幅矩形、浮层排版、`BuildVideo` | 直引 `gpu` 包；直调 `video/ffmpeg` 符号；自己算时间戳 |

**验证口径（硬）**：`examples/video/*` 的桥接调用只算**原子验证**；生产接线只认 `ui/kit video_*` 组件或 2.5D/模块功能包真正调用（AGENTS.md 生产/原子验证口径）。视频直传 API 在组件未接前保持「已验证待接线」，不得拿真窗桥接当生产证据。

线程归属：ffmpeg 调用只在播放器后台解码线程（音视频各走自己的 ffmpeg 打开，互不串行阻塞）；显卡调用只在光栅线程；两边经有界队列交接，显示侧丢旧保时间线并计数，不堆内存。

---

## 3. video 侧：ffmpeg（含硬解）方案

现状：`video/ffmpeg` 的硬解绑定已齐（`HWDevice`：建设备、建帧上下文、拿缓冲、回传；vaapi 真机探针已通），但 `video/ffmpeg/decode.go` 的 `Next` / `convertFrame` 还只走软解。这是本期要推翻重写的部分。

### 3.0 五层口径（A 方案 · 硬，跟不上只慢播不报错）

跟不上的片不报错：快片（60 帧以上、大 4K）按时间线丢旧帧并记数，慢机器只慢播。独显（本机 940MX）能算不能贴，画面上墙只走核显，独显路走不通老实回落。

1. **video 解码层**：打开时把解码线程数打开（现单线程）；出帧从 RGBA 整块改成 NV12 两小块，硬解本来就是这个格式直接拿，软解只做一遍交叉排，不算数。
2. **video 内存池**：缓冲按两小块尺寸建，一帧 2K 从 14MB 降到 5MB 多，池子个数和封顶不变。
3. **render 直传层**：加平面纹理槽和平面上传，画画口加 YUV 转色入口，转色搬进显卡算；老整块口留着当回落，不认小块格式就走老路。
4. **两套显卡后端**：各加一个 YUV 转色画画程序，对称实现，同一套颜色数；丢帧按时间线记数，不静默。
5. **窗口和节点**：17 扇窗取帧换传参（整块改两小块），单测改对拍（同帧新老路误差写死，超了红）。

### 3.1 硬解流程（本期实现 · 硬）

```text
打开：按平台列出硬解类型 → 建硬解设备（建不起来就记一笔，直接走软解，不崩）
解码：解码器优先绑硬解 → 取帧时看是不是硬解帧
      → 是：经 av_hwframe_transfer_data 拿回内存 → sws 转 RGBA（走现有转色路）
      → 否：原软解路
搜进度：跳后清解码器状态（Flush），硬解上下文不清零复用，落地戳照旧返回
关闭：解码器、设备、帧池按序释放，不漏
```

平台顺序（一家一家过，Linux 先行）：Linux 先 VA-API，真机有卡再加 NVDEC；Windows 按 D3D11VA / D3D12VA / QSV / NVDEC 顺序；macOS 走 VideoToolbox；安卓走 MediaCodec。每家独立门禁，不一次全押。

### 3.2 状态与统计（必须如实）

`Info` / `Stats` 新增四键（只加键，不改旧键语义）：`hw_active`（本路是否硬解）、`hw_name`（ffmpeg 硬解类型名，如 vaapi / nvdec / d3d11va / videotoolbox / mediacodec，软解记 soft）、`hw_fallbacks`（累计回落次数）、`hw_transfer_ms_avg`（硬解回传耗时均值）。硬解失败必须记 `hw_fallbacks` + 进容错分类（`video/fault_classify.go` 新增 `KindHwFallback` 桶），不静默。解码器谈判失败（只要真硬解上下文，比如 vaapi 进了本构建，谈崩）不算打开失败，自动转软解继续播；只有软解也起不来才报坏片。

### 3.3 队列时钟跳进度与内存（沿用 + 收紧）

- 队列有界（默认 4，`video/clock.DefaultCap`，`Options.QueueCap` 可调），Push 满则阻塞背压解码线程、显示侧 `PollDue` 丢旧保时间线并计 `Dropped`；时钟按时间戳显示，跟不上丢旧保时间线；跳进度语义不变（请求式，第二次跳覆盖第一次）。
- 内存封顶按档写死：480p 128MB、720p 256MB、1080p 512MB、1440p（含 2K）1024MB、4K 2048MB；解前按宽高乘引用数加队列预估，超了秒报装不下。
- 帧缓冲走池复用（YUV / RGBA / 工作区），稳态不新开大内存；尺寸变了才换池。

### 3.4 落点文件（允许重写）

- 重写：`video/ffmpeg/decode.go`（硬解分支 + 回传 + 统计）、`video/backend_ffmpeg.go`（设备选择 + 回落接线 + 池子接线）。
- 保留只改：`video/player_types.go`（加四个状态键）、`video/player_open.go`、`video/player_playback.go`、`video/player_seek.go`、`video/mem_pool.go`、`video/io_source.go`、`video/clock/*`、`video/registry_probe.go`、`video/fault_classify.go`。
- 单测：硬解门禁只认真路（真设备建得起来才断言硬解帧，建不起来走软解回落断言 + 跳过注明原因，禁止静默假绿）。

---

## 4. render 侧：hal 统一视频直传（最优路 · 硬）

目标：大片（2K/4K）每帧只付一次整块上传 + 一次显卡缩放，不付逐行循环、不付通用缓存淘汰、不付每帧重建纹理。

### 4.1 一帧的旅程（A 方案：NV12 平面 + 显卡转色）

1. ffmpeg 后台线程解出 NV12（Y 一块 + UV 交织一块），进池复用缓冲；`video` 只出帧，不包图不碰显卡。
2. 消费侧在光栅线程直接传平面：两小块各一次整块上传（Y 用 R8，UV 用 RG8），宽高步长一次给对。
3. 预乘与转色都跳过 CPU：视频帧不透明，显卡程序里按 BT.601/709 一次算出 RGBA，不再逐像素重算。
4. 上传原地重写：同一路视频永远同一组平面纹理（`CreateTexture` 一次，`WriteTexture` 每帧重写，`TexSubImage2D` 语义那条路优先）。
5. 画图显卡缩放：解码分辨率不动，显示大小由目标矩形定，走贴图四边形。
6. 没新帧不传：沿用 `FrameDamage` + `PresentFrameDamage` / `PresentFrameDamageRects` 脏区机制，没新帧不重传、不重提脏区，复用上次纹理。
7. 同源多尺寸共用一组纹理：大窗小窗只是目标矩形不同。
8. 回落：平面格式不认、创建/写入失败、无设备，一律回老整块口，再不行回通用 `DrawImage`，保证有画面并记回退数。

### 4.2 新增口（只列形状，实现时再定签名细节）

- 查当前后端：把当前 `hal.Device` / `hal.Queue` 借出来（只借不拥有，不接管释放），附带通用能力查询（命令拷贝、纹理上限、行对齐）；后端名字只做诊断展示，分支只看能力。
- 视频纹理句柄：A 方案一组两块（Y 用 R8、UV 用 RG8；宽高、纹理、视图各一），尺寸变了才重建；关窗释放。
- 画视频帧：A 方案输入 NV12 两平面 + 目标矩形 + 透明度，内部走平面直传纹理 + 显卡转色；失败回落老整块口，再不行回通用 `DrawImage`，并记回退数。

### 4.3 视频缓存单列

通用图片缓存 64MB 不变，视频纹理另建独立池，不参与界面图的淘汰。A 方案一组两槽（Y + UV），健康态视频槽淘汰数为零，淘汰即红（以采集到的淘汰计数为准）。4K 窗用完整设备档，不用低显存紧缩档。平面一帧多大：1080p 约 3MB，2K 约 5MB 多，4K 约 12MB 多（RGBA 整块的约四成）。

### 4.4 后端自动跟随（硬）

- 视频自己不建设备，只借窗口那套（`AcquireSharedPresentDevice` 那套共享语义：第一扇窗建，后面共用）。
- 创建口只有一句（`ResolveBackend` 读 `GPUI_BACKEND` → `SelectBackend` 建实例，`newPresentSwapchain` 同理），之后全走 `hal` 无感知。
- 分支只看通用能力（支不支持命令拷贝、纹理尺寸上限、行对齐要求），不写后端名字；以后加新后端，`hal` 注册进来就能用，视频路不用改。

### 4.5 失败回落

纹理创建失败、写入失败、无设备，一律回落通用 `DrawImage` 路，保证有画面，同时记 `cpu_fallback_ops`。回退暴涨即红。

### 4.6 未关账登记（2026-10-23 实测，一条现象一条位置，修完勾）

| # | 现象（实测） | 位置 | 归属 | 待办 |
|---|---|---|---|---|
| V-1 | **retained 窗上视频整块留白**：同一节点同一帧，实况走查（FullPaint 直绘）能出画面（CPU 收敛取样点 red=32896，清空色 65535），UI 建包出的 retained 封包在该节点矩形上 **0 个可绘制图层**。根因是 retained 建层走查全程不调 `node.Paint()`，视频叶子在层树里没有任何表示 | `ui/embedder/pipeline_app.go:1894-1925`（draw 闭包只 `RasterizeDirty` + `CompositeFramePacketTextured`）、`ui/rendering/video_direct_node.go:165`（`Paint` 无人调）、`ui/rendering/layer_build.go:578`（建层不询问节点）；复现 `ui/rendering/layer_build_video_retained_test.go`（C4 已从 SKIP 改回 `t.Fatalf`） | 线程线 `ENGINE_THREAD_T_PLAN.md` §18.1 #17 / S2b **C4**（给 `RenderVideo` 建 `scene.TextureLayer`，语义照 Flutter `TextureLayer`：不进纹理缓存、每帧必出损伤子树） | C4 收口：三扇真窗（`video_player` / `video_4k_play` / `video_vr8_embed`）加像素断言（首帧 + 换帧两次采样不同），复现测试从 SKIP 改回 `t.Fatalf`。**2026-10-24 进展**：C4 代码已落（`scene.TextureLayer` + `render.TextureSource` registry + `VideoBridge.PaintTexture/ID/PaintStats`），复现测试已改回 `t.Fatalf` 并通过，三窗已加 retained 封包两帧像素断言。**2026-10-24 三窗实跑（非 -race，功能口径）**：retained 出画全绿——`video_player` 墨 1.00/1.00 动 0.10（桥 Frames 62/62，显示 62）；`video_4k_play` 主/小窗墨 1.00/1.00 动 0.0425/0.0900（纹理出画峰值 2）；`video_vr8_embed` 主/小窗墨 1.00/1.00 动 0.09125/0.1275（纹理出画峰值 2，embed/resize/last_var 已绿）。**V-1 的 retained 出画已被三窗证明，但 C4 整体不标关**：4K p95 23.021ms 超 22ms（VR8 21.846ms 已过），三窗 `-race` 在 #18 根修复后不再 abort 但性能门禁红，视频线仍无 Golden（retained 只断墨/动；色度 0 偏绿的根因已由 V-4 修复，真 GPU 像素测试已绿，三窗已复跑、retained 快照彩条正常）。**2026-10-24 晚补**：4K p95 已由异步双缓冲上传降到 **19.37ms**（fps 58.92、uploads=frames=107、fallbacks=0、retained 墨 1.00/1.00），渲染侧渲染门已过；C4 仍不标关只剩视频 Golden 与 4K 解码吞吐（V-5，真窗实际只有 ~3.5 帧/s 可传） |
| V-2 | **节点喂帧口没有回落**：`RenderVideo.UploadPlanes` 只要池路不收这帧就直接丢帧（节点连 `hasFrame` 都不置，Paint 无画、回落计数不加）。实测 4 种形状：RGBA 行对齐 64×32 + 有设备 = 收；RGBA 非对齐 32×16（行 128 不满足 256 对齐）+ 有设备 = **丢**；RGBA 64×32 但桥还没挂上设备（`NewVideoBridge(nil)`，各窗要等 ticker 第 30 拍才 `EnsureDevice`）= **丢**；NV12 奇数 33×17 = 丢（`UploadPlanesFrame` 要求宽高皆偶）。同一帧走桥老口 `Show` 则正常收敛（`ok=true fallbacks=1`） | `render/video_bridge.go:167-197`（`UploadFrame` 池 nil / `videoBytesPerRow` 不对齐即 false）、`render/video_bridge.go:318-356`（`UploadPlanesFrame` 偶数门）、`ui/rendering/video_direct_node.go:66-92` | 视频线本线 | §4.1 第 8 条的回落纪律落到节点口：池路不收时走 `Show` 那条 CPU 回落并计数；本文件 v2.6 写的「奇数/异形帧走桥老口兜底记数」实测**在节点口不成立**，随本条一并订正。**2026-10-24**：C4 未动节点喂帧口，本条继续开放。**同日实跑订正**：`video_player` 首次 retained 失败（桥 Frames 0、显示 2）根因不是本条——该窗按需出帧，ticker 在短片段结束后就空闲，永远到不了第 30 拍 `EnsureDevice`，桥从未挂上设备；已把示例改为每拍借设备直到首帧（`examples/video/video_player/main.go`），复跑桥 Frames 62、retained 绿。**2026-10-24 已修（工作区未提交，单测）**：`VideoBridge.UploadFrame` 池 nil / 行距不对齐 / 上传失败时不再返回丢帧，而是把 RGBA 拷进 CPU shadow、置 `hasFrame` 并返回 true；`drawCurrentAt` 去掉 `slot == nil` 的早退，无 slot 或 `DrawVideoSlot` 失败都走 `DrawVideoFrame` 的 CPU 回落并计一次 fallback；切形状时释放旧 slot（RGBA↔NV12 不画旧帧）。节点 `RenderVideo.UploadPlanes` 的平面路失败（奇数/无平面设备/上传错误）改为落到 RGBA pix，不再直接 false。单测：`TestVideoBridgeMisalignedRGBAStagesCPUShadow`（32×16 有设备、行 128 不对齐 → frames=1 uploads=0 fallbacks=1）、`TestRenderVideoUploadPlanes_RGBAShapeRoutes`（无设备 → true + fallback）、`TestRenderVideoUploadPlanes_OddPlanesFallBackToRGBA`（33×17 奇数 → RGBA 回落）全绿。**边界**：生产三窗喂的是对齐 NV12，V-2 回落形状只有单测、无真窗；C4 不因此标关。**存量红（HEAD `e00e6dbc` 同红，非 V-2 引入）**：`TestVideoBridgeFallbackConverged`/`TestVideoBridgePlanesFallback` 单独跑报 `Bridge fallback left no damage`/`planes fallback left no damage`；本次只修丢帧与回落计数，不把这两个存量红算绿。 |
| V-3 | **跨设备槽失败开放**：槽纹理建在 A 设备、画的却是 B 设备的会话时，`DrawVideoSlot` 照样返回 true、桥记一次 direct 零回落，像素却一个不落（实测取样点仍是清空色 65535，`ctxGPU=true` 证明确有活 GPU 会话）。生产里桥借的是窗口同一共享设备，暂时碰不到；缺的是「槽设备 == 绘制会话设备」这道校验 | `render/context_image_gpu.go`（`DrawVideoSlot`/`DrawVideoPlanes` 只判 `IsDeviceReady`，不判 view 归属）；测量：noop 设备槽 + 真 GPU 上下文，`direct=true ok=true px=65535` | 视频线本线（与 #17/S2b **C4** 的 `TextureSource`/registry 同笔修最省） | **已修（2026-10-24，单测）**：`GPURenderContext.Device()` 暴露共享设备；`DrawVideoSlot`/`DrawVideoPlanes` 在 `IsDeviceReady` 后调 `videoDeviceMatches`，归属不符返回 false 并记 `video:device-mismatch`，桥走 CPU 回落并计数；负向单测 `render/video_device_test.go:TestVideoDeviceMatches` 锁定「同设备放行 / 跨设备拒绝 / 未知设备保持旧路」。**真窗负向已跑（2026-10-24，工作区未提交）**：`render/video_v3_window_x11_linux_test.go:TestV3_VideoCrossDeviceRealWindow` 开真 X11 窗，B 为窗口会话设备、A 为第二原生设备，A 上 64×32 槽上传红帧后交给 B 的活会话：`DrawVideoSlot` 返回 false、`LastCPUFallbackReason()="video:device-mismatch"`、`cpu_fb=1`；调用方 CPU 回落经真窗读回像素 `(255,0,0)`，`presents=1`。生产桥仍借同一共享设备，跨设备是负向契约，不改变生产路径。 |
| V-4 | **视频色度读成 0，画面偏绿**：同一帧 BT.601 limited-range 纯红 NV12（`Y=76,U=84,V=255`）走 `DrawVideoPlanes`，真 GPU 读回中心像素 `(0,224,0)`，期望 `(255,0,0)`；计数 `gpu_ops=1 cpu_fallback_ops=0`，说明 GPU 路真跑了但 UV 没采到。根因：UV 纹理按 `w×h/2` RG8 建，而 NV12 一行 UV 只有 `w` 字节 = `w/2` 个 RG8 texel；归一化采样中心落在零填充的右半边，U/V 都读成 0，按 BT.601 公式正好变成 `(0,224,0)` 绿。 | `render/video_planepool.go`（`Acquire` 的 UV 纹理宽、`UploadPlanes` 的 UV 上传宽）；复现/回归 `render/video_yuv_color_gpu_test.go:TestVideoYUVPlanes_GPUColor`（padded 64×32 + aligned 256×64 两子例） | 视频线本线（C4 出 Golden 的前置门） | **已修（2026-10-24，工作区未提交）**：UV 纹理改为 `w/2×h/2` RG8，`WriteTexture` 的 UV `size.Width` 改为 `w/2`（行距仍按 `w` 字节对齐）；纯红帧两子例都读回 `(255,0,0)`，GPU 路无 CPU 回落。**真窗复跑（2026-10-24，非 -race）**：三扇 retained 快照都已恢复色条——`video_player` 中心像素 `(244,244,81)`（源中心 YUV `210,64,138`，BT.601 limited CPU 约 `(242,243,97)`，不再偏绿）、4K 主/小窗和 VR8 主/小窗均为正常彩条；但真窗仍没有逐像素颜色门禁，视频 Golden 也未出，C4 不标关。 |
| V-5 | **4K 硬解吞吐不足，窗口出帧被解码卡住**：4K 片段独立解码实测 150 帧用 34.26s（**4.4 帧/s**），`decode_avg=230.5ms`、`decode_p95=239.6ms`、`hw=vaapi active`、`hw_fallbacks=0`、转回 NV12 平均 117.1ms；同机 ffmpeg 纯软件 4K 解码约 100ms/帧。4K 真窗因此每秒只交接 ~3.5 帧（30s 共 107 帧），画面靠 retained 纹理层每帧重绘保持 58.9fps——渲染侧已不是瓶颈，出帧瓶颈在硬解通路/回读 | 探针：`video/testdata/p3_4k_3840_2160_30fps.mp4` 独立播放（临时探针已删，可用 `video/gate_perf_decode_test.go` 口径复测）；机器：Intel HD 520 + GeForce 940MX、4 核、7GB | 视频解码线（P1 硬解 / P3-A 线程化解码），非 C4 渲染 | 摸底 vaapi 解码 → CPU 回读 NV12 的 230ms 构成（探针已量 transfer 117.1ms），按线程化/零回读方向优化；预算线不降。本条只登记，本次未改解码行为。 |
| V-6 | **Linux GL 补行上传丢 UV 行距（存量缺陷）**：`WriteTexture` 的 `UNPACK_ROW_LENGTH` 按「只有 R8 是 1 字节/texel、其余 4 字节」算，RG8 实际是 **2 字节/texel**。宽 1920 时 `videoPlanePitch` 补行到 2048：UV 紧行 1920B、算出的 `rowLen=2048/4=512` 不大于 `size.Width=960`，于是不设 `UNPACK_ROW_LENGTH`，`TexSubImage2D` 按紧行读 2048 跨度的缓冲，1080p 在 Linux GL 上 UV 错行；Windows GLES 的 `WriteTexture` 则完全不看 `BytesPerRow`，任何补行宽都会错。wgpu 路显式传 `BytesPerRow`，不受影响（本机视频窗走 wgpu，未真机复现 GL 现象） | `gpu/gwgpu/gles/queue_linux.go:181-190`（`pixelBytes` 只特判 R8）、`gpu/gwgpu/gles/queue.go:170-184`（无行距处理）；对照 `render/video_planepool.go` 的 `videoPlanePitch`/`padPlaneRows` | 渲染/GPU 后端线（视频补行上传） | 在 GL 真窗复现后按 2 字节/texel 修 Linux 的 `rowLen`，Windows 补同样的 `UNPACK_ROW_LENGTH` 处理，并加逐像素回归。本次只登记，不改行为。 |

---

## 5. hal 侧：首版够用，加法预留

已齐（直接复用）：`CreateTexture`、`CreateTextureView`、`WriteTexture`、队列提交，两套后端都有。GL 那套是原地更新语义，大帧友好；WebGPU 那套是标准写纹理路。R8/RG8 两格式两套后端都认（GL 走 R8/RG8，WebGPU 同名格式），A 方案不加新格式。

本期 `hal` 不加必选口，只预留三个可选能力位（后端各自实现，调用方先问能力再走，不点名后端）：持久映射 staging、行对齐查询、硬解句柄直贴占位。首版达标不依赖它们。YUV 转色程序写在两套后端各自的四边形着色器里，对称实现、同一套颜色数，`render` 只调口不碰实现。

本期 `hal` 不加必选口，只预留三个可选能力位（后端各自实现，调用方先问能力再走，不点名后端）：持久映射 staging、行对齐查询、硬解句柄直贴占位。首版达标不依赖它们。

合入前必查（沿用引擎纪律）：`render/` 非创建入口出现具体后端类型即打回。

---

## 6. kit 视频组件（`ui/kit video_*` · 硬）

扁平五件，不建子包：

| 文件 | 放什么 | 不放什么 |
|---|---|---|
| `video_props.go` | 片源（文件/内存/URL）、自动播、循环、静音、音量、变速、目标矩形模式（充满/包含/拉伸）、封面、字幕开关占位 | 解码细节、ffmpeg 符号 |
| `video_state.go` | 播/停/缓冲中/播完/出错状态机 + 队列水位/时钟漂移透出（读 `video.Stats` 真值，不自算） | 自己另起解码线程、自己算时间戳 |
| `video_render.go` | 画幅矩形计算（缩放只走显示目标矩形，解码分辨率不动）、控制条/浮层排版 | 逐像素拷贝上墙 |
| `video_widget.go` | `BuildVideo` 唯一入口 + Ctx 订阅（主题/语言/方向）+ 事件回调（首帧/播完/出错/缓冲变化） | 绕过 `video` 直调 `video/ffmpeg` |
| `video_theme.go` | 尺寸/色板/字号默认值（走 Ctx，不写死） | 用户文案硬编码 |

规则：组件只调 `video` 公开口；画画默认走 §4 视频直传口，不存在或失败才走老 `DrawImage`；组件里看不到后端分支；示例只做摆位与演示数据（组件示例边界纪律）。

---

## 7. 性能预算（2K/4K 一次达标 · 硬）

一帧多大（RGBA）：1080p 约 8MB，2K（2560×1440）约 14MB，4K（3840×2160）约 33MB。30 帧每秒上传量：1080p 约 240MB，2K 约 420MB，4K 约 950MB。

| 档 | 片例 | 硬门禁 |
|---|---|---|
| 1080p | 1920×1080，30/60 帧 | fps ≥ 55 且 p95 ≤ 22ms，每帧分配 ≤ 2048B，池命中看稳态零增长，峰值 ≤ 512MB |
| 2K | 2560×1440，30/60 帧 | 同上 fps/p95 线，峰值 ≤ 1024MB，视频槽零淘汰 |
| 4K | 3840×2160，30 帧 | 同上 fps/p95 线，峰值 ≤ 2048MB，视频槽零淘汰，首尾帧非黑 |
| 4K60 | 3840×2160，60 帧 | 同上，允许按时间线丢旧帧（丢数如实计，不算失败），其余同 4K |

通用纪律：稳态大帧缓冲走池零新开，每帧小对象合计 ≤ 2048B；队列有界；封顶不爆；GC 可观测；降分辨率偷过直接判失败；无基线谈更顺打回。

---

## 8. 真窗与验收

旧 `video_vr*` / `video_vc*` 窗做为回归底线保留（代码允许按新链路重写桥接，门禁线不动；基础播控、跳进度、声音不断线由这些旧窗回归覆盖）。本期新增四扇，扇扇独立目录、独立 main：

| 窗 | 演什么 | 时长 | 过线 |
|---|---|---|---|
| `examples/video/video_hw_play` | 同一片硬解 vs 软解：画面一致（容差比对 + 软解对照），`hw_active` 如实，断硬解自动回落不断播 | 30s | 硬解帧与软解帧差分布达标 + fps/p95 双绿 |
| `examples/video/video_2k_play` | 2K 真片循环播 + 缩放 + 浮层同屏 | 30s | §7 的 2K 行全绿 + 三证据 |
| `examples/video/video_4k_play` | 4K 真片循环播 + 缩放 + 同源大小窗 | 60s | §7 的 4K 行全绿 + 三证据 |
| `examples/kit_video` | kit 组件独立窗：摆位、播控、事件、主题，不占 VR 名额 | 30s | 组件播控全绿 + 三证据 |

每扇窗结束输出全族 JSON（帧时、管线、播放、CPU、内存、GPU、解码、首帧、回归、合规十族，字段口径沿 §7 门禁 + V-U9 三证据），不达标 `FAIL:` + `exit 1`，README 写人眼可见效果。大片不进仓库：`video/testdata/` 只放小片与 2K/4K 生成脚本（大二进制不提交，本地与 CI 均由脚本现场生成后跑窗）。

跨平台标记（合入前按行扫完，不留空白格；P1 落定值，2026-09-30）：

| 能力 | Linux WebGPU | Linux GL | Windows WebGPU | Windows GL | macOS |
|---|---|---|---|---|---|
| 硬解（本期） | ✅ VA-API 在流（x64 真机 vaapi active，软硬首帧均值差 0.01；arm64/386/arm 库无硬解编进，老实软解 + hw_fallbacks 记 1） | ✅ 同左（经同一 ffmpeg 通路，P0 查询口后端中性） | 🔌 代码就绪待真机（d3d11va→dxva2→d3d12va→qsv→cuda 顺序试，库含 d3d11va/dxva2，DLL 加载走 LoadLibrary；本机无 Windows 卡未验，走软解回落不断播） | 🔌 同左（经同一 ffmpeg 通路） | 🔌 库未到占位（videotoolbox 顺序已留，仓无 dylib，Available 假则单测 Skip 注明原因，软解不断播） |
| 视频直传 | 首版达标（P0 地基，后端中性） | 首版达标（P0 地基，后端中性） | 首版达标（P0 地基，后端中性） | 首版达标（P0 地基，后端中性） | 首版达标（P0 地基，后端中性） |
| kit 组件 | 首版达标（P4，未开工） | 首版达标（P4，未开工） | 首版达标（P4，未开工） | 首版达标（P4，未开工） | 首版达标（P4，未开工） |

图例：✅ 真机在流/单测绿 · 🔌 代码就绪待真机或待库（注明原因，不空着）。

---

## 9. 实施分期（P0–P5，一期一收口）

| 期 | 干什么 | 落点 | 收口证据 |
|---|---|---|---|
| P0 | `render` 视频地基：后端查询口 + 视频纹理独立池骨架 + 通用路回落计数 | `render` 新增视频文件 + 单测 | 查询口双后端绿 + 池借还零泄漏 + `go vet` 过 + 合入前 grep 门禁（§5 口径，创建入口与注册行除外零命中） |
| P1 | `video` 硬解接线：设备选择 + 解码绑硬解 + 回传 + 统计 + 回落 | `video/ffmpeg/decode.go`、`video/backend_ffmpeg.go` 重写 + 单测 | 真机硬解帧断言或软解回落断言 + 旧回归逐文件绿 |
| P2 | 直传打通：画视频帧口 + 原地重写 + 显卡缩放 + 没新帧不传 | `render` 直传实现 + 一扇旧窗改直传 | 1080p 窗 fps/p95 双绿 + 上传次数等于显示新帧数（无多余重传） |
| P3 | 2K/4K 达标：单次整拷（或零拷包）+ 预乘跳过 + 预算单列 + 两扇新窗 | 桥接收敛 + `examples/video/video_2k_play` + `examples/video/video_4k_play` | §7 的 2K/4K 行全绿 + 三证据 |
| P3-A | 平面直传（只做 A）：NV12 两小块上传 + 显卡转色 + 线程化解码，17 扇窗换传参 | `video` 出平面 + `render` 平面槽口 + 两后端 YUV 程序 + 窗节点换参 | 2K30/1080P60 全绿 + 新老路对拍误差达标 + 三证据；跟不上的快片只慢播不报错，丢帧如实记 |
| P4 | kit 组件：扁平五件 + `BuildVideo` + `examples/kit_video` 窗 | `ui/kit/video_*.go` + `examples/kit_video` | 组件播控全绿 + 不碰 `gpu` 包合规过 |
| P5 | 回归收口：旧窗全重跑 + 文档 API 目录同步 + 跨平台格补齐 | 全量回归 + 本文件修订行 | 所有窗绿 + 机器校验过 + 提交清单干净 |

每期纪律：先读真源再动手；测试数据入 `testdata/`；回归按测试文件逐个跑；只提交本期自己文件，提交前与用户确认；未推送不算同步，推送需用户确认。

---

## 10. 修订

| 版本 | 说明 |
|---|---|
| v2.0（2026-09-30） | 推翻重写：硬解收进本期、`render` 经 `hal` 直传自动跟后端、2K/4K 一次达标、kit 组件落案、旧代码允许重写。旧版（v1.x）以 git 历史为准，不再逐条保留。 |
| v2.1（2026-09-30） | 收敛优化：base/full 两变体口径、Player 公开口补 Info、脏区走 `FrameDamage` + `PresentFrameDamage` / `PresentFrameDamageRects`、后端查询只看能力、队列默认 4 与背压语义、包图边界收归消费侧、跨平台补 Windows GL 列、P2 证据收紧为上传次数对帧数。 |
| v2.2（2026-09-30） | P0 地基落地：`render/video_direct.go`（后端查询口 + 视频独立池骨架 + 回落计数）+ 单测四绿 + 总账同步；真机双后端窗验留待 P2 直传打通时补。 |
| v2.3（2026-09-30） | P1 跨平台收口：硬解只走 ffmpeg（设备/协商/回传/转色全是库函数，Go 只做接线）；选型按系统分路（Linux vaapi，Windows d3d11va→dxva2，macOS videotoolbox 占位，安卓 mediacodec 占位）+ 32 位守卫 + 上下文布局自检 + dxva2 补齐 + DLL 加载分 Unix/Windows 双实现；Linux x64 vaapi 在流，其余系统老实回落；跨平台格一次填齐。 |
| v2.4（2026-10-01） | P3-A 五层口径落文档：video 出 NV12 平面 + 线程化解码、池按平面建、render 加平面槽口 + YUV 画口（老整块口留回落）、两后端对称 YUV 程序、17 扇窗换传参 + 对拍单测；跟不上的快片只慢播不报错，独显路不通老实回落。 |
| v2.5（2026-10-02） | 3.0 第 4 层落地：YUV 转色搬进显卡，一份 WGSL（`render/internal/gpu/shaders/video_yuv.wgsl` + `video_yuv_pipeline.go`）两后端通用（WebGPU 真编译绿，GLES 翻译双纹理映射绿，绑定形状沿用生产双纹理先例）；`DrawVideoPlanes` 真排队，无会话/坏参/变体缺失照样失败闭合走 CPU 转回落，计数口径不变。 |
| v2.6（2026-10-02） | 3.0 第 5 层落地：窗口只调节点单新口 `RenderVideo.UploadPlanes`（平面走 GPU 转色，奇数/异形帧走桥老口兜底记数），17 扇 19 处全换并逐个编译；2K 真窗 80/80 上传零回落、首尾亮 111.8/110.2、`yuv_ready=1`，p95 31.7 随整机抖动（帧率 59.1 达标，p95 门待压）；离屏逐像素对拍因 1x 适配器无 MSAA 解析路走不通，已删，改由 2K 真窗作证。 |
| v2.7（2026-10-23） | 新增 §4.6 未关账登记（线程线 S2b/C0 复现顺带查出的三条，全部带实测数字）：V-1 retained 窗视频整块留白（实况走查能出画面、retained 封包 0 图层，根因在建层不调 `node.Paint`，归 #17/S2b C4 的 `TextureLayer`）；V-2 节点喂帧口无回落（非对齐/未挂设备/奇数平面三种形状实测丢帧且不计数，订正 v2.6「奇数/异形帧走桥老口兜底记数」——老口 `Show` 有兜底，节点口没有）；V-3 跨设备槽失败开放（`DrawVideoSlot` 认不出槽不属于当前会话，照记 direct 零回落、像素不落）。本行只登记，不改行为。 |
| v2.8（2026-10-24） | C4 纹理层接视频（线程线 §18.1 #17 第六笔，工作区未提交）：`ui/scene` 新增 `TextureLayer`（id 化、永不缓存、每帧重画）+ `ui/rendering` 的 `TextureLeaf`/`TextureLeafProvider`/`addTextureLeaves`，`RenderVideo` 交主视图 + PiP 两片；`render/texture_source.go` 新增 `TextureSource`/`TexturePaintOptions` + 注册表四函数，`VideoBridge` 实现 `PaintTexture`/`ID`/`PaintStats` 并自注册/自注销；C0 复现测试从 SKIP 改回 `t.Fatalf`，三窗加 retained 封包两帧像素断言。**V-1 仍未关**：三窗本机未实跑、无证据；**V-2 未动**；V-3 已修（2026-10-24，单测：`DrawVideoSlot`/`DrawVideoPlanes` 设备归属校验 + `TestVideoDeviceMatches`，真窗负向未跑）；视频线仍无 Golden。P0/P1 已修（2026-10-24）：`PaintStats` 增 `ViewLastOK`、`VideoNodeStats` 增 `DrewLast`/`SmallDrewLast`，4K/2K 门禁改断最近一帧；VR8 去掉 `paintsB > paintsA` 自证门；纹理层 miss 仅曾出画后补损伤（`TextureLayer.painted`；从未出画层不污染）。 |
| v2.9（2026-10-24） | C4 三窗实跑 + 两处门禁修正（工作区未提交）：① `video_player` 按需 ticker 在第 30 拍前空闲，桥从未挂设备（Frames 0/显示 2/retained 缺片）；改为每拍 `BorrowVideoBackend` 直到首帧，复跑 Frames 62、retained 墨 1.00/1.00 动 0.10、exit 0。② `video_vr8_embed` 的 `lastVar=pixVar(f.Pix)` 对 NV12 帧读到 nil（首/尾 MAD 0.0 假黑屏）；改为 Pix 为空时取 Y 平面（冷启动 `loadEmbed` 同步修），复跑 `last_var=62.76`、`embed_ok=1`、`resize_ok=1`、retained 主/小窗墨 1.00/1.00 动 0.09125/0.1275。③ 4K retained 主/小窗墨 1.00/1.00 动 0.0425/0.0900、纹理出画峰值 2；4K/VR8 `fps_wall` 56.7/59.1 达标，但 `p95` 22.99/22.19ms 超 22ms，三窗 `-race` 均崩在 wgpu-hal 29.0.4 `native.rs:359` `Trying to destroy a SwapchainAcquireSemaphore that is still in use by a SurfaceTexture`（归 #18）；纹理层 miss 仅曾出画后补损伤（`TextureLayer.painted`，`TestTextured_TextureLayerMissPaintsAndDamagesNothing` 绿、`TestLayerTree_VideoTextureMissDamagesRect` 仍绿）；视频线仍无 Golden。**V-1 retained 出画已证，C4 不标关**。 |
| v2.10（2026-10-24） | #18 根修复 + V-2/V-3 收口（工作区未提交）：① #18 修在 `gpu/rwgpu`（`Surface.current` + `DiscardCurrent`/`DiscardCurrentIf` + `discardPresentationLocked` 走 `wgpuSurfacePresent`；`Configure`/`Unconfigure`/`Release` 先清 native current；非 success acquire 的 `dropTex` 也清 `presentation.acquired_texture`）与 `gpu/webgpu`（acquire epoch；超时 `BumpAcquireEpoch`，晚到 `DiscardCurrentIf` 后返回 `ErrAcquireTimeout`；`isSkipFrameSurfaceErr` 归跳帧，BeginFrame 不在 acquire 在飞时 Configure）。单测 `TestSurfaceDiscardCurrent_NilSafe`/`TestSurfaceAcquireEpoch_BumpInvalidatesInFlight`/`TestIsSkipFrameSurfaceErr_AcquireTimeout` 绿。三视频窗 `-race` 不再 abort：player 110 显示/35 上屏（race 慢，retained 快照超时）、4K retained 1.00/1.00 动 0.0525/0.1225（fps 11.9/p95 51.75）、VR8 retained/embed/resize 绿（fps 47.9/p95 37.019）；非 race player 55.5fps retained 绿、4K retained 绿 p95 23.021、VR8 绿 p95 21.846。R4 `-race` 通过；R3 `-race` 不 abort 但既有 `ui/rendering.clearPaintDirty`/`NeedsPaint` 竞态与 fps 红归 S3。② V-2 已修（单测）：池 nil/行距不对齐/上传失败保留 CPU shadow，`drawCurrentAt` 计数回落；节点 `UploadPlanes` 平面失败转 RGBA；三测绿；生产三窗喂对齐 NV12，回落形状无真窗。③ V-3 真窗负向已跑：`TestV3_VideoCrossDeviceRealWindow` 真 X11 窗 A/B 双设备，direct=false、reason=`video:device-mismatch`、cpu_fb=1、像素 `(255,0,0)`、presents=1。**C4 仍不标关**：4K p95 23.021ms 超 22；视频 Golden 未出（色度 0/偏绿独立问题未修）；`-race` 性能门禁红另账。 |
| v2.11（2026-10-24） | 色度根因修复（工作区未提交）：V-4 登记并修复——NV12 的 UV 纹理原按 `w×h/2` RG8 建，NV12 一行 UV 是 `w` 字节 = `w/2` 个 RG8 texel，归一化中心采到零填充右半边，U/V 读 0、按 BT.601 变绿。改为 UV 纹理 `w/2×h/2`、上传宽 `w/2`（行距仍 `w` 字节对齐）；新增真 GPU 回归 `render/video_yuv_color_gpu_test.go:TestVideoYUVPlanes_GPUColor`（padded 64×32 / aligned 256×64），纯红帧从 `(0,224,0)` 修到 `(255,0,0)`，两子例绿。**C4 仍不标关**：三扇真窗已复跑，retained 快照彩条正常；仍无逐像素颜色门禁、视频 Golden 未出、4K p95 22.906ms 仍超 22ms。 |
| v2.12（2026-10-24） | 4K p95 根因修复：异步双缓冲 NV12 上传（工作区未提交）。根因实测：UI 线程同步 `UploadPlanes` 4K NV12 单次 ~5–8ms（无争用基准 avg 8.00ms / p95 27.26ms，wgpu 队列锁整调用持有），叠加逐帧 `MarkNeedsPaint` 让含视频的图层每帧重录，p95 23.02ms。修法按用户定的 B 案「能力口 + 光栅回退」：① `gpu/hal` 新增可选口 `CrossThreadTextureWriter`（webgpu/noop 报 true；GLES 双平台与 metal 报 false，理由写在各实现注释）；② `VideoBridge.UploadPlanesFrameAsync`/`UploadPending` 状态机 idle→copying→ready→(published) idle，跨线程后端由 worker 上传后**双缓冲换槽**（`pslot` 画、`pback` 写，只在完整帧上换），GL/metal 由光栅绘制时 `publishPendingPlaneLocked` 上传；`Close` 先等 copy worker 再释放；③ `WriteTexture` 按 ~1MiB 分块（`Origin.Y` 行偏移 + 块行数），单次持队列锁时间从整帧降到块；④ 上传成功不再额外拷 CPU shadow（只在上传被拒时拷，V-2 回落）；⑤ `RenderVideo.UploadPlanesAsync`/`UploadPending`，`noteFrame` 只在首帧标脏（活纹理层每帧自会重画，逐帧重录正是 p95 尾巴）；⑥ 4K 窗在 `UploadPending` 为真时不 `Poll`（解码平面归路线所有），只对成功交接计显示，去掉 `Redraws<1 && Fallbacks==0` 旧门（`uploads==frames` + 两视图最近一帧出画已覆盖）。**真窗证据（4K 30s，exit 0）**：`fps_wall=58.92`、`interval_p95=19.37ms`（修前 23.02）、`video_frames=video_uploads=107`、`fallbacks=0`、`evictions=0`、retained 主/小窗墨 1.00/1.00、动 0.0975/0.20、`uploadsA/B=7/10`、RSS 峰值 742720KB。**余量反证**：把同一 4K 帧按住按显示率重传（1731 次/30s）仍 p95 17.93ms / fps 57.7，证明渲染上传侧已不是瓶颈；真窗 30s 只传 107 帧是解码吞吐所致，登记 V-5。单测 `render/video_async_upload_test.go` 8 例（跨线程发布/双缓冲换槽/光栅发布/忙拒绝/两条上传失败回落/Close 等 worker/影子缓冲换位）`-race` 绿，`ui/rendering` 两个视频测试文件 `-race` 绿。顺带登记 V-6（Linux GL 补行 UV 行距丢失，存量缺陷，只登记不改行为）。**修后回归（工作区当前版）**：4K 复跑 exit 0（fps 58.95、p95 19.31ms、uploads=frames=104、回落 0、retained 主/小窗墨 1.00/1.00 动 0.0975/0.20、峰值 RSS 669MB）；2K 30s exit 0（fps 59.63、p95 20.25ms、uploads=frames=254、回落 0、槽淘汰 0、峰值 595MB、首尾亮 111.8/99.0）；`video_player` 15s exit 0（77 显示/77 上传、retained 墨 1.00/1.00 动 0.15）；`video_vr8_embed` 30s exit 0（embed/resize/scale/clip/overlay 全绿、retained 主/小窗墨 1.00/1.00 动 0.16/0.20）。`video` 包逐文件回归有存量红（本次未动 `video/`，非本笔引入）：`TestLoopSteadyBytes` per_frame_B=2205>2048、`TestVR3FFmpegParity` 三子例 player md5 为空、若干 seek 用例红；不据此改本笔行为。**C4 仍不标关**：视频 Golden 未出；4K 真窗出帧受解码吞吐限制（V-5）；`-race` 性能门禁红另账。 |
