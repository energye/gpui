package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ResampleAudio 重采样模块: SwrContext/AudioFifo/采样格式 全量导出, 结构体方法直接可用.
//
// Say it plain: 把声音转成要的采样率声道格式, 播之前先对齐.

// Resampler holder.
type Resampler struct{ ptr unsafe.Pointer }

func (x *Resampler) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// AudioFifo holder.
type AudioFifo struct{ ptr unsafe.Pointer }

func (x *AudioFifo) Ptr() unsafe.Pointer {
	mustUse(ensureModResample())
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvAudioFifoAlloc     func(sample_fmt int32, channels int32, nb_samples int32) unsafe.Pointer
	fAvAudioFifoDrain     func(af unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoFree      func(af unsafe.Pointer)
	fAvAudioFifoPeek      func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoPeekAt    func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32, offset int32) int32
	fAvAudioFifoRead      func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoRealloc   func(af unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoReset     func(af unsafe.Pointer)
	fAvAudioFifoSize      func(af unsafe.Pointer) int32
	fAvAudioFifoSpace     func(af unsafe.Pointer) int32
	fAvAudioFifoWrite     func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvGetBytesPerSample  func(sample_fmt int32) int32
	fAvGetPackedSampleFmt func(sample_fmt int32) int32
	fAvGetPlanarSampleFmt func(sample_fmt int32) int32
	fAvGetSampleFmt       func(name string) int32
	fAvGetSampleFmtName   func(sample_fmt int32) unsafe.Pointer
	fAvGetSampleFmtString func(buf unsafe.Pointer, buf_size int32, sample_fmt int32) unsafe.Pointer
	fAvSampleFmtIsPlanar  func(sample_fmt int32) int32
	fSwrAlloc             func() unsafe.Pointer
	fSwrAllocSetOpts2     func(ps *unsafe.Pointer, out_ch_layout unsafe.Pointer, out_sample_fmt int32, out_sample_rate int32, in_ch_layout unsafe.Pointer, in_sample_fmt int32, in_sample_rate int32, log_offset int32, log_ctx unsafe.Pointer) int32
	fSwrBuildMatrix2      func(in_layout unsafe.Pointer, out_layout unsafe.Pointer, center_mix_level float64, surround_mix_level float64, lfe_mix_level float64, maxval float64, rematrix_volume float64, matrix unsafe.Pointer, stride uintptr, matrix_encoding int32, log_context unsafe.Pointer) int32
	fSwrClose             func(s unsafe.Pointer)
	fSwrConfigFrame       func(swr unsafe.Pointer, out unsafe.Pointer, in unsafe.Pointer) int32
	fSwrConvert           func(s unsafe.Pointer, out unsafe.Pointer, out_count int32, in unsafe.Pointer, in_count int32) int32
	fSwrConvertFrame      func(swr unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) int32
	fSwrDropOutput        func(s unsafe.Pointer, count int32) int32
	fSwrFree              func(s *unsafe.Pointer)
	fSwrGetClass          func() unsafe.Pointer
	fSwrGetDelay          func(s unsafe.Pointer, base int64) int64
	fSwrGetOutSamples     func(s unsafe.Pointer, in_samples int32) int32
	fSwrInit              func(s unsafe.Pointer) int32
	fSwrInjectSilence     func(s unsafe.Pointer, count int32) int32
	fSwrIsInitialized     func(s unsafe.Pointer) int32
	fSwrNextPts           func(s unsafe.Pointer, pts int64) int64
	fSwrSetChannelMapping func(s unsafe.Pointer, channel_map unsafe.Pointer) int32
	fSwrSetCompensation   func(s unsafe.Pointer, sample_delta int32, compensation_distance int32) int32
	fSwrSetMatrix         func(s unsafe.Pointer, matrix unsafe.Pointer, stride int32) int32
)

// ensureModResample 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modResampleOnce sync.Once

func ensureModResample() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modResampleOnce.Do(func() { registerResampleAudio(libHandle) })
	return nil
}

