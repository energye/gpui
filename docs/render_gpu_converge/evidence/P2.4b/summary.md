# P2.4b 收口报告：预算统一（六口径 → 一套 Budget，按域分账）（2026-10-30）

> 任务：台账 §2.1 P2.4b；方案：PLAN §2.2-B6 / §7.3；评估：`assessment.md`（D1–D4 经用户批准：全 GPU 侧 10 点 / descriptor 统一 ForAdapter / 声明级分域 / TexturePool 只入表）。
> 性质：**零行为**——预算数字逐项等于改动前常量（pin 单测锁死），淘汰算法一行未动；唯一语义变化是 P2.4a D3 推迟的 standalone descriptor 统一（本机单显卡选卡结果不变）。

## 0 结论（先看这）

| 项 | 结果 |
|---|---|
| 声明点收敛 | GPU 侧 **10 域 + 1 全局口径** 全部收敛到 `render.DefaultBudget(d)` 唯一表（22 个调用点）；旧私有常量声明残留 grep **0 命中** |
| 公平份额 | 两套算术合一为 `render.FairSplit(total, windows, floor)`：ImageCache（条数/字节）与 picture cache（字节）共用；单窗路径逐位不变 |
| 域归属 | 声明级分域：每域上限三元组入表；运行时用量聚合与 `res/registry` 归一见 P2.4c/d；`gpu/hal/vram_ledger.go` 全局账本与 rwgpu/gles 双端记账**不动** |
| descriptor 统一 | `render.OpenStandaloneDevice` 改 `DeviceDescriptorForAdapter(label, adpt)`（P2.4a D3 推迟项）；测试装箱保持 plain 口径 |
| 循环 import 规避 | `render/text` 不新增 import：GlyphMaskAtlas 默认参数由 `internal/gpu` 调用方从表传入（`newGlyphMaskAtlasFromBudget`） |
| 公开 API | `render` +4 导出（`BudgetDomain`/`Budget`/`DefaultBudget`/`FairSplit`）+1 方法（`String`）；`render/internal/gpu` **零差异**（898→898） |
| 门禁 | 构建 5 目标 / vet 61·58 逐条不变 / 逐测试文件 473 个（423 PASS、32 FAIL 全定性零新增红）/ `-race` / conformance K1 / API 对账 / 负向对照 全绿 |
| 真窗 | native 两扇 ✅（`ui_l1_blank` 3571 帧、`ui_base_multiwin_cap` 8 窗三相位账本稳态 316MB）；两扇受**既有缺陷**阻塞并已 A/B 证明非本步回归：vram ladder 8 窗 native present 卡死（**新增登记 I1**，归 P2.4e）、`ui_l1_blank` go 档 = **既有 C7**（0x3002） |

## 1 改动清单（18 文件：3 新增 + 15 修改）

| 文件 | 改动 |
|---|---|
| `render/budget.go`（新） | `BudgetDomain` 11 域枚举 + `String`；`Budget{MaxEntries, MaxBytes, MinEntries, MinBytes, TTL}`；`DefaultBudget(d)` 唯一声明表（数字=改动前逐个相等）；`FairSplit` 唯一公平份额算术；`pictureCacheBudgetDivisor=4` 入表（自 `shared_budget.go` 搬入） |
| `render/internal/gpu/image_cache.go` | 默认 128 条 / 64MiB 改读表；`effBudgetFor`/`effBudgetBytesFor` 拆出可测纯函数，多窗分支改调 `FairSplit`（输出不变） |
| `render/internal/gpu/texture_pool.go` | 128MiB → `DefaultBudget(BudgetTexturePool).MaxBytes>>20`（仅入表，删池归 P2.4d） |
| `render/internal/gpu/gpu_textures.go` | `stencilPoolCap` 8 → 表值（仍为包内 var，测试可覆写） |
| `render/internal/gpu/depth_clip.go` | 24 条 / 16MiB / TTL 5s 三口径 → 表值；`const` 改 `var` 以保测试覆写能力；字节比较处显式 `int` 转换 |
| `render/internal/gpu/dual_tex_blend.go` | 32MiB 总预算 + 每桶 8 条 → 表值；新增命名常量 `dualTexOutPoolBucketCap` 替代内联 8 |
| `render/internal/gpu/gpu_render_context.go` | 层离屏池每桶 4 条 + 32MiB → 表值（bytes 走 `uint64`） |
| `render/internal/gpu/glyph_color_engine.go` | 4 页 → 表值 |
| `render/internal/gpu/glyph_mask_engine.go` | 新增 `newGlyphMaskAtlasFromBudget` 注入构造器：16384 条 / 4 页由调用方从表传入（`render/text` 不 import `render`） |
| `render/shared_budget.go` | divisor 与下限入表；`pictureCacheFairMaxN` 改调 `FairSplit` |
| `render/video_texpool.go` / `render/video_direct.go` | `videoPoolMaxSlots=8`、`videoDefaultMaxStaging=64MiB` 从 `video_direct.go` 搬到 `video_texpool.go` 并从表取值（同包、无 import 变化） |
| `render/device_open.go` | `OpenStandaloneDevice` 用 `DeviceDescriptorForAdapter`（P2.4a D3） |
| `render/shared_r0_test.go` | 引用改 `DefaultBudget`（断言值不变） |
| `render/budget_default_test.go`（新） | 声明表 pin：逐域锁死 11 组数值（含 `FairSplit` 三条边界） |
| `render/internal/gpu/budget_evict_test.go`（新） | 淘汰语义 10 个：depth-clip 四口径 / dualTex 字节+桶上限 / stencilPool LRU / glyph 上传节流三态 / ImageCache 公平份额分支 |
| `render/video_texpool_budget_test.go`（新） | video 池 `maxSlots` 驱逐 |

