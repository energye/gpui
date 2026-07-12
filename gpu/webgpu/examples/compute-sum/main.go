// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Command compute-sum demonstrates a parallel reduction (sum) using a GPU
// compute shader. It uploads an array of uint32 values to the GPU, dispatches
// a compute shader that sums contiguous pairs, and reads back the partial
// results. The final summation is performed on the CPU.
//
// The example is headless (no window required) and works on any supported GPU.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"

	// Register all available GPU backends (Vulkan, DX12, GLES, Metal, etc.)
	_ "github.com/energye/gpui/gpu/webgpu/hal/allbackends"
)

// sumShaderWGSL performs pairwise addition: output[i] = input[2*i] + input[2*i+1].
// Each workgroup thread handles one output element.
const sumShaderWGSL = `
@group(0) @binding(0) var<storage, read> input: array<u32>;
@group(0) @binding(1) var<storage, read_write> output: array<u32>;

struct Params {
    count: u32,
}
@group(0) @binding(2) var<uniform> params: Params;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let i = id.x;
    if (i >= params.count) {
        return;
    }
    let a = input[2u * i];
    let b = input[2u * i + 1u];
    output[i] = a + b;
}
`

const (
	numElements    = 256
	outCount       = numElements / 2
	inputBufSize   = uint64(numElements * 4)
	outputBufSize  = uint64(outCount * 4)
	stagingBufSize = outputBufSize
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("FATAL: %v", err)
	}
}

func run() error {
	fmt.Println("=== Compute Shader: Parallel Sum ===")
	fmt.Println()

	device, cleanup, err := initDevice()
	if err != nil {
		return err
	}
	defer cleanup()

	inputData, cpuSum := prepareInput()
	fmt.Printf("4. Input: %d elements, CPU sum = %d\n", numElements, cpuSum)

	bufs, err := createBuffers(device, inputData)
	if err != nil {
		return err
	}
	defer bufs.release()

	ps, err := createPipeline(device, bufs)
	if err != nil {
		return err
	}
	defer ps.release()

	gpuSum, err := dispatchAndReadBack(device, ps, bufs)
	if err != nil {
		return err
	}

	return verify(cpuSum, gpuSum)
}

func initDevice() (*webgpu.Device, func(), error) {
	fmt.Print("1. Creating instance... ")
	instance, err := webgpu.CreateInstance(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateInstance: %w", err)
	}
	fmt.Println("OK")

	fmt.Print("2. Requesting adapter... ")
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		instance.Release()
		return nil, nil, fmt.Errorf("RequestAdapter: %w", err)
	}
	fmt.Printf("OK (%s)\n", adapter.Info().Name)

	fmt.Print("3. Creating device... ")
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		adapter.Release()
		instance.Release()
		return nil, nil, fmt.Errorf("RequestDevice: %w", err)
	}
	fmt.Println("OK")

	cleanup := func() {
		device.Release()
		adapter.Release()
		instance.Release()
	}
	return device, cleanup, nil
}

func prepareInput() ([]byte, uint32) {
	inputData := make([]byte, inputBufSize)
	var cpuSum uint32
	for i := uint32(0); i < numElements; i++ {
		binary.LittleEndian.PutUint32(inputData[i*4:], i+1)
		cpuSum += i + 1
	}
	return inputData, cpuSum
}

type bufferSet struct {
	input, output, staging, uniform *webgpu.Buffer
}

func (b *bufferSet) release() {
	b.uniform.Release()
	b.staging.Release()
	b.output.Release()
	b.input.Release()
}

func createBuffers(device *webgpu.Device, inputData []byte) (*bufferSet, error) {
	fmt.Print("5. Creating buffers... ")
	inputBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "input", Size: inputBufSize,
		Usage: webgpu.BufferUsageStorage | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create input buffer: %w", err)
	}
	outputBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "output", Size: outputBufSize,
		Usage: webgpu.BufferUsageStorage | webgpu.BufferUsageCopySrc,
	})
	if err != nil {
		return nil, fmt.Errorf("create output buffer: %w", err)
	}
	stagingBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "staging", Size: stagingBufSize,
		Usage: webgpu.BufferUsageCopyDst | webgpu.BufferUsageMapRead,
	})
	if err != nil {
		return nil, fmt.Errorf("create staging buffer: %w", err)
	}

	uniformData := make([]byte, 4)
	binary.LittleEndian.PutUint32(uniformData, outCount)
	uniformBuf, err := device.CreateBuffer(&webgpu.BufferDescriptor{
		Label: "params", Size: 4,
		Usage: webgpu.BufferUsageUniform | webgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create uniform buffer: %w", err)
	}

	if err := device.Queue().WriteBuffer(inputBuf, 0, inputData); err != nil {
		return nil, fmt.Errorf("write input buffer: %w", err)
	}
	if err := device.Queue().WriteBuffer(uniformBuf, 0, uniformData); err != nil {
		return nil, fmt.Errorf("write uniform buffer: %w", err)
	}
	fmt.Println("OK")

	return &bufferSet{input: inputBuf, output: outputBuf, staging: stagingBuf, uniform: uniformBuf}, nil
}

