//go:build !nogpu

package gpu

import (
	"bytes"
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

// TestOpt22_IndexedMesh_FewerUploadBytes packs unique verts + indices and
// verifies the queued command is smaller than an expanded triangle list.
func TestOpt22_IndexedMesh_FewerUploadBytes(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	rc := shared.NewRenderContext()
	t.Cleanup(func() { rc.Close() })

	// Fan disc: 1 center + 4 rim = 5 unique verts, 4 tris = 12 indices.
	positions := []render.Point{
		{X: 32, Y: 32}, {X: 48, Y: 32}, {X: 32, Y: 48}, {X: 16, Y: 32}, {X: 32, Y: 16},
	}
	colors := make([]render.RGBA, len(positions))
	for i := range colors {
		colors[i] = render.RGBA{R: 1, A: 1}
	}
	indices := []uint16{0, 1, 2, 0, 2, 3, 0, 3, 4, 0, 4, 1}

	target := render.GPURenderTarget{Width: 64, Height: 64, Data: make([]byte, 64*64*4), Stride: 64 * 4}
	rc.QueueColoredMeshIndexed(target, positions, colors, indices)
	if len(rc.pendingConvexCommands) != 1 {
		t.Fatalf("cmds=%d", len(rc.pendingConvexCommands))
	}
	cmd := rc.pendingConvexCommands[0]
	if len(cmd.PackedVerts) != 5*convexMeshVertexStride {
		t.Fatalf("packed verts bytes=%d want %d", len(cmd.PackedVerts), 5*convexMeshVertexStride)
	}
	if len(cmd.Indices) != 12 {
		t.Fatalf("indices=%d want 12", len(cmd.Indices))
	}
	expandedBytes := 12 * convexMeshVertexStride
	if len(cmd.PackedVerts) >= expandedBytes {
		t.Fatalf("indexed pack not smaller than expanded: %d >= %d", len(cmd.PackedVerts), expandedBytes)
	}
	if convexCmdIndexCount(&cmd) != 12 {
		t.Fatalf("index count helper=%d", convexCmdIndexCount(&cmd))
	}
	ranges := buildConvexBlendRangesIndexed([]ConvexDrawCommand{cmd}, 0, 0)
	if len(ranges) != 1 || !ranges[0].indexed || ranges[0].indexCount != 12 {
		t.Fatalf("ranges=%+v", ranges)
	}
}

// TestOpt22_StashPreservesPackedVerts ensures present-stash deep-copies mesh
// bytes so a layer Queue cannot overwrite parent PackedVerts.
func TestOpt22_StashPreservesPackedVerts(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	rc := shared.NewRenderContext()
	t.Cleanup(func() { rc.Close() })

	parentTarget := render.GPURenderTarget{Width: 64, Height: 64, Data: make([]byte, 64*64*4), Stride: 64 * 4}
	rc.QueueColoredMesh(parentTarget,
		[]render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}},
		[]render.RGBA{{R: 1, A: 1}, {R: 1, A: 1}, {R: 1, A: 1}}, true)
	if len(rc.pendingConvexCommands) != 1 || len(rc.pendingConvexCommands[0].PackedVerts) == 0 {
		t.Fatal("expected packed parent mesh")
	}
	parentCopy := append([]byte(nil), rc.pendingConvexCommands[0].PackedVerts...)

	// Different View forces prepareTarget → stashPresentPending.
	layerTarget := parentTarget
	layerTarget.ViewWidth = 64
	layerTarget.ViewHeight = 64
	// Non-nil view pointer distinguishes targets; use a fake non-zero pointer
	// via NewTextureView only if available — instead flip Width and use Data nil
	// with a synthetic view through prepareTarget path.
	// Easiest: call stashPresentPending after ensuring hasPendingTarget.
	if !rc.hasPendingTarget {
		t.Fatal("expected pending target after parent queue")
	}
	// Simulate entering a layer RT by stashing then queueing over shared packed buf.
	rc.stashPresentPending()
	if !rc.presentStash.active || len(rc.presentStash.convex) != 1 {
		t.Fatalf("stash active=%v convex=%d", rc.presentStash.active, len(rc.presentStash.convex))
	}
	// Overwrite rc packed scratch as a subsequent layer mesh would.
	rc.QueueColoredMesh(parentTarget,
		[]render.Point{{X: 1, Y: 1}, {X: 20, Y: 1}, {X: 1, Y: 20}},
		[]render.RGBA{{G: 1, A: 1}, {G: 1, A: 1}, {G: 1, A: 1}}, true)
	stashed := rc.presentStash.convex[0].PackedVerts
	if !bytes.Equal(stashed, parentCopy) {
		t.Fatalf("stash packed corrupted after layer queue (len stash=%d parent=%d)", len(stashed), len(parentCopy))
	}
}
