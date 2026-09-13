# video_vr8_embed — VR8 真窗内嵌真窗

跑法（关闭用 30 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/video_vr8_embed
```

## 人眼可见效果

- 左边四行：同片 720p 出数——拆盒（1280x720/5fps/5采样/关键帧≥1）、参数（档位/等级/片头图参数/切帧/IDR）、冷解码（均值/p95+显示序递增）、转色（首尾帧 MAD 非黑+播完零丢）。任一行红整窗红。
- 右边内嵌：同片 720p 循环 30 秒，主 480x270 + 同源小窗 240x135（缩放只走显示目标矩形，解码仍全 720p）；主窗包圆角裁剪 + 整组透明，半透明浮层盖住主窗一角，旁边面板文字同屏；10 秒缩一次 20 秒还原，画面不断不崩不花。
- 底部 HUD：fps/解码/队列/内存，外加直播帧数与改尺寸阶段。

## 门禁

- 内嵌全过 `embed_ok=1`：直播>0 帧、尾帧 MAD≥2、两尺寸同源、裁剪+透明节点就绪、浮层就绪、改尺寸走完且改后直播继续涨，否则 `FAIL:` + `exit 1`。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 只做内嵌验收，不代替任何解码/播放窗；`video` 本体仍不碰 `render`，桥接只在窗侧。
