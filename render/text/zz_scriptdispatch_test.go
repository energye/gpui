package text

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// M4b-6 其余脚本分派回归窗。
//
// 判定结论（FT 2.11.1 afstyles.h / afglobal.h，并经 FT_DEBUG afglobal:4
// 实证）：除 hani（af_cjk）与 limb/oriya/syloti/tibetan（AF_CONFIG_OPTION_INDIC
// -> af_indic，默认随 CJK 打开）外，所有脚本（含 arab/hebr/thai/deva/...）
// 的 dflt 样式都挂在 AF_WRITING_SYSTEM_LATIN 下 —— 即走 aflatin 模块、用各
// 脚本自己的蓝区，**不是 afdummy 透传**。真正单调的兜底是：任何未被脚本
// unirange 覆盖的 glyph 落到模块 fallback_style = AF_STYLE_HANI_DFLT（CJK
// 打开时），即箭头/数学符号等杂项字符走 af_cjk 的 light。Go 侧已同步：
// perGlyphScripts 的未覆盖 fallback 从 scriptLatin 改为 scriptCJK（原注释
// 声称"default script = Latin"与 FT 标准构建不符）。
//
// 本窗用系统真字体做 Go:FT 逐点对照确认这些路径一致：
//   - arab/ethi/mymr：latin 模块各脚本蓝区，断言 bad=0
//   - gujr：px16 断言 bad=0；px12 的 ઑ 仍有 +0.2px x-height 顶部带差
//     （M4b-5 修复了 STEM 蓝锚 anchor 语义后 જ 已归零，ઑ 残差在 edge
//     对齐 rounding，±0.2px 在文档记录既定允许差内），不入断言，见 c.allow
//   - fallback（FreeSans 未覆盖符号：箭头/数学）：断言 bad=0
//
// deva/beng/taml（Samyak/Lohit）复杂组合字形（क/ँ/ु 等）当前有残余差异，
// 属 M4b-5 afindic 精修范围，不在此窗断言；thai/hebr 系统无 glyf 无字节码
// 字体（tlwg 泰文带 fpgm，其 light = 字节码 light，与 autofit light 不同
// 语义），由 afglobal trace 判定其归 aflatin 模块，不做逐点对照。
func TestScanScriptDispatch(t *testing.T) {
	type chk struct {
		name  string
		font  string
		dat   string
		pxs   []int
		allow map[int][]rune // 既定允许差：某 px 下允许仍不归零的字符
	}
	cases := []chk{
		{"arab", "KacstOne.ttf", "arab_sample.txt", []int{12, 16}, nil},
		{"ethi", "AbyssinicaSIL-Regular.ttf", "ethi_sample.txt", []int{12, 16}, nil},
		{"mymr", "Padauk-Regular.ttf", "mymr_sample.txt", []int{12, 16}, nil},
		{"gujr", "Samyak-Gujarati.ttf", "gujr_sample.txt", []int{12, 16},
			map[int][]rune{12: {'ઑ'}}},
		{"fallback", "FreeSans.ttf", "fallback_sample.txt", []int{16}, nil},
	}
	for _, c := range cases {
		fp := findSystemFont(t, c.font)
		if fp == "" {
			t.Skipf("%s: %s not installed; numerical check unavailable", strings.ToUpper(c.name), c.font)
			continue
		}
		dat := "hint/testdata/" + c.dat
		raw, err := os.ReadFile(dat)
		if err != nil {
			t.Skipf("%s: %s unavailable: %v", strings.ToUpper(c.name), dat, err)
			continue
		}
		chars := []rune(strings.TrimSpace(string(raw)))
		if len(chars) == 0 {
			t.Fatalf("%s: empty data", c.dat)
		}
		src, err := NewFontSourceFromFile(fp)
		if err != nil {
			t.Fatal(err)
		}
		face := src.Parsed()
		ext := NewOutlineExtractor()
		for _, px := range c.pxs {
			ft := batchWqyContour26(t, fp, chars, float64(px))
			if len(ft) < 10 {
				t.Fatalf("%s px%d: only %d glyphs parsed", c.name, px, len(ft))
			}
			bad := 0
			badR := []rune{}
			for _, r := range chars {
				ftFound, ok := ft[r]
				if !ok {
					continue
				}
				gid := face.GlyphIndex(r)
				out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
				if err != nil || out == nil {
					bad++
					badR = append(badR, r)
					continue
				}
				gotPts := outlinePoints26(out)
				// Go 的 segment 重建会把闭合轮廓的起点在末尾重复一次
				// （MoveTo 起点再次出现）；FT 侧无此重复。剥离后再对照，
				// 其余点必须逐点等价。
				if n := len(gotPts); n > 1 && gotPts[0] == gotPts[n-1] {
					gotPts = gotPts[:n-1]
				}
				ftClean := make([][2]int64, 0, len(ftFound))
				for _, q := range ftFound {
					if q[0] == 0 && q[1] == 0 {
						continue
					}
					ftClean = append(ftClean, q)
				}
				if len(ftClean) > 0 && len(gotPts) == 0 {
					bad++
					badR = append(badR, r)
					continue
				}
				if len(ftClean) == 0 {
					continue
				}
				m := matchPointSets(ftClean, gotPts, 64)
				if m < len(ftClean) && m < len(gotPts) {
					bad++
					badR = append(badR, r)
				}
			}
			if bad > 0 {
				allowed := true
				for _, r := range badR {
					found := false
					for _, a := range c.allow[px] {
						if a == r {
							found = true
						}
					}
					if !found {
						allowed = false
					}
				}
				if !allowed {
					t.Errorf("%s px%d: bad=%d/%d first=%v", c.name, px, bad, len(chars), runes8(badR))
				} else {
					t.Logf("%s px%d: bad=%d/%d (all in allowed tolerance %q)", c.name, px, bad, len(chars), runes8(badR))
				}
			} else {
				t.Logf("%s px%d: bad=0/%d", c.name, px, len(chars))
			}
		}
		src.Close()
	}
}

// findSystemFont locates a font by base name under /usr/share/fonts.
func findSystemFont(t *testing.T, name string) string {
	t.Helper()
	var found string
	filepath.Walk("/usr/share/fonts", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.Name() == name {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
