package textinput

import (
	"sort"
	"strings"
	"testing"
	"time"
)

// editorAddTextTime appends one rune at the caret; OnChange stays nil so the
// measurement covers the Editor storage/edit path, not the UI sync (V2).
func editorAddTextTime(t *testing.T, lines, reps int) time.Duration {
	t.Helper()
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", lines))
	e.OnChange = nil
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		t0 := time.Now()
		if !e.AddText("x") {
			t.Fatalf("AddText rejected at %d", i)
		}
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

// TestEditorEdit_NoLinear锁 Editor 编辑路径的规模无关性:1M 行(树模式)
// 与 500 行(字符串模式)的单次追加应同量级,不能被整串拼接拉成线性.
func TestEditorEdit_NoLinear(t *testing.T) {
	small := editorAddTextTime(t, 500, 50)
	big := editorAddTextTime(t, 60000, 50)
	ratio := float64(big) / float64(small)
	t.Logf("Editor AddText 500行=%v 60000行=%v ratio=%.2f", small, big, ratio)
	if ratio > 8 {
		t.Fatalf("Editor编辑疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}

func BenchmarkEditorAddText(b *testing.B) {
	for _, n := range []int{10000, 100000, 1000000} {
		e := New()
		e.SetSingleLine(false)
		e.SetTextSimple(strings.Repeat("line content here\n", n))
		e.OnChange = nil
		b.Run(strings.TrimSpace(strings.ReplaceAll(map[int]string{10000: "1e4", 100000: "1e5", 1000000: "1e6"}[n], " ", "")), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				e.AddText("x")
			}
		})
	}
}

func editorBackspaceTime(t *testing.T, lines, reps int) time.Duration {
	t.Helper()
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", lines))
	e.OnChange = nil
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		t0 := time.Now()
		e.Backspace()
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func TestEditorBackspace_NoLinear(t *testing.T) {
	small := editorBackspaceTime(t, 500, 50)
	big := editorBackspaceTime(t, 60000, 50)
	ratio := float64(big) / float64(small)
	t.Logf("Editor Backspace 500行=%v 60000行=%v ratio=%.2f", small, big, ratio)
	if ratio > 8 {
		t.Fatalf("Backspace疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}

func editorLineNavTime(t *testing.T, lines, reps int) time.Duration {
	t.Helper()
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", lines))
	e.OnChange = nil
	er := e.EditableRange()
	start := er.End()
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		e.SetSelection(TextRange{Base: start, Extent: start})
		t0 := time.Now()
		e.MoveCursorUp()
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func TestEditorLineNav_NoLinear(t *testing.T) {
	small := editorLineNavTime(t, 500, 50)
	big := editorLineNavTime(t, 60000, 50)
	ratio := float64(big) / float64(small)
	t.Logf("Editor MoveCursorUp 500行=%v 60000行=%v ratio=%.2f", small, big, ratio)
	if ratio > 8 {
		t.Fatalf("行导航疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}

func editorIMETime(t *testing.T, lines, reps int) time.Duration {
	t.Helper()
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", lines))
	e.OnChange = nil
	e.BeginComposing()
	er := e.EditableRange()
	at := er.End()
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		t0 := time.Now()
		e.UpdateComposingText("x", TextRange{Base: at, Extent: at})
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func TestEditorIME_NoLinear(t *testing.T) {
	small := editorIMETime(t, 500, 50)
	big := editorIMETime(t, 60000, 50)
	ratio := float64(big) / float64(small)
	t.Logf("Editor IME composing 500行=%v 60000行=%v ratio=%.2f", small, big, ratio)
	if ratio > 8 {
		t.Fatalf("IME合成疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}
