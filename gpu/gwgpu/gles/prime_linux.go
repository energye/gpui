//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"os"
	"strings"
)

// primeEnvForPower maps the shared GPUI_POWER policy to X11 GL env.
//
// high → DRI_PRIME=1 (Mesa render-node select: Mesa 有效). The NVIDIA
// proprietary offload pair is deliberately unset: 闭源 NVIDIA 无效 — it
// only redirects GLX while our path is EGL, and on Xwayland it fails
// window creation with 0x3005 (NVIDIA EGL cannot present an Xwayland
// pixmap). Discrete selection on the proprietary stack rides P2-0 vendor
// Explicit user values win.
// Applies before the first EGL display init in this process.
func primeEnvForPower() (pairs [][2]string, apply bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GPUI_POWER"))) {
	case "high", "discrete", "dgpu":
	default:
		return nil, false
	}
	var out [][2]string
	if strings.TrimSpace(os.Getenv("DRI_PRIME")) == "" {
		out = append(out, [2]string{"DRI_PRIME", "1"})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func applyPrimeEnvForPower() {
	if pairs, ok := primeEnvForPower(); ok {
		for _, kv := range pairs {
			_ = os.Setenv(kv[0], kv[1])
		}
	}
}
