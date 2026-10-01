//go:build linux && !(js && wasm)

package gles

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

func TestNewAdapterContext_Accessors(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)

	if ctx.GL() != glCtx {
		t.Fatalf("GL() = %p, want %p", ctx.GL(), glCtx)
	}
	if ctx.EGL() != nil {
		t.Fatalf("EGL() = %v, want nil", ctx.EGL())
	}
	if ctx.owns {
		t.Fatal("owns should be false")
	}
}

func TestAdapterContext_Destroy_OwnedNilEGLIsNoOp(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, true)
	ctx.Destroy()
	// eglCtx is nil → Destroy body skipped; GL table retained.
	if ctx.GL() != glCtx {
		t.Fatal("Destroy with nil eglCtx should not clear GL table")
	}
}

func TestAdapterContext_Destroy_BorrowedDoesNotClear(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)
	ctx.Destroy()
	if ctx.GL() != glCtx {
		t.Fatal("borrowed Destroy must not clear GL table")
	}
	if ctx.owns {
		t.Fatal("borrowed flag must stay false")
	}
}

func TestAdapterContext_LockUnlock_NilEGL(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)

	got := ctx.Lock()
	if got != glCtx {
		t.Fatalf("Lock() = %p, want %p", got, glCtx)
	}
	// Must not panic even without egl.Init (Unlock short-circuits on nil eglCtx).
	ctx.Unlock()
}

func TestAdapterContext_LockForSurface_NilEGL(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)

	got := ctx.LockForSurface(egl.NoSurface)
	if got != glCtx {
		t.Fatalf("LockForSurface() = %p, want %p", got, glCtx)
	}
	ctx.Unlock()
}

func TestAdapterContext_LockSerializesConcurrentAccess(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)

	const goroutines = 32
	const iters = 50
	var wg sync.WaitGroup
	var counter int

	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iters {
				_ = ctx.Lock()
				counter++
				ctx.Unlock()
			}
		}()
	}
	wg.Wait()

	want := goroutines * iters
	if counter != want {
		t.Fatalf("counter = %d, want %d (mutex failed to serialize)", counter, want)
	}
}

func TestInstance_EnumerateAdapters_NoContextReturnsPlaceholder(t *testing.T) {
	inst := &Instance{}
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) != 1 {
		t.Fatalf("len(adapters) = %d, want 1", len(adapters))
	}
	info := adapters[0].Info
	if info.Vendor != vendorUnknown {
		t.Errorf("Vendor = %q, want %q", info.Vendor, vendorUnknown)
	}
	if !strings.Contains(info.DriverInfo, "RequestAdapterWithSurface") {
		t.Errorf("DriverInfo %q should guide caller to RequestAdapterWithSurface", info.DriverInfo)
	}
	if adapters[0].Adapter.(*Adapter).ctx != nil {
		t.Fatal("placeholder adapter must have nil AdapterContext")
	}
}

func TestInstance_Destroy_NilSafe(t *testing.T) {
	inst := &Instance{}
	inst.Release() // must not panic
	inst.Release()
}

func TestInstance_Destroy_ClearsOwnedContext(t *testing.T) {
	glCtx := &gl.Context{}
	inst := &Instance{ctx: NewAdapterContext(nil, glCtx, true)}
	inst.Release()
	if inst.ctx != nil {
		t.Fatal("Instance.Destroy should nil out ctx")
	}
}

func TestSurface_Destroy_SharedDoesNotDestroyAdapterContext(t *testing.T) {
	glCtx := &gl.Context{}
	shared := NewAdapterContext(nil, glCtx, true)
	surf := &Surface{
		ctx:         shared,
		ownsContext: false,
	}
	surf.Destroy()
	if surf.ctx != nil {
		t.Fatal("Surface.Destroy should nil local ctx pointer")
	}
	if shared.GL() != glCtx {
		t.Fatal("shared AdapterContext must survive Surface.Destroy when ownsContext=false")
	}
}

