package hint

// CFF 蓝区解析（pshinter light 移植的数据来源）。
//
// Noto Sans CJK 等系统 CJK 字体为 CFF 轮廓（OpenType OTTO），FreeType 的
// FT_LOAD_TARGET_LIGHT 对 CFF 字体走 pshinter 的 light 模式，其蓝区来自
// CFF Private DICT 的 BlueValues / OtherBlues / BlueFuzz / BlueScale /
// BlueShift / StdHW / StdVW（见 freetype-2.14.3 src/pshinter/pshglob.c）。
// go-text 解析器会丢弃这些 operator，因此此处从原始字体字节自解析。

import (
	"encoding/binary"
	"fmt"
)

// cffBlues 是 CFF Private DICT 的蓝区相关参数（字体单位）。
type cffBlues struct {
	blueValues    []float64 // op 6：成对的 [bottom,top] 区
	otherBlues    []float64 // op 7：下沉区（descender 方向）
	familyBlues   []float64 // op 8：族蓝区（跨字号对齐，一般 CJK 无）
	familyOtherBlues []float64 // op 9
	blueScale     float64   // 12/9
	blueShift     float64   // 12/10
	blueFuzz      float64   // 12/11
	stdHW         float64   // op 10（横笔标准宽）
	stdVW         float64   // op 11（竖笔标准宽）
	stemSnapH     []float64 // 12/12
	stemSnapV     []float64 // 12/13
	defaultWidthX float64   // op 20
	nominalWidthX float64   // op 21
	languageGroup int       // 12/17（0=Latin，1=CJK；1 且 BlueValues 为 dummy → cf2 emBox 幽灵区）
	unitsPerEm    float64
}

// cffTableData 定位字体字节中 CFF 表的绝对偏移与长度。
// 支持 TTF/OTF（sfnt）与 TTC。TTC 的 table directory offset 相对文件头
// （即绝对偏移），因此子字体的表定位直接在完整字节上按绝对位置进行。
func cffTableData(raw []byte, faceIndex int) (int, int, error) {
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
		return findTable(raw, fontBase, "CFF ")
	default:
		return 0, 0, fmt.Errorf("not a CFF font (tag %q)", tag)
	}
}

// findTable 在 fontBase 处的 sfnt 表目录里找 tag。
// 返回的表偏移为文件绝对偏移（TTC 语义：目录内偏移相对文件头）。
func findTable(raw []byte, fontBase int, tag string) (int, int, error) {
	if fontBase+6 > len(raw) {
		return 0, 0, fmt.Errorf("font base out of range")
	}
	numTables := int(binary.BigEndian.Uint16(raw[fontBase+4 : fontBase+6]))
	for i := 0; i < numTables; i++ {
		e := fontBase + 12 + i*16
		if e+16 > len(raw) {
			break
		}
		if string(raw[e:e+4]) == tag {
			tblOff := int(binary.BigEndian.Uint32(raw[e+8 : e+12]))
			tblLen := int(binary.BigEndian.Uint32(raw[e+12 : e+16]))
			if tblOff+tblLen > len(raw) {
				return 0, 0, fmt.Errorf("%s table out of range", tag)
			}
			return tblOff, tblLen, nil
		}
	}
	return 0, 0, fmt.Errorf("no %s table", tag)
}

