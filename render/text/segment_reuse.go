package text

import (
	"sort"
	"unicode/utf8"

	"golang.org/x/text/unicode/bidi"
)

// SegmentReuse incrementally segments newText by reusing oldSegs across a
// single edit range [oldA,oldB) -> [newA,newB).
//
// LTR-only fast path: when the old segmentation has no odd levels and the new
// text holds no strong RTL characters, levels stay 0 and only script runs
// matter. Prefix segments fully inside the common head and suffix segments
// fully inside the common tail are reused; only the straddling window plus
// adjacent Common/Inherited runs (whose resolution depends on both sides) is
// re-segmented. Seams with equal scripts merge, matching full segmentation.
//
// Any unsafe shape (RTL present, bad ranges, non-rune boundaries, invalid old
// coverage) falls back to SegmentText(newText), so results always equal the
// full pass.
func SegmentReuse(oldText string, oldSegs []Segment, newText string, oldA, oldB, newA, newB int) []Segment {
	full := func() []Segment { return SegmentText(newText) }
	if oldA < 0 || oldB < oldA || oldA > len(oldText) || oldB > len(oldText) ||
		newA < 0 || newB < newA || newA > len(newText) || newB > len(newText) {
		return full()
	}
	if oldA != newA {
		return full()
	}
	if len(oldText)-oldB != len(newText)-newB {
		return full()
	}
	if !runeBoundary(oldText, oldA) || !runeBoundary(oldText, oldB) ||
		!runeBoundary(newText, newA) || !runeBoundary(newText, newB) {
		return full()
	}
	if len(oldSegs) == 0 {
		return full()
	}
	if oldSegs[0].Start != 0 || oldSegs[len(oldSegs)-1].End != len(oldText) {
		return full()
	}
	for i, s := range oldSegs {
		if s.Start < 0 || s.End < s.Start || s.End > len(oldText) {
			return full()
		}
		if s.Level%2 == 1 {
			return full()
		}
		if i > 0 && s.Start != oldSegs[i-1].End {
			return full()
		}
		if !runeBoundary(oldText, s.Start) || !runeBoundary(oldText, s.End) {
			return full()
		}
	}
	pre := oldA
	sL0 := sort.Search(len(oldSegs), func(i int) bool { return oldSegs[i].End > pre })
	sR0 := sort.Search(len(oldSegs), func(i int) bool { return oldSegs[i].Start >= oldB })
	if sL0 > sR0 || sR0 > len(oldSegs) {
		return full()
	}
	// A strong straddler shields its side: neighboring weak runs resolve
	// against it (unchanged), so weak-dropping is only needed on sides
	// without a strong straddler. Splitting is only safe for all-strong
	// edits with strong edges (weak edges re-resolve against the edit).
	hasHead := sL0 < len(oldSegs) && oldSegs[sL0].Start < pre && oldSegs[sL0].End > pre &&
		!isWeakScript(oldSegs[sL0].Script)
	hasTail := sR0 > 0 && oldSegs[sR0-1].Start < oldB && oldSegs[sR0-1].End > oldB &&
		!isWeakScript(oldSegs[sR0-1].Script)
	leftKeep, rightKeep := sL0, sR0
	if !hasHead {
		for leftKeep > 0 && isWeakScript(oldSegs[leftKeep-1].Script) {
			leftKeep--
		}
	}
	if !hasTail {
		for rightKeep < len(oldSegs) && isWeakScript(oldSegs[rightKeep].Script) {
			rightKeep++
		}
	}
	winOldA := pre
	if leftKeep == 0 {
		winOldA = 0
	} else {
		winOldA = oldSegs[leftKeep-1].End
	}
	winOldB := oldB
	if rightKeep == len(oldSegs) {
		winOldB = len(oldText)
	} else if rightKeep > 0 {
		winOldB = oldSegs[rightKeep].Start
		if winOldB < oldB {
			return full()
		}
	} else {
		winOldB = oldB
	}
	if winOldA > pre || winOldB < oldB || winOldA > winOldB {
		return full()
	}
	delta := len(newText) - len(oldText)
	winNewA := winOldA
	winNewB := winOldB + delta
	// Newly inserted weak runs resolve against neighboring strong scripts,
	// which live outside a minimal window. If the new window holds any weak
	// rune, widen once over the adjacent kept strong segment on each side so
	// the re-segmented window carries both contexts.
	if windowHasWeak(newText[winNewA:winNewB]) {
		if leftKeep > 0 {
			leftKeep--
			if leftKeep == 0 {
				winOldA = 0
			} else {
				winOldA = oldSegs[leftKeep-1].End
			}
		}
		if rightKeep < len(oldSegs) {
			rightKeep++
			if rightKeep == len(oldSegs) {
				winOldB = len(oldText)
			} else {
				winOldB = oldSegs[rightKeep].Start
			}
		}
		winNewA = winOldA
		winNewB = winOldB + delta
	}
	// Strong straddlers split AFTER widening (widening re-includes edge
	// regions, so splitting earlier would overlap the widened window).
	// Guards: the edit itself holds no weak rune (else the straddler parts
	// plus new weaks re-resolve together), and each partial keeps a strong
	// edge (a partial ending/starting with Common/Inherited re-resolves
	// against the edit). This is what makes single-segment (homogeneous)
	// lines incremental; everything else stays whole (context-dependent).
	var head, tail *Segment
	editHasWeak := windowHasWeak(newText[newA:newB])
	if hasHead && !editHasWeak && oldSegs[sL0].Start >= winOldA &&
		isStrongRuneBefore(newText, pre) {
		sg := oldSegs[sL0]
		head = &Segment{Text: newText[sg.Start:pre], Start: sg.Start, End: pre,
			Direction: sg.Direction, Script: sg.Script, Level: sg.Level}
		winOldA = pre
		winNewA = pre
	}
	if hasTail && !editHasWeak && oldSegs[sR0-1].End <= winOldB &&
		isStrongRuneAt(newText, newB) {
		sg := oldSegs[sR0-1]
		ns, ne := oldB+delta, sg.End+delta
		tail = &Segment{Text: newText[ns:ne], Start: ns, End: ne,
			Direction: sg.Direction, Script: sg.Script, Level: sg.Level}
		winOldB = oldB
		winNewB = newB
	}
	if winNewA > newA || winNewB < newB || winNewA > winNewB ||
		winNewB > len(newText) {
		return full()
	}
	if !runeBoundary(newText, winNewA) || !runeBoundary(newText, winNewB) {
		return full()
	}
	if hasStrongRTL(newText[winNewA:winNewB]) {
		return full()
	}
	midRaw := SegmentText(newText[winNewA:winNewB])
	mid := make([]Segment, 0, len(midRaw))
	for _, s := range midRaw {
		if s.Level%2 == 1 {
			return full()
		}
		mid = append(mid, Segment{
			Text:      newText[winNewA+s.Start : winNewA+s.End],
			Start:     winNewA + s.Start,
			End:       winNewA + s.End,
			Direction: s.Direction,
			Script:    s.Script,
			Level:     s.Level,
		})
	}
	out := make([]Segment, 0, leftKeep+len(mid)+len(oldSegs)-rightKeep+2)
	for _, s := range oldSegs[:leftKeep] {
		out = append(out, Segment{
			Text:      newText[s.Start:s.End],
			Start:     s.Start,
			End:       s.End,
			Direction: s.Direction,
			Script:    s.Script,
			Level:     s.Level,
		})
	}
	if head != nil {
		out = append(out, *head)
	}
	out = append(out, mid...)
	if tail != nil {
		out = append(out, *tail)
	}
	for _, s := range oldSegs[rightKeep:] {
		ns, ne := s.Start+delta, s.End+delta
		if ns < 0 || ne > len(newText) || ns > ne {
			return full()
		}
		out = append(out, Segment{
			Text:      newText[ns:ne],
			Start:     ns,
			End:       ne,
			Direction: s.Direction,
			Script:    s.Script,
			Level:     s.Level,
		})
	}
	// Seam strength (checked BEFORE merging: merging erases same-script
	// seams and could hide a weak run whose resolution changed): no weak run
	// may cross a stitch seam, else its resolution could depend on changed
	// context (e.g. a trailing space merged as Latin flips to Common when Han
	// is appended). Every seam needs a strong rune on both sides (or a text
	// boundary); interior weak runs are then bracketed by unchanged strongs
	// inside their own piece and resolve identically to the full pass.
	for i := 1; i < len(out); i++ {
		p := out[i].Start
		if p != out[i-1].End || !runeBoundary(newText, p) {
			return full()
		}
		if p > 0 && p < len(newText) {
			if !isStrongRuneBefore(newText, p) || !isStrongRuneAt(newText, p) {
				return full()
			}
		}
		if out[i].Start > out[i].End || out[i].End > len(newText) {
			return full()
		}
	}
	out = mergeAdjacentSegments(out)
	if len(out) == 0 {
		return full()
	}
	if out[0].Start != 0 || out[len(out)-1].End != len(newText) {
		return full()
	}
	return out
}

