//go:build !(js && wasm)

package webgpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

// CommandEncoder records GPU commands for later submission.
// On the wgpu-native backend, this wraps rwgpu CommandEncoder.
type CommandEncoder struct {
	r        *rwgpu.CommandEncoder
	device   *Device
	released bool
}

// BeginRenderPass begins a render pass.
// R7.0: convert attachments on the caller's stack so the common 1-color-target
// path does not allocate. Descriptors are only live for the duration of the
// rwgpu BeginRenderPass call (native copies immediately).
// Param is hal.RenderPassDescriptor (callers' keyed literals unchanged).
func (e *CommandEncoder) BeginRenderPass(desc *hal.RenderPassDescriptor) (*RenderPassEncoder, error) {
	if e.released {
		return nil, ErrReleased
	}
	var rDesc rwgpu.RenderPassDescriptor
	var stackCA [4]rwgpu.RenderPassColorAttachment
	var depthStack rwgpu.RenderPassDepthStencilAttachment
	if desc != nil {
		rDesc.Label = desc.Label
		n := len(desc.ColorAttachments)
		var cas []rwgpu.RenderPassColorAttachment
		if n <= len(stackCA) {
			cas = stackCA[:n]
		} else {
			cas = make([]rwgpu.RenderPassColorAttachment, n)
		}
		for i, ca := range desc.ColorAttachments {
			att := rwgpu.RenderPassColorAttachment{
				LoadOp:  ca.LoadOp,
				StoreOp: ca.StoreOp,
				ClearValue: rwgpu.Color{
					R: ca.ClearValue.R,
					G: ca.ClearValue.G,
					B: ca.ClearValue.B,
					A: ca.ClearValue.A,
				},
			}
			if ca.View != nil {
				if wv, ok := ca.View.(*TextureView); ok && wv != nil && wv.r != nil {
					att.View = wv.r
				}
			}
			if ca.ResolveTarget != nil {
				if wv, ok := ca.ResolveTarget.(*TextureView); ok && wv != nil && wv.r != nil {
					att.ResolveTarget = wv.r
				}
			}
			cas[i] = att
		}
		rDesc.ColorAttachments = cas
		if desc.DepthStencilAttachment != nil {
			dsa := desc.DepthStencilAttachment
			depthStack = rwgpu.RenderPassDepthStencilAttachment{
				DepthLoadOp:       dsa.DepthLoadOp,
				DepthStoreOp:      dsa.DepthStoreOp,
				DepthClearValue:   dsa.DepthClearValue,
				DepthReadOnly:     dsa.DepthReadOnly,
				StencilLoadOp:     dsa.StencilLoadOp,
				StencilStoreOp:    dsa.StencilStoreOp,
				StencilClearValue: dsa.StencilClearValue,
				StencilReadOnly:   dsa.StencilReadOnly,
			}
			if dsa.View != nil {
				if wv, ok := dsa.View.(*TextureView); ok && wv != nil && wv.r != nil {
					depthStack.View = wv.r
				}
			}
			rDesc.DepthStencilAttachment = &depthStack
		}
	}
	rp, err := e.r.BeginRenderPass(&rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to begin render pass: %w", err)
	}

	return &RenderPassEncoder{r: rp}, nil
}

// BeginComputePass begins a compute pass.
// Param is hal.ComputePassDescriptor (render passes Label only).
func (e *CommandEncoder) BeginComputePass(desc *hal.ComputePassDescriptor) (*ComputePassEncoder, error) {
	if e.released {
		return nil, ErrReleased
	}
	var rDesc *rwgpu.ComputePassDescriptor
	if desc != nil {
		rDesc = &rwgpu.ComputePassDescriptor{
			Label: desc.Label,
		}
	}

	rp, err := e.r.BeginComputePass(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to begin compute pass: %w", err)
	}

	return &ComputePassEncoder{r: rp}, nil
}

// CopyBufferToBuffer copies data between buffers.
// Implements hal.CommandEncoder (takes hal.Buffer interfaces, internal unpack).
func (e *CommandEncoder) CopyBufferToBuffer(src hal.Buffer, srcOffset uint64, dst hal.Buffer, dstOffset uint64, size uint64) {
	if e.released {
		return
	}
	wsrc, ok1 := src.(*Buffer)
	wdst, ok2 := dst.(*Buffer)
	if !ok1 || !ok2 || wsrc == nil || wdst == nil {
		return
	}
	e.r.CopyBufferToBuffer(wsrc.r, srcOffset, wdst.r, dstOffset, size)
}

