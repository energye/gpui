package rwgpu

import (
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
)

// InstanceDescriptor configures instance creation.
// Matches the gogpu/wgpu API for cross-project compatibility.
//
// Pass nil to CreateInstance for default configuration (all primary backends enabled).
//
// Env (applied when using CreateInstance):
//
//	GPUI_BACKEND=gl|vulkan|primary|all|gl+vulkan
//	GPUI_VRAM_BUDGET_PCT=1..100  (wgpu-native memory budget %; expert-only)
type InstanceDescriptor struct {
	// Backends selects which GPU backends to enable.
	// Zero = All (wgpu-native default) unless env overrides.
	Backends types.Backends
	// Flags controls instance features like debug layers and validation.
	Flags types.InstanceFlags
	// BudgetForDeviceCreationPct is 1–100 memory budget percent for device
	// creation (Vulkan/DX12 via InstanceExtras). 0 = unset.
	BudgetForDeviceCreationPct uint8
	// BudgetForDeviceLossPct optional companion for device-loss reclaim.
	BudgetForDeviceLossPct uint8
	// XlibDisplay, when non-zero, is Display* for GL/X11 (WGPUNativeDisplayHandle).
	// Required for GPUI_BACKEND=gl with X11 surfaces ("gl not compatible with surface"
	// without this). Screen defaults to 0 when XlibScreen < 0.
	XlibDisplay uintptr
	// XlibScreen is the X11 screen number (DefaultScreen).
	XlibScreen int32
}

// instanceDescriptorWire is the FFI-compatible C-layout struct for wgpuCreateInstance.
// v29 layout: nextInChain(8)+requiredFeatureCount(8)+requiredFeatures(8)+requiredLimits(8) = 32 bytes.
type instanceDescriptorWire struct {
	NextInChain          uintptr // *ChainedStruct
	RequiredFeatureCount uintptr // size_t
	RequiredFeatures     uintptr // *InstanceFeatureName (const)
	RequiredLimits       uintptr // *InstanceLimits (const, nullable)
}

// instanceExtrasWire matches WGPUInstanceExtras on linux/amd64 (112 bytes).
// Offsets verified against lib/include/webgpu/wgpu.h via offsetof.
type instanceExtrasWire struct {
	ChainNext               uintptr  // 0
	ChainSType              uint32   // 8
	_pad12                  uint32   // 12
	Backends                uint64   // 16
	Flags                   uint64   // 24
	Dx12Compiler            uint32   // 32
	Gles3MinorVersion       uint32   // 36
	GLFenceBehaviour        uint32   // 40
	_pad44                  uint32   // 44
	DxcPathData             uintptr  // 48
	DxcPathLength           uintptr  // 56
	DxcMaxShaderModel       uint32   // 64
	Dx12PresentationSystem  uint32   // 68
	BudgetForDeviceCreation uintptr  // 72
	BudgetForDeviceLoss     uintptr  // 80
	DisplayHandle           [24]byte // 88
}

// InstanceLimits describes the limits required at instance creation.
// New in v29 — passed as RequiredLimits in instanceDescriptorWire.
type InstanceLimits struct {
	NextInChain          uintptr // *ChainedStruct (nullable)
	TimedWaitAnyMaxCount uint64
}

// Bool is a WebGPU boolean (uint32).
type Bool uint32

const (
	// False is the WebGPU boolean false value (0).
	False Bool = 0
	// True is the WebGPU boolean true value (1).
	True Bool = 1
)

// ChainedStruct is used for struct chaining (both input and output).
// In v29 ChainedStructOut was unified with ChainedStruct — use ChainedStruct everywhere.
type ChainedStruct struct {
	Next  uintptr // *ChainedStruct
	SType uint32
}

// ChainedStructOut is kept for backward compatibility.
// Deprecated: Use ChainedStruct. In v29 there is no separate ChainedStructOut in C header.
type ChainedStructOut = ChainedStruct

