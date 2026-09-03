package text

import (
	"reflect"
	"testing"
)

// M1-c红灯:UAX#29字素簇起点(字节偏移,末尾哨兵=串长).
// 注意:必须用e+U+0301(3字节),预合成的é(U+00E9)本来就是单字,不算组合.
func TestClusterBoundary_CombiningMark(t *testing.T) {
	if got, want := ClusterStarts("e\u0301"), []int{0, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("组合音标应为一簇,得%v want %v", got, want)
	}
}

func TestClusterBoundary_ZWJ(t *testing.T) {
	s := "👨\u200d👩\u200d👧"
	got := ClusterStarts(s)
	if len(got) != 2 || got[0] != 0 || got[1] != len(s) {
		t.Fatalf("ZWJ家庭应为一簇,得%v", got)
	}
}

func TestClusterBoundary_SurrogatePair(t *testing.T) {
	if got, want := ClusterStarts("😀"), []int{0, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("代理对(4字节)应为一簇,得%v want %v", got, want)
	}
}

func TestClusterBoundary_PlainASCII(t *testing.T) {
	if got, want := ClusterStarts("ab"), []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("纯ASCII逐字成簇,得%v want %v", got, want)
	}
}
