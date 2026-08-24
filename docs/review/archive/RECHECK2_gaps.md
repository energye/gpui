# 复核查漏报告第二轮（RECHECK2 gaps）— 上轮盲区专项

> 日期：2026-08-24。任务：扫上一轮 RECHECK_gaps.md 没碰的 7 个盲区，只抓 P0/P1。
> 上轮已覆盖（本轮不再重复）：gpu/context、ggcanvas、窗口生命周期、panic/死循环/sleep 高危模式、lib 版本。
> **结论先行：未发现新的 P0（崩溃/内存安全级）。新发现 1 条 P1（wrgate 门禁有系统性「放宽点」，可让卡顿真窗拿绿）、若干 P2。shader 编译器四后端测试全绿、滤镜/SDF/并发锁三个区域干净。**

---

## 一、新发现

### M1（新 P1）examples/wrgate 门禁存在三处叠加的「放宽点」，短跑/掉帧真窗可以静默拿到 FPS 绿

**直话**：门禁的阈值本身不是写死的放宽数字（都是调用方传参），但 `EvaluateGates` 里的默认行为叠起来，构成一条「跑不满 5 秒 → FPS/P95 门禁整个跳过」「FPS 不够 60 只查 55」「wall FPS 难看时优先用 interval FPS」的通路。对一个以「防假绿」为存在意义的框架，这是 P1。

证据（`examples/wrgate/report.go`）：
1. **短跑免检分支**：`report.go:340-348` —— `minEl` 默认 5 秒，`RequirePersistentFPS && r.ElapsedSec >= minEl` 才进入 FPS 检查；`ElapsedSec < 5s` 时 **FPS 和 P95 两道门全部静默跳过**，连提示都没有。注释自辩「align with min RUN_SECONDS（U16）」，即靠规范约束真窗时长 ≥5s，框架层不强制。
2. **FPS 默认 55 不是 60**：`report.go:345-348` —— `minFPS` 默认 55（"60Hz-class gate" 的名义下放走 5 帧）。掉帧到 55~59 的真窗照常绿。
3. **interval FPS 掩护 wall FPS**：`report.go:350-353` —— 只要 `FPSInterval > 0` 就用它，wall FPS（含拆窗/收尾卡顿）只在 interval 缺失时兜底。帧间隔均匀但每帧都慢的场景（如恒定 50fps）interval 反而「稳定」，更容易过。
4. 连带：`MaxP95Ms` 检查写在 FPS 分支体内（`report.go:357-360`），短跑时一并失效。

建议方向（不动手，等确认）：给「跳过」加显式标记（Report 里回 `fps_gate=skipped`），或在 SchemaOnly 外禁止静默短路；默认 55 是否提到 58~60 由用户定。

### M2（新 P2）ui/animation：Stop() 把任意状态一刀切成 Dismissed，反向动画中途停止丢「完成」语义

`ui/animation/controller.go:206-218`：`Stop()` 无条件 `setStatus(StatusDismissed)`。若动画正在 reverse（1→0）中途被打断，Flutter 语义应停在中间并按需发 Completed/Dismissed 的判定，这里一律 Dismissed，监听方无法区分「反向完成」和「中途掐断」。当前仓库消费者少（AnimatedOpacity 的 OnComplete 只认 Completed），影响面小，故 P2。

### M3（新 P2）ui/animation：AnimatedOpacity.Mutations 切片无上限

`ui/animation/implicit.go:29,46-52`：每个 tick 往 `a.Mutations` append 一条，只在 `Start()` 时截断（implicit.go:77）。repeat 型长驻动画（spinner 用 SetRepeat）会随时间无限增长。公开字段还是测试后门，建议改成环形缓冲或仅在 debug 开关下记录。

### M4（新 P2）gpu/shader：四个后端各自有充分测试，但没有「同一份 WGSL 过全部后端、断言语义一致」的同源金标语料

