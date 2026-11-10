# P4.3 步骤②③：DrawShadow 层合一实现尝试与回退（2026-11-10）

> 性质：**失败记录 + 回退结论**（§6.8 失败不掩盖）。用户硬约束「搞不定就如实说
> 没修好并回退，禁止交付留残缝/半吊子改动」。本文件记录尝试过程、客观数据、
> 回退清单与下一步。① 分段归因成果不受影响（见 `stage1-timing/attribution.md`）。

## 0 拍板与方案

用户两次拍板：① 后拍板 **bounded layer RT** 方向；影响面评估后拍板
**②③ 在 DrawShadow 层合一**（缓存紧致界大小的阴影 GPU view，命中直接贴图）。
理由：静态帧连 66ms 的隔离层 GPU↔CPU 往返都不付；缓存内容本身即「按界裁剪」
的结果；不碰 WGSL/graph 核心、零公开 API。

## 1 落地内容（已全部回退，见 §5）

- `render/shadow_cache.go`（新）：`shadowKey`（path 内容哈希/elevation/color/
  `totalMatrix()`/dpr/紧致界，**禁指针键**）、`shadowEntry`（独立
  `CreateOffscreenTexture` GPU view + release）、LRU（32 条 / 32 MiB 上限）、
  `shadowDeviceBounds`（path 设备界 + offsetY + 3σ pad + clip/surface 相交）、
  `cacheShadowTile`（从隔离层 pixmap 抠紧致界 → `UploadRGBAToView` 上传）、
  `releaseShadowCache`（Context.Close 释放，谁建谁放）。
- `render/context_misc_draw.go`：DrawShadow 命中查缓存 → 贴图 return；未命中
  走原链，在 `ApplyBlur` 后 `PopLayer` 前存 tile。投影分支（#4 洞族）显式不缓存。
- `render/context.go` / `context_construct.go`：缓存字段 + Close 释放。
- `render/p43_shadow_blur_perf_test.go`（新）：3 枚能力单测 + `GPUI_P43_BREAK`
  mutation 开关。

## 2 单元级证据（绿，但不足以收口）

| 断言 | 实现前 | 实现后 |
|---|---|---|
| 静态二帧第二帧 < 首帧 1/4 | FAIL ratio=0.799 | PASS first=59.7ms second=13.6ms ratio=0.227 |
| 裁剪/缓存前后像素逐位一致 | FAIL max diff=48（既有帧间非确定性） | PASS diff=0（缓存顺带消除该抖动） |
| 命中/失效（elevation/颜色重算） | — | PASS |
| mutation `GPUI_P43_BREAK=no-cache` | — | 反证红 FAIL ratio=0.767 |
| 命中日志 | — | `cache store 280×210` / `cache hit`，紧致界确实生效 |

双构建（默认 / `-tags nogpu`）rc=0。

## 3 真窗证据（native，单窗 `-auto-only` 标准关闭、前后查显示健康）

| 运行 | golden diff | fps_interval | interval_p95 | presents | 结论 |
|---|---|---|---|---|---|
| 缓存开启 | **33.0371% FAIL** | 59.3 | 18.97ms (<22ms 线) | 356 | fps 达标、画面 FAIL |
| `GPUI_P43_BREAK=no-cache` | 0.0000% | 3.36 | 310.8ms | 24 | 画面正确、原症状 |
| `GPUI_P43_BREAK=store-only`（存不命中） | 0.0000% | 3.36 | 310.8ms | 23 | 存储动作无害 |

**缓存确实在窗口链命中**：`cache hit` 2492 次 / `cache store` 7 次（6 阴影 + 1），
证明 Context 跨帧复用，静态帧只算一次。

## 4 客观定位（不用 mean，逐像素）

- `gold vs nocache`：0.1888%，bbox (192,741)-(429,778) = HUD 数字摆动（门禁排除区）。
- `gold vs cache`：27.12% 全窗（门禁区 33.04%），bbox (205,82)-(1167,775)。
- 差异幅度直方图：白底区 1–4 级差（1 级 33409 px、2 级 16705 px…）；非白底区
  5–12 级差。样本 `(475,120)` nocache (246,246,246) → cache (255,255,255)：
  **阴影淡区被冲成纯白**。阴影本就很淡（tonal 25% + blur 扩散），「阴影没画出来」
  正好产生这个量级。
- `cacheShadowTile` canvas dump：tile 内容是 `(0,0,0,a)` 的 premul 黑阴影、
  alpha 1~25 渐变，**内容本身正确**。
- `red-tile` 诊断（tile 填不透明纯红）：快照里只有 **1272 / ~58800** 个红像素，
  即命中贴图**几乎没进最终呈现帧**。
- `gpu_ops 196→197`、view 非 nil：贴图命令确实提交，但结果不呈现。

**六种命中贴图方式全部失败，golden diff 恒定 ~33%**：子区域 quad → 33.0371%；
半 texel UV inset → 35.05%；全画布 view + 整层 blit（x=0,y=0,W,H）→ 33.0371%；
绕 clip `drawGPUTextureWithOpacityFullSurface` → 33.0371%（gpu_ops 确认提交）；
`PushLayerIsolated` 层内贴图 + `PopLayer` → 34.57%。

**根因归类**：不是采样/通道/坐标问题（全画布整层 blit 与 PopLayer 完全同形仍失败），
而是 **P4.1 批 3 已登记在册的 GPU 层链洞 #4/#5 族**（「层合成统一在 gpuTex tier
末尾重放」「投影×隔离层×blur×GPU readback 内容不同步」）：在 DrawShadow 内部直接
queue 的 GPU texture draw 无法可靠进入 present 帧。用户交接明确「别顺手修」该线，
且 AGENTS 禁止在示例层绕引擎洞、禁止猜着修——六种试错后停止。

## 5 回退清单（本会话 ②③ 实现，已全部撤回）

- 删：`render/shadow_cache.go`、`render/p43_shadow_blur_perf_test.go`
- `git checkout --`：`render/context.go`、`render/context_construct.go`、
  `render/context_misc_draw.go`
- **保留（① 成果，env 门控默认关、零行为变化）**：`render/p43_timing.go`、
  `render/internal/gpu/p43_timing.go`、`render/filter_ops_gpu.go` 与
  `render/internal/gpu/filter_gpu_graph.go` 的 12 处分段计时点、
  `render/p43_rough_probe_test.go`、`render/testdata/p43_timing_cases.json`
- 回退后：`go build ./render/...` 与 `-tags nogpu` 双 rc=0，
  `TestP43RoughProbe` 仍 PASS（① 可复现）。

## 6 状态与下一步

- ① 分段归因：✅ 完成（正本 `stage1-timing/attribution.md`）。
- ②③：**没修好，已回退**；P4.3 保持 🟡。金图双后端 0.0000% 锁定未重种。
- 原症状（330ms/帧 ≈3fps）仍在，等待下一步。
- 下一步候选（**都要先做影响面评估 + 停在「停下报告」经用户拍板**）：
  1. 先修底层 GPU 层链洞（P4.1 #4/#5 族，属在册别线），让 DrawShadow 内部的
     GPU texture draw 能可靠进 present 帧，再回来做 ②③；
  2. 换命中实现路径：缓存 CPU 侧紧致界 `[]byte`，命中时把 tile 写回隔离层
     pixmap 走 `PopLayer` 的 **CPU composite**（不经 GPU texture draw tier）；
     需先验证双后端逐位一致与 GPU pending 语义，且要重新做成本断言。
