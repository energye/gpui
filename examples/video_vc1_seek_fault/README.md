# video_vc1_seek_fault — VC1 边播边跳+坏文件组合真窗

跑法（关闭用 30 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/video_vc1_seek_fault
```

## 人眼可见效果

- 左上三跳：好片双 IDR（96x96）跳第二 GOP（1800）、第一 GOP（600）、跨戳（1700），每行显示目标/落点/关键帧/往前解/恢复。跳完画面对得上，不黑屏不花屏。
- 左下五坏例：缺文件、非 MP4、截断尾、花屏隔离 F20、F17 分区，每例点名层+工具人话。坏例跑完好片照播，互不影响。
- 右边直播：好片边播边跳（10 秒跳 1800、20 秒跳 600，播完绕回走跳），每次跳完下一拍画面立刻换。跳点和恢复写在 HUD 行里。
- 落点预算 500 毫秒（5fps 片一帧 200 毫秒，留足余量）；跳后 3 秒内必须恢复（实测都是同一拍，几毫秒）。

## 门禁

- 三跳 `seek_ok=1`、落点差≤500、跳后立刻有画面，否则 `FAIL:` + `exit 1`。
- 五坏例 `fault_cases_pass==total`（5/5），坏输入零崩溃，否则挂。
- 直播至少播出 1 帧、定时跳≥2 次、恢复≤3000 毫秒，否则挂。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 只做集成回归，不代替任何单能力窗关闭 VR。
