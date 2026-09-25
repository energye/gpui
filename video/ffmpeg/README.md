# video/ffmpeg — ffmpeg 的 Go 遥控器

大白话：这个包让你在 Go 里直接使唤 ffmpeg，不用写一行 C，也不用开 CGO。
它连的是仓库自带的单文件库 `libgpui_ffmpeg.so`（ffmpeg 7.1.5），
打开视频、解码、转颜色、重采样声音，全是调这个库干活。

包里分两层，用哪层看你要干什么：

- **只想播视频**：只认高层解码器 `decode.go`。`Open(路)` 打开，
  `Next()` 一帧一帧取（RGBA，到手就能显示），`SeekTo(毫秒)` 跳进度，
  `Close()` 关掉。`video` 的播放器就是这么接的，别绕过去。
- **想自己使唤 ffmpeg 干别的**（比如只拆盒不解码、只转音频、挂滤镜）：
  用下面 13 个功能模块，so 里能用的公开函数全导出来了，
  每个都是 Go 写法，直接调就行。

## 库文件在哪

- 缺省在 `gpui/lib/ffmpeg/<系统>-<架构>/` 下找（共 6 个构建：linux 四架构 + win 双架构，mac 待补）。
- 环境变量 `GPUI_FFMPEG_PATH` 可以指定路径，测试和特殊目录用它。
- 库不在就别硬调：先问 `Available()`，回来 false 说明库没加载上。
- `LibPath()` 告诉你最后用的是哪个文件，`Version()` 报版本号（7.1.5）。

## 配套小文件（不是 13 模块，但是包的一部分）

| 文件        | 管什么                                                              |
|-------------|---------------------------------------------------------------------|
| `decode.go` | 高层解码器：`Open` / `Next` / `SeekTo` / `Close`，播放唯一入口       |
| `lib.go`    | 加载 so + 注册全部函数 + 解码直连的老接口（和模块指同一个 so 函数） |
| `types.go`  | `AVRational` 分数（时间换算用，偏移按 7.1 头文件钉死）              |
| `err_go.go` | 内部帮手：空指针哨兵、C 字符串转 Go、错误码翻人话                   |
| `doc.go`    | 包说明 + 覆盖口径（哪 4 个函数主动跳过，见下）                      |

## 13 个功能模块一览

| 文件 | 一句话：它是干什么的 | 拿到的结构体（括号里是它抱着的 C 指针） |
|------|----------------------|------------------------------------------|
| `packet.go` | 数据包：拆盒吐出来的就是它，一包一包喂给解码器 | `Packet`（包）、`PacketSideData`（包的附加数据） |
| `frame.go` | 帧：解出来的画面或声音，一帧就是一张图或一段声 | `Frame`（帧）、`FrameSideData`（帧的附加数据） |
| `dict_opt.go` | 字典和选项：打开文件、开解码器时传参数都走它 | `Dictionary`（键值对）、`OptObject`（可调参数的对象） |
| `buffer_mem.go` | 内存：申请、引用计数、缓冲池、队列、拼字符串 | `Mem`、`Buffer`、`BufferPool`、`Fifo`、`BPrint` |
| `error_log.go` | 报错和杂务：错误码翻人话、日志开关、版本、时间戳换算、CPU 数 | `Library`（版本）、`Log`（日志）、`Math`（时间换算）、`Clock`（时钟）、`Cpu` |
| `format_demux.go` | 拆盒：打开文件、找音视频流、读包、跳进度 | `FormatContext`（文件上下文）、`Stream`（流）、`IOContext`（读写）、`Format`（格式） |
| `codec_encode.go` | 编解码：找解码器、开解码器、送包取帧、码流过滤 | `CodecContext`（解码器实例）、`Codec`（解码器本身）、`CodecParameters`（流参数）、`Parser`（切帧）、`BitStreamFilter`（码流滤镜） |
| `scale_color.go` | 转色和缩放：YUV 转屏幕要的 RGBA，尺寸也能顺手改 | `Scaler`（转色器）、`Image`（图片帮手）。常用常量：`PixFmt*`（像素格式）、`Sws*`（算法） |
| `resample_audio.go` | 音频重采样：声道、采样率、采样格式不一样时掰成一样 | `Resampler`（重采样器）、`AudioFifo`（音频队列） |
| `filter_graph.go` | 滤镜图：把多个滤镜连成链（缩放、裁剪、混音都归它） | `FilterGraph`（图）、`FilterContext`（滤镜实例）、`Filter`（滤镜本身）、`FilterSink`（出口）、`FilterSource`（入口） |
| `device_io.go` | 设备：列摄像头、麦克风这些输入输出设备 | `DeviceList`（设备表） |
| `media_desc.go` | 查资料：像素格式、声道布局、采样率这些只读信息，不干活只问 | `MediaDesc`（无状态，全是查询） |
| `crypto_hash_misc.go` | 杂项工具箱：硬解设备、哈希加密、写文件、猜格式、小计算 | `HWDevice`（硬解设备）、`Crypto`（哈希校验）、`Muxer`（写文件）、`Prober`（猜格式）、`Samples`（采样帮手）、`Util`（零碎小函数） |

## 用法约定（13 个模块统一）

- 第一个参数是“谁的”指针，就挂成那个结构体的方法。
  比如 `avcodec_send_packet(解码器, 包)` 写成 `codecCtx.SendPacket(pkt)`。
- 返回错误码的函数：名字在 keep 名单里的留 `int32`，
  剩下的直接转 Go 的 `error`，负数就是出错，不用自己查表。
- 空指针安全：接收器是 nil 时直接返回零值或哨兵错，不会崩。
- `Free` / `Unref` / `Close` 结尾的就是“用完还回去”，记得调，不然漏内存。
- 传字符串只管传 Go 的 `string`，取回来的 C 字符串包里已经转好了。

## 覆盖口径：绑了多少、没绑哪几个

- so 里 `av*`（除 `avpriv` 内部）/`sws_*` / `swr_*` 开头的符号一共 995 个，绑了 991 个。
- 另外 11 个周边符号也在 so 里导出了，顺手一起绑了：
  `swscale_*` / `swresample_*` 的版本配置（各 3 个），
  加 5 个 `swri_*` 重采样帮手。去重后一共 1002 个，`nm -D` 双向对过。
- 4 个函数主动没绑，都是 C 的变参函数（参数个数不定），purego 写不出来：
  `av_asprintf`、`avio_printf`、`av_log_once`、`av_strlcatf`。
  替代写法：字符串先在 Go 里拼好再传；拼串用 `BPrint` 那套；
  日志用 `Log.SetLevel` + 定长消息。
- so 里还剩 13 个 `swri_*` 内部帮手没绑（重采样底层的私有函数，
  公开头文件里没有声明，按只导公开 API 的口径不导）。
- 同一个 so 函数偶尔有两个 Go 入口（比如 `lib.go` 的解码直连和模块方法，
  `sws_*` 在转色和杂项各有一套写法），底层是同一个函数，用哪个都行。

## 快速上手

```go
// 只看三帧长什么样（记得把 GPUI_FFMPEG_PATH 指到你的 so，或放默认位置）:
dec, err := ffmpeg.Open("xxx.mp4")
if err != nil {
    log.Fatal(err)
}
defer dec.Close()
fmt.Println(dec.Info()) // 宽、高、帧率、时长、编码，一句话看完
for i := 0; i < 3; i++ {
    f, err := dec.Next()
    if err != nil {
        break
    }
    fmt.Println(f.Width, f.Height, len(f.RGBA), f.PtsMs)
    f.Release()
}
```

```go
// 按需直调：报错码翻人话，像素格式查名字
msg := ffmpeg.StrError(-1094995529) // "Invalid data found when processing input"
name := ffmpeg.PixFmtName(ffmpeg.PixFmtYUV420P) // "yuv420p"
```

## 全部 API 对照表（Go 入口 → ffmpeg 函数）

下面 13 节就是全部家当：so 里绑的每个函数都在里面，
左边是 Go 里怎么写，右边是它对应的 ffmpeg 原函数。点开看就行。

### packet — 数据包（27 个）

数据包：拆盒吐出来的就是它，一包一包喂给解码器。`NewPacket` 新建一个空包，用完 `Free` 还回去。