func TestSurface_Destroy_OwnedCallsAdapterContextDestroy(t *testing.T) {
	glCtx := &gl.Context{}
	owned := NewAdapterContext(nil, glCtx, true)
	surf := &Surface{
		ctx:         owned,
		ownsContext: true,
	}
	surf.Destroy()
	if surf.ctx != nil {
		t.Fatal("Surface.Destroy should nil local ctx pointer")
	}
	// owns=true but eglCtx=nil → AdapterContext.Destroy is a no-op for GL table;
	// ownership path still invoked (no panic). Covered fully in integration tests.
}

func TestSurface_GetAdapterInfo_NilCtxReturnsPlaceholder(t *testing.T) {
	surf := &Surface{}
	info := surf.GetAdapterInfo()
	if info.Adapter == nil {
		t.Fatal("expected placeholder adapter")
	}
	if info.Info.Vendor != vendorUnknown {
		t.Errorf("Vendor = %q, want %q", info.Info.Vendor, vendorUnknown)
	}
	if info.Info.Backend != gputypes.BackendGL {
		t.Errorf("Backend = %v, want BackendGL", info.Info.Backend)
	}
}

func TestSurface_Configure_NilCtxReturnsError(t *testing.T) {
	surf := &Surface{}
	err := surf.Configure(nil, &hal.SurfaceConfiguration{Width: 64, Height: 64})
	if err == nil || !strings.Contains(err.Error(), "AdapterContext") {
		t.Fatalf("Configure(nil ctx) error = %v", err)
	}
}

func TestSurface_Configure_ZeroArea(t *testing.T) {
	surf := &Surface{ctx: NewAdapterContext(nil, &gl.Context{}, false)}
	err := surf.Configure(nil, &hal.SurfaceConfiguration{Width: 0, Height: 64})
	if !errors.Is(err, hal.ErrZeroArea) {
		t.Fatalf("Configure(zero width) = %v, want ErrZeroArea", err)
	}
}

func TestQueue_Present_InvalidSurfaceType(t *testing.T) {
	q := &Queue{ctx: NewAdapterContext(nil, &gl.Context{}, false)}
	err := q.Present(nil, nil, nil)
	if err == nil {
		t.Fatal("Present(nil) should return error")
	}
	if !strings.Contains(err.Error(), "invalid surface") {
		t.Errorf("error %q should mention invalid surface", err.Error())
	}
}

func TestQueue_WriteBuffer_InvalidType(t *testing.T) {
	q := &Queue{ctx: NewAdapterContext(nil, &gl.Context{}, false)}
	err := q.WriteBuffer(nil, 0, []byte{1})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("WriteBuffer(nil) error = %v", err)
	}
}

func TestDevice_DestroyBuffer_NilCtxSafe(t *testing.T) {
	d := &Device{} // no AdapterContext
	d.DestroyBuffer(&Buffer{id: 42, glCtx: &gl.Context{}})
	// Must not panic; without ctx we skip GL delete.
}

func TestDevice_DestroyPaths_AcquireLock(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, false)
	d := &Device{ctx: ctx}

	// All Destroy* paths must Lock/Unlock even with nil EGL (no MakeCurrent).
	d.DestroyBuffer(&Buffer{glCtx: glCtx})
	d.DestroyTexture(&Texture{glCtx: glCtx})
	d.DestroySampler(&Sampler{glCtx: glCtx})
	d.DestroyShaderModule(&ShaderModule{glCtx: glCtx})
	d.DestroyRenderPipeline(&RenderPipeline{glCtx: glCtx})
	d.DestroyComputePipeline(&ComputePipeline{glCtx: glCtx})
	d.DestroyFence(&Fence{glCtx: glCtx})
	d.DestroyQuerySet(&QuerySet{queries: []uint32{7}, glCtx: glCtx})
}

