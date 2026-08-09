package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	ftLoadNoHinting    = 0x0002
	ftLoadNoStemDarken = 0x0040
	ftLoadTargetLight  = 1 << 16
	ftRenderNormal     = 0
	ftPixelModeGray    = 2
)

type ftBitmap struct {
	rows       uint32
	width      uint32
	pitch      int32
	_          uint32 // pad to align buffer
	buffer     uintptr
	numGrays   uint16
	pixelMode  uint8
	paletteMod uint8
	_          uint32 // pad to align palette
	palette    uintptr
}

var (
	libft       uintptr
	ftInit      func(lib *uintptr) int
	ftNewFace   func(lib uintptr, buf *byte, n int64, idx int64, face *uintptr) int
	ftSetCharSz func(face uintptr, cw, ch int64, hres, vres uint32) int
	ftCharIdx   func(face uintptr, ch rune) uint32
	ftLoadGlyph func(face uintptr, idx uint32, flags int32) int
	ftRender    func(slot uintptr, mode int32) int
	ftSetTrans  func(face uintptr, matrix, delta uintptr)
	ftGetAdv    func(face uintptr, gid uint32, flags int32, adv *int64) int
	ftSetVarD   func(face uintptr, numCoords uint32, coords *int64) int
	ftSetVarB   func(face uintptr, numCoords uint32, coords *int64) int
	ftGetVarD   func(face uintptr, numCoords uint32, coords *int64) int
	ftGetVarB   func(face uintptr, numCoords uint32, coords *int64) int
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: ftexp <ttf|ttc> [rune] [sizePx] [hint] [pgmOut]")
		fmt.Println("       ftexp line <ttf> <sizePx> <hint> <pgmOut> <text>")
		fmt.Println("       ftexp contour <ttf> <rune> <sizePx> <hint:l|n>")
		fmt.Println("       ftexp bcontour <ttf> <sizePx> <hint:l|n> <listFile> [blend]")
		fmt.Println("       ftexp bpgm <ttf> <sizePx> <hint:l|n> <listFile>")
		fmt.Println("  hint: l=light (default), n=nohint")
		fmt.Println("  bcontour 可选第 6 参数 blend：归一化坐标（2.14），0/缺省 = 默认实例")
		os.Exit(1)
	}
	if os.Args[1] == "metrics" {
		runMetrics()
		return
	}
	if os.Args[1] == "contour" {
		runContour()
		return
	}
	if os.Args[1] == "bcontour" {
		runBatchContour()
		return
	}
	if os.Args[1] == "fvar" {
		runFvar()
		return
	}
	if os.Args[1] == "line" {
		runLine()
		return
	}
	if os.Args[1] == "bpgm" {
		runBatchPGM()
		return
	}
	path := os.Args[1]
	r := '标'
	if len(os.Args) > 2 {
		rr := []rune(os.Args[2])
		if len(rr) > 0 {
			r = rr[0]
		}
	}
	px := float64(16)
	if len(os.Args) > 3 {
		fmt.Sscanf(os.Args[3], "%g", &px)
	}
	loadFlags := int32(ftLoadTargetLight)
	if len(os.Args) > 4 && os.Args[4] == "n" {
		loadFlags = ftLoadNoHinting
	}
	pgmOut := ""
	if len(os.Args) > 5 {
		pgmOut = os.Args[5]
	}
	sx, sy := 0.0, 0.0
	if len(os.Args) > 6 {
		fmt.Sscanf(os.Args[6], "%g", &sx)
	}
	if len(os.Args) > 7 {
		fmt.Sscanf(os.Args[7], "%g", &sy)
	}

	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Println("FAIL dlopen libfreetype.so.6")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftSetCharSz, libft, "FT_Set_Char_Size")
	purego.RegisterLibFunc(&ftCharIdx, libft, "FT_Get_Char_Index")
	purego.RegisterLibFunc(&ftLoadGlyph, libft, "FT_Load_Glyph")
	purego.RegisterLibFunc(&ftRender, libft, "FT_Render_Glyph")
	purego.RegisterLibFunc(&ftSetTrans, libft, "FT_Set_Transform")
	purego.RegisterLibFunc(&ftGetAdv, libft, "FT_Get_Advance")

	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		fmt.Printf("FAIL FT_Init_FreeType err=%d\n", err)
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("FAIL read:", err)
		os.Exit(1)
	}
	// Use index 0; .ttc collections expose faces per index.
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Printf("FAIL FT_New_Memory_Face err=%d\n", err)
		os.Exit(1)
	}
	sz := int64(px * 64)
	if err := ftSetCharSz(face, sz, sz, 0, 0); err != 0 {
		fmt.Printf("FAIL FT_Set_Char_Size err=%d\n", err)
		os.Exit(1)
	}
	// Subpixel shift: FT_Vector delta in 26.6 fixed point, Y-up → negate y.
	if sx != 0 || sy != 0 {
		type ftVector struct {
			x, y int64
		}
		delta := ftVector{x: int64(sx * 64), y: int64(-sy * 64)}
		ftSetTrans(face, 0, uintptr(unsafe.Pointer(&delta)))
	}
	gid := ftCharIdx(face, r)
	if gid == 0 {
		fmt.Printf("FAIL glyph 0 for %q\n", r)
		os.Exit(1)
	}
	if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
		fmt.Printf("FAIL FT_Load_Glyph err=%d\n", err)
		os.Exit(1)
	}
	// face layout (x86_64, freetype.h FT_FaceRec): num_*(32) + names(16) +
	// ints+ptrs(24) + generic(16) + bbox 4×FT_Pos(32) + 8 shorts(16)
	// → glyph slot ptr at offset 152.
	slot := *(*uintptr)(unsafe.Pointer(face + 152))
	if slot == 0 {
		fmt.Println("FAIL slot nil")
		os.Exit(1)
	}
	if err := ftRender(slot, ftRenderNormal); err != 0 {
		fmt.Printf("FAIL FT_Render_Glyph err=%d\n", err)
		os.Exit(1)
	}
	// slot layout (FT_GlyphSlotRec): bitmap at 152 (FT_Bitmap = 40 bytes),
	// bitmap_left at 192, bitmap_top at 196.
	bmp := *(*ftBitmap)(unsafe.Pointer(slot + 152))
	left := *(*int32)(unsafe.Pointer(slot + 192))
	top := *(*int32)(unsafe.Pointer(slot + 196))
	if bmp.pixelMode != ftPixelModeGray {
		fmt.Printf("FAIL pixel_mode=%d want 2(gray)\n", bmp.pixelMode)
		os.Exit(1)
	}
	rows, width, pitch := int(bmp.rows), int(bmp.width), int(bmp.pitch)
	var fg int
	var comps int
	seen := make([][]bool, rows)
	for y := 0; y < rows; y++ {
		seen[y] = make([]bool, width)
		row := (*[1 << 20]byte)(unsafe.Pointer(bmp.buffer))[y*pitch : y*pitch+width]
		for x := 0; x < width; x++ {
			if row[x] > 0 {
				fg++
				seen[y][x] = true
			}
		}
	}
	var stack [][2]int
	for y := 0; y < rows; y++ {
		for x := 0; x < width; x++ {
			if !seen[y][x] {
				continue
			}
			comps++
			seen[y][x] = false
			stack = append(stack, [2]int{x, y})
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || ny < 0 || nx >= width || ny >= rows || !seen[ny][nx] {
						continue
					}
					seen[ny][nx] = false
					stack = append(stack, [2]int{nx, ny})
				}
			}
		}
	}
	fmt.Printf("OK rune=%c size=%.0f bitmap=%dx%d pitch=%d left=%d top=%d fg=%d ratio=%.3f comps=%d\n",
		r, px, width, rows, pitch, left, top, fg,
		float64(fg)/float64(width*rows), comps)
	if pgmOut != "" {
		var sb []byte
		sb = append(sb, fmt.Sprintf("P2\n%d %d\n255\n", width, rows)...)
		for y := 0; y < rows; y++ {
			row := (*[1 << 20]byte)(unsafe.Pointer(bmp.buffer))[y*pitch : y*pitch+width]
			for x := 0; x < width; x++ {
				sb = append(sb, fmt.Sprintf("%d ", row[x])...)
			}
			sb = append(sb, '\n')
		}
		if err := os.WriteFile(pgmOut, sb, 0o644); err != nil {
			fmt.Println("FAIL write pgm:", err)
			os.Exit(1)
		}
	}
	runtime.KeepAlive(data)
}

