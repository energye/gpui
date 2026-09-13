package main

import (
	"errors"
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// comboState is the VC3 startup gate on the 720p clip: embed numbers
// (demux 1280x720, params, cold decode to Ended with monotonic stamps and
// non-black variance) plus the registry proof (probe names, capability
// lists, same-clip play through the registry, window-side stub decoder,
// three readable unsupported cases). Numbers always come from the real
// parse/play/registry calls, never hand-written. The live embed
// composition (scale/clip/opacity/overlay/resize) is proved in main.go.
type comboState struct {
	name string
	path string
	// Embed demux.
	width, height uint32
	fps           float64
	durMs         int64
	samples       int
	keyframes     int
	// Embed params.
	profile  string
	level    string
	spsN     int
	ppsN     int
	split    int
	idrSplit int
	// Embed cold play proof (one pass to Ended, wall clock).
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
	// Registry proof.
	container string
	codec     string
	regDec    int64
	regShown  int64
	regFrames int
	regEnded  bool
	regP95    float64
	infoC     string
	infoD     string
	stubR, stubG, stubB uint8
	stubOK              bool
	badShellErr         string
	badCodecErr         string
	badColorErr         string
	regPass             int
	regTotal            int
	err                 error
}

// stubGrayDecoder is the window-side second decoder: arrives purely
// through RegisterDecoder, no core change. Flat grey synthetic picture.
type stubGrayDecoder struct {
	w, h uint32
	fed  int
}

func (s *stubGrayDecoder) DecodeNALU(nalu []byte) error {
	s.fed++
	return nil
}

func (s *stubGrayDecoder) FinishPicture() (*h264.Picture, error) {
	p, err := h264.NewPicture(s.w, s.h)
	if err != nil {
		return nil, err
	}
	for i := range p.Y {
		p.Y[i] = 180
	}
	return p, nil
}

func (s *stubGrayDecoder) Sampling() string { return color.SamplingYUV420P }

func stubGraySplit(buf []byte, lengthSize int) ([][]byte, error) {
	return [][]byte{buf}, nil
}

// coldPlay is one open + poll-to-Ended pass over a clip.
type coldPlay struct {
	decoded  int64
	shown    int64
	dropped  int64
	frames   int
	avg      float64
	p95      float64
	drift    int64
	ended    bool
	firstVar float64
	lastVar  float64
	ptsMono  bool
	infoC    string
	infoD    string
}

// coldPlayToEnded opens path and polls to Ended on the wall clock.
// Pixels stay owned by the helper; callers only keep variances.
func coldPlayToEnded(path string) (coldPlay, error) {
	var cp coldPlay
	p, err := govideo.OpenFile(path, govideo.Options{})
	if err != nil {
		return cp, err
	}
	defer p.Close()
	var lastPTS int64 = -1
	cp.ptsMono = true
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
				cp.ptsMono = false
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
	cp.decoded, cp.shown, cp.dropped = s.Decoded, s.Shown, s.Dropped
	cp.frames = p.Info().Frames
	cp.avg, cp.p95, cp.drift = s.DecodeMsAvg, s.DecodeMsP95, s.DriftMs
	cp.ended = gotEnd && s.Ended
	cp.firstVar = pixVar(firstPix)
	cp.lastVar = pixVar(lastPix)
	cp.infoC, cp.infoD = p.Info().Container, p.Info().Codec
	return cp, nil
}

// supportsFirstStage reports whether the registry carries mp4/h264/yuv420p.
func supportsFirstStage() (hasC, hasD, hasS bool) {
	for _, n := range govideo.SupportedContainers() {
		if n == "mp4" {
			hasC = true
		}
	}
	for _, n := range govideo.SupportedCodecs() {
		if n == "h264" {
			hasD = true
		}
	}
	for _, n := range govideo.SupportedSamplings() {
		if n == color.SamplingYUV420P {
			hasS = true
		}
	}
	return hasC, hasD, hasS
}

// verifyStubGray registers name as a grey stub decoder and checks the
// 96x96 flat-grey picture converts through the color registry.
func verifyStubGray(name string) (r, g, b uint8, ok bool) {
	govideo.RegisterDecoder(name, func() govideo.Decoder { return &stubGrayDecoder{w: 96, h: 96} }, stubGraySplit)
	d, err := govideo.NewDecoder(name)
	if err != nil {
		return 0, 0, 0, false
	}
	units, err := govideo.SplitUnits(name, []byte{0x01, 0x02}, 4)
	if err != nil || len(units) != 1 {
		return 0, 0, 0, false
	}
	for _, u := range units {
		if err := d.DecodeNALU(u); err != nil {
			return 0, 0, 0, false
		}
	}
	pic, err := d.FinishPicture()
	if err != nil || pic == nil || pic.Width != 96 || pic.Height != 96 {
		return 0, 0, 0, false
	}
	cf, err := color.Convert(d.Sampling(), pic.Y, pic.Cb, pic.Cr, 96, 96, color.Options{})
	if err != nil || len(cf.Pix) != 96*96*4 {
		return 0, 0, 0, false
	}
	r, g, b, a := cf.At(48, 48)
	dr := int(r) - int(g)
	if dr < 0 {
		dr = -dr
	}
	db := int(g) - int(b)
	if db < 0 {
		db = -db
	}
	if a != 255 || dr > 2 || db > 2 {
		return r, g, b, false
	}
	return r, g, b, true
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

func (st *comboState) fail(format string, args ...any) {
	if st.err == nil {
		st.err = fmt.Errorf(format, args...)
	}
}

func (st *comboState) regCheck(name string, ok bool, detail string) {
	st.regTotal++
	if ok {
		st.regPass++
	} else {
		st.fail("%s: %s", name, detail)
	}
}

func loadCombo(name, mp4Path string) *comboState {
	st := &comboState{name: name, path: mp4Path}
	// Embed gate: demux numbers off the real shell.
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		st.fail("拆盒失败: %v", err)
		return st
	}
	v := movie.Video
	if v == nil || len(v.Samples) == 0 {
		st.fail("拆盒失败: 没视频轨或没采样")
		return st
	}
	st.width, st.height = v.Width, v.Height
	st.fps = v.FrameRate
	st.durMs = v.DurationMs
	st.samples = len(v.Samples)
	st.keyframes = len(v.Keyframes)
	if st.width != 1280 || st.height != 720 {
		st.fail("档位错: %dx%d 不是720p(降档偷过直接判FAIL)", st.width, st.height)
		return st
	}
	if st.keyframes < 1 || st.durMs <= 0 {
		st.fail("拆盒失败: 关键帧%d 时长%d毫秒", st.keyframes, st.durMs)
		return st
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		st.fail("参数失败: %v", err)
		return st
	}
	ps := h264.NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		st.fail("参数失败: %v", err)
		return st
	}
	sps, err := h264.ParseSPS(avcc.SPS[0])
	if err != nil {
		st.fail("参数失败: %v", err)
		return st
	}
	if !h264.LevelSupported(sps.LevelIDC) {
		st.fail("参数失败: 等级%s超限", sps.Level)
		return st
	}
	st.profile, st.level = sps.Profile, sps.Level
	st.spsN, st.ppsN = len(ps.SPS), len(ps.PPS)
	f, err := os.Open(mp4Path)
	if err != nil {
		st.fail("参数失败: 打不开 %w", err)
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
			st.fail("参数失败: 采样%d读不到 %v", s.Number, err)
			return st
		}
		units, err := h264.SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			st.fail("参数失败: 采样%d切分 %v", s.Number, err)
			return st
		}
		ordered = append(ordered, units...)
	}
	frames, err := h264.SplitFrames(ordered)
	if err != nil {
		st.fail("参数失败: 切帧 %v", err)
		return st
	}
	st.split = len(frames)
	for _, fr := range frames {
		if fr.IsIDR {
			st.idrSplit++
		}
	}
	if st.spsN < 1 || st.ppsN < 1 || st.split < 1 || st.idrSplit < 1 {
		st.fail("参数失败: 片头%d 图%d 切%d帧 IDR%d帧", st.spsN, st.ppsN, st.split, st.idrSplit)
		return st
	}
	// Embed gate: one cold pass to Ended (pixels pinned by VR2
	// TestDecode720pExact; here liveness + variance + monotonic).
	cp, err := coldPlayToEnded(mp4Path)
	if err != nil {
		st.fail("打不开: %v", err)
		return st
	}
	st.decoded, st.shown, st.dropped = cp.decoded, cp.shown, cp.dropped
	st.decodeAvg, st.decodeP95, st.driftMs = cp.avg, cp.p95, cp.drift
	st.ended = cp.ended
	st.firstVar, st.lastVar = cp.firstVar, cp.lastVar
	st.ptsMono = cp.ptsMono
	if !st.ended {
		st.fail("播不完: 显示%d 解码%d 没到结尾", st.shown, st.decoded)
		return st
	}
	if st.shown != st.decoded || st.decoded != int64(st.samples) {
		st.fail("播不全: 显示%d 解码%d 采样%d", st.shown, st.decoded, st.samples)
		return st
	}
	if st.dropped != 0 {
		st.fail("丢帧: 丢%d", st.dropped)
		return st
	}
	if !st.ptsMono {
		st.fail("时间戳没递增")
		return st
	}
	if st.firstVar < 2 || st.lastVar < 2 {
		st.fail("黑屏嫌疑: 首帧MAD%.1f 尾帧MAD%.1f", st.firstVar, st.lastVar)
		return st
	}
	// Registry proof, same 8 cases as the VR9 window.
	container, codec, err := govideo.ProbeFile(mp4Path)
	if err != nil {
		st.fail("探测失败: %v", err)
		return st
	}
	st.container, st.codec = container, codec
	st.regCheck("探测", container != "" && codec != "", fmt.Sprintf("容器=%q 编码=%q", container, codec))
	hasC, hasD, hasS := supportsFirstStage()
	st.regCheck("能力查询", hasC && hasD && hasS,
		fmt.Sprintf("容器%v 编码%v 采样%v", govideo.SupportedContainers(), govideo.SupportedCodecs(), govideo.SupportedSamplings()))
	rcp, err := coldPlayToEnded(mp4Path)
	if err != nil {
		st.fail("注册表播不开: %v", err)
		return st
	}
	st.infoC, st.infoD = rcp.infoC, rcp.infoD
	st.regCheck("注册表名", st.infoC == container && st.infoD == codec,
		fmt.Sprintf("窗=%s/%s 探针=%s/%s", st.infoC, st.infoD, container, codec))
	st.regDec, st.regShown, st.regFrames = rcp.decoded, rcp.shown, rcp.frames
	st.regP95 = rcp.p95
	st.regEnded = rcp.ended
	st.regCheck("注册表播完", st.regEnded && st.regShown == int64(st.regFrames) && st.regFrames > 0,
		fmt.Sprintf("显示%d 帧数%d 结尾=%v", st.regShown, st.regFrames, st.regEnded))
	const stub = "vc3-window-stub-gray"
	r, g, b, ok := verifyStubGray(stub)
	st.stubR, st.stubG, st.stubB = r, g, b
	st.stubOK = ok
	st.regCheck("桩解码器", ok, "灰桩走注册表转出失败")
	junk, err := os.CreateTemp("", "vc3-junk-*.bin")
	if err != nil {
		st.regCheck("坏盒可读", false, fmt.Sprintf("建临时文件失败: %v", err))
	} else {
		_, _ = junk.WriteString("this is not a video shell at all, just text")
		junk.Close()
		defer os.Remove(junk.Name())
		_, oerr := govideo.OpenFile(junk.Name(), govideo.Options{})
		st.badShellErr = fmt.Sprint(oerr)
		ok := oerr != nil && errors.Is(oerr, govideo.ErrUnsupportedContainer) &&
			govideo.Classify(oerr).Kind == govideo.KindBadClip
		st.regCheck("坏盒可读", ok, fmt.Sprintf("错=%v", oerr))
	}
	_, cerr := govideo.NewDecoder("bogus-codec-xxx")
	st.badCodecErr = fmt.Sprint(cerr)
	st.regCheck("坏编码可读", cerr != nil && errors.Is(cerr, govideo.ErrUnsupportedCodec), fmt.Sprintf("错=%v", cerr))
	_, serr := color.Convert("bogus-sampling-xxx", []byte{1}, []byte{1}, []byte{1}, 2, 2, color.Options{})
	st.badColorErr = fmt.Sprint(serr)
	st.regCheck("坏采样可读", serr != nil && errors.Is(serr, color.ErrUnsupportedSampling), fmt.Sprintf("错=%v", serr))
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

func (st *comboState) infoLines() []string {
	if st.err != nil {
		return []string{"组合门禁失败：" + shortErr(st.err.Error(), 60)}
	}
	return []string{
		fmt.Sprintf("注册 %d/%d 探测%s/%s 灰桩%v", st.regPass, st.regTotal, st.container, st.codec, st.stubOK),
		fmt.Sprintf("三坏例人话零崩 注册播%d帧到结尾=%v", st.regDec, st.regEnded),
		fmt.Sprintf("内嵌 冷解码%d播完零丢递增非黑", st.decoded),
		fmt.Sprintf("直播 同片循环 缩放裁剪浮层改尺寸同屏验"),
	}
}
