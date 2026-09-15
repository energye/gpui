# video_a2_sync — A2 音画对齐真窗

1200×800。左边四组探针（走真引擎），右边有声片循环直播。

跑法：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_a2_sync -auto-only
go run ./examples/video_a2_sync -manual-seconds 30
```

人眼可见：左边 identity/play/seek/silent 四组全过（8/8），右边 320x240
测试图循环播，5 秒与 10 秒各跳一次后画面恢复，HUD 显示声画差（A-V）
始终在 200 毫秒内，主钟为 audio。声音本身不出喇叭（出声是 A4 的事），
本窗只证明声钟领着画钟走、跳进度双队同序号、静音片回落画面主钟。

门禁（-auto-only，见 main.go）：探针 8/8 + 直播有画面 + 声音有包 +
主钟 audio + 声画差≤200ms + 定时跳≥2 + 跳后恢复≤3s + 首帧≤2000ms +
内存峰值≤上限 + §2.2 全族键齐，不达标 `FAIL:` + `exit 1`。
