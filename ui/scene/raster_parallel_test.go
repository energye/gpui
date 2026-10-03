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
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// B1 并行光栅门：并行与串行统计一字不差，不可见层跳过重放但计数照记。

// buildMixedPacket 建一包：可见层（蓝框）+ 不可见层（零 alpha）+ 文本层。
func buildMixedPacket(t *testing.T) (*scene.LayerBuilder, []*scene.PictureLayer) {
	t.Helper()
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	visible := b.AddPicture(true)
	visible.Record(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 20, 20, 0, 0, 1, 1)
	})
	invisible := b.AddPicture(true)
	invisible.Record(func(r *scene.PictureRecorder) {
		r.FillRect(10, 10, 20, 20, 0, 0, 1, 0)
	})
	textLayer := b.AddPicture(true)
	textLayer.Record(func(r *scene.PictureRecorder) {
		r.DrawString("hi", 5, 30, nil, 0, 0, 0, 1)
	})
	return b, []*scene.PictureLayer{visible, invisible, textLayer}
}

func rasterOnce(t *testing.T, b *scene.LayerBuilder, dc *render.Context) scene.RasterStats {
	t.Helper()
	pkt := b.BuildPacket(1, 1, 64, 64)
	return scene.RasterizeDirtyToContext(pkt, dc)
}

func TestParallelRaster_StatsMatchSerial(t *testing.T) {
	b, layers := buildMixedPacket(t)
	if len(layers) != 3 {
		t.Fatalf("want 3 layers, got %d", len(layers))
	}

	dcPar := render.NewContext(64, 64)
	defer dcPar.Close()
	dcPar.BeginFrame()
	dcPar.ClearWithColor(render.White)
	stPar := rasterOnce(t, b, dcPar)

	// 串行对照：强制单路。
	t.Setenv("GPUI_RASTER_WORKERS", "1")
	scene.CloseRasterPool()
	b2, _ := buildMixedPacket(t)
	dcSer := render.NewContext(64, 64)
	defer dcSer.Close()
	dcSer.BeginFrame()
	dcSer.ClearWithColor(render.White)
	stSer := rasterOnce(t, b2, dcSer)
	scene.CloseRasterPool()

	if stPar.RasterLayerCount != stSer.RasterLayerCount {
		t.Fatalf("RasterLayerCount par=%d ser=%d", stPar.RasterLayerCount, stSer.RasterLayerCount)
	}
	if stPar.SkippedLayerCount != stSer.SkippedLayerCount {
		t.Fatalf("SkippedLayerCount par=%d ser=%d", stPar.SkippedLayerCount, stSer.SkippedLayerCount)
	}
	if stPar.ReplayedOps != stSer.ReplayedOps {
		t.Fatalf("ReplayedOps par=%d ser=%d", stPar.ReplayedOps, stSer.ReplayedOps)
	}
	if stPar.ReplayedOps < 3 {
		t.Fatalf("want all 3 layers counted, stats=%+v", stPar)
	}

	// 像素一致：可见蓝框落下去了。
	br, _, bb := sampleRGB(dcPar.Image().At(20, 20))
	if bb < 0xC000 || br > 0x4000 {
		t.Fatalf("par pixels (20,20)=#%04x want blue", bb)
	}
	br2, _, bb2 := sampleRGB(dcSer.Image().At(20, 20))
	if br2 != br || bb2 != bb {
		t.Fatalf("par/ser pixels differ par=(%04x,%04x) ser=(%04x,%04x)", br, bb, br2, bb2)
	}
}

func TestParallelRaster_ScanSkipsInvisible(t *testing.T) {
	t.Setenv("GPUI_RASTER_WORKERS", "8")
	defer scene.CloseRasterPool()
	_, layers := buildMixedPacket(t)
	got := scene.ParallelScanVisible(layers)
	if len(got) != 3 {
		t.Fatalf("want 3 scans, got %d", len(got))
	}
	if !got[0] {
		t.Fatal("blue rect layer must scan visible")
	}
	if got[1] {
		t.Fatal("zero-alpha layer must scan invisible")
	}
}

func TestParallelRaster_EmptyAndSingle(t *testing.T) {
	defer scene.CloseRasterPool()
	if got := scene.ParallelScanVisible(nil); len(got) != 0 {
		t.Fatalf("empty scan want 0, got %d", len(got))
	}
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	one := b.AddPicture(true)
	one.Record(func(r *scene.PictureRecorder) {
		r.FillRect(1, 1, 5, 5, 1, 0, 0, 1)
	})
	got := scene.ParallelScanVisible([]*scene.PictureLayer{one})
	if len(got) != 1 || !got[0] {
		t.Fatalf("single scan=%+v want [{true}]", got)
	}
}

// TestParallelRaster_TwoWindowsSharePool 模拟同应用双窗：两包并发扫同一池，
// 结果各归各，中间关一次池也不坏不串。
func TestParallelRaster_TwoWindowsSharePool(t *testing.T) {
	t.Setenv("GPUI_RASTER_WORKERS", "4")
	defer scene.CloseRasterPool()
	_, winA := buildMixedPacket(t)
	_, winB := buildMixedPacket(t)
	done := make(chan bool, 2)
	var gotA, gotB []bool
	go func() {
		gotA = scene.ParallelScanVisible(winA)
		done <- true
	}()
	go func() {
		gotB = scene.ParallelScanVisible(winB)
		done <- true
	}()
	<-done
	scene.CloseRasterPool()
	<-done
	if len(gotA) != 3 || len(gotB) != 3 {
		t.Fatalf("want 3+3 scans, got %d+%d", len(gotA), len(gotB))
	}
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("window results differ at %d: %+v vs %+v", i, gotA[i], gotB[i])
		}
	}
	if !gotA[0] || gotA[1] {
		t.Fatalf("unexpected scan results: %+v", gotA)
	}
}