// parseCFFBlues 解析 CFF 表的蓝区参数。
//
// Noto CJK 等系统 CFF 字体为 CID-keyed：TOP DICT 不含 Private；
// 蓝区位于 FD（Font DICT）的 Private DICT，每个 glyph 由 FDSelect 选 FD。
// 因此：
//  1. 解析 TOP DICT，取 CharStrings / FDArray / FDSelect 的偏移。
//  2. 用 FDSelect 选 gid 所属 FD。
//  3. 解析该 FD 的 Private DICT。
//
// 若字号字体非 CID（普通 CFF），TOP DICT 直接有 Priva
// 若字号字体非 CID（普通 CFF），TOP DICT 直接有 Private。
func parseCFFBlues(cff []byte, unitsPerEm int, gid uint16) (*cffBlues, error) {
	if len(cff) < 4 {
		return nil, fmt.Errorf("cff too short")
	}
	hdrSize := int(cff[2])
	if hdrSize < 4 || hdrSize > len(cff) {
		return nil, fmt.Errorf("bad cff header size")
	}
	p := hdrSize

	// NAME INDEX：跳过
	p, err := skipIndex(cff, p)
	if err != nil {
		return nil, err
	}
	// TOP DICT INDEX
	topDicts, next, err := readIndex(cff, p)
	if err != nil {
		return nil, err
	}
	if len(topDicts) == 0 {
		return nil, fmt.Errorf("no top dict")
	}
	p = next
	// STRING INDEX：跳过
	p, err = skipIndex(cff, p)
	if err != nil {
		return nil, err
	}
	// GLOBAL SUBRS INDEX：跳过
	_, _, err = readIndex(cff, p)
	if err != nil {
		return nil, err
	}

	topOps := dictOps(topDicts[0])

	// 非 CID：TOP DICT 直接有 Private [size, offset]
	if privSize, privOff, ok := dictPrivate(topOps); ok {
		return parsePrivateDict(cff[privOff:privOff+privSize], unitsPerEm)
	}

	// CID：取 FDSelect 与 FDArray
	fdSelectOff, ok1 := dictFDSelect(topOps)
	fdArrayOff, ok2 := dictFDArray(topOps)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("cff: no private and no cid fdarray")
	}
	fdIdx, err := readFDSelect(cff, fdSelectOff, gid)
	if err != nil {
		return nil, err
	}
	fdDicts, err := readIndexBytes(cff, fdArrayOff)
	if err != nil {
		return nil, err
	}
	if fdIdx >= len(fdDicts) {
		return nil, fmt.Errorf("cff: fd %d out of range", fdIdx)
	}
	fdOps := dictOps(fdDicts[fdIdx])
	privSize, privOff, ok := dictPrivate(fdOps)
	if !ok {
		return nil, fmt.Errorf("cff: fd %d has no private", fdIdx)
	}
	return parsePrivateDict(cff[privOff:privOff+privSize], unitsPerEm)
}

// dictPrivate 从 DICT operator 列表取 Private [size, offset]。
func dictPrivate(ops []dictOp) (size, offset int, ok bool) {
	if len(ops) == 0 {
		return 0, 0, false
	}
	// DICT 操作符的 operand 逆序入栈：ops 里最后一个 = 第一个 operand。
	// 这里保存原始 operand 顺序，取 operands[0]=size, operands[1]=offset
	// （CFF 规定 Private 的 operands 顺序为 [size, offset]）。
	// 注意：Private 不一定在 DICT 末尾（Noto Sans Thai 顺序为
	// ..., Private, CharStrings），须全表查找而非只查最后一项。
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].op == 18 && len(ops[i].operands) >= 2 {
			return int(ops[i].operands[0]), int(ops[i].operands[1]), true
		}
	}
	return 0, 0, false
}

// dictFDSelect 取 Top DICT 12/37 的 FDSelect 偏移。
func dictFDSelect(ops []dictOp) (int, bool) {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].op == 1237 && len(ops[i].operands) > 0 {
			return int(ops[i].operands[len(ops[i].operands)-1]), true
		}
	}
	return 0, false
}

// dictFDArray 取 Top DICT 12/36 的 FDArray 偏移。
func dictFDArray(ops []dictOp) (int, bool) {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].op == 1236 && len(ops[i].operands) > 0 {
			return int(ops[i].operands[len(ops[i].operands)-1]), true
		}
	}
	return 0, false
}

