package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// FormatDemux 解复用模块: AVFormat/AVStream/AVIO 全量导出, 结构体方法直接可用.
//
// Say it plain: 从文件或网路上把盒子打开, 找到音视频流, 一包一包读出来,
// 跳进度也走这里. 上层播视频先从这进门.
// FormatContext owns one AVFormatContext* (nil-safe, 记得 CloseInput/FreeContext).
type FormatContext struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *FormatContext) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// Stream owns one AVStream* (跟着 FormatContext 走, 不单独释放).
type Stream struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *Stream) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// IOContext owns one AVIOContext* (nil-safe).
type IOContext struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *IOContext) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// Format owns one AVInputFormat*/AVOutputFormat* (跟着库走, 不释放).
type Format struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (x *Format) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvAddIndexEntry                    func(st unsafe.Pointer, pos int64, timestamp int64, size int32, distance int32, flags int32) int32
	fAvDumpFormat                       func(ic unsafe.Pointer, index int32, url unsafe.Pointer, is_output int32)
	fAvFindBestStream                   func(ic unsafe.Pointer, typ unsafe.Pointer, wanted_stream_nb int32, related_stream int32, decoder_ret *unsafe.Pointer, flags int32) int32
	fAvformatAllocContext               func() unsafe.Pointer
	fAvformatAllocOutputContext2        func(ctx *unsafe.Pointer, oformat unsafe.Pointer, format_name unsafe.Pointer, filename unsafe.Pointer) int32
	fAvformatCloseInput                 func(s *unsafe.Pointer)
	fAvformatConfiguration              func() unsafe.Pointer
	fAvformatFindStreamInfo             func(ic unsafe.Pointer, options *unsafe.Pointer) int32
	fAvformatFlush                      func(s unsafe.Pointer) int32
	fAvformatFreeContext                func(s unsafe.Pointer)
	fAvformatGetClass                   func() unsafe.Pointer
	fAvformatGetMovAudioTags            func() unsafe.Pointer
	fAvformatGetMovVideoTags            func() unsafe.Pointer
	fAvformatGetRiffAudioTags           func() unsafe.Pointer
	fAvformatGetRiffVideoTags           func() unsafe.Pointer
	fAvformatIndexGetEntriesCount       func(st unsafe.Pointer) int32
	fAvformatIndexGetEntry              func(st unsafe.Pointer, idx int32) unsafe.Pointer
	fAvformatIndexGetEntryFromTimestamp func(st unsafe.Pointer, wanted_timestamp int64, flags int32) unsafe.Pointer
	fAvformatInitOutput                 func(s unsafe.Pointer, options *unsafe.Pointer) int32
	fAvformatLicense                    func() unsafe.Pointer
	fAvformatMatchStreamSpecifier       func(s unsafe.Pointer, st unsafe.Pointer, spec unsafe.Pointer) int32
	fAvformatNetworkDeinit              func() int32
	fAvformatNetworkInit                func() int32
	fAvformatNewStream                  func(s unsafe.Pointer, c unsafe.Pointer) unsafe.Pointer
	fAvformatOpenInput                  func(ps *unsafe.Pointer, url unsafe.Pointer, fmt unsafe.Pointer, options *unsafe.Pointer) int32
	fAvformatQueryCodec                 func(ofmt unsafe.Pointer, codec_id unsafe.Pointer, std_compliance int32) int32
	fAvformatQueueAttachedPictures      func(s unsafe.Pointer) int32
	fAvformatSeekFile                   func(s unsafe.Pointer, stream_index int32, min_ts int64, ts int64, max_ts int64, flags int32) int32
	fAvformatStreamGroupAddStream       func(stg unsafe.Pointer, st unsafe.Pointer) int32
	fAvformatStreamGroupCreate          func(s unsafe.Pointer, typ unsafe.Pointer, options *unsafe.Pointer) unsafe.Pointer
	fAvformatStreamGroupName            func(typ unsafe.Pointer) unsafe.Pointer
	fAvformatVersion                    func() unsafe.Pointer
	fAvformatWriteHeader                func(s unsafe.Pointer, options *unsafe.Pointer) unsafe.Pointer
	fAvGuessCodec                       func(fmt unsafe.Pointer, short_name unsafe.Pointer, filename unsafe.Pointer, mime_type unsafe.Pointer, typ unsafe.Pointer) unsafe.Pointer
	fAvGuessFormat                      func(short_name unsafe.Pointer, filename unsafe.Pointer, mime_type unsafe.Pointer) unsafe.Pointer
	fAvGuessFrameRate                   func(ctx unsafe.Pointer, stream unsafe.Pointer, frame unsafe.Pointer) AVRational
	fAvGuessSampleAspectRatio           func(format unsafe.Pointer, stream unsafe.Pointer, frame unsafe.Pointer) AVRational
	fAvIndexSearchTimestamp             func(st unsafe.Pointer, timestamp int64, flags int32) int32
	fAvioAccept                         func(s unsafe.Pointer, c *unsafe.Pointer) int32
	fAvioAllocContext                   func(buffer unsafe.Pointer, buffer_size int32, write_flag int32, opaque unsafe.Pointer) unsafe.Pointer
	fAvioCheck                          func(url unsafe.Pointer, flags int32) int32
	fAvioClose                          func(s unsafe.Pointer) int32
	fAvioCloseDir                       func(s *unsafe.Pointer) int32
	fAvioCloseDynBuf                    func(s unsafe.Pointer, pbuffer *unsafe.Pointer) int32
	fAvioClosep                         func(s *unsafe.Pointer) int32
	fAvioContextFree                    func(s *unsafe.Pointer)
	fAvioEnumProtocols                  func(opaque *unsafe.Pointer, output int32) unsafe.Pointer
	fAvioFeof                           func(s unsafe.Pointer) int32
	fAvioFindProtocolName               func(url unsafe.Pointer) unsafe.Pointer
	fAvioFlush                          func(s unsafe.Pointer) unsafe.Pointer
	fAvioFreeDirectoryEntry             func(entry *unsafe.Pointer)
	fAvioGetDynBuf                      func(s unsafe.Pointer, pbuffer *unsafe.Pointer) int32
	fAvioGetStr                         func(pb unsafe.Pointer, maxlen int32, buf unsafe.Pointer, buflen int32) int32
	fAvioGetStr16be                     func(pb unsafe.Pointer, maxlen int32, buf unsafe.Pointer, buflen int32) int32
	fAvioGetStr16le                     func(pb unsafe.Pointer, maxlen int32, buf unsafe.Pointer, buflen int32) int32
	fAvioHandshake                      func(c unsafe.Pointer) int32
	fAvioOpen                           func(s *unsafe.Pointer, url unsafe.Pointer, flags int32) unsafe.Pointer
	fAvioOpen2                          func(s *unsafe.Pointer, url unsafe.Pointer, flags int32, int_cb unsafe.Pointer, options *unsafe.Pointer) int32
	fAvioOpenDir                        func(s *unsafe.Pointer, url unsafe.Pointer, options *unsafe.Pointer) int32
	fAvioOpenDynBuf                     func(s *unsafe.Pointer) int32
	fAvioPause                          func(h unsafe.Pointer, pause int32) int32
	fAvioPrintStringArray               func(s unsafe.Pointer, strings unsafe.Pointer)
	fAvioProtocolGetClass               func(name unsafe.Pointer) unsafe.Pointer
	fAvioPutStr                         func(s unsafe.Pointer, str unsafe.Pointer) int32
	fAvioPutStr16be                     func(s unsafe.Pointer, str unsafe.Pointer) int32
	fAvioPutStr16le                     func(s unsafe.Pointer, str unsafe.Pointer) int32
	fAvioR8                             func(s unsafe.Pointer) int32
	fAvioRb16                           func(s unsafe.Pointer) uint32
	fAvioRb24                           func(s unsafe.Pointer) uint32
	fAvioRb32                           func(s unsafe.Pointer) uint32
	fAvioRb64                           func(s unsafe.Pointer) uint64
	fAvioRead                           func(s unsafe.Pointer, buf unsafe.Pointer, size int32) int32
	fAvioReadDir                        func(s unsafe.Pointer, next *unsafe.Pointer) int32
	fAvioReadPartial                    func(s unsafe.Pointer, buf unsafe.Pointer, size int32) int32
	fAvioReadToBprint                   func(h unsafe.Pointer, pb unsafe.Pointer, max_size uintptr) int32
	fAvioRl16                           func(s unsafe.Pointer) uint32
	fAvioRl24                           func(s unsafe.Pointer) uint32
	fAvioRl32                           func(s unsafe.Pointer) uint32
	fAvioRl64                           func(s unsafe.Pointer) uint64
	fAvioSeek                           func(s unsafe.Pointer, offset int64, whence int32) unsafe.Pointer
	fAvioSeekTime                       func(h unsafe.Pointer, stream_index int32, timestamp int64, flags int32) int64
	fAvioSize                           func(s unsafe.Pointer) int64
	fAvioSkip                           func(s unsafe.Pointer, offset int64) int64
	fAvioVprintf                        func(s unsafe.Pointer, fmt unsafe.Pointer, ap unsafe.Pointer) int32
	fAvioW8                             func(s unsafe.Pointer, b int32)
	fAvioWb16                           func(s unsafe.Pointer, val uint32)
	fAvioWb24                           func(s unsafe.Pointer, val uint32)
	fAvioWb32                           func(s unsafe.Pointer, val uint32)
	fAvioWb64                           func(s unsafe.Pointer, val uint64)
	fAvioWl16                           func(s unsafe.Pointer, val uint32)
	fAvioWl24                           func(s unsafe.Pointer, val uint32)
	fAvioWl32                           func(s unsafe.Pointer, val uint32)
	fAvioWl64                           func(s unsafe.Pointer, val uint64)
	fAvioWrite                          func(s unsafe.Pointer, buf unsafe.Pointer, size int32)
	fAvioWriteMarker                    func(s unsafe.Pointer, time int64, typ unsafe.Pointer)
	fAvReadFrame                        func(s unsafe.Pointer, pkt unsafe.Pointer) int32
	fAvSeekFrame                        func(s unsafe.Pointer, stream_index int32, timestamp int64, flags int32) int32
	fAvUrlSplit                         func(proto unsafe.Pointer, proto_size int32, authorization unsafe.Pointer, authorization_size int32, hostname unsafe.Pointer, hostname_size int32, port_ptr unsafe.Pointer, path unsafe.Pointer, path_size int32, url unsafe.Pointer)
)

