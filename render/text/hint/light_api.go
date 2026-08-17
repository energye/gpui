package hint

import (
	"encoding/binary"
	"fmt"
)

// 本文件是 hint 引擎对外的唯一公开入口（渲染层接线层）：
//   - CFF1（pshinter light）：cff.go/cffcs.go/psh_light.go 已逐点对照
//     FT-light 全字集闭环（docs §5.1 M2/M3，3000 字 + 韩 + 泰）。
//   - CFF2（cf2 light）：cff2.go/cffcs.go(M5)/psh_light.go 对照
//     SourceSans3 VF 全字母（探针阶段）。
//
// 输出约定：
//   - 26.6 定点（int64），Y-up（字形像素空间，baseline=0，向上为正），
//     与 ftexp 的 contour 26.6 输出同构（逐点已对齐）。
//   - 点序 = charstring 解释序（含 off-curve 控制点），
//     与渲染层 go-text 提取的段并不按序相等，调用方需按 On 标志重建。

// LightPt 是 light 拟合后的一个轮廓点（26.6 定点，Y-up）。
type LightPt struct {
	X, Y int64 // 26.6 定点
	On   bool  // on-curve（true）或 off-curve（控制点）
}

// LightHint 对单个字形执行 CFF/CFF2 light 拟合（对齐 FT_LOAD_TARGET_LIGHT）。
//
// raw    ：字体的原始文件字节（可能是 TTC/OTC 容器）
// faceIdx ：容器内 face 索引（普通字体传 0）
// isCFF2 ：true 走 CFF2（可变字体），false 走 CFF1
// gid    ：字形索引
// px     ：ppem 字号（像素）
//
// 返回：
//   - pts：全部轮廓点（26.6 定点，Y-up），含 off-curve 控制点，
//     pts 点序与如下 contours 分组对应，如 FT outline。
//   - contours[0] 是第一个轮廓的点数，pts[:contours[0]] 为第一轮廓。
//   - width：glyph 的 advance（字体单位；无 width 参数时 = nominalWidthX 之后
//     的排布，与 FT cff_slot_load 语义一致，可忽略）。
func LightHint(raw []byte, faceIdx int, isCFF2 bool, gid uint16, px float64) (pts []LightPt, contours []int, width float64, err error) {
	return LightHintVar(raw, faceIdx, isCFF2, gid, px, nil)
}