实测 `go test ./gpu/shader/spirv/... ./msl/... ./glsl/... ./hlsl/... ./dxil/...` 全绿（2026-08-24）。各后端覆盖都不差：SPIR-V 有 naga 官方 15 条 reference shader 回归（`spirv/internal/codegen/reference_shaders_test.go:59`）、DXIL 有 dxcvalidator 金标位流对照（`internal/dxcvalidator/testdata/golden-dxc/`）；MSL 有 xcrun 真机编译钩子但仅 Darwin 生效（`xcrun_helper_test_darwin.go/_stub.go`），Linux CI 上 MSL/HLSL/GLSL 只有文本级单测，没有编译器在环验证。缺的是一份共享语料同时驱动四后端做交叉断言（如 uniform 布局、控制流、纹理采样语义一致）。属流程保障缺口而非已知错误，定 P2。

### M5（P2 观察）render 根包工作树上有 6 个红测 + 性能回归报告，本轮不做归属结论

`go test ./render/` 实测 FAIL：`TestAAProbe_CircleDiag`、`TestContext_StrokeWithDash_Rectangle`（dash 无缺口）、`TestStrokeString_DifferentFromFill`、`TestS3c_M3_BlendHue`、`TestS3c_M3_PathBooleanDifference`（挖洞位置是红的）、`TestS69_Contract_FromJSON`，另有 `U05_KitchenSinkStress` p50 性能回退报告。注意当前工作区有大量**其他会话的未提交改动**（git status：docs/ENGINE_UI_WIDGET_RENDER.md 被改、多个 tmp_* 目录与二进制），无法排除是别的在途线引入；SDF 相关单测单跑是绿的。**需要用户裁决归属后再定性**，本报告只挂账。

---

## 二、逐区域结论（上轮盲区 → 本轮答案）

### 1. gpu/shader 编译器后端（spirv/hlsl/msl/glsl/dxil）
- 一致性测试保障：见 M4（各后端分测充分，跨后端同源语料缺失）。
- 越界/panic 风险抽查：codegen 内 panic 共 5 处生产代码——`spirv/internal/codegen/ray_query.go:73,104,118,389`（emit 失败时 panic，输入是自家 IR 不受外部 WGSL 直接驱动）+ `dxil/internal/bitcode/writer.go:163`（char6 编码非法字符，内部域）。全仓 shader 包**无 recover()**，编译器 panic 会穿透到调用方；但主入口 `Compile`（naga.go:80-113）各阶段都走 error 返回，panic 属「内部不变量破坏」型，未见可达路径。类型去重（`internal/registry`）键构造、SPIR-V 主流程 `Backend.Compile`（backend.go:300-386）阶段顺序正确，双检锁/Reset 复用逻辑无越界。**无 P0/P1**（一致性缺口见 M4）。

### 2. render/filters 与 render/raster 注册回调 + AdaptiveFiller 实现
- 注册：`render/filters/register.go:12-27` 六个闭包（blur/blurXY/dropshadow/colormatrix/grayscale/invert）直连 `render/internal/filter` 实现；`render/raster/raster.go:24` 注册 AdaptiveFiller（目录文档标注 🔌 无人走的独立入口，正常）。
- 实现质量（逐函数核对了读写边界）：
  - `blur.go`：行列两趟分离卷积，源/目的各自钳位，temp 池 `getTempBuffer` 小了就换大的、池化上限 16M 元素，无越界。
  - `shadow.go`：extractAlpha 带 offset 且四面钳位；compositeShadow 对 srcX/srcY/dst 都有界检查；阴影色预乘正确。小瑕疵：offset 用 `int(f.OffsetX)` 截断，亚像素偏移丢失（保真度 P3）。
  - `colormatrix.go`：正确做了「预乘→直色→矩阵→再预乘」往返（226-251 行），比常见错实现（直接对预乘值做矩阵）质量高。小瑕疵：hue rotate 的 sin/cos 是手写泰勒近似（`colormatrix.go:291-303`，注释自己承认 "use math.Cos in production"），误差 ~4e-3 弧度级（P3）。
  - `AdaptiveFiller`（`render/internal/gpu/adaptive_filler.go`）：段数×画布面积双阈值选路，逻辑简单清晰，两个子 filler 都有界回调。**本区域干净，无 P0/P1**。

