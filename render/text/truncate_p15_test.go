package text

import (
	"os"
	"testing"
	"unicode/utf8"
)

// P1-5 钉：省略号截断必须真的放得下标记、逐 run 结构不拍扁。
// 旧状 ShapeUncached("…", face) 在 face.Source()==nil（MultiFace/styledFace）
// 时返回 nil，省略号静默消失；且截断行被展平成单 run，face/样式全丢。

func ellipsisRefs(t *testing.T, faces ...Face) map[GlyphID]bool {
	t.Helper()
	out := map[GlyphID]bool{}
	for _, f := range faces {
		if g := ShapeUncached("…", f); len(g) == 1 {
			out[g[0].GID] = true
		}
	}
	return out
}

func loadTTFFace(t *testing.T, path string, size float64) Face {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test font missing: %s (%v)", path, err)
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Skipf("test font unusable: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src.Face(size)
}

// MultiFace 段落（CJK）截断：标记必须存在（旧状零标记 = 静默消失）。
func TestTruncate_EllipsisSurvivesMultiFace(t *testing.T) {
	latin := loadTTFFace(t, "testdata/goregular.ttf", 20.0)
	cjk := loadTTFFace(t, "testdata/wqy-microhei.ttc", 20.0)
	mf, err := NewMultiFace(latin, cjk)
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}
	ellGIDs := ellipsisRefs(t, latin, cjk)
	if len(ellGIDs) == 0 {
		t.Skip("no test face renders U+2026")
	}
	lay := LayoutText("世界你好世界你好世界你好", mf, LayoutOptions{
		MaxWidth: 80, MaxLines: 1, Overflow: OverflowEllipsis,
	})
	if len(lay.Lines) == 0 {
		t.Fatal("empty layout")
	}
	line := &lay.Lines[0]
	if len(line.Glyphs) == 0 {
		t.Fatal("truncated line has no glyphs")
	}
	last := line.Glyphs[len(line.Glyphs)-1]
	if !ellGIDs[last.GID] {
		t.Fatalf("tail glyph GID=%d, want an ellipsis GID from the shared fonts %v（MultiFace 下省略号静默消失）", last.GID, ellGIDs)
	}
	// 标记 cluster 必须是段内合法 rune 下标且不越全文。
	total := utf8.RuneCountInString("世界你好世界你好世界你好")
	if last.Cluster < 0 || last.Cluster >= total {
		t.Fatalf("ellipsis cluster=%d out of range [0,%d)", last.Cluster, total)
	}
}

// 截断行必须保留逐 run 结构：前缀 run + 标记 run 至少两个 run，禁止拍扁。
func TestTruncate_KeepsPerRunStructure(t *testing.T) {
	latin := loadTTFFace(t, "testdata/goregular.ttf", 20.0)
	cjk := loadTTFFace(t, "testdata/wqy-microhei.ttc", 20.0)
	mf, err := NewMultiFace(latin, cjk)
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}
	ellGIDs := ellipsisRefs(t, latin, cjk)
	if len(ellGIDs) == 0 {
		t.Skip("no test face renders U+2026")
	}
	lay := LayoutText("Hello 世界你好 Hello 世界你好", mf, LayoutOptions{
		MaxWidth: 140, MaxLines: 1, Overflow: OverflowEllipsis,
	})
	line := &lay.Lines[0]
	if len(line.Runs) < 2 {
		t.Fatalf("truncated line runs=%d, want >=2（前缀 + 标记，拍扁成单 run 即样式/face 丢失）", len(line.Runs))
	}
	marker := line.Runs[len(line.Runs)-1]
	if len(marker.Glyphs) == 0 || !ellGIDs[marker.Glyphs[len(marker.Glyphs)-1].GID] {
		t.Fatal("last run must carry the ellipsis marker")
	}
	// 前缀 glyph 与标记拼出的 line.Glyphs 必须仍是各 run 的按序拼接。
	n := 0
	for ri := range line.Runs {
		n += len(line.Runs[ri].Glyphs)
	}
	if n != len(line.Glyphs) {
		t.Fatalf("runs flatten=%d glyphs, line carries %d", n, len(line.Glyphs))
	}
}
