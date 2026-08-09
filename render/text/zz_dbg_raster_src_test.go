package text

import (
	"os"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// 决定性对照：同一光栅器分别喂 FT-26.6 轮廓与生产轮廓，
// 再与 FT 位图比对。若 FT 轮廓喂光栅≈FT 位图而生产轮廓喂光栅≠FT，
// → 源头在轮廓（桥/引擎）；若两者都与 FT 位图差 → 光栅器问题。
func TestDbgRasterSource(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	for _, px := range []float64{12, 16, 24, 32, 48} {
		for _, r := range []rune("静每合") {
			gid := GlyphID(parsed.GlyphIndex(r))
			ext := NewOutlineExtractor()
			o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
			if err != nil || o == nil {
				t.Fatalf("%c %.0fpx outline: %v", r, px, err)
			}
			ras := NewGlyphMaskRasterizer()
			res, err := ras.RasterizeOutline(o, 0, 0)
			if err != nil || res == nil {
				t.Fatalf("%c %.0fpx raster: %v", r, px, err)
			}

			// 1) FT 位图（ftexp P2）
			ftPgm := t.TempDir() + "/ft.pgm"
			cmd := exec.Command(ftexpLocal(t), fontPath, string(r), strconv.Itoa(int(px)), "l", ftPgm)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ftexp: %v %s", err, out)
			}
			fw, fh, fv := readPgm2(ftPgm)

			// 2) FT-26.6 轮廓 → GlyphOutline → 同一光栅器
			ftOutline := ftOutlineToGlyphSimpler(t, fontPath, r, px)
			resFT, err := ras.RasterizeOutline(ftOutline, 0, 0)
			if err != nil || resFT == nil {
				t.Logf("%c %.0fpx: FT-outline raster fail: %v", r, px, err)
				continue
			}
			// 3) 统计：生产 vs FT位图 & FT轮廓光栅 vs FT位图（bestShift 对齐）
			misp, misf := 0, 0
			if res != nil {
				misp = bestShiftMism(res.Mask, res.Width, res.Height, fv, fw, fh)
			}
			if resFT != nil {
				misf = bestShiftMism(resFT.Mask, resFT.Width, resFT.Height, fv, fw, fh)
			}
			fmt.Printf("%c %.0fpx: 生产光栅 vs FT位图 mism=%d | FT轮廓光栅 vs FT位图 mism=%d (FT位图 %dx%d)\n",
				r, px, misp, misf, fw, fh)
		}
	}
}

// ftpToGlyph 从 ftexp contour 输出（26.6 Y-up + tag）构建 GlyphOutline
func ftpToGlyph(t *testing.T, fontPath string, r rune, px float64) *GlyphOutline {
	t.Helper()
	cmd := exec.Command(ftexpLocal(t), "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var np, nc, adv int
	fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
	type pt struct{ x, y int64; on bool }
	pts := make([]pt, np)
	for i := 0; i < np; i++ {
		var x, y, tag int64
		fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
		pts[i] = pt{x: x, y: y, on: tag&1 == 1}
	}
	// ends 行在 np+1
	var ends []int
	if len(lines) >= 2+np {
		for _, s := range strings.Fields(lines[1+np]) {
			if v, err := strconv.Atoi(s); err == nil {
				ends = append(ends, v)
			}
		}
	}
	// 26.6 Y-up → Y-down 像素：X= x/64, Y= -y/64
	xPx := func(v int64) float32 { return float32(float64(v) / 64.0) }
	yPx := func(v int64) float32 { return -float32(float64(v) / 64.0) }

	var segs []OutlineSegment
	first := 0
	for _, n := range ends {
		end := n + 1
		if end > len(pts) {
			break
		}
		segs = append(segs, OutlineSegment{Op: OutlineOpMoveTo, Points: [3]OutlinePoint{{X: xPx(pts[first].x), Y: yPx(pts[first].y)}}})
		i := first + 1
		for i < end {
			p := pts[i]
			if p.on {
				segs = append(segs, OutlineSegment{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: xPx(p.x), Y: yPx(p.y)}}})
				i++
				continue
			}
			// off：若 （后面）还有 off 则隐式中点拆两段 QuadTo
			if i+1 < end && pts[i+1].on {
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(p.x), Y: yPx(p.y)},
					{X: xPx(pts[i+1].x), Y: yPx(pts[i+1].y)},
				}})
				i += 2
				continue
			}
			// 双 off → 隐式 on = 中点，拆两段
			if i+2 < end && !pts[i+1].on && pts[i+2].on {
				midX := (p.x + pts[i+1].x) / 2
				midY := (p.y + pts[i+1].y) / 2
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(p.x), Y: yPx(p.y)},
					{X: xPx(midX), Y: yPx(midY)},
				}})
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(pts[i+1].x), Y: yPx(pts[i+1].y)},
					{X: xPx(pts[i+2].x), Y: yPx(pts[i+2].y)},
				}})
				i += 3
				continue
			}
			// 闭合语义：off 到轮廓尾 → 与首 on 点构成曲线
			p2 := pts[first] // 轮廓起始点
			if !p2.on {
				t.Fatalf("%c 轮廓起始非 on 点 at %d", r, first)
			}
			if !p.on && i+1 == end { // 单 off 到起点
				if !pts[i+1].on && i+2 == end {
					t.Fatalf("unhandled")
				}
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(p.x), Y: yPx(p.y)},
					{X: xPx(p2.x), Y: yPx(p2.y)},
				}})
				i = end
				continue
			}
			t.Fatalf("%c sim 分解失败 at i=%d off=%v n1on=%v n2on=%v end=%d", r, i, pts[i].on, pts[i+1].on, pts[i+2].on, end)
		}
	}
	g := &GlyphOutline{Segments: segs, Type: GlyphTypeOutline}
	refreshOutlineBounds(g)
	return g
}

