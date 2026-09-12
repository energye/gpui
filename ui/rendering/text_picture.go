package rendering

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/scene"
)

// recordRenderText records a RenderText into a PictureRecorder using the same
// single-source layout Paint uses: single-string content goes through
// DisplayLines (hard breaks plus MaxWidth soft wrap), multi-run content
// through layoutRunLines spans (per-run face, size and color).
//
// Shaped rows record pre-shaped glyph ops (the same glyph slices Paint
// submits, same cull windows, same per-partition rebase) instead of
// re-shaping a substring via DrawString: substring shaping is context
// sensitive (kerning, ligatures, script joining), so a re-shaped band drifts
// from the full-line layout Paint shows (B/C tail sat ~12px off, reading as
// overflow plus a soft/blur mismatch). Unshaped fallbacks keep DrawString.
func recordRenderText(r *scene.PictureRecorder, t *RenderText, ox, oy float64) {
	if r == nil || t == nil {
		return
	}
	// Fresh recording: rows merge their culled state below (OR semantics —
	// one band-limited row is enough to require scroll expiry).
	t.noteRecordedBand(false)
	t.noteRecordedBandY(false)
	if t.hasRuns() {
		recordRunsInto(r, t, ox, oy)
		return
	}
	a := t.A
	if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
		a = 1
	}
	face := t.effectiveFace()
	lay := t.ensureLayout()
	lines := t.DisplayLines()
	ascent := t.fontSize() * 0.8
	if m, ok := t.Metrics(); ok && m.Ascent > 0 {
		ascent = m.Ascent
	}
	rowLo, rowHi := 0, len(lines)
	culledY := false
	if t.hasViewportHintY && lay != nil && lay.LineCount() > 0 {
		rowLo, rowHi = visibleRowBandOf(lay, t.viewportScrollY, t.viewportHeight)
		if rowLo < 0 {
			rowLo = 0
		}
		if rowHi > len(lines) {
			rowHi = len(lines)
		}
		culledY = rowLo > 0 || rowHi < lay.LineCount()
	}
	for i, line := range lines {
		if i < rowLo || i >= rowHi {
			continue
		}
		if line == "" {
			continue
		}
		y := oy + ascent
		if lay != nil && i < lay.LineCount() {
			y = oy + ascent + lay.LineTop(i)
		} else if lay != nil {
			y = oy + ascent + float64(i)*t.lineHeightLogical()
		}
		if lay != nil && i < lay.LineCount() && recordShapedRow(r, t, lay, i, ox, y, face, a) {
			continue
		}
		r.DrawString(line, ox, y, face, t.R, t.G, t.B, a)
	}
	t.mergeRecordedBandY(culledY)
}

// noteRecordedBand remembers which scroll window a recording holds: culled
// recordings are only valid inside that band (plus margin); full recordings
// (unshaped fallbacks, multi-run spans) never expire on scroll.
//
// Only dirty builds may move the band: every packet build re-runs this record
// closure, but the retained texture is only refreshed for dirty pictures —
// moving the note on clean builds would chase the live scroll and expiry
// could never fire (empty box past the margin with recordedIn stuck at 1).
func (t *RenderText) noteRecordedBand(culled bool) {
	if t == nil || !t.NeedsPaint() {
		return
	}
	t.recordedEver = true
	t.recordedCulled = culled
	if culled {
		t.recordedScrollX = t.viewportScrollX
		t.recordedVisW = t.viewportWidth
	}
}

// mergeRecordedBand ORs one row's culled state into the recording: a single
// band-limited row is enough to require scroll expiry (re-recording the
// whole text on expiry is always correct, just occasionally extra).
// Same dirty-build gate as noteRecordedBand.
func (t *RenderText) mergeRecordedBand(culled bool) {
	if t == nil || !culled || !t.NeedsPaint() {
		return
	}
	t.recordedEver = true
	t.recordedCulled = true
	t.recordedScrollX = t.viewportScrollX
	t.recordedVisW = t.viewportWidth
}

// ViewportBandExpired reports whether the retained texture (recorded for the
// stored band) still covers [scrollX, scrollX+visW]. Same margin as
// hCullWindow, so record and expiry can never disagree on coverage.
// A band that was never established (no dirty build recorded yet — e.g. the
// warm-up paint cleared the fresh flags before the first texture existed)
// always expires: the next build is dirty, records, and establishes it.
func (t *RenderText) ViewportBandExpired(scrollX, visW float64) bool {
	if t == nil || visW <= 0 {
		return false
	}
	if !t.recordedEver {
		return true
	}
	if !t.recordedCulled {
		return false
	}
	if visW != t.recordedVisW {
		return true
	}
	d := scrollX - t.recordedScrollX
	if d < 0 {
		d = -d
	}
	return d > viewportMargin
}

// noteRecordedBandY remembers the Y scroll window a culled recording holds,
// mirroring noteRecordedBand on the vertical axis.
func (t *RenderText) noteRecordedBandY(culled bool) {
	if t == nil || !t.NeedsPaint() {
		return
	}
	t.recordedEver = true
	t.recordedCulledY = culled
	if culled {
		t.recordedScrollY = t.viewportScrollY
		t.recordedVisH = t.viewportHeight
	}
}

// mergeRecordedBandY ORs the Y culled state into the recording.
func (t *RenderText) mergeRecordedBandY(culled bool) {
	if t == nil || !culled || !t.NeedsPaint() {
		return
	}
	t.recordedEver = true
	t.recordedCulledY = true
	t.recordedScrollY = t.viewportScrollY
	t.recordedVisH = t.viewportHeight
}