抱着的结构体：`Packet`、`PacketSideData`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewPacket` | `av_packet_alloc` |
| `PacketSideData.Ptr` | `av_packet_clone` |
| `PacketSideData.Ptr` | `av_packet_free` |
| `PacketSideData.Ptr` | `av_packet_get_side_data` |
| `PacketSideData.Ptr` | `av_packet_new_side_data` |
| `PacketSideData.Ptr` | `av_packet_pack_dictionary` |
| `PacketSideData.Ptr` | `av_packet_ref` |
| `PacketSideData.Ptr` | `av_packet_copy_props` |
| `PacketSideData.Ptr` | `av_packet_move_ref` |
| `PacketSideData.Ptr` | `av_packet_unref` |
| `PacketSideData.Ptr` | `av_packet_free_side_data` |
| `PacketSideData.Ptr` | `av_packet_add_side_data` |
| `PacketSideData.Ptr` | `av_packet_shrink_side_data` |
| `PacketSideData.Ptr` | `av_new_packet` |
| `PacketSideData.Ptr` | `av_grow_packet` |
| `PacketSideData.Ptr` | `av_shrink_packet` |
| `PacketSideData.Ptr` | `av_packet_from_data` |
| `PacketSideData.Ptr` | `av_packet_make_refcounted` |
| `PacketSideData.Ptr` | `av_packet_make_writable` |
| `PacketSideData.Ptr` | `av_packet_rescale_ts` |
| `PacketSideData.Ptr` | `av_packet_unpack_dictionary` |
| `PacketSideData.Ptr` | `av_packet_side_data_add` |
| `PacketSideData.Ptr` | `av_packet_side_data_free` |
| `PacketSideData.Ptr` | `av_packet_side_data_get` |
| `PacketSideData.Ptr` | `av_packet_side_data_name` |
| `PacketSideData.Ptr` | `av_packet_side_data_new` |
| `PacketSideData.Ptr` | `av_packet_side_data_remove` |
</details>

### frame — 帧（26 个）

帧：解出来的画面或声音，一帧就是一张图或一段声。`NewFrame` 新建空帧，用完 `Free` 还回去。

抱着的结构体：`Frame`、`FrameSideData`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewFrame` | `av_frame_alloc` |
| `FrameSideData.Ptr` | `av_frame_free` |
| `FrameSideData.Ptr` | `av_frame_ref` |
| `FrameSideData.Ptr` | `av_frame_clone` |
| `FrameSideData.Ptr` | `av_frame_unref` |
| `FrameSideData.Ptr` | `av_frame_move_ref` |
| `FrameSideData.Ptr` | `av_frame_copy` |
| `FrameSideData.Ptr` | `av_frame_copy_props` |
| `FrameSideData.Ptr` | `av_frame_get_buffer` |
| `FrameSideData.Ptr` | `av_frame_is_writable` |
| `FrameSideData.Ptr` | `av_frame_make_writable` |
| `FrameSideData.Ptr` | `av_frame_apply_cropping` |
| `FrameSideData.Ptr` | `av_frame_get_plane_buffer` |
| `FrameSideData.Ptr` | `av_frame_get_side_data` |
| `FrameSideData.Ptr` | `av_frame_new_side_data` |
| `FrameSideData.Ptr` | `av_frame_new_side_data_from_buf` |
| `FrameSideData.Ptr` | `av_frame_remove_side_data` |
| `FrameSideData.Ptr` | `av_frame_side_data_add` |
| `FrameSideData.Ptr` | `av_frame_side_data_clone` |
| `FrameSideData.Ptr` | `av_frame_side_data_desc` |
| `FrameSideData.Ptr` | `av_frame_side_data_free` |
| `FrameSideData.Ptr` | `av_frame_side_data_get_c` |
| `FrameSideData.Ptr` | `av_frame_side_data_name` |
| `FrameSideData.Ptr` | `av_frame_side_data_new` |
| `FrameSideData.Ptr` | `av_frame_side_data_remove` |
| `FrameSideData.Ptr` | `av_frame_replace` |
</details>

### dict_opt — 字典和选项（62 个）

字典和选项：打开文件、开解码器时传参数都走它。`NewDictionary` 建空字典，`Opt` 取各对象的可调参数。

抱着的结构体：`Dictionary`、`DictionaryEntry`、`OptObject`、`Option`、`OptionRanges`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `OptionRanges.Ptr` | `av_dict_copy` |
| `OptionRanges.Ptr` | `av_dict_count` |
| `OptionRanges.Ptr` | `av_dict_free` |
| `OptionRanges.Ptr` | `av_dict_get` |
| `OptionRanges.Ptr` | `av_dict_get_string` |
| `OptionRanges.Ptr` | `av_dict_iterate` |
| `OptionRanges.Ptr` | `av_dict_parse_string` |
| `OptionRanges.Ptr` | `av_dict_set` |
| `OptionRanges.Ptr` | `av_dict_set_int` |
| `OptionRanges.Ptr` | `av_opt_child_class_iterate` |
| `OptionRanges.Ptr` | `av_opt_child_next` |
| `OptObject.Copy` | `av_opt_copy` |
| `OptionRanges.Ptr` | `av_opt_eval_double` |
| `OptionRanges.Ptr` | `av_opt_eval_flags` |
| `OptionRanges.Ptr` | `av_opt_eval_float` |
| `OptionRanges.Ptr` | `av_opt_eval_int` |
| `OptionRanges.Ptr` | `av_opt_eval_int64` |
| `OptionRanges.Ptr` | `av_opt_eval_q` |
| `OptionRanges.Ptr` | `av_opt_eval_uint` |
| `OptObject.Find` | `av_opt_find` |
| `OptionRanges.Ptr` | `av_opt_find2` |
| `OptionRanges.Ptr` | `av_opt_flag_is_set` |
| `OptionRanges.Ptr` | `av_opt_free` |
| `OptionRanges.Ptr` | `av_opt_freep_ranges` |
| `OptionRanges.Ptr` | `av_opt_get` |
| `OptionRanges.Ptr` | `av_opt_get_array` |
| `OptionRanges.Ptr` | `av_opt_get_array_size` |
| `OptionRanges.Ptr` | `av_opt_get_chlayout` |
| `OptionRanges.Ptr` | `av_opt_get_dict_val` |
| `OptObject.GetDouble` | `av_opt_get_double` |
| `OptionRanges.Ptr` | `av_opt_get_image_size` |
| `OptObject.GetInt` | `av_opt_get_int` |
| `OptionRanges.Ptr` | `av_opt_get_key_value` |
| `OptionRanges.Ptr` | `av_opt_get_pixel_fmt` |
| `OptObject.GetQ` | `av_opt_get_q` |
| `OptionRanges.Ptr` | `av_opt_get_sample_fmt` |
| `OptionRanges.Ptr` | `av_opt_get_video_rate` |
| `OptionRanges.Ptr` | `av_opt_is_set_to_default` |
| `OptionRanges.Ptr` | `av_opt_is_set_to_default_by_name` |
| `OptionRanges.Ptr` | `av_opt_next` |
| `OptionRanges.Ptr` | `av_opt_ptr` |
| `OptionRanges.Ptr` | `av_opt_query_ranges` |
| `OptionRanges.Ptr` | `av_opt_query_ranges_default` |
| `OptionRanges.Ptr` | `av_opt_serialize` |
| `OptObject.Set` | `av_opt_set` |
| `OptionRanges.Ptr` | `av_opt_set_array` |
| `OptionRanges.Ptr` | `av_opt_set_bin` |
| `OptionRanges.Ptr` | `av_opt_set_chlayout` |
| `OptObject.SetDefaults` | `av_opt_set_defaults` |
| `OptionRanges.Ptr` | `av_opt_set_defaults2` |
| `OptionRanges.Ptr` | `av_opt_set_dict` |
| `OptionRanges.Ptr` | `av_opt_set_dict2` |
| `OptionRanges.Ptr` | `av_opt_set_dict_val` |
| `OptObject.SetDouble` | `av_opt_set_double` |
| `OptionRanges.Ptr` | `av_opt_set_from_string` |
| `OptionRanges.Ptr` | `av_opt_set_image_size` |
| `OptObject.SetInt` | `av_opt_set_int` |
| `OptionRanges.Ptr` | `av_opt_set_pixel_fmt` |
| `OptObject.SetQ` | `av_opt_set_q` |
| `OptionRanges.Ptr` | `av_opt_set_sample_fmt` |
| `OptionRanges.Ptr` | `av_opt_set_video_rate` |
| `OptionRanges.Ptr` | `av_opt_show2` |
</details>

### buffer_mem — 内存（52 个）

内存：申请、引用计数、缓冲池、队列、拼字符串。`NewBuffer` / `NewBufferPool` / `NewFifo` / `NewBPrint` 四个新建函数。

