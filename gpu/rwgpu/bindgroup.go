package rwgpu

import (
	"runtime"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
)

// BufferBindingLayout describes buffer binding properties.
//
// This type matches gputypes.BufferBindingLayout for cross-project compatibility.
// Used as a pointer field in BindGroupLayoutEntry; nil means "not a buffer binding".
type BufferBindingLayout = types.BufferBindingLayout

// SamplerBindingLayout describes sampler binding properties.
//
// This type matches gputypes.SamplerBindingLayout for cross-project compatibility.
// Used as a pointer field in BindGroupLayoutEntry; nil means "not a sampler binding".
type SamplerBindingLayout = types.SamplerBindingLayout

// TextureBindingLayout describes texture binding properties.
//
// This type matches gputypes.TextureBindingLayout for cross-project compatibility.
// Used as a pointer field in BindGroupLayoutEntry; nil means "not a texture binding".
type TextureBindingLayout = types.TextureBindingLayout

// StorageTextureBindingLayout describes storage texture binding properties.
//
// This type matches gputypes.StorageTextureBindingLayout for cross-project compatibility.
// Used as a pointer field in BindGroupLayoutEntry; nil means "not a storage texture binding".
type StorageTextureBindingLayout = types.StorageTextureBindingLayout

// BindGroupLayoutEntry describes a single binding in a bind group layout.
//
// Exactly one of Buffer, Sampler, Texture, or StorageTexture must be non-nil.
// This matches the gogpu/wgpu API for cross-project compatibility.
type BindGroupLayoutEntry struct {
	// Binding is the binding number (must match @binding in shader).
	Binding uint32
	// Visibility specifies which shader stages can access this binding.
	Visibility types.ShaderStage
	// BindingArraySize enables binding arrays when non-zero.
	BindingArraySize uint32
	// Buffer describes a buffer binding (nil if not a buffer binding).
	Buffer *BufferBindingLayout
	// Sampler describes a sampler binding (nil if not a sampler binding).
	Sampler *SamplerBindingLayout
	// Texture describes a texture binding (nil if not a texture binding).
	Texture *TextureBindingLayout
	// StorageTexture describes a storage texture binding (nil if not a storage texture binding).
	StorageTexture *StorageTextureBindingLayout
}

// BindGroupLayoutDescriptor describes a bind group layout.
type BindGroupLayoutDescriptor struct {
	Label   string
	Entries []BindGroupLayoutEntry
}

// =============================================================================
// Wire structs for FFI (with converted enum values and uint64 ShaderStage)
// wgpu-native uses uint64 for WGPUShaderStageFlags (via WGPUFlags typedef)
// =============================================================================

// bufferBindingLayoutWire is the FFI-compatible struct with wgpu-native enum values.
type bufferBindingLayoutWire struct {
	NextInChain      uintptr
	Type             uint32 // wgpu-native value (converted from gputypes)
	HasDynamicOffset Bool
	MinBindingSize   uint64
}

// samplerBindingLayoutWire is the FFI-compatible struct with wgpu-native enum values.
// Size: 16 bytes (8 + 4 + 4 padding) - must match C struct padding
type samplerBindingLayoutWire struct {
	NextInChain uintptr
	Type        uint32  // wgpu-native value
	_pad        [4]byte // padding to 8-byte alignment (C struct padding)
}

// textureBindingLayoutWire is the FFI-compatible struct with wgpu-native enum values.
// Size: 24 bytes (8 + 4 + 4 + 4 + 4 padding) - must match C struct padding
type textureBindingLayoutWire struct {
	NextInChain   uintptr
	SampleType    uint32  // wgpu-native value
	ViewDimension uint32  // wgpu-native value
	Multisampled  Bool    // 4 bytes
	_pad          [4]byte // padding to 8-byte alignment
}

// storageTextureBindingLayoutWire is the FFI-compatible struct with wgpu-native enum values.
// Size: 24 bytes (8 + 4 + 4 + 4 + 4 padding) - must match C struct padding
type storageTextureBindingLayoutWire struct {
	NextInChain   uintptr
	Access        uint32  // wgpu-native value
	Format        uint32  // wgpu-native value
	ViewDimension uint32  // wgpu-native value
	_pad          [4]byte // padding to 8-byte alignment
}

// bindGroupLayoutEntryWire is the FFI-compatible struct with converted enums.
// CRITICAL: Visibility is uint64 because wgpu-native defines WGPUShaderStageFlags as uint64!
type bindGroupLayoutEntryWire struct {
	NextInChain      uintptr
	Binding          uint32
	_pad             [4]byte // padding to align Visibility to 8 bytes
	Visibility       uint64  // WGPUShaderStageFlags = uint64 in wgpu-native!
	BindingArraySize uint32
	_pad2            [4]byte // padding to align Buffer to 8 bytes
	Buffer           bufferBindingLayoutWire
	Sampler          samplerBindingLayoutWire
	Texture          textureBindingLayoutWire
	StorageTexture   storageTextureBindingLayoutWire
}

