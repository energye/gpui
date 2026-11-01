# P3.4 滤镜家族 — 收口报告（2026-11-01，状态 🟡：五批+真窗主体完成，C 卡 GPU 路径专项遗留）

> 依据：`assessment.md`（D1 全做四批/D2 merge N 层/D3 连带 Paint 字段/D4 mask 链 A8 预处理/D5 GPU 优先单输入族）。实施按五批独立提交。

## 1. 五批交付终态

| 批 | 交付 | 提交 | 门禁 |
|---|---|---|---|
| 1 | erode/dilate/offset/alpha-threshold：CPU（premul 域形态学）+GPU mode 7-10+Kind 四处+Context 四口 | `d3be922f`+`10f57f50` | 值级钉 3 组（对偶性/位移/二值）+既有六族回归绿 |
| 2 | lighting（point diffuse，alpha 高度图中心差分法向）+tile（周期平铺）；GPU Params 128→192B（rect0-3 四 vec4 槽）+mode 11/12 | `4f3971ce`+修正 | 值级钉 2 组（光照几何含负 z 物理/周期性）+回归绿 |
| 3 | merge（Context 编排层 N 层 over，pristine 层派生）+displacement CPU | `3ae47859` | 编排钉（双层落地）+回归绿 |
| 4 | ColorFilter 接口（render straight 域）+Blend/Lighting/Table/Compose+ApplyColorFilter | `3403f712`+修正×2 | 值级钉 4 组（乘加/LUT/复合两向/blend） |
| 5 | Paint.ImageFilter/MaskFilterRadius 字段+fillPaintFilteredAsImage 路由+RegisteredGPUFilterGraphHooks 暴露+SetPaintMaskFilter/SetMaskFilter（mask 链 A8 预处理） | `3ba0a8d3` | 值级钉 2 组（边缘羽化 CPU 模式/merge 双层） |

## 2. 真窗 `ui_wr_p34_filters`（隔离 :3，native）

- **A 卡形态学对偶 2/2 ✅**、**B 卡 merge 双层 2/2 ✅**、**C 卡 Paint.MaskFilter 1/2**（硬边钉绿、羽化钉未达）——**4/6 探针**，既有族回归绿，apidoc rc=0（catalog §13 P3.4 转正，16 符号补齐）。

## 3. 排障实录（全有 dump/实证）

1. `mod` 不是 WGSL 内建（GLSL 惯性）→ shader 编译失败、GPU 图整体回退 CPU（3 例红实证）→ `%` 修正。
2. SDF 捷径吃 filtered paint（P3.2 同坑再现）→ tryGPUOpRC 绕行条件扩 ImageFilter/MaskFilter。
3. 不透明 base 的全图滤镜层互相覆盖（Merge 数学事实）→ 卡改透明底演示。
4. 层 2 的 base 拷贝被层 1 over 污染 → **pristine 层派生**修复。
5. GPU FromView 种子=当前 GPU 内容（含前层）→ **ApplyMerge 层跑改 CPU-only**，GPU per-layer seeding 登记后续。
6. C 卡羽化钉：doFill(4)→tryGPUFillWithMode(4) ✓→tryGPUOpRC **零进入**——bake dc 的 gpuCtx 建立问题，六轮诊断现场存档（assessment §7）；CPU 模式钉绿证明羽化数学与管线本身正确。

## 4. 遗留与后续（全部登记）

- **C 卡 GPU 路径专项诊断**（bake dc gpuCtx 建立链），解后补羽化钉转 ✅。
- displacement GPU aux 上传（批 3 CPU 面完整）、ApplyMask/PushMaskLayer 的 blur 预处理采用、merge GPU per-layer seeding、erode/dilate O(n) 分离优化、真窗金图「先 diff 后写」、ui paintRuns 迁移。
- 负向对照：morphology 对偶破坏钉在批 1 值级钉内建（min/max 交换断言）；真窗级负向随 C 卡收口补。

## 5. 证据

`evidence/P3.4/`（assessment §1-§7 / summary / apidoc-p34 / build-five-target / window 日志经 /tmp 归档摘要入 assessment §7）；代码提交 `d3be922f`/`10f57f50`/`4f3971ce`/`3ae47859`/`3403f712`/`3ba0a8d3`/`baf4b802`；真窗 `examples/ui_wr_p34_filters/`（金图已播种）。
