package watermark

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// MarkLineSnap is one frozen text row: content plus style already resolved
// against the theme at refresh time (per-line font wins over global).
type MarkLineSnap struct {
	Text     string
	FontSize float64
	Color    render.RGBA
	Align    string
	Width    float64 // estimated text width at FontSize
}

// WatermarkSnap is the frozen paint input for one watermark frame (R2-6,
// button snapshot paradigm). The UI thread refreshes it on every dirty
// (refreshSnapshot inside dirtyMark); the raster-thread paint path
// (paintMarks/paintTextTile) reads only this value, never the live
// Watermark. Setters may keep mutating the widget while raster paints —
// the pixels always match the last UI refresh, with no cross-thread reads
// or writes of widget state.
type WatermarkSnap struct {
	// Host is the laid-out host size at refresh time (tiling bounds).
	Host rendering.Size
	// HasMark: text rows or ready image present.
	HasMark bool
	// ImageMode wins over text when true; ImageBuf is the retained decode
	// result (alive until the next refresh replaces it).
	ImageMode bool
	ImageBuf  *render.ImageBuf
	// Tile geometry at refresh time (ResolvedMarkSize/Gap/Offset applied).
	MarkW, MarkH float64
	GapX, GapY   float64
	OffX, OffY   float64
	// RotateDeg is the tile rotation in degrees.
	RotateDeg float64
	// Lines are the frozen text rows (empty in image mode).
	Lines []MarkLineSnap
	// Face is the paint-only font face at refresh time.
	Face text.Face
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from dirtyMark and Layout). Pure value build: no raster-side field is
// touched, so a concurrent paint of the previous snapshot is unaffected.
func (w *Watermark) refreshSnapshot() {
	if w == nil {
		return
	}
	s := WatermarkSnap{Face: w.face}
	if w.host != nil {
		sz := w.host.Size()
		s.Host = rendering.Size{Width: sz.Width, Height: sz.Height}
	}
	s.HasMark = w.hasMarkLocked()
	if w.imageReady && !w.imageError && w.imageBuf != nil && !w.imageBuf.Disposed() {
		s.ImageMode = true
		s.ImageBuf = w.imageBuf
	}
	s.MarkW, s.MarkH = w.resolvedMarkSizeLocked()
	s.GapX, s.GapY = w.resolvedGapLocked()
	s.OffX, s.OffY = w.resolvedOffsetLocked()
	s.RotateDeg = w.rotate
	if !s.ImageMode {
		for _, ln := range w.lines {
			if ln.Text == "" {
				continue
			}
			ms := MarkLineSnap{Text: ln.Text, Align: defaultAlignLocked(w, ln)}
			ms.FontSize = w.lineFontSize(ln)
			ms.Color = w.lineColor(ln)
			ms.Width, _ = rendering.EstimateTextSize(ln.Text, ms.FontSize, approxCharW)
			s.Lines = append(s.Lines, ms)
		}
	}
	w.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; never nil for a live widget
// (zero value paints nothing).
func (w *Watermark) loadSnapshot() WatermarkSnap {
	if w == nil {
		return WatermarkSnap{}
	}
	if v, ok := w.snap.Load().(WatermarkSnap); ok {
		return v
	}
	return WatermarkSnap{}
}

// hasMarkLocked is HasMark without the nil guard (UI, inside refresh).
func (w *Watermark) hasMarkLocked() bool {
	if w.imageReady && !w.imageError && w.imageBuf != nil && !w.imageBuf.Disposed() {
		return true
	}
	for _, ln := range w.lines {
		if ln.Text != "" {
			return true
		}
	}
	return false
}

// resolvedMarkSizeLocked is ResolvedMarkSize without nil guards (UI, inside
// refresh); image mode width/height uses image pixel size as fallback.
func (w *Watermark) resolvedMarkSizeLocked() (mw, mh float64) {
	if w.imageReady && !w.imageError && w.imageBuf != nil && !w.imageBuf.Disposed() {
		if w.hasWidth {
			mw = w.width
		} else if w.imageW > 0 {
			mw = float64(w.imageW)
		} else {
			mw = DefaultWidth
		}
		if w.hasHeight {
			mh = w.height
		} else if w.imageH > 0 {
			mh = float64(w.imageH)
		} else {
			mh = DefaultHeight
		}
		return mw, mh
	}
	if !w.hasMarkLocked() {
		return 0, 0
	}
	if w.hasWidth {
		mw = w.width
	} else {
		for _, ln := range w.lines {
			if ln.Text == "" {
				continue
			}
			lw, _ := rendering.EstimateTextSize(ln.Text, w.lineFontSize(ln), approxCharW)
			if lw > mw {
				mw = lw
			}
		}
	}
	if w.hasHeight {
		mh = w.height
	} else {
		rows := 0
		for _, ln := range w.lines {
			if ln.Text == "" {
				continue
			}
			mh += w.lineFontSize(ln) * 1.25
			rows++
		}
		if rows > 1 {
			mh += float64(rows-1) * FontGap
		}
	}
	return mw, mh
}

func (w *Watermark) resolvedGapLocked() (x, y float64) {
	if w == nil {
		return DefaultGapX, DefaultGapY
	}
	return w.gapX, w.gapY
}

func (w *Watermark) resolvedOffsetLocked() (x, y float64) {
	if w == nil {
		return DefaultGapX / 2, DefaultGapY / 2
	}
	if w.hasOffset {
		return w.offX, w.offY
	}
	return w.gapX / 2, w.gapY / 2
}

// defaultAlignLocked resolves one row's text alignment (UI, inside refresh).
func defaultAlignLocked(w *Watermark, ln WatermarkContentLine) string {
	if ln.HasFont && ln.Font.TextAlign != "" {
		return ln.Font.TextAlign
	}
	if w != nil && w.font.TextAlign != "" {
		return w.font.TextAlign
	}
	return DefaultTextAlign
}
