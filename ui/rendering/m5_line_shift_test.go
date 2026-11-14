package rendering

import (
	"math"
	"testing"
)

func m5ShiftEqualInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func m5ShiftCompare(t *testing.T, got, want *TextLayout, label string) {
	t.Helper()
	if got.LineCount() != want.LineCount() {
		t.Fatalf("%s 行数 got %d want %d", label, got.LineCount(), want.LineCount())
	}
	for i := 0; i < want.LineCount(); i++ {
		gs, ge, gw, gh, gok := got.Line(i)
		ws, we, ww, wh, wok := want.Line(i)
		if !gok || !wok || gs != ws || ge != we || gw != ww || gh != wh {
			t.Fatalf("%s 第%d行 got [%d,%d) w=%.3f h=%.3f want [%d,%d) w=%.3f h=%.3f",
				label, i, gs, ge, gw, gh, ws, we, ww, wh)
		}
		if !m5ShiftEqualInts(got.LineClusterStarts(i), want.LineClusterStarts(i)) {
			t.Fatalf("%s 第%d行簇起点不一致", label, i)
		}
	}
	for off := 0; off <= len(want.Text); off++ {
		gi, gx, gok := got.CaretForOffset(off)
		wi, wx, wok := want.CaretForOffset(off)
		if gok != wok || (gok && (gi != wi || math.Abs(gx-wx) > 1e-9)) {
			t.Fatalf("%s CaretForOffset(%d) got (%d,%.4f,%v) want (%d,%.4f,%v)", label, off, gi, gx, gok, wi, wx, wok)
		}
		gox, goy, goh, gook := got.GetOffsetForCaret(off, AffinityDownstream, 1.5)
		wox, woy, woh, wook := want.GetOffsetForCaret(off, AffinityDownstream, 1.5)
		if gook != wook || (gook && (math.Abs(gox-wox) > 1e-9 || math.Abs(goy-woy) > 1e-9 || math.Abs(goh-woh) > 1e-9)) {
			t.Fatalf("%s GetOffsetForCaret(%d) got (%.4f,%.4f,%.4f) want (%.4f,%.4f,%.4f)", label, off, gox, goy, goh, wox, woy, woh)
		}
	}
	gb := got.BoxesForRange(0, len(want.Text))
	wb := want.BoxesForRange(0, len(want.Text))
	if len(gb) != len(wb) {
		t.Fatalf("%s BoxesForRange len got %d want %d", label, len(gb), len(wb))
	}
	for i := range wb {
		// 真字体下增量与全量两条路径的行高浮点运算顺序不同，存在 1e-16 级
		// 固有噪声（46.08749999999999 vs 46.0875）；1e-9 容差对 nil face 的
		// 精确相等判定无影响。
		if math.Abs(gb[i].Min.X-wb[i].Min.X) > 1e-9 || math.Abs(gb[i].Min.Y-wb[i].Min.Y) > 1e-9 ||
			math.Abs(gb[i].Max.X-wb[i].Max.X) > 1e-9 || math.Abs(gb[i].Max.Y-wb[i].Max.Y) > 1e-9 {
			t.Fatalf("%s BoxesForRange[%d] got %+v want %+v", label, i, gb[i], wb[i])
		}
	}
	for i := 0; i < want.LineCount(); i++ {
		_, _, _, h, _ := want.Line(i)
		y := want.LineTop(i) + h/2
		for _, x := range []float64{-1, 0, 4, 30, 1e6} {
			gi, ga := got.GetPositionForOffset(x, y)
			wi, wa := want.GetPositionForOffset(x, y)
			if gi != wi || ga != wa {
				t.Fatalf("%s GetPositionForOffset(%.1f,%.1f) got (%d,%d) want (%d,%d)", label, x, y, gi, ga, wi, wa)
			}
		}
	}
}

func TestLineShift_EquivQueries_M5(t *testing.T) {
	for _, w := range []float64{0, 120, 300} {
		c := newLayoutCache()
		cur := "alpha beta gamma\ndelta epsilon zeta\neta theta iota kappa\nlambda mu"
		c.update(cur, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)
		type edit struct {
			pos, del int
			ins      string
		}
		edits := []edit{
			{0, 0, "X"},
			{1, 0, "Y"},
			{0, 1, ""},
			{10, 0, "\n"},
			{10, 1, ""},
			{len(cur) / 2, 1, "ZZ"},
			{0, 0, "\n\n"},
			{len(cur) - 1, 0, "\nTAIL"},
		}
		for k, e := range edits {
			pos, del := e.pos, e.del
			if pos > len(cur) {
				pos = len(cur)
			}
			if pos+del > len(cur) {
				del = len(cur) - pos
			}
			cur = cur[:pos] + e.ins + cur[pos+del:]
			got := c.update(cur, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)
			want := BuildTextLayout(cur, nil, 14, w, 1.2)
			m5ShiftCompare(t, got, want, "small")
			if k == 0 && (got.shift == nil || got.shift.at(got.LineCount()-1) == 0) {
				t.Fatalf("small 首步插入未生成后缀增量,未覆盖增量路径")
			}
		}
	}
}

// TestLineShift_Compaction_M5覆盖"替换区间内含旧后缀增量事件"与行数变化
// 两条烘焙路径,并逐次与全量构建对照全部查询口.
func TestLineShift_Compaction_M5(t *testing.T) {
	for _, w := range []float64{0, 120} {
		c := newLayoutCache()
		cur := m1ManyLinesDoc(100)
		c.update(cur, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)

		step := func(pos, del int, ins, label string, wantShift bool) {
			t.Helper()
			pos, del = m5ShiftClamp(pos, del, len(cur))
			cur = cur[:pos] + ins + cur[pos+del:]
			got := c.update(cur, nil, 14, w, 1.2, 0.55, 0, TextOverflowClip)
			want := BuildTextLayout(cur, nil, 14, w, 1.2)
			m5ShiftCompare(t, got, want, label)
			if wantShift && (got.shift == nil || got.shift.at(got.LineCount()-1) == 0) {
				t.Fatalf("%s 未生成后缀增量,未覆盖增量路径", label)
			}
		}
		// 第10行内插字(留下一条行后缀增量事件).
		pos10 := 10 * len("line content here\n")
		step(pos10+3, 0, "X", "row10-insert", true)
		// 删除第10行行尾换行:替换区间[10,11]包含上一步在行11上的事件,触发烘焙.
		nl10 := 10*len("line content here\n") + len("line content here")
		step(nl10, 1, "", "row10-merge-compact", false)
		// 行数变化:插入换行.
		step(50, 0, "\n", "row-insert-newline", false)
		// 跨行删除(含换行),行数再变.
		step(200, 40, "", "span-delete", false)
		// 前缀插入.
		step(0, 0, "PREFIX\n", "prefix-insert", false)
	}
}

func m5ShiftClamp(pos, del, n int) (int, int) {
	if pos < 0 {
		pos = 0
	}
	if pos > n {
		pos = n
	}
	if del < 0 {
		del = 0
	}
	if pos+del > n {
		del = n - pos
	}
	return pos, del
}
