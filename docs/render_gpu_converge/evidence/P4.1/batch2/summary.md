# P4.1 批 2（子项② 分数裁剪 AA）收口记录（2026-11-10）

> 拆批依据：D3a（③→②→④→①）。标准口径：Skia 对非整数矩形裁剪走 analytic FP
> （`ClipStack.cpp:249-252` 总策略 analytic FP → stencil → atlas → SW mask；
> 整数矩形才用纯 scissor）。PLAN §6 修法「走路径裁剪或分数 scissor」落地为
> 分数矩形升级 Radius=0 rrect entry（analytic SDF AA）。

## 0 直话

- **批 2 全量收口 ✅**：分数裁剪矩形从「scissor Floor/Ceil 外扩硬边溢出（GPU）/
  bounds Contains 二值（CPU）」升级为 analytic SDF AA（0.75px smoothstep），
  单测 9/9 先红后绿、真窗 10 探针全过、双后端金图各自 verify 0.0000%（逐位）、
  负向转红、回归零新增、race/vet/apidoc 绿。
- 整数矩形保持纯 scissor 快路**零变化**（integer_rect_lock 单测 + 真窗 B 卡锁定）。
- GPU 侧**零改动**：分数矩形存为 rrect entry 后自动流入既有 `IsRRectOnly →
  RRectBounds → SetClipRRect` SDF 通路（`RRectBounds` 无 radius 下限；
  `rrectCoverage`/`cover_aa.wgsl` 对 r=0 自然退化为 box SDF，无除法，实证）。

## 1 改动清单（两个文件，无公开 API 变化）

| 文件 | 改动 |
|---|---|
| `render/internal/clip/stack.go` | 新增 `PushRectFractional`（rrect entry Radius=0，绕过 PushRRect 的 r<=0 退化）与 `ReplaceRectFractional` |
| `render/clip_op.go` | `ClipRectOp` 轴对齐分支设备矩形整数检测（Floor 比较）；带分数时 Intersect/Replace 两 op 分流到 Fractional 版；Difference 登记边界（mask 路独立，本批不动） |

## 2 验证载体与证据（evidence/P4.1/batch2/）

- **能力单测**：`render/clip_fractional_test.go` :: TestClipFractionalCasesCPU /
  CasesGPU / CPUMatchesGPU（3 场景 × 3 套 = 9/9）+ 数据
  `render/testdata/clip_fractional_cases.json`。先红后绿：fractional_rect_clip 与
  fractional_clip_replace 实现前红（半覆盖缺失/scissor 溢出），实现后绿；
  integer_rect_lock 回归锁定（现绿，锁定零变化）。CPU 与 GPU 逐位一致
  （CPUMatchesGPU 过，AA 边缘 max<=6）。日志 `unit-tests.txt`。
- **真窗**：`examples/ui_wr_p41_fractional_clip`（-auto-only + 常驻人工二合一；
  人工窗 rc=0 正常退出，`manual-window.txt`）。四卡：A 分数 clip 半覆盖 AA 边、
  B 整数 clip 锁定、C 分数 clip + 内容（门禁静态/人工窗旋转演示）、D 嵌套
  分数+整数（交集形状）。像素探针 10/10 ok（双后端一致）。
- **金图**：`examples/ui_wr_p41_fractional_clip/testdata/goldens/...png`（native）
  与 `..._go.png`（go/GL），**双后端各自 verify 0.0000% / 763584 px**（逐位，
  P0.1 容差 0）；sha256 `golden-sha256.txt`。门禁画面完全静态（-auto-only 冻结
  旋转演示，旋转属人工窗检查项）。
- **双后端**：webgpu（native）✅；gwgpu-gles（GPUI_BACKEND=go）✅；gwgpu-metal
  不可验（无 macOS 真机）。`gate-native.txt` / `gate-go.txt`（seed 各
  `gate-native-seed.txt` / `gate-go-seed.txt`）。
- **负向对照**：`GPUI_P41C_BREAK=hard-edge`（分数 clip 强制整数化=旧硬边行为）→
  A_edge_half 探针红（got=(229,38,38)≈全色 vs 半覆盖）、exit 1（rc=1），
  `negative-hard-edge.txt`；还原复绿。
- **回归**：render 包全量失败集合与基线逐名一致（零新增）；render/internal/clip
  包 ok；`-race`（TestClip|TestVerts|TestVerticesPaintBlend）ok；vet 干净；
  apidoc rc=0。

## 3 边界与发现登记

1. **Difference op 未分流**：`PushRectDifference` 走 mask 路径（独立覆盖机制），
   分数矩形的 Difference AA 不在本批（登记；Skia 同走 SW mask 族）。
2. **DPR≠1 下「整数」clip 设备空间分数化**：真窗 DPR=1.5567 时用户整数 clip 经
   CTM 变设备分数 → 全部走 rrect SDF AA 通路（行为更好=AA 化）；「整数硬边」
   语义在 dpr=1（单测）下锁定。真窗 B 卡探针只断 inside/outside（不断硬边）与此一致。
3. **嵌套多 rrect clip GPU 单表达**（既有边界，非本批引入）：GPU `RRectBounds`
   只返回最近一个 rrect entry，外层 rrect 退化 scissor 粗裁；真窗 D 卡只断交集
   形状不断嵌套 AA。多 rrect 精确表达归后续能力批。
4. **门禁金图静态化**：-auto-only 冻结旋转演示（fps 类动态内容与逐位金图冲突，
   P3.9b 冻结相位同款思路）；旋转 AA 稳定性属人工窗检查项。

## 4 下一步

- 批 3（子项④ DrawShadow 非仿射投影剪影，D2a 口径）按 D3a 顺序开工。
- 批 1 遗留（#3 GPU 路/B 卡）与批 4（MSAA even-odd + go4x）依 D1a/D3a 排程。
