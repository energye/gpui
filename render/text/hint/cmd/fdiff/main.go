// fdiff 逐字 diff 对照器：自研 light 渲染 vs 系统 FreeType light（开发期度量衡）。
//
// 用法:
//
//	fdiff <font> <mode> <px> <runes|@group> [outdir]
//
//	mode: cjk   = hint.ModeLightCJK（afcjk 移植，M0 起）
//	      latin = hint.ModeLightLatin（tt_engine light，L0 起）
//	      none  = 原始轮廓（骨架期/基线对照）
//	px:   字号，可逗号分隔多档（如 12,14,16）
//	runes:直接字符序列，或 @分组名（VerifierSet 组名）
//
// 对每个 (rune,px)：自研光栅 PGM 与 ftexp(FT-light) PGM 逐像素 diff，
// 输出每字形一行指标 + 汇总。PGM 落盘 outdir（默认 /tmp/opencode/fdiff_out）。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/hint"
)

type pgm struct {
	w, h int
	px   []int // 0..255
}

func readPGM(path string) (*pgm, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := bufio.NewReader(f)
	var magic string
	if _, err := fmt.Fscan(rd, &magic); err != nil {
		return nil, err
	}
	var w, h, maxv int
	if _, err := fmt.Fscan(rd, &w, &h, &maxv); err != nil {
		return nil, err
	}
	vals := make([]int, 0, w*h)
	for i := 0; i < w*h; i++ {
		var v int
		if _, err := fmt.Fscan(rd, &v); err != nil {
			return nil, fmt.Errorf("PGM %s short: %d/%d: %w", path, i, w*h, err)
		}
		vals = append(vals, v)
	}
	return &pgm{w: w, h: h, px: vals}, nil
}