// readFDSelect 解析 CID FDSelect，返回 gid 的 FD 下标。
func readFDSelect(cff []byte, off int, gid uint16) (int, error) {
	if off >= len(cff) {
		return 0, fmt.Errorf("fdselect out of range")
	}
	format := cff[off]
	switch format {
	case 0:
		pos := off + 1 + int(gid)
		if pos >= len(cff) {
			return 0, fmt.Errorf("fdselect f0 out of range")
		}
		return int(cff[pos]), nil
	case 3:
		pos := off + 1
		if pos+2 > len(cff) {
			return 0, fmt.Errorf("fdselect f3 out of range")
		}
		nRanges := int(binary.BigEndian.Uint16(cff[pos : pos+2]))
		pos += 2
		for i := 0; i < nRanges; i++ {
			if pos+3 > len(cff) {
				return 0, fmt.Errorf("fdselect f3 range out of range")
			}
			first := binary.BigEndian.Uint16(cff[pos : pos+2])
			fd := int(cff[pos+2])
			pos += 3
			if i+1 < nRanges {
				if pos+2 > len(cff) {
					return 0, fmt.Errorf("fdselect f3 next out of range")
				}
				next := binary.BigEndian.Uint16(cff[pos : pos+2])
				if gid >= first && gid < next {
					return fd, nil
				}
			} else if gid >= first {
				return fd, nil
			}
		}
		return 0, fmt.Errorf("fdselect: gid %d not found", gid)
	default:
		return 0, fmt.Errorf("unknown fdselect format %d", format)
	}
}

// readIndexBytes 读取 CFF INDEX 的原始条目字节（不解析内容）。
func readIndexBytes(data []byte, p int) ([][]byte, error) {
	items, _, err := readIndex(data, p)
	return items, err
}

// skipIndex 跳过 CFF INDEX 结构，返回下一个数据起始位置。
// CFF INDEX 的 offset 是 1-based，相对「偏移数组之后」的 data 区起点。
func skipIndex(data []byte, p int) (int, error) {
	if p+2 > len(data) {
		return 0, fmt.Errorf("index header out of range")
	}
	count := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if count == 0 {
		return p, nil
	}
	if p >= len(data) {
		return 0, fmt.Errorf("index offsize out of range")
	}
	offSize := int(data[p])
	p++
	if offSize < 1 || offSize > 4 {
		return 0, fmt.Errorf("bad offSize %d", offSize)
	}
	// 跳过 (count+1) 个偏移；p 现在 = 偏移数组之后 = data 区起点
	offsEnd := p + (count+1)*offSize
	if offsEnd > len(data) {
		return 0, fmt.Errorf("index offsets out of range")
	}
	// 最后一个偏移（1-based）→ data 区长度 = offs[count]-1
	last := indexOff(data, offsEnd-offSize, offSize)
	if last < 1 {
		return 0, fmt.Errorf("bad index offset")
	}
	next := offsEnd + last - 1
	if next > len(data) {
		return 0, fmt.Errorf("index data out of range")
	}
	return next, nil
}

// readIndex 读取 CFF INDEX，返回各条目字节与下一个数据位置（语义同 skipIndex）。
func readIndex(data []byte, p int) ([][]byte, int, error) {
	if p+2 > len(data) {
		return nil, 0, fmt.Errorf("index header out of range")
	}
	count := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if count == 0 {
		return nil, p, nil
	}
	if p >= len(data) {
		return nil, 0, fmt.Errorf("index offsize out of range")
	}
	offSize := int(data[p])
	p++
	if offSize < 1 || offSize > 4 {
		return nil, 0, fmt.Errorf("bad offSize %d", offSize)
	}
	offs := make([]int, count+1)
	for i := 0; i <= count; i++ {
		offs[i] = indexOff(data, p, offSize)
		p += offSize
	}
	if offs[0] != 1 {
		return nil, 0, fmt.Errorf("bad index first offset %d", offs[0])
	}
	dataStart := p // 偏移数组之后 = data 区起点
	if dataStart+offs[count]-1 > len(data) {
		return nil, 0, fmt.Errorf("index data out of range")
	}
	items := make([][]byte, count)
	for i := 0; i < count; i++ {
		a := dataStart + offs[i] - 1
		b := dataStart + offs[i+1] - 1
		items[i] = data[a:b]
	}
	return items, dataStart + offs[count] - 1, nil
}

