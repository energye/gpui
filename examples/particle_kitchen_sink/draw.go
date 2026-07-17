//go:build linux && !nogpu

package main

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

// stageRect returns non-fullscreen particle stage centered in window.
func stageRect(fw, fh, region float64) (x, y, w, h float64) {
	if region <= 0.2 {
		region = 0.65
	}
	if region > 1 {
		region = 1
	}
	w = fw * region
	h = fh * region
	x = (fw - w) * 0.5
	y = (fh-h)*0.5 + 10
	return
}

type glowRT struct {
	dc   *render.Context
	img  *render.ImageBuf
	w, h int
	// presentW/H: on-screen footprint (may be larger than offscreen RT for
	// downsample-blur quality pattern used by games/Skia blur approximations).
	presentW, presentH int
}

func (g *glowRT) ensure(w, h int) *render.Context {
	if w < 8 {
		w = 8
	}
	if h < 8 {
		h = 8
	}
	if g.dc != nil && g.w == w && g.h == h {
		return g.dc
	}
	if g.dc != nil {
		_ = g.dc.Close()
		g.dc = nil
	}
	g.dc = render.NewContext(w, h)
	// Continuous filtered offscreen: skip 4x MSAA resolve every recompute.
	g.dc.SetEffectSurface(true)
	g.w, g.h = w, h
	g.img = nil
	return g.dc
}

func (g *glowRT) publish() *render.ImageBuf {
	if g == nil || g.dc == nil {
		return nil
	}
	// Prefer GPU-resident filter result — zero Export/readback present path.
	if _, _, _, ok := g.dc.GPUFilterTexture(); ok {
		return nil
	}
	if !g.dc.ExportImageBuf(&g.img) {
		return nil
	}
	// Retained-layer cache: keep GenerationID stable across non-recompute
	// frames so DrawImage hits GPU image cache (mem_anim effectRT pattern).
	// Export already NotifyPixelsChanged; do NOT MarkEphemeral here.
	return g.img
}

func (g *glowRT) hasCached() bool {
	if g == nil {
		return false
	}
	if g.dc != nil {
		if _, _, _, ok := g.dc.GPUFilterTexture(); ok {
			return true
		}
	}
	return g.img != nil && g.w > 0 && g.h > 0
}

func (g *glowRT) presentCached(dc *render.Context, x, y float64) bool {
	return g.presentCachedOpacity(dc, x, y, 1)
}

func (g *glowRT) presentCachedOpacity(dc *render.Context, x, y float64, opacity float64) bool {
	if g == nil || dc == nil {
		return false
	}
	if opacity <= 0 {
		opacity = 1
	}
	dw, dh := g.w, g.h
	if g.presentW > 0 && g.presentH > 0 {
		dw, dh = g.presentW, g.presentH
	}
	// Engine GPU filter publish: composite texture directly (no ImageBuf upload).
	if g.dc != nil {
		if view, _, _, ok := g.dc.GPUFilterTexture(); ok {
			dc.DrawGPUTextureWithOpacity(view, x, y, dw, dh, float32(opacity))
			return true
		}
	}
	if g.img == nil {
		return false
	}
	dc.DrawImageEx(g.img, render.DrawImageOptions{
		X: x, Y: y, DstWidth: float64(dw), DstHeight: float64(dh),
		Opacity: opacity, Interpolation: render.InterpBilinear,
	})
	return true
}

func (g *glowRT) close() {
	if g == nil {
		return
	}
	if g.dc != nil {
		_ = g.dc.Close()
		g.dc = nil
	}
	g.img = nil
	g.w, g.h = 0, 0
}

var (
	gGlow      glowRT
	gDigGrad   glowRT
	gDigFilter glowRT
	gDigBlend  glowRT
	gDigImage  *render.ImageBuf
	gAtlas     *render.ImageBuf
	gChecker   *render.ImageBuf
	gCheckW    int
	gCheckH    int
	gMeshP     []render.Point
	gMeshC     []render.RGBA
	gMeshI     []uint16
	gGlowP     []render.Point
	gGlowC     []render.RGBA
	gGlowI     []uint16
	gDiscP     []render.Point
	gDiscC     []render.RGBA
	gDiscI     []uint16
	gWaveP     []render.Point
	gWaveC     []render.RGBA
	gWaveI     []uint16
	gSprites   []render.AtlasSprite
)

func ensureAtlas() *render.ImageBuf {
	if gAtlas != nil {
		return gAtlas
	}
	img, err := render.NewImageBuf(32, 32, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := float64(x-16), float64(y-16)
			d := math.Sqrt(dx*dx + dy*dy)
			if d < 12 {
				a := uint8(255 * (1 - d/12))
				_ = img.SetRGBA(x, y, 255, 210, 90, a)
			} else {
				_ = img.SetRGBA(x, y, 0, 0, 0, 0)
			}
		}
	}
	gAtlas = img
	return gAtlas
}

// ensureChecker builds a static light checkerboard once per stage size.
// Per-frame hundreds of DrawRectangle were pure submit noise.
// Memory of this host ImageBuf is secondary; engine VRAM/pools are the real target.
func ensureChecker(w, h int) *render.ImageBuf {
	if w < 8 {
		w = 8
	}
	if h < 8 {
		h = 8
	}
	if gChecker != nil && gCheckW == w && gCheckH == h {
		return gChecker
	}
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	const cell = 18
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if ((x/cell)+(y/cell))&1 == 0 {
				_ = img.SetRGBA(x, y, 209, 214, 224, 255) // light
			} else {
				_ = img.SetRGBA(x, y, 184, 189, 199, 255) // mid
			}
		}
	}
	gChecker = img
	gCheckW, gCheckH = w, h
	return gChecker
}

