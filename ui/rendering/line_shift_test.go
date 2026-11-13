package rendering

import "testing"

func TestRowShift_SuffixAdd(t *testing.T) {
	s := newRowShift(5)
	if got := s.at(0); got != 0 {
		t.Fatalf("初始第0行平移应为0, got %d", got)
	}
	s.addSuffix(2, 3)
	for i, want := range []int{0, 0, 3, 3, 3} {
		if got := s.at(i); got != want {
			t.Fatalf("addSuffix(2,3) 后第%d行=%d, want %d", i, got, want)
		}
	}
	s.addSuffix(0, -1)
	for i, want := range []int{-1, -1, 2, 2, 2} {
		if got := s.at(i); got != want {
			t.Fatalf("addSuffix(0,-1) 后第%d行=%d, want %d", i, got, want)
		}
	}
	s.addSuffix(5, 9)
	s.addSuffix(-1, 9)
	s.addSuffix(1, 0)
	for i, want := range []int{-1, -1, 2, 2, 2} {
		if got := s.at(i); got != want {
			t.Fatalf("越界/零增量后第%d行=%d, want %d", i, got, want)
		}
	}
}

func TestRowShift_ClampAndNil(t *testing.T) {
	var s *rowShift
	s.addSuffix(0, 1)
	if got := s.at(3); got != 0 {
		t.Fatalf("nil rowShift 应恒为0, got %d", got)
	}
	if newRowShift(0) != nil {
		t.Fatalf("零行不应分配增量树")
	}
	r := newRowShift(2)
	r.addSuffix(0, 7)
	if got := r.at(-5); got != 7 {
		t.Fatalf("负索引应钳到首行, got %d", got)
	}
	if got := r.at(99); got != 7 {
		t.Fatalf("越界索引应钳到末行, got %d", got)
	}
}

func TestRowShift_Reshape(t *testing.T) {
	check := func(label string, r *rowShift, want []int) {
		t.Helper()
		if r == nil {
			t.Fatalf("%s: reshape返回nil", label)
		}
		for i, w := range want {
			if got := r.at(i); got != w {
				t.Fatalf("%s 第%d行=%d want %d", label, i, got, w)
			}
		}
	}
	s := newRowShift(5)
	s.addSuffix(2, 3) // [0,0,3,3,3]
	// 替换第2行为1条新行,文本+1:新行shift=3,旧3/4行(3)→4.
	check("insert1", s.reshape(2, 2, 1, 5, 1), []int{0, 0, 3, 4, 4})

	s2 := newRowShift(5)
	s2.addSuffix(2, 3)
	s2.addSuffix(4, 1) // [0,0,3,3,4]
	// 删第2/3行,文本-2:旧4行(4)成为新2行,shift=4-2=2.
	check("delete2", s2.reshape(2, 3, 0, 3, -2), []int{0, 0, 2})

	s3 := newRowShift(5)
	s3.addSuffix(2, 3)
	s3.addSuffix(4, 1) // [0,0,3,3,4]
	// 替换第2行为1条新行,文本+1:新2=3,旧3(3)→4,旧4(4)→5.
	check("insertWithTailEvent", s3.reshape(2, 2, 1, 5, 1), []int{0, 0, 3, 4, 5})
}
