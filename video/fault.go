package video

import (
	"errors"
	"os"
)

// Fault kinds for VR6 triage. Stable strings for JSON; Chinese text lives
// in Fault.CN so the window can show it directly. The ffmpeg backend
// absorbs decode details natively, so only the shell/clip/cap buckets
// remain; the retired Go-decoder buckets stay as deprecated aliases so
// old callers still compile.
const (
	KindBadBox     = "bad-box"
	KindTruncated  = "truncated"
	KindBadClip    = "bad-clip"
	KindMemOverCap = "mem-over-cap"
	KindUnknown    = "unknown"

	// Deprecated: retired with the Go mp4/h264/h265/aac/color packages
	// (ffmpeg absorbs these natively). Kept so old callers compile;
	// Classify never returns them on the ffmpeg backend.
	KindMissingParam = "missing-params"
	KindF17          = "f17-old-tools"
	KindF20          = "f20-lost-reference"
	KindLevel        = "level-over-limit"
	KindInterlace    = "f12-interlace"
	KindProfile      = "profile-beyond-stage"
	KindColor        = "color-unsupported"
	KindAudio        = "audio-decode"
	KindH265         = "h265-headers-only"
)

// Fault names the layer and the tool behind an error.
// Classify never panics; unknown inputs map to KindUnknown.
type Fault struct {
	Kind  string
	Layer string
	Tool  string
	CN    string
}

// Classify sorts any open/decode/seek error into its fault bucket.
// It unwraps with errors.Is, so wrapped player errors keep their bucket.
func Classify(err error) Fault {
	if err == nil {
		return Fault{Kind: KindUnknown, Layer: "video", Tool: "-", CN: "无错误"}
	}
	switch {
	case errors.Is(err, ErrMemOverCap):
		return Fault{Kind: KindMemOverCap, Layer: "video", Tool: "封顶", CN: "装不下（video层：解前预估超内存封顶，见S7按档上限）"}
	case errors.Is(err, ErrUnsupportedContainer) || errors.Is(err, ErrUnsupportedCodec):
		return Fault{Kind: KindBadClip, Layer: "video", Tool: "注册表", CN: "格式不支持（注册表层：容器/编码不在支持表里，先问能力再开）"}
	case errors.Is(err, ErrNoVideo) || errors.Is(err, ErrNoFrames) ||
		errors.Is(err, ErrBadClip) || errors.Is(err, ErrClosed) || errors.Is(err, ErrDecodeEOF):
		return Fault{Kind: KindBadClip, Layer: "video", Tool: "播放器", CN: "片子打不开（video层：无视频轨/无可解帧/已关闭）"}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Fault{Kind: KindBadClip, Layer: "io", Tool: "文件", CN: "文件不存在（io层：路径错）"}
	}
	// ffmpeg backend: open/probe/decode failures carry the "ffmpeg:"
	// prefix and the open side hangs ErrBadClip; bucket them readably.
	if containsSub(err.Error(), "ffmpeg:") {
		return Fault{Kind: KindBadClip, Layer: "ffmpeg", Tool: "解码器", CN: "片子打不开（ffmpeg层：容器/编码不支持或文件损坏）"}
	}
	// Truncation surfaces as unexpected EOF inside the demuxer.
	if containsSub(err.Error(), "unexpected EOF") || containsSub(err.Error(), "EOF") {
		return Fault{Kind: KindTruncated, Layer: "ffmpeg", Tool: "截断", CN: "文件截断（ffmpeg层：流尾提前结束）"}
	}
	return Fault{Kind: KindUnknown, Layer: "unknown", Tool: "-", CN: "未知错误：" + firstLine(err.Error())}
}

// Readable renders the fault for the window list: kind + human text.
func (f Fault) Readable() string {
	return f.Kind + "：" + f.CN
}

func containsSub(s, sub string) bool {
	if sub == "" {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
