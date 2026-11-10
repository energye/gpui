package raster

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// P4.1 batch-1 recording round-trip: the mesh paint wiring (vertex alpha,
// paint alpha, Porter-Duff blend) must survive Recorder → playback bit for
// bit, and the recorded stream must actually carry the mesh commands (a
// silently dropped command would otherwise replay as a blank frame).
func TestP41VerticesPaintRoundTrip(t *testing.T) {
	tri := []render.Point{{X: 8, Y: 8}, {X: 88, Y: 8}, {X: 48, Y: 88}}
	cols := []render.RGBA{{R: 1, A: 0.5}, {G: 1, A: 0.5}, {B: 1, A: 0.5}}
	quad := render.Mesh{
		Positions: []render.Point{{X: 100, Y: 20}, {X: 180, Y: 20}, {X: 180, Y: 90}, {X: 100, Y: 90}},
		Indices:   []uint16{0, 1, 2, 0, 2, 3},
	}
	paint := func(p render.Canvas) {
		p.SetRGBA(0.10, 0.12, 0.16, 1)
		p.DrawRectangle(0, 0, 192, 112)
		_ = p.Fill()

		// Gouraud vertices with vertex alpha + paint alpha 0.5.
		p.Push()
		p.SetRGBA(1, 1, 1, 0.5)
		p.DrawVertices(tri, cols, render.VertexModeTriangles)
		p.Pop()

		// Indexed mesh through the Porter-Duff layer path.
		p.Push()
		p.SetRGBA(1, 0.4, 0.1, 1)
		p.SetBlendMode(render.BlendPlus)
		p.DrawMesh(quad)
		p.SetBlendMode(render.BlendNormal)
		p.Pop()
	}
	assertC2bParity(t, 192, 112, paint)

	// The recording must contain both mesh command families.
	rec := recording.NewRecorder(192, 112)
	paint(rec)
	r := rec.FinishRecording()
	seen := map[recording.CommandType]int{}
	for _, c := range r.Commands() {
		seen[c.Type()]++
	}
	if seen[recording.CmdDrawVertices] == 0 {
		t.Fatalf("no CmdDrawVertices in the recording")
	}
	if seen[recording.CmdDrawMesh] == 0 {
		t.Fatalf("no CmdDrawMesh in the recording")
	}
}
