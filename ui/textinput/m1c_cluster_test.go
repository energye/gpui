package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// M1-c红灯:横向移动必须按字素簇跳,不得劈开组合音标.
func TestClusterBoundary_ArrowSkipsMark_M1(t *testing.T) {
	txt := "e\u0301x"
	ed := New()
	ed.SetText(txt, TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	ed.SetCaret(0)
	lay := rendering.BuildTextLayout(txt, nil, 14, 0, 1.2)
	if !ed.MoveVisual(1, lay) {
		t.Fatalf("右移失败")
	}
	if got := ed.GetCursorOffset(); got != 3 {
		t.Fatalf("右移应整簇跳过e+音标落到3,得%d", got)
	}
	if !ed.MoveVisual(-1, lay) {
		t.Fatalf("左移失败")
	}
	if got := ed.GetCursorOffset(); got != 0 {
		t.Fatalf("左移应整簇跳回0,得%d", got)
	}
}

// M1-c红灯:IME置位(affinity)在簇内必须吸附到边界.
func TestClusterBoundary_AffinitySnap_M1(t *testing.T) {
	txt := "e\u0301x"
	ed := New()
	ed.SetText(txt, TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	ed.SetCaretWithAffinity(1, AffinityDownstream)
	if got := ed.GetCursorOffset(); got != 0 {
		t.Fatalf("downstream置位1应吸附到簇首0,得%d", got)
	}
	ed.SetCaretWithAffinity(1, AffinityUpstream)
	if got := ed.GetCursorOffset(); got != 3 {
		t.Fatalf("upstream置位1应吸附到簇尾3,得%d", got)
	}
}

// M1-c红灯:上下键(粘滞列)不得落进簇内.
func TestClusterBoundary_VerticalNoSplit_M1(t *testing.T) {
	txt := "e\u0301x\ny"
	ed := New()
	ed.SetText(txt, TextRange{Base: 0, Extent: 0}, TextRange{}, 0)
	ed.SetCaret(6)
	lay := rendering.BuildTextLayout(txt, nil, 14, 0, 1.2)
	if !ed.MoveVisualUp(lay) {
		t.Fatalf("上移失败")
	}
	if got := ed.GetCursorOffset(); got == 1 {
		t.Fatalf("上移落进簇内(1),必须吸附到边界")
	}
}
