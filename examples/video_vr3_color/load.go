package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// vectorCase mirrors video/color/testdata/vr3_vectors.json: stimuli and
// expectations both live in the file, this side only reads and converts.
type vectorCase struct {
	Desc          string  `json:"desc"`
	W             int     `json:"w"`
	H             int     `json:"h"`
	Y             []uint8 `json:"y"`
	Cb            []uint8 `json:"cb"`
	Cr            []uint8 `json:"cr"`
	FullRange     bool    `json:"full_range"`
	Matrix        uint32  `json:"matrix"`
	MatrixPresent bool    `json:"matrix_present"`
	WantRGBA      []uint8 `json:"want_rgba"`
}

// vectorResult is one spec case converted by the real engine.
type vectorResult struct {
	desc     string
	ours     []uint8
	want     []uint8
	w, h     int
	maxDiff  int
	diffByte int64
}

// colorGate is the whole VR3 engine verdict the window reports.
type colorGate struct {
	vectors   []*vectorResult
	maxDiff   int
	diffPct   float64
	profile   string
	frames    int
	decodeMs  float64
	convertMs float64
	realFrame *color.Frame
	realW     int
	realH     int
	err       error
}

func resolveData(names ...string) string {
	cands := []string{}
	for _, n := range names {
		cands = append(cands, n, "../../"+n)
	}
	for _, p := range cands {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return names[0]
}

func loadGate() *colorGate {
	g := &colorGate{}
	vecPath := resolveData("video/color/testdata/vr3_vectors.json")
	raw, err := os.ReadFile(vecPath)
	if err != nil {
		g.err = fmt.Errorf("向量表打不开: %w", err)
		return g
	}
	var vf struct {
		Cases []vectorCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &vf); err != nil {
		g.err = fmt.Errorf("向量表读不懂: %w", err)
		return g
	}
	var total, diff int64
	for _, vc := range vf.Cases {
		opt := color.Options{FullRange: vc.FullRange, Matrix: vc.Matrix, MatrixPresent: vc.MatrixPresent}
		f, err := color.Convert(color.SamplingYUV420P, vc.Y, vc.Cb, vc.Cr, vc.W, vc.H, opt)
		if err != nil {
			g.err = fmt.Errorf("%s 转失败: %w", vc.Desc, err)
			return g
		}
		vr := &vectorResult{desc: vc.Desc, w: vc.W, h: vc.H, want: vc.WantRGBA}
		vr.ours = append([]uint8(nil), f.Pix...)
		for i := range f.Pix {
			d := int(f.Pix[i]) - int(vc.WantRGBA[i])
			if d < 0 {
				d = -d
			}
			if d > vr.maxDiff {
				vr.maxDiff = d
			}
			if d != 0 {
				vr.diffByte++
			}
			if d > g.maxDiff {
				g.maxDiff = d
			}
		}
		total += int64(len(f.Pix))
		diff += vr.diffByte
		g.vectors = append(g.vectors, vr)
	}
	if total > 0 {
		g.diffPct = float64(diff) / float64(total) * 100
	}
	if err := g.decodeReal(); err != nil {
		g.err = err
	}
	return g
}

// decodeReal runs demux + decode + convert on a real I frame so the window
// proves the color step plugs into the decoder output, not just vectors.
func (g *colorGate) decodeReal() error {
	t0 := time.Now()
	mp4Path := resolveData("video/testdata/vr2_b_intra.mp4")
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		return fmt.Errorf("盒子打不开: %w", err)
	}
	v := movie.Video
	if v == nil {
		return fmt.Errorf("盒子里没视频轨")
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		return err
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := h264.NewDecoder(nil)
	for _, r := range avcc.SPS {
		if err := dec.DecodeNALU(r); err != nil {
			return fmt.Errorf("片头参数: %w", err)
		}
	}
	for _, r := range avcc.PPS {
		if err := dec.DecodeNALU(r); err != nil {
			return fmt.Errorf("图参数: %w", err)
		}
	}
	s := v.Samples[0]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		return fmt.Errorf("sample unreadable: %w", err)
	}
	units, err := h264.SplitAVCC(buf, avcc.LengthSize)
	if err != nil {
		return err
	}
	for _, u := range units {
		if err := dec.DecodeNALU(u); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	pic, err := dec.FinishPicture()
	if err != nil {
		return fmt.Errorf("finish: %w", err)
	}
	g.decodeMs = float64(time.Since(t0).Microseconds()) / 1000.0
	opt := color.Options{}
	if sps, err := h264.ParseSPS(avcc.SPS[0]); err == nil {
		g.profile = sps.Profile
		if sps.VUI != nil {
			opt = color.OptionsFromVUI(sps.VUI.FullRange, sps.VUI.ColourPresent, sps.VUI.ColourMatrix)
		}
	}
	t1 := time.Now()
	cf, err := color.Convert(color.SamplingYUV420P, pic.Y, pic.Cb, pic.Cr, int(pic.Width), int(pic.Height), opt)
	if err != nil {
		return fmt.Errorf("转色失败: %w", err)
	}
	g.convertMs = float64(time.Since(t1).Microseconds()) / 1000.0
	g.realFrame = cf
	g.realW, g.realH = int(pic.Width), int(pic.Height)
	g.frames = 1
	return nil
}

// patch finds a vector by keyword for side-by-side display.
func (g *colorGate) patch(key string) *vectorResult {
	for _, vr := range g.vectors {
		if containsStr(vr.desc, key) {
			return vr
		}
	}
	return nil
}

func (g *colorGate) infoLines() []string {
	if g.err != nil {
		return []string{"转色失败：" + shortErr(translateColorError(g.err.Error()), 44)}
	}
	return []string{
		fmt.Sprintf("向量 %d组 每通道最大差 %d(容差3) 差异%.4f%%", len(g.vectors), g.maxDiff, g.diffPct),
		fmt.Sprintf("真I帧 %dx%d %s 解码%.1f毫秒 转色%.3f毫秒", g.realW, g.realH, g.profile, g.decodeMs, g.convertMs),
		fmt.Sprintf("范围有限+全 601+709 注册表[%v]", color.Supported()),
	}
}