func writePGM(path string, w, h int, vals []int) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "P2\n%d %d\n255\n", w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sb.WriteString(strconv.Itoa(vals[y*w+x]))
			if x != w-1 {
				sb.WriteByte(' ')
			}
		}
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// inkBox 返回非零墨区 bbox [x0,y0,x1,y1)，无墨返回 nil。
func inkBox(p *pgm) [4]int {
	x0, y0, x1, y1 := p.w, p.h, -1, -1
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			if p.px[y*p.w+x] > 0 {
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	if x1 < 0 {
		return [4]int{0, 0, 0, 0}
	}
	return [4]int{x0, y0, x1 + 1, y1 + 1}
}

func ink(p *pgm) int {
	n := 0
	for _, v := range p.px {
		n += v
	}
	return n
}

// diff 按 ink bbox 左上角平移对齐后，在并集 bbox 内逐像素比较（缺失区补 0）。
// 返回 (mism0, mism16, maxDelta, unionPx, shiftX, shiftY)。
func diff(a, b *pgm) (mism0, mism16, maxDelta, unionPx, shiftX, shiftY int) {
	ab, bb := inkBox(a), inkBox(b)
	if ab[2] <= ab[0] || bb[2] <= bb[0] {
		return 0, 0, 0, 0, 0, 0
	}
	shiftX = bb[0] - ab[0]
	shiftY = bb[1] - ab[1]
	u := [4]int{
		min(ab[0], bb[0]), min(ab[1], bb[1]),
		max(ab[2], bb[2]), max(ab[3], bb[3]),
	}
	if u[2] <= u[0] || u[3] <= u[1] {
		return 0, 0, 0, 0, shiftX, shiftY
	}
	for y := u[1]; y < u[3]; y++ {
		for x := u[0]; x < u[2]; x++ {
			av, bv := 0, 0
			ax, ay := x-shiftX, y-shiftY // a 坐标平移到 b 坐标系
			if ax >= 0 && ax < a.w && ay >= 0 && ay < a.h {
				av = a.px[ay*a.w+ax]
			}
			if x >= 0 && x < b.w && y >= 0 && y < b.h {
				bv = b.px[y*b.w+x]
			}
			d := av - bv
			if d < 0 {
				d = -d
			}
			if d > maxDelta {
				maxDelta = d
			}
			if d > 0 {
				mism0++
			}
			if d > 16 {
				mism16++
			}
			unionPx++
		}
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runeToPGM(e *hint.Engine, parsed text.ParsedFont, r rune, px float64, mode string) ([]int, int, int, error) {
	var outline *text.GlyphOutline
	var err error
	switch mode {
	case "none":
		ext := text.NewOutlineExtractor()
		outline, err = ext.ExtractOutline(parsed, text.GlyphID(parsed.GlyphIndex(r)), px)
		if err != nil {
			return nil, 0, 0, err
		}
	default:
		var m hint.Mode
		if mode == "latin" {
			m = hint.ModeLightLatin
		} else {
			m = hint.ModeLightCJK
		}
		outline, err = e.Hint(parsed, text.GlyphID(parsed.GlyphIndex(r)), px, m)
		if err != nil {
			return nil, 0, 0, err
		}
	}
	if outline == nil || outline.IsEmpty() {
		return nil, 0, 0, nil
	}
	ras := text.NewGlyphMaskRasterizer()
	res, err := ras.RasterizeOutline(outline, 0, 0)
	if err != nil || res == nil {
		return nil, 0, 0, err
	}
	vals := make([]int, res.Width*res.Height)
	for i, b := range res.Mask {
		vals[i] = int(b)
	}
	return vals, res.Width, res.Height, nil
}

func main() {
	contour := flag.Bool("contour", false, "outline-level compare (FT 26.6 hinted outline vs self)")
	flag.Parse()
	args := flag.Args()
	if len(args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: fdiff <font> <mode:cjk|latin|none> <px:12,14,16> <runes|@group> [outdir]")
		os.Exit(2)
	}
	fontPath, mode, pxArg, runesArg := args[0], args[1], args[2], args[3]
	outdir := "/tmp/opencode/fdiff_out"
	if len(args) > 4 {
		outdir = args[4]
	}
	os.MkdirAll(outdir, 0o755)

	var runes []rune
	if strings.HasPrefix(runesArg, "@") {
		name := strings.TrimPrefix(runesArg, "@")
		found := false
		for _, g := range hint.VerifierSet() {
			if g.Name == name {
				runes = []rune(g.Chars)
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(os.Stderr, "unknown group %q\n", name)
			os.Exit(2)
		}
	} else {
		runes = []rune(runesArg)
	}
	var pxes []float64
	for _, s := range strings.Split(pxArg, ",") {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad px %q\n", s)
			os.Exit(2)
		}
		pxes = append(pxes, v)
	}

	src, err := text.NewFontSourceFromFile(fontPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "font:", err)
		os.Exit(1)
	}
	parsed := src.Parsed()
	e := hint.New()

	if *contour {
		runContourCompare(fontPath, parsed, e, mode, pxes, runes)
		return
	}
	fmt.Printf("== fdiff %s mode=%s px=%s runes=%d\n", fontPath, mode, pxArg, len(runes))
	var totM0, totM16, totInkSelf, totInkFT, totUnion int
	for _, px := range pxes {
		for _, r := range runes {
			if parsed.GlyphIndex(r) == 0 {
				fmt.Printf("skip  %q px=%.0f (no glyph)\n", r, px)
				continue
			}
			selfVals, sw, sh, err := runeToPGM(e, parsed, r, px, mode)
			if err != nil {
				fmt.Printf("FAIL  %q px=%.0f self: %v\n", r, px, err)
				continue
			}
			tag := mode // none|cjk|latin 直接做后缀，避免重复覆盖
			pgmOut := filepath.Join(outdir, fmt.Sprintf("self_%d_%d_%s.pgm", r, int(px), tag))
			if selfVals != nil {
				writePGM(pgmOut, sw, sh, selfVals)
			}
			ftHint := "l"
			if mode == "none" {
				ftHint = "n"
			}
			ftPgm := filepath.Join(outdir, fmt.Sprintf("ft_%d_%d_%s.pgm", r, int(px), tag))
			cmd := exec.Command("/tmp/opencode/ftexp/ftexp", fontPath, string(r), strconv.Itoa(int(px)), ftHint, ftPgm)
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("FAIL  %q px=%.0f ft: %v %s\n", r, px, err, string(out))
				continue
			}
			if selfVals == nil {
				fmt.Printf("empty %q px=%.0f (self no ink)\n", r, px)
				continue
			}
			ft, err := readPGM(ftPgm)
			if err != nil {
				fmt.Printf("FAIL  %q px=%.0f readft: %v\n", r, px, err)
				continue
			}
			self := &pgm{w: sw, h: sh, px: selfVals}
			m0, m16, md, up, sx, sy := diff(self, ft)
			ikS, ikF := ink(self), ink(ft)
			sb, fb := inkBox(self), inkBox(ft)
			fmt.Printf("%-25q px=%.0f  misth0=%-4d misth16=%-4d maxd=%-3d shift=(%+d,%+d)  ink self=%5d ft=%5d  bbox self=[%d,%d,%d,%d] ft=[%d,%d,%d,%d]  union=%d\n",
				r, px, m0, m16, md, sx, sy, ikS, ikF, sb[0], sb[1], sb[2], sb[3], fb[0], fb[1], fb[2], fb[3], up)
			totM0 += m0
			totM16 += m16
			totInkSelf += ikS
			totInkFT += ikF
			totUnion += up
		}
	}
	ratio := 0.0
	if totUnion > 0 {
		ratio = math.Round(float64(totM0)/float64(totUnion)*1000) / 10
	}
	fmt.Printf("== TOTAL mism=%d mism16=%d union=%d ratio=%.1f%% ink self=%d ft=%d\n",
		totM0, totM16, totUnion, ratio, totInkSelf, totInkFT)
}

// ---- 轮廓级对照（M0 主验证）：自研 hint 轮廓 vs FT light 网格化轮廓 ----

type ftContour struct {
	np, nc int
	adv26  int // 16.16 定点
	xs, ys []float64
	tags   []int
	ends   []int
}

func runFTContour(fontPath string, r rune, px float64, light bool) (*ftContour, error) {
	hintArg := "l"
	if !light {
		hintArg = "n"
	}
	cmd := exec.Command("/tmp/opencode/ftexp/ftexp", "contour", fontPath, string(r), strconv.Itoa(int(px)), hintArg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ftexp: %v %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("short contour output")
	}
	var fc ftContour
	if _, err := fmt.Sscanf(lines[0], "%d %d %d", &fc.np, &fc.nc, &fc.adv26); err != nil {
		return nil, fmt.Errorf("header: %v", err)
	}
	if fc.np <= 0 || fc.np > 10000 {
		return nil, fmt.Errorf("bad n_points %d", fc.np)
	}
	fc.xs = make([]float64, fc.np)
	fc.ys = make([]float64, fc.np)
	fc.tags = make([]int, fc.np)
	for i := 0; i < fc.np; i++ {
		var x, y int64
		var t int
		if _, err := fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &t); err != nil {
			return nil, fmt.Errorf("point %d: %v", i, err)
		}
		fc.xs[i] = float64(x) / 64
		fc.ys[i] = float64(y) / 64
		fc.tags[i] = t & 1
	}
	// ends line
	if len(lines) >= 2+fc.np {
		parts := strings.Fields(lines[1+fc.np])
		for _, p := range parts {
			v, _ := strconv.Atoi(p)
			fc.ends = append(fc.ends, v)
		}
	}
	return &fc, nil
}

