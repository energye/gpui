// Package res implements Skia-style GPU resource lifecycle management:
// logical references (SourceKey), strong usage references (Ref), a
// resolution registry, a resource cache, and submission-tracked
// deferred destruction.
//
// Naming alignment with Skia:
//   - SourceKey + Ref ≈ GrSurfaceProxy (logical + strong reference)
//   - Registry        ≈ proxy → resource mapping + GrGpuResource refcounts
//   - Cache           ≈ GrResourceCache (keyed reuse, budget, LRU)
//   - Submission      ≈ command-buffer refs (keep alive until fence)
//
// The package is pure Go: it never touches wgpu directly. Native resources
// are abstracted behind the [Native] interface so everything is unit-testable
// without a GPU.
//
// Concurrency: instances are NOT thread-safe, matching the serialized
// GPU render path (one GPURenderContext at a time).
package res

// Kind identifies the kind of GPU resource a handle refers to.
type Kind uint8

const (
	KindTextureView Kind = iota
	KindTexture
	KindBuffer
	KindSampler
	KindBindGroup
)

// Role identifies a logical texture role for deferred resolution, aligned
// with real view sources in the engine (no invented roles).
type Role uint8

const (
	RoleNone Role = iota
	// RoleSessionResolve / RoleSessionMSAA / RoleSessionStencil are the
	// textureSet trio (offscreen MSAA color, depth/stencil, resolve).
	RoleSessionResolve
	RoleSessionMSAA
	RoleSessionStencil
	// RoleFrameScratch is the advanced-blend intermediate BGRA texture.
	RoleFrameScratch
	// RoleLayerRT is a pooled layer offscreen render target.
	RoleLayerRT
	// RoleCoverResult is a per-draw stencil-cover texture.
	RoleCoverResult
	// RoleAtlasPage is a glyph/MSDF atlas texture page.
	RoleAtlasPage
	// RoleExternal marks borrowed views owned outside the res system
	// (swapchain views, shared atlases). Tracked but never released here.
	RoleExternal
)

// SourceKey is a logical reference to a texture, resolved at flush time —
// never a snapshot of the concrete resource. This mirrors
// GrSurfaceProxy::lazyInstantiation timing: resolution happens when the
// draw is recorded, so a texture rebuilt between queue and flush resolves
// to the current active instance.
type SourceKey struct {
	Kind  Kind
	Role  Role
	Index uint32 // 0 = current active instance for the role; atlas pages = page number
}

// IsNil reports whether the key is the zero value.
func (k SourceKey) IsNil() bool {
	return k.Kind == 0 && k.Role == 0 && k.Index == 0
}

// View is the texture reference carried by draw commands. It has two states,
// mirroring GrSurfaceProxy's deferred vs instantiated forms:
//   - Deferred: Key is set → the view is resolved at flush time to the
//     current active instance (never a stale snapshot).
//   - Direct: Key is nil and Ref is set → a strong reference to a specific
//     resource (rc-owned temporaries); the resource is guaranteed alive
//     through the command's lifetime.
//
// Commands must Release any Direct Ref after consumption (frame end).
type View struct {
	Key SourceKey // nonzero → deferred resolution
	Ref Ref       // used when Key is nil
}

// IsNil reports whether the View is the zero value (no texture).
func (v View) IsNil() bool { return v.Key.IsNil() && v.Ref.IsNil() }

// Equals reports whether two Views reference the same logical texture:
// identical deferred keys or identical direct registry ids.
func (v View) Equals(o View) bool {
	if !v.Key.IsNil() || !o.Key.IsNil() {
		return v.Key == o.Key
	}
	if v.Ref.IsNil() || o.Ref.IsNil() {
		return v.Ref.IsNil() && o.Ref.IsNil()
	}
	return v.Ref.id == o.Ref.id && v.Ref.reg == o.Ref.reg
}

// RefID returns the direct-reference registry id (0 for deferred/nil views).
func (v View) RefID() uint32 {
	if v.Ref.IsNil() {
		return 0
	}
	return v.Ref.id
}

// ViewFromKey builds a deferred View resolved at flush time.
func ViewFromKey(k SourceKey) View { return View{Key: k} }

// ViewFromRef builds a direct (strongly referenced) View.
func ViewFromRef(r Ref) View { return View{Ref: r} }