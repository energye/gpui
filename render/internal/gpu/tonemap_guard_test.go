//go:build !nogpu

package gpu

import (
	"errors"
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// M5 P5 收口负向：tonemap 出口只接受 RGBA16Float 源；8bit 源必须具名拒绝，
// 不能把 SDR 内容再走一遍曲线（那会把画面整体压暗）。
func TestP5TonemapViewRejectsNonFloatSource(t *testing.T) {
	shared := NewGPUShared()
	shared.mu.Lock()
	err := shared.initGPU()
	shared.mu.Unlock()
	if err != nil {
		t.Skipf("GPU not available: %v", err)
	}
	defer shared.Close()
	rc := shared.NewRenderContext()
	defer rc.Close()

	view, release, err := rc.CreateOffscreenTextureFormat(16, 16, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("create 8bit source: %v", err)
	}
	defer release()
	if _, _, err := rc.TonemapView(view, 16, 16, 1); err == nil {
		t.Fatal("TonemapView accepted an 8-bit source")
	}

	// And the strict float-format entry still rejects non-renderable formats
	// before allocating (the fallback counter only moves through the explicit
	// OrFallback entry, exercised in TestP2OffscreenFormatFailClosedAndFallback).
	if _, _, err := rc.CreateOffscreenTextureFormat(16, 16, types.TextureFormatDepth24PlusStencil8); !errors.Is(err, ErrOffscreenFormatUnsupported) {
		t.Fatalf("depth format error = %v, want ErrOffscreenFormatUnsupported", err)
	}
}
