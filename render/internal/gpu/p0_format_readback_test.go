//go:build !nogpu

package gpu

import (
	"bytes"
	"testing"

	"github.com/energye/gpui/render"
)

// P0.2: native texture format / readback calibration tests.
// Covers unaligned row pitch, BGRA conversion, R8 mask readback, and solid clear-via-upload.

func TestTextureUploadDownloadNative_UnalignedWidth(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	backend := &Backend{device: device, queue: queue, initialized: true}

	// Width 3 → 12 bytes/row, not a multiple of 256. Must still roundtrip.
	const width, height = 3, 5
	tex, err := CreateTexture(backend, TextureConfig{
		Width: width, Height: height, Format: TextureFormatRGBA8, Label: "unaligned-rgba",
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Close()

	src := render.NewPixmap(width, height)
	for i := range src.Data() {
		src.Data()[i] = byte((i*41 + 7) & 0xff)
	}
	// Force fully opaque alpha so premultiply edge cases do not hide errors.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			off := (y*width + x) * 4
			src.Data()[off+3] = 255
		}
	}
	src.NotifyPixelsChanged()

	if err := tex.UploadPixmap(src); err != nil {
		t.Fatalf("UploadPixmap unaligned: %v", err)
	}
	got, err := tex.DownloadPixmap()
	if err != nil {
		t.Fatalf("DownloadPixmap unaligned: %v", err)
	}
	if !bytes.Equal(got.Data(), src.Data()) {
		t.Fatalf("unaligned RGBA roundtrip mismatch\n want %v\n got  %v", src.Data(), got.Data())
	}
}

func TestTextureUploadDownloadNative_BGRA8(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	backend := &Backend{device: device, queue: queue, initialized: true}

	const width, height = 4, 2 // 16 bytes/row, unaligned to 256
	tex, err := CreateTexture(backend, TextureConfig{
		Width: width, Height: height, Format: TextureFormatBGRA8, Label: "bgra8",
	})
	if err != nil {
		t.Fatalf("CreateTexture BGRA8: %v", err)
	}
	defer tex.Close()

	// Upload path treats pixmap bytes as texture-native layout. Write BGRA-ordered
	// source data and expect DownloadPixmap to convert back to RGBA.
	src := render.NewPixmap(width, height)
	// Pixel (0,0): B=10,G=20,R=30,A=255 → after download RGBA(30,20,10,255)
	data := src.Data()
	for i := 0; i < width*height; i++ {
		off := i * 4
		data[off+0] = 10 // B
		data[off+1] = 20 // G
		data[off+2] = 30 // R
		data[off+3] = 255
	}
	src.NotifyPixelsChanged()

	if err := tex.UploadPixmap(src); err != nil {
		t.Fatalf("UploadPixmap BGRA8: %v", err)
	}
	got, err := tex.DownloadPixmap()
	if err != nil {
		t.Fatalf("DownloadPixmap BGRA8: %v", err)
	}
	gd := got.Data()
	if gd[0] != 30 || gd[1] != 20 || gd[2] != 10 || gd[3] != 255 {
		t.Fatalf("BGRA→RGBA pixel0 = rgba(%d,%d,%d,%d), want (30,20,10,255)", gd[0], gd[1], gd[2], gd[3])
	}
}