// runLine renders a full text line with FreeType (hinted advances), composing
// each glyph bitmap into one PGM row. Usage: ftexp line <ttc> <px> <hint> <out> <text>
func runLine() {
	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftSetCharSz, libft, "FT_Set_Char_Size")
	purego.RegisterLibFunc(&ftCharIdx, libft, "FT_Get_Char_Index")
	purego.RegisterLibFunc(&ftLoadGlyph, libft, "FT_Load_Glyph")
	purego.RegisterLibFunc(&ftRender, libft, "FT_Render_Glyph")
	purego.RegisterLibFunc(&ftGetAdv, libft, "FT_Get_Advance")
	path := os.Args[2]
	px := float64(16)
	fmt.Sscanf(os.Args[3], "%g", &px)
	loadFlags := int32(ftLoadTargetLight)
	if os.Args[4] == "n" {
		loadFlags = ftLoadNoHinting
	}
	pgmOut := os.Args[5]
	text := os.Args[6]

	var lib uintptr
	ftInit(&lib)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Printf("FAIL FT_New_Memory_Face err=%d\n", err)
		os.Exit(1)
	}
	sz := int64(px * 64)
	ftSetCharSz(face, sz, sz, 0, 0)

	// First pass: total advance for row width.
	totalAdv := int64(0)
	for _, r := range text {
		if r == ' ' || r == '\t' {
			totalAdv += sz
			continue
		}
		gid := ftCharIdx(face, r)
		if gid == 0 {
			totalAdv += sz
			continue
		}
		ftLoadGlyph(face, gid, loadFlags)
		var ha int64
		ftGetAdv(face, gid, loadFlags, &ha)
		horiAdv := ha
		totalAdv += horiAdv
	}

	rowW := int(totalAdv/65536) + 24
	rowH := int(px) + 8
	buf := make([]byte, rowW*rowH)

	x := int64(0)
	for _, r := range text {
		if r == ' ' || r == '\t' {
			x += sz
			continue
		}
		gid := ftCharIdx(face, r)
		if gid == 0 {
			x += sz
			continue
		}
		ftLoadGlyph(face, gid, loadFlags)
		slot := *(*uintptr)(unsafe.Pointer(face + 152))
		var ha int64
		ftGetAdv(face, gid, loadFlags, &ha)
		horiAdv := ha
		ftRender(slot, ftRenderNormal)
		bmp := *(*ftBitmap)(unsafe.Pointer(slot + 152))
		left := *(*int32)(unsafe.Pointer(slot + 192))
		top := *(*int32)(unsafe.Pointer(slot + 196))
		rows, width, pitch := int(bmp.rows), int(bmp.width), int(bmp.pitch)
		if bmp.pixelMode == ftPixelModeGray && width > 0 && rows > 0 {
			baseX := int(x/65536) + int(left)
			baseY := int(px) + 4 - int(top)
			for y := 0; y < rows; y++ {
				src := (*[1 << 20]byte)(unsafe.Pointer(bmp.buffer))[y*pitch : y*pitch+width]
				for i := 0; i < width; i++ {
					dx, dy := baseX+i, baseY+y
					if dx >= 0 && dx < rowW && dy >= 0 && dy < rowH && src[i] > buf[dy*rowW+dx] {
						buf[dy*rowW+dx] = src[i]
					}
				}
			}
		}
		x += horiAdv
	}
	var sb []byte
	sb = append(sb, fmt.Sprintf("P2\n%d %d\n255\n", rowW, rowH)...)
	for y := 0; y < rowH; y++ {
		for i := 0; i < rowW; i++ {
			sb = append(sb, fmt.Sprintf("%d ", buf[y*rowW+i])...)
		}
		sb = append(sb, '\n')
	}
	if err := os.WriteFile(pgmOut, sb, 0o644); err != nil {
		fmt.Println("FAIL write:", err)
		os.Exit(1)
	}
	fmt.Printf("OK line px=%.0f %dx%d adv=%d\n", px, rowW, rowH, totalAdv)
	runtime.KeepAlive(data)
}

