package main

import (
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// soakState is the VC2 startup gate on the 1080p clip: demux numbers,
// param numbers, cold decode proof (5 frames, monotonic stamps, variance
// non-black), estimate-vs-cap check, plus pool warmup. Numbers always come
// from the real parse/play, never hand-written. The 120s soak itself
// (slope/leak/hitch/CPU/GC) is proved live in main.go.
type soakState struct {
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

func loadSoak(name, mp4Path string) *soakState {
	st := &soakState{name: name, path: mp4Path, memCapKB: govideo.MemCapKBFor1080p}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		st.err = fmt.Errorf("拆盒失败: %w", err)
		return st
	}
	v := movie.Video
	if v == nil || len(v.Samples) == 0 {
		st.err = fmt.Errorf("拆盒失败: 没视频轨或没采样")
		return st
	}
	st.width, st.height = v.Width, v.Height
	st.fps = v.FrameRate
	st.durMs = v.DurationMs
	st.samples = len(v.Samples)
	st.keyframes = len(v.Keyframes)
	if st.width != 1920 || st.height != 1080 {
		st.err = fmt.Errorf("档位错: %dx%d 不是1080p(降档偷过直接判FAIL)", st.width, st.height)
		return st
	}
	if st.keyframes < 1 || st.durMs <= 0 {
		st.err = fmt.Errorf("拆盒失败: 关键帧%d 时长%d毫秒", st.keyframes, st.durMs)
		return st
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		st.err = fmt.Errorf("参数失败: %w", err)
		return st
	}
	ps := h264.NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		st.err = fmt.Errorf("参数失败: %w", err)
		return st
	}
	sps, err := h264.ParseSPS(avcc.SPS[0])
	if err != nil {
		st.err = fmt.Errorf("参数失败: %w", err)
		return st
	}
	if !h264.LevelSupported(sps.LevelIDC) {
		st.err = fmt.Errorf("参数失败: 等级%s超限", sps.Level)
		return st
	}
	st.profile, st.level = sps.Profile, sps.Level
	st.spsN, st.ppsN = len(ps.SPS), len(ps.PPS)
	f, err := os.Open(mp4Path)
	if err != nil {
		st.err = fmt.Errorf("参数失败: 打不开 %w", err)
		return st
	}
	defer f.Close()
	var ordered [][]byte
	for i, s := range v.Samples {
		if i >= 8 {
			break
		}
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			st.err = fmt.Errorf("参数失败: 采样%d读不到 %w", s.Number, err)
			return st
		}
		units, err := h264.SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			st.err = fmt.Errorf("参数失败: 采样%d切分 %w", s.Number, err)
			return st
		}
		ordered = append(ordered, units...)
	}
	frames, err := h264.SplitFrames(ordered)
	if err != nil {
		st.err = fmt.Errorf("参数失败: 切帧 %w", err)
		return st
	}
	st.split = len(frames)
	for _, fr := range frames {
		if fr.IsIDR {
			st.idrSplit++
		}
	}
	if st.spsN < 1 || st.ppsN < 1 || st.split < 1 || st.idrSplit < 1 {
		st.err = fmt.Errorf("参数失败: 片头%d 图%d 切%d帧 IDR%d帧", st.spsN, st.ppsN, st.split, st.idrSplit)
		return st
	}
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
	// Opaque-premul identity cross-check (VC2 window stores premul-labeled
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

func (st *soakState) infoLines() []string {
	if st.err != nil {
		return []string{"长跑门禁失败：" + shortErr(st.err.Error(), 60)}
	}
	return []string{
		fmt.Sprintf("拆盒 1920x1080 %.1ffps 时长%dms 采样%d 关键帧%d", st.fps, st.durMs, st.samples, st.keyframes),
		fmt.Sprintf("参数 %s %s 片头%d 图%d 切%d帧 IDR%d帧", st.profile, st.level, st.spsN, st.ppsN, st.split, st.idrSplit),
		fmt.Sprintf("冷解码 %d帧 均%.1fms p95%.1fms(预算50ms) 递增=%v", st.decoded, st.decodeAvg, st.decodeP95, st.ptsMono),
		fmt.Sprintf("转色 首MAD%.1f 尾MAD%.1f 非黑 播完=%v 零丢", st.firstVar, st.lastVar, st.ended),
		fmt.Sprintf("封顶 预估%.1fMB/512MB池64MB 三池就绪", float64(st.estimateB)/(1<<20)),
	}
}