// outlineMetrics 从轮廓计算度量（Y-down 语义，输出统一为 Y-up 像素）。
// top: 最高点（Y-up），bottom: 最低点（Y-up），left/right: 墨区边缘。
func outlineMetrics(o *text.GlyphOutline) (top, bottom, left, right float64) {
	top, bottom, left, right = 0, 0, 0, 0
	first := true
	for _, seg := range o.Segments {
		for _, p := range seg.Points {
			// Y-down → Y-up = -y
			up := -float64(p.Y)
			x := float64(p.X)
			if first {
				top, bottom, left, right = up, up, x, x
				first = false
			} else {
				if up > top {
					top = up
				}
				if up < bottom {
					bottom = up
				}
				if x < left {
					left = x
				}
				if x > right {
					right = x
				}
			}
		}
	}
	return
}

func runContourCompare(fontPath string, parsed text.ParsedFont, e *hint.Engine, mode string, pxes []float64, runes []rune) {
	fmt.Printf("== contour %s mode=%s\n", fontPath, mode)
	for _, px := range pxes {
		for _, r := range runes {
			gid := parsed.GlyphIndex(r)
			if gid == 0 {
				continue
			}
			light := mode != "none"
			fc, err := runFTContour(fontPath, r, px, light)
			if err != nil {
				fmt.Printf("FAIL  %q px=%.0f ft: %v\n", r, px, err)
				continue
			}
			// FT 度量（Y-up）
			ftTop, ftBot, ftLeft, ftRight := -1e9, 1e9, 1e9, -1e9
			for i := 0; i < fc.np; i++ {
				if fc.ys[i] > ftTop {
					ftTop = fc.ys[i]
				}
				if fc.ys[i] < ftBot {
					ftBot = fc.ys[i]
				}
				if fc.xs[i] < ftLeft {
					ftLeft = fc.xs[i]
				}
				if fc.xs[i] > ftRight {
					ftRight = fc.xs[i]
				}
			}
			// 自研轮廓
			var outline *text.GlyphOutline
			switch mode {
			case "none":
				outline, err = text.NewOutlineExtractor().ExtractOutline(parsed, text.GlyphID(gid), px)
			case "vert":
				outline, err = text.NewOutlineExtractor().ExtractOutlineHinted(parsed, text.GlyphID(gid), px, text.HintingVertical)
			default:
				var m hint.Mode
				if mode == "latin" {
					m = hint.ModeLightLatin
				} else {
					m = hint.ModeLightCJK
				}
				outline, err = e.Hint(parsed, text.GlyphID(gid), px, m)
			}
			if err != nil || outline == nil || outline.IsEmpty() {
				fmt.Printf("FAIL  %q px=%.0f self: %v\n", r, px, err)
				continue
			}
			selfTop, selfBot, selfLeft, selfRight := outlineMetrics(outline)
			fmt.Printf("%-25q px=%.0f  top  ft=%7.3f self=%7.3f d=%+.3f | bot  ft=%7.3f self=%7.3f d=%+.3f | left ft=%6.3f self=%6.3f right ft=%6.3f self=%6.3f | adv ft=%.2f\n",
				r, px,
				ftTop, selfTop, selfTop-ftTop,
				ftBot, selfBot, selfBot-ftBot,
				ftLeft, selfLeft, ftRight, selfRight,
				float64(fc.adv26)/65536)
		}
	}
}
