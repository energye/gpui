# RECHECK2 动态验证报告（真跑实测）

> 日期：2026-08-24。方法：全部命令真实执行（go1.25.5 linux/amd64），探针程序在 /tmp 下独立 module 真跑，非静态读码。
> 探针源码：`/tmp/x04probe/main.go`、`/tmp/x04t/main.go`（截断斜率法）、`/tmp/x05probe/main.go`。

## 1. 编译矩阵

### go build ./...

实际红灯包共 **4 个**：

| 包 | 错误 |
|---|---|
| `render/tmp_ref_stroke` | `main redeclared in this block`（main.go:14 与 contours.go:12 重复声明）|
| `render/tmp_ref_stroke/wind` | `main redeclared in this block`（wind/main.go:12 与 sqrtest.go:10、verbs.go:11 重复）|
| `tmp_c4dbg` | `main redeclared in this block`（col_profile.go:12 与 framediff.go:30、peek.go:10 重复）|
| `tmp_vlprobe` | `owner.LayerCache undefined (rendering.PipelineOwner 无此字段/方法)` main.go:76；`undefined: rendering.BuildFramePacketCached` main.go:102 |

一致性判定：**部分一致**。台账预期 tmp_vlprobe + render/tmp_ref_stroke 两处红；实际多出两处：`render/tmp_ref_stroke/wind`（可视为 tmp_ref_stroke 一族）与 `tmp_c4dbg`（**多出来的新红**）。均为临时调试包的 main 重复声明，不涉及正式代码。

### go vet ./...

实际红灯包共 **8 个**（vet 输出为警告+错误混合）：

| 包 | 问题 |
|---|---|
| `render/tmp_ref_stroke` / `render/tmp_ref_stroke/wind` / `tmp_c4dbg` / `tmp_vlprobe` | 同 build（编译都过不了 vet 必红）|
| `examples/ui_wr_r19_snap` | fmt.Sprintf `%d` 传了 float64 类型（SnapCoord）main.go:141 |
| `render/internal/gpu` | gpu_render_context.go:3603、3651 两处 unreachable code |
| `render/internal/gpu/res` | view_test.go:13,14 possible misuse of unsafe.Pointer |
| `ui/platform` | 大量 wayland_*.go unsafe.Pointer 警告 + 测试文件同类警告 |
| `ui/rendering` | text.go:592 self-assignment of s to s |

一致性判定：**不一致**——vet 红 8 个包，远多于「2 处」。其中 `render/internal/gpu`、`ui/platform`、`ui/rendering` 属正式代码（多为告警级：unsafe 用法提示、自赋值、不可达代码），此前静态审查未把 vet 告警计入红灯口径，需明确这是口径差异还是遗漏。

## 2. 定向测试

| 目标 | 结果 | 判定 |
|---|---|---|
| `./gpu/rwgpu/...` | FAIL（5 个测试）：texture_test.go ×4 + thread_safety_test.go ×1，全部报 `CreateInstance failed: wgpu: native library not loaded or failed to initialize` | **环境缺失**（wgpu 原生库未加载，非代码逻辑错）|
| `./render/`（根包） | FAIL，6 个测试函数：见下表 | 混合：环境相关 + 真 FAIL |
| `./ui/rendering/...` | ok 0.162s | 通过 ✅ |
| `./ui/embedder/...` | ok 0.150s | 通过 ✅ |
| `-run 'TestShaper|TestShape|TestLayout' ./render/text/...` | 全 ok（text 0.123s / msdf ok / cache·emoji·hint no tests to run） | 通过 ✅ |

`./render/` 根包 6 个失败明细：