抱着的结构体：`Mem`、`Buffer`、`BufferPool`、`Fifo`、`BPrint`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewBuffer` | `av_buffer_alloc` |
| `BPrint.Ptr` | `av_buffer_allocz` |
| `BPrint.Ptr` | `av_buffer_create` |
| `BPrint.Ptr` | `av_buffer_default_free` |
| `BPrint.Ptr` | `av_buffer_get_opaque` |
| `BPrint.Ptr` | `av_buffer_get_ref_count` |
| `BPrint.Ptr` | `av_buffer_is_writable` |
| `BPrint.Ptr` | `av_buffer_make_writable` |
| `BPrint.Ptr` | `av_buffer_pool_buffer_get_opaque` |
| `BPrint.Ptr` | `av_buffer_pool_get` |
| `NewBufferPool` | `av_buffer_pool_init` |
| `BPrint.Ptr` | `av_buffer_pool_init2` |
| `BPrint.Ptr` | `av_buffer_pool_uninit` |
| `BPrint.Ptr` | `av_buffer_realloc` |
| `BPrint.Ptr` | `av_buffer_ref` |
| `BPrint.Ptr` | `av_buffer_replace` |
| `NewBPrint` | `av_malloc` |
| `BPrint.Ptr` | `av_malloc_array` |
| `BPrint.Ptr` | `av_mallocz` |
| `BPrint.Ptr` | `av_realloc` |
| `BPrint.Ptr` | `av_realloc_array` |
| `BPrint.Ptr` | `av_realloc_f` |
| `BPrint.Ptr` | `av_reallocp` |
| `BPrint.Ptr` | `av_reallocp_array` |
| `BPrint.Ptr` | `av_free` |
| `BPrint.Ptr` | `av_freep` |
| `BPrint.Ptr` | `av_memdup` |
| `BPrint.Ptr` | `av_memcpy_backptr` |
| `NewFifo` | `av_fifo_alloc2` |
| `BPrint.Ptr` | `av_fifo_auto_grow_limit` |
| `BPrint.Ptr` | `av_fifo_can_read` |
| `BPrint.Ptr` | `av_fifo_can_write` |
| `BPrint.Ptr` | `av_fifo_drain2` |
| `BPrint.Ptr` | `av_fifo_elem_size` |
| `BPrint.Ptr` | `av_fifo_freep2` |
| `BPrint.Ptr` | `av_fifo_grow2` |
| `BPrint.Ptr` | `av_fifo_peek` |
| `BPrint.Ptr` | `av_fifo_peek_to_cb` |
| `BPrint.Ptr` | `av_fifo_read` |
| `BPrint.Ptr` | `av_fifo_read_to_cb` |
| `BPrint.Ptr` | `av_fifo_reset2` |
| `BPrint.Ptr` | `av_fifo_write` |
| `BPrint.Ptr` | `av_fifo_write_from_cb` |
| `BPrint.Ptr` | `av_bprint_append_data` |
| `BPrint.Ptr` | `av_bprint_chars` |
| `BPrint.Ptr` | `av_bprint_clear` |
| `BPrint.Ptr` | `av_bprint_escape` |
| `BPrint.Ptr` | `av_bprint_finalize` |
| `BPrint.Ptr` | `av_bprint_get_buffer` |
| `NewBPrint` | `av_bprint_init` |
| `BPrint.Ptr` | `av_bprint_init_for_buffer` |
| `BPrint.Ptr` | `av_bprint_strftime` |
</details>

### error_log — 报错和杂务（29 个）

报错和杂务：错误码翻人话、日志开关、版本、时间戳换算、CPU 数。都是无状态的，直接调。

抱着的结构体：`Library`、`Log`、`Math`、`Clock`、`Cpu`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `StrError`（lib.go 内部直连） | `av_strerror` |
| `Log.Level` | `av_log_get_level` |
| `Log.SetLevel` | `av_log_set_level` |
| `Log.Flags` | `av_log_get_flags` |
| `Log.SetFlags` | `av_log_set_flags` |
| `Log.SetCallback` | `av_log_set_callback` |
| `Log.DefaultCallback` | `av_log_default_callback` |
| `Log.FormatLine` | `av_log_format_line` |
| `Log.FormatLine2` | `av_log_format_line2` |
| `Library.Version` | `av_version_info` |
| `Library.UtilVersion` | `avutil_version` |
| `Library.Configuration` | `avutil_configuration` |
| `Library.License` | `avutil_license` |
| `Math.Rescale` | `av_rescale` |
| `Math.RescaleRnd` | `av_rescale_rnd` |
| `Math.RescaleQ` | `av_rescale_q` |
| `Math.RescaleQRnd` | `av_rescale_q_rnd` |
| `Math.RescaleDelta` | `av_rescale_delta` |
| `Math.AddQ` | `av_add_q` |
| `Math.AddStable` | `av_add_stable` |
| `Math.CompareMod` | `av_compare_mod` |
| `Math.CompareTs` | `av_compare_ts` |
| `Clock.NowUs` | `av_gettime` |
| `Clock.NowRelativeUs` | `av_gettime_relative` |
| `Clock.IsMonotonic` | `av_gettime_relative_is_monotonic` |
| `Clock.SleepUs` | `av_usleep` |
| `Cpu.Count` | `av_cpu_count` |
| `Cpu.ForceCount` | `av_cpu_force_count` |
| `Cpu.MaxAlign` | `av_cpu_max_align` |
</details>

### format_demux — 拆盒（98 个）

拆盒：打开文件、找音视频流、读包、跳进度。先 `OpenInput` 拿到 `FormatContext`，后面全挂它身上。

抱着的结构体：`FormatContext`、`Stream`、`IOContext`、`Format`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Format.Ptr` | `av_add_index_entry` |
| `Format.Ptr` | `av_dump_format` |
| `Format.Ptr` | `av_find_best_stream` |
| `Format.Ptr` | `avformat_alloc_context` |
| `Format.Ptr` | `avformat_alloc_output_context2` |
| `Format.Ptr` | `avformat_close_input` |
| `Format.Ptr` | `avformat_configuration` |
| `Format.Ptr` | `avformat_find_stream_info` |
| `Format.Ptr` | `avformat_flush` |
| `Format.Ptr` | `avformat_free_context` |
| `Format.Ptr` | `avformat_get_class` |
| `Format.Ptr` | `avformat_get_mov_audio_tags` |
| `Format.Ptr` | `avformat_get_mov_video_tags` |
| `Format.Ptr` | `avformat_get_riff_audio_tags` |
| `Format.Ptr` | `avformat_get_riff_video_tags` |
| `Format.Ptr` | `avformat_index_get_entries_count` |
| `Format.Ptr` | `avformat_index_get_entry` |
| `Format.Ptr` | `avformat_index_get_entry_from_timestamp` |
| `Format.Ptr` | `avformat_init_output` |
| `Format.Ptr` | `avformat_license` |
| `Format.Ptr` | `avformat_match_stream_specifier` |
| `Format.Ptr` | `avformat_network_deinit` |
| `Format.Ptr` | `avformat_network_init` |
| `Format.Ptr` | `avformat_new_stream` |
| `Format.Ptr` | `avformat_open_input` |
| `Format.Ptr` | `avformat_query_codec` |
| `Format.Ptr` | `avformat_queue_attached_pictures` |
| `Format.Ptr` | `avformat_seek_file` |
| `Format.Ptr` | `avformat_stream_group_add_stream` |
| `Format.Ptr` | `avformat_stream_group_create` |
| `Format.Ptr` | `avformat_stream_group_name` |
| `Format.Ptr` | `avformat_version` |
| `Format.Ptr` | `avformat_write_header` |
| `Format.Ptr` | `av_guess_codec` |
| `Format.Ptr` | `av_guess_format` |
| `Format.Ptr` | `av_guess_frame_rate` |
| `Format.Ptr` | `av_guess_sample_aspect_ratio` |
| `Format.Ptr` | `av_index_search_timestamp` |
| `Format.Ptr` | `avio_accept` |
| `Format.Ptr` | `avio_alloc_context` |
| `Format.Ptr` | `avio_check` |
| `Format.Ptr` | `avio_close` |
| `Format.Ptr` | `avio_close_dir` |
| `Format.Ptr` | `avio_close_dyn_buf` |
| `Format.Ptr` | `avio_closep` |
| `Format.Ptr` | `avio_context_free` |
| `Format.Ptr` | `avio_enum_protocols` |
| `Format.Ptr` | `avio_feof` |
| `Format.Ptr` | `avio_find_protocol_name` |
| `Format.Ptr` | `avio_flush` |
| `Format.Ptr` | `avio_free_directory_entry` |
| `Format.Ptr` | `avio_get_dyn_buf` |
| `Format.Ptr` | `avio_get_str` |
| `Format.Ptr` | `avio_get_str16be` |
| `Format.Ptr` | `avio_get_str16le` |
| `Format.Ptr` | `avio_handshake` |
| `Format.Ptr` | `avio_open` |
| `Format.Ptr` | `avio_open2` |
| `Format.Ptr` | `avio_open_dir` |
| `Format.Ptr` | `avio_open_dyn_buf` |
| `Format.Ptr` | `avio_pause` |
| `Format.Ptr` | `avio_print_string_array` |
| `Format.Ptr` | `avio_protocol_get_class` |
| `Format.Ptr` | `avio_put_str` |
| `Format.Ptr` | `avio_put_str16be` |
| `Format.Ptr` | `avio_put_str16le` |
| `Format.Ptr` | `avio_r8` |
| `Format.Ptr` | `avio_rb16` |
| `Format.Ptr` | `avio_rb24` |
| `Format.Ptr` | `avio_rb32` |
| `Format.Ptr` | `avio_rb64` |
| `Format.Ptr` | `avio_read` |
| `Format.Ptr` | `avio_read_dir` |
| `Format.Ptr` | `avio_read_partial` |
| `Format.Ptr` | `avio_read_to_bprint` |
| `Format.Ptr` | `avio_rl16` |
| `Format.Ptr` | `avio_rl24` |
| `Format.Ptr` | `avio_rl32` |
| `Format.Ptr` | `avio_rl64` |
| `Format.Ptr` | `avio_seek` |
| `Format.Ptr` | `avio_seek_time` |
| `Format.Ptr` | `avio_size` |
| `Format.Ptr` | `avio_skip` |
| `Format.Ptr` | `avio_vprintf` |
| `Format.Ptr` | `avio_w8` |
| `Format.Ptr` | `avio_wb16` |
| `Format.Ptr` | `avio_wb24` |
| `Format.Ptr` | `avio_wb32` |
| `Format.Ptr` | `avio_wb64` |
| `Format.Ptr` | `avio_wl16` |
| `Format.Ptr` | `avio_wl24` |
| `Format.Ptr` | `avio_wl32` |
| `Format.Ptr` | `avio_wl64` |
| `Format.Ptr` | `avio_write` |
| `Format.Ptr` | `avio_write_marker` |
| `Format.Ptr` | `av_read_frame` |
| `Format.Ptr` | `av_seek_frame` |
| `Format.Ptr` | `av_url_split` |
</details>

### codec_encode — 编解码（76 个）

编解码：找解码器、开解码器、送包取帧、码流过滤。先 `FindDecoder` 拿到 `Codec`，再 `AllocContext` 开实例。

