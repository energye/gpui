package rendering

import (
	"testing"

	"github.com/energye/gpui/render"
)

type countingLineSource struct {
	constLineSource
	lineAtCalls  int
	lastLine     int
	lineStartHit int
}

func (s *countingLineSource) LineAt(k int) string {
	s.lineAtCalls++
	s.lastLine = k
	return s.constLineSource.LineAt(k)
}

func (s *countingLineSource) LineStart(k int) int {
	s.lineStartHit++
	return s.constLineSource.LineStart(k)
}

func TestRenderText_VirtualPaint(t *testing.T) {
	const text = "line content here"
	src := &countingLineSource{constLineSource: constLineSource{n: 1000000, text: text, width: len(text) + 1}}
	rt := NewRenderText("")
	rt.SetVirtualSource(src)
	lh := lineHeightFor(nil, 14, 1.2)
	scrollY := 500000 * lh
	rt.SetViewportRect(0, 800, scrollY, 600)
	dc := render.NewContext(800, 600)
	pc := NewPaintContext(dc, 1)
	rt.Paint(pc)

	if src.lineAtCalls == 0 {
		t.Fatalf("虚拟绘制没有物化任何行")
	}
	if src.lineAtCalls > 200 {
		t.Fatalf("虚拟绘制物化了 %d 行，未按视口裁剪", src.lineAtCalls)
	}
	if src.lastLine < 499900 || src.lastLine > 500100 {
		t.Fatalf("绘制窗口落在第 %d 行，期望 500000 附近", src.lastLine)
	}
	if rt.vtext.CacheLen() > 3*200 {
		t.Fatalf("cache=%d 未保持 O(可见行)", rt.vtext.CacheLen())
	}
}

func TestRenderText_VirtualPaintNeedsWindow(t *testing.T) {
	src := &countingLineSource{constLineSource: constLineSource{n: 1000, text: "x", width: 2}}
	rt := NewRenderText("")
	rt.SetVirtualSource(src)
	dc := render.NewContext(100, 100)
	rt.Paint(NewPaintContext(dc, 1))
	if src.lineAtCalls != 0 {
		t.Fatalf("无 SetViewportRect 时不应物化行，实际 %d", src.lineAtCalls)
	}
}
