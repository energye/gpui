package textinput

import (
	"math"
	"strings"

	"github.com/energye/gpui/ui/rendering"
)

// editScroll is the single scroll/record funnel shared by every edit box
// (InputBox, ViewportInputBox, MultiLineInputBox), aligned with Flutter's
// one-RenderEditable design: single-line vs multi-line is a property
// (scrollMetrics.multi), scrolling is one implementation, not per-type
// scroll code.
//
// It owns the scroll offsets and performs, in one place, the whole scroll
// update: ensure-caret-visible, content clamp, pixel snap, offset apply,
// cull-hint set, retained-band expiry. The three box types only feed caret
// metrics + content extents from their own layout queries and keep their
// highlight builders, caret bars and event handling.
type editScroll struct {
	txt      *rendering.RenderText
	viewport *rendering.RenderViewport // nil: offset applies to txt (plain/multi)
	scrollX  float64
	scrollY  float64
}

// scrollMetrics is one frame of scroll inputs. Caret position and content
// extents come from each box's own layout queries (single-line and
// multi-line layouts differ); everything else is shared funnel behavior.
type scrollMetrics struct {
	caretX, caretY, caretH float64
	maxW                   float64 // widest content row, for the X clamp
	totalH, lineH          float64 // content height + caret row height (multi only)
	visW, visH, pad        float64
	textY                  float64 // single-line text pen Y (multi: unused)
	multi                  bool    // enable the vertical axis
	margin                 float64 // caret reveal/clamp gap (Flutter _caretMargin)
}

// marginOrDefault reports the caret gap, falling back to the legacy 4px
// when the caller did not set one.
func (m scrollMetrics) marginOrDefault() float64 {
	if m.margin > 0 {
		return m.margin
	}
	return 4
}

// clear resets both offsets (empty text). Replaces the three copies of
// `if disp == "": scroll = 0` (plus the viewport offset reset).
func (s *editScroll) clear() {
	if s == nil {
		return
	}
	s.scrollX, s.scrollY = 0, 0
	if s.viewport != nil {
		s.viewport.SetScrollOffset(0, 0)
	}
}

// ensureVisible is RenderEditable.showCursor: reveal the caret, clamp into
// content, snap, apply, hint, expire. Every box calls this from sync, so a
// scroll-only sync can never again leave a stale recording live.
func (s *editScroll) ensureVisible(m scrollMetrics) {
	if s == nil || s.txt == nil {
		return
	}
	gap := m.marginOrDefault()
	if m.caretX-s.scrollX > m.visW-gap {
		s.scrollX = m.caretX - m.visW + gap
	}
	if m.caretX-s.scrollX < gap {
		s.scrollX = m.caretX - gap
	}
	if m.multi {
		if m.caretY-s.scrollY < 4 {
			s.scrollY = m.caretY - 4
		}
		if m.caretY+m.lineH-s.scrollY > m.visH-4 {
			s.scrollY = m.caretY + m.lineH - m.visH + 4
		}
	}
	s.commit(m, true)
}

// refresh re-applies the current offsets, hint and band without moving
// scroll (no layout to follow, e.g. empty text). Same visible result as a
// plain offset re-apply; the band check is a no-op on its own window.
func (s *editScroll) refresh(m scrollMetrics) {
	if s == nil || s.txt == nil {
		return
	}
	if s.viewport != nil {
		s.viewport.SetScrollOffset(s.scrollX, 0)
	} else if m.multi {
		s.txt.SetOffset(rendering.Point{X: m.pad - s.scrollX, Y: m.pad - s.scrollY})
	} else {
		s.txt.SetOffset(rendering.Point{X: m.pad - s.scrollX, Y: m.textY})
	}
	s.txt.SetViewportHint(s.scrollX, m.visW)
	if s.txt.ViewportBandExpired(s.scrollX, m.visW) {
		s.txt.MarkNeedsPaint()
	}
}

// scrollBy steps the offsets (drag-select / auto-scroll ticks) and commits.
// No caret following and no shrink-rollback: callers remap the caret from
// the pointer afterwards (which re-enters sync on change).
func (s *editScroll) scrollBy(dx, dy float64, m scrollMetrics) {
	if s == nil || s.txt == nil {
		return
	}
	s.scrollX += dx
	if m.multi {
		s.scrollY += dy
	}
	s.commit(m, false)
}

// commit clamps into content, snaps to whole pixels (retained blits stay
// texel-aligned), applies the offset, sets the cull hint and expires the
// retained band past its margin. followCaret adds the shrink-rollback
// (deleted text pulls a stranded scrollX back), which only sync does.
func (s *editScroll) commit(m scrollMetrics, followCaret bool) {
	if s.scrollX < 0 {
		s.scrollX = 0
	}
	maxW := m.maxW
	if followCaret && maxW < m.caretX {
		maxW = m.caretX
	}
	maxScroll := maxW - m.visW + m.marginOrDefault()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.scrollX > maxScroll {
		s.scrollX = maxScroll
	}
	if m.multi {
		if s.scrollY < 0 {
			s.scrollY = 0
		}
		maxY := m.totalH - m.visH
		if maxY < 0 {
			maxY = 0
		}
		if s.scrollY > maxY {
			s.scrollY = maxY
		}
	}
	s.scrollX = math.Round(s.scrollX)
	s.scrollY = math.Round(s.scrollY)
	if s.viewport != nil {
		s.viewport.SetScrollOffset(s.scrollX, 0)
	} else if m.multi {
		s.txt.SetOffset(rendering.Point{X: m.pad - s.scrollX, Y: m.pad - s.scrollY})
	} else {
		s.txt.SetOffset(rendering.Point{X: m.pad - s.scrollX, Y: m.textY})
	}
	s.txt.SetViewportHint(s.scrollX, m.visW)
	if s.txt.ViewportBandExpired(s.scrollX, m.visW) {
		s.txt.MarkNeedsPaint()
	}
}

// syncBoxText mirrors the editor text into txt (password-masked /
// placeholder-substituted), reusing the incremental span channel when
// valid. Replaces the three copies in the box syncs; the wrap MaxWidth
// stays with the multi-line box (layout config, set before this call).
func syncBoxText(txt *rendering.RenderText, ed *Editor, placeholder string, usePlaceholder bool) string {
	disp := ed.GetText()
	isPassword := ed.IsPassword()
	if isPassword && disp != "" {
		disp = strings.Repeat(string(ed.ObscuringCharacter()), len([]rune(disp)))
	} else if usePlaceholder {
		disp = placeholder
	}
	if !isPassword && !usePlaceholder {
		if oA, oB, nA, nB, ok := ed.ConsumeEditSpan(); ok {
			txt.SetTextSpan(disp, oA, oB, nA, nB)
		} else {
			txt.SetText(disp)
		}
	} else {
		txt.SetText(disp)
	}
	return disp
}

// maskedCaretByte maps the editor caret through the password mask
// (Flutter TextField.obscuringCharacter): editor byte -> rune index ->
// mask byte. Replaces the two copies in the single-line syncs.
func maskedCaretByte(ed *Editor, disp string, curByte int) int {
	text := ed.GetText()
	if curByte > len(text) {
		curByte = len(text)
	}
	runeIdx := 0
	for i := range text[:curByte] {
		if (text[i] & 0xC0) != 0x80 {
			runeIdx++
		}
	}
	maskedRunes := []rune(disp)
	if runeIdx > len(maskedRunes) {
		runeIdx = len(maskedRunes)
	}
	return len(string(maskedRunes[:runeIdx]))
}