// CopyBufferToTexture copies data from a buffer to a texture.
// Implements hal.CommandEncoder (BufferTextureCopy is aliased to hal; region
// .TextureBase.Texture is hal.Texture, unpacked per region).
func (e *CommandEncoder) CopyBufferToTexture(src hal.Buffer, dst hal.Texture, regions []hal.BufferTextureCopy) {
	if e.released {
		return
	}
	wsrc, ok1 := src.(*Buffer)
	wdst, ok2 := dst.(*Texture)
	if !ok1 || !ok2 || wsrc == nil || wdst == nil {
		return
	}
	for _, r := range regions {
		tex, ok := r.TextureBase.Texture.(*Texture)
		if !ok || tex == nil || tex.r == nil {
			continue
		}
		rSrc := &rwgpu.TexelCopyBufferInfo{
			Buffer: wsrc.r.Handle(),
			Layout: rwgpu.TexelCopyBufferLayout{
				Offset:       r.BufferLayout.Offset,
				BytesPerRow:  r.BufferLayout.BytesPerRow,
				RowsPerImage: r.BufferLayout.RowsPerImage,
			},
		}
		rDst := &rwgpu.TexelCopyTextureInfo{
			Texture:  wdst.r.Handle(),
			MipLevel: r.TextureBase.MipLevel,
			Origin:   rwgpu.Origin3D{X: r.TextureBase.Origin.X, Y: r.TextureBase.Origin.Y, Z: r.TextureBase.Origin.Z},
		}
		rSize := &rwgpu.Extent3D{
			Width:              r.Size.Width,
			Height:             r.Size.Height,
			DepthOrArrayLayers: r.Size.DepthOrArrayLayers,
		}
		e.r.CopyBufferToTexture(rSrc, rDst, rSize)
	}
}

// ClearBuffer clears a buffer region to zero.
// Implements hal.CommandEncoder (takes hal.Buffer interface, internal unpack).
func (e *CommandEncoder) ClearBuffer(buffer hal.Buffer, offset, size uint64) {
	if e.released {
		return
	}
	wb, ok := buffer.(*Buffer)
	if !ok || wb == nil {
		return
	}
	e.r.ClearBuffer(wb.r, offset, size)
}

// CopyTextureToBuffer copies data from a texture to a buffer.
// Implements hal.CommandEncoder (BufferTextureCopy is aliased to hal; region
// .TextureBase.Texture is hal.Texture, unpacked per region).
func (e *CommandEncoder) CopyTextureToBuffer(src hal.Texture, dst hal.Buffer, regions []hal.BufferTextureCopy) {
	if e.released {
		return
	}
	wsrc, ok1 := src.(*Texture)
	wdst, ok2 := dst.(*Buffer)
	if !ok1 || !ok2 || wsrc == nil || wdst == nil {
		return
	}
	// R7.6: ≤4 regions on stack (glyph atlas / readback common case is 1).
	n := len(regions)
	var stack [4]rwgpu.BufferTextureCopy
	var rRegions []rwgpu.BufferTextureCopy
	if n <= len(stack) {
		rRegions = stack[:n]
	} else {
		rRegions = make([]rwgpu.BufferTextureCopy, n)
	}
	for i, r := range regions {
		tex, ok := r.TextureBase.Texture.(*Texture)
		if !ok || tex == nil || tex.r == nil {
			continue
		}
		rRegions[i] = rwgpu.BufferTextureCopy{
			BufferLayout: rwgpu.ImageDataLayout(r.BufferLayout),
			TextureBase: rwgpu.ImageCopyTexture{
				Texture:  tex.r,
				MipLevel: r.TextureBase.MipLevel,
				Origin:   rwgpu.Origin3D(r.TextureBase.Origin),
				Aspect:   rwgpu.TextureAspect(r.TextureBase.Aspect),
			},
			Size: rwgpu.Extent3D{Width: r.Size.Width, Height: r.Size.Height, DepthOrArrayLayers: r.Size.DepthOrArrayLayers},
		}
	}
	e.r.CopyTextureToBuffer(wsrc.r, wdst.r, rRegions)
}