func fillMeshTris(start, count int, sim *simWorld, sx, sy float64, aScale float64) {
	if count <= 0 {
		return
	}
	needV := count * 3
	if cap(gMeshP) < needV {
		gMeshP = make([]render.Point, needV)
		gMeshC = make([]render.RGBA, needV)
		gMeshI = make([]uint16, count*3)
	} else {
		gMeshP = gMeshP[:needV]
		gMeshC = gMeshC[:needV]
		gMeshI = gMeshI[:count*3]
	}
	for j := 0; j < count; j++ {
		p := sim.ps[start+j]
		cx, cy := sx+p.x, sy+p.y
		rr := p.r * 2.2
		i0 := j * 3
		gMeshP[i0] = render.Point{X: cx, Y: cy - rr}
		gMeshP[i0+1] = render.Point{X: cx - rr*0.9, Y: cy + rr*0.6}
		gMeshP[i0+2] = render.Point{X: cx + rr*0.9, Y: cy + rr*0.6}
		cr, cg, cb := hsvRGB(p.hue, 0.75, 0.98)
		col := render.RGBA{R: cr, G: cg, B: cb, A: aScale}
		gMeshC[i0], gMeshC[i0+1], gMeshC[i0+2] = col, col, col
		gMeshI[i0] = uint16(i0)
		gMeshI[i0+1] = uint16(i0 + 1)
		gMeshI[i0+2] = uint16(i0 + 2)
	}
}

// fillStageDiscs builds disc meshes in stage/world coordinates (solid path batch).
func fillStageDiscs(sim *simWorld, count int, sx, sy, t, rScale, alpha float64) {
	// Match former path-disc coloring: hsv(hue+t*0.02, 0.85, 0.98).
	fillStageDiscsStride(sim, count, 0, 1, sx, sy, t, rScale, alpha, t*0.02, 0.85, 0.98)
}

// fillStageDiscsStride fills discs for indices start,start+step,... < count.
func fillStageDiscsStride(sim *simWorld, count, start, step int, sx, sy, t, rScale, alpha, hueOff, sat, val float64) {
	const segs = 10
	if count <= 0 || step <= 0 {
		gDiscP = gDiscP[:0]
		gDiscC = gDiscC[:0]
		gDiscI = gDiscI[:0]
		return
	}
	n := 0
	for i := start; i < count; i += step {
		n++
	}
	needV := n * (segs + 1)
	needI := n * segs * 3
	if cap(gDiscP) < needV {
		gDiscP = make([]render.Point, needV)
		gDiscC = make([]render.RGBA, needV)
	} else {
		gDiscP = gDiscP[:needV]
		gDiscC = gDiscC[:needV]
	}
	if cap(gDiscI) < needI {
		gDiscI = make([]uint16, needI)
	} else {
		gDiscI = gDiscI[:needI]
	}
	j := 0
	for i := start; i < count; i += step {
		p := sim.ps[i]
		cx, cy := sx+p.x, sy+p.y
		rr := p.r * rScale
		cr, cg, cb := hsvRGB(p.hue+hueOff, sat, val)
		col := render.RGBA{R: cr, G: cg, B: cb, A: alpha}
		base := j * (segs + 1)
		gDiscP[base] = render.Point{X: cx, Y: cy}
		gDiscC[base] = col
		for s := 0; s < segs; s++ {
			ang := float64(s) * (2 * math.Pi / float64(segs))
			gDiscP[base+1+s] = render.Point{X: cx + rr*math.Cos(ang), Y: cy + rr*math.Sin(ang)}
			gDiscC[base+1+s] = col
			i0 := j*segs*3 + s*3
			gDiscI[i0] = uint16(base)
			gDiscI[i0+1] = uint16(base + 1 + s)
			next := s + 1
			if next == segs {
				next = 0
			}
			gDiscI[i0+2] = uint16(base + 1 + next)
		}
		j++
	}
}

