package text

import (
	"github.com/go-text/typesetting/segmenter"
)

// ClusterStarts返回s中字素簇起点的字节偏移,末尾附len(s)哨兵.
// 空串返回[]int{0}.实现为UAX#29(复用内置segmenter,已覆盖GB11表情序列、
// GB12/GB13地区指示符、Extend/ZWJ;wrap.go已依赖该包,零新依赖).
func ClusterStarts(s string) []int {
	if s == "" {
		return []int{0}
	}
	var seg segmenter.Segmenter
	seg.InitWithString(s)
	byteOffs := make([]int, 0, len(s)+1)
	for i := range s {
		byteOffs = append(byteOffs, i)
	}
	byteOffs = append(byteOffs, len(s))
	out := make([]int, 0, len(byteOffs))
	it := seg.GraphemeIterator()
	for it.Next() {
		g := it.Grapheme()
		if g.Offset < 0 || g.Offset >= len(byteOffs)-1 {
			continue
		}
		out = append(out, byteOffs[g.Offset])
	}
	out = append(out, len(s))
	return out
}

// SnapCluster把off吸附到所在簇边界:已在边界保持不动;
// 在簇内时downstream取簇首,upstream取簇尾.
func SnapCluster(s string, off int, downstream bool) int {
	if off <= 0 || off >= len(s) {
		return off
	}
	return SnapInStarts(ClusterStarts(s), off, downstream)
}

// SnapInStarts在已算好的簇起点表上吸附off(供调用方复用缓存过的起点表,
// 免重复切分;语义与SnapCluster一致,唯一真源).
func SnapInStarts(starts []int, off int, downstream bool) int {
	if len(starts) == 0 {
		return off
	}
	prev := starts[0]
	for _, st := range starts {
		if st >= off {
			if st == off {
				return off
			}
			if downstream {
				return prev
			}
			return st
		}
		prev = st
	}
	return off
}
