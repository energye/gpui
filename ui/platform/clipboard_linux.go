//go:build linux

package platform

import (
	"bytes"
	"os/exec"
	"strings"
)

// NewSystemClipboard returns the Linux system clipboard adapter (xclip/xsel + memory).
func NewSystemClipboard() Clipboard {
	return NewFallbackClipboard(&xclipPrimary{})
}

// xclipPrimary talks to the OS clipboard via CLI tools (no Xlib selection owner).
type xclipPrimary struct{}

func (xclipPrimary) ReadText() (string, bool) {
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "clipboard", "-o")
		out, err := cmd.Output()
		if err == nil {
			return string(out), true
		}
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command(path, "--clipboard", "--output")
		out, err := cmd.Output()
		if err == nil {
			return string(out), true
		}
	}
	return "", false
}

func (xclipPrimary) WriteText(s string) error {
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(s)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command(path, "--clipboard", "--input")
		cmd.Stdin = bytes.NewReader([]byte(s))
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return err
		}
	}
	return errXClipMissing
}

var errXClipMissing = errString("platform: xclip/xsel not available")

type errString string

func (e errString) Error() string { return string(e) }

// XClipAvailable reports whether a Linux clipboard CLI is on PATH.
func XClipAvailable() bool {
	if _, err := exec.LookPath("xclip"); err == nil {
		return true
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return true
	}
	return false
}

// NewXClipClipboard is an alias for NewSystemClipboard (compat with older call sites).
func NewXClipClipboard() Clipboard { return NewSystemClipboard() }
