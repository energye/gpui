package rendering

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestRenderText_FitRunPrefix_WordPriority verifies that soft wrapping through
// fitRunPrefix breaks at word boundaries first (UAX#14, Flutter/SkParagraph
// alignment), never in the middle of a word — except the single over-long
// word fallback, which is per-character and covered by the other test.
func TestRenderText_FitRunPrefix_WordPriority(t *testing.T) {
	face, _, err := text.LoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("LoadDefaultFace unavailable: %v", err)
	}
	rt := &RenderText{}
	run := TextRun{Face: face, FontSize: 14}
	full := "Hello world test phrase"
	words := strings.Fields(full)

	for _, budget := range []float64{20, 30, 40, 50, 60, 80, 100, 120, 140, 160} {
		chunk, rest := fitRunPrefix(rt, run, full, budget)
		if chunk == "" {
			t.Fatalf("budget=%v: empty chunk", budget)
		}
		if chunk+rest != full {
			t.Errorf("budget=%v: split not lossless: chunk=%q rest=%q", budget, chunk, rest)
		}
		if budget > 160 && rest != "" {
			t.Errorf("budget=%v: whole text should fit", budget)
		}
		if rest == "" {
			continue // whole text on one line — no boundary to check
		}
		// Word-boundary contract: every complete word in chunk must be a full
		// word, and if chunk mid-cuts a word, that word is the first over-long
		// word (chunk = its whole prefix path). Practically: chunk's final
		// word must equal a full word of the source, OR chunk is a strict
		// prefix of the first word (over-long fallback for tiny budgets).
		chunkTrim := strings.TrimRight(chunk, " \t")
		if chunkTrim == "" {
			continue
		}
		finalSeg := chunkTrim
		if sp := strings.LastIndexByte(chunkTrim, ' '); sp >= 0 {
			finalSeg = chunkTrim[sp+1:]
		}
		fullWord := false
		for _, w := range words {
			if finalSeg == w {
				fullWord = true
				break
			}
		}
		// Over-long first-word fallback: chunk fits less than the whole
		// first word (only valid for budgets below the word's width).
		firstWordPrefix := strings.HasPrefix(words[0], finalSeg) && finalSeg != ""
		if !fullWord && !firstWordPrefix {
			t.Errorf("budget=%v: mid-word break chunk=%q rest=%q", budget, chunk, rest)
		}
	}
}

// TestRenderText_FitRunPrefix_SingleOverlongWord verifies the per-character
// fallback: a single word wider than the budget still produces a chunk
// (no infinite loop, lossless split).
func TestRenderText_FitRunPrefix_SingleOverlongWord(t *testing.T) {
	face, _, err := text.LoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("LoadDefaultFace unavailable: %v", err)
	}
	rt := &RenderText{}
	run := TextRun{Face: face, FontSize: 14}
	long := "Supercalifragilisticexpialidocious"
	for _, budget := range []float64{5, 10, 15, 20, 30} {
		chunk, rest := fitRunPrefix(rt, run, long, budget)
		if chunk == "" {
			t.Fatalf("budget=%v: expected non-empty chunk (char fallback)", budget)
		}
		if chunk+rest != long {
			t.Errorf("budget=%v: split not lossless", budget)
		}
		if rest == "" && budget < 160 {
			t.Errorf("budget=%v: over-long word should leave remainder", budget)
		}
	}
}
