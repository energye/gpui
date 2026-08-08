package hint

// CFF2 可变字体解析（M5）：cf2 语义融合数据源。
//
// CFF2 表布局与 CFF1 相同（header / NAME INDEX / TOP DICT INDEX /
// STRING INDEX / GLOBAL SUBRS INDEX），但：
//   - TOP DICT 有 vstore（24）指向 VariationStore。
//   - Private DICT 有 vsindex（22），无 defaultWidthX/nominalWidthX。
//   - charstring 无 width 参数，支持 vsindex(15)/blend(16) 指令。
//
// 本文件自解析 CFF2 表（复用 cff.go 的 INDEX/DICT/FDSelect 工具），
// 产出 cff2FontData：CharStrings、每 FD 的 Subrs 与蓝区（cf2 需要）、
// 以及变体数据（VariationStore + 坐标 → blend 标量）。

import (
	"encoding/binary"
	"fmt"
)

// cff2FontData 是 CFF2 表的完整解析（M5 charstring 解释器的数据源）。
type cff2FontData struct {
	charStrings [][]byte                      // 每 gid 一条 charstring
	globalSubrs [][]byte                      // Global Subrs INDEX
	fds         []cffFD                       // FDArray 每 FD（subrs + 蓝区）
	fdSelect    func(gid uint16) (int, error) // 非 CID 时为 nil
	vstore      *cff2VarStore                 // VariationStore（可为 nil）
	coords      []cff2Coord                   // 归一化变体坐标（wght 等）
	unitsPerEm  float64
}

// cff2Coord 是一个变体轴的归一化坐标（-1..1）。
type cff2Coord struct {
	axis  int
	value float64
}

// cff2VarStore 是 OpenType ItemVariationStore（cff2 用子集）。
// 结构：regionList(轴数+region 表) + itemVariationData 数组。
type cff2VarStore struct {
	axisCount    int
	regions      []cff2Region
	variationData []cff2VarData
}

// cff2Region 是一个 variation region（各轴的 start/peak/end）。
type cff2Region struct {
	starts, peaks, ends []float64 // 每轴
}

// cff2VarData 是一个 ItemVariationData 子表（region 下标 + delta 数据）。
type cff2VarData struct {
	regionIndexes []int
	deltas        []byte // 原始字节，长度 = rowCount*wordDeltaCount*2
	rowCount      int
	shortDelta    bool // 0 = Int16 delta，1 = Int8 delta
}

// cff2BlendData 是 blend 指令的运行数据（坐标 → 当前标量）。
type cff2BlendData struct {
	vstore  *cff2VarStore
	coords  []cff2Coord
	scalars []float64 // 当前 vsIndex 下各 region 的标量
}

// setScalars 计算当前 vsIndex（vsindex 指令）下的 region 标量。
// 语义同 go-text cff2CharstringHandler.setVSIndex + FT blend_build_vector。
func (b *cff2BlendData) setScalars(vsIndex int, coords []cff2Coord) {
	if b.vstore == nil || b.vstore.axisCount == 0 {
		b.scalars = nil
		return
	}
	if vsIndex < 0 || vsIndex >= len(b.vstore.variationData) {
		b.scalars = nil
		return
	}
	vd := b.vstore.variationData[vsIndex]
	b.scalars = make([]float64, len(vd.regionIndexes))
	for i, ri := range vd.regionIndexes {
		if ri < 0 || ri >= len(b.vstore.regions) {
			b.scalars[i] = 0
			continue
		}
		b.scalars[i] = b.vstore.regions[ri].evaluate(coords)
	}
}

// evaluate 计算该 region 在给定坐标下的标量（区间外 0，区间内插值，
// 负区间取反——OpenType spec「Region scalar」）。
func (r cff2Region) evaluate(coords []cff2Coord) float64 {
	var s float64 = 1
	for _, c := range coords {
		if c.axis >= len(r.starts) {
			continue
		}
		start, peak, end := r.starts[c.axis], r.peaks[c.axis], r.ends[c.axis]
		v := c.value
		if start == peak && peak == end {
			continue // 该轴无效果
		}
		var t float64
		if start > peak || (start == peak && end != peak) {
			// 负区间：v <= peak 或 v >= end 时 0
			if v > peak && v < end {
				t = (end - v) / (end - peak)
			} else if v == peak {
				t = 1
			}
		} else {
			// 正区间
			if v >= start && v < peak {
				t = (v - start) / (peak - start)
			} else if v == peak {
				t = 1
			} else if v > peak && v <= end {
				t = (end - v) / (end - peak)
			}
		}
		s *= t
	}
	return s
}

