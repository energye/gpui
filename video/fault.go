package video

import (
	"errors"
	"fmt"
	"os"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// Fault kinds for VR6 triage. Stable strings for JSON; Chinese text lives
// in Fault.CN so the window can show it directly.
const (
	KindBadBox       = "bad-box"
	KindMissingParam = "missing-params"
	KindTruncated    = "truncated"
	KindF17          = "f17-old-tools"
	KindF20          = "f20-lost-reference"
	KindLevel        = "level-over-limit"
	KindInterlace    = "f12-interlace"
	KindProfile      = "profile-beyond-stage"
	KindColor        = "color-unsupported"
	KindBadClip      = "bad-clip"
	KindMemOverCap   = "mem-over-cap"
	KindUnknown      = "unknown"
)

// Fault names the layer and the tool behind an error: bad boxes point at
// mp4, missing sets at h264 params, F17/F20/F12/level at their spec tool.
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
	case errors.Is(err, mp4.ErrTruncated) || errors.Is(err, h264.ErrTruncated):
		return Fault{Kind: KindTruncated, Layer: "mp4/h264", Tool: "截断", CN: "文件截断（盒子越界/样本读不到，mp4/h264截断层）"}
	case errors.Is(err, mp4.ErrBadBox) || errors.Is(err, mp4.ErrNoMoov) ||
		errors.Is(err, mp4.ErrNoVideoTrack) || errors.Is(err, mp4.ErrNoSampleTable) ||
		errors.Is(err, mp4.ErrBadSampleTable):
		return Fault{Kind: KindBadBox, Layer: "mp4", Tool: "盒子", CN: "盒子坏了（mp4层：缺moov/无视频轨/样表不一致）"}
	case errors.Is(err, mp4.ErrUnsupported):
		return Fault{Kind: KindBadBox, Layer: "mp4", Tool: "盒子", CN: "盒子不支持（mp4层：特性超本期范围）"}
	case errors.Is(err, mp4.ErrFragmented):
		return Fault{Kind: KindBadBox, Layer: "mp4", Tool: "盒子", CN: "盒子碎了（mp4层：分段moof坏/无视频分段，见B1门禁）"}
	case errors.Is(err, h264.ErrSliceGroups) || errors.Is(err, h264.ErrDataPartitioning) ||
		errors.Is(err, h264.ErrRedundantPic) || errors.Is(err, h264.ErrUnsupportedNAL):
		return Fault{Kind: KindF17, Layer: "h264", Tool: "F17", CN: "老容错工具F17（条带组/数据分区/冗余片/扩展切片，本期只认不解）"}
	case errors.Is(err, h264.ErrLostReference):
		return Fault{Kind: KindF20, Layer: "h264", Tool: "F20", CN: "参考帧丢了F20（坏帧已隔离，继续播剩余好帧）"}
	case errors.Is(err, h264.ErrUnsupportedLevel):
		return Fault{Kind: KindLevel, Layer: "h264", Tool: "F2", CN: "等级超限F2（超1-5.2，播放器带不动）"}
	case errors.Is(err, h264.ErrStageScope):
		msg := err.Error()
		if containsSub(msg, "F12") {
			return Fault{Kind: KindInterlace, Layer: "h264", Tool: "F12", CN: "隔行F12（场模式/MBAFF，本期只认不解）"}
		}
		return Fault{Kind: KindProfile, Layer: "h264", Tool: "F1", CN: "档位超范围（非B/M/H，本期只做三档）"}
	case errors.Is(err, h264.ErrBadAVCC) || errors.Is(err, h264.ErrNoParamSets) ||
		errors.Is(err, h264.ErrBadSPS) || errors.Is(err, h264.ErrBadPPS) ||
		errors.Is(err, h264.ErrMissingSPS) || errors.Is(err, h264.ErrMissingPPS):
		return Fault{Kind: KindMissingParam, Layer: "h264", Tool: "参数集", CN: "缺参数（h264层：SPS/PPS缺/坏/对不上切片）"}
	case errors.Is(err, h264.ErrBadNALU) || errors.Is(err, h264.ErrBadSliceHeader) ||
		errors.Is(err, h264.ErrNoNALU):
		// Slice-header failures without the F20 marker are still corrupt
		// samples: isolate the frame, keep playing.
		return Fault{Kind: KindF20, Layer: "h264", Tool: "F20", CN: "坏帧F20（切片头/载荷坏，已隔离继续播）"}
	case errors.Is(err, color.ErrUnsupportedSampling) || errors.Is(err, color.ErrUnsupportedMatrix):
		return Fault{Kind: KindColor, Layer: "color", Tool: "F11", CN: "颜色不支持（采样/矩阵超BT.601/709范围）"}
	case errors.Is(err, color.ErrBadSize) || errors.Is(err, color.ErrBadPlanes):
		return Fault{Kind: KindColor, Layer: "color", Tool: "尺寸", CN: "颜色尺寸坏（YUV平面与宽高对不上）"}
	case errors.Is(err, ErrNoVideo) || errors.Is(err, ErrNoFrames) ||
		errors.Is(err, ErrBadClip) || errors.Is(err, ErrClosed) || errors.Is(err, ErrDecodeEOF):
		return Fault{Kind: KindBadClip, Layer: "video", Tool: "播放器", CN: "片子打不开（video层：无视频轨/无可解帧/已关闭）"}
	case errors.Is(err, ErrMemOverCap):
		return Fault{Kind: KindMemOverCap, Layer: "video", Tool: "封顶", CN: "装不下（video层：解前预估超内存封顶，见S7按档上限）"}
	case errors.Is(err, ErrUnsupportedContainer) || errors.Is(err, ErrUnsupportedCodec):
		return Fault{Kind: KindBadClip, Layer: "video", Tool: "注册表", CN: "格式不支持（注册表层：容器/编码不在支持表里，先问能力再开）"}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Fault{Kind: KindBadClip, Layer: "io", Tool: "文件", CN: "文件不存在（io层：路径错）"}
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

// checkStreamLimits enforces F1/F2 at open time: only B/M/H profiles and
// levels 1-5.2 play; anything else fails fast with a namable error.
func checkStreamLimits(sps *h264.SPS, path string) error {
	if sps == nil {
		return nil
	}
	if !h264.IsBaselineMainHigh(sps.ProfileIDC) {
		return fmt.Errorf("video: profile %s beyond stage %s: %w", sps.Profile, path, h264.ErrStageScope)
	}
	if !h264.LevelSupported(sps.LevelIDC) {
		return fmt.Errorf("video: level %s over limit %s: %w", sps.Level, path, h264.ErrUnsupportedLevel)
	}
	return nil
}
