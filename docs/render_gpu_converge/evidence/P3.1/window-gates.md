# P3.1 真窗（隔离 :3 Xwayland，P1.4 同款）

## native (webgpu/Vulkan) run1 — 金图播种 + 像素探针
ui_wr_p31_linear_blend: pixel p31_a_srgb_difference          ok=true got 0.3020 want 0.2980 tol 0.0078 (byte 77)
ui_wr_p31_linear_blend: pixel p31_b_linear_difference        ok=true got 0.6588 want 0.6573 tol 0.0118 (byte 168)
ui_wr_p31_linear_blend: pixel p31_c_normal_linear_degrades   ok=true got 0.5020 want 0.5020 tol 0.0039 (byte 128)
ui_wr_p31_linear_blend: OK presents=300 pixel=3/3 golden=0.0000% resize=0 mode=gate elapsed=5.4s

## native run2 — 金图逐位
ui_wr_p31_linear_blend: OK presents=298 pixel=3/3 golden=0.0000% resize=0 mode=gate elapsed=5.3s

## go 档 (gwgpu-gles) run1
ui_wr_p31_linear_blend: pixel p31_a_srgb_difference          ok=true got 0.3020 want 0.2980 tol 0.0078 (byte 77)
ui_wr_p31_linear_blend: pixel p31_b_linear_difference        ok=true got 0.6588 want 0.6573 tol 0.0118 (byte 168)
ui_wr_p31_linear_blend: pixel p31_c_normal_linear_degrades   ok=true got 0.5020 want 0.5020 tol 0.0039 (byte 128)
ui_wr_p31_linear_blend: OK presents=297 pixel=3/3 golden=0.0000% resize=0 mode=gate elapsed=5.6s

## 负向对照：摘 WGSL linear 分支（B 卡必须红）
ui_wr_p31_linear_blend: pixel p31_b_linear_difference        ok=false got 0.3020 want 0.6573 tol 0.0118 (byte 77)
FAIL: pixel probes 2/3 ok
rc=1 ✓

## 还原复绿
ui_wr_p31_linear_blend: pixel p31_b_linear_difference        ok=true got 0.6588 want 0.6573 tol 0.0118 (byte 168)