// toWire converts a BindGroupLayoutEntry to its wire representation.
// Nil sub-layout pointers produce zero-value wire structs (BindingNotUsed sentinel).
func (e *BindGroupLayoutEntry) toWire() bindGroupLayoutEntryWire {
	wire := bindGroupLayoutEntryWire{
		Binding:          e.Binding,
		Visibility:       uint64(e.Visibility), // widen uint32 to uint64
		BindingArraySize: e.BindingArraySize,
	}
	if e.Buffer != nil {
		wire.Buffer = bufferBindingLayoutWire{
			Type:             toWGPUBufferBindingType(e.Buffer.Type),
			HasDynamicOffset: boolToWGPU(e.Buffer.HasDynamicOffset),
			MinBindingSize:   e.Buffer.MinBindingSize,
		}
	}
	if e.Sampler != nil {
		wire.Sampler = samplerBindingLayoutWire{
			Type: toWGPUSamplerBindingType(e.Sampler.Type),
		}
	}
	if e.Texture != nil {
		wire.Texture = textureBindingLayoutWire{
			SampleType:    toWGPUTextureSampleType(e.Texture.SampleType),
			ViewDimension: uint32(e.Texture.ViewDimension),
			Multisampled:  boolToWGPU(e.Texture.Multisampled),
		}
	}
	if e.StorageTexture != nil {
		wire.StorageTexture = storageTextureBindingLayoutWire{
			Access:        toWGPUStorageTextureAccess(e.StorageTexture.Access),
			Format:        uint32(e.StorageTexture.Format),
			ViewDimension: uint32(e.StorageTexture.ViewDimension),
		}
	}
	return wire
}

// bindGroupLayoutDescriptorWire is the FFI-compatible descriptor.
type bindGroupLayoutDescriptorWire struct {
	NextInChain uintptr
	Label       StringView
	EntryCount  uintptr
	Entries     uintptr // *bindGroupLayoutEntryWire
}

// BindGroupEntry describes a single binding in a bind group.
// Exactly one of Buffer, Sampler, or TextureView must be non-nil.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      *Buffer      // For buffer bindings (nil if not used)
	Offset      uint64       // Buffer offset (ignored for non-buffer bindings)
	Size        uint64       // Buffer binding size; 0 = whole buffer
	Sampler     *Sampler     // For sampler bindings (nil if not used)
	TextureView *TextureView // For texture view bindings (nil if not used)
}

const wgpuWholeSize = ^uint64(0)

// bindGroupEntryWire is the FFI-compatible C-layout struct for wgpu-native.
// CRITICAL: layout must match WGPUBindGroupEntry exactly.
// nextInChain(8)+binding(4)+pad(4)+buffer(8)+offset(8)+size(8)+sampler(8)+textureView(8) = 56 bytes.
type bindGroupEntryWire struct {
	NextInChain uintptr // *ChainedStruct
	Binding     uint32
	_pad        [4]byte // padding for FFI alignment
	Buffer      uintptr // WGPUBuffer (nullable)
	Offset      uint64
	Size        uint64
	Sampler     uintptr // WGPUSampler (nullable)
	TextureView uintptr // WGPUTextureView (nullable)
}

// toWire converts a BindGroupEntry to its FFI wire representation.
func (e *BindGroupEntry) toWire() bindGroupEntryWire {
	wire := bindGroupEntryWire{
		Binding: e.Binding,
		Offset:  e.Offset,
		Size:    e.Size,
	}
	if e.Buffer != nil {
		wire.Buffer = e.Buffer.handle
		if wire.Size == 0 {
			wire.Size = wgpuWholeSize
		}
	}
	if e.Sampler != nil {
		wire.Sampler = e.Sampler.handle
	}
	if e.TextureView != nil {
		wire.TextureView = e.TextureView.handle
	}
	return wire
}

// BindGroupDescriptor describes a bind group.
type BindGroupDescriptor struct {
	Label   string
	Layout  *BindGroupLayout
	Entries []BindGroupEntry
}

// bindGroupDescriptorWire is the FFI-compatible C-layout struct for wgpu-native.
type bindGroupDescriptorWire struct {
	NextInChain uintptr // *ChainedStruct
	Label       StringView
	Layout      uintptr // WGPUBindGroupLayout
	EntryCount  uintptr // size_t
	Entries     uintptr // *bindGroupEntryWire
}

// CreateBindGroupLayout creates a bind group layout.
// Entries are converted from gputypes to wgpu-native enum values before FFI call.
// Returns an error if the FFI call fails or the device/descriptor is nil.
func (d *Device) CreateBindGroupLayout(desc *BindGroupLayoutDescriptor) (*BindGroupLayout, error) {
	if err := prepareDeviceCall("CreateBindGroupLayout", d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, &WGPUError{Op: "CreateBindGroupLayout", Message: "descriptor is nil"}
	}

	var wireDesc bindGroupLayoutDescriptorWire
	wireDesc.Label = stringToStringView(desc.Label)
	wireDesc.EntryCount = uintptr(len(desc.Entries))

	// R7.6: stack-allocate layout entries for common small layouts (≤8).
	var wireEntries []bindGroupLayoutEntryWire
	var wireStack [8]bindGroupLayoutEntryWire
	if n := len(desc.Entries); n > 0 {
		if n <= len(wireStack) {
			wireEntries = wireStack[:n]
		} else {
			wireEntries = make([]bindGroupLayoutEntryWire, n)
		}
		for i := range desc.Entries {
			wireEntries[i] = desc.Entries[i].toWire()
		}
		wireDesc.Entries = uintptr(unsafe.Pointer(&wireEntries[0]))
	}

	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _ := call2(procDeviceCreateBindGroupLayout, d.handle, uintptr(unsafe.Pointer(&wireDesc)))
	runtime.KeepAlive(wireEntries)
	runtime.KeepAlive(wireDesc)
	runtime.KeepAlive(desc)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateBindGroupLayout", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "BindGroupLayout")
	return &BindGroupLayout{handle: handle, device: d.handle}, nil
}

