//go:build !nogpu

package gpu

import (
	"bytes"
	"fmt"
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// P0.2: native texture format / readback calibration tests.
// Covers unaligned row pitch, BGRA conversion, R8 mask readback, and solid clear-via-upload.
// P2.4e G2: the legacy GPUTexture wrapper was deleted (production talks to hal
// directly); these calibration tests now drive hal natively — same assertions,
// no wrapper capability lost.

// testTexFormat is the calibration test's own format tag.
type testTexFormat uint8

const (
	testFmtRGBA8 testTexFormat = iota
	testFmtBGRA8
	testFmtR8
)

func (f testTexFormat) bytesPerPixel() int {
	switch f {
	case testFmtRGBA8, testFmtBGRA8:
		return 4
	default:
		return 1
	}
}

func (f testTexFormat) halFormat() types.TextureFormat {
	switch f {
	case testFmtBGRA8:
		return types.TextureFormatBGRA8Unorm
	case testFmtR8:
		return types.TextureFormatR8Unorm
	default:
		return types.TextureFormatRGBA8Unorm
	}
}

// createTestTexture allocates a CopySrc|CopyDst|TextureBinding texture.
func createTestTexture(device hal.Device, f testTexFormat, w, h int, label string) (hal.Texture, error) {
	return device.CreateTexture(&hal.TextureDescriptor{
		Label:         label,
		Size:          hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        f.halFormat(),
		Usage:         types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding,
	})
}

// packTestUpload prepares tightly or pitch-aligned upload bytes (verbatim port
// of the deleted wrapper's packTextureUpload). For multi-row uploads WebGPU
// requires BytesPerRow to be a multiple of 256. R8 accepts either a packed R8
// plane (len=w*h) or an RGBA pixmap (len=w*h*4), alpha channel as the mask.
func packTestUpload(f testTexFormat, width, height int, src []byte) (data []byte, bytesPerRow uint32, err error) {
	if width <= 0 || height <= 0 {
		return nil, 0, fmt.Errorf("invalid dimensions %dx%d", width, height)
	}
	bpp := f.bytesPerPixel()
	tightRow := width * bpp
	need := tightRow * height

	switch f {
	case testFmtR8:
		switch len(src) {
		case width * height:
			// already packed R8
		case width * height * 4:
			packed := make([]byte, width*height)
			for i := 0; i < width*height; i++ {
				packed[i] = src[i*4+3]
			}
			src = packed
		default:
			return nil, 0, fmt.Errorf("R8 upload expects %d or %d bytes, got %d",
				width*height, width*height*4, len(src))
		}
	default:
		if len(src) < need {
			return nil, 0, fmt.Errorf("upload expects at least %d bytes, got %d", need, len(src))
		}
	}

	alignedRow := alignTextureBytesPerRow(uint32(tightRow)) //nolint:gosec // positive dims
	if int(alignedRow) == tightRow {
		return src[:need], alignedRow, nil
	}

	out := make([]byte, int(alignedRow)*height)
	for y := 0; y < height; y++ {
		copy(out[y*int(alignedRow):y*int(alignedRow)+tightRow], src[y*tightRow:(y+1)*tightRow])
	}
	return out, alignedRow, nil
}

// uploadTestPixmap writes a pixmap plane into the texture (queue.WriteTexture).
func uploadTestPixmap(queue hal.Queue, tex hal.Texture, f testTexFormat, w, h int, src []byte) error {
	data, bytesPerRow, err := packTestUpload(f, w, h, src)
	if err != nil {
		return err
	}
	return queue.WriteTexture(&hal.ImageCopyTexture{
		Texture:  tex,
		MipLevel: 0,
		Origin:   hal.Origin3D{},
		Aspect:   types.TextureAspectAll,
	}, data, &hal.ImageDataLayout{
		Offset:       0,
		BytesPerRow:  bytesPerRow,
		RowsPerImage: uint32(h), //nolint:gosec // dimensions validated
	}, &hal.Extent3D{
		Width:              uint32(w), //nolint:gosec // dimensions validated
		Height:             uint32(h), //nolint:gosec // dimensions validated
		DepthOrArrayLayers: 1,
	})
}

// downloadTestPixmap copies the texture into a staging buffer and expands it
// to an RGBA8 pixmap (BGRA swizzle / R8 white+alpha, same semantics as the
// deleted wrapper's DownloadPixmap).
func downloadTestPixmap(device hal.Device, queue hal.Queue, tex hal.Texture, f testTexFormat, w, h int) (*render.Pixmap, error) {
	bpp := f.bytesPerPixel()
	bytesPerRow := uint32(w * bpp) //nolint:gosec // dimensions validated
	alignedBytesPerRow := alignTextureBytesPerRow(bytesPerRow)
	stagingBufSize := uint64(alignedBytesPerRow) * uint64(h)

	stagingBuf, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "p0_readback_staging",
		Size:  stagingBufSize,
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create staging buffer: %w", err)
	}
	defer stagingBuf.Destroy()

	encoder, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p0_readback_encoder"})
	if err != nil {
		return nil, fmt.Errorf("create encoder: %w", err)
	}
	encoder.CopyTextureToBuffer(tex, stagingBuf, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedBytesPerRow,
			RowsPerImage: uint32(h), //nolint:gosec // dimensions validated
		},
		TextureBase: hal.ImageCopyTexture{Texture: tex, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
		Size:        hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},
	}})
	cmdBuf, err := encoder.Finish()
	if err != nil {
		return nil, fmt.Errorf("finish encoder: %w", err)
	}
	defer device.FreeCommandBuffer(cmdBuf)

	if _, err := queue.Submit(cmdBuf); err != nil {
		return nil, fmt.Errorf("submit readback: %w", err)
	}

	mapping, err := device.MapBuffer(stagingBuf, 0, stagingBufSize)
	if err != nil {
		return nil, fmt.Errorf("map staging buffer: %w", err)
	}
	readback := make([]byte, stagingBufSize)
	copy(readback, unsafe.Slice((*byte)(mapping.Ptr), stagingBufSize)) //nolint:gosec // hal.BufferMapping opaque pointer
	if err := device.UnmapBuffer(stagingBuf); err != nil {
		return nil, fmt.Errorf("unmap staging buffer: %w", err)
	}

	pixmap := render.NewPixmap(w, h)
	dst := pixmap.Data()
	for y := 0; y < h; y++ {
		srcRow := readback[y*int(alignedBytesPerRow) : y*int(alignedBytesPerRow)+int(bytesPerRow)]
		dstRow := dst[y*w*4 : (y+1)*w*4]
		switch f {
		case testFmtRGBA8:
			copy(dstRow, srcRow)
		case testFmtBGRA8:
			convertBGRAToRGBA(srcRow, dstRow, w)
		case testFmtR8:
			for x := 0; x < w; x++ {
				dstRow[x*4+0] = 255
				dstRow[x*4+1] = 255
				dstRow[x*4+2] = 255
				dstRow[x*4+3] = srcRow[x]
			}
		}
	}
	pixmap.NotifyPixelsChanged()
	return pixmap, nil
}

