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

## 门禁（§12.1 VR3 行，对等线，自定 3 已删）

- 11 组向量逐字节零差异（`video/color/testdata/vr3_vectors.json`，`maxDiff==0`），真帧至少 1 帧，`yuv_ready=1`，否则 `FAIL:` + `exit 1`。
- 三片对等（`video/testdata/vr3_ffmpeg.json`）：`vr2_b_intra` 96x96 + `vr2_480p` 854x480 + `vr2_720p` 1280x720，各 5 帧播到 `Ended` 且每帧 `rgba md5` 等于基线 `ours_rgba`；基线与 ffmpeg 差分布为 R 最大 2、G 最大 3、B 最大 2，R/B `P99≤2`、G `P99≤3`，任一片超线即 `FAIL:`。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。

## 对等说明（ffmpeg 4.4.2，同机）

对等点：`libswscale/yuv2rgb.c:ff_yuv2rgb_coeffs` 系数表 + `YUV2RGBFUNC` 查表转色、
`libswscale/output.c:yuv2rgba64_*_c_template` 矩阵数学、`libswscale/swscale.c:sws_scale`
分片转色，对我方 `video/color/color.go:tableFor` + `convertBand`。

采数命令：

```bash
ffprobe -v error -select_streams v:0 -show_entries stream=width,height,avg_frame_rate,codec_name,profile,level,duration,nb_frames -of default=nw=1 <clip>
ffmpeg -v error -y -i <clip> -pix_fmt rgba -f rawvideo <clip>.rgba
```

三片聚合（解码已由 VR2 锁死为零差异，此处纯转色差）：
`b_intra` 46080 像素最大 2/3/2、`P99` 2/2/2、均值 0.4941（首帧 9216 像素均值 0.4662）；
`480p` 2049600 像素最大 2/3/2、`P99` 2/2/1、均值 0.4161；
`720p` 4608000 像素最大 2/3/2、`P99` 2/2/1、均值 0.4145。
超差的是 swscale 8 位查表舍入噪声；同批拿独立浮点公式再算最大差只有 1，说明公式本身是对的。
旧自定 `≤3` 作废，改按本节 `P99` 线；不支持的颜色矩阵和采样格式报人话错，见 `video/color` 单测。

## 性能基线（本机 i5-6200U）

- 320x240 转色约 0.56 毫秒，复用缓冲零分配；相对初版省约 20%（背靠背三次：0.70→0.56毫秒，输出逐位不变）。
- 1080p 单线程约 18 毫秒：60 帧预算下必须并行化，这个数就是 VR4/VR7 的起跑线。
