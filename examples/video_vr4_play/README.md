# video_vr4_play — VR4 播放管线真窗

跑法（关闭用 30 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/video_vr4_play
```

## 人眼可见效果

- 左边三行门禁：B 帧重排 96x96、480p 裁边、720p，每行显示档位、解码帧数、显示帧数、丢帧数。三档全播出来、零丢帧才算绿。
- 右边直播图：第一档循环播，中间三分之一时间暂停（画面定住），后三分之一恢复（画面继续）。暂停和恢复写在 HUD 行里。
- 底部队列水位进 HUD：解码帧数、显示帧数、丢帧数实时可见。

## 门禁

- 三档全播出，`yuv_ready=1`，`dropped_old_frames=0`（小片跟得上不该丢；追帧计数由单测锁），直播至少 1 帧，否则 `FAIL:` + `exit 1`。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
