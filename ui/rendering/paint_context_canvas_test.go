package rendering_test

import (
	"testing"
	"unsafe"

	// Test seam: ui must not import gpu in production code, but building a
	// non-nil TextureView handle in a unit test needs the owning package.
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/rendering"
)

// Paint code all over the tree guards with `pc.DC == nil`. That guard only works
// if a nil *render.Context never survives being boxed into the render.Canvas
// interface — an interface holding a nil pointer is non-nil, and every one of
// those guards would then walk into a nil dereference deep inside a draw call.

func TestPaintContext_TypedNilCanvasNormalizesAtConstruction(t *testing.T) {
	var nilCtx *render.Context
	if pc := rendering.NewPaintContext(nilCtx, 1); pc.DC != nil {
		t.Errorf("NewPaintContext typed-nil: pc.DC must be nil, got %#v", pc.DC)
	}
	if pc := rendering.NewPaintContext(nil, 1); pc.DC != nil {
		t.Errorf("NewPaintContext(nil): pc.DC must be nil, got %#v", pc.DC)
	}
	// The child cursor copies the field, so the normalisation must survive it.
	if pc := rendering.NewPaintContext(nilCtx, 1).WithOrigin(10, 20); pc.DC != nil {
		t.Errorf("WithOrigin typed-nil: pc.DC must be nil, got %#v", pc.DC)
	}

	live := render.NewContext(40, 40)
	if pc := rendering.NewPaintContext(live, 1); pc.DC == nil {
		t.Error("NewPaintContext(live *render.Context): pc.DC must not be nil")
	}
	rec := recording.NewRecorder(40, 40)
	if pc := rendering.NewPaintContext(rec, 1); pc.DC == nil {
		t.Error("NewPaintContext(*recording.Recorder): pc.DC must not be nil")
	}
}

func TestPaintContext_LiveContextSeparatesLiveFromRecording(t *testing.T) {
	live := render.NewContext(40, 40)
	pc := rendering.NewPaintContext(live, 1)
	got, ok := pc.LiveContext()
	if !ok {
		t.Fatal("live walk must report a live context")
	}
	if got != live {
		t.Errorf("LiveContext returned %p, want the same context %p", got, live)
	}

	recPC := rendering.NewPaintContext(recording.NewRecorder(40, 40), 1)
	if got, ok := recPC.LiveContext(); ok || got != nil {
		t.Errorf("recording walk must report (nil,false), got (%v,%v)", got, ok)
	}

	if got, ok := (&rendering.PaintContext{}).LiveContext(); ok || got != nil {
		t.Errorf("empty cursor must report (nil,false), got (%v,%v)", got, ok)
	}
}

// A live-only draw that meets a recording walk must leave a number behind, never
// an invisible hole.

func TestPaintContext_RequireLiveContextCountsRefusals(t *testing.T) {
	rendering.ResetLiveOnlySkips()
	defer rendering.ResetLiveOnlySkips()

	livePC := rendering.NewPaintContext(render.NewContext(40, 40), 1)
	if _, ok := livePC.RequireLiveContext(); !ok {
		t.Fatal("live cursor must hand out its context")
	}
	if n := rendering.LiveOnlySkips(); n != 0 {
		t.Fatalf("a live draw must not count as a refusal, skips=%d", n)
	}

	recPC := rendering.NewPaintContext(recording.NewRecorder(40, 40), 1)
	if dc, ok := recPC.RequireLiveContext(); ok || dc != nil {
		t.Fatalf("recording cursor must refuse, got (%v,%v)", dc, ok)
	}
	if n := rendering.LiveOnlySkips(); n != 1 {
		t.Fatalf("refusal must count exactly once, skips=%d", n)
	}

	// Each further refusal adds one — the counter cannot saturate or reset.
	for i := 2; i <= 4; i++ {
		recPC.RequireLiveContext()
		if n := rendering.LiveOnlySkips(); n != int64(i) {
			t.Fatalf("after %d refusals skips must be %d, got %d", i, i, n)
		}
	}
}

// A device-owned texture view is not live-only: the recording stores the
// command and playback binds the view on the raster thread, so a FullPaint walk
// that records its frame keeps the texture on screen.

func TestDrawGPUTexture_RecordsOnRecordingWalk(t *testing.T) {
	rendering.ResetLiveOnlySkips()
	defer rendering.ResetLiveOnlySkips()

	var token byte
	view := gpucontext.NewTextureView(unsafe.Pointer(&token))

	rec := recording.NewRecorder(40, 40)
	pc := rendering.NewPaintContext(rec, 1)
	if !rendering.DrawGPUTexture(pc, view, 1, 2, 10, 20) {
		t.Fatal("DrawGPUTexture must report true on a recording walk")
	}
	if n := rendering.LiveOnlySkips(); n != 0 {
		t.Fatalf("a recordable texture draw must not count as a refusal, skips=%d", n)
	}
	cmds := rec.FinishRecording().Commands()
	if len(cmds) != 1 {
		t.Fatalf("recording must hold exactly one command, got %d", len(cmds))
	}
	cmd, ok := cmds[0].(recording.DrawGPUTextureCommand)
	if !ok {
		t.Fatalf("recorded command is %T, want recording.DrawGPUTextureCommand", cmds[0])
	}
	if cmd.View != view || cmd.X != 1 || cmd.Y != 2 || cmd.W != 10 || cmd.H != 20 {
		t.Errorf("recorded command = %+v, want the draw's view and geometry", cmd)
	}

	// On a live cursor the façade hands the draw to render and reports success.
	// The view is nil here because a unit test owns no GPU device; render's own
	// texture path no-ops on a nil view, which is exactly the delegation being
	// asserted — the façade must not swallow a live draw.
	livePC := rendering.NewPaintContext(render.NewContext(40, 40), 1)
	if !rendering.DrawGPUTexture(livePC, render.TextureView{}, 1, 1, 10, 10) {
		t.Error("DrawGPUTexture must report true and delegate on a live cursor")
	}
	if n := rendering.LiveOnlySkips(); n != 0 {
		t.Fatalf("a delegated live draw must not count as a refusal, skips=%d", n)
	}
}
