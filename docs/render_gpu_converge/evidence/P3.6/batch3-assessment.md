# P3.6 批 3 影响面评估：三扇真窗（2026-11-03 开工登记）

直话：批 1a–1d 的能力（格式/解码/消费链/mipmap/组件接线）全部已交付且各有单测钉，但**三扇真窗一扇都还没建**——批 2 的档位生效证据明确说「由显存读回钉与批 3 真窗承担」，批 1c 的消费链对账与批 1d 的播放链都停在单测层。批 3 就是把这三份「已验证待接线」的能力各自立一扇独立真窗（AGENTS「每主能力独立真窗，禁组合窗」）。

一手实测（两个探查子代理广度扫描 + 本机复核关键行号）：

## 1 三扇窗的卡面（design.md §3 既有定义）与落法

### 1.1 `examples/ui_wr_p36_imgfmt`（格式窗）

- 卡面（design.md:74）：EXIF 八档方向对拍卡、GIF/APNG/动图 WebP 帧序列与 dispose/blend 合成卡、嗅探优先于扩展名卡。
- 现状：三块能力全有单测与独立参照（批 1a 方向 8 档对 Pillow `exif_transpose` 参照网格、批 1b 15 个动图样本对 oracle 逐字节、嗅探有内容不符显式报错钉）。真窗价值 = 真实 GPU 呈现 + 整窗金图。
- 落法：照 P3.5 卡片形状三卡并排；每卡**静态确定性**——方向卡画 8 档缩略（源图 × 方向应用后）、帧序列卡画 oracle 参照帧（`DecodeFrame(i)` 后并排）、嗅探卡画识别结果文本 + 缩略。无时变 → 冻结天然成立，金图零容差。

### 1.2 `examples/ui_wr_p36_animplay`（播放链窗，D9 新增）

- 卡面（design.md:75）：连播期间 `imageCache` 条目数/`usedBytes`/`hal.VramLiveBytes`/`VramLiveCount` 四路对账卡、空闲池命中（`CreateTexture` 次数）卡、同一动图两消费者不冻结对拍卡、图案填充采样场逐帧跟手卡。
- 实测的关键缺口（决定本批公开面）：
  - `ImageCacheStats{Entries, UsedBytes, Creates, Hits/Misses/Uploads...}` 在 `render/internal/gpu/image_cache.go:701-756`，**锁在 internal 包**（Go internal 规则只许 render/... import），render 公开层零转发（全仓 grep 只有 `sdf_gpu.go:506-524` 的 SDFAccelerator 内部转发）。→ 待拍 D20。
  - 显存四路里的 `VramLiveBytes/VramLiveCount` 已有 render 转发口 `render.VramLiveBytes/VramLiveCount`（`shared_budget.go:60-65`），examples 先例 `ui_render_pelican/main.go:242`、`ui_base_vram_cap/main.go:134` 直接用 ✓。
  - 两消费者卡：**真 GPU 离屏 target 在 examples/ui_wr_* 无先例**（探查结论；最近参照是 pelican 测试的 `render.NewPresentTarget`（上屏）与 P3.5 窗的 CPU `render.NewContext` 孪生（非 GPU 消费者））。→ 待拍 D21。
  - 图案填充卡：`render.NewImagePattern(buf,...)`（`context_image_pattern.go:28`）的 GPU 采样场键 `patternFieldSeed` 混 `GenerationID()`（`brush_native.go:557-567`）——动图 buf 逐帧 `NotifyPixelsChanged` → 键变 → 新场，画面跟手即是像素断言。
- 播放链驱动：本窗用 **render/rendering 层原语**（`RenderImage.SetAnimatedImage` + `PipelineOwner.AddTicker`）摆窗直驱——kit 组件的生产窗已由 `examples/kit/anim-image`（批 1d）承担，本窗验的是批 1c 消费链（缓存/显存对账），用原语不带组件层。

### 1.3 `examples/ui_wr_p36_mipmap`（mipmap 窗）