func registerResampleAudio(h uintptr) {
	purego.RegisterLibFunc(&fAvAudioFifoAlloc, h, "av_audio_fifo_alloc")
	purego.RegisterLibFunc(&fAvAudioFifoDrain, h, "av_audio_fifo_drain")
	purego.RegisterLibFunc(&fAvAudioFifoFree, h, "av_audio_fifo_free")
	purego.RegisterLibFunc(&fAvAudioFifoPeek, h, "av_audio_fifo_peek")
	purego.RegisterLibFunc(&fAvAudioFifoPeekAt, h, "av_audio_fifo_peek_at")
	purego.RegisterLibFunc(&fAvAudioFifoRead, h, "av_audio_fifo_read")
	purego.RegisterLibFunc(&fAvAudioFifoRealloc, h, "av_audio_fifo_realloc")
	purego.RegisterLibFunc(&fAvAudioFifoReset, h, "av_audio_fifo_reset")
	purego.RegisterLibFunc(&fAvAudioFifoSize, h, "av_audio_fifo_size")
	purego.RegisterLibFunc(&fAvAudioFifoSpace, h, "av_audio_fifo_space")
	purego.RegisterLibFunc(&fAvAudioFifoWrite, h, "av_audio_fifo_write")
	purego.RegisterLibFunc(&fAvGetBytesPerSample, h, "av_get_bytes_per_sample")
	purego.RegisterLibFunc(&fAvGetPackedSampleFmt, h, "av_get_packed_sample_fmt")
	purego.RegisterLibFunc(&fAvGetPlanarSampleFmt, h, "av_get_planar_sample_fmt")
	purego.RegisterLibFunc(&fAvGetSampleFmt, h, "av_get_sample_fmt")
	purego.RegisterLibFunc(&fAvGetSampleFmtName, h, "av_get_sample_fmt_name")
	purego.RegisterLibFunc(&fAvGetSampleFmtString, h, "av_get_sample_fmt_string")
	purego.RegisterLibFunc(&fAvSampleFmtIsPlanar, h, "av_sample_fmt_is_planar")
	purego.RegisterLibFunc(&fSwrAlloc, h, "swr_alloc")
	purego.RegisterLibFunc(&fSwrAllocSetOpts2, h, "swr_alloc_set_opts2")
	purego.RegisterLibFunc(&fSwrBuildMatrix2, h, "swr_build_matrix2")
	purego.RegisterLibFunc(&fSwrClose, h, "swr_close")
	purego.RegisterLibFunc(&fSwrConfigFrame, h, "swr_config_frame")
	purego.RegisterLibFunc(&fSwrConvert, h, "swr_convert")
	purego.RegisterLibFunc(&fSwrConvertFrame, h, "swr_convert_frame")
	purego.RegisterLibFunc(&fSwrDropOutput, h, "swr_drop_output")
	purego.RegisterLibFunc(&fSwrFree, h, "swr_free")
	purego.RegisterLibFunc(&fSwrGetClass, h, "swr_get_class")
	purego.RegisterLibFunc(&fSwrGetDelay, h, "swr_get_delay")
	purego.RegisterLibFunc(&fSwrGetOutSamples, h, "swr_get_out_samples")
	purego.RegisterLibFunc(&fSwrInit, h, "swr_init")
	purego.RegisterLibFunc(&fSwrInjectSilence, h, "swr_inject_silence")
	purego.RegisterLibFunc(&fSwrIsInitialized, h, "swr_is_initialized")
	purego.RegisterLibFunc(&fSwrNextPts, h, "swr_next_pts")
	purego.RegisterLibFunc(&fSwrSetChannelMapping, h, "swr_set_channel_mapping")
	purego.RegisterLibFunc(&fSwrSetCompensation, h, "swr_set_compensation")
	purego.RegisterLibFunc(&fSwrSetMatrix, h, "swr_set_matrix")
}

// Alloc 音频采样队列存取（对 av_audio_fifo_alloc；参数 sample_fmt（采样格式枚举数，如 1=S16）、channels、nb_samples；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (x *Resampler) Alloc(sample_fmt int32, channels int32, nb_samples int32) unsafe.Pointer {
	mustUse(ensureModResample())
	return fAvAudioFifoAlloc(sample_fmt, channels, nb_samples)
}

// Drain 音频采样队列存取（对 av_audio_fifo_drain；参数 nb_samples；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Drain(nb_samples int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoDrain(x.ptr, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_drain", ret)
	}
	return nil
}

// Free 音频采样队列存取（对 av_audio_fifo_free；无参数；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Free() {
	mustUse(ensureModResample())
	if x == nil {
		return
	}
	fAvAudioFifoFree(x.ptr)
}