func registerFormatDemux(h uintptr) {
	purego.RegisterLibFunc(&fAvAddIndexEntry, h, "av_add_index_entry")
	purego.RegisterLibFunc(&fAvDumpFormat, h, "av_dump_format")
	purego.RegisterLibFunc(&fAvFindBestStream, h, "av_find_best_stream")
	purego.RegisterLibFunc(&fAvformatAllocContext, h, "avformat_alloc_context")
	purego.RegisterLibFunc(&fAvformatAllocOutputContext2, h, "avformat_alloc_output_context2")
	purego.RegisterLibFunc(&fAvformatCloseInput, h, "avformat_close_input")
	purego.RegisterLibFunc(&fAvformatConfiguration, h, "avformat_configuration")
	purego.RegisterLibFunc(&fAvformatFindStreamInfo, h, "avformat_find_stream_info")
	purego.RegisterLibFunc(&fAvformatFlush, h, "avformat_flush")
	purego.RegisterLibFunc(&fAvformatFreeContext, h, "avformat_free_context")
	purego.RegisterLibFunc(&fAvformatGetClass, h, "avformat_get_class")
	purego.RegisterLibFunc(&fAvformatGetMovAudioTags, h, "avformat_get_mov_audio_tags")
	purego.RegisterLibFunc(&fAvformatGetMovVideoTags, h, "avformat_get_mov_video_tags")
	purego.RegisterLibFunc(&fAvformatGetRiffAudioTags, h, "avformat_get_riff_audio_tags")
	purego.RegisterLibFunc(&fAvformatGetRiffVideoTags, h, "avformat_get_riff_video_tags")
	purego.RegisterLibFunc(&fAvformatIndexGetEntriesCount, h, "avformat_index_get_entries_count")
	purego.RegisterLibFunc(&fAvformatIndexGetEntry, h, "avformat_index_get_entry")
	purego.RegisterLibFunc(&fAvformatIndexGetEntryFromTimestamp, h, "avformat_index_get_entry_from_timestamp")
	purego.RegisterLibFunc(&fAvformatInitOutput, h, "avformat_init_output")
	purego.RegisterLibFunc(&fAvformatLicense, h, "avformat_license")
	purego.RegisterLibFunc(&fAvformatMatchStreamSpecifier, h, "avformat_match_stream_specifier")
	purego.RegisterLibFunc(&fAvformatNetworkDeinit, h, "avformat_network_deinit")
	purego.RegisterLibFunc(&fAvformatNetworkInit, h, "avformat_network_init")
	purego.RegisterLibFunc(&fAvformatNewStream, h, "avformat_new_stream")
	purego.RegisterLibFunc(&fAvformatOpenInput, h, "avformat_open_input")
	purego.RegisterLibFunc(&fAvformatQueryCodec, h, "avformat_query_codec")
	purego.RegisterLibFunc(&fAvformatQueueAttachedPictures, h, "avformat_queue_attached_pictures")
	purego.RegisterLibFunc(&fAvformatSeekFile, h, "avformat_seek_file")
	purego.RegisterLibFunc(&fAvformatStreamGroupAddStream, h, "avformat_stream_group_add_stream")
	purego.RegisterLibFunc(&fAvformatStreamGroupCreate, h, "avformat_stream_group_create")
	purego.RegisterLibFunc(&fAvformatStreamGroupName, h, "avformat_stream_group_name")
	purego.RegisterLibFunc(&fAvformatVersion, h, "avformat_version")
	purego.RegisterLibFunc(&fAvformatWriteHeader, h, "avformat_write_header")
	purego.RegisterLibFunc(&fAvGuessCodec, h, "av_guess_codec")
	purego.RegisterLibFunc(&fAvGuessFormat, h, "av_guess_format")
	purego.RegisterLibFunc(&fAvGuessFrameRate, h, "av_guess_frame_rate")
	purego.RegisterLibFunc(&fAvGuessSampleAspectRatio, h, "av_guess_sample_aspect_ratio")
	purego.RegisterLibFunc(&fAvIndexSearchTimestamp, h, "av_index_search_timestamp")
	purego.RegisterLibFunc(&fAvioAccept, h, "avio_accept")
	purego.RegisterLibFunc(&fAvioAllocContext, h, "avio_alloc_context")
	purego.RegisterLibFunc(&fAvioCheck, h, "avio_check")
	purego.RegisterLibFunc(&fAvioClose, h, "avio_close")
	purego.RegisterLibFunc(&fAvioCloseDir, h, "avio_close_dir")
	purego.RegisterLibFunc(&fAvioCloseDynBuf, h, "avio_close_dyn_buf")
	purego.RegisterLibFunc(&fAvioClosep, h, "avio_closep")
	purego.RegisterLibFunc(&fAvioContextFree, h, "avio_context_free")
	purego.RegisterLibFunc(&fAvioEnumProtocols, h, "avio_enum_protocols")
	purego.RegisterLibFunc(&fAvioFeof, h, "avio_feof")
	purego.RegisterLibFunc(&fAvioFindProtocolName, h, "avio_find_protocol_name")
	purego.RegisterLibFunc(&fAvioFlush, h, "avio_flush")
	purego.RegisterLibFunc(&fAvioFreeDirectoryEntry, h, "avio_free_directory_entry")
	purego.RegisterLibFunc(&fAvioGetDynBuf, h, "avio_get_dyn_buf")
	purego.RegisterLibFunc(&fAvioGetStr, h, "avio_get_str")
	purego.RegisterLibFunc(&fAvioGetStr16be, h, "avio_get_str16be")
	purego.RegisterLibFunc(&fAvioGetStr16le, h, "avio_get_str16le")
	purego.RegisterLibFunc(&fAvioHandshake, h, "avio_handshake")
	purego.RegisterLibFunc(&fAvioOpen, h, "avio_open")
	purego.RegisterLibFunc(&fAvioOpen2, h, "avio_open2")
	purego.RegisterLibFunc(&fAvioOpenDir, h, "avio_open_dir")
	purego.RegisterLibFunc(&fAvioOpenDynBuf, h, "avio_open_dyn_buf")
	purego.RegisterLibFunc(&fAvioPause, h, "avio_pause")
	purego.RegisterLibFunc(&fAvioPrintStringArray, h, "avio_print_string_array")
	purego.RegisterLibFunc(&fAvioProtocolGetClass, h, "avio_protocol_get_class")
	purego.RegisterLibFunc(&fAvioPutStr, h, "avio_put_str")
	purego.RegisterLibFunc(&fAvioPutStr16be, h, "avio_put_str16be")
	purego.RegisterLibFunc(&fAvioPutStr16le, h, "avio_put_str16le")
	purego.RegisterLibFunc(&fAvioR8, h, "avio_r8")
	purego.RegisterLibFunc(&fAvioRb16, h, "avio_rb16")
	purego.RegisterLibFunc(&fAvioRb24, h, "avio_rb24")
	purego.RegisterLibFunc(&fAvioRb32, h, "avio_rb32")
	purego.RegisterLibFunc(&fAvioRb64, h, "avio_rb64")
	purego.RegisterLibFunc(&fAvioRead, h, "avio_read")
	purego.RegisterLibFunc(&fAvioReadDir, h, "avio_read_dir")
	purego.RegisterLibFunc(&fAvioReadPartial, h, "avio_read_partial")
	purego.RegisterLibFunc(&fAvioReadToBprint, h, "avio_read_to_bprint")
	purego.RegisterLibFunc(&fAvioRl16, h, "avio_rl16")
	purego.RegisterLibFunc(&fAvioRl24, h, "avio_rl24")
	purego.RegisterLibFunc(&fAvioRl32, h, "avio_rl32")
	purego.RegisterLibFunc(&fAvioRl64, h, "avio_rl64")
	purego.RegisterLibFunc(&fAvioSeek, h, "avio_seek")
	purego.RegisterLibFunc(&fAvioSeekTime, h, "avio_seek_time")
	purego.RegisterLibFunc(&fAvioSize, h, "avio_size")
	purego.RegisterLibFunc(&fAvioSkip, h, "avio_skip")
	purego.RegisterLibFunc(&fAvioVprintf, h, "avio_vprintf")
	purego.RegisterLibFunc(&fAvioW8, h, "avio_w8")
	purego.RegisterLibFunc(&fAvioWb16, h, "avio_wb16")
	purego.RegisterLibFunc(&fAvioWb24, h, "avio_wb24")
	purego.RegisterLibFunc(&fAvioWb32, h, "avio_wb32")
	purego.RegisterLibFunc(&fAvioWb64, h, "avio_wb64")
	purego.RegisterLibFunc(&fAvioWl16, h, "avio_wl16")
	purego.RegisterLibFunc(&fAvioWl24, h, "avio_wl24")
	purego.RegisterLibFunc(&fAvioWl32, h, "avio_wl32")
	purego.RegisterLibFunc(&fAvioWl64, h, "avio_wl64")
	purego.RegisterLibFunc(&fAvioWrite, h, "avio_write")
	purego.RegisterLibFunc(&fAvioWriteMarker, h, "avio_write_marker")
	purego.RegisterLibFunc(&fAvReadFrame, h, "av_read_frame")
	purego.RegisterLibFunc(&fAvSeekFrame, h, "av_seek_frame")
	purego.RegisterLibFunc(&fAvUrlSplit, h, "av_url_split")
}

