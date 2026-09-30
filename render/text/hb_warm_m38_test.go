//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import "testing"

func TestLoadMultiFace_WarmsHbFonts_M38(t *testing.T) {
	hs, ok := GetShaper().(*HbShaper)
	if !ok {
		t.Skipf("custom shaper %T, Hb warm-up not applicable", GetShaper())
	}
	if _, _, err := LoadMultiFace(14); err != nil {
		t.Skipf("no system font for warm-up test: %v", err)
	}
	hs.ClearCache()
	face, _, err := LoadMultiFace(14)
	if err != nil || face == nil {
		t.Skipf("reload after ClearCache: %v", err)
	}
	var faces []Face
	if mf, ok := face.(*MultiFace); ok {
		faces = mf.faces
	} else {
		faces = []Face{face}
	}
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	for _, f := range faces {
		if f == nil {
			continue
		}
		src := f.Source()
		if src == nil {
			continue
		}
		if _, ok := hs.faces[src]; !ok {
			t.Fatalf("source %p not warmed by LoadMultiFace", src)
		}
	}
}