func TestTextureUploadDownloadNative_UnalignedWidth(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	// Width 3 → 12 bytes/row, not a multiple of 256. Must still roundtrip.
	const width, height = 3, 5
	tex, err := createTestTexture(device, testFmtRGBA8, width, height, "unaligned-rgba")
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Destroy()

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

	if err := uploadTestPixmap(queue, tex, testFmtRGBA8, width, height, src.Data()); err != nil {
		t.Fatalf("upload unaligned: %v", err)
	}
	got, err := downloadTestPixmap(device, queue, tex, testFmtRGBA8, width, height)
	if err != nil {
		t.Fatalf("download unaligned: %v", err)
	}
	if !bytes.Equal(got.Data(), src.Data()) {
		t.Fatalf("unaligned RGBA roundtrip mismatch\n want %v\n got  %v", src.Data(), got.Data())
	}
}

func TestTextureUploadDownloadNative_BGRA8(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const width, height = 4, 2 // 16 bytes/row, unaligned to 256
	tex, err := createTestTexture(device, testFmtBGRA8, width, height, "bgra8")
	if err != nil {
		t.Fatalf("CreateTexture BGRA8: %v", err)
	}
	defer tex.Destroy()

	// Upload path treats pixmap bytes as texture-native layout. Write BGRA-ordered
	// source data and expect download to convert back to RGBA.
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

	if err := uploadTestPixmap(queue, tex, testFmtBGRA8, width, height, src.Data()); err != nil {
		t.Fatalf("upload BGRA8: %v", err)
	}
	got, err := downloadTestPixmap(device, queue, tex, testFmtBGRA8, width, height)
	if err != nil {
		t.Fatalf("download BGRA8: %v", err)
	}
	gd := got.Data()
	if gd[0] != 30 || gd[1] != 20 || gd[2] != 10 || gd[3] != 255 {
		t.Fatalf("BGRA→RGBA pixel0 = rgba(%d,%d,%d,%d), want (30,20,10,255)", gd[0], gd[1], gd[2], gd[3])
	}
}