func (x *Stream) AddIndexEntry(pos int64, timestamp int64, size int32, distance int32, flags int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAddIndexEntry(x.ptr, pos, timestamp, size, distance, flags); ret < 0 {
		return codeErr("av_add_index_entry", ret)
	}
	return nil
}

func (x *FormatContext) DumpFormat(index int32, url unsafe.Pointer, is_output int32) {
	if x == nil {
		return
	}
	fAvDumpFormat(x.ptr, index, url, is_output)
}

func (x *FormatContext) FindBestStream(typ unsafe.Pointer, wanted_stream_nb int32, related_stream int32, decoder_ret *unsafe.Pointer, flags int32) int32 {
	if x == nil {
		return 0
	}
	return fAvFindBestStream(x.ptr, typ, wanted_stream_nb, related_stream, decoder_ret, flags)
}

func (x *FormatContext) AllocContext() unsafe.Pointer {
	return fAvformatAllocContext()
}

func (x *FormatContext) AllocOutputContext2(ctx *unsafe.Pointer, oformat unsafe.Pointer, format_name unsafe.Pointer, filename unsafe.Pointer) error {
	if ret := fAvformatAllocOutputContext2(ctx, oformat, format_name, filename); ret < 0 {
		return codeErr("avformat_alloc_output_context2", ret)
	}
	return nil
}

