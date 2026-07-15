package render

import "testing"

func TestS64_ComposeColorMatrix_Identity(t *testing.T) {
	id := [20]float32{
		1, 0, 0, 0, 0,
		0, 1, 0, 0, 0,
		0, 0, 1, 0, 0,
		0, 0, 0, 1, 0,
	}
	scale := [20]float32{
		0.5, 0, 0, 0, 0,
		0, 0.5, 0, 0, 0,
		0, 0, 0.5, 0, 0,
		0, 0, 0, 1, 0,
	}
	got := composeColorMatrix4x5(id, scale)
	for i := range scale {
		if got[i] != scale[i] {
			t.Fatalf("id∘scale mismatch at %d: %v vs %v", i, got[i], scale[i])
		}
	}
	got2 := composeColorMatrix4x5(scale, id)
	for i := range scale {
		if got2[i] != scale[i] {
			t.Fatalf("scale∘id mismatch at %d", i)
		}
	}
}

func TestS64_CoalesceImageFilterNodes_MergesMatrices(t *testing.T) {
	id := ImageFilterNode{Kind: ImageFilterColorMatrix, Matrix: [20]float32{
		1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0,
	}}
	sc := ImageFilterNode{Kind: ImageFilterColorMatrix, Matrix: [20]float32{
		2, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 1, 0,
	}}
	out := coalesceImageFilterNodes([]ImageFilterNode{id, sc, {Kind: ImageFilterBlur, Radius: 0}})
	// blur radius 0 not runnable; matrices merge to 1
	if len(out) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(out), out)
	}
	if out[0].Kind != ImageFilterColorMatrix {
		t.Fatalf("kind=%v", out[0].Kind)
	}
	if out[0].Matrix[0] != 2 {
		t.Fatalf("merged m00=%v want 2", out[0].Matrix[0])
	}
}