抱着的结构体：`CodecContext`、`Codec`、`CodecParameters`、`Parser`、`BitStreamFilter`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `BitStreamFilter.Ptr` | `avcodec_alloc_context3` |
| `BitStreamFilter.Ptr` | `avcodec_close` |
| `FindDecoder` | `avcodec_find_decoder` |
| `FindDecoderByName` | `avcodec_find_decoder_by_name` |
| `FindEncoder` | `avcodec_find_encoder` |
| `BitStreamFilter.Ptr` | `avcodec_find_encoder_by_name` |
| `BitStreamFilter.Ptr` | `avcodec_flush_buffers` |
| `BitStreamFilter.Ptr` | `avcodec_free_context` |
| `CodecName` | `avcodec_get_name` |
| `CodecType` | `avcodec_get_type` |
| `BitStreamFilter.Ptr` | `avcodec_is_open` |
| `BitStreamFilter.Ptr` | `avcodec_open2` |
| `BitStreamFilter.Ptr` | `avcodec_send_packet` |
| `BitStreamFilter.Ptr` | `avcodec_receive_frame` |
| `BitStreamFilter.Ptr` | `avcodec_send_frame` |
| `BitStreamFilter.Ptr` | `avcodec_receive_packet` |
| `NewCodecParameters` | `avcodec_parameters_alloc` |
| `BitStreamFilter.Ptr` | `avcodec_parameters_free` |
| `BitStreamFilter.Ptr` | `avcodec_parameters_copy` |
| `BitStreamFilter.Ptr` | `avcodec_parameters_from_context` |
| `BitStreamFilter.Ptr` | `avcodec_parameters_to_context` |
| `BitStreamFilter.Ptr` | `av_codec_is_decoder` |
| `BitStreamFilter.Ptr` | `av_codec_is_encoder` |
| `BitStreamFilter.Ptr` | `av_codec_iterate` |
| `NewParser` | `av_parser_init` |
| `BitStreamFilter.Ptr` | `av_parser_parse2` |
| `BitStreamFilter.Ptr` | `av_parser_close` |
| `NewBitStreamFilter` | `av_bsf_alloc` |
| `BitStreamFilter.Ptr` | `av_bsf_init` |
| `BitStreamFilter.Ptr` | `av_bsf_send_packet` |
| `BitStreamFilter.Ptr` | `av_bsf_receive_packet` |
| `BitStreamFilter.Ptr` | `av_bsf_flush` |
| `BitStreamFilter.Ptr` | `av_bsf_free` |
| `NewBitStreamFilter` | `av_bsf_get_by_name` |
| `BitStreamFilter.Ptr` | `av_bsf_list_alloc` |
| `BitStreamFilter.Ptr` | `av_bsf_list_append` |
| `BitStreamFilter.Ptr` | `av_bsf_list_append2` |
| `BitStreamFilter.Ptr` | `av_bsf_list_finalize` |
| `BitStreamFilter.Ptr` | `av_bsf_list_free` |
| `BitStreamFilter.Ptr` | `av_bsf_list_parse_str` |
| `BitStreamFilter.Ptr` | `av_bsf_get_class` |
| `BitStreamFilter.Ptr` | `av_bsf_get_null_filter` |
| `BitStreamFilter.Ptr` | `av_bsf_iterate` |
| `BitStreamFilter.Ptr` | `avcodec_align_dimensions` |
| `BitStreamFilter.Ptr` | `avcodec_align_dimensions2` |
| `BitStreamFilter.Ptr` | `avcodec_configuration` |
| `BitStreamFilter.Ptr` | `avcodec_dct_alloc` |
| `BitStreamFilter.Ptr` | `avcodec_dct_get_class` |
| `BitStreamFilter.Ptr` | `avcodec_dct_init` |
| `BitStreamFilter.Ptr` | `avcodec_decode_subtitle2` |
| `BitStreamFilter.Ptr` | `avcodec_default_execute` |
| `BitStreamFilter.Ptr` | `avcodec_default_execute2` |
| `BitStreamFilter.Ptr` | `avcodec_default_get_buffer2` |
| `BitStreamFilter.Ptr` | `avcodec_default_get_encode_buffer` |
| `BitStreamFilter.Ptr` | `avcodec_default_get_format` |
| `BitStreamFilter.Ptr` | `avcodec_descriptor_get` |
| `BitStreamFilter.Ptr` | `avcodec_descriptor_get_by_name` |
| `BitStreamFilter.Ptr` | `avcodec_descriptor_next` |
| `BitStreamFilter.Ptr` | `avcodec_encode_subtitle` |
| `BitStreamFilter.Ptr` | `avcodec_fill_audio_frame` |
| `BitStreamFilter.Ptr` | `avcodec_find_best_pix_fmt_of_list` |
| `BitStreamFilter.Ptr` | `avcodec_get_class` |
| `BitStreamFilter.Ptr` | `avcodec_get_hw_config` |
| `BitStreamFilter.Ptr` | `avcodec_get_hw_frames_parameters` |
| `BitStreamFilter.Ptr` | `av_codec_get_id` |
| `BitStreamFilter.Ptr` | `avcodec_get_subtitle_rect_class` |
| `BitStreamFilter.Ptr` | `avcodec_get_supported_config` |
| `BitStreamFilter.Ptr` | `av_codec_get_tag` |
| `BitStreamFilter.Ptr` | `av_codec_get_tag2` |
| `BitStreamFilter.Ptr` | `avcodec_license` |
| `BitStreamFilter.Ptr` | `avcodec_pix_fmt_to_codec_tag` |
| `BitStreamFilter.Ptr` | `avcodec_profile_name` |
| `BitStreamFilter.Ptr` | `avcodec_string` |
| `BitStreamFilter.Ptr` | `avcodec_version` |
| `BitStreamFilter.Ptr` | `av_parser_iterate` |
| `BitStreamFilter.Ptr` | `avsubtitle_free` |
</details>

### scale_color — 转色和缩放（37 个）

转色和缩放：YUV 转屏幕要的 RGBA，尺寸也能顺手改。`NewScaler` 建转色器，`PixFmtName` 查格式名。

抱着的结构体：`Scaler`、`Image`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewScaler` | `sws_getContext` |
| `Scaler.Ptr` | `sws_getCachedContext` |
| `Scaler.Ptr` | `sws_scale` |
| `Scaler.Ptr` | `sws_scale_frame` |
| `Scaler.Ptr` | `sws_freeContext` |
| `Scaler.Ptr` | `sws_alloc_context` |
| `Scaler.Ptr` | `sws_init_context` |
| `IsSupportedInput` | `sws_isSupportedInput` |
| `IsSupportedOutput` | `sws_isSupportedOutput` |
| `Scaler.Ptr` | `sws_isSupportedEndiannessConversion` |
| `Scaler.Ptr` | `av_image_get_buffer_size` |
| `Scaler.Ptr` | `av_image_alloc` |
| `Scaler.Ptr` | `av_image_copy_plane` |
| `Scaler.Ptr` | `av_image_check_size` |
| `Scaler.Ptr` | `av_image_check_size2` |
| `Scaler.Ptr` | `av_image_check_sar` |
| `Scaler.Ptr` | `av_pix_fmt_desc_get` |
| `PixFmtName` | `av_get_pix_fmt_name` |
| `PixFmtFromName` | `av_get_pix_fmt` |
| `Scaler.Ptr` | `sws_allocVec` |
| `Scaler.Ptr` | `sws_convertPalette8ToPacked24` |
| `Scaler.Ptr` | `sws_convertPalette8ToPacked32` |
| `Scaler.Ptr` | `sws_frame_end` |
| `Scaler.Ptr` | `sws_frame_start` |
| `Scaler.Ptr` | `sws_freeFilter` |
| `Scaler.Ptr` | `sws_freeVec` |
| `Scaler.Ptr` | `sws_get_class` |
| `Scaler.Ptr` | `sws_getCoefficients` |
| `Scaler.Ptr` | `sws_getColorspaceDetails` |
| `Scaler.Ptr` | `sws_getDefaultFilter` |
| `Scaler.Ptr` | `sws_getGaussianVec` |
| `Scaler.Ptr` | `sws_normalizeVec` |
| `Scaler.Ptr` | `sws_receive_slice` |
| `Scaler.Ptr` | `sws_receive_slice_alignment` |
| `Scaler.Ptr` | `sws_scaleVec` |
| `Scaler.Ptr` | `sws_send_slice` |
| `Scaler.Ptr` | `sws_setColorspaceDetails` |
</details>

### resample_audio — 音频重采样（37 个）

音频重采样：声道、采样率、采样格式不一样时掰成一样。先建 `Resampler`，转好的帧先进 `AudioFifo` 排队。

抱着的结构体：`Resampler`、`AudioFifo`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `AudioFifo.Ptr` | `av_audio_fifo_alloc` |
| `AudioFifo.Ptr` | `av_audio_fifo_drain` |
| `AudioFifo.Ptr` | `av_audio_fifo_free` |
| `AudioFifo.Ptr` | `av_audio_fifo_peek` |
| `AudioFifo.Ptr` | `av_audio_fifo_peek_at` |
| `AudioFifo.Ptr` | `av_audio_fifo_read` |
| `AudioFifo.Ptr` | `av_audio_fifo_realloc` |
| `AudioFifo.Ptr` | `av_audio_fifo_reset` |
| `AudioFifo.Ptr` | `av_audio_fifo_size` |
| `AudioFifo.Ptr` | `av_audio_fifo_space` |
| `AudioFifo.Ptr` | `av_audio_fifo_write` |
| `AudioFifo.Ptr` | `av_get_bytes_per_sample` |
| `AudioFifo.Ptr` | `av_get_packed_sample_fmt` |
| `AudioFifo.Ptr` | `av_get_planar_sample_fmt` |
| `AudioFifo.Ptr` | `av_get_sample_fmt` |
| `AudioFifo.Ptr` | `av_get_sample_fmt_name` |
| `AudioFifo.Ptr` | `av_get_sample_fmt_string` |
| `AudioFifo.Ptr` | `av_sample_fmt_is_planar` |
| `AudioFifo.Ptr` | `swr_alloc` |
| `AudioFifo.Ptr` | `swr_alloc_set_opts2` |
| `AudioFifo.Ptr` | `swr_build_matrix2` |
| `AudioFifo.Ptr` | `swr_close` |
| `AudioFifo.Ptr` | `swr_config_frame` |
| `AudioFifo.Ptr` | `swr_convert` |
| `AudioFifo.Ptr` | `swr_convert_frame` |
| `AudioFifo.Ptr` | `swr_drop_output` |
| `AudioFifo.Ptr` | `swr_free` |
| `AudioFifo.Ptr` | `swr_get_class` |
| `AudioFifo.Ptr` | `swr_get_delay` |
| `AudioFifo.Ptr` | `swr_get_out_samples` |
| `AudioFifo.Ptr` | `swr_init` |
| `AudioFifo.Ptr` | `swr_inject_silence` |
| `AudioFifo.Ptr` | `swr_is_initialized` |
| `AudioFifo.Ptr` | `swr_next_pts` |
| `AudioFifo.Ptr` | `swr_set_channel_mapping` |
| `AudioFifo.Ptr` | `swr_set_compensation` |
| `AudioFifo.Ptr` | `swr_set_matrix` |
</details>

### filter_graph — 滤镜图（64 个）

滤镜图：把多个滤镜连成链（缩放、裁剪、混音都归它）。`buffersrc` 进，`buffersink` 出，别接反。

抱着的结构体：`FilterGraph`、`FilterContext`、`Filter`、`FilterSink`、`FilterSource`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Filter.Ptr` | `av_buffersink_get_channels` |
| `Filter.Ptr` | `av_buffersink_get_ch_layout` |
| `Filter.Ptr` | `av_buffersink_get_color_range` |
| `Filter.Ptr` | `av_buffersink_get_colorspace` |
| `Filter.Ptr` | `av_buffersink_get_format` |
| `Filter.Ptr` | `av_buffersink_get_frame` |
| `Filter.Ptr` | `av_buffersink_get_frame_flags` |
| `Filter.Ptr` | `av_buffersink_get_frame_rate` |
| `Filter.Ptr` | `av_buffersink_get_h` |
| `Filter.Ptr` | `av_buffersink_get_hw_frames_ctx` |
| `Filter.Ptr` | `av_buffersink_get_sample_aspect_ratio` |
| `Filter.Ptr` | `av_buffersink_get_sample_rate` |
| `Filter.Ptr` | `av_buffersink_get_samples` |
| `Filter.Ptr` | `av_buffersink_get_time_base` |
| `Filter.Ptr` | `av_buffersink_get_type` |
| `Filter.Ptr` | `av_buffersink_get_w` |
| `Filter.Ptr` | `av_buffersink_set_frame_size` |
| `Filter.Ptr` | `av_buffersrc_add_frame` |
| `Filter.Ptr` | `av_buffersrc_add_frame_flags` |
| `Filter.Ptr` | `av_buffersrc_close` |
| `Filter.Ptr` | `av_buffersrc_get_nb_failed_requests` |
| `Filter.Ptr` | `av_buffersrc_get_status` |
| `Filter.Ptr` | `av_buffersrc_parameters_alloc` |
| `Filter.Ptr` | `av_buffersrc_parameters_set` |
| `Filter.Ptr` | `av_buffersrc_write_frame` |
| `Filter.Ptr` | `avfilter_config_links` |
| `Filter.Ptr` | `avfilter_configuration` |
| `Filter.Ptr` | `avfilter_filter_pad_count` |
| `Filter.Ptr` | `avfilter_free` |
| `Filter.Ptr` | `avfilter_get_by_name` |
| `Filter.Ptr` | `avfilter_get_class` |
| `Filter.Ptr` | `avfilter_graph_alloc` |
| `Filter.Ptr` | `avfilter_graph_alloc_filter` |
| `Filter.Ptr` | `avfilter_graph_config` |
| `Filter.Ptr` | `avfilter_graph_create_filter` |
| `Filter.Ptr` | `avfilter_graph_dump` |
| `Filter.Ptr` | `avfilter_graph_free` |
| `Filter.Ptr` | `avfilter_graph_get_filter` |
| `Filter.Ptr` | `avfilter_graph_parse` |
| `Filter.Ptr` | `avfilter_graph_parse2` |
| `Filter.Ptr` | `avfilter_graph_parse_ptr` |
| `Filter.Ptr` | `avfilter_graph_queue_command` |
| `Filter.Ptr` | `avfilter_graph_request_oldest` |
| `Filter.Ptr` | `avfilter_graph_segment_apply` |
| `Filter.Ptr` | `avfilter_graph_segment_apply_opts` |
| `Filter.Ptr` | `avfilter_graph_segment_create_filters` |
| `Filter.Ptr` | `avfilter_graph_segment_free` |
| `Filter.Ptr` | `avfilter_graph_segment_init` |
| `Filter.Ptr` | `avfilter_graph_segment_link` |
| `Filter.Ptr` | `avfilter_graph_segment_parse` |
| `Filter.Ptr` | `avfilter_graph_send_command` |
| `Filter.Ptr` | `avfilter_graph_set_auto_convert` |
| `Filter.Ptr` | `avfilter_init_dict` |
| `Filter.Ptr` | `avfilter_init_str` |
| `Filter.Ptr` | `avfilter_inout_alloc` |
| `Filter.Ptr` | `avfilter_inout_free` |
| `Filter.Ptr` | `avfilter_insert_filter` |
| `Filter.Ptr` | `avfilter_license` |
| `Filter.Ptr` | `avfilter_link` |
| `Filter.Ptr` | `avfilter_link_free` |
| `Filter.Ptr` | `avfilter_pad_get_name` |
| `Filter.Ptr` | `avfilter_pad_get_type` |
| `Filter.Ptr` | `avfilter_process_command` |
| `Filter.Ptr` | `avfilter_version` |
</details>