func (x *FormatContext) CloseInput(s *unsafe.Pointer) {
	fAvformatCloseInput(s)
}

func (x *FormatContext) Configuration() unsafe.Pointer {
	return fAvformatConfiguration()
}

func (x *FormatContext) FindStreamInfo(options *unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatFindStreamInfo(x.ptr, options); ret < 0 {
		return codeErr("avformat_find_stream_info", ret)
	}
	return nil
}

func (x *FormatContext) Flush() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatFlush(x.ptr); ret < 0 {
		return codeErr("avformat_flush", ret)
	}
	return nil
}

func (x *FormatContext) FreeContext() {
	if x == nil {
		return
	}
	fAvformatFreeContext(x.ptr)
}

func (x *FormatContext) GetClass() unsafe.Pointer {
	return fAvformatGetClass()
}

func (x *FormatContext) GetMovAudioTags() unsafe.Pointer {
	return fAvformatGetMovAudioTags()
}

func (x *FormatContext) GetMovVideoTags() unsafe.Pointer {
	return fAvformatGetMovVideoTags()
}

func (x *FormatContext) GetRiffAudioTags() unsafe.Pointer {
	return fAvformatGetRiffAudioTags()
}

func (x *FormatContext) GetRiffVideoTags() unsafe.Pointer {
	return fAvformatGetRiffVideoTags()
}

