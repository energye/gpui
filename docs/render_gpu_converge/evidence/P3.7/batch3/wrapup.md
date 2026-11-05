# P3.7 批 3 收尾 · 四扇 scene 示例归位 + 离屏腿观测口（2026-11-05）

拍板（用户）：四扇 scene 示例走**诚实化 + 改登记**；`_gpu` 示例报假 `gpu_ops` 这条**本批直接修**；
scene 离屏「走了哪条腿 + 真 GPU op 数」**引擎补可读出口**；GPU 腿早退 stats 不重置**本批一并修**。

---

## 1 台账原登记的定性是错的（先纠事实）

§2.1 登记的工程债原文：「四扇 scene 示例窗 `render_scene`/`render_scene_gpu`/`render_visual_scene`/
`render_visual_scene_gpu` 缺 `-auto-only` 标准关闭自检 ⇒ 进不了窗口门禁」，批 3e-2 那行还补了一句
「无 `-auto-only`（不具备标准关闭自检，按 GPU 安全条不能强杀）⇒ 本步以编译+包内测覆盖」。

实测：**这四扇一扇都不是窗**——四份 main.go 里都没有 `platform.Open`、没有事件循环，跑完自己退出：

```
render_scene              rc=0 :: Dirty Render: 4 tiles in 4.66ms / Incremental 93.8% fewer tiles
render_scene_gpu          rc=0 :: Rendered via: ... / Time: 354ms / Saved: tmp/render_scene_gpu.png
render_visual_scene       rc=0 :: accelerator=<nil> direct=false tiles_total=64 tiles_rendered=64
render_visual_scene_gpu   rc=0 :: accelerator=<nil> direct=false ... 
```

所以「不能强杀、需要标准关闭自检」对它们不成立（没有窗口可关），`-auto-only` 也不适用。
scene 的像素门禁**早有真窗在做**：`examples/ui_render_base_scene`（带 `-auto-only`、testdata golden、
覆盖 SceneBuilder/Scene/Encoding/Decoder/GPUSceneRenderer/Renderer 瓦片腿/LayerCache/DamageTracker）。
这四扇与它能力重复，登记为「离屏出图示例（原子验证），非真窗，不进窗口门禁」。

## 2 顺着查出来的真问题：示例在报假数

| 位置 | 内容 | 性质 |
|---|---|---|
| `examples/render_visual_scene/main.go:51` | `fmt.Println("gpu_ops=0 cpu_fallback_ops=0 note=scene_renderer")` | 字面量冒充测量 |
| `examples/render_visual_scene_gpu/main.go:53` | 同上 | 字面量冒充测量 |
| `examples/render_visual_scene_gpu/main.go:47-51` | `TilesRendered == 0 ⇒ path=gpu` | 无效判据 |
| `examples/render_scene_gpu/main.go:50-54` | `TilesRendered == 0 ⇒ "Rendered via: GPU"` | 无效判据 |

判据为什么无效（引擎侧根因）：`Renderer.RenderWithContext` 的 GPU 优先分支是

```go
if render.Accelerator() != nil {
    if err := r.renderGPU(target, scene); err == nil {
        return nil            // ← 早退，不碰 r.stats
    }
}
r.stats = RenderStats{}       // ← 只有 CPU 分支才重置
```

1. GPU 腿早退时**不重置 stats** ⇒ 上一帧 CPU 的 `TilesRendered=64` 会残留进 GPU 帧的 `Stats()`；
   「0 块瓦片」既可能是「这帧走了 GPU 腿」也可能是「这帧啥也没画」，反推不出腿。
2. GPU 腿内部逐形状的 CPU 回落发生在 `render.Context` 里，`RenderStats` 一个字段都不露；
   真计数器只在 `dc.RenderPathStats()`（`render/context_gpu.go:24` 的 `GPUOps`/`CPUFallbackOps` 自增），
   而那个 `dc` 在 `renderGPU` 函数内就 `Close()` 了 ⇒ 示例无处可取，只能硬印 0。

带标签实测（`-tags gpui_visual_gpu`，`WGPU_NATIVE_PATH` 已导出）：

```
accelerator=sdf-gpu direct=false
path=gpu tiles_rendered=0
gpu_ops=0 cpu_fallback_ops=0 note=scene_renderer   ← 仍是那句字面量
```

