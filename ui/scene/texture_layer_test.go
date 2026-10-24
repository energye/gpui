//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// paintProbeSource is a texture source that fills its quad in one color and
// remembers every call: the pixels prove the composite CTM reached the source,
// the recorded options prove what the layer handed over.
type paintProbeSource struct {
	r, g, b float64
	calls   []render.TexturePaintOptions
}

func (p *paintProbeSource) PaintTexture(dc *render.Context, opts render.TexturePaintOptions) (bool, bool) {
	p.calls = append(p.calls, opts)
	if dc == nil {
		return false, false
	}
	dc.Push()
	dc.SetRGBA(p.r, p.g, p.b, 1)
	dc.DrawRectangle(opts.X, opts.Y, opts.Width, opts.Height)
	_ = dc.Fill()
	dc.Pop()
	return true, true
}

func TestTextureLayer_KindAndRect(t *testing.T) {
	scene.ResetLayerIDGen()
	tl := scene.NewTextureLayer(7, 10, 20, 300, 200)
	if tl.Kind() != "texture" {
		t.Fatalf("Kind=%q want texture", tl.Kind())
	}
	if tl.LayerID() == 0 {
		t.Fatal("LayerID=0")
	}
	if len(tl.Children()) != 0 {
		t.Fatalf("Children=%d want a leaf", len(tl.Children()))
	}
	x, y, w, h := tl.Rect()
	if x != 10 || y != 20 || w != 300 || h != 200 {
		t.Fatalf("Rect=(%v,%v,%v,%v) want (10,20,300,200)", x, y, w, h)
	}
	if tl.TextureID != 7 {
		t.Fatalf("TextureID=%d want 7", tl.TextureID)
	}
}

// TestCompositeToContext_TextureUsesCompositeCTM locks the thread contract: the
// layer carries only local coordinates, and the source paints under the CTM the
// composite walk installed, so an offset layer moves the quad without the layer
// knowing anything about it.
func TestCompositeToContext_TextureUsesCompositeCTM(t *testing.T) {
	scene.ResetLayerIDGen()
	src := &paintProbeSource{r: 1, g: 0, b: 0}
	id := render.RegisterTextureSource(src)
	defer render.UnregisterTextureSource(id)

	b := scene.NewLayerBuilder()
	b.PushOffset(20, 10)
	tl := b.AddTexture(id, 0, 0, 40, 30)
	tl.DrawIndex = 3
	b.Pop()

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	st := scene.CompositeToContext(b.Root(), dc)
	if st.TexturesVisited != 1 || st.TexturesPainted != 1 || st.TexturesMissed != 0 {
		t.Fatalf("texture stats=%+v want visited=1 painted=1 missed=0", st)
	}
	if len(src.calls) != 1 {
		t.Fatalf("source calls=%d want 1", len(src.calls))
	}
	got := src.calls[0]
	if got.X != 0 || got.Y != 0 || got.Width != 40 || got.Height != 30 {
		t.Fatalf("source opts=%+v want the layer rect, not a pre-translated one", got)
	}
	if got.DrawIndex != 3 {
		t.Fatalf("DrawIndex=%d want 3 (per-view evidence must survive the handoff)", got.DrawIndex)
	}
	// Red where the offset put the quad, white where the quad is not.
	if rr, gg, bb := sampleAt(dc.Image(), 30, 20); rr < 0xC000 || gg > 0x4000 || bb > 0x4000 {
		t.Fatalf("inside quad (30,20)=#%04x%04x%04x want red", rr, gg, bb)
	}
	if rr, gg, bb := sampleAt(dc.Image(), 90, 70); rr < 0xC000 || gg < 0xC000 || bb < 0xC000 {
		t.Fatalf("outside quad (90,70)=#%04x%04x%04x want white", rr, gg, bb)
	}
}

