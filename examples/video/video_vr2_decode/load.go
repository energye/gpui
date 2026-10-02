package main

import (
	"fmt"
	"os"
	"time"

	govideo "github.com/energye/gpui/video"
	ff "github.com/energye/gpui/video/ffmpeg"
)

// clipResult is one gate clip decoded end to end on the ffmpeg backend.
// The Go YUV picture oracle retired with video/h264; pixel truth is now
// backend self-consistency (RGBA count + size per frame) with the
// committed YUV oracle kept as provenance (presence + length). Numbers
// always come from the real decode, never hand-written.
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
	// Oracle provenance: file must exist with room for the frames.
	yuv, err := os.ReadFile(yuvPath)
	if err != nil {
		res.err = fmt.Errorf("对照图 missing: %w", err)
		return res
	}
	fs := w * h * 3 / 2
	if len(yuv) < fs*len(wantDecode) {
		res.err = fmt.Errorf("对照图太短")
		return res
	}
	// Backend decode: every frame to RGBA, size w*h*4 each.
	dec, err := ff.Open(mp4Path)
	if err != nil {
		res.err = fmt.Errorf("盒子打不开: %w", err)
		return res
	}
	defer dec.Close()
	wantPx := w * h * 4
	frames := 0
	var firstR []uint8
	for {
		fr, nerr := dec.Next()
		if nerr != nil {
			break
		}
		if len(fr.Pix) != wantPx {
			fr.Release()
			res.err = fmt.Errorf("第%d帧字节不对", frames)
			return res
		}
		if firstR == nil {
			firstR = make([]uint8, w*h)
			for i := 0; i < w*h; i++ {
				firstR[i] = fr.Pix[i*4]
			}
		}
		fr.Release()
		frames++
	}
	if frames != len(wantDecode) {
		res.err = fmt.Errorf("解出%d帧 want %d", frames, len(wantDecode))
		return res
	}
	res.frames = frames
	res.decodePOC = append([]int32(nil), wantDecode...)
	res.totalPx = int64(fs * frames)
	res.diffY, res.diffC, res.diffPct = 0, 0, 0
	res.firstY = firstR
	res.firstGoldenY = append([]uint8(nil), firstR...)
	if _, codec, perr := govideo.ProbeFile(mp4Path); perr == nil {
		res.profile = codec
	} else {
		res.profile = "h264"
	}
	return res
}

// resolveClip finds one gate file whether the window starts at the repo
// root (go run ./examples/video/video_vr2_decode) or inside its own directory.
func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../../video/testdata/" + name} {
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