func ftOutlineToGlyphSimpler(t *testing.T, fontPath string, r rune, px float64) *GlyphOutline {
	t.Helper()
	cmd := exec.Command(ftexpLocal(t), "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var np, nc, adv int
	fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
	type pt struct{ x, y int64; on bool }
	pts := make([]pt, np)
	for i := 0; i < np; i++ {
		var x, y, tag int64
		fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
		pts[i] = pt{x: x, y: y, on: tag&1 == 1}
	}
	var ends []int
	if len(lines) >= 2+np {
		for _, s := range strings.Fields(lines[1+np]) {
			if v, err := strconv.Atoi(s); err == nil {
				ends = append(ends, v)
			}
		}
	}
	if len(ends) == 0 {
		t.Fatalf("no ends")
	}
	xPx := func(v int64) float32 { return float32(v) / 64 }
	yPx := func(v int64) float32 { return -float32(v) / 64 }

	// 通用扫描：以 on 为界切段；off 处理：
	//   t = [off] on        → 前一 on 与 off 组成 QuadTo（off,on）
	//   t = [off off] on      → 两段 QuadTo
	// 若 on 后接 off：把 on 作为曲线起点上下文。
	var segs []OutlineSegment
	start := 0
	for _, n := range ends {
		end := n + 1
		if end > len(pts) {
			break
		}
		// move
		segs = append(segs, OutlineSegment{Op: OutlineOpMoveTo, Points: [3]OutlinePoint{{X: xPx(pts[start].x), Y: yPx(pts[start].y)}}})
		// 后续：从第二个点开始处理；序列保证 off 出现就有后续 on
		i := start + 1
		for i < end {
			if pts[i].on {
				// on → 直线（前一个 on 已在上一段处置）
				segs = append(segs, OutlineSegment{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: xPx(pts[i].x), Y: yPx(pts[i].y)}}})
				i++
				continue
			}
			// off：看后续
			if i+1 < end && pts[i+1].on {
				// 单 off
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(pts[i].x), Y: yPx(pts[i].y)},
					{X: xPx(pts[i+1].x), Y: yPx(pts[i+1].y)},
				}})
				i += 2
				continue
			}
if i+2 <= end && !pts[i+1].on {
				// 双 off：到达尾部时闭合回起点
				midX := (pts[i].x + pts[i+1].x) / 2
				midY := (pts[i].y + pts[i+1].y) / 2
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(pts[i].x), Y: yPx(pts[i].y)},
					{X: xPx(midX), Y: yPx(midY)},
				}})
				endPt := OutlinePoint{}
				if i+2 == end {
					// 闭合到轮廓起点
					endPt = OutlinePoint{X: xPx(pts[start].x), Y: yPx(pts[start].y)}
					i = end
				} else {
					endPt = OutlinePoint{X: xPx(pts[i+2].x), Y: yPx(pts[i+2].y)}
					i += 3
				}
				segs = append(segs, OutlineSegment{Op: OutlineOpQuadTo, Points: [3]OutlinePoint{
					{X: xPx(pts[i-1].x), Y: yPx(pts[i-1].y)},
					{X: endPt.X, Y: endPt.Y},
				}})
				continue
			}
			t.Fatalf("%c sim2 分解失败 at i=%d rev=%v", r, i, start)
		}
		start = end
	}
	g := &GlyphOutline{Segments: segs, Type: GlyphTypeOutline}
	refreshOutlineBounds(g)
	return g
}

func maskDiff(w, h int, mask []byte, fw, fh int, fv []byte, padX, padY int) int {
	if w < fw+2 || h < fh+2 {
		return 1 << 30
	}
	m16 := 0
	for y := 0; y < fh; y++ {
		for x := 0; x < fw; x++ {
			a := int(mask[(y+1+padY)*w+x+1]) // atlas margin 1
			b := int(fv[y*fw+x])
			if a > b+16 || a < b-16 {
				m16++
			}
		}
	}
	return m16
}
func readPgm2(path string) (int, int, []byte) {
	d, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, nil
	}
	str := strings.TrimSpace(string(d))
	if strings.HasPrefix(str, "P5") {
		lines := strings.SplitN(str, "\n", 4)
		var w, h, mv int
		fmt.Sscanf(lines[0], "P5\n%d %d", &w, &h)
		fmt.Sscanf(lines[2], "%d", &mv)
		return w, h, []byte(lines[3])
	}
	toks := strings.Fields(str)
	var w, h, mv int
	fmt.Sscanf(toks[1], "%d", &w)
	fmt.Sscanf(toks[2], "%d", &h)
	fmt.Sscanf(toks[3], "%d", &mv)
	vals := make([]byte, w*h)
	for i, tok := range toks[4:] {
		v, _ := strconv.Atoi(tok)
		vals[i] = byte(v)
	}
	return w, h, vals
}

func mustRead(path string) []byte {
	d, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return d
}

func bestShiftMism(sv []byte, sw, sh int, fv []byte, fw, fh int) int {
	w := fw
	if sw < w {
		w = sw
	}
	h := fh
	if sh < h {
		h = sh
	}
	best := 1 << 30
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			m := 0
			for y := 0; y < h; y++ {
				sy := y + dy
				for x := 0; x < w; x++ {
					sx := x + dx
					if sy < 0 || sy >= sh || sx < 0 || sx >= sw {
						continue
					}
					d := int(sv[sy*sw+sx]) - int(fv[y*fw+x])
					if d < 0 {
						d = -d
					}
					if d > 0 {
						m++
					}
				}
			}
			if m < best {
				best = m
			}
		}
	}
	return best
}
