package render

import (
	"fmt"
	"math"
	"testing"
)

// 值级测试：Path 造型三 API（Trim / WithCorners / Discrete）与 API 注释契约逐项对齐。
// 背景（2026-10-19，A 组第 2 项）：Discrete 此前实测「偏移恒单侧 + 虚线周期 2×segLen」，
// 与注释不符；现按注释 + Skia 切段口径修正，本文件锁死契约，防回归。

// edgeSeg 是路径中的一段直线（MoveTo/LineTo 产出）。
type edgeSeg struct{ x0, y0, x1, y1 float64 }

func pathEdges(p *Path) []edgeSeg {
	var out []edgeSeg
	cs := p.Coords()
	ci := 0
	var cx, cy float64
	has := false
	for _, v := range p.Verbs() {
		switch v {
		case MoveTo:
			cx, cy = cs[ci], cs[ci+1]
			ci += 2
			has = true
		case LineTo:
			x, y := cs[ci], cs[ci+1]
			ci += 2
			if has {
				out = append(out, edgeSeg{cx, cy, x, y})
			}
			cx, cy = x, y
			has = true
		case QuadTo:
			ci += 4
		case CubicTo:
			ci += 6
		case Close:
		}
	}
	return out
}

func edgeLen(s edgeSeg) float64 { return math.Hypot(s.x1-s.x0, s.y1-s.y0) }

func polylineLen(pts []Point) float64 {
	var l float64
	for i := 1; i < len(pts); i++ {
		l += pts[i-1].Distance(pts[i])
	}
	return l
}

func pathCoordBounds(p *Path) (minX, minY, maxX, maxY float64) {
	cs := p.Coords()
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for i := 0; i+1 < len(cs); i += 2 {
		minX = math.Min(minX, cs[i])
		maxX = math.Max(maxX, cs[i])
		minY = math.Min(minY, cs[i+1])
		maxY = math.Max(maxY, cs[i+1])
	}
	return
}

// Trim：弧长子段长度/端点精确、结果必是开放折线、反向与空输入退化为空。
func TestPathTrim_SubsetContract(t *testing.T) {
	dia := NewPath()
	dia.MoveTo(140, 57)
	dia.LineTo(225, 132)
	dia.LineTo(140, 207)
	dia.LineTo(55, 132)
	dia.Close()

	baseLen := polylineLen(dia.Flatten(0.25))
	tr := dia.Trim(0.25, 0.6)
	edges := pathEdges(tr)
	if len(edges) == 0 {
		t.Fatal("Trim produced no segments")
	}
	var got float64
	for _, e := range edges {
		got += edgeLen(e)
	}
	if want := baseLen * 0.35; math.Abs(got-want) > baseLen*0.005 {
		t.Fatalf("trimmed length = %.4f, want ~%.4f (0.35 x %.4f)", got, want, baseLen)
	}
	for _, v := range tr.Verbs() {
		if v != MoveTo && v != LineTo {
			t.Fatalf("Trim must return an open polyline, got verb %v", v)
		}
	}
	first, last := edges[0], edges[len(edges)-1]
	if math.Hypot(first.x0-225, first.y0-132) > 0.05 {
		t.Fatalf("trim start = (%.3f,%.3f), want diamond right vertex (225,132)", first.x0, first.y0)
	}
	// 0.6 弧长落在底左边 40% 处（B(140,207) + 0.4*(L(55,132)-B) = (106,177)）
	if math.Hypot(last.x1-106, last.y1-177) > 0.05 {
		t.Fatalf("trim end = (%.3f,%.3f), want ~(106,177)", last.x1, last.y1)
	}
	if n := len(dia.Trim(0.7, 0.3).Verbs()); n != 0 {
		t.Fatalf("inverted Trim verbs=%d, want 0", n)
	}
	if n := len(NewPath().Trim(0, 1).Verbs()); n != 0 {
		t.Fatalf("empty path Trim verbs=%d, want 0", n)
	}
	// 曲线按 0.1px 级展平：半圆长度 ≈ 周长一半
	circ := NewPath()
	circ.Circle(140, 132, 60)
	circLen := polylineLen(circ.Flatten(0.25))
	if got := polylineLen(circ.Trim(0, 0.5).Flatten(0.25)); math.Abs(got-circLen/2) > circLen*0.01 {
		t.Fatalf("half-circle Trim len=%.3f, want ~%.3f", got, circLen/2)
	}
}

