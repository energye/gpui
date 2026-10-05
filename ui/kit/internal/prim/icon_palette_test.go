//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package prim

import (
	"testing"

	"github.com/energye/gpui/ui/theme"
)

// Palette must match @ant-design/colors generate()[0] for the three
// demo primaries (blue/pink/green preset bg values from antd docs).
func TestIcon_PaletteSecondary(t *testing.T) {
	cases := []struct{ base, want0 string }{
		{"#1677ff", "#e6f4ff"},
		{"#eb2f96", "#fff0f6"},
		{"#52c41a", "#f6ffed"},
	}
	for _, c := range cases {
		pal := AntdPalette(c.base)
		if len(pal) != 10 {
			t.Fatalf("palette(%s) len=%d want 10", c.base, len(pal))
		}
		if pal[0] != c.want0 {
			t.Errorf("palette(%s)[0]=%s want %s (full=%v)", c.base, pal[0], c.want0, pal)
		}
		if pal[5] != c.base {
			t.Errorf("palette(%s)[5]=%s want base %s", c.base, pal[5], c.base)
		}
	}
	got := AntdSecondaryForMain(theme.Hex("#eb2f96"))
	if got.A != 1 {
		t.Fatalf("derived secondary alpha=%v want 1 (opaque, never translucent)", got.A)
	}
	want := theme.Hex("#fff0f6")
	if got.R != want.R || got.G != want.G || got.B != want.B {
		t.Errorf("derived secondary=%v want %v", got, want)
	}
}
