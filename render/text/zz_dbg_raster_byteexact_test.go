package text

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 阶段 A1 基线：FT 位图（bitmap_left/top 精确对齐）vs 自研光栅逐字节对照。
// 同一 FT 26.6 轮廓 → 自研光栅 与 FT_Render_Glyph 位图，按 FT left/top 映射
// 到字形坐标逐字节比较（含 0 值）。光栅器对齐 ftgrays 后该探针归零，
// 再转为正式 TestRasterFTByteExact。
func TestDbgRasterByteExact(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	for _, px := range []float64{12, 16, 24, 32, 48} {
		for _, r := range []rune("静每合日田") {
			_ = GlyphID(parsed.GlyphIndex(r))

			// 1) FT 位图 + left/top（ftexp stdout OK 行解析）
			ftPgm := filepath.Join(t.TempDir(), "ft.pgm")
			cmd := exec.Command(ftexpLocal(t), fontPath, string(r), strconv.Itoa(int(px)), "l", ftPgm)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ftexp: %v %s", err, out)
			}
			var left, top int
			if _, err := fmt.Sscanf(string(out), "", &left, &top); err != nil {
				// OK 行: ... left=%d top=%d ...
				for _, f := range strings.Fields(string(out)) {
					if v, err := strconv.Atoi(strings.TrimPrefix(f, "left=")); err == nil && strings.HasPrefix(f, "left=") {
						left = v
					}
					if v, err := strconv.Atoi(strings.TrimPrefix(f, "top=")); err == nil && strings.HasPrefix(f, "top=") {
						top = v
					}
				}
			}
			fw, fh, fv := readPgm2(ftPgm)
			if len(fv) == 0 {
				t.Fatalf("%c %.0fpx: no FT bitmap", r, px)
			}

			// 2) 同一 FT 26.6 轮廓 → 自研光栅
			ftOutline := ftOutlineToGlyphSimpler(t, fontPath, r, px)
			ras := NewGlyphMaskRasterizer()
			res, err := ras.RasterizeOutline(ftOutline, 0, 0)
			if err != nil || res == nil {
				t.Fatalf("%c %.0fpx: raster: %v", r, px, err)
			}

			// 3) 映射：FT(fx,fy) → 自研(mx,my)
			//   x: BearingX + mx = left + fx           → mx = left + fx - BearingX
			//   y: BearingY - my = top - fy            → my = BearingY - top + fy
			bad := 0
			total := 0
			for fy := 0; fy < fh; fy++ {
				my := int(res.BearingY) - top + fy
				if my < 0 || my >= res.Height {
					bad += fw
					total += fw
					continue
				}
				for fx := 0; fx < fw; fx++ {
					mx := left + fx - int(res.BearingX)
					total++
					if mx < 0 || mx >= res.Width {
						bad++
						continue
					}
					a := int(fv[fy*fw+fx])
					b := int(res.Mask[my*res.Width+mx])
					if a != b {
						bad++
					}
				}
			}
			fmt.Printf("%c %.0fpx: FT位图 %dx%d left=%d top=%d | 自研 %dx%d bx=%.0f by=%.0f | 逐字节 bad=%d/%d\n",
				r, px, fw, fh, left, top, res.Width, res.Height, res.BearingX, res.BearingY, bad, total)
		}
	}
}

var _ = os.Stat
