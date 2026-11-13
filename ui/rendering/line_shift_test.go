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
