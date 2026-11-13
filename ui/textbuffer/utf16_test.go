package textbuffer

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func refUTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2
		}
	}
	return n
}

func refByteOffsetForUTF16(s string, utf16Off int) int {
	if utf16Off <= 0 {
		return 0
	}
	off := 0
	cu := 0
	for _, r := range s {
		sz := len(string(r))
		cuLen := 1
		if r > 0xFFFF {
			cuLen = 2
		}
		if cu+cuLen > utf16Off {
			break
		}
		cu += cuLen
		off += sz
		if cu >= utf16Off {
			break
		}
	}
	if cu < utf16Off {
		return len(s)
	}
	return off
}

func checkUTF16(t *testing.T, buf *Buffer, model string, forceTree bool) {
	t.Helper()
	if got, want := buf.UTF16Len(), refUTF16Len(model); got != want {
		t.Fatalf("tree=%v UTF16Len=%d want %d", forceTree, got, want)
	}
	total := refUTF16Len(model)
	for pos := 0; pos <= total; pos++ {
		if got, want := buf.ByteOffsetForUTF16(pos), refByteOffsetForUTF16(model, pos); got != want {
			t.Fatalf("tree=%v pos=%d ByteOffsetForUTF16=%d want %d", forceTree, pos, got, want)
		}
	}
	if got, want := buf.ByteOffsetForUTF16(-1), 0; got != want {
		t.Fatalf("tree=%v pos=-1 got %d want %d", forceTree, got, want)
	}
	if got, want := buf.ByteOffsetForUTF16(1<<30), len(model); got != want {
		t.Fatalf("tree=%v pos=huge got %d want %d", forceTree, got, want)
	}
}

func TestUTF16Offsets(t *testing.T) {
	docs := []string{
		"",
		"hello",
		"a\nb\nc",
		"世\n界\n👨‍👩‍👧",
		"a世b👨‍👩‍👧c",
		strings.Repeat("a世👨‍👩‍👧\n", 500),
	}
	for _, doc := range docs {
		for _, forceTree := range []bool{false, true} {
			buf := NewFromString(doc)
			if forceTree {
				buf.ForceTree(true)
				buf.Insert(0, "")
			}
			checkUTF16(t, buf, doc, forceTree)
		}
	}
	// 编辑后仍一致(编辑位置均在 rune 边界).
	model := strings.Repeat("alpha世\n", 100)
	buf := NewFromString(model)
	buf.ForceTree(true)
	buf.Insert(0, "")
	buf.Insert(0, "PREFIX👨‍👩‍👧\n")
	model = "PREFIX👨‍👩‍👧\n" + model
	mid := len(model) / 2
	for mid < len(model) && (model[mid]&0xC0) == 0x80 {
		mid++
	}
	buf.Insert(mid, "XYZ世")
	model = model[:mid] + "XYZ世" + model[mid:]
	da, db := 20, 40
	for da < len(model) && (model[da]&0xC0) == 0x80 {
		da++
	}
	for db < len(model) && (model[db]&0xC0) == 0x80 {
		db++
	}
	buf.Delete(da, db)
	model = model[:da] + model[db:]
	checkUTF16(t, buf, model, true)
}

func testByteOffsetTime(buf *Buffer, u16len, reps int) time.Duration {
	start := time.Now()
	for i := 0; i < reps; i++ {
		pos := u16len - 1 - (i*7)%(u16len/4)
		_ = buf.ByteOffsetForUTF16(pos)
	}
	return time.Since(start) / time.Duration(reps)
}