## 2 等价性证明（§4.1：先补后切）

- **先补**：`budget_default_test.go` 与 10 个淘汰语义用例先对**改动前常量/行为**跑绿；**再切**：12 处切表后同测试复绿（数字未动 = 行为未动）。
- **pin 锁值**：`DefaultBudget` 每个域逐字段断言等于评估表 §0 的现值；任何一处数字漂移都会红。
- **淘汰算法零改动**：只换常量来源与派生函数，LRU/TTL/桶尾丢弃等逻辑一行未改；新增用例锁的是既有行为（此前无测的 6 组缺口）。
- **负向对照**：临时把 `BudgetImageCache.MaxBytes` 64MiB→32MiB → pin 红 → 还原绿 → 残留 grep 零命中（`negative.txt`）。

## 3 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| 五目标交叉编译 | ✅ linux×2 / windows / darwin×2：段3 默认 + 段4 nogpu 全 exit 0（321/317、321/317、327/323、319/319、319/319 包） | `build/*.txt` |
| vet | ✅ 默认 61 / nogpu 58 条诊断，与 P2.4a **逐字节一致** | `vet-full.txt`（脚本 `tools/vet-full.sh`） |
| 残留 grep | ✅ 10 类旧常量声明全 0 命中；`DefaultBudget` 调用点 22；`ui` 直引 `gpu` 白名单 5（既有） | `grep-residual.txt` |
| API 快照 | ✅ `render` +6 声明行（`BudgetDomain`/`Budget` 2 type + `DefaultBudget`/`FairSplit`/`String` + 结构体口径差）；`render/internal/gpu` 零差异 | `api-diff.txt`、`api-render*.txt` |
| 总账同步 | ✅ `RENDER_API_CATALOG.md` 新增 Budget 行；`scripts/apidoc` rc=0。**本轮修正**：上一轮把 Budget 行与 StandaloneDevice 行用 `\|\|` 粘成一行（表形坏、apidoc 只查符号不查表形故未拦），已拆回两行并逐行核实 4 列对齐 | `apidoc-check.txt` |
| 逐测试文件 | ✅ 473 个：**423 PASS / 17 仅编译 / 1 标签排除 / 32 FAIL**，32 个全部定性为既有，零新增红 | `per-file-tests.txt`、`per-file-tests-notes.md` |
| `-race` | ✅ `render` 声明表/公平份额/video 池/设备入口 + `internal/gpu` 淘汰面双包全过 | `race.txt` |
| conformance（K1） | ✅ `TestConformanceDualBackend` PASS；K1 上界 ±2 条/131072B 维持在用例 JSON（未回退） | `conformance-rerun.txt` |
| 负向对照 | ✅ 改一个表值 → pin 红 → 还原绿 → 零残留 | `negative.txt` |
| 真窗 native | ✅ l1_blank 3571 帧 rc=0；multiwin 8 窗三相位 rc=0、账本稳态 316MB | `l1_blank_native.json`、`multiwin_native.json` |
| 真窗 ladder native | ⛔ 阻塞（既有缺陷，A/B 已证）：8 窗相位 `wgpuSurfacePresent` 不返回 | `vram_ladder_native.stderr`、`vram_ladder_hang-stack.txt`、`ab-clean-55495fdd.*`、`window-gates-rerun.txt` |
| 真窗 l1_blank go | ⛔ 既有 C7（A/B 已证）：`eglMakeCurrent 0x3002` → 开窗 rc=1 或关窗等锁死 | `l1_blank_go_c7-0x3002.stderr`、`l1_blank_go_hang-stack.txt` |

