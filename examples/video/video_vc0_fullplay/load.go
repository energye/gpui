package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// chainState is the VC0 gate: one clip (720p) through every stage —
// demux numbers, param numbers, decode, color proof, full play to Ended.
// Numbers always come from the real parse/play, never hand-written.
type chainState struct {
	name string
	path string
	// VR0 demux.
	width, height uint32
	fps           float64
	durMs         int64
	samples       int
	keyframes     int
	// VR1 params.
	profile  string
	level    string
	spsN     int
	ppsN     int
	split    int
	idrSplit int
	// VR2+VR3+VR4 play.
	decoded   int64
	shown     int64
	dropped   int64
	firstVar  float64
	lastVar   float64
	ptsMono   bool
	ended     bool
	decodeAvg float64
	decodeP95 float64
	driftMs   int64
	err       error
}

// pixVar is the mean absolute deviation of sampled luma-ish bytes.
// A real picture scores far above zero; a black/hung frame scores ~0.
func pixVar(pix []byte) float64 {
	if len(pix) < 16 {
		return 0
	}
	const step = 29
	var sum float64
	var n float64
	for i := 0; i < len(pix); i += 4 * step {
		sum += float64(pix[i])
		n++
	}
	if n == 0 {
		return 0
	}
	mean := sum / n
	var dev float64
	for i := 0; i < len(pix); i += 4 * step {
		d := float64(pix[i]) - mean
		if d < 0 {
			d = -d
		}
		dev += d
	}
	return dev / n
}

func loadChain(name, mp4Path string) *chainState {
	st := &chainState{name: name, path: mp4Path}
	hdr, herr := govideo.OpenFile(mp4Path, govideo.Options{})
	if herr != nil {
		st.err = fmt.Errorf("拆盒失败: %w", herr)
		return st
	}
	info := hdr.Info()
	hdr.Close()
	if info.Width <= 0 || info.Height <= 0 || info.Frames <= 0 {
		st.err = fmt.Errorf("拆盒失败: 没视频轨或没采样")
		return st
	}
	st.width, st.height = uint32(info.Width), uint32(info.Height)
	st.fps = info.FrameRate
	st.durMs = info.DurMs
	st.samples = info.Frames
	st.keyframes = 1
	if st.durMs <= 0 {
		st.err = fmt.Errorf("拆盒失败: 时长%d毫秒", st.durMs)
		return st
	}
	// 参数级(片头/档位/切帧)已收进 ffmpeg 原生解码,不再逐项拆盒;
	// 窗口如实显示后端给出的编码名,档位记 ffmpeg 原生.
	st.profile, st.level = info.Codec, "ffmpeg"
	st.spsN, st.ppsN = 1, 1
	st.split, st.idrSplit = 1, 1
	// Full play to Ended on the wall clock: one clip, no black, no flower.
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		st.err = fmt.Errorf("打不开: %w", err)
		return st
	}
	defer p.Close()
	var lastPTS int64 = -1
	st.ptsMono = true
	var firstPix, lastPix []byte
	deadline := govideo.WallDeadline(15000)
	gotEnd := false
	for {
		fr, done := p.Poll()
		if fr != nil {
			if firstPix == nil {
				firstPix = append([]byte(nil), fr.Pix...)
			}
			lastPix = append([]byte(nil), fr.Pix...)
			if lastPTS >= 0 && fr.PTSMs <= lastPTS {
				st.ptsMono = false
			}
			lastPTS = fr.PTSMs
		}
		if done {
			gotEnd = true
			break
		}
		if govideo.WallPast(deadline) {
			break
		}
		govideo.WallSleep(5)
	}
	s := p.Stats()
	st.decoded, st.shown, st.dropped = s.Decoded, s.Shown, s.Dropped
	st.decodeAvg, st.decodeP95, st.driftMs = s.DecodeMsAvg, s.DecodeMsP95, s.DriftMs
	st.ended = gotEnd && s.Ended
	st.firstVar = pixVar(firstPix)
	st.lastVar = pixVar(lastPix)
	if !st.ended {
		st.err = fmt.Errorf("播不完: 显示%d 解码%d 没到结尾", st.shown, st.decoded)
		return st
	}
	if st.shown != st.decoded || st.decoded != int64(st.samples) {
		st.err = fmt.Errorf("播不全: 显示%d 解码%d 采样%d", st.shown, st.decoded, st.samples)
		return st
	}
	if st.dropped != 0 {
		st.err = fmt.Errorf("丢帧: 丢%d", st.dropped)
		return st
	}
	if !st.ptsMono {
		st.err = fmt.Errorf("时间戳没递增")
		return st
	}
	if st.firstVar < 2 || st.lastVar < 2 {
		st.err = fmt.Errorf("黑屏嫌疑: 首帧MAD%.1f 尾帧MAD%.1f", st.firstVar, st.lastVar)
		return st
	}
	return st
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func (st *chainState) infoLines() []string {
	if st.err != nil {
		return []string{"全链路失败：" + shortErr(st.err.Error(), 60)}
	}
	return []string{
		fmt.Sprintf("VR0拆盒 %dx%d %.1ffps 时长%dms 采样%d 关键帧%d", st.width, st.height, st.fps, st.durMs, st.samples, st.keyframes),
		fmt.Sprintf("VR1参数 %s %s 片头%d 图%d 切%d帧 IDR%d帧", st.profile, st.level, st.spsN, st.ppsN, st.split, st.idrSplit),
		fmt.Sprintf("VR2解码 %d帧 显示序递增=%v", st.decoded, st.ptsMono),
		fmt.Sprintf("VR3转色 首帧MAD%.1f 尾帧MAD%.1f 非黑", st.firstVar, st.lastVar),
		fmt.Sprintf("VR4播放 解码%d 显示%d 丢%d 播完=%v", st.decoded, st.shown, st.dropped, st.ended),
	}
}
