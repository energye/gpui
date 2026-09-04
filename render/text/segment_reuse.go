package text

import (
	"sort"
	"unicode/utf8"

	"golang.org/x/text/unicode/bidi"
)

// SegmentReuse incrementally segments newText by reusing oldSegs across a
// single edit range [oldA,oldB) -> [newA,newB).
//
// LTR-only fast path: old levels are even and the new window holds no strong
// RTL, so levels stay 0 and only script runs matter. Segments fully inside
// the common head/tail are reused; only the straddling window plus adjacent
// Common/Inherited runs is re-segmented. Seams with equal scripts merge.
//
// Any unsafe shape falls back to SegmentText(newText), so the result always
// equals the full pass.
func SegmentReuse(oldText string, oldSegs []Segment, newText string, oldA, oldB, newA, newB int) []Segment {
	full := func() []Segment { return SegmentText(newText) }
	if !checkReuseRange(oldText, newText, oldA, oldB, newA, newB) {
		return full()
	}
	if !checkOldSegs(oldText, oldSegs) {
		return full()
	}
	leftKeep, rightKeep, loc, ok := keptRange(oldSegs, oldA, oldB)
	if !ok {
		return full()
	}
	win, leftKeep, rightKeep, ok := placeWindow(oldText, newText, oldSegs, leftKeep, rightKeep, oldA, oldB)
	if !ok {
		return full()
	}
	win, head, tail, ok := splitPartials(newText, oldSegs, loc, win, oldA, oldB, newA, newB)
	if !ok {
		return full()
	}
	if !checkWindow(newText, win, newA, newB) {
		return full()
	}
	mid := resegmentWindow(newText, win)
	if mid == nil {
		return full()
	}
	return stitch(newText, oldSegs, leftKeep, rightKeep, head, tail, mid, win.delta, full)
}

// checkReuseRange validates the edit description against both texts.
func checkReuseRange(oldText, newText string, oldA, oldB, newA, newB int) bool {
	if oldA < 0 || oldB < oldA || oldA > len(oldText) || oldB > len(oldText) ||
		newA < 0 || newB < newA || newA > len(newText) || newB > len(newText) {
		return false
	}
	if oldA != newA {
		return false
	}
	if len(oldText)-oldB != len(newText)-newB {
		return false
	}
	return runeBoundary(oldText, oldA) && runeBoundary(oldText, oldB) &&
		runeBoundary(newText, newA) && runeBoundary(newText, newB)
}

// checkOldSegs validates coverage, even levels, contiguity and boundaries.
func checkOldSegs(oldText string, oldSegs []Segment) bool {
	if len(oldSegs) == 0 {
		return false
	}
	if oldSegs[0].Start != 0 || oldSegs[len(oldSegs)-1].End != len(oldText) {
		return false
	}
	for i, s := range oldSegs {
		if s.Start < 0 || s.End < s.Start || s.End > len(oldText) {
			return false
		}
		if s.Level%2 == 1 {
			return false
		}
		if i > 0 && s.Start != oldSegs[i-1].End {
			return false
		}
		if !runeBoundary(oldText, s.Start) || !runeBoundary(oldText, s.End) {
			return false
		}
	}
	return true
}

// keptLoc records straddlers used by window placement and partial splitting.
type keptLoc struct {
	sL0, sR0         int
	hasHead, hasTail bool
}

