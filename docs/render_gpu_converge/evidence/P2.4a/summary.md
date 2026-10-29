# P2.4a 收口报告：设备创建/选卡收敛（三份 → 一份）（2026-10-30）

> 任务：台账 §2.1 P2.4a；方案：PLAN §1.3/§2.2-B1（`adapter_policy.go` 定策略 + 创建入口唯一）。
> 前置：评估 `assessment.md`（方向 D1–D5 经用户拍板：方案 A 三处全收敛、Vulkan 掩码钉死、descriptor 口径归 P2.4b、A3/C7 先解剖按根因归属、`render/gpu` 包装标实验）。
> 性质：P2 行为零变化子步（vello 独立路径获得 policy 选卡属 D1 拍板语义收敛，本机单 GPU 选卡结果不变，见 §1）。

## 0 结论（先看这）

| 项 | 结果 |
|---|---|
| 创建入口 | `webgpu.CreateInstance` 白名单 **3 → 1**（`render/backend.go:118 createNativeInstance` 唯一句）；`render/internal/gpu` 的两处直调（vello_accelerator.go:1330、gpu_shared.go:769）删除 |
| 唯一独立入口 | 新增导出 `render.OpenStandaloneDevice(label) (*StandaloneDevice, error)`（`render/device_open.go`）：严格 Vulkan 掩码（env 不劫持）+ `RequestAdapterWithPolicy` + OOM 重试 + 半开自动回收；gpu_shared 与 vello 两处独立建链全部改调 |
| 重试助手归一 | `requestDeviceWithRetry` 收敛为一份（`render/device_open.go`），present 链（present_construct + shared_recovery）与独立入口共用 |
| 删除面 | `render/internal/gpu/device.go`：2 组镜像（`renderDeviceDescriptor`/`requestDeviceWithRetry` + 双常量）+ 6 死代码符号（`GPUInfo`/`getGPUInfo`/`logGPUInfo`/`createDevice`/`getDeviceQueue`/`releaseAdapter`/`releaseDevice`）；含测试全仓零引用已证 |
| A3（46 个 RequestDevice SKIP） | ✅ 关闭：真因是显存 OOM 被 wgpu 包装成 Validation Error（`Caused by: Not enough memory left`），46/46 同因，环境依赖非代码缺陷，见 `a3-dissection.md` |
| C7（GL 第 4 窗 0x3002） | 🔍 根因落定：开窗的 EGL 绑定操作 × 唯一 GL 线程（ui/raster/hub）Submit keep 窗口期的竞态；修复归 **P2.4e**（推荐开窗 GL 步骤投递 hub FrameJob），见 `c7-dissection.md` |
| 台账新增审计发现 | P2.1/P2.3 的 `render/internal/gpu` 逐文件轮是无 GPU 假绿（脚本未带 `WGPU_NATIVE_PATH`，GPU 用例全 SKIP 文件级 rc=0）；本轮带 GPU 重跑恢复 P1.3 纪律，登记待办池（P2.6 全量重放必须带 lib 或显式记「无 GPU 轮」） |
| 状态 | 门禁 8/9 绿 + 真窗/生命周期证据齐（§3/§5）；逐文件 28 FAIL 全部定性为既有（§4）；生命周期 go 档红为 C7 既有根因复现（有解剖、归 P2.4e，非本步回归），按 P2.1「既有环境红」先例随本步标 ✅，C7 自身保持 ⬜（P2.4e） |

## 1 改动清单（11 文件 + 2 新增）