// cff2ReadIndex 解析 CFF2 的 INDEX：count 为 4 字节 uint32（CFF1 是 2 字节
// uint16），header size=5（count 4 + offSize 1）。
func cff2ReadIndex(cff []byte, p int) ([][]byte, int, error) {
	if p+5 > len(cff) {
		return nil, 0, fmt.Errorf("cff2 index header out of range")
	}
	count := int(binary.BigEndian.Uint32(cff[p:]))
	p += 4
	if count == 0 {
		return nil, p, nil
	}
	offSize := int(cff[p])
	p++
	if offSize < 1 || offSize > 4 {
		return nil, 0, fmt.Errorf("cff2 index bad offSize %d", offSize)
	}
	offs := make([]int, count+1)
	for i := 0; i <= count; i++ {
		v := 0
		for j := 0; j < offSize; j++ {
			v = v<<8 | int(cff[p+j])
		}
		offs[i] = v
		p += offSize
	}
	if offs[0] != 1 {
		return nil, 0, fmt.Errorf("cff2 index first offset %d", offs[0])
	}
	dataStart := p
	if dataStart+offs[count]-1 > len(cff) {
		return nil, 0, fmt.Errorf("cff2 index data out of range")
	}
	items := make([][]byte, count)
	for i := 0; i < count; i++ {
		a := dataStart + offs[i] - 1
		b := dataStart + offs[i+1] - 1
		items[i] = cff[a:b]
	}
	return items, dataStart + offs[count] - 1, nil
}

// cff2SkipIndex 跳过 CFF2 的 INDEX（counter 4 字节），同 cff2ReadIndex 语义。
func cff2SkipIndex(cff []byte, p int) (int, error) {
	_, next, err := cff2ReadIndex(cff, p)
	return next, err
}

// cff2TableData 定位字体字节中 CFF2 表的绝对偏移与长度（同 cffTableData 但找 "CFF2"）。
func cff2TableData(raw []byte, faceIndex int) (int, int, error) {
	if len(raw) < 4 {
		return 0, 0, fmt.Errorf("font data too short")
	}
	var fontBase int
	tag := string(raw[0:4])
	switch tag {
	case "ttcf":
		if len(raw) < 12 {
			return 0, 0, fmt.Errorf("ttc too short")
		}
		numFonts := int(binary.BigEndian.Uint32(raw[8:12]))
		if faceIndex >= numFonts {
			return 0, 0, fmt.Errorf("ttc face %d out of range", faceIndex)
		}
		if 12+faceIndex*4+4 > len(raw) {
			return 0, 0, fmt.Errorf("ttc offset out of range")
		}
		fontBase = int(binary.BigEndian.Uint32(raw[12+faceIndex*4 : 16+faceIndex*4]))
		if fontBase+4 > len(raw) {
			return 0, 0, fmt.Errorf("ttc font base out of range")
		}
		tag = string(raw[fontBase : fontBase+4])
	case "OTTO", "true", "\x00\x01\x00\x00":
		fontBase = 0
	default:
		return 0, 0, fmt.Errorf("unknown sfnt tag %q", tag)
	}
	switch tag {
	case "OTTO", "true", "\x00\x01\x00\x00":
		return findTable(raw, fontBase, "CFF2")
	default:
		return 0, 0, fmt.Errorf("not a CFF2 font (tag %q)", tag)
	}
}