type pipelineSet struct {
	shader, bgLayout, plLayout interface{ Release() }
	bindGroup                  *webgpu.BindGroup
	pipeline                   *webgpu.ComputePipeline
}

func (p *pipelineSet) release() {
	p.pipeline.Release()
	p.plLayout.Release()
	p.bindGroup.Release()
	p.bgLayout.Release()
	p.shader.Release()
}

func createPipeline(device *webgpu.Device, bufs *bufferSet) (*pipelineSet, error) {
	fmt.Print("6. Creating compute pipeline... ")
	shader, err := device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "sum-shader", WGSL: sumShaderWGSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create shader: %w", err)
	}
	bgLayout, err := device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
		Label: "sum-bgl",
		Entries: []webgpu.BindGroupLayoutEntry{
			{Binding: 0, Visibility: webgpu.ShaderStageCompute, Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeReadOnlyStorage}},
			{Binding: 1, Visibility: webgpu.ShaderStageCompute, Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeStorage}},
			{Binding: 2, Visibility: webgpu.ShaderStageCompute, Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create bind group layout: %w", err)
	}
	bindGroup, err := device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label: "sum-bg", Layout: bgLayout,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: bufs.input, Size: inputBufSize},
			{Binding: 1, Buffer: bufs.output, Size: outputBufSize},
			{Binding: 2, Buffer: bufs.uniform, Size: 4},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create bind group: %w", err)
	}
	plLayout, err := device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "sum-pl", BindGroupLayouts: []*webgpu.BindGroupLayout{bgLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("create pipeline layout: %w", err)
	}
	pipeline, err := device.CreateComputePipeline(&webgpu.ComputePipelineDescriptor{
		Label: "sum-pipeline", Layout: plLayout, Module: shader, EntryPoint: "main",
	})
	if err != nil {
		return nil, fmt.Errorf("create compute pipeline: %w", err)
	}
	fmt.Println("OK")

	return &pipelineSet{
		shader: shader, bgLayout: bgLayout, plLayout: plLayout,
		bindGroup: bindGroup, pipeline: pipeline,
	}, nil
}

func dispatchAndReadBack(device *webgpu.Device, ps *pipelineSet, bufs *bufferSet) (uint32, error) {
	fmt.Print("7. Dispatching compute... ")
	encoder, err := device.CreateCommandEncoder(nil)
	if err != nil {
		return 0, fmt.Errorf("create encoder: %w", err)
	}
	pass, err := encoder.BeginComputePass(nil)
	if err != nil {
		return 0, fmt.Errorf("begin compute pass: %w", err)
	}
	pass.SetPipeline(ps.pipeline)
	pass.SetBindGroup(0, ps.bindGroup, nil)
	pass.Dispatch((outCount+63)/64, 1, 1)
	if err := pass.End(); err != nil {
		return 0, fmt.Errorf("end compute pass: %w", err)
	}
	encoder.CopyBufferToBuffer(bufs.output, 0, bufs.staging, 0, outputBufSize)
	cmdBuf, err := encoder.Finish()
	if err != nil {
		return 0, fmt.Errorf("finish encoder: %w", err)
	}
	if _, err := device.Queue().Submit(cmdBuf); err != nil {
		return 0, fmt.Errorf("submit: %w", err)
	}
	fmt.Println("OK")

	fmt.Print("8. Reading results... ")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bufs.staging.Map(ctx, webgpu.MapModeRead, 0, outputBufSize); err != nil {
		return 0, fmt.Errorf("map staging buffer: %w", err)
	}
	rng, err := bufs.staging.MappedRange(0, outputBufSize)
	if err != nil {
		_ = bufs.staging.Unmap()
		return 0, fmt.Errorf("staging MappedRange: %w", err)
	}
	resultBytes := rng.Bytes()
	var gpuSum uint32
	for i := 0; i < outCount; i++ {
		gpuSum += binary.LittleEndian.Uint32(resultBytes[i*4:])
	}
	if err := bufs.staging.Unmap(); err != nil {
		return 0, fmt.Errorf("unmap staging buffer: %w", err)
	}
	fmt.Println("OK")
	return gpuSum, nil
}

func verify(cpuSum, gpuSum uint32) error {
	fmt.Println()
	fmt.Printf("CPU reference sum: %d\n", cpuSum)
	fmt.Printf("GPU partial sum:   %d\n", gpuSum)

	if gpuSum == cpuSum {
		fmt.Println("PASS: GPU sum matches CPU reference")
		return nil
	}

	fmt.Printf("FAIL: mismatch (diff = %d)\n", int64(cpuSum)-int64(gpuSum))
	return fmt.Errorf("sum mismatch: GPU=%d, CPU=%d", gpuSum, cpuSum)
}