### device_io — 设备（10 个）

设备：列摄像头、麦克风这些输入输出设备，桌面录制、直播抓源用它找设备。

抱着的结构体：`DeviceList`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `DeviceList.Ptr` | `avdevice_version` |
| `DeviceList.Ptr` | `avdevice_configuration` |
| `DeviceList.Ptr` | `avdevice_license` |
| `DeviceList.Ptr` | `avdevice_register_all` |
| `DeviceList.Ptr` | `avdevice_list_devices` |
| `DeviceList.Ptr` | `avdevice_free_list_devices` |
| `DeviceList.Ptr` | `avdevice_list_input_sources` |
| `DeviceList.Ptr` | `avdevice_list_output_sinks` |
| `DeviceList.Ptr` | `avdevice_app_to_dev_control_message` |
| `DeviceList.Ptr` | `avdevice_dev_to_app_control_message` |
</details>

### media_desc — 查资料（88 个）

查资料：像素格式、声道布局、采样率这些只读信息，不干活只问，无状态直接调。

抱着的结构体：`MediaDesc`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `MediaDesc.Ptr` | `av_ambient_viewing_environment_alloc` |
| `MediaDesc.Ptr` | `av_ambient_viewing_environment_create_side_data` |
| `MediaDesc.Ptr` | `av_channel_description` |
| `MediaDesc.Ptr` | `av_channel_description_bprint` |
| `MediaDesc.Ptr` | `av_channel_from_string` |
| `MediaDesc.Ptr` | `av_channel_layout_ambisonic_order` |
| `MediaDesc.Ptr` | `av_channel_layout_channel_from_index` |
| `MediaDesc.Ptr` | `av_channel_layout_channel_from_string` |
| `MediaDesc.Ptr` | `av_channel_layout_check` |
| `MediaDesc.Ptr` | `av_channel_layout_compare` |
| `MediaDesc.Ptr` | `av_channel_layout_copy` |
| `MediaDesc.Ptr` | `av_channel_layout_custom_init` |
| `MediaDesc.Ptr` | `av_channel_layout_default` |
| `MediaDesc.Ptr` | `av_channel_layout_describe` |
| `MediaDesc.Ptr` | `av_channel_layout_describe_bprint` |
| `MediaDesc.Ptr` | `av_channel_layout_from_mask` |
| `MediaDesc.Ptr` | `av_channel_layout_from_string` |
| `MediaDesc.Ptr` | `av_channel_layout_index_from_channel` |
| `MediaDesc.Ptr` | `av_channel_layout_index_from_string` |
| `MediaDesc.Ptr` | `av_channel_layout_retype` |
| `MediaDesc.Ptr` | `av_channel_layout_standard` |
| `MediaDesc.Ptr` | `av_channel_layout_subset` |
| `MediaDesc.Ptr` | `av_channel_layout_uninit` |
| `MediaDesc.Ptr` | `av_channel_name` |
| `MediaDesc.Ptr` | `av_channel_name_bprint` |
| `MediaDesc.Ptr` | `av_color_primaries_from_name` |
| `MediaDesc.Ptr` | `av_color_primaries_name` |
| `MediaDesc.Ptr` | `av_color_range_from_name` |
| `MediaDesc.Ptr` | `av_color_range_name` |
| `MediaDesc.Ptr` | `av_color_space_from_name` |
| `MediaDesc.Ptr` | `av_color_space_name` |
| `MediaDesc.Ptr` | `av_color_transfer_from_name` |
| `MediaDesc.Ptr` | `av_color_transfer_name` |
| `MediaDesc.Ptr` | `av_content_light_metadata_alloc` |
| `MediaDesc.Ptr` | `av_content_light_metadata_create_side_data` |
| `MediaDesc.Ptr` | `av_display_matrix_flip` |
| `MediaDesc.Ptr` | `av_display_rotation_get` |
| `MediaDesc.Ptr` | `av_display_rotation_set` |
| `MediaDesc.Ptr` | `av_dovi_alloc` |
| `MediaDesc.Ptr` | `av_dovi_find_level` |
| `MediaDesc.Ptr` | `av_dovi_metadata_alloc` |
| `MediaDesc.Ptr` | `av_dynamic_hdr_plus_alloc` |
| `MediaDesc.Ptr` | `av_dynamic_hdr_plus_create_side_data` |
| `MediaDesc.Ptr` | `av_dynamic_hdr_plus_from_t35` |
| `MediaDesc.Ptr` | `av_dynamic_hdr_plus_to_t35` |
| `MediaDesc.Ptr` | `av_film_grain_params_alloc` |
| `MediaDesc.Ptr` | `av_film_grain_params_create_side_data` |
| `MediaDesc.Ptr` | `av_film_grain_params_select` |
| `MediaDesc.Ptr` | `av_get_bits_per_pixel` |
| `MediaDesc.Ptr` | `av_get_pix_fmt` |
| `MediaDesc.Ptr` | `av_get_pix_fmt_loss` |
| `MediaDesc.Ptr` | `av_get_pix_fmt_name` |
| `MediaDesc.Ptr` | `av_get_pix_fmt_string` |
| `MediaDesc.Ptr` | `av_mastering_display_metadata_alloc` |
| `MediaDesc.Ptr` | `av_mastering_display_metadata_alloc_size` |
| `MediaDesc.Ptr` | `av_mastering_display_metadata_create_side_data` |
| `MediaDesc.Ptr` | `av_parse_color` |
| `MediaDesc.Ptr` | `av_parse_ratio` |
| `MediaDesc.Ptr` | `av_parse_time` |
| `MediaDesc.Ptr` | `av_parse_video_rate` |
| `MediaDesc.Ptr` | `av_parse_video_size` |
| `MediaDesc.Ptr` | `av_pix_fmt_desc_get` |
| `MediaDesc.Ptr` | `av_pix_fmt_desc_get_id` |
| `MediaDesc.Ptr` | `av_pix_fmt_desc_next` |
| `MediaDesc.Ptr` | `av_spherical_alloc` |
| `MediaDesc.Ptr` | `av_spherical_from_name` |
| `MediaDesc.Ptr` | `av_spherical_projection_name` |
| `MediaDesc.Ptr` | `av_spherical_tile_bounds` |
| `MediaDesc.Ptr` | `av_stereo3d_alloc` |
| `MediaDesc.Ptr` | `av_stereo3d_alloc_size` |
| `MediaDesc.Ptr` | `av_stereo3d_create_side_data` |
| `MediaDesc.Ptr` | `av_stereo3d_from_name` |
| `MediaDesc.Ptr` | `av_stereo3d_primary_eye_from_name` |
| `MediaDesc.Ptr` | `av_stereo3d_primary_eye_name` |
| `MediaDesc.Ptr` | `av_stereo3d_type_name` |
| `MediaDesc.Ptr` | `av_stereo3d_view_from_name` |
| `MediaDesc.Ptr` | `av_stereo3d_view_name` |
| `MediaDesc.Ptr` | `av_timecode_adjust_ntsc_framenum2` |
| `MediaDesc.Ptr` | `av_timecode_check_frame_rate` |
| `MediaDesc.Ptr` | `av_timecode_get_smpte` |
| `MediaDesc.Ptr` | `av_timecode_get_smpte_from_framenum` |
| `MediaDesc.Ptr` | `av_timecode_init` |
| `MediaDesc.Ptr` | `av_timecode_init_from_components` |
| `MediaDesc.Ptr` | `av_timecode_init_from_string` |
| `MediaDesc.Ptr` | `av_timecode_make_mpeg_tc_string` |
| `MediaDesc.Ptr` | `av_timecode_make_smpte_tc_string` |
| `MediaDesc.Ptr` | `av_timecode_make_smpte_tc_string2` |
| `MediaDesc.Ptr` | `av_timecode_make_string` |
</details>