## 4 逐文件 32 FAIL 定性（零新增红）

- **31 个与 P1.3 基线逐文件同红**：`render/internal/gpu` 18（与 P2.4a 同集）、`render` 9（P2.4a 为 10，`mem_window` 并发竞态本轮未复现）、`ui/scene` 4（P1.3 §4.2 已登记）、`render/text` 1。
- **1 个**：`zz_b2_bitmap_scan_test.go` 为 per-file 脚本 20m 超时所杀；按 P1.3 对长跑文件的 3600s 口径单跑复验：**ok 1264.3s，4/4 PASS**（P1.3 同文件 1256.2s，同档）。
- 本轮脚本照 H1 纪律导出 `WGPU_NATIVE_PATH`，GPU 用例真跑，非无 GPU 假绿。

## 5 真窗与生命周期（隔离嵌套 Wayland）

隔离环境：`dbus-run-session -- gnome-shell --nested --wayland-display=<name> --no-x11`；当前桌面 X :1 只做健康检查，未在其上开窗。

| 轮 | rc | 关键结果 | 判定 |
|---|---|---|---|
| ui_l1_blank native | 0 | frame 3571 / present 3572 / missed_vsync 0 / hitch 9 | ✅ |
| ui_base_multiwin_cap native `-auto-only` | 0 | n=2 59.7 / n=4 58.5 / n=8 40.1 fps；账本 316MB 稳态；pixel 16/16；golden per-window diff 0 | ✅ |
| ui_base_vram_cap ladder native | 124 | n=2 fps 44.1 / 拒增 +0 / 91.5MB；n=4 fps 23.5 / 拒增 +472 / 181.5MB；n=8 fps 0.0 / 拒增 +0 / 201.0MB → 卡死 | ⛔ 既有缺陷（I1） |
| ui_l1_blank go | 124 / 1 | `eglMakeCurrent 0x3002`；或关窗 `AdapterContext` 等锁死 | ⛔ 既有 C7 |

**A/B（决定性）**：在 `55495fdd`（P2.4a 干净树 worktree）上用同一合成器、同一参数复跑：
- ladder：n=8 同样 `fps_min=0.0`、拒增 +0、203.4MB，栈同为 `rwgpu.(*Surface).Present` 里的 C 调用阻塞 7 分钟；
- l1_blank go：同样关窗死锁，栈同为 `gles.(*AdapterContext).Destroy`。
→ 两者均为**既有缺陷**，与 P2.4b 无因果。

**账面解读（能拿到的部分）**：ladder 前两相位已覆盖本步关心的预算行为——4 窗相位拒增压到 +472 且账本 181.5MB 未破 0.9 水线（768×0.9=691MB）；8 窗相位 `live=201.0MB` 同样远低于水线。8 窗拒增曲线与 hold 段 RSS 斜率因既有阻塞无法取得，登记 I1 待 P2.4e 修复后补跑（P2.6 全量重放必跑）。

金图说明：`ui_l1_blank` 设计内无金图（`baseline/windows.md`），替代判定 = JSON 探针；vram 窗 golden 对账归 P2.6。

## 6 归位与遗留

| 项 | 去向 |
|---|---|
| **I1（新）** vram ladder native 8 窗 present 卡死（Wayland） | 台账 §2.2 待办池；拟归 **P2.4e**（后端呈现入口/多表面 present），解剖前不定修复方向；修复后补跑 8 窗拒增曲线与 hold 斜率 |
| **C7**（已有）l1_blank go 单窗 0x3002 | 既有条目追加 Wayland 单窗形态证据；修复仍归 P2.4e |
| H1（已有）GPU 包逐文件必须带 lib | 本轮已遵守 |
| 运行时每域用量聚合 | P2.4c/d（res/registry 归一） |
| TexturePool 删池 | P2.4d |