`direct=false` 说明**GPU 腿被采纳 ≠ 真 GPU 求值**：形状在 Context 里逐个回落 CPU。
这正是「示例报的 0 既不是 GPU 数也不是 CPU 数」的形态。

## 3 修法（引擎侧，位置在事实所在处）

`render/scene/renderer.go`：

- `renderGPU` 返回 `(render.RenderPathStats, error)`：`FlushGPU()` 之后、`dc.Close()` 之前取计数。
- `RenderStats` 增四枚字段：`GPULegTaken bool` / `GPUOps int` / `CPUFallbackOps int` / `LastCPUFallback string`，
  注释写死「`TilesRendered == 0` 不是走过 GPU 的证据」。
- 新增 `resetStats()`，`RenderWithContext` 与 `RenderDirtyWithContext` 两个入口都在起帧时调它
  （原来只有 CPU 分支重置，脏区分支自己重置，GPU 分支不重置 ⇒ 三个入口三种口径）。
- GPU 腿失败回落瓦片腿时**不写** GPU 计数：`Stats` 只报真正出了图的那条腿。

示例侧（本批收尾改）：删两行硬印的假 `gpu_ops`，两扇 `_gpu` 的「0 块瓦片 ⇒ GPU」判据换成
`gpu_leg=%v gpu_ops=%d cpu_fallback_ops=%d last_cpu_fallback=%q` 真字段。

## 4 钉（`render/scene/gpu_leg_stats_test.go`，替身加速器 = 每形状 `ErrFallbackToCPU`）

- A `TestP37StatsReportsTileLeg`：无加速器 ⇒ `GPULegTaken=false`、`TilesRendered>0`、GPU 计数必为 0。
- B `TestP37StatsDoNotLeakAcrossLegs`（残留洞）：同器先瓦片帧 ⇒ 注册替身再 GPU 帧 ⇒ `GPULegTaken=true` 且
  `TilesRendered` 必归 0（不许带上一帧的数）；替身确实收到过形状；`CPUFallbackOps` 必须非零
  （钉「计数真从 Context 转交上来」，不是又一位硬印 0）；撤掉替身再回瓦片帧 ⇒ GPU 计数清零。
- C `TestP37StatsResetOnDirtyEntry`：GPU 帧之后走 `RenderDirty` ⇒ `GPULegTaken` 与 GPU 计数不残留。

### 负向（全部打断→红、还原→绿，实测原文）

| 号 | 打断点 | 变红的钉与真实输出 |
|---|---|---|
| N10 | 摘 `RenderWithContext` 入口的 `resetStats()` | B：`GPU 帧残留了上一帧的瓦片数：16（want 0）——Stats 不可信，「0 块瓦片⇒GPU」的判据就是这么来的` |
| N11 | 摘 GPU 计数转交里的 `CPUFallbackOps` 一行 | B：`Context 的回落计数没转交上来：{... GPULegTaken:true GPUOps:0 CPUFallbackOps:0 LastCPUFallback:fill:gg: falling back to CPU rendering}` |
| N12 | 摘脏区入口的 `resetStats()` | C：`脏区帧带着上一帧 GPU 计数：GPULegTaken=true ops=0/2` |

N11 首写法（把三行一起摘）会让 `ps` 变成未使用而编译失败 ⇒ 那种「红」不是行为红；改成只摘单行后拿到真实红，
如实记这条无效变异。

### 示例侧实测（改完之后）

```
render_scene             Render Statistics:（纯瓦片腿，本来就无假数）
render_scene_gpu         gpu_leg=true gpu_ops=21 cpu_fallback_ops=0 last_cpu_fallback="" tiles_rendered=0
render_visual_scene      tiles_total=64 tiles_rendered=64 gpu_leg=false gpu_ops=0 cpu_fallback_ops=0
render_visual_scene_gpu  gpu_leg=false gpu_ops=0 cpu_fallback_ops=0 tiles_rendered=64
render_visual_scene_gpu（-tags gpui_visual_gpu）
                         accelerator=sdf-gpu direct=false → gpu_leg=true gpu_ops=21 cpu_fallback_ops=0
```

