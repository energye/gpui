// Package main — Image axis scene (ENGINE_UI_RENDER_BASE FImg-* / §24).
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	uiio "github.com/energye/gpui/ui/io"
	"github.com/energye/gpui/ui/rendering"
)

type pendingImg struct {
	img *render.ImageBuf
	err error
	tag string
}

type imageScene struct {
	Root *rendering.AbsoluteBox

	AsyncRO   *rendering.RenderImage
	SyncRO    *rendering.RenderImage
	DCPanel   *rendering.RenderBox
	StatusTxt *rendering.RenderText

	src         *render.ImageBuf // synthetic source for DC demos
	pending     chan pendingImg
	tempPNG     string
	face        text.Face
	phase       float64
	swapCount   int
	status      string
	decodeOnce  sync.Once
	applyMu     sync.Mutex
	lastApplied string
}

func label(s string, size float64, r, g, b float64, face text.Face) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	if face != nil {
		t.SetFace(face)
	}
	return t
}

func panelBox(w, h float64, paint func(pc *rendering.PaintContext, size rendering.Size)) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	b.SetRepaintBoundary(true)
	b.OnPaint = paint
	return b
}

func fillRect(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Fill()
}

func strokeRect(pc *rendering.PaintContext, x, y, w, h, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Stroke()
}

// makeChecker creates an in-memory checkerboard ImageBuf (FImg synthetic source).
func makeChecker(w, h, cell int) (*render.ImageBuf, error) {
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil, err
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := ((x/cell)+(y/cell))%2 == 0
			if on {
				_ = img.SetRGBA(x, y, 40, 120, 220, 255)
			} else {
				_ = img.SetRGBA(x, y, 240, 200, 60, 255)
			}
		}
	}
	// corner markers for SrcRect visibility
	for i := 0; i < 8; i++ {
		_ = img.SetRGBA(i, 0, 255, 0, 0, 255)
		_ = img.SetRGBA(0, i, 255, 0, 0, 255)
		_ = img.SetRGBA(w-1-i, h-1, 0, 255, 0, 255)
		_ = img.SetRGBA(w-1, h-1-i, 0, 255, 0, 255)
	}
	return img, nil
}

// writePNG exports a std image for ui/io.DecodeFile (async path).
func writeTempPNG(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// smooth vertical gradient + diagonal band
			r := uint8(30 + x*200/w)
			g := uint8(40 + y*180/h)
			b := uint8(90 + ((x + y) * 80 / (w + h)))
			if (x+y)/12%2 == 0 {
				r, g = g, r
			}
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func buildImageScene(winW, winH float64, face text.Face) (*imageScene, error) {
	s := &imageScene{
		face:    face,
		pending: make(chan pendingImg, 4),
		status:  "loading…",
	}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	s.Root = root

	root.Place(label("Image axis — FImg-* (RenderImage · ui/io · DC draw variants)", 13, 0.75, 0.78, 0.85, face), 12, 6)
	root.Place(label("Decode off UI thread · SetImage paint-only · RSS in metrics JSON", 10, 0.5, 0.55, 0.6, face), 12, 26)

	src, err := makeChecker(96, 96, 12)
	if err != nil {
		return nil, fmt.Errorf("checker: %w", err)
	}
	s.src = src

	// --- Row 1: RenderImage sync (immediate) + async (DecodeFile) ---
	root.Place(label("FImg-DRAW · RenderImage", 11, 0.95, 0.75, 0.4, face), 12, 48)

	s.SyncRO = rendering.NewRenderImage(96, 96)
	s.SyncRO.SetRepaintBoundary(true)
	// Shared with s.src (DC demos); scene owns Dispose of src, not the RO.
	s.SyncRO.SetImageShared(src)
	root.Place(s.SyncRO, 12, 68)
	root.Place(label("sync SetImageShared", 10, 0.6, 0.65, 0.7, face), 12, 168)

	s.AsyncRO = rendering.NewRenderImage(96, 96)
	s.AsyncRO.SetRepaintBoundary(true)
	s.AsyncRO.SetLoading()
	s.AsyncRO.PR, s.AsyncRO.PG, s.AsyncRO.PB = 0.18, 0.22, 0.28
	root.Place(s.AsyncRO, 130, 68)
	root.Place(label("async DecodeFile", 10, 0.6, 0.65, 0.7, face), 130, 168)

	s.StatusTxt = label("status: loading…", 11, 0.7, 0.85, 1, face)
	s.StatusTxt.SetRepaintBoundary(true)
	root.Place(s.StatusTxt, 250, 100)

	// --- Row 2: DC draw variants ---
	root.Place(label("UI: DrawImageBuf · Ex/SrcRect · Rounded · Circular · Nine", 11, 0.95, 0.75, 0.4, face), 12, 196)

	s.DCPanel = panelBox(winW-24, 280, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.12, 0.14, 0.18, 1)
		strokeRect(pc, 1, 1, sz.Width-2, sz.Height-2, 1, 0.3, 0.35, 0.4, 1)
		if pc.DC == nil || s.src == nil {
			return
		}
		img := s.src
		ox, oy := pc.OriginX, pc.OriginY

		// 1) DrawImageBuf 1:1 (UI façade)
		rendering.DrawImageBuf(pc, img, 16, 24, 0, 0)
		// 2) DrawImageBuf scaled
		rendering.DrawImageBuf(pc, img, 130, 24, 72, 72)
		// 3) SrcRect crop still via DC Ex (no UI src-rect façade yet)
		srcR := image.Rect(0, 0, 48, 48)
		pc.DC.DrawImageEx(img, render.DrawImageOptions{
			X: ox + 220, Y: oy + 24, DstWidth: 72, DstHeight: 72,
			SrcRect: &srcR, Opacity: 1,
		})
		// 4) Rounded — UI façade (序9)
		rendering.DrawImageRounded(pc, img, 320, 24, 16)
		// 5) Circular (center + radius) still DC until circular UI lands
		pc.DC.DrawImageCircular(img, ox+460, oy+24+48, 40)
		// 6) Nine-patch — UI façade (序9)
		center := image.Rect(24, 24, 72, 72)
		rendering.DrawImageNine(pc, img, center, 530, 24, 160, 96)

		// captions via DC text if font available
		if s.face != nil {
			pc.DC.SetFont(s.face)
		}
		pc.DC.SetRGBA(0.65, 0.7, 0.75, 1)
		pc.DC.DrawString("DrawImageBuf", ox+8, oy+140)
		pc.DC.DrawString("Ex scale", ox+130, oy+140)
		pc.DC.DrawString("SrcRect", ox+220, oy+140)
		pc.DC.DrawString("Rounded", ox+320, oy+140)
		pc.DC.DrawString("Circular", ox+430, oy+140)
		pc.DC.DrawString("Nine", ox+530, oy+140)

		// pulse bar (paint-only) proves continuous frames while images static
		barW := 40 + 120*s.phase
		fillRect(pc, 16, 170, barW, 10, 0.3+0.5*s.phase, 0.7, 0.9, 1)
		pc.DC.SetRGBA(0.55, 0.6, 0.65, 1)
		pc.DC.DrawString("paint pulse (no layout) · async hop on ticker", ox+16, oy+210)
	})
	root.Place(s.DCPanel, 12, 216)

	root.Place(label(
		"FImg-CODEC via ui/io · FImg-DISPOSE: release temp PNG on exit · not TextureLayer/P6",
		10, 0.5, 0.52, 0.55, face,
	), 12, winH-22)

	return s, nil
}

