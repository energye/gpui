package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/scene"
	"github.com/energye/gpui/render/text"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

// 全局快照引用（main 里 loadImage 后注入，供探针采样）。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

var probeKeys = []string{
	"fold_reference_command",
	"direct_draw_pixel",
	"nested_three_deep",
	"cycle_refused",
	"depth_refused",
	"k5_cluster_scene",
}

var (
	colInkA8  = [3]float64{0.89, 0.29, 0.29}
	colInkC8  = [3]float64{0.24, 0.81, 0.48}
	colInkB8  = [3]float64{0.24, 0.51, 0.89}
	colText8  = [3]float64{0.9, 0.92, 0.95}
)

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	inner := recording.NewRecorder(60, 60)
	inner.SetRGBA(colInkC8[0], colInkC8[1], colInkC8[2], 1)
	inner.DrawRectangle(10, 10, 40, 40)
	_ = inner.Fill()

	// 折叠=引用命令进外层流（批 6 语义钉，H21 SceneReplay 同款承载）。
	outer := recording.NewRecorder(100, 100)
	outer.DrawPicture(inner)
	outer.SetRGBA(1, 0, 0, 1)
	folded := recording.NewRecorder(100, 100)
	if err := outer.FinishRecording().Playback(recording.NewCanvasBackend(folded)); err != nil {
		add("fold_reference_command", false, "fold: "+err.Error())
	} else {
		// Behavior-level pin: the folded stream carries the picture by
		// reference and replays its content into a live context.
		dc := render.NewContext(100, 100)
		err2 := folded.FinishRecording().Playback(recording.NewCanvasBackend(dc))
		ok := err2 == nil && nearRGB(px(dc, 30, 30), colInkC8, 0.02)
		add("fold_reference_command", ok, fmt.Sprintf("refold err=%v center=%v", err2, px(dc, 30, 30)))
	}

	// 直绘像素：CTM 组合 + picture 内容。
	pic := recording.NewRecorder(100, 100)
	pic.SetRGBA(colInkA8[0], colInkA8[1], colInkA8[2], 1)
	pic.DrawRectangle(10, 10, 40, 40)
	_ = pic.Fill()
	pos := recording.NewRecorder(100, 100)
	pos.Translate(30, 0)
	pos.DrawPicture(pic)
	dc := render.NewContext(100, 100)
	if err := pos.FinishRecording().Playback(recording.NewCanvasBackend(dc)); err != nil {
		add("direct_draw_pixel", false, err.Error())
	} else if c := px(dc, 50, 30); nearRGB(c, colInkA8, 0.02) {
		add("direct_draw_pixel", true, "picture lands shifted by outer CTM")
	} else {
		add("direct_draw_pixel", false, fmt.Sprintf("(50,30)=%v want red", c))
	}

	// 三层嵌套。
	mid := recording.NewRecorder(60, 60)
	mid.DrawPicture(inner)
	top := recording.NewRecorder(60, 60)
	top.DrawPicture(mid)
	dc2 := render.NewContext(60, 60)
	errN := top.FinishRecording().Playback(recording.NewCanvasBackend(dc2))
	add("nested_three_deep", errN == nil && nearRGB(px(dc2, 30, 30), colInkC8, 0.02),
		fmt.Sprintf("err=%v center=%v", errN, px(dc2, 30, 30)))

	// 环：自引用回放拒绝（fail-closed 跳过+计数，不致命——哨兵计数口可见）。
	self := recording.NewRecorder(100, 100)
	self.DrawPicture(self)
	dcC := render.NewContext(100, 100)
	before := dcC.PictureRefusals()
	errC := self.FinishRecording().Playback(recording.NewCanvasBackend(dcC))
	refused := dcC.PictureRefusals() > before
	add("cycle_refused", errC == nil && refused,
		fmt.Sprintf("err=%v refusals=%d", errC, dcC.PictureRefusals()))

	// 深度：65 层链拒绝。
	prev := recording.NewRecorder(100, 100)
	root := prev
	for range 65 {
		n := recording.NewRecorder(100, 100)
		n.DrawPicture(prev)
		prev = n
	}
	errD := root.FinishRecording().Playback(recording.NewCanvasBackend(render.NewContext(100, 100)))
	add("depth_refused", errD == nil || contains_(errD.Error(), "depth"), fmt.Sprintf("err=%v", errD))

	// K5：场景级混排 Cluster 编码往返。
	add("k5_cluster_scene", k5ClusterProbe(), "scene DrawText → decode → clusters pinned")

	return out
}

