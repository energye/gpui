# P3.6 批 3 方案：三扇真窗（2026-11-03 拍板后落盘）

拍板（用户 2026-11-03，四条全按推荐，见 `batch3-assessment.md` §5）：**D20** render 包级转发函数 `ImageCacheStats()`；**D21** 两消费者卡 = 共享 buf 双节点；**D22** 一批三扇；**D23** imgfmt 帧序列卡静态并排。

## 1 D20：imageCache 统计转发口

- `render/image_stats.go`（新）：公开形状 `type ImageCacheStats struct { Entries int; UsedBytes int64; Creates, Hits, Misses, Uploads, Evictions uint64 }`（字段逐一映射 `render/internal/gpu/image_cache.go:701-727` 的内部形状）；包级注册口 `RegisterImageCacheStatsCollector(fn func() (ImageCacheStats, bool))`（幂等覆盖，照 `RegisterGPUPoolStatsCollector` 的形状 `gpu_shared.go:249`）；包级查询 `ImageCacheStats() (ImageCacheStats, bool)`（无 collector 注册时 `(零值, false)`——非 GPU 会话/纯 CPU 路安全）。
- internal 接线：`render/internal/gpu` 建 holder 处注册 collector，fn 动态解引用默认 session（`SDFAccelerator.defaultCtx.session.imageCache`，`sdf_gpu.go:44,506`；session 未建/已置 nil 返回 false）。
- 钉：`render/image_stats_test.go`——(1) 无注册时 `(零值,false)`；(2) 注册假 collector 后读回；(3) 字段映射（真实 GPU 会话，GPU 可用时 Entries≥1、Uploads 随上传涨——环境不可用 Skipf）。`ui/depcheck` 不受影响（examples 只 import render）。

## 2 三扇窗（骨架照 P3.5：`wrkit.NewShell(1200×800)` + 卡片 + `ShellContentRects` 金图掩码 + `GoldenGate`/`CheckPixelProbes` + `-auto-only` 冻结 + `-manual-seconds` 人工档）

### 2.1 `ui_wr_p36_imgfmt`（三卡静态，D23）

- 卡 A EXIF 方向：源图 8 档 `ImageOrientation` 应用后缩略并排，标签标档位；探针比对 `testdata/imgfmt/oracle`（方向应用后应与 Pillow 参照一致——窗内画的是同一份解码产物，像素断言取每档一块特征区 vs oracle PNG）。
- 卡 B 帧序列合成：`gif_restore_prev.gif`/`gif_restore_bg.gif`/`apng_keep_erase`（disposal 三态）逐帧 `DecodeFrame(i)` 后并排缩略，帧下标 disposal/blend 标签；像素探针取 restore-bg 帧的背景矩形（恢复底色）与 keep 帧叠加区。
- 卡 C 嗅探：内容 vs 扩展名不符的显式报错样本（批 1a 钉过的通路）展示识别结果 + 正确解码缩略；逻辑探针跑 `SniffEncoding` 断言。
- 冻结：全静态，金图天然确定；逻辑探针 = 方向 8 档 `SwapsWidthHeight` 断言 + 嗅探断言 + 帧数/时长断言。

### 2.2 `ui_wr_p36_animplay`（播放链 + 对账，D21）

- 卡 A 主播放：`RenderImage.SetAnimatedImage(gif_keep)` + `PipelineOwner.AddTicker` 直驱（render/rendering 原语，不带 kit 组件）；画面持续动（manual 档可见）。
- 卡 B 对账 HUD（文本行画在卡上，`ImageCacheStats` + `render.VramLiveBytes/VramLiveCount` 每 tick 采样）：连播期间断言 **Entries 恒 1**（同一 buf 同代际键只占一条——批 1c 图号=内容版本）、**Creates 恒平**（纹理复用不 churn）、**Uploads 随帧单调涨**（逐帧重传）、VramLive 不随帧涨（换代即归还——批 1c 监听器逐出）。auto 档冻结帧位后取末值断言；逻辑探针记录全跑期间 min/max。
- 卡 C 两消费者：卡 A 的 buf 经 `SetImageShared` 装第二个 `RenderImage` 镜像卡；断言两卡画面同帧位不冻结（批 1c 修复的「第二消费者冻结」）、Uploads 含两节点重传。
- 卡 D 图案填充：`render.NewImagePattern(动图 buf, …)` 填充一块矩形；断言画面跟帧（帧间着墨变化——同一卡在两个冻结相位差拍两次？auto 单快照取「跟手」结构断言：pattern 卡着墨区与直画卡着墨区同帧一致）。manual 档人眼看连续动画。
- 冻结：auto 结束前 `Pause+SeekFrame(0)`（照 kit 窗先例），对账取冻结前最后采样。
- 线程纪律：全部 UI 线程（ticker + paint），`-race` 门禁组含本窗包。

### 2.3 `ui_wr_p36_mipmap`（三档并排 + 显存对账 + 双后端）

- 卡 A 三档并排：同一张高频细节图（用 `render/testdata/imgfmt` 的网格图或程序生成的确定性棋盘+细线图——**测试数据入 testdata/**，用现成 imgfmt 样本）按 `DrawImageOptions.Mipmap{None,Nearest,Linear}` 缩到 1/8 绘制；摩尔/混叠差异肉眼可见；像素探针取三档各一块高频区断言三档两两不同（None 与 Linear 差应最大——批 2 显存读回钉同口径）。
- 卡 B 显存对账：`render.VramLiveBytes` 增量 vs `mip_math` 公式（`1+floor(log2(max))` 层 × 各级字节和）——三档各自开关后的账差断言（容差 0，公式是单点真值）。
- 双后端：`GPUI_BACKEND=go` 与 `=native` 各跑一遍 `-auto-only`，金图各自基线（两端采样器实现不同，逐位一致不跨后端要求；同后端两连 diff=0）。

## 3 门禁与验收

- 每扇：`-auto-only` 逻辑探针 + 像素探针 + 金图（首跑 `GPUI_ACCEPT_GOLDEN=1` 种基线，复跑 diff=0）+ WM_DELETE 正常关闭 + `wrgate.BuildReport` JSON（fps/presents/vsync 字段照 P3.5 门禁）。
- 包门禁：apidoc rc=0（公开面 +1 类型 +1 函数 +1 注册口）、vet 触及包、定向、`-race -count=2` 触及组、五目标×2、逐文件对批 1d 基线（737 条）、金图（三扇新基线 + 存量零变化）。
- 人工档：三扇各 `-manual-seconds` 常驻待人工（收真事件打日志改标题）。

## 4 顺序（每步可编译可测）

1. D20 转发口 + 钉；
2. imgfmt 窗（静态、风险最低）→ 三证据收口；
3. animplay 窗（对账卡 + 双节点 + 图案）→ 三证据收口；
4. mipmap 窗（三档 + 对账 + 双后端两遍）→ 三证据收口；
5. 三窗门禁脚本 + 包门禁全套 + 文档收口（catalog/台账/修订表/batch3.md）+ 提交确认。

## 5 登记不做

- 真 GPU 离屏 present-target 会话（D21 改共享 buf 双节点后不需要；跨 session 消费者留后续专项）；
- imgfmt/animplay/mipmap 的 Windows/macOS 真机档（本机 Linux；跨平台标记按域真源格补「桌面已验/真机待验」）；
- mipmap 窗不做小数 LOD 滑块交互（批 2 显存读回钉已覆盖小数 LOD 判别；窗只并排三档）。