// CopyTextureToTexture copies data between textures.
// Implements hal.CommandEncoder (TextureCopy is aliased to hal; region
// .Source/.Destination.Texture are hal.Texture, unpacked per region).
func (e *CommandEncoder) CopyTextureToTexture(src, dst hal.Texture, regions []hal.TextureCopy) {
	if e.released {
		return
	}
	wsrc, ok1 := src.(*Texture)
	wdst, ok2 := dst.(*Texture)
	if !ok1 || !ok2 || wsrc == nil || wdst == nil {
		return
	}
	// R7.6: ≤4 regions on stack.
	n := len(regions)
	var stack [4]rwgpu.TextureCopy
	var rRegions []rwgpu.TextureCopy
	if n <= len(stack) {
		rRegions = stack[:n]
	} else {
		rRegions = make([]rwgpu.TextureCopy, n)
	}
	for i, r := range regions {
		stex, ok1 := r.Source.Texture.(*Texture)
		dtex, ok2 := r.Destination.Texture.(*Texture)
		if !ok1 || !ok2 || stex == nil || dtex == nil || stex.r == nil || dtex.r == nil {
			continue
		}
		rRegions[i] = rwgpu.TextureCopy{
			Source: rwgpu.ImageCopyTexture{
				Texture:  stex.r,
				MipLevel: r.Source.MipLevel,
				Origin:   rwgpu.Origin3D(r.Source.Origin),
				Aspect:   rwgpu.TextureAspect(r.Source.Aspect),
			},
			Destination: rwgpu.ImageCopyTexture{
				Texture:  dtex.r,
				MipLevel: r.Destination.MipLevel,
				Origin:   rwgpu.Origin3D(r.Destination.Origin),
				Aspect:   rwgpu.TextureAspect(r.Destination.Aspect),
			},
			Size: rwgpu.Extent3D{Width: r.Size.Width, Height: r.Size.Height, DepthOrArrayLayers: r.Size.DepthOrArrayLayers},
		}
	}
	e.r.CopyTextureToTexture(wsrc.r, wdst.r, rRegions)
}

// TransitionTextures transitions texture states for synchronization.
// On the wgpu-native backend, this is a no-op. wgpu-native handles barriers internally.
// Implements hal.CommandEncoder (param is hal.TextureBarrier; body ignores barriers).
func (e *CommandEncoder) TransitionTextures(_ []hal.TextureBarrier) {
	// No-op: wgpu-native manages resource state transitions automatically.
}

// DiscardEncoding discards the encoder without producing a command buffer.
func (e *CommandEncoder) DiscardEncoding() {
	if e.released {
		return
	}
	e.released = true
	if e.r != nil {
		e.r.Release()
		e.r = nil
	}
}

// Finish completes command recording and returns a CommandBuffer.
// The native command encoder is released after Finish (wgpu refcounting);
// callers must still FreeCommandBuffer/Release the resulting CommandBuffer
// after GPU completion.
func (e *CommandEncoder) Finish() (*CommandBuffer, error) {
	if e.released {
		return nil, ErrReleased
	}
	e.released = true
	rcb, err := e.r.Finish()
	// Drop encoder ref regardless of Finish success — handle is consumed.
	if e.r != nil {
		e.r.Release()
		e.r = nil
	}
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to finish command encoder: %w", err)
	}

	return &CommandBuffer{r: rcb}, nil
}

// BeginEncoding implements hal.CommandEncoder: Rust has no Begin step, no-op.
func (e *CommandEncoder) BeginEncoding(_ string) error { return nil }

// EndEncoding implements hal.CommandEncoder: same as Finish.
func (e *CommandEncoder) EndEncoding() (hal.CommandBuffer, error) { return e.Finish() }

// ResetAll implements hal.CommandEncoder: Rust has no pooling, no-op.
func (e *CommandEncoder) ResetAll(_ []hal.CommandBuffer) {}

// Destroy implements hal.CommandEncoder: same as DiscardEncoding.
func (e *CommandEncoder) Destroy() { e.DiscardEncoding() }

// TransitionBuffers implements hal.CommandEncoder: no-op on Rust.
func (e *CommandEncoder) TransitionBuffers(_ []hal.BufferBarrier) {}

