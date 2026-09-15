# video_s8_index — S8 跳转索引真窗

跑法（关闭用 15 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_s8_index -auto-only
go run ./examples/video_s8_index -manual-seconds 30
```

## 人眼可见效果

- 左边三行门禁：跳8次（小片2跳+长片6跳，落点等于线性扫描）、播出8尾（跳后尾帧立刻播出来，不黑屏不倒退，首现晚于落点只允重排延迟800毫秒内）、路径2项（小片走缓冲快路、长片走索引流式）。三组全绿（18/18零坏）才算绿。
- 右边直播：流式长片（320x240/200帧/40组/4段）用索引跳循环播，5秒跳到20000、10秒跳到5000，播完绕回走跳，每次跳完下一拍画面立刻换，不黑屏不花屏。跳点和恢复写在 HUD 行里。
- 步数对数级由 `video/s8_index_test.go` 锁死（长片 bound 20/24/26，实测 10/11/12），窗内只验落点一致与可见恢复，不重算步数。

## 门禁

- 三组全过：`passed==total_items==18` 且 `failed_items==0`，前解远小于全片（防从头解偷工），否则 `FAIL:` + `exit 1`。
- 18 = 跳8次落点等于线性 + 播出8尾 + 路径2项，与 `video/s8_index_test.go` 三项一一对上（逐戳邻域逐位一致+步数对数级+流式建索引/小片无索引），输出逐位一致；播出尾帧语义与 `video/b1_ffmpeg_test.go` 跳转段同口径（首现≥落点、单调、800毫秒内）。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 上屏至少1次，直播至少1帧、定时跳≥2次、恢复≤3000毫秒，否则挂。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- `-auto-only`：先跑自检（不过直接 exit 1 不开窗），再开短窗取数判门禁。
- `-manual-seconds N`：常驻N秒收真事件，每事件打日志并改标题显示累计数，结束出汇总JSON；不带N则常驻到关窗。
- 测试片 `video/testdata/vr5_seek.mp4`（96x96/10帧/双IDR）与 `video/testdata/vr_stream_long.mp4`（320x240/200帧/40组）已进仓常跑。