// cff2ParseAll 解析 CFF2 表全部需要的数据。
//
// CFF2 表结构（CFF2 spec / 与 CFF1 不同）：
//
//	Header: major(1) minor(1) hdrSize(1) topDictLength(2)
//	Top DICT（单个，长度 = topDictLength）
//	Global Subrs INDEX
//	CharStrings INDEX / FDArray INDEX / FDSelect / VariationStore
func cff2ParseAll(cff []byte, unitsPerEm int) (*cff2FontData, error) {
	if len(cff) < 5 {
		return nil, fmt.Errorf("cff2 too short")
	}
	hdrSize := int(cff[2])
	topDictLen := int(binary.BigEndian.Uint16(cff[3:5]))
	if hdrSize < 5 || hdrSize > len(cff) {
		return nil, fmt.Errorf("bad cff2 header size")
	}
	if hdrSize+topDictLen > len(cff) {
		return nil, fmt.Errorf("cff2 top dict out of range")
	}
	topDict := cff[hdrSize : hdrSize+topDictLen]
	p := hdrSize + topDictLen

	// Global Subrs INDEX（紧跟 TOP DICT 之后）
	gsubrs, _, err := cff2ReadIndex(cff, p)
	if err != nil {
		return nil, err
	}

	topOps := dictOps(topDict)
	csOff, ok := dictOpOffset(topOps, 17)
	if !ok {
		return nil, fmt.Errorf("cff2: no CharStrings")
	}
	charStrings, _, err := cff2ReadIndex(cff, csOff)
	if err != nil {
		return nil, fmt.Errorf("cff2: charstrings: %w", err)
	}

	out := &cff2FontData{
		charStrings: charStrings,
		globalSubrs: gsubrs,
		unitsPerEm:  float64(unitsPerEm),
	}

	// vstore（24）
	if vstoreOff, ok := dictOpOffset(topOps, 24); ok {
		vs, err := parseCFF2VarStore(cff, vstoreOff)
		if err != nil {
			return nil, err
		}
		out.vstore = vs
	}

	// FDArray（escape 12 36 = 1236）+ FDSelect（escape 12 37 = 1237）
	fdArrayOff, ok2 := dictOpOffset(topOps, 1236)
	if !ok2 {
		return nil, fmt.Errorf("cff2: no FDArray")
	}
	fdDicts, _, err := cff2ReadIndex(cff, fdArrayOff)
	if err != nil {
		return nil, err
	}

	for _, fdDict := range fdDicts {
		privSize, privOff, ok := dictPrivate(dictOps(fdDict))
		if !ok {
			return nil, fmt.Errorf("cff2: fd has no private")
		}
		fd, err := parseCFF2FD(cff[privOff:privOff+privSize], privOff, cff, unitsPerEm)
		if err != nil {
			return nil, err
		}
		out.fds = append(out.fds, *fd)
	}
	if fdSelectOff, ok := dictOpOffset(topOps, 1237); ok {
		out.fdSelect = func(gid uint16) (int, error) {
			return readFDSelect(cff, fdSelectOff, gid)
		}
	}
	return out, nil
}

// parseCFF2FD 解析单个 FD 的 Private（蓝区 + Subrs + vsindex）。
// Subrs 偏移相对 Private DICT 起点（与 CFF1 相同语义）。
func parseCFF2FD(priv []byte, privOff int, cff []byte, unitsPerEm int) (*cffFD, error) {
	ops := dictOps(priv)
	fd := &cffFD{}
	for _, op := range ops {
		if op.op == 19 && len(op.operands) >= 1 {
			subrsOff := int(op.operands[len(op.operands)-1])
			items, _, err := cff2ReadIndex(cff, privOff+subrsOff)
			if err != nil {
				return nil, fmt.Errorf("cff2: fd subrs: %w", err)
			}
			fd.subrs = items
		}
	}
	b, err := parsePrivateDict(priv, unitsPerEm)
	if err != nil {
		return nil, err
	}
	fd.blues = b
	fd.languageGroup = b.languageGroup
	return fd, nil
}

