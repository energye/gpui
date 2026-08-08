package hint

import (
	"encoding/binary"
	"fmt"
	"os"
)

// fontFileRaw 读取字体文件全部字节（测试辅助：hint 包不依赖渲染层）。
func fontFileRaw(t testingT, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("font unavailable: %v", err)
	}
	return raw
}

// sfntFaceBase 返回 TTC/单字体文件的 face 目录基址。
func sfntFaceBase(raw []byte, faceIndex int) (int, error) {
	if len(raw) < 4 {
		return 0, fmt.Errorf("font data too short")
	}
	switch string(raw[0:4]) {
	case "ttcf":
		if len(raw) < 12 {
			return 0, fmt.Errorf("ttc too short")
		}
		n := int(binary.BigEndian.Uint32(raw[8:12]))
		if faceIndex >= n {
			return 0, fmt.Errorf("ttc face %d out of range", faceIndex)
		}
		if 12+faceIndex*4+4 > len(raw) {
			return 0, fmt.Errorf("ttc offset out of range")
		}
		return int(binary.BigEndian.Uint32(raw[12+faceIndex*4 : 16+faceIndex*4])), nil
	case "OTTO", "true", "\x00\x01\x00\x00":
		return 0, nil
	}
	return 0, fmt.Errorf("unknown sfnt tag %q", string(raw[0:4]))
}

// sfntTableOffset 在 base 处的 sfnt 目录中定位 tag 表，返回文件绝对偏移。
func sfntTableOffset(raw []byte, base int, tag string) (int, error) {
	if base+4+2 > len(raw) {
		return 0, fmt.Errorf("sfnt base out of range")
	}
	if string(raw[base:base+4]) != "OTTO" && string(raw[base:base+4]) != "true" &&
		string(raw[base:base+4]) != "\x00\x01\x00\x00" {
		return 0, fmt.Errorf("not a sfnt face at base %d", base)
	}
	n := int(binary.BigEndian.Uint16(raw[base+4 : base+6]))
	for i := 0; i < n; i++ {
		e := base + 12 + i*16
		if e+16 > len(raw) {
			break
		}
		if string(raw[e:e+4]) == tag {
			off := int(binary.BigEndian.Uint32(raw[e+8 : e+12]))
			ln := int(binary.BigEndian.Uint32(raw[e+12 : e+16]))
			if off+ln > len(raw) {
				return 0, fmt.Errorf("%s table out of range", tag)
			}
			return off, nil
		}
	}
	return 0, fmt.Errorf("no %s table", tag)
}

// fontUpem 从 head 表读 unitsPerEm。
func fontUpem(raw []byte, faceIndex int) (int, error) {
	base, err := sfntFaceBase(raw, faceIndex)
	if err != nil {
		return 0, err
	}
	headOff, err := sfntTableOffset(raw, base, "head")
	if err != nil {
		return 0, err
	}
	if headOff+20 > len(raw) {
		return 0, fmt.Errorf("head table too short")
	}
	upem := binary.BigEndian.Uint16(raw[headOff+18 : headOff+20])
	if upem == 0 {
		return 0, fmt.Errorf("upem 0")
	}
	return int(upem), nil
}

