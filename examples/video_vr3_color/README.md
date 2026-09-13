# video_vr3_color — VR3 颜色转换真窗

跑法（关闭用 5 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/video_vr3_color
```

## 人眼可见效果

- 左边三行门禁：向量组数与每通道最大差、真 I 帧解码加转色耗时、范围矩阵与注册表。最大差写在标题行，全绿才算过。
- 右边五对色块并排：上是转出来的，下是理论色（红、绿、蓝、肤色、中灰）。两排肉眼一致，差值进门禁。
- 灰阶条两条：上是转出来的渐变，下是理论渐变，从黑到白不断层。
- 右侧大图是真解出来的 I 帧转彩色后的样子（之前 VR2 只敢摆灰度图）。

## 门禁

- 11 组向量每通道差 `≤3`，真帧至少 1 帧，`yuv_ready=1`，否则 `FAIL:` + `exit 1`。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。

## 容差说明

容差 3 不是拍脑袋：拿真解码帧逐像素对照 ffmpeg 4.4 默认 swscale 输出，
9216 像素里 R/B 通道最大差 2、G 通道最大差 3、均值 0.47，
超差的全是 G 通道固定 +3（swscale 8 位查表舍入噪声）。
同一批真值拿独立浮点公式再算一遍，最大差只有 1（纯舍入），
说明转色公式本身是对的，3 只是给对照工具的噪声留余量。
不支持的颜色矩阵和采样格式报人话错，见 `video/color` 单测。
