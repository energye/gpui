# video/ffmpeg — ffmpeg 的 Go 遥控器

大白话：这个包让你在 Go 里直接使唤 ffmpeg，不用写一行 C，也不用开 CGO。
它连的是仓库自带的单文件库 `libgpui_ffmpeg.so`（ffmpeg 7.1.5），
打开视频、解码、转颜色、重采样声音，全是调这个库干活。

包里分两层，用哪层看你要干什么：

- **只想播视频**：只认高层解码器 `decode.go`。`Open(路)` 打开，
  `Next()` 一帧一帧取（RGBA，到手就能显示），`SeekTo(毫秒)` 跳进度，
  `Close()` 关掉。`video` 的播放器就是这么接的，别绕过去。
- **只想解声音**：认 `audio_decode.go`。`OpenAudio(路)` 打开音轨，
  `Next()` 一块一块取（48kHz 立体声 float PCM，直接送喇叭），
  无音轨的片子报 `no audio track`，别当成坏片。
- **想自己使唤 ffmpeg 干别的**（比如只拆盒不解码、只转音频、挂滤镜）：
  用下面 13 个功能模块，so 里能用的公开函数全导出来了，
  每个都是 Go 写法，直接调就行。

## 库文件在哪

- 缺省在 `gpui/lib/ffmpeg/<系统>-<架构>/` 下找（共 6 个构建：linux 四架构 + win 双架构，mac 待补）。
- 两个版本：基础版 `libgpui_ffmpeg.(so|dll|dylib)` 只管看片（解码+拆盒），
  默认加载；高级版 `libgpui_ffmpeg_full.(so|dll|dylib)` 加写文件
  （复用+编码+烧字），`GPUI_FFMPEG_VARIANT=full` 才加载。
  `Variant()` 报当前是哪个版本，`IsFull()` 报是不是高级版。
- 环境变量 `GPUI_FFMPEG_PATH` 可以指定路径，测试和特殊目录用它（指哪加载哪，不受版本限制）。
- 库不在就别硬调：先问 `Available()`，回来 false 说明库没加载上。
- `LibPath()` 告诉你最后用的是哪个文件，`Version()` 报版本号（7.1.5）。

## 配套小文件（不是 13 模块，但是包的一部分）

| 文件        | 管什么                                                              |
|-------------|---------------------------------------------------------------------|
| `decode.go` | 高层解码器：`Open` / `Next` / `SeekTo` / `Close`，播放唯一入口       |
| `audio_decode.go` | 高层声音解码器：`OpenAudio` / `Next` / `SeekTo` / `Close`，48kHz 立体声 float PCM（无音轨报 no-audio，不是坏片） |
| `lib.go`    | 加载 so + 注册全部函数 + 存 so 句柄（数据符号走 `Dlsym` 读） + 解码直连的老接口（和模块指同一个 so 函数） |
| `types.go`  | `AVRational` 分数（时间换算用，偏移按 7.1 头文件钉死）              |
| `err_go.go` | 内部帮手：空指针哨兵、C 字符串转 Go、错误码翻人话                   |
| `doc.go`    | 包说明 + 覆盖口径（4 个变参走 Go 拼串版 + 16 个数据符号走 `Dlsym` 读，见下）                      |
| `data_const.go` | 数据常量：16 个数据符号的 Go 入口（10 个上下文大小 + 6 个版本串），走 `Dlsym` 读，不是函数调用 |
| `variadic_go.go` | 变参四件套的 Go 拼串版：`Asprintf` / `Util.Strlcatf` / `IOContext.Printf` / `Log.Logf` / `Log.Once` / `BPrint.BprintfF`（先拼好再调不带变参的函数，浮点也对） |

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
- 每个方法头上都有一句中文说明：干什么、对哪个 ffmpeg 函数、参数传什么、
  返回什么、nil 会不会崩。看代码时 `go doc` 直接出，不用来回翻本文。
- 返回错误码的函数：名字在 keep 名单里的留 `int32`，
  剩下的直接转 Go 的 `error`，负数就是出错，不用自己查表。
- 空指针安全：接收器是 nil 时直接返回零值或哨兵错，不会崩。
- `Free` / `Unref` / `Close` 结尾的就是“用完还回去”，记得调，不然漏内存。
- 传字符串只管传 Go 的 `string`，取回来的 C 字符串包里已经转好了。

## 覆盖口径：绑了多少、没绑哪几个

- so 里 `av*`（除 `avpriv` 内部）/`sws_*` / `swr_*` 开头的符号一共 1011 个：991 个函数全绑了 + 16 个数据全包了 + 4 个变参全有 Go 拼串版可用。
- 另外 11 个周边符号也在 so 里导出了，顺手一起绑了：
  `swscale_*` / `swresample_*` 的版本配置（各 3 个），
  加 5 个 `swri_*` 重采样帮手。函数注册 1005 行、去重后 1002 个（`lib.go` 解码直连 14 个与模块重复，属同一函数两个入口），`nm -D` 双向对过。
- 4 个 C 变参函数（参数个数不定）不直调，都有 Go 拼串版可用（`variadic_go.go`，`TestVariadicGo` 全钉死）：
  `av_asprintf` → `Asprintf`（Go 里拼好直接回字符串，不用管 C 内存）；
  `av_strlcatf` → `Util.Strlcatf`（拼好调 `av_strlcat` 接到 C 缓冲尾巴）；
  `avio_printf` → `IOContext.Printf`（拼好调 `avio_write` 写进动态流，`CloseDynBuf` 收尾取内容）；
  `av_log_once` → `Log.Once`（拼好调定长 `av_log`，首打一次走首级、之后走次级），另有 `Log.Logf` / `BPrint.BprintfF` 同走拼串。
  为啥不直调：purego 的 `...any` 只是把 Go 参数一个个放进寄存器，不是 C 变参，实测整数和字符串能混过去、浮点必错（1.5 变 0.0）。4 个函数全是 printf 风格，早晚收到浮点，直调就是埋错。
- 16 个数据符号全包了（`data_const.go`）：10 个 `const int` 上下文大小（`Crypto.AesSize` 等 9 个 + `Util.TreeNodeSize`）走 `Dlsym` 读数字，
  6 个 `const char[]` 版本串（`Library.CodecFfversion` 等）走 `Dlsym` 读字符串。
  函数用 `RegisterLibFunc` 绑，数据用 `Dlsym` 取地址再读，两条路都能用（`TestDataConst` 全钉死）。
- so 里还剩 13 个 `swri_*` 内部帮手没绑（重采样底层的私有函数，
  公开头文件里没有声明，按只导公开 API 的口径不导）。
- 同一个 so 函数偶尔有两个 Go 入口（比如 `lib.go` 的解码直连和模块方法，
  `av_get_pix_fmt` 在转色和查资料各有一套写法），底层是同一个函数，用哪个都行。
- 回调函数：日志回调走 `NewLogCallback` + `Log.SetGoCallback` 从 Go 直接写；
  自定义 IO 的读写定位回调（`AllocIOContext` 后三个参数）传 `purego.NewCallback`
  做的指针，不用就传 nil。回调指针建一次反复用，别放循环里。

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


下面 14 节就是全部家当：so 里绑的每个函数和每个数据都在里面，
左边是 Go 里怎么写，右边是它对应的 ffmpeg 原函数或数据符号。点开看就行。

### packet — 数据包（27 个）

数据包：拆盒吐出来的就是它，一包一包喂给解码器。`NewPacket` 新建一个空包，用完 `Free` 还回去。

抱着的结构体：`Packet`、`PacketSideData`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewPacket` | `av_packet_alloc` |
| `Packet.Clone` | `av_packet_clone` |
| `Packet.Free` | `av_packet_free` |
| `Packet.GetSideData` | `av_packet_get_side_data` |
| `Packet.NewSideData` | `av_packet_new_side_data` |
| `Dictionary.PackDictionary` | `av_packet_pack_dictionary` |
| `Packet.Ref` | `av_packet_ref` |
| `Packet.CopyProps` | `av_packet_copy_props` |
| `Packet.MoveRef` | `av_packet_move_ref` |
| `Packet.Unref` | `av_packet_unref` |
| `Packet.FreeSideData` | `av_packet_free_side_data` |
| `Packet.AddSideData` | `av_packet_add_side_data` |
| `Packet.ShrinkSideData` | `av_packet_shrink_side_data` |
| `Packet.NewPacketData` | `av_new_packet` |
| `Packet.Grow` | `av_grow_packet` |
| `Packet.Shrink` | `av_shrink_packet` |
| `Packet.FromData` | `av_packet_from_data` |
| `Packet.MakeRefcounted` | `av_packet_make_refcounted` |
| `Packet.MakeWritable` | `av_packet_make_writable` |
| `Packet.RescaleTs` | `av_packet_rescale_ts` |
| `UnpackDictionary` | `av_packet_unpack_dictionary` |
| `SideDataAdd` | `av_packet_side_data_add` |
| `SideDataFree` | `av_packet_side_data_free` |
| `SideDataGet` | `av_packet_side_data_get` |
| `SideDataName` | `av_packet_side_data_name` |
| `SideDataNew` | `av_packet_side_data_new` |
| `SideDataRemove` | `av_packet_side_data_remove` |
</details>
### frame — 帧（26 个）

帧：解出来的画面或声音，一帧就是一张图或一段声。`NewFrame` 新建空帧，用完 `Free` 还回去。

抱着的结构体：`Frame`、`FrameSideData`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewFrame` | `av_frame_alloc` |
| `Frame.Free` | `av_frame_free` |
| `Frame.Ref` | `av_frame_ref` |
| `Frame.Clone` | `av_frame_clone` |
| `Frame.Unref` | `av_frame_unref` |
| `Frame.MoveRef` | `av_frame_move_ref` |
| `Frame.Copy` | `av_frame_copy` |
| `Frame.CopyProps` | `av_frame_copy_props` |
| `Frame.GetBuffer` | `av_frame_get_buffer` |
| `Frame.IsWritable` | `av_frame_is_writable` |
| `Frame.MakeWritable` | `av_frame_make_writable` |
| `Frame.ApplyCropping` | `av_frame_apply_cropping` |
| `Frame.GetPlaneBuffer` | `av_frame_get_plane_buffer` |
| `Frame.GetSideData` | `av_frame_get_side_data` |
| `Frame.NewSideData` | `av_frame_new_side_data` |
| `Frame.NewSideDataFromBuf` | `av_frame_new_side_data_from_buf` |
| `Frame.RemoveSideData` | `av_frame_remove_side_data` |
| `FrameSideDataAdd` | `av_frame_side_data_add` |
| `FrameSideDataClone` | `av_frame_side_data_clone` |
| `FrameSideDataDesc` | `av_frame_side_data_desc` |
| `FrameSideDataFree` | `av_frame_side_data_free` |
| `FrameSideDataGet` | `av_frame_side_data_get_c` |
| `FrameSideDataName` | `av_frame_side_data_name` |
| `FrameSideDataNew` | `av_frame_side_data_new` |
| `FrameSideDataRemove` | `av_frame_side_data_remove` |
| `Frame.Replace` | `av_frame_replace` |
</details>
### dict_opt — 字典和选项（62 个）

