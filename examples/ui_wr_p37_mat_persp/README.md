# ui_wr_p37_mat_persp — 材质×透视/bicubic 专项窗

P3.7 材质×透视/bicubic 立项（2026-11-07，`evidence/P3.7/batch6/mat-persp-assessment.md`）
的专用真窗（用户拍板：新专项窗承管材质×透视画面，③⑥ 只迁拒面）。满窗即内容
（六面板 3×2），指标行浮左上（金图 mask `y<32`）。

面板：

- **P1 恒等材质+透视**：对照 | 材质 两格并排同几何（非均匀权重 1.5/1.2/1/2 的
  凸四角透视）——identity 材质+透视 ≡ 无材质透视**逐位**（A 批权重透传的窗内
  版；探针 `identity_persp_pair_bitwise`）。
- **P2 渐变带·透视正确插值**：uv.y 带状材质骑梯形（×2 几何，TL/TR w=2 远、
  BR/BL w=1 近）——色带分界的透视正确落点 y=40（仿射 60）：(80,30) 红、
  (80,55) 蓝，**55 锚专抓权重拍平**（若 hom_w 被拍平成 1，分界退到 60，该点
  错画红）；离屏+快照双锚（`band_boundary_perspective_correct` /
  `band_red_anchor`/`band_blue_anchor`）。
- **P3 bicubic 纹素 echo**：对照 | 材质——echo 材质（4 参契约，返回引擎纹素
  原样）≡ 无材质 bicubic 核**逐位**（B 批「引擎喂纹素」的窗内版；
  `bicubic_echo_pair_bitwise`）。
- **P4 精灵路 material+persp**：对照 | 材质——批绘口自带透视+权重透传逐位
  （`sprite_persp_pair_bitwise`）。
- **P5 投影层组合**：对照 | 材质——材质+画布投影层（projW 权重，homography
  w=1-0.003y 不跨眼平面）逐位（`projection_combo_pair_bitwise`）。
- **P6 能力矩阵（quad 口 × identity 材质）**：12 组合逐格真实表态——
  2 透视（affine/persp）× 3 采样档（nearest/linear/bicubic）× 2 blend
  （Normal/Multiply）。stance 由离屏**逐格实测**（`mpMatrixFacts`：每格真调
  `DrawImageQuad`，验 err 与出图），不许写死：**5 画 + 7 拒**——

  | | nearest | linear | bicubic |
  |---|---|---|---|
  | affine | ✅ 画 | ✅ 画 | ✅ 画 |
  | persp | ✅ 画 | ✅ 画 | ⛔ `ErrQuadPerspectiveBicubic`（透视槽边界，先于材质） |
  | affine+Mult | ⛔ `ErrQuadUnsupportedBlend` | ⛔ 同左 | ⛔ 同左 |
  | persp+Mult | ⛔ 同左 | ⛔ 同左 | ⛔ 同左 |

  窗内绿格每帧真画（checker）、红格拒底（芯片色由事实驱动，红=真、琥珀=异常）；
  `matrix_stance_exact` 逐格哨兵对 + 放行格出图断言；快照锚
  `matrix_green_cell_drawn`/`matrix_red_cell_refused`。CPU 腿 stance=skip+计数
  （P3.2 无求值器契约，env 档不进本窗矩阵）。另有眼后现场拒芯片
  （`ErrQuadBehindEye` 几何族——材质+透视放行后可拒面收敛）+
  `blend_material_skip_counted`（图像口）。

三证据合证：9 枚离屏逻辑探针（四对逐位 + 解析锚 + 12 格矩阵 stance + 拒绝事实
+ gpu_ops 有界）+ 终帧 5 枚语义像素断言 + 双后端金图逐位 0.0000%/921600px 且
两基线逐位相同。H15 定帧 240 停机。

## 跑法

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib
RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only  # 播种，仍不算过
go run ./examples/ui_wr_p37_mat_persp                                                # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/ui_wr_p37_mat_persp -auto-only       # 第二后端
```

公开面：零渲染层 API 新增（消费 A 批权重透传与 B 批纹素契约的既有面）；
轮次正本 `evidence/P3.7/batch6/matpersp-window-runs.txt`。
