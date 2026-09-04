package rendering

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/render/text"
)

// m5DejaVuFace loads the system DejaVuSans for shaped-path measurement
// (nil face would run the estimate path and under-report by ~1000x).
func m5DejaVuFace(t *testing.T) text.Face {
	t.Helper()
	b, err := os.ReadFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skip("need DejaVuSans system font")
	}
	src, err := text.NewFontSource(b)
	if err != nil {
		t.Fatalf("open DejaVu: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src.Face(16)
}

// TestSingleLongLineKeystroke_M5 measures the PRODUCTION incremental path
// (SetTextSpan + TextLayout, what InputBox.Sync runs) on a single 50k-char
// line with a real shaped face — the C-box shape.
func TestSingleLongLineKeystroke_M5(t *testing.T) {
	face := m5DejaVuFace(t)
	const n = 50000
	base := strings.Repeat("a世", n/2)
	mid := len(base) / 2
	edited := base[:mid] + "X" + base[mid+1:]
	rt := NewRenderText(base)
	rt.SetFace(face)
	_ = rt.TextLayout()
	var ds []time.Duration
	for i := 0; i < 11; i++ {
		t0 := time.Now()
		rt.SetTextSpan(edited, mid, mid+1, mid, mid+1)
		_ = rt.TextLayout()
		ds = append(ds, time.Since(t0))
		rt.SetTextSpan(base, mid, mid+1, mid, mid+1)
		_ = rt.TextLayout()
	}
	sorteds := append([]time.Duration(nil), ds...)
	for i := range sorteds {
		for j := i + 1; j < len(sorteds); j++ {
			if sorteds[j] < sorteds[i] {
				sorteds[i], sorteds[j] = sorteds[j], sorteds[i]
			}
		}
	}
	med := sorteds[len(sorteds)/2]
	t.Logf("single 50k line incremental keystroke median = %v", med)
	// §9 遗留#1：16ms 门禁需行内整形复用（当前整行重整，O(L) 实测约 70ms）。
	// 此处只设 200ms 防退化绊线（3× 现状），不冒充门禁达标。
	if med > 200*time.Millisecond {
		t.Fatalf("single-line incremental %v regressed far beyond O(L) baseline", med)
	}
}