// fillGlowDiscs builds a single colored mesh of N disc approximations (segs
// triangles each) for the glow offscreen RT — same sample count/colors as the
// former per-circle Fill path, one DrawMesh instead of N path submits.
func fillGlowDiscs(sim *simWorld, samples int, gw, gh int, sw, sh float64) {
	const segs = 10
	if samples <= 0 {
		gGlowP = gGlowP[:0]
		gGlowC = gGlowC[:0]
		gGlowI = gGlowI[:0]
		return
	}
	needV := samples * (segs + 1)
	needI := samples * segs * 3
	if cap(gGlowP) < needV {
		gGlowP = make([]render.Point, needV)
		gGlowC = make([]render.RGBA, needV)
	} else {
		gGlowP = gGlowP[:needV]
		gGlowC = gGlowC[:needV]
	}
	if cap(gGlowI) < needI {
		gGlowI = make([]uint16, needI)
	} else {
		gGlowI = gGlowI[:needI]
	}
	for j := 0; j < samples; j++ {
		p := sim.ps[j]
		u := p.x / sw
		v := p.y / sh
		cx := u * float64(gw)
		cy := v * float64(gh)
		rr := 6 + p.r*0.3
		cr, cg, cb := hsvRGB(p.hue, 0.35, 1)
		col := render.RGBA{R: cr, G: cg, B: cb, A: 0.9}
		base := j * (segs + 1)
		gGlowP[base] = render.Point{X: cx, Y: cy}
		gGlowC[base] = col
		for s := 0; s < segs; s++ {
			ang := float64(s) * (2 * math.Pi / float64(segs))
			gGlowP[base+1+s] = render.Point{X: cx + rr*math.Cos(ang), Y: cy + rr*math.Sin(ang)}
			gGlowC[base+1+s] = col
			i0 := j*segs*3 + s*3
			gGlowI[i0] = uint16(base)
			gGlowI[i0+1] = uint16(base + 1 + s)
			next := s + 1
			if next == segs {
				next = 0
			}
			gGlowI[i0+2] = uint16(base + 1 + next)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func drawFrame(dc *render.Context, fonts fontPack, cfg featureConfig, sim *simWorld, fw, fh, t float64, frame int) (string, int) {
	// Window chrome (optional alternating clear to stress full redraw path)
	if cfg.AltClear && frame%2 == 1 {
		dc.SetRGB(0.12, 0.05, 0.08)
	} else {
		dc.SetRGB(0.07, 0.08, 0.11)
	}
	dc.DrawRectangle(0, 0, fw, fh)
	_ = dc.Fill()

	sx, sy, sw, sh := stageRect(fw, fh, cfg.Region)

	// Stage: LIGHT checker (static ImageBuf) so Multiply/Screen/alpha are visible.
	dc.SetRGB(0.82, 0.84, 0.88)
	dc.DrawRoundedRectangle(sx-2, sy-2, sw+4, sh+4, 10)
	_ = dc.Fill()
	if chk := ensureChecker(int(sw), int(sh)); chk != nil {
		dc.DrawImageEx(chk, render.DrawImageOptions{
			X: sx, Y: sy, DstWidth: sw, DstHeight: sh,
			Opacity: 1, Interpolation: render.InterpNearest,
		})
	} else {
		// Fallback: 4 large tiles (never hundreds of cells).
		dc.SetRGBA(0.72, 0.74, 0.78, 1)
		dc.DrawRectangle(sx, sy, sw*0.5, sh*0.5)
		_ = dc.Fill()
		dc.DrawRectangle(sx+sw*0.5, sy+sh*0.5, sw*0.5, sh*0.5)
		_ = dc.Fill()
	}
	dc.SetRGBA(0.25, 0.45, 0.85, 0.85)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(sx-2, sy-2, sw+4, sh+4, 10)
	_ = dc.Stroke()

	dc.Push()
	dc.ClipRect(sx, sy, sw, sh)

	// Optional translucent layer card(s)
	if cfg.Layer {
		nLayer := cfg.MultiLayer
		if nLayer <= 0 {
			nLayer = 1
		}
		if nLayer > 6 {
			nLayer = 6
		}
		for li := 0; li < nLayer; li++ {
			alpha := 0.35 / float64(nLayer)
			dc.PushLayer(render.BlendNormal, 0.55+0.1*float64(li))
			hue := float64(li) * 0.17
			r, g, b := hsvRGB(hue, 0.55, 0.95)
			offset := float64(li) * 10
			dc.SetRGBA(r, g, b, alpha+0.12)
			dc.DrawRoundedRectangle(sx+16+offset, sy+16+offset, sw-32-2*offset, sh-32-2*offset, 14)
			_ = dc.Fill()
			dc.PopLayer()
		}
	}

	// ---- BASE particles (dense, always visible; mesh is GPU-cheap) ----
	// Do NOT gut particle count to hit FPS — fix advanced-blend engine instead.
	n := len(sim.ps)
	baseMesh := n
	if baseMesh > 4000 {
		baseMesh = 4000
	}
	// Dense mesh batch is the GPU-cheap path. PathSubmitHeavy deliberately skips
	// it to surface path encode/submit costs (do not "fix" by forcing mesh here).
	if !cfg.PathSubmitHeavy && (cfg.Solid || cfg.Mesh || cfg.Blend || cfg.Glow || cfg.Atlas) {
		fillMeshTris(0, baseMesh, sim, sx, sy, 0.95)
		dc.SetBlendMode(render.BlendNormal)
		dc.SetAntiAlias(true)
		dc.DrawMesh(render.Mesh{Positions: gMeshP, Colors: gMeshC, Indices: gMeshI})
	}

	// Solid discs / path-submit storm
	if cfg.Solid || cfg.PathSubmitHeavy {
		nCirc := n
		capC := 200
		if cfg.PathSubmitHeavy {
			// many independent path fills — diagnostic for submit cost
			capC = minInt(n, 420)
		} else if cfg.Blend {
			capC = 120
		}
		if nCirc > capC {
			nCirc = capC
		}
		dc.SetBlendMode(render.BlendNormal)
		dc.SetAntiAlias(true)
		if cfg.PathSubmitHeavy {
			for i := 0; i < nCirc; i++ {
				p := sim.ps[i]
				cr, cg, cb := hsvRGB(p.hue+t*0.02, 0.85, 0.98)
				dc.SetRGBA(cr, cg, cb, 0.92)
				dc.DrawCircle(sx+p.x, sy+p.y, p.r*1.4)
				_ = dc.Fill()
			}
		} else {
			// Same sample count/colors/radii as path discs, one DrawMesh (Skia-class
			// batching). PathSubmitHeavy keeps the slow per-path path for diagnosis.
			fillStageDiscs(sim, nCirc, sx, sy, t, 1.4, 0.92)
			dc.DrawMesh(render.Mesh{Positions: gDiscP, Colors: gDiscC, Indices: gDiscI})
		}
	}

	// Trails (bounded)
	if cfg.Trails {
		dc.SetLineWidth(1.2)
		nt := minInt(n, 120)
		for i := 0; i < nt; i++ {
			base := i * sim.trailN
			r, g, b := hsvRGB(sim.ps[i].hue, 0.55, 0.85)
			dc.SetRGBA(r, g, b, 0.35)
			dc.NewSubPath()
			dc.MoveTo(sx+sim.trailX[base+sim.trailN-1], sy+sim.trailY[base+sim.trailN-1])
			for k := sim.trailN - 2; k >= 0; k-- {
				dc.LineTo(sx+sim.trailX[base+k], sy+sim.trailY[base+k])
			}
			_ = dc.Stroke()
		}
	}

	// ---- BLEND: full-stage density + advanced modes ----
	// Default: layer-batched Screen/Multiply (engine resolve at Present).
	// PerCircleBlend trap: per-circle SetBlendMode — historically ~1fps via Poll dual-tex.
	if cfg.Blend {
		bc := cfg.BlendCircles
		if bc <= 0 {
			bc = 96
		}
		if bc > 300 {
			bc = 300
		}
		if bc > n {
			bc = n
		}
		// Semi-transparent density via mesh (GPU batch) — not hundreds of path fills.
		alphaMesh := minInt(n, 800)
		fillMeshTris(0, alphaMesh, sim, sx, sy, 0.38)
		dc.SetBlendMode(render.BlendNormal)
		dc.DrawMesh(render.Mesh{Positions: gMeshP, Colors: gMeshC, Indices: gMeshI})

		if cfg.PerCircleBlend {
			// REGRESSION TRAP — do not "fix" by disabling this probe.
			for i := 0; i < bc; i++ {
				p := sim.ps[i]
				if i%2 == 0 {
					dc.SetBlendMode(render.BlendScreen)
				} else {
					dc.SetBlendMode(render.BlendMultiply)
				}
				cr, cg, cb := hsvRGB(p.hue+0.2, 0.9, 1)
				dc.SetRGBA(cr, cg, cb, 0.85)
				dc.DrawCircle(sx+p.x, sy+p.y, p.r*1.55)
				_ = dc.Fill()
			}
			dc.SetBlendMode(render.BlendNormal)
		} else {
			// Screen / Multiply groups — same sample counts, mesh-batched discs
			// inside one advanced layer each (not per-circle path submits).
			dc.PushLayer(render.BlendScreen, 1.0)
			dc.SetBlendMode(render.BlendNormal)
			fillStageDiscsStride(sim, bc, 0, 2, sx, sy, t, 1.6, 0.85, 0.15, 0.9, 1)
			dc.DrawMesh(render.Mesh{Positions: gDiscP, Colors: gDiscC, Indices: gDiscI})
			dc.PopLayer()
			dc.PushLayer(render.BlendMultiply, 1.0)
			dc.SetBlendMode(render.BlendNormal)
			fillStageDiscsStride(sim, bc, 1, 2, sx, sy, t, 1.5, 0.8, 0.35, 0.85, 0.9)
			dc.DrawMesh(render.Mesh{Positions: gDiscP, Colors: gDiscC, Indices: gDiscI})
			dc.PopLayer()
			dc.SetBlendMode(render.BlendNormal)
		}
	}

	// Extra mesh stress when Mesh flag on (second wave of triangles)
	if cfg.Mesh && n > 200 {
		extra := n - 200
		if extra > 400 {
			extra = 400
		}
		fillMeshTris(200, extra, sim, sx, sy, 0.55)
		dc.SetBlendMode(render.BlendNormal)
		dc.DrawMesh(render.Mesh{Positions: gMeshP, Colors: gMeshC, Indices: gMeshI})
	}

	// Atlas sprites
	if cfg.Atlas {
		if atlas := ensureAtlas(); atlas != nil {
			m := minInt(n, 64)
			if cap(gSprites) < m {
				gSprites = make([]render.AtlasSprite, m)
			} else {
				gSprites = gSprites[:m]
			}
			for i := 0; i < m; i++ {
				p := sim.ps[i]
				gSprites[i] = render.AtlasSprite{
					SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32,
					DstX: sx + p.x - 10, DstY: sy + p.y - 10,
					DstW: 20, DstH: 20,
					Opacity: 0.7 + 0.25*math.Abs(math.Sin(t+p.seed)),
				}
			}
			dc.DrawAtlas(atlas, gSprites)
		}
	}

	// Glow RT overlay (bounded offscreen + real ApplyBlur + retained present).
	// Content density unchanged: stage-relative RT size, 24 samples, blur=2.
	// Hitch/memory are engine concerns (Flush/Export/image cache/filter pool).
	if cfg.Glow {
		// On-screen footprint (unchanged visual size).
		visW, visH := int(sw*0.28), int(sh*0.22)
		if visW < 64 {
			visW = 64
		}
		if visH < 48 {
			visH = 48
		}
		// Downsample-blur pattern (Skia/game common): process at ~0.5x, upsample
		// on present. Sample count and visual coverage unchanged; pixels in blur
		// drop ~4x → less CPU/GPU without gutting content.
		gw, gh := visW/2, visH/2
		if gw < 32 {
			gw = 32
		}
		if gh < 24 {
			gh = 24
		}
		gGlow.presentW, gGlow.presentH = visW, visH
		rt := gGlow.ensure(gw, gh)
		rt.ClearWithColor(render.Transparent)
		samples := minInt(n, 24)
		fillGlowDiscs(sim, samples, gw, gh, sw, sh)
		rt.SetBlendMode(render.BlendNormal)
		rt.SetAntiAlias(true)
		rt.DrawMesh(render.Mesh{Positions: gGlowP, Colors: gGlowC, Indices: gGlowI})
		rt.ApplyBlur(2)
		_ = gGlow.publish()
		_ = gGlow.presentCachedOpacity(dc, sx+sw*0.36, sy+sh*0.38, 0.62)
	}

	dc.Pop() // clip

	// Outside-stage live marker (always)
	markers := 0
	dc.SetRGBA(1, 0.7, 0.2, 1)
	dc.DrawCircle(fw*0.08+14*math.Sin(t*2.2), fh*0.9, 8)
	_ = dc.Fill()
	markers++

	// Stage corner badges so operator knows content path ran
	dc.SetRGBA(0.1, 0.7, 0.35, 0.95)
	dc.DrawCircle(sx+14, sy+14, 6)
	_ = dc.Fill()
	markers++
	dc.SetRGBA(0.9, 0.25, 0.2, 0.95)
	dc.DrawCircle(sx+sw-14, sy+14, 6)
	_ = dc.Fill()
	markers++

	// Feature markers (visible + counted)
	if cfg.Blend {
		dc.SetRGBA(0.2, 0.85, 1.0, 0.95)
		dc.DrawRectangle(sx+sw-28, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers++
	}
	if cfg.Glow {
		dc.SetRGBA(1.0, 0.55, 0.15, 0.95)
		dc.DrawRectangle(sx+sw-48, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers++
	}
	if cfg.Layer {
		dc.SetRGBA(0.75, 0.35, 0.95, 0.9)
		dc.DrawRectangle(sx+sw-68, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers++
	}
	if cfg.PerCircleBlend {
		dc.SetRGBA(1, 0.2, 0.4, 1)
		dc.DrawCircle(sx+sw*0.5, sy+18, 7)
		_ = dc.Fill()
		markers++
	}
	if cfg.ResizeOscillate {
		dc.SetRGBA(0.95, 0.95, 0.2, 1)
		dc.DrawRectangle(sx+8, sy+sh-22, 22, 12)
		_ = dc.Fill()
		markers++
	}
	if cfg.PathSubmitHeavy {
		dc.SetRGBA(0.3, 1, 0.5, 1)
		dc.DrawRectangle(sx+34, sy+sh-22, 22, 12)
		_ = dc.Fill()
		markers++
	}
	if cfg.AltClear {
		dc.SetRGBA(1, 0.4, 0.7, 1)
		dc.DrawRectangle(sx+60, sy+sh-22, 22, 12)
		_ = dc.Fill()
		markers++
	}

	// Dig axes after stage clip is popped so labels/markers stay visible.
	markers += drawDigFeatures(dc, fonts, cfg, sx, sy, sw, sh, t, frame)

	if cfg.Text {
		ensureFont(dc, fonts, 13)
		dc.SetRGBA(0.95, 0.97, 1, 1)
		dc.DrawString(fmt.Sprintf("粒子舞台 region=%.0f%% N=%d 彩色三角+圆+半透明；右下色块=功能标记", cfg.Region*100, cfg.ParticleN), sx, sy-10)
		if cfg.TextBi {
			ensureFont(dc, fonts, 15)
			dc.SetRGBA(1, 0.95, 0.55, 1)
			dc.DrawString("中文渲染 Chinese + English Latin 012345", sx+8, sy+22)
			dc.SetRGBA(0.55, 0.9, 1, 1)
			dc.DrawString("ABCDEFG abcdefg 混合排版 Mix", sx+8, sy+42)
			markers++
		}
	}

	return cfg.Expect, markers
}

// drawDigFeatures exercises Skia-facing APIs that particle axes alone miss.
// Returns extra marker count so contentProbe still sees real work.
func drawDigFeatures(dc *render.Context, fonts fontPack, cfg featureConfig, sx, sy, sw, sh, t float64, frame int) int {
	markers := 0
	if cfg.Clip {
		// Nested moving clip windows over stage content already drawn? We draw
		// accent shapes inside nested clips so empty/wrong clip is visible.
		dc.Push()
		dc.ClipRect(sx+sw*0.08+8*math.Sin(t), sy+sh*0.08, sw*0.42, sh*0.42)
		dc.ClipRect(sx+sw*0.18, sy+sh*0.16+6*math.Cos(t*1.3), sw*0.38, sh*0.38)
		dc.SetRGBA(0.15, 0.75, 0.95, 0.55)
		dc.DrawRoundedRectangle(sx+sw*0.12, sy+sh*0.12, sw*0.4, sh*0.4, 18)
		_ = dc.Fill()
		dc.SetRGBA(1, 0.85, 0.2, 0.9)
		dc.DrawCircle(sx+sw*0.28+10*math.Sin(t*2), sy+sh*0.3, 14)
		_ = dc.Fill()
		dc.Pop()
		// Clip marker
		dc.SetRGBA(0.2, 0.9, 0.85, 1)
		dc.DrawRectangle(sx+sw-88, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers += 2
	}

	if cfg.EvenOdd {
		cx, cy, r := sx+sw*0.22, sy+sh*0.72, 34.0
		dc.SetFillRule(render.FillRuleEvenOdd)
		dc.SetRGBA(0.95, 0.35, 0.55, 0.9)
		dc.DrawCircle(cx, cy, r)
		dc.DrawCircle(cx, cy, r*0.45)
		_ = dc.Fill()
		dc.SetFillRule(render.FillRuleNonZero)
		dc.SetRGBA(0.35, 0.85, 0.45, 0.9)
		dc.DrawCircle(cx+78, cy, r)
		dc.DrawCircle(cx+78, cy, r*0.45)
		_ = dc.Fill()
		markers += 2
	}

	if cfg.Dash {
		dc.SetRGBA(0.95, 0.95, 1, 0.95)
		dc.SetLineWidth(2.5)
		dc.SetDash(10, 6, 3, 6)
		dc.SetDashOffset(math.Mod(t*40, 30))
		dc.DrawRoundedRectangle(sx+sw*0.55, sy+sh*0.62, sw*0.35, sh*0.22, 10)
		_ = dc.Stroke()
		dc.SetDash() // clear
		markers++
	}

	if cfg.Xform {
		dc.Push()
		dc.Translate(sx+sw*0.78, sy+sh*0.28)
		dc.Rotate(t * 0.7)
		dc.Scale(1+0.08*math.Sin(t*2), 1+0.08*math.Cos(t*2))
		dc.SetRGBA(1, 0.45, 0.2, 0.85)
		dc.DrawRectangle(-22, -14, 44, 28)
		_ = dc.Fill()
		dc.SetRGBA(0.2, 0.85, 1, 0.9)
		dc.DrawCircle(0, 0, 10)
		_ = dc.Fill()
		dc.Pop()
		// small stack storm
		for i := 0; i < 6; i++ {
			dc.Push()
			dc.Translate(sx+sw*0.1+float64(i)*18, sy+sh*0.88)
			dc.Rotate(t + float64(i)*0.4)
			dc.SetRGBA(0.6+0.05*float64(i), 0.3, 1-0.08*float64(i), 0.75)
			dc.DrawRectangle(-6, -6, 12, 12)
			_ = dc.Fill()
			dc.Pop()
		}
		markers += 2
	}

	if cfg.ImagePx {
		if gDigImage == nil {
			img, err := render.NewImageBuf(48, 48, render.FormatRGBA8)
			if err == nil && img != nil {
				for y := 0; y < 48; y++ {
					for x := 0; x < 48; x++ {
						_ = img.SetRGBA(x, y, uint8(40+x*4), uint8(80+y*3), 200, 255)
					}
				}
				// diagonal mark
				for i := 0; i < 48; i++ {
					_ = img.SetRGBA(i, i, 255, 240, 40, 255)
				}
				gDigImage = img
			}
		}
		if gDigImage != nil {
			ox := sx + sw*0.72 + 6*math.Sin(t)
			oy := sy + sh*0.72
			dc.DrawImage(gDigImage, ox, oy)
			markers++
		}
	}

	if cfg.Grad {
		tw, th := 220, 120
		if frame%3 != 0 && gDigGrad.hasCached() {
			_ = gDigGrad.presentCached(dc, sx+12, sy+sh*0.55)
		} else {
			rt := gDigGrad.ensure(tw, th)
			rt.ClearWithColor(render.RGBA{R: 0.08, G: 0.09, B: 0.12, A: 1})
			lin := render.NewLinearGradientBrush(8, 8, float64(tw)-8, 40).
				AddColorStop(0, render.RGBA{R: 1, G: 0.3, B: 0.2, A: 1}).
				AddColorStop(0.5, render.RGBA{R: 1, G: 0.9, B: 0.2, A: 1}).
				AddColorStop(1, render.RGBA{R: 0.2, G: 0.8, B: 1, A: 1})
			rt.SetFillBrush(lin)
			rt.DrawRoundedRectangle(8, 8, float64(tw-16), 36, 8)
			_ = rt.Fill()
			rad := render.NewRadialGradientBrush(70, 78, 0, 28).
				AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
				AddColorStop(1, render.RGBA{R: 0.15, G: 0.25, B: 0.55, A: 1})
			rt.SetFillBrush(rad)
			rt.DrawCircle(70, 78, 28)
			_ = rt.Fill()
			swp := render.NewSweepGradientBrush(160, 78, t*0.8).
				AddColorStop(0, render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1}).
				AddColorStop(0.5, render.RGBA{R: 0.2, G: 1, B: 0.4, A: 1}).
				AddColorStop(1, render.RGBA{R: 0.3, G: 0.4, B: 1, A: 1})
			rt.SetFillBrush(swp)
			rt.DrawCircle(160, 78, 28)
			_ = rt.Fill()
			if img := gDigGrad.publish(); img != nil {
				dc.DrawImageEx(img, render.DrawImageOptions{
					X: sx + 12, Y: sy + sh*0.55,
					DstWidth: float64(tw), DstHeight: float64(th),
					Opacity: 0.95, Interpolation: render.InterpBilinear,
				})
			}
		}
		dc.SetRGBA(1, 0.75, 0.2, 1)
		dc.DrawRectangle(sx+sw-108, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers += 2
	}

	if cfg.Filter {
		tw, th := 140, 90
		if frame%2 == 1 && gDigFilter.hasCached() {
			_ = gDigFilter.presentCached(dc, sx+sw*0.55, sy+12)
		} else {
			rt := gDigFilter.ensure(tw, th)
			rt.ClearWithColor(render.RGBA{R: 0.12, G: 0.14, B: 0.2, A: 1})
			rt.SetRGBA(1, 0.55, 0.2, 0.95)
			rt.DrawCircle(float64(tw)*0.35+8*math.Sin(t), float64(th)*0.5, 22)
			_ = rt.Fill()
			rt.SetRGBA(0.3, 0.7, 1, 0.9)
			rt.DrawRoundedRectangle(float64(tw)*0.45, float64(th)*0.28, 48, 36, 8)
			_ = rt.Fill()
			rt.ApplyBlur(3)
			_ = gDigFilter.publish()
			_ = gDigFilter.presentCachedOpacity(dc, sx+sw*0.55, sy+12, 0.92)
		}
		dc.SetRGBA(1, 0.4, 0.85, 1)
		dc.DrawRectangle(sx+sw-128, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers += 2
	}

	if cfg.BlendSep {
		tw, th := 200, 110
		if frame%3 == 0 || !gDigBlend.hasCached() {
			rt := gDigBlend.ensure(tw, th)
			rt.ClearWithColor(render.RGBA{R: 0.28, G: 0.36, B: 0.52, A: 1})
			rt.SetRGBA(0.92, 0.92, 0.96, 0.35)
			rt.DrawRectangle(0, 0, float64(tw)/2, float64(th)/2)
			_ = rt.Fill()
			rt.DrawRectangle(float64(tw)/2, float64(th)/2, float64(tw)/2, float64(th)/2)
			_ = rt.Fill()
			rt.SetBlendMode(render.BlendMultiply)
			rt.SetRGBA(1, 0.45, 0.1, 0.9)
			rt.DrawCircle(float64(tw)*0.35+10*math.Sin(t), float64(th)*0.5, 30)
			_ = rt.Fill()
			rt.SetBlendMode(render.BlendScreen)
			rt.SetRGBA(0.2, 0.55, 1, 0.85)
			rt.DrawCircle(float64(tw)*0.6+10*math.Cos(t), float64(th)*0.5, 30)
			_ = rt.Fill()
			rt.SetBlendMode(render.BlendOverlay)
			rt.SetRGBA(0.95, 0.9, 0.25, 0.75)
			rt.DrawRoundedRectangle(float64(tw)*0.35, float64(th)*0.32+6*math.Sin(t*1.4), 55, 34, 8)
			_ = rt.Fill()
			rt.SetBlendMode(render.BlendNormal)
			if img := gDigBlend.publish(); img != nil {
				dc.DrawImageEx(img, render.DrawImageOptions{
					X: sx + sw*0.28, Y: sy + 16,
					DstWidth: float64(tw), DstHeight: float64(th),
					Opacity: 1, Interpolation: render.InterpBilinear,
				})
			}
		} else {
			_ = gDigBlend.presentCached(dc, sx+sw*0.28, sy+16)
		}
		dc.SetRGBA(0.4, 1, 0.55, 1)
		dc.DrawRectangle(sx+sw-148, sy+sh-28, 16, 16)
		_ = dc.Fill()
		markers += 2
	}

	if cfg.MeshWave {
		const cols, rows = 56, 32
		nVert := (cols + 1) * (rows + 1)
		nIdx := cols * rows * 6
		if cap(gWaveP) < nVert {
			gWaveP = make([]render.Point, nVert)
			gWaveC = make([]render.RGBA, nVert)
		} else {
			gWaveP = gWaveP[:nVert]
			gWaveC = gWaveC[:nVert]
		}
		if cap(gWaveI) < nIdx {
			gWaveI = make([]uint16, nIdx)
		} else {
			gWaveI = gWaveI[:nIdx]
		}
		ox, oy := sx+sw*0.08, sy+sh*0.18
		spanW, spanH := sw*0.84, sh*0.48
		cw, ch := spanW/float64(cols), spanH/float64(rows)
		amp := ch * 1.1
		if amp < 4 {
			amp = 4
		}
		if amp > 10 {
			amp = 10
		}
		k := math.Pi * 0.95
		vi := 0
		for j := 0; j <= rows; j++ {
			fy := float64(j) / float64(rows)
			rowPhase := fy * 0.5
			for i := 0; i <= cols; i++ {
				fx := float64(i) / float64(cols)
				wave := amp * math.Sin(t*0.95+fx*k+rowPhase)
				gWaveP[vi] = render.Point{X: ox + float64(i)*cw, Y: oy + float64(j)*ch + wave}
				gWaveC[vi] = render.RGBA{R: 0.12 + 0.85*fx, G: 0.3 + 0.55*fy, B: 0.95 - 0.55*fy, A: 0.92}
				vi++
			}
		}
		ii := 0
		for j := 0; j < rows; j++ {
			for i := 0; i < cols; i++ {
				i0 := uint16(j*(cols+1) + i)
				i1 := i0 + 1
				i2 := i0 + uint16(cols+1)
				i3 := i2 + 1
				gWaveI[ii+0], gWaveI[ii+1], gWaveI[ii+2] = i0, i1, i2
				gWaveI[ii+3], gWaveI[ii+4], gWaveI[ii+5] = i1, i3, i2
				ii += 6
			}
		}
		dc.SetAntiAlias(true)
		dc.DrawMesh(render.Mesh{Positions: gWaveP, Colors: gWaveC, Indices: gWaveI})
		markers += 2
	}

	_ = fonts
	return markers
}

// textBiFingerprint draws CJK + Latin into an offscreen RT and checks both
// bands have non-empty ink. Catches missing Latin/CJK glyphs.
func textBiFingerprint(fonts fontPack) (ok bool, note string) {
	const W, H = 320, 64
	rt := render.NewContext(W, H)
	if rt == nil {
		return false, "nil_rt"
	}
	defer rt.Close()
	rt.BeginFrame()
	rt.SetRGB(0, 0, 0)
	rt.DrawRectangle(0, 0, W, H)
	_ = rt.Fill()
	ensureFont(rt, fonts, 18)
	rt.SetRGBA(1, 1, 1, 1)
	rt.DrawString("中文渲染测试", 8, 24)
	rt.DrawString("English Latin ABC xyz 0123", 8, 48)
	var img *render.ImageBuf
	if !rt.ExportImageBuf(&img) || img == nil {
		return false, "export_fail"
	}
	w, h := img.Bounds()
	data := img.Data()
	if len(data) < w*h*4 {
		if pd := img.PremultipliedData(); len(pd) >= w*h*4 {
			data = pd
		} else {
			return false, "no_pixel_data"
		}
	}
	ink := func(y0, y1 int) int {
		n := 0
		for y := y0; y < y1 && y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 4
				if i+2 >= len(data) {
					continue
				}
				// non-black ink
				if data[i] > 20 || data[i+1] > 20 || data[i+2] > 20 {
					n++
				}
			}
		}
		return n
	}
	cjk := ink(8, 30)
	lat := ink(34, 56)
	if cjk < 40 {
		return false, fmt.Sprintf("cjk_empty ink=%d", cjk)
	}
	if lat < 40 {
		return false, fmt.Sprintf("latin_empty ink=%d", lat)
	}
	return true, fmt.Sprintf("ok cjk=%d latin=%d", cjk, lat)
}

func probeOK(dc *render.Context) (bool, string) {
	if dc == nil {
		return false, "nil_dc"
	}
	st := dc.RenderPathStats()
	if st.CPUFallbackOps > 0 {
		return false, "cpu_fb>0:" + st.LastCPUFallbackReason
	}
	if st.GPUOps <= 0 {
		return false, "gpu_ops=0"
	}
	return true, "ok"
}

// contentProbe checks that the frame path produced meaningful work for the
// enabled features (not empty green). Uses RenderPathStats + feature markers.
// Does NOT force full-frame CPU Image() readback on the present path.
func contentProbe(dc *render.Context, cfg featureConfig, drewMarkers int) (bool, string) {
	if dc == nil {
		return false, "nil_dc"
	}
	st := dc.RenderPathStats()
	if st.GPUOps <= 0 {
		return false, "no_gpu_ops"
	}
	if drewMarkers < 2 {
		return false, fmt.Sprintf("markers=%d want>=2", drewMarkers)
	}
	if cfg.MinParticleN > 0 && cfg.ParticleN < cfg.MinParticleN {
		return false, fmt.Sprintf("gutted_n=%d<%d", cfg.ParticleN, cfg.MinParticleN)
	}
	minOps := 1
	if cfg.Blend || cfg.Glow || cfg.Layer || cfg.Mesh || cfg.Grad || cfg.Filter || cfg.MeshWave || cfg.BlendSep {
		minOps = 3
	}
	if cfg.Clip || cfg.Dash || cfg.EvenOdd || cfg.Xform || cfg.ImagePx || cfg.TextBi {
		if minOps < 2 {
			minOps = 2
		}
	}
	if cfg.Blend && cfg.PerCircleBlend {
		minOps = 2
	}
	if st.GPUOps < minOps {
		return false, fmt.Sprintf("gpu_ops=%d want>=%d feats=%s", st.GPUOps, minOps, cfg.featuresSummary())
	}
	return true, fmt.Sprintf("ok gpu_ops=%d markers=%d", st.GPUOps, drewMarkers)
}

// pixelFingerprint draws known pure RGB patches into a tiny offscreen RT and
// samples ExportImageBuf pixels. Catches empty/wrong raster without full-window
// present-path readback every frame. Call once after warm-up.
func pixelFingerprint() (ok bool, note string, samples string) {
	const W, H = 64, 24
	rt := render.NewContext(W, H)
	if rt == nil {
		return false, "nil_rt", ""
	}
	defer rt.Close()
	rt.BeginFrame()
	rt.SetRGB(0, 0, 0)
	rt.DrawRectangle(0, 0, W, H)
	_ = rt.Fill()
	// Pure channel patches (opaque)
	rt.SetRGBA(1, 0, 0, 1)
	rt.DrawRectangle(4, 4, 16, 16)
	_ = rt.Fill()
	rt.SetRGBA(0, 1, 0, 1)
	rt.DrawRectangle(24, 4, 16, 16)
	_ = rt.Fill()
	rt.SetRGBA(0, 0, 1, 1)
	rt.DrawRectangle(44, 4, 16, 16)
	_ = rt.Fill()
	// Force software-visible pixmap for Export (RT path)
	var img *render.ImageBuf
	if !rt.ExportImageBuf(&img) || img == nil {
		// Some GPU-only contexts may need Flush; ExportImageBuf already Flushes.
		return false, "export_fail", ""
	}
	w, h := img.Bounds()
	if w < W || h < H {
		return false, fmt.Sprintf("bounds=%dx%d", w, h), ""
	}
	data := img.Data()
	if len(data) < w*h*4 {
		// PremultipliedData fallback
		if pd := img.PremultipliedData(); len(pd) >= w*h*4 {
			data = pd
		} else {
			return false, "no_pixel_data", ""
		}
	}
	sample := func(x, y int) (r, g, b, a uint8) {
		i := (y*w + x) * 4
		if i+3 >= len(data) {
			return 0, 0, 0, 0
		}
		return data[i], data[i+1], data[i+2], data[i+3]
	}
	// centers of patches
	rr, rg, rb, ra := sample(12, 12)
	gr, gg, gb, ga := sample(32, 12)
	br, bg, bb, ba := sample(52, 12)
	samples = fmt.Sprintf("R=%d,%d,%d,%d G=%d,%d,%d,%d B=%d,%d,%d,%d",
		rr, rg, rb, ra, gr, gg, gb, ga, br, bg, bb, ba)
	// Premul or straight: red patch must dominate R, green G, blue B; not black/white empty.
	redOK := ra > 200 && rr > 180 && rg < 80 && rb < 80
	greenOK := ga > 200 && gg > 180 && gr < 80 && gb < 80
	blueOK := ba > 200 && bb > 180 && br < 80 && bg < 80
	if !redOK || !greenOK || !blueOK {
		return false, "pixel_mismatch:" + samples, samples
	}
	return true, "ok " + samples, samples
}

// stageContentSignature samples a tiny offline reconstruction of stage markers
// (not present surface) to ensure marker colors are drawable through the same
// render.Context API used by the window path.
func stageContentSignature() (ok bool, note string) {
	rt := render.NewContext(48, 48)
	if rt == nil {
		return false, "nil_rt"
	}
	defer rt.Close()
	rt.BeginFrame()
	rt.SetRGB(0.82, 0.84, 0.88)
	rt.DrawRectangle(0, 0, 48, 48)
	_ = rt.Fill()
	// green + red badges like stage corners
	rt.SetRGBA(0.1, 0.7, 0.35, 0.95)
	rt.DrawCircle(12, 12, 6)
	_ = rt.Fill()
	rt.SetRGBA(0.9, 0.25, 0.2, 0.95)
	rt.DrawCircle(36, 12, 6)
	_ = rt.Fill()
	var img *render.ImageBuf
	if !rt.ExportImageBuf(&img) || img == nil {
		return false, "export_fail"
	}
	w, h := img.Bounds()
	data := img.Data()
	if len(data) < w*h*4 {
		if pd := img.PremultipliedData(); len(pd) >= w*h*4 {
			data = pd
		} else {
			return false, "no_data"
		}
	}
	at := func(x, y int) (r, g, b uint8) {
		i := (y*w + x) * 4
		return data[i], data[i+1], data[i+2]
	}
	// green-ish badge
	r1, g1, b1 := at(12, 12)
	// red-ish badge
	r2, g2, b2 := at(36, 12)
	// background gray
	r0, g0, b0 := at(24, 40)
	// badges must differ from background and each other
	if g1 <= r1 || g1 < 80 {
		return false, fmt.Sprintf("green_badge_weak rgb=%d,%d,%d", r1, g1, b1)
	}
	if r2 <= g2 || r2 < 80 {
		return false, fmt.Sprintf("red_badge_weak rgb=%d,%d,%d", r2, g2, b2)
	}
	if abs8(r1, r0)+abs8(g1, g0)+abs8(b1, b0) < 40 {
		return false, "green_equals_bg"
	}
	if abs8(r2, r0)+abs8(g2, g0)+abs8(b2, b0) < 40 {
		return false, "red_equals_bg"
	}
	return true, fmt.Sprintf("ok g=%d,%d,%d r=%d,%d,%d bg=%d,%d,%d", r1, g1, b1, r2, g2, b2, r0, g0, b0)
}

func abs8(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
