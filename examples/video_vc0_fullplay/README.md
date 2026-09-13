# video_vc0_fullplay — VC0 全链路组合真窗

跑法（关闭用 30 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/video_vc0_fullplay
```

## 人眼可见效果

- 左边五站：同一片 720p 出数——VR0 拆盒（尺寸/帧率/时长/采样/关键帧）、VR1 参数（档位/等级/片头图参数/切帧/IDR）、VR2 解码（帧数+显示序递增）、VR3 转色（首尾帧 MAD 非黑）、VR4 播放（解码=显示=采样、零丢、播到结尾）。任一站红整窗红。
- 右边直播图：同片循环 30 秒，画面一直动，不黑不花，证明组合后进程平稳。
- 底部 HUD：解码/显示/丢帧与直播计数实时可见。

## 门禁

- 五站全过 `chain_ok=1`：拆盒关键帧≥1且时长>0、参数片头图≥1且切帧≥1且 IDR≥1、播到 Ended、显示=解码=采样数、零丢、显示序递增、首尾帧 MAD≥2，否则 `FAIL:` + `exit 1`。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 只做集成回归，不代替任何单能力窗关闭 VR。
