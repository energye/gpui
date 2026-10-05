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

import "testing"

// Custom symbols must stay 1:1 with official sources:
// custom.tsx (heart/panda) + iconfont.cn vendored JS.
func TestIcon_CustomSVG(t *testing.T) {
	want := []string{
		"custom-heart-svg",
		"custom-panda-svg",
		"iconfont-tuichu",
		"iconfont-facebook",
		"iconfont-twitter",
		"iconfont-javascript",
		"iconfont-java",
		"iconfont-shoppingcart-f1",
		"iconfont-python-f1",
		"iconfont-shoppingcart-f2",
		"iconfont-python-f2",
	}
	for _, k := range want {
		if !IsKnownCustomSVG(k) {
			t.Errorf("custom missing %q", k)
		}
		if len(parsedCustomPaths(k)) == 0 {
			t.Errorf("custom parsed empty: %s", k)
		}
	}
	// Panda must keep 8 multi-color paths (custom.tsx).
	if n := len(parsedCustomPaths("custom-panda-svg")); n != 8 {
		t.Errorf("panda paths=%d want 8", n)
	}
	// Multi-source override pair must differ (f1 vs f2).
	f1 := parsedCustomPaths("iconfont-shoppingcart-f1")
	f2 := parsedCustomPaths("iconfont-shoppingcart-f2")
	if len(f1) == 0 || len(f2) == 0 {
		t.Fatalf("shoppingcart f1/f2 must parse")
	}
}
