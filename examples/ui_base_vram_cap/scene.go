// Scene for ui_base_vram_cap: cascading repaint-boundary cards.
//
// Every card is its own RepaintBoundary, so the retained packet carries one
// cacheable picture layer per card and the picture-texture cache working set
// (cards + headroom) is what the shared-budget fair share must refuse past.
// Cards are static and deterministic: once recorded they stay cached until
// the budget forces an eviction, so the only re-record churn visible in a
// phase is churn the refusal path itself caused. The HUD band (top 40px) is
// the only animated region and is excluded from the Golden mask.
package main

import (
	"fmt"
	"sync/atomic"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/rendering"
)

const (
	cardW, cardH = 256, 256
	// cardsPerWin fixes the per-window cache working set. 160 cards of
	// 256x256 RGBA demand ~40MB of cache entries per window: inside the
	// fair share at 2 windows (~96MB), past it at 4 (~48MB) and 8 (~24MB)
	// under the default 768MB process budget — so the ladder starts
	// refusing growth exactly when the shared pressure arrives.
	cardsPerWin = 160
	// hudH is the metric band; excluded from the Golden mask.
	hudH = 40
)

// cardPalette keeps card fills deterministic and distinct per index.
var cardPalette = [6][3]float64{
	{0.16, 0.30, 0.45},
	{0.32, 0.22, 0.42},
	{0.16, 0.38, 0.30},
	{0.42, 0.28, 0.18},
	{0.34, 0.34, 0.16},
	{0.36, 0.20, 0.28},
}

// buildCards places count repaint-boundary boxes into root in a cascading
// deck (4 columns, 10px vertical steps) so every card keeps at least a
// sliver inside the window: fully culled layers would drop out of the packet
// and stop counting toward the cache working set.
//
// OnPaint contract: PaintContext carries the box's absolute origin and raw
// pc.DC draws bypass it, so the callback translates the DC to pc.Abs(0,0)
// first (multiwin gets away with raw DC draws only because its single box
// sits at Place(0,0)).
func buildCards(root *rendering.AbsoluteBox, idxBase, count int) {
	for i := 0; i < count; i++ {
		card := rendering.NewRenderBox()
		card.FixedWidth, card.FixedHeight = cardW, cardH
		card.SetRepaintBoundary(true)
		idx := idxBase*cardsPerWin + i
		c := cardPalette[idx%len(cardPalette)]
		card.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			dc := pc.DC
			ox, oy := pc.Abs(0, 0)
			dc.Push()
			dc.Translate(ox, oy)
			dc.SetRGB(c[0], c[1], c[2])
			dc.DrawRectangle(0, 0, cardW, cardH)
			_ = dc.Fill()
			dc.SetRGBA(0.92, 0.94, 0.98, 0.9)
			if face := wrkit.FaceAt(13); face != nil {
				dc.SetFont(face)
			}
			dc.DrawString(fmt.Sprintf("C%d", idx), 8, 18)
			dc.DrawRectangle(12, 24, cardW-24, cardH-36)
			dc.SetRGB(c[0]*0.55, c[1]*0.55, c[2]*0.55)
			_ = dc.Fill()
			dc.Pop()
		}
		col, row := i%4, i/4
		root.Place(card, float64(24+col*96), float64(hudH+6+row*10))
	}
}

// hudScene is the per-window metric band. The sampler goroutine writes the
// atomics; paint reads them. Values freeze before the gate snapshot so the
// committed PNG is deterministic.
type hudScene struct {
	idx      int
	budgetMB float64
	fps      atomic.Uint64 // fps*100
	presents atomic.Int64
	liveMB   atomic.Uint64 // live ledger MB*100
	refusals atomic.Int64
	frozen   atomic.Bool
}

func newHudScene(idx int, budgetMB float64) *hudScene {
	return &hudScene{idx: idx, budgetMB: budgetMB}
}

func (h *hudScene) freeze() {
	if h != nil {
		h.frozen.Store(true)
	}
}

func (h *hudScene) fpsFloat() float64 {
	if h == nil {
		return 0
	}
	return float64(h.fps.Load()) / 100
}

// paint draws the HUD band: window id, fps, presents, live ledger and the
// window's cumulative cache refusals. All values come from atomics.
// Origin-aware like the cards (see buildCards).
func (h *hudScene) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc := pc.DC
	w := size.Width
	if w <= 0 {
		w = winW
	}
	ox, oy := pc.Abs(0, 0)
	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGB(0.06, 0.07, 0.10)
	dc.DrawRectangle(0, 0, w, hudH)
	_ = dc.Fill()
	if face := wrkit.FaceAt(13); face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(0.78, 0.88, 0.98, 1)
	dc.DrawString(fmt.Sprintf("VRAMCAP W%d fps=%.0f presents=%d live=%.1f/%.0fMB refusals=%d",
		h.idx+1, h.fpsFloat(), h.presents.Load(), float64(h.liveMB.Load())/100, h.budgetMB, h.refusals.Load()), 8, 24)
	dc.Pop()
}
