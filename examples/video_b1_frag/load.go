package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/energye/gpui/video"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// B1 gate clips: fragmented MP4 (phone-style边录边存), same two clips as
// video/b1_ffmpeg_test.go + video/testdata/b1_ffmpeg.json. Numbers come
// from the real engine, never hand-written; baseline JSON is read for
// expect values so the window never drifts from the gate.

type b1ClipExpect struct {
	File      string `json:"file"`
	YUVFile   string `json:"yuv_file"`
	MP4Bytes  int64  `json:"mp4_bytes"`
	YUVBytes  int    `json:"yuv_bytes"`
	YUVMD5    string `json:"yuv_md5"`
	FragCount int    `json:"frag_count"`
	Stream    struct {
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		CodecTag   string `json:"codec_tag"`
		Profile    string `json:"profile"`
		ProfileIDC int    `json:"profile_idc"`
		Level      int    `json:"level"`
		NbFrames   int    `json:"nb_frames"`
	} `json:"stream"`
	Expect struct {
		Samples   int    `json:"samples"`
		Keyframes int    `json:"keyframes"`
		HasCTTS   bool   `json:"has_ctts"`
		Timescale uint32 `json:"timescale"`
		DurMs     int64  `json:"duration_ms"`
		Decoded   int    `json:"decoded"`
		Shown     int    `json:"shown"`
		Dropped   int    `json:"dropped"`
		Ended     bool   `json:"ended"`
		MonoPTS   bool   `json:"pts_monotonic"`
	} `json:"expect"`
}

type b1SeekExpect struct {
	Clip      string `json:"clip"`
	TargetMs  int64  `json:"target_ms"`
	OursFloor int64  `json:"ours_floor_ms"`
}

type b1Baseline struct {
	Clips []b1ClipExpect `json:"clips"`
	Seeks []b1SeekExpect `json:"seeks"`
}

type b1Group struct {
	Name   string `json:"name"`
	Passed int    `json:"passed"`
	Total  int    `json:"total"`
	Note   string `json:"note"`
}

type b1Evidence struct {
	Groups      []b1Group `json:"groups"`
	Passed      int       `json:"passed"`
	Total       int       `json:"total"`
	Failed      int       `json:"failed_items"`
	DecodeDiff  int64     `json:"decode_diff_px"`
	DecodeTotal int64     `json:"decode_total_px"`
	Clips       string    `json:"clips"`
	Profile     string    `json:"profile"`
	ErrText     string    `json:"err"`
}