func indexOff(data []byte, p, offSize int) int {
	switch offSize {
	case 1:
		return int(data[p])
	case 2:
		return int(binary.BigEndian.Uint16(data[p : p+2]))
	case 3:
		return int(data[p])<<16 | int(data[p+1])<<8 | int(data[p+2])
	default:
		return int(binary.BigEndian.Uint32(data[p : p+4]))
	}
}

// topDictPrivate 已由 dictPrivate（operand 顺序 [size, offset]）取代。

type dictOp struct {
	op       int
	operands []float64
}

// dictOps 解析 CFF DICT 数据为 operator 列表（含 operand 栈）。
func dictOps(dict []byte) []dictOp {
	var ops []dictOp
	var stack []float64
	i := 0
	for i < len(dict) {
		b := dict[i]
		switch {
		case b >= 32 && b <= 246: // 1 字节正数
			stack = append(stack, float64(int(b)-139))
			i++
		case b >= 247 && b <= 250: // 2 字节正数
			if i+1 >= len(dict) {
				return ops
			}
			v := (int(b)-247)*256 + int(dict[i+1]) + 108
			stack = append(stack, float64(v))
			i += 2
		case b >= 251 && b <= 254: // 2 字节负数
			if i+1 >= len(dict) {
				return ops
			}
			v := -(int(b)-251)*256 - int(dict[i+1]) - 108
			stack = append(stack, float64(v))
			i += 2
		case b == 28: // 短整数
			if i+2 >= len(dict) {
				return ops
			}
			stack = append(stack, float64(int16(binary.BigEndian.Uint16(dict[i+1:i+3]))))
			i += 3
		case b == 29: // 长整数
			if i+4 >= len(dict) {
				return ops
			}
			stack = append(stack, float64(int32(binary.BigEndian.Uint32(dict[i+1:i+5]))))
			i += 5
		case b == 30: // 实数
			v, n := cffReal(dict[i+1:])
			stack = append(stack, v)
			i += 1 + n
		case b == 12: // 转义 operator
			if i+1 < len(dict) {
				op := 1200 + int(dict[i+1])
				ops = append(ops, dictOp{op: op, operands: stack})
				stack = nil
				i += 2
			} else {
				i++
			}
		case b <= 27 || b == 31: // operator
			ops = append(ops, dictOp{op: int(b), operands: stack})
			stack = nil
			i++
		default:
			return ops
		}
	}
	return ops
}

// cffReal 解析 CFF 实数编码（op 30）。
func cffReal(data []byte) (float64, int) {
	neg := false
	frac := false
	fracDiv := 1.0
	var val float64
	exponent := 0
	expNeg := false
	expFrac := false
	n := 0
	for _, b := range data {
		n++
		hi, lo := b>>4, b&0x0F
		for _, nib := range []byte{hi, lo} {
			switch {
			case nib <= 9:
				if frac {
					fracDiv /= 10
					val += float64(nib) * fracDiv
				} else {
					val = val*10 + float64(nib)
				}
			case nib == 10:
				frac = true
			case nib == 11:
				expFrac = true
			case nib == 12:
				neg = true
			case nib == 13:
				expNeg = true
			case nib == 14:
				exponent = exponent*10 + 1
			case nib == 15:
				e := exponent
				if expNeg {
					e = -e
				}
				if neg {
					val = -val
				}
				return val * pow10(e), n
			}
			_ = expFrac
		}
		if lo == 15 {
			break
		}
	}
	return val, n
}

func pow10(e int) float64 {
	v := 1.0
	if e >= 0 {
		for i := 0; i < e; i++ {
			v *= 10
		}
	} else {
		for i := 0; i < -e; i++ {
			v /= 10
		}
	}
	return v
}