### 3. ui/gestures 手势竞技场
- **R1「孤岛」判断仍然成立，且有加重证据**：全仓 grep 证实——`ui/` 里消费 gestures 的生产代码只有 `ui/rendering/scrollable.go` 一个文件；而 `NewScrollable(` 的构造点**全部在测试里**（viewport_test、nested_scroll_test），examples/ui_l1_scroll 用裸 Viewport+定时器滚动，examples/ui_l2_shell 自己手工接 Dispatcher。框架层 `ui/embedder/InputRouter.Route`（input_router.go:130-151）只把事件投给命中的单个 `input.PointerHandler`，**完全不经过手势竞技场**。手势系统要生效，每个应用都得自己接线。
- 状态机本体：arena.go 的 Accept/Reject/sweep/finish 状态迁移逐条核对，Route 前快照成员表避免迭代中变异，胜者外全部拒绝、Up 后清场，逻辑对；tap.go 的「slop 内 Up 才赢、超 slop 自拒、赢了之后超 slop 不开火（arena 已 resolved 时 Reject 安全短路）」符合 Flutter 语义；pan.go 的 accept 时机/增量计算/fling 前置条件正确。dispatcher 批处理合并连续 move 到最新点，顺序保持。测试 `go test ./ui/gestures/` 绿。
- 隐患（P3）：① 全栈没有任何地方**生产** `input.PointerCancel`（input/pointer.go:10 定义后全仓零引用；recognizer 的 HandleEvent 也没有 cancel 分支），指针序列异常中断只能等 Up 或泄漏一个 arena；② `Scrollable.JoinPointer`（scrollable.go:186-198）把 pan 加进外部传入的 manager `m`，而自身事件路由固定用 `s.mgr`——两个 manager 不一致时事件永远送不到，目前无人调用所以未爆发，属接线地雷。因孤岛状态本身就是更大问题，这两条并入孤岛议题不单列 P1。

### 4. ui/animation 动画系统
- 插值器：curve.go 四条曲线全部先 clamp01 再变换，NaN 防御（clamp01 显式处理 NaN→0），CurveFunc nil 安全。正确。
- 时间源：Controller 实现 scheduler.Ticker，Tick(dt) dt<0 归零；TickerRegistry.TickAll（ticker.go:59-74）快照迭代 + 存活重建，回调在锁外调用，注册表自身 Add 去重。暂停恢复：无显式 Pause API，Stop/Start 即停启，Start 从 0 重来、Reverse 按 `(1-linear)*duration` 映射 elapsed 续走（controller.go:188-191），续走数学正确。
- 发现 M2（Stop 语义）与 M3（Mutations 无上限）两条 P2，其余干净。`go test ./ui/animation/` 绿。

### 5. render/sdf*.go + sdf_accelerator
- `sdf.go` 距离场公式（圆/描边圆/圆角矩形/描边）与 smoothstep AA（afwidth=0.75 与 GPU sdf_render.wgsl 同源）核对无误；rrect SDF 是标准 iq 公式。
- `sdf_accelerator.go`：六种 Fill/Stroke 的包围盒全部 `max(0,…)`/`min(W-1,…)` 双向钳位，blendPixel 入口还有第二道界检查（309 行），ClipCoverage/MaskCoverage 逐像素调制（CPU 侧裁剪不穿帮，正是上上轮 GPU 画穿问题的 CPU 对照实现）；forceSDF 双重开关语义清楚；getColorFromPaint 渐变取 (0,0) 色是有文档的近似。`go test -run 'TestSDFAccelerator|TestSDF' ./render/` 绿。NaN 防线在上游 shape_detect/nan_safety 层。**本区域干净，无 P0/P1**。