### crypto_hash_misc — 杂项工具箱（416 个）

杂项工具箱：硬解设备、哈希校验、写文件、猜格式、零碎小计算。按需直调，用哪个拿哪个。

抱着的结构体：`HWDevice`、`Crypto`、`Muxer`、`Prober`、`Samples`、`Util`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Crypto.Ptr` | `av_ac3_parse_header` |
| `Crypto.Ptr` | `av_adts_header_parse` |
| `Crypto.Ptr` | `av_adler32_update` |
| `Crypto.Ptr` | `av_aes_alloc` |
| `Crypto.Ptr` | `av_aes_crypt` |
| `Crypto.Ptr` | `av_aes_ctr_alloc` |
| `Crypto.Ptr` | `av_aes_ctr_crypt` |
| `Crypto.Ptr` | `av_aes_ctr_free` |
| `Crypto.Ptr` | `av_aes_ctr_get_iv` |
| `Crypto.Ptr` | `av_aes_ctr_increment_iv` |
| `Crypto.Ptr` | `av_aes_ctr_init` |
| `Crypto.Ptr` | `av_aes_ctr_set_full_iv` |
| `Crypto.Ptr` | `av_aes_ctr_set_iv` |
| `Crypto.Ptr` | `av_aes_ctr_set_random_iv` |
| `Crypto.Ptr` | `av_aes_init` |
| `Crypto.Ptr` | `av_alloc_vdpaucontext` |
| `Crypto.Ptr` | `av_append_packet` |
| `Crypto.Ptr` | `av_append_path_component` |
| `Crypto.Ptr` | `av_assert0_fpu` |
| `Crypto.Ptr` | `av_base64_decode` |
| `Crypto.Ptr` | `av_base64_encode` |
| `Crypto.Ptr` | `av_basename` |
| `Crypto.Ptr` | `av_bessel_i0` |
| `Crypto.Ptr` | `av_blowfish_alloc` |
| `Crypto.Ptr` | `av_blowfish_crypt` |
| `Crypto.Ptr` | `av_blowfish_crypt_ecb` |
| `Crypto.Ptr` | `av_blowfish_init` |
| `Crypto.Ptr` | `av_bmg_get` |
| `Crypto.Ptr` | `av_bprintf` |
| `Crypto.Ptr` | `av_calloc` |
| `Crypto.Ptr` | `av_camellia_alloc` |
| `Crypto.Ptr` | `av_camellia_crypt` |
| `Crypto.Ptr` | `av_camellia_init` |
| `Crypto.Ptr` | `av_cast5_alloc` |
| `Crypto.Ptr` | `av_cast5_crypt` |
| `Crypto.Ptr` | `av_cast5_crypt2` |
| `Crypto.Ptr` | `av_cast5_init` |
| `Crypto.Ptr` | `av_chroma_location_enum_to_pos` |
| `Crypto.Ptr` | `av_chroma_location_from_name` |
| `Crypto.Ptr` | `av_chroma_location_name` |
| `Crypto.Ptr` | `av_chroma_location_pos_to_enum` |
| `Crypto.Ptr` | `av_cmp_i` |
| `Crypto.Ptr` | `av_cpb_properties_alloc` |
| `Crypto.Ptr` | `av_crc_get_table` |
| `Crypto.Ptr` | `av_crc` |
| `Crypto.Ptr` | `av_crc_init` |
| `Crypto.Ptr` | `av_csp_approximate_trc_gamma` |
| `Crypto.Ptr` | `av_csp_luma_coeffs_from_avcsp` |
| `Crypto.Ptr` | `av_csp_primaries_desc_from_id` |
| `Crypto.Ptr` | `av_csp_primaries_id_from_desc` |
| `Crypto.Ptr` | `av_csp_trc_func_from_id` |
| `Crypto.Ptr` | `av_d2q` |
| `Crypto.Ptr` | `av_d3d11va_alloc_context` |
| `Crypto.Ptr` | `av_dct_calc` |
| `Crypto.Ptr` | `av_dct_end` |
| `Crypto.Ptr` | `av_dct_init` |
| `Crypto.Ptr` | `av_default_get_category` |
| `Crypto.Ptr` | `av_default_item_name` |
| `Crypto.Ptr` | `av_demuxer_iterate` |
| `Crypto.Ptr` | `av_des_alloc` |
| `Crypto.Ptr` | `av_des_crypt` |
| `Crypto.Ptr` | `av_des_init` |
| `Crypto.Ptr` | `av_des_mac` |
| `Crypto.Ptr` | `av_detection_bbox_alloc` |
| `Crypto.Ptr` | `av_detection_bbox_create_side_data` |
| `Crypto.Ptr` | `av_dirac_parse_sequence_header` |
| `Crypto.Ptr` | `av_dirname` |
| `Crypto.Ptr` | `av_disposition_from_string` |
| `Crypto.Ptr` | `av_disposition_to_string` |
| `Crypto.Ptr` | `av_div_i` |
| `Crypto.Ptr` | `av_div_q` |
| `Crypto.Ptr` | `av_downmix_info_update_side_data` |
| `Crypto.Ptr` | `av_dv_codec_profile` |
| `Crypto.Ptr` | `av_dv_codec_profile2` |
| `Crypto.Ptr` | `av_dv_frame_profile` |
| `Crypto.Ptr` | `av_dynarray2_add` |
| `Crypto.Ptr` | `av_dynarray_add` |
| `Crypto.Ptr` | `av_dynarray_add_nofree` |
| `Crypto.Ptr` | `av_encryption_info_add_side_data` |
| `Crypto.Ptr` | `av_encryption_info_alloc` |
| `Crypto.Ptr` | `av_encryption_info_clone` |
| `Crypto.Ptr` | `av_encryption_info_free` |
| `Crypto.Ptr` | `av_encryption_info_get_side_data` |
| `Crypto.Ptr` | `av_encryption_init_info_add_side_data` |
| `Crypto.Ptr` | `av_encryption_init_info_alloc` |
| `Crypto.Ptr` | `av_encryption_init_info_free` |
| `Crypto.Ptr` | `av_encryption_init_info_get_side_data` |
| `Crypto.Ptr` | `av_escape` |
| `Crypto.Ptr` | `av_executor_alloc` |
| `Crypto.Ptr` | `av_executor_execute` |
| `Crypto.Ptr` | `av_executor_free` |
| `Crypto.Ptr` | `av_expr_count_func` |
| `Crypto.Ptr` | `av_expr_count_vars` |
| `Crypto.Ptr` | `av_expr_eval` |
| `Crypto.Ptr` | `av_expr_free` |
| `Crypto.Ptr` | `av_expr_parse` |
| `Crypto.Ptr` | `av_expr_parse_and_eval` |
| `Crypto.Ptr` | `av_fast_malloc` |
| `Crypto.Ptr` | `av_fast_mallocz` |
| `Crypto.Ptr` | `av_fast_padded_malloc` |
| `Crypto.Ptr` | `av_fast_padded_mallocz` |
| `Crypto.Ptr` | `av_fast_realloc` |
| `Crypto.Ptr` | `av_fft_calc` |
| `Crypto.Ptr` | `av_fft_end` |
| `Crypto.Ptr` | `av_fft_init` |
| `Crypto.Ptr` | `av_fft_permute` |
| `Crypto.Ptr` | `av_file_map` |
| `Crypto.Ptr` | `av_filename_number_test` |
| `Crypto.Ptr` | `av_file_unmap` |
| `Crypto.Ptr` | `av_filter_iterate` |
| `Crypto.Ptr` | `av_find_best_pix_fmt_of_2` |
| `Crypto.Ptr` | `av_find_default_stream_index` |
| `Crypto.Ptr` | `av_find_info_tag` |
| `Crypto.Ptr` | `av_find_input_format` |
| `Crypto.Ptr` | `av_find_nearest_q_idx` |
| `Crypto.Ptr` | `av_find_program_from_stream` |
| `Crypto.Ptr` | `av_fmt_ctx_get_duration_estimation_method` |
| `Crypto.Ptr` | `av_force_cpu_flags` |
| `Crypto.Ptr` | `av_format_inject_global_side_data` |
| `Crypto.Ptr` | `av_fourcc_make_string` |
| `Crypto.Ptr` | `av_gcd` |
| `Crypto.Ptr` | `av_gcd_q` |
| `Crypto.Ptr` | `av_get_alt_sample_fmt` |
| `Crypto.Ptr` | `av_get_audio_frame_duration` |
| `Crypto.Ptr` | `av_get_audio_frame_duration2` |
| `Crypto.Ptr` | `av_get_bits_per_sample` |
| `Crypto.Ptr` | `av_get_cpu_flags` |
| `Crypto.Ptr` | `av_get_exact_bits_per_sample` |
| `Crypto.Ptr` | `av_get_frame_filename` |
| `Crypto.Ptr` | `av_get_frame_filename2` |
| `Crypto.Ptr` | `av_get_known_color_name` |
| `Crypto.Ptr` | `av_get_media_type_string` |
| `Crypto.Ptr` | `av_get_output_timestamp` |
| `Crypto.Ptr` | `av_get_packet` |
| `Crypto.Ptr` | `av_get_padded_bits_per_pixel` |
| `Crypto.Ptr` | `av_get_pcm_codec` |
| `Crypto.Ptr` | `av_get_picture_type_char` |
| `Crypto.Ptr` | `av_get_profile_name` |
| `Crypto.Ptr` | `av_get_random_seed` |
| `Crypto.Ptr` | `av_get_time_base_q` |
| `Crypto.Ptr` | `av_get_token` |
| `Crypto.Ptr` | `av_hash_alloc` |
| `Crypto.Ptr` | `av_hash_final` |
| `Crypto.Ptr` | `av_hash_final_b64` |
| `Crypto.Ptr` | `av_hash_final_bin` |
| `Crypto.Ptr` | `av_hash_final_hex` |
| `Crypto.Ptr` | `av_hash_freep` |
| `Crypto.Ptr` | `av_hash_get_name` |
| `Crypto.Ptr` | `av_hash_get_size` |
| `Crypto.Ptr` | `av_hash_init` |
| `Crypto.Ptr` | `av_hash_names` |
| `Crypto.Ptr` | `av_hash_update` |
| `Crypto.Ptr` | `av_hex_dump` |
| `Crypto.Ptr` | `av_hex_dump_log` |
| `Crypto.Ptr` | `av_hmac_alloc` |
| `Crypto.Ptr` | `av_hmac_calc` |
| `Crypto.Ptr` | `av_hmac_final` |
| `Crypto.Ptr` | `av_hmac_free` |
| `Crypto.Ptr` | `av_hmac_init` |
| `Crypto.Ptr` | `av_hmac_update` |
| `Crypto.Ptr` | `av_hwdevice_ctx_alloc` |
| `Crypto.Ptr` | `av_hwdevice_ctx_create` |
| `Crypto.Ptr` | `av_hwdevice_ctx_create_derived` |
| `Crypto.Ptr` | `av_hwdevice_ctx_create_derived_opts` |
| `Crypto.Ptr` | `av_hwdevice_ctx_init` |
| `Crypto.Ptr` | `av_hwdevice_find_type_by_name` |
| `Crypto.Ptr` | `av_hwdevice_get_hwframe_constraints` |
| `Crypto.Ptr` | `av_hwdevice_get_type_name` |
| `Crypto.Ptr` | `av_hwdevice_hwconfig_alloc` |
| `Crypto.Ptr` | `av_hwdevice_iterate_types` |
| `Crypto.Ptr` | `av_hwframe_constraints_free` |
| `Crypto.Ptr` | `av_hwframe_ctx_alloc` |
| `Crypto.Ptr` | `av_hwframe_ctx_create_derived` |
| `Crypto.Ptr` | `av_hwframe_ctx_init` |
| `Crypto.Ptr` | `av_hwframe_get_buffer` |
| `Crypto.Ptr` | `av_hwframe_map` |
| `Crypto.Ptr` | `av_hwframe_transfer_data` |
| `Crypto.Ptr` | `av_hwframe_transfer_get_formats` |
| `Crypto.Ptr` | `av_i2int` |
| `Crypto.Ptr` | `av_iamf_audio_element_add_layer` |
| `Crypto.Ptr` | `av_iamf_audio_element_alloc` |
| `Crypto.Ptr` | `av_iamf_audio_element_free` |
| `Crypto.Ptr` | `av_iamf_audio_element_get_class` |
| `Crypto.Ptr` | `av_iamf_mix_presentation_add_submix` |
| `Crypto.Ptr` | `av_iamf_mix_presentation_alloc` |
| `Crypto.Ptr` | `av_iamf_mix_presentation_free` |
| `Crypto.Ptr` | `av_iamf_mix_presentation_get_class` |
| `Crypto.Ptr` | `av_iamf_param_definition_alloc` |
| `Crypto.Ptr` | `av_iamf_param_definition_get_class` |
| `Crypto.Ptr` | `av_iamf_submix_add_element` |
| `Crypto.Ptr` | `av_iamf_submix_add_layout` |
| `Crypto.Ptr` | `av_imdct_calc` |
| `Crypto.Ptr` | `av_imdct_half` |
| `Crypto.Ptr` | `av_init_packet` |
| `Crypto.Ptr` | `av_input_audio_device_next` |
| `Crypto.Ptr` | `av_input_video_device_next` |
| `Crypto.Ptr` | `av_int2i` |
| `Crypto.Ptr` | `av_int_list_length_for_size` |
| `Crypto.Ptr` | `av_interleaved_write_frame` |
| `Crypto.Ptr` | `av_interleaved_write_uncoded_frame` |
| `Crypto.Ptr` | `av_jni_get_java_vm` |
| `Crypto.Ptr` | `av_jni_set_java_vm` |
| `Crypto.Ptr` | `av_lfg_init` |
| `Crypto.Ptr` | `av_lfg_init_from_data` |
| `Crypto.Ptr` | `av_log` |
| `Crypto.Ptr` | `av_lzo1x_decode` |
| `Crypto.Ptr` | `av_match_ext` |
| `Crypto.Ptr` | `av_match_list` |
| `Crypto.Ptr` | `av_match_name` |
| `Crypto.Ptr` | `av_max_alloc` |
| `Crypto.Ptr` | `av_md5_alloc` |
| `Crypto.Ptr` | `av_md5_final` |
| `Crypto.Ptr` | `av_md5_init` |
| `Crypto.Ptr` | `av_md5_sum` |
| `Crypto.Ptr` | `av_md5_update` |
| `Crypto.Ptr` | `av_mdct_calc` |
| `Crypto.Ptr` | `av_mdct_end` |
| `Crypto.Ptr` | `av_mdct_init` |
| `Crypto.Ptr` | `av_mediacodec_alloc_context` |
| `Crypto.Ptr` | `av_mediacodec_default_free` |
| `Crypto.Ptr` | `av_mediacodec_default_init` |
| `Crypto.Ptr` | `av_mediacodec_release_buffer` |
| `Crypto.Ptr` | `av_mediacodec_render_buffer_at_time` |
| `Crypto.Ptr` | `av_mod_i` |
| `Crypto.Ptr` | `av_mul_i` |
| `Crypto.Ptr` | `av_mul_q` |
| `Crypto.Ptr` | `av_murmur3_alloc` |
| `Crypto.Ptr` | `av_murmur3_final` |
| `Crypto.Ptr` | `av_murmur3_init` |
| `Crypto.Ptr` | `av_murmur3_init_seeded` |
| `Crypto.Ptr` | `av_murmur3_update` |
| `Crypto.Ptr` | `av_muxer_iterate` |
| `Crypto.Ptr` | `av_nearer_q` |
| `Crypto.Ptr` | `av_new_program` |
| `Crypto.Ptr` | `av_output_audio_device_next` |
| `Crypto.Ptr` | `av_output_video_device_next` |
| `Crypto.Ptr` | `av_pixelutils_get_sad_fn` |
| `Crypto.Ptr` | `av_pkt_dump2` |
| `Crypto.Ptr` | `av_pkt_dump_log2` |
| `Crypto.Ptr` | `av_probe_input_buffer` |
| `Crypto.Ptr` | `av_probe_input_buffer2` |
| `Crypto.Ptr` | `av_probe_input_format` |
| `Crypto.Ptr` | `av_probe_input_format2` |
| `Crypto.Ptr` | `av_probe_input_format3` |
| `Crypto.Ptr` | `av_program_add_stream_index` |
| `Crypto.Ptr` | `av_q2intfloat` |
| `Crypto.Ptr` | `av_qsv_alloc_context` |
| `Crypto.Ptr` | `av_random_bytes` |
| `Crypto.Ptr` | `av_rc4_alloc` |
| `Crypto.Ptr` | `av_rc4_crypt` |
| `Crypto.Ptr` | `av_rc4_init` |
| `Crypto.Ptr` | `av_rdft_calc` |
| `Crypto.Ptr` | `av_rdft_end` |
| `Crypto.Ptr` | `av_rdft_init` |
| `Crypto.Ptr` | `av_read_image_line` |
| `Crypto.Ptr` | `av_read_image_line2` |
| `Crypto.Ptr` | `av_read_pause` |
| `Crypto.Ptr` | `av_read_play` |
| `Crypto.Ptr` | `av_reduce` |
| `Crypto.Ptr` | `av_ripemd_alloc` |
| `Crypto.Ptr` | `av_ripemd_final` |
| `Crypto.Ptr` | `av_ripemd_init` |
| `Crypto.Ptr` | `av_ripemd_update` |
| `Crypto.Ptr` | `av_samples_alloc` |
| `Crypto.Ptr` | `av_samples_alloc_array_and_samples` |
| `Crypto.Ptr` | `av_samples_copy` |
| `Crypto.Ptr` | `av_samples_fill_arrays` |
| `Crypto.Ptr` | `av_samples_get_buffer_size` |
| `Crypto.Ptr` | `av_samples_set_silence` |
| `Crypto.Ptr` | `av_sdp_create` |
| `Crypto.Ptr` | `av_set_options_string` |
| `Crypto.Ptr` | `av_sha512_alloc` |
| `Crypto.Ptr` | `av_sha512_final` |
| `Crypto.Ptr` | `av_sha512_init` |
| `Crypto.Ptr` | `av_sha512_update` |
| `Crypto.Ptr` | `av_sha_alloc` |
| `Crypto.Ptr` | `av_sha_final` |
| `Crypto.Ptr` | `av_sha_init` |
| `Crypto.Ptr` | `av_sha_update` |
| `Crypto.Ptr` | `av_shr_i` |
| `Crypto.Ptr` | `av_size_mult` |
| `Crypto.Ptr` | `av_small_strptime` |
| `Crypto.Ptr` | `av_sscanf` |
| `Crypto.Ptr` | `av_strcasecmp` |
| `Crypto.Ptr` | `av_strdup` |
| `Crypto.Ptr` | `av_strndup` |
| `Crypto.Ptr` | `av_stream_add_side_data` |
| `Crypto.Ptr` | `av_stream_get_class` |
| `Crypto.Ptr` | `av_stream_get_codec_timebase` |
| `Crypto.Ptr` | `av_stream_get_parser` |
| `Crypto.Ptr` | `av_stream_get_side_data` |
| `Crypto.Ptr` | `av_stream_group_get_class` |
| `Crypto.Ptr` | `av_stream_new_side_data` |
| `Crypto.Ptr` | `av_strireplace` |
| `Crypto.Ptr` | `av_stristart` |
| `Crypto.Ptr` | `av_stristr` |
| `Crypto.Ptr` | `av_strlcat` |
| `Crypto.Ptr` | `av_strlcpy` |
| `Crypto.Ptr` | `av_strncasecmp` |
| `Crypto.Ptr` | `av_strnstr` |
| `Crypto.Ptr` | `av_strstart` |
| `Crypto.Ptr` | `av_strtod` |
| `Crypto.Ptr` | `av_strtok` |
| `Crypto.Ptr` | `av_sub_i` |
| `Crypto.Ptr` | `av_sub_q` |
| `Crypto.Ptr` | `av_tea_alloc` |
| `Crypto.Ptr` | `av_tea_crypt` |
| `Crypto.Ptr` | `av_tea_init` |
| `Crypto.Ptr` | `av_thread_message_flush` |
| `Crypto.Ptr` | `av_thread_message_queue_alloc` |
| `Crypto.Ptr` | `av_thread_message_queue_free` |
| `Crypto.Ptr` | `av_thread_message_queue_nb_elems` |
| `Crypto.Ptr` | `av_thread_message_queue_recv` |
| `Crypto.Ptr` | `av_thread_message_queue_send` |
| `Crypto.Ptr` | `av_thread_message_queue_set_err_recv` |
| `Crypto.Ptr` | `av_thread_message_queue_set_err_send` |
| `Crypto.Ptr` | `av_thread_message_queue_set_free_func` |
| `Crypto.Ptr` | `av_timegm` |
| `Crypto.Ptr` | `av_tree_destroy` |
| `Crypto.Ptr` | `av_tree_enumerate` |
| `Crypto.Ptr` | `av_tree_find` |
| `Crypto.Ptr` | `av_tree_insert` |
| `Crypto.Ptr` | `av_tree_node_alloc` |
| `Crypto.Ptr` | `av_ts_make_time_string2` |
| `Crypto.Ptr` | `av_twofish_alloc` |
| `Crypto.Ptr` | `av_twofish_crypt` |
| `Crypto.Ptr` | `av_twofish_init` |
| `Crypto.Ptr` | `av_tx_init` |
| `Crypto.Ptr` | `av_tx_uninit` |
| `Crypto.Ptr` | `av_utf8_decode` |
| `Crypto.Ptr` | `av_uuid_parse` |
| `Crypto.Ptr` | `av_uuid_parse_range` |
| `Crypto.Ptr` | `av_uuid_unparse` |
| `Crypto.Ptr` | `av_uuid_urn_parse` |
| `Crypto.Ptr` | `av_vbprintf` |
| `Crypto.Ptr` | `av_vdpau_alloc_context` |
| `Crypto.Ptr` | `av_vdpau_bind_context` |
| `Crypto.Ptr` | `av_vdpau_get_surface_parameters` |
| `Crypto.Ptr` | `av_vdpau_hwaccel_get_render2` |
| `Crypto.Ptr` | `av_vdpau_hwaccel_set_render2` |
| `Crypto.Ptr` | `av_video_enc_params_alloc` |
| `Crypto.Ptr` | `av_video_enc_params_create_side_data` |
| `Crypto.Ptr` | `av_video_hint_alloc` |
| `Crypto.Ptr` | `av_video_hint_create_side_data` |
| `Crypto.Ptr` | `av_vkfmt_from_pixfmt` |
| `Crypto.Ptr` | `av_vk_frame_alloc` |
| `Crypto.Ptr` | `av_vlog` |
| `Crypto.Ptr` | `av_vorbis_parse_frame` |
| `Crypto.Ptr` | `av_vorbis_parse_frame_flags` |
| `Crypto.Ptr` | `av_vorbis_parse_free` |
| `Crypto.Ptr` | `av_vorbis_parse_init` |
| `Crypto.Ptr` | `av_vorbis_parse_reset` |
| `Crypto.Ptr` | `av_write_frame` |
| `Crypto.Ptr` | `av_write_image_line` |
| `Crypto.Ptr` | `av_write_image_line2` |
| `Crypto.Ptr` | `av_write_trailer` |
| `Crypto.Ptr` | `av_write_uncoded_frame` |
| `Crypto.Ptr` | `av_write_uncoded_frame_query` |
| `Crypto.Ptr` | `av_xiphlacing` |
| `Crypto.Ptr` | `av_xtea_alloc` |
| `Crypto.Ptr` | `av_xtea_crypt` |
| `Crypto.Ptr` | `av_xtea_init` |
| `Crypto.Ptr` | `av_xtea_le_crypt` |
| `Crypto.Ptr` | `av_xtea_le_init` |
| `Crypto.Ptr` | `swresample_configuration` |
| `Crypto.Ptr` | `swresample_license` |
| `Crypto.Ptr` | `swresample_version` |
| `Crypto.Ptr` | `swri_audio_convert` |
| `Crypto.Ptr` | `swri_audio_convert_alloc` |
| `Crypto.Ptr` | `swri_audio_convert_free` |
| `Crypto.Ptr` | `swri_resample_dsp_init` |
| `Crypto.Ptr` | `swri_resample_dsp_x86_init` |
| `Crypto.Ptr` | `swscale_configuration` |
| `Crypto.Ptr` | `swscale_license` |
| `Crypto.Ptr` | `swscale_version` |
| `Crypto.Ptr` | `av_add_i` |
| `Crypto.Ptr` | `av_dynamic_hdr_vivid_alloc` |
| `Crypto.Ptr` | `av_dynamic_hdr_vivid_create_side_data` |
| `Crypto.Ptr` | `avformat_transfer_internal_stream_timing_info` |
| `Crypto.Ptr` | `av_image_copy` |
| `Crypto.Ptr` | `av_image_copy_plane_uc_from` |
| `Crypto.Ptr` | `av_image_copy_to_buffer` |
| `Crypto.Ptr` | `av_image_copy_uc_from` |
| `Crypto.Ptr` | `av_image_fill_arrays` |
| `Crypto.Ptr` | `av_image_fill_black` |
| `Crypto.Ptr` | `av_image_fill_color` |
| `Crypto.Ptr` | `av_image_fill_linesizes` |
| `Crypto.Ptr` | `av_image_fill_max_pixsteps` |
| `Crypto.Ptr` | `av_image_fill_plane_sizes` |
| `Crypto.Ptr` | `av_image_fill_pointers` |
| `Crypto.Ptr` | `av_image_get_linesize` |
| `Crypto.Ptr` | `av_log2` |
| `Crypto.Ptr` | `av_log2_16bit` |
| `Crypto.Ptr` | `av_log2_i` |
| `Crypto.Ptr` | `av_parse_cpu_caps` |
| `Crypto.Ptr` | `av_pix_fmt_count_planes` |
| `Crypto.Ptr` | `av_pix_fmt_get_chroma_sub_sample` |
| `Crypto.Ptr` | `av_pix_fmt_swap_endianness` |
| `Util.SwsAllocVec` | `sws_allocVec` |
| `Util.SwsConvertPalette8ToPacked24` | `sws_convertPalette8ToPacked24` |
| `Util.SwsConvertPalette8ToPacked32` | `sws_convertPalette8ToPacked32` |
| `Util.SwsFrameEnd` | `sws_frame_end` |
| `Util.SwsFrameStart` | `sws_frame_start` |
| `Util.SwsFreeFilter` | `sws_freeFilter` |
| `Util.SwsFreeVec` | `sws_freeVec` |
| `Util.SwsGetClass` | `sws_get_class` |
| `Util.SwsGetCoefficients` | `sws_getCoefficients` |
| `Util.SwsGetColorspaceDetails` | `sws_getColorspaceDetails` |
| `Util.SwsGetDefaultFilter` | `sws_getDefaultFilter` |
| `Util.SwsGetGaussianVec` | `sws_getGaussianVec` |
| `Util.SwsNormalizeVec` | `sws_normalizeVec` |
| `Util.SwsReceiveSlice` | `sws_receive_slice` |
| `Util.SwsReceiveSliceAlignment` | `sws_receive_slice_alignment` |
| `Util.SwsScaleVec` | `sws_scaleVec` |
| `Util.SwsSendSlice` | `sws_send_slice` |
| `Util.SwsSetColorspaceDetails` | `sws_setColorspaceDetails` |
</details>
