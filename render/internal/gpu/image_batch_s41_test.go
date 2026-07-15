//go:build !nogpu

package gpu

import "testing"

func TestS41_CanMergeImageDraw(t *testing.T) {
	base := ImageDrawCommand{
		GenerationID:   42,
		ImgWidth:       32,
		ImgHeight:      32,
		Opacity:        1,
		ViewportWidth:  512,
		ViewportHeight: 512,
		Nearest:        false,
	}
	same := base
	if !canMergeImageDraw(&base, &same) {
		t.Fatal("expected identical cmds to merge")
	}
	diffColor := base
	diffColor.Opacity = 0.5
	if canMergeImageDraw(&base, &diffColor) {
		t.Fatal("different opacity must not merge")
	}
	diffTex := base
	diffTex.GenerationID = 99
	if canMergeImageDraw(&base, &diffTex) {
		t.Fatal("different GenerationID must not merge")
	}
	zero := base
	zero.GenerationID = 0
	if canMergeImageDraw(&zero, &base) {
		t.Fatal("zero GenerationID must not merge")
	}
	diffNear := base
	diffNear.Nearest = true
	if canMergeImageDraw(&base, &diffNear) {
		t.Fatal("different filter must not merge")
	}
}

func TestS41_ImageDrawVertexCount(t *testing.T) {
	if imageDrawVertexCount(imageDrawCall{}) != 6 {
		t.Fatalf("zero vertexCount should default to 6")
	}
	if imageDrawVertexCount(imageDrawCall{vertexCount: 24}) != 24 {
		t.Fatalf("want 24")
	}
}

func TestS41_SliceImageResourcesByVertexRange(t *testing.T) {
	// Simulate 6 quads: batches of 3 + 2 + 1 (firstVertex 0/18/30)
	combined := &imageFrameResources{
		drawCalls: []imageDrawCall{
			{firstVertex: 0, vertexCount: 18},  // cmds 0..2
			{firstVertex: 18, vertexCount: 12}, // cmds 3..4
			{firstVertex: 30, vertexCount: 6},  // cmd 5
		},
	}
	s := &GPURenderSession{}

	// Group covering cmds 0..2
	g0 := s.sliceImageResources(combined, 0, 3)
	if g0 == nil || len(g0.drawCalls) != 1 || g0.drawCalls[0].firstVertex != 0 {
		t.Fatalf("group0 unexpected: %+v", g0)
	}
	// Group covering cmds 3..5
	g1 := s.sliceImageResources(combined, 3, 3)
	if g1 == nil || len(g1.drawCalls) != 2 {
		t.Fatalf("group1 want 2 draws, got %+v", g1)
	}
	// Empty
	if s.sliceImageResources(combined, 0, 0) != nil {
		t.Fatal("empty range should be nil")
	}
}

func TestS41_BatchSealPreventsCrossGroupMergeLogic(t *testing.T) {
	// Pure logic: with seals at index 3, runs should split even if mergeable.
	cmds := make([]ImageDrawCommand, 6)
	for i := range cmds {
		cmds[i] = ImageDrawCommand{
			GenerationID: 7, ImgWidth: 8, ImgHeight: 8,
			Opacity: 1, ViewportWidth: 100, ViewportHeight: 100,
		}
	}
	seal := make([]bool, 6)
	seal[3] = true // group boundary

	// Emulate run finder from buildImageResources.
	var runs [][2]int
	for i := 0; i < len(cmds); {
		j := i + 1
		for j < len(cmds) {
			if seal[j] {
				break
			}
			if !canMergeImageDraw(&cmds[i], &cmds[j]) {
				break
			}
			j++
		}
		runs = append(runs, [2]int{i, j})
		i = j
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs (0..3 and 3..6), got %v", runs)
	}
	if runs[0] != [2]int{0, 3} || runs[1] != [2]int{3, 6} {
		t.Fatalf("unexpected runs %v", runs)
	}
}

func TestS41_MergeRunCounts64SameTexture(t *testing.T) {
	// 64 identical images, no seals → 1 draw covering 64 quads.
	const n = 64
	cmds := make([]ImageDrawCommand, n)
	for i := range cmds {
		cmds[i] = ImageDrawCommand{
			GenerationID: 1001, ImgWidth: 32, ImgHeight: 32,
			Opacity: 1, ViewportWidth: 512, ViewportHeight: 512,
		}
	}
	var runs int
	var quads int
	for i := 0; i < n; {
		j := i + 1
		for j < n && canMergeImageDraw(&cmds[i], &cmds[j]) {
			j++
		}
		runs++
		quads += j - i
		i = j
	}
	if runs != 1 || quads != n {
		t.Fatalf("want 1 run / %d quads, got runs=%d quads=%d", n, runs, quads)
	}
}
