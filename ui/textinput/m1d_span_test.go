//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package textinput

import (
	"testing"
)

func TestEditorSpan_AddConsume_M1(t *testing.T) {
	ed := New()
	ed.SetText("aaa\nbbb", TextRange{Base: 7, Extent: 7}, TextRange{}, 0)
	// SetText本身记了全量区间,先消费掉.
	_, _, _, _, _ = ed.ConsumeEditSpan()
	ed.SetCaret(5)
	if !ed.AddText("X") {
		t.Fatalf("AddText失败")
	}
	oA, oB, nA, nB, ok := ed.ConsumeEditSpan()
	if !ok || oA != 5 || oB != 5 || nA != 5 || nB != 6 {
		t.Fatalf("区间应为(5,5,5,6),得(%d,%d,%d,%d,%v)", oA, oB, nA, nB, ok)
	}
	if _, _, _, _, ok := ed.ConsumeEditSpan(); ok {
		t.Fatalf("消费后应清除")
	}
}

func TestEditorSpan_DoubleInvalidate_M1(t *testing.T) {
	ed := New()
	ed.SetText("aaa\nbbb", TextRange{Base: 7, Extent: 7}, TextRange{}, 0)
	_, _, _, _, _ = ed.ConsumeEditSpan()
	ed.SetCaret(7)
	ed.AddText("X")
	ed.AddText("Y")
	if _, _, _, _, ok := ed.ConsumeEditSpan(); ok {
		t.Fatalf("sync前两次变更应失效回退diff")
	}
}
