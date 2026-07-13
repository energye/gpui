// Package main demonstrates using the Buffer Introspection API.
package main

import (
	"fmt"
	"github.com/energye/gpui/gpu/rwgpu"
	"log"
)

func main() {
	// Create WebGPU instance
	instance, err := rwgpu.CreateInstance(nil)
	if err != nil {
		log.Fatalf("Failed to create instance: %v", err)
	}
	defer instance.Release()

	// Request adapter
	adapter, err := instance.RequestAdapter(&rwgpu.RequestAdapterOptions{
		PowerPreference: rwgpu.PowerPreferenceHighPerformance,
	})
	if err != nil {
		log.Fatalf("Failed to request adapter: %v", err)
	}
	defer adapter.Release()

	// Request device
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		log.Fatalf("Failed to request device: %v", err)
	}
	defer device.Release()

	// Create a storage buffer
	bufferSize := uint64(1024 * 1024) // 1MB
	buffer, err := device.CreateBuffer(&rwgpu.BufferDescriptor{
		Label: "",
		Usage: rwgpu.BufferUsageStorage | rwgpu.BufferUsageCopySrc | rwgpu.BufferUsageCopyDst,
		Size:  bufferSize,
	})
	if err != nil {
		log.Fatalf("create buffer: %v", err)
	}
	defer buffer.Release()

	// Demonstrate buffer introspection
	fmt.Println("=== Buffer Introspection ===")

	// Get buffer size
	size := buffer.Size()
	fmt.Printf("Buffer size: %d bytes (%.2f MB)\n", size, float64(size)/(1024*1024))

	// Get buffer usage
	usage := buffer.Usage()
	fmt.Printf("Buffer usage: %s\n", usageToString(usage))

	// Get buffer map state
	mapState := buffer.MapState()
	fmt.Printf("Buffer map state: %s\n", mapStateToString(mapState))

	// Create a mappable buffer
	fmt.Println("\n=== Mappable Buffer Example ===")
	mappableBuffer, err := device.CreateBuffer(&rwgpu.BufferDescriptor{
		Label:            "",
		Usage:            rwgpu.BufferUsageMapRead | rwgpu.BufferUsageCopyDst,
		Size:             1024,
		MappedAtCreation: true,
	})
	if err != nil {
		log.Fatalf("create mappable buffer: %v", err)
	}
	defer mappableBuffer.Release()

	// Check state when mapped at creation
	mapState = mappableBuffer.MapState()
	fmt.Printf("Initial map state (MappedAtCreation): %s\n", mapStateToString(mapState))

	// Unmap the buffer
	if err := mappableBuffer.Unmap(); err != nil {
		log.Printf("unmap: %v", err)
	}

	// Check state after unmap
	mapState = mappableBuffer.MapState()
	fmt.Printf("Map state after Unmap(): %s\n", mapStateToString(mapState))

	// Map async
	fmt.Println("\nMapping buffer asynchronously...")
	mapPending, mapErr := mappableBuffer.MapAsync(rwgpu.MapModeRead, 0, 1024)
	if mapErr != nil {
		log.Printf("MapAsync failed: %v", mapErr)
	} else {
		// Drive polling until resolved.
		for {
			if ready, _ := mapPending.Status(); ready {
				break
			}
			// In a real app, call device.Poll(false) here.
		}
		mapPending.Release()

		mapState = mappableBuffer.MapState()
		fmt.Printf("Map state after MapAsync(): %s\n", mapStateToString(mapState))

		// Unmap again
		mappableBuffer.Unmap() //nolint:errcheck
		mapState = mappableBuffer.MapState()
		fmt.Printf("Map state after final Unmap(): %s\n", mapStateToString(mapState))
	}

	fmt.Println("\n=== Buffer Lifecycle Demonstration ===")
	fmt.Println("Buffer introspection allows you to:")
	fmt.Println("- Query buffer size at runtime")
	fmt.Println("- Check which usage flags are set")
	fmt.Println("- Verify mapping state before operations")
	fmt.Println("- Debug buffer lifecycle issues")
}

func usageToString(usage rwgpu.BufferUsage) string {
	var flags []string

	if usage&rwgpu.BufferUsageMapRead != 0 {
		flags = append(flags, "MapRead")
	}
	if usage&rwgpu.BufferUsageMapWrite != 0 {
		flags = append(flags, "MapWrite")
	}
	if usage&rwgpu.BufferUsageCopySrc != 0 {
		flags = append(flags, "CopySrc")
	}
	if usage&rwgpu.BufferUsageCopyDst != 0 {
		flags = append(flags, "CopyDst")
	}
	if usage&rwgpu.BufferUsageIndex != 0 {
		flags = append(flags, "Index")
	}
	if usage&rwgpu.BufferUsageVertex != 0 {
		flags = append(flags, "Vertex")
	}
	if usage&rwgpu.BufferUsageUniform != 0 {
		flags = append(flags, "Uniform")
	}
	if usage&rwgpu.BufferUsageStorage != 0 {
		flags = append(flags, "Storage")
	}
	if usage&rwgpu.BufferUsageIndirect != 0 {
		flags = append(flags, "Indirect")
	}
	if usage&rwgpu.BufferUsageQueryResolve != 0 {
		flags = append(flags, "QueryResolve")
	}

	if len(flags) == 0 {
		return "None"
	}

	result := flags[0]
	for i := 1; i < len(flags); i++ {
		result += " | " + flags[i]
	}
	return result
}

func mapStateToString(state rwgpu.BufferMapState) string {
	switch state {
	case rwgpu.BufferMapStateUnmapped:
		return "Unmapped"
	case rwgpu.BufferMapStatePending:
		return "Pending"
	case rwgpu.BufferMapStateMapped:
		return "Mapped"
	default:
		return fmt.Sprintf("Unknown (%d)", state)
	}
}