// Peek 音频采样队列存取（对 av_audio_fifo_peek；参数 data（声道指针数组）、nb_samples；回实际看到的采样数（不够就给现有的，不报错），失败回 error；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Peek(data unsafe.Pointer, nb_samples int32) (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvAudioFifoPeek(x.ptr, data, nb_samples); ret < 0 {
		return 0, codeErr("av_audio_fifo_peek", ret)
	} else {
		return ret, nil
	}
}

// PeekAt 音频采样队列存取（对 av_audio_fifo_peek_at；参数 data、nb_samples、offset（从第几个开始看）；回实际看到的采样数，失败回 error；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) PeekAt(data unsafe.Pointer, nb_samples int32, offset int32) (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvAudioFifoPeekAt(x.ptr, data, nb_samples, offset); ret < 0 {
		return 0, codeErr("av_audio_fifo_peek_at", ret)
	} else {
		return ret, nil
	}
}

// Read 音频采样队列存取（对 av_audio_fifo_read；参数 data、nb_samples；回实际读走的采样数（不够就给现有的，不报错），失败回 error；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Read(data unsafe.Pointer, nb_samples int32) (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvAudioFifoRead(x.ptr, data, nb_samples); ret < 0 {
		return 0, codeErr("av_audio_fifo_read", ret)
	} else {
		return ret, nil
	}
}

// Realloc 音频采样队列存取（对 av_audio_fifo_realloc；参数 nb_samples；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Realloc(nb_samples int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoRealloc(x.ptr, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_realloc", ret)
	}
	return nil
}

// Reset 音频采样队列存取（对 av_audio_fifo_reset；无参数；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Reset() {
	mustUse(ensureModResample())
	if x == nil {
		return
	}
	fAvAudioFifoReset(x.ptr)
}

// Size 音频采样队列存取（对 av_audio_fifo_size；无参数；回数值；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Size() int32 {
	mustUse(ensureModResample())
	if x == nil {
		return 0
	}
	return fAvAudioFifoSize(x.ptr)
}

// Space 问队列还能写几个采样（对 av_audio_fifo_space；回可写采样数，失败回 error；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Space() (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvAudioFifoSpace(x.ptr); ret < 0 {
		return 0, codeErr("av_audio_fifo_space", ret)
	} else {
		return ret, nil
	}
}

// Write 音频采样队列存取（对 av_audio_fifo_write；参数 data（声道指针数组）、nb_samples；回实际写进的采样数（队列满了就给能装下的，不报错），失败回 error；nil 接收器直接回零值，不崩）。
func (x *AudioFifo) Write(data unsafe.Pointer, nb_samples int32) (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	if ret := fAvAudioFifoWrite(x.ptr, data, nb_samples); ret < 0 {
		return 0, codeErr("av_audio_fifo_write", ret)
	} else {
		return ret, nil
	}
}

// GetBytesPerSample 问采样格式每个采样占几字节（对 av_get_bytes_per_sample；参数 sample_fmt（枚举数）；回字节数，错格式回 0；无状态调用）。
func (x *Resampler) GetBytesPerSample(sample_fmt int32) int32 {
	mustUse(ensureModResample())
	return fAvGetBytesPerSample(sample_fmt)
}

// GetPackedSampleFmt 找采样格式的 packed 版（对 av_get_packed_sample_fmt；参数 sample_fmt；回数值或个数；无状态调用）。
func (x *Resampler) GetPackedSampleFmt(sample_fmt int32) int32 {
	mustUse(ensureModResample())
	return fAvGetPackedSampleFmt(sample_fmt)
}

// GetPlanarSampleFmt 找采样格式的 planar 版（对 av_get_planar_sample_fmt；参数 sample_fmt；回数值或个数；无状态调用）。
func (x *Resampler) GetPlanarSampleFmt(sample_fmt int32) int32 {
	mustUse(ensureModResample())
	return fAvGetPlanarSampleFmt(sample_fmt)
}

// GetSampleFmt 按名字找采样格式编号（对 av_get_sample_fmt；参数 name（如 "s16"）；回枚举数，对不上回 -1（NONE）；无状态调用）。
func (x *Resampler) GetSampleFmt(name string) int32 {
	mustUse(ensureModResample())
	return fAvGetSampleFmt(name)
}

