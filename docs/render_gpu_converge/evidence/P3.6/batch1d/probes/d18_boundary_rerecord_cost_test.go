package rendering_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// 临时探针（批 1d D18 决策用，跑完删）：动画逐帧推进时，重绘边界的每帧开销。
func TestProbe_BoundaryRerecordCostWithAnimation(t *testing.T) {
	const W, H = 400, 400
	buf, err := render.NewImageBuf(120, 120, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Dispose()
	buf.Fill(220, 30, 30, 255)
	buf.NotifyPixelsChanged()

	box := rendering.NewAbsoluteBox(360, 360)
	box.Background = &rendering.Color{R: 0.05, G: 0.05, B: 0.08, A: 1}
	box.SetRepaintBoundary(true)
	img := rendering.NewRenderImage(120, 120)
	img.SetImageShared(buf)
	box.Place(img, 10, 10)
	for i := 0; i < 24; i++ {
		cb := rendering.NewRenderColorBox(30, 12, 0.2, float64(i)/24.0, 0.6, 1)
		box.Place(cb, 10+float64(i%6)*55, 150+float64(i/6)*40)
	}

	root := rendering.NewAbsoluteBox(W, H)
	root.Place(box, 20, 20)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	const frames = 200
	var best, total time.Duration
	var rrSum, skSum int64
	best = time.Hour

	// 预热一帧（SetImageShared 会标脏，第一帧必然重录，不计入统计）
	{
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
	}
	if img.NeedsPaint() || box.NeedsPaint() {
		t.Fatal("warm-up frame must settle the tree")
	}

	for f := 0; f < frames; f++ {
		buf.Fill(uint8(20+f%200), 30, 30, 255)
		buf.NotifyPixelsChanged()
		if img.NeedsPaint() || box.NeedsPaint() {
			t.Fatalf("frame %d: in-place pixel write must not dirty nodes", f)
		}
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		t0 := time.Now()
		root.Paint(pc)
		el := time.Since(t0)
		st := cache.Stats()
		rrSum += st.FrameRerecord
		skSum += st.FrameSkip
		total += el
		if el < best {
			best = el
		}
		// 每帧查一次画面：边界若重放，像素必须仍是最新那一帧（活缓冲语义）
		r, g, b, _ := dc.Image().At(70, 70).RGBA() // 图块中心 (20+10+60, 20+10+60)
		if want := uint8(20 + f%200); int(r>>8) != int(want) || g > 0x2000 || b > 0x2000 {
			t.Fatalf("frame %d: pixel=(%d,%d,%d) want R=%d", f, r>>8, g>>8, b>>8, want)
		}
	}
	t.Logf("PROBE frames=%d total=%v avg=%v best=%v rerecord/frame=%.2f skip/frame=%.2f",
		frames, total, total/time.Duration(frames), best,
		float64(rrSum)/frames, float64(skSum)/frames)
}

// 探针二：边界里带文字（排版进录制 pass 才会显形）。
func TestProbe_BoundaryRerecordCostWithText(t *testing.T) {
	const W, H = 400, 400
	face, _, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("no default face: %v", err)
	}
	buf, err := render.NewImageBuf(120, 120, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Dispose()
	buf.Fill(220, 30, 30, 255)
	buf.NotifyPixelsChanged()

	box := rendering.NewAbsoluteBox(360, 360)
	box.Background = &rendering.Color{R: 0.05, G: 0.05, B: 0.08, A: 1}
	box.SetRepaintBoundary(true)
	img := rendering.NewRenderImage(120, 120)
	img.SetImageShared(buf)
	box.Place(img, 10, 10)
	for i := 0; i < 12; i++ {
		tx := rendering.NewRenderText("The quick brown fox jumps over the lazy dog 0123456789")
		tx.SetFace(face)
		box.Place(tx, 10, 150+float64(i)*18)
	}

	root := rendering.NewAbsoluteBox(W, H)
	root.Place(box, 20, 20)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	const frames = 120
	var best, total time.Duration
	var rrSum, skSum int64
	best = time.Hour
	{
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
	}
	for f := 0; f < frames; f++ {
		buf.Fill(uint8(20+f%200), 30, 30, 255)
		buf.NotifyPixelsChanged()
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		t0 := time.Now()
		root.Paint(pc)
		el := time.Since(t0)
		st := cache.Stats()
		rrSum += st.FrameRerecord
		skSum += st.FrameSkip
		total += el
		if el < best {
			best = el
		}
	}
	t.Logf("PROBE-TEXT frames=%d total=%v avg=%v best=%v rerecord/frame=%.2f skip/frame=%.2f",
		frames, total, total/time.Duration(frames), best,
		float64(rrSum)/frames, float64(skSum)/frames)
}

// 探针三：把边界自己的内容加厚（200 个子节点），让重录 pass 的开销显形。
func TestProbe_BoundaryRerecordCostFatBoundary(t *testing.T) {
	for _, kids := range []int{25, 200} {
		probeFatBoundary(t, kids)
	}
}

func probeFatBoundary(t *testing.T, kids int) {
	const W, H = 700, 700
	buf, err := render.NewImageBuf(64, 64, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Dispose()
	buf.Fill(220, 30, 30, 255)
	buf.NotifyPixelsChanged()

	box := rendering.NewAbsoluteBox(640, 640)
	box.Background = &rendering.Color{R: 0.05, G: 0.05, B: 0.08, A: 1}
	box.SetRepaintBoundary(true)
	img := rendering.NewRenderImage(64, 64)
	img.SetImageShared(buf)
	box.Place(img, 10, 10)
	for i := 0; i < kids; i++ {
		cb := rendering.NewRenderColorBox(24, 10, 0.2, float64(i%24)/24.0, 0.6, 1)
		box.Place(cb, 20+float64(i%16)*38, 90+float64(i/16)*22)
	}

	root := rendering.NewAbsoluteBox(W, H)
	root.Place(box, 20, 20)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	const frames = 150
	var best, total time.Duration
	var rrSum int64
	best = time.Hour
	{
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
	}
	for f := 0; f < frames; f++ {
		buf.Fill(uint8(20+f%200), 30, 30, 255)
		buf.NotifyPixelsChanged()
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		t0 := time.Now()
		root.Paint(pc)
		el := time.Since(t0)
		rrSum += cache.Stats().FrameRerecord
		total += el
		if el < best {
			best = el
		}
	}
	t.Logf("PROBE-FAT kids=%d frames=%d avg=%v best=%v rerecord/frame=%.2f",
		kids, frames, total/time.Duration(frames), best, float64(rrSum)/frames)
}
