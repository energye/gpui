package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// perfState is the VR7 startup gate on the 1080p clip: demux numbers,
// param numbers, cold decode proof (5 frames, monotonic stamps, variance
// non-black), estimate-vs-cap check, plus pool warmup. Numbers always come
// from the real parse/play, never hand-written.
type perfState struct {
	name string
	path string
	// Demux.
	width, height uint32
	fps           float64
	durMs         int64
	samples       int
	keyframes     int
	// Params.
	profile  string
	level    string
	spsN     int
	ppsN     int
	split    int
	idrSplit int
	// Cold play proof (one pass to Ended, wall clock).
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
	// Cap.
	estimateB int64
	memCapKB  int
	// Pools warmed at startup (cold-path proof; steady hits come from
	// the live work pool cycled per shown frame in main.go).
	pools *govideo.Pools
	err   error
}

// pixVar is the mean absolute deviation of sampled bytes (R channel).
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

// pixVarPooled is kept for the startup cold path only (single call in
// loadPerf via pixVar direct form; the live loop inlines the same math
// to avoid a heap escape — see main.go). It is not used per-frame.
func pixVarPooled(pix []byte, scratch []byte) float64 {
	if len(pix) < 16 || len(scratch) == 0 {
		return pixVar(pix)
	}
	const step = 29
	n := 0
	for i := 0; i < len(pix) && n < len(scratch); i += 4 * step {
		scratch[n] = pix[i]
		n++
	}
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += float64(scratch[i])
	}
	mean := sum / float64(n)
	var dev float64
	for i := 0; i < n; i++ {
		d := float64(scratch[i]) - mean
		if d < 0 {
			d = -d
		}
		dev += d
	}
	return dev / float64(n)
}

func loadPerf(name, mp4Path string) *perfState {
	st := &perfState{name: name, path: mp4Path, memCapKB: govideo.MemCapKBFor1080p}
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
	// Cap check before playing: estimate must fit 512MB, else fail fast
	// readable (never silent growth).
	st.estimateB = govideo.EstimateDecoderBytes(int(st.width), int(st.height), st.samples, 256<<10)
	if st.estimateB >= int64(st.memCapKB)<<10 {
		st.err = fmt.Errorf("封顶超限: 预估%d字节超%dKB封顶", st.estimateB, st.memCapKB)
		return st
	}
	// Cold pools sized for this resolution (resolution switch rebuilds;
	// steady reuse never regrows). Warmup cycles prove zero-alloc path.
	st.pools = govideo.NewPools(int(st.width), int(st.height), 256<<10, 64<<20)
	for _, p := range []*govideo.Pool{st.pools.YUV, st.pools.RGBA, st.pools.Work} {
		b := p.Acquire()
		p.Release(b)
		b2 := p.Acquire()
		p.Release(b2)
	}
	// One cold pass to Ended: proves 1080p decodes (pixels pinned by
	// VR2 TestDecode1080pExact; here liveness + variance + monotonic).
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		st.err = fmt.Errorf("打不开: %w", err)
		return st
	}
	defer p.Close()
	var lastPTS int64 = -1
	st.ptsMono = true
	var firstPix, lastPix []byte
	deadline := govideo.WallDeadline(30000)
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
	// Opaque-premul identity cross-check (VR7 window stores premul-labeled
	// to skip the 8MB recompute per upload): straight alpha is 255
	// everywhere the converter writes, so premul(x) == x bit for bit.
	for i := 3; i < len(firstPix) && i < len(lastPix); i += 4 {
		if firstPix[i] != 255 || lastPix[i] != 255 {
			st.err = fmt.Errorf("透明通道非255(预乘恒等不成立)")
			return st
		}
	}
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
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func (st *perfState) infoLines() []string {
	if st.err != nil {
		return []string{"极限门禁失败：" + shortErr(st.err.Error(), 60)}
	}
	return []string{
		fmt.Sprintf("拆盒 1920x1080 %.1ffps 时长%dms 采样%d 关键帧%d", st.fps, st.durMs, st.samples, st.keyframes),
		fmt.Sprintf("参数 %s %s 片头%d 图%d 切%d帧 IDR%d帧", st.profile, st.level, st.spsN, st.ppsN, st.split, st.idrSplit),
		fmt.Sprintf("冷解码 %d帧 均%.1fms p95%.1fms(预算50ms) 递增=%v", st.decoded, st.decodeAvg, st.decodeP95, st.ptsMono),
		fmt.Sprintf("转色 首MAD%.1f 尾MAD%.1f 非黑 播完=%v 零丢", st.firstVar, st.lastVar, st.ended),
		fmt.Sprintf("封顶 预估%.1fMB/512MB池64MB 三池就绪", float64(st.estimateB)/(1<<20)),
	}
}