// runFvar prints the font's fvar axes plus FreeType's default design and
// normalized blend coordinates. Usage:
//
//	ftexp fvar <font>
func runFvar() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ftexp fvar <font>")
		os.Exit(2)
	}
	path := os.Args[2]
	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen libfreetype.so.6")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftGetVarD, libft, "FT_Get_Var_Design_Coordinates")
	purego.RegisterLibFunc(&ftGetVarB, libft, "FT_Get_Var_Blend_Coordinates")

	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL init")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	// fvar 轴数直接解析字体 fvar 表（FT_Get_Num_MM_Axes 已废弃移除）。
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	numTables := int(raw[4])<<8 | int(raw[5])
	var n int
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if string(raw[off:off+4]) == "fvar" {
			to := int(uint32(raw[off+8])<<24 | uint32(raw[off+9])<<16 | uint32(raw[off+10])<<8 | uint32(raw[off+11]))
			n = int(raw[to+8]<<8 | raw[to+9])
			break
		}
	}
	if n <= 0 {
		n = 1
	}
	design := make([]int64, n)
	blend := make([]int64, n)
	ftGetVarD(face, uint32(n), &design[0])
	ftGetVarB(face, uint32(n), &blend[0])
	fmt.Printf("axes=%d\n", n)
	fmt.Printf("design:")
	for _, c := range design {
		fmt.Printf(" %g", float64(c)/65536)
	}
	fmt.Printf("\nblend:")
	for _, c := range blend {
		fmt.Printf(" %g", float64(c)/65536)
	}
	fmt.Printf("\n")
	// 可选第 4 参数：设置该 design 坐标后读 blend（验证 design→blend 转换）。
	if len(os.Args) >= 4 {
		var w float64
		fmt.Sscanf(os.Args[3], "%g", &w)
		purego.RegisterLibFunc(&ftSetVarD, libft, "FT_Set_Var_Design_Coordinates")
		d := int64(w * 65536)
		ftSetVarD(face, 1, &d)
		ftGetVarB(face, uint32(n), &blend[0])
		fmt.Printf("set design=%g → blend:", w)
		for _, c := range blend {
			fmt.Printf(" %g", float64(c)/65536)
		}
		fmt.Printf("\n")
	}
	runtime.KeepAlive(data)
}