func resolveTestdata(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func loadBaseline() (b1Baseline, error) {
	var base b1Baseline
	buf, err := os.ReadFile(resolveTestdata("b1_ffmpeg.json"))
	if err != nil {
		return base, fmt.Errorf("基线缺了: %w", err)
	}
	if err := json.Unmarshal(buf, &base); err != nil {
		return base, fmt.Errorf("基线坏了: %w", err)
	}
	if len(base.Clips) == 0 {
		return base, fmt.Errorf("基线没片子")
	}
	return base, nil
}

func profileMatch(idc byte, name string) bool {
	switch idc {
	case 66:
		return name == "Baseline" || name == "Constrained Baseline"
	case 77:
		return name == "Main"
	case 100:
		return name == "High"
	default:
		return false
	}
}

// checkHeader pins demux parity for one clip: fragmented flag, segment
// count, size, codec, profile/level, sample count, keyframe directory,
// timescale/duration. Mirrors TestB1HeaderParity, window side.
func checkHeader(clip b1ClipExpect) error {
	mp4Path := resolveTestdata(clip.File)
	fi, err := os.Stat(mp4Path)
	if err != nil {
		return fmt.Errorf("%s没了: %v", clip.File, err)
	}
	if fi.Size() != clip.MP4Bytes {
		return fmt.Errorf("%s字节%d要%d", clip.File, fi.Size(), clip.MP4Bytes)
	}
	m, err := mp4.ParseFile(mp4Path)
	if err != nil {
		return fmt.Errorf("%s打不开: %v", clip.File, err)
	}
	if !m.Fragmented {
		return fmt.Errorf("%s没标分段", clip.File)
	}
	if m.FragCount != clip.FragCount {
		return fmt.Errorf("%s段数%d要%d", clip.File, m.FragCount, clip.FragCount)
	}
	v := m.Video
	if v == nil {
		return fmt.Errorf("%s没视频轨", clip.File)
	}
	if int(v.Width) != clip.Stream.Width || int(v.Height) != clip.Stream.Height {
		return fmt.Errorf("%s尺寸%dx%d要%dx%d", clip.File, v.Width, v.Height, clip.Stream.Width, clip.Stream.Height)
	}
	if v.Codec != clip.Stream.CodecTag {
		return fmt.Errorf("%s编码%q要%q", clip.File, v.Codec, clip.Stream.CodecTag)
	}
	if len(v.AVCConfig) < 4 {
		return fmt.Errorf("%s参数太短", clip.File)
	}
	if int(v.AVCConfig[1]) != clip.Stream.ProfileIDC || !profileMatch(v.AVCConfig[1], clip.Stream.Profile) {
		return fmt.Errorf("%s档位对不上", clip.File)
	}
	if int(v.AVCConfig[3]) != clip.Stream.Level {
		return fmt.Errorf("%s等级对不上", clip.File)
	}
	if v.SampleCount != clip.Stream.NbFrames || v.SampleCount != clip.Expect.Samples {
		return fmt.Errorf("%s采样%d要%d", clip.File, v.SampleCount, clip.Expect.Samples)
	}
	if len(v.Keyframes) != clip.Expect.Keyframes {
		return fmt.Errorf("%s关键帧%d要%d", clip.File, len(v.Keyframes), clip.Expect.Keyframes)
	}
	if v.HasCTTS != clip.Expect.HasCTTS {
		return fmt.Errorf("%s有无CTTS对不上", clip.File)
	}
	if v.Timescale != clip.Expect.Timescale || v.DurationMs != clip.Expect.DurMs {
		return fmt.Errorf("%s时间对不上", clip.File)
	}
	if v.FragCount != clip.FragCount {
		return fmt.Errorf("%s轨段数对不上", clip.File)
	}
	return nil
}

// checkDecodeExact pins picture parity for one clip: every frame vs the
// ffmpeg oracle byte-exact. Returns passed frames, total frames, diff
// pixels. Mirrors TestB1DecodeExact, window side.
func checkDecodeExact(clip b1ClipExpect) (passed, total int, diffPx int64, err error) {
	mp4Path := resolveTestdata(clip.File)
	yuvPath := resolveTestdata(clip.YUVFile)
	yuv, err := os.ReadFile(yuvPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("对照图没了: %v", err)
	}
	if len(yuv) != clip.YUVBytes {
		return 0, 0, 0, fmt.Errorf("对照图字节对不上")
	}
	sum := md5.Sum(yuv)
	if got := hex.EncodeToString(sum[:]); got != clip.YUVMD5 {
		return 0, 0, 0, fmt.Errorf("对照图md5变了")
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		return 0, 0, 0, err
	}
	v := movie.Video
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		return 0, 0, 0, err
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	dec := h264.NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return 0, 0, 0, err
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return 0, 0, 0, err
		}
	}
	var pics []*h264.Picture
	for i, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			return 0, 0, 0, fmt.Errorf("采样%d读不出: %v", i, err)
		}
		units, err := h264.SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			return 0, 0, 0, err
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				return 0, 0, 0, fmt.Errorf("采样%d解不出: %v", i, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			return 0, 0, 0, fmt.Errorf("采样%d收不齐: %v", i, err)
		}
		pics = append(pics, pic)
	}
	if len(pics) != clip.Expect.Samples {
		return 0, 0, 0, fmt.Errorf("解出%d帧要%d", len(pics), clip.Expect.Samples)
	}
	ordered := pics
	if clip.File == "b1_frag5.mp4" {
		ordered = append([]*h264.Picture(nil), pics...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].POC < ordered[j].POC })
	}
	w, h := clip.Stream.Width, clip.Stream.Height
	fs := w * h * 3 / 2
	passed = 0
	for fi, pic := range ordered {
		if int(pic.Width) != w || int(pic.Height) != h {
			return passed, len(ordered), diffPx, fmt.Errorf("第%d帧尺寸不对", fi)
		}
		ey := yuv[fi*fs : fi*fs+w*h]
		ecb := yuv[fi*fs+w*h : fi*fs+w*h+w*h/4]
		ecr := yuv[fi*fs+w*h+w*h/4 : (fi+1)*fs]
		frameDiff := int64(0)
		for i := range ey {
			if pic.Y[i] != ey[i] {
				frameDiff++
			}
		}
		for i := range ecb {
			if pic.Cb[i] != ecb[i] {
				frameDiff++
			}
			if pic.Cr[i] != ecr[i] {
				frameDiff++
			}
		}
		diffPx += frameDiff
		if frameDiff == 0 {
			passed++
		}
	}
	return passed, len(ordered), diffPx, nil
}

