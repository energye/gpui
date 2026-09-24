package types

// GWGPU compat: types required by gpu/gwgpu (ported from gogpu hal/gles).
// Sourced from github.com/gogpu/gputypes v0.8.0 (draw.go, geometry.go,
// ray_tracing.go); kept here so gwgpu needs no external gputypes dependency.

// Viewport describes viewport transformation parameters.
type Viewport struct {
	X        float32
	Y        float32
	Width    float32
	Height   float32
	MinDepth float32
	MaxDepth float32
}

// ScissorRect describes a scissor clipping rectangle.
type ScissorRect struct {
	X      uint32
	Y      uint32
	Width  uint32
	Height uint32
}

// DrawArgs describes parameters for a non-indexed draw call.
type DrawArgs struct {
	VertexCount   uint32
	InstanceCount uint32
	FirstVertex   uint32
	FirstInstance uint32
}

// DrawIndexedArgs describes parameters for an indexed draw call.
type DrawIndexedArgs struct {
	IndexCount    uint32
	InstanceCount uint32
	FirstIndex    uint32
	BaseVertex    int32
	FirstInstance uint32
}

// AccelerationStructureCopyMode determines the type of copy operation.
type AccelerationStructureCopyMode uint8

const (
	AccelerationStructureCopyModeClone AccelerationStructureCopyMode = iota
	AccelerationStructureCopyModeCompact
)

// AccelerationStructureFlags are optional flags for acceleration structure creation.
type AccelerationStructureFlags uint8

const (
	ASFlagAllowUpdate             AccelerationStructureFlags = 0x01
	ASFlagAllowCompaction         AccelerationStructureFlags = 0x02
	ASFlagPreferFastTrace         AccelerationStructureFlags = 0x04
	ASFlagPreferFastBuild         AccelerationStructureFlags = 0x08
	ASFlagLowMemory               AccelerationStructureFlags = 0x10
	ASFlagUseTransform            AccelerationStructureFlags = 0x20
	ASFlagAllowRayHitVertexReturn AccelerationStructureFlags = 0x40
)

// Contains returns true if all flags in other are set.
func (f AccelerationStructureFlags) Contains(other AccelerationStructureFlags) bool {
	return f&other == other
}

// AccelerationStructureGeometryFlags are per-geometry flags.
type AccelerationStructureGeometryFlags uint8

const (
	ASGeometryFlagOpaque                      AccelerationStructureGeometryFlags = 0x01
	ASGeometryFlagNoDuplicateAnyHitInvocation AccelerationStructureGeometryFlags = 0x02
)

// Contains returns true if all flags in other are set.
func (f AccelerationStructureGeometryFlags) Contains(other AccelerationStructureGeometryFlags) bool {
	return f&other == other
}
