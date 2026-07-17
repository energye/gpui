package render

import "testing"

func TestDiag_HairlineRow(t *testing.T) {
	dc := NewContext(160, 80, WithDeviceScale(2.0))
	defer dc.Close()
	dc.ClearWithColor(White)
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(0)
	dc.DrawLine(10, 20, 70, 20)
	_ = dc.Stroke()
	_ = dc.FlushGPU()
	for y := 35; y <= 45; y++ {
		ink := 0
		dark := 0
		minR := 1.0
		for x := 20; x < 140; x++ {
			p := dc.pixmap.GetPixel(x, y)
			if p.R < 0.99 {
				ink++
			}
			if p.R < 200.0/255.0 {
				dark++
			}
			if p.R < minR {
				minR = p.R
			}
		}
		t.Logf("y=%d ink=%d dark200=%d minR=%.3f", y, ink, dark, minR)
	}
}

func TestDiag_Caret(t *testing.T) {
	// A2-like: caret at logical? scale1
	dc := NewContext(280, 80)
	defer dc.Close()
	dc.ClearWithColor(White)
	// field white already
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(20, 24, 240, 32, 4)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.1)
	dc.SetLineWidth(1)
	dc.DrawLine(120, 30, 120, 50)
	_ = dc.Stroke()
	_ = dc.FlushGPU()
	// sample around caret
	for x := 118; x <= 122; x++ {
		for y := 30; y <= 50; y += 5 {
			p := dc.pixmap.GetPixel(x, y)
			t.Logf("caret (%d,%d) R=%.3f G=%.3f B=%.3f A=%.3f", x, y, p.R, p.G, p.B, p.A)
		}
	}
}

func TestDiag_MaskNested(t *testing.T) {
	const size = 100
	dc := NewContext(size, size)
	outerMask := NewMask(size, size)
	for y := 0; y < size; y++ {
		for x := 0; x < size/2; x++ {
			outerMask.Set(x, y, 255)
		}
	}
	innerMask := NewMask(size, size)
	for y := 0; y < size/2; y++ {
		for x := 0; x < size; x++ {
			innerMask.Set(x, y, 255)
		}
	}
	dc.PushMaskLayer(outerMask)
	dc.PushMaskLayer(innerMask)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, size, size)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PopLayer()
	img := dc.Image()
	for _, pt := range [][2]int{{25, 25}, {75, 25}, {25, 75}, {75, 75}} {
		_, _, _, a := img.At(pt[0], pt[1]).RGBA()
		t.Logf("(%d,%d) a=%d", pt[0], pt[1], a)
	}
	t.Logf("stats=%s", dc.RenderPathStats().LogLine())
}
