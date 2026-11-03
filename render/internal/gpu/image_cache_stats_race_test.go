//go:build !nogpu

package gpu

import (
	"sync/atomic"
	"testing"

	"github.com/energye/gpui/render"
	intImage "github.com/energye/gpui/render/internal/image"
)

// D20 交付后审查 P0-2 钉：Stats() 的采样读可来自任意线程（UI 线程对账卡），写侧
// gen/lastUploadBytes 在光栅线程。本测试让两个线程同窗口跑真实读写面，-race 下
// 任何非原子字段都会被检出。
func TestImageCache_StatsSampledFromAnotherThread_RaceFree(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	img, err := intImage.NewImageBuf(8, 8, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	img.Fill(10, 10, 10, 255)

	cmd := func() *ImageDrawCommand {
		gen := img.GenerationID()
		return &ImageDrawCommand{
			PixelData: img.PremultipliedData(), GenerationID: gen,
			Src: img, SrcGen: gen,
			ImgWidth: 8, ImgHeight: 8, ImgStride: 32,
		}
	}
	if _, err := ic.GetOrUpload(cmd()); err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var sampled atomic.Uint64
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			st := ic.Stats()
			_ = st.Generations
			sampled.Add(1)
		}
	}()

	const frames = 300
	for i := 0; i < frames; i++ {
		img.Fill(byte(i), 10, 10, 255)
		img.NotifyPixelsChanged()
		if _, err := ic.GetOrUpload(cmd()); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)

	if sampled.Load() == 0 {
		t.Fatal("stats goroutine never sampled; the race surface was not exercised")
	}
}

// 口径 B 的漂移防钉：Stats 的派生计数镜像（Entries/MipEntries/Watched）必须与
// 光栅线程直读 map 的地面真值逐检查点一致。镜像漂移 = 门禁数字说谎，比竞态更隐蔽。
func TestImageCache_DerivedCountersMatchMaps(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	check := func(what string) {
		t.Helper()
		mip := 0
		for _, e := range ic.entries {
			if e.levels > 1 {
				mip++
			}
		}
		st := ic.Stats()
		if st.Entries != len(ic.entries) || st.Watched != len(ic.bySrc) || st.MipEntries != mip {
			t.Fatalf("%s: stats=(entries %d, watched %d, mip %d) 与 map 真值=(%d, %d, %d) 漂移",
				what, st.Entries, st.Watched, st.MipEntries, len(ic.entries), len(ic.bySrc), mip)
		}
	}

	newImg := func(size int) *intImage.ImageBuf {
		img, err := intImage.NewImageBuf(size, size, intImage.FormatRGBAPremul)
		if err != nil {
			t.Fatal(err)
		}
		img.Fill(10, 10, 10, 255)
		return img
	}
	upload := func(img *intImage.ImageBuf, mip render.MipmapMode) {
		gen := img.GenerationID()
		if _, err := ic.GetOrUpload(&ImageDrawCommand{
			PixelData: img.PremultipliedData(), GenerationID: gen,
			Src: img, SrcGen: gen, Mip: mip,
			ImgWidth: img.Width(), ImgHeight: img.Height(), ImgStride: img.Width() * 4,
		}); err != nil {
			t.Fatal(err)
		}
	}

	a := newImg(8)
	upload(a, render.MipmapNone)
	check("first upload")

	b := newImg(32)
	upload(b, render.MipmapLinear)
	check("mip upload")

	// 原地改写：换代 → 旧条目退役被 drain、新条目插入，镜像须跟着走。
	a.Fill(200, 10, 10, 255)
	a.NotifyPixelsChanged()
	upload(a, render.MipmapNone)
	check("rewrite + drain")

	// 源释放 → 其条目整体退役。
	b.Dispose()
	ic.drainRetired()
	check("source disposed + drain")

	// 预算清空路径（evictOldest → removeEntry）也要扣干净。
	ic.PurgeEvictable()
	check("purge")
	a.Dispose()
	ic.drainRetired()
	check("final drain")
}