| 文件 | 改动 |
|---|---|
| `render/backend.go` | `SelectBackend` 原样保留（env 胜出、BackendsPrimary）；新增包内 `createNativeInstance(mask)`——全仓唯一 `webgpu.CreateInstance` 句 |
| `render/device_open.go`（新） | `StandaloneDevice` 结构 + `OpenStandaloneDevice`（唯一独立入口）+ `requestDeviceWithRetry`（唯一重试实现，自 present_construct 上收） |
| `render/present_construct.go` | 删 `requestPresentDeviceWithRetry` 副本，改调归一后的 `requestDeviceWithRetry`（同体同文） |
| `render/shared_recovery.go` | 同上（改调用名） |
| `render/internal/gpu/gpu_shared.go` | `initGPU` 自开分支 → `render.OpenStandaloneDevice("gg-shared")`；借共享分支原样；CPU 适配器检测/软件模式判定原样 |
| `render/internal/gpu/vello_accelerator.go` | `initGPU` → `render.OpenStandaloneDevice("gg-vello")`（D1：裸 `RequestAdapter(nil)` 改 policy 驱动）；新增 `adapter` 字段随 Close 释放（原为泄漏的局部句柄，资源卫生微修）；其余生命周期原样 |
| `render/internal/gpu/device.go` | 删 2 组镜像 + 6 死符号（§0 删除面）；`Backend` 保留（P2.3 拍板）；`Backend.Close` 内联 `releaseDevice` |
| `render/internal/gpu/native_device_test.go` | `renderDeviceDescriptor` → `render.DeviceDescriptor`（逐行同体，零行为） |
| `render/gpu/gpu.go` | `SetDeviceProvider`/`AbandonDevice` doc 标 EXPERIMENTAL（D5：生产未接线，接线决策归 embedder/P3+） |
| `render/device_open_test.go`（新） | 能力单测：重试单测 4（OOM 重试/耗尽/非 OOM 不重试/nil）+ 真机探针（无 lib/显存压力诚实 SKIP）+ 错误链分类锁定 |

行为变化面（D1 拍板范围内）：混合显卡机器上 vello compute 独立路径的选卡从裸 `RequestAdapter(nil)` 变为 policy 驱动（Default 混合偏好 + OOM 降级阶梯）；本机（单 Vulkan 设备 940MX）选卡结果与像素门禁不变。vello Close 新增 adapter 释放（资源账本更干净，无渲染行为影响）。

## 2 等价性证明

- **代码等价**：`renderDeviceDescriptor` ≡ `render.DeviceDescriptor`（逐行同体，常量同值 9）；`requestDeviceWithRetry` ≡ 原 `requestPresentDeviceWithRetry`（同逻辑；仅删一行镜像侧的重复 Warn 日志，信息由入口侧错误链承载）。
- **双分支真机验证**：vello 分支 `TestRenderSceneAuto_*` 3 PASS + `render/examples/compute_pipeline` 烟跑（CPU/GPU 像素逐点一致，rc=0）；GPUShared 分支 `TestGoldenSoftware_GPUvsCPU`（直调 `initGPU`）+ `TestLinearDiagonalRampMatchesColorAt`（走 `ensureGPU`）PASS。
- **能力单测**：`render/device_open_test.go` 5 用例 PASS（含真机探针 `TestOpenStandaloneDevice_UsableDevice`：真机建设备 + 1x1 纹理探针 + 逆序释放）。
- **A3 佐证**：收敛不动 OOM 判定——`IsGPUOutOfMemory` 对 "Validation Error + Not enough memory left" 包装仍命中（重试/降级健壮），单测锁定。

## 3 门禁结果