// TestDestroy_RetryAfterBindFailure locks the retry contract: a Destroy
// whose bind fails must keep the GL ids AND the ledger slot so a later
// Destroy (after recovery) really deletes instead of leaking. Nil-EGL
// contexts always fail TryLock, which reproduces the driver-gone path
// without a GPU.
func TestDestroy_RetryAfterBindFailure(t *testing.T) {
	newNilCtx := func() *AdapterContext {
		return NewAdapterContext(nil, &gl.Context{}, false)
	}
	t.Run("Buffer", func(t *testing.T) {
		b := &Buffer{id: 41, size: 64, ctx: newNilCtx()}
		b.Destroy()
		if b.id == 0 {
			t.Fatal("bind failure must restore buffer id for retry")
		}
		if hal.VramLiveBytes() == 0 {
			t.Skip("ledger disabled in this env; id-restore above is the assertion")
		}
	})
	t.Run("Texture", func(t *testing.T) {
		tx := &Texture{id: 42, fbo: 9, ctx: newNilCtx(),
			size:      hal.Extent3D{Width: 64, Height: 64, DepthOrArrayLayers: 1},
			mipLevels: 1, sampleCount: 1, format: gputypes.TextureFormatRGBA8Unorm}
		tx.Destroy()
		if tx.id == 0 || tx.fbo == 0 {
			t.Fatal("bind failure must restore texture id/fbo for retry")
		}
	})
	t.Run("Sampler", func(t *testing.T) {
		s := &Sampler{id: 43, ctx: newNilCtx()}
		s.Destroy()
		if s.id == 0 {
			t.Fatal("bind failure must restore sampler id for retry")
		}
	})
	t.Run("RenderPipeline", func(t *testing.T) {
		p := &RenderPipeline{programID: 44, ctx: newNilCtx()}
		p.Destroy()
		if p.programID == 0 {
			t.Fatal("bind failure must restore program id for retry")
		}
	})
	t.Run("FenceSyncs", func(t *testing.T) {
		f := &Fence{pending: []glFence{{sync: 1234, value: 9}}, ctx: newNilCtx()}
		f.Destroy()
		if len(f.pending) == 0 {
			t.Fatal("bind failure must restore fence syncs for retry")
		}
		p2 := &Fence{pending: []glFence{{sync: 1235, value: 10}}, ctx: newNilCtx()}
		p2.Reset()
		if len(p2.pending) == 0 {
			t.Fatal("bind failure must restore fence syncs for retry (Reset)")
		}
	})
	t.Run("QuerySet", func(t *testing.T) {
		q := &QuerySet{queries: []uint32{45}, ctx: newNilCtx()}
		q.Destroy()
		if len(q.queries) == 0 {
			t.Fatal("bind failure must restore query ids for retry")
		}
	})
}

func TestAdapterContext_Destroy_SerializedWithLock(t *testing.T) {
	glCtx := &gl.Context{}
	ctx := NewAdapterContext(nil, glCtx, true)

	_ = ctx.Lock()
	done := make(chan struct{})
	go func() {
		ctx.Destroy() // must wait until Unlock
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Destroy must not complete while Lock is held")
	case <-time.After(50 * time.Millisecond):
	}
	ctx.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Destroy did not complete after Unlock")
	}
}

func TestHALInterface_AdapterContextWiring(t *testing.T) {
	var _ hal.Instance = (*Instance)(nil)
	var _ hal.Adapter = (*Adapter)(nil)
	var _ hal.Device = (*Device)(nil)
	var _ hal.Queue = (*Queue)(nil)
	var _ hal.Surface = (*Surface)(nil)

	// Zero Instance must still satisfy EnumerateAdapters contract.
	adapters := (&Instance{}).EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("EnumerateAdapters must return placeholder")
	}
	if adapters[0].Info.Backend != gputypes.BackendGL {
		t.Errorf("Backend = %v, want BackendGL", adapters[0].Info.Backend)
	}
}
