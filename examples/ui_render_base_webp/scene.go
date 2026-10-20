package main

import (
	"fmt"
	"image"
	"math"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
	contentW     = cardW - 2*contentX0 // 406
	contentH     = cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

const (
	halfW = 199.0
	halfX = halfW + 8.0

	losslessScale = 2.0
	losslessX     = contentX0 + 3
	losslessY     = contentY0 + 28

	zoomCropX, zoomCropY = 64, 36
	zoomCropW, zoomCropH = 24, 24
	zoomScale            = 4.0
	zoomX                = contentX0 + halfX + 12
	zoomY                = contentY0 + 8

	refX = contentX0 + halfX + 12
	refY = contentY0 + 112

	lossyX = contentX0 + 20
	lossyY = contentY0 + 32
	lossyW = 160
	lossyH = 120

	lossyZoomCropX, lossyZoomCropY = 40, 30
	lossyZoomCropW, lossyZoomCropH = 80, 60
	lossyZoomScale                 = 2.0
	lossyZoomX                     = contentX0 + halfX + 16
	lossyZoomY                     = contentY0 + 32

	cellW, cellH = 199.0, 92.0
	thumbW       = 96.0
	thumbH       = 64.0
)

var (
	colCardBG  = [3]float64{0.10, 0.11, 0.13}
	colBorder  = [3]float64{0.32, 0.36, 0.44}
	checkerA   = [3]float64{0.18, 0.20, 0.24}
	checkerB   = [3]float64{0.30, 0.33, 0.39}
	checkerSz  = 16.0
	webpAccent = [3]float64{0.95, 0.72, 0.25}
)

// webpAsset is one decoded image plus the standard-library view used by the
// pixel oracle.
type webpAsset struct {
	buf *render.ImageBuf
	std image.Image
}

var (
	assetOnce sync.Once
	assetErr  error

	losslessWebP *webpAsset // LoadWebP(.webp)
	byExtWebP    *webpAsset // LoadImage(.webp)
	byBodyWebP   *webpAsset // LoadImage(.bin), content sniffing
	refPNG       *webpAsset // LoadImage(.png), lossless reference
	lossyWebP    *webpAsset // LoadWebP(lossy .webp)

	losslessZoom *webpAsset // pre-baked 24x24 crop of the lossless asset
	lossyZoom    *webpAsset // pre-baked 80x60 crop of the lossy asset
)

const assetDir = "examples/ui_render_base_webp/testdata"

func loadWebPAsset(name string) (*webpAsset, error) {
	path := assetDir + "/" + name
	buf, err := render.LoadWebP(path)
	if err != nil {
		return nil, err
	}
	std := buf.ToStdImage()
	if std == nil {
		buf.Dispose()
		return nil, fmt.Errorf("%s: ToStdImage returned nil", path)
	}
	return &webpAsset{buf: buf, std: std}, nil
}

func loadImageAsset(name string) (*webpAsset, error) {
	path := assetDir + "/" + name
	buf, err := render.LoadImage(path)
	if err != nil {
		return nil, err
	}
	std := buf.ToStdImage()
	if std == nil {
		buf.Dispose()
		return nil, fmt.Errorf("%s: ToStdImage returned nil", path)
	}
	return &webpAsset{buf: buf, std: std}, nil
}

func ensureAssets() {
	assetOnce.Do(func() {
		var err error
		if losslessWebP, err = loadWebPAsset("webp_lossless.webp"); err != nil {
			assetErr = err
			return
		}
		if byExtWebP, err = loadImageAsset("webp_lossless.webp"); err != nil {
			assetErr = err
			return
		}
		if byBodyWebP, err = loadImageAsset("webp_lossless.bin"); err != nil {
			assetErr = err
			return
		}
		if refPNG, err = loadImageAsset("webp_lossless_ref.png"); err != nil {
			assetErr = err
			return
		}
		if lossyWebP, err = loadWebPAsset("webp_lossy.webp"); err != nil {
			assetErr = err
			return
		}
		if losslessZoom, err = bakeCrop(losslessWebP, zoomCropX, zoomCropY, zoomCropW, zoomCropH); err != nil {
			assetErr = err
			return
		}
		if lossyZoom, err = bakeCrop(lossyWebP, lossyZoomCropX, lossyZoomCropY, lossyZoomCropW, lossyZoomCropH); err != nil {
			assetErr = err
		}
	})
}

// bakeCrop copies one source-space rectangle into its own ImageBuf once. The
// window then scales that buffer with DrawImageEx; doing this per frame would
// add an unnecessary readback to every paint.
func bakeCrop(a *webpAsset, sx, sy, sw, sh int) (*webpAsset, error) {
	if a == nil || a.buf == nil {
		return nil, fmt.Errorf("bakeCrop: nil source")
	}
	if sx < 0 || sy < 0 || sx+sw > a.buf.Width() || sy+sh > a.buf.Height() {
		return nil, fmt.Errorf("bakeCrop: crop %dx%d at %d,%d outside %dx%d", sw, sh, sx, sy, a.buf.Width(), a.buf.Height())
	}
	dc := render.NewContext(sw, sh)
	dc.DrawImageEx(a.buf, render.DrawImageOptions{
		X: -float64(sx), Y: -float64(sy),
		DstWidth: float64(a.buf.Width()), DstHeight: float64(a.buf.Height()),
		Interpolation: render.InterpNearest,
	})
	std := dc.Image()
	dc.Close()
	if std == nil {
		return nil, fmt.Errorf("bakeCrop: Image returned nil")
	}
	buf := render.ImageBufFromImage(std)
	if buf == nil {
		return nil, fmt.Errorf("bakeCrop: ImageBufFromImage returned nil")
	}
	return &webpAsset{buf: buf, std: buf.ToStdImage()}, nil
}

func disposeAssets() {
	for _, a := range []*webpAsset{losslessWebP, byExtWebP, byBodyWebP, refPNG, lossyWebP, losslessZoom, lossyZoom} {
		if a != nil && a.buf != nil {
			a.buf.Dispose()
		}
	}
}

// checkerColor is the independent background oracle under transparent WebP
// pixels. Coordinates are relative to the checker's top-left corner, matching
// drawChecker's cell origin (not the card content origin).
func checkerColor(relX, relY float64) [3]float64 {
	cx := int(relX / checkerSz)
	cy := int(relY / checkerSz)
	if (cx+cy)&1 == 0 {
		return checkerA
	}
	return checkerB
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildWebPScene(shell *wrkit.ShellChrome) {
	ensureAssets()
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A 无损 WebP + Alpha：2× / 4× 裁片 / 1× PNG 参考", paintA)
	card(1, "B 有损 WebP q80：1× / 2× 最近邻", paintB)
	card(2, "C 四条加载路径：同一个无损源", paintC)
	card(3, "D 失败模式：缺失 / 非法 / 空数据", paintD)

	// C card path labels sit above each thumbnail, outside the bitmap draw.
	labels := []struct {
		text string
		x, y float64
	}{
		{"LoadWebP(.webp)", cardAt[2][0] + contentX0 + 4, cardAt[2][1] + contentY0 + 2},
		{"LoadImage(.webp)", cardAt[2][0] + contentX0 + halfX + 4, cardAt[2][1] + contentY0 + 2},
		{"LoadImage(.bin)", cardAt[2][0] + contentX0 + 4, cardAt[2][1] + contentY0 + cellH + 2},
		{"LoadPNG(ref)", cardAt[2][0] + contentX0 + halfX + 4, cardAt[2][1] + contentY0 + cellH + 2},
	}
	for _, l := range labels {
		shell.Body.Place(wrkit.Label(l.text, 9.5, 0.62, 0.76, 0.88), l.x, l.y)
	}

	// D card rows: icon + label, all static. The actual error contract is
	// asserted by the value-level probes; this card makes the result visible.
	rows := []struct {
		text string
		y    float64
	}{
		{"缺失文件 → error", cardAt[3][1] + contentY0 + 24},
		{"PNG 数据当 WebP → error", cardAt[3][1] + contentY0 + 70},
		{"空文件 → error", cardAt[3][1] + contentY0 + 116},
		{"三条都返回 error，无 panic", cardAt[3][1] + contentY0 + 162},
	}
	for _, r := range rows {
		shell.Body.Place(wrkit.Label(r.text, 11, 0.78, 0.84, 0.92),
			cardAt[3][0]+contentX0+30, r.y)
	}
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func slotFrame(dc *render.Context, x, y, w, h float64) {
	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.DrawRectangle(x, y, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(x, y+h-1, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(x, y, 1, h)
	_ = dc.Fill()
	dc.DrawRectangle(x+w-1, y, 1, h)
	_ = dc.Fill()
}

func drawChecker(dc *render.Context, x, y, w, h float64) {
	for row := 0; float64(row)*checkerSz < h; row++ {
		cellY := y + float64(row)*checkerSz
		cellH := math.Min(checkerSz, y+h-cellY)
		for col := 0; float64(col)*checkerSz < w; col++ {
			cellX := x + float64(col)*checkerSz
			cellW := math.Min(checkerSz, x+w-cellX)
			c := checkerA
			if (row+col)&1 != 0 {
				c = checkerB
			}
			dc.SetRGBA(c[0], c[1], c[2], 1)
			dc.DrawRectangle(cellX, cellY, cellW, cellH)
			_ = dc.Fill()
		}
	}
}

func drawAsset(pc *rendering.PaintContext, a *webpAsset, x, y, w, h float64, nearest bool) {
	if pc == nil || pc.DC == nil || a == nil || a.buf == nil {
		return
	}
	ax, ay := pc.Abs(x, y)
	interp := render.InterpBilinear
	if nearest {
		interp = render.InterpNearest
	}
	pc.DC.DrawImageEx(a.buf, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: w, DstHeight: h, Interpolation: interp,
	})
}

func drawErrorBadge(dc *render.Context, x, y, size float64) {
	dc.SetRGBA(0.86, 0.20, 0.18, 1)
	dc.DrawRectangle(x, y, size, size)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	dc.SetLineWidth(2)
	dc.DrawLine(x+4, y+4, x+size-4, y+size-4)
	_ = dc.Stroke()
	dc.DrawLine(x+size-4, y+4, x+4, y+size-4)
	_ = dc.Stroke()
}

func paintA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureAssets()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	// Left: 2x lossless WebP over a checkerboard so alpha=0 is visible.
	drawChecker(dc, ox+contentX0, oy+contentY0, halfW, contentH)
	slotFrame(dc, ox+contentX0, oy+contentY0, halfW, contentH)
	drawAsset(pc, losslessWebP, losslessX, losslessY, 96*losslessScale, 64*losslessScale, true)

	// Right top: 4x nearest crop showing the checker block and alpha hole.
	slotFrame(dc, ox+zoomX, oy+zoomY, zoomCropW*zoomScale, zoomCropH*zoomScale)
	drawAsset(pc, losslessZoom, zoomX, zoomY, zoomCropW*zoomScale, zoomCropH*zoomScale, true)

	// Right bottom: the PNG reference at 1x. The pixels must match WebP.
	slotFrame(dc, ox+refX, oy+refY, thumbW, thumbH)
	drawAsset(pc, refPNG, refX, refY, thumbW, thumbH, true)
}

func paintB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureAssets()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	slotFrame(dc, ox+lossyX, oy+lossyY, lossyW, lossyH)
	drawAsset(pc, lossyWebP, lossyX, lossyY, lossyW, lossyH, false)

	slotFrame(dc, ox+lossyZoomX, oy+lossyZoomY, lossyZoomCropW*lossyZoomScale, lossyZoomCropH*lossyZoomScale)
	drawAsset(pc, lossyZoom, lossyZoomX, lossyZoomY, lossyZoomCropW*lossyZoomScale, lossyZoomCropH*lossyZoomScale, true)
}

func paintC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureAssets()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	assets := []*webpAsset{losslessWebP, byExtWebP, byBodyWebP, refPNG}
	cells := [4][2]float64{
		{contentX0, contentY0},
		{contentX0 + halfX, contentY0},
		{contentX0, contentY0 + cellH},
		{contentX0 + halfX, contentY0 + cellH},
	}
	for i, a := range assets {
		x, y := cells[i][0]+(cellW-thumbW)/2, cells[i][1]+22
		slotFrame(dc, ox+x-1, oy+y-1, thumbW+2, thumbH+2)
		drawAsset(pc, a, x, y, thumbW, thumbH, true)
	}
}

func paintD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	ys := []float64{contentY0 + 24, contentY0 + 70, contentY0 + 116, contentY0 + 162}
	for i, y := range ys {
		if i == 3 {
			// Final row is the successful "no panic" control, drawn green.
			dc.SetRGBA(0.25, 0.72, 0.40, 1)
			dc.DrawRectangle(ox+contentX0+4, oy+y, 16, 16)
			_ = dc.Fill()
			dc.SetRGBA(1, 1, 1, 1)
			dc.SetLineWidth(2)
			dc.DrawLine(ox+contentX0+8, oy+y+8, ox+contentX0+11, oy+y+12)
			_ = dc.Stroke()
			dc.DrawLine(ox+contentX0+11, oy+y+12, ox+contentX0+17, oy+y+3)
			_ = dc.Stroke()
			continue
		}
		drawErrorBadge(dc, ox+contentX0+4, oy+y, 16)
	}

	// A small footer strip makes the "decoded, not placeholder" result
	// visible even if the human only looks at the last card.
	dc.SetRGBA(webpAccent[0], webpAccent[1], webpAccent[2], 1)
	dc.DrawRectangle(ox+contentX0+4, oy+contentY0+contentH, contentW-8, 2)
	_ = dc.Fill()
}