func runeBoundary(s string, i int) bool {
	return i == 0 || i == len(s) || (i >= 0 && i < len(s) && utf8.RuneStart(s[i]))
}

func isWeakScript(s Script) bool {
	return s == ScriptCommon || s == ScriptInherited
}

func windowHasWeak(s string) bool {
	for _, r := range s {
		if DetectScript(r) == ScriptCommon || DetectScript(r) == ScriptInherited {
			return true
		}
	}
	return false
}

// isStrongRuneAt reports whether the rune starting at byte i is strong
// (non-Common, non-Inherited). False at boundaries/empty.
func isStrongRuneAt(s string, i int) bool {
	if i < 0 || i >= len(s) || !runeBoundary(s, i) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return DetectScript(r) != ScriptCommon && DetectScript(r) != ScriptInherited
}

// isStrongRuneBefore reports whether the rune ending at byte i is strong.
func isStrongRuneBefore(s string, i int) bool {
	if i <= 0 || i > len(s) || !runeBoundary(s, i) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return DetectScript(r) != ScriptCommon && DetectScript(r) != ScriptInherited
}

func hasStrongRTL(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			i++
			continue
		}
		p, _ := bidi.LookupRune(r)
		switch p.Class() {
		case bidi.R, bidi.AL, bidi.RLO, bidi.RLE, bidi.RLI, bidi.FSI:
			return true
		}
		i += size
	}
	return false
}

func mergeAdjacentSegments(segs []Segment) []Segment {
	if len(segs) == 0 {
		return segs
	}
	out := segs[:1]
	for _, s := range segs[1:] {
		p := &out[len(out)-1]
		if p.End == s.Start && p.Level == s.Level &&
			p.Direction == s.Direction && p.Script == s.Script {
			p.End = s.End
			p.Text += s.Text
			continue
		}
		out = append(out, s)
	}
	return out
}