// CreateInstance creates a new WebGPU instance.
// Pass nil for default configuration (all primary backends enabled).
//
// When backends or budgets are set (or via GPUI_* env), chains WGPUInstanceExtras
// so GL-only / budget-limited instances are possible (Skia/Flutter-class control).
func CreateInstance(desc *InstanceDescriptor) (*Instance, error) {
	if err := checkInit(); err != nil {
		return nil, err
	}

	var local InstanceDescriptor
	if desc != nil {
		local = *desc
	}
	applyInstanceEnv(&local)

	useExtras := local.Backends != 0 || local.Flags != 0 ||
		local.BudgetForDeviceCreationPct > 0 || local.BudgetForDeviceLossPct > 0 ||
		local.XlibDisplay != 0

	var wire instanceDescriptorWire
	var extras instanceExtrasWire
	var budgetCreate, budgetLoss uint8

	if useExtras {
		extras.ChainSType = uint32(STypeInstanceExtras)
		if local.Backends != 0 {
			extras.Backends = backendsToInstanceBackend(local.Backends)
		}
		if local.Flags != 0 {
			extras.Flags = uint64(local.Flags)
		}
		if local.BudgetForDeviceCreationPct > 0 {
			budgetCreate = local.BudgetForDeviceCreationPct
			if budgetCreate > 100 {
				budgetCreate = 100
			}
			extras.BudgetForDeviceCreation = uintptr(unsafe.Pointer(&budgetCreate))
		}
		if local.BudgetForDeviceLossPct > 0 {
			budgetLoss = local.BudgetForDeviceLossPct
			if budgetLoss > 100 {
				budgetLoss = 100
			}
			extras.BudgetForDeviceLoss = uintptr(unsafe.Pointer(&budgetLoss))
		}
		if local.XlibDisplay != 0 {
			// WGPUNativeDisplayHandle: type(u32)+pad(u32)+display(*)+screen(i32)+pad
			type xlibHandle struct {
				Type    uint32
				_pad    uint32
				Display uintptr
				Screen  int32
				_pad2   int32
			}
			screen := local.XlibScreen
			if screen < 0 {
				screen = 0
			}
			h := xlibHandle{
				Type:    0x00000001, // WGPUNativeDisplayHandleType_Xlib
				Display: local.XlibDisplay,
				Screen:  screen,
			}
			// Copy into fixed [24]byte field.
			hb := (*[24]byte)(unsafe.Pointer(&h))
			copy(extras.DisplayHandle[:], hb[:])
			runtime.KeepAlive(h)
		}
		wire.NextInChain = uintptr(unsafe.Pointer(&extras))
	}

	var wirePtr uintptr
	if useExtras || desc != nil {
		// Non-nil desc historically passed empty wire; keep that when no extras.
		wirePtr = uintptr(unsafe.Pointer(&wire))
	}
	// Pure nil desc + no env → null pointer (native defaults).
	if desc == nil && !useExtras {
		wirePtr = 0
	}

	handle, _, _ := procCreateInstance.Call(wirePtr)
	runtime.KeepAlive(wire)
	runtime.KeepAlive(extras)
	runtime.KeepAlive(budgetCreate)
	runtime.KeepAlive(budgetLoss)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateInstance", Message: "failed to create instance"}
	}

	trackResource(handle, "Instance")
	return &Instance{handle: handle}, nil
}

func applyInstanceEnv(desc *InstanceDescriptor) {
	if desc == nil {
		return
	}
	switch os.Getenv("GPUI_BACKEND") {
	case "gl", "opengl", "gles":
		desc.Backends = types.BackendsGL
	case "vulkan":
		desc.Backends = types.BackendsVulkan
	case "primary":
		desc.Backends = types.BackendsPrimary
	case "all":
		desc.Backends = types.BackendsAll
	case "gl+vulkan", "low":
		desc.Backends = types.BackendsGL | types.BackendsVulkan
	}
	if v := os.Getenv("GPUI_VRAM_BUDGET_PCT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			desc.BudgetForDeviceCreationPct = uint8(n) //nolint:gosec
			if desc.BudgetForDeviceLossPct == 0 {
				desc.BudgetForDeviceLossPct = uint8(n) //nolint:gosec
			}
		}
	}
}

func backendsToInstanceBackend(b types.Backends) uint64 {
	if b == types.BackendsAll {
		return uint64(InstanceBackendAll)
	}
	if b == types.BackendsPrimary {
		return uint64(InstanceBackendPrimary)
	}
	if b == types.BackendsSecondary {
		return uint64(InstanceBackendSecondary)
	}
	var out uint64
	if b&types.BackendsVulkan != 0 {
		out |= uint64(InstanceBackendVulkan)
	}
	if b&types.BackendsGL != 0 {
		out |= uint64(InstanceBackendGL)
	}
	if b&types.BackendsMetal != 0 {
		out |= uint64(InstanceBackendMetal)
	}
	if b&types.BackendsDX12 != 0 {
		out |= uint64(InstanceBackendDX12)
	}
	if b&types.BackendsBrowserWebGPU != 0 {
		out |= uint64(InstanceBackendBrowserWebGPU)
	}
	return out
}

// Release releases the instance resources.
func (i *Instance) Release() {
	if i == nil {
		return
	}
	// Instance has no parent device; always release native when handle is live.
	releaseNativeHandle(&i.handle, false, func(h uintptr) {
		procInstanceRelease.Call(h) //nolint:errcheck
	})
}

// ProcessEvents processes pending async events.
func (i *Instance) ProcessEvents() {
	if i == nil || i.handle == 0 {
		return
	}
	procInstanceProcessEvents.Call(i.handle) //nolint:errcheck
}