func (x *Stream) IndexGetEntriesCount() int32 {
	if x == nil {
		return 0
	}
	return fAvformatIndexGetEntriesCount(x.ptr)
}

func (x *Stream) IndexGetEntry(idx int32) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvformatIndexGetEntry(x.ptr, idx)
}

func (x *Stream) IndexGetEntryFromTimestamp(wanted_timestamp int64, flags int32) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvformatIndexGetEntryFromTimestamp(x.ptr, wanted_timestamp, flags)
}

func (x *FormatContext) InitOutput(options *unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatInitOutput(x.ptr, options); ret < 0 {
		return codeErr("avformat_init_output", ret)
	}
	return nil
}

func (x *FormatContext) License() unsafe.Pointer {
	return fAvformatLicense()
}

func (x *FormatContext) MatchStreamSpecifier(st unsafe.Pointer, spec unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatMatchStreamSpecifier(x.ptr, st, spec); ret < 0 {
		return codeErr("avformat_match_stream_specifier", ret)
	}
	return nil
}

func (x *FormatContext) NetworkDeinit() error {
	if ret := fAvformatNetworkDeinit(); ret < 0 {
		return codeErr("avformat_network_deinit", ret)
	}
	return nil
}

func (x *FormatContext) NetworkInit() error {
	if ret := fAvformatNetworkInit(); ret < 0 {
		return codeErr("avformat_network_init", ret)
	}
	return nil
}

