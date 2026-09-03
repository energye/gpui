package rendering

import (
	"testing"
)

// M1-d span通道:正确区间走增量且结果一致;撒谎区间回退且结果仍一致.

func TestSpanHit_M1(t *testing.T) {
	rt := NewRenderText("aaa\nbbb\nccc")
	_ = rt.TextLayout()
	hits0 := rt.lcache.live.spansHit
	rt.SetTextSpan("aaa\nBBXb\nccc", 4, 7, 4, 8)
	got := rt.TextLayout()
	if rt.lcache.live.spansHit != hits0+1 {
		t.Fatalf("正确区间应走span增量")
	}
	want := BuildTextLayout("aaa\nBBXb\nccc", nil, 14, 0, 1.2)
	if !m1CacheLinesEqual(got, want) {
		t.Fatalf("span快照与直接构建不一致")
	}
}

func TestSpanFallback_M1(t *testing.T) {
	rt := NewRenderText("aaa\nbbb\nccc")
	_ = rt.TextLayout()
	// 非法区间(长度方程不对):当普通SetText用,结果仍对.
	rt.SetTextSpan("aaa\nBBXb\nccc", 0, 1, 0, 99)
	got := rt.TextLayout()
	want := BuildTextLayout("aaa\nBBXb\nccc", nil, 14, 0, 1.2)
	if !m1CacheLinesEqual(got, want) {
		t.Fatalf("回退快照与直接构建不一致")
	}
	// 过期链:两次SetTextSpan只跑一次布局,第二区间相对中间串,
	// RenderText直接丢区间走普通通道,结果仍对.
	rt2 := NewRenderText("aaa\nbbb\nccc")
	_ = rt2.TextLayout()
	rt2.SetTextSpan("aaa\nBBb\nccc", 4, 7, 4, 7)
	rt2.SetTextSpan("aaa\nBBXb\nccc", 4, 7, 4, 8)
	got2 := rt2.TextLayout()
	if !m1CacheLinesEqual(got2, want) {
		t.Fatalf("过期链回退快照不一致")
	}
	// 引擎级撒谎区间:抽查不过→spansMiss+1,结果仍对.
	c := newLayoutCache()
	textLayoutGen++
	c.update("aaa\nbbb\nccc", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip)
	miss0 := c.live.spansMiss
	got3 := c.updateSpan("aaa\nBBXb\nccc", nil, 14, 0, 1.2, 0.55, 0, TextOverflowClip,
		editSpan{oldA: 0, oldB: 1, newA: 0, newB: 2})
	if c.live.spansMiss != miss0+1 {
		t.Fatalf("引擎抽查不过应记miss")
	}
	if !m1CacheLinesEqual(got3, want) {
		t.Fatalf("引擎回退快照不一致")
	}
}