// TestByteOffsetForUTF16_NoLinear锁UTF-16偏移映射复杂度,当前整串扫描先红.
func TestByteOffsetForUTF16_NoLinear(t *testing.T) {
	const reps = 100
	small := NewFromString(strings.Repeat("a世b\n", 40000))
	small.ForceTree(true)
	big := NewFromString(strings.Repeat("a世b\n", 4000000))
	big.ForceTree(true)
	smallU, bigU := small.UTF16Len(), big.UTF16Len()
	testByteOffsetTime(small, smallU, 10)
	testByteOffsetTime(big, bigU, 10)
	ds := testByteOffsetTime(small, smallU, reps)
	db := testByteOffsetTime(big, bigU, reps)
	ratio := float64(db) / float64(ds)
	t.Logf("ByteOffsetForUTF16 small=%v big=%v ratio=%.2f", ds, db, ratio)
	if ratio > 8 {
		t.Fatalf("UTF-16偏移疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}

func BenchmarkByteOffsetForUTF16(b *testing.B) {
	for _, n := range []int{10000, 100000, 1000000} {
		buf := NewFromString(strings.Repeat("a世b\n", n))
		buf.ForceTree(true)
		total := buf.UTF16Len()
		b.Run(strings.TrimSpace(strings.ReplaceAll(map[int]string{10000: "1e4", 100000: "1e5", 1000000: "1e6"}[n], " ", "")), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = buf.ByteOffsetForUTF16(total - 1 - i%(total/4))
			}
		})
	}
}

func refUTF16ToRune(s string, pos int) int {
	off, idx := 0, 0
	for _, r := range s {
		if off >= pos {
			break
		}
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		off += w
		idx++
	}
	return idx
}

func refRuneToUTF16(s string, k int) int {
	if k <= 0 {
		return 0
	}
	n := 0
	for _, r := range s {
		if k == 0 {
			break
		}
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
		k--
	}
	return n
}

func refStep(s string, cur, dir int) int {
	if dir > 0 {
		b := refByteOffsetForUTF16(s, cur)
		r, _ := utf8.DecodeRuneInString(s[b:])
		if r > 0xFFFF {
			cur += 2
		} else {
			cur++
		}
	} else if dir < 0 {
		b := refByteOffsetForUTF16(s, cur)
		r, _ := utf8.DecodeLastRuneInString(s[:b])
		if r > 0xFFFF {
			cur -= 2
		} else {
			cur--
		}
	}
	if cur < 0 {
		cur = 0
	}
	if n := refUTF16Len(s); cur > n {
		cur = n
	}
	return cur
}

func TestRuneMappings(t *testing.T) {
	docs := []string{"", "abc", "a世b", "👨‍👩‍👧x", strings.Repeat("a世👨‍👩‍👧\n", 200)}
	for _, doc := range docs {
		for _, forceTree := range []bool{false, true} {
			buf := NewFromString(doc)
			if forceTree {
				buf.ForceTree(true)
				buf.Insert(0, "")
			}
			total := refUTF16Len(doc)
			if got, want := buf.RuneCount(), len([]rune(doc)); got != want {
				t.Fatalf("tree=%v RuneCount=%d want %d", forceTree, got, want)
			}
			for pos := 0; pos <= total; pos++ {
				if got, want := buf.UTF16ToRuneIndex(pos), refUTF16ToRune(doc, pos); got != want {
					t.Fatalf("tree=%v pos=%d UTF16ToRuneIndex=%d want %d", forceTree, pos, got, want)
				}
			}
			for k := 0; k <= len([]rune(doc)); k++ {
				if got, want := buf.RuneIndexToUTF16(k), refRuneToUTF16(doc, k); got != want {
					t.Fatalf("tree=%v k=%d RuneIndexToUTF16=%d want %d", forceTree, k, got, want)
				}
			}
			// UTF16OffsetAt / StepUTF16 只在 rune 边界校验（Editor 的偏移约定）.
			byteOff := 0
			for _, r := range doc {
				if got, want := buf.UTF16OffsetAt(byteOff), refUTF16Len(doc[:byteOff]); got != want {
					t.Fatalf("tree=%v byte=%d UTF16OffsetAt=%d want %d", forceTree, byteOff, got, want)
				}
				byteOff += len(string(r))
			}
			for pos := 0; pos <= total; pos++ {
				gotF, _ := buf.StepUTF16(pos, 1)
				if want := refStep(doc, pos, 1); gotF != want {
					t.Fatalf("tree=%v pos=%d Step+ got %d want %d", forceTree, pos, gotF, want)
				}
				gotB, _ := buf.StepUTF16(pos, -1)
				if want := refStep(doc, pos, -1); gotB != want {
					t.Fatalf("tree=%v pos=%d Step- got %d want %d", forceTree, pos, gotB, want)
				}
			}
		}
	}
}

func TestRuneMapping_NoLinear(t *testing.T) {
	const reps = 100
	small := NewFromString(strings.Repeat("a世b\n", 40000))
	small.ForceTree(true)
	big := NewFromString(strings.Repeat("a世b\n", 4000000))
	big.ForceTree(true)
	measure := func(b *Buffer) time.Duration {
		total := b.UTF16Len()
		start := time.Now()
		for i := 0; i < reps; i++ {
			pos := total - 1 - (i*7)%(total/4)
			k := b.UTF16ToRuneIndex(pos)
			_ = b.RuneIndexToUTF16(k)
		}
		return time.Since(start) / time.Duration(reps)
	}
	measure(small)
	measure(big)
	ds, db := measure(small), measure(big)
	ratio := float64(db) / float64(ds)
	t.Logf("rune mapping small=%v big=%v ratio=%.2f", ds, db, ratio)
	if ratio > 8 {
		t.Fatalf("rune映射疑似线性:big/small=%.1f,门禁≤8", ratio)
	}
}
