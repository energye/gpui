# video_vc3_embed_extend — VC3 三合一组合窗（VR8内嵌 + VR9注册 + VR4播放）

跑法（关闭用 30 秒）：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/video_vc3_embed_extend
```

## 人眼可见效果

- 左边四行：注册 8/8（探测mp4/h264+能力齐+注册播完+灰桩+三坏例人话零崩）+ 内嵌冷解码（5帧播完零丢递增非黑）+ 直播同屏说明。任一行红整窗红。
- 右边内嵌：同片 720p 循环 30 秒，主 480x270 + 同源小窗 240x135（缩放只走显示目标矩形，解码仍全 720p）；主窗包圆角裁剪 + 整组透明，半透明浮层盖住主窗一角，旁边面板文字同屏；10 秒缩一次 20 秒还原，画面不断不崩不花。注册用例数与灰桩像素同屏可见。
- 底部 HUD：fps/解码/队列/内存，外加注册用例与改尺寸阶段。

## 门禁

- 三合一全过 `combo_ok=1`：内嵌 `embed_ok=1`（直播>0 帧、尾帧 MAD≥2、两尺寸同源、裁剪+透明节点就绪、浮层就绪、改尺寸走完且改后直播继续涨）且注册 `registry_cases_pass==total`（探测+能力+注册名+注册播+灰桩+三坏例共 8 项），否则 `FAIL:` + `exit 1`。
- 同片走注册表播到 Ended，不支持的盒子/编码/采样报人话错零崩溃，否则挂。
- 动画窗 60 档：`fps_wall≥55` 且 `interval_p95_ms≤22`，否则挂。
- §2.2 全族 JSON 打到 stdout，缺键就挂。支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` 回归。
- 首帧超 2000 毫秒挂，内存峰值超 1GB 上限挂。
- 只做组合集成回归，不代替任何单能力窗；`video` 本体仍不碰 `render`，桥接只在窗侧。
