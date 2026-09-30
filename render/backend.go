// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package render

import (
	"fmt"
	"os"
	"strings"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"

	// Register the pure-Go GL backend so SelectBackend can create it.
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
)

// Backend picks which GPU implementation render creates.
// BackendNative is the default (WebGPU); BackendGo is the pure-Go GL path.
type Backend int

const (
	// BackendNative is the default WebGPU implementation.
	BackendNative Backend = iota
	// BackendGo is the pure-Go GL implementation.
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
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GPUI_BACKEND"))) {
	case "", "native":
		return BackendNative, nil
	case "go":
		return BackendGo, nil
	default:
		return BackendNative, fmt.Errorf("render: unknown GPUI_BACKEND %q (want native|go)", os.Getenv("GPUI_BACKEND"))
	}
}

// SelectBackend creates a hal.Instance for the wanted backend.
// want==BackendNative uses WebGPU; want==BackendGo uses the registered
// pure-Go GL backend. Callers: app layer (after NewApp via Config),
// window creation, or direct user code — creation funnels here so the
// branch stays in this one sentence.
func SelectBackend(want Backend) (hal.Instance, error) {
	desc := &hal.InstanceDescriptor{Backends: types.BackendsPrimary}
	switch want {
	case BackendGo:
		be, ok := hal.GetBackend(types.BackendGL)
		if !ok || be == nil {
			return nil, fmt.Errorf("render: Go GL backend not registered (BackendGL)")
		}
		inst, err := be.CreateInstance(desc)
		if err != nil {
			return nil, fmt.Errorf("render: Go GL CreateInstance: %w", err)
		}
		return inst, nil
	default:
		inst, err := webgpu.CreateInstance(desc)
		if err != nil {
			return nil, fmt.Errorf("render: CreateInstance: %w", err)
		}
		return inst, nil
	}
}
