# video_a4_sink — A4 宿主出声真窗

1200×800。左边五组探针（走真引擎），右边有声片循环播，声音真进喇叭。

跑法：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/video_a4_sink -auto-only
go run ./examples/video_a4_sink -manual-seconds 30
```

放自己的片子（只换直播这一路，探针永远钉在门禁小片上）：

```bash
RUN_SECONDS=15 go run ./examples/video_a4_sink -auto-only -file video/testdata/vr_oceans.mp4
go run ./examples/video_a4_sink -file /path/to/你的.mp4
```

`-file` 是演示档：探针照样全过才开窗，直播的画面和声音走你的片子，
声画差只如实上报不限 200 毫秒（JSON 里 `live_gate` 会标 `demo`，
默认门禁片是 `strict`）。原因：200 毫秒线是按 320x240 门禁片标的，
大片在窗里会系统性落后：窗内解码约 14.6 帧每秒（和渲染抢 CPU，
背景解码跟不上片子本身的 24 帧），每拍约 52 毫秒而构建加光栅
不足 0.3 毫秒（present 与上传路的账，归渲染线），声音链路本身
零丢实时。所以声画差读数报的是画面滞后，不是声音坏。

人眼可见：左边 identity/convert/wav/pump/device 五组全过，右边 320x240
测试图循环播，喇叭里是 440Hz 正弦声，HUD 显示播了多少声音包；
第 8 秒做一次拔线演习（主动杀掉出声子进程模拟拔耳机），泵暂停并报
可读错，约 2 秒后自动恢复继续响，画面全程不断。
声音本身由宿主侧送喇叭：paplay 优先（走 Pulse），没有再走 aplay，
都没才诚实报缺设备；引擎只出 PCM，不管喇叭（§4 纪律）。

门禁（-auto-only，见 main.go）：探针非设备组全过 + 直播有画面 +
喇叭吃到包 + 主钟 audio + 声画差≤200ms + 拔线演习走完（杀→停→恢，
且留下可读错）+ 首帧≤2000ms + 内存峰值≤上限 + §2.2 全族键齐，
不达标 `FAIL:` + `exit 1`。演习那几秒和循环绕回那几拍的声画差不计入
（读数本身是前后两帧相减，故障窗口内采它等于罚演习成功），门禁只看
平稳期的差。JSON 里能看到 `live_max_av_ms` 是平稳期最大值。

无喇叭的机器：不装绿。探针里 device 组记 0/1，JSON 里
`verdict` 为 `skip`，`exit 0`，stderr 写 `SKIP:` 原因；
WAV 链（头 3 包逐位对）证明送喇叭的字节本身是对的。

手动拔线：播着的时候拔耳机（或 `pactl` 挂起声卡），窗里 drill
状态会显示暂停和可读错，插回去约 2 秒自动恢复；线程不死，
时钟不跳，恢复后接着响。

诊断键（只上报，不限门禁，见 main.go `buildReport`）：

```text
queue_depth_avg/max  播放器帧队列水位真值（之前写死 0）
dropped_old_frames   音频主钟下追帧丢的旧画面真值
clock_drift_ms       播放器时钟漂移真值
convert_ms_avg/p95   转色耗时真值（注意：播放器统计只给转色计时，
                     H.264 解码本身不计时，所以 decode_ms_* 保持 0
                     并在 n_a_reason 里写明，不拿转色冒充解码）
frame_build_ms       窗口构建耗时（调度快照真值）
frame_raster_ms      窗口光栅耗时（调度快照真值）
present_mode/policy  本次 present 形态与策略（调度快照真值）
gpu_backend          适配器种类：integrated（本机 Intel Vulkan 默认，
                     零回退）/ discrete（GPUI_POWER=high 切 NVIDIA）/
                     software / unknown；gpu_fallbacks 为打开时降级级数
display_backend      显示后端（本机 x11）
audio_decoded/shown/depth 引擎侧音频编解水位（和泵的 played 包数对照看）
```

真机说明：本机是真 GPU 桌面（Intel 集显 + NVIDIA 940MX，
glxinfo 直连 NVIDIA）。窗口默认走集显 Vulkan（`gpu_backend`
会如实报 `integrated`），`GPUI_POWER=high` 切独显（报 `discrete`）。
大片在两档下都慢得差不多（约 18 帧上屏、每拍约 53 毫秒），
说明瓶颈不在 GPU 算力，在 CPU 侧（解码被争用 + 整面上传），
提速归解码 S1/S2 线与渲染线，本窗不藏数。
