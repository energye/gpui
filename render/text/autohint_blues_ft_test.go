package text

import (
	"testing"
)

func _rawFontData(font ParsedFont) []byte {
	if provider, ok := font.(RawFontDataProvider); ok {
		return provider.RawFontData()
	}
	return nil
}

// TestFindBestYFTAlign verifies findBestYContour reproduces FreeType's
// afcjk blue-zone values (afcjk.c af_cjk_metrics_init_blues) for every
// character in the CJKB top fills/flats groups. Reference values are
// captured directly from FT_TRACE output (wqy-microhei-nohint.ttf,
// freetype 2.13.1). A want of 0 marks a character FT reports as
// "contains no (usable) outlines" and that we must skip as well.
func TestFindBestYFTAlign(t *testing.T) {
	fontPath := "testdata/wqy-microhei-nohint.ttf"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	face := src.Parsed()
	raw := _rawFontData(face)

	// FT trace: "blue zone 0 (top)" -> [overshoot values] group (fills).
	pairs := map[rune]int32{
		'他': 0, // FT: no usable outlines
		'们': 1584, '你': 1608, '來': 1656, '們': 1616, '到': 1656,
		'和': 1488, '地': 0, // FT: no usable
		'对': 1656, '對': 1656, '就': 1656, '席': 1664, '我': 1664,
		'时': 1656, '時': 1664, '會': 1640, '来': 1656, '為': 1664,
		'能': 976, '舰': 1640, '說': 1656, '说': 1664, '这': 1616,
		'這': 1688,
	}
	// FT trace: "blue zone 0 (top)" -> [reference values] group (flats).
	flatPairs := map[rune]int32{
		'同': 1592, '己': 0, '愿': 1600, '既': 1568, '星': 1608,
		'是': 1616, '景': 1608, '民': 1584, '照': 1584, '现': 1592,
		'現': 1584, '理': 1568, '用': 1560, '置': 1616, '要': 1600,
		'軍': 1600, '军': 0, '那': 1560, '配': 1600, '里': 1592,
		'開': 1624, '雷': 1624, '露': 1648, '面': 1576, '顾': 1592,
	}
	for name, group := range map[string]map[rune]int32{"flats": flatPairs, "fills": pairs} {
		for ch, want := range group {
			gid := face.GlyphIndex(ch)
			if gid == 0 {
				continue
			}
			got, ok := findBestYContour(raw, GlyphID(gid), true)
			if want == 0 {
				if ok {
					t.Errorf("%s: %q: expected skip (no usable), got bestY=%d ok=true", name, ch, got)
				}
				continue
			}
			if !ok || got != want {
				t.Errorf("%s: %q: got bestY=%d ok=%v, want %d", name, ch, got, ok, want)
			}
		}
	}
}
