# video_vr2_decode — VR2 H.264 画面解码真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_vr2_decode
```

## 人眼可见效果

- 左边三行门禁：B 门禁 96x96、480p 裁边、720p，每行显示档位、帧数、和对照图的差异点数。三行全是差异 0 才算绿。
- 右边两张小图并排：左边是解出来的首帧，右边是对照真值首帧（只看亮度灰度，颜色是 VR3 的事）。两张图肉眼一致，差异率进门禁。
- 底部两行小字：1080p 以上大档在本地另验（文件太大不进仓库），隔行只认不解（报错进单测）。

## 门禁

- 解出 15 帧（三档各 5 帧），`yuv_ready=1`，`pixel_golden_diff_pct=0`，否则 `FAIL:` + `exit 1`。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
