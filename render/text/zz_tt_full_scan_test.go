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

import (
	"os"
	"strings"
	"testing"
)

// TestScanWqyCJKFullComposite：Full hint 下 composite 子组件逐条 hint 回归窗。
//
// 修复：ttGlyphLoader.hintComponent 回调 —— 每个带 bytecode 的子组件在合并前
// 先 hint 一次（tt_instance.hintGlyph），再叠加组件变换/位移合入父轮廓，最后
// 父轮廓跑 composite 自身程序（若有），对齐 FT 两段 hint（子 is_composite=0，
// 父 is_composite=1）语义。Trace 级验证：全字集 FT-go 指令序列 3000 gids 0 bad。
//
// 本窗做像素级回归：ftexp bcontour 取 wqy 的 FT_LOAD_TARGET_NORMAL(full) 26.6
// 轮廓，Go 取 ExtractOutlineHinted(..., HintingFull)，对 4 个复合字 + 全 3000
// 字逐点最近邻匹配（1/64 容差，复用 zz_wqy_scan_test.go 口径）。
func TestScanWqyCJKFullComposite(t *testing.T) {
	raw, err := os.ReadFile("hint/testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != 3000 {
		t.Fatalf("cjk3000.txt = %d chars, want 3000", len(chars))
	}
	fontPath := "testdata/wqy-microhei.ttf"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	face := src.Parsed()
	ext := NewOutlineExtractor()

	// 4 个 composite 子组件自带 bytecode 的字（wqy gid：14031/4366/14009/14046）。
	compRunes := map[rune]bool{'素': true, '坟': true, '綊': true, '累': true}
	compBad := map[rune]int{}
	compSeen := map[rune]bool{}

	for _, px := range []int{12, 16} {
		ft := batchWqyContour26Hint(t, fontPath, chars, float64(px), "f")
		if len(ft) < 2900 {
			t.Fatalf("wqy full px%d: only %d glyphs parsed, want >=2900 (parser regression)", px, len(ft))
		}
		bad := 0
		var badR []rune
		for _, r := range chars {
			ftFound, ok := ft[r]
			if !ok {
				continue
			}
			gid := face.GlyphIndex(r)
			out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingFull)
			if err != nil || out == nil {
				bad++
				compSeen[r] = true
				if compRunes[r] {
					compBad[r]++
				}
				if len(badR) < 30 {
					badR = append(badR, r)
				}
				continue
			}
			gotPts := outlinePoints26(out)
			if len(ftFound) > 0 && len(gotPts) == 0 {
				bad++
				compSeen[r] = true
				if compRunes[r] {
					compBad[r]++
				}
				if len(badR) < 30 {
					badR = append(badR, r)
				}
				continue
			}
			matched := matchPointSets(ftFound, gotPts, 64)
			if matched < len(ftFound) && matched < len(gotPts) {
				bad++
				compSeen[r] = true
				if compRunes[r] {
					compBad[r]++
				}
				if len(badR) < 30 {
					badR = append(badR, r)
				}
			}
		}
		t.Logf("composite full px%d: bad=%d/%d badR=%v", px, bad, len(chars), runes8(badR))
		// 4 个复合字是本次修复核心：必须 0 bad。全字集同样 0 bad。
		for r := range compRunes {
			if compSeen[r] && compBad[r] > 0 {
				t.Errorf("full px%d: composite %q bad=%d (must be 0)", px, r, compBad[r])
			}
		}
		if bad > 0 {
			t.Errorf("full px%d: bad=%d/%d (must be 0)", px, bad, len(chars))
		}
	}
}
