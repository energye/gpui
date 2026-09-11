# video_vr0_demux — VR0 MP4 拆盒真窗

1200×800，关闭用 `RUN_SECONDS=5`。

## 人眼可见效果

- 左边是文件信息面板：来源、品牌、编码、显示尺寸、旋转、时间刻度、时长、帧率、采样数、关键帧数、avcC、宽高比、编辑列表、拆盒耗时、文件字节数。
- 右边是关键帧位置条：深色底条上绿色小块是关键帧，位置按显示时间戳除以总时长排，底下写前 4 个关键帧的采样号和毫秒数，再底下写总时长和关键帧总数。
- 底部一行是坏盒子结果：截断 100 字节的坏输入必须给出中文可读错，不崩；前面绿字 `坏盒子正常报错` 是过，红字 `坏盒子没报对` 是挂。
- 顶部大字 `VR0 MP4拆盒`，第二行是拆盒状态，绿字是通过预览，红字是失败原因。
- 右下角有个随相位变色移动的小方块，证明窗口在持续出帧；底部是实时 fps 和门禁预览条。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/video_vr0_demux
```

指向真机片子（不进仓库）：

```bash
VIDEO_MP4_PATH=/path/to/phone.mp4 RUN_SECONDS=5 go run ./examples/video_vr0_demux
```

回归基线：

```bash
SAVE_BASELINE_JSON=/tmp/vr0_base.json RUN_SECONDS=5 go run ./examples/video_vr0_demux
BASELINE_JSON=/tmp/vr0_base.json RUN_SECONDS=5 go run ./examples/video_vr0_demux
```

## 门禁（不满足就 `FAIL:` 加 `exit 1`）

- 全族 JSON 缺键就挂，键见 `main.go` 的 `requiredKeys`。
- 真窗 `present_count>=1`，不空跑。
- `track_found=1`，`keyframe_count>=1`，`duration_ms>0`。
- 坏输入必须可读报错且不崩，`bad_file_ok` 为假就挂。
- 首帧 `time_to_first_frame_ms>2000` 就挂（本机约几十毫秒）。
- 内存峰值超 `mem_cap_kb`（524288）就挂。
- `RUN_SECONDS<5` 直接挂。

正确性窗不硬卡 fps，但 `vsync_source` 必须输出；空值会填 `unknown`。

## 预算与容差

- 拆盒是冷路径，一次解析约零点几毫秒，`decode_ms_avg` 和 `decode_ms_p95` 取同一次实测，不设硬上限，只如实输出。
- 内存封顶 512MB，5 秒窗远到不了，超了就是泄漏或无界增长。
- 像素 Golden 暂按 0 上报：本窗是信息面板加位置条，不是视频画面，画面对靠面板存在加位置计算加持续出帧三者合看；同引擎逐位 Golden 基线等稳态快照留档后再收紧。

## 哪些字段是 N/A

本窗只做拆盒，没有解码队列、时钟、跳进度、SPS、PPS、YUV、颜色，所以这些全报 0，原因写在 `n_a_reason`：

`demux-only: queue/clock/seek/sps/pps/yuv/color/golden N/A in VR0`

- `queue_depth_avg、queue_depth_max、dropped_old_frames、alloc_per_frame_B、pool_hit_pct` 全 0。
- `clock_drift_ms、seek_ok、seek_landing_delta_ms` 全 0。
- `sps_ok、pps_ok、frames_split、yuv_ready、color_diff_per_channel、pixel_golden_diff_pct` 全 0。
- `cpu_decode_pct` 报 0，拆盒耗时已进 `decode_ms`。

## 实现策略、边界、失败条件

- 正确：合成 MP4 走 `video/mp4.Parse` 真解析，面板数字全从解析结果来，不手写假数；`VIDEO_MP4_PATH` 指向真机片子时走 `Parse` 同一路。
- 脏：好输入解析失败也会进窗显示红字，但终判挂；坏输入截断尾部，报截断错。
- 缓存：无帧池，本窗只有一次解析，无复用可谈，`pool_hit_pct` 按 N/A 报 0。
- 边界：无 `stss` 当全是关键帧由引擎层保证；`stco、co64` 双写只认一个；`ctts` 缺席当显示时间等于解码时间。
- 失败：缺 `moov`、无视频轨、碎片 `moof`、盒子越界全转可读错，不崩。
- 可见：信息 8 行加关键帧条加坏错行加状态大字，缺一块就是窗口 bug。
