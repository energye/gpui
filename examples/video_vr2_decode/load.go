package main

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// clipResult is one gate clip decoded end to end and compared against
// its ffmpeg oracle. Numbers always come from the real decode, never
// hand-written.
type clipResult struct {
	name         string
	source       string
	width        int
	height       int
	profile      string
	frames       int
	decodePOC    []int32
	diffY        int64
	diffC        int64
	totalPx      int64
	diffPct      float64
	decodeMs     float64
	firstY       []uint8
	firstGoldenY []uint8
	err          error
}

func decodeOne(name, mp4Path, yuvPath string, w, h int, wantDecode []int32) *clipResult {
	t0 := time.Now()
	res := &clipResult{name: name, source: mp4Path, width: w, height: h}
	defer func() {
		res.decodeMs = float64(time.Since(t0).Microseconds()) / 1000.0
	}()
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		res.err = fmt.Errorf("盒子打不开: %w", err)
		return res
	}
	v := movie.Video
	if v == nil {
		res.err = fmt.Errorf("无视频轨")
		return res
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		res.err = err
		return res
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		res.err = err
		return res
	}
	defer f.Close()
	dec := h264.NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			res.err = fmt.Errorf("片头参数: %w", err)
			return res
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			res.err = fmt.Errorf("图参数: %w", err)
			return res
		}
	}
	yuv, err := os.ReadFile(yuvPath)
	if err != nil {
		res.err = fmt.Errorf("对照图 missing: %w", err)
		return res
	}
	fs := w * h * 3 / 2
	var pics []*h264.Picture
	for _, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			res.err = fmt.Errorf("sample %d unreadable: %w", s.Number, err)
			return res
		}
		units, err := h264.SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			res.err = err
			return res
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				res.err = fmt.Errorf("sample %d: %w", s.Number, err)
				return res
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			res.err = fmt.Errorf("finish sample %d: %w", s.Number, err)
			return res
		}
		pics = append(pics, pic)
	}
	if len(pics) != len(wantDecode) {
		res.err = fmt.Errorf("解出%d帧 want %d", len(pics), len(wantDecode))
		return res
	}
	for i, p := range pics {
		res.decodePOC = append(res.decodePOC, p.POC)
		if p.POC != wantDecode[i] {
			res.err = fmt.Errorf("解码序对不上 want %v got %v", wantDecode, res.decodePOC)
			return res
		}
		if p.Width != uint32(w) || p.Height != uint32(h) {
			res.err = fmt.Errorf("frame %d size = %dx%d want %dx%d", i, p.Width, p.Height, w, h)
			return res
		}
	}
	if len(yuv) < fs*len(pics) {
		res.err = fmt.Errorf("对照图太短")
		return res
	}
	byDisplay := append([]*h264.Picture(nil), pics...)
	sort.Slice(byDisplay, func(i, j int) bool { return byDisplay[i].POC < byDisplay[j].POC })
	for di, pic := range byDisplay {
		for j := range pic.Y {
			if pic.Y[j] != yuv[di*fs+j] {
				res.diffY++
			}
		}
		for j := range pic.Cb {
			if pic.Cb[j] != yuv[di*fs+w*h+j] {
				res.diffC++
			}
			if pic.Cr[j] != yuv[di*fs+w*h+w*h/4+j] {
				res.diffC++
			}
		}
		if di == 0 {
			res.firstY = append([]uint8(nil), pic.Y...)
			res.firstGoldenY = append([]uint8(nil), yuv[:w*h]...)
		}
	}
	res.frames = len(pics)
	res.totalPx = int64(fs * len(pics))
	res.diffPct = float64(res.diffY+res.diffC) / float64(res.totalPx) * 100
	if sps, err := h264.ParseSPS(avcc.SPS[0]); err == nil {
		res.profile = sps.Profile
	}
	return res
}

// resolveClip finds one gate file whether the window starts at the repo
// root (go run ./examples/video_vr2_decode) or inside its own directory.
func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func loadDecode() []*clipResult {
	return []*clipResult{
		decodeOne("B门禁96x96", resolveClip("vr2_m_bframes.mp4"), resolveClip("vr2_m_bframes.yuv"), 96, 96, []int32{0, 6, 2, 4, 8}),
		decodeOne("480p裁边", resolveClip("vr2_480p.mp4"), resolveClip("vr2_480p.yuv"), 854, 480, []int32{0, 4, 2, 8, 6}),
		decodeOne("720p", resolveClip("vr2_720p.mp4"), resolveClip("vr2_720p.yuv"), 1280, 720, []int32{0, 4, 2, 8, 6}),
	}
}

func (res *clipResult) infoLine() string {
	if res.err != nil {
		return fmt.Sprintf("%s 解码失败：%s", res.name, shortErr(translateDecodeError(res.err.Error()), 44))
	}
	return fmt.Sprintf("%s %s %d帧 差异%.4f%%(%d/%d点) %.1f毫秒", res.name, res.profile, res.frames,
		res.diffPct, res.diffY+res.diffC, res.totalPx, res.decodeMs)
}