// glyphIndexForRune 通过 cmap（format 12/4/0）查 rune→gid。
func glyphIndexForRune(raw []byte, faceIndex int, r rune) (uint16, error) {
	base, err := sfntFaceBase(raw, faceIndex)
	if err != nil {
		return 0, err
	}
	cmapOff, err := sfntTableOffset(raw, base, "cmap")
	if err != nil {
		return 0, err
	}
	if cmapOff+4 > len(raw) {
		return 0, fmt.Errorf("cmap too short")
	}
	n := int(binary.BigEndian.Uint16(raw[cmapOff+2 : cmapOff+4]))
	// harfbuzz 顺序（go-text cmap.go ProcessCmap，hb-ot-cmap-table.hh）：
	// symbol(3,0) → UCS4(3,10) → unicode full13(0,6) → full(0,4) →
	// unicode cs(3,1) → BMP(0,3) → deprecated(0,2)。
	type id struct{ plat, enc int }
	order := []id{
		{3, 0}, {3, 10}, {0, 6}, {0, 4}, {3, 1}, {0, 3}, {0, 2},
	}
	best := -1
	for _, want := range order {
		for i := 0; i < n; i++ {
			e := cmapOff + 4 + i*8
			if e+8 > len(raw) {
				break
			}
			plat := int(binary.BigEndian.Uint16(raw[e : e+2]))
			enc := int(binary.BigEndian.Uint16(raw[e+2 : e+4]))
			if plat == want.plat && enc == want.enc {
				best = e
				break
			}
		}
		if best >= 0 {
			break
		}
	}
	if best < 0 {
		return 0, fmt.Errorf("no unicode cmap subtable")
	}
	sub := cmapOff + int(binary.BigEndian.Uint32(raw[best+4 : best+8]))
	if sub+2 > len(raw) {
		return 0, fmt.Errorf("cmap subtable out of range")
	}
	format := binary.BigEndian.Uint16(raw[sub : sub+2])
	switch format {
	case 12:
		if sub+16 > len(raw) {
			return 0, fmt.Errorf("cmap12 out of range")
		}
		ng := int(binary.BigEndian.Uint32(raw[sub+12 : sub+16]))
		if sub+16+ng*12 > len(raw) {
			return 0, fmt.Errorf("cmap12 groups out of range")
		}
		cp := uint32(r)
		for g := 0; g < ng; g++ {
			o := sub + 16 + g*12
			start := binary.BigEndian.Uint32(raw[o : o+4])
			end := binary.BigEndian.Uint32(raw[o+4 : o+8])
			sg := binary.BigEndian.Uint32(raw[o+8 : o+12])
			if cp >= start && cp <= end {
				return uint16(sg + cp - start), nil
			}
		}
		return 0, nil
	case 4:
		if sub+16 > len(raw) {
			return 0, fmt.Errorf("cmap4 out of range")
		}
		segX2 := int(binary.BigEndian.Uint16(raw[sub+6 : sub+8]))
		segEnd := segX2 / 2
		if sub+16+3*segX2 > len(raw) {
			return 0, fmt.Errorf("cmap4 out of range")
		}
		cp := uint16(r)
		for i := 0; i < segEnd; i++ {
			end := binary.BigEndian.Uint16(raw[sub+14+i*2 : sub+16+i*2])
			if cp > end {
				continue
			}
			start := binary.BigEndian.Uint16(raw[sub+16+segX2+i*2 : sub+18+segX2+i*2])
			if cp < start {
				break
			}
			delta := int16(binary.BigEndian.Uint16(raw[sub+16+2*segX2+i*2 : sub+18+2*segX2+i*2]))
			idRO := int(binary.BigEndian.Uint16(raw[sub+16+3*segX2+i*2 : sub+18+3*segX2+i*2]))
			if idRO == 0 {
				return uint16(int32(cp) + int32(delta)), nil
			}
			addr := sub + 16 + 3*segX2 + i*2 + idRO + int(cp-start)*2
			if addr+2 > len(raw) {
				return 0, fmt.Errorf("cmap4 idRange out of range")
			}
			gid := binary.BigEndian.Uint16(raw[addr : addr+2])
			if gid == 0 {
				return 0, nil
			}
			return uint16(int32(gid) + int32(delta)), nil
		}
		return 0, nil
	case 0:
		if r >= 0 && r < 256 {
			return uint16(r), nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("cmap format %d unsupported", format)
}

type testingT interface {
	Helper()
	Skipf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// testFont 是 hint 包测试的自足字体（替代渲染层 ParsedFont）。
type testFont struct {
	raw  []byte
	face int
	unitsPerEm int
}

// openTestFont 读取字体并解析 upem + cmap（face 0，与渲染层默认一致）。
func openTestFont(t testingT, path string) *testFont {
	t.Helper()
	raw := fontFileRaw(t, path)
	upem, err := fontUpem(raw, 0)
	if err != nil {
		t.Fatalf("upem(%s): %v", path, err)
	}
	return &testFont{raw: raw, face: 0, unitsPerEm: upem}
}

// UnitsPerEm 返回字体 upem。
func (f *testFont) UnitsPerEm() int { return f.unitsPerEm }

// GlyphIndex 返回 rune 的 gid（缺失为 0）。
func (f *testFont) GlyphIndex(r rune) int {
	g, err := glyphIndexForRune(f.raw, f.face, r)
	if err != nil {
		return 0
	}
	return int(g)
}