func toff16(raw []byte, at int) int {
	return int(raw[at])<<8 | int(raw[at+1])
}

// runBatchContour processes a whole character list per FreeType process,
// dumping hinted 26.6 outlines for every rune. Usage:
//
//	ftexp bcontour <ttc> <sizePx> <hint:l|n> <runestring>[:<outstem0|raw>]
//
// Output uses the same format as runContour per rune, with a leading line
// "# R <rune> <np> <nc> <adv>" before each glyph block so the caller can
// associate blocks to runes without per-rune exec.
// outlineHeuristicOK 判断 slot 偏移 off 处是否像 FT_Outline（n_contours/
// n_points/points/tags/contours 五字段 + ends 严格递增且末项 == n_points-1）。
// 所有指针解引用都在 recover 保护下：陈旧/垃圾指针可能指向未映射内存，
// Go 运行时会把它转成可恢复 panic，恢复后按「不匹配」处理，避免整个进程
// 崩溃（旧的逐候选扫描曾因此整进程 panic）。
func outlineHeuristicOK(slot uintptr, off int) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	nc := *(*int16)(unsafe.Pointer(slot + uintptr(off)))
	np := *(*int16)(unsafe.Pointer(slot + uintptr(off) + 2))
	if nc <= 0 || nc > 100 || np <= 0 || np > 10000 {
		return false
	}
	pts := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 8))
	tags := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 16))
	ends := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 24))
	if pts == 0 || tags == 0 || ends == 0 {
		return false
	}
	prev := int16(-1)
	for c := 0; c < int(nc); c++ {
		e := *(*int16)(unsafe.Pointer(ends + uintptr(c)*2))
		if e < prev || e < 0 || e >= np {
			return false
		}
		prev = e
	}
	return prev == np-1
}