### 6. 并发压力点 grep
- `sync.Mutex/RWMutex` 全仓 34 处清单逐一过了一遍关键者：`pipeline_cache_core.go` 双检锁（RLock 快查 → Lock 双检 → 创建）正确无重入；`gpu_texture.go` released atomic.Bool + RWMutex 分工明确（UploadPixmap 锁内只拷字段、WriteTexture 在锁外执行，不持锁做 FFI）；`textured.go` PictureTextureCache 明确注释内部 helper 必须持锁调用；`tile.go` TilePool、`mask_r8_modulate.go`、`kernel.go` 缓存均「锁内无回调、回调不触锁」。未发现锁内调用可能重入同锁的路径。
- `atomic.` 19 处全部是 Uint64 计数/ID 分配/Bool 标志，类型匹配，无 int64/uint64 混用、无误用于需要复合原子性的场景。
- 特例记录（非问题）：`boundary_cache.go` 故意无锁（UI 单线程假设，BeginFrame/eviction 都在 present-paint 走），与 textured.go 的多线程缓存形成对照，设计意图清楚。**本区域干净。**

### 7. examples/wrgate 门禁框架
- 除 M1 外：`GateOptions` 所有阈值由调用方注入、0=off 语义统一；EvaluateRetainedExtras 的 dirty-id/multi-frames/cache-invalidations 门实现诚实（值缺失即 FAIL，不做默认通过）；CheckSchema 强制指标键齐全；numExtra 类型解码覆盖 int/int64/uint64/float64。`go vet` 净。**除 M1 外干净。**

---

## 三、扫描范围与方法

| 盲区 | 方法 |
|---|---|
| gpu/shader | 列目录全貌 → 通读 naga.go 编译管线、spirv Backend.Compile 主流程、ray_query.go、registry.go → grep `panic(`/`recover()` → 四后端 + dxil 子包全量测试（exit 0 全绿） |
| filters/raster | 通读 register.go、adaptive_filler.go、filter/{blur,kernel,shadow,colormatrix}.go 全文 → 逐函数核对读写边界 → 抽查 SparseStripsFiller/tile 池 |
| ui/gestures | 通读 arena/base/recognizer/tap/pan/dispatcher/doc 三遍 → 生产消费者 grep（ui/ 与 examples/ 分别扫）→ 对照 scrollable.go 接线 → 包测试绿 |
| ui/animation | 通读 controller/curve/status/implicit + scheduler/ticker.go → 包测试绿 + go vet |
| sdf | 通读 sdf.go/sdf_accelerator.go → 包围盒/NaN/预乘合成核对 → SDF 定向测试绿 |
| 并发 | grep sync.Mutex/RWMutex（34 处）/atomic.（19 处）→ 重点通读 pipeline_cache_core、backend、gpu_texture、textured、boundary_cache、kernel cache |
| wrgate | 通读 report.go 门禁主体（GateOptions/EvaluateGates/EvaluateRetainedExtras/numExtra 全部阈值默认值） |
| 附带 | go vet 四包净；`go test ./render/` 全量暴露 6 红 + 性能回退（M5 挂账）；`go test ./ui/rendering/ ./ui/embedder/` 绿 |

共约 45 步工具调用，未改动任何引擎文件。

## 四、一句话总结

七个盲区里，滤镜/SDF/并发锁三块是真干净，shader 编译器各后端自测扎实但缺跨后端同源金标（P2），动画有两条 P2；真正值得动手的是 **wrgate 门禁的短跑免检 + 55fps + interval 掩护三连（M1，P1）**，另外 render 根包工作树上挂着 6 个红测等待归属裁决（M5）。