// keptRange finds whole reusable segments. A strong straddler shields its
// side (neighbors resolve against it, unchanged); other sides drop adjacent
// weak segments into the window.
func keptRange(oldSegs []Segment, pre, oldB int) (leftKeep, rightKeep int, loc keptLoc, ok bool) {
	loc.sL0 = sort.Search(len(oldSegs), func(i int) bool { return oldSegs[i].End > pre })
	loc.sR0 = sort.Search(len(oldSegs), func(i int) bool { return oldSegs[i].Start >= oldB })
	if loc.sL0 > loc.sR0 || loc.sR0 > len(oldSegs) {
		return 0, 0, loc, false
	}
	loc.hasHead = loc.sL0 < len(oldSegs) && oldSegs[loc.sL0].Start < pre &&
		oldSegs[loc.sL0].End > pre && !isWeakScript(oldSegs[loc.sL0].Script)
	loc.hasTail = loc.sR0 > 0 && oldSegs[loc.sR0-1].Start < oldB &&
		oldSegs[loc.sR0-1].End > oldB && !isWeakScript(oldSegs[loc.sR0-1].Script)
	leftKeep, rightKeep = loc.sL0, loc.sR0
	if !loc.hasHead {
		for leftKeep > 0 && isWeakScript(oldSegs[leftKeep-1].Script) {
			leftKeep--
		}
	}
	if !loc.hasTail {
		for rightKeep < len(oldSegs) && isWeakScript(oldSegs[rightKeep].Script) {
			rightKeep++
		}
	}
	return leftKeep, rightKeep, loc, true
}

// reuseWin is the re-segmented byte window in old and new coordinates.
type reuseWin struct {
	oldA, oldB int
	newA, newB int
	delta      int
}

// placeWindow bounds the window to kept regions, then widens once over
// adjacent kept strongs when the new window holds weak runes (they resolve
// against neighboring strongs outside a minimal window). Updated kept counts
// are returned alongside the window.
func placeWindow(oldText, newText string, oldSegs []Segment, leftKeep, rightKeep, oldA, oldB int) (reuseWin, int, int, bool) {
	fail := func() (reuseWin, int, int, bool) { return reuseWin{}, 0, 0, false }
	var w reuseWin
	if leftKeep == 0 {
		w.oldA = 0
	} else {
		w.oldA = oldSegs[leftKeep-1].End
	}
	if rightKeep == len(oldSegs) {
		w.oldB = len(oldText)
	} else if rightKeep > 0 {
		w.oldB = oldSegs[rightKeep].Start
		if w.oldB < oldB {
			return fail()
		}
	} else {
		w.oldB = oldB
	}
	if w.oldA > oldA || w.oldB < oldB || w.oldA > w.oldB {
		return fail()
	}
	w.delta = len(newText) - len(oldText)
	w.newA = w.oldA
	w.newB = w.oldB + w.delta
	if windowHasWeak(newText[w.newA:w.newB]) {
		if leftKeep > 0 {
			leftKeep--
			if leftKeep == 0 {
				w.oldA = 0
			} else {
				w.oldA = oldSegs[leftKeep-1].End
			}
		}
		if rightKeep < len(oldSegs) {
			rightKeep++
			if rightKeep == len(oldSegs) {
				w.oldB = len(oldText)
			} else {
				w.oldB = oldSegs[rightKeep].Start
			}
		}
		w.newA = w.oldA
		w.newB = w.oldB + w.delta
	}
	return w, leftKeep, rightKeep, true
}

// splitPartials carves strong straddler heads/tails out of the window (their
// script is context-free), shrinking the window to the edit. Runs after
// widening so partials never overlap the widened window. Weak edits, weak
// edges, or out-of-window straddlers keep the window whole.
func splitPartials(newText string, oldSegs []Segment, loc keptLoc, w reuseWin, oldA, oldB, newA, newB int) (reuseWin, *Segment, *Segment, bool) {
	fail := func() (reuseWin, *Segment, *Segment, bool) { return reuseWin{}, nil, nil, false }
	if windowHasWeak(newText[newA:newB]) {
		return w, nil, nil, true
	}
	var head, tail *Segment
	pre := oldA
	if loc.hasHead && oldSegs[loc.sL0].Start >= w.oldA && isStrongRuneBefore(newText, pre) {
		sg := oldSegs[loc.sL0]
		head = &Segment{Text: newText[sg.Start:pre], Start: sg.Start, End: pre,
			Direction: sg.Direction, Script: sg.Script, Level: sg.Level}
		w.oldA = pre
		w.newA = pre
	}
	if loc.hasTail && oldSegs[loc.sR0-1].End <= w.oldB && isStrongRuneAt(newText, newB) {
		sg := oldSegs[loc.sR0-1]
		ns, ne := oldB+w.delta, sg.End+w.delta
		tail = &Segment{Text: newText[ns:ne], Start: ns, End: ne,
			Direction: sg.Direction, Script: sg.Script, Level: sg.Level}
		w.oldB = oldB
		w.newB = newB
	}
	_ = fail
	return w, head, tail, true
}

