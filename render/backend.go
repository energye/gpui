//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// Backend picks which GPU implementation render creates.
// BackendNative is the default; BackendGo is the pure-Go GL path.
type Backend int

const (
	// BackendNative is the default WebGPU implementation.
	BackendNative Backend = iota
	// BackendGo is the pure-Go GL implementation (linux/windows only; other
	// targets are WebGPU-only and SelectBackend reports it unregistered).
	BackendGo
)

func (b Backend) String() string {
	switch b {
	case BackendGo:
		return "go"
	default:
		return "native"
	}
}

// ResolveBackend reads GPUI_BACKEND only (env wins over code).
// unset/empty/native → BackendNative; go → BackendGo.
// Anything else is an error (no silent substitution, no legacy values).
//
// NOTE: GPUI_BACKEND is also read by the wgpu-native layer
// (gpu/rwgpu applyInstanceEnv: gl|vulkan|primary|all|gl+vulkan) to narrow
// ITS OWN instance backends. The render-level values here (native|go)
// do not collide with those: render routes first in SelectBackend, and
// neither value is meaningful to rwgpu, so they must not leak into a
// wgpu-native instance descriptor.
func ResolveBackend() (Backend, error) {
	return ResolveBackendFor(BackendNative)
}

// ResolveBackendFor resolves the effective backend: GPUI_BACKEND env wins
// when set (even over an explicit code choice); otherwise the code choice
// stands. BackendNative (zero value) means default. Unknown env values
// error (no silent substitution, no legacy values).
func ResolveBackendFor(code Backend) (Backend, error) {
	if v := strings.TrimSpace(os.Getenv("GPUI_BACKEND")); v != "" {
		switch strings.ToLower(v) {
		case "native":
			return BackendNative, nil
		case "go":
			return BackendGo, nil
		default:
			return BackendNative, fmt.Errorf("render: unknown GPUI_BACKEND %q (want native|go)", os.Getenv("GPUI_BACKEND"))
		}
	}
	switch code {
	case BackendNative, BackendGo:
		return code, nil
	default:
		return BackendNative, fmt.Errorf("render: unknown Backend %d (want native|go)", int(code))
	}
}

// SelectBackend creates a hal.Instance for the wanted backend.
// want==BackendNative uses WebGPU; want==BackendGo uses the registered
// pure-Go GL backend. Callers: app layer (after NewApp via Config),
// window creation, or direct user code — creation funnels here so the
// branch stays in this one sentence.
func SelectBackend(want Backend) (hal.Instance, error) {
	want, err := ResolveBackendFor(want)
	if err != nil {
		return nil, err
	}
	switch want {
	case BackendGo:
		be, ok := hal.GetBackend(types.BackendGL)
		if !ok || be == nil {
			return nil, fmt.Errorf("render: Go GL backend not registered (BackendGL)")
		}
		inst, err := be.CreateInstance(&hal.InstanceDescriptor{Backends: types.BackendsPrimary})
		if err != nil {
			return nil, fmt.Errorf("render: Go GL CreateInstance: %w", err)
		}
		return inst, nil
	default:
		inst, err := createNativeInstance(types.BackendsPrimary)
		if err != nil {
			return nil, fmt.Errorf("render: CreateInstance: %w", err)
		}
		return inst, nil
	}
}

// createNativeInstance is the single webgpu.CreateInstance sentence in the
// repo — the backend-choice whitelist entry (PLAN 创建入口唯一; AGENTS
// 渲染层后端解耦). SelectBackend funnels present/external opens
// (BackendsPrimary, GPUI_BACKEND decides native|go); OpenStandaloneDevice
// funnels standalone probe/offscreen/compute opens through the strict
// Vulkan mask via device_open.go.
func createNativeInstance(mask types.Backends) (hal.Instance, error) {
	return webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: mask})
}