- 卡面（design.md:76）：同一图按 `None/Nearest/Linear` 三档缩样并排（摩尔/混叠可见差）、CPU/GPU 双档同源。
- 双后端前提已核：纯 Go GL 端逐层上传已实现（`gpu/gwgpu/gles/device.go:262-294` 逐层分配 + `TEXTURE_MAX_LEVEL`、`queue.go:191` 按 `MipLevel` 上传），批 2 采样器按档绑定两端对齐。→ `GPUI_BACKEND=go/native` 各跑一遍照 P3.5 先例。
- 显存对账：三档各开链后 `render.VramLiveBytes` 增量对 `gpu/hal/mip_math.go` 精确求和公式（批 2 交付的单点公式，CPU/纹理/账三端共用）——对账卡有硬参照。

## 2 窗骨架（复用 P3.5 形状，不新造）

`wrkit.NewShell(1200×800)`（48px 顶栏 + 260px 图例 + Body + 72px HUD）+ 卡片 `OnPaint` + `wrkit.ShellContentRects()` 金图掩码（顶栏/HUD 排除）+ `wrkit.GoldenGate/CheckPixelProbes` + `-auto-only` 相位写死（P3.5 先例：`sweepPhase=0.35` 冻结）+ `-manual-seconds` 常驻人工档 + `GPUI_BACKEND=go/native` 双后端各一遍。单窗 main.go 量级 686–912 行（P3.5 实测）。

## 3 门禁口径

照 P3.5 `windows-close.md` 与批 1d 同套：每扇 `-auto-only` 逻辑探针 + 像素探针 + 金图（首跑 `GPUI_ACCEPT_GOLDEN=1` 种基线、复跑 diff=0）、双后端、正常关闭（WM_DELETE）、全套包门禁（apidoc/vet/定向/race/五目标/逐文件对基线）。真窗在 GNOME Wayland + XWayland 下跑（本会话显示环境本就在 AGENTS 隔离清单内，单窗一轮一验证）。

## 4 待拍板（D20–D23）

- **D20 imageCache 统计转发口**：(A) render 包级函数 `ImageCacheStats()`（先例：`GPUPoolStats()`/`FilterPoolStats()` 均包级函数；经 accelerator 默认实例转发，internal→render 一条转发）；(B) `GPUAccelerator` 接口加方法（契约面变大、所有实现同批改）；(C) 不转发，对账卡只用 Vram 两路 + 帧率（对账强度降级，Entries/Creates 测不到）。推荐 (A)。
- **D21 两消费者卡落法**：(A) 真 GPU 离屏 target（无 ui_wr 先例、工程量大，要新起 present-target 会话）；(B) 双窗口（两 session，缓存/设备归属复杂）；(C) 同窗两个 `RenderImage` 经 `SetImageShared` 共享同一动图 buf（**批 1c 钉 `TestImageCache_TwoConsumersBothReupload` 的同一条可达通路**：一 buf 多节点，同 session 内第二消费者；工程量小、语义与单测钉一致）；真 GPU 离屏 target 登记后续。推荐 (C)。
- **D22 切分**：(A) 三扇一批交付（三目录三 main，门禁一次跑、账一次收）；(B) 每扇独立小批（3a imgfmt → 3b animplay（含 D20 转发口）→ 3c mipmap），逐扇三证据+提交。推荐 (A) 一批三扇：三扇相互独立、共享一套门禁脚本与文档收口，拆三批的簿记成本大于收益。
- **D23 imgfmt 窗帧序列卡是否接播放**：design 定义是「帧序列与 dispose/blend 合成卡」（静态并排逐帧）——播放链动画是 animplay 窗的事。确认 imgfmt 保持静态（冻结确定性最简单）。

本轮零代码变更（评估 + 登记）。

## 5 拍板（用户 2026-11-03，四条全按推荐）

- **D20=(A)** render 包级转发函数（照 `GPUPoolStats`/`FilterPoolStats` 先例），对账卡拿全量统计；
- **D21=(C)** 两消费者卡 = 同窗两个 `RenderImage` 经 `SetImageShared` 共享同一动图 buf（与批 1c 单测钉同一条可达通路）；真 GPU 离屏 target 登记后续；
- **D22=(A)** 一批三扇，共享一套门禁脚本与文档收口；
- **D23** imgfmt 帧序列卡保持静态并排，播放归 animplay 窗。

方案落盘 `batch3-design.md`。
