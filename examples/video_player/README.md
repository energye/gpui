# video_player — 完整播放小样（能点能拖，只演示用法）

跑法：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/video_player [clip.mp4]
# 或：CLIP=/path/to/clip.mp4 go run ./examples/video_player
# 默认播 video/testdata/vr2_720p.mp4
# 只跑一阵自动退（验证用）：RUN_SECONDS=15 go run ./examples/video_player
```

关窗即退（点窗口 X，或按 Q/Esc）。

## 人眼可见效果

- 中间大画面播视频，底下是播放/重播按钮 + 进度条 + 时间 + 状态。
- 有音轨的片子默认就出声（喇叭里直接响，状态栏显示“出声 paplay-pulse”）；
  文件名后面标着音频规格（如“音频 48.0kHz立体声”）。
- 没音轨的片子照常播画面（标“无音轨”）；没喇叭的机器也照常播画面
  （标“无喇叭，只播画面”），从不报错卡死。
- 点按钮、点进度条、敲键盘，画面立刻有反应，不黑不卡死。
- 窗口随便拉大拉小，画面跟着等比放大缩小，永远不变形，按钮和进度条贴着底边走。

## 操作

- 空格 / P，或点播放按钮：播 / 停（播完后按它重播）。
- 点打开按钮，或按 O：弹系统选片框（有 zenity/kdialog 时），没装就把片子拖进来。
- 把文件管理器里的片子直接拖进窗口：松手就换片播。
- ← / →：退 1 秒 / 进 1 秒；↑ / ↓：±5 秒；Home / End：头 / 尾；R：重播。
- 点进度条：跳到那里。Q / Esc：退出。
- 换片失败（格式不对、文件没了）只红字提示，正在播的不停。

## 架构（一句话）

`video` 只出自有帧（宽高 + 像素 + 时间戳）和 PCM（采样 + 时间戳），
窗侧转成 `render/ImageBuf` 送显示、送系统喇叭：
`OpenFile → Poll 取帧 → 行拷贝上墙 → Pause/Resume/SeekTo`，
声音另起泵线程 `PollAudio → 转 s16 → paplay/aplay`（`examples/hostsink`
包，和 A4 门禁窗同路，门禁演习件除外）。
暂停/变速/跳转不用管泵：引擎双钟同冻同走，泵自己跟上。
`video` 本体不碰 `render`/`ui`，桥接只在窗侧（合规门禁同 VR 窗）。

## 不是门禁窗

- 不出 §2.2 JSON，不判 FAIL，不进 VR/VC 表，不算关窗。
- 要看判定去隔壁：全链路看 `video_vc0_fullplay`，边播边跳看
  `video_vc1_seek_fault`，长跑看 `video_vc2_soak`，内嵌+注册看
  `video_vc3_embed_extend`。