// WithCorners：闭合路径全部角圆化、开放路径首尾保持尖角、r<=0 等价克隆、超大半径夹紧。
func TestPathWithCorners_Contract(t *testing.T) {
	verts := [][2]float64{{60, 70}, {220, 70}, {220, 130}, {140, 130}, {140, 190}, {60, 190}}
	l := NewPath()
	l.MoveTo(verts[0][0], verts[0][1])
	for _, v := range verts[1:] {
		l.LineTo(v[0], v[1])
	}
	l.Close()

	rc := l.WithCorners(20)
	quads := 0
	for _, v := range rc.Verbs() {
		if v == QuadTo {
			quads++
		}
	}
	if quads != 6 {
		t.Fatalf("closed hexagon: quad verbs=%d, want 6 (all corners)", quads)
	}
	if minX, minY, maxX, maxY := pathCoordBounds(rc); minX < 60-1e-6 || minY < 70-1e-6 || maxX > 220+1e-6 || maxY > 190+1e-6 {
		t.Fatalf("rounded path escaped bounds: (%.2f,%.2f)-(%.2f,%.2f)", minX, minY, maxX, maxY)
	}

	open := NewPath()
	open.MoveTo(verts[0][0], verts[0][1])
	for _, v := range verts[1:] {
		open.LineTo(v[0], v[1])
	}
	ro := open.WithCorners(20)
	csv := ro.Coords()
	if csv[0] != 60 || csv[1] != 70 || csv[len(csv)-2] != 60 || csv[len(csv)-1] != 190 {
		t.Fatalf("open polyline ends moved: first=(%.3f,%.3f) last=(%.3f,%.3f), want (60,70)/(60,190)",
			csv[0], csv[1], csv[len(csv)-2], csv[len(csv)-1])
	}
	quads = 0
	for _, v := range ro.Verbs() {
		if v == QuadTo {
			quads++
		}
	}
	if quads != 4 {
		t.Fatalf("open polyline: quad verbs=%d, want 4 (middle corners only)", quads)
	}

	r0 := l.WithCorners(0)
	r0cs := r0.Coords()
	if len(r0cs) != len(verts)*2 {
		t.Fatalf("r=0 coords=%d, want %d (clone of the polyline)", len(r0cs), len(verts)*2)
	}
	for i, v := range verts {
		if r0cs[2*i] != v[0] || r0cs[2*i+1] != v[1] {
			t.Fatalf("r=0 point %d = (%.3f,%.3f), want (%.1f,%.1f)", i, r0cs[2*i], r0cs[2*i+1], v[0], v[1])
		}
	}

	sq := NewPath()
	sq.MoveTo(0, 0)
	sq.LineTo(20, 0)
	sq.LineTo(20, 20)
	sq.LineTo(0, 20)
	sq.Close()
	if minX, minY, maxX, maxY := pathCoordBounds(sq.WithCorners(1000)); minX < -1e-6 || minY < -1e-6 || maxX > 20+1e-6 || maxY > 20+1e-6 {
		t.Fatalf("huge radius escaped clamp: (%.2f,%.2f)-(%.2f,%.2f)", minX, minY, maxX, maxY)
	}
}

// Discrete：虚线周期 = segLen（前半画/后半空），±deviation 两侧交替且确定性。
func TestPathDiscrete_PeriodAndTwoSidedJitter(t *testing.T) {
	line := NewPath()
	line.MoveTo(0, 0)
	line.LineTo(300, 0)

	dash := line.Discrete(22, 0)
	dEdges := pathEdges(dash)
	if len(dEdges) < 12 {
		t.Fatalf("dash pieces=%d, want >=12 (300px at period 22)", len(dEdges))
	}
	var drawn float64
	for i, e := range dEdges {
		if e.y0 != 0 || e.y1 != 0 {
			t.Fatalf("deviation=0 must stay on the path: piece %d y=(%.3f,%.3f)", i, e.y0, e.y1)
		}
		if l := edgeLen(e); math.Abs(l-11) > 0.01 {
			t.Fatalf("dash piece %d len=%.3f, want 11 (segLen/2)", i, l)
		}
		drawn += edgeLen(e)
	}
	if r := drawn / 300; r < 0.48 || r > 0.55 {
		t.Fatalf("dash duty=%.3f, want ~0.5 (half of each segLen period drawn)", r)
	}
	for i := 1; i < len(dEdges); i++ {
		if gap := dEdges[i].x0 - dEdges[i-1].x1; math.Abs(gap-11) > 0.01 {
			t.Fatalf("dash gap %d=%.3f, want 11 (segLen/2)", i, gap)
		}
	}

	jit := line.Discrete(22, 7)
	jEdges := pathEdges(jit)
	if len(jEdges) != len(dEdges) {
		t.Fatalf("jitter pieces=%d, want %d (same chopping as dash)", len(jEdges), len(dEdges))
	}
	pos, neg := 0, 0
	for i, e := range jEdges {
		if math.Abs(math.Abs(e.y0)-7) > 1e-9 || math.Abs(e.y0-e.y1) > 1e-9 {
			t.Fatalf("jitter piece %d offset=(%.3f,%.3f), want a straight +-7 offset", i, e.y0, e.y1)
		}
		if e.y0 > 0 {
			pos++
		} else {
			neg++
		}
		if i > 0 && (jEdges[i-1].y0 > 0) == (e.y0 > 0) {
			t.Fatalf("jitter pieces %d/%d both at y=%.1f: sides must alternate", i-1, i, e.y0)
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("jitter one-sided: +%d/-%d, want both sides", pos, neg)
	}
	if d := pos - neg; d > 1 || d < -1 {
		t.Fatalf("jitter side counts +%d/-%d, want balanced", pos, neg)
	}

	if a, b := fmt.Sprint(dash.Coords()), fmt.Sprint(line.Discrete(22, 0).Coords()); a != b {
		t.Fatalf("Discrete(deviation=0) is not deterministic")
	}
	if a, b := fmt.Sprint(jit.Coords()), fmt.Sprint(line.Discrete(22, 7).Coords()); a != b {
		t.Fatalf("Discrete(deviation!=0) is not deterministic")
	}

	if n := len(NewPath().Discrete(10, 5).Verbs()); n != 0 {
		t.Fatalf("empty path Discrete verbs=%d, want 0", n)
	}
	one := NewPath()
	one.MoveTo(5, 5)
	if n := len(one.Discrete(10, 5).Verbs()); n != 0 {
		t.Fatalf("single-point Discrete verbs=%d, want 0", n)
	}
	if n := len(pathEdges(line.Discrete(0, 0))); n == 0 {
		t.Fatalf("segLen<=0 clamp produced no geometry")
	}
}