// parsePrivateDict 解析 Private DICT 的蓝区 operator。
// 注意：BlueValues/OtherBlues/FamilyBlues/FamilyOtherBlues/StemSnapH/V 是
// DELTA 字段（FT cff_kind_delta_fixed：逐项累积求和，cffparse.c:1458）。
func parsePrivateDict(dict []byte, unitsPerEm int) (*cffBlues, error) {
	ops := dictOps(dict)
	b := &cffBlues{unitsPerEm: float64(unitsPerEm)}
	// 默认值（CFF 规范 / FT cffload.c 默认）
	b.blueScale = 0.039625
	b.blueShift = 7
	b.blueFuzz = 1
	accum := func(ops []float64) []float64 {
		out := make([]float64, len(ops))
		var acc float64
		for i, v := range ops {
			acc += v
			out[i] = acc
		}
		return out
	}
	for _, op := range ops {
		switch op.op {
		case 6: // BlueValues（delta）
			b.blueValues = accum(op.operands)
		case 7: // OtherBlues（delta）
			b.otherBlues = accum(op.operands)
		case 8: // FamilyBlues（delta，CJK 一般无）
			b.familyBlues = accum(op.operands)
		case 9: // FamilyOtherBlues（delta，CJK 一般无）
			b.familyOtherBlues = accum(op.operands)
		case 10: // StdHW
			if len(op.operands) > 0 {
				b.stdHW = op.operands[0]
			}
		case 11: // StdVW
			if len(op.operands) > 0 {
				b.stdVW = op.operands[0]
			}
		case 1209: // BlueScale
			if len(op.operands) > 0 {
				b.blueScale = op.operands[0]
			}
		case 1210: // BlueShift
			if len(op.operands) > 0 {
				b.blueShift = op.operands[0]
			}
		case 1211: // BlueFuzz
			if len(op.operands) > 0 {
				b.blueFuzz = op.operands[0]
			}
		case 1212: // StemSnapH（delta）
			b.stemSnapH = accum(op.operands)
		case 1213: // StemSnapV（delta）
			b.stemSnapV = accum(op.operands)
		case 20: // defaultWidthX
			if len(op.operands) > 0 {
				b.defaultWidthX = op.operands[0]
			}
		case 21: // nominalWidthX
			if len(op.operands) > 0 {
				b.nominalWidthX = op.operands[0]
			}
		case 1217: // LanguageGroup（默认 0；CJK 字体 =1）
			if len(op.operands) > 0 {
				b.languageGroup = int(op.operands[0])
			}
		}
	}
	// BlueValues 可缺省（cf2 幽灵区场景不需要真实蓝区；M2 用）
	return b, nil
}

// cffFD 是 CID 字体中一个 FD 的 Private 数据（蓝区 + Subrs + LanguageGroup）。
type cffFD struct {
	blues         *cffBlues
	subrs         [][]byte // Private DICT op 19 指向的 Subrs INDEX 条目
	languageGroup int
}

// cffFontData 是 CFF 表的完整解析（M1 charstring 解释器的数据源）。
// 一次解析得到：CharStrings INDEX、GlobalSubrs、每 FD 的 Subrs 与蓝区。
type cffFontData struct {
	charStrings [][]byte                      // 每 gid 一条 charstring
	globalSubrs [][]byte                      // Global Subrs INDEX
	fds         []cffFD                       // 非 CID：单 FD；CID：FDArray 每 FD 一项
	fdSelect    func(gid uint16) (int, error) // 非 CID 时为 nil
	unitsPerEm  float64
}

