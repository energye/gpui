package text

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestDbgProdLightPixels 生产路径（ExtractOutlineHinted→RasterizeHinted）
// vs ftexp FT-light 位图逐像素对照。与 fdiff（简化引擎）区分：验证真窗
// 实际走的 light 链。含 8px。
func TestDbgProdLightPixels(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	runes := "静每合"
	pxes := []float64{8, 10, 12, 14, 16, 20, 24, 28, 32, 48}
	type diffInfo struct {
		fontPath, r, px, hint string
		out                   string
	}
	for _, px := range pxes {
		for _, r := range []rune(runes) {
			gid := GlyphID(parsed.GlyphIndex(r))
			ext := NewOutlineExtractor()
			o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
			if err != nil || o == nil {
				t.Fatalf("%c %0.fpx outline: %v", r, px, err)
			}
			ras := NewGlyphMaskRasterizer()
			res, err := ras.RasterizeOutline(o, 0, 0)
			if err != nil || res == nil {
				t.Fatalf("%c %0.fpx raster: %v", r, px, err)
			}
			selfPath := filepath.Join(t.TempDir(), fmt.Sprintf("self_%d_%d.pgm", r, int(px)))
			ftPath := filepath.Join(t.TempDir(), fmt.Sprintf("ft_%d_%d.pgm", r, int(px)))
			writeP5(selfPath, res.Width, res.Height, res.Mask)
			if px == 16 {
				cp, _ := os.MkdirTemp("", "dbg16-")
				writeP5(filepath.Join(cp, fmt.Sprintf("self_%d_16.pgm", r)), res.Width, res.Height, res.Mask)
				t.Logf("DUMP16 %c -> %s", r, filepath.Join(cp, fmt.Sprintf("self_%d_16.pgm", r)))
				cmd2 := exec.Command(ftexpLocal(t), fontPath, string(r), "16", "l", filepath.Join(cp, fmt.Sprintf("ft_%d_16.pgm", r)))
				out, _ := cmd2.CombinedOutput()
				t.Logf("ft dump: %s", out)
			}
			ftexp := ftexpLocal(t)
			cmd := exec.Command(ftexp, fontPath, string(r), strconv.Itoa(int(px)), "l", ftPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ftexp: %v %s", err, out)
			}
			diffP5(selfPath, ftPath, r, px)
		}
	}
}

var ftexpLocalOnce sync.Once
var ftexpLocalBin string
var ftexpLocalErr error

// ftexpLocal 返回 ftexp 度量衡二进制：$FTEXP_BIN → 仓库内产物 → 拷贝源码到
// t.TempDir() 自动 go build（AGENTS.md：不提交二进制，/tmp 只允许 TempDir）。
func ftexpLocal(t *testing.T) string {
	t.Helper()
	ftexpLocalOnce.Do(func() {
		if p := os.Getenv("FTEXP_BIN"); p != "" {
			ftexpLocalBin = p
			return
		}
		srcDir := "hint/testdata/ftexp"
		if st, err := os.Stat(filepath.Join(srcDir, "ftexp")); err == nil && !st.IsDir() {
			ftexpLocalBin = filepath.Join(srcDir, "ftexp")
			return
		}
		dir, err := os.MkdirTemp("", "ftexp-build-")
		if err != nil {
			ftexpLocalErr = err
			return
		}
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			ftexpLocalErr = err
			return
		}
		for _, en := range entries {
			if en.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(srcDir, en.Name()))
			if err != nil {
				ftexpLocalErr = err
				return
			}
			if err := os.WriteFile(filepath.Join(dir, en.Name()), data, 0o644); err != nil {
				ftexpLocalErr = err
				return
			}
		}
		cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "ftexp"), ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			ftexpLocalErr = fmt.Errorf("go build: %v %s", err, out)
			return
		}
		ftexpLocalBin = filepath.Join(dir, "ftexp")
	})
	if ftexpLocalErr != nil {
		t.Fatalf("ftexp build: %v", ftexpLocalErr)
	}
	return ftexpLocalBin
}

