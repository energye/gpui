package sprite

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type batchSpriteDef struct {
	Tag     string    `json:"tag"`
	Image   string    `json:"image"`
	Src     []float64 `json:"src"`
	Dst     []float64 `json:"dst"`
	Opacity float64   `json:"opacity"`
}

type batchGroupDef struct {
	Image string   `json:"image"`
	Tags  []string `json:"tags"`
}

type batchCase struct {
	Name        string           `json:"name"`
	Sprites     []batchSpriteDef `json:"sprites"`
	WantCalls   int              `json:"want_calls"`
	WantStored  *int             `json:"want_stored,omitempty"`
	WantSkipped *int             `json:"want_skipped,omitempty"`
	WantGroups  []batchGroupDef  `json:"want_groups"`
}

type batchFile struct {
	Cases []batchCase `json:"cases"`
}

func loadBatchCases(t *testing.T) batchFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "batch_cases.json"))
	if err != nil {
		t.Fatalf("read batch_cases.json: %v", err)
	}
	var f batchFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode batch_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("batch_cases.json has no cases")
	}
	return f
}

func mustFindBatchCase(t *testing.T, f batchFile, name string) batchCase {
	t.Helper()
	for _, c := range f.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("batch_cases.json has no case %q", name)
	return batchCase{}
}

func batchRect(t *testing.T, v []float64, what string) core.Rect {
	t.Helper()
	if len(v) != 4 {
		t.Fatalf("%s has %d numbers, want 4", what, len(v))
	}
	return core.NewRect(v[0], v[1], v[2], v[3])
}

func batchAddCase(t *testing.T, b *Batch, c batchCase) {
	t.Helper()
	for _, d := range c.Sprites {
		s, err := NewSprite(core.AssetID(d.Image), batchRect(t, d.Src, d.Tag+"/src"), batchRect(t, d.Dst, d.Tag+"/dst"), d.Opacity)
		if err != nil {
			t.Fatalf("%s: NewSprite %q: %v", c.Name, d.Tag, err)
		}
		s.Tag = d.Tag
		if _, err := b.Add(s); err != nil {
			t.Fatalf("%s: Add %q: %v", c.Name, d.Tag, err)
		}
	}
}

func batchFlushTags(b *Batch) (map[string][]string, []string) {
	got := map[string][]string{}
	var order []string
	b.Flush(func(id core.AssetID, sprites []Sprite) {
		order = append(order, string(id))
		for _, s := range sprites {
			got[string(id)] = append(got[string(id)], s.Tag)
		}
	})
	return got, order
}

