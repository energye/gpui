package rendering

// 挂账清偿（2026-11-14）：G1 收口（rowShift 后缀增量重构）的未验证项
// 「真字体（face!=nil）插入路径未单测」——此前 line_shift 全部等价性
// 测试都是 nil face 估算口径。本文件把同一套编辑序列在真字体下重跑：
// 增量路径（layoutCache.update，face!=nil）与全量构建（BuildTextLayout，
// face!=nil）逐查询口对照。shift 派生绝对偏移与 face 无关（tail 平移），
// 但插入后的行内重整（row_reuse/整形缓存）在真字体下走真实整形链，
// 这里锁的是「真字体增量 == 真字体全量」。

import (
	"testing"
)

func TestV5LineShift_RealFace_EquivQueries(t *testing.T) {
	face := m5DejaVuFace(t)
	for _, w := range []float64{0, 160, 360} {
		c := newLayoutCache()
		cur := "alpha beta gamma\ndelta epsilon zeta\neta theta iota kappa\nlambda mu"
		c.update(cur, face, 14, w, 1.2, 0.55, 0, TextOverflowClip)
		type edit struct {
			pos, del int
			ins      string
		}
		edits := []edit{
			{5, 0, "X"},                 // 行内插入
			{1, 0, "Y"},                 // 行首插入
			{0, 1, ""},                  // 删除
			{10, 0, "\n"},               // 行数变化（拆行）
			{10, 1, ""},                 // 行数变化（并回）
			{len(cur) / 2, 2, "ZZZ"},    // 中段替换
			{0, 0, "\n\n"},              // 前缀换行
			{len(cur) - 1, 0, "\nTAIL"}, // 尾部换行
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
			got := c.update(cur, face, 14, w, 1.2, 0.55, 0, TextOverflowClip)
			want := BuildTextLayout(cur, face, 14, w, 1.2)
			m5ShiftCompare(t, got, want, "realface")
			if k == 0 && (got.shift == nil || got.shift.at(got.LineCount()-1) == 0) {
				t.Fatalf("realface 首步插入未生成后缀增量,未覆盖增量路径")
			}
		}
	}
}

func TestV5LineShift_RealFace_Compaction(t *testing.T) {
	face := m5DejaVuFace(t)
	c := newLayoutCache()
	cur := m1ManyLinesDoc(100)
	c.update(cur, face, 14, 120, 1.2, 0.55, 0, TextOverflowClip)

	step := func(pos, del int, ins, label string, wantShift bool) {
		t.Helper()
		pos, del = m5ShiftClamp(pos, del, len(cur))
		cur = cur[:pos] + ins + cur[pos+del:]
		got := c.update(cur, face, 14, 120, 1.2, 0.55, 0, TextOverflowClip)
		want := BuildTextLayout(cur, face, 14, 120, 1.2)
		m5ShiftCompare(t, got, want, label)
		if wantShift && (got.shift == nil || got.shift.at(got.LineCount()-1) == 0) {
			t.Fatalf("%s 未生成后缀增量,未覆盖增量路径", label)
		}
	}
	pos10 := 10 * len("line content here\n")
	step(pos10+3, 0, "X", "row10-insert", true)
	nl10 := 10*len("line content here\n") + len("line content here")
	step(nl10, 1, "", "row10-merge-compact", false)
	step(50, 0, "\n", "row-insert-newline", false)
	step(200, 40, "", "span-delete", false)
	step(0, 0, "PREFIX\n", "prefix-insert", false)
}