// GetSampleFmtName 按编号找采样格式名字（对 av_get_sample_fmt_name；参数 sample_fmt（枚举数）；回静态串借用不释放，对不上回 nil；无状态调用）。
func (x *Resampler) GetSampleFmtName(sample_fmt int32) unsafe.Pointer {
	mustUse(ensureModResample())
	return fAvGetSampleFmtName(sample_fmt)
}

// GetSampleFmtString 把采样格式名写进自备缓冲（对 av_get_sample_fmt_string；参数 buf（调用方给的内存）、buf_size、sample_fmt；回的就是 buf；无状态调用）。
func (x *Resampler) GetSampleFmtString(buf unsafe.Pointer, buf_size int32, sample_fmt int32) unsafe.Pointer {
	mustUse(ensureModResample())
	return fAvGetSampleFmtString(buf, buf_size, sample_fmt)
}

// SampleFmtIsPlanar 问采样格式是不是分平面存（对 av_sample_fmt_is_planar；参数 sample_fmt；回数值；无状态调用）。
func (x *Resampler) SampleFmtIsPlanar(sample_fmt int32) int32 {
	mustUse(ensureModResample())
	return fAvSampleFmtIsPlanar(sample_fmt)
}

// Alloc2 新建重采样器（对 swr_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (x *Resampler) Alloc2() unsafe.Pointer {
	mustUse(ensureModResample())
	return fSwrAlloc()
}

// AllocSetOpts2 新建重采样器（对 swr_alloc_set_opts2；参数 ps、out_ch_layout、out_sample_fmt、out_sample_rate、in_ch_layout、in_sample_fmt、in_sample_rate、log_offset、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (x *Resampler) AllocSetOpts2(ps *unsafe.Pointer, out_ch_layout unsafe.Pointer, out_sample_fmt int32, out_sample_rate int32, in_ch_layout unsafe.Pointer, in_sample_fmt int32, in_sample_rate int32, log_offset int32, log_ctx unsafe.Pointer) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if ret := fSwrAllocSetOpts2(ps, out_ch_layout, out_sample_fmt, out_sample_rate, in_ch_layout, in_sample_fmt, in_sample_rate, log_offset, log_ctx); ret < 0 {
		return codeErr("swr_alloc_set_opts2", ret)
	}
	return nil
}

// BuildMatrix2 按声道布局算混音矩阵（对 swr_build_matrix2；参数 in_layout/out_layout（声道布局指针）、5 个 double 系数、matrix（double 输出缓冲）、stride（行跨度，传 unsafe.Sizeof(double)*声道数一类）、matrix_encoding（混音编码枚举数）、log_context（传 nil）；成功回 nil，失败回 error；nil 接收器直接回零值，不崩）。
func (x *Resampler) BuildMatrix2(in_layout unsafe.Pointer, out_layout unsafe.Pointer, center_mix_level float64, surround_mix_level float64, lfe_mix_level float64, maxval float64, rematrix_volume float64, matrix unsafe.Pointer, stride uintptr, matrix_encoding int32, log_context unsafe.Pointer) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if ret := fSwrBuildMatrix2(in_layout, out_layout, center_mix_level, surround_mix_level, lfe_mix_level, maxval, rematrix_volume, matrix, stride, matrix_encoding, log_context); ret < 0 {
		return codeErr("swr_build_matrix2", ret)
	}
	return nil
}

// Close 关重采样器，留着下次再配（对 swr_close；只清状态不释放，想重配再调 Init；nil 接收器直接回，不崩）。
func (x *Resampler) Close() {
	mustUse(ensureModResample())
	if x == nil {
		return
	}
	fSwrClose(x.ptr)
}

// ConfigFrame 按输入输出帧配重采样器（对 swr_config_frame；参数 out、in；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) ConfigFrame(out unsafe.Pointer, in unsafe.Pointer) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConfigFrame(x.ptr, out, in); ret < 0 {
		return codeErr("swr_config_frame", ret)
	}
	return nil
}

// Convert 转一批采样，裸指针版（对 swr_convert；参数 out/in（声道指针数组）、out_count/in_count；只报成功失败，要产出数用 ConvertCount；nil 接收器直接回零值，不崩）。
func (x *Resampler) Convert(out unsafe.Pointer, out_count int32, in unsafe.Pointer, in_count int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConvert(x.ptr, out, out_count, in, in_count); ret < 0 {
		return codeErr("swr_convert", ret)
	}
	return nil
}

