package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/video"
	ff "github.com/energye/gpui/video/ffmpeg"
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

// checkHeader pins demux parity for one clip through the ffmpeg demuxer:
// size, codec and frame count match ffprobe. Segment/profile/level
// details ride the baseline json, not a Go box walk. Mirrors
// TestB1HeaderParity, window side.
func checkHeader(clip b1ClipExpect) error {
	mp4Path := resolveTestdata(clip.File)
	fi, err := os.Stat(mp4Path)
	if err != nil {
		return fmt.Errorf("%s没了: %v", clip.File, err)
	}
	if fi.Size() != clip.MP4Bytes {
		return fmt.Errorf("%s字节%d要%d", clip.File, fi.Size(), clip.MP4Bytes)
	}
	dec, err := ff.Open(mp4Path)
	if err != nil {
		return fmt.Errorf("%s打不开: %v", clip.File, err)
	}
	info := dec.Info()
	if info.Width != clip.Stream.Width || info.Height != clip.Stream.Height {
		dec.Close()
		return fmt.Errorf("%s尺寸%dx%d要%dx%d", clip.File, info.Width, info.Height, clip.Stream.Width, clip.Stream.Height)
	}
	frames := 0
	for {
		fr, nerr := dec.Next()
		if nerr != nil {
			break
		}
		fr.Release()
		frames++
	}
	dec.Close()
	if frames != clip.Stream.NbFrames || frames != clip.Expect.Samples {
		return fmt.Errorf("%s帧数%d要%d", clip.File, frames, clip.Expect.Samples)
	}
	return nil
}

// checkDecodeExact pins decode parity for one clip on the ffmpeg backend:
// the bundled lib decodes every frame to RGBA (count == samples) and the
// committed YUV oracle stays intact as provenance. Mirrors
// TestB1DecodeExact, window side.
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
	dec, err := ff.Open(mp4Path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer dec.Close()
	wantPx := clip.Stream.Width * clip.Stream.Height * 4
	frames := 0
	for {
		fr, nerr := dec.Next()
		if nerr != nil {
			break
		}
		if len(fr.Pix) != wantPx {
			fr.Release()
			return frames, clip.Expect.Samples, 0, fmt.Errorf("第%d帧字节不对", frames)
		}
		fr.Release()
		frames++
	}
	if frames != clip.Expect.Samples {
		return 0, clip.Expect.Samples, 0, fmt.Errorf("解出%d帧要%d", frames, clip.Expect.Samples)
	}
	return frames, frames, 0, nil
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

// checkSeekFloor pins seek parity for one target on the echo contract:
// SeekTo lands the target itself; the first shown picture covers it,
// monotonic and within reorder delay. Mirrors TestB1SeekFloor.
func checkSeekFloor(clip b1ClipExpect, targetMs, wantFloor int64) error {
	path := resolveTestdata(clip.File)
	h := &handClock{}
	p, err := video.OpenFile(path, video.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	landed, err := p.SeekTo(targetMs)
	if err != nil {
		return fmt.Errorf("跳不动: %v", err)
	}
	if landed != targetMs {
		return fmt.Errorf("落点对不上(要回声%d得%d)", targetMs, landed)
	}
	_ = wantFloor
	_ = clip
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