同一段场景、同一台机器，腿身份与 op 数第一次有了可比的真值：不导加速器 ⇒ `gpu_leg=false / tiles=64`；
导了 ⇒ `gpu_leg=true / gpu_ops=21 / tiles=0`。这正是示例当年硬印 `gpu_ops=0` 所掩盖的差别。

## 5 门禁实测（2026-11-05）

- 逐文件全量 sweep（干净重跑，37 包 824 文件）：**792 PASS / 8 FAIL / 23 SKIP_TAGGED / 1 NO_TESTS**；
  相对 batch2 基线（729/9/21/1）**零新增红**，PASS +63 全部来自名单补上的 render/scene+recording+raster
  （55 PASS/2 SKIP_TAGGED）与本批新增测试文件。基线红名单里 `render/s6_9_heavy_budget_test.go` 本轮绿——
  它是按 `tmp/` 同日机器基线浮动的计时门（p50 阈值，连续三次复跑虽稳），**不计为本批收益**，
  如实登记为门禁抖动源。
- **名单洞（本批补）**：`tools/per-file-tests.pkgs` 原为 34 包（render→gpu 主链 9 + ui 25），
  而批 3 整批的改动落在 `render/scene`(31 测试文件)/`render/recording`(19)/`render/recording/backends/raster`(6)
  三包——**此前逐文件门禁对本批改面是零覆盖**。收尾把三包补进正本清单（现 37 包）并重跑，
  三包实测 55 PASS / 2 SKIP_TAGGED（`render/scene/benchmark_test.go`、`render/recording/backend_stub_test.go` 无 Test 函数）。
- 五目标交叉构建（尺子沿用批 1d/批 2）：linux/amd64 335·331、linux/arm64 335·331、windows/amd64 341·337、
  darwin/amd64 333·329、darwin/arm64 333·329（默认/nogpu 包数与批 2 同尺子基线逐目标齐平），
  `{默认,nogpu}` **10/10 绿、加载失败 0**。附带如实记两条旧口径：本轮 raw `go build ./...` 段在五目标也全 rc=0，
  因为 P1.1b 时代登记的「已知问题包」`examples/ui_wr_t_stress` 目录已不存在（T 系列窗已改名为
  `ui_wr_c9_stress_cache`），剩下的 `ui/scene/internal/gpupixel`「无编译 Go 文件」被可构建清单自动排除；
  脚本里那句 `expected 1: known-broken pkgs` 是旧注释，不改尺子定义、只按实测登记。
- `go build ./...` rc=0；`go vet ./render/scene ./render/recording ./examples/render_scene_gpu
  ./examples/render_visual_scene ./examples/render_visual_scene_gpu` rc=0；gofmt 触及文件净。
- `CGO_ENABLED=1 go test -race ./render/scene ./render/recording ./render/recording/backends/raster` 全 ok。
- `go run ./scripts/apidoc` rc=0（`RenderStats` 新四字段与离屏腿身份行登记见目录 §6.2）。
- 真窗：本批收尾**未新开窗**——四扇经实测不是窗（无 `platform.Open`、无事件循环、跑完自退出），
  不存在「标准关闭自检」这一说；scene 的像素门禁仍由既有真窗 `examples/ui_render_base_scene`
  （带 `-auto-only`、testdata golden、全族覆盖）承担。3e-3 那枚真窗 `ui_wr_p32_shader -auto-only`
  的结果见 `batch3e3/negative.txt` §5。

## 6 口径与遗留

- 四扇 `render_scene`/`render_scene_gpu`/`render_visual_scene`/`render_visual_scene_gpu` 按实测**改登记**为
  「离屏出图示例（原子验证），非真窗，不进窗口门禁」；台账 §2.1 那条「缺 `-auto-only` ⇒ 进不了窗口门禁」的
  定性作废，按本节事实重写。
- 示例层不做像素门禁（与 `ui_render_base_scene` 重复），本批只把假数换成真数——拍板见本文开头。
- 遗留（不改本批结论）：`RenderWithDamage` 与 `Render` 两条入口的腿优先级仍只在 `RenderWithContext`
  里做 GPU 优先（脏区/damage 走瓦片腿），这是既有设计；若批 6 要把材质/多纹理摆进真窗，脏区腿的
  材质表态需要一并核（本批 CPU 腿五处拒画只覆盖瓦片腿）。
