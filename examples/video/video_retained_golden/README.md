# video_retained_golden — C4 纹理层固定帧 Golden 真窗

跑法（需要真机 GPU 窗口）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/video/video_retained_golden -auto-only        # 门禁：颜色 + 稳定 + Golden
GPUI_ACCEPT_GOLDEN=1 go run ./examples/video/video_retained_golden -auto-only   # 种基线
go run ./examples/video/video_retained_golden -manual-seconds 15               # 人工看 15 秒
```

## 人眼可见效果

窗口右边整块是视频画面：一帧固定 NV12 素材画出八个纯色块（红、绿、蓝、白 / 黄、青、品红、灰），中间横竖各一道白线。画面与素材逐像素一致，肉眼看不出模糊、偏色或色块错位。

左边图例、顶栏和底部 HUD 只是说明和计数器，Golden 掩码只覆盖右边视频那一块。

## 为什么固定帧

解码出来的帧每次都在变，拿它做 Golden 必然抖。这扇窗把解码摘掉：仓库里提交一帧 NV12 素材，开窗后只上传一次，之后一直复用。retained 路径每帧重画的都是同一个纹理，所以两次回放快照必须逐字节相同，Golden 才能零容差。

素材尺寸 904×656 正好等于 1200×800 窗口下 `wrkit` 的 body 尺寸，视频 1:1 画出，不经过缩放采样，Golden 不受采样器实现影响。

## 素材

- 文件：`video/testdata/c4_golden_904x656_nv12.yuv`（889,536 字节，NV12，md5 `22926ecfe1355180b02c331d9ebec03f`）。
- 重生成：`bash video/testdata/gen_c4_golden.sh`。脚本会核对 md5；素材变了必须重新种 Golden 并在提交里说明。
- 八个色块的 U/V 各不相同（不是灰色系自证），所以把 U/V 读反或 UV 行宽算错都会在颜色探针或 Golden 上现形。

## 门禁

自动模式先冻结一帧，再让 retained 路径回放两次（A、B），然后：

- 上传纪律：`video_frames=1`、`video_uploads=1`、`video_fallbacks=0`（固定帧只许上传一次）。
- 出画：桥最近一帧画了视频，合成至少画了一层纹理（`retained_tex_painted_max>=1`），至少上屏一帧。
- 颜色探针：红、绿、蓝、品红四个点，期望值由素材同一位置的 YUV 按 BT.601 limited 现算，容差 8/255（`docs/UI_PIXEL_ASSERTION_STANDARD.md`）。
- 稳定：A、B 两次回放 `diff=0`，否则说明纹理层不是确定性的。
- Golden：A 与 `testdata/showcase_retained_golden_dpr<scale>_base.png` 在视频掩码内逐字节对比，零容差。

任何一项不达标都是 `FAIL:` + `exit 1`。JSON 打到 stdout，含 `pixel_color_ok/total`、`retained_stability_diff_pct`、`pixel_golden_diff_pct`。

## 种基线流程

首次运行没有基线时会写 `testdata/showcase_retained_golden_dpr<scale>_base.png` 并提示「不算 PASS」。必须：

1. 用 `GPUI_ACCEPT_GOLDEN=1` 种下基线，打开 PNG 人眼确认八色块和位置正确；
2. 不带环境变量再跑一次，要求颜色全过、稳定 0、Golden 0；
3. 基线随本次改动一起提交。

基线按 DPR 分文件（`dpr1`、`dpr2`…）。换显示器比例会换文件名，不会拿错尺子的基线报 100% 差异；此时需要在新 DPR 下重新种一份。

## 边界

- 只验 C4 渲染路径：上传 → `RenderVideo` → `scene.TextureLayer` → retained 合成。解码正确性归 VR3，播放器帧率归 2K/4K 窗。
- 人工模式（`-manual-seconds N`，N>=5）常驻显示：真实事件逐个打日志并写进窗口标题，结束时打印计数和 JSON（`mode=manual`、`pixel_evidence=manual_not_asserted`、`pixel_golden_diff_pct=null`），不做快照和门禁。
