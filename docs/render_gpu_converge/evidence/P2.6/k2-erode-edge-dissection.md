# K2 解剖：erode 探针 cpu 档红 = GPU 栅格化边缘列覆盖差经 min/max 窗传播

> 台账来源：§2.2 K2（P3.4 C 卡收口 2026-11-01 登记：「CPU 滤镜链的 erode 边缘
> 语义与 GPU 图存在差异；复核 CPU 链边缘语义」）。解剖日期 2026-11-11。
> **定性改判：不是窗口语义分叉，归 SDF 覆盖线（在册）。**

## 复现（临时对拍测试，跑完即删）

`render` 包内（`render_test`）对拍：暗底 + 白矩形 `DrawRectangle(60,50,60,60)`
+ `ApplyErode(5)`，CPU 腿（`WithCPUOnly`）vs GPU 腿逐像素 A/B：

- 输出差分 **404/46080 px，max=17**，全在矩形边缘传播带；
- 关键样本（erode 后）：`(65,55)`（距左缘 5px）cpu=(255,255,255) vs
  gpu=(238,238,239)、`(64,55)` cpu=25(背景) vs gpu=42。

## 根因（两级，均有实测）

1. **erode/dilate 窗口语义两侧一致（销证）**：CPU（`render/internal/filter/
   morphology.go:22-27,94-139`）与 GPU WGSL（`filter_gpu_graph.go:179-200`）
   都是 resvg/SVG-feMorphology 口径——半开 [-rx,rx) 窗、OOB 邻域跳过、
   premul 逐通道 min/max，代码逐条对齐；登记里怀疑的「CPU 链边缘语义与
   GPU 图存在差异」**不成立**。
2. **真根因 = GPU 栅格化整数边界列的 AA 覆盖差（输入级）**：去掉滤镜对拍
   同一矩形——`(60,55)`（矩形左缘第一列）cpu=**255**（精确覆盖 1.0）vs
   gpu=**237**（≈0.93 覆盖）、`(59,55)` cpu=25 vs gpu=42。GPU 单点采样在
   整数边界把边缘列画成 AA 中间值（与 P4.2 对拍「AA 带差异 max18，归
   SDF 覆盖线」同族同源）。erode/dilate 的 min/max 窗是**放大器**：这条
   1px 输入差沿半径传播 5px → 窗口级 404px 差。

与 Skia 的差距：Skia GPU 光栅化（解析 AA）在整数边界覆盖精确 1.0/0.0，
与 CPU 一致——SDF 覆盖线闭合即对齐。

## K2 窗探针红的重定性

`ui_wr_p34_filters` A 卡 erode 探针在 `GOGPU_RENDER_MODE=cpu` 红
（native+cpu 与 go+cpu 同红）= 探针的期望值来自 GPU 档画面，CPU 档跑出的
GPU 输入差被放大传播——**不是 CPU 链的洞**（CPU 侧是 resvg 标准语义、
正确的一方）。

## 归位与验收（待拍板）

- **归 SDF 覆盖线**（`evidence/SDF/coverage-line.md`，在册）：新增验证项
  「morphology 传播」——覆盖线闭合后以本对拍为验收（CPU vs GPU 全图
  diff px 收敛到 0 或 AA 带 ≤2，erode/dilate 各一）。
- erode/dilate O(n) 可分离优化（同批登记的 P3.4 后续）与此无关，独立待续。
- K2 窗探针 cpu 档红维持「cpu 组合非验收档」口径不放宽，待覆盖线闭合自然转绿。