| 门禁 | 结果 | 证据 |
|---|---|---|
| 残留 grep | ✅ 漏斗唯一、删除符号零引用、分层干净 | `grep-residual.txt` |
| 五目标交叉编译 | ✅ linux×2 / windows / darwin×2，默认段3 + nogpu 段4 全 exit 0（321/317、321/317、327/323、319/319、319/319 包）；段1/2 仅 `ui_wr_t_stress`（缺组件）与 `gpupixel`（仅测试）既有红 | `build/<目标>.txt` |
| vet | ✅ 默认 61 / nogpu 58，与 P2.1 基线**逐条一致**（diff 为空） | `vet-full.txt` |
| API 快照 | ✅ render +1 func（`OpenStandaloneDevice`）+1 type（`StandaloneDevice`）；render/gpu 签名零变化纯注释（D5）；render/internal/gpu 导出面 −1（`GPUInfo`）+ 8 个未导出删除由 grep 证；重试助手保持未导出（比评估预案少 1 个公共函数） | `api-diff.txt` |
| 逐测试文件 | ✅ 321 文件逐个跑（render 189 + render/internal/gpu 131 + render/gpu 1）：**280 PASS / 12 仅编译 / 1 标签排除 / 28 FAIL 全部定性为既有**（27 与 P1.3 基线逐文件一致，1 为并发挤压下的 X11 竞态、单跑 7/7 PASS），见 §4 | `per-file-tests.txt` |
| 负向对照 | ✅ 5 步：绿 → 探针引用已删符号编译红（5 个 undefined）→ 撤销 → 绿 → 零残留 → 漏斗唯一守卫 | `negative.txt` |
| `-race`（P2.4 附加） | ✅ CGO_ENABLED=1：render 能力面（入口/policy/backend）+ internal/gpu 双独立分支 + 设备消费代表用例全过无竞态报告 | `race.txt` |
| 总账同步 | ✅ `RENDER_API_CATALOG.md` 新增 `StandaloneDevice`/`OpenStandaloneDevice` 行；`scripts/apidoc` rc=0 | `apidoc-check.txt` |
| 真窗三证据 | ✅ ui_l1_blank native+go 双档（嵌套 Wayland 隔离，60s/档）：JSON 探针齐、rc=0，见 §5 | `l1_blank_*.json/.stderr` |
| 生命周期 | 🟡 multiwin native ✅（8 窗账本稳态、门禁自判 PASS）；multiwin go ❌ 第 8 窗 0x3002 = **C7 既有根因复现**（Wayland 面，与设备创建收敛无关，已修正解剖报告并归 P2.4e） | `multiwin_*.json/.stderr` |

金图说明：ui_l1_blank 按 `baseline/windows.md` **无固定金图**（设计内，能力标签「窗口/平台 · 开窗清屏上屏」）；替代判定 = 窗口 JSON 探针（frame/present/hitch 计数）+ P1.4 同窗 PASS 对账；真窗重放逐位对账归 P2.6 全量重放。

## 4 逐文件 FAIL 定性（28 个）

27 个文件与 P1.3 基线（`baseline/tests/summary.tsv`）**逐文件同为 FAIL**，归属已在 P1.3 §4 登记：共享布局未 pin（bindgroup_switch/convex_renderer/depth_clip/opt21/27/28/29/34/36/38/40/render_session/sdf_render/stencil_*，归 P2.5 测试卫生）、真实渲染回归（context_clip_depth/context_coverage/context_misc_draw_gpu/rasterizer_tile_value/text_transform_gpu/ttc_cjk_gpu_repro/video_direct_upload/filter_present_coherence/gpu_texture，归 P2.6/P4）、环境类（m5_color_glyph：P2.3 已定性为环境性 FAIL）、s6_9_heavy_budget（P1.3 同红）。

1 个差异：`mem_window_x11_linux_test.go`（P1.3 PASS）本轮串行批跑 FAIL（`BeginFrame: hal: surface outdated`）——**单跑复测 7/7 PASS**，批跑时与五目标构建门禁并发（CPU 挤压）触发的 X11 resize churn 竞态，非本步新增红（本步 present 链仅重试助手改名，同体执行）。

**附带审计发现（重要）**：P2.1/P2.3 的逐文件轮对 `render/internal/gpu` 记了 0 FAIL，但其脚本未导出 `WGPU_NATIVE_PATH`——native GPU 用例全部 `t.Skipf`（CreateInstance unavailable）→ 文件级 rc=0 假绿。本轮带 lib 重跑恢复了 P1.3 的真实红面（27 个），证明**逐文件门禁对 GPU 包必须带 `WGPU_NATIVE_PATH`，或显式记「无 GPU 轮」**——登记待办池，P2.6 全量重放必须遵守。