type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

// checkPlayToEnd pins playback parity for one clip: hand-clock play to
// Ended with zero drops and monotonic stamps. Mirrors TestB1PlayToEnd.
func checkPlayToEnd(clip b1ClipExpect) error {
	path := resolveTestdata(clip.File)
	h := &handClock{}
	p, err := video.OpenFile(path, video.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	info := p.Info()
	if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
		return fmt.Errorf("尺寸对不上")
	}
	step := int64(200)
	if info.FrameRate > 1 {
		step = int64(float64(1000)/info.FrameRate + 0.5)
		if step < 1 {
			step = 1
		}
	}
	var seqs, pts []int64
	ended := false
	deadline := time.Now().Add(90 * time.Second)
	loops := 4*clip.Stream.NbFrames + 8
	if clip.Stream.NbFrames > 64 {
		loops = clip.Stream.NbFrames + 8
	}
	for i := 0; i < loops && !ended; i++ {
		if time.Now().After(deadline) {
			return fmt.Errorf("播不完")
		}
		h.now += step
		fr, done := p.Poll()
		if fr != nil {
			seqs = append(seqs, fr.Seq)
			pts = append(pts, fr.PTSMs)
		}
		ended = done
		if clip.Stream.NbFrames > 64 {
			runtime.Gosched()
			time.Sleep(time.Millisecond)
		}
	}
	if !ended {
		return fmt.Errorf("没播到尾")
	}
	st := p.Stats()
	if st.Decoded != int64(clip.Expect.Decoded) || st.Shown != int64(clip.Expect.Shown) {
		return fmt.Errorf("解码/显示对不上")
	}
	if st.Dropped != int64(clip.Expect.Dropped) {
		return fmt.Errorf("丢帧对不上")
	}
	for i, s := range seqs {
		if s != int64(i) {
			return fmt.Errorf("顺序对不上")
		}
	}
	if clip.Expect.MonoPTS {
		for i := 1; i < len(pts); i++ {
			if pts[i] <= pts[i-1] {
				return fmt.Errorf("时间戳不单调")
			}
		}
	}
	if !st.Ended {
		return fmt.Errorf("没标播完")
	}
	return nil
}

