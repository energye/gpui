# P3.1 线性混合接线 — 收口报告（2026-11-01）

> 依据：`assessment.md`（D1–D5 用户拍板：路线 B / BlendSpace 枚举 / 新真窗 / hdr GPU 面移出）。

## 1. 交付

- **公开 API**：`BlendSpace` 枚举（`BlendSpaceSRGB`=0 默认 / `BlendSpaceLinear`）+ `Paint.BlendSpace`（Clone 复制）+ `DrawImageOptions.BlendSpace` + `Context.SetBlendSpace/BlendSpace()`；`Canvas` 接口 102→104；`Recorder` 同名口；录制四命令（FillPath/StrokePath/FillRect/StrokeRect）+`BlendSpace` 字段，`Backend` 契约四方法加 space 参（实现者 stub 同步）。
- **GPU（路线 B）**：dual-tex WGSL `Params.linear` 标志（offset 28，原 `_pad1`）+ `fs_main` 线性域合成（unpremul cb/cs 解码 → B(Cb,Cs) 与 W3C co 合成全程线性 → co 编码回 sRGB；alpha 永不做 gamma）；`packDualTexParams/dualTexWriteParams` 全链透传；共享单管线零管线变体（→ 原计划「管线缓存键/batch 谓词加 linear 维度」**取消**，降级路径行为与 sRGB 绘制一致无错误合并风险）。
- **CPU**：`internal/image/draw.go blend()` straight 域线性包装（decode→复用既有 switch→encode）；`software_blend.go` `compositeAdvancedLinear` 委托 `internal/blend.GetBlendFuncLinear`（u8 premul 正本）；`internal/blend` 反向依赖 render 修复（blend.go 迷你 API 签名 `render.RGBA`→`color.ColorF32`，零消费者、分层单向恢复）。
- **降级探针**：`GPUShared.linearFixedFunctionFallbacks`（atomic）+ `noteLinearFixedFunctionFallback`（helper 内聚判断模式表，防调用点漂移）。

## 2. 语义收紧（实现期拍板执行偏差，如实登记）

1. **linear 生效域=advanced 15 模式，全后端统一**：实现中发现 CPU Normal+linear 若真线性会与 GPU 固定功能降级不一致（违反双后端一致硬门禁）→ 固定功能族（SourceOver+Porter-Duff）CPU 同步降级（`GetBlendFuncLinear`/`draw.go` 按 `IsAdvancedBlendMode` 门控），P3.1b sRGB 目标格式切换时双端一并解锁。既有 `linear_test` 两个用例期望随语义改写（SourceOver 降级等价钉 + Multiply 线性钉）。
2. **CPU 精度口径修正**：原计划 float 直通改造放弃（28 模式重写不值 ≤1-2/255），改为 u8 域全链+实测分级口径——亮/中调 ±2/255、暗部（linear<0.1）±5/255（实测 multiply 0.25×0.25=4.5/255，一个线性 u8 步长在暗部值多个 sRGB 码）；GPU dual-tex 全 float 是精度主路（真窗 B 卡 168 vs float 参考 167.6）。
3. **固定功能降级遥测**：wrgate 探针段出口留 P3.1b（与 sRGB 格式切换的遥测面一并做）；本步由包内计数单测+真窗 C 卡像素等价断言覆盖。

## 3. 门禁终态

| 门禁 | 结果 |
|---|---|
| 全仓构建 | ✅（raw 仅 C2 既有 `ui_wr_t_stress` 红） |
| vet（改动面 5 包） | ✅ 0 新增 |
| 能力单测（先红后绿） | ✅ `TestLinearVsSRGBBlending`（模式集合同红/异）、`TestLinearBlendingAccuracy`、`TestLinearPrecisionEnvelope`（28×5×5 网格 vs float 参考）、`TestBlendSpaceDefaultIsSRGB`、`TestSoftwareFillLinearModeSet`（Normal 降级 128/Difference sRGB 76/Difference linear 168±3）、`TestImageDrawLinearDomain`（Normal 降级/Multiply linear 224）、recording 三钉（逐命令携带/环境默认/回放还原）、GPU 三钉（linear 位 offset 28/降级计数内聚判断/模式表前提） |
| conformance 双后端 | ✅ rc=0（带 lib，H1 纪律） |
| 真窗 `ui_wr_p31_linear_blend` | ✅ native（webgpu/Vulkan）+go（gles）双档 rc=0：A 卡 77、B 卡 **168**（float ref 167.6——GPU 线性混合像素级生效）、C 卡 128；像素 3/3、金图两档 0.0000%、gpu_ops>0/cpu_fb=0 |
| 负向对照 | ✅ 摘 WGSL linear 分支 → B 卡红（got 77=sRGB 域值，证明该分支是 linear 生效唯一来源）rc=1；还原复绿 168 rc=0，零残留 |
| 基线回归 | per-file 34 包正本后台跑（`tmp/per-file-tests/`）；已出部分中唯一 FAIL=`TestMiscDrawGPUPath/DrawColorEffectSurfaceAdvancedBlend` 与 P2.6 基线重放同名日志**同红**（`evidence/P2.6/b-tests/per-file-tests/render__context_misc_draw_gpu_test.log`），零新增红 |
| 五目标构建 | `build-five-target.txt`（后台落盘） |
| apidoc | ✅ rc=0（catalog §13 P3.1 行转正 + §1 BlendSpace 行 + §0 计数注记） |

## 4. 遗留与移交

- **P3.1b（已立项）**：sRGB 目标格式全局切换（swapchain 白名单+~50 管线 target 参数化+RT 联动），解锁固定功能 13 模式线性混合（双端），含 wrgate 降级遥测出口与基线重采流程——HDR/广色域前置专项。
- 默认域行为零变化成立（`TestBlendSpaceDefaultIsSRGB` + C 卡 128 + P2.6 基线同红零新增）。
- 证据：`evidence/P3.1/`（assessment / summary / window-gates.md / window-runs×5 / conformance.txt / apidoc-p31(-pre).txt / build-five-target.txt）+ 真窗 `examples/ui_wr_p31_linear_blend/`（testdata 金图已播种）。