func (x *FormatContext) NewStream(c unsafe.Pointer) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvformatNewStream(x.ptr, c)
}

func (x *FormatContext) OpenInput(ps *unsafe.Pointer, url unsafe.Pointer, fmt unsafe.Pointer, options *unsafe.Pointer) error {
	if ret := fAvformatOpenInput(ps, url, fmt, options); ret < 0 {
		return codeErr("avformat_open_input", ret)
	}
	return nil
}

func (x *Format) QueryCodec(codec_id unsafe.Pointer, std_compliance int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatQueryCodec(x.ptr, codec_id, std_compliance); ret < 0 {
		return codeErr("avformat_query_codec", ret)
	}
	return nil
}

func (x *FormatContext) QueueAttachedPictures() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatQueueAttachedPictures(x.ptr); ret < 0 {
		return codeErr("avformat_queue_attached_pictures", ret)
	}
	return nil
}

func (x *FormatContext) SeekFile(stream_index int32, min_ts int64, ts int64, max_ts int64, flags int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvformatSeekFile(x.ptr, stream_index, min_ts, ts, max_ts, flags); ret < 0 {
		return codeErr("avformat_seek_file", ret)
	}
	return nil
}

func (x *FormatContext) StreamGroupAddStream(stg unsafe.Pointer, st unsafe.Pointer) error {
	if ret := fAvformatStreamGroupAddStream(stg, st); ret < 0 {
		return codeErr("avformat_stream_group_add_stream", ret)
	}
	return nil
}

func (x *FormatContext) StreamGroupCreate(typ unsafe.Pointer, options *unsafe.Pointer) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvformatStreamGroupCreate(x.ptr, typ, options)
}

func (x *FormatContext) StreamGroupName(typ unsafe.Pointer) unsafe.Pointer {
	return fAvformatStreamGroupName(typ)
}

func (x *FormatContext) Version() unsafe.Pointer {
	return fAvformatVersion()
}

func (x *FormatContext) WriteHeader(options *unsafe.Pointer) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvformatWriteHeader(x.ptr, options)
}

func (x *Format) GuessCodec(short_name unsafe.Pointer, filename unsafe.Pointer, mime_type unsafe.Pointer, typ unsafe.Pointer) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvGuessCodec(x.ptr, short_name, filename, mime_type, typ)
}

func (x *FormatContext) GuessFormat(short_name unsafe.Pointer, filename unsafe.Pointer, mime_type unsafe.Pointer) unsafe.Pointer {
	return fAvGuessFormat(short_name, filename, mime_type)
}

func (x *FormatContext) GuessFrameRate(stream unsafe.Pointer, frame unsafe.Pointer) AVRational {
	if x == nil {
		return AVRational{}
	}
	return fAvGuessFrameRate(x.ptr, stream, frame)
}

func (x *FormatContext) GuessSampleAspectRatio(stream unsafe.Pointer, frame unsafe.Pointer) AVRational {
	if x == nil {
		return AVRational{}
	}
	return fAvGuessSampleAspectRatio(x.ptr, stream, frame)
}

func (x *Stream) IndexSearchTimestamp(timestamp int64, flags int32) int32 {
	if x == nil {
		return 0
	}
	return fAvIndexSearchTimestamp(x.ptr, timestamp, flags)
}

func (x *IOContext) Accept(c *unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioAccept(x.ptr, c); ret < 0 {
		return codeErr("avio_accept", ret)
	}
	return nil
}

func (x *FormatContext) AllocIOContext(buffer unsafe.Pointer, buffer_size int32, write_flag int32, opaque unsafe.Pointer) unsafe.Pointer {
	return fAvioAllocContext(buffer, buffer_size, write_flag, opaque)
}

func (x *FormatContext) Check(url unsafe.Pointer, flags int32) error {
	if ret := fAvioCheck(url, flags); ret < 0 {
		return codeErr("avio_check", ret)
	}
	return nil
}

func (x *IOContext) Close() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioClose(x.ptr); ret < 0 {
		return codeErr("avio_close", ret)
	}
	return nil
}

func (x *FormatContext) CloseDir(s *unsafe.Pointer) error {
	if ret := fAvioCloseDir(s); ret < 0 {
		return codeErr("avio_close_dir", ret)
	}
	return nil
}