// startAsyncDecode writes a temp PNG and decodes on ui/io workers.
// done is invoked from the worker (any goroutine) after enqueue to pending.
func (s *imageScene) startAsyncDecode(notify func(msg string)) {
	if s == nil {
		return
	}
	s.decodeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gpui-img-axis-*")
		if err != nil {
			s.queuePending(nil, err, "mkdir")
			if notify != nil {
				notify(err.Error())
			}
			return
		}
		s.tempPNG = filepath.Join(dir, "async.png")
		if err := writeTempPNG(s.tempPNG, 128, 128); err != nil {
			s.queuePending(nil, err, "write-png")
			if notify != nil {
				notify(err.Error())
			}
			return
		}
		// Artificial work on worker to make "not on UI" observable in short runs.
		uiio.Default.DecodeFile(s.tempPNG, func(r uiio.Result) {
			tag := "ok"
			if r.Err != nil {
				tag = r.Err.Error()
			}
			s.queuePending(r.Img, r.Err, tag)
			if notify != nil {
				if r.Err != nil {
					notify(r.Err.Error())
				} else {
					notify("")
				}
			}
		})
	})
}

func (s *imageScene) queuePending(img *render.ImageBuf, err error, tag string) {
	if s == nil || s.pending == nil {
		return
	}
	select {
	case s.pending <- pendingImg{img: img, err: err, tag: tag}:
	default:
		// drop if full — ticker will still show loading
	}
}

func (s *imageScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt * 0.7
	if s.phase > 1 {
		s.phase -= 1
	}

	// Hop worker results onto "UI" path: only mutate RO here.
	select {
	case p := <-s.pending:
		s.applyMu.Lock()
		if p.err != nil || p.img == nil {
			if s.AsyncRO != nil {
				s.AsyncRO.SetError()
			}
			s.status = "async error: " + p.tag
			s.lastApplied = "error"
		} else {
			if s.AsyncRO != nil {
				s.AsyncRO.SetImage(p.img)
			}
			s.swapCount++
			s.status = fmt.Sprintf("async ready · swaps=%d · %s", s.swapCount, p.tag)
			s.lastApplied = "ready"
		}
		if s.StatusTxt != nil {
			s.StatusTxt.SetText(s.status)
		}
		s.applyMu.Unlock()
	default:
	}

	// Periodic re-paint of DC panel (pulse) without layout.
	if s.DCPanel != nil {
		s.DCPanel.MarkNeedsPaint()
	}
	if schedule != nil {
		schedule()
	}
}

func (s *imageScene) cleanup() {
	if s == nil {
		return
	}
	if s.tempPNG != "" {
		dir := filepath.Dir(s.tempPNG)
		_ = os.Remove(s.tempPNG)
		_ = os.Remove(dir)
	}
}