字典和选项：打开文件、开解码器时传参数都走它。

抱着的结构体：`Dictionary`（键值对）、`OptObject`（可调参数的对象）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Dictionary.Copy` | `av_dict_copy` |
| `Dictionary.Count` | `av_dict_count` |
| `Dictionary.Free` | `av_dict_free` |
| `Dictionary.Get` | `av_dict_get` |
| `Dictionary.GetString` | `av_dict_get_string` |
| `Dictionary.Iterate` | `av_dict_iterate` |
| `Dictionary.ParseString` | `av_dict_parse_string` |
| `Dictionary.Set` | `av_dict_set` |
| `Dictionary.SetInt` | `av_dict_set_int` |
| `ChildClassIterate` | `av_opt_child_class_iterate` |
| `OptObject.ChildNext` | `av_opt_child_next` |
| `OptObject.Copy` | `av_opt_copy` |
| `OptObject.EvalDouble` | `av_opt_eval_double` |
| `OptObject.EvalFlags` | `av_opt_eval_flags` |
| `OptObject.EvalFloat` | `av_opt_eval_float` |
| `OptObject.EvalInt` | `av_opt_eval_int` |
| `OptObject.EvalInt64` | `av_opt_eval_int64` |
| `OptObject.EvalQ` | `av_opt_eval_q` |
| `OptObject.EvalUint` | `av_opt_eval_uint` |
| `OptObject.Find` | `av_opt_find` |
| `OptObject.Find2` | `av_opt_find2` |
| `OptObject.FlagIsSet` | `av_opt_flag_is_set` |
| `FreeOptions` | `av_opt_free` |
| `OptionRanges.FreeRanges` | `av_opt_freep_ranges` |
| `OptObject.Get` | `av_opt_get` |
| `OptObject.GetArray` | `av_opt_get_array` |
| `OptObject.GetArraySize` | `av_opt_get_array_size` |
| `OptObject.GetChlayout` | `av_opt_get_chlayout` |
| `OptObject.GetDictVal` | `av_opt_get_dict_val` |
| `OptObject.GetDouble` | `av_opt_get_double` |
| `OptObject.GetImageSize` | `av_opt_get_image_size` |
| `OptObject.GetInt` | `av_opt_get_int` |
| `GetKeyValue` | `av_opt_get_key_value` |
| `OptObject.GetPixFmt` | `av_opt_get_pixel_fmt` |
| `OptObject.GetQ` | `av_opt_get_q` |
| `OptObject.GetSampleFmt` | `av_opt_get_sample_fmt` |
| `OptObject.GetVideoRate` | `av_opt_get_video_rate` |
| `OptObject.IsDefault` | `av_opt_is_set_to_default` |
| `OptObject.IsDefaultByName` | `av_opt_is_set_to_default_by_name` |
| `OptObject.NextOption` | `av_opt_next` |
| `OptObject.FieldPtr` | `av_opt_ptr` |
| `OptObject.QueryRanges` | `av_opt_query_ranges` |
| `OptObject.QueryRangesDefault` | `av_opt_query_ranges_default` |
| `OptObject.Serialize` | `av_opt_serialize` |
| `OptObject.Set` | `av_opt_set` |
| `OptObject.SetArray` | `av_opt_set_array` |
| `OptObject.SetBin` | `av_opt_set_bin` |
| `OptObject.SetChlayout` | `av_opt_set_chlayout` |
| `OptObject.SetDefaults` | `av_opt_set_defaults` |
| `OptObject.SetDefaults2` | `av_opt_set_defaults2` |
| `OptObject.SetDict` | `av_opt_set_dict` |
| `OptObject.SetDict2` | `av_opt_set_dict2` |
| `OptObject.SetDictVal` | `av_opt_set_dict_val` |
| `OptObject.SetDouble` | `av_opt_set_double` |
| `OptObject.SetFromString` | `av_opt_set_from_string` |
| `OptObject.SetImageSize` | `av_opt_set_image_size` |
| `OptObject.SetInt` | `av_opt_set_int` |
| `OptObject.SetPixFmt` | `av_opt_set_pixel_fmt` |
| `OptObject.SetQ` | `av_opt_set_q` |
| `OptObject.SetSampleFmt` | `av_opt_set_sample_fmt` |
| `OptObject.SetVideoRate` | `av_opt_set_video_rate` |
| `OptObject.ShowOptions` | `av_opt_show2` |
</details>
### buffer_mem — 内存（53 个）

内存：申请、引用计数、缓冲池、队列、拼字符串。

抱着的结构体：`Mem`、`Buffer`、`BufferPool`、`Fifo`、`BPrint`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewBuffer` | `av_buffer_alloc` |
| `NewBufferZeroed` | `av_buffer_allocz` |
| `WrapBuffer` | `av_buffer_create` |
| `DefaultFree` | `av_buffer_default_free` |
| `Buffer.Opaque` | `av_buffer_get_opaque` |
| `Buffer.RefCount` | `av_buffer_get_ref_count` |
| `Buffer.IsWritable` | `av_buffer_is_writable` |
| `Buffer.MakeWritable` | `av_buffer_make_writable` |
| `Buffer.PoolOpaque` | `av_buffer_pool_buffer_get_opaque` |
| `BufferPool.Get` | `av_buffer_pool_get` |
| `NewBufferPool` | `av_buffer_pool_init` |
| `NewBufferPoolCustom` | `av_buffer_pool_init2` |
| `BufferPool.Uninit` | `av_buffer_pool_uninit` |
| `ReallocBuffer` | `av_buffer_realloc` |
| `Buffer.Ref` | `av_buffer_ref` |
| `Buffer.Unref` | `av_buffer_unref` |
| `Buffer.Replace` | `av_buffer_replace` |
| `Mem.Alloc` | `av_malloc` |
| `Mem.AllocArray` | `av_malloc_array` |
| `Mem.AllocZ` | `av_mallocz` |
| `Mem.Realloc` | `av_realloc` |
| `Mem.ReallocArray` | `av_realloc_array` |
| `Mem.ReallocF` | `av_realloc_f` |
| `Mem.ReallocP` | `av_reallocp` |
| `Mem.ReallocPArray` | `av_reallocp_array` |
| `Mem.Free` | `av_free` |
| `Mem.Freep` | `av_freep` |
| `Mem.Dup` | `av_memdup` |
| `Mem.MemcpyBackptr` | `av_memcpy_backptr` |
| `NewFifo` | `av_fifo_alloc2` |
| `Fifo.SetGrowLimit` | `av_fifo_auto_grow_limit` |
| `Fifo.CanRead` | `av_fifo_can_read` |
| `Fifo.CanWrite` | `av_fifo_can_write` |
| `Fifo.Drain` | `av_fifo_drain2` |
| `Fifo.ElemSize` | `av_fifo_elem_size` |
| `Fifo.Freep` | `av_fifo_freep2` |
| `Fifo.Grow2` | `av_fifo_grow2` |
| `Fifo.Peek` | `av_fifo_peek` |
| `Fifo.PeekToCallback` | `av_fifo_peek_to_cb` |
| `Fifo.Read` | `av_fifo_read` |
| `Fifo.ReadToCallback` | `av_fifo_read_to_cb` |
| `Fifo.Reset` | `av_fifo_reset2` |
| `Fifo.Write` | `av_fifo_write` |
| `Fifo.WriteFromCallback` | `av_fifo_write_from_cb` |
| `BPrint.AppendData` | `av_bprint_append_data` |
| `BPrint.AppendChar` | `av_bprint_chars` |
| `BPrint.Clear` | `av_bprint_clear` |
| `BPrint.Escape` | `av_bprint_escape` |
| `BPrint.Finalize` | `av_bprint_finalize` |
| `BPrint.GetBuffer` | `av_bprint_get_buffer` |
| `NewBPrint` | `av_bprint_init` |
| `BPrint.InitForBuffer` | `av_bprint_init_for_buffer` |
| `BPrint.AppendTime` | `av_bprint_strftime` |
</details>
### error_log — 报错和杂务（29 个）

报错和杂务：错误码翻人话、日志开关（Go 回调走 `NewLogCallback` + `Log.SetGoCallback`）、版本、时间戳换算、CPU 数。都是无状态的，直接调。

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

拆盒：打开文件、找音视频流、读包、跳进度。先 `OpenInput` 拿到 `FormatContext`，后面全挂它身上。自定义数据源走 `AllocIOContext`（读写定位三个回调传 `purego.NewCallback` 做的指针，不用就传 nil）。

