# P3.4 滤镜家族：15 节点 + MaskFilter 全矩阵（2026-11-10）

> 起因：能力审计发现 P3.4 虽标 ✅，但真窗只有 3 张卡（morphology/merge/mask），
> Erode/Dilate/Offset/AlphaThreshold/Lighting/Tile/Displacement 在 `render/` 下
> 连节点级单测都是零。本批把「一个节点一张卡 + 一条语义契约」补全。

## 1 载体

- 单测：`render/p34_filter_nodes_test.go`——16 个节点，同一组几何与语义断言分别跑
  **CPU 腿**（`WithCPUOnly`）与 **GPU 腿**（有设备时）。断言是语义（通道/亮度/位移方向），
  不是逐位对拍：模糊的 GPU/CPU 对齐属 P4.2。
- 真窗：`examples/ui_wr_p34_filters` 由 3 卡扩为 **4×4 十六卡**（1400×900），
  29 个区域探针 + 16 张卡的金图逐位；按后端分基线（`p34_final.png` / `p34_final_go.png`）。
- 负向对照：`probe` 与单测共用几何，任何一张卡行为退化都会同时打红两处；
  既有 `GPUI_P41_BREAK` 类开关与本窗无关，本批不新增 break 开关（无行为分支可切）。

## 2 节点矩阵

| # | 节点（API） | 卡面契约（探针） | 单测 |
|---|---|---|---|
| 0 | `ApplyBlur(8)` | 核心仍白；边缘半亮（模糊过渡） | CPU+GPU |
| 1 | `ApplyBlurXY(0,10)` | 竖直抹开；左右不外扩 | CPU+GPU |
| 2 | `ApplyDropShadow(20,20,4)` | 内容红；偏移带出现阴影 alpha | CPU+GPU |
| 3 | `ApplyColorMatrix` | R/B 换道：红→蓝 | CPU+GPU |
| 4 | `ApplyGrayscale` | 三通道相等且≈luma(红)=54 | CPU+GPU |
| 5 | `ApplyInvert` | 蓝→黄 | CPU+GPU |
| 6 | `ApplyErode(5)` | 角被吃掉（bg）；主体仍白 | CPU+GPU |
| 7 | `ApplyDilate(5)` | 原角外长出（白）；远处不受影响 | CPU+GPU |
| 8 | `ApplyOffset(120,0)` | 副本在 +120；原位腾空 | CPU+GPU |
| 9 | `ApplyAlphaThreshold(180)` | α0.5 块清空；α0.95 保留 | CPU+GPU |
| 10 | `ApplyLighting` | 左上受光更亮；右下更暗 | CPU+GPU |
| 11 | `ApplyTile([10,10,50,50])` | 40px 相位格：格内白、格间空 | CPU+GPU |
| 12 | `ApplyBloom(σ6)` | 核心白；外侧有光晕 | CPU+GPU |
| 13 | `ApplyDisplacementMap` | +20px 位移：内容左移、原角腾空 | CPU（GPU 见 §3） |
| 14 | `ApplyMerge(offset, invert)` | offset 副本青色；invert 层洋红 | CPU |
| 15 | `Paint.MaskFilter(4)` | 软边有过渡；硬边外即 bg | CPU+GPU |

## 3 修出的真洞：GPU 位移图「声称支持但没实现」

- 现象：`ApplyDisplacementMap` 在 GPU 腿完全不位移（探针位仍是底色），CPU 腿正确。
- 根因：`imageFilterGraphGPUSupported` 在 `ImageFilterDisplacement` 携带 map 时返回 true，
  但 GPU 图里**没有 displacement 分支**（全仓无 shader / 无 aux 上传）⇒ 节点被静默跳过。
  这与「Displacement GPU aux 未接」的老登记一致，但旧代码把「未接」写成了「支持」。
- 修法（fail-closed）：该节点直接返回 false，让整个图回落 CPU 腿；真窗 route 里
  `card13:cpu_fb=1` 如实计数。GPU 位移 pass 作为后续项（P3.4 follow-up）登记。

## 4 验证

| 项 | 结果 |
|---|---|
| 单测 CPU 腿 | 16/16 节点全绿 |
| 单测 GPU 腿 | 16/16 全绿（displacement 走 CPU 兜底） |
| 真窗 native | 29/29 探针，金图 **0.0000%**（354 presents，重种后复验） |
| 真窗 go/GL | 29/29 探针，金图 **0.0000%**（347 presents，重种后复验） |
| gpu_ops | 43545，cpu_fallback_ops=0；逐卡 route 全部有 GPU 产出（位移卡 cpu_fb=1 为预期） |
| 门禁 | build / `-tags nogpu` / vet / apidoc rc=0；`TestFilterOps*`、`TestP12GPUFixedPixel`、`TestVerticesPaint*` 全绿 |
| 金图 | 旧 3 卡基线归档 `golden-reseed/p34_final_old_3cards.png`（sha256 同目录） |

## 5 边界与教训

- **窗口尺寸**：1600×1000 会被 WM 拉满到屏幕（1868×1021），此时
  `dpr=surface/winW` 与真实布局不一致，探针整体偏移；改 1400×900 后 surface 与
  设计尺寸 1:1（dpr=1.0）。大窗一律先核对 `snap` 尺寸再量像素。
- GPU 位移未实现（registered follow-up，见 §3）。
- 模糊/光照等节点的 GPU 与 CPU 逐位一致性不在本批（属 P4.2），本批只锁语义。