// scanOutlineOffset 是旧式「首个命中」扫描，仅作固定偏移对某字形失效时的
// 兜底。它仍可能偶发命中陈旧数据，但只影响单个字形（不再是批量级错误）。
func scanOutlineOffset(slot uintptr) int {
	for off := 0; off < 256; off += 8 {
		if outlineHeuristicOK(slot, off) {
			return off
		}
	}
	return -1
}

// probeOutlineOffset 加载 probeGids 中的字形，统计每个通过启发式校验的 slot
// 偏移的出现次数，返回众数——即真实 FT_Outline 偏移（对 FT 版本无关）。
// 垃圾命中只覆盖个别字形，无法超越真实偏移的接近 100% 命中率。无任何命中
// 时返回 -1，调用方退回 scanOutlineOffset。
func probeOutlineOffset(face uintptr, loadFlags int32, probeGids []uint32) int {
	tally := make(map[int]int)
	for _, gid := range probeGids {
		if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
			continue
		}
		slot := *(*uintptr)(unsafe.Pointer(face + 152))
		if slot == 0 {
			continue
		}
		for off := 0; off < 256; off += 8 {
			if outlineHeuristicOK(slot, off) {
				tally[off]++
			}
		}
	}
	best, bestN := -1, 0
	for off, n := range tally {
		if n > bestN {
			best, bestN = off, n
		}
	}
	return best
}