func TestTextureUploadDownloadNative_R8(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	backend := &Backend{device: device, queue: queue, initialized: true}

	const width, height = 7, 3 // 7 bytes/row, unaligned
	tex, err := CreateTexture(backend, TextureConfig{
		Width: width, Height: height, Format: TextureFormatR8, Label: "r8-mask",
	})
	if err != nil {
		t.Fatalf("CreateTexture R8: %v", err)
	}
	defer tex.Close()

	// Pixmap is always RGBA8; R8 UploadPixmap accepts RGBA and uses alpha as mask.
	src := render.NewPixmap(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v := byte((x*17 + y*3) & 0xff)
			off := (y*width + x) * 4
			src.Data()[off+0] = 255
			src.Data()[off+1] = 255
			src.Data()[off+2] = 255
			src.Data()[off+3] = v
		}
	}
	src.NotifyPixelsChanged()

	if err := tex.UploadPixmap(src); err != nil {
		t.Fatalf("UploadPixmap R8 from RGBA pixmap: %v", err)
	}
	got, err := tex.DownloadPixmap()
	if err != nil {
		t.Fatalf("DownloadPixmap R8: %v", err)
	}
	// Download expands R8 to white RGB + alpha=mask
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			want := byte((x*17 + y*3) & 0xff)
			off := (y*width + x) * 4
			if got.Data()[off+0] != 255 || got.Data()[off+1] != 255 || got.Data()[off+2] != 255 {
				t.Fatalf("R8 RGB at %d,%d = %v, want white", x, y, got.Data()[off:off+3])
			}
			if got.Data()[off+3] != want {
				t.Fatalf("R8 alpha at %d,%d = %d, want %d", x, y, got.Data()[off+3], want)
			}
		}
	}
}

func TestTextureClearNative_RGBAOpaqueColor(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	backend := &Backend{device: device, queue: queue, initialized: true}

	const width, height = 5, 4 // unaligned
	tex, err := CreateTexture(backend, TextureConfig{
		Width: width, Height: height, Format: TextureFormatRGBA8, Label: "clear-rgba",
	})
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Close()

	// Clear via upload of solid color (authoritative clear semantics for GPUTexture).
	src := render.NewPixmap(width, height)
	for i := 0; i < width*height; i++ {
		off := i * 4
		src.Data()[off+0] = 10
		src.Data()[off+1] = 20
		src.Data()[off+2] = 30
		src.Data()[off+3] = 255
	}
	src.NotifyPixelsChanged()
	if err := tex.UploadPixmap(src); err != nil {
		t.Fatalf("clear via UploadPixmap: %v", err)
	}
	got, err := tex.DownloadPixmap()
	if err != nil {
		t.Fatalf("Download after clear: %v", err)
	}
	for i := 0; i < width*height; i++ {
		off := i * 4
		if got.Data()[off+0] != 10 || got.Data()[off+1] != 20 || got.Data()[off+2] != 30 || got.Data()[off+3] != 255 {
			t.Fatalf("pixel %d = %v, want rgba(10,20,30,255)", i, got.Data()[off:off+4])
		}
	}
}

func TestPackTextureUpload_R8AlphaPlane(t *testing.T) {
	const width, height = 3, 2
	rgba := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		rgba[i*4+3] = byte(10 + i)
	}
	data, bpr, err := packTextureUpload(TextureFormatR8, width, height, rgba)
	if err != nil {
		t.Fatalf("packTextureUpload: %v", err)
	}
	if bpr != 256 {
		t.Fatalf("bytesPerRow = %d, want 256 (padded)", bpr)
	}
	for i := 0; i < width*height; i++ {
		y := i / width
		x := i % width
		got := data[y*int(bpr)+x]
		want := byte(10 + i)
		if got != want {
			t.Fatalf("packed[%d,%d]=%d want %d", x, y, got, want)
		}
	}
}

func TestPackTextureUpload_UnalignedRGBA(t *testing.T) {
	const width, height = 3, 5
	src := make([]byte, width*height*4)
	for i := range src {
		src[i] = byte(i)
	}
	data, bpr, err := packTextureUpload(TextureFormatRGBA8, width, height, src)
	if err != nil {
		t.Fatalf("packTextureUpload: %v", err)
	}
	if bpr != 256 {
		t.Fatalf("bytesPerRow = %d, want 256", bpr)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width*4; x++ {
			got := data[y*int(bpr)+x]
			want := src[y*width*4+x]
			if got != want {
				t.Fatalf("row %d byte %d = %d want %d", y, x, got, want)
			}
		}
	}
}
