# P3.6 批 3 交付：三扇真窗 + D20 统计转发口（2026-11-03）

拍板（`batch3-assessment.md` §5，四条全按推荐）：**D20** render 包级转发；**D21** 共享 buf 双节点；**D22** 一批三扇；**D23** imgfmt 帧序列静态并排。方案 `batch3-design.md`。

## 1 D20：imageCache 统计转发口（`render/image_stats.go`）

- 公开面 +1 类型 `ImageCacheStats`（字段 `Entries/UsedBytes/Creates/Hits/Misses/Uploads/Evictions/RetiredGens/Watched/MipEntries/MipLevelUploads`，逐一映射 internal imageCache 统计）+2 函数 `ImageCacheSnapshot() (ImageCacheStats, bool)`（ok=false=无装配/会话未建，调用方不猜值）与 `RegisterImageCacheStatsCollector(fn)`（幂等覆盖）。
- internal 接线两处实测修正：
  1. **诊断读跨线程安全**：原 `ImageCache` 计数器为裸字段（单线程契约），真窗对账卡从 UI 线程采样 = 跨线程读。九个计数器改 `atomic`、`usedBytes/entries` 快照加 `statsMu`（写侧仍局限光栅线程，缓存操作的单线程契约不变——锁只保护诊断快照一致性）。
  2. **锚点从 defaultCtx 改任一活跃会话**：窗口绘制（present target）建自己的渲染上下文，不走 accelerator 默认上下文——`HasDefaultImageCache` 永远 false。新增 `ImageCacheStatsFromLive()`（遍历 `GPUShared.liveCtxs` 取第一个有 imageCache 的会话），collector 注册改用之。
- 钉 `render/image_stats_test.go`：无装配 false / 读回 / 覆盖幂等 / nil 无操作（哨兵证明）/ 无会话透传。apidoc rc=0，catalog §3.5 新增「图像缓存统计转发」行。

## 2 三扇真窗（骨架照 P3.5：wrkit.Shell + ShellContentRects 掩码 + GoldenGate + auto-only + manual 档；金图基线按后端分文件 `_native/_go`）

### 2.1 `examples/ui_wr_p36_imgfmt`（格式窗，静态）

- 卡 A：EXIF 八档缩略并排（方向应用后 5–8 档宽高交换可见）；卡 B：dispose 三态首末帧定格（restore-prev 回填 / restore-bg 擦底 / apng keep+erase）；卡 C：嗅探优先（PNG 内容冒 .gif 扩展名等三例解码缩略）。
- 逻辑 12 条（8 档解码尺寸交换表照批 1a 口径、三素材帧数、嗅探语义）+ 像素 3 探针（区域着墨）+ 金图掩码 ShellContentRects。
- 过程修正：方向交换表首版把 6/8 档写错（权威表在 `TestImageOrientationSwapsWidthHeight`）；帧序列卡首版三帧溢出卡宽改首末两帧（中间帧合成由单测 oracle 逐字节锁）。

### 2.2 `examples/ui_wr_p36_animplay`（播放链窗，D9）

- 卡 A 主播放（`RenderImage.SetAnimatedImage` + owner ticker 直驱，窗侧驱动器照 kit 状态机形状自持时间轴）；卡 C 镜像消费者（`SetImageShared` 同一 buf——批 1c「一 buf 多节点」通路的真窗回归面）；卡 D 图案填充（动图 buf 做 `ImagePattern` 源）；卡 B 对账 HUD（金图掩码排除，数字随帧移动是设计内行为）。
- **消费链对账九项全部落地**（5s 连播采样）：`entries_max==1`（图号=内容版本：同一 buf 换帧不涨条目）、`uploads_last=21` 随帧涨且单调、`retired_last=20`（换代即退役广播）、`watched=1`、VramLive 稳态平坦（换代即归还）、播放推进、镜像卡与主卡画面同步。
- **如实登记**：`creates==uploads==21`（一传一建）——批 1c 明确「不接分桶池」，换号退役销毁/新号新建是当前真实行为，对账卡断言 `creates_bounded`（有界不失控）而非「平坦」，池化后才会平坦（登记不做）。
- 过程修正：对账采样锚点（§1 的两条 internal 修正由此实测发现）；`SetFillPattern(nil)` 还原会 panic（`BrushFromPattern` 解引用 nil）→ 改 `SetFillPattern(NewSolidPattern(...))`；探针坐标须用窗口逻辑坐标（Body 偏移 284,60 + 卡内摆放）。