func (x *IOContext) CloseDynBuf(pbuffer *unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioCloseDynBuf(x.ptr, pbuffer); ret < 0 {
		return codeErr("avio_close_dyn_buf", ret)
	}
	return nil
}

func (x *FormatContext) Closep(s *unsafe.Pointer) error {
	if ret := fAvioClosep(s); ret < 0 {
		return codeErr("avio_closep", ret)
	}
	return nil
}

func (x *FormatContext) ContextFree(s *unsafe.Pointer) {
	fAvioContextFree(s)
}

func (x *FormatContext) EnumProtocols(opaque *unsafe.Pointer, output int32) unsafe.Pointer {
	return fAvioEnumProtocols(opaque, output)
}

func (x *IOContext) Feof() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioFeof(x.ptr); ret < 0 {
		return codeErr("avio_feof", ret)
	}
	return nil
}

func (x *FormatContext) FindProtocolName(url unsafe.Pointer) unsafe.Pointer {
	return fAvioFindProtocolName(url)
}

func (x *IOContext) Flush() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvioFlush(x.ptr)
}

func (x *FormatContext) FreeDirectoryEntry(entry *unsafe.Pointer) {
	fAvioFreeDirectoryEntry(entry)
}

func (x *IOContext) GetDynBuf(pbuffer *unsafe.Pointer) int32 {
	if x == nil {
		return 0
	}
	return fAvioGetDynBuf(x.ptr, pbuffer)
}

func (x *IOContext) GetStr(maxlen int32, buf unsafe.Pointer, buflen int32) int32 {
	if x == nil {
		return 0
	}
	return fAvioGetStr(x.ptr, maxlen, buf, buflen)
}

func (x *IOContext) GetStr16be(maxlen int32, buf unsafe.Pointer, buflen int32) int32 {
	if x == nil {
		return 0
	}
	return fAvioGetStr16be(x.ptr, maxlen, buf, buflen)
}

func (x *IOContext) GetStr16le(maxlen int32, buf unsafe.Pointer, buflen int32) int32 {
	if x == nil {
		return 0
	}
	return fAvioGetStr16le(x.ptr, maxlen, buf, buflen)
}

func (x *IOContext) Handshake() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioHandshake(x.ptr); ret < 0 {
		return codeErr("avio_handshake", ret)
	}
	return nil
}

func (x *FormatContext) Open(s *unsafe.Pointer, url unsafe.Pointer, flags int32) unsafe.Pointer {
	return fAvioOpen(s, url, flags)
}

func (x *FormatContext) Open2(s *unsafe.Pointer, url unsafe.Pointer, flags int32, int_cb unsafe.Pointer, options *unsafe.Pointer) error {
	if ret := fAvioOpen2(s, url, flags, int_cb, options); ret < 0 {
		return codeErr("avio_open2", ret)
	}
	return nil
}

func (x *FormatContext) OpenDir(s *unsafe.Pointer, url unsafe.Pointer, options *unsafe.Pointer) error {
	if ret := fAvioOpenDir(s, url, options); ret < 0 {
		return codeErr("avio_open_dir", ret)
	}
	return nil
}

func (x *FormatContext) OpenDynBuf(s *unsafe.Pointer) error {
	if ret := fAvioOpenDynBuf(s); ret < 0 {
		return codeErr("avio_open_dyn_buf", ret)
	}
	return nil
}

func (x *IOContext) Pause(pause int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioPause(x.ptr, pause); ret < 0 {
		return codeErr("avio_pause", ret)
	}
	return nil
}

func (x *IOContext) PrintStringArray(strings unsafe.Pointer) {
	if x == nil {
		return
	}
	fAvioPrintStringArray(x.ptr, strings)
}

func (x *FormatContext) ProtocolGetClass(name unsafe.Pointer) unsafe.Pointer {
	return fAvioProtocolGetClass(name)
}

func (x *IOContext) PutStr(str unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioPutStr(x.ptr, str); ret < 0 {
		return codeErr("avio_put_str", ret)
	}
	return nil
}

func (x *IOContext) PutStr16be(str unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioPutStr16be(x.ptr, str); ret < 0 {
		return codeErr("avio_put_str16be", ret)
	}
	return nil
}

func (x *IOContext) PutStr16le(str unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioPutStr16le(x.ptr, str); ret < 0 {
		return codeErr("avio_put_str16le", ret)
	}
	return nil
}

func (x *IOContext) R8() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioR8(x.ptr); ret < 0 {
		return codeErr("avio_r8", ret)
	}
	return nil
}

func (x *IOContext) Rb16() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRb16(x.ptr)
}

func (x *IOContext) Rb24() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRb24(x.ptr)
}

func (x *IOContext) Rb32() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRb32(x.ptr)
}

func (x *IOContext) Rb64() uint64 {
	if x == nil {
		return 0
	}
	return fAvioRb64(x.ptr)
}