// checkSeekFloor pins seek parity for one target: floor identity plus
// first show at/after landing, monotonic, within reorder delay.
// Mirrors TestB1SeekFloor.
func checkSeekFloor(clip b1ClipExpect, targetMs, wantFloor int64) error {
	path := resolveTestdata(clip.File)
	h := &handClock{}
	p, err := video.OpenFile(path, video.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	movie, err := mp4.ParseFile(path)
	if err != nil {
		return err
	}
	wantPos, wantLanded := -1, int64(0)
	for i, s := range movie.Video.Samples {
		if s.PTSMs <= targetMs && (wantPos < 0 || s.PTSMs > wantLanded) {
			wantPos, wantLanded = i, s.PTSMs
		}
	}
	if wantPos < 0 || wantLanded != wantFloor {
		return fmt.Errorf("地板对不上")
	}
	landed, err := p.SeekTo(targetMs)
	if err != nil {
		return fmt.Errorf("跳不动: %v", err)
	}
	wantShow := wantLanded
	if clip.File == "b1_frag5.mp4" {
		key := movie.Video.Keyframes[0]
		for _, k := range movie.Video.Keyframes[1:] {
			if k.PTSMs <= targetMs {
				key = k
			} else {
				break
			}
		}
		keyPos := -1
		for i, s := range movie.Video.Samples {
			if s.Number == key.SampleNumber {
				keyPos = i
				break
			}
		}
		if keyPos > wantPos {
			wantShow = movie.Video.Samples[keyPos].PTSMs
		}
	}
	if landed != wantShow {
		return fmt.Errorf("落点对不上")
	}
	h.now = landed
	var stamps []int64
	for i := 0; i < 100 && len(stamps) < 3; i++ {
		h.now += 200
		fr, _ := p.Poll()
		if fr != nil {
			stamps = append(stamps, fr.PTSMs)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if len(stamps) == 0 {
		return fmt.Errorf("跳后黑屏")
	}
	if stamps[0] < landed {
		return fmt.Errorf("首现早于落点")
	}
	for i := 1; i < len(stamps); i++ {
		if stamps[i] <= stamps[i-1] {
			return fmt.Errorf("跳后时间不单调")
		}
	}
	if stamps[0]-landed > 800 {
		return fmt.Errorf("落点恢复太慢")
	}
	return nil
}

// loadB1 runs the four B1 gates and folds them into groups/items counts:
// header 2 clips + decode 105 frames + play 2 clips + seek 5 jumps = 114.
func loadB1() b1Evidence {
	ev := b1Evidence{}
	base, err := loadBaseline()
	if err != nil {
		ev.ErrText = err.Error()
		return ev
	}
	names := ""
	for i, c := range base.Clips {
		if i > 0 {
			names += "+"
		}
		names += c.File
	}
	ev.Clips = names
	ev.Profile = "Main"

	// Group header: 2 clips.
	hdrPass := 0
	hdrNote := ""
	for _, c := range base.Clips {
		if err := checkHeader(c); err != nil {
			if hdrNote == "" {
				hdrNote = c.File + "：" + err.Error()
			}
		} else {
			hdrPass++
		}
	}
	if hdrNote == "" {
		hdrNote = "分段头全对"
	}
	ev.Groups = append(ev.Groups, b1Group{Name: "header", Passed: hdrPass, Total: len(base.Clips), Note: hdrNote})

	// Group decode: per-frame exact.
	decPass, decTotal := 0, 0
	decNote := ""
	for _, c := range base.Clips {
		p, t, diff, err := checkDecodeExact(c)
		decTotal += t
		ev.DecodeTotal += int64(t * c.Stream.Width * c.Stream.Height * 3 / 2)
		if err != nil {
			if decNote == "" {
				decNote = c.File + "：" + err.Error()
			}
			continue
		}
		decPass += p
		ev.DecodeDiff += diff
		if p != t && decNote == "" {
			decNote = c.File + "有坏帧"
		}
	}
	if decNote == "" {
		decNote = "逐字节全对"
	}
	ev.Groups = append(ev.Groups, b1Group{Name: "decode", Passed: decPass, Total: decTotal, Note: decNote})

	// Group play: 2 clips to Ended.
	playPass := 0
	playNote := ""
	for _, c := range base.Clips {
		if err := checkPlayToEnd(c); err != nil {
			if playNote == "" {
				playNote = c.File + "：" + err.Error()
			}
		} else {
			playPass++
		}
	}
	if playNote == "" {
		playNote = "播到尾零丢"
	}
	ev.Groups = append(ev.Groups, b1Group{Name: "play", Passed: playPass, Total: len(base.Clips), Note: playNote})

	// Group seek: baseline seeks.
	byClip := map[string]b1ClipExpect{}
	for _, c := range base.Clips {
		byClip[c.File] = c
	}
	seekPass := 0
	seekNote := ""
	for _, sk := range base.Seeks {
		clip, ok := byClip[sk.Clip]
		if !ok {
			if seekNote == "" {
				seekNote = sk.Clip + "没在基线里"
			}
			continue
		}
		if err := checkSeekFloor(clip, sk.TargetMs, sk.OursFloor); err != nil {
			if seekNote == "" {
				seekNote = fmt.Sprintf("%s/%d：%s", sk.Clip, sk.TargetMs, err.Error())
			}
		} else {
			seekPass++
		}
	}
	if seekNote == "" {
		seekNote = "落地板全对"
	}
	ev.Groups = append(ev.Groups, b1Group{Name: "seek", Passed: seekPass, Total: len(base.Seeks), Note: seekNote})

	for _, g := range ev.Groups {
		ev.Passed += g.Passed
		ev.Total += g.Total
	}
	ev.Failed = ev.Total - ev.Passed
	if ev.Failed < 0 {
		ev.Failed = 0
	}
	if ev.ErrText == "" {
		for _, g := range ev.Groups {
			if g.Passed != g.Total {
				ev.ErrText = g.Name + "：" + g.Note
				break
			}
		}
	}
	return ev
}

func (ev b1Evidence) infoLines() []string {
	var out []string
	for _, g := range ev.Groups {
		out = append(out, fmt.Sprintf("%s %d/%d %s", g.Name, g.Passed, g.Total, g.Note))
	}
	return out
}