// ViewportBandExpiredY reports whether a Y-culled recording still covers
// [scrollY, scrollY+visH]. Any pixel move re-records: the visible slice is
// only a few rows, so re-recording is cheap and record/expiry can never
// disagree (no blank rows past the margin).
func (t *RenderText) ViewportBandExpiredY(scrollY, visH float64) bool {
	if t == nil || visH <= 0 {
		return false
	}
	if !t.recordedEver {
		return true
	}
	if !t.recordedCulledY {
		return false
	}
	if visH != t.recordedVisH {
		return true
	}
	d := scrollY - t.recordedScrollY
	if d < 0 {
		d = -d
	}
	return d > 0.5
}

// recordShapedRow mirrors Paint's shaped submission for one row at the
// layer-local pen (ox, y): bulk glyph batch or per-face composite partitions,
// with the same viewport cull windows. False = row not shaped-routable,
// caller falls back to DrawString.
func recordShapedRow(r *scene.PictureRecorder, t *RenderText, lay *TextLayout, row int, ox, y float64, face text.Face, a float64) bool {
	glyphs := lay.LineGlyphs(row)
	if len(glyphs) == 0 || glyphs[0].GID == 0 {
		return false
	}
	// Bulk batch: same gate as Paint (sourced single face, no color runs).
	if face != nil && face.Source() != nil && !lay.LineHasColorRun(row) {
		culled := false
		if lo, hi, ok := t.hCullWindow(lay.LineCount(), len(glyphs)); ok {
			s, e := cullRangeForWindow(glyphs, lo, hi)
			glyphs = glyphs[s:e]
			culled = true
		}
		if len(glyphs) == 0 {
			t.mergeRecordedBand(culled)
			return true
		}
		r.DrawShapedGlyphs(glyphs, ox, y, face, false, t.R, t.G, t.B, a)
		t.mergeRecordedBand(culled)
		return true
	}
	// Composite faces (MultiFace, the B/C case): per-face partitions.
	if recordCompositeRow(r, t, lay, row, ox, y, a) {
		return true
	}
	return false
}

// recordCompositeRow mirrors paintCompositeRuns at the layer-local pen:
// color runs keep absolute glyph.X at (ox, y) with no window cull (GPU
// clips them); mask runs cull per partition then rebase to their own origin
// (GPU batches lay out from the pen, unrebased partitions stack at the line
// head). False = not routable, caller falls back to DrawString.
func recordCompositeRow(r *scene.PictureRecorder, t *RenderText, lay *TextLayout, row int, ox, y float64, a float64) bool {
	if !lay.LineBulkRoutable(row) {
		return false
	}
	glyphs := lay.LineGlyphs(row)
	runs := lay.LineGlyphRuns(row)
	cullLo, cullHi, cull := t.hCullWindow(lay.LineCount(), len(glyphs))
	for _, run := range runs {
		part := glyphs[run.Start:run.End]
		if run.IsColor {
			if len(part) > 0 {
				r.DrawShapedGlyphs(part, ox, y, run.Face, true, t.R, t.G, t.B, a)
			}
			continue
		}
		if cull {
			s, e := cullRangeForWindow(part, cullLo, cullHi)
			part = part[s:e]
		}
		if len(part) == 0 {
			continue
		}
		shifted, off := rebaseGlyphs(part)
		r.DrawShapedGlyphs(shifted, ox+off, y, run.Face, false, t.R, t.G, t.B, a)
	}
	t.mergeRecordedBand(cull)
	return true
}

// recordRunsInto records multi-run content span by span, mirroring paintRuns.
func recordRunsInto(r *scene.PictureRecorder, t *RenderText, ox, oy float64) {
	lines := t.layoutRunLines()
	if len(lines) == 0 {
		return
	}
	tops := make([]float64, len(lines))
	top := 0.0
	for i := range lines {
		tops[i] = top
		top += lines[i].Height
	}
	rowLo, rowHi := 0, len(lines)
	if t.hasViewportHintY {
		rowLo, rowHi = visibleRunBand(tops, lines, t.viewportScrollY, t.viewportHeight)
		t.mergeRecordedBandY(rowLo > 0 || rowHi < len(lines))
	}
	lineTop := 0.0
	for li, ln := range lines {
		if li < rowLo || li >= rowHi {
			lineTop += ln.Height
			continue
		}
		maxAscent := 0.0
		for _, sp := range ln.Spans {
			asc := sp.FontSize * 0.8
			if sp.Face != nil {
				if m := sp.Face.Metrics(); m.Ascent > 0 {
					asc = m.Ascent
				}
			}
			if asc > maxAscent {
				maxAscent = asc
			}
		}
		if maxAscent == 0 {
			if m, ok := t.Metrics(); ok && m.Ascent > 0 {
				maxAscent = m.Ascent
			} else {
				maxAscent = t.fontSize() * 0.8
			}
		}
		baseline := oy + lineTop + maxAscent
		for _, sp := range ln.Spans {
			if sp.Text == "" {
				continue
			}
			face := sp.Face
			if face == nil {
				face = t.effectiveFace()
			}
			r.DrawString(sp.Text, ox+sp.X, baseline, face, sp.R, sp.G, sp.B, sp.A)
		}
		lineTop += ln.Height
	}
}