// ResolveQuerySet implements hal.CommandEncoder: no-op on Rust (no timestamp queries).
func (e *CommandEncoder) ResolveQuerySet(_ hal.QuerySet, _, _ uint32, _ hal.Buffer, _ uint64) {
}

// BuildAccelerationStructures implements hal.CommandEncoder: no-op on Rust.
func (e *CommandEncoder) BuildAccelerationStructures(_ []hal.BuildAccelerationStructureDescriptor) {
}

// PlaceAccelerationStructureBarrier implements hal.CommandEncoder: no-op on Rust.
func (e *CommandEncoder) PlaceAccelerationStructureBarrier(_ hal.AccelerationStructureBarrier) {
}

// CopyAccelerationStructure implements hal.CommandEncoder: no-op on Rust.
func (e *CommandEncoder) CopyAccelerationStructure(_, _ hal.AccelerationStructure, _ types.AccelerationStructureCopyMode) {
}

// ReadAccelerationStructureCompactSize implements hal.CommandEncoder: no-op on Rust.
func (e *CommandEncoder) ReadAccelerationStructureCompactSize(_ hal.AccelerationStructure, _ hal.Buffer, _ uint64) {
}

// CommandBuffer holds recorded GPU commands ready for submission.
// On the wgpu-native backend, this wraps rwgpu CommandBuffer.
type CommandBuffer struct {
	r         *rwgpu.CommandBuffer
	submitted bool
}

// Release drops the native command-buffer reference. Safe after Submit once
// the GPU has finished using it (or after Device.WaitIdle / next-frame drain).
func (cb *CommandBuffer) Release() {
	if cb == nil || cb.r == nil {
		return
	}
	cb.r.Release()
	cb.r = nil
}

// Destroy implements hal.CommandBuffer: same as Release.
func (cb *CommandBuffer) Destroy() { cb.Release() }

var _ hal.CommandBuffer = (*CommandBuffer)(nil)

// --- Render pass descriptor conversion ---

func convertRenderPassDescriptorRust(desc *hal.RenderPassDescriptor) *rwgpu.RenderPassDescriptor {
	if desc == nil {
		return &rwgpu.RenderPassDescriptor{}
	}

	rDesc := &rwgpu.RenderPassDescriptor{
		Label: desc.Label,
	}

	rDesc.ColorAttachments = make([]rwgpu.RenderPassColorAttachment, len(desc.ColorAttachments))
	for i, ca := range desc.ColorAttachments {
		att := rwgpu.RenderPassColorAttachment{
			LoadOp:  ca.LoadOp,
			StoreOp: ca.StoreOp,
			ClearValue: rwgpu.Color{
				R: ca.ClearValue.R,
				G: ca.ClearValue.G,
				B: ca.ClearValue.B,
				A: ca.ClearValue.A,
			},
		}
		if ca.View != nil {
			if wv, ok := ca.View.(*TextureView); ok && wv != nil && wv.r != nil {
				att.View = wv.r
			}
		}
		if ca.ResolveTarget != nil {
			if wv, ok := ca.ResolveTarget.(*TextureView); ok && wv != nil && wv.r != nil {
				att.ResolveTarget = wv.r
			}
		}
		rDesc.ColorAttachments[i] = att
	}

	if desc.DepthStencilAttachment != nil {
		dsa := desc.DepthStencilAttachment
		rDSA := &rwgpu.RenderPassDepthStencilAttachment{
			DepthLoadOp:       dsa.DepthLoadOp,
			DepthStoreOp:      dsa.DepthStoreOp,
			DepthClearValue:   dsa.DepthClearValue,
			DepthReadOnly:     dsa.DepthReadOnly,
			StencilLoadOp:     dsa.StencilLoadOp,
			StencilStoreOp:    dsa.StencilStoreOp,
			StencilClearValue: dsa.StencilClearValue,
			StencilReadOnly:   dsa.StencilReadOnly,
		}
		if dsa.View != nil {
			if wv, ok := dsa.View.(*TextureView); ok && wv != nil && wv.r != nil {
				rDSA.View = wv.r
			}
		}
		rDesc.DepthStencilAttachment = rDSA
	}

	return rDesc
}