// cffParseAll 解析 CFF 表全部需要的数据（蓝区 + 子例程 + charstrings）。
func cffParseAll(cff []byte, unitsPerEm int) (*cffFontData, error) {
	if len(cff) < 4 {
		return nil, fmt.Errorf("cff too short")
	}
	hdrSize := int(cff[2])
	if hdrSize < 4 || hdrSize > len(cff) {
		return nil, fmt.Errorf("bad cff header size")
	}
	p := hdrSize

	// NAME INDEX：跳过
	p, err := skipIndex(cff, p)
	if err != nil {
		return nil, err
	}
	// TOP DICT INDEX
	topDicts, next, err := readIndex(cff, p)
	if err != nil {
		return nil, err
	}
	if len(topDicts) == 0 {
		return nil, fmt.Errorf("no top dict")
	}
	p = next
	// STRING INDEX：跳过
	p, err = skipIndex(cff, p)
	if err != nil {
		return nil, err
	}
	// GLOBAL SUBRS INDEX
	gsubrs, _, err := readIndex(cff, p)
	if err != nil {
		return nil, err
	}

	topOps := dictOps(topDicts[0])
	csOff, ok := dictOpOffset(topOps, 17)
	if !ok {
		return nil, fmt.Errorf("cff: no CharStrings")
	}
	charStrings, _, err := readIndex(cff, csOff)
	if err != nil {
		return nil, fmt.Errorf("cff: charstrings: %w", err)
	}

	out := &cffFontData{
		charStrings: charStrings,
		globalSubrs: gsubrs,
		unitsPerEm:  float64(unitsPerEm),
	}

	// 非 CID：TOP DICT 直接有 Private [size, offset]
	if privSize, privOff, ok := dictPrivate(topOps); ok {
		fd, err := parseCFFFD(cff[privOff:privOff+privSize], privOff, cff, unitsPerEm)
		if err != nil {
			return nil, err
		}
		out.fds = []cffFD{*fd}
		return out, nil
	}

	// CID：FDSelect + FDArray
	fdSelectOff, ok1 := dictOpOffset(topOps, 1237)
	fdArrayOff, ok2 := dictOpOffset(topOps, 1236)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("cff: no private and no cid fdarray")
	}
	fdDicts, err := readIndexBytes(cff, fdArrayOff)
	if err != nil {
		return nil, err
	}
	for _, fdDict := range fdDicts {
		privSize, privOff, ok := dictPrivate(dictOps(fdDict))
		if !ok {
			return nil, fmt.Errorf("cff: fd has no private")
		}
		fd, err := parseCFFFD(cff[privOff:privOff+privSize], privOff, cff, unitsPerEm)
		if err != nil {
			return nil, err
		}
		out.fds = append(out.fds, *fd)
	}
	out.fdSelect = func(gid uint16) (int, error) {
		return readFDSelect(cff, fdSelectOff, gid)
	}
	return out, nil
}

// parseCFFFD 解析单个 FD 的 Private（蓝区 + Subrs + LanguageGroup）。
// Subrs 偏移相对 Private DICT 起点（cffload.c:2137：
// base + private_offset + local_subrs_offset），数据在完整 CFF 表上读。
func parseCFFFD(priv []byte, privOff int, cff []byte, unitsPerEm int) (*cffFD, error) {
	ops := dictOps(priv)
	var subrsOff int
	hasSubrs := false
	for _, op := range ops {
		if op.op == 19 && len(op.operands) >= 1 {
			// Subrs 是单操作数偏移（cfftoken.h:100 CFF_FIELD_NUM），
			// 指向一个 CFF INDEX；长度由 INDEX 自身决定。
			subrsOff = int(op.operands[len(op.operands)-1])
			hasSubrs = true
		}
	}
	fd := &cffFD{}
	if hasSubrs {
		items, _, err := readIndex(cff[privOff+subrsOff:], 0)
		if err != nil {
			return nil, fmt.Errorf("cff: fd subrs: %w", err)
		}
		fd.subrs = items
	}
	b, err := parsePrivateDict(priv, unitsPerEm)
	if err != nil {
		return nil, err
	}
	fd.blues = b
	fd.languageGroup = b.languageGroup
	return fd, nil
}

// dictOpOffset 取某 operator 的最后一个 operand 作为偏移。
func dictOpOffset(ops []dictOp, op int) (int, bool) {
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i].op == op && len(ops[i].operands) > 0 {
			return int(ops[i].operands[len(ops[i].operands)-1]), true
		}
	}
	return 0, false
}