func runBatchContour() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "usage: ftexp bcontour <ttf|ttc> <sizePx> <hint:l|n> <runestring>")
		os.Exit(2)
	}
	path := os.Args[2]
	px := float64(16)
	fmt.Sscanf(os.Args[3], "%g", &px)
	loadFlags := int32(ftLoadTargetLight)
	if os.Args[4] == "n" {
		loadFlags = ftLoadNoHinting
	}
	if os.Args[4] == "d" {
		loadFlags = ftLoadTargetLight | ftLoadNoStemDarken
	}
	if os.Args[4] == "f" {
		loadFlags = 0 // FT_LOAD_TARGET_NORMAL = full hint
	}
	rawText, err := os.ReadFile(os.Args[5])
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read list:", err)
		os.Exit(1)
	}

	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen libfreetype.so.6")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftSetCharSz, libft, "FT_Set_Char_Size")
	purego.RegisterLibFunc(&ftCharIdx, libft, "FT_Get_Char_Index")
	purego.RegisterLibFunc(&ftLoadGlyph, libft, "FT_Load_Glyph")
	purego.RegisterLibFunc(&ftGetAdv, libft, "FT_Get_Advance")
	purego.RegisterLibFunc(&ftSetVarB, libft, "FT_Set_MM_Blend_Coordinates")
	purego.RegisterLibFunc(&ftSetVarD, libft, "FT_Set_Var_Design_Coordinates")

	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL init")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	sz := int64(px * 64)
	if err := ftSetCharSz(face, sz, sz, 0, 0); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL chsize")
		os.Exit(1)
	}
	// 可选第 6 参数 variant：变体坐标。
	// 缺省/0 = 默认实例；第 7 参数 "d" = design 模式（FT_Set_Var_Design_Coordinates，
	// w 为设计坐标，FT 自算归一化+avar），否则 = blend 模式
	// （FT_Set_MM_Blend_Coordinates，w 为 avar 后归一化坐标，2.14 定点，
	// 即引擎 LightHintVar 的 coords 值）。
	if len(os.Args) >= 7 {
		var w float64
		fmt.Sscanf(os.Args[6], "%g", &w)
		if w != 0 {
			design := len(os.Args) >= 8 && os.Args[7] == "d"
			if design {
				d := int64(w * 65536)
				if err := ftSetVarD(face, 1, &d); err != 0 {
					fmt.Fprintf(os.Stderr, "FAIL var design err=%d\n", err)
					os.Exit(1)
				}
			} else {
				blendC := int64(w * 65536)
				if err := ftSetVarB(face, 1, &blendC); err != 0 {
					fmt.Fprintf(os.Stderr, "FAIL var blend err=%d\n", err)
					os.Exit(1)
				}
			}
		}
	}

	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	runes := []rune(string(rawText))
	// 先探针定 FT_Outline 偏移（众数）：slot 内存暴力扫描会偶发命中陈旧/
	// 未初始化字节（ASLR 相关，逐进程不同），旧式「首个命中」会让个别字形
	// 读到垃圾轮廓（点数、坐标全错）。真实 outline 每字形 100% 命中启发式，
	// 众数即真实偏移，且与 FT 版本无关。
	outlineOff := -1
	if len(runes) > 0 {
		probeN := len(runes)
		if probeN > 128 {
			probeN = 128
		}
		probeGids := make([]uint32, 0, probeN)
		for _, r := range runes[:probeN] {
			if gid := ftCharIdx(face, r); gid != 0 {
				probeGids = append(probeGids, gid)
			}
		}
		outlineOff = probeOutlineOffset(face, loadFlags, probeGids)
	}
	for _, r := range runes {
		gid := ftCharIdx(face, r)
		if gid == 0 {
			fmt.Fprintf(bw, "# R %U MISSING\n", r)
			continue
		}
		if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
			fmt.Fprintf(bw, "# R %U LOADFAIL\n", r)
			continue
		}
		slot := *(*uintptr)(unsafe.Pointer(face + 152))
		if slot == 0 {
			fmt.Fprintf(bw, "# R %U SLOTFAIL\n", r)
			continue
		}
		off := outlineOff
		if off < 0 || !outlineHeuristicOK(slot, off) {
			off = scanOutlineOffset(slot)
		}
		if off < 0 {
			fmt.Fprintf(bw, "# R %U FAIL\n", r)
			continue
		}
		outline := slot + uintptr(off)
		np := int(*(*int16)(unsafe.Pointer(outline + 2)))
		nc := int(*(*int16)(unsafe.Pointer(outline)))
		pts := *(*uintptr)(unsafe.Pointer(outline + 8))
		tags := *(*uintptr)(unsafe.Pointer(outline + 16))
		ends := *(*uintptr)(unsafe.Pointer(outline + 24))
		var adv int64
		if err := ftGetAdv(face, gid, loadFlags, &adv); err != 0 {
			adv = 0
		}
		fmt.Fprintf(bw, "# R %U %d %d %d\n", r, np, nc, adv)
		for i := 0; i < np; i++ {
			x := *(*int64)(unsafe.Pointer(pts + uintptr(i)*16))
			y := *(*int64)(unsafe.Pointer(pts + uintptr(i)*16 + 8))
			t := *(*byte)(unsafe.Pointer(tags + uintptr(i)))
			fmt.Fprintf(bw, "%d %d %d\n", x, y, int(t))
		}
		for c := 0; c < nc; c++ {
			if c > 0 {
				bw.WriteByte(' ')
			}
			fmt.Fprintf(bw, "%d", *(*int16)(unsafe.Pointer(ends + uintptr(c)*2)))
		}
		bw.WriteByte('\n')
	}
	runtime.KeepAlive(data)
}