// checkWindow validates final window bounds, rune alignment and RTL absence.
func checkWindow(newText string, w reuseWin, newA, newB int) bool {
	if w.newA > newA || w.newB < newB || w.newA > w.newB || w.newB > len(newText) {
		return false
	}
	if !runeBoundary(newText, w.newA) || !runeBoundary(newText, w.newB) {
		return false
	}
	return !hasStrongRTL(newText[w.newA:w.newB])
}

// resegmentWindow re-segments the window into newText coordinates. Nil when
// the window shows odd levels (must stay LTR-only).
func resegmentWindow(newText string, w reuseWin) []Segment {
	raw := SegmentText(newText[w.newA:w.newB])
	mid := make([]Segment, 0, len(raw))
	for _, s := range raw {
		if s.Level%2 == 1 {
			return nil
		}
		mid = append(mid, Segment{
			Text:      newText[w.newA+s.Start : w.newA+s.End],
			Start:     w.newA + s.Start,
			End:       w.newA + s.End,
			Direction: s.Direction,
			Script:    s.Script,
			Level:     s.Level,
		})
	}
	return mid
}

// stitch emits prefix, head, mid, tail and suffix in one pass: each piece is
// contiguity-checked, seam-strength-checked (no weak run may cross a stitch
// seam) and merged into the previous piece when scripts match. Checking
// before merging keeps same-script seams honest. Full coverage is verified
// at the end; any violation falls back to the full pass.
func stitch(newText string, oldSegs []Segment, leftKeep, rightKeep int, head, tail *Segment, mid []Segment, delta int, full func() []Segment) []Segment {
	out := make([]Segment, 0, leftKeep+len(mid)+len(oldSegs)-rightKeep+2)
	emit := func(s Segment) bool {
		if s.Start < 0 || s.End < s.Start || s.End > len(newText) || !runeBoundary(newText, s.Start) || !runeBoundary(newText, s.End) {
			return false
		}
		if n := len(out); n > 0 {
			p := out[n-1].End
			if s.Start != p {
				return false
			}
			if p > 0 && p < len(newText) && (!isStrongRuneBefore(newText, p) || !isStrongRuneAt(newText, p)) {
				return false
			}
			if prev := &out[n-1]; prev.End == s.Start && prev.Level == s.Level &&
				prev.Direction == s.Direction && prev.Script == s.Script {
				prev.End = s.End
				prev.Text += s.Text
				return true
			}
		}
		out = append(out, s)
		return true
	}
	for _, s := range oldSegs[:leftKeep] {
		if !emit(Segment{Text: newText[s.Start:s.End], Start: s.Start, End: s.End,
			Direction: s.Direction, Script: s.Script, Level: s.Level}) {
			return full()
		}
	}
	if head != nil && !emit(*head) {
		return full()
	}
	for _, s := range mid {
		if !emit(s) {
			return full()
		}
	}
	if tail != nil && !emit(*tail) {
		return full()
	}
	for _, s := range oldSegs[rightKeep:] {
		ns, ne := s.Start+delta, s.End+delta
		if !emit(Segment{Text: newText[ns:ne], Start: ns, End: ne,
			Direction: s.Direction, Script: s.Script, Level: s.Level}) {
			return full()
		}
	}
	if len(out) == 0 || out[0].Start != 0 || out[len(out)-1].End != len(newText) {
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