func TestTextureUploadDownloadNative_R8(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const width, height = 7, 3 // 7 bytes/row, unaligned
	tex, err := createTestTexture(device, testFmtR8, width, height, "r8-mask")
	if err != nil {
		t.Fatalf("CreateTexture R8: %v", err)
	}
	defer tex.Destroy()

	// Pixmap is always RGBA8; R8 upload accepts RGBA and uses alpha as mask.
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

	if err := uploadTestPixmap(queue, tex, testFmtR8, width, height, src.Data()); err != nil {
		t.Fatalf("upload R8 from RGBA pixmap: %v", err)
	}
	got, err := downloadTestPixmap(device, queue, tex, testFmtR8, width, height)
	if err != nil {
		t.Fatalf("download R8: %v", err)
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

	const width, height = 5, 4 // unaligned
	tex, err := createTestTexture(device, testFmtRGBA8, width, height, "clear-rgba")
	if err != nil {
		t.Fatalf("CreateTexture: %v", err)
	}
	defer tex.Destroy()

	// Clear via upload of solid color (authoritative clear semantics).
	src := render.NewPixmap(width, height)
	for i := 0; i < width*height; i++ {
		off := i * 4
		src.Data()[off+0] = 10
		src.Data()[off+1] = 20
		src.Data()[off+2] = 30
		src.Data()[off+3] = 255
	}
	src.NotifyPixelsChanged()
	if err := uploadTestPixmap(queue, tex, testFmtRGBA8, width, height, src.Data()); err != nil {
		t.Fatalf("clear via upload: %v", err)
	}
	got, err := downloadTestPixmap(device, queue, tex, testFmtRGBA8, width, height)
	if err != nil {
		t.Fatalf("download after clear: %v", err)
	}
	for i := 0; i < width*height; i++ {
		off := i * 4
		if got.Data()[off+0] != 10 || got.Data()[off+1] != 20 || got.Data()[off+2] != 30 || got.Data()[off+3] != 255 {
			t.Fatalf("pixel %d = %v, want rgba(10,20,30,255)", i, got.Data()[off:off+4])
		}
	}
}

func TestPackTestUpload_R8AlphaPlane(t *testing.T) {
	const width, height = 3, 2
	rgba := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		rgba[i*4+3] = byte(10 + i)
	}
	data, bpr, err := packTestUpload(testFmtR8, width, height, rgba)
	if err != nil {
		t.Fatalf("packTestUpload: %v", err)
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

func TestPackTestUpload_UnalignedRGBA(t *testing.T) {
	const width, height = 3, 5
	src := make([]byte, width*height*4)
	for i := range src {
		src[i] = byte(i)
	}
	data, bpr, err := packTestUpload(testFmtRGBA8, width, height, src)
	if err != nil {
		t.Fatalf("packTestUpload: %v", err)
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
