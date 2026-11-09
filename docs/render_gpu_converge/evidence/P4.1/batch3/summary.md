# P4.1 批 3（子项④ DrawShadow 设备空间抬升）收口记录（2026-11-10）

> D2a 口径（2026-11-10 拍板，PLAN §6 已回写）：对齐 Flutter Impeller spot 单 pass
> 模型（本库 tonal 公式/抬升/模糊已逐字对齐），只补真缺——非仿射投影下的抬升语义。

## 0 直话

- **批 3 语义修复 ✅（CPU 路径像素收口）+ 真窗仿射/平射锁定 ✅ + 双后端金图 0**。
- 红测实证缺陷：真透视（W←y 矩阵，P3.7 真窗同款形式）下，现状把抬升平移放在
  用户空间（投影之前），透视把位移再变形——shadow 可见带墨水只有期望语义的
  **79.4%**（红绿对：场景一=现状透视 DrawShadow vs 场景二=手工设备化 path +
  恒等链 DrawShadow 的期望语义真值）。
- 修法：非仿射投影分支剪影先 `projTransformPath` 设备化（doFill 同款：曲线展平
  逐点映射、behind-eye 整笔拒绝），抬升直接加设备 y（`translatePathDev`），
  `withProjectedPathScope` 恒等链 `doFillUnprojected`（绕开二次投影）——修后
  与期望语义一致（红转绿）。
- **GPU 层链时序洞 #4 实测坐实**：同一测试 GPU 路径下场景一墨水在 183k~125k
  摆动（race 恒 51.8%）——「投影×隔离层×blur×GPU readback」组合的内容不同步，
  与批 1 #3 同族。测试锁 CPU（Setenv）收语义，GPU 链挂 #4 待诊断。
- **真窗另抓两个洞**：#5 tier 顺序洞（「隔离层→直接绘制」序列被 gpuTex tier 末尾
  统一重放破坏——层内容盖住后画的直接绘制；真窗绕过=本体先画 shadow 后画，
  contact-shadow 视觉亦合 Material 惯例）；#6 状态组合域洞（卡片内局部
  SetProjection × dpr × GPU 层丢内容——P3.7 真窗都是整窗投影未踩过；真窗
  改避开该组合域，透视 shadow 画面演示挂 #4/#6 收口后补）。

## 1 改动清单

| 文件 | 改动 |
|---|---|
| `render/context_misc_draw.go` | DrawShadow 非仿射分支：projTransformPath 设备化 + translatePathDev 抬升 + withProjectedPathScope 恒等填充；新增 `translatePathDev`（verbs 保真拷贝、y+lift、conic 权重不动） |
| `examples/ui_wr_p41_drawshadow/` | 新真窗（四卡：A 平射 tonal/抬升锁定、B 仿射 CTM 换算、C elevation 阶梯、D tonal 色对比；负向 skip-shadow） |
| `render/drawshadow_projection_test.go` | 红绿对单测（锁 CPU；GPU 链 #4 洞注释） |

## 2 验证载体与证据（evidence/P4.1/batch3/）

- **能力单测**：`render/drawshadow_projection_test.go` ::
  `TestDrawShadowProjectionLiftIsDeviceSpace`（新增，先红后绿：现状 79.4% 红 →
  修后绿；锁 CPU——GPU 链 #4 洞）。日志 `unit-tests.txt`。
- **真窗**：`examples/ui_wr_p41_drawshadow`（-auto-only + 常驻人工二合一；人工
  rc=0）。像素探针 9/9（双后端一致）：A 平射 shadow 带/白区、B 仿射 CTM shadow
  带（变换换算坐标）、C elevation 阶梯三带、D 黑/红 tonal 双带。
- **金图**：`examples/ui_wr_p41_drawshadow/testdata/goldens/...png`（native）与
  `..._go.png`，**双后端各自 verify 0.0000% / 763584 px**（逐位）；sha256
  `golden-sha256.txt`。
- **双后端**：webgpu（native）✅；gwgpu-gles（GPUI_BACKEND=go）✅；metal 不可验
  （无真机）。`gate-native.txt`/`gate-go.txt`（seed `*-seed.txt`）。
- **负向对照**：`GPUI_P41S_BREAK=skip-shadow`（A 卡不画 shadow）→ A 带探针红、
  exit 1（rc=1），`negative-skip-shadow.txt`；还原复绿。
- **回归**：render 包全量失败集合与基线逐名一致（零新增）；`-race`
  （TestDrawShadow）ok；vet 干净；apidoc rc=0。

## 3 洞登记（本轮新增三条，均已落台账）

| # | 现象 | 状态 |
|---|---|---|
| #4 | 「投影×隔离层×blur×GPU readback」内容不同步：headless 场景一墨水 183k~125k 摆动、race 恒 51.8%；与 #3 同族（GPU 层内容在合成/回读链上不同步） | ⬜ 待诊断（语义收口已由 CPU 路完成；真窗透视 shadow 演示挂本洞收口后补） |
| #5 | tier 顺序洞：帧内「隔离层→直接绘制」序列，层合成统一在 gpuTex tier 末尾重放，盖住后画的直接绘制 | ⬜ 登记（真窗以「本体先画、shadow 后画」绕过=contact-shadow 视觉；架构修法=层合成按 Pop 位置插序，归引擎层后续批次） |
| #6 | 状态组合域：卡片内局部 SetProjection × dpr × GPU 层丢内容（B/C 卡全白、shadow 错位压 LEGEND）——P3.7 真窗均整窗投影未覆盖该组合 | ⬜ 登记（真窗已避开该组合域；收口后补透视 shadow 演示卡） |

## 4 下一步

- 批 4（子项① MSAA even-odd stencil-invert + go4x 修复，D1a）按 D3a 开工。
- #3/#4/#5/#6 四洞同族（GPU 层链同步/顺序/组合），建议归并一个诊断批次处理。