func (x *IOContext) Read(buf unsafe.Pointer, size int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioRead(x.ptr, buf, size); ret < 0 {
		return codeErr("avio_read", ret)
	}
	return nil
}

func (x *FormatContext) ReadDir(s unsafe.Pointer, next *unsafe.Pointer) error {
	if ret := fAvioReadDir(s, next); ret < 0 {
		return codeErr("avio_read_dir", ret)
	}
	return nil
}

func (x *IOContext) ReadPartial(buf unsafe.Pointer, size int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioReadPartial(x.ptr, buf, size); ret < 0 {
		return codeErr("avio_read_partial", ret)
	}
	return nil
}

func (x *IOContext) ReadToBprint(pb unsafe.Pointer, max_size uintptr) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioReadToBprint(x.ptr, pb, max_size); ret < 0 {
		return codeErr("avio_read_to_bprint", ret)
	}
	return nil
}

func (x *IOContext) Rl16() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRl16(x.ptr)
}

func (x *IOContext) Rl24() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRl24(x.ptr)
}

func (x *IOContext) Rl32() uint32 {
	if x == nil {
		return 0
	}
	return fAvioRl32(x.ptr)
}

func (x *IOContext) Rl64() uint64 {
	if x == nil {
		return 0
	}
	return fAvioRl64(x.ptr)
}

func (x *IOContext) SeekPos(offset int64, whence int32) unsafe.Pointer {
	if x == nil {
		return nil
	}
	return fAvioSeek(x.ptr, offset, whence)
}

func (x *IOContext) SeekTime(stream_index int32, timestamp int64, flags int32) int64 {
	if x == nil {
		return 0
	}
	return fAvioSeekTime(x.ptr, stream_index, timestamp, flags)
}

func (x *IOContext) Size() int64 {
	if x == nil {
		return 0
	}
	return fAvioSize(x.ptr)
}

func (x *IOContext) Skip(offset int64) int64 {
	if x == nil {
		return 0
	}
	return fAvioSkip(x.ptr, offset)
}

func (x *IOContext) Vprintf(fmt unsafe.Pointer, ap unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvioVprintf(x.ptr, fmt, ap); ret < 0 {
		return codeErr("avio_vprintf", ret)
	}
	return nil
}

func (x *IOContext) W8(b int32) {
	if x == nil {
		return
	}
	fAvioW8(x.ptr, b)
}

func (x *IOContext) Wb16(val uint32) {
	if x == nil {
		return
	}
	fAvioWb16(x.ptr, val)
}

func (x *IOContext) Wb24(val uint32) {
	if x == nil {
		return
	}
	fAvioWb24(x.ptr, val)
}

func (x *IOContext) Wb32(val uint32) {
	if x == nil {
		return
	}
	fAvioWb32(x.ptr, val)
}

func (x *IOContext) Wb64(val uint64) {
	if x == nil {
		return
	}
	fAvioWb64(x.ptr, val)
}

func (x *IOContext) Wl16(val uint32) {
	if x == nil {
		return
	}
	fAvioWl16(x.ptr, val)
}

func (x *IOContext) Wl24(val uint32) {
	if x == nil {
		return
	}
	fAvioWl24(x.ptr, val)
}

func (x *IOContext) Wl32(val uint32) {
	if x == nil {
		return
	}
	fAvioWl32(x.ptr, val)
}

func (x *IOContext) Wl64(val uint64) {
	if x == nil {
		return
	}
	fAvioWl64(x.ptr, val)
}

func (x *IOContext) Write(buf unsafe.Pointer, size int32) {
	if x == nil {
		return
	}
	fAvioWrite(x.ptr, buf, size)
}

func (x *IOContext) WriteMarker(time int64, typ unsafe.Pointer) {
	if x == nil {
		return
	}
	fAvioWriteMarker(x.ptr, time, typ)
}

func (x *FormatContext) ReadFrame(pkt unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvReadFrame(x.ptr, pkt); ret < 0 {
		return codeErr("av_read_frame", ret)
	}
	return nil
}

func (x *FormatContext) SeekFrame(stream_index int32, timestamp int64, flags int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvSeekFrame(x.ptr, stream_index, timestamp, flags); ret < 0 {
		return codeErr("av_seek_frame", ret)
	}
	return nil
}

func (x *FormatContext) UrlSplit(proto unsafe.Pointer, proto_size int32, authorization unsafe.Pointer, authorization_size int32, hostname unsafe.Pointer, hostname_size int32, port_ptr unsafe.Pointer, path unsafe.Pointer, path_size int32, url unsafe.Pointer) {
	fAvUrlSplit(proto, proto_size, authorization, authorization_size, hostname, hostname_size, port_ptr, path, path_size, url)
}