// runBatchPGM renders each rune of a character list into its own PGM and
// prints one block per rune so the caller can compare without per-rune exec.
//
//	ftexp bpgm <ttc> <sizePx> <hint:l|n> <runestring>
//
// Block format (per rune):
//
//	# B <rune> <w> <h> <left> <top>   (w=0,h=0 when MISSING/FAIL)
//	<w*h gray values, one row per line>
func runBatchPGM() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "usage: ftexp bpgm <ttf|ttc> <sizePx> <hint:l|n> <runestring>")
		os.Exit(2)
	}
	path := os.Args[2]
	px := float64(16)
	fmt.Sscanf(os.Args[3], "%g", &px)
	loadFlags := int32(ftLoadTargetLight)
	if os.Args[4] == "n" {
		loadFlags = ftLoadNoHinting
	}
	if os.Args[4] == "d" {
		loadFlags = ftLoadTargetLight | ftLoadNoStemDarken
	}
	if os.Args[4] == "f" {
		loadFlags = 0 // FT_LOAD_TARGET_NORMAL = full hint
	}
	rawText, err := os.ReadFile(os.Args[5])
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read list:", err)
		os.Exit(1)
	}

	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen libfreetype.so.6")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftSetCharSz, libft, "FT_Set_Char_Size")
	purego.RegisterLibFunc(&ftCharIdx, libft, "FT_Get_Char_Index")
	purego.RegisterLibFunc(&ftLoadGlyph, libft, "FT_Load_Glyph")
	purego.RegisterLibFunc(&ftRender, libft, "FT_Render_Glyph")

	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL init")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	sz := int64(px * 64)
	if err := ftSetCharSz(face, sz, sz, 0, 0); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL chsize")
		os.Exit(1)
	}

	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	for _, r := range []rune(string(rawText)) {
		gid := ftCharIdx(face, r)
		if gid == 0 {
			fmt.Fprintf(bw, "# B %U 0 0 0 0\n", r)
			continue
		}
		if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
			fmt.Fprintf(bw, "# B %U 0 0 0 0\n", r)
			continue
		}
		slot := *(*uintptr)(unsafe.Pointer(face + 152))
		if slot == 0 {
			fmt.Fprintf(bw, "# B %U 0 0 0 0\n", r)
			continue
		}
		if err := ftRender(slot, ftRenderNormal); err != 0 {
			fmt.Fprintf(bw, "# B %U 0 0 0 0\n", r)
			continue
		}
		bmp := *(*ftBitmap)(unsafe.Pointer(slot + 152))
		left := *(*int32)(unsafe.Pointer(slot + 192))
		top := *(*int32)(unsafe.Pointer(slot + 196))
		rows, width, pitch := int(bmp.rows), int(bmp.width), int(bmp.pitch)
		if bmp.pixelMode != ftPixelModeGray || width <= 0 || rows <= 0 {
			fmt.Fprintf(bw, "# B %U 0 0 0 0\n", r)
			continue
		}
		fmt.Fprintf(bw, "# B %U %d %d %d %d\n", r, width, rows, left, top)
		for y := 0; y < rows; y++ {
			src := (*[1 << 20]byte)(unsafe.Pointer(bmp.buffer))[y*pitch : y*pitch+width]
			for i := 0; i < width; i++ {
				if i > 0 {
					bw.WriteByte(' ')
				}
				fmt.Fprintf(bw, "%d", src[i])
			}
			bw.WriteByte('\n')
		}
	}
	runtime.KeepAlive(data)
}

