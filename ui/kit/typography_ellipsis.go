//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

import (
	"unicode/utf8"
)

// Ellipsis cutter (antd Base/Ellipsis.tsx binary-search semantics,
// typography.md §6.4 TYP-S4): measure the full text against the row
// budget, reserve the operator/suffix row, binary-search the largest
// prefix that fits, then slice + "…" + suffix.
//
// The measure function is injected so the render layer's real metrics
// can replace the estimator in tests and later legs.

// TextMeasurer reports how many text rows s occupies at the given width.
// A deterministic estimator is used when no face is attached (char-width
// heuristic); the render layer's MeasureString is the production source.
type TextMeasurer func(s string, width float64) int

// EllipsisCut is one cutter outcome.
type EllipsisCut struct {
	// Text is the truncated content (no ellipsis mark, no suffix).
	Text string
	// Ellipsized reports whether truncation happened.
	Ellipsized bool
}

// CutEllipsis runs the binary-search cut.
//
// text:    full content
// width:   available width per row
// rows:    visible row budget (<=0 → 1)
// suffix:  text reserved after the mark (kept out of the cut budget)
// mark:    ellipsis mark ("…")
// measure: row counter at the given width
//
// The suffix participates like antd's: its rows are reserved from the
// budget, so the cut converges with the suffix visible.
func CutEllipsis(text string, width float64, rows int, suffix, mark string, measure TextMeasurer) EllipsisCut {
	if rows <= 0 {
		rows = 1
	}
	if measure == nil || width <= 0 || text == "" {
		return EllipsisCut{Text: text}
	}
	// Fast path: everything fits (mark not needed; suffix measured only
	// when present so the empty suffix adds no newline row).
	if suffix != "" {
		if measure(text+"\n"+suffix, width) <= rows {
			return EllipsisCut{Text: text}
		}
	} else if measure(text, width) <= rows {
		return EllipsisCut{Text: text}
	}
	// Binary search the largest prefix where cut+mark (+suffix on the
	// final row) stays inside the row budget — the suffix participates
	// in the same budget, exactly like antd's reserved operator row.
	lo, hi := 0, utf8.RuneCountInString(text)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		cut := string([]rune(text)[:mid])
		var probe string
		if suffix != "" {
			probe = cut + mark + "\n" + suffix
		} else {
			probe = cut + mark
		}
		if measure(probe, width) <= rows {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return EllipsisCut{Text: mark, Ellipsized: true}
	}
	return EllipsisCut{
		Text:       string([]rune(text)[:lo]) + mark,
		Ellipsized: true,
	}
}

// CutEllipsisMiddle is the middle ellipsis (TYP-S5 middle form): fixed
// head + "…" + fixed tail, no search. antd keeps head and tail segments
// and never re-measures.
func CutEllipsisMiddle(text string, headRunes, tailRunes int, mark string) EllipsisCut {
	if headRunes < 0 {
		headRunes = 0
	}
	if tailRunes < 0 {
		tailRunes = 0
	}
	runes := []rune(text)
	if headRunes+tailRunes >= len(runes) {
		return EllipsisCut{Text: text}
	}
	head := string(runes[:headRunes])
	tail := string(runes[len(runes)-tailRunes:])
	return EllipsisCut{
		Text:       head + mark + tail,
		Ellipsized: true,
	}
}

// EstimateRows is the no-face deterministic measurer: a rough per-rune
// width model (CJK wide, latin narrow) divided by the row width. It is
// only for tests without a face; production passes a measure bound to
// the real face.
func EstimateRows(width float64) TextMeasurer {
	return func(s string, w float64) int {
		if w <= 0 {
			w = width
		}
		rows := 1
		lineW := 0.0
		for _, r := range s {
			if r == '\n' {
				rows++
				lineW = 0
				continue
			}
			rw := 8.0        // latin ≈ half the 14px body em
			if r >= 0x2E80 { // CJK and fullwidth ranges
				rw = 14.0
			}
			if lineW+rw > w {
				rows++
				lineW = rw
			} else {
				lineW += rw
			}
		}
		return rows
	}
}