func equalTags(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// A: same-image sprites submit once in Add order; mixed images group stable.
func TestBatchGroupsFromCases(t *testing.T) {
	f := loadBatchCases(t)
	for _, c := range f.Cases {
		b := NewBatch()
		batchAddCase(t, b, c)
		if c.WantStored != nil && b.Len() != *c.WantStored {
			t.Errorf("%s: Len = %d, want %d", c.Name, b.Len(), *c.WantStored)
		}
		if c.WantSkipped != nil && b.Skipped() != *c.WantSkipped {
			t.Errorf("%s: Skipped = %d, want %d", c.Name, b.Skipped(), *c.WantSkipped)
		}
		got, order := batchFlushTags(b)
		if len(order) != c.WantCalls {
			t.Errorf("%s: calls = %d, want %d", c.Name, len(order), c.WantCalls)
		}
		if len(order) != len(c.WantGroups) {
			t.Errorf("%s: groups = %d, want %d", c.Name, len(order), len(c.WantGroups))
			continue
		}
		for i, wg := range c.WantGroups {
			if order[i] != wg.Image {
				t.Errorf("%s: group %d image = %q, want %q", c.Name, i, order[i], wg.Image)
			}
			if !equalTags(got[wg.Image], wg.Tags) {
				t.Errorf("%s: group %q tags = %v, want %v", c.Name, wg.Image, got[wg.Image], wg.Tags)
			}
		}
		if b.Len() != 0 {
			t.Errorf("%s: Flush did not drain, Len = %d", c.Name, b.Len())
		}
	}
}

// B: empty, zero-size, and corrupt inputs never crash; bad paths report codes.
func TestBatchEdgesNoCrash(t *testing.T) {
	// Empty batch flushes zero calls and never calls emit.
	b := NewBatch()
	calls := b.Flush(func(core.AssetID, []Sprite) { t.Error("empty Flush emitted") })
	if calls != 0 {
		t.Errorf("empty Flush = %d, want 0", calls)
	}
	if b.Len() != 0 || b.Skipped() != 0 {
		t.Errorf("empty batch Len/Skipped = %d/%d, want 0/0", b.Len(), b.Skipped())
	}
	// Zero-size sprites skip quietly with nil error, never stored.
	zeroSrc, err := NewSprite("tex/forest", core.NewRect(0, 0, 0, 32), core.NewRect(0, 0, 32, 32), 1)
	if err != nil {
		t.Fatalf("NewSprite zero src: %v", err)
	}
	if Skippable(zeroSrc) != true {
		t.Error("zero-src Skippable = false, want true")
	}
	added, err := b.Add(zeroSrc)
	if err != nil || added {
		t.Errorf("Add zero-src = %v,%v, want false,nil", added, err)
	}
	zeroDst, err := NewSprite("tex/forest", core.NewRect(0, 0, 32, 32), core.NewRect(0, 0, 0, 32), 1)
	if err != nil {
		t.Fatalf("NewSprite zero dst: %v", err)
	}
	if added, err := b.Add(zeroDst); err != nil || added {
		t.Errorf("Add zero-dst = %v,%v, want false,nil", added, err)
	}
	if b.Len() != 0 || b.Skipped() != 2 {
		t.Errorf("after skips Len/Skipped = %d/%d, want 0/2", b.Len(), b.Skipped())
	}
	// Negative destination size is kept (mirrored draw at the render edge).
	mir, err := NewSprite("tex/forest", core.NewRect(0, 0, 32, 32), core.NewRect(0, 0, -32, 32), 1)
	if err != nil {
		t.Fatalf("NewSprite mirrored: %v", err)
	}
	if Skippable(mir) {
		t.Error("mirrored Dst Skippable = true, want false")
	}
	// Bad Adds store nothing and report invalid-arg.
	bad := []Sprite{
		{Image: "", Src: core.NewRect(0, 0, 8, 8), Dst: core.NewRect(0, 0, 8, 8), Opacity: 1},
		{Image: "tex/a", Src: core.NewRect(math.NaN(), 0, 8, 8), Dst: core.NewRect(0, 0, 8, 8), Opacity: 1},
		{Image: "tex/a", Src: core.NewRect(0, 0, 8, 8), Dst: core.NewRect(0, 0, 8, math.Inf(1)), Opacity: 1},
		{Image: "tex/a", Src: core.NewRect(0, 0, 8, 8), Dst: core.NewRect(0, 0, 8, 8), Opacity: math.NaN()},
	}
	for i, s := range bad {
		if added, err := b.Add(s); err == nil || added {
			t.Errorf("bad Add[%d] = %v,%v, want false,error", i, added, err)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad Add[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if b.Len() != 0 {
		t.Errorf("bad Adds stored %d, want 0", b.Len())
	}
	if _, err := NewSprite("", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty image NewSprite err = %v, want invalid-arg", err)
	}
	// Nil receivers never panic.
	var nilB *Batch
	if added, err := nilB.Add(Sprite{Image: "tex/a"}); err == nil || added {
		t.Errorf("nil Add = %v,%v, want false,error", added, err)
	}
	if nilB.Flush(func(core.AssetID, []Sprite) { t.Error("nil Flush emitted") }) != 0 {
		t.Error("nil Flush != 0, want 0")
	}
	if nilB.Len() != 0 || nilB.Skipped() != 0 {
		t.Error("nil Len/Skipped != 0")
	}
	nilB.Clear()
	// Nil emit is a quiet no-op preserving the batch.
	keep := NewBatch()
	s, err := NewSprite("tex/a", core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), 1)
	if err != nil {
		t.Fatalf("NewSprite keep: %v", err)
	}
	if _, err := keep.Add(s); err != nil {
		t.Fatalf("Add keep: %v", err)
	}
	if got := keep.Flush(nil); got != 0 || keep.Len() != 1 {
		t.Errorf("nil-emit Flush = %d Len = %d, want 0/1", got, keep.Len())
	}
	// Opacity mapping matches the render edge: <=0 defaults, >1 clamps.
	for _, tc := range []struct {
		in, want float64
	}{{0, 1}, {-2, 1}, {0.5, 0.5}, {1, 1}, {2, 1}} {
		if got := EffectiveOpacity(tc.in); got != tc.want {
			t.Errorf("EffectiveOpacity(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// C does not need pixels (pure math, draws nothing): the group path must
// be lossless and replays bitwise identical, with the same skip rule the
// existing render DrawAtlas uses.
func TestBatchBoundaryIdentical(t *testing.T) {
	f := loadBatchCases(t)
	for _, c := range f.Cases {
		build := func() *Batch {
			b := NewBatch()
			batchAddCase(t, b, c)
			return b
		}
		a, bb := build(), build()
		gotA, orderA := batchFlushTags(a)
		gotB, orderB := batchFlushTags(bb)
		if len(orderA) != len(orderB) {
			t.Fatalf("%s: replay calls %d vs %d", c.Name, len(orderA), len(orderB))
		}
		for i := range orderA {
			if orderA[i] != orderB[i] || !equalTags(gotA[orderA[i]], gotB[orderB[i]]) {
				t.Fatalf("%s: replay diverged at group %d", c.Name, i)
			}
		}
		// Emitted slices are fresh copies: writing them cannot alias the batch.
		b := build()
		var first []Sprite
		b.Flush(func(_ core.AssetID, sprites []Sprite) {
			if first == nil {
				first = sprites
			}
		})
		if len(first) > 0 {
			first[0].Tag = "mutated-probe"
			again := build()
			got, _ := batchFlushTags(again)
			for _, tags := range got {
				for _, tg := range tags {
					if tg == "mutated-probe" {
						t.Fatalf("%s: emitted slice aliases stored batch", c.Name)
					}
				}
			}
		}
	}
	// Skip rule mirrors render DrawAtlas exactly:
	// Src<=0 skips, Dst==0 skips, negative Dst is kept.
	for _, tc := range []struct {
		src, dst core.Rect
		want     bool
	}{
		{core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 8, 8), false},
		{core.NewRect(0, 0, 0, 8), core.NewRect(0, 0, 8, 8), true},
		{core.NewRect(0, 0, 8, 0), core.NewRect(0, 0, 8, 8), true},
		{core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, 0, 8), true},
		{core.NewRect(0, 0, 8, 8), core.NewRect(0, 0, -8, 8), false},
	} {
		s := Sprite{Image: "tex/a", Src: tc.src, Dst: tc.dst, Opacity: 1}
		if got := Skippable(s); got != tc.want {
			t.Errorf("Skippable src=%v dst=%v = %v, want %v", tc.src, tc.dst, got, tc.want)
		}
	}
}

// D: 1000 sprites flush with a measured cost and call count.
func TestBatchPerfThousand(t *testing.T) {
	// Synthetic load only (no golden): golden groups stay in
	// batch_cases.json. Seeded rand keeps the load replayable.
	r := core.NewRand(20260915)
	const n = 1000
	b := NewBatch()
	for i := 0; i < n; i++ {
		img := core.AssetID("tex/forest")
		if i%10 == 0 {
			img = core.AssetID("tex/rock")
		}
		s, err := NewSprite(img,
			core.NewRect(float64(int(r.RangeFloat(0, 240))), 0, 16, 32),
			core.NewRect(r.RangeFloat(0, 1024), r.RangeFloat(0, 512), 16, 32), 1)
		if err != nil {
			t.Fatalf("sprite %d: %v", i, err)
		}
		if _, err := b.Add(s); err != nil {
			t.Fatalf("Add %d: %v", i, err)
		}
	}
	if b.Len() != n {
		t.Fatalf("Len = %d, want %d", b.Len(), n)
	}
	start := time.Now()
	var emitted, groups int
	calls := b.Flush(func(_ core.AssetID, sprites []Sprite) {
		groups++
		emitted += len(sprites)
	})
	el := time.Since(start)
	if calls != 2 || groups != 2 || emitted != n {
		t.Fatalf("Flush calls/groups/emitted = %d/%d/%d, want 2/2/%d", calls, groups, emitted, n)
	}
	t.Logf("batch-1000: %d sprites in %v (%d draws, %.1f sprites/draw)", n, el, calls, float64(n)/float64(calls))
	// Same-image thousand submits once.
	one := NewBatch()
	for i := 0; i < n; i++ {
		s, err := NewSprite("tex/forest",
			core.NewRect(0, 0, 16, 32),
			core.NewRect(float64(i%64)*16, float64(i/64)*32, 16, 32), 1)
		if err != nil {
			t.Fatalf("one %d: %v", i, err)
		}
		if _, err := one.Add(s); err != nil {
			t.Fatalf("one Add %d: %v", i, err)
		}
	}
	if got := one.Flush(func(core.AssetID, []Sprite) {}); got != 1 {
		t.Errorf("same-image 1000 Flush = %d, want 1", got)
	}
}

// E: long runs neither grow nor diverge between replays.
func TestBatchLongRunStable(t *testing.T) {
	f := loadBatchCases(t)
	c := mustFindBatchCase(t, f, "mixed_images_interleaved")
	mk := func() *Batch {
		b := NewBatch()
		batchAddCase(t, b, c)
		return b
	}
	first, orderFirst := batchFlushTags(mk())
	for i := 0; i < 5000; i++ {
		got, order := batchFlushTags(mk())
		if len(order) != len(orderFirst) {
			t.Fatalf("rep %d calls %d vs %d", i, len(order), len(orderFirst))
		}
		for j := range order {
			if order[j] != orderFirst[j] || !equalTags(got[order[j]], first[orderFirst[j]]) {
				t.Fatalf("rep %d diverged at group %d", i, j)
			}
		}
	}
	// Drain returns to baseline: repeated fill+flush never accumulates.
	b := NewBatch()
	for i := 0; i < 200; i++ {
		batchAddCase(t, b, c)
		b.Flush(func(core.AssetID, []Sprite) {})
		if b.Len() != 0 || b.Skipped() != 0 {
			t.Fatalf("cycle %d Len/Skipped = %d/%d, want 0/0", i, b.Len(), b.Skipped())
		}
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen groups in batch_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the tags.
func TestBatchOffscreenGolden(t *testing.T) {
	f := loadBatchCases(t)
	same := mustFindBatchCase(t, f, "same_image_batch")
	if same.WantCalls != 1 || len(same.WantGroups) != 1 {
		t.Fatalf("same_image_batch calls/groups = %d/%d, want 1/1", same.WantCalls, len(same.WantGroups))
	}
	// Shape: one image draws the whole batch in Add order.
	b := NewBatch()
	batchAddCase(t, b, same)
	got, order := batchFlushTags(b)
	if len(order) != 1 || order[0] != "tex/forest" {
		t.Fatalf("same-image order = %v, want [tex/forest]", order)
	}
	var wantTags []string
	for _, d := range same.Sprites {
		wantTags = append(wantTags, d.Tag)
	}
	if !equalTags(got["tex/forest"], wantTags) {
		t.Errorf("same-image tags = %v, want Add order %v", got["tex/forest"], wantTags)
	}
	// Shape: interleaved images split into first-seen groups, each stable.
	mixed := mustFindBatchCase(t, f, "mixed_images_interleaved")
	if len(mixed.WantGroups) != 2 || mixed.WantGroups[0].Image == mixed.WantGroups[1].Image {
		t.Fatalf("mixed groups = %+v, want 2 distinct images", mixed.WantGroups)
	}
	// Shape: zero-size sprites never reach the draw.
	zero := mustFindBatchCase(t, f, "with_zero_skipped")
	if zero.WantStored == nil || zero.WantSkipped == nil {
		t.Fatal("with_zero_skipped misses want_stored/want_skipped")
	}
	zb := NewBatch()
	batchAddCase(t, zb, zero)
	if zb.Len() != *zero.WantStored || zb.Skipped() != *zero.WantSkipped {
		t.Errorf("zero Len/Skipped = %d/%d, want %d/%d",
			zb.Len(), zb.Skipped(), *zero.WantStored, *zero.WantSkipped)
	}
	gotZ, _ := batchFlushTags(zb)
	for _, tags := range gotZ {
		for _, tg := range tags {
			if tg == "z0" || tg == "z1" {
				t.Errorf("zero-size tag %q reached the draw", tg)
			}
		}
	}
	// Shape: thousand-tree pattern stays one draw (window intent:
	// game_sprite --case=batch draws a thousand trees in one submit;
	// the window itself lands with P2, this golden is the offscreen proof).
	mini := mustFindBatchCase(t, f, "thousand_trees_mini")
	if mini.WantCalls != 1 {
		t.Errorf("thousand_trees_mini calls = %d, want 1", mini.WantCalls)
	}
	mb := NewBatch()
	batchAddCase(t, mb, mini)
	if got := mb.Flush(func(core.AssetID, []Sprite) {}); got != 1 {
		t.Errorf("thousand_trees_mini Flush = %d, want 1", got)
	}
}