func contains_(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func px(dc *render.Context, x, y int) [3]float64 {
	img, _ := dc.Image().(image.Image)
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8) / 255, float64(g >> 8) / 255, float64(b >> 8) / 255}
}

// k5ClusterProbe records mixed-script text through Scene.DrawText (shaping
// fills ShapedGlyph.Cluster), then decodes the encoding and pins the
// cluster values — the K5 carrier chain end to end.
func k5ClusterProbe() bool {
	src, err := text.NewFontSourceFromFile("render/text/testdata/wqy-microhei.ttf")
	if err != nil {
		return false
	}
	face := src.Face(20)
	sc := scene.NewScene()
	b := scene.SolidBrush(render.RGBA{R: 1, G: 1, B: 1, A: 1})
	if err := sc.DrawText("AB中", face, 10, 20, b); err != nil {
		return false
	}
	enc := sc.Encoding()
	dec := scene.NewDecoder(enc)
	if !dec.Next() || dec.Tag() != scene.TagText {
		return false
	}
	_, glyphs, _, _ := dec.Text()
	if len(glyphs) != 3 {
		return false
	}
	// Cluster is a RUNE index (shaped.go true source): "AB中" → A@0, B@1, 中@2.
	want := []int32{0, 1, 2}
	for i, w := range want {
		if glyphs[i].Cluster != w {
			return false
		}
	}
	return true
}

// --- pixel probes on the window snapshot ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_reuse_positions",
		"pixel_nested_ink",
		"pixel_cycle_refused_label",
		"pixel_mixed_text_ink",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

	regionHasColor := func(key string, card int, x0, y0, x1, y1 float64, want [3]float64) {
		ax0 := bx + cardAt[card][0] + innerX + x0
		ay0 := by + cardAt[card][1] + innerY + y0
		ax1 := bx + cardAt[card][0] + innerX + x1
		ay1 := by + cardAt[card][1] + innerY + y1
		found, samples := regionInSnapshot(img, ax0, ay0, ax1, ay1, want, dpr)
		add(key, found, fmt.Sprintf("samples=%d want=%v", samples, want))
	}

	// 卡 1：三处摆位（中间一处带 1.5 倍 CTM）都有绿方块。
	regionHasColor("pixel_reuse_positions", 0, 20, 40, 420, 240, colInkC8)
	// 卡 2：三层嵌套的绿墨。
	regionHasColor("pixel_nested_ink", 1, 40, 60, 200, 200, colInkC8)
	// 卡 3：环拒绝牌文案（绿字）。
	regionHasColor("pixel_cycle_refused_label", 2, 10, 40, 460, 100, colInkC8)
	// 卡 4：混排文本墨（前景色）。
	regionHasColor("pixel_mixed_text_ink", 3, 10, 40, 400, 120, colText8)
	return out
}

// regionInSnapshot scans a card region for a pixel near want.
func regionInSnapshot(img image.Image, ax0, ay0, ax1, ay1 float64, want [3]float64, dpr float64, tol ...float64) (bool, int) {
	t := 0.09
	if len(tol) > 0 {
		t = tol[0]
	}
	if img == nil {
		return false, 0
	}
	px0, py0 := int(math.Round(ax0*dpr)), int(math.Round(ay0*dpr))
	px1, py1 := int(math.Round(ax1*dpr)), int(math.Round(ay1*dpr))
	samples := 0
	for py := py0; py <= py1; py++ {
		for px := px0; px <= px1; px++ {
			got, ok := pixelAt(img, px, py)
			if !ok {
				continue
			}
			samples++
			if nearRGB(got, want, t) {
				return true, samples
			}
		}
	}
	return false, samples
}

func pixelAt(img image.Image, x, y int) ([3]float64, bool) {
	if img == nil {
		return [3]float64{}, false
	}
	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return [3]float64{}, false
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
}