// ConvertCount behaves like Convert but returns the produced sample
// count per channel (swr_convert's non-negative return).
func (x *Resampler) ConvertCount(out unsafe.Pointer, out_count int32, in unsafe.Pointer, in_count int32) (int32, error) {
	if err := ensureModResample(); err != nil {
		return 0, err
	}
	if x == nil {
		return 0, errNilFF
	}
	ret := fSwrConvert(x.ptr, out, out_count, in, in_count)
	if ret < 0 {
		return 0, codeErr("swr_convert", ret)
	}
	return ret, nil
}

// ConvertFrame 转一批采样，裸指针版（对 swr_convert_frame；参数 output、input；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) ConvertFrame(output unsafe.Pointer, input unsafe.Pointer) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConvertFrame(x.ptr, output, input); ret < 0 {
		return codeErr("swr_convert_frame", ret)
	}
	return nil
}

// DropOutput 丢掉重采样器里攒的输出（对 swr_drop_output；参数 count；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) DropOutput(count int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrDropOutput(x.ptr, count); ret < 0 {
		return codeErr("swr_drop_output", ret)
	}
	return nil
}

// Free 释放重采样器（对 swr_free；参数 s；按签名取回值；nil 接收器直接回零值，不崩）。
func (x *Resampler) Free(s *unsafe.Pointer) {
	mustUse(ensureModResample())
	fSwrFree(s)
}

// GetClass 取重采样器的选项类（对 swr_get_class；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (x *Resampler) GetClass() unsafe.Pointer {
	mustUse(ensureModResample())
	return fSwrGetClass()
}

// GetDelay 问重采样器里还压着多少采样（对 swr_get_delay；参数 base；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *Resampler) GetDelay(base int64) int64 {
	mustUse(ensureModResample())
	if x == nil {
		return 0
	}
	return fSwrGetDelay(x.ptr, base)
}

// GetOutSamples 问吃这么多输入最多出多少输出（对 swr_get_out_samples；参数 in_samples；回数值；nil 接收器直接回零值，不崩）。
func (x *Resampler) GetOutSamples(in_samples int32) int32 {
	mustUse(ensureModResample())
	if x == nil {
		return 0
	}
	return fSwrGetOutSamples(x.ptr, in_samples)
}

// Init 初始化重采样器，配好后调（对 swr_init；无参数；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) Init() error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrInit(x.ptr); ret < 0 {
		return codeErr("swr_init", ret)
	}
	return nil
}

// InjectSilence 往重采样器里塞静音补空（对 swr_inject_silence；参数 count；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) InjectSilence(count int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrInjectSilence(x.ptr, count); ret < 0 {
		return codeErr("swr_inject_silence", ret)
	}
	return nil
}

// IsInitialized 问重采样器配好没有（对 swr_is_initialized；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (x *Resampler) IsInitialized() int32 {
	mustUse(ensureModResample())
	if x == nil {
		return 0
	}
	return fSwrIsInitialized(x.ptr)
}

// NextPts 按时间戳算下一包该几点（对 swr_next_pts；参数 pts；回数值；nil 接收器直接回零值，不崩）。
func (x *Resampler) NextPts(pts int64) int64 {
	mustUse(ensureModResample())
	if x == nil {
		return 0
	}
	return fSwrNextPts(x.ptr, pts)
}

// SetChannelMapping 设声道映射表（对 swr_set_channel_mapping；参数 channel_map；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) SetChannelMapping(channel_map unsafe.Pointer) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetChannelMapping(x.ptr, channel_map); ret < 0 {
		return codeErr("swr_set_channel_mapping", ret)
	}
	return nil
}

// SetCompensation 设采样补偿，拉伸或压缩时间（对 swr_set_compensation；参数 sample_delta、compensation_distance；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) SetCompensation(sample_delta int32, compensation_distance int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetCompensation(x.ptr, sample_delta, compensation_distance); ret < 0 {
		return codeErr("swr_set_compensation", ret)
	}
	return nil
}

// SetMatrix 设混音矩阵（对 swr_set_matrix；参数 matrix、stride；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (x *Resampler) SetMatrix(matrix unsafe.Pointer, stride int32) error {
	if err := ensureModResample(); err != nil {
		return err
	}
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetMatrix(x.ptr, matrix, stride); ret < 0 {
		return codeErr("swr_set_matrix", ret)
	}
	return nil
}