// parseCFF2VarStore 解析 ItemVariationStore。
//
// CFF2 TOP DICT vstore(24) 偏移指向一个表：
//
//	uint16 length                      // Size of the VariationStore table
//	ItemVariationStore table           // OpenType 结构
//
// ItemVariationStore（offset 均相对 ItemVariationStore 起点）：
//
//	uint16 format                      // = 1
//	uint32 variationRegionListOffset
//	uint16 itemVariationDataCount
//	uint32 itemVariationDataOffsets[count]
func parseCFF2VarStore(cff []byte, off int) (*cff2VarStore, error) {
	if off+2 > len(cff) {
		return nil, fmt.Errorf("cff2: vstore out of range")
	}
	size := int(binary.BigEndian.Uint16(cff[off:]))
	p := off + 2
	end := p + size
	if end > len(cff) {
		return nil, fmt.Errorf("cff2: vstore length out of range")
	}
	if p+8 > end {
		return nil, fmt.Errorf("cff2: vstore header truncated")
	}
	format := binary.BigEndian.Uint16(cff[p:])
	if format != 1 {
		return nil, fmt.Errorf("cff2: vstore format %d", format)
	}
	regionListOff := int(binary.BigEndian.Uint32(cff[p+2:]))
	vdCount := int(binary.BigEndian.Uint16(cff[p+6:]))
	p += 8

	// VariationRegionList（相对 ItemVariationStore 起点）
	// axisCount(2) regionCount(2) region[axisCount*6 each]
	rl := (off + 2) + regionListOff
	if rl+4 > end {
		return nil, fmt.Errorf("cff2: regionList header truncated")
	}
	axisCount := int(binary.BigEndian.Uint16(cff[rl:]))
	regionCount := int(binary.BigEndian.Uint16(cff[rl+2:]))
	vs := &cff2VarStore{axisCount: axisCount}
	rp := rl + 4
	vs.regions = make([]cff2Region, 0, regionCount)
	for i := 0; i < regionCount; i++ {
		if rp+axisCount*6 > end {
			return nil, fmt.Errorf("cff2: region %d truncated", i)
		}
		r := cff2Region{
			starts: make([]float64, axisCount),
			peaks:  make([]float64, axisCount),
			ends:   make([]float64, axisCount),
		}
		for a := 0; a < axisCount; a++ {
			r.starts[a] = float64(int16(binary.BigEndian.Uint16(cff[rp:]))) / 16384
			rp += 2
		}
		for a := 0; a < axisCount; a++ {
			r.peaks[a] = float64(int16(binary.BigEndian.Uint16(cff[rp:]))) / 16384
			rp += 2
		}
		for a := 0; a < axisCount; a++ {
			r.ends[a] = float64(int16(binary.BigEndian.Uint16(cff[rp:]))) / 16384
			rp += 2
		}
		vs.regions = append(vs.regions, r)
	}

	// ItemVariationData 数组（偏移相对 ItemVariationStore 起点）
	vs.variationData = make([]cff2VarData, 0, vdCount)
	for i := 0; i < vdCount; i++ {
		if p+4*(i+1) > end {
			return nil, fmt.Errorf("cff2: varData offset %d truncated", i)
		}
		vdOff := int(binary.BigEndian.Uint32(cff[p+i*4:]))
		if vdOff == 0 {
			continue // 空 offset（可选 VarData）
		}
		vd, err := parseCFF2VarData(cff, (off+2)+vdOff, end)
		if err != nil {
			return nil, err
		}
		vs.variationData = append(vs.variationData, *vd)
	}
	return vs, nil
}

// parseCFF2VarData 解析一个 ItemVariationData 子表。
//
//	uint16 itemCount
//	uint16 wordDeltaCount          // 每行前 N 个 delta 用 int16，其余 int8
//	uint16 regionIndexCount
//	uint16 regionIndexes[count]
//	deltaSets[itemCount]           // 每行 regionIndexCount 个 delta
func parseCFF2VarData(cff []byte, p, end int) (*cff2VarData, error) {
	if p+6 > end {
		return nil, fmt.Errorf("cff2: varData header truncated")
	}
	itemCount := int(binary.BigEndian.Uint16(cff[p:]))
	wordDeltaCount := int(binary.BigEndian.Uint16(cff[p+2:]))
	regionIndexCount := int(binary.BigEndian.Uint16(cff[p+4:]))
	p += 6
	vd := &cff2VarData{
		regionIndexes: make([]int, regionIndexCount),
		rowCount:      itemCount,
		shortDelta:    wordDeltaCount == regionIndexCount,
	}
	for j := 0; j < regionIndexCount; j++ {
		if p+2 > end {
			return nil, fmt.Errorf("cff2: varData regionIndex truncated")
		}
		vd.regionIndexes[j] = int(binary.BigEndian.Uint16(cff[p:]))
		p += 2
	}
	rowBytes := wordDeltaCount*2 + (regionIndexCount-wordDeltaCount)*1
	if p+rowBytes*itemCount > end {
		return nil, fmt.Errorf("cff2: varData deltas truncated")
	}
	vd.deltas = cff[p : p+rowBytes*itemCount]
	return vd, nil
}
