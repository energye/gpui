//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"testing"
)

func TestClusterBoundary_ClickNoSplit_M1(t *testing.T) {
	txt := "e\u0301x"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	var x1 float64
	for _, c := range lay.LineCarets(0) {
		if c.ByteOff == 1 {
			x1 = c.X
		}
	}
	off, _ := lay.GetPositionForOffset(x1, lay.LineTop(0)+1)
	if off == 1 {
		t.Fatalf("点击落进簇内(1),必须吸附到边界")
	}
	if off != 0 {
		t.Fatalf("簇内点击应吸附到簇首0,得%d", off)
	}
}

func TestClusterBoundary_ClickEdges_M1(t *testing.T) {
	txt := "e\u0301x"
	lay := BuildTextLayout(txt, nil, 14, 0, 1.2)
	top := lay.LineTop(0) + 1
	if off, _ := lay.GetPositionForOffset(-5, top); off != 0 {
		t.Fatalf("行首点击应为0,得%d", off)
	}
	tail := lay.LineCarets(0)
	last := tail[len(tail)-1]
	if off, _ := lay.GetPositionForOffset(last.X+50, top); off != last.ByteOff {
		t.Fatalf("行尾点击应为%d,得%d", last.ByteOff, off)
	}
}