func writeP5(path string, w, h int, mask []byte) {
	var b strings.Builder
	fmt.Fprintf(&b, "P5\n%d %d\n255\n", w, h)
	b.Write(mask)
	os.WriteFile(path, []byte(b.String()), 0o644)
}

func readP5(path string) (int, int, []byte, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, nil, err
	}
	str := strings.TrimSpace(string(d))
	if strings.HasPrefix(str, "P5") {
		lines := strings.SplitN(str, "\n", 4)
		if len(lines) < 4 {
			return 0, 0, nil, fmt.Errorf("bad P5 header")
		}
		var w, h, mv int
		fmt.Sscanf(lines[0], "P5\n%d %d", &w, &h)
		if w == 0 || h == 0 {
			fmt.Sscanf(lines[1], "%d %d", &w, &h)
		}
		fmt.Sscanf(lines[2], "%d", &mv)
		if mv != 255 {
			return 0, 0, nil, fmt.Errorf("maxval %d", mv)
		}
		return w, h, []byte(lines[3]), nil
	}
	toks := strings.Fields(str)
	if len(toks) < 4 || toks[0] != "P2" {
		return 0, 0, nil, fmt.Errorf("bad header %q", toks)
	}
	w, err := strconv.Atoi(toks[1])
	if err != nil {
		return 0, 0, nil, err
	}
	h, err := strconv.Atoi(toks[2])
	if err != nil {
		return 0, 0, nil, err
	}
	if len(toks) != 4+w*h {
		return 0, 0, nil, fmt.Errorf("P2 len %d want %d", len(toks), 4+w*h)
	}
	vals := make([]byte, w*h)
	for i, tok := range toks[4:] {
		v, err := strconv.Atoi(tok)
		if err != nil {
			return 0, 0, nil, err
		}
		vals[i] = byte(v)
	}
	return w, h, vals, nil
}

func diffP5(self, ft string, rg rune, px float64) {
	sw, sh, sv, err1 := readP5(self)
	fw, fh, fv, err2 := readP5(ft)
	if err1 != nil || err2 != nil {
		fmt.Printf("read err %c %.0fpx: %v %v\n", rg, px, err1, err2)
		return
	}
	// self 有 1px AA margin：通过最优 shift（-3..3）吸收后与 FT 重叠区比较。
	w := fw
	if sw < w {
		w = sw
	}
	h := fh
	if sh < h {
		h = sh
	}
	// 最优 shift 搜索（-3..3）后基于重叠区计算 mism/ink。
	best := 1 << 30
	bestDX, bestDY := 0, 0
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
					sv_ := sv[sy*sw+sx]
					fv_ := fv[y*fw+x]
					d := int(sv_) - int(fv_)
					if d < 0 {
						d = -d
					}
					if d > 0 {
						m++
					}
				}
			}
			if m < best {
				best, bestDX, bestDY = m, dx, dy
			}
		}
	}
	mism := 0
	mism16 := 0
	inkS, inkF := 0, 0
	n := 0
	for y := 0; y < h; y++ {
		sy := y + bestDY
		for x := 0; x < w; x++ {
			sx := x + bestDX
			if sy < 0 || sy >= sh || sx < 0 || sx >= sw {
				continue
			}
			a := sv[sy*sw+sx]
			b := fv[y*fw+x]
			inkS += int(a)
			inkF += int(b)
			n++
			da := int(a) - int(b)
			if da < 0 {
				da = -da
			}
			if da > 16 {
				mism16++
			}
			if da > 0 {
				mism++
			}
		}
	}
	fmt.Printf("%q %.0fpx: mism=%d/%d mism16=%d ink self=%d ft=%d bestShift=(%+d,%+d)\n",
		rg, px, mism, n, mism16, inkS, inkF, bestDX, bestDY)
}