抱着的结构体：`FormatContext`、`Stream`、`IOContext`、`Format`。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Stream.AddIndexEntry` | `av_add_index_entry` |
| `FormatContext.DumpFormat` | `av_dump_format` |
| `FormatContext.FindBestStream` | `av_find_best_stream` |
| `FormatContext.AllocContext` | `avformat_alloc_context` |
| `FormatContext.AllocOutputContext2` | `avformat_alloc_output_context2` |
| `FormatContext.CloseInput` | `avformat_close_input` |
| `FormatContext.Configuration` | `avformat_configuration` |
| `FormatContext.FindStreamInfo` | `avformat_find_stream_info` |
| `FormatContext.Flush` | `avformat_flush` |
| `FormatContext.FreeContext` | `avformat_free_context` |
| `FormatContext.GetClass` | `avformat_get_class` |
| `FormatContext.GetMovAudioTags` | `avformat_get_mov_audio_tags` |
| `FormatContext.GetMovVideoTags` | `avformat_get_mov_video_tags` |
| `FormatContext.GetRiffAudioTags` | `avformat_get_riff_audio_tags` |
| `FormatContext.GetRiffVideoTags` | `avformat_get_riff_video_tags` |
| `Stream.IndexGetEntriesCount` | `avformat_index_get_entries_count` |
| `Stream.IndexGetEntry` | `avformat_index_get_entry` |
| `Stream.IndexGetEntryFromTimestamp` | `avformat_index_get_entry_from_timestamp` |
| `FormatContext.InitOutput` | `avformat_init_output` |
| `FormatContext.License` | `avformat_license` |
| `FormatContext.MatchStreamSpecifier` | `avformat_match_stream_specifier` |
| `FormatContext.NetworkDeinit` | `avformat_network_deinit` |
| `FormatContext.NetworkInit` | `avformat_network_init` |
| `FormatContext.NewStream` | `avformat_new_stream` |
| `FormatContext.OpenInput` | `avformat_open_input` |
| `Format.QueryCodec` | `avformat_query_codec` |
| `FormatContext.QueueAttachedPictures` | `avformat_queue_attached_pictures` |
| `FormatContext.SeekFile` | `avformat_seek_file` |
| `FormatContext.StreamGroupAddStream` | `avformat_stream_group_add_stream` |
| `FormatContext.StreamGroupCreate` | `avformat_stream_group_create` |
| `FormatContext.StreamGroupName` | `avformat_stream_group_name` |
| `FormatContext.Version` | `avformat_version` |
| `FormatContext.WriteHeader` | `avformat_write_header` |
| `Format.GuessCodec` | `av_guess_codec` |
| `FormatContext.GuessFormat` | `av_guess_format` |
| `FormatContext.GuessFrameRate` | `av_guess_frame_rate` |
| `FormatContext.GuessSampleAspectRatio` | `av_guess_sample_aspect_ratio` |
| `Stream.IndexSearchTimestamp` | `av_index_search_timestamp` |
| `IOContext.Accept` | `avio_accept` |
| `FormatContext.AllocIOContext` | `avio_alloc_context` |
| `FormatContext.Check` | `avio_check` |
| `IOContext.Close` | `avio_close` |
| `FormatContext.CloseDir` | `avio_close_dir` |
| `IOContext.CloseDynBuf` | `avio_close_dyn_buf` |
| `FormatContext.Closep` | `avio_closep` |
| `FormatContext.ContextFree` | `avio_context_free` |
| `FormatContext.EnumProtocols` | `avio_enum_protocols` |
| `IOContext.Feof` | `avio_feof` |
| `FormatContext.FindProtocolName` | `avio_find_protocol_name` |
| `IOContext.Flush` | `avio_flush` |
| `FormatContext.FreeDirectoryEntry` | `avio_free_directory_entry` |
| `IOContext.GetDynBuf` | `avio_get_dyn_buf` |
| `IOContext.GetStr` | `avio_get_str` |
| `IOContext.GetStr16be` | `avio_get_str16be` |
| `IOContext.GetStr16le` | `avio_get_str16le` |
| `IOContext.Handshake` | `avio_handshake` |
| `FormatContext.Open` | `avio_open` |
| `FormatContext.Open2` | `avio_open2` |
| `FormatContext.OpenDir` | `avio_open_dir` |
| `FormatContext.OpenDynBuf` | `avio_open_dyn_buf` |
| `IOContext.Pause` | `avio_pause` |
| `IOContext.PrintStringArray` | `avio_print_string_array` |
| `FormatContext.ProtocolGetClass` | `avio_protocol_get_class` |
| `IOContext.PutStr` | `avio_put_str` |
| `IOContext.PutStr16be` | `avio_put_str16be` |
| `IOContext.PutStr16le` | `avio_put_str16le` |
| `IOContext.R8` | `avio_r8` |
| `IOContext.Rb16` | `avio_rb16` |
| `IOContext.Rb24` | `avio_rb24` |
| `IOContext.Rb32` | `avio_rb32` |
| `IOContext.Rb64` | `avio_rb64` |
| `IOContext.Read` | `avio_read` |
| `FormatContext.ReadDir` | `avio_read_dir` |
| `IOContext.ReadPartial` | `avio_read_partial` |
| `IOContext.ReadToBprint` | `avio_read_to_bprint` |
| `IOContext.Rl16` | `avio_rl16` |
| `IOContext.Rl24` | `avio_rl24` |
| `IOContext.Rl32` | `avio_rl32` |
| `IOContext.Rl64` | `avio_rl64` |
| `IOContext.SeekPos` | `avio_seek` |
| `IOContext.SeekTime` | `avio_seek_time` |
| `IOContext.Size` | `avio_size` |
| `IOContext.Skip` | `avio_skip` |
| `IOContext.Vprintf` | `avio_vprintf` |
| `IOContext.W8` | `avio_w8` |
| `IOContext.Wb16` | `avio_wb16` |
| `IOContext.Wb24` | `avio_wb24` |
| `IOContext.Wb32` | `avio_wb32` |
| `IOContext.Wb64` | `avio_wb64` |
| `IOContext.Wl16` | `avio_wl16` |
| `IOContext.Wl24` | `avio_wl24` |
| `IOContext.Wl32` | `avio_wl32` |
| `IOContext.Wl64` | `avio_wl64` |
| `IOContext.Write` | `avio_write` |
| `IOContext.WriteMarker` | `avio_write_marker` |
| `FormatContext.ReadFrame` | `av_read_frame` |
| `FormatContext.SeekFrame` | `av_seek_frame` |
| `FormatContext.UrlSplit` | `av_url_split` |
</details>
### codec_encode — 编解码（76 个）

编解码：找解码器、开解码器、送包取帧、码流过滤。

抱着的结构体：`CodecContext`（解码器实例）、`Codec`（解码器本身）、`CodecParameters`（流参数）、`Parser`（切帧）、`BitStreamFilter`（码流滤镜）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Codec.AllocContext` | `avcodec_alloc_context3` |
| `CodecContext.Close` | `avcodec_close` |
| `FindDecoder` | `avcodec_find_decoder` |
| `FindDecoderByName` | `avcodec_find_decoder_by_name` |
| `FindEncoder` | `avcodec_find_encoder` |
| `FindEncoderByName` | `avcodec_find_encoder_by_name` |
| `CodecContext.FlushBuffers` | `avcodec_flush_buffers` |
| `CodecContext.FreeContext` | `avcodec_free_context` |
| `CodecName` | `avcodec_get_name` |
| `CodecType` | `avcodec_get_type` |
| `CodecContext.IsOpen` | `avcodec_is_open` |
| `CodecContext.Open` | `avcodec_open2` |
| `CodecContext.SendPacket` | `avcodec_send_packet` |
| `CodecContext.ReceiveFrame` | `avcodec_receive_frame` |
| `CodecContext.SendFrame` | `avcodec_send_frame` |
| `CodecContext.ReceivePacket` | `avcodec_receive_packet` |
| `NewCodecParameters` | `avcodec_parameters_alloc` |
| `CodecParameters.Free` | `avcodec_parameters_free` |
| `CodecParameters.Copy` | `avcodec_parameters_copy` |
| `CodecParameters.FromContext` | `avcodec_parameters_from_context` |
| `CodecParameters.ToContext` | `avcodec_parameters_to_context` |
| `Codec.IsDecoder` | `av_codec_is_decoder` |
| `Codec.IsEncoder` | `av_codec_is_encoder` |
| `Codec.Iterate` | `av_codec_iterate` |
| `NewParser` | `av_parser_init` |
| `Parser.Parse2` | `av_parser_parse2` |
| `Parser.Close` | `av_parser_close` |
| `NewBitStreamFilter` | `av_bsf_alloc` |
| `BitStreamFilter.Init` | `av_bsf_init` |
| `BitStreamFilter.SendPacket` | `av_bsf_send_packet` |
| `BitStreamFilter.ReceivePacket` | `av_bsf_receive_packet` |
| `BitStreamFilter.Flush` | `av_bsf_flush` |
| `BitStreamFilter.Free` | `av_bsf_free` |
| `NewBitStreamFilter` | `av_bsf_get_by_name` |
| `AllocBSFList` | `av_bsf_list_alloc` |
| `BitStreamFilterList.Append` | `av_bsf_list_append` |
| `BitStreamFilterList.AppendByName` | `av_bsf_list_append2` |
| `BitStreamFilterList.Finalize` | `av_bsf_list_finalize` |
| `BitStreamFilterList.Free` | `av_bsf_list_free` |
| `ParseBSFList` | `av_bsf_list_parse_str` |
| `BitStreamFilter.BsfGetClass` | `av_bsf_get_class` |
| `BitStreamFilter.BsfGetNullFilter` | `av_bsf_get_null_filter` |
| `BitStreamFilter.BsfIterate` | `av_bsf_iterate` |
| `Codec.AvcodecAlignDimensions` | `avcodec_align_dimensions` |
| `Codec.AvcodecAlignDimensions2` | `avcodec_align_dimensions2` |
| `Codec.AvcodecConfiguration` | `avcodec_configuration` |
| `Codec.AvcodecDctAlloc` | `avcodec_dct_alloc` |
| `Codec.AvcodecDctGetClass` | `avcodec_dct_get_class` |
| `Codec.AvcodecDctInit` | `avcodec_dct_init` |
| `Codec.AvcodecDecodeSubtitle2` | `avcodec_decode_subtitle2` |
| `Codec.AvcodecDefaultExecute` | `avcodec_default_execute` |
| `Codec.AvcodecDefaultExecute2` | `avcodec_default_execute2` |
| `Codec.AvcodecDefaultGetBuffer2` | `avcodec_default_get_buffer2` |
| `Codec.AvcodecDefaultGetEncodeBuffer` | `avcodec_default_get_encode_buffer` |
| `Codec.AvcodecDefaultGetFormat` | `avcodec_default_get_format` |
| `Codec.AvcodecDescriptorGet` | `avcodec_descriptor_get` |
| `Codec.AvcodecDescriptorGetByName` | `avcodec_descriptor_get_by_name` |
| `Codec.AvcodecDescriptorNext` | `avcodec_descriptor_next` |
| `Codec.AvcodecEncodeSubtitle` | `avcodec_encode_subtitle` |
| `Codec.AvcodecFillAudioFrame` | `avcodec_fill_audio_frame` |
| `Codec.AvcodecFindBestPixFmtOfList` | `avcodec_find_best_pix_fmt_of_list` |
| `Codec.AvcodecGetClass` | `avcodec_get_class` |
| `Codec.AvcodecGetHwConfig` | `avcodec_get_hw_config` |
| `Codec.AvcodecGetHwFramesParameters` | `avcodec_get_hw_frames_parameters` |
| `Codec.CodecGetId` | `av_codec_get_id` |
| `Codec.AvcodecGetSubtitleRectClass` | `avcodec_get_subtitle_rect_class` |
| `Codec.AvcodecGetSupportedConfig` | `avcodec_get_supported_config` |
| `Codec.CodecGetTag` | `av_codec_get_tag` |
| `Codec.CodecGetTag2` | `av_codec_get_tag2` |
| `Codec.AvcodecLicense` | `avcodec_license` |
| `Codec.AvcodecPixFmtToCodecTag` | `avcodec_pix_fmt_to_codec_tag` |
| `Codec.AvcodecProfileName` | `avcodec_profile_name` |
| `Codec.AvcodecString` | `avcodec_string` |
| `Codec.AvcodecVersion` | `avcodec_version` |
| `Parser.ParserIterate` | `av_parser_iterate` |
| `Codec.SubtitleFree` | `avsubtitle_free` |
</details>
### scale_color — 转色和缩放（40 个）

