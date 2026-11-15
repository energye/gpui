// scene.go paints the framepresent showcase: chrome comes from the shared
// direct-drive helpers (wrkit.Direct*), card content is W1-specific — all of
// it drawn through render.Context primitives; the direct-drive window has no
// rendering-tree walk, every pixel here IS the render API surface under test.
package main

import (
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// W1-specific probe anchors (saturated colors for snapshot probes + grab diffs)
var (
	colStripe1 = [3]float64{0.85, 0.30, 0.20}
	colStripe2 = [3]float64{0.20, 0.55, 0.85}
	colCheckA  = [3]float64{0.90, 0.86, 0.30}
	colCheckB  = [3]float64{0.16, 0.20, 0.34}
	colDotA    = [3]float64{0.35, 0.72, 0.55}
	colDotB    = [3]float64{0.55, 0.40, 0.72}
	colPatchL  = [3]float64{0.15, 0.75, 0.35}
	colPatchR  = [3]float64{0.85, 0.55, 0.15}
)

// offscreen leg palette (card D readbacks) — arrays can't be consts.
const (
	offW, offH = 320, 200
)

var (
	v1Base   = [3]float64{0.30, 0.34, 0.55}
	v1Bar    = [3]float64{0.16, 0.18, 0.30}
	varRight = [3]float64{0.62, 0.28, 0.38}
)

// offRectL/R: the two distant damage patches in offscreen space.
var offRectL = image.Rect(24, 40, 128, 120)
var offRectR = image.Rect(200, 96, 296, 176)

var (
	faceTitle text.Face
	faceLeg   text.Face
	faceCard  text.Face
	faceSmall text.Face
)

func ensureFaces() {
	faceTitle = wrkit.FaceAt(16)
	faceLeg = wrkit.FaceAt(12)
	faceCard = wrkit.FaceAt(13)
	faceSmall = wrkit.FaceAt(11)
}

func strokeRect(dc *render.Context, x, y, w, h float64, lw float64, c [3]float64) {
	wrkit.DirectStrokeRect(dc, x, y, w, h, lw, c)
}

func drawText(dc *render.Context, s string, x, y float64, face text.Face, c [3]float64) {
	wrkit.DirectDrawText(dc, s, x, y, face, c)
}

// paintChrome draws the top band + legend (golden region). Static strings only.
func paintChrome(dc *render.Context) {
	wrkit.PaintDirectChrome(dc, winTitle, []string{
		"A 全量呈现: PresentWith→FrameFull",
		"B 空闲帧: 无脏→Idle 零上屏",
		"C 损伤呈现: 两块远距→multi",
		"D 离屏直驱: nil present 读回",
		"probe: outcome 模式/OS 计数",
		"pixel: 快照探针+抓屏对照",
		"golden: 顶栏+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	}, faceTitle, faceLeg)
}

// cardFrame paints the shared card chrome (bg + border + title line).
func cardFrame(dc *render.Context, i int, title string) {
	wrkit.PaintDirectCardFrame(dc, i, title, faceCard)
}

// paintCardA — full-present card: full-coverage diagonal stripes + label.
func paintCardA(dc *render.Context) {
	x, y := wrkit.DirectCardAt(0)
	cardFrame(dc, 0, "A 全量呈现 PresentWith → PresentFrameFull")
	// 5 diagonal stripes across the card (deterministic geometry)
	for i := 0; i < 5; i++ {
		c := colStripe1
		if i%2 == 1 {
			c = colStripe2
		}
		bx := x + 16 + float64(i)*84
		wrkit.DirectFillRect(dc, bx, y+40, 64, wrkit.DCardH-96, c, 1)
	}
	drawText(dc, "mode=full · os_fullΔ=1 · 全帧重写", x+16, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardB — idle card: checker grid + frozen dot (idle frames must not move it).
func paintCardB(dc *render.Context) {
	x, y := wrkit.DirectCardAt(1)
	cardFrame(dc, 1, "B 空闲帧 PresentFrameAuto → Idle")
	for row := 0; row < 8; row++ {
		for col := 0; col < 12; col++ {
			c := colCheckA
			if (row+col)%2 == 1 {
				c = colCheckB
			}
			wrkit.DirectFillRect(dc, x+16+float64(col)*34, y+40+float64(row)*26, 32, 24, c, 1)
		}
	}
	drawText(dc, "idle×3: osΔ=0 · 屏像素不变", x+16, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintCardC — damage card: baseline dot grid + the two patch outlines.
func paintCardC(dc *render.Context) {
	x, y := wrkit.DirectCardAt(2)
	cardFrame(dc, 2, "C 损伤呈现 PresentFrameDamageRects")
	for row := 0; row < 9; row++ {
		for col := 0; col < 13; col++ {
			c := colDotA
			if (row+col)%2 == 1 {
				c = colDotB
			}
			dc.SetRGBA(c[0], c[1], c[2], 1)
			dc.DrawRectangle(x+18+float64(col)*32, y+44+float64(row)*26, 6, 6)
			_ = dc.Fill()
		}
	}
	drawText(dc, "multi: 远距双块 独立剪刀", x+16, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// damagePatchRects returns the two live damage patch rects (logical coords,
// inside card C, far apart: union/sum ≈ 3.4 ≥ DamageMultiWasteRatio 1.35).
func damagePatchRects() (image.Rectangle, image.Rectangle) {
	x, y := wrkit.DirectCardAt(2)
	pl := image.Rect(int(x+40), int(y+60), int(x+160), int(y+140))
	pr := image.Rect(int(x+260), int(y+170), int(x+380), int(y+250))
	return pl, pr
}

// paintDamagePatches fills only the two damage patches (the damage-phase frame:
// chrome and the rest of the card are NOT redrawn — LoadOpLoad keeps them).
func paintDamagePatches(dc *render.Context) {
	pl, pr := damagePatchRects()
	wrkit.DirectFillRect(dc, float64(pl.Min.X), float64(pl.Min.Y), float64(pl.Dx()), float64(pl.Dy()), colPatchL, 1)
	wrkit.DirectFillRect(dc, float64(pr.Min.X), float64(pr.Min.Y), float64(pr.Dx()), float64(pr.Dy()), colPatchR, 1)
}

// paintCardD — offscreen direct-drive card: readback thumbnails + plan probes.
func paintCardD(dc *render.Context, leg *offLeg) {
	x, y := wrkit.DirectCardAt(3)
	cardFrame(dc, 3, "D 离屏直驱读回 nil present")
	if leg == nil {
		drawText(dc, "offscreen: GPU 不可用（诚实降级）", x+16, y+60, faceSmall, wrkit.DColTextDim)
		return
	}
	// thumbnails: v1 / single / multi / full, 96×60 each
	thumb := func(img image.Image, ix float64, label string) {
		if img == nil {
			return
		}
		dc.DrawImageEx(render.ImageBufFromImage(img), render.DrawImageOptions{
			X: x + 16 + ix, Y: y + 36, DstWidth: 96, DstHeight: 60})
		drawText(dc, label, x+16+ix, y+112, faceSmall, wrkit.DColTextDim)
	}
	thumb(leg.imgV1, 0, "v1 full")
	thumb(leg.imgSingle, 102, "single")
	thumb(leg.imgMulti, 204, "multi")
	thumb(leg.imgFull, 306, "full")
	// plan probe lines (pure PlanFramePresent decisions, deterministic)
	if len(leg.planLines) > 0 {
		drawText(dc, leg.planLines[0], x+16, y+150, faceSmall, wrkit.DColTextDim)
	}
	if len(leg.planLines) > 1 {
		drawText(dc, leg.planLines[1], x+16, y+170, faceSmall, wrkit.DColTextDim)
	}
	drawText(dc, "PresentFrame/Damage/Rects/Auto/Full/Plan 直调 + Image() 读回",
		x+16, y+wrkit.DCardH-28, faceSmall, wrkit.DColTextDim)
}

// paintShowcase paints chrome + all four cards' final deterministic state.
func paintShowcase(dc *render.Context, leg *offLeg) {
	paintChrome(dc)
	paintCardA(dc)
	paintCardB(dc)
	paintCardC(dc)
	// card C baseline outlines over the grid
	pl, pr := damagePatchRects()
	strokeRect(dc, float64(pl.Min.X)+0.5, float64(pl.Min.Y)+0.5, float64(pl.Dx())-1, float64(pl.Dy())-1, 2, wrkit.DColTextHi)
	strokeRect(dc, float64(pr.Min.X)+0.5, float64(pr.Min.Y)+0.5, float64(pr.Dx())-1, float64(pr.Dy())-1, 2, wrkit.DColTextHi)
	paintCardD(dc, leg)
}

// paintStatus draws the dynamic status band (outside golden rects).
func paintStatus(dc *render.Context, lines []string) {
	wrkit.PaintDirectStatus(dc, lines, faceSmall)
}
