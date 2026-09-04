package rendering

import (
	"github.com/energye/gpui/ui/scene"
)

// recordRenderText records a RenderText into a PictureRecorder using the same
// single-source layout Paint uses: single-string content goes through
// DisplayLines (hard breaks plus MaxWidth soft wrap), multi-run content
// through layoutRunLines spans (per-run face, size and color).
func recordRenderText(r *scene.PictureRecorder, t *RenderText, ox, oy float64) {
	if r == nil || t == nil {
		return
	}
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
	for i, line := range lines {
		if line == "" {
			continue
		}
		y := oy + ascent
		if lay != nil && i < lay.LineCount() {
			y = oy + ascent + lay.LineTop(i)
		} else if lay != nil {
			y = oy + ascent + float64(i)*t.lineHeightLogical()
		}
		r.DrawString(line, ox, y, face, t.R, t.G, t.B, a)
	}
}

// recordRunsInto records multi-run content span by span, mirroring paintRuns.
func recordRunsInto(r *scene.PictureRecorder, t *RenderText, ox, oy float64) {
	lines := t.layoutRunLines()
	if len(lines) == 0 {
		return
	}
	lineTop := 0.0
	for _, ln := range lines {
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