// TestCompositeToContext_TextureMissIsCounted: an id that resolves to nothing
// (producer closed, frame in flight) must report a miss and cover no pixels —
// never pretend the rect was painted.
func TestCompositeToContext_TextureMissIsCounted(t *testing.T) {
	scene.ResetLayerIDGen()
	src := &paintProbeSource{r: 0, g: 1, b: 0}
	id := render.RegisterTextureSource(src)
	render.UnregisterTextureSource(id)

	b := scene.NewLayerBuilder()
	b.AddTexture(id, 0, 0, 40, 30)
	b.AddTexture(0, 50, 0, 40, 30) // id 0 = no source

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)

	st := scene.CompositeToContext(b.Root(), dc)
	if st.TexturesVisited != 2 || st.TexturesPainted != 0 || st.TexturesMissed != 2 {
		t.Fatalf("texture stats=%+v want visited=2 painted=0 missed=2", st)
	}
	if len(src.calls) != 0 {
		t.Fatalf("unregistered source was called %d times", len(src.calls))
	}
	if rr, gg, bb := sampleAt(dc.Image(), 10, 10); rr < 0xC000 || gg < 0xC000 || bb < 0xC000 {
		t.Fatalf("(10,10)=#%04x%04x%04x want untouched white", rr, gg, bb)
	}
}

// TestTextured_TextureLayerAlwaysDamagesAndNeverCaches is the Flutter
// texture_layer.cc contract on the retained path: a texture layer paints and
// damages its rect on EVERY frame — including a frame where nothing else is
// dirty (the producer rewrote the texture in place, so a cached blit would show
// a stale frame) — and never enters the picture-texture cache.
func TestTextured_TextureLayerAlwaysDamagesAndNeverCaches(t *testing.T) {
	scene.ResetLayerIDGen()
	src := &paintProbeSource{r: 1, g: 0, b: 0}
	id := render.RegisterTextureSource(src)
	defer render.UnregisterTextureSource(id)

	b := scene.NewLayerBuilder()
	b.PushOffset(20, 10)
	b.AddTexture(id, 0, 0, 40, 30)
	b.Pop()
	pkt := b.BuildPacket(1, 1, 100, 80)
	if len(pkt.DirtyLayerIDs) != 0 {
		t.Fatalf("DirtyLayerIDs=%v want none: a texture layer never marks itself dirty", pkt.DirtyLayerIDs)
	}

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)

	want := image.Rect(20, 10, 60, 40)
	for frame := 0; frame < 2; frame++ {
		// Same sealed packet, second pass: nothing in the UI tree changed, only
		// what the producer wrote into its texture.
		st := scene.CompositeFramePacketTextured(pkt, dc, tex)
		if st.TexturesPainted != 1 {
			t.Fatalf("frame %d: TexturesPainted=%d want 1", frame, st.TexturesPainted)
		}
		if len(st.DamageRects) != 1 || st.DamageRects[0] != want {
			t.Fatalf("frame %d: DamageRects=%v want [%v] (quad mapped by the composite CTM)", frame, st.DamageRects, want)
		}
		if got := tex.Len(); got != 0 {
			t.Fatalf("frame %d: cache entries=%d want 0 (a texture layer is never raster-cached)", frame, got)
		}
	}
	if len(src.calls) != 2 {
		t.Fatalf("source calls=%d want one per frame", len(src.calls))
	}
	if _, n := scene.CollectCacheableKeys(pkt); n != 0 {
		t.Fatalf("cacheable keys=%d want 0 (texture layers are not cache candidates)", n)
	}
	if n := scene.CountCacheablePictureLayers(pkt); n != 0 {
		t.Fatalf("CountCacheablePictureLayers=%d want 0", n)
	}
}

// TestTextured_TextureLayerMissPaintsAndDamagesNothing: a layer naming a gone
// producer covers no pixels, so it must not add damage either — re-uploading a
// region nobody painted costs bandwidth and changes nothing on screen.
func TestTextured_TextureLayerMissPaintsAndDamagesNothing(t *testing.T) {
	scene.ResetLayerIDGen()
	src := &paintProbeSource{r: 1, g: 0, b: 0}
	id := render.RegisterTextureSource(src)
	render.UnregisterTextureSource(id)

	b := scene.NewLayerBuilder()
	b.AddTexture(id, 0, 0, 40, 30)
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tex := scene.NewPictureTextureCache(dc, 0)

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if st.TexturesPainted != 0 || st.TexturesMissed != 1 {
		t.Fatalf("texture stats painted=%d missed=%d want 0/1", st.TexturesPainted, st.TexturesMissed)
	}
	if len(st.DamageRects) != 0 {
		t.Fatalf("DamageRects=%v want none for a layer that painted nothing", st.DamageRects)
	}
	if rr, gg, bb := sampleAt(dc.Image(), 10, 10); rr < 0xC000 || gg < 0xC000 || bb < 0xC000 {
		t.Fatalf("(10,10)=#%04x%04x%04x want untouched white", rr, gg, bb)
	}
}
