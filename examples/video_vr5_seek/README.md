# video_vr5_seek — VR5 跳进度真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_vr5_seek
```

## 人眼可见效果

- 左边四行门禁：双关键帧片跳第二GOP（1800）、跳第一GOP（600）、B帧重排96x96（800）、480p裁边（800），每行显示目标/落点/关键帧/往前解帧数/是否立刻恢复。四跳全绿才算过。
- 右边直播图：双关键帧片用跳进度循环播（播完绕回走跳），5秒跳到1800、10秒跳到600，每次跳完下一拍画面立刻换，不黑屏不花屏。跳点和恢复写在 HUD 行里。
- 落点预算500毫秒（5fps片一帧200毫秒，留足余量）；跳后3秒内必须恢复（实测都是同一拍，几毫秒）。

## 门禁

- 四跳 `seek_ok=1`、`seek_landing_delta_ms≤500`、跳后立刻有画面，否则 `FAIL:` + `exit 1`；第二GOP前解4帧（全片10帧）防从头解偷工。
- 直播至少播出1帧、定时跳≥2次、恢复≤3000毫秒，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 测试片 `video/testdata/vr5_seek.mp4`（96x96/10帧/双IDR）本地生成：`ffmpeg -f lavfi -i testsrc=size=96x96:rate=5:duration=2 -c:v libx264 -profile:v main -pix_fmt yuv420p -bf 2 -g 5 -x264-params scenecut=0:b-adapt=0`，小文件进仓。