### 2.3 `examples/ui_wr_p36_mipmap`（mipmap 窗）

- 卡面：同一张确定性噪声图（LCG 种子固定）按 `Mipmap{None,Nearest,Linear}` 三档缩到 1/8 并排 + 4× 最近邻放大对照。
- 像素判据用**区域标准差**：噪声图预滤波不改均值（三档均值都近中灰），改的是高频方差——实测 std `None=0.1434 >> Nearest/Linear=0.0361`（None 双线性保留高频、链档 8×8 预滤波抹平）。
- 对账卡分段采样 `VramLiveBytes`（基线→None→Nearest→Linear 四段，各档独立 buf）：**判据用段间差**（相对量）——`nearest−none=137KB`（建链增量）> 理论链外额 50%、`linear−nearest=4KB ≈ 0`（两档同链规模，符合批 2「Nearest/Linear 都建链」语义）、None 段最轻。分段绝对增量混有窗口期其他首次分配（swapchain/管线），绝对对账归批 2 的离屏显存读回钉，真窗锁定相对关系（如实登记）。
- 双后端：gles 逐层上传已实现（`device.go:262-294`、`queue.go:191`），`GPUI_BACKEND=go/native` 各自基线各自锁。

## 3 门禁（冻结树，`batch3/` 产物）

- apidoc rc=0；vet 五包干净；定向绿；conformance 双后端 ok；五目标×{默认,nogpu} 10/10。
- **race `-count=2`**：`render/internal/gpu` 全包绿（imageCache 诊断原子化后缓存全测无恙）；`ui/kit`、`ui/rendering` 定向绿。三处红逐条归因（临时 worktree `e51265b2` 同命令对照）：
  1. `render` 定向 `TestImageCacheSnapshotForwarding` —— **本批测试自伤**：注册口是进程级单值，`-count=2` 第二遍的「未注册」断言撞上第一遍残留的哨兵 collector。修测试语义（开头注册「无会话」标准态）后 `-race -count=2` 绿。
  2. `ui/scheduler` 全包 `TestFrameDue_SlowStampsDoNotThrottleBelowSoftwareInterval` —— 两树分布对照：当前树 3/3 红、**HEAD 1 绿 2 红** ⇒ 全包 `-race` 口径下的边缘 flaky（该测试为时序敏感，定向 `-count=1` 两树皆绿），与已归因的 race 债同类，非本批引入（本批未触 ui/scheduler）。
  3. `ui/embedder` 全包红 —— 批 1d §4 已归因的波动/存量债同态。
- 主逐文件 **738 条 = 706 PASS/21 SKIP/10 FAIL/1 NO_TESTS**（738 = 批 1d 基线 737 + 本批新增测试文件 `image_stats_test.go`）。FAIL 名单相对基线漂移 4 条全是测量/时序类：`< s6_9`（本轮 U05/H06 都达标，反而过——正是 §4/批 1d 归因的「stress 边缘波动」结论的直接佐证）、`< shaping_runs`（批 1d 归因的波动本轮恢复）、`> m1_noquadratic`（单跑 0.686s 绿）、`> scheduler/pacing`（单跑 0.531s 绿）。
- 补充逐文件 80 条与基线 FAIL 名单逐条相同。
- **三扇真窗 × 双后端 6/6 全绿**（门禁串行重跑同绿）：逻辑+像素+金图各自两连 0.0000%。

## 3.1 金图与探针清单

| 窗 | 逻辑 | 像素 | 金图掩码 | 双后端基线 |
|---|---|---|---|---|
| imgfmt | 12（方向交换表/帧数/嗅探） | 3（区域着墨） | ShellContentRects | `_native/_go` 各两连 0 |
| animplay | 9（对账八项+播放推进） | 3（三卡着墨） | 三画面卡（对账数字排除） | 同上 |
| mipmap | 4（分段对账相对关系） | 4（三格着墨+std 分离） | 三卡全高 | 同上 |

## 4 登记不做

- 真 GPU 离屏 present-target 会话（D21 改共享 buf 双节点）；
- creates 池化复用（批 1c 登记的分桶池优化，落地后 animplay 对账卡的 creates 才会平坦）；
- mipmap 窗绝对显存对账（归批 2 离屏钉；真窗用段间差相对判据）；
- 三窗的 Windows/macOS 真机档（本机 Linux；跨平台标记随域真源格补）。
