//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package wrkit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// C1 (evidence/P2.6/c1-screen-grab-black-assessment.md): the 22 real windows
// that assert live-screen pixels used to carry 22 copies of grabClientArea.
// Two bugs made them fail under composed displays:
//
//   - xwininfo's window-id line is "xwininfo: Window id: 0x..."; cutting each
//     line at the FIRST colon yields the key "xwininfo", so "Window id" never
//     matched, winID stayed empty, and the preferred xwd -id branch was dead.
//   - ffmpeg x11grab captures the X ROOT region. Under rootless Xwayland the
//     Wayland compositor owns the surfaces, so the root is unpainted and the
//     grab is black. xwd -id <winID> reads the window's own backing store and
//     returns the real presented pixels (measured: 26万+ non-background px).
//
// This helper is the single true source: parse the id correctly, prefer
// xwd -id, fall back to x11grab only for real Xorg servers (or when forced),
// and report ErrScreenUnsupported so callers can record an explicit skip when
// the window has no X11 window at all (native Wayland, no xwininfo match).

// ErrScreenUnsupported marks "this display cannot provide live screen pixels".
// Callers must record screen_check_skipped with a reason, never a green and
// never a silent pass.
var ErrScreenUnsupported = errors.New("screen check unsupported: no X11 window for this title (native Wayland session)")

// ScreenUnsupported reports whether err is the explicit unsupported marker.
func ScreenUnsupported(err error) bool { return errors.Is(err, ErrScreenUnsupported) }

// GrabClientArea captures the live client area of the window titled title into
// outPath (PNG). It returns the client width/height, or ErrScreenUnsupported
// when the window has no X11 window (native Wayland session). The window must
// still be on screen: the snapshot is a synthetic Close repaint, so a correct
// snapshot over a black presented frame must still fail the screen check.
//
// Negative control: GPUI_C1_FORCE_X11GRAB=1 forces the x11grab fallback so a
// composed display still shows a black grab (the gate must stay red).
func GrabClientArea(title, outPath string) (int, int, error) {
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		return 0, 0, ErrScreenUnsupported
	}
	info := exec.Command("xwininfo", "-name", title)
	info.Env = append(os.Environ(), "LC_ALL=C") // stable field names
	raw, err := info.Output()
	if err != nil {
		// Native Wayland: the window is a Wayland surface, not an X11 window.
		return 0, 0, ErrScreenUnsupported
	}

	var x, y, w, h int
	winID := ""
	for _, ln := range strings.Split(string(raw), "\n") {
		s := strings.TrimSpace(ln)
		// "xwininfo: Window id: 0xa00001 ..." — the key sits AFTER the first
		// colon, so match the substring instead of cutting the whole line.
		if i := strings.Index(s, "Window id:"); i >= 0 {
			if f := strings.Fields(s[i+len("Window id:"):]); len(f) > 0 {
				winID = f[0]
			}
			continue
		}
		k, v, ok := strings.Cut(s, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		n, convErr := strconv.Atoi(v)
		if convErr != nil {
			continue
		}
		switch k {
		case "Absolute upper-left X":
			x = n
		case "Absolute upper-left Y":
			y = n
		case "Width":
			w = n
		case "Height":
			h = n
		}
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("xwininfo: no geometry for %q", title)
	}

	// Preferred: xwd by window id — occlusion-immune, and valid under
	// rootless Xwayland (unlike root-region x11grab).
	if os.Getenv("GPUI_C1_FORCE_X11GRAB") != "1" && winID != "" {
		if _, lookErr := exec.LookPath("xwd"); lookErr == nil {
			tmp := filepath.Join(os.TempDir(), fmt.Sprintf("gpui_screen_%d.xwd", os.Getpid()))
			defer os.Remove(tmp)
			dump := exec.Command("xwd", "-silent", "-id", winID, "-out", tmp)
			dump.Env = append(os.Environ(), "LC_ALL=C")
			if dump.Run() == nil {
				conv := exec.Command("ffmpeg", "-y", "-loglevel", "error",
					"-i", tmp, "-frames:v", "1", outPath)
				if out, convErr := conv.CombinedOutput(); convErr == nil {
					return w, h, nil
				} else {
					_ = out // fall through to x11grab
				}
			}
		}
	}

	// Fallback: grab the on-screen rectangle (real Xorg servers only).
	grab := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "x11grab", "-draw_mouse", "0",
		"-video_size", fmt.Sprintf("%dx%d", w, h),
		"-i", fmt.Sprintf("%s+%d,%d", disp, x, y),
		"-frames:v", "1", outPath)
	if out, grabErr := grab.CombinedOutput(); grabErr != nil {
		return 0, 0, fmt.Errorf("ffmpeg x11grab: %w: %s", grabErr, strings.TrimSpace(string(out)))
	}
	return w, h, nil
}