转色和缩放：YUV 转屏幕要的 RGBA，尺寸也能顺手改。`NewScaler` 建转色器，`PixFmtName` 查格式名。`sws_*` 全归这里。

抱着的结构体：`Scaler`（转色器）、`Image`（图片帮手）。常用常量：`PixFmt*`（像素格式）、`Sws*`（算法）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `NewScaler` | `sws_getContext` |
| `CachedScaler` | `sws_getCachedContext` |
| `Scaler.Scale` | `sws_scale` |
| `Scaler.ScaleFrame` | `sws_scale_frame` |
| `Scaler.Free` | `sws_freeContext` |
| `AllocScalerContext` | `sws_alloc_context` |
| `Scaler.InitContext` | `sws_init_context` |
| `IsSupportedInput` | `sws_isSupportedInput` |
| `IsSupportedOutput` | `sws_isSupportedOutput` |
| `IsEndianSupported` | `sws_isSupportedEndiannessConversion` |
| `ImageBufferSize` | `av_image_get_buffer_size` |
| `ImageAlloc` | `av_image_alloc` |
| `ImageCopyPlane` | `av_image_copy_plane` |
| `ImageCheckSize` | `av_image_check_size` |
| `ImageCheckSize2` | `av_image_check_size2` |
| `ImageCheckSar` | `av_image_check_sar` |
| `PixFmtDesc` | `av_pix_fmt_desc_get` |
| `PixFmtName` | `av_get_pix_fmt_name` |
| `PixFmtFromName` | `av_get_pix_fmt` |
| `Scaler.SwscaleConfiguration` | `swscale_configuration` |
| `Scaler.SwscaleLicense` | `swscale_license` |
| `Scaler.SwscaleVersion` | `swscale_version` |
| `Scaler.SwsAllocVec` | `sws_allocVec` |
| `Scaler.SwsConvertPalette8ToPacked24` | `sws_convertPalette8ToPacked24` |
| `Scaler.SwsConvertPalette8ToPacked32` | `sws_convertPalette8ToPacked32` |
| `Scaler.SwsFrameEnd` | `sws_frame_end` |
| `Scaler.SwsFrameStart` | `sws_frame_start` |
| `Scaler.SwsFreeFilter` | `sws_freeFilter` |
| `Scaler.SwsFreeVec` | `sws_freeVec` |
| `Scaler.SwsGetClass` | `sws_get_class` |
| `Scaler.SwsGetCoefficients` | `sws_getCoefficients` |
| `Scaler.SwsGetColorspaceDetails` | `sws_getColorspaceDetails` |
| `Scaler.SwsGetDefaultFilter` | `sws_getDefaultFilter` |
| `Scaler.SwsGetGaussianVec` | `sws_getGaussianVec` |
| `Scaler.SwsNormalizeVec` | `sws_normalizeVec` |
| `Scaler.SwsReceiveSlice` | `sws_receive_slice` |
| `Scaler.SwsReceiveSliceAlignment` | `sws_receive_slice_alignment` |
| `Scaler.SwsScaleVec` | `sws_scaleVec` |
| `Scaler.SwsSendSlice` | `sws_send_slice` |
| `Scaler.SwsSetColorspaceDetails` | `sws_setColorspaceDetails` |
</details>
### resample_audio — 音频重采样（37 个）

音频重采样：声道、采样率、采样格式不一样时掰成一样。