// LightHintVar 同 LightHint，额外支持 CFF2 变体坐标（归一化，avar 已应用）。
// coords 为 nil 或空 = 默认实例。CFF1 忽略 coords。
func LightHintVar(raw []byte, faceIdx int, isCFF2 bool, gid uint16, px float64, coords []float32) (pts []LightPt, contours []int, width float64, err error) {
	if len(raw) == 0 {
		return nil, nil, 0, fmt.Errorf("hint: empty raw")
	}
	upem := hintFontUpem(raw, faceIdx)
	if upem <= 0 {
		upem = 1000
	}

	var cs *csOutline
	var fd *cffFD
	if isCFF2 {
		cd, err := cff2ParseCached(raw, faceIdx)
		if err != nil {
			return nil, nil, 0, err
		}
		fdIdx := cffFontIndex(cd.fdSelect, gid)
		if fdIdx < 0 || fdIdx >= len(cd.fds) {
			return nil, nil, 0, fmt.Errorf("hint: cff2 fd %d invalid", fdIdx)
		}
		fd = &cd.fds[fdIdx]
		if int(gid) >= len(cd.charStrings) {
			return nil, nil, 0, fmt.Errorf("hint: cff2 gid %d out of range", gid)
		}
		blend := &cff2BlendData{vstore: cd.vstore, coords: cff2VarCoords(cd, coords)}
		cs, err = interpretCharstring2(cd.charStrings[gid], fd.subrs, cd.globalSubrs, 0, 0, blend)
		if err != nil {
			return nil, nil, 0, err
		}
	} else {
		cd, err := cffParseCached(raw, faceIdx)
		if err != nil {
			return nil, nil, 0, err
		}
		fdIdx := cffFontIndex(cd.fdSelect, gid)
		if fdIdx < 0 || fdIdx >= len(cd.fds) {
			return nil, nil, 0, fmt.Errorf("hint: cff fd %d invalid", fdIdx)
		}
		fd = &cd.fds[fdIdx]
		if int(gid) >= len(cd.charStrings) {
			return nil, nil, 0, fmt.Errorf("hint: cff1 gid %d out of range", gid)
		}
		cs, err = interpretCharstring(cd.charStrings[gid], fd.subrs, cd.globalSubrs, fd.blues.nominalWidthX)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	scale := lightHintScale(px, upem)
	// 对照链（zz_scan_3000 / zz_m2_verify）用 darken=0 与 ftexp light 逐点
	// 对齐。FT light 下 cf2 的 darkening 默认也关闭（FT_DARKENING 不应用于
	// light 模式），故生产同样传 0 保持一致性。
	res := hintCFFLight(cs, fd, scale, 0, 0)
	if res == nil {
		return nil, nil, cs.width, nil
	}

	out := make([]LightPt, len(res.pts))
	for i, p := range res.pts {
		out[i] = LightPt{X: int64(p[0]) >> 10, Y: int64(p[1]) >> 10, On: res.on[i]}
	}
	return out, append([]int(nil), res.contours...), cs.width, nil
}

// cff2VarCoords 合并用户坐标与默认（全 0）坐标。
func cff2VarCoords(cd *cff2FontData, coords []float32) []cff2Coord {
	n := 0
	if cd != nil && cd.vstore != nil {
		n = cd.vstore.axisCount
	}
	if n <= 0 {
		return nil
	}
	out := make([]cff2Coord, 0, n)
	for i := 0; i < n; i++ {
		v := float64(0)
		if coords != nil && i < len(coords) {
			v = float64(coords[i])
		}
		out = append(out, cff2Coord{axis: i, value: v})
	}
	return out
}

// cffFontIndex 解析 fdSelect（缺省 = 0）。
func cffFontIndex(sel func(uint16) (int, error), gid uint16) int {
	if sel == nil {
		return 0
	}
	i, err := sel(gid)
	if err != nil {
		return -1
	}
	return i
}

// cff2DefaultCoords 返回默认实例的归一化变体坐标（全 0）。
// 与探针 TestProbeCFF2LightMatchesFT 对照线一致（FT 默认实例 = normalized 0）。
func cff2DefaultCoords(cd *cff2FontData) []cff2Coord {
	if cd == nil {
		return nil
	}
	n := cd.vstore.axisCount
	if n <= 0 {
		return nil
	}
	out := make([]cff2Coord, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, cff2Coord{axis: i, value: 0})
	}
	return out
}

// lightHintScale：hinted 26.6 scale = (x_scale + 32) / 64（psft.c:279），
// 与测试路径 m2HintScale 相同（生产文件不能引用测试 helper，故内联）。
func lightHintScale(px float64, upem int) cf2Fixed {
	xScale := lightFTScale(px, int64(upem))
	return cf2Fixed((int64(xScale) + 32) / 64)
}

// lightFTScale：FT_DivFix(charWidth26_6, upem) —— 字身 16.16 比例。
func lightFTScale(px float64, upem int64) int64 {
	cw := int64(px*64 + 0.5)
	return lightDivFix(cw, upem)
}

// lightDivFix：FT_DivFix(a,b) = (a<<16 + b/2) / b。
func lightDivFix(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	q := (a<<16 + b/2) / b
	return q
}

// fontUpem 读取字体 head 表 unitsPerEm（TTC 容器取 faceIdx 的 offset）。
func hintFontUpem(raw []byte, faceIdx int) int {
	base, ok := sfntFontBase(raw, faceIdx)
	if !ok {
		return 0
	}
	headOff, ln, err := findTable(raw, base, "head")
	if err != nil || ln < 18 {
		return 0
	}
	return int(binary.BigEndian.Uint16(raw[headOff+18 : headOff+20]))
}

// sfntFontBase 返回 sfnt 容器中 faceIdx 的字体起点（非 TTC 为 0）。
func sfntFontBase(raw []byte, faceIdx int) (int, bool) {
	if len(raw) < 4 {
		return 0, false
	}
	switch string(raw[0:4]) {
	case "ttcf":
		if len(raw) < 12 {
			return 0, false
		}
		numFonts := int(binary.BigEndian.Uint32(raw[8:12]))
		if faceIdx < 0 || faceIdx >= numFonts {
			return 0, false
		}
		if 12+faceIdx*4+4 > len(raw) {
			return 0, false
		}
		base := int(binary.BigEndian.Uint32(raw[12+faceIdx*4 : 16+faceIdx*4]))
		if base+4 > len(raw) {
			return 0, false
		}
		return base, true
	case "OTTO", "true", "\x00\x01\x00\x00":
		return 0, true
	}
	return 0, false
}