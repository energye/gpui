# video_vr1_params — VR1 参数与切分真窗

1200×800，关闭用 `RUN_SECONDS=5`。

## 人眼可见效果

- 左边是参数面板：来源、档位和三档认可情况、等级、尺寸、熵编码是哪套、片头和图参数个数加流内个数、打包写法、NAL总数和切出帧数、解析耗时。
- 右边是类型统计：片头参数、图参数、关键帧、切片、补充信息、分隔符各多少个，横条越长越多；底下列前 4 帧，每帧写第几帧、是不是关键帧、几个切片、多少字节。
- 底部一行是缺参数坏流结果：拿切片指着从没见过的参数集去查，必须报中文可读错，不崩；绿字是正常报错，红字是没报对。
- 顶部大字 `VR1 H.264参数与切分`，第二行是解析状态。
- 右下角小方块随阶段变色移动，证明窗口持续出帧；底部是实时帧率条。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/video_vr1_params
```

指向真机片子（不进仓库，走盒内长度前缀加带外参数，再读前 8 个采样切帧）：

```bash
VIDEO_MP4_PATH=/path/to/phone.mp4 RUN_SECONDS=5 go run ./examples/video_vr1_params
```

回归基线：

```bash
SAVE_BASELINE_JSON=/tmp/vr1_base.json RUN_SECONDS=5 go run ./examples/video_vr1_params
BASELINE_JSON=/tmp/vr1_base.json RUN_SECONDS=5 go run ./examples/video_vr1_params
```

## 门禁（不满足就 `FAIL:` 加 `exit 1`）

- 全族 JSON 缺键就挂，键见 `main.go` 的 `requiredKeys`。
- 真窗 `present_count>=1`，不空跑。
- `sps_ok=1`，`pps_ok=1`，`frames_split>=1`。
- 等级不在 1–5.2 就挂；超限大等级去 VR6 报可读错，这里先卡住。
- 缺参数坏流没报对就挂。
- 首帧 `time_to_first_frame_ms>2000` 就挂。
- 内存峰值超 `mem_cap_kb`（524288）就挂。
- `RUN_SECONDS<5` 直接挂。

正确性窗不硬卡 fps，但 `vsync_source` 必须输出；空值会填 `unknown`。

## 预算与容差

- 参数解析加切帧是冷路径，一次约零点几到一毫秒，`decode_ms_avg` 和 `decode_ms_p95` 取同一次实测，只如实输出。
- 真片只读前 8 个采样且总量封顶 2MB，窗口不把几百兆全吞进来。
- 内存封顶 512MB，超了就是泄漏。
- 像素 Golden 暂按 0 上报：本窗是参数面板加统计条，不是视频画面，画面对靠面板存在加统计计算加持续出帧三者合看；逐位 Golden 基线等稳态快照留档后再收紧。

## 哪些字段是 N/A

本窗只做参数和切分，没有解码队列、时钟、跳进度、YUV、颜色，所以这些全报 0，原因写在 `n_a_reason`。

- `queue_depth_avg、queue_depth_max、dropped_old_frames、alloc_per_frame_B、pool_hit_pct` 全 0。
- `clock_drift_ms、seek_ok、seek_landing_delta_ms` 全 0。
- `yuv_ready、color_diff_per_channel、pixel_golden_diff_pct` 全 0。
- `cpu_decode_pct` 报 0，解析耗时已进 `decode_ms`。

## 实现策略、边界、失败条件

- 正确：合成流和真片都走 `video/h264` 真解析，面板数字全从解析结果来；三档认可在窗内同步用合成基础和主档单元验证，真片只证明它自己的那档。
- 脏：好输入解析失败也进窗显示红字，但终判挂；缺参数流走参数集查缺口报错。
- 缓存：无帧池，本窗只有一次解析，`pool_hit_pct` 按 N/A 报 0。
- 边界：两种打包法都认（盒内长度前缀和起始码）；带内带外参数集都收；条带组、数据分区、扩展切片一律报可读错不乱切；无起始码、零长度、截断长度全报错。
- 失败：无视频轨、参数区坏、采样读不到、切片头坏，全转可读错，不崩。
- 可见：参数 8 行加类型统计加前 4 帧加坏流一行加状态大字，缺一块就是窗口 bug。