抱着的结构体：`Resampler`（重采样器）、`AudioFifo`（音频队列）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Resampler.Alloc` | `av_audio_fifo_alloc` |
| `AudioFifo.Drain` | `av_audio_fifo_drain` |
| `AudioFifo.Free` | `av_audio_fifo_free` |
| `AudioFifo.Peek` | `av_audio_fifo_peek` |
| `AudioFifo.PeekAt` | `av_audio_fifo_peek_at` |
| `AudioFifo.Read` | `av_audio_fifo_read` |
| `AudioFifo.Realloc` | `av_audio_fifo_realloc` |
| `AudioFifo.Reset` | `av_audio_fifo_reset` |
| `AudioFifo.Size` | `av_audio_fifo_size` |
| `AudioFifo.Space` | `av_audio_fifo_space` |
| `AudioFifo.Write` | `av_audio_fifo_write` |
| `Resampler.GetBytesPerSample` | `av_get_bytes_per_sample` |
| `Resampler.GetPackedSampleFmt` | `av_get_packed_sample_fmt` |
| `Resampler.GetPlanarSampleFmt` | `av_get_planar_sample_fmt` |
| `Resampler.GetSampleFmt` | `av_get_sample_fmt` |
| `Resampler.GetSampleFmtName` | `av_get_sample_fmt_name` |
| `Resampler.GetSampleFmtString` | `av_get_sample_fmt_string` |
| `Resampler.SampleFmtIsPlanar` | `av_sample_fmt_is_planar` |
| `Resampler.Alloc2` | `swr_alloc` |
| `Resampler.AllocSetOpts2` | `swr_alloc_set_opts2` |
| `Resampler.BuildMatrix2` | `swr_build_matrix2` |
| `Resampler.Close` | `swr_close` |
| `Resampler.ConfigFrame` | `swr_config_frame` |
| `Resampler.Convert` | `swr_convert` |
| `Resampler.ConvertFrame` | `swr_convert_frame` |
| `Resampler.DropOutput` | `swr_drop_output` |
| `Resampler.Free` | `swr_free` |
| `Resampler.GetClass` | `swr_get_class` |
| `Resampler.GetDelay` | `swr_get_delay` |
| `Resampler.GetOutSamples` | `swr_get_out_samples` |
| `Resampler.Init` | `swr_init` |
| `Resampler.InjectSilence` | `swr_inject_silence` |
| `Resampler.IsInitialized` | `swr_is_initialized` |
| `Resampler.NextPts` | `swr_next_pts` |
| `Resampler.SetChannelMapping` | `swr_set_channel_mapping` |
| `Resampler.SetCompensation` | `swr_set_compensation` |
| `Resampler.SetMatrix` | `swr_set_matrix` |
</details>
### filter_graph — 滤镜图（64 个）

滤镜图：把多个滤镜连成链（缩放、裁剪、混音都归它）。

抱着的结构体：`FilterGraph`（图）、`FilterContext`（滤镜实例）、`Filter`（滤镜本身）、`FilterSink`（出口）、`FilterSource`（入口）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `FilterContext.GetChannels` | `av_buffersink_get_channels` |
| `FilterSink.GetChLayout` | `av_buffersink_get_ch_layout` |
| `FilterSink.GetColorRange` | `av_buffersink_get_color_range` |
| `FilterSink.GetColorspace` | `av_buffersink_get_colorspace` |
| `FilterSink.GetFormat` | `av_buffersink_get_format` |
| `FilterSink.GetFrame` | `av_buffersink_get_frame` |
| `FilterSink.GetFrameFlags` | `av_buffersink_get_frame_flags` |
| `FilterSink.GetFrameRate` | `av_buffersink_get_frame_rate` |
| `FilterSink.GetH` | `av_buffersink_get_h` |
| `FilterSink.GetHwFramesCtx` | `av_buffersink_get_hw_frames_ctx` |
| `FilterSink.GetSampleAspectRatio` | `av_buffersink_get_sample_aspect_ratio` |
| `FilterSink.GetSampleRate` | `av_buffersink_get_sample_rate` |
| `FilterSink.GetSamples` | `av_buffersink_get_samples` |
| `FilterSink.GetTimeBase` | `av_buffersink_get_time_base` |
| `FilterSink.GetType` | `av_buffersink_get_type` |
| `FilterSink.GetW` | `av_buffersink_get_w` |
| `FilterSink.SetFrameSize` | `av_buffersink_set_frame_size` |
| `FilterSource.AddFrame` | `av_buffersrc_add_frame` |
| `FilterContext.AddFrameFlags` | `av_buffersrc_add_frame_flags` |
| `FilterContext.Close` | `av_buffersrc_close` |
| `FilterContext.GetNbFailedRequests` | `av_buffersrc_get_nb_failed_requests` |
| `FilterSource.GetStatus` | `av_buffersrc_get_status` |
| `FilterSource.ParametersAlloc` | `av_buffersrc_parameters_alloc` |
| `FilterSource.ParametersSet` | `av_buffersrc_parameters_set` |
| `FilterContext.WriteFrame` | `av_buffersrc_write_frame` |
| `FilterContext.ConfigLinks` | `avfilter_config_links` |
| `FilterGraph.Configuration` | `avfilter_configuration` |
| `Filter.FilterPadCount` | `avfilter_filter_pad_count` |
| `FilterContext.Free` | `avfilter_free` |
| `FilterGraph.GetByName` | `avfilter_get_by_name` |
| `FilterGraph.GetClass` | `avfilter_get_class` |
| `FilterGraph.GraphAlloc` | `avfilter_graph_alloc` |
| `FilterGraph.GraphAllocFilter` | `avfilter_graph_alloc_filter` |
| `FilterGraph.GraphConfig` | `avfilter_graph_config` |
| `FilterGraph.GraphCreateFilter` | `avfilter_graph_create_filter` |
| `FilterGraph.GraphDump` | `avfilter_graph_dump` |
| `FilterGraph.GraphFree` | `avfilter_graph_free` |
| `FilterGraph.GraphGetFilter` | `avfilter_graph_get_filter` |
| `FilterGraph.GraphParse` | `avfilter_graph_parse` |
| `FilterGraph.GraphParse2` | `avfilter_graph_parse2` |
| `FilterGraph.GraphParsePtr` | `avfilter_graph_parse_ptr` |
| `FilterGraph.GraphQueueCommand` | `avfilter_graph_queue_command` |
| `FilterGraph.GraphRequestOldest` | `avfilter_graph_request_oldest` |
| `FilterGraph.GraphSegmentApply` | `avfilter_graph_segment_apply` |
| `FilterGraph.GraphSegmentApplyOpts` | `avfilter_graph_segment_apply_opts` |
| `FilterGraph.GraphSegmentCreateFilters` | `avfilter_graph_segment_create_filters` |
| `FilterGraph.GraphSegmentFree` | `avfilter_graph_segment_free` |
| `FilterGraph.GraphSegmentInit` | `avfilter_graph_segment_init` |
| `FilterGraph.GraphSegmentLink` | `avfilter_graph_segment_link` |
| `FilterGraph.GraphSegmentParse` | `avfilter_graph_segment_parse` |
| `FilterGraph.GraphSendCommand` | `avfilter_graph_send_command` |
| `FilterGraph.GraphSetAutoConvert` | `avfilter_graph_set_auto_convert` |
| `FilterContext.InitDict` | `avfilter_init_dict` |
| `FilterContext.InitStr` | `avfilter_init_str` |
| `FilterGraph.InoutAlloc` | `avfilter_inout_alloc` |
| `FilterGraph.InoutFree` | `avfilter_inout_free` |
| `FilterGraph.InsertFilter` | `avfilter_insert_filter` |
| `FilterGraph.License` | `avfilter_license` |
| `FilterContext.Link` | `avfilter_link` |
| `FilterGraph.LinkFree` | `avfilter_link_free` |
| `FilterGraph.PadGetName` | `avfilter_pad_get_name` |
| `FilterGraph.PadGetType` | `avfilter_pad_get_type` |
| `FilterContext.ProcessCommand` | `avfilter_process_command` |
| `FilterGraph.Version` | `avfilter_version` |
</details>
### device_io — 设备（10 个）

设备：列摄像头、麦克风这些输入输出设备。

抱着的结构体：`DeviceList`（设备表）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `DeviceList.Version` | `avdevice_version` |
| `DeviceList.Configuration` | `avdevice_configuration` |
| `DeviceList.License` | `avdevice_license` |
| `DeviceList.RegisterAll` | `avdevice_register_all` |
| `DeviceList.ListDevices` | `avdevice_list_devices` |
| `DeviceList.FreeList` | `avdevice_free_list_devices` |
| `DeviceList.ListInputSources` | `avdevice_list_input_sources` |
| `DeviceList.ListOutputSinks` | `avdevice_list_output_sinks` |
| `DeviceList.AppToDev` | `avdevice_app_to_dev_control_message` |
| `DeviceList.DevToApp` | `avdevice_dev_to_app_control_message` |
</details>
### media_desc — 查资料（88 个）

查资料：像素格式、声道布局、采样率这些只读信息，不干活只问。

抱着的结构体：`MediaDesc`（无状态，全是查询）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `MediaDesc.AmbientViewingEnvironmentAlloc` | `av_ambient_viewing_environment_alloc` |
| `MediaDesc.AmbientViewingEnvironmentCreateSideData` | `av_ambient_viewing_environment_create_side_data` |
| `MediaDesc.ChannelDescription` | `av_channel_description` |
| `MediaDesc.ChannelDescriptionBprint` | `av_channel_description_bprint` |
| `MediaDesc.ChannelFromString` | `av_channel_from_string` |
| `MediaDesc.ChannelLayoutAmbisonicOrder` | `av_channel_layout_ambisonic_order` |
| `MediaDesc.ChannelLayoutChannelFromIndex` | `av_channel_layout_channel_from_index` |
| `MediaDesc.ChannelLayoutChannelFromString` | `av_channel_layout_channel_from_string` |
| `MediaDesc.ChannelLayoutCheck` | `av_channel_layout_check` |
| `MediaDesc.ChannelLayoutCompare` | `av_channel_layout_compare` |
| `MediaDesc.ChannelLayoutCopy` | `av_channel_layout_copy` |
| `MediaDesc.ChannelLayoutCustomInit` | `av_channel_layout_custom_init` |
| `MediaDesc.ChannelLayoutDefault` | `av_channel_layout_default` |
| `MediaDesc.ChannelLayoutDescribe` | `av_channel_layout_describe` |
| `MediaDesc.ChannelLayoutDescribeBprint` | `av_channel_layout_describe_bprint` |
| `MediaDesc.ChannelLayoutFromMask` | `av_channel_layout_from_mask` |
| `MediaDesc.ChannelLayoutFromString` | `av_channel_layout_from_string` |
| `MediaDesc.ChannelLayoutIndexFromChannel` | `av_channel_layout_index_from_channel` |
| `MediaDesc.ChannelLayoutIndexFromString` | `av_channel_layout_index_from_string` |
| `MediaDesc.ChannelLayoutRetype` | `av_channel_layout_retype` |
| `MediaDesc.ChannelLayoutStandard` | `av_channel_layout_standard` |
| `MediaDesc.ChannelLayoutSubset` | `av_channel_layout_subset` |
| `MediaDesc.ChannelLayoutUninit` | `av_channel_layout_uninit` |
| `MediaDesc.ChannelName` | `av_channel_name` |
| `MediaDesc.ChannelNameBprint` | `av_channel_name_bprint` |
| `MediaDesc.ColorPrimariesFromName` | `av_color_primaries_from_name` |
| `MediaDesc.ColorPrimariesName` | `av_color_primaries_name` |
| `MediaDesc.ColorRangeFromName` | `av_color_range_from_name` |
| `MediaDesc.ColorRangeName` | `av_color_range_name` |
| `MediaDesc.ColorSpaceFromName` | `av_color_space_from_name` |
| `MediaDesc.ColorSpaceName` | `av_color_space_name` |
| `MediaDesc.ColorTransferFromName` | `av_color_transfer_from_name` |
| `MediaDesc.ColorTransferName` | `av_color_transfer_name` |
| `MediaDesc.ContentLightMetadataAlloc` | `av_content_light_metadata_alloc` |
| `MediaDesc.ContentLightMetadataCreateSideData` | `av_content_light_metadata_create_side_data` |
| `MediaDesc.DisplayMatrixFlip` | `av_display_matrix_flip` |
| `MediaDesc.DisplayRotationGet` | `av_display_rotation_get` |
| `MediaDesc.DisplayRotationSet` | `av_display_rotation_set` |
| `MediaDesc.DoviAlloc` | `av_dovi_alloc` |
| `MediaDesc.DoviFindLevel` | `av_dovi_find_level` |
| `MediaDesc.DoviMetadataAlloc` | `av_dovi_metadata_alloc` |
| `MediaDesc.DynamicHdrPlusAlloc` | `av_dynamic_hdr_plus_alloc` |
| `MediaDesc.DynamicHdrPlusCreateSideData` | `av_dynamic_hdr_plus_create_side_data` |
| `MediaDesc.DynamicHdrPlusFromT35` | `av_dynamic_hdr_plus_from_t35` |
| `MediaDesc.DynamicHdrPlusToT35` | `av_dynamic_hdr_plus_to_t35` |
| `MediaDesc.FilmGrainParamsAlloc` | `av_film_grain_params_alloc` |
| `MediaDesc.FilmGrainParamsCreateSideData` | `av_film_grain_params_create_side_data` |
| `MediaDesc.FilmGrainParamsSelect` | `av_film_grain_params_select` |
| `MediaDesc.GetBitsPerPixel` | `av_get_bits_per_pixel` |
| `MediaDesc.GetPixFmt` | `av_get_pix_fmt` |
| `MediaDesc.GetPixFmtLoss` | `av_get_pix_fmt_loss` |
| `MediaDesc.GetPixFmtName` | `av_get_pix_fmt_name` |
| `MediaDesc.GetPixFmtString` | `av_get_pix_fmt_string` |
| `MediaDesc.MasteringDisplayMetadataAlloc` | `av_mastering_display_metadata_alloc` |
| `MediaDesc.MasteringDisplayMetadataAllocSize` | `av_mastering_display_metadata_alloc_size` |
| `MediaDesc.MasteringDisplayMetadataCreateSideData` | `av_mastering_display_metadata_create_side_data` |
| `MediaDesc.ParseColor` | `av_parse_color` |
| `MediaDesc.ParseRatio` | `av_parse_ratio` |
| `MediaDesc.ParseTime` | `av_parse_time` |
| `MediaDesc.ParseVideoRate` | `av_parse_video_rate` |
| `MediaDesc.ParseVideoSize` | `av_parse_video_size` |
| `MediaDesc.PixFmtDescGet` | `av_pix_fmt_desc_get` |
| `MediaDesc.PixFmtDescGetId` | `av_pix_fmt_desc_get_id` |
| `MediaDesc.PixFmtDescNext` | `av_pix_fmt_desc_next` |
| `MediaDesc.SphericalAlloc` | `av_spherical_alloc` |
| `MediaDesc.SphericalFromName` | `av_spherical_from_name` |
| `MediaDesc.SphericalProjectionName` | `av_spherical_projection_name` |
| `MediaDesc.SphericalTileBounds` | `av_spherical_tile_bounds` |
| `MediaDesc.Stereo3dAlloc` | `av_stereo3d_alloc` |
| `MediaDesc.Stereo3dAllocSize` | `av_stereo3d_alloc_size` |
| `MediaDesc.Stereo3dCreateSideData` | `av_stereo3d_create_side_data` |
| `MediaDesc.Stereo3dFromName` | `av_stereo3d_from_name` |
| `MediaDesc.Stereo3dPrimaryEyeFromName` | `av_stereo3d_primary_eye_from_name` |
| `MediaDesc.Stereo3dPrimaryEyeName` | `av_stereo3d_primary_eye_name` |
| `MediaDesc.Stereo3dTypeName` | `av_stereo3d_type_name` |
| `MediaDesc.Stereo3dViewFromName` | `av_stereo3d_view_from_name` |
| `MediaDesc.Stereo3dViewName` | `av_stereo3d_view_name` |
| `MediaDesc.TimecodeAdjustNtscFramenum2` | `av_timecode_adjust_ntsc_framenum2` |
| `MediaDesc.TimecodeCheckFrameRate` | `av_timecode_check_frame_rate` |
| `MediaDesc.TimecodeGetSmpte` | `av_timecode_get_smpte` |
| `MediaDesc.TimecodeGetSmpteFromFramenum` | `av_timecode_get_smpte_from_framenum` |
| `MediaDesc.TimecodeInit` | `av_timecode_init` |
| `MediaDesc.TimecodeInitFromComponents` | `av_timecode_init_from_components` |
| `MediaDesc.TimecodeInitFromString` | `av_timecode_init_from_string` |
| `MediaDesc.TimecodeMakeMpegTcString` | `av_timecode_make_mpeg_tc_string` |
| `MediaDesc.TimecodeMakeSmpteTcString` | `av_timecode_make_smpte_tc_string` |
| `MediaDesc.TimecodeMakeSmpteTcString2` | `av_timecode_make_smpte_tc_string2` |
| `MediaDesc.TimecodeMakeString` | `av_timecode_make_string` |
</details>
### crypto_hash_misc — 杂项工具箱（395 个）

杂项工具箱：硬解设备、哈希校验、写文件、猜格式、零碎小计算。按需直调，用哪个拿哪个。枚举参数同样是 `int32`（比如硬件类型、像素格式）。

抱着的结构体：`HWDevice`（硬解设备）、`Crypto`（哈希校验）、`Muxer`（写文件）、`Prober`（猜格式）、`Samples`（采样帮手）、`Util`（零碎小函数）。

<details>
<summary>点开看全部 API（Go 入口 → ffmpeg 函数）</summary>

| Go 入口 | ffmpeg 函数 |
|---------|-------------|
| `Util.Ac3ParseHeader` | `av_ac3_parse_header` |
| `Util.AdtsHeaderParse` | `av_adts_header_parse` |
| `Crypto.Adler32Update` | `av_adler32_update` |
| `Crypto.AesAlloc` | `av_aes_alloc` |
| `Crypto.AesCrypt` | `av_aes_crypt` |
| `Crypto.AesCtrAlloc` | `av_aes_ctr_alloc` |
| `Crypto.AesCtrCrypt` | `av_aes_ctr_crypt` |
| `Crypto.AesCtrFree` | `av_aes_ctr_free` |
| `Crypto.AesCtrGetIv` | `av_aes_ctr_get_iv` |
| `Crypto.AesCtrIncrementIv` | `av_aes_ctr_increment_iv` |
| `Crypto.AesCtrInit` | `av_aes_ctr_init` |
| `Crypto.AesCtrSetFullIv` | `av_aes_ctr_set_full_iv` |
| `Crypto.AesCtrSetIv` | `av_aes_ctr_set_iv` |
| `Crypto.AesCtrSetRandomIv` | `av_aes_ctr_set_random_iv` |
| `Crypto.AesInit` | `av_aes_init` |
| `HWDevice.AllocVdpaucontext` | `av_alloc_vdpaucontext` |
| `Util.AppendPacket` | `av_append_packet` |
| `Util.AppendPathComponent` | `av_append_path_component` |
| `Util.Assert0Fpu` | `av_assert0_fpu` |
| `Crypto.Base64Decode` | `av_base64_decode` |
| `Crypto.Base64Encode` | `av_base64_encode` |
| `Util.Basename` | `av_basename` |
| `Util.BesselI0` | `av_bessel_i0` |
| `Crypto.BlowfishAlloc` | `av_blowfish_alloc` |
| `Crypto.BlowfishCrypt` | `av_blowfish_crypt` |
| `Crypto.BlowfishCryptEcb` | `av_blowfish_crypt_ecb` |
| `Crypto.BlowfishInit` | `av_blowfish_init` |
| `Util.BmgGet` | `av_bmg_get` |
| `Util.Bprintf` | `av_bprintf` |
| `Util.Calloc` | `av_calloc` |
| `Crypto.CamelliaAlloc` | `av_camellia_alloc` |
| `Crypto.CamelliaCrypt` | `av_camellia_crypt` |
| `Crypto.CamelliaInit` | `av_camellia_init` |
| `Crypto.Cast5Alloc` | `av_cast5_alloc` |
| `Crypto.Cast5Crypt` | `av_cast5_crypt` |
| `Crypto.Cast5Crypt2` | `av_cast5_crypt2` |
| `Crypto.Cast5Init` | `av_cast5_init` |
| `Util.ChromaLocationEnumToPos` | `av_chroma_location_enum_to_pos` |
| `Util.ChromaLocationFromName` | `av_chroma_location_from_name` |
| `Util.ChromaLocationName` | `av_chroma_location_name` |
| `Util.ChromaLocationPosToEnum` | `av_chroma_location_pos_to_enum` |
| `Util.CmpI` | `av_cmp_i` |
| `Util.CpbPropertiesAlloc` | `av_cpb_properties_alloc` |
| `Crypto.CrcGetTable` | `av_crc_get_table` |
| `Crypto.Crc` | `av_crc` |
| `Crypto.CrcInit` | `av_crc_init` |
| `Util.CspApproximateTrcGamma` | `av_csp_approximate_trc_gamma` |
| `Util.CspLumaCoeffsFromAvcsp` | `av_csp_luma_coeffs_from_avcsp` |
| `Util.CspPrimariesDescFromId` | `av_csp_primaries_desc_from_id` |
| `Util.CspPrimariesIdFromDesc` | `av_csp_primaries_id_from_desc` |
| `Util.CspTrcFuncFromId` | `av_csp_trc_func_from_id` |
| `Util.D2q` | `av_d2q` |
| `Util.D3d11vaAllocContext` | `av_d3d11va_alloc_context` |
| `Util.DctCalc` | `av_dct_calc` |
| `Util.DctEnd` | `av_dct_end` |
| `Util.DctInit` | `av_dct_init` |
| `Util.DefaultGetCategory` | `av_default_get_category` |
| `Util.DefaultItemName` | `av_default_item_name` |
| `Util.DemuxerIterate` | `av_demuxer_iterate` |
| `Crypto.DesAlloc` | `av_des_alloc` |
| `Crypto.DesCrypt` | `av_des_crypt` |
| `Crypto.DesInit` | `av_des_init` |
| `Crypto.DesMac` | `av_des_mac` |
| `Util.DetectionBboxAlloc` | `av_detection_bbox_alloc` |
| `Util.DetectionBboxCreateSideData` | `av_detection_bbox_create_side_data` |
| `Util.DiracParseSequenceHeader` | `av_dirac_parse_sequence_header` |
| `Util.Dirname` | `av_dirname` |
| `Util.DispositionFromString` | `av_disposition_from_string` |
| `Util.DispositionToString` | `av_disposition_to_string` |
| `Util.DivI` | `av_div_i` |
| `Util.DivQ` | `av_div_q` |
| `Util.DownmixInfoUpdateSideData` | `av_downmix_info_update_side_data` |
| `Util.DvCodecProfile` | `av_dv_codec_profile` |
| `Util.DvCodecProfile2` | `av_dv_codec_profile2` |
| `Util.DvFrameProfile` | `av_dv_frame_profile` |
| `Util.Dynarray2Add` | `av_dynarray2_add` |
| `Util.DynarrayAdd` | `av_dynarray_add` |
| `Util.DynarrayAddNofree` | `av_dynarray_add_nofree` |
| `Util.EncryptionInfoAddSideData` | `av_encryption_info_add_side_data` |
| `Util.EncryptionInfoAlloc` | `av_encryption_info_alloc` |
| `Util.EncryptionInfoClone` | `av_encryption_info_clone` |
| `Util.EncryptionInfoFree` | `av_encryption_info_free` |
| `Util.EncryptionInfoGetSideData` | `av_encryption_info_get_side_data` |
| `Util.EncryptionInitInfoAddSideData` | `av_encryption_init_info_add_side_data` |
| `Util.EncryptionInitInfoAlloc` | `av_encryption_init_info_alloc` |
| `Util.EncryptionInitInfoFree` | `av_encryption_init_info_free` |
| `Util.EncryptionInitInfoGetSideData` | `av_encryption_init_info_get_side_data` |
| `Util.Escape` | `av_escape` |
| `Util.ExecutorAlloc` | `av_executor_alloc` |
| `Util.ExecutorExecute` | `av_executor_execute` |
| `Util.ExecutorFree` | `av_executor_free` |
| `Util.ExprCountFunc` | `av_expr_count_func` |
| `Util.ExprCountVars` | `av_expr_count_vars` |
| `Util.ExprEval` | `av_expr_eval` |
| `Util.ExprFree` | `av_expr_free` |
| `Util.ExprParse` | `av_expr_parse` |
| `Util.ExprParseAndEval` | `av_expr_parse_and_eval` |
| `Util.FastMalloc` | `av_fast_malloc` |
| `Util.FastMallocz` | `av_fast_mallocz` |
| `Util.FastPaddedMalloc` | `av_fast_padded_malloc` |
| `Util.FastPaddedMallocz` | `av_fast_padded_mallocz` |
| `Util.FastRealloc` | `av_fast_realloc` |
| `Util.FftCalc` | `av_fft_calc` |
| `Util.FftEnd` | `av_fft_end` |
| `Util.FftInit` | `av_fft_init` |
| `Util.FftPermute` | `av_fft_permute` |
| `Util.FileMap` | `av_file_map` |
| `Util.FilenameNumberTest` | `av_filename_number_test` |
| `Util.FileUnmap` | `av_file_unmap` |
| `Util.FilterIterate` | `av_filter_iterate` |
| `Prober.FindBestPixFmtOf2` | `av_find_best_pix_fmt_of_2` |
| `Util.FindDefaultStreamIndex` | `av_find_default_stream_index` |
| `Util.FindInfoTag` | `av_find_info_tag` |
| `Prober.FindInputFormat` | `av_find_input_format` |
| `Util.FindNearestQIdx` | `av_find_nearest_q_idx` |
| `Util.FindProgramFromStream` | `av_find_program_from_stream` |
| `Util.FmtCtxGetDurationEstimationMethod` | `av_fmt_ctx_get_duration_estimation_method` |
| `Util.ForceCpuFlags` | `av_force_cpu_flags` |
| `Util.FormatInjectGlobalSideData` | `av_format_inject_global_side_data` |
| `Util.FourccMakeString` | `av_fourcc_make_string` |
| `Util.Gcd` | `av_gcd` |
| `Util.GcdQ` | `av_gcd_q` |
| `Util.GetAltSampleFmt` | `av_get_alt_sample_fmt` |
| `Samples.GetAudioFrameDuration` | `av_get_audio_frame_duration` |
| `Samples.GetAudioFrameDuration2` | `av_get_audio_frame_duration2` |
| `Util.GetBitsPerSample` | `av_get_bits_per_sample` |
| `Util.GetCpuFlags` | `av_get_cpu_flags` |
| `Util.GetExactBitsPerSample` | `av_get_exact_bits_per_sample` |
| `Util.GetFrameFilename` | `av_get_frame_filename` |
| `Util.GetFrameFilename2` | `av_get_frame_filename2` |
| `Util.GetKnownColorName` | `av_get_known_color_name` |
| `Util.GetMediaTypeString` | `av_get_media_type_string` |
| `Util.GetOutputTimestamp` | `av_get_output_timestamp` |
| `Util.GetPacket` | `av_get_packet` |
| `Util.GetPaddedBitsPerPixel` | `av_get_padded_bits_per_pixel` |
| `Util.GetPcmCodec` | `av_get_pcm_codec` |
| `Util.GetPictureTypeChar` | `av_get_picture_type_char` |
| `Util.GetProfileName` | `av_get_profile_name` |
| `Util.GetRandomSeed` | `av_get_random_seed` |
| `Util.GetTimeBaseQ` | `av_get_time_base_q` |
| `Util.GetToken` | `av_get_token` |
| `Crypto.HashAlloc` | `av_hash_alloc` |
| `Crypto.HashFinal` | `av_hash_final` |
| `Crypto.HashFinalB64` | `av_hash_final_b64` |
| `Crypto.HashFinalBin` | `av_hash_final_bin` |
| `Crypto.HashFinalHex` | `av_hash_final_hex` |
| `Crypto.HashFreep` | `av_hash_freep` |
| `Crypto.HashGetName` | `av_hash_get_name` |
| `Crypto.HashGetSize` | `av_hash_get_size` |
| `Crypto.HashInit` | `av_hash_init` |
| `Crypto.HashNames` | `av_hash_names` |
| `Crypto.HashUpdate` | `av_hash_update` |
| `Util.HexDump` | `av_hex_dump` |
| `Util.HexDumpLog` | `av_hex_dump_log` |
| `Crypto.HmacAlloc` | `av_hmac_alloc` |
| `Crypto.HmacCalc` | `av_hmac_calc` |
| `Crypto.HmacFinal` | `av_hmac_final` |
| `Crypto.HmacFree` | `av_hmac_free` |
| `Crypto.HmacInit` | `av_hmac_init` |
| `Crypto.HmacUpdate` | `av_hmac_update` |
| `HWDevice.HwdeviceCtxAlloc` | `av_hwdevice_ctx_alloc` |
| `HWDevice.HwdeviceCtxCreate` | `av_hwdevice_ctx_create` |
| `HWDevice.HwdeviceCtxCreateDerived` | `av_hwdevice_ctx_create_derived` |
| `HWDevice.HwdeviceCtxCreateDerivedOpts` | `av_hwdevice_ctx_create_derived_opts` |
| `HWDevice.HwdeviceCtxInit` | `av_hwdevice_ctx_init` |
| `HWDevice.HwdeviceFindTypeByName` | `av_hwdevice_find_type_by_name` |
| `HWDevice.HwdeviceGetHwframeConstraints` | `av_hwdevice_get_hwframe_constraints` |
| `HWDevice.HwdeviceGetTypeName` | `av_hwdevice_get_type_name` |
| `HWDevice.HwdeviceHwconfigAlloc` | `av_hwdevice_hwconfig_alloc` |
| `HWDevice.HwdeviceIterateTypes` | `av_hwdevice_iterate_types` |
| `HWDevice.HwframeConstraintsFree` | `av_hwframe_constraints_free` |
| `HWDevice.HwframeCtxAlloc` | `av_hwframe_ctx_alloc` |
| `HWDevice.HwframeCtxCreateDerived` | `av_hwframe_ctx_create_derived` |
| `HWDevice.HwframeCtxInit` | `av_hwframe_ctx_init` |
| `HWDevice.HwframeGetBuffer` | `av_hwframe_get_buffer` |
| `HWDevice.HwframeMap` | `av_hwframe_map` |
| `HWDevice.HwframeTransferData` | `av_hwframe_transfer_data` |
| `HWDevice.HwframeTransferGetFormats` | `av_hwframe_transfer_get_formats` |
| `Util.I2int` | `av_i2int` |
| `Util.IamfAudioElementAddLayer` | `av_iamf_audio_element_add_layer` |
| `Util.IamfAudioElementAlloc` | `av_iamf_audio_element_alloc` |
| `Util.IamfAudioElementFree` | `av_iamf_audio_element_free` |
| `Util.IamfAudioElementGetClass` | `av_iamf_audio_element_get_class` |
| `Util.IamfMixPresentationAddSubmix` | `av_iamf_mix_presentation_add_submix` |
| `Util.IamfMixPresentationAlloc` | `av_iamf_mix_presentation_alloc` |
| `Util.IamfMixPresentationFree` | `av_iamf_mix_presentation_free` |
| `Util.IamfMixPresentationGetClass` | `av_iamf_mix_presentation_get_class` |
| `Util.IamfParamDefinitionAlloc` | `av_iamf_param_definition_alloc` |
| `Util.IamfParamDefinitionGetClass` | `av_iamf_param_definition_get_class` |
| `Util.IamfSubmixAddElement` | `av_iamf_submix_add_element` |
| `Util.IamfSubmixAddLayout` | `av_iamf_submix_add_layout` |
| `Util.ImdctCalc` | `av_imdct_calc` |
| `Util.ImdctHalf` | `av_imdct_half` |
| `Util.InitPacket` | `av_init_packet` |
| `Util.InputAudioDeviceNext` | `av_input_audio_device_next` |
| `Util.InputVideoDeviceNext` | `av_input_video_device_next` |
| `Util.Int2i` | `av_int2i` |
| `Util.IntListLengthForSize` | `av_int_list_length_for_size` |
| `Muxer.InterleavedWriteFrame` | `av_interleaved_write_frame` |
| `Muxer.InterleavedWriteUncodedFrame` | `av_interleaved_write_uncoded_frame` |
| `Util.JniGetJavaVm` | `av_jni_get_java_vm` |
| `Util.JniSetJavaVm` | `av_jni_set_java_vm` |
| `Util.LfgInit` | `av_lfg_init` |
| `Util.LfgInitFromData` | `av_lfg_init_from_data` |
| `Util.Log` | `av_log` |
| `Util.Lzo1xDecode` | `av_lzo1x_decode` |
| `Util.MatchExt` | `av_match_ext` |
| `Util.MatchList` | `av_match_list` |
| `Util.MatchName` | `av_match_name` |
| `Util.MaxAlloc` | `av_max_alloc` |
| `Crypto.Md5Alloc` | `av_md5_alloc` |
| `Crypto.Md5Final` | `av_md5_final` |
| `Crypto.Md5Init` | `av_md5_init` |
| `Crypto.Md5Sum` | `av_md5_sum` |
| `Crypto.Md5Update` | `av_md5_update` |
| `Util.MdctCalc` | `av_mdct_calc` |
| `Util.MdctEnd` | `av_mdct_end` |
| `Util.MdctInit` | `av_mdct_init` |
| `HWDevice.MediacodecAllocContext` | `av_mediacodec_alloc_context` |
| `HWDevice.MediacodecDefaultFree` | `av_mediacodec_default_free` |
| `HWDevice.MediacodecDefaultInit` | `av_mediacodec_default_init` |
| `HWDevice.MediacodecReleaseBuffer` | `av_mediacodec_release_buffer` |
| `HWDevice.MediacodecRenderBufferAtTime` | `av_mediacodec_render_buffer_at_time` |
| `Util.ModI` | `av_mod_i` |
| `Util.MulI` | `av_mul_i` |
| `Util.MulQ` | `av_mul_q` |
| `Crypto.Murmur3Alloc` | `av_murmur3_alloc` |
| `Crypto.Murmur3Final` | `av_murmur3_final` |
| `Crypto.Murmur3Init` | `av_murmur3_init` |
| `Crypto.Murmur3InitSeeded` | `av_murmur3_init_seeded` |
| `Crypto.Murmur3Update` | `av_murmur3_update` |
| `Util.MuxerIterate` | `av_muxer_iterate` |
| `Util.NearerQ` | `av_nearer_q` |
| `Util.NewProgram` | `av_new_program` |
| `Util.OutputAudioDeviceNext` | `av_output_audio_device_next` |
| `Util.OutputVideoDeviceNext` | `av_output_video_device_next` |
| `Util.PixelutilsGetSadFn` | `av_pixelutils_get_sad_fn` |
| `Util.PktDump2` | `av_pkt_dump2` |
| `Util.PktDumpLog2` | `av_pkt_dump_log2` |
| `Prober.ProbeInputBuffer` | `av_probe_input_buffer` |
| `Prober.ProbeInputBuffer2` | `av_probe_input_buffer2` |
| `Prober.ProbeInputFormat` | `av_probe_input_format` |
| `Prober.ProbeInputFormat2` | `av_probe_input_format2` |
| `Prober.ProbeInputFormat3` | `av_probe_input_format3` |
| `Util.ProgramAddStreamIndex` | `av_program_add_stream_index` |
| `Util.Q2intfloat` | `av_q2intfloat` |
| `Util.QsvAllocContext` | `av_qsv_alloc_context` |
| `Util.RandomBytes` | `av_random_bytes` |
| `Crypto.Rc4Alloc` | `av_rc4_alloc` |
| `Crypto.Rc4Crypt` | `av_rc4_crypt` |
| `Crypto.Rc4Init` | `av_rc4_init` |
| `Util.RdftCalc` | `av_rdft_calc` |
| `Util.RdftEnd` | `av_rdft_end` |
| `Util.RdftInit` | `av_rdft_init` |
| `Util.ReadImageLine` | `av_read_image_line` |
| `Util.ReadImageLine2` | `av_read_image_line2` |
| `Util.ReadPause` | `av_read_pause` |
| `Util.ReadPlay` | `av_read_play` |
| `Util.Reduce` | `av_reduce` |
| `Crypto.RipemdAlloc` | `av_ripemd_alloc` |
| `Crypto.RipemdFinal` | `av_ripemd_final` |
| `Crypto.RipemdInit` | `av_ripemd_init` |
| `Crypto.RipemdUpdate` | `av_ripemd_update` |
| `Samples.SamplesAlloc` | `av_samples_alloc` |
| `Samples.SamplesAllocArrayAndSamples` | `av_samples_alloc_array_and_samples` |
| `Samples.SamplesCopy` | `av_samples_copy` |
| `Samples.SamplesFillArrays` | `av_samples_fill_arrays` |
| `Samples.SamplesGetBufferSize` | `av_samples_get_buffer_size` |
| `Samples.SamplesSetSilence` | `av_samples_set_silence` |
| `Util.SdpCreate` | `av_sdp_create` |
| `Util.SetOptionsString` | `av_set_options_string` |
| `Crypto.Sha512Alloc` | `av_sha512_alloc` |
| `Crypto.Sha512Final` | `av_sha512_final` |
| `Crypto.Sha512Init` | `av_sha512_init` |
| `Crypto.Sha512Update` | `av_sha512_update` |
| `Crypto.ShaAlloc` | `av_sha_alloc` |
| `Crypto.ShaFinal` | `av_sha_final` |
| `Crypto.ShaInit` | `av_sha_init` |
| `Crypto.ShaUpdate` | `av_sha_update` |
| `Util.ShrI` | `av_shr_i` |
| `Util.SizeMult` | `av_size_mult` |
| `Util.SmallStrptime` | `av_small_strptime` |
| `Util.Sscanf` | `av_sscanf` |
| `Util.Strcasecmp` | `av_strcasecmp` |
| `Util.Strdup` | `av_strdup` |
| `Util.StrNDup` | `av_strndup` |
| `Util.StreamAddSideData` | `av_stream_add_side_data` |
| `Util.StreamGetClass` | `av_stream_get_class` |
| `Util.StreamGetCodecTimebase` | `av_stream_get_codec_timebase` |
| `Util.StreamGetParser` | `av_stream_get_parser` |
| `Util.StreamGetSideData` | `av_stream_get_side_data` |
| `Util.StreamGroupGetClass` | `av_stream_group_get_class` |
| `Util.StreamNewSideData` | `av_stream_new_side_data` |
| `Util.Strireplace` | `av_strireplace` |
| `Util.Stristart` | `av_stristart` |
| `Util.Stristr` | `av_stristr` |
| `Util.Strlcat` | `av_strlcat` |
| `Util.Strlcpy` | `av_strlcpy` |
| `Util.Strncasecmp` | `av_strncasecmp` |
| `Util.Strnstr` | `av_strnstr` |
| `Util.Strstart` | `av_strstart` |
| `Util.Strtod` | `av_strtod` |
| `Util.Strtok` | `av_strtok` |
| `Util.SubI` | `av_sub_i` |
| `Util.SubQ` | `av_sub_q` |
| `Crypto.TeaAlloc` | `av_tea_alloc` |
| `Crypto.TeaCrypt` | `av_tea_crypt` |
| `Crypto.TeaInit` | `av_tea_init` |
| `Util.ThreadMessageFlush` | `av_thread_message_flush` |
| `Util.ThreadMessageQueueAlloc` | `av_thread_message_queue_alloc` |
| `Util.ThreadMessageQueueFree` | `av_thread_message_queue_free` |
| `Util.ThreadMessageQueueNbElems` | `av_thread_message_queue_nb_elems` |
| `Util.ThreadMessageQueueRecv` | `av_thread_message_queue_recv` |
| `Util.ThreadMessageQueueSend` | `av_thread_message_queue_send` |
| `Util.ThreadMessageQueueSetErrRecv` | `av_thread_message_queue_set_err_recv` |
| `Util.ThreadMessageQueueSetErrSend` | `av_thread_message_queue_set_err_send` |
| `Util.ThreadMessageQueueSetFreeFunc` | `av_thread_message_queue_set_free_func` |
| `Util.Timegm` | `av_timegm` |
| `Util.TreeDestroy` | `av_tree_destroy` |
| `Util.TreeEnumerate` | `av_tree_enumerate` |
| `Util.TreeFind` | `av_tree_find` |
| `Util.TreeInsert` | `av_tree_insert` |
| `Util.TreeNodeAlloc` | `av_tree_node_alloc` |
| `Util.TsMakeTimeString2` | `av_ts_make_time_string2` |
| `Crypto.TwofishAlloc` | `av_twofish_alloc` |
| `Crypto.TwofishCrypt` | `av_twofish_crypt` |
| `Crypto.TwofishInit` | `av_twofish_init` |
| `Util.TxInit` | `av_tx_init` |
| `Util.TxUninit` | `av_tx_uninit` |
| `Util.Utf8Decode` | `av_utf8_decode` |
| `Util.UuidParse` | `av_uuid_parse` |
| `Util.UuidParseRange` | `av_uuid_parse_range` |
| `Util.UuidUnparse` | `av_uuid_unparse` |
| `Util.UuidUrnParse` | `av_uuid_urn_parse` |
| `Util.Vbprintf` | `av_vbprintf` |
| `HWDevice.VdpauAllocContext` | `av_vdpau_alloc_context` |
| `HWDevice.VdpauBindContext` | `av_vdpau_bind_context` |
| `HWDevice.VdpauGetSurfaceParameters` | `av_vdpau_get_surface_parameters` |
| `HWDevice.VdpauHwaccelGetRender2` | `av_vdpau_hwaccel_get_render2` |
| `HWDevice.VdpauHwaccelSetRender2` | `av_vdpau_hwaccel_set_render2` |
| `Util.VideoEncParamsAlloc` | `av_video_enc_params_alloc` |
| `Util.VideoEncParamsCreateSideData` | `av_video_enc_params_create_side_data` |
| `Util.VideoHintAlloc` | `av_video_hint_alloc` |
| `Util.VideoHintCreateSideData` | `av_video_hint_create_side_data` |
| `Util.VkfmtFromPixfmt` | `av_vkfmt_from_pixfmt` |
| `Util.VkFrameAlloc` | `av_vk_frame_alloc` |
| `Util.Vlog` | `av_vlog` |
| `Util.VorbisParseFrame` | `av_vorbis_parse_frame` |
| `Util.VorbisParseFrameFlags` | `av_vorbis_parse_frame_flags` |
| `Util.VorbisParseFree` | `av_vorbis_parse_free` |
| `Util.VorbisParseInit` | `av_vorbis_parse_init` |
| `Util.VorbisParseReset` | `av_vorbis_parse_reset` |
| `Muxer.WriteFrame` | `av_write_frame` |
| `Muxer.WriteImageLine` | `av_write_image_line` |
| `Muxer.WriteImageLine2` | `av_write_image_line2` |
| `Muxer.WriteTrailer` | `av_write_trailer` |
| `Muxer.WriteUncodedFrame` | `av_write_uncoded_frame` |
| `Muxer.WriteUncodedFrameQuery` | `av_write_uncoded_frame_query` |
| `Util.Xiphlacing` | `av_xiphlacing` |
| `Crypto.XteaAlloc` | `av_xtea_alloc` |
| `Crypto.XteaCrypt` | `av_xtea_crypt` |
| `Crypto.XteaInit` | `av_xtea_init` |
| `Crypto.XteaLeCrypt` | `av_xtea_le_crypt` |
| `Crypto.XteaLeInit` | `av_xtea_le_init` |
| `Util.SwresampleConfiguration` | `swresample_configuration` |
| `Util.SwresampleLicense` | `swresample_license` |
| `Util.SwresampleVersion` | `swresample_version` |
| `Util.SwriAudioConvert` | `swri_audio_convert` |
| `Util.SwriAudioConvertAlloc` | `swri_audio_convert_alloc` |
| `Util.SwriAudioConvertFree` | `swri_audio_convert_free` |
| `Util.SwriResampleDspInit` | `swri_resample_dsp_init` |
| `Util.SwriResampleDspX86Init` | `swri_resample_dsp_x86_init` |
| `Util.AddI` | `av_add_i` |
| `Util.DynamicHdrVividAlloc` | `av_dynamic_hdr_vivid_alloc` |
| `Util.DynamicHdrVividCreateSideData` | `av_dynamic_hdr_vivid_create_side_data` |
| `Util.AvformatTransferInternalStreamTimingInfo` | `avformat_transfer_internal_stream_timing_info` |
| `Util.ImageCopy` | `av_image_copy` |
| `Util.ImageCopyPlaneUcFrom` | `av_image_copy_plane_uc_from` |
| `Util.ImageCopyToBuffer` | `av_image_copy_to_buffer` |
| `Util.ImageCopyUcFrom` | `av_image_copy_uc_from` |
| `Util.ImageFillArrays` | `av_image_fill_arrays` |
| `Util.ImageFillBlack` | `av_image_fill_black` |
| `Util.ImageFillColor` | `av_image_fill_color` |
| `Util.ImageFillLinesizes` | `av_image_fill_linesizes` |
| `Util.ImageFillMaxPixsteps` | `av_image_fill_max_pixsteps` |
| `Util.ImageFillPlaneSizes` | `av_image_fill_plane_sizes` |
| `Util.ImageFillPointers` | `av_image_fill_pointers` |
| `Util.ImageGetLinesize` | `av_image_get_linesize` |
| `Util.Log2` | `av_log2` |
| `Util.Log216bit` | `av_log2_16bit` |
| `Util.Log2I` | `av_log2_i` |
| `Util.ParseCpuCaps` | `av_parse_cpu_caps` |
| `Util.PixFmtCountPlanes` | `av_pix_fmt_count_planes` |
| `Util.PixFmtGetChromaSubSample` | `av_pix_fmt_get_chroma_sub_sample` |
| `Util.PixFmtSwapEndianness` | `av_pix_fmt_swap_endianness` |
</details>

### data_const — 数据常量（16 个）

数据常量：so 里剩下 16 个数据符号，全是只读数据，不是函数。10 个上下文大小走 `Dlsym` 读数字，6 个版本串走 `Dlsym` 读字符串。函数用 `RegisterLibFunc` 绑，数据用 `Dlsym` 取地址再读（`TestDataConst` 全钉死）。

抱着的结构体：`Crypto`（9 个大小）、`Util`（1 个大小）、`Library`（6 个版本串）。

<details>
<summary>点开看全部 API（Go 入口 → 数据符号）</summary>

| Go 入口 | 数据符号 |
|---------|-------------|
| `Crypto.AesSize` | `av_aes_size` |
| `Crypto.CamelliaSize` | `av_camellia_size` |
| `Crypto.Cast5Size` | `av_cast5_size` |
| `Crypto.TeaSize` | `av_tea_size` |
| `Crypto.TwofishSize` | `av_twofish_size` |
| `Crypto.Md5Size` | `av_md5_size` |
| `Crypto.RipemdSize` | `av_ripemd_size` |
| `Crypto.ShaSize` | `av_sha_size` |
| `Crypto.Sha512Size` | `av_sha512_size` |
| `Util.TreeNodeSize` | `av_tree_node_size` |
| `Library.CodecFfversion` | `av_codec_ffversion` |
| `Library.DeviceFfversion` | `av_device_ffversion` |
| `Library.FilterFfversion` | `av_filter_ffversion` |
| `Library.FormatFfversion` | `av_format_ffversion` |
| `Library.UtilFfversion` | `av_util_ffversion` |
| `Library.SwrFfversion` | `swr_ffversion` |
</details>
