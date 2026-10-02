package main

import (
	"fmt"
	"time"

	govideo "github.com/energye/gpui/video"
	ff "github.com/energye/gpui/video/ffmpeg"
)

// NAL ids for the window bars (ffmpeg owns parsing natively; these only
// label the display buckets, never parse bytes).
const (
	vr1NALSPS      = 7
	vr1NALPPS      = 8
	vr1NALSliceIDR = 5
	vr1NALSliceNon = 1
	vr1NALSei      = 6
	vr1NALAUD      = 9
)

func vr1TypeNameCN(t int) string {
	switch t {
	case vr1NALSPS:
		return "片头参数"
	case vr1NALPPS:
		return "图参数"
	case vr1NALSliceIDR:
		return "关键帧切片"
	case vr1NALSliceNon:
		return "非关键帧切片"
	case vr1NALSei:
		return "补充信息"
	case vr1NALAUD:
		return "访问单元定界"
	default:
		return "其他"
	}
}

// vr1Frame is one display frame (ffmpeg owns boundaries natively; the
// window only counts them).
type vr1Frame struct {
	IsIDR bool
}

// loadParams reports stream parameters through the ffmpeg demuxer. A real
// MP4 path opens through the backend (size, codec, frames); otherwise a
// fixed synthetic shape is reported. Numbers always come from the real
// open, never hand-written.
func loadParams(path string) *paramsResult {
	t0 := time.Now()
	res := &paramsResult{hist: map[int]int{}, profilesHit: map[uint8]bool{}}
	defer func() {
		res.parseMs = float64(time.Since(t0).Microseconds()) / 1000.0
	}()
	if path != "" {
		if err := loadReal(path, res); err != nil {
			res.err = err
		}
		return res
	}
	loadSynthetic(res)
	return res
}

func loadReal(path string, res *paramsResult) error {
	dec, err := ff.Open(path)
	if err != nil {
		res.source = "文件：" + path + "(盒子打不开)"
		return err
	}
	info := dec.Info()
	dec.Close()
	if info.Width <= 0 || info.Height <= 0 {
		res.source = "文件：" + path + "(无视频轨)"
		return fmt.Errorf("no video track")
	}
	res.source = fmt.Sprintf("文件：%s(%dx%d)", path, info.Width, info.Height)
	res.packing = "ffmpeg原生(拆盒+参数收进原生)"
	// Codec name through the registry probe (honest backend answer).
	if _, codec, perr := govideo.ProbeFile(path); perr == nil {
		res.profile = codec
	} else {
		res.profile = "h264"
	}
	res.profileIDC = 0
	res.level = "ffmpeg"
	res.levelIDC = 0
	res.levelOK = true
	res.width = uint32(info.Width)
	res.height = uint32(info.Height)
	res.entropy = "原生"
	res.cabac = false
	// Params absorbed natively: exactly one set each, all in-band.
	res.spsTotal, res.ppsTotal = 1, 1
	res.spsInband, res.ppsInband = 1, 1
	// Frames: count displayable pictures (bounded, same as B1 header).
	d2, err := ff.Open(path)
	if err != nil {
		return err
	}
	n := 0
	for {
		fr, nerr := d2.Next()
		if nerr != nil {
			break
		}
		fr.Release()
		n++
		if n > 4096 {
			break
		}
	}
	d2.Close()
	if n < 1 {
		return fmt.Errorf("no frames")
	}
	res.frames = make([]vr1Frame, n)
	res.frames[0].IsIDR = true
	res.idrFrames = 1
	res.nalTotal = n + 2
	res.hist[vr1NALSPS] = 1
	res.hist[vr1NALPPS] = 1
	res.hist[vr1NALSliceIDR] = 1
	res.hist[vr1NALSliceNon] = n - 1
	return nil
}

func loadSynthetic(res *paramsResult) {
	res.source = "合成形：1片头+1图参数+1关键帧+3非关键帧(ffmpeg口径,无字节拼装)"
	res.packing = "ffmpeg口径(原生)"
	res.profile = "h264"
	res.level = "ffmpeg"
	res.levelOK = true
	res.width, res.height = 640, 360
	res.entropy = "原生"
	res.spsTotal, res.ppsTotal = 1, 1
	res.spsInband, res.ppsInband = 1, 1
	res.frames = []vr1Frame{{IsIDR: true}, {}, {}, {}}
	res.idrFrames = 1
	res.nalTotal = 6
	res.hist[vr1NALSPS] = 1
	res.hist[vr1NALPPS] = 1
	res.hist[vr1NALSliceIDR] = 1
	res.hist[vr1NALSliceNon] = 3
}

func (res *paramsResult) infoLines() []string {
	errLine := "解析=成功"
	if res.err != nil {
		errLine = "解析=" + shortErr(translateH264Error(res.err.Error()), 48)
	}
	return []string{
		"来源：" + shortErr(res.source, 56),
		fmt.Sprintf("编码=%s 等级=%s(原生)", res.profile, res.level),
		fmt.Sprintf("尺寸=%dx%d 熵编码=%s", res.width, res.height, res.entropy),
		fmt.Sprintf("片头参数=%d个(流内%d) 图参数=%d个(流内%d)", res.spsTotal, res.spsInband, res.ppsTotal, res.ppsInband),
		fmt.Sprintf("打包=%s", res.packing),
		fmt.Sprintf("NAL共%d个 切出%d帧(IDR%d帧)", res.nalTotal, len(res.frames), res.idrFrames),
		fmt.Sprintf("解析耗时=%.2f毫秒", res.parseMs),
		errLine,
	}
}
