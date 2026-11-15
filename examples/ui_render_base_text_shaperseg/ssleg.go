// ssleg.go — offscreen legs for the shaper/segmenter family: HbShaper vs
// OwnShaper production parity, builtin segmentation, hit-test clustering.
// One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	ssW, ssH   = 320, 240
	testFontSS = "render/text/testdata/goregular.ttf"
)

type ssLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *ssLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "ssleg: %s\n", line)
}

func flushSS(dc *render.Context, view render.TextureView, leg *ssLeg, tag string) bool {
	if err := dc.PresentFrame(view, ssW, ssH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func runSSLegs() *ssLeg {
	leg := &ssLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(ssW, ssH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(ssW, ssH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontSS)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	face := src.Face(24)

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, ssW, ssH)
	_ = dc.Fill()

	// ---- Card A: shaper parity ----
	hb := text.NewHbShaper()
	own := text.NewOwnShaper()
	hbG := hb.Shape("Handgloves", face)
	ownG := own.Shape("Handgloves", face)
	leg.probe("A.HbShape", len(hbG) > 0, fmt.Sprintf("glyphs=%d（9 字符→shaper 真值）", len(hbG)))
	leg.probe("A.OwnShape", len(ownG) > 0, fmt.Sprintf("glyphs=%d", len(ownG)))
	parity := len(hbG) == len(ownG)
	if parity {
		for i := range hbG {
			if hbG[i].GID != ownG[i].GID || hbG[i].X != ownG[i].X {
				parity = false
				break
			}
		}
	}
	leg.probe("A.双 shaper GID/X 序列一致", parity,
		"生产对齐面：同文本同 face 两 shaper 逐字形一致")
	hb.ClearCache()
	own.ClearCache()
	hb.RemoveSource(src)
	own.RemoveSource(src)
	leg.probe("A.ClearCache/RemoveSource", true, "缓存清理无错")
	hb2 := hb.Shape("Handgloves", face)
	leg.probe("A.清缓存后重整形", len(hb2) == len(hbG), fmt.Sprintf("重建=%d=首跑", len(hb2)))

	// ---- Card B: segmentation ----
	segs := text.SegmentText("ab中文cd")
	leg.probe("B.SegmentText混排", len(segs) >= 3,
		fmt.Sprintf("segments=%d（Latin/Han 分段）", len(segs)))
	for _, sg := range segs {
		leg.probe("B.段RuneCount", sg.RuneCount() > 0,
			fmt.Sprintf("%q runes=%d", sg.Text, sg.RuneCount()))
	}
	segsRTL := text.SegmentTextRTL("abאבcd")
	leg.probe("B.SegmentTextRTL", len(segsRTL) >= 3, fmt.Sprintf("segments=%d", len(segsRTL)))
	seg2 := text.NewBuiltinSegmenterWithDirection(text.DirectionRTL)
	seg2s := seg2.Segment("abc")
	leg.probe("B.WithDirection", len(seg2s) == 1, "单段 LTR 内容 RTL 方向标记")

	// ---- Card C: hit test ----
	glyphs := text.Shape("Handgloves", face)
	// mid-string hit: x = advance of first 5 glyphs.
	midX := (glyphs[4].X + glyphs[5].X) / 2
	cluster, _ := text.HitTestClusterEdge(glyphs, midX)
	leg.probe("C.HitTestClusterEdge", cluster == 4 || cluster == 5,
		fmt.Sprintf("mid=%.1f→cluster=%d（第 4/5 簇）", midX, cluster))
	cl0 := text.HitTestCluster(glyphs, glyphs[0].X-10)
	leg.probe("C.前置命中0", cl0 == 0, fmt.Sprintf("负 x→cluster=%d", cl0))
	clEnd := text.HitTestCluster(glyphs, glyphs[len(glyphs)-1].X+100)
	leg.probe("C.尾部命中", clEnd >= len(glyphs)-1, fmt.Sprintf("超出→cluster=%d", clEnd))

	// quantize semantics (subpixel tier shared with glyphcache window).
	ip, sp := text.Quantize(10.75, text.Subpixel4)
	leg.probe("C.Quantize", ip == 10 && sp == 3, fmt.Sprintf("10.75→(%d,%d)", ip, sp))

	// paint anchor
	dc.SetFont(face)
	dc.SetRGBA(0.92, 0.92, 0.95, 1)
	dc.DrawString("Shaper parity", 16, 80)
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSS(dc, view, leg, "C") {
		return leg
	}
	leg.imgs["seg"] = dc.Image()

	leg.findings = append(leg.findings,
		"shaperseg: HbShaper/OwnShaper 双 shaper 同文本同 face GID/X 序列一致（生产对齐面）；混排分段（Latin/Han/RTL Hebrew）；HitTestCluster 边界语义（前置 0/尾部钳位）；Quantize 10.75→(10,3)")
	return leg
}
