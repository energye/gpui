package scene_test

import (
	"testing"

	"github.com/energye/gpui/ui/scene"
)

// T1 seal: Seal/IsSealed mark the EndFrame handoff (G3).
func TestFramePacket_SealMarksEndFrame(t *testing.T) {
	var nilPkt *scene.FramePacket
	if nilPkt.IsSealed() {
		t.Fatal("nil packet must not report sealed")
	}
	nilPkt.Seal() // no-op, must not panic

	pkt := &scene.FramePacket{FrameID: 7}
	if pkt.IsSealed() {
		t.Fatal("fresh packet must start unsealed")
	}
	pkt.Seal()
	if !pkt.IsSealed() {
		t.Fatal("Seal must mark handed-off read-only")
	}
}

// T1 four stamps (G10): build pair honest, raster pair zero until T2.
func TestFramePacket_FourStampsHonest(t *testing.T) {
	pkt := &scene.FramePacket{}
	if pkt.BuildMs() != 0 || pkt.RasterMs() != 0 {
		t.Fatal("unstamped packet must report 0 durations")
	}
	pkt.MarkBuildBegin()
	pkt.MarkBuildEnd()
	if pkt.BuildBeginNs == 0 || pkt.BuildEndNs == 0 {
		t.Fatal("build stamps must be set")
	}
	if pkt.BuildEndNs < pkt.BuildBeginNs {
		t.Fatal("build end must not precede begin")
	}
	if pkt.RasterBeginNs != 0 || pkt.RasterEndNs != 0 {
		t.Fatal("raster stamps must stay zero until the raster thread marks them (T2)")
	}
	pkt.MarkRasterBegin()
	pkt.MarkRasterEnd()
	if pkt.RasterMs() < 0 {
		t.Fatal("raster duration must not be negative")
	}
}

// T1 G12: hooks register pre-seal, refuse post-seal, clone copies.
func TestFramePacket_PostFrameHooksSealGate(t *testing.T) {
	pkt := &scene.FramePacket{}
	if pkt.AddPostFrameHook(nil) {
		t.Fatal("nil hook must be refused")
	}
	called := 0
	if !pkt.AddPostFrameHook(func(frameID uint64) { called++ }) {
		t.Fatal("pre-seal hook must register")
	}
	pkt.Seal()
	if pkt.AddPostFrameHook(func(frameID uint64) {}) {
		t.Fatal("post-seal hook must be refused")
	}
	cp := pkt.CloneShallow()
	if len(cp.PostFrameHooks) != 1 {
		t.Fatalf("clone must copy hooks, got %d", len(cp.PostFrameHooks))
	}
	cp.PostFrameHooks[0](pkt.FrameID)
	if called != 1 {
		t.Fatal("cloned hook must still fire")
	}
	// Mutating the clone's hook slice must not touch the original.
	cp.PostFrameHooks[0] = nil
	if pkt.PostFrameHooks[0] == nil {
		t.Fatal("hook slice shared — CloneShallow must copy")
	}
}

// T1 hops table (D10): every UI-thread direct call is listed, unknown refused.
func TestFramePacket_HopsTableComplete(t *testing.T) {
	for _, name := range []string{"blink", "ime", "clipboard", "overlay", "input", "focus"} {
		if !scene.IsAllowedHop(name) {
			t.Fatalf("hop %q must be listed", name)
		}
	}
	if scene.IsAllowedHop("gpu-submit") {
		t.Fatal("gpu-submit must never be an allowed hop (ui never touches gpu)")
	}
	hops := scene.ListFrameHops()
	if len(hops) != len(scene.FrameHops) {
		t.Fatal("ListFrameHops must mirror the table")
	}
	hops[0].Name = "mutated"
	if scene.FrameHops[0].Name == "mutated" {
		t.Fatal("ListFrameHops must return a copy")
	}
}

// T1 CloneShallow carries the new EndFrame state (seal/producer/stamps/regen).
func TestFramePacket_CloneShallowCarriesSealState(t *testing.T) {
	pkt := &scene.FramePacket{FrameID: 1, Producer: scene.ProducerUI, RegenerateFrom: 0}
	pkt.MarkBuildBegin()
	pkt.MarkBuildEnd()
	pkt.Seal()
	cp := pkt.CloneShallow()
	if !cp.IsSealed() {
		t.Fatal("clone of a sealed packet must stay sealed")
	}
	if cp.Producer != scene.ProducerUI {
		t.Fatalf("producer lost: %q", cp.Producer)
	}
	if cp.BuildBeginNs != pkt.BuildBeginNs || cp.BuildEndNs != pkt.BuildEndNs {
		t.Fatal("build stamps lost in clone")
	}
	if !scene.ShareRoot(pkt, cp) && (pkt.Root != nil || cp.Root != nil) {
		t.Fatal("clone must share root (both nil here, ShareRoot false is honest)")
	}
}
