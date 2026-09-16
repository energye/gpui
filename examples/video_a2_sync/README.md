# video_a2_sync — A2 音画对齐真窗

1200×800。左边四组探针（走真引擎），右边有声片循环直播。

跑法：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_a2_sync -auto-only
go run ./examples/video_a2_sync -manual-seconds 30
```

人眼可见：左边 identity/play/seek/silent 四组全过（8/8），右边真片
（vr_oceans.mp4，960x400，480x200 显示）循环播，5 秒与 10 秒各跳一次
后画面恢复，HUD 显示声画差（A-V，如实上报）和主钟 audio。
声音本身不出喇叭（出声是 A4 的事），本窗只证明声钟领着画钟走、
跳进度双队同序号、静音片回落画面主钟。门禁片已换真片：旧 77KB
合成小片盖不住泵漂移，已删（见 video/testdata/a2_ffmpeg.json）。

门禁（-auto-only，见 main.go）：探针 8/8 + 直播有画面 + 声音有包 +
主钟 audio + 定时跳≥2 + 跳后恢复≤3s + 首帧≤2000ms +
内存峰值≤上限 + §2.2 全族键齐，不达标 `FAIL:` + `exit 1`。
窗内声画差只上报不限门禁：本机约 10 帧每秒渲染下，读数是画面滞后
不是同步坏（§6.2），200 毫秒线由无头泵门禁（同片）继续卡。
