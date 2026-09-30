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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// EGL vendor arbitration (libglvnd) picks whichever ICD answers first for
// an X11 display. On hybrid boxes that lottery can land on the software
// rasterizer (llvmpipe) while real GPUs sit idle — the window then renders
// but never presents (black screen, 2026-09-29). The standard mechanism
// is __EGL_VENDOR_LIBRARY_FILENAMES: a colon-separated list of vendor
// JSON files; only listed vendors are used. Same env-before-first-EGL-call
// shape as prime_linux.go (DRI_PRIME); explicit user values always win.
//
// high → NVIDIA ICD first
// low → Mesa ICD first
// unset → untouched
//
// ICD files are discovered by scanning the vendor dir and matching
// library_path (distro file names differ: 10_nvidia.json here, elsewhere
// nvidia.json etc.). Missing dir or no match → unset, fail open.
const glvndVendorDir = "/usr/share/glvnd/egl_vendor.d"

// vendorPinForPower maps policy to a __EGL_VENDOR_LIBRARY_FILENAMES value.
// Empty return = leave the env alone. dir is a parameter so tests can
func vendorPinForPower(power, eglVendorEnv, dir string) (string, bool) {
	if strings.TrimSpace(eglVendorEnv) != "" {
		return "", false
	}
	var want string
	switch strings.ToLower(strings.TrimSpace(power)) {
	case "high", "discrete", "dgpu":
		want = "nvidia"
	case "low", "integrated", "igpu":
		want = "mesa"
	default:
		return "", false
	}
	path := findVendorJSON(dir, want)
	if path == "" {
		return "", false
	}
	return path, true
}

type glvndICD struct {
	ICD struct {
		LibraryPath string `json:"library_path"`
	} `json:"ICD"`
}

func findVendorJSON(dir, want string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var icd glvndICD
		if err := json.Unmarshal(raw, &icd); err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(icd.ICD.LibraryPath), want) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func applyVendorPinForPower() {
	if v, ok := vendorPinForPower(
		os.Getenv("GPUI_POWER"),
		os.Getenv("__EGL_VENDOR_LIBRARY_FILENAMES"),
		glvndVendorDir,
	); ok {
		_ = os.Setenv("__EGL_VENDOR_LIBRARY_FILENAMES", v)
	}
}

// rendererIsSoftware reports whether a GL_RENDERER string is a software
// rasterizer (llvmpipe/softpipe/swrast). Used to verify a pinned vendor
// actually delivered hardware; detection only, never a hard failure.
func rendererIsSoftware(renderer string) bool {
	r := strings.ToLower(renderer)
	for _, sw := range []string{"llvmpipe", "softpipe", "swrast", "software rasterizer", "swiftshader"} {
		if strings.Contains(r, sw) {
			return true
		}
	}
	return false
}