| 测试 | 报错摘要 | 性质初判 |
|---|---|---|
| TestAAProbe_CircleDiag | `fill:gg: falling back to CPU rendering` cpu_fallback=6 | GPU 库缺失→CPU 回退被测试禁止（环境依赖）|
| TestContext_StrokeWithDash_Rectangle | 矩形顶边无 dash 间隙 | **真 FAIL**（虚线渲染逻辑）|
| TestStrokeString_DifferentFromFill | 'O' 描边 1253px > 填充 538px，断言描边应更少像素 | **真 FAIL**（描边像素量异常）|
| TestS3c_M3_BlendHue | GPUOps=0，要求 GPU 路径 | 环境依赖（无 GPU 时必挂）|
| TestS3c_M3_PathBooleanDifference | difference 挖洞处期望浅色，得到 `255,0,0` | **真 FAIL**（布尔差集挖洞没生效，注意：与本次 X04 实测「并集正确」并不矛盾，差集是另一条路径）|
| TestS69_Contract_FromJSON | H01/H02/H03/H05/U05 五项性能预算超限（如 H01 p50=0.60 > 0.41） | **性能预算 FAIL**（本机跑分超阈值；数值随机器浮动）|

注：rwgpu 的失败测试没有按「环境缺失」做 t.Skipf 静默跳过，而是直接 FAIL，符合「禁止静默假绿」纪律但归类上属环境缺失。

## 3. X04 布尔运算行为实测（探针真跑）

构造：EvenOdd 自交五角星（外径150/内径60），400×400 画布，标准管线 `NewContext → AppendPath → Fill(EvenOdd) → FlushGPU → 读 Image()`，逐位对比 RGBA。

| 实验 | 预期（按台账 X04） | 实测 | 一致？ |
|---|---|---|---|
| 直画 vs `BooleanPath(star, star, PathOpUnion)` 后再画 | 若布尔实现破坏填充语义应明显不同 | **显著差异(Δ灰度>8)仅 1029 px**，全集中在图形边缘（中心区<40px 为 0 个）；五角星中心洞保留（center 灰度两边都=0 黑）| **基本一致**：差异属抗锯齿级别边缘重采样，不是结构性错误；并集结果保住了 EvenOdd 语义 |
| 超 2048px 路径静默截断 | 存在 ±2048 夹紧导致长路径画不全 | 斜率检测法：细条从(0,~13)到(endX,189)，在 x=700 处量高度位置——endX=2048/2049 时 y≈70，endX=4096 y≈40，16384 y≈17，30000 y≈14，**全部精确等于理论值**；若夹到 2048 应恒停 y=70 | **不一致**：本版本未复现静默截断，坐标没有被 ±2048 夹 |

补充数据：
- 并集后路径坐标数 2648 vs 原 20（扫描线矩形化输出，符合 BooleanPath 文档描述的实现方式）。
- 第一版探针（漏 FlushGPU）曾得到「直画图空白」的假象——离屏绘制必须 `Fill()+FlushGPU()` 后再读像素，否则全是空图假绿。此坑值得写进真窗规范。

结论：X04 台账里「布尔运算破坏填充语义」在当前代码上**未复现**（并集行为正常）；「超 2048px 静默截断」也**未复现**。该条目疑似已修复或台账记录过时，建议回查台账来源版本。

## 4. X05 shaping 行为抽查（探针真跑）

字体：系统 DejaVuSans.ttf（LoadDefaultFace(48) 实际加载成功）。

| 检查点 | Shape("fi") | LayoutGlyphs("fi") |
|---|---|---|
| GID 序列 | `[5042]`（单 glyph） | `[73 76]` = f(73) + i(76) 简单拼接 |
| 是否连字 | 是（5042 ≠ 73 也 ≠ 76） | 否 |

两条管线行为确实不同，与台账 X05 结论**一致**：Shape 走 GSUB 连字，LayoutGlyphs 不做连字。

## 总结

- 编译矩阵：build 多出 `tmp_c4dbg` 一个新红包（连同 wind 共 4）；vet 红灯面比台账口径大（8 包，含正式包告警级问题）。
- 测试：ui/rendering、ui/embedder、text shaping 三路全绿；render 根包 6 失败中约 2.5 处是 GPU 库缺失的环境依赖，3 处是真 FAIL（dash 虚线、描边像素量、布尔差集挖洞），1 处是性能预算超限。
- X04 布尔：并集行为正常（仅边缘 AA 差异 1029px）、无 ±2048 截断——两项均未复现台账问题。
- X05 shaping：Shape 有连字（GID 5042）、LayoutGlyphs 纯拼接（73,76），两管线确实不同，与台账一致。
