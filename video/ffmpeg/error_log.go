package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// ErrorLog 错误日志模块: 报错转人话 + 日志开关 + 版本 + 时间戳换算,
// 结构体方法直接可用.
//
// Say it plain: 出错先把负数码翻成人话, 日志嫌吵就调等级关掉,
// 时间戳在不同时钟之间换算走这里, 不用手算.

// Library holder for version queries (无状态).
type Library struct{}

// Log holder for log-level switches (无状态, 全局生效).
type Log struct{}

// Math holder for timestamp/rescale math (无状态, 纯算数).
type Math struct{}

// Clock holder for wall-clock reads (无状态).
type Clock struct{}

// Cpu holder for cpu queries (无状态).
type Cpu struct{}

var (
	fErrStrerror   func(int32, unsafe.Pointer, uintptr) int32
	fLogGetLevel   func() int32
	fLogSetLevel   func(int32)
	fLogGetFlags   func() int32
	fLogSetFlags   func(int32)
	fLogSetCb      func(unsafe.Pointer)
	fLogDefaultCb  func(unsafe.Pointer, int32, unsafe.Pointer, unsafe.Pointer)
	fLogFmtLine    func(unsafe.Pointer, int32, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, int32, *int32)
	fLogFmtLine2   func(unsafe.Pointer, int32, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer, int32, *int32) int32
	fVerInfo       func() string
	fUtilVersion   func() uint32
	fUtilConfig    func() string
	fUtilLicense   func() string
	fRescale       func(int64, int64, int64) int64
	fRescaleRnd    func(int64, int64, int64, int32) int64
	fRescaleQ      func(int64, AVRational, AVRational) int64
	fRescaleQRnd   func(int64, AVRational, AVRational, int32) int64
	fRescaleDelta  func(AVRational, int64, AVRational, int32, *int64, AVRational) int64
	fAddQ          func(AVRational, AVRational) AVRational
	fAddStable     func(AVRational, int64, AVRational, int64) int64
	fCompareMod    func(uint64, uint64, uint64) int64
	fCompareTs     func(int64, AVRational, int64, AVRational) int32
	fGettime       func() int64
	fGettimeRel    func() int64
	fGettimeMono   func() int32
	fUsleep        func(uint32) int32
	fCpuCount      func() int32
	fCpuForceCount func(int32)
	fCpuMaxAlign   func() uintptr
)

func registerErrorLog(h uintptr) {
	purego.RegisterLibFunc(&fErrStrerror, h, "av_strerror")
	purego.RegisterLibFunc(&fLogGetLevel, h, "av_log_get_level")
	purego.RegisterLibFunc(&fLogSetLevel, h, "av_log_set_level")
	purego.RegisterLibFunc(&fLogGetFlags, h, "av_log_get_flags")
	purego.RegisterLibFunc(&fLogSetFlags, h, "av_log_set_flags")
	purego.RegisterLibFunc(&fLogSetCb, h, "av_log_set_callback")
	purego.RegisterLibFunc(&fLogDefaultCb, h, "av_log_default_callback")
	purego.RegisterLibFunc(&fLogFmtLine, h, "av_log_format_line")
	purego.RegisterLibFunc(&fLogFmtLine2, h, "av_log_format_line2")
	purego.RegisterLibFunc(&fVerInfo, h, "av_version_info")
	purego.RegisterLibFunc(&fUtilVersion, h, "avutil_version")
	purego.RegisterLibFunc(&fUtilConfig, h, "avutil_configuration")
	purego.RegisterLibFunc(&fUtilLicense, h, "avutil_license")
	purego.RegisterLibFunc(&fRescale, h, "av_rescale")
	purego.RegisterLibFunc(&fRescaleRnd, h, "av_rescale_rnd")
	purego.RegisterLibFunc(&fRescaleQ, h, "av_rescale_q")
	purego.RegisterLibFunc(&fRescaleQRnd, h, "av_rescale_q_rnd")
	purego.RegisterLibFunc(&fRescaleDelta, h, "av_rescale_delta")
	purego.RegisterLibFunc(&fAddQ, h, "av_add_q")
	purego.RegisterLibFunc(&fAddStable, h, "av_add_stable")
	purego.RegisterLibFunc(&fCompareMod, h, "av_compare_mod")
	purego.RegisterLibFunc(&fCompareTs, h, "av_compare_ts")
	purego.RegisterLibFunc(&fGettime, h, "av_gettime")
	purego.RegisterLibFunc(&fGettimeRel, h, "av_gettime_relative")
	purego.RegisterLibFunc(&fGettimeMono, h, "av_gettime_relative_is_monotonic")
	purego.RegisterLibFunc(&fUsleep, h, "av_usleep")
	purego.RegisterLibFunc(&fCpuCount, h, "av_cpu_count")
	purego.RegisterLibFunc(&fCpuForceCount, h, "av_cpu_force_count")
	purego.RegisterLibFunc(&fCpuMaxAlign, h, "av_cpu_max_align")
}

