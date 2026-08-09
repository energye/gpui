package main

import (
	"bufio"
	"fmt"
	"os"
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
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: ftexp <ttf|ttc> [rune] [sizePx] [hint] [pgmOut]")
		fmt.Println("       ftexp line <ttf> <sizePx> <hint> <pgmOut> <text>")
		fmt.Println("       ftexp contour <ttf> <rune> <sizePx> <hint:l|n>")
		fmt.Println("       ftexp bcontour <ttf> <sizePx> <hint:l|n> <listFile>")
		fmt.Println("       ftexp bpgm <ttf> <sizePx> <hint:l|n> <listFile>")
		fmt.Println("  hint: l=light (default), n=nohint")
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
	fmt.Printf("DBG face=%x slot=%x\n", face, slot)
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
		fmt.Printf("DBG adv %q = %d (%gpx)\n", r, horiAdv, float64(horiAdv)/65536)
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
}

// runBatchContour processes a whole character list per FreeType process,
// dumping hinted 26.6 outlines for every rune. Usage:
//
//	ftexp bcontour <ttc> <sizePx> <hint:l|n> <runestring>[:<outstem0|raw>]
//
// Output uses the same format as runContour per rune, with a leading line
// "# R <rune> <np> <nc> <adv>" before each glyph block so the caller can
// associate blocks to runes without per-rune exec.
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
		outline := uintptr(0)
		for off := 0; off < 256; off += 8 {
			nc := *(*int16)(unsafe.Pointer(slot + uintptr(off)))
			np := *(*int16)(unsafe.Pointer(slot + uintptr(off) + 2))
			if nc <= 0 || nc > 100 || np <= 0 || np > 10000 {
				continue
			}
			pts := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 8))
			tags := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 16))
			ends := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 24))
			if pts == 0 || tags == 0 || ends == 0 {
				continue
			}
			ok := true
			prev := int16(-1)
			for c := 0; c < int(nc); c++ {
				e := *(*int16)(unsafe.Pointer(ends + uintptr(c)*2))
				if e < prev || e < 0 || e >= np {
					ok = false
					break
				}
				prev = e
			}
			if ok && prev == np-1 {
				outline = slot + uintptr(off)
				break
			}
		}
		if outline == 0 {
			fmt.Fprintf(bw, "# R %U FAIL\n", r)
			continue
		}
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
	if err := ftLoadGlyph(face, gid, loadFlags); err != 0 {
		fmt.Fprintln(os.Stderr, "FAIL loadglyph")
		os.Exit(1)
	}
	slot := *(*uintptr)(unsafe.Pointer(face + 152))
	if slot == 0 {
		fmt.Fprintln(os.Stderr, "FAIL slot nil")
		os.Exit(1)
	}
	// Probe FT_Outline location: scan slot for {n_contours,n_points,points,tags,contours}
	// with contours strictly increasing ending at n_points-1.
	outline := uintptr(0)
	for off := 0; off < 256; off += 8 {
		nc := *(*int16)(unsafe.Pointer(slot + uintptr(off)))
		np := *(*int16)(unsafe.Pointer(slot + uintptr(off) + 2))
		if nc <= 0 || nc > 100 || np <= 0 || np > 10000 {
			continue
		}
		pts := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 8))
		tags := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 16))
		ends := *(*uintptr)(unsafe.Pointer(slot + uintptr(off) + 24))
		if pts == 0 || tags == 0 || ends == 0 {
			continue
		}
		ok := true
		prev := int16(-1)
		for c := 0; c < int(nc); c++ {
			e := *(*int16)(unsafe.Pointer(ends + uintptr(c)*2))
			if e < prev || e < 0 || e >= np {
				ok = false
				break
			}
			prev = e
		}
		if ok && prev == np-1 {
			outline = slot + uintptr(off)
			break
		}
	}
	if outline == 0 {
		fmt.Fprintln(os.Stderr, "FAIL outline probe")
		os.Exit(1)
	}
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
}