## 5 真窗与生命周期（隔离显示）

隔离环境：嵌套 gnome-shell Wayland（`--no-x11`，display `gpui-p24a`）——文档化隔离方案（`ENGINE_WAYLAND_NESTED_TEST.md` 同族）；其内 Xwayland 因 dbus-run-session 无 user systemd 无法启动（日志在案），故真窗走 Wayland-native（`baseline/windows.md` ui_l1_blank 平台行「X11 或 Wayland/native」允许）。批量 8 窗全程未触当前桌面（:1 只做健康检查）。

| 轮 | rc | 关键探针 | 备注 |
|---|---|---|---|
| ui_l1_blank native | 0 | frame 3582 / present 3583 / hitch 5 / missed_vsync 0 | present 链经收敛后入口建链 |
| ui_l1_blank go | 0 | frame 3596 / present 3597 / hitch 0 / missed_vsync 0 | 首跑 rc=124 为脚本 timeout 预算不足（编译+60s>90s），加预算复跑通过，证据已覆盖 |
| multiwin native `-auto-only` | 0 | 8 窗三相位（n=2 fps 59.3 / n=4 49.3 / n=8 37.9），rss 313→316MB 稳态，门禁自判 PASS | 8 窗批量 + 借共享不变式 + 账本稳态 |
| multiwin go `-auto-only` | 1 | 第 8 窗打开 `eglMakeCurrent 0x3002` | **C7 复现（Wayland 面，窗号浮动）**：X11 机制链已解剖落定，Wayland 为第二个待解剖机制（context-per-surface 下仍触发），修复归 P2.4e（见 `c7-dissection.md` 修正段）；7 窗借共享工作正常，失败与设备创建收敛无关 |

（本轮数据落盘 `l1_blank_*.json/.stderr`、`multiwin_*.json/.stderr`、汇总 `window-gates.txt`；标准退出：门禁窗自走 RUN_SECONDS 结束，无硬杀。）

## 6 与台账/池的联动

- §2.2 A3 → ✅（解剖闭合，`a3-dissection.md`）；C7 → 根因落定、修复归 P2.4e（`c7-dissection.md`）。
- 新增池项（本轮发现）：P2.1/P2.3 逐文件轮无 GPU 假绿（gate 纪律），P2.6 重放前修。
- §2.1 载体登记的兑现情况：能力单测 ✅（device_open_test + 321 文件逐个）；真窗 ✅（ui_l1_blank 双档 + multiwin 双档，隔离显示）；金图 —（ui_l1_blank 设计内无金图，替代判定已写明）；双后端 ✅（native+go 真窗双档、metal 交叉编译 darwin×2、本机不可运行已标注）；负向对照 ✅；`RENDER_API_CATALOG.md` 已同步 + apidoc rc=0；multiwin 门禁轮改写的 8 张 `testdata/multiwin_win*.png` 快照已 `git restore` 还原（验证副产物，不入提交）。

## 7 遗留与移交

- P2.4e：C7 修复（推荐开窗 GL 步骤投递 hub FrameJob；验收 = ladder go 档 8 窗跑通）+ 开窗路径上 unbound-context 探测空值伴生现象一并消除。
- P2.4b：standalone 与 present 的 descriptor 口径统一（`DeviceDescriptorForAdapter` 水位联动是否覆盖 standalone）。
- P2.5：共享布局未 pin 等 27 个既有测试卫生 FAIL（P1.3 §4.2 已登记）；`native_device_test.go` skip 文案补充 OOM 说明；逐文件门禁 GPU 包带 lib 的纪律入脚本。
- P3+：`render/gpu.SetDeviceProvider/AbandonDevice` 的 embedder 接线决策（本步已标实验）。
