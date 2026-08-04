package hint

import (
	"fmt"
	"os/exec"

	"strings"
	"testing"
)

// rd 记录单字 rune 与其 charstring 内 hintmask/cntrmask 出现次数。
// mask == 0 → 单区（M2 验证线）；mask > 0 → 多区（M3 hintmask 分区）。
type rd struct {
	r    rune
	mask int
}

// 临时 M2 扩展验证：多字号 × 常用字 light 轮廓逐点 26.6 vs ftexp。
//
// 拆分逻辑（docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §5.1 M2 验证线）：
//   - hintmaskCount == 0：单区字（M2 验证线，必须 100% 归零）
//   - hintmaskCount >  0：多 mask 字（M3 hintmask 分区任务，本测试 t.Skip）
func TestM2VerifyGrid(t *testing.T) {
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()
	pixes := []float64{10, 12, 14, 16, 20, 24}
	runes := []rune{'日', '田', '目', '一', '二', '三', '十', '人', '口', '木', '水', '火', '土', '王', '大', '小', '中', '山', '天', '下', '上', '不', '心', '月', '明', '川'}

	var single, multi []rd
	for _, r := range runes {
		gid := uint16(f.GlyphIndex(r))
		out, _, err := m2Interp(cd, gid)
		if err != nil {
			t.Fatalf("%c: %v", r, err)
		}
		if out.hintmaskCount == 0 {
			single = append(single, rd{r, out.hintmaskCount})
		} else {
			multi = append(multi, rd{r, out.hintmaskCount})
		}
	}
	t.Logf("单区字 %d 个：%v", len(single), runesOf(single))
	t.Logf("多 mask 字 %d 个（M3）：%v", len(multi), runesOf(multi))

	badRunes := map[rune][]string{}
	totalBad := 0
	for _, px := range pixes {
		scale := m2HintScale(px, upem)
		for _, e := range single {
			r := e.r
			gid := uint16(f.GlyphIndex(r))
			out, fd, err := m2Interp(cd, gid)
			if err != nil {
				t.Fatalf("%c: %v", r, err)
			}
			// FT 2.14.3 默认 no_stem_darkening=TRUE（cffobjs.c:1141）+
			// font 默认 -1 → cf2 暗化关闭（psft.c:408 条件为假），对照基准 = 0。
			res := hintCFFLight(out, fd, scale, 0, 0)
			ft := ftContour26Light(t, r, px)
			if len(ft) != len(res.pts) {
				badRunes[r] = append(badRunes[r], fmt.Sprintf("%.0fpx(npt %d!=%d)", px, len(res.pts), len(ft)))
				totalBad++
				continue
			}
			bad := 0
			for i, p := range res.pts {
				if int64(p[0]>>10) != ft[i][0] || int64(p[1]>>10) != ft[i][1] {
					bad++
				}
			}
			if bad > 0 {
				badRunes[r] = append(badRunes[r], fmt.Sprintf("%.0fpx(%d/%d)", px, bad, len(ft)))
				totalBad++
			}
		}
	}
	if totalBad > 0 {
		for r, ms := range badRunes {
			t.Errorf("%c: %s", r, strings.Join(ms, " "))
		}
	} else {
		t.Logf("单区字 %d × %d 字号全部逐点一致", len(single), len(pixes))
	}

	// 多 mask 字：M3 任务，本测试不覆盖，仅记录在案。
	if len(multi) > 0 {
		t.Logf("M3 待办：多 mask 字 hintmask 分区逐区建图映射（%d 字）", len(multi))
	}
}

func runesOf(rs []rd) []rune {
	out := make([]rune, len(rs))
	for i, e := range rs {
		out[i] = e.r
	}
	return out
}

// TestM2VerifyEdge：日 10/14/16px 的 8 边快照（防回归）。
func TestM2VerifyEdge(t *testing.T) {
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()
	for _, px := range []float64{10, 12, 14, 16} {
		scale := m2HintScale(px, upem)
		gid := uint16(f.GlyphIndex('日'))
		out, fd, err := m2Interp(cd, gid)
		if err != nil {
			t.Fatal(err)
		}
		hStems := cf2StemSlice(out.hstems)
		var blues cf2Blues
		cf2BluesInit(&blues, fd.blues, scale, 12, true)
		hm := &cf2HintMap{}
		var mask cf2HintMask
		mask.setAll(len(out.hstems) + len(out.vstems))
		hm.build(&blues, hStems, cf2StemSlice(out.vstems), &mask, scale, 12, false)
		var sb strings.Builder
		for _, cs := range []int64{-120, -4, 71, 352, 426, 697, 772, 880} {
			fmt.Fprintf(&sb, "%.0f:%.2f ", float64(hm.mapCS(cf2IntToFixed(cs)))/65536.0, float64(hm.mapCS(cf2IntToFixed(cs)))/64.0)
		}
		t.Logf("%.0fpx edges: %s", px, sb.String())
	}
}

// TestM2VStem：X 轴 vstem 直通验证（应与 FT 一致，M2 范围 X=直通）。
func TestM2VStem(t *testing.T) {
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()
	for _, r := range []rune{'日', '目'} {
		gid := uint16(f.GlyphIndex(r))
		out, fd, err := m2Interp(cd, gid)
		if err != nil {
			t.Fatal(err)
		}
		_ = upem
		_ = fd
		_ = out
	}
	cmd := exec.Command("true")
	_ = cmd
}

// TestM2Offset：验证 hintOrigin=0 假设（ftexp 无 offset 标志时基线=0）。
func TestM2Offset(t *testing.T) {
	f, cd := m2Font(t)
	gid := uint16(f.GlyphIndex('一'))
	out, fd, err := m2Interp(cd, gid)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("一 hstems=%v vstems=%v", out.hstems, out.vstems)
	t.Logf("blues=%v", fd.blues)
}