// runContour loads a glyph with the given hint flags and dumps the hinted
// outline (FT_Outline) as 26.6 fixed-point vertices:
//
//	NPOINTS NCONTOURS ADV26
//	x26 y26 tag   (per point)
//	E0 E1 ...     (contour ends, FT_Vector = 2×int64, tags = 1 byte/point)
func runContour() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: ftexp contour <ttf|ttc> <rune> <sizePx> <hint:l|n>")
		os.Exit(2)
	}
	path := os.Args[2]
	rr := []rune(os.Args[3])
	r := '标'
	if len(rr) > 0 {
		r = rr[0]
	}
	px := float64(16)
	fmt.Sscanf(os.Args[4], "%g", &px)
	loadFlags := int32(ftLoadTargetLight)
	if len(os.Args) > 5 && os.Args[5] == "n" {
		loadFlags = ftLoadNoHinting
	}
	if len(os.Args) > 5 && os.Args[5] == "d" {
		loadFlags = ftLoadTargetLight | ftLoadNoStemDarken
	}
	if len(os.Args) > 5 && os.Args[5] == "f" {
		loadFlags = 0 // FT_LOAD_TARGET_NORMAL = full hint
	}

	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen libfreetype.so.6")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	purego.RegisterLibFunc(&ftSetCharSz, libft, "FT_Set_Char_Size")
	purego.RegisterLibFunc(&ftCharIdx, libft, "FT_Get_Char_Index")
	purego.RegisterLibFunc(&ftLoadGlyph, libft, "FT_Load_Glyph")
	purego.RegisterLibFunc(&ftGetAdv, libft, "FT_Get_Advance")

	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL init")
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	sz := int64(px * 64)
	if err := ftSetCharSz(face, sz, sz, 0, 0); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL chsize")
		os.Exit(1)
	}
	gid := ftCharIdx(face, r)
	if gid == 0 {
		fmt.Fprintln(os.Stderr, "FAIL no glyph")
		os.Exit(1)
	}
	// 先探针定 FT_Outline 偏移（众数，理由同 bcontour）：用 gid 1..128 探测，
	// 与目标字形无关，避免「首个命中」扫描偶发读到陈旧数据。探测会覆盖 slot
	// 状态，所以必须在加载目标字形之前执行。
	var probeGids []uint32
	for g := uint32(1); g <= 128; g++ {
		probeGids = append(probeGids, g)
	}
	outlineOff := probeOutlineOffset(face, loadFlags, probeGids)
	if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL loadglyph")
		os.Exit(1)
	}
	slot := *(*uintptr)(unsafe.Pointer(face + 152))
	if slot == 0 {
		fmt.Fprintln(os.Stderr, "FAIL slot nil")
		os.Exit(1)
	}
	off := outlineOff
	if off < 0 || !outlineHeuristicOK(slot, off) {
		off = scanOutlineOffset(slot)
	}
	if off < 0 {
		fmt.Fprintln(os.Stderr, "FAIL outline probe")
		os.Exit(1)
	}
	outline := slot + uintptr(off)
	np := int(*(*int16)(unsafe.Pointer(outline + 2)))
	nc := int(*(*int16)(unsafe.Pointer(outline)))
	pts := *(*uintptr)(unsafe.Pointer(outline + 8))
	tags := *(*uintptr)(unsafe.Pointer(outline + 16))
	ends := *(*uintptr)(unsafe.Pointer(outline + 24))
	var adv int64
	if err := ftGetAdv(face, gid, loadFlags, &adv); err != 0 {
		adv = 0
	}
	fmt.Printf("%d %d %d\n", np, nc, adv)
	for i := 0; i < np; i++ {
		x := *(*int64)(unsafe.Pointer(pts + uintptr(i)*16))
		y := *(*int64)(unsafe.Pointer(pts + uintptr(i)*16 + 8))
		t := *(*byte)(unsafe.Pointer(tags + uintptr(i)))
		fmt.Printf("%d %d %d\n", x, y, int(t))
	}
	for c := 0; c < nc; c++ {
		if c > 0 {
			fmt.Printf(" ")
		}
		fmt.Printf("%d", *(*int16)(unsafe.Pointer(ends + uintptr(c)*2)))
	}
	fmt.Println()
	runtime.KeepAlive(data)
}

// runMetrics dumps face-level metrics from FT_FaceRec (loaded at index 0):
// units_per_em, ascender, descender, height (font units).
func runMetrics() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ftexp metrics <ttf|ttc>")
		os.Exit(2)
	}
	path := os.Args[2]
	libft, _ = purego.Dlopen("libfreetype.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libft == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen")
		os.Exit(1)
	}
	purego.RegisterLibFunc(&ftInit, libft, "FT_Init_FreeType")
	purego.RegisterLibFunc(&ftNewFace, libft, "FT_New_Memory_Face")
	var lib uintptr
	if err := ftInit(&lib); err != 0 {
		os.Exit(1)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}
	var face uintptr
	if err := ftNewFace(lib, &data[0], int64(len(data)), 0, &face); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	// FT_FaceRec (x86_64): ... generic(16) bbox(32) units_per_em@136
	// ascender@138 descender@140 height@142 ... glyph@152 (已知)
	units := *(*int16)(unsafe.Pointer(face + 136))
	asc := *(*int16)(unsafe.Pointer(face + 138))
	desc := *(*int16)(unsafe.Pointer(face + 140))
	height := *(*int16)(unsafe.Pointer(face + 142))
	fmt.Printf("upem=%d ascender=%d descender=%d height=%d\n", int(units), int(asc), int(desc), int(height))
	runtime.KeepAlive(data)
}