// StrError turns a negative AVERROR into text (errText 同款, 公开给上层).
func StrError(code int32) string { return errText(code) }

// Version returns the ffmpeg version string ("7.1.5").
func (Library) Version() string {
	if ensureLoaded() != nil {
		return ""
	}
	return fVerInfo()
}

// UtilVersion returns the libavutil version number (avutil_version).
func (Library) UtilVersion() uint32 {
	if ensureLoaded() != nil {
		return 0
	}
	return fUtilVersion()
}

// Configuration returns the ffmpeg build configuration string.
func (Library) Configuration() string {
	if ensureLoaded() != nil {
		return ""
	}
	return fUtilConfig()
}

// License returns the ffmpeg license string.
func (Library) License() string {
	if ensureLoaded() != nil {
		return ""
	}
	return fUtilLicense()
}

// Level returns the current log level.
func (Log) Level() int32 { return fLogGetLevel() }

// SetLevel sets the log level (LogError 只留报错, 传 LogQuiet 全关).
func (Log) SetLevel(level int32) { fLogSetLevel(level) }

// Flags returns the current log flags.
func (Log) Flags() int32 { return fLogGetFlags() }

// SetFlags sets log flags (AV_LOG_PRINT_LEVEL 等).
func (Log) SetFlags(flags int32) { fLogSetFlags(flags) }

// SetCallback installs a custom log callback (传 nil 恢复默认;
// 回调签名 void(*)(void*, int, const char*, va_list), 纯地址透传).
func (Log) SetCallback(cb unsafe.Pointer) { fLogSetCb(cb) }

// Rescale computes a*b/c (时间戳换算, 上溢钳位).
func (Math) Rescale(a, b, c int64) int64 { return fRescale(a, b, c) }

// RescaleRnd computes a*b/c with rounding.
func (Math) RescaleRnd(a, b, c int64, rnd int32) int64 { return fRescaleRnd(a, b, c, rnd) }

// RescaleQ converts timestamp a from bq to cq.
func (Math) RescaleQ(a int64, bq, cq AVRational) int64 { return fRescaleQ(a, bq, cq) }

// RescaleQRnd converts timestamp a from bq to cq with rounding.
func (Math) RescaleQRnd(a int64, bq, cq AVRational, rnd int32) int64 {
	return fRescaleQRnd(a, bq, cq, rnd)
}

// AddQ adds two rationals.
func (Math) AddQ(b, c AVRational) AVRational { return fAddQ(b, c) }

// CompareTs compares ts_a/tb_a vs ts_b/tb_b (-1/0/1).
func (Math) CompareTs(tsA int64, tbA AVRational, tsB int64, tbB AVRational) int32 {
	return fCompareTs(tsA, tbA, tsB, tbB)
}

// NowUs returns wall microseconds.
func (Clock) NowUs() int64 { return fGettime() }

// NowRelativeUs returns monotonic microseconds.
func (Clock) NowRelativeUs() int64 { return fGettimeRel() }

// SleepUs sleeps usec microseconds.
func (Clock) SleepUs(usec uint32) error {
	if ret := fUsleep(usec); ret < 0 {
		return codeErr("av_usleep", ret)
	}
	return nil
}

// Count returns logical cpu count.
func (Cpu) Count() int { return int(fCpuCount()) }

// MaxAlign returns malloc alignment.
func (Cpu) MaxAlign() int { return int(fCpuMaxAlign()) }
