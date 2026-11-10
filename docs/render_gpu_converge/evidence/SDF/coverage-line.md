# SDF 覆盖线：GPU shader 覆盖精度（2026-11-10，评估/登记）

## 现象与现状

- CPU 侧：批 4f 已改成**解析行积分**（圆/椭圆/圆角矩形行截面闭式解），
  `sdf_coverage_quality_test.go` 锁 avg≤2 / max≤8，实测 circle 0.26 / ellipse 0.02 / rrect 0.01。
- GPU 侧：`sdf_render.wgsl` 仍是**单点采样 + 0.75px smoothstep**，没有解析覆盖，
  曲线边缘与 CPU 不一致（P4.2 实测 AA 带 max 18，典型 9）。

## 为什么还没修（历史实证）

- 批 4f 试过两版 shader：解析行积分版（asin/atan2 原函数）与 32×32 子采样版。
  两版 **naga 编译均通过**，但本机 NVIDIA 驱动**整条 shader 静默拒绝**
  （画面全空、零错误输出）⇒ 按「shader 改动必须真窗像素验收」纪律回滚。
- 结论：不是数学问题，是本机驱动/管线对该 shader 形态的可见性不可用；
  再次尝试必须先解决「驱动拒绝时能被看见」的诊断前置。

## 候选路线（按风险排序）

1. **覆盖 LUT 纹理**：预计算 1D 距离→覆盖率表（256 项）上传为纹理，shader 只做
   查表 + 双线性插值。逻辑最简、无超越函数，绕过驱动对复杂分支的拒绝。
2. **4x MSAA 覆盖率**：用 4 个采样点的覆盖数近似（0/0.25/…/1），与 CPU 解析仍有
   量化差（±0.125），但比 smoothstep 好；需确认本机 MSAA 管线可用。
3. 维持 CPU 兜底：对精度敏感的形状强制 CPU（现 `SDFAccelerator` 的 cpuFallback 已是解析版）。

## 验收前置

- 真窗像素断言（不是只跑单测）：同一形状 GPU vs CPU 边缘带逐像素，avg/max 有上限；
- 驱动拒绝可见性：shader 建好后先用 1×1 探针画布确认非空再进真窗；
- Golden 重种只在该窗（预计仅 SDF 边缘带变化）。

## 2026-11-11 新增验证项：morphology 传播放大器（K2 改判归此）

- 证据：`evidence/P2.6/k2-erode-edge-dissection.md`——GPU 栅格化整数边界列
  AA 覆盖差（`(60,55)` cpu=255 vs gpu=237、`(59,55)` cpu=25 vs gpu=42，
  白矩形左缘），被 erode/dilate 的 min/max 窗沿半径传播（1px 输入差 →
  5px 传播带，404px、max17）。窗口语义两侧本就一致（resvg 口径逐条对齐）。
- 验收追加：覆盖线闭合后跑 erode(5)/dilate(5) 白矩形 CPU vs GPU 全图
  逐像素对拍，diff px 收敛到 0 或 AA 带 ≤2（对拍脚本形态见 K2 证据）；
  `ui_wr_p34_filters` A 卡探针 cpu 档应自然转绿（维持「cpu 组合非验收档」
  口径不放宽）。

## 状态

**登记，未动代码**（本机驱动限制 + 需真窗验收）。P4.2 的 AA 带容差（≤24）
是当前上界；本条闭合后应收紧到 ≤8。