// CreateBindGroupLayoutSimple creates a bind group layout with the given entries.
// Returns an error if the FFI call fails or the device is nil.
func (d *Device) CreateBindGroupLayoutSimple(entries []BindGroupLayoutEntry) (*BindGroupLayout, error) {
	return d.CreateBindGroupLayout(&BindGroupLayoutDescriptor{
		Entries: entries,
	})
}

// Release releases the bind group layout.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (bgl *BindGroupLayout) Release() {
	if bgl == nil {
		return
	}
	releaseNativeHandle(&bgl.handle, isOwnerDeviceLost(bgl.device), func(h uintptr) {
		procBindGroupLayoutRelease.Call(h) //nolint:errcheck
	})
}

// Handle returns the underlying handle.
func (bgl *BindGroupLayout) Handle() uintptr { return bgl.handle }

// CreateBindGroup creates a bind group.
// Returns an error if the FFI call fails or the device/descriptor is nil.
func (d *Device) CreateBindGroup(desc *BindGroupDescriptor) (*BindGroup, error) {
	if err := prepareDeviceCall("CreateBindGroup", d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, &WGPUError{Op: "CreateBindGroup", Message: "descriptor is nil"}
	}
	if desc.Layout == nil {
		return nil, &WGPUError{Op: "CreateBindGroup", Message: "layout is nil"}
	}

	// Convert Go-idiomatic entries to FFI wire entries.
	// R7.0: stack-allocate wire entries for the common small bind group (≤8).
	var wireEntries []bindGroupEntryWire
	var wireStack [8]bindGroupEntryWire
	var wireEntriesPtr uintptr
	if n := len(desc.Entries); n > 0 {
		if n <= len(wireStack) {
			wireEntries = wireStack[:n]
		} else {
			wireEntries = make([]bindGroupEntryWire, n)
		}
		for i := range desc.Entries {
			wireEntries[i] = desc.Entries[i].toWire()
		}
		wireEntriesPtr = uintptr(unsafe.Pointer(&wireEntries[0]))
	}

	wire := bindGroupDescriptorWire{
		Label:      stringToStringView(desc.Label),
		Layout:     desc.Layout.handle,
		EntryCount: uintptr(len(desc.Entries)),
		Entries:    wireEntriesPtr,
	}

	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _ := call2(procDeviceCreateBindGroup, d.handle, uintptr(unsafe.Pointer(&wire)))
	runtime.KeepAlive(wireEntries)
	runtime.KeepAlive(wire)
	runtime.KeepAlive(desc)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateBindGroup", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "BindGroup")
	return &BindGroup{handle: handle, device: d.handle}, nil
}

// CreateBindGroupSimple creates a bind group with the given entries.
// Returns an error if the FFI call fails or the device/layout is nil.
func (d *Device) CreateBindGroupSimple(layout *BindGroupLayout, entries []BindGroupEntry) (*BindGroup, error) {
	return d.CreateBindGroup(&BindGroupDescriptor{
		Layout:  layout,
		Entries: entries,
	})
}

// Release releases the bind group.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (bg *BindGroup) Release() {
	if bg == nil {
		return
	}
	releaseNativeHandle(&bg.handle, isOwnerDeviceLost(bg.device), func(h uintptr) {
		procBindGroupRelease.Call(h) //nolint:errcheck
	})
}

// Handle returns the underlying handle.
func (bg *BindGroup) Handle() uintptr { return bg.handle }

// BufferBindingEntry creates a BindGroupEntry for a buffer.
func BufferBindingEntry(binding uint32, buffer *Buffer, offset, size uint64) BindGroupEntry {
	return BindGroupEntry{
		Binding: binding,
		Buffer:  buffer,
		Offset:  offset,
		Size:    size,
	}
}

// TextureBindingEntry creates a BindGroupEntry for a texture view.
func TextureBindingEntry(binding uint32, textureView *TextureView) BindGroupEntry {
	return BindGroupEntry{
		Binding:     binding,
		TextureView: textureView,
	}
}

// SamplerBindingEntry creates a BindGroupEntry for a sampler.
func SamplerBindingEntry(binding uint32, sampler *Sampler) BindGroupEntry {
	return BindGroupEntry{
		Binding: binding,
		Sampler: sampler,
	}
